-- +goose Up
-- +goose StatementBegin

CREATE TABLE reset_tokens (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash BYTEA NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    is_used BOOLEAN NOT NULL DEFAULT FALSE
);

-- Ensures that each reset token is unique
-- and allows fast lookup when validating a reset link

CREATE UNIQUE INDEX idx_reset_tokens_token_hash -- 
    ON reset_tokens(token_hash);

 -- Speeds up operations that work with all reset tokens
-- belonging to a specific user.

CREATE INDEX idx_reset_tokens_user_id -- 
    ON reset_tokens(user_id);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP TABLE IF EXISTS reset_tokens;

-- +goose StatementEnd
