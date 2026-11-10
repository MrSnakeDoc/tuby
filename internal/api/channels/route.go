package channels

import (
	"github.com/go-chi/chi/v5"

	"git.nexacloud.dev/MrSnakeDoc/tuby/internal/api/deps"
)

func Register(router chi.Router, dependencies deps.Deps) {
	handler := newHandler(dependencies.Subscriptions, dependencies.Updates, dependencies.Store)
	router.Post("/api/channels", handler.add)
	router.Post("/api/channels/order", handler.saveOrder)
	router.Delete("/api/channels/{channelID}", handler.delete)
	router.Post("/api/channels/fetch", handler.fetchAll)
	router.Post("/api/channels/{channelID}/fetch", handler.fetch)
	router.Post("/api/channels/{channelID}/seen", handler.markChannelSeen)
	router.Put("/api/channels/{channelID}/settings", handler.updateSettings)
	router.Post("/api/videos/{videoID}/watch-later", handler.saveLater)
	router.Delete("/api/videos/{videoID}/watch-later", handler.removeLater)
	router.Post("/api/videos/{videoID}/seen", handler.markSeen)
}
