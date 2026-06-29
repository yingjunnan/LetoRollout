package config

import (
	"fmt"
	"os"
	"strings"
)

type Config struct {
	Addr                    string
	AdminToken             string
	TokensPath             string
	LogTailLines           int64
	AllowedNamespaces      []string
	RequiredDeploymentLabel string
	LocalPreview           bool
}

func Load() Config {
	addr := os.Getenv("ADDR")
	if addr == "" {
		addr = ":8080"
	}

	tokensPath := strings.TrimSpace(os.Getenv("TOKENS_PATH"))
	if tokensPath == "" {
		tokensPath = "/data/tokens.json"
	}

	return Config{
		Addr:                     addr,
		AdminToken:               os.Getenv("ADMIN_TOKEN"),
		TokensPath:               tokensPath,
		LogTailLines:             envIntOr("LOG_TAIL_LINES", 500),
		AllowedNamespaces:        splitCSV(os.Getenv("ALLOWED_NAMESPACES")),
		RequiredDeploymentLabel:  strings.TrimSpace(os.Getenv("REQUIRED_DEPLOYMENT_LABEL")),
		LocalPreview:             os.Getenv("LOCAL_PREVIEW") == "1",
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

func envIntOr(key string, def int64) int64 {
	if v := os.Getenv(key); v != "" {
		var n int64
		if _, err := fmt.Sscanf(v, "%d", &n); err == nil {
			return n
		}
	}
	return def
}
