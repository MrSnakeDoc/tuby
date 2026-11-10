package home

import (
	"net/http"
	"strings"

	"git.nexacloud.dev/MrSnakeDoc/tuby/internal/api/deps"
	"git.nexacloud.dev/MrSnakeDoc/tuby/internal/store"
	"git.nexacloud.dev/MrSnakeDoc/tuby/internal/web"
)

type handler struct {
	templates *web.Renderer
	version   string
	buildID   string
	store     *store.Store
}

func newHandler(dependencies deps.Deps) *handler {
	return &handler{
		templates: dependencies.Templates, version: dependencies.Version,
		buildID: dependencies.BuildID, store: dependencies.Store,
	}
}

func (h *handler) execute(writer http.ResponseWriter, request *http.Request) {
	data, err := h.pageData(request)
	if err != nil {
		http.Error(writer, "load homepage", http.StatusInternalServerError)
		return
	}
	data.Version = h.version
	data.BuildID = h.buildID
	if request.Header.Get("HX-Request") == "true" {
		err = h.templates.RenderContent(writer, data)
	} else {
		err = h.templates.RenderHome(writer, data)
	}
	if err != nil {
		http.Error(writer, "render homepage", http.StatusInternalServerError)
		return
	}
}

func (h *handler) pageData(request *http.Request) (web.HomeData, error) {
	data := web.HomeData{Page: "home"}
	if h.store == nil {
		return data, nil
	}
	summaries, err := h.store.ChannelSummaries(request.Context())
	if err != nil {
		return data, err
	}
	data.Channels = channelCards(summaries)
	categories, err := h.store.Categories(request.Context())
	if err != nil {
		return data, err
	}
	data.Categories = categoryGroups(categories, data.Channels)
	for _, channel := range summaries {
		data.MissedCount += channel.UnseenCount
	}
	data.WatchLaterCount, err = h.store.WatchLaterCount(request.Context())
	if err != nil {
		return data, err
	}
	path := strings.Trim(request.URL.Path, "/")
	switch {
	case path == "":
		return data, nil
	case path == "missed":
		data.Page = "missed"
		videos, err := h.store.UnseenVideos(request.Context())
		data.Videos = videoCards(videos, summaries)
		return data, err
	case path == "watch-later":
		data.Page = "later"
		videos, err := h.store.WatchLaterVideos(request.Context())
		data.Videos = videoCards(videos, summaries)
		return data, err
	case strings.HasPrefix(path, "channels/"):
		id := strings.TrimPrefix(path, "channels/")
		channel, found := findChannel(data.Channels, id)
		if !found {
			return data, store.ErrChannelNotFound
		}
		data.Page = "channel"
		data.CurrentChannel = &channel
		videos, err := h.store.ChannelVideos(request.Context(), id)
		data.Videos = videoCards(videos, summaries)
		return data, err
	default:
		return data, store.ErrChannelNotFound
	}
}

func channelCards(summaries []store.ChannelSummary) []web.ChannelCard {
	result := make([]web.ChannelCard, 0, len(summaries))
	for _, value := range summaries {
		result = append(result, web.ChannelCard{
			ID: value.YouTubeID, Title: value.Title, ImageURL: value.ImageURL,
			CategoryID:    value.CategoryID,
			ThumbnailURLs: value.ThumbnailURLs, VideoCount: value.VideoCount, UnseenCount: value.UnseenCount,
			IncludeLong: value.IncludeLong, IncludeShorts: value.IncludeShorts,
		})
	}
	return result
}

func categoryGroups(categories []store.Category, channels []web.ChannelCard) []web.CategoryGroup {
	groups := make([]web.CategoryGroup, 0, len(categories))
	indexes := make(map[int64]int, len(categories))
	for _, category := range categories {
		indexes[category.ID] = len(groups)
		groups = append(groups, web.CategoryGroup{
			ID: category.ID, Name: category.Name, IsDefault: category.IsDefault, Channels: make([]web.ChannelCard, 0),
		})
	}
	for _, channel := range channels {
		index, exists := indexes[channel.CategoryID]
		if exists {
			groups[index].Channels = append(groups[index].Channels, channel)
		}
	}
	return groups
}

func videoCards(videos []store.Video, channels []store.ChannelSummary) []web.VideoCard {
	result := make([]web.VideoCard, 0, len(videos))
	for _, value := range videos {
		channel, _ := findChannel(channelCards(channels), value.ChannelYouTubeID)
		result = append(result, web.VideoCard{
			ID: value.YouTubeID, ChannelID: value.ChannelYouTubeID, ChannelTitle: channel.Title,
			Title: value.Title, URL: value.URL, ThumbnailURL: value.ThumbnailURL,
			Published: value.PublishedAt, Unseen: value.Unseen, WatchLater: value.WatchLater,
		})
	}
	return result
}

func findChannel(channels []web.ChannelCard, id string) (web.ChannelCard, bool) {
	for _, channel := range channels {
		if channel.ID == id {
			return channel, true
		}
	}
	return web.ChannelCard{}, false
}
