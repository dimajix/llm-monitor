package storage

import (
	"context"
	"database/sql"
	"os"
	"testing"

	_ "github.com/lib/pq"
)

func TestPostgresStorage_Branching(t *testing.T) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}

	ctx := context.Background()
	storage, err := NewPostgresStorage(dsn)
	if err != nil {
		t.Fatalf("Failed to connect to storage: %v", err)
	}

	// Clean up for test
	_, _ = storage.db.Exec("DELETE FROM messages")
	_, _ = storage.db.Exec("DELETE FROM branches")
	_, _ = storage.db.Exec("DELETE FROM conversations")

	// 1. Create a conversation
	_, branch, err := storage.CreateConversation(ctx, nil)
	if err != nil {
		t.Fatalf("Failed to create conversation: %v", err)
	}

	// 2. Add two messages
	m1, err := storage.AddMessage(ctx, "", &Message{BranchID: branch.ID, SimpleMessage: SimpleMessage{Role: "user", Content: "Hello"}})
	if err != nil {
		t.Fatalf("Failed to add message 1: %v", err)
	}
	m2, err := storage.AddMessage(ctx, m1.ID, &Message{SimpleMessage: SimpleMessage{Role: "assistant", Content: "Hi there!"}})
	if err != nil {
		t.Fatalf("Failed to add message 2: %v", err)
	}

	// 3. Add a third message to the same branch
	m3, err := storage.AddMessage(ctx, m2.ID, &Message{SimpleMessage: SimpleMessage{Role: "user", Content: "How are you?"}})
	if err != nil {
		t.Fatalf("Failed to add message 3: %v", err)
	}

	// 4. Now fork from m2 by adding a DIFFERENT message.
	// We use m2.ID as the parent.
	m4, err := storage.AddMessage(ctx, m2.ID, &Message{SimpleMessage: SimpleMessage{Role: "user", Content: "What is the weather?"}})
	if err != nil {
		t.Fatalf("Failed to add message 4: %v", err)
	}

	// Check branch properties
	var parentMsgID sql.NullString
	err = storage.db.QueryRow("SELECT parent_message_id FROM branches WHERE id = $1", m4.BranchID).Scan(&parentMsgID)
	if err != nil {
		t.Fatalf("Failed to query branch: %v", err)
	}
	if !parentMsgID.Valid || parentMsgID.String != m2.ID {
		t.Errorf("New branch parent message ID expected %s, got %v", m2.ID, parentMsgID)
	}

	if m4.BranchID == branch.ID {
		t.Errorf("Expected a new branch for m4, but got same branch ID")
	}

	if m4.SequenceNumber != 3 {
		t.Errorf("Expected sequence number 3 for m4, got %d", m4.SequenceNumber)
	}

	// 5. Verify m3 is still in the original branch
	historyOriginal, err := storage.GetBranchHistory(ctx, branch.ID)
	if err != nil {
		t.Fatalf("Failed to get original branch history: %v", err)
	}
	if len(historyOriginal) != 3 {
		t.Errorf("Expected 3 messages in original branch history, got %d", len(historyOriginal))
	}
	foundM3 := false
	for _, m := range historyOriginal {
		if m.ID == m3.ID {
			foundM3 = true
			break
		}
	}
	if !foundM3 {
		t.Errorf("m3 not found in original branch history")
	}

	// 6. Verify m4 is in the new branch history
	historyNew, err := storage.GetBranchHistory(ctx, m4.BranchID)
	if err != nil {
		t.Fatalf("Failed to get new branch history: %v", err)
	}
	if len(historyNew) != 3 {
		t.Errorf("Expected 3 messages in new branch history, got %d", len(historyNew))
	}
	// History should be m1, m2, m4
	expectedIDs := []string{m1.ID, m2.ID, m4.ID}
	for i, m := range historyNew {
		if m.ID != expectedIDs[i] {
			t.Errorf("At index %d: expected message ID %s, got %s", i, expectedIDs[i], m.ID)
		}
	}

	// 7. Test Idempotency (now removed, should create a new message)
	m4_repeat, err := storage.AddMessage(ctx, m2.ID, &Message{SimpleMessage: SimpleMessage{Role: "user", Content: "What is the weather?"}})
	if err != nil {
		t.Fatalf("Failed to add message 4 repeat: %v", err)
	}
	if m4_repeat.ID == m4.ID {
		t.Errorf("Idempotency should be removed: expected different message ID, got same %s", m4.ID)
	}

	// 8. Test FindMessageByHistory
	history := []SimpleMessage{
		{Role: "user", Content: "Hello"},
		{Role: "assistant", Content: "Hi there!"},
		{Role: "user", Content: "What is the weather?"},
	}
	foundID, err := storage.FindMessageByHistory(ctx, history)
	if err != nil {
		t.Fatalf("FindMessageByHistory failed: %v", err)
	}
	if foundID != m4.ID {
		t.Errorf("FindMessageByHistory: expected %s, got %s", m4.ID, foundID)
	}

	// Test partial history
	historyPartial := []SimpleMessage{
		{Role: "user", Content: "Hello"},
		{Role: "assistant", Content: "Hi there!"},
	}
	foundIDPartial, err := storage.FindMessageByHistory(ctx, historyPartial)
	if err != nil {
		t.Fatalf("FindMessageByHistory failed: %v", err)
	}
	if foundIDPartial != m2.ID {
		t.Errorf("FindMessageByHistory (partial): expected %s, got %s", m2.ID, foundIDPartial)
	}

	// 9. Test ListConversations
	overviews, err := storage.ListConversations(ctx, Pagination{Limit: 1000, Offset: 0})
	if err != nil {
		t.Fatalf("ListConversations failed: %v", err)
	}
	if len(overviews) != 1 {
		t.Errorf("Expected 1 conversation overview, got %d", len(overviews))
	} else {
		if overviews[0].FirstMessage == nil {
			t.Errorf("Expected first message to be populated")
		} else if overviews[0].FirstMessage.ID != m1.ID {
			t.Errorf("Expected first message ID %s, got %s", m1.ID, overviews[0].FirstMessage.ID)
		}
	}

	// 10. Test SearchMessages
	searchResults, err := storage.SearchMessages(ctx, "weather", Pagination{Limit: 1000, Offset: 0})
	if err != nil {
		t.Fatalf("SearchMessages failed: %v", err)
	}
	// m4 and m4_repeat both have "weather"
	if len(searchResults) != 2 {
		t.Errorf("Expected 2 search results, got %d", len(searchResults))
	}

	// 11. Test GetConversationMessages
	convMessages, err := storage.GetConversationMessages(ctx, branch.ConversationID)
	if err != nil {
		t.Fatalf("GetConversationMessages failed: %v", err)
	}
	// m1, m2, m3, m4, m4_repeat
	if len(convMessages) != 5 {
		t.Errorf("Expected 5 conversation messages, got %d", len(convMessages))
	}

	// 12. Test GetBranch
	b, err := storage.GetBranch(ctx, m4.BranchID)
	if err != nil {
		t.Fatalf("GetBranch failed: %v", err)
	}
	if b == nil {
		t.Fatalf("Expected branch to be found")
	}
	if b.ID != m4.BranchID {
		t.Errorf("Expected branch ID %s, got %s", m4.BranchID, b.ID)
	}
	if b.ParentMessageID == nil || *b.ParentMessageID != m2.ID {
		t.Errorf("Expected parent message ID %s, got %v", m2.ID, b.ParentMessageID)
	}
}
