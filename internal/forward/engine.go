package forward

import (
	"context"
	"fmt"
	"log"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type ruleContext struct {
	ruleID    string
	bytesIn   uint64
	bytesOut  uint64
	active    int32
	totalConn uint64
	ctx       context.Context
	cancel    context.CancelFunc
	listeners sync.WaitGroup  // tracks active listeners
}

type Engine struct {
	rules     map[string]*Rule
	ruleStats map[string]*ruleContext
	mu        sync.RWMutex
	wg        sync.WaitGroup
	rootCtx   context.Context
	stopAll   context.CancelFunc
}

func NewEngine() *Engine {
	ctx, cancel := context.WithCancel(context.Background())
	return &Engine{
		rules:     make(map[string]*Rule),
		ruleStats: make(map[string]*ruleContext),
		rootCtx:   ctx,
		stopAll:   cancel,
	}
}

// AddRule adds a rule without starting it (Enable stays false)
func (e *Engine) AddRule(rule *Rule) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if rule.ID == "" {
		return fmt.Errorf("rule id is required")
	}
	if rule.Listen == "" || rule.Target == "" {
		return fmt.Errorf("listen and target are required")
	}
	if _, exists := e.rules[rule.ID]; exists {
		return fmt.Errorf("rule id already exists")
	}

	// Auto-fix: if listen/target is just a port, prepend :
	rule.Listen = fixAddr(rule.Listen)
	rule.Target = fixAddr(rule.Target)

	// Create context for this rule
	ruleCtx, cancel := context.WithCancel(context.Background())

	e.rules[rule.ID] = rule
	e.ruleStats[rule.ID] = &ruleContext{
		ruleID: rule.ID,
		ctx:    ruleCtx,
		cancel: cancel,
	}

	// Check port conflict with running rules before returning success
	if err := e.checkPortConflict(rule); err != nil {
		// Remove the rule we just added
		delete(e.rules, rule.ID)
		delete(e.ruleStats, rule.ID)
		return err
	}

	return nil
}

// StartRule starts listening for a rule (checks for conflicts)
func (e *Engine) StartRule(id string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	rule, ok := e.rules[id]
	if !ok {
		return fmt.Errorf("rule not found")
	}

	if rule.Enable {
		return fmt.Errorf("rule already started")
	}

	// Check for port conflicts
	if err := e.checkPortConflict(rule); err != nil {
		return err
	}

	// Create new context (in case rule was previously stopped)
	ruleCtx, cancel := context.WithCancel(context.Background())
	e.ruleStats[rule.ID].ctx = ruleCtx
	e.ruleStats[rule.ID].cancel = cancel

	// Start listening
	rule.Enable = true
	e.startListeners(rule)

	return nil
}

// StopRule stops listening for a rule
func (e *Engine) StopRule(id string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	rule, ok := e.rules[id]
	if !ok {
		return fmt.Errorf("rule not found")
	}

	if !rule.Enable {
		return nil // Already stopped
	}

	rule.Enable = false
	e.stopListeners(rule)

	return nil
}

// UpdateRule updates a rule, handles changes smartly
func (e *Engine) UpdateRule(rule *Rule) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	oldRule, ok := e.rules[rule.ID]
	if !ok {
		return fmt.Errorf("rule not found")
	}

	// Auto-fix: if listen/target is just a port, prepend :
	rule.Listen = fixAddr(rule.Listen)
	rule.Target = fixAddr(rule.Target)

	// If rule is running, handle changes
	if oldRule.Enable {
		listenChanged := rule.Listen != oldRule.Listen
		targetChanged := rule.Target != oldRule.Target

		if listenChanged || targetChanged {
			// Stop old listeners first and wait for them to fully close
			e.stopListeners(oldRule)
			// Create new context for the new listeners
			ruleCtx, cancel := context.WithCancel(context.Background())
			e.ruleStats[rule.ID].ctx = ruleCtx
			e.ruleStats[rule.ID].cancel = cancel
			// Preserve Enable state
			rule.Enable = oldRule.Enable
			e.rules[rule.ID] = rule
			e.startListeners(rule)
		} else {
			// Nothing changed: update anyway, preserve Enable
			rule.Enable = oldRule.Enable
			e.rules[rule.ID] = rule
		}
	} else {
		// Rule is stopped, just update data
		e.rules[rule.ID] = rule
	}

	return nil
}

// DeleteRule stops rule if running and removes it
func (e *Engine) DeleteRule(id string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	rule, ok := e.rules[id]
	if !ok {
		return fmt.Errorf("rule not found")
	}

	if rule.Enable {
		e.stopListeners(rule)
	}

	delete(e.rules, id)
	delete(e.ruleStats, id)

	return nil
}

func (e *Engine) checkPortConflict(rule *Rule) error {
	// Note: Caller must hold the lock. This function checks for port conflicts
	// with all existing rules (running or not).

	for _, r := range e.rules {
		if r.ID == rule.ID {
			continue // Skip ourselves
		}
		// Check if same port
		if r.Listen != rule.Listen {
			continue
		}
		// Same port - check protocol conflict
		if r.Protocol == "tcp+udp" || rule.Protocol == "tcp+udp" {
			// Any protocol on tcp+udp conflicts
			return fmt.Errorf("port conflict: another rule (%s) already using %s with protocol %s", r.ID, rule.Listen, r.Protocol)
		}
		if r.Protocol == rule.Protocol {
			// Same protocol on same port = conflict
			return fmt.Errorf("port conflict: rule %s already using %s with protocol %s", r.ID, rule.Listen, r.Protocol)
		}
		// Different protocols (tcp vs udp) on same port = conflict (we don't allow this)
		return fmt.Errorf("port conflict: rule %s already using %s with protocol %s", r.ID, rule.Listen, r.Protocol)
	}
	return nil
}

func (e *Engine) startListeners(rule *Rule) {
	stats := e.ruleStats[rule.ID]
	switch rule.Protocol {
	case "tcp":
		stats.listeners.Add(1)
		e.wg.Add(1)
		go e.runTCPListener(rule, stats)
	case "udp":
		stats.listeners.Add(1)
		e.wg.Add(1)
		go e.runUDPListener(rule, stats)
	case "tcp+udp":
		stats.listeners.Add(2)
		e.wg.Add(2)
		go e.runTCPListener(rule, stats)
		e.wg.Add(2)
		go e.runUDPListener(rule, stats)
	}
}

func (e *Engine) stopListeners(rule *Rule) {
	if ctx, ok := e.ruleStats[rule.ID]; ok {
		ctx.cancel()
		// Wait for all listeners to fully stop using WaitGroup
		ctx.listeners.Wait()
	}
}

func (e *Engine) GetRules() []*Rule {
	e.mu.RLock()
	defer e.mu.RUnlock()
	rules := make([]*Rule, 0, len(e.rules))
	for _, r := range e.rules {
		rules = append(rules, r)
	}
	return rules
}

func (e *Engine) GetStatus() *Status {
	e.mu.RLock()
	defer e.mu.RUnlock()

	status := &Status{
		TotalRules:    len(e.rules),
		RuleStats:     make([]RuleStats, 0, len(e.ruleStats)),
		TotalBytesIn:  0,
		TotalBytesOut: 0,
	}

	for _, ctx := range e.ruleStats {
		stats := RuleStats{
			RuleID:      ctx.ruleID,
			BytesIn:     atomic.LoadUint64(&ctx.bytesIn),
			BytesOut:    atomic.LoadUint64(&ctx.bytesOut),
			ActiveConns: int(atomic.LoadInt32(&ctx.active)),
			TotalConns:  atomic.LoadUint64(&ctx.totalConn),
		}
		status.RuleStats = append(status.RuleStats, stats)
		status.TotalBytesIn += stats.BytesIn
		status.TotalBytesOut += stats.BytesOut
		status.TotalConns += stats.TotalConns
		if e.rules[ctx.ruleID] != nil && e.rules[ctx.ruleID].Enable {
			status.ActiveRules++
		}
	}
	return status
}

func (e *Engine) runTCPListener(rule *Rule, stats *ruleContext) {
	defer e.wg.Done()
	defer stats.listeners.Done()

	ln, err := net.Listen("tcp", rule.Listen)
	if err != nil {
		log.Printf("[%s] TCP listen error: %v", rule.ID, err)
		return
	}
	defer ln.Close()

	log.Printf("[%s] TCP listening on %s -> %s", rule.ID, rule.Listen, rule.Target)

	for {
		select {
		case <-stats.ctx.Done():
			return
		default:
		}

		ln.(*net.TCPListener).SetDeadline(time.Now().Add(1e9))

		conn, err := ln.Accept()
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				continue
			}
			continue
		}

		atomic.AddUint64(&stats.totalConn, 1)
		atomic.AddInt32(&stats.active, 1)
		e.wg.Add(1)
		go e.handleTCP(rule, conn, stats)
	}
}

func (e *Engine) handleTCP(rule *Rule, src net.Conn, stats *ruleContext) {
	defer e.wg.Done()
	defer atomic.AddInt32(&stats.active, -1)
	defer src.Close()

	dst, err := net.DialTimeout("tcp", rule.Target, 5*time.Second)
	if err != nil {
		log.Printf("[%s] TCP dial error: %v", rule.ID, err)
		return
	}
	defer dst.Close()

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		defer dst.Close()
		buf := make([]byte, 32*1024)
		for {
			select {
			case <-stats.ctx.Done():
				return
			default:
			}
			n, err := src.Read(buf)
			if err != nil {
				return
			}
			if _, err := dst.Write(buf[:n]); err != nil {
				return
			}
			atomic.AddUint64(&stats.bytesIn, uint64(n))
		}
	}()

	go func() {
		defer wg.Done()
		defer src.Close()
		buf := make([]byte, 32*1024)
		for {
			select {
			case <-stats.ctx.Done():
				return
			default:
			}
			n, err := dst.Read(buf)
			if err != nil {
				return
			}
			if _, err := src.Write(buf[:n]); err != nil {
				return
			}
			atomic.AddUint64(&stats.bytesOut, uint64(n))
		}
	}()

	wg.Wait()
}

func (e *Engine) runUDPListener(rule *Rule, stats *ruleContext) {
	defer e.wg.Done()
	defer stats.listeners.Done()

	addr, err := net.ResolveUDPAddr("udp", rule.Listen)
	if err != nil {
		log.Printf("[%s] UDP resolve error: %v", rule.ID, err)
		return
	}

	ln, err := net.ListenUDP("udp", addr)
	if err != nil {
		log.Printf("[%s] UDP listen error: %v", rule.ID, err)
		return
	}
	defer ln.Close()

	log.Printf("[%s] UDP listening on %s -> %s", rule.ID, rule.Listen, rule.Target)

	buf := make([]byte, 32*1024)
	for {
		select {
		case <-stats.ctx.Done():
			return
		default:
		}

		ln.SetReadDeadline(time.Now().Add(1e9))
		n, clientAddr, err := ln.ReadFromUDP(buf)
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				continue
			}
			continue
		}

		atomic.AddUint64(&stats.bytesIn, uint64(n))

		targetAddr, err := net.ResolveUDPAddr("udp", rule.Target)
		if err != nil {
			continue
		}

		targetConn, err := net.DialUDP("udp", nil, targetAddr)
		if err != nil {
			continue
		}

		_, err = targetConn.Write(buf[:n])
		if err != nil {
			targetConn.Close()
			continue
		}
		atomic.AddUint64(&stats.bytesOut, uint64(n))

		targetConn.SetReadDeadline(time.Now().Add(5 * time.Second))
		n, err = targetConn.Read(buf)
		if err == nil && n > 0 {
			atomic.AddUint64(&stats.bytesIn, uint64(n))
			ln.WriteToUDP(buf[:n], clientAddr)
		}
		targetConn.Close()
	}
}

func (e *Engine) Stop() {
	e.mu.Lock()
	for _, rule := range e.rules {
		if rule.Enable {
			rule.Enable = false
			if ctx, ok := e.ruleStats[rule.ID]; ok {
				ctx.cancel()
			}
		}
	}
	e.mu.Unlock()

	e.stopAll()
	e.wg.Wait()
}

// fixAddr adds colon prefix if address is just a port number
func fixAddr(addr string) string {
	if strings.Contains(addr, ":") || strings.Contains(addr, ".") {
		return addr
	}
	return ":" + addr
}
