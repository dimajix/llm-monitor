-- Enable UUID extension
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- 1. Conversation Table: High-level container
CREATE TABLE conversations (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    request_type VARCHAR(50) NOT NULL,
    metadata JSONB
);

-- 2. Message Table (Forward Declaration of sort for Branch references)
-- We'll create branches first, then messages, then add the foreign key.

-- 3. Branch Table: Defines paths within a conversation
CREATE TABLE branches (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    conversation_id UUID NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
    parent_branch_id UUID REFERENCES branches(id),
    parent_message_id UUID, -- Will add FK constraint later
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- 4. Message Table: The actual content
CREATE TABLE messages (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    conversation_id UUID NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
    branch_id UUID NOT NULL REFERENCES branches(id) ON DELETE CASCADE,
    role VARCHAR(50) NOT NULL,
    content TEXT NOT NULL,
    model VARCHAR(255),
    sequence_number INT NOT NULL,
    cumulative_hash VARCHAR(64) NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    child_branch_ids UUID[] DEFAULT '{}',
    upstream_status_code INT,
    upstream_error TEXT,
    prompt_tokens INT,
    completion_tokens INT,
    prompt_eval_duration BIGINT,
    eval_duration BIGINT,
    parent_message_id UUID REFERENCES messages(id),
    client_host VARCHAR(128),
    upstream_host VARCHAR(128),
    metadata JSONB,
    
    UNIQUE (branch_id, sequence_number)
);

-- Add foreign key constraint to branches for parent_message_id
ALTER TABLE branches 
ADD CONSTRAINT fk_parent_message 
FOREIGN KEY (parent_message_id) REFERENCES messages(id);

-- Indexes for performance
CREATE INDEX idx_messages_branch_seq ON messages (branch_id, sequence_number);
CREATE INDEX idx_messages_conversation ON messages (conversation_id);
CREATE INDEX idx_messages_hash ON messages (cumulative_hash);
CREATE INDEX idx_messages_children ON messages USING GIN (child_branch_ids);
CREATE INDEX idx_messages_parent ON messages (parent_message_id);

-- Schema versioning
CREATE TABLE IF NOT EXISTS schema_version (
    version INT PRIMARY KEY,
    applied_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

INSERT INTO schema_version (version) VALUES (8) ON CONFLICT (version) DO UPDATE SET version = 8;
