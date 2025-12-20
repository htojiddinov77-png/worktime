-- +goose Up
-- +goose StatementBegin

CREATE TABLE IF NOT EXISTS statuses(
    id BIGSERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL UNIQUE
);

INSERT INTO statuses (name)
VALUES('active'), ('inactive')
ON CONFLICT(name) DO NOTHING; 

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS statuses;
-- +goose StatementEnd
