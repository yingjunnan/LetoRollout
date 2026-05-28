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
	t.Setenv("ALLOWED_NAMESPACES", "dev, staging,,prod")
	t.Setenv("REQUIRED_DEPLOYMENT_LABEL", "letorollout/enabled=true")

	cfg := Load()

	if cfg.Addr != ":9090" {
		t.Fatalf("Addr = %q, want :9090", cfg.Addr)
	}
	if cfg.AuthToken != "secret" {
		t.Fatalf("AuthToken = %q, want secret", cfg.AuthToken)
	}
	if got, want := cfg.AllowedNamespaces, []string{"dev", "staging", "prod"}; !sameStrings(got, want) {
		t.Fatalf("AllowedNamespaces = %#v, want %#v", got, want)
	}
	if cfg.RequiredDeploymentLabel != "letorollout/enabled=true" {
		t.Fatalf("RequiredDeploymentLabel = %q, want letorollout/enabled=true", cfg.RequiredDeploymentLabel)
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
