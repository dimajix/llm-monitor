package storage

import (
	"context"
	"llm-monitor/internal/config"
	"time"
)

// Conversation represents a high-level container for a chat session.
type Conversation struct {
	ID          string                 `json:"id"`
	CreatedAt   time.Time              `json:"created_at"`
	RequestType string                 `json:"request_type"`
	Metadata    map[string]interface{} `json:"metadata,omitzero"`
}

// ConversationOverview provides a summary of a conversation.
type ConversationOverview struct {
	Conversation
	FirstMessage *Message `json:"first_message,omitzero"`
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
	Role               string        `json:"role"`
	Content            string        `json:"content"`
	Model              string        `json:"model,omitzero"`
	PromptTokens       int           `json:"prompt_tokens,omitzero"`
	CompletionTokens   int           `json:"completion_tokens,omitzero"`
	PromptEvalDuration time.Duration `json:"prompt_eval_duration,omitzero"`
	EvalDuration       time.Duration `json:"eval_duration,omitzero"`
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

// Pagination defines parameters for paginated queries.
type Pagination struct {
	Limit  int
	Offset int
}

// Storage defines the interface for persisting and retrieving conversation data.
type Storage interface {
	// CreateConversation creates a new conversation and its initial branch.
	CreateConversation(ctx context.Context, metadata map[string]interface{}, requestType string) (*Conversation, *Branch, error)

	// GetConversation retrieves a conversation by ID.
	GetConversation(ctx context.Context, id string) (*Conversation, error)

	// AddMessage adds a message to an existing branch.
	// If parentMessageID is provided, it uses that message as the parent.
	// If the parent message is not the tip of its branch, a new branch is automatically created.
	AddMessage(ctx context.Context, parentMessageID string, message *Message) (*Message, error)

	// GetBranchHistory retrieves the full message history for a specific branch.
	GetBranchHistory(ctx context.Context, branchID string) ([]Message, error)

	// FindMessageByHistory finds the deepest matching message ID
	// for the provided sequence of (role, content) pairs within a specific request type.
	FindMessageByHistory(ctx context.Context, history []SimpleMessage, requestType string) (messageID string, err error)

	// ListConversations returns a list of all conversations, including their first message.
	ListConversations(ctx context.Context, p Pagination) ([]ConversationOverview, error)

	// SearchMessages searches for messages containing the given text snippet.
	SearchMessages(ctx context.Context, query string, p Pagination) ([]Message, error)

	// GetConversationMessages retrieves all messages belonging to a conversation.
	GetConversationMessages(ctx context.Context, conversationID string) ([]Message, error)

	// GetBranch retrieves a branch by ID.
	GetBranch(ctx context.Context, branchID string) (*Branch, error)
}

// CreateStorage creates a storage instance based on configuration
func CreateStorage(cfg config.Storage) (Storage, error) {
	if cfg.Type == "postgres" && cfg.Postgres != nil {
		return NewPostgresStorage(cfg.Postgres.DSN)
	}
	return nil, nil
}
