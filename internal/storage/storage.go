package storage

import (
	"context"
	"encoding/json"
	"llm-monitor/internal/config"
	"time"

	"github.com/google/uuid"
)

// Conversation represents a high-level container for a chat session.
type Conversation struct {
	ID          uuid.UUID              `json:"id"`
	CreatedAt   time.Time              `json:"created_at"`
	RequestType string                 `json:"request_type"`
	Metadata    map[string]interface{} `json:"metadata,omitzero"`
}

// ConversationOverview provides a summary of a conversation.
type ConversationOverview struct {
	Conversation
	SystemPrompt *Message `json:"system_prompt,omitzero"`
	FirstMessage *Message `json:"first_message,omitzero"`
	BranchCount  int      `json:"branch_count"`
}

// Branch represents a path within a conversation.
type Branch struct {
	ID              uuid.UUID  `json:"id"`
	ConversationID  uuid.UUID  `json:"conversation_id"`
	ParentBranchID  *uuid.UUID `json:"parent_branch_id,omitzero"`
	ParentMessageID *uuid.UUID `json:"parent_message_id,omitzero"`
	CreatedAt       time.Time  `json:"created_at"`
}

// Tool represents a reusable tool definition.
type Tool struct {
	ID          uuid.UUID       `json:"id"`
	Name        string          `json:"name"`
	Description string          `json:"description,omitzero"`
	Parameters  json.RawMessage `json:"parameters,omitzero"`
}

// ToolCall represents an actual tool call in a message.
type ToolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

// SimpleMessage represents a basic chat message with role and content.
type SimpleMessage struct {
	Role               string         `json:"role"`
	Content            string         `json:"content"`
	Model              string         `json:"model,omitzero"`
	PromptTokens       int            `json:"prompt_tokens,omitzero"`
	CompletionTokens   int            `json:"completion_tokens,omitzero"`
	PromptEvalDuration time.Duration  `json:"prompt_eval_duration,omitzero"`
	EvalDuration       time.Duration  `json:"eval_duration,omitzero"`
	ClientHost         string         `json:"client_host,omitzero"`
	UpstreamHost       string         `json:"upstream_host,omitzero"`
	Metadata           map[string]any `json:"metadata,omitzero"`
	Tools              []Tool         `json:"tools,omitzero"`
	ToolCalls          []ToolCall     `json:"tool_calls,omitzero"`
	ToolCallID         string         `json:"tool_call_id,omitzero"`
}

// Message represents a single chat message.
type Message struct {
	SimpleMessage
	ID                 uuid.UUID   `json:"id"`
	ConversationID     uuid.UUID   `json:"conversation_id"`
	BranchID           uuid.UUID   `json:"branch_id"`
	SequenceNumber     int         `json:"sequence_number"`
	ChildBranchIDs     []uuid.UUID `json:"child_branch_ids,omitzero"`
	CreatedAt          time.Time   `json:"created_at"`
	ParentMessageID    *uuid.UUID  `json:"parent_message_id,omitzero"`
	UpstreamStatusCode int         `json:"upstream_status_code,omitzero"`
	UpstreamError      *string     `json:"upstream_error,omitzero"`
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
	GetConversation(ctx context.Context, id uuid.UUID) (*Conversation, error)

	// AddMessage adds a message to an existing branch.
	// If parentMessageID is provided, it uses that message as the parent.
	// If the parent message is not the tip of its branch, a new branch is automatically created.
	AddMessage(ctx context.Context, parentMessageID uuid.UUID, message *Message) (*Message, error)

	// GetBranchHistory retrieves the full message history for a specific branch.
	GetBranchHistory(ctx context.Context, branchID uuid.UUID) ([]Message, error)

	// FindMessageByHistory finds the deepest matching message ID
	// for the provided sequence of (role, content) pairs within a specific request type.
	FindMessageByHistory(ctx context.Context, history []SimpleMessage, requestType string) (messageID uuid.UUID, err error)

	// ListConversations returns a list of all conversations, including their first message.
	ListConversations(ctx context.Context, p Pagination) ([]ConversationOverview, error)

	// SearchMessages searches for messages containing the given text snippet.
	SearchMessages(ctx context.Context, query string, p Pagination) ([]Message, error)

	// GetConversationMessages retrieves all messages belonging to a conversation.
	GetConversationMessages(ctx context.Context, conversationID uuid.UUID) ([]Message, error)

	// GetBranch retrieves a branch by ID.
	GetBranch(ctx context.Context, branchID uuid.UUID) (*Branch, error)
}

// CreateStorage creates a storage instance based on configuration
func CreateStorage(cfg config.Storage) (Storage, error) {
	if cfg.Type == "postgres" && cfg.Postgres != nil {
		return NewPostgresStorage(cfg.Postgres.DSN)
	}
	return nil, nil
}
