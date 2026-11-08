package healthz

import (
	"encoding/json"
	"net/http"
	"time"

	"git.nexacloud.dev/MrSnakeDoc/tuby/internal/api/deps"
)

func newHandler(dependencies deps.Deps) *handler {
	return &handler{dependencies: dependencies, now: time.Now}
}

func (h *handler) execute(writer http.ResponseWriter, _ *http.Request) {
	writer.Header().Set("Cache-Control", "no-store")
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(writer).Encode(response{
		Status:        "ok",
		Service:       "tuby",
		UptimeSeconds: h.now().Sub(h.dependencies.StartTime).Seconds(),
		Version:       h.dependencies.Version,
		Commit:        h.dependencies.Commit,
		BuildDate:     h.dependencies.BuildDate,
		GoVersion:     h.dependencies.GoVersion,
	})
}
