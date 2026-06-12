package config

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config holds all loaded configuration.
type Config struct {
	APIBaseURL string
	APIToken   string
	Senders    []string
}

type yamlConfig struct {
	APIBaseURL string `yaml:"api_base_url"`
	APIToken   string `yaml:"api_token"`
}

// Load reads config.yaml and senders.txt from dir. config.yaml holds the
// Office-Hub base URL + the Personal API Token (generate it under Profil →
// Sicherheit → API-Token). config.yaml is gitignored, so the token stays out
// of version control.
func Load(dir string) (*Config, error) {
	yc, err := loadYAML(filepath.Join(dir, "config.yaml"))
	if err != nil {
		return nil, fmt.Errorf("config.yaml: %w", err)
	}
	senders, err := loadSenders(filepath.Join(dir, "senders.txt"))
	if err != nil {
		return nil, fmt.Errorf("senders.txt: %w", err)
	}
	if yc.APIBaseURL == "" {
		return nil, fmt.Errorf("config.yaml: api_base_url is required")
	}
	if yc.APIToken == "" {
		return nil, fmt.Errorf("config.yaml: api_token is required")
	}
	if len(senders) == 0 {
		return nil, fmt.Errorf("senders.txt: at least one sender address is required")
	}
	return &Config{
		APIBaseURL: strings.TrimRight(yc.APIBaseURL, "/"),
		APIToken:   yc.APIToken,
		Senders:    senders,
	}, nil
}

func loadYAML(path string) (*yamlConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var yc yamlConfig
	if err := yaml.Unmarshal(data, &yc); err != nil {
		return nil, err
	}
	return &yc, nil
}

func loadSenders(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var senders []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		senders = append(senders, line)
	}
	return senders, scanner.Err()
}
