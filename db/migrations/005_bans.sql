-- +goose Up
-- +goose StatementBegin
CREATE TABLE bans (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    reason TEXT NOT NULL DEFAULT '부정 사용',
    banned_by BIGINT NOT NULL REFERENCES users(id),
    banned_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    unbanned_at TIMESTAMP WITH TIME ZONE,
    unbanned_by BIGINT REFERENCES users(id)
);

CREATE INDEX idx_bans_user_id ON bans(user_id);
CREATE UNIQUE INDEX idx_bans_active_per_user ON bans(user_id) WHERE unbanned_at IS NULL;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_bans_active_per_user;
DROP INDEX IF EXISTS idx_bans_user_id;
DROP TABLE IF EXISTS bans;
-- +goose StatementEnd
