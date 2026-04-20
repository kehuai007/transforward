package service

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

const serviceName = "transforward"

func Install() error {
	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %v", err)
	}

	if runtime.GOOS == "windows" {
		return installWindows(exePath)
	}
	return installLinux(exePath)
}

func Uninstall() error {
	if runtime.GOOS == "windows" {
		return uninstallWindows()
	}
	return uninstallLinux()
}

func installWindows(exePath string) error {
	installDir := GetInstallPath()

	// Check write permission before attempting install
	if err := os.MkdirAll(installDir, 0755); err != nil {
		return fmt.Errorf("install requires administrator privileges: %v", err)
	}

	// Test write permission by creating a temp file
	testFile := filepath.Join(installDir, ".write_test")
	if err := os.WriteFile(testFile, []byte{}, 0644); err != nil {
		return fmt.Errorf("install requires administrator privileges: insufficient permissions to write to %s", installDir)
	}
	os.Remove(testFile)

	// Copy binary to install directory
	exeName := filepath.Base(exePath)
	installExePath := filepath.Join(installDir, exeName)

	// Read current executable
	data, err := os.ReadFile(exePath)
	if err != nil {
		return fmt.Errorf("failed to read executable: %v", err)
	}

	// Write to install directory
	if err := os.WriteFile(installExePath, data, 0755); err != nil {
		return fmt.Errorf("failed to write executable: %v", err)
	}

	// Create service using sc.exe
	cmd := exec.Command("sc.exe", "create", serviceName, "binPath=", installExePath, "DisplayName=", "TransForward Service")
	if err := cmd.Run(); err != nil {
		if strings.Contains(err.Error(), "already exists") {
			cmd = exec.Command("sc.exe", "config", serviceName, "binPath=", installExePath)
			return cmd.Run()
		}
		return err
	}

	// Set auto-start
	cmd = exec.Command("sc.exe", "config", serviceName, "start=", "auto")
	return cmd.Run()
}

func uninstallWindows() error {
	// Stop service first
	cmd := exec.Command("sc.exe", "stop", serviceName)
	cmd.Run()

	// Delete service
	cmd = exec.Command("sc.exe", "delete", serviceName)
	return cmd.Run()
}

func installLinux(exePath string) error {
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
`, exePath)

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

	// Start service immediately
	cmd = exec.Command("systemctl", "start", serviceName)
	return cmd.Run()
}

func uninstallLinux() error {
	// Stop service
	cmd := exec.Command("systemctl", "stop", serviceName)
	cmd.Run()

	// Disable and remove service file
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
