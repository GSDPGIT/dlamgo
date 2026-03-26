package main

import (
	"crypto/rand"
	"encoding/base64"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Addr               string
	DatabasePath       string
	JWTSecret          string
	AdminUsername      string
	AdminPassword      string
	AllowedOrigins     []string
	TokenTTL           time.Duration
	WSTicketTTL        time.Duration
	NodeCommandTimeout time.Duration
	LoginRateLimit     int
}

func LoadConfig() Config {
	cfg := Config{
		Addr:               env("APP_ADDR", ":6365"),
		DatabasePath:       env("DATABASE_PATH", filepath.Join("data", "flux-panel.db")),
		JWTSecret:          os.Getenv("JWT_SECRET"),
		AdminUsername:      env("ADMIN_USERNAME", "admin_user"),
		AdminPassword:      os.Getenv("ADMIN_PASSWORD"),
		AllowedOrigins:     splitCSV(os.Getenv("CORS_ALLOWED_ORIGINS")),
		TokenTTL:           7 * 24 * time.Hour,
		WSTicketTTL:        2 * time.Minute,
		NodeCommandTimeout: 10 * time.Second,
		LoginRateLimit:     envInt("LOGIN_RATE_LIMIT_PER_MINUTE", 12),
	}

	if cfg.JWTSecret == "" {
		cfg.JWTSecret = randomToken(48)
	}
	if cfg.AdminPassword == "" {
		cfg.AdminPassword = randomToken(20)
	}

	_ = os.MkdirAll(filepath.Dir(cfg.DatabasePath), 0o755)
	return cfg
}

func env(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func envInt(key string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func splitCSV(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	items := strings.Split(value, ",")
	out := make([]string, 0, len(items))
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item != "" {
			out = append(out, item)
		}
	}
	return out
}

func randomToken(length int) string {
	if length <= 0 {
		return ""
	}
	raw := make([]byte, length)
	if _, err := rand.Read(raw); err != nil {
		return strconv.FormatInt(time.Now().UnixNano(), 10)
	}
	token := base64.RawURLEncoding.EncodeToString(raw)
	if len(token) > length {
		return token[:length]
	}
	return token
}
