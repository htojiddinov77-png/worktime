-- +goose Up
-- +goose StatementBegin

CREATE TABLE IF NOT EXISTS batches (
    id          BIGSERIAL PRIMARY KEY,

    project_id  BIGINT NOT NULL
        REFERENCES projects(id) ON DELETE RESTRICT,

    created_by  BIGINT NOT NULL
        REFERENCES users(id) ON DELETE RESTRICT,

    description TEXT NOT NULL DEFAULT '',
    amount      BIGINT NOT NULL, -- total amount for the whole batch

    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    paid_at     TIMESTAMPTZ NULL
);

-- Useful indexes

-- List batches by project
CREATE INDEX IF NOT EXISTS idx_batches_project_created_at
    ON batches (project_id, created_at DESC);

-- Filter paid/unpaid by project
CREATE INDEX IF NOT EXISTS idx_batches_project_paid_at
    ON batches (project_id, paid_at);

-- Direct paid_at filtering
CREATE INDEX IF NOT EXISTS idx_batches_paid_at
    ON batches (paid_at);


-- Batch items table
CREATE TABLE IF NOT EXISTS batch_items (
    batch_id   BIGINT NOT NULL
        REFERENCES batches(id) ON DELETE CASCADE,

    session_id BIGINT NOT NULL
        REFERENCES work_sessions(id) ON DELETE RESTRICT,

    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    PRIMARY KEY (batch_id, session_id)
);

-- IMPORTANT BUSINESS RULE:
-- One session can belong to only one batch (cannot be paid twice)
CREATE UNIQUE INDEX IF NOT EXISTS uq_batch_items_session_id
    ON batch_items (session_id);

-- +goose StatementEnd


-- +goose Down
-- +goose StatementBegin


DROP INDEX IF EXISTS uq_batch_items_session_id;
DROP TABLE IF EXISTS batch_items;

DROP INDEX IF EXISTS idx_batches_paid_at;
DROP INDEX IF EXISTS idx_batches_project_paid_at;
DROP INDEX IF EXISTS idx_batches_project_created_at;
DROP TABLE IF EXISTS batches;

-- +goose StatementEnd