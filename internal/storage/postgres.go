package storage

import (
	"context"
	"crypto/sha256"
	"database/sql"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"fmt"

	_ "github.com/lib/pq"
	"github.com/sirupsen/logrus"
)

type PostgresStorage struct {
	db *sql.DB
}

//go:embed schema.sql
var schemaSQL string

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

func (s *PostgresStorage) CreateConversation(ctx context.Context, metadata map[string]interface{}) (*Conversation, *Branch, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, nil, err
	}
	defer tx.Rollback()

	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		return nil, nil, err
	}

	var conv Conversation
	err = tx.QueryRowContext(ctx,
		"INSERT INTO conversations (metadata) VALUES ($1) RETURNING id, created_at",
		metadataJSON,
	).Scan(&conv.ID, &conv.CreatedAt)
	if err != nil {
		return nil, nil, err
	}
	conv.Metadata = metadata

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

func (s *PostgresStorage) GetConversation(ctx context.Context, id string) (*Conversation, error) {
	var conv Conversation
	var metadataJSON []byte
	err := s.db.QueryRowContext(ctx,
		"SELECT id, created_at, metadata FROM conversations WHERE id = $1",
		id,
	).Scan(&conv.ID, &conv.CreatedAt, &metadataJSON)
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

func (s *PostgresStorage) AddMessage(ctx context.Context, parentMessageID string, message *Message) (*Message, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	var branchID string
	var lastHash string
	var lastSeq int

	if parentMessageID != "" {
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

			branchID = newBranchID
		}
	} else {
		// No parent message means this must be the first message in a conversation.
		// However, we need a branchID to associate it with.
		// If message.BranchID is provided, we use it.
		branchID = message.BranchID
		if branchID == "" {
			return nil, fmt.Errorf("branchID is required when parentMessageID is empty")
		}

		lastHash = ""
		lastSeq = 0
	}

	nextSeq := lastSeq + 1
	newHash := computeHash(lastHash, message.Role, message.Content)

	var msg Message
	err = tx.QueryRowContext(ctx,
		"INSERT INTO messages (conversation_id, branch_id, role, content, model, sequence_number, cumulative_hash, upstream_status_code, upstream_error, parent_message_id) VALUES ((SELECT conversation_id FROM branches WHERE id = $1), $1, $2, $3, $4, $5, $6, $7, $8, $9) RETURNING id, conversation_id, branch_id, role, content, model, sequence_number, created_at, upstream_status_code, upstream_error, parent_message_id",
		branchID, message.Role, message.Content, message.Model, nextSeq, newHash, message.UpstreamStatusCode, message.UpstreamError, optional(parentMessageID),
	).Scan(&msg.ID, &msg.ConversationID, &msg.BranchID, &msg.Role, &msg.Content, &msg.Model, &msg.SequenceNumber, &msg.CreatedAt, &msg.UpstreamStatusCode, &msg.UpstreamError, &msg.ParentMessageID)

	if err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return &msg, nil
}

func (s *PostgresStorage) GetBranchHistory(ctx context.Context, branchID string) ([]Message, error) {
	query := `
		WITH RECURSIVE branch_path AS (
			SELECT id, parent_branch_id, parent_message_id, 0 as level
			FROM branches WHERE id = $1
			UNION ALL
			SELECT b.id, b.parent_branch_id, b.parent_message_id, bp.level + 1
			FROM branches b
			JOIN branch_path bp ON b.id = bp.parent_branch_id
		)
		SELECT m.id, m.conversation_id, m.branch_id, m.role, m.content, m.model, m.sequence_number, m.cumulative_hash, m.created_at, m.upstream_status_code, m.upstream_error, m.parent_message_id
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
	defer rows.Close()

	var history []Message
	for rows.Next() {
		var m Message
		var statusCode sql.NullInt32
		var errorText sql.NullString
		var parentMsgIDVal sql.NullString
		var cumulativeHash string
		var modelVal sql.NullString
		if err := rows.Scan(&m.ID, &m.ConversationID, &m.BranchID, &m.Role, &m.Content, &modelVal, &m.SequenceNumber, &cumulativeHash, &m.CreatedAt, &statusCode, &errorText, &parentMsgIDVal); err != nil {
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
		if parentMsgIDVal.Valid {
			m.ParentMessageID = &parentMsgIDVal.String
		}
		history = append(history, m)
	}
	return history, nil
}

func (s *PostgresStorage) FindMessageByHistory(ctx context.Context, history []SimpleMessage) (string, error) {
	if len(history) == 0 {
		return "", nil
	}

	currentHash := computeHistoryHash(history)
	var mID string
	err := s.db.QueryRowContext(ctx,
		"SELECT id FROM messages WHERE cumulative_hash = $1 ORDER BY created_at DESC LIMIT 1",
		currentHash,
	).Scan(&mID)

	if err == nil {
		return mID, nil
	}
	if err != sql.ErrNoRows {
		return "", err
	}
	return "", nil
}

func (s *PostgresStorage) ListConversations(ctx context.Context) ([]ConversationOverview, error) {
	query := `
		SELECT c.id, c.created_at, c.metadata,
		       m.id, m.conversation_id, m.branch_id, m.role, m.content, m.model, m.sequence_number, m.created_at, m.upstream_status_code, m.upstream_error, m.parent_message_id
		FROM conversations c
		LEFT JOIN LATERAL (
			SELECT * FROM messages m 
			WHERE m.conversation_id = c.id 
			ORDER BY m.sequence_number ASC LIMIT 1
		) m ON true
		ORDER BY c.created_at DESC
	`
	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var overviews []ConversationOverview
	for rows.Next() {
		var o ConversationOverview
		var metadata []byte
		var m Message
		var mID, mConvID, mBranchID, mRole, mContent, mModel, mError, mParentID sql.NullString
		var mSeq sql.NullInt32
		var mCreatedAt sql.NullTime
		var mStatus sql.NullInt32

		err := rows.Scan(
			&o.ID, &o.CreatedAt, &metadata,
			&mID, &mConvID, &mBranchID, &mRole, &mContent, &mModel, &mSeq, &mCreatedAt, &mStatus, &mError, &mParentID,
		)
		if err != nil {
			return nil, err
		}

		if metadata != nil {
			if err := json.Unmarshal(metadata, &o.Metadata); err != nil {
				return nil, err
			}
		}

		if mID.Valid {
			m.ID = mID.String
			m.ConversationID = mConvID.String
			m.BranchID = mBranchID.String
			m.Role = mRole.String
			m.Content = mContent.String
			m.Model = mModel.String
			m.SequenceNumber = int(mSeq.Int32)
			m.CreatedAt = mCreatedAt.Time
			if mStatus.Valid {
				m.UpstreamStatusCode = int(mStatus.Int32)
			}
			if mError.Valid {
				m.UpstreamError = &mError.String
			}
			if mParentID.Valid {
				m.ParentMessageID = &mParentID.String
			}
			o.FirstMessage = &m
		}

		overviews = append(overviews, o)
	}
	return overviews, nil
}

func (s *PostgresStorage) SearchMessages(ctx context.Context, query string) ([]Message, error) {
	sqlQuery := `
		SELECT id, conversation_id, branch_id, role, content, model, sequence_number, created_at, upstream_status_code, upstream_error, parent_message_id
		FROM messages
		WHERE content ILIKE $1
		ORDER BY created_at DESC
	`
	rows, err := s.db.QueryContext(ctx, sqlQuery, "%"+query+"%")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return s.scanMessages(rows)
}

func (s *PostgresStorage) GetConversationMessages(ctx context.Context, conversationID string) ([]Message, error) {
	query := `
		SELECT id, conversation_id, branch_id, role, content, model, sequence_number, created_at, upstream_status_code, upstream_error, parent_message_id
		FROM messages
		WHERE conversation_id = $1
		ORDER BY sequence_number ASC, created_at ASC
	`
	rows, err := s.db.QueryContext(ctx, query, conversationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return s.scanMessages(rows)
}

func (s *PostgresStorage) GetBranch(ctx context.Context, branchID string) (*Branch, error) {
	var b Branch
	var parentBranchID, parentMessageID sql.NullString
	err := s.db.QueryRowContext(ctx,
		"SELECT id, conversation_id, parent_branch_id, parent_message_id, created_at FROM branches WHERE id = $1",
		branchID,
	).Scan(&b.ID, &b.ConversationID, &parentBranchID, &parentMessageID, &b.CreatedAt)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	if parentBranchID.Valid {
		b.ParentBranchID = &parentBranchID.String
	}
	if parentMessageID.Valid {
		b.ParentMessageID = &parentMessageID.String
	}

	return &b, nil
}

func (s *PostgresStorage) scanMessages(rows *sql.Rows) ([]Message, error) {
	var messages []Message
	for rows.Next() {
		var m Message
		var modelVal, errorText, parentMsgIDVal sql.NullString
		var statusCode sql.NullInt32
		err := rows.Scan(
			&m.ID, &m.ConversationID, &m.BranchID, &m.Role, &m.Content, &modelVal, &m.SequenceNumber, &m.CreatedAt, &statusCode, &errorText, &parentMsgIDVal,
		)
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
		if parentMsgIDVal.Valid {
			m.ParentMessageID = &parentMsgIDVal.String
		}
		messages = append(messages, m)
	}
	return messages, nil
}

func optional(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func computeHistoryHash(history []SimpleMessage) string {
	currentHash := ""
	for _, m := range history {
		currentHash = computeHash(currentHash, m.Role, m.Content)
	}
	return currentHash
}

func computeHash(prevHash, role, content string) string {
	h := sha256.New()
	h.Write([]byte(prevHash))
	h.Write([]byte(role))
	h.Write([]byte(content))
	return hex.EncodeToString(h.Sum(nil))
}
