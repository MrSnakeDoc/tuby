package router

import (
	"github.com/go-chi/chi/v5"

	"github.com/MrSnakeDoc/tuby/internal/api/categories"
	"github.com/MrSnakeDoc/tuby/internal/api/channels"
	"github.com/MrSnakeDoc/tuby/internal/api/deps"
	apievents "github.com/MrSnakeDoc/tuby/internal/api/events"
	"github.com/MrSnakeDoc/tuby/internal/api/healthz"
	"github.com/MrSnakeDoc/tuby/internal/api/home"
	"github.com/MrSnakeDoc/tuby/internal/api/videos"
)

func RegisterRoutes(router chi.Router, dependencies deps.Deps) {
	Register(router, dependencies, channels.Register)
	Register(router, dependencies, categories.Register)
	Register(router, dependencies, apievents.Register)
	Register(router, dependencies, home.Register)
	Register(router, dependencies, healthz.Register)
	Register(router, dependencies, videos.Register)
}
