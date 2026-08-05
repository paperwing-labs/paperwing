package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	Address        string
	DatabasePath   string
	AttachmentPath string
	ShutdownWait   time.Duration
}

func Load() (Config, error) {
	databasePath := env("PAPERWING_DATABASE_PATH", "./data/paperwing.db")
	cfg := Config{
		Address:        env("PAPERWING_ADDRESS", "127.0.0.1:8080"),
		DatabasePath:   databasePath,
		AttachmentPath: env("PAPERWING_ATTACHMENT_PATH", "./data/attachments"),
		ShutdownWait:   15 * time.Second,
	}
	if raw := os.Getenv("PAPERWING_SHUTDOWN_SECONDS"); raw != "" {
		seconds, err := strconv.Atoi(raw)
		if err != nil || seconds < 1 || seconds > 300 {
			return Config{}, fmt.Errorf("PAPERWING_SHUTDOWN_SECONDS must be between 1 and 300")
		}
		cfg.ShutdownWait = time.Duration(seconds) * time.Second
	}
	return cfg, nil
}

func env(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
