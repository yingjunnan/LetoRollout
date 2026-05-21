package config

import "os"

type Config struct {
	Addr      string
	AuthToken string
}

func Load() Config {
	addr := os.Getenv("ADDR")
	if addr == "" {
		addr = ":8080"
	}

	return Config{
		Addr:      addr,
		AuthToken: os.Getenv("AUTH_TOKEN"),
	}
}
