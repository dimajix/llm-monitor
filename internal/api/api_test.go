package api

import (
	"context"
	"encoding/json"
	"llm-monitor/internal/storage"
	"net/http"
	"net/http/httptest"
	"testing"
)

type mockStorage struct {
	storage.Storage
	listConversationsFunc func(ctx context.Context) ([]storage.ConversationOverview, error)
}

func (m *mockStorage) ListConversations(ctx context.Context) ([]storage.ConversationOverview, error) {
	return m.listConversationsFunc(ctx)
}

func TestAPIHandler_ListConversations(t *testing.T) {
	mock := &mockStorage{
		listConversationsFunc: func(ctx context.Context) ([]storage.ConversationOverview, error) {
			return []storage.ConversationOverview{
				{
					Conversation: storage.Conversation{ID: "conv1"},
					FirstMessage: &storage.Message{SimpleMessage: storage.SimpleMessage{Content: "First"}},
				},
			}, nil
		},
	}

	h := NewAPIHandler(mock)
	req := httptest.NewRequest("GET", "/conversations", nil)
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var resp []storage.ConversationOverview
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if len(resp) != 1 || resp[0].ID != "conv1" {
		t.Errorf("Unexpected response: %+v", resp)
	}
}
