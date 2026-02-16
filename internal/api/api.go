package api

import (
	"encoding/json"
	"llm-monitor/internal/storage"
	"net/http"
	"strconv"
	"strings"

	"github.com/sirupsen/logrus"
)

type APIHandler struct {
	storage storage.Storage
}

func NewAPIHandler(s storage.Storage) *APIHandler {
	return &APIHandler{storage: s}
}

func (h *APIHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/api/v1/conversations") {
		h.handleConversations(w, r)
		return
	}
	if strings.HasPrefix(r.URL.Path, "/api/v1/search") {
		h.handleSearch(w, r)
		return
	}
	if strings.HasPrefix(r.URL.Path, "/api/v1/branches/") {
		h.handleBranch(w, r)
		return
	}

	http.NotFound(w, r)
}

func (h *APIHandler) handleConversations(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/conversations")
	path = strings.Trim(path, "/")

	if path == "" {
		// List all conversations
		p := h.getPagination(r)
		overviews, err := h.storage.ListConversations(ctx, p)
		if err != nil {
			logrus.WithError(err).Error("Failed to list conversations")
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
		respondJSON(w, overviews)
		return
	}

	// Get specific conversation messages
	// path is the ID
	messages, err := h.storage.GetConversationMessages(ctx, path)
	if err != nil {
		logrus.WithError(err).Errorf("Failed to get messages for conversation %s", path)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	if len(messages) == 0 {
		// Check if conversation exists
		conv, err := h.storage.GetConversation(ctx, path)
		if err != nil {
			logrus.WithError(err).Errorf("Failed to check conversation %s", path)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
		if conv == nil {
			http.NotFound(w, r)
			return
		}
	}
	respondJSON(w, messages)
}

func (h *APIHandler) handleSearch(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	query := r.URL.Query().Get("q")
	if query == "" {
		http.Error(w, "Query parameter 'q' is required", http.StatusBadRequest)
		return
	}

	messages, err := h.storage.SearchMessages(ctx, query, h.getPagination(r))
	if err != nil {
		logrus.WithError(err).Error("Failed to search messages")
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	respondJSON(w, messages)
}

func (h *APIHandler) handleBranch(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	branchID := strings.TrimPrefix(r.URL.Path, "/branches/")
	if branchID == "" {
		http.Error(w, "Branch ID is required", http.StatusBadRequest)
		return
	}

	branch, err := h.storage.GetBranch(ctx, branchID)
	if err != nil {
		logrus.WithError(err).Errorf("Failed to get branch %s", branchID)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	if branch == nil {
		http.NotFound(w, r)
		return
	}

	messages, err := h.storage.GetBranchHistory(ctx, branchID)
	if err != nil {
		logrus.WithError(err).Errorf("Failed to get branch history %s", branchID)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	result := struct {
		Branch   *storage.Branch   `json:"branch"`
		Messages []storage.Message `json:"messages"`
	}{
		Branch:   branch,
		Messages: messages,
	}

	respondJSON(w, result)
}

func respondJSON(w http.ResponseWriter, data any) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(data); err != nil {
		logrus.WithError(err).Error("Failed to encode JSON response")
	}
}

func (h *APIHandler) getPagination(r *http.Request) storage.Pagination {
	limitStr := r.URL.Query().Get("limit")
	offsetStr := r.URL.Query().Get("offset")

	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		limit = 100
	}
	if limit > 1000 {
		limit = 1000
	}

	offset, err := strconv.Atoi(offsetStr)
	if err != nil || offset < 0 {
		offset = 0
	}

	return storage.Pagination{
		Limit:  limit,
		Offset: offset,
	}
}
