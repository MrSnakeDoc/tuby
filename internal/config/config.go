package config

import (
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
)

const (
	defaultLogLevel = "info"
	defaultPort     = "8090"
)

type Config struct {
	ListenAddr   string
	AllowedCIDRS []string
	LogLevel     string
	PrettyLog    bool
	Navire       Navire
}

func Load() (*Config, error) {
	navire, err := loadNavire()
	if err != nil {
		return nil, err
	}
	cfg := &Config{
		ListenAddr:   ":" + valueOrDefault("TUBY_PORT", defaultPort),
		AllowedCIDRS: allowedCIDRS(valueOrDefault("TUBY_ALLOWED_CIDRS", "*")),
		LogLevel:     strings.ToLower(valueOrDefault("TUBY_LOG_LEVEL", defaultLogLevel)),
		PrettyLog:    boolOrDefault("TUBY_PRETTY_LOG", true),
		Navire:       navire,
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

func (c Config) Validate() error {
	if err := validatePort(c.ListenAddr); err != nil {
		return err
	}
	if err := validateAllowedCIDRS(c.AllowedCIDRS); err != nil {
		return err
	}
	if err := validateLogLevel(c.LogLevel); err != nil {
		return err
	}
	return c.Navire.Validate()
}

func valueOrDefault(name, fallback string) string {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback
	}
	return value
}

func boolOrDefault(name string, fallback bool) bool {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return false
	}
	return parsed
}

func validateLogLevel(level string) error {
	switch level {
	case "debug", "info", "warn", "error":
		return nil
	default:
		return fmt.Errorf("TUBY_LOG_LEVEL must be debug, info, warn, or error")
	}
}

func validatePort(address string) error {
	_, port, err := net.SplitHostPort(address)
	if err != nil {
		return fmt.Errorf("TUBY_PORT must be a valid port")
	}
	value, err := strconv.Atoi(port)
	if err != nil || value < 1 || value > 65535 {
		return fmt.Errorf("TUBY_PORT must be between 1 and 65535")
	}
	return nil
}

func allowedCIDRS(value string) []string {
	parts := strings.Split(value, ",")
	allowed := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			allowed = append(allowed, trimmed)
		}
	}
	return allowed
}

func validateAllowedCIDRS(values []string) error {
	if len(values) == 0 {
		return fmt.Errorf("TUBY_ALLOWED_CIDRS must not be empty")
	}
	if len(values) == 1 && values[0] == "*" {
		return nil
	}
	for _, value := range values {
		if value == "*" {
			return fmt.Errorf("TUBY_ALLOWED_CIDRS cannot combine * with IPs or CIDRs")
		}
		if net.ParseIP(value) != nil {
			continue
		}
		if _, _, err := net.ParseCIDR(value); err != nil {
			return fmt.Errorf("TUBY_ALLOWED_CIDRS contains invalid IP or CIDR %q", value)
		}
	}
	return nil
}
