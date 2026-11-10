package categories

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"git.nexacloud.dev/MrSnakeDoc/tuby/internal/api/respond"
	"git.nexacloud.dev/MrSnakeDoc/tuby/internal/events"
	"git.nexacloud.dev/MrSnakeDoc/tuby/internal/store"
)

type handler struct {
	store   *store.Store
	updates *events.Hub
}

type categoryResponse struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	Position  int    `json:"position"`
	IsDefault bool   `json:"is_default"`
}

type errorResponse struct {
	Error string `json:"error"`
	Code  string `json:"code"`
	Field string `json:"field,omitempty"`
}

type orderRequest struct {
	Categories []categoryOrderRequest `json:"categories"`
}

type categoryOrderRequest struct {
	ID         int64    `json:"id"`
	ChannelIDs []string `json:"channel_ids"`
}

func (h *handler) create(writer http.ResponseWriter, request *http.Request) {
	category, err := h.store.CreateCategory(request.Context(), request.FormValue("name"))
	if err != nil {
		respondCategoryError(writer, err)
		return
	}
	h.publish("category.created", category.ID, nil)
	triggerUpdate(writer)
	respond.JSON(writer, http.StatusCreated, responseFromCategory(category))
}

func (h *handler) rename(writer http.ResponseWriter, request *http.Request) {
	categoryID, ok := categoryID(writer, request)
	if !ok {
		return
	}
	category, err := h.store.RenameCategory(request.Context(), categoryID, request.FormValue("name"))
	if err != nil {
		respondCategoryError(writer, err)
		return
	}
	h.publish("category.renamed", category.ID, nil)
	triggerUpdate(writer)
	respond.JSON(writer, http.StatusOK, responseFromCategory(category))
}

func (h *handler) delete(writer http.ResponseWriter, request *http.Request) {
	categoryID, ok := categoryID(writer, request)
	if !ok {
		return
	}
	channelIDs, err := h.store.DeleteCategory(request.Context(), categoryID)
	if err != nil {
		respondCategoryError(writer, err)
		return
	}
	h.publish("category.deleted", categoryID, channelIDs)
	triggerUpdate(writer)
	writer.WriteHeader(http.StatusNoContent)
}

func (h *handler) saveOrder(writer http.ResponseWriter, request *http.Request) {
	var payload orderRequest
	if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
		respond.Error(writer, http.StatusBadRequest, "invalid library order")
		return
	}
	order, categoryIDs, channelIDs := storeOrder(payload.Categories)
	if err := h.store.SaveLibraryOrder(request.Context(), order); err != nil {
		respond.Error(writer, http.StatusBadRequest, "save library order failed")
		return
	}
	h.publishOrder(categoryIDs, channelIDs)
	triggerUpdate(writer)
	writer.WriteHeader(http.StatusNoContent)
}

func (h *handler) publish(reason string, categoryID int64, channelIDs []string) {
	if h.updates == nil {
		return
	}
	h.updates.Publish(events.Update{
		Reason: reason, Scopes: events.AllScopes(), CategoryIDs: []int64{categoryID}, ChannelIDs: channelIDs,
	})
}

func (h *handler) publishOrder(categoryIDs []int64, channelIDs []string) {
	if h.updates == nil {
		return
	}
	h.updates.Publish(events.Update{
		Reason: "library.reordered", Scopes: events.AllScopes(), CategoryIDs: categoryIDs, ChannelIDs: channelIDs,
	})
}

func storeOrder(values []categoryOrderRequest) ([]store.CategoryOrder, []int64, []string) {
	order := make([]store.CategoryOrder, 0, len(values))
	categoryIDs := make([]int64, 0, len(values))
	channelIDs := make([]string, 0)
	for _, value := range values {
		order = append(order, store.CategoryOrder{CategoryID: value.ID, ChannelIDs: value.ChannelIDs})
		categoryIDs = append(categoryIDs, value.ID)
		channelIDs = append(channelIDs, value.ChannelIDs...)
	}
	return order, categoryIDs, channelIDs
}

func categoryID(writer http.ResponseWriter, request *http.Request) (int64, bool) {
	value, err := strconv.ParseInt(chi.URLParam(request, "categoryID"), 10, 64)
	if err != nil || value <= 0 {
		respond.Error(writer, http.StatusNotFound, "category not found")
		return 0, false
	}
	return value, true
}

func responseFromCategory(category store.Category) categoryResponse {
	return categoryResponse{
		ID: category.ID, Name: category.Name, Position: category.Position, IsDefault: category.IsDefault,
	}
}

func respondCategoryError(writer http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, store.ErrCategoryNameExists):
		respond.JSON(writer, http.StatusConflict, errorResponse{
			Error: "A category with this name already exists.", Code: "category_exists", Field: "name",
		})
	case errors.Is(err, store.ErrCategoryNameEmpty):
		respond.JSON(writer, http.StatusUnprocessableEntity, errorResponse{
			Error: "Category name is required.", Code: "category_name_required", Field: "name",
		})
	case errors.Is(err, store.ErrCategoryNameTooLong):
		respond.JSON(writer, http.StatusUnprocessableEntity, errorResponse{
			Error: "Category name must not exceed 50 characters.", Code: "category_name_too_long", Field: "name",
		})
	case errors.Is(err, store.ErrCategoryNotFound):
		respond.JSON(writer, http.StatusNotFound, errorResponse{Error: "Category not found.", Code: "category_not_found"})
	case errors.Is(err, store.ErrDefaultCategory):
		respond.JSON(writer, http.StatusConflict, errorResponse{
			Error: "The default category cannot be deleted.", Code: "default_category",
		})
	case errors.Is(err, store.ErrDefaultCategoryName):
		respond.JSON(writer, http.StatusConflict, errorResponse{
			Error: "The default category cannot be renamed.", Code: "default_category_name",
		})
	default:
		respond.JSON(writer, http.StatusInternalServerError, errorResponse{
			Error: "Category operation failed.", Code: "category_operation_failed",
		})
	}
}

func triggerUpdate(writer http.ResponseWriter) {
	writer.Header().Set("HX-Trigger", "tuby:update")
}
