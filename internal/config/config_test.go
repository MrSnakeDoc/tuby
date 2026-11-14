package config

import (
	"reflect"
	"testing"
	"time"
)

func TestLoadUsesDefaults(t *testing.T) {
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ListenAddr != ":8090" || cfg.LogLevel != "info" || !cfg.PrettyLog {
		t.Fatalf("Load() = %+v, want defaults", cfg)
	}
	if len(cfg.AllowedCIDRS) != 1 || cfg.AllowedCIDRS[0] != "*" {
		t.Fatalf("AllowedCIDRS = %#v, want wildcard", cfg.AllowedCIDRS)
	}
	if cfg.Navire.Enabled || cfg.Navire.URL != "" || cfg.Navire.RequestTimeout != 5*time.Second ||
		cfg.Navire.MaxAttempts != 4 || cfg.Navire.RetryInitialWait != 2*time.Second ||
		cfg.Navire.RetryMaxWait != 30*time.Second || len(cfg.Navire.Messages) != 0 {
		t.Fatalf("Navire = %+v, want disabled defaults", cfg.Navire)
	}
}

func TestLoadUsesConfiguredNavireValues(t *testing.T) {
	t.Setenv("TUBY_NAVIRE_ENABLED", "true")
	t.Setenv("TUBY_NAVIRE_URL", "https://navire.example.com/")
	t.Setenv("TUBY_NAVIRE_REQUEST_TIMEOUT", "7s")
	t.Setenv("TUBY_NAVIRE_MAX_ATTEMPTS", "6")
	t.Setenv("TUBY_NAVIRE_RETRY_INITIAL_WAIT", "3s")
	t.Setenv("TUBY_NAVIRE_RETRY_MAX_WAIT", "1m")
	t.Setenv("TUBY_NOTIFICATION_MESSAGES", `["First","Unicode · message"]`)
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.Navire.Enabled || cfg.Navire.URL != "https://navire.example.com" ||
		cfg.Navire.RequestTimeout != 7*time.Second || cfg.Navire.MaxAttempts != 6 ||
		cfg.Navire.RetryInitialWait != 3*time.Second || cfg.Navire.RetryMaxWait != time.Minute {
		t.Fatalf("Navire = %+v", cfg.Navire)
	}
	if !reflect.DeepEqual(cfg.Navire.Messages, []string{"First", "Unicode · message"}) {
		t.Fatalf("Messages = %#v", cfg.Navire.Messages)
	}
}

func TestLoadRequiresNavireURLOnlyWhenEnabled(t *testing.T) {
	if _, err := Load(); err != nil {
		t.Fatalf("disabled Navire: %v", err)
	}
	t.Setenv("TUBY_NAVIRE_ENABLED", "true")
	if _, err := Load(); err == nil {
		t.Fatal("enabled Navire without URL was accepted")
	}
}

func TestLoadRejectsInvalidNavireValues(t *testing.T) {
	tests := []struct {
		name, environment, value string
	}{
		{name: "enabled", environment: "TUBY_NAVIRE_ENABLED", value: "sometimes"},
		{name: "URL", environment: "TUBY_NAVIRE_URL", value: "ftp://navire.example.com"},
		{name: "timeout", environment: "TUBY_NAVIRE_REQUEST_TIMEOUT", value: "soon"},
		{name: "attempts", environment: "TUBY_NAVIRE_MAX_ATTEMPTS", value: "0"},
		{name: "initial wait", environment: "TUBY_NAVIRE_RETRY_INITIAL_WAIT", value: "0s"},
		{name: "max wait", environment: "TUBY_NAVIRE_RETRY_MAX_WAIT", value: "invalid"},
		{name: "messages", environment: "TUBY_NOTIFICATION_MESSAGES", value: `["missing quote]`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv(test.environment, test.value)
			if _, err := Load(); err == nil {
				t.Fatalf("%s=%q was accepted", test.environment, test.value)
			}
		})
	}
}

func TestLoadRejectsRetryInitialWaitAboveMaximum(t *testing.T) {
	t.Setenv("TUBY_NAVIRE_RETRY_INITIAL_WAIT", "1m")
	t.Setenv("TUBY_NAVIRE_RETRY_MAX_WAIT", "30s")
	if _, err := Load(); err == nil {
		t.Fatal("retry initial wait above maximum was accepted")
	}
}

func TestLoadUsesConfiguredValues(t *testing.T) {
	t.Setenv("TUBY_PORT", "9090")
	t.Setenv("TUBY_LOG_LEVEL", "debug")
	t.Setenv("TUBY_PRETTY_LOG", "false")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ListenAddr != ":9090" || cfg.LogLevel != "debug" || cfg.PrettyLog {
		t.Fatalf("Load() = %+v, want configured values", cfg)
	}
}

func TestLoadParsesAllowedCIDRS(t *testing.T) {
	t.Setenv("TUBY_ALLOWED_CIDRS", "127.0.0.1, 10.0.0.0/8, ::1")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"127.0.0.1", "10.0.0.0/8", "::1"}
	if len(cfg.AllowedCIDRS) != len(want) {
		t.Fatalf("AllowedCIDRS = %#v, want %#v", cfg.AllowedCIDRS, want)
	}
	for index, value := range want {
		if cfg.AllowedCIDRS[index] != value {
			t.Fatalf("AllowedCIDRS = %#v, want %#v", cfg.AllowedCIDRS, want)
		}
	}
}

func TestLoadRejectsInvalidValues(t *testing.T) {
	tests := []struct {
		name, port string
	}{
		{name: "non numeric port", port: "nope"},
		{name: "out of range port", port: "65536"},
		{name: "invalid allowed CIDR"},
		{name: "invalid log level"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("TUBY_PORT", test.port)
			if test.name == "invalid allowed CIDR" {
				t.Setenv("TUBY_ALLOWED_CIDRS", "invalid")
			}
			if test.name == "invalid log level" {
				t.Setenv("TUBY_LOG_LEVEL", "verbose")
			}
			if _, err := Load(); err == nil {
				t.Fatal("Load() error = nil, want an error")
			}
		})
	}
}
