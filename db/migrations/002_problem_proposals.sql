-- +goose Up
-- +goose StatementBegin
CREATE TYPE proposal_status AS ENUM (
    'draft',
    'submitted',
    'reviewing',
    'approved',
    'rejected',
    'published'
);

CREATE TABLE problem_proposals (
    id BIGSERIAL PRIMARY KEY,
    author_user_id BIGINT NOT NULL REFERENCES users(id),
    title VARCHAR(200) NOT NULL,
    statement_md TEXT NOT NULL,
    time_limit VARCHAR(16) NOT NULL,
    memory_limit VARCHAR(16) NOT NULL,
    examples JSONB NOT NULL DEFAULT '[]'::jsonb,
    testcases BYTEA NOT NULL DEFAULT ''::bytea,
    reference_solution_source TEXT NOT NULL DEFAULT '',
    reference_solution_language VARCHAR(32) NOT NULL DEFAULT '',
    status proposal_status NOT NULL DEFAULT 'draft',
    review_notes TEXT,
    example_validation_result JSONB,
    published_problem_id INT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);
CREATE INDEX idx_proposals_author ON problem_proposals(author_user_id);
CREATE INDEX idx_proposals_status ON problem_proposals(status);
CREATE INDEX idx_proposals_created_at ON problem_proposals(created_at DESC);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS problem_proposals;
DROP TYPE IF EXISTS proposal_status;
-- +goose StatementEnd
