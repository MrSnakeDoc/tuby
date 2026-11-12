package home

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"git.nexacloud.dev/MrSnakeDoc/tuby/internal/api/deps"
	"git.nexacloud.dev/MrSnakeDoc/tuby/internal/store"
	"git.nexacloud.dev/MrSnakeDoc/tuby/internal/web"
)

func TestCategoryGroupsPreserveCategoryAndChannelOrder(t *testing.T) {
	categories := []store.Category{
		{ID: 2, Name: "Tech", Position: 1},
		{ID: 1, Name: "General", Position: 2, IsDefault: true},
		{ID: 3, Name: "Empty", Position: 3},
	}
	channels := []web.ChannelCard{
		{ID: "first", CategoryID: 2},
		{ID: "second", CategoryID: 2},
		{ID: "general", CategoryID: 1},
	}
	groups := categoryGroups(categories, channels)
	if len(groups) != 3 || groups[0].Name != "Tech" || groups[1].Name != "General" ||
		groups[2].Name != "Empty" || len(groups[2].Channels) != 0 {
		t.Fatalf("groups = %#v", groups)
	}
	if len(groups[0].Channels) != 2 || groups[0].Channels[0].ID != "first" ||
		groups[0].Channels[1].ID != "second" || !groups[1].IsDefault {
		t.Fatalf("groups = %#v", groups)
	}
}

func TestHomeRendersCategoryGroupsAndEmptyCategories(t *testing.T) {
	database, err := store.Open(filepath.Join(t.TempDir(), "tuby.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })
	category, err := database.CreateCategory(context.Background(), "Tech")
	if err != nil {
		t.Fatal(err)
	}
	if err := database.SaveChannel(context.Background(), store.Channel{
		YouTubeID: "UC123", CategoryID: category.ID, SourceURL: "https://youtube.com/channel/UC123", Title: "Example",
	}); err != nil {
		t.Fatal(err)
	}
	if err := database.SaveLibraryOrder(context.Background(), []store.CategoryOrder{
		{CategoryID: category.ID, ChannelIDs: []string{"UC123"}},
		{CategoryID: store.DefaultCategoryID, ChannelIDs: []string{}},
	}); err != nil {
		t.Fatal(err)
	}
	renderer, err := web.New()
	if err != nil {
		t.Fatal(err)
	}
	handler := newHandler(deps.Deps{Templates: renderer, Store: database, BuildID: "test-build"})
	request := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	response := httptest.NewRecorder()
	handler.execute(response, request)
	body := response.Body.String()
	if strings.Contains(body, "data-notification-picker") || strings.Contains(body, "notification-settings.js") {
		t.Fatal("notification controls are rendered while Navire is disabled")
	}
	for _, expected := range []string{
		`class="category-list"`, `data-category-id="1"`, `data-category-id="2"`,
		`Uncategorized`, `Tech`, `No channels in this category.`, `category-management.js?v=test-build`,
		`id="channel-category"`, `<option value="2">Tech</option>`, `data-inline-category-start`,
	} {
		if !strings.Contains(body, expected) {
			t.Fatalf("homepage does not contain %q", expected)
		}
	}
	if count := strings.Count(body, "data-category-rename"); count != 1 {
		t.Fatalf("rename actions = %d, want only the non-default category", count)
	}
	if !strings.Contains(body, `<option value="1" selected>Uncategorized</option>`) {
		t.Fatal("add-channel form does not select the default category when it is ordered last")
	}
	channelRequest := httptest.NewRequest(http.MethodGet, "/channels/UC123", http.NoBody)
	channelResponse := httptest.NewRecorder()
	handler.execute(channelResponse, channelRequest)
	channelBody := channelResponse.Body.String()
	if !strings.Contains(channelBody, `<option value="2" selected>Tech</option>`) ||
		!strings.Contains(channelBody, `id="settings-category"`) {
		t.Fatalf("channel settings do not select the stored category")
	}
}

func TestNotificationSelectorsRenderWhenNavireIsEnabled(t *testing.T) {
	database, err := store.Open(filepath.Join(t.TempDir(), "tuby.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })
	if err := database.SaveChannel(context.Background(), store.Channel{
		YouTubeID: "UC123", SourceURL: "https://youtube.com/channel/UC123", Title: "Example",
	}); err != nil {
		t.Fatal(err)
	}
	if err := database.UpdateChannelNotificationLevel(
		context.Background(), "UC123", store.NotificationCritical, time.Now(),
	); err != nil {
		t.Fatal(err)
	}
	renderer, err := web.New()
	if err != nil {
		t.Fatal(err)
	}
	handler := newHandler(deps.Deps{
		Templates: renderer, Store: database, BuildID: "test-build", NotificationsEnabled: true,
	})
	homeResponse := httptest.NewRecorder()
	handler.execute(homeResponse, httptest.NewRequest(http.MethodGet, "/", http.NoBody))
	notificationBody := homeResponse.Body.String()
	for _, expected := range []string{
		`notification-settings.js?v=test-build`, `data-notification-picker`,
		`data-notification-level="critical"`, `dog.svg?v=test-build`,
		`Wake Me Up Before You Go-Go!`, `I Like to Move It!`, `Who Let the Dogs Out?!`,
	} {
		if !strings.Contains(notificationBody, expected) {
			t.Fatalf("notification-enabled homepage does not contain %q", expected)
		}
	}
	optionOrder := []string{`data-level="off"`, `data-level="low"`, `data-level="medium"`, `data-level="critical"`}
	lastOption := -1
	for _, option := range optionOrder {
		position := strings.Index(notificationBody, option)
		if position <= lastOption {
			t.Fatalf("notification option %q is not in the expected order", option)
		}
		lastOption = position
	}
	channelResponse := httptest.NewRecorder()
	handler.execute(channelResponse, httptest.NewRequest(http.MethodGet, "/channels/UC123", http.NoBody))
	notificationChannelBody := channelResponse.Body.String()
	pickerPosition := strings.Index(notificationChannelBody, "data-notification-picker")
	countPosition := strings.Index(notificationChannelBody, "0 VIDEOS")
	if pickerPosition < 0 || countPosition < 0 || pickerPosition > countPosition {
		t.Fatal("channel notification selector is not rendered before the video count")
	}
}
