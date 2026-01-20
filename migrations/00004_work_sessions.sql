-- +goose Up
-- +goose StatementBegin

CREATE TABLE IF NOT EXISTS work_sessions(
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    project_id BIGINT REFERENCES  projects(id) ON DELETE CASCADE,
    start_at TIMESTAMPTZ NOT NULL,
    end_at TIMESTAMPTZ,
    note TEXT,
    created_at TIMESTAMPTZ
)

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS work_sessions;
-- +goose StatementEnd
