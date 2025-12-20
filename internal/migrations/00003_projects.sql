-- +goose Up
-- +goose StatementBegin

CREATE TABLE IF NOT EXISTS projects(
    id BIGSERIAL PRIMARY KEY,
    name VARCHAR(255),
    status_id BIGINT NOT NULL REFERENCES  statuses(id),
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
)

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS projects;
-- +goose StatementEnd
