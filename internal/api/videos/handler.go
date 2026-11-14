package videos

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/MrSnakeDoc/tuby/internal/api/deps"
	"github.com/MrSnakeDoc/tuby/internal/api/respond"
	"github.com/MrSnakeDoc/tuby/internal/store"
)

type handler struct {
	durations deps.VideoDurations
}

func (h *handler) fetchDuration(writer http.ResponseWriter, request *http.Request) {
	if h.durations == nil {
		respond.Error(writer, http.StatusServiceUnavailable, "Video duration is temporarily unavailable. Try again later.")
		return
	}
	err := h.durations.Fetch(request.Context(), chi.URLParam(request, "videoID"))
	if errors.Is(err, store.ErrVideoNotFound) {
		respond.Error(writer, http.StatusNotFound, "This video no longer exists in Tuby.")
		return
	}
	if err != nil {
		respond.Error(writer, http.StatusBadGateway, "The video duration could not be fetched. Try again later.")
		return
	}
	writer.WriteHeader(http.StatusNoContent)
}
