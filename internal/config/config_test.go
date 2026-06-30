package config

import "testing"

func TestLoadUsesDefaults(t *testing.T) {
	t.Setenv("ADDR", "")
	t.Setenv("ADMIN_TOKEN", "")
	t.Setenv("TOKENS_PATH", "")
	t.Setenv("LOG_TAIL_LINES", "")
	t.Setenv("LOCAL_PREVIEW", "")

	cfg := Load()

	if cfg.Addr != ":8080" {
		t.Fatalf("Addr = %q, want :8080", cfg.Addr)
	}
	if cfg.AdminToken != "" {
		t.Fatalf("AdminToken = %q, want empty", cfg.AdminToken)
	}
	if cfg.TokensPath != "/data/tokens.json" {
		t.Fatalf("TokensPath = %q, want /data/tokens.json", cfg.TokensPath)
	}
	if cfg.LogTailLines != 500 {
		t.Fatalf("LogTailLines = %d, want 500", cfg.LogTailLines)
	}
	if cfg.LocalPreview {
		t.Fatalf("LocalPreview = %v, want false", cfg.LocalPreview)
	}
}

func TestLoadReadsEnvironment(t *testing.T) {
	t.Setenv("ADDR", ":9090")
	t.Setenv("ADMIN_TOKEN", "adm-secret")
	t.Setenv("TOKENS_PATH", "/var/lib/letorollout/tokens.json")
	t.Setenv("LOG_TAIL_LINES", "1234")
	t.Setenv("ALLOWED_NAMESPACES", "dev, staging,,prod")
	t.Setenv("REQUIRED_DEPLOYMENT_LABEL", "letorollout/enabled=true")
	t.Setenv("LOCAL_PREVIEW", "1")

	cfg := Load()

	if cfg.Addr != ":9090" {
		t.Fatalf("Addr = %q, want :9090", cfg.Addr)
	}
	if cfg.AdminToken != "adm-secret" {
		t.Fatalf("AdminToken = %q, want adm-secret", cfg.AdminToken)
	}
	if cfg.TokensPath != "/var/lib/letorollout/tokens.json" {
		t.Fatalf("TokensPath = %q, want /var/lib/letorollout/tokens.json", cfg.TokensPath)
	}
	if cfg.LogTailLines != 1234 {
		t.Fatalf("LogTailLines = %d, want 1234", cfg.LogTailLines)
	}
	if got, want := cfg.AllowedNamespaces, []string{"dev", "staging", "prod"}; !sameStrings(got, want) {
		t.Fatalf("AllowedNamespaces = %#v, want %#v", got, want)
	}
	if cfg.RequiredDeploymentLabel != "letorollout/enabled=true" {
		t.Fatalf("RequiredDeploymentLabel = %q, want letorollout/enabled=true", cfg.RequiredDeploymentLabel)
	}
	if !cfg.LocalPreview {
		t.Fatalf("LocalPreview = %v, want true", cfg.LocalPreview)
	}
}

func TestLoadLogTailLinesFallsBackOnInvalid(t *testing.T) {
	t.Setenv("LOG_TAIL_LINES", "not-a-number")

	cfg := Load()

	if cfg.LogTailLines != 500 {
		t.Fatalf("LogTailLines = %d, want 500 fallback", cfg.LogTailLines)
	}
}

func sameStrings(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}
