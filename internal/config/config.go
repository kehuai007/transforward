package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

type Config struct {
	mu           sync.RWMutex
	WebPort      int          `json:"web_port"`
	PasswordHash string       `json:"password_hash"`
	Rules        []RuleConfig `json:"rules"`
	LogLevel     string       `json:"log_level"`
}

type RuleConfig struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Protocol string `json:"protocol"`
	Listen   string `json:"listen"`
	Target   string `json:"target"`
	Enable   bool   `json:"enable"`
}

var (
	cfg      *Config
	cfgMutex sync.RWMutex
)

func Get() *Config {
	cfgMutex.RLock()
	defer cfgMutex.RUnlock()
	return cfg
}

func Load(path string) error {
	cfgMutex.Lock()
	defer cfgMutex.Unlock()

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			cfg = &Config{
				WebPort:  8081,
				LogLevel: "info",
				Rules:    []RuleConfig{},
			}
			return save(path, cfg)
		}
		return err
	}

	cfg = &Config{}
	return json.Unmarshal(data, cfg)
}

func Save(path string) error {
	cfgMutex.RLock()
	defer cfgMutex.RUnlock()
	return save(path, cfg)
}

func Update(fn func(*Config)) {
	cfgMutex.Lock()
	defer cfgMutex.Unlock()
	fn(cfg)
}

func save(path string, c *Config) error {
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func GetDataDir() string {
	// 数据存储在运行目录下的 .<进程名> 目录
	exePath, err := os.Executable()
	if err != nil {
		// Fallback to HOME directory
		if onWindows() {
			return filepath.Join(os.Getenv("USERPROFILE"), ".transforward")
		}
		home := os.Getenv("HOME")
		if home == "" {
			home = "/root"
		}
		return filepath.Join(home, ".transforward")
	}

	exeName := filepath.Base(exePath)
	// 移除 .exe 扩展名
	exeName = strings.TrimSuffix(exeName, ".exe")
	// 驼峰化: transforward-windows-amd64 -> transforwardd
	dataDir := "." + camelize(exeName)

	return filepath.Join(filepath.Dir(exePath), dataDir)
}

func camelize(s string) string {
	// 简单驼峰化: 移除平台后缀
	// transforward-windows-amd64 -> transforwardd
	// transforward-linux-arm64 -> transforwardd
	if idx := strings.LastIndex(s, "-"); idx > 0 {
		s = s[:idx]
	}
	return s
}

func GetConfigPath() string {
	return filepath.Join(GetDataDir(), "config.json")
}

func onWindows() bool {
	return os.PathSeparator == '\\'
}
