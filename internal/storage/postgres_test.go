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
	conv, branch, err := storage.CreateConversation(ctx, nil)
	if err != nil {
		t.Fatalf("Failed to create conversation: %v", err)
	}

	// 2. Add two messages
	m1, err := storage.AddMessage(ctx, conv.ID, branch.ID, "", "user", "Hello", 0, "")
	if err != nil {
		t.Fatalf("Failed to add message 1: %v", err)
	}
	m2, err := storage.AddMessage(ctx, conv.ID, branch.ID, m1.ID, "assistant", "Hi there!", 0, "")
	if err != nil {
		t.Fatalf("Failed to add message 2: %v", err)
	}

	// 3. Add a third message to the same branch
	m3, err := storage.AddMessage(ctx, conv.ID, branch.ID, m2.ID, "user", "How are you?", 0, "")
	if err != nil {
		t.Fatalf("Failed to add message 3: %v", err)
	}

	// 4. Now fork from m2 by adding a DIFFERENT message.
	// We use m2.ID as the parent.
	m4, err := storage.AddMessage(ctx, conv.ID, branch.ID, m2.ID, "user", "What is the weather?", 0, "")
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

	// 7. Test Idempotency
	m4_repeat, err := storage.AddMessage(ctx, conv.ID, branch.ID, m2.ID, "user", "What is the weather?", 0, "")
	if err != nil {
		t.Fatalf("Failed to add message 4 repeat: %v", err)
	}
	if m4_repeat.ID != m4.ID {
		t.Errorf("Idempotency failed: expected message ID %s, got %s", m4.ID, m4_repeat.ID)
	}
}
