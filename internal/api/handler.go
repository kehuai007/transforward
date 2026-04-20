package api

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"
	"transforward/internal/auth"
	"transforward/internal/config"
	"transforward/internal/forward"

	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type Handler struct {
	engine *forward.Engine
	wsHub  *WSHub
}

func NewHandler(engine *forward.Engine) *Handler {
	return &Handler{
		engine: engine,
		wsHub:  NewWSHub(engine),
	}
}

func (h *Handler) HandleLogin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request"}`, http.StatusBadRequest)
		return
	}

	cfg := config.Get()
	if cfg.PasswordHash == "" {
		// First time setup
		if req.Password == "" {
			http.Error(w, `{"error":"password required"}`, http.StatusBadRequest)
			return
		}
		if err := auth.SetPassword(req.Password); err != nil {
			http.Error(w, `{"error":"failed to set password"}`, http.StatusInternalServerError)
			return
		}
		cfg = config.Get()
	}

	if !auth.CheckPassword(cfg.PasswordHash, req.Password) {
		http.Error(w, `{"error":"invalid password"}`, http.StatusUnauthorized)
		return
	}

	token, err := auth.GenerateToken()
	if err != nil {
		http.Error(w, `{"error":"failed to generate token"}`, http.StatusInternalServerError)
		return
	}
	validTokens[token] = true

	json.NewEncoder(w).Encode(map[string]string{"token": token})
}

func (h *Handler) HandleRules(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		rules := h.engine.GetRules()
		json.NewEncoder(w).Encode(rules)
	case http.MethodPost:
		var rule forward.Rule
		if err := json.NewDecoder(r.Body).Decode(&rule); err != nil {
			http.Error(w, `{"error":"invalid request"}`, http.StatusBadRequest)
			return
		}
		if err := h.engine.AddRule(&rule); err != nil {
			http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusBadRequest)
			return
		}
		saveRulesConfig()
		h.wsHub.Broadcast(forward.WSMessage{Type: "rules_updated", Data: nil})
		w.Write([]byte(`{"success":true}`))
	case http.MethodPut:
		var rule forward.Rule
		if err := json.NewDecoder(r.Body).Decode(&rule); err != nil {
			http.Error(w, `{"error":"invalid request"}`, http.StatusBadRequest)
			return
		}
		if err := h.engine.UpdateRule(&rule); err != nil {
			http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusBadRequest)
			return
		}
		saveRulesConfig()
		h.wsHub.Broadcast(forward.WSMessage{Type: "rules_updated", Data: nil})
		w.Write([]byte(`{"success":true}`))
	case http.MethodDelete:
		id := mux.Vars(r)["id"]
		if err := h.engine.DeleteRule(id); err != nil {
			http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusBadRequest)
			return
		}
		saveRulesConfig()
		h.wsHub.Broadcast(forward.WSMessage{Type: "rules_updated", Data: nil})
		w.Write([]byte(`{"success":true}`))
	}
}

func (h *Handler) HandleStatus(w http.ResponseWriter, r *http.Request) {
	status := h.engine.GetStatus()
	json.NewEncoder(w).Encode(status)
}

func (h *Handler) HandleStartRule(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	if err := h.engine.StartRule(id); err != nil {
		http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusBadRequest)
		return
	}
	saveRulesConfig()
	h.wsHub.Broadcast(forward.WSMessage{Type: "rules_updated", Data: nil})
	w.Write([]byte(`{"success":true}`))
}

func (h *Handler) HandleStopRule(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	if err := h.engine.StopRule(id); err != nil {
		http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusBadRequest)
		return
	}
	saveRulesConfig()
	h.wsHub.Broadcast(forward.WSMessage{Type: "rules_updated", Data: nil})
	w.Write([]byte(`{"success":true}`))
}

func (h *Handler) HandleChangePassword(w http.ResponseWriter, r *http.Request) {
	var req struct {
		OldPassword string `json:"old_password"`
		NewPassword string `json:"new_password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request"}`, http.StatusBadRequest)
		return
	}

	cfg := config.Get()
	if !auth.CheckPassword(cfg.PasswordHash, req.OldPassword) {
		http.Error(w, `{"error":"invalid old password"}`, http.StatusUnauthorized)
		return
	}

	if err := auth.SetPassword(req.NewPassword); err != nil {
		http.Error(w, `{"error":"failed to set password"}`, http.StatusInternalServerError)
		return
	}

	w.Write([]byte(`{"success":true}`))
}

func (h *Handler) HandleConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		cfg := config.Get()
		// Don't expose password hash
		cfgCopy := *cfg
		cfgCopy.PasswordHash = ""
		json.NewEncoder(w).Encode(cfgCopy)
	} else if r.Method == http.MethodPut {
		var req struct {
			WebPort  int    `json:"web_port"`
			LogLevel string `json:"log_level"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, `{"error":"invalid request"}`, http.StatusBadRequest)
			return
		}
		config.Update(func(c *config.Config) {
			if req.WebPort > 0 {
				c.WebPort = req.WebPort
			}
			if req.LogLevel != "" {
				c.LogLevel = req.LogLevel
			}
		})
		config.Save(config.GetConfigPath())
		w.Write([]byte(`{"success":true}`))
	}
}

func (h *Handler) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade error: %v", err)
		return
	}

	client := &WSClient{hub: h.wsHub, conn: conn, send: make(chan []byte, 256)}
	h.wsHub.register <- client

	go client.writePump()
	go client.readPump()
}

// WSHub maintains active WebSocket clients
type WSHub struct {
	engine    *forward.Engine
	clients   map[*WSClient]bool
	broadcast chan []byte
	register  chan *WSClient
	unregister chan *WSClient
	mu        sync.Mutex
}

func NewWSHub(engine *forward.Engine) *WSHub {
	hub := &WSHub{
		engine:   engine,
		clients:  make(map[*WSClient]bool),
		broadcast:  make(chan []byte, 256),
		register:   make(chan *WSClient),
		unregister: make(chan *WSClient),
	}
	go hub.run()
	return hub
}

func (h *WSHub) run() {
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			h.mu.Unlock()
		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
			}
			h.mu.Unlock()
		case message := <-h.broadcast:
			h.mu.Lock()
			for client := range h.clients {
				select {
				case client.send <- message:
				default:
					close(client.send)
					delete(h.clients, client)
				}
			}
			h.mu.Unlock()
		case <-ticker.C:
			// Broadcast status update to all clients
			if h.engine != nil {
				status := h.engine.GetStatus()
				msg := forward.WSMessage{Type: "status_update", Data: status}
				data, err := json.Marshal(msg)
				if err == nil {
					h.mu.Lock()
					for client := range h.clients {
						select {
						case client.send <- data:
						default:
							close(client.send)
							delete(h.clients, client)
						}
					}
					h.mu.Unlock()
				}
			}
		}
	}
}

func (h *WSHub) Broadcast(msg forward.WSMessage) {
	data, err := json.Marshal(msg)
	if err != nil {
		return
	}
	h.broadcast <- data
}

type WSClient struct {
	hub  *WSHub
	conn *websocket.Conn
	send chan []byte
}

func (c *WSClient) readPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()

	for {
		_, _, err := c.conn.ReadMessage()
		if err != nil {
			break
		}
	}
}

func (c *WSClient) writePump() {
	ticker := time.NewTicker(5 * time.Second)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.conn.WriteMessage(websocket.TextMessage, message); err != nil {
				return
			}
		case <-ticker.C:
			// Send ping
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func saveRulesConfig() {
	rules := engineInstance.GetRules()
	ruleConfigs := make([]config.RuleConfig, 0, len(rules))
	for _, r := range rules {
		ruleConfigs = append(ruleConfigs, config.RuleConfig{
			ID:       r.ID,
			Name:     r.Name,
			Protocol: string(r.Protocol),
			Listen:   r.Listen,
			Target:   r.Target,
			Enable:   r.Enable,
		})
	}
	config.Update(func(c *config.Config) {
		c.Rules = ruleConfigs
	})
	if err := config.Save(config.GetConfigPath()); err != nil {
		log.Printf("Failed to save rules config: %v", err)
	}
}

var engineInstance *forward.Engine

func SetEngine(e *forward.Engine) {
	engineInstance = e
}
