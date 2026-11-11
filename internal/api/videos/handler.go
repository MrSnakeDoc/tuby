package videos

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"git.nexacloud.dev/MrSnakeDoc/tuby/internal/api/deps"
	"git.nexacloud.dev/MrSnakeDoc/tuby/internal/api/respond"
	"git.nexacloud.dev/MrSnakeDoc/tuby/internal/store"
)

type handler struct {
	durations deps.VideoDurations
}

func (h *handler) fetchDuration(writer http.ResponseWriter, request *http.Request) {
	if h.durations == nil {
		respond.Error(writer, http.StatusServiceUnavailable, "video duration service unavailable")
		return
	}
	err := h.durations.Fetch(request.Context(), chi.URLParam(request, "videoID"))
	if errors.Is(err, store.ErrVideoNotFound) {
		respond.Error(writer, http.StatusNotFound, "video not found")
		return
	}
	if err != nil {
		respond.Error(writer, http.StatusBadGateway, "fetch video duration failed")
		return
	}
	writer.WriteHeader(http.StatusNoContent)
}
