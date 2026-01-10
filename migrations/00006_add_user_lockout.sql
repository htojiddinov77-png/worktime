-- +goose Up
-- +goose StatementBegin

ALTER TABLE users
ADD COLUMN IF NOT EXISTS is_locked BOOLEAN NOT NULL DEFAULT FALSE,
ADD COLUMN IF NOT EXISTS last_failed_login TIMESTAMPTZ NULL,
ADD COLUMN IF NOT EXISTS failed_attempts INT NOT NULL DEFAULT 0;


ALTER TABLE users
ADD CONSTRAINT users_failed_attempts_non_negative -- constraint is rule 
CHECK (failed_attempts >= 0);

-- +goose StatementEnd


-- +goose Down
-- +goose StatementBegin

ALTER TABLE users
DROP CONSTRAINT IF EXISTS users_failed_attempts_non_negative;

ALTER TABLE users
DROP COLUMN IF EXISTS is_locked,
DROP COLUMN IF EXISTS last_failed_login,
DROP COLUMN IF EXISTS failed_attempts;

-- +goose StatementEnd
