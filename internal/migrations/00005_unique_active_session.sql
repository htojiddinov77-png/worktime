-- +goose Up
-- +goose StatementBegin

CREATE UNIQUE INDEX IF NOT EXISTS one_active_session_per_user
ON work_sessions(user_id)
WHERE end_at IS NULL;

-- +goose StatementEnd


-- +goose Down
-- +goose StatementBegin

DROP INDEX IF EXISTS one_active_session_per_user;

-- +goose StatementEnd
