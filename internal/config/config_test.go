package config

import "testing"

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
