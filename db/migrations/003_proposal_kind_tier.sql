-- +goose Up
-- +goose StatementBegin
CREATE TYPE proposal_kind AS ENUM (
    'general',
    'sale'
);

CREATE TYPE sale_tier AS ENUM (
    'platinum',
    'diamond',
    'ruby'
);

ALTER TABLE problem_proposals
    ADD COLUMN kind proposal_kind NOT NULL DEFAULT 'general',
    ADD COLUMN sale_tier sale_tier;

ALTER TABLE problem_proposals
    ADD CONSTRAINT problem_proposals_sale_tier_required
    CHECK ((kind = 'sale' AND sale_tier IS NOT NULL) OR (kind = 'general' AND sale_tier IS NULL));

CREATE INDEX idx_proposals_kind ON problem_proposals(kind);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_proposals_kind;
ALTER TABLE problem_proposals DROP CONSTRAINT IF EXISTS problem_proposals_sale_tier_required;
ALTER TABLE problem_proposals DROP COLUMN IF EXISTS sale_tier;
ALTER TABLE problem_proposals DROP COLUMN IF EXISTS kind;
DROP TYPE IF EXISTS sale_tier;
DROP TYPE IF EXISTS proposal_kind;
-- +goose StatementEnd
