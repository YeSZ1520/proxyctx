package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

const (
	defaultBenchmarkURL = "https://www.google.com/generate_204"
)

type Config struct {
	Proxies      []Proxy `yaml:"proxies"`
	Choise       string  `yaml:"choise"`
	Benchmark    string  `yaml:"benchmark"`
	BenchmarkURL string  `yaml:"benchmark-url"`
	TestURL      string  `yaml:"test-url"`
}

type Proxy struct {
	Name              string            `yaml:"name"`
	Type              string            `yaml:"type"`
	Server            string            `yaml:"server"`
	Port              int               `yaml:"port"`
	UUID              string            `yaml:"uuid"`
	AlterID           int               `yaml:"alterId"`
	Cipher            string            `yaml:"cipher"`
	TLS               bool              `yaml:"tls"`
	SkipCertVerify    bool              `yaml:"skip-cert-verify"`
	TFO               bool              `yaml:"tfo"`
	UDP               bool              `yaml:"udp"`
	Flow              string            `yaml:"flow"`
	ClientFingerprint string            `yaml:"client-fingerprint"`
	ServerName        string            `yaml:"servername"`
	Network           string            `yaml:"network"`
	WSOpts            *WebSocketOptions `yaml:"ws-opts"`
}

type WebSocketOptions struct {
	Path    string            `yaml:"path"`
	Headers map[string]string `yaml:"headers"`
}

func (c *Config) BenchmarkTarget() string {
	if c.Benchmark != "" {
		return c.Benchmark
	}
	if c.BenchmarkURL != "" {
		return c.BenchmarkURL
	}
	if c.TestURL != "" {
		return c.TestURL
	}
	return defaultBenchmarkURL
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	if len(cfg.Proxies) == 0 {
		return nil, errors.New("no proxies defined")
	}
	return &cfg, nil
}

func FindDefaultConfig() (string, error) {
	localPath := filepath.Join(".config", "proxyctx", "config.yaml")
	if fileExists(localPath) {
		return localPath, nil
	}

	homePath := ""
	homeDir, _ := os.UserHomeDir()
	if homeDir != "" {
		homePath = filepath.Join(homeDir, ".config", "proxyctx", "config.yaml")
		if fileExists(homePath) {
			return homePath, nil
		}
	}

	if homePath == "" {
		return "", fmt.Errorf("no config found: expected %q or %q", localPath, "~/.config/proxyctx/config.yaml")
	}
	return "", fmt.Errorf("no config found: expected %q or %q", localPath, homePath)
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !info.IsDir()
}
