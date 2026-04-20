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
	// Create service using sc.exe
	cmd := exec.Command("sc.exe", "create", serviceName, "binPath=", exePath, "DisplayName=", "TransForward Service")
	if err := cmd.Run(); err != nil {
		// If service already exists, try to update
		if strings.Contains(err.Error(), "already exists") {
			cmd = exec.Command("sc.exe", "config", serviceName, "binPath=", exePath)
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
	cmds := []string{"systemctl", "daemon-reload"}
	for _, c := range cmds {
		cmd := exec.Command(c)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to run %s: %v", c, err)
		}
	}

	cmd := exec.Command("systemctl", "enable", serviceName)
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
