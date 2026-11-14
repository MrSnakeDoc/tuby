package channels

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/MrSnakeDoc/tuby/internal/api/bind"
	"github.com/MrSnakeDoc/tuby/internal/api/deps"
	"github.com/MrSnakeDoc/tuby/internal/api/respond"
	"github.com/MrSnakeDoc/tuby/internal/events"
	"github.com/MrSnakeDoc/tuby/internal/store"
	"github.com/MrSnakeDoc/tuby/internal/subscription"
	"github.com/MrSnakeDoc/tuby/internal/youtube"
)

type orderRequest struct {
	ChannelIDs []string `json:"channel_ids"`
}

type handler struct {
	subscriptions deps.Subscriptions
	store         *store.Store
	updates       *events.Hub
}

type refreshResponse struct {
	Refreshed int              `json:"refreshed"`
	Failed    []refreshFailure `json:"failed"`
}

type refreshFailure struct {
	ChannelID string `json:"channel_id"`
	Error     string `json:"error"`
}

type settingsResponse struct {
	ChannelID     string `json:"channel_id"`
	Title         string `json:"title"`
	CategoryID    int64  `json:"category_id"`
	IncludeLong   bool   `json:"include_long"`
	IncludeShorts bool   `json:"include_shorts"`
}

type notificationRequest struct {
	Level store.NotificationLevel `json:"level"`
}

type notificationResponse struct {
	ChannelID string                  `json:"channel_id"`
	Level     store.NotificationLevel `json:"level"`
}

func newHandler(subscriptions deps.Subscriptions, updates *events.Hub, stores ...*store.Store) *handler {
	var database *store.Store
	if len(stores) > 0 {
		database = stores[0]
	}
	return &handler{subscriptions: subscriptions, store: database, updates: updates}
}

func (h *handler) add(writer http.ResponseWriter, request *http.Request) {
	preferences, err := contentPreferences(request)
	if err != nil {
		respond.Error(writer, http.StatusBadRequest, "Select a valid category.")
		return
	}
	_, err = h.subscriptions.Add(request.Context(), request.FormValue("url"), preferences)
	if err != nil {
		failure := classifyAddError(err)
		respond.Error(writer, failure.status, failure.message)
		return
	}
	writer.Header().Set("HX-Redirect", "/")
	triggerUpdate(writer)
	writer.WriteHeader(http.StatusCreated)
}

func (h *handler) updateSettings(writer http.ResponseWriter, request *http.Request) {
	preferences, err := contentPreferences(request)
	if err != nil {
		respond.Error(writer, http.StatusBadRequest, "Select a valid category.")
		return
	}
	err = h.subscriptions.UpdateSettings(
		request.Context(),
		chi.URLParam(request, "channelID"),
		preferences,
	)
	if err != nil {
		respondUpdateSettingsError(writer, err)
		return
	}
	triggerUpdate(writer)
	writer.WriteHeader(http.StatusNoContent)
}

func (h *handler) settings(writer http.ResponseWriter, request *http.Request) {
	if h.store == nil {
		respond.Error(writer, http.StatusServiceUnavailable, "channel settings unavailable")
		return
	}
	channel, err := h.store.Channel(request.Context(), chi.URLParam(request, "channelID"))
	if err != nil {
		respondChannelError(writer, err)
		return
	}
	respond.JSON(writer, http.StatusOK, settingsResponse{
		ChannelID: channel.YouTubeID, Title: channel.Title, CategoryID: channel.CategoryID,
		IncludeLong: channel.IncludeLong, IncludeShorts: channel.IncludeShorts,
	})
}

func (h *handler) updateNotification(writer http.ResponseWriter, request *http.Request) {
	if h.subscriptions == nil {
		respond.Error(writer, http.StatusServiceUnavailable, "channel notifications unavailable")
		return
	}
	var payload notificationRequest
	if err := bind.JSON(writer, request, &payload); err != nil || !payload.Level.Valid() {
		respond.Error(writer, http.StatusBadRequest, "Select a valid notification level.")
		return
	}
	channelID := chi.URLParam(request, "channelID")
	if err := h.subscriptions.UpdateNotificationLevel(request.Context(), channelID, payload.Level); err != nil {
		respondChannelNotificationError(writer, err)
		return
	}
	h.publish(events.Update{
		Reason: "channel.notification.updated", Scopes: events.AllScopes(), ChannelIDs: []string{channelID},
	})
	triggerUpdate(writer)
	respond.JSON(writer, http.StatusOK, notificationResponse{ChannelID: channelID, Level: payload.Level})
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
	h.publish(events.Update{Reason: "channel.reordered", Scopes: events.AllScopes(), ChannelIDs: payload.ChannelIDs})
	triggerUpdate(writer)
	writer.WriteHeader(http.StatusNoContent)
}

func (h *handler) delete(writer http.ResponseWriter, request *http.Request) {
	if err := h.subscriptions.Delete(request.Context(), chi.URLParam(request, "channelID")); err != nil {
		respondChannelError(writer, err)
		return
	}
	triggerUpdate(writer)
	writer.WriteHeader(http.StatusNoContent)
}

func (h *handler) fetch(writer http.ResponseWriter, request *http.Request) {
	if err := h.subscriptions.Refresh(request.Context(), chi.URLParam(request, "channelID")); err != nil {
		respondChannelError(writer, err)
		return
	}
	triggerUpdate(writer)
	respond.JSON(writer, http.StatusOK, refreshResponse{Refreshed: 1, Failed: []refreshFailure{}})
}

func (h *handler) markChannelSeen(writer http.ResponseWriter, request *http.Request) {
	if err := h.store.MarkChannelWatched(request.Context(), chi.URLParam(request, "channelID")); err != nil {
		respond.Error(writer, http.StatusInternalServerError, "Tuby could not mark this channel as watched. Try again.")
		return
	}
	h.publish(events.Update{Reason: "channel.seen", Scopes: events.AllScopes(), ChannelIDs: []string{chi.URLParam(request, "channelID")}})
	triggerUpdate(writer)
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
	triggerUpdate(writer)
	respond.JSON(writer, status, response)
}

func (h *handler) saveLater(writer http.ResponseWriter, request *http.Request) {
	h.setLater(writer, request, true)
}

func (h *handler) removeLater(writer http.ResponseWriter, request *http.Request) {
	h.setLater(writer, request, false)
}

func (h *handler) markSeen(writer http.ResponseWriter, request *http.Request) {
	videoID := chi.URLParam(request, "videoID")
	if err := h.store.MarkVideoWatched(request.Context(), videoID); err != nil {
		respondVideoMutationError(writer, err, "mark this video as watched")
		return
	}
	h.publish(events.Update{Reason: "video.seen", Scopes: events.AllScopes(), VideoIDs: []string{videoID}})
	triggerUpdate(writer)
	writer.WriteHeader(http.StatusNoContent)
}

func (h *handler) setLater(writer http.ResponseWriter, request *http.Request, enabled bool) {
	videoID := chi.URLParam(request, "videoID")
	if err := h.store.SetVideoWatchLater(request.Context(), videoID, enabled); err != nil {
		respondVideoMutationError(writer, err, "update Watch later")
		return
	}
	h.publish(events.Update{Reason: "video.watch_later", Scopes: events.AllScopes(), VideoIDs: []string{videoID}})
	triggerUpdate(writer)
	writer.WriteHeader(http.StatusNoContent)
}

func (h *handler) publish(update events.Update) {
	if h.updates != nil {
		h.updates.Publish(update)
	}
}

func triggerUpdate(writer http.ResponseWriter) {
	writer.Header().Set("HX-Trigger", "tuby:update")
}

func contentPreferences(request *http.Request) (subscription.ContentPreferences, error) {
	categoryID, err := optionalCategoryID(request.FormValue("category_id"))
	if err != nil {
		return subscription.ContentPreferences{}, err
	}
	return subscription.ContentPreferences{
		IncludeLong:   request.FormValue("include_long") != "",
		IncludeShorts: request.FormValue("include_shorts") != "",
		CategoryID:    categoryID,
	}, nil
}

func optionalCategoryID(value string) (int64, error) {
	if value == "" {
		return 0, nil
	}
	categoryID, err := strconv.ParseInt(value, 10, 64)
	if err != nil || categoryID <= 0 {
		return 0, errors.New("invalid category ID")
	}
	return categoryID, nil
}

func respondChannelError(writer http.ResponseWriter, err error) {
	if errors.Is(err, store.ErrChannelNotFound) {
		respond.Error(writer, http.StatusNotFound, "This channel no longer exists.")
		return
	}
	var fetchError *youtube.FetchError
	if errors.As(err, &fetchError) {
		failure := classifyFeedFetchError(fetchError)
		respond.Error(writer, failure.status, failure.message)
		return
	}
	if errors.Is(err, subscription.ErrChannelFeedUnavailable) {
		respond.Error(writer, http.StatusBadGateway, "YouTube returned an unreadable RSS feed. Try again later.")
		return
	}
	respond.Error(writer, http.StatusInternalServerError, "Tuby could not complete this channel action. Try again.")
}

func respondUpdateSettingsError(writer http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, subscription.ErrVideoTypesRequired):
		respond.Error(writer, http.StatusBadRequest, "Select Long, Shorts, or both.")
	case errors.Is(err, store.ErrCategoryNotFound):
		respond.Error(writer, http.StatusBadRequest, "That category no longer exists. Refresh the page and try again.")
	case errors.Is(err, store.ErrChannelNotFound):
		respond.Error(writer, http.StatusNotFound, "This channel no longer exists.")
	default:
		respond.Error(writer, http.StatusInternalServerError, "Tuby could not save these settings. Try again.")
	}
}

func respondVideoMutationError(writer http.ResponseWriter, err error, action string) {
	if errors.Is(err, store.ErrVideoNotFound) {
		respond.Error(writer, http.StatusNotFound, "This video no longer exists in Tuby.")
		return
	}
	respond.Error(writer, http.StatusInternalServerError, "Tuby could not "+action+". Try again.")
}

func respondChannelNotificationError(writer http.ResponseWriter, err error) {
	if errors.Is(err, store.ErrChannelNotFound) {
		respond.Error(writer, http.StatusNotFound, "This channel no longer exists.")
		return
	}
	respond.Error(writer, http.StatusInternalServerError, "Tuby could not update this notification level. Try again.")
}

func failures(values []subscription.RefreshFailure) []refreshFailure {
	result := make([]refreshFailure, 0, len(values))
	for _, value := range values {
		result = append(result, refreshFailure{ChannelID: value.ChannelID, Error: value.Err.Error()})
	}
	return result
}
