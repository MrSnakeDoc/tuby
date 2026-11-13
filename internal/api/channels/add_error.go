package channels

import (
	"errors"
	"net/http"

	"git.nexacloud.dev/MrSnakeDoc/tuby/internal/store"
	"git.nexacloud.dev/MrSnakeDoc/tuby/internal/subscription"
	"git.nexacloud.dev/MrSnakeDoc/tuby/internal/youtube"
)

type addErrorResponse struct {
	status  int
	message string
}

func classifyAddError(err error) addErrorResponse {
	switch {
	case errors.Is(err, subscription.ErrInvalidChannelReference):
		return addErrorResponse{http.StatusBadRequest, "Enter a valid YouTube channel URL or UC… channel ID."}
	case errors.Is(err, subscription.ErrVideoTypesRequired):
		return addErrorResponse{http.StatusBadRequest, "Select Long, Shorts, or both."}
	case errors.Is(err, store.ErrCategoryNotFound):
		return addErrorResponse{http.StatusBadRequest, "That category no longer exists. Refresh the page and try again."}
	case errors.Is(err, subscription.ErrChannelResolution):
		return addErrorResponse{http.StatusBadGateway, "YouTube could not resolve this channel. Check the URL or try again later."}
	}
	var fetchError *youtube.FetchError
	if errors.As(err, &fetchError) {
		return classifyFeedFetchError(fetchError)
	}
	if errors.Is(err, subscription.ErrChannelFeedUnavailable) {
		return addErrorResponse{http.StatusBadGateway, "YouTube returned an unreadable RSS feed. Try again later."}
	}
	return addErrorResponse{http.StatusInternalServerError, "Tuby could not add this channel. Try again later."}
}

func classifyFeedFetchError(err *youtube.FetchError) addErrorResponse {
	switch {
	case err.StatusCode == http.StatusNotFound:
		return addErrorResponse{
			status:  http.StatusBadGateway,
			message: "YouTube returned 404 for this channel's RSS feed. This is often temporary; try again later.",
		}
	case err.StatusCode == http.StatusTooManyRequests:
		return addErrorResponse{http.StatusServiceUnavailable, "YouTube is rate-limiting requests. Try again later."}
	case err.Temporary:
		return addErrorResponse{http.StatusServiceUnavailable, "YouTube is temporarily unavailable. Try again later."}
	default:
		return addErrorResponse{http.StatusBadGateway, "YouTube refused this channel's RSS feed. Check the channel or try again later."}
	}
}
