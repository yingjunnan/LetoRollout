package config

import "testing"

func TestLoadUsesDefaults(t *testing.T) {
	t.Setenv("ADDR", "")
	t.Setenv("AUTH_TOKEN", "")

	cfg := Load()

	if cfg.Addr != ":8080" {
		t.Fatalf("Addr = %q, want :8080", cfg.Addr)
	}
	if cfg.AuthToken != "" {
		t.Fatalf("AuthToken = %q, want empty", cfg.AuthToken)
	}
}

func TestLoadReadsEnvironment(t *testing.T) {
	t.Setenv("ADDR", ":9090")
	t.Setenv("AUTH_TOKEN", "secret")

	cfg := Load()

	if cfg.Addr != ":9090" {
		t.Fatalf("Addr = %q, want :9090", cfg.Addr)
	}
	if cfg.AuthToken != "secret" {
		t.Fatalf("AuthToken = %q, want secret", cfg.AuthToken)
	}
}
