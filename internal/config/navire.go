package config

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	defaultNavireRequestTimeout = 5 * time.Second
	defaultNavireMaxAttempts    = 4
	defaultNavireInitialWait    = 2 * time.Second
	defaultNavireMaxWait        = 30 * time.Second
)

type Navire struct {
	Enabled          bool
	URL              string
	RequestTimeout   time.Duration
	MaxAttempts      int
	RetryInitialWait time.Duration
	RetryMaxWait     time.Duration
	Messages         []string
}

func loadNavire() (Navire, error) {
	enabled, err := optionalBool("TUBY_NAVIRE_ENABLED", false)
	if err != nil {
		return Navire{}, err
	}
	requestTimeout, err := optionalDuration("TUBY_NAVIRE_REQUEST_TIMEOUT", defaultNavireRequestTimeout)
	if err != nil {
		return Navire{}, err
	}
	maxAttempts, err := optionalPositiveInt("TUBY_NAVIRE_MAX_ATTEMPTS", defaultNavireMaxAttempts)
	if err != nil {
		return Navire{}, err
	}
	initialWait, err := optionalDuration("TUBY_NAVIRE_RETRY_INITIAL_WAIT", defaultNavireInitialWait)
	if err != nil {
		return Navire{}, err
	}
	maxWait, err := optionalDuration("TUBY_NAVIRE_RETRY_MAX_WAIT", defaultNavireMaxWait)
	if err != nil {
		return Navire{}, err
	}
	messages, err := notificationMessages(os.Getenv("TUBY_NOTIFICATION_MESSAGES"))
	if err != nil {
		return Navire{}, err
	}
	configuration := Navire{
		Enabled: enabled, URL: strings.TrimRight(strings.TrimSpace(os.Getenv("TUBY_NAVIRE_URL")), "/"),
		RequestTimeout: requestTimeout, MaxAttempts: maxAttempts,
		RetryInitialWait: initialWait, RetryMaxWait: maxWait, Messages: messages,
	}
	if err := configuration.Validate(); err != nil {
		return Navire{}, err
	}
	return configuration, nil
}

func (c Navire) Validate() error {
	if c.Enabled && c.URL == "" {
		return fmt.Errorf("TUBY_NAVIRE_URL is required when TUBY_NAVIRE_ENABLED is true")
	}
	if c.URL != "" {
		if err := validateNavireURL(c.URL); err != nil {
			return err
		}
	}
	if c.RequestTimeout <= 0 {
		return fmt.Errorf("TUBY_NAVIRE_REQUEST_TIMEOUT must be positive")
	}
	if c.MaxAttempts <= 0 {
		return fmt.Errorf("TUBY_NAVIRE_MAX_ATTEMPTS must be positive")
	}
	if c.RetryInitialWait <= 0 || c.RetryMaxWait <= 0 {
		return fmt.Errorf("TUBY_NAVIRE retry waits must be positive")
	}
	if c.RetryInitialWait > c.RetryMaxWait {
		return fmt.Errorf("TUBY_NAVIRE_RETRY_INITIAL_WAIT must not exceed TUBY_NAVIRE_RETRY_MAX_WAIT")
	}
	return nil
}

func validateNavireURL(value string) error {
	parsed, err := url.Parse(value)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return fmt.Errorf("TUBY_NAVIRE_URL must be a valid HTTP(S) URL")
	}
	if parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return fmt.Errorf("TUBY_NAVIRE_URL must not contain credentials, a query, or a fragment")
	}
	return nil
}

func optionalBool(name string, fallback bool) (bool, error) {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback, nil
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return false, fmt.Errorf("%s must be a boolean: %w", name, err)
	}
	return parsed, nil
}

func optionalDuration(name string, fallback time.Duration) (time.Duration, error) {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback, nil
	}
	parsed, err := time.ParseDuration(value)
	if err != nil || parsed <= 0 {
		return 0, fmt.Errorf("%s must be a positive duration", name)
	}
	return parsed, nil
}

func optionalPositiveInt(name string, fallback int) (int, error) {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback, nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		return 0, fmt.Errorf("%s must be a positive integer", name)
	}
	return parsed, nil
}

func notificationMessages(value string) ([]string, error) {
	if strings.TrimSpace(value) == "" {
		return []string{}, nil
	}
	var messages []string
	if err := json.Unmarshal([]byte(value), &messages); err != nil {
		return nil, fmt.Errorf("TUBY_NOTIFICATION_MESSAGES must be a JSON array of strings: %w", err)
	}
	if messages == nil {
		return []string{}, nil
	}
	return messages, nil
}
