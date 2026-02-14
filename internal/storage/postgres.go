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

func (s *PostgresStorage) AddMessage(ctx context.Context, conversationID string, branchID string, role, content string, statusCode int, errorText string) (*Message, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	// Get parent branch's last message to compute cumulative hash and next sequence number
	var lastHash string
	var nextSeq int
	err = tx.QueryRowContext(ctx,
		"SELECT cumulative_hash, sequence_number FROM messages WHERE branch_id = $1 ORDER BY sequence_number DESC LIMIT 1",
		branchID,
	).Scan(&lastHash, &nextSeq)
	if err != nil && err != sql.ErrNoRows {
		return nil, err
	}

	if err == sql.ErrNoRows {
		// First message in branch. If it's a child branch, we need to get the hash from the parent message.
		var parentMessageID sql.NullString
		err = tx.QueryRowContext(ctx, "SELECT parent_message_id FROM branches WHERE id = $1", branchID).Scan(&parentMessageID)
		if err != nil {
			return nil, err
		}
		if parentMessageID.Valid {
			err = tx.QueryRowContext(ctx, "SELECT cumulative_hash, sequence_number FROM messages WHERE id = $1", parentMessageID.String).Scan(&lastHash, &nextSeq)
			if err != nil {
				return nil, err
			}
		} else {
			lastHash = ""
			nextSeq = 0
		}
	}
	nextSeq++

	newHash := computeHash(lastHash, role, content)

	var msg Message
	err = tx.QueryRowContext(ctx,
		"INSERT INTO messages (conversation_id, branch_id, role, content, sequence_number, cumulative_hash, upstream_status_code, upstream_error) VALUES ($1, $2, $3, $4, $5, $6, $7, $8) RETURNING id, conversation_id, branch_id, role, content, sequence_number, cumulative_hash, created_at, upstream_status_code, upstream_error",
		conversationID, branchID, role, content, nextSeq, newHash, statusCode, errorText,
	).Scan(&msg.ID, &msg.ConversationID, &msg.BranchID, &msg.Role, &msg.Content, &msg.SequenceNumber, &msg.CumulativeHash, &msg.CreatedAt, &msg.UpstreamStatusCode, &msg.UpstreamError)
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
			SELECT id, parent_branch_id, parent_message_id 
			FROM branches WHERE id = $1
			UNION ALL
			SELECT b.id, b.parent_branch_id, b.parent_message_id
			FROM branches b
			JOIN branch_path bp ON b.id = bp.parent_branch_id
		)
		SELECT m.id, m.conversation_id, m.branch_id, m.role, m.content, m.sequence_number, m.cumulative_hash, m.created_at, m.upstream_status_code, m.upstream_error
		FROM messages m
		JOIN branch_path bp ON m.branch_id = bp.id
		WHERE (m.branch_id = $1) 
		   OR (m.sequence_number <= (SELECT m2.sequence_number FROM messages m2 WHERE m2.id = bp.parent_message_id))
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
		if err := rows.Scan(&m.ID, &m.ConversationID, &m.BranchID, &m.Role, &m.Content, &m.SequenceNumber, &m.CumulativeHash, &m.CreatedAt, &statusCode, &errorText); err != nil {
			return nil, err
		}
		if statusCode.Valid {
			m.UpstreamStatusCode = int(statusCode.Int32)
		}
		if errorText.Valid {
			m.UpstreamError = errorText.String
		}
		history = append(history, m)
	}
	return history, nil
}

func (s *PostgresStorage) FindBranchByHistory(ctx context.Context, conversationID string, history []struct{ Role, Content string }) (string, error) {
	currentHash := ""
	for _, m := range history {
		currentHash = computeHash(currentHash, m.Role, m.Content)
	}

	var branchID string
	err := s.db.QueryRowContext(ctx,
		"SELECT branch_id FROM messages WHERE conversation_id = $1 AND cumulative_hash = $2",
		conversationID, currentHash,
	).Scan(&branchID)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return branchID, nil
}

func (s *PostgresStorage) CreateBranch(ctx context.Context, conversationID string, parentBranchID string, parentMessageID string) (*Branch, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	var branch Branch
	err = tx.QueryRowContext(ctx,
		"INSERT INTO branches (conversation_id, parent_branch_id, parent_message_id) VALUES ($1, $2, $3) RETURNING id, conversation_id, parent_branch_id, parent_message_id, created_at",
		conversationID, parentBranchID, parentMessageID,
	).Scan(&branch.ID, &branch.ConversationID, &branch.ParentBranchID, &branch.ParentMessageID, &branch.CreatedAt)
	if err != nil {
		return nil, err
	}

	_, err = tx.ExecContext(ctx,
		"UPDATE messages SET child_branch_ids = array_append(child_branch_ids, $1) WHERE id = $2",
		branch.ID, parentMessageID,
	)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return &branch, nil
}

func computeHash(prevHash, role, content string) string {
	h := sha256.New()
	h.Write([]byte(prevHash))
	h.Write([]byte(role))
	h.Write([]byte(content))
	return hex.EncodeToString(h.Sum(nil))
}
