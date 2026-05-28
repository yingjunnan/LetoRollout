package config

import (
	"os"
	"strings"
)

type Config struct {
	Addr                    string
	AuthToken               string
	AllowedNamespaces       []string
	RequiredDeploymentLabel string
}

func Load() Config {
	addr := os.Getenv("ADDR")
	if addr == "" {
		addr = ":8080"
	}

	return Config{
		Addr:                    addr,
		AuthToken:               os.Getenv("AUTH_TOKEN"),
		AllowedNamespaces:       splitCSV(os.Getenv("ALLOWED_NAMESPACES")),
		RequiredDeploymentLabel: strings.TrimSpace(os.Getenv("REQUIRED_DEPLOYMENT_LABEL")),
	}
}

func splitCSV(value string) []string {
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}
