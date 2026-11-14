package healthz

import (
	"time"

	"github.com/MrSnakeDoc/tuby/internal/api/deps"
)

type response struct {
	Status        string  `json:"status"`
	Service       string  `json:"service"`
	UptimeSeconds float64 `json:"uptime_seconds"`
	Version       string  `json:"version"`
	Commit        string  `json:"commit"`
	BuildDate     string  `json:"build_date"`
	GoVersion     string  `json:"go_version"`
}

type handler struct {
	dependencies deps.Deps
	now          func() time.Time
}
