package web

import (
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"time"
)

//go:embed templates/*.html static/css/*.css static/img/*.svg static/js/*.js
var assets embed.FS

type Renderer struct {
	templates *template.Template
	static    http.Handler
}

type HomeData struct {
	Version         string
	Page            string
	Channels        []ChannelCard
	Videos          []VideoCard
	CurrentChannel  *ChannelCard
	MissedCount     int
	WatchLaterCount int
	Now             time.Time
}

type ChannelCard struct {
	ID, Title, ImageURL     string
	ThumbnailURLs           []string
	VideoCount, UnseenCount int
}
type VideoCard struct {
	ID, ChannelID, ChannelTitle, Title, URL, ThumbnailURL string
	Published                                             time.Time
	Unseen, WatchLater                                    bool
}

func New() (*Renderer, error) {
	templates, err := template.New("index.html").Funcs(template.FuncMap{"timeAgo": timeAgo, "thumbnailJSON": thumbnailJSON}).ParseFS(assets, "templates/*.html")
	if err != nil {
		return nil, fmt.Errorf("parse templates: %w", err)
	}
	staticFiles, err := fs.Sub(assets, "static")
	if err != nil {
		return nil, fmt.Errorf("load static assets: %w", err)
	}
	return &Renderer{
		templates: templates,
		static:    http.FileServer(http.FS(staticFiles)),
	}, nil
}

func thumbnailJSON(values []string) string {
	encoded, _ := json.Marshal(values)
	return string(encoded)
}

func (r *Renderer) RenderHome(writer http.ResponseWriter, data HomeData) error {
	writer.Header().Set("Content-Type", "text/html; charset=utf-8")
	return r.templates.ExecuteTemplate(writer, "index.html", data)
}

func (r *Renderer) RenderContent(writer http.ResponseWriter, data HomeData) error {
	writer.Header().Set("Content-Type", "text/html; charset=utf-8")
	return r.templates.ExecuteTemplate(writer, "content", data)
}

func (r *Renderer) Static() http.Handler {
	return r.static
}

func timeAgo(value time.Time) string {
	days := int(time.Since(value).Hours() / 24)
	if days <= 0 {
		return "Today"
	}
	if days == 1 {
		return "Yesterday"
	}
	return fmt.Sprintf("%d days ago", days)
}
