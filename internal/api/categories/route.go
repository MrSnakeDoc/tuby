package categories

import (
	"github.com/go-chi/chi/v5"

	"github.com/MrSnakeDoc/tuby/internal/api/deps"
)

func Register(router chi.Router, dependencies deps.Deps) {
	handler := &handler{store: dependencies.Store, updates: dependencies.Updates}
	router.Post("/api/categories", handler.create)
	router.Post("/api/categories/order", handler.saveOrder)
	router.Put("/api/categories/{categoryID}", handler.rename)
	router.Delete("/api/categories/{categoryID}", handler.delete)
}
