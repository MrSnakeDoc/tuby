package categories

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/MrSnakeDoc/tuby/internal/api/deps"
	"github.com/MrSnakeDoc/tuby/internal/events"
	"github.com/MrSnakeDoc/tuby/internal/store"
)

func TestCreateCategoryReturnsNormalizedCategoryAndConflict(t *testing.T) {
	database := openTestStore(t)
	router := categoryRouter(database, nil)
	response := serveForm(router, http.MethodPost, "/api/categories", url.Values{"name": {"  Tech   News  "}})
	if response.Code != http.StatusCreated {
		t.Fatalf("status = %d: %s", response.Code, response.Body.String())
	}
	var created categoryResponse
	if err := json.Unmarshal(response.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if created.Name != "Tech News" || created.Position != 2 || created.IsDefault {
		t.Fatalf("category = %#v", created)
	}
	response = serveForm(router, http.MethodPost, "/api/categories", url.Values{"name": {"tech news"}})
	if response.Code != http.StatusConflict {
		t.Fatalf("duplicate status = %d: %s", response.Code, response.Body.String())
	}
	var failure errorResponse
	if err := json.Unmarshal(response.Body.Bytes(), &failure); err != nil {
		t.Fatal(err)
	}
	if failure.Code != "category_exists" || failure.Field != "name" {
		t.Fatalf("failure = %#v", failure)
	}
}

func TestRenameCategoryPublishesUpdate(t *testing.T) {
	database := openTestStore(t)
	category, err := database.CreateCategory(context.Background(), "Tech")
	if err != nil {
		t.Fatal(err)
	}
	hub := events.New("v2-test")
	updates, unsubscribe := hub.Subscribe()
	defer unsubscribe()
	router := categoryRouter(database, hub)
	path := "/api/categories/" + strconv.FormatInt(category.ID, 10)
	response := serveForm(router, http.MethodPut, path, url.Values{"name": {"Science"}})
	if response.Code != http.StatusOK || response.Header().Get("HX-Trigger") != "tuby:update" {
		t.Fatalf("status = %d, headers = %v", response.Code, response.Header())
	}
	update := <-updates
	if update.Reason != "category.renamed" || len(update.CategoryIDs) != 1 || update.CategoryIDs[0] != category.ID {
		t.Fatalf("update = %#v", update)
	}
}

func TestDeleteCategoryMovesChannelsAndPublishesTheirIDs(t *testing.T) {
	database := openTestStore(t)
	ctx := context.Background()
	category, err := database.CreateCategory(ctx, "Tech")
	if err != nil {
		t.Fatal(err)
	}
	if err := database.SaveChannel(ctx, store.Channel{
		YouTubeID: "UC123", CategoryID: category.ID, SourceURL: "https://youtube.com/channel/UC123", Title: "Example",
	}); err != nil {
		t.Fatal(err)
	}
	hub := events.New("v2-test")
	updates, unsubscribe := hub.Subscribe()
	defer unsubscribe()
	router := categoryRouter(database, hub)
	path := "/api/categories/" + strconv.FormatInt(category.ID, 10)
	response := serveForm(router, http.MethodDelete, path, nil)
	if response.Code != http.StatusNoContent {
		t.Fatalf("status = %d: %s", response.Code, response.Body.String())
	}
	channel, err := database.Channel(ctx, "UC123")
	if err != nil {
		t.Fatal(err)
	}
	if channel.CategoryID != store.DefaultCategoryID {
		t.Fatalf("channel category = %d", channel.CategoryID)
	}
	update := <-updates
	if update.Reason != "category.deleted" || len(update.ChannelIDs) != 1 || update.ChannelIDs[0] != "UC123" {
		t.Fatalf("update = %#v", update)
	}
}

func TestDeleteDefaultCategoryIsRejected(t *testing.T) {
	database := openTestStore(t)
	router := categoryRouter(database, nil)
	path := "/api/categories/" + strconv.FormatInt(store.DefaultCategoryID, 10)
	response := serveForm(router, http.MethodDelete, path, nil)
	if response.Code != http.StatusConflict || !strings.Contains(response.Body.String(), "default_category") {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
}

func TestRenameDefaultCategoryIsRejected(t *testing.T) {
	database := openTestStore(t)
	router := categoryRouter(database, nil)
	path := "/api/categories/" + strconv.FormatInt(store.DefaultCategoryID, 10)
	response := serveForm(router, http.MethodPut, path, url.Values{"name": {"General"}})
	if response.Code != http.StatusConflict || !strings.Contains(response.Body.String(), "default_category_name") {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
}

func TestSaveLibraryOrderPublishesCompleteTree(t *testing.T) {
	database := openTestStore(t)
	ctx := context.Background()
	category, err := database.CreateCategory(ctx, "Tech")
	if err != nil {
		t.Fatal(err)
	}
	for _, channel := range []store.Channel{
		{YouTubeID: "UC-a", SourceURL: "https://youtube.com/channel/UC-a", Title: "A"},
		{YouTubeID: "UC-b", CategoryID: category.ID, SourceURL: "https://youtube.com/channel/UC-b", Title: "B"},
	} {
		if err := database.SaveChannel(ctx, channel); err != nil {
			t.Fatal(err)
		}
	}
	hub := events.New("v2-test")
	updates, unsubscribe := hub.Subscribe()
	defer unsubscribe()
	payload := `{"categories":[{"id":` + strconv.FormatInt(category.ID, 10) +
		`,"channel_ids":["UC-b","UC-a"]},{"id":1,"channel_ids":[]}]}`
	request := httptest.NewRequest(http.MethodPost, "/api/categories/order", strings.NewReader(payload))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	categoryRouter(database, hub).ServeHTTP(response, request)
	if response.Code != http.StatusNoContent || response.Header().Get("HX-Trigger") != "tuby:update" {
		t.Fatalf("status = %d, headers = %v, body = %s", response.Code, response.Header(), response.Body.String())
	}
	update := <-updates
	if update.Reason != "library.reordered" || len(update.CategoryIDs) != 2 || len(update.ChannelIDs) != 2 {
		t.Fatalf("update = %#v", update)
	}
	moved, err := database.Channel(ctx, "UC-a")
	if err != nil {
		t.Fatal(err)
	}
	if moved.CategoryID != category.ID || moved.Position != 2 {
		t.Fatalf("moved channel = %#v", moved)
	}
}

func categoryRouter(database *store.Store, updates *events.Hub) http.Handler {
	router := chi.NewRouter()
	Register(router, deps.Deps{Store: database, Updates: updates})
	return router
}

func serveForm(handler http.Handler, method, path string, values url.Values) *httptest.ResponseRecorder {
	var body io.Reader = http.NoBody
	if values != nil {
		body = strings.NewReader(values.Encode())
	}
	request := httptest.NewRequest(method, path, body)
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

func openTestStore(t *testing.T) *store.Store {
	t.Helper()
	database, err := store.Open(filepath.Join(t.TempDir(), "tuby.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })
	return database
}
