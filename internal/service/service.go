package service

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"transforward/internal/auth"
	"transforward/internal/config"
)

const serviceName = "transforward"

func Install() error {
	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %v", err)
	}

	cfg := config.Get()
	if runtime.GOOS == "windows" {
		return installWindows(exePath, cfg.WebPort, "admin")
	}
	return installLinux(exePath, cfg.WebPort, "admin")
}

func Uninstall() error {
	if runtime.GOOS == "windows" {
		return uninstallWindows()
	}
	return uninstallLinux()
}

func installWindows(exePath string, port int, password string) error {
	installDir := GetInstallPath()

	// Create install directory if not exists
	if err := os.MkdirAll(installDir, 0755); err != nil {
		return fmt.Errorf("install requires administrator privileges: %v", err)
	}

	// Test write permission
	testFile := filepath.Join(installDir, ".write_test")
	if err := os.WriteFile(testFile, []byte{}, 0644); err != nil {
		return fmt.Errorf("install requires administrator privileges: insufficient permissions to write to %s", installDir)
	}
	os.Remove(testFile)

	// Copy binary to install directory
	exeName := filepath.Base(exePath)
	installExePath := filepath.Join(installDir, exeName)

	data, err := os.ReadFile(exePath)
	if err != nil {
		return fmt.Errorf("failed to read executable: %v", err)
	}

	if err := os.WriteFile(installExePath, data, 0755); err != nil {
		return fmt.Errorf("failed to write executable: %v", err)
	}

	// Create data directory and config in install directory
	dataDir := getDataDirFromExe(installExePath)
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return fmt.Errorf("failed to create data directory: %v", err)
	}

	// Generate default password hash
	passwordHash, err := auth.HashPassword(password)
	if err != nil {
		return fmt.Errorf("failed to hash password: %v", err)
	}

	// Write config
	cfgMap := map[string]interface{}{
		"web_port":      port,
		"password_hash": passwordHash,
		"rules":         []interface{}{},
		"log_level":     "info",
	}
	cfgPath := filepath.Join(dataDir, "config.json")
	if err := writeConfig(cfgPath, cfgMap); err != nil {
		return fmt.Errorf("failed to write config: %v", err)
	}

	// Create service
	cmd := exec.Command("sc.exe", "create", serviceName, "binPath=", installExePath, "DisplayName=", "TransForward Service")
	if err := cmd.Run(); err != nil {
		if strings.Contains(err.Error(), "already exists") {
			cmd = exec.Command("sc.exe", "config", serviceName, "binPath=", installExePath)
			return cmd.Run()
		}
		return err
	}

	cmd = exec.Command("sc.exe", "config", serviceName, "start=", "auto")
	if err := cmd.Run(); err != nil {
		return err
	}

	// Start service
	cmd = exec.Command("sc.exe", "start", serviceName)
	return cmd.Run()
}

func uninstallWindows() error {
	cmd := exec.Command("sc.exe", "stop", serviceName)
	cmd.Run()

	cmd = exec.Command("sc.exe", "delete", serviceName)
	return cmd.Run()
}

func installLinux(exePath string, port int, password string) error {
	installDir := GetInstallPath()
	if err := os.MkdirAll(installDir, 0755); err != nil {
		return fmt.Errorf("failed to create install directory: %v", err)
	}

	// Copy binary to install directory
	exeName := filepath.Base(exePath)
	installExePath := filepath.Join(installDir, exeName)

	data, err := os.ReadFile(exePath)
	if err != nil {
		return fmt.Errorf("failed to read executable: %v", err)
	}

	if err := os.WriteFile(installExePath, data, 0755); err != nil {
		return fmt.Errorf("failed to write executable: %v", err)
	}

	// Create data directory and config
	dataDir := getDataDirFromExe(installExePath)
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return fmt.Errorf("failed to create data directory: %v", err)
	}

	passwordHash, err := auth.HashPassword(password)
	if err != nil {
		return fmt.Errorf("failed to hash password: %v", err)
	}

	cfgMap := map[string]interface{}{
		"web_port":      port,
		"password_hash": passwordHash,
		"rules":         []interface{}{},
		"log_level":     "info",
	}
	cfgPath := filepath.Join(dataDir, "config.json")
	if err := writeConfig(cfgPath, cfgMap); err != nil {
		return fmt.Errorf("failed to write config: %v", err)
	}

	// Create systemd service file
	serviceContent := fmt.Sprintf(`[Unit]
Description=TransForward Service
After=network.target

[Service]
Type=simple
ExecStart=%s
Restart=on-failure
RestartSec=5

[Install]
WantedBy=multi-user.target
`, installExePath)

	servicePath := "/etc/systemd/system/" + serviceName + ".service"
	if err := os.WriteFile(servicePath, []byte(serviceContent), 0644); err != nil {
		return fmt.Errorf("failed to write service file: %v", err)
	}

	// Reload systemd and enable service
	for _, c := range []string{"systemctl", "daemon-reload"} {
		cmd := exec.Command(c)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to run %s: %v", c, err)
		}
	}

	var cmd *exec.Cmd
	cmd = exec.Command("systemctl", "enable", serviceName)
	if err := cmd.Run(); err != nil {
		return err
	}

	cmd = exec.Command("systemctl", "start", serviceName)
	return cmd.Run()
}

func uninstallLinux() error {
	cmd := exec.Command("systemctl", "stop", serviceName)
	cmd.Run()

	cmd = exec.Command("systemctl", "disable", serviceName)
	cmd.Run()

	servicePath := "/etc/systemd/system/" + serviceName + ".service"
	os.Remove(servicePath)
	return nil
}

func Start() error {
	if runtime.GOOS == "windows" {
		cmd := exec.Command("sc.exe", "start", serviceName)
		return cmd.Run()
	}
	cmd := exec.Command("systemctl", "start", serviceName)
	return cmd.Run()
}

func Stop() error {
	if runtime.GOOS == "windows" {
		cmd := exec.Command("sc.exe", "stop", serviceName)
		return cmd.Run()
	}
	cmd := exec.Command("systemctl", "stop", serviceName)
	return cmd.Run()
}

func Restart() error {
	if runtime.GOOS == "windows" {
		cmd := exec.Command("sc.exe", "stop", serviceName)
		cmd.Run()
		cmd = exec.Command("sc.exe", "start", serviceName)
		return cmd.Run()
	}
	cmd := exec.Command("systemctl", "restart", serviceName)
	return cmd.Run()
}

func GetInstallPath() string {
	if runtime.GOOS == "windows" {
		return filepath.Join(os.Getenv("ProgramFiles"), "TransForward")
	}
	return "/usr/local/bin"
}

func getDataDirFromExe(exePath string) string {
	exeName := filepath.Base(exePath)
	exeName = strings.TrimSuffix(exeName, ".exe")
	if idx := strings.LastIndex(exeName, "-"); idx > 0 {
		exeName = exeName[:idx]
	}
	dataDir := "." + exeName + "d"
	return filepath.Join(filepath.Dir(exePath), dataDir)
}

func writeConfig(path string, cfg map[string]interface{}) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}