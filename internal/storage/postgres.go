// Package storage provides a PostgreSQL-based storage implementation for conversations, branches, and messages.
package storage

import (
	"context"
	"crypto/sha256"
	"database/sql"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/sirupsen/logrus"
)

// PostgresStorage represents a PostgreSQL storage backend for conversations, branches, and messages.
type PostgresStorage struct {
	db *sql.DB
}

//go:embed schema.sql
var schemaSQL string

// NewPostgresStorage creates a new PostgreSQL storage instance with the given DSN.
// It initializes the database schema if it doesn't already exist.
// Returns a pointer to PostgresStorage and an error if initialization fails.
func NewPostgresStorage(dsn string) (*PostgresStorage, error) {
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, err
	}

	s := &PostgresStorage{db: db}
	if err := s.initSchema(context.Background()); err != nil {
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	return s, nil
}

// initSchema initializes the database schema if it doesn't already exist.
// It checks for the existence of the schema_version table and creates the schema if needed.
// Returns an error if schema initialization fails.
func (s *PostgresStorage) initSchema(ctx context.Context) error {
	var exists bool
	err := s.db.QueryRowContext(ctx, "SELECT EXISTS (SELECT FROM information_schema.tables WHERE table_name = 'schema_version')").Scan(&exists)
	if err != nil {
		return err
	}

	if !exists {
		logrus.Info("Initializing database schema")
		_, err = s.db.ExecContext(ctx, schemaSQL)
		if err != nil {
			return err
		}
	} else {
		var version int
		err = s.db.QueryRowContext(ctx, "SELECT MAX(version) FROM schema_version").Scan(&version)
		if err != nil {
			return err
		}
		logrus.WithField("version", version).Info("Database schema is up to date")
	}

	return nil
}

// CreateConversation creates a new conversation with the given metadata and returns the conversation and its initial branch.
// Returns a pointer to Conversation, a pointer to Branch, and an error.
func (s *PostgresStorage) CreateConversation(ctx context.Context, metadata map[string]interface{}, requestType string) (*Conversation, *Branch, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, nil, err
	}
	defer func(tx *sql.Tx) {
		_ = tx.Rollback()
	}(tx)

	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		return nil, nil, err
	}

	var conv Conversation
	err = tx.QueryRowContext(ctx,
		"INSERT INTO conversations (metadata, request_type) VALUES ($1, $2) RETURNING id, created_at, request_type",
		metadataJSON, requestType,
	).Scan(&conv.ID, &conv.CreatedAt, &conv.RequestType)
	if err != nil {
		return nil, nil, err
	}
	conv.Metadata = metadata
	conv.RequestType = requestType

	var branch Branch
	err = tx.QueryRowContext(ctx,
		"INSERT INTO branches (conversation_id) VALUES ($1) RETURNING id, conversation_id, created_at",
		conv.ID,
	).Scan(&branch.ID, &branch.ConversationID, &branch.CreatedAt)
	if err != nil {
		return nil, nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, nil, err
	}

	return &conv, &branch, nil
}

// GetConversation retrieves a conversation by its ID.
// Returns a pointer to Conversation and an error.
func (s *PostgresStorage) GetConversation(ctx context.Context, id uuid.UUID) (*Conversation, error) {
	var conv Conversation
	var metadataJSON []byte
	err := s.db.QueryRowContext(ctx,
		"SELECT id, created_at, request_type, metadata FROM conversations WHERE id = $1",
		id,
	).Scan(&conv.ID, &conv.CreatedAt, &conv.RequestType, &metadataJSON)
	if err != nil {
		return nil, err
	}

	if len(metadataJSON) > 0 {
		if err := json.Unmarshal(metadataJSON, &conv.Metadata); err != nil {
			logrus.WithError(err).Warn("Failed to unmarshal conversation metadata")
		}
	}

	return &conv, nil
}

// AddMessage adds a new message to a conversation, potentially forking the branch if needed.
// Returns a pointer to Message and an error.
func (s *PostgresStorage) AddMessage(ctx context.Context, parentMessageID uuid.UUID, message *Message) (*Message, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func(tx *sql.Tx) {
		_ = tx.Rollback()
	}(tx)

	var branchID uuid.UUID
	var lastHash string
	var lastSeq int

	if parentMessageID != uuid.Nil {
		// Use specific parent message
		err = tx.QueryRowContext(ctx,
			"SELECT branch_id, cumulative_hash, sequence_number FROM messages WHERE id = $1",
			parentMessageID,
		).Scan(&branchID, &lastHash, &lastSeq)
		if err != nil {
			return nil, err
		}

		// Check if we need to fork: if parentMessageID already has a child message
		var hasChildren bool
		err = tx.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM messages WHERE parent_message_id = $1)", parentMessageID).Scan(&hasChildren)
		if err != nil {
			return nil, err
		}

		if hasChildren {
			// Fork! Create a new branch from lastMsgID
			var newBranchID string
			err = tx.QueryRowContext(ctx,
				"INSERT INTO branches (conversation_id, parent_branch_id, parent_message_id) VALUES ((SELECT conversation_id FROM branches WHERE id = $1), $1, $2) RETURNING id",
				branchID, parentMessageID,
			).Scan(&newBranchID)
			if err != nil {
				return nil, err
			}

			// Update child_branch_ids in parent message
			_, err = tx.ExecContext(ctx,
				"UPDATE messages SET child_branch_ids = array_append(child_branch_ids, $1) WHERE id = $2",
				newBranchID, parentMessageID,
			)
			if err != nil {
				return nil, err
			}

			branchID, _ = uuid.Parse(newBranchID)
		}
	} else {
		// No parent message means this must be the first message in a conversation.
		// However, we need a branchID to associate it with.
		// If message.BranchID is provided, we use it.
		branchID = message.BranchID
		if branchID == uuid.Nil {
			return nil, fmt.Errorf("branchID is required when parentMessageID is empty")
		}

		lastHash = ""
		lastSeq = 0
	}

	nextSeq := lastSeq + 1
	newHash := computeHash(lastHash, message.Role, message.Content)

	metadataJSON, err := json.Marshal(message.Metadata)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal message metadata: %w", err)
	}

	var msg Message
	err = tx.QueryRowContext(ctx,
		"INSERT INTO messages (conversation_id, branch_id, role, content, model, sequence_number, cumulative_hash, upstream_status_code, upstream_error, prompt_tokens, completion_tokens, prompt_eval_duration, eval_duration, parent_message_id, client_host, upstream_host, metadata) VALUES ((SELECT conversation_id FROM branches WHERE id = $1), $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16) RETURNING id, conversation_id, branch_id, role, content, model, sequence_number, created_at, upstream_status_code, upstream_error, prompt_tokens, completion_tokens, prompt_eval_duration, eval_duration, parent_message_id, client_host, upstream_host, metadata",
		branchID, message.Role, message.Content, message.Model, nextSeq, newHash, message.UpstreamStatusCode, message.UpstreamError, message.PromptTokens, message.CompletionTokens, int64(message.PromptEvalDuration), int64(message.EvalDuration), optionalUUID(parentMessageID), message.ClientHost, message.UpstreamHost, metadataJSON,
	).Scan(&msg.ID, &msg.ConversationID, &msg.BranchID, &msg.Role, &msg.Content, &msg.Model, &msg.SequenceNumber, &msg.CreatedAt, &msg.UpstreamStatusCode, &msg.UpstreamError, &msg.PromptTokens, &msg.CompletionTokens, &msg.PromptEvalDuration, &msg.EvalDuration, &msg.ParentMessageID, &msg.ClientHost, &msg.UpstreamHost, &metadataJSON)

	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(metadataJSON, &msg.Metadata); err != nil {
		logrus.WithError(err).Warn("Failed to unmarshal message metadata")
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return &msg, nil
}

// GetBranchHistory retrieves the complete history of messages for a given branch.
// Returns a slice of Message and an error.
func (s *PostgresStorage) GetBranchHistory(ctx context.Context, branchID uuid.UUID) ([]Message, error) {
	query := `
		WITH RECURSIVE branch_path AS (
			SELECT id, parent_branch_id, parent_message_id, 0 as level
			FROM branches WHERE id = $1
			UNION ALL
			SELECT b.id, b.parent_branch_id, b.parent_message_id, bp.level + 1
			FROM branches b
			JOIN branch_path bp ON b.id = bp.parent_branch_id
		)
		SELECT m.id, m.conversation_id, m.branch_id, m.role, m.content, m.model, m.sequence_number, m.created_at, m.child_branch_ids,  m.upstream_status_code, m.upstream_error, m.prompt_tokens, m.completion_tokens, m.prompt_eval_duration, m.eval_duration, m.parent_message_id, m.client_host, m.upstream_host, m.metadata
		FROM messages m
		JOIN branch_path bp ON m.branch_id = bp.id
		WHERE (bp.level = 0) 
		   OR (m.sequence_number <= (SELECT m2.sequence_number FROM messages m2 WHERE m2.id = (SELECT bp2.parent_message_id FROM branch_path bp2 WHERE bp2.level = bp.level - 1)))
		ORDER BY m.sequence_number ASC;
	`
	rows, err := s.db.QueryContext(ctx, query, branchID)
	if err != nil {
		return nil, err
	}
	defer func(rows *sql.Rows) {
		_ = rows.Close()
	}(rows)

	return s.scanMessages(rows)
}

// FindMessageByHistory searches for a message in the database based on a history of messages.
// Returns the message ID if found, or an empty string and an error.
func (s *PostgresStorage) FindMessageByHistory(ctx context.Context, history []SimpleMessage, requestType string) (uuid.UUID, error) {
	if len(history) == 0 {
		return uuid.Nil, nil
	}

	currentHash := computeHistoryHash(history)
	var mID string
	err := s.db.QueryRowContext(ctx,
		"SELECT m.id FROM messages m JOIN conversations c ON m.conversation_id = c.id WHERE m.cumulative_hash = $1 AND c.request_type = $2 ORDER BY m.created_at DESC LIMIT 1",
		currentHash, requestType,
	).Scan(&mID)

	if err == nil {
		return uuid.Parse(mID)
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return uuid.Nil, err
	}
	return uuid.Nil, nil
}

// ListConversations retrieves a paginated list of conversations with their first messages.
// Returns a slice of ConversationOverview and an error.
func (s *PostgresStorage) ListConversations(ctx context.Context, p Pagination) ([]ConversationOverview, error) {
	query := `
		SELECT c.id, c.created_at, c.request_type, c.metadata,
	   			m1.id, m1.conversation_id, m1.branch_id, m1.role, m1.content, m1.model, m1.sequence_number, m1.created_at, m1.child_branch_ids, m1.upstream_status_code, m1.upstream_error, m1.prompt_tokens, m1.completion_tokens, m1.prompt_eval_duration, m1.eval_duration, m1.parent_message_id, m1.client_host, m1.upstream_host, m1.metadata,
	   			m2.id, m2.conversation_id, m2.branch_id, m2.role, m2.content, m2.model, m2.sequence_number, m2.created_at, m2.child_branch_ids, m2.upstream_status_code, m2.upstream_error, m2.prompt_tokens, m2.completion_tokens, m2.prompt_eval_duration, m2.eval_duration, m2.parent_message_id, m2.client_host, m2.upstream_host, m2.metadata
		FROM conversations c
		LEFT JOIN LATERAL (
			SELECT * FROM messages m 
			WHERE m.conversation_id = c.id AND m.role != 'system'
			ORDER BY m.sequence_number ASC LIMIT 1
		) m1 ON true
		LEFT JOIN LATERAL (
			SELECT * FROM messages m 
			WHERE m.conversation_id = c.id AND m.role = 'system'
			ORDER BY m.sequence_number ASC LIMIT 1
		) m2 ON true
		ORDER BY c.created_at DESC
		LIMIT $1 OFFSET $2
	`
	rows, err := s.db.QueryContext(ctx, query, p.Limit, p.Offset)
	if err != nil {
		return nil, err
	}
	defer func(rows *sql.Rows) {
		_ = rows.Close()
	}(rows)

	var overviews []ConversationOverview
	for rows.Next() {
		var o ConversationOverview
		var metadata []byte
		var m1ID, m1ConvID, m1BranchID, m1Role, m1Content, m1Model, m1Error, m1ParentID, m1ClientHost, m1UpstreamHost sql.NullString
		var m1ChildBranchIDs []string
		var m1Seq sql.NullInt32
		var m1CreatedAt sql.NullTime
		var m1Status, m1PromptTokens, m1CompletionTokens sql.NullInt32
		var m1PromptEvalDuration, m1EvalDuration sql.NullInt64
		var m1Metadata []byte

		var m2ID, m2ConvID, m2BranchID, m2Role, m2Content, m2Model, m2Error, m2ParentID, m2ClientHost, m2UpstreamHost sql.NullString
		var m2ChildBranchIDs []string
		var m2Seq sql.NullInt32
		var m2CreatedAt sql.NullTime
		var m2Status, m2PromptTokens, m2CompletionTokens sql.NullInt32
		var m2PromptEvalDuration, m2EvalDuration sql.NullInt64
		var m2Metadata []byte

		err := rows.Scan(
			&o.ID, &o.CreatedAt, &o.RequestType, &metadata,
			&m1ID, &m1ConvID, &m1BranchID, &m1Role, &m1Content, &m1Model, &m1Seq, &m1CreatedAt, pq.Array(&m1ChildBranchIDs), &m1Status, &m1Error, &m1PromptTokens, &m1CompletionTokens, &m1PromptEvalDuration, &m1EvalDuration, &m1ParentID, &m1ClientHost, &m1UpstreamHost, &m1Metadata,
			&m2ID, &m2ConvID, &m2BranchID, &m2Role, &m2Content, &m2Model, &m2Seq, &m2CreatedAt, pq.Array(&m2ChildBranchIDs), &m2Status, &m2Error, &m2PromptTokens, &m2CompletionTokens, &m2PromptEvalDuration, &m2EvalDuration, &m2ParentID, &m2ClientHost, &m2UpstreamHost, &m2Metadata,
		)
		if err != nil {
			return nil, err
		}

		if metadata != nil {
			if err := json.Unmarshal(metadata, &o.Metadata); err != nil {
				return nil, err
			}
		}

		if m1ID.Valid {
			var m1 Message
			m1.ID, _ = uuid.Parse(m1ID.String)
			m1.ConversationID, _ = uuid.Parse(m1ConvID.String)
			m1.BranchID, _ = uuid.Parse(m1BranchID.String)
			m1.Role = m1Role.String
			m1.Content = m1Content.String
			m1.Model = m1Model.String
			m1.SequenceNumber = int(m1Seq.Int32)
			m1.CreatedAt = m1CreatedAt.Time
			for _, idStr := range m1ChildBranchIDs {
				if uid, err := uuid.Parse(idStr); err == nil {
					m1.ChildBranchIDs = append(m1.ChildBranchIDs, uid)
				}
			}
			if m1Status.Valid {
				m1.UpstreamStatusCode = int(m1Status.Int32)
			}
			if m1Error.Valid {
				m1.UpstreamError = &m1Error.String
			}
			if m1PromptTokens.Valid {
				m1.PromptTokens = int(m1PromptTokens.Int32)
			}
			if m1CompletionTokens.Valid {
				m1.CompletionTokens = int(m1CompletionTokens.Int32)
			}
			if m1PromptEvalDuration.Valid {
				m1.PromptEvalDuration = time.Duration(m1PromptEvalDuration.Int64)
			}
			if m1EvalDuration.Valid {
				m1.EvalDuration = time.Duration(m1EvalDuration.Int64)
			}
			if m1ParentID.Valid {
				mid, _ := uuid.Parse(m1ParentID.String)
				m1.ParentMessageID = &mid
			}
			if m1ClientHost.Valid {
				m1.ClientHost = m1ClientHost.String
			}
			if m1UpstreamHost.Valid {
				m1.UpstreamHost = m1UpstreamHost.String
			}
			if len(m1Metadata) > 0 {
				if err := json.Unmarshal(m1Metadata, &m1.Metadata); err != nil {
					logrus.WithError(err).Warn("Failed to unmarshal message metadata")
				}
			}
			o.FirstMessage = &m1
		}

		if m2ID.Valid {
			var m2 Message
			m2.ID, _ = uuid.Parse(m2ID.String)
			m2.ConversationID, _ = uuid.Parse(m2ConvID.String)
			m2.BranchID, _ = uuid.Parse(m2BranchID.String)
			m2.Role = m2Role.String
			m2.Content = m2Content.String
			m2.Model = m2Model.String
			m2.SequenceNumber = int(m2Seq.Int32)
			m2.CreatedAt = m2CreatedAt.Time
			for _, idStr := range m2ChildBranchIDs {
				if uid, err := uuid.Parse(idStr); err == nil {
					m2.ChildBranchIDs = append(m2.ChildBranchIDs, uid)
				}
			}
			if m2Status.Valid {
				m2.UpstreamStatusCode = int(m2Status.Int32)
			}
			if m2Error.Valid {
				m2.UpstreamError = &m2Error.String
			}
			if m2PromptTokens.Valid {
				m2.PromptTokens = int(m2PromptTokens.Int32)
			}
			if m2CompletionTokens.Valid {
				m2.CompletionTokens = int(m2CompletionTokens.Int32)
			}
			if m2PromptEvalDuration.Valid {
				m2.PromptEvalDuration = time.Duration(m2PromptEvalDuration.Int64)
			}
			if m2EvalDuration.Valid {
				m2.EvalDuration = time.Duration(m2EvalDuration.Int64)
			}
			if m2ParentID.Valid {
				mid, _ := uuid.Parse(m2ParentID.String)
				m2.ParentMessageID = &mid
			}
			if m2ClientHost.Valid {
				m2.ClientHost = m2ClientHost.String
			}
			if m2UpstreamHost.Valid {
				m2.UpstreamHost = m2UpstreamHost.String
			}
			if len(m2Metadata) > 0 {
				if err := json.Unmarshal(m2Metadata, &m2.Metadata); err != nil {
					logrus.WithError(err).Warn("Failed to unmarshal message metadata")
				}
			}
			o.SystemPrompt = &m2
		}

		overviews = append(overviews, o)
	}
	return overviews, nil
}

// SearchMessages searches for messages containing the specified query string.
// Returns a slice of Message and an error.
func (s *PostgresStorage) SearchMessages(ctx context.Context, query string, p Pagination) ([]Message, error) {
	sqlQuery := `
		SELECT id, conversation_id, branch_id, role, content, model, sequence_number, created_at, child_branch_ids, upstream_status_code, upstream_error, prompt_tokens, completion_tokens, prompt_eval_duration, eval_duration, parent_message_id, client_host, upstream_host, metadata
		FROM messages
		WHERE content ILIKE $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`
	rows, err := s.db.QueryContext(ctx, sqlQuery, "%"+query+"%", p.Limit, p.Offset)
	if err != nil {
		return nil, err
	}
	defer func(rows *sql.Rows) {
		_ = rows.Close()
	}(rows)

	return s.scanMessages(rows)
}

// GetConversationMessages retrieves all messages for a given conversation ID.
// Returns a slice of Message and an error.
func (s *PostgresStorage) GetConversationMessages(ctx context.Context, conversationID uuid.UUID) ([]Message, error) {
	query := `
		SELECT id, conversation_id, branch_id, role, content, model, sequence_number, created_at, child_branch_ids, upstream_status_code, upstream_error, prompt_tokens, completion_tokens, prompt_eval_duration, eval_duration, parent_message_id, client_host, upstream_host, metadata
		FROM messages
		WHERE conversation_id = $1
		ORDER BY sequence_number ASC, created_at ASC
	`
	rows, err := s.db.QueryContext(ctx, query, conversationID)
	if err != nil {
		return nil, err
	}
	defer func(rows *sql.Rows) {
		_ = rows.Close()
	}(rows)

	return s.scanMessages(rows)
}

// GetBranch retrieves a branch by its ID.
// Returns a pointer to Branch and an error.
func (s *PostgresStorage) GetBranch(ctx context.Context, branchID uuid.UUID) (*Branch, error) {
	var b Branch
	var parentBranchID, parentMessageID sql.NullString
	err := s.db.QueryRowContext(ctx,
		"SELECT id, conversation_id, parent_branch_id, parent_message_id, created_at FROM branches WHERE id = $1",
		branchID,
	).Scan(&b.ID, &b.ConversationID, &parentBranchID, &parentMessageID, &b.CreatedAt)

	if //goland:noinspection GoDirectComparisonOfErrors
	err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	if parentBranchID.Valid {
		pbid, _ := uuid.Parse(parentBranchID.String)
		b.ParentBranchID = &pbid
	}
	if parentMessageID.Valid {
		pmid, _ := uuid.Parse(parentMessageID.String)
		b.ParentMessageID = &pmid
	}

	return &b, nil
}

// scanMessages scans a sql.Rows object and returns a slice of Message.
// Returns a slice of Message and an error.
func (s *PostgresStorage) scanMessages(rows *sql.Rows) ([]Message, error) {
	var messages []Message
	for rows.Next() {
		m, err := s.scanMessage(rows)
		if err != nil {
			return nil, err
		}
		if m != nil {
			messages = append(messages, *m)
		}
	}
	return messages, nil
}

func (s *PostgresStorage) scanMessage(rows *sql.Rows) (*Message, error) {
	var m Message
	var modelVal, errorText, parentMsgIDVal, clientHostVal, upstreamHostVal sql.NullString
	var statusCode, promptTokens, completionTokens sql.NullInt32
	var promptEvalDuration, evalDuration sql.NullInt64
	var metadataJSON []byte
	err := rows.Scan(
		&m.ID, &m.ConversationID, &m.BranchID, &m.Role, &m.Content, &modelVal, &m.SequenceNumber, &m.CreatedAt, pq.Array(&m.ChildBranchIDs), &statusCode, &errorText, &promptTokens, &completionTokens, &promptEvalDuration, &evalDuration, &parentMsgIDVal, &clientHostVal, &upstreamHostVal, &metadataJSON,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if modelVal.Valid {
		m.Model = modelVal.String
	}
	if statusCode.Valid {
		m.UpstreamStatusCode = int(statusCode.Int32)
	}
	if errorText.Valid {
		m.UpstreamError = &errorText.String
	}
	if promptTokens.Valid {
		m.PromptTokens = int(promptTokens.Int32)
	}
	if completionTokens.Valid {
		m.CompletionTokens = int(completionTokens.Int32)
	}
	if promptEvalDuration.Valid {
		m.PromptEvalDuration = time.Duration(promptEvalDuration.Int64)
	}
	if evalDuration.Valid {
		m.EvalDuration = time.Duration(evalDuration.Int64)
	}
	if parentMsgIDVal.Valid {
		pmid, _ := uuid.Parse(parentMsgIDVal.String)
		m.ParentMessageID = &pmid
	}
	if clientHostVal.Valid {
		m.ClientHost = clientHostVal.String
	}
	if upstreamHostVal.Valid {
		m.UpstreamHost = upstreamHostVal.String
	}
	if len(metadataJSON) > 0 {
		if err := json.Unmarshal(metadataJSON, &m.Metadata); err != nil {
			logrus.WithError(err).Warn("Failed to unmarshal message metadata")
		}
	}
	return &m, nil
}

// optional returns a pointer to the given string if it's not empty, otherwise returns nil.
func optional(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func optionalUUID(s uuid.UUID) *uuid.UUID {
	if s == uuid.Nil {
		return nil
	}
	return &s
}

// computeHistoryHash computes a hash for a sequence of messages.
// Returns the computed hash as a string.
func computeHistoryHash(history []SimpleMessage) string {
	currentHash := ""
	for _, m := range history {
		currentHash = computeHash(currentHash, m.Role, m.Content)
	}
	return currentHash
}

// computeHash computes a SHA256 hash of the previous hash, role, and content.
// Returns the computed hash as a hex-encoded string.
func computeHash(prevHash, role, content string) string {
	h := sha256.New()
	h.Write([]byte(prevHash))
	h.Write([]byte(role))
	h.Write([]byte(content))
	return hex.EncodeToString(h.Sum(nil))
}
