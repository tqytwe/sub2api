-- User-visible conversation history for mobile feedback tickets.

CREATE TABLE IF NOT EXISTS mobile_feedback_messages (
    id BIGSERIAL PRIMARY KEY,
    feedback_id BIGINT NOT NULL REFERENCES mobile_feedback(id) ON DELETE CASCADE,
    sender_type TEXT NOT NULL,
    content TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT mobile_feedback_messages_sender_check CHECK (sender_type IN ('user', 'support')),
    CONSTRAINT mobile_feedback_messages_content_check CHECK (char_length(content) BETWEEN 1 AND 2000)
);

CREATE INDEX IF NOT EXISTS idx_mobile_feedback_messages_feedback_created
    ON mobile_feedback_messages (feedback_id, created_at ASC, id ASC);
