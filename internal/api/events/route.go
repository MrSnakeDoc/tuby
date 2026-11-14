package events

import (
	"github.com/go-chi/chi/v5"

	"github.com/MrSnakeDoc/tuby/internal/api/deps"
)

func Register(router chi.Router, dependencies deps.Deps) {
	router.Get("/api/events", (&handler{updates: dependencies.Updates}).stream)
}
