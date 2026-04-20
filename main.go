package main

import (
	"context"
	"embed"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
	"transforward/internal/api"
	"transforward/internal/auth"
	"transforward/internal/config"
	"transforward/internal/forward"
	"transforward/internal/service"

	"github.com/gorilla/mux"
)

//go:embed web
var webFS embed.FS

var (
	flagInstall   = flag.Bool("install", false, "Install service")
	flagUninstall = flag.Bool("uninstall", false, "Uninstall service")
	flagStart     = flag.Bool("start", false, "Start service")
	flagStop      = flag.Bool("stop", false, "Stop service")
	flagRestart   = flag.Bool("restart", false, "Restart service")
	flagReset     = flag.Bool("reset", false, "Reset password")
	flagVersion   = flag.Bool("version", false, "Show version")
	flagPort      = flag.Int("port", 0, "Custom web port (e.g., -port=8080)")
)

func main() {
	flag.Parse()

	cfgPath := config.GetConfigPath()
	if err := config.Load(cfgPath); err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Apply custom port if specified
	if *flagPort > 0 {
		config.Update(func(c *config.Config) {
			c.WebPort = *flagPort
		})
		config.Save(cfgPath)
		fmt.Printf("Port set to %d\n", *flagPort)
	}

	if *flagInstall {
		if err := service.Install(); err != nil {
			log.Fatalf("Install failed: %v", err)
		}
		fmt.Println("Service installed successfully")
		return
	}

	if *flagUninstall {
		if err := service.Uninstall(); err != nil {
			log.Fatalf("Uninstall failed: %v", err)
		}
		fmt.Println("Service uninstalled successfully")
		return
	}

	if *flagStart {
		if err := service.Start(); err != nil {
			log.Fatalf("Start failed: %v", err)
		}
		fmt.Println("Service started")
		return
	}

	if *flagStop {
		if err := service.Stop(); err != nil {
			log.Fatalf("Stop failed: %v", err)
		}
		fmt.Println("Service stopped")
		return
	}

	if *flagRestart {
		if err := service.Restart(); err != nil {
			log.Fatalf("Restart failed: %v", err)
		}
		fmt.Println("Service restarted")
		return
	}

	if *flagReset {
		handleReset()
		return
	}

	if *flagVersion {
		fmt.Println("transforward v1.0.0")
		return
	}

	runServer()
}

func handleReset() {
	fmt.Print("Enter new password: ")
	var password string
	fmt.Scanln(&password)
	if password == "" {
		fmt.Println("Password cannot be empty")
		return
	}

	if err := auth.SetPassword(password); err != nil {
		log.Fatalf("Failed to reset password: %v", err)
	}
	fmt.Println("Password reset successfully")
}

func runServer() {
	cfg := config.Get()

	if auth.NeedInit() {
		fmt.Println("=== First time setup ===")
		fmt.Print("Please set a password for web UI: ")
		var password string
		fmt.Scanln(&password)
		if err := auth.SetPassword(password); err != nil {
			log.Fatalf("Failed to set password: %v", err)
		}
		fmt.Println("Password set successfully")
	}

	engine := forward.NewEngine()
	api.SetEngine(engine)

	for _, r := range cfg.Rules {
		rule := forward.Rule{
			ID:       r.ID,
			Name:     r.Name,
			Protocol: forward.Protocol(r.Protocol),
			Listen:   r.Listen,
			Target:   r.Target,
			Enable:   r.Enable,
		}
		if err := engine.AddRule(&rule); err != nil {
			log.Printf("Failed to add rule %s: %v", r.ID, err)
		}
	}

	handler := api.NewHandler(engine)

	router := mux.NewRouter()
	router.Use(api.AuthMiddleware)

	router.HandleFunc("/api/login", handler.HandleLogin).Methods("POST")
	router.HandleFunc("/api/rules", handler.HandleRules).Methods("GET", "POST")
	router.HandleFunc("/api/rules/{id}", handler.HandleRules).Methods("PUT", "DELETE")
	router.HandleFunc("/api/rules/{id}/start", handler.HandleStartRule).Methods("POST")
	router.HandleFunc("/api/rules/{id}/stop", handler.HandleStopRule).Methods("POST")
	router.HandleFunc("/api/status", handler.HandleStatus).Methods("GET")
	router.HandleFunc("/api/password", handler.HandleChangePassword).Methods("PUT")
	router.HandleFunc("/api/config", handler.HandleConfig).Methods("GET", "PUT")
	router.HandleFunc("/ws", handler.HandleWebSocket)

	router.HandleFunc("/", serveIndex)
	router.HandleFunc("/dist/{path}", serveDist)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.WebPort),
		Handler: router,
	}

	go func() {
		log.Printf("Server starting on :%d", cfg.WebPort)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	<-quit
	log.Println("Shutting down...")

	// Give connected clients time to finish
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Printf("Server shutdown error: %v", err)
	}

	engine.Stop()
	log.Println("Server stopped")
}

func serveIndex(w http.ResponseWriter, r *http.Request) {
	data, err := webFS.ReadFile("web/index.html")
	if err != nil {
		http.Error(w, "Web UI not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(data)
}

func serveDist(w http.ResponseWriter, r *http.Request) {
	path := mux.Vars(r)["path"]
	data, err := webFS.ReadFile("web/" + path)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	ext := strings.TrimPrefix(path[strings.LastIndex(path, "."):], ".")
	contentTypes := map[string]string{
		"js":   "application/javascript",
		"css":  "text/css",
		"html": "text/html",
		"png":  "image/png",
		"jpg":  "image/jpeg",
		"svg":  "image/svg+xml",
	}

	if ct, ok := contentTypes[ext]; ok {
		w.Header().Set("Content-Type", ct)
	}

	w.Write(data)
}
