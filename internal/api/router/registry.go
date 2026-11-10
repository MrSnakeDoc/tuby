package router

import (
	"github.com/go-chi/chi/v5"

	"git.nexacloud.dev/MrSnakeDoc/tuby/internal/api/categories"
	"git.nexacloud.dev/MrSnakeDoc/tuby/internal/api/channels"
	"git.nexacloud.dev/MrSnakeDoc/tuby/internal/api/deps"
	apievents "git.nexacloud.dev/MrSnakeDoc/tuby/internal/api/events"
	"git.nexacloud.dev/MrSnakeDoc/tuby/internal/api/healthz"
	"git.nexacloud.dev/MrSnakeDoc/tuby/internal/api/home"
)

func RegisterRoutes(router chi.Router, dependencies deps.Deps) {
	Register(router, dependencies, channels.Register)
	Register(router, dependencies, categories.Register)
	Register(router, dependencies, apievents.Register)
	Register(router, dependencies, home.Register)
	Register(router, dependencies, healthz.Register)
}
