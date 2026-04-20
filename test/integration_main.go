package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const BASE_URL = "http://127.0.0.1:8081"
var client = &http.Client{Timeout: 5 * time.Second}

type Rule struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Protocol string `json:"protocol"`
	Listen   string `json:"listen"`
	Target   string `json:"target"`
	Enable   bool   `json:"enable"`
}

type Status struct {
	TotalRules    int `json:"total_rules"`
	ActiveRules   int `json:"active_rules"`
	TotalConns    int `json:"total_conns"`
	TotalBytesIn  int `json:"total_bytes_in"`
	TotalBytesOut int `json:"total_bytes_out"`
}

func login() string {
	req, _ := http.NewRequest("POST", BASE_URL+"/api/login", bytes.NewBuffer([]byte(`{"password":"admin"}`)))
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("Login request failed: %v\n", err)
		return ""
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		fmt.Printf("Login failed: %s\n", body)
		return ""
	}
	var result map[string]string
	json.Unmarshal(body, &result)
	token := result["token"]
	fmt.Printf("[LOGIN] Success, token: %s...\n", token[:20])
	return token
}

func apiRequest(method, path, token string, body interface{}) (*http.Response, string) {
	var bodyBytes []byte
	if body != nil {
		bodyBytes, _ = json.Marshal(body)
	}
	req, _ := http.NewRequest(method, BASE_URL+path, bytes.NewBuffer(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Sprintf("Request failed: %v", err)
	}
	respBody, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	return resp, string(respBody)
}

func getRules(token string) []Rule {
	_, body := apiRequest("GET", "/api/rules", token, nil)
	var rules []Rule
	json.Unmarshal([]byte(body), &rules)
	return rules
}

func getStatus(token string) *Status {
	_, body := apiRequest("GET", "/api/status", token, nil)
	var status Status
	json.Unmarshal([]byte(body), &status)
	return &status
}

func addRule(token string, rule Rule) error {
	resp, body := apiRequest("POST", "/api/rules", token, rule)
	if resp == nil {
		return fmt.Errorf("Add rule failed: %s", body)
	}
	if resp.StatusCode >= 400 {
		return fmt.Errorf("Add rule failed: %s", body)
	}
	fmt.Printf("[ADD] Rule %s (%s %s->%s) added\n", rule.Name, rule.Protocol, rule.Listen, rule.Target)
	return nil
}

func startRule(token, id string) error {
	resp, body := apiRequest("POST", "/api/rules/"+id+"/start", token, nil)
	if resp == nil {
		return fmt.Errorf("Start rule failed: %s", body)
	}
	if resp.StatusCode >= 400 {
		return fmt.Errorf("Start rule failed: %s", body)
	}
	fmt.Printf("[START] Rule %s started\n", id)
	return nil
}

func stopRule(token, id string) error {
	resp, body := apiRequest("POST", "/api/rules/"+id+"/stop", token, nil)
	if resp == nil {
		return fmt.Errorf("Stop rule failed: %s", body)
	}
	if resp.StatusCode >= 400 {
		return fmt.Errorf("Stop rule failed: %s", body)
	}
	fmt.Printf("[STOP] Rule %s stopped\n", id)
	return nil
}

func updateRule(token string, rule Rule) error {
	resp, body := apiRequest("PUT", "/api/rules/"+rule.ID, token, rule)
	if resp == nil {
		return fmt.Errorf("Update rule failed: %s", body)
	}
	if resp.StatusCode >= 400 {
		return fmt.Errorf("Update rule failed: %s", body)
	}
	fmt.Printf("[UPDATE] Rule %s updated\n", rule.ID)
	return nil
}

func deleteRule(token, id string) error {
	resp, body := apiRequest("DELETE", "/api/rules/"+id, token, nil)
	if resp == nil {
		return fmt.Errorf("Delete rule failed: %s", body)
	}
	if resp.StatusCode >= 400 {
		return fmt.Errorf("Delete rule failed: %s", body)
	}
	fmt.Printf("[DELETE] Rule %s deleted\n", id)
	return nil
}

// Test cases
func testAddAndStartStop() {
	fmt.Println("\n=== TEST: Add and Start/Stop ===")
	token := login()
	if token == "" {
		fmt.Println("ABORT: Cannot login")
		return
	}

	rule := Rule{
		ID:       "test_basic_1",
		Name:     "Basic TCP Test",
		Protocol: "tcp",
		Listen:   ":19999",
		Target:   "127.0.0.1:8081",
	}

	if err := addRule(token, rule); err != nil {
		fmt.Printf("FAIL: %v\n", err)
		return
	}

	if err := startRule(token, rule.ID); err != nil {
		fmt.Printf("FAIL: %v\n", err)
		return
	}

	time.Sleep(500 * time.Millisecond)
	status := getStatus(token)
	if status.ActiveRules != 1 {
		fmt.Printf("FAIL: Expected 1 active rule, got %d\n", status.ActiveRules)
		return
	}

	if err := stopRule(token, rule.ID); err != nil {
		fmt.Printf("FAIL: %v\n", err)
		return
	}

	time.Sleep(500 * time.Millisecond)
	status = getStatus(token)
	if status.ActiveRules != 0 {
		fmt.Printf("FAIL: Expected 0 active rules, got %d\n", status.ActiveRules)
		return
	}

	// Start again
	if err := startRule(token, rule.ID); err != nil {
		fmt.Printf("FAIL: %v\n", err)
		return
	}

	time.Sleep(500 * time.Millisecond)
	status = getStatus(token)
	if status.ActiveRules != 1 {
		fmt.Printf("FAIL: Expected 1 active rule after restart, got %d\n", status.ActiveRules)
		return
	}

	// Stop again
	if err := stopRule(token, rule.ID); err != nil {
		fmt.Printf("FAIL: %v\n", err)
		return
	}

	// Delete
	if err := deleteRule(token, rule.ID); err != nil {
		fmt.Printf("FAIL: %v\n", err)
		return
	}

	fmt.Println("PASS: Basic add/start/stop/start/stop/delete works\n")
}

func testTCPUDPPortConflict() {
	fmt.Println("\n=== TEST: TCP+UDP Port Conflict Detection ===")
	token := login()
	if token == "" {
		fmt.Println("ABORT: Cannot login")
		return
	}

	// Clean up any existing rules
	rules := getRules(token)
	for _, r := range rules {
		deleteRule(token, r.ID)
	}
	time.Sleep(300 * time.Millisecond)

	// Add TCP rule on port 19998
	tcpRule := Rule{
		ID:       "test_tcp_port",
		Name:     "TCP on 19998",
		Protocol: "tcp",
		Listen:   ":19998",
		Target:   "127.0.0.1:8081",
	}
	if err := addRule(token, tcpRule); err != nil {
		fmt.Printf("FAIL: %v\n", err)
		return
	}
	// Start TCP first
	if err := startRule(token, tcpRule.ID); err != nil {
		fmt.Printf("FAIL: %v\n", err)
		return
	}

	// Try to add UDP rule on same port - should fail
	udpRule := Rule{
		ID:       "test_udp_port",
		Name:     "UDP on 19998",
		Protocol: "udp",
		Listen:   ":19998",
		Target:   "127.0.0.1:8081",
	}
	resp, body := apiRequest("POST", "/api/rules", token, udpRule)
	if resp == nil || resp.StatusCode < 400 {
		fmt.Printf("FAIL: Should reject UDP on same port as TCP\n")
		stopRule(token, tcpRule.ID)
		deleteRule(token, tcpRule.ID)
		deleteRule(token, udpRule.ID)
		return
	}
	fmt.Printf("[EXPECTED] UDP on same port rejected: %s\n", body)

	// Try to add TCP+UDP rule on same port - should fail
	tcpUdpRule := Rule{
		ID:       "test_tcpudp_port",
		Name:     "TCP+UDP on 19998",
		Protocol: "tcp+udp",
		Listen:   ":19998",
		Target:   "127.0.0.1:8081",
	}
	resp, body = apiRequest("POST", "/api/rules", token, tcpUdpRule)
	if resp == nil || resp.StatusCode < 400 {
		fmt.Printf("FAIL: Should reject TCP+UDP on same port as TCP\n")
		stopRule(token, tcpRule.ID)
		deleteRule(token, tcpRule.ID)
		deleteRule(token, tcpUdpRule.ID)
		return
	}
	fmt.Printf("[EXPECTED] TCP+UDP on same port rejected: %s\n", body)

	// Clean up
	stopRule(token, tcpRule.ID)
	deleteRule(token, tcpRule.ID)
	fmt.Println("PASS: Port conflict detection works\n")
}

func testEditRunningRule() {
	fmt.Println("\n=== TEST: Edit Running Rule (Target Change) ===")
	token := login()
	if token == "" {
		fmt.Println("ABORT: Cannot login")
		return
	}

	// Clean up
	rules := getRules(token)
	for _, r := range rules {
		deleteRule(token, r.ID)
	}
	time.Sleep(300 * time.Millisecond)

	// Add and start TCP rule
	rule := Rule{
		ID:       "test_edit_target",
		Name:     "Edit Target Test",
		Protocol: "tcp",
		Listen:   ":19997",
		Target:   "127.0.0.1:8081",
	}
	if err := addRule(token, rule); err != nil {
		fmt.Printf("FAIL: %v\n", err)
		return
	}
	if err := startRule(token, rule.ID); err != nil {
		fmt.Printf("FAIL: %v\n", err)
		return
	}
	time.Sleep(500 * time.Millisecond)

	// Edit target while running
	updatedRule := Rule{
		ID:       rule.ID,
		Name:     "Edit Target Test",
		Protocol: "tcp",
		Listen:   ":19997",
		Target:   "127.0.0.1:9999", // Changed target
	}
	if err := updateRule(token, updatedRule); err != nil {
		fmt.Printf("FAIL: %v\n", err)
		return
	}
	time.Sleep(500 * time.Millisecond)

	status := getStatus(token)
	if status.ActiveRules != 1 {
		fmt.Printf("FAIL: Expected 1 active rule after target edit, got %d\n", status.ActiveRules)
		return
	}

	// Verify the target changed
	rules = getRules(token)
	for _, r := range rules {
		if r.ID == rule.ID {
			if r.Target != "127.0.0.1:9999" {
				fmt.Printf("FAIL: Target not updated, got %s\n", r.Target)
				return
			}
			break
		}
	}

	// Clean up
	stopRule(token, rule.ID)
	deleteRule(token, rule.ID)
	fmt.Println("PASS: Edit running rule (target change) works\n")
}

func testEditRunningRuleListenChange() {
	fmt.Println("\n=== TEST: Edit Running Rule (Listen Change) ===")
	token := login()
	if token == "" {
		fmt.Println("ABORT: Cannot login")
		return
	}

	// Clean up
	rules := getRules(token)
	for _, r := range rules {
		deleteRule(token, r.ID)
	}
	time.Sleep(300 * time.Millisecond)

	// Add and start TCP rule
	rule := Rule{
		ID:       "test_edit_listen",
		Name:     "Edit Listen Test",
		Protocol: "tcp",
		Listen:   ":19996",
		Target:   "127.0.0.1:8081",
	}
	if err := addRule(token, rule); err != nil {
		fmt.Printf("FAIL: %v\n", err)
		return
	}
	if err := startRule(token, rule.ID); err != nil {
		fmt.Printf("FAIL: %v\n", err)
		return
	}
	time.Sleep(500 * time.Millisecond)

	// Edit listen while running (should restart listener)
	updatedRule := Rule{
		ID:       rule.ID,
		Name:     "Edit Listen Test",
		Protocol: "tcp",
		Listen:   ":19995", // Changed listen port
		Target:   "127.0.0.1:8081",
	}
	if err := updateRule(token, updatedRule); err != nil {
		fmt.Printf("FAIL: %v\n", err)
		return
	}
	time.Sleep(800 * time.Millisecond) // Wait for listener restart

	status := getStatus(token)
	if status.ActiveRules != 1 {
		fmt.Printf("FAIL: Expected 1 active rule after listen edit, got %d\n", status.ActiveRules)
		return
	}

	// Verify the listen changed
	rules = getRules(token)
	for _, r := range rules {
		if r.ID == rule.ID {
			if r.Listen != ":19995" {
				fmt.Printf("FAIL: Listen not updated, got %s\n", r.Listen)
				return
			}
			break
		}
	}

	// Clean up
	stopRule(token, rule.ID)
	deleteRule(token, rule.ID)
	fmt.Println("PASS: Edit running rule (listen change) works\n")
}

func testTCPUDPCombined() {
	fmt.Println("\n=== TEST: TCP+UDP Combined Rule ===")
	token := login()
	if token == "" {
		fmt.Println("ABORT: Cannot login")
		return
	}

	// Clean up
	rules := getRules(token)
	for _, r := range rules {
		deleteRule(token, r.ID)
	}
	time.Sleep(300 * time.Millisecond)

	// Add TCP+UDP rule
	rule := Rule{
		ID:       "test_tcpudp",
		Name:     "TCP+UDP Test",
		Protocol: "tcp+udp",
		Listen:   ":19994",
		Target:   "127.0.0.1:8081",
	}
	if err := addRule(token, rule); err != nil {
		fmt.Printf("FAIL: %v\n", err)
		return
	}
	if err := startRule(token, rule.ID); err != nil {
		fmt.Printf("FAIL: %v\n", err)
		return
	}
	time.Sleep(500 * time.Millisecond)

	status := getStatus(token)
	if status.ActiveRules != 1 {
		fmt.Printf("FAIL: Expected 1 active rule for TCP+UDP, got %d\n", status.ActiveRules)
		return
	}

	// Edit target while running
	updatedRule := Rule{
		ID:       rule.ID,
		Name:     "TCP+UDP Test",
		Protocol: "tcp+udp",
		Listen:   ":19994",
		Target:   "127.0.0.1:9999",
	}
	if err := updateRule(token, updatedRule); err != nil {
		fmt.Printf("FAIL: %v\n", err)
		return
	}
	time.Sleep(800 * time.Millisecond)

	status = getStatus(token)
	if status.ActiveRules != 1 {
		fmt.Printf("FAIL: Expected 1 active rule after target edit, got %d\n", status.ActiveRules)
		return
	}

	// Stop and delete
	stopRule(token, rule.ID)
	deleteRule(token, rule.ID)
	fmt.Println("PASS: TCP+UDP combined rule works\n")
}

func testStopStartSameRule() {
	fmt.Println("\n=== TEST: Stop/Start Same Rule Multiple Times ===")
	token := login()
	if token == "" {
		fmt.Println("ABORT: Cannot login")
		return
	}

	// Clean up
	rules := getRules(token)
	for _, r := range rules {
		deleteRule(token, r.ID)
	}
	time.Sleep(300 * time.Millisecond)

	// Add rule
	rule := Rule{
		ID:       "test_stop_start",
		Name:     "Stop Start Test",
		Protocol: "tcp",
		Listen:   ":19993",
		Target:   "127.0.0.1:8081",
	}
	if err := addRule(token, rule); err != nil {
		fmt.Printf("FAIL: %v\n", err)
		return
	}

	// Cycle stop/start 3 times
	for i := 0; i < 3; i++ {
		if err := startRule(token, rule.ID); err != nil {
			fmt.Printf("FAIL: Start attempt %d failed: %v\n", i+1, err)
			return
		}
		time.Sleep(300 * time.Millisecond)

		if err := stopRule(token, rule.ID); err != nil {
			fmt.Printf("FAIL: Stop attempt %d failed: %v\n", i+1, err)
			return
		}
		time.Sleep(300 * time.Millisecond)
	}

	// Final start and verify
	if err := startRule(token, rule.ID); err != nil {
		fmt.Printf("FAIL: Final start failed: %v\n", err)
		return
	}
	time.Sleep(500 * time.Millisecond)

	status := getStatus(token)
	if status.ActiveRules != 1 {
		fmt.Printf("FAIL: Expected 1 active rule after cycling, got %d\n", status.ActiveRules)
		return
	}

	// Clean up
	stopRule(token, rule.ID)
	deleteRule(token, rule.ID)
	fmt.Println("PASS: Stop/Start cycle works\n")
}

func testDeleteRunningRule() {
	fmt.Println("\n=== TEST: Delete Running Rule ===")
	token := login()
	if token == "" {
		fmt.Println("ABORT: Cannot login")
		return
	}

	// Clean up
	rules := getRules(token)
	for _, r := range rules {
		deleteRule(token, r.ID)
	}
	time.Sleep(300 * time.Millisecond)

	// Add and start rule
	rule := Rule{
		ID:       "test_delete_running",
		Name:     "Delete Running Test",
		Protocol: "tcp",
		Listen:   ":19992",
		Target:   "127.0.0.1:8081",
	}
	if err := addRule(token, rule); err != nil {
		fmt.Printf("FAIL: %v\n", err)
		return
	}
	if err := startRule(token, rule.ID); err != nil {
		fmt.Printf("FAIL: %v\n", err)
		return
	}
	time.Sleep(500 * time.Millisecond)

	// Delete without stopping first
	if err := deleteRule(token, rule.ID); err != nil {
		fmt.Printf("FAIL: %v\n", err)
		return
	}
	time.Sleep(500 * time.Millisecond)

	status := getStatus(token)
	if status.TotalRules != 0 {
		fmt.Printf("FAIL: Expected 0 total rules after delete, got %d\n", status.TotalRules)
		return
	}

	fmt.Println("PASS: Delete running rule works\n")
}

func testEditStoppedRule() {
	fmt.Println("\n=== TEST: Edit Stopped Rule ===")
	token := login()
	if token == "" {
		fmt.Println("ABORT: Cannot login")
		return
	}

	// Clean up
	rules := getRules(token)
	for _, r := range rules {
		deleteRule(token, r.ID)
	}
	time.Sleep(300 * time.Millisecond)

	// Add rule (not started)
	rule := Rule{
		ID:       "test_edit_stopped",
		Name:     "Edit Stopped Test",
		Protocol: "tcp",
		Listen:   ":19991",
		Target:   "127.0.0.1:8081",
	}
	if err := addRule(token, rule); err != nil {
		fmt.Printf("FAIL: %v\n", err)
		return
	}

	// Edit while stopped
	updatedRule := Rule{
		ID:       rule.ID,
		Name:     "Edit Stopped Test Updated",
		Protocol: "tcp",
		Listen:   ":19990",
		Target:   "127.0.0.1:9999",
	}
	if err := updateRule(token, updatedRule); err != nil {
		fmt.Printf("FAIL: %v\n", err)
		return
	}

	// Start and verify
	if err := startRule(token, rule.ID); err != nil {
		fmt.Printf("FAIL: %v\n", err)
		return
	}
	time.Sleep(500 * time.Millisecond)

	status := getStatus(token)
	if status.ActiveRules != 1 {
		fmt.Printf("FAIL: Expected 1 active rule, got %d\n", status.ActiveRules)
		return
	}

	// Verify values
	rules = getRules(token)
	for _, r := range rules {
		if r.ID == rule.ID {
			if r.Listen != ":19990" || r.Target != "127.0.0.1:9999" {
				fmt.Printf("FAIL: Values not updated, listen=%s target=%s\n", r.Listen, r.Target)
				return
			}
			break
		}
	}

	// Clean up
	stopRule(token, rule.ID)
	deleteRule(token, rule.ID)
	fmt.Println("PASS: Edit stopped rule works\n")
}

func testAddDuplicateID() {
	fmt.Println("\n=== TEST: Add Duplicate ID ===")
	token := login()
	if token == "" {
		fmt.Println("ABORT: Cannot login")
		return
	}

	// Clean up
	rules := getRules(token)
	for _, r := range rules {
		deleteRule(token, r.ID)
	}
	time.Sleep(300 * time.Millisecond)

	rule := Rule{
		ID:       "test_duplicate_id",
		Name:     "Duplicate ID Test",
		Protocol: "tcp",
		Listen:   ":19989",
		Target:   "127.0.0.1:8081",
	}
	if err := addRule(token, rule); err != nil {
		fmt.Printf("FAIL: %v\n", err)
		return
	}

	// Try to add same ID again
	resp, body := apiRequest("POST", "/api/rules", token, rule)
	if resp == nil || resp.StatusCode < 400 {
		fmt.Printf("FAIL: Should reject duplicate ID\n")
		deleteRule(token, rule.ID)
		return
	}
	fmt.Printf("[EXPECTED] Duplicate ID rejected: %s\n", body)

	// Clean up
	deleteRule(token, rule.ID)
	fmt.Println("PASS: Duplicate ID detection works\n")
}

func main() {
	fmt.Println("========================================")
	fmt.Println("TransForward Integration Test Suite")
	fmt.Println("========================================")

	// Wait for server to be ready
	fmt.Println("\nWaiting for server...")
	time.Sleep(2 * time.Second)

	// Run all tests
	testAddAndStartStop()
	testTCPUDPPortConflict()
	testEditRunningRule()
	testEditRunningRuleListenChange()
	testTCPUDPCombined()
	testStopStartSameRule()
	testDeleteRunningRule()
	testEditStoppedRule()
	testAddDuplicateID()

	fmt.Println("========================================")
	fmt.Println("All tests completed")
	fmt.Println("========================================")
}