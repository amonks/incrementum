package swarm

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/BurntSushi/toml"
	internalstrings "github.com/amonks/incrementum/internal/strings"
)

// DefaultPort is used when no port is configured.
const DefaultPort = 8088

// Config captures swarm settings from .incrementum/config.toml.
type Config struct {
	Swarm SwarmConfig `toml:"swarm"`
}

// SwarmConfig holds swarm-specific settings.
type SwarmConfig struct {
	Port int `toml:"port"`
}

// LoadConfig reads swarm settings from .incrementum/config.toml.
func LoadConfig(repoPath string) (SwarmConfig, error) {
	configPath := filepath.Join(repoPath, ".incrementum", "config.toml")
	data, err := os.ReadFile(configPath)
	if os.IsNotExist(err) {
		return SwarmConfig{}, nil
	}
	if err != nil {
		return SwarmConfig{}, fmt.Errorf("read swarm config: %w", err)
	}
	var cfg Config
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return SwarmConfig{}, fmt.Errorf("parse swarm config: %w", err)
	}
	return cfg.Swarm, nil
}

// ResolveAddr returns the swarm server address for the repo.
func ResolveAddr(repoPath, addr string) (string, error) {
	if !internalstrings.IsBlank(addr) {
		return normalizeAddr(addr)
	}
	cfg, err := LoadConfig(repoPath)
	if err != nil {
		return "", err
	}
	port := cfg.Port
	if port == 0 {
		port = DefaultPort
	}
	return fmt.Sprintf("127.0.0.1:%d", port), nil
}

func normalizeAddr(addr string) (string, error) {
	trimmed := strings.TrimSpace(addr)
	if trimmed == "" {
		return "", fmt.Errorf("address is required")
	}
	if strings.Contains(trimmed, ":") {
		return trimmed, nil
	}
	port, err := strconv.Atoi(trimmed)
	if err != nil {
		return "", fmt.Errorf("invalid port %q", trimmed)
	}
	if port <= 0 || port > 65535 {
		return "", fmt.Errorf("port out of range: %d", port)
	}
	return fmt.Sprintf("127.0.0.1:%d", port), nil
}
