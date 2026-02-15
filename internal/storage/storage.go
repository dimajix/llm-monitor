package storage

import (
	"context"
	"time"
)

// Conversation represents a high-level container for a chat session.
type Conversation struct {
	ID        string                 `json:"id"`
	CreatedAt time.Time              `json:"created_at"`
	Metadata  map[string]interface{} `json:"metadata,omitzero"`
}

// Branch represents a path within a conversation.
type Branch struct {
	ID              string    `json:"id"`
	ConversationID  string    `json:"conversation_id"`
	ParentBranchID  *string   `json:"parent_branch_id,omitzero"`
	ParentMessageID *string   `json:"parent_message_id,omitzero"`
	CreatedAt       time.Time `json:"created_at"`
}

// SimpleMessage represents a basic chat message with role and content.
type SimpleMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
	Model   string `json:"model,omitzero"`
}

// Message represents a single chat message.
type Message struct {
	SimpleMessage
	ID                 string    `json:"id"`
	ConversationID     string    `json:"conversation_id"`
	BranchID           string    `json:"branch_id"`
	SequenceNumber     int       `json:"sequence_number"`
	ChildBranchIDs     []string  `json:"child_branch_ids,omitzero"`
	CreatedAt          time.Time `json:"created_at"`
	ParentMessageID    *string   `json:"parent_message_id,omitzero"`
	UpstreamStatusCode int       `json:"upstream_status_code,omitzero"`
	UpstreamError      *string   `json:"upstream_error,omitzero"`
}

// Storage defines the interface for persisting and retrieving conversation data.
type Storage interface {
	// CreateConversation creates a new conversation and its initial branch.
	CreateConversation(ctx context.Context, metadata map[string]interface{}) (*Conversation, *Branch, error)

	// GetConversation retrieves a conversation by ID.
	GetConversation(ctx context.Context, id string) (*Conversation, error)

	// AddMessage adds a message to an existing branch.
	// If parentMessageID is provided, it uses that message as the parent.
	// If the parent message is not the tip of its branch, a new branch is automatically created.
	AddMessage(ctx context.Context, parentMessageID string, message *Message) (*Message, error)

	// GetBranchHistory retrieves the full message history for a specific branch.
	GetBranchHistory(ctx context.Context, branchID string) ([]Message, error)

	// FindMessageByHistory finds the deepest matching message ID
	// for the provided sequence of (role, content) pairs.
	FindMessageByHistory(ctx context.Context, history []SimpleMessage) (messageID string, err error)
}
