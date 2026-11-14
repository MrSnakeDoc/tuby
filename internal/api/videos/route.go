package videos

import (
	"github.com/go-chi/chi/v5"

	"github.com/MrSnakeDoc/tuby/internal/api/deps"
)

func Register(router chi.Router, dependencies deps.Deps) {
	handler := &handler{durations: dependencies.VideoDurations}
	router.Post("/api/videos/{videoID}/duration", handler.fetchDuration)
}
