package channels

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"git.nexacloud.dev/MrSnakeDoc/tuby/internal/api/deps"
	"git.nexacloud.dev/MrSnakeDoc/tuby/internal/api/respond"
	"git.nexacloud.dev/MrSnakeDoc/tuby/internal/store"
	"git.nexacloud.dev/MrSnakeDoc/tuby/internal/subscription"
)

type orderRequest struct {
	ChannelIDs []string `json:"channel_ids"`
}

type handler struct {
	subscriptions deps.Subscriptions
	store         *store.Store
}

type refreshResponse struct {
	Refreshed int              `json:"refreshed"`
	Failed    []refreshFailure `json:"failed"`
}

type refreshFailure struct {
	ChannelID string `json:"channel_id"`
	Error     string `json:"error"`
}

func newHandler(subscriptions deps.Subscriptions, stores ...*store.Store) *handler {
	var database *store.Store
	if len(stores) > 0 {
		database = stores[0]
	}
	return &handler{subscriptions: subscriptions, store: database}
}

func (h *handler) add(writer http.ResponseWriter, request *http.Request) {
	_, err := h.subscriptions.Add(request.Context(), request.FormValue("url"))
	if err != nil {
		respond.Error(writer, http.StatusBadRequest, "add channel failed")
		return
	}
	writer.Header().Set("HX-Redirect", "/")
	writer.WriteHeader(http.StatusCreated)
}

func (h *handler) saveOrder(writer http.ResponseWriter, request *http.Request) {
	var payload orderRequest
	if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
		respond.Error(writer, http.StatusBadRequest, "invalid channel order")
		return
	}
	if err := h.store.SaveChannelOrder(request.Context(), payload.ChannelIDs); err != nil {
		respond.Error(writer, http.StatusBadRequest, "save channel order failed")
		return
	}
	writer.WriteHeader(http.StatusNoContent)
}

func (h *handler) delete(writer http.ResponseWriter, request *http.Request) {
	if err := h.subscriptions.Delete(request.Context(), chi.URLParam(request, "channelID")); err != nil {
		respondChannelError(writer, err)
		return
	}
	writer.WriteHeader(http.StatusNoContent)
}

func (h *handler) fetch(writer http.ResponseWriter, request *http.Request) {
	if err := h.subscriptions.Refresh(request.Context(), chi.URLParam(request, "channelID")); err != nil {
		respondChannelError(writer, err)
		return
	}
	respond.JSON(writer, http.StatusOK, refreshResponse{Refreshed: 1, Failed: []refreshFailure{}})
}

func (h *handler) markChannelSeen(writer http.ResponseWriter, request *http.Request) {
	if err := h.store.MarkChannelWatched(request.Context(), chi.URLParam(request, "channelID")); err != nil {
		respond.Error(writer, http.StatusInternalServerError, "mark channel as seen failed")
		return
	}
	writer.WriteHeader(http.StatusNoContent)
}

func (h *handler) fetchAll(writer http.ResponseWriter, request *http.Request) {
	report, err := h.subscriptions.RefreshAll(request.Context())
	if err != nil {
		respondChannelError(writer, err)
		return
	}
	response := refreshResponse{Refreshed: report.Refreshed, Failed: failures(report.Failures)}
	status := http.StatusOK
	if len(response.Failed) > 0 {
		status = http.StatusMultiStatus
	}
	respond.JSON(writer, status, response)
}

func (h *handler) saveLater(writer http.ResponseWriter, request *http.Request) {
	h.setLater(writer, request, true)
}

func (h *handler) removeLater(writer http.ResponseWriter, request *http.Request) {
	h.setLater(writer, request, false)
}

func (h *handler) markSeen(writer http.ResponseWriter, request *http.Request) {
	if err := h.store.MarkVideoWatched(request.Context(), chi.URLParam(request, "videoID")); err != nil {
		respond.Error(writer, http.StatusNotFound, "video not found")
		return
	}
	writer.WriteHeader(http.StatusNoContent)
}

func (h *handler) setLater(writer http.ResponseWriter, request *http.Request, enabled bool) {
	if err := h.store.SetVideoWatchLater(request.Context(), chi.URLParam(request, "videoID"), enabled); err != nil {
		respond.Error(writer, http.StatusNotFound, "video not found")
		return
	}
	writer.WriteHeader(http.StatusNoContent)
}

func respondChannelError(writer http.ResponseWriter, err error) {
	if errors.Is(err, store.ErrChannelNotFound) {
		respond.Error(writer, http.StatusNotFound, "channel not found")
		return
	}
	respond.Error(writer, http.StatusBadGateway, "channel operation failed")
}

func failures(values []subscription.RefreshFailure) []refreshFailure {
	result := make([]refreshFailure, 0, len(values))
	for _, value := range values {
		result = append(result, refreshFailure{ChannelID: value.ChannelID, Error: value.Err.Error()})
	}
	return result
}
