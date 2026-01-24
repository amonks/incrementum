package swarm

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveAddrFromConfig(t *testing.T) {
	repoPath := t.TempDir()
	configDir := filepath.Join(repoPath, ".incrementum")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatalf("create config dir: %v", err)
	}
	config := []byte("[swarm]\nport = 9001\n")
	if err := os.WriteFile(filepath.Join(configDir, "config.toml"), config, 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	addr, err := ResolveAddr(repoPath, "")
	if err != nil {
		t.Fatalf("resolve addr: %v", err)
	}
	if addr != "127.0.0.1:9001" {
		t.Fatalf("expected config port addr, got %q", addr)
	}
}

func TestResolveAddrUsesExplicitPort(t *testing.T) {
	addr, err := ResolveAddr(t.TempDir(), "9102")
	if err != nil {
		t.Fatalf("resolve addr: %v", err)
	}
	if addr != "127.0.0.1:9102" {
		t.Fatalf("expected explicit port addr, got %q", addr)
	}
}
