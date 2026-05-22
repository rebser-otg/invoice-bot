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
	ForwardTo   string
	Senders     []string
	MessageText string
}

type yamlConfig struct {
	ForwardTo string `yaml:"forward_to"`
}

// Load reads config.yaml, senders.txt, and message.txt from dir.
func Load(dir string) (*Config, error) {
	yc, err := loadYAML(filepath.Join(dir, "config.yaml"))
	if err != nil {
		return nil, fmt.Errorf("config.yaml: %w", err)
	}
	senders, err := loadSenders(filepath.Join(dir, "senders.txt"))
	if err != nil {
		return nil, fmt.Errorf("senders.txt: %w", err)
	}
	if yc.ForwardTo == "" {
		return nil, fmt.Errorf("config.yaml: forward_to is required")
	}
	if len(senders) == 0 {
		return nil, fmt.Errorf("senders.txt: at least one sender address is required")
	}
	msg, err := os.ReadFile(filepath.Join(dir, "message.txt"))
	if err != nil {
		return nil, fmt.Errorf("message.txt: %w", err)
	}
	return &Config{
		ForwardTo:   yc.ForwardTo,
		Senders:     senders,
		MessageText: string(msg),
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
