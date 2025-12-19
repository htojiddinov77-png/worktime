-- +goose Up
-- +goose StatementBegin

CREATE TABLE IF NOT EXISTS statuses(
    id BIGSERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL UNIQUE,
);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS statuses;
-- +goose StatementEnd
