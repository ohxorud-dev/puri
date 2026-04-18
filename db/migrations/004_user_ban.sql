-- +goose Up
-- +goose StatementBegin
ALTER TABLE users
    ADD COLUMN is_banned BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN banned_at TIMESTAMP WITH TIME ZONE;

CREATE INDEX idx_users_is_banned ON users(is_banned) WHERE is_banned = TRUE;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_users_is_banned;
ALTER TABLE users DROP COLUMN IF EXISTS banned_at;
ALTER TABLE users DROP COLUMN IF EXISTS is_banned;
-- +goose StatementEnd
