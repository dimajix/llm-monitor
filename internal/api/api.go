package api

import (
	"encoding/json"
	"llm-monitor/internal/storage"
	"llm-monitor/web"
	"net/http"
	"strconv"

	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
)

type APIHandler struct {
	storage storage.Storage
}

func NewAPIHandler(s storage.Storage) http.Handler {
	h := &APIHandler{storage: s}
	mux := http.NewServeMux()

	// Define routes with method and path parameters (Go 1.22+ style)
	mux.HandleFunc("GET /api/v1/conversations", h.listConversations)
	mux.HandleFunc("GET /api/v1/conversations/{id}", h.getConversationMessages)
	mux.HandleFunc("GET /api/v1/search", h.searchMessages)
	mux.HandleFunc("GET /api/v1/branches/{id}", h.getBranchMessages)

	// Serve static UI assets
	uiHandler := web.NewUIHandler()
	mux.Handle("/", uiHandler)

	// Wrap mux with CORS and Logging middleware
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		ww := &responseWriter{ResponseWriter: w, status: http.StatusOK}
		mux.ServeHTTP(ww, r)

		logrus.WithFields(logrus.Fields{
			"method":   r.Method,
			"path":     r.URL.Path,
			"status":   ww.status,
			"duration": time.Since(start),
			"remote":   r.RemoteAddr,
		}).Info("HTTP request")
	})
}

type responseWriter struct {
	http.ResponseWriter
	status int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.status = code
	rw.ResponseWriter.WriteHeader(code)
}

func (h *APIHandler) listConversations(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	p := h.getPagination(r)
	overviews, err := h.storage.ListConversations(ctx, p)
	if err != nil {
		logrus.WithError(err).Error("Failed to list conversations")
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	respondJSON(w, overviews)
}

func (h *APIHandler) getConversationMessages(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := r.PathValue("id")
	if id == "" {
		http.Error(w, "Conversation ID is required", http.StatusBadRequest)
		return
	}

	uid, err := uuid.Parse(id)
	if err != nil {
		logrus.WithError(err).Errorf("Failed to parse conversation id %s", id)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	conv, err := h.storage.GetConversation(ctx, uid)
	if err != nil {
		logrus.WithError(err).Errorf("Failed to check conversation %s", id)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	if conv == nil {
		http.NotFound(w, r)
		return
	}

	messages, err := h.storage.GetConversationMessages(ctx, uid)
	if err != nil {
		logrus.WithError(err).Errorf("Failed to get messages for conversation %s", id)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	result := struct {
		Conversation *storage.Conversation `json:"conversation"`
		Messages     []storage.Message     `json:"messages"`
	}{
		Conversation: conv,
		Messages:     messages,
	}

	respondJSON(w, result)
}

func (h *APIHandler) searchMessages(w http.ResponseWriter, r *http.Request) {
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

func (h *APIHandler) getBranchMessages(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	branchID := r.PathValue("id")
	if branchID == "" {
		http.Error(w, "Branch ID is required", http.StatusBadRequest)
		return
	}
	uid, err := uuid.Parse(branchID)
	if err != nil {
		logrus.WithError(err).Errorf("Failed to parse branch id %s", uid)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	branch, err := h.storage.GetBranch(ctx, uid)
	if err != nil {
		logrus.WithError(err).Errorf("Failed to get branch %s", uid)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	if branch == nil {
		http.NotFound(w, r)
		return
	}

	messages, err := h.storage.GetBranchHistory(ctx, uid)
	if err != nil {
		logrus.WithError(err).Errorf("Failed to get branch history %s", uid)
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
