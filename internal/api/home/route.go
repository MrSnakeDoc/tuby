package home

import (
	"github.com/go-chi/chi/v5"

	"git.nexacloud.dev/MrSnakeDoc/tuby/internal/api/deps"
)

func Register(router chi.Router, dependencies deps.Deps) {
	handler := newHandler(dependencies)
	router.Get("/", handler.execute)
	router.Get("/channels/{channelID}", handler.execute)
	router.Get("/missed", handler.execute)
	router.Get("/watch-later", handler.execute)
}
