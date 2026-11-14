package router

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/MrSnakeDoc/tuby/internal/api/deps"
)

type Registrar func(chi.Router, deps.Deps)

type Middleware = func(http.Handler) http.Handler

func Register(router chi.Router, dependencies deps.Deps, registrar Registrar, middleware ...Middleware) {
	if len(middleware) == 0 {
		registrar(router, dependencies)
		return
	}
	registrar(router.With(middleware...), dependencies)
}
