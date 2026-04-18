package repository

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func itoa(i int) string    { return strconv.Itoa(i) }
func joinComma(s []string) string { return strings.Join(s, ", ") }

type ProposalExample struct {
	Input  string `json:"input"`
	Output string `json:"output"`
}

type ProposalExampleValidation struct {
	Index           int32  `json:"index"`
	Passed          bool   `json:"passed"`
	ExpectedOutput  string `json:"expected_output"`
	ActualOutput    string `json:"actual_output"`
	Result          string `json:"result"`
	ExecutionTimeMs int32  `json:"execution_time_ms"`
}

type Proposal struct {
	ID                        int64
	AuthorUserID              int64
	Title                     string
	StatementMd               string
	TimeLimit                 string
	MemoryLimit               string
	ExamplesJSON              []byte
	TestcasesGz               []byte
	ReferenceSolutionSource   string
	ReferenceSolutionLanguage string
	Status                    string
	ReviewNotes               *string
	ExampleValidationResult   []byte
	PublishedProblemID        *int32
	CreatedAt                 *time.Time
	UpdatedAt                 *time.Time
	Kind                      string
	SaleTier                  *string
}

type ProposalRepository struct {
	pool *pgxpool.Pool
}

func NewProposalRepository(pool *pgxpool.Pool) *ProposalRepository {
	return &ProposalRepository{pool: pool}
}

func (r *ProposalRepository) Create(ctx context.Context, authorID int64, title string, kind string, saleTier *string) (*Proposal, error) {
	var p Proposal
	err := r.pool.QueryRow(ctx,
		`INSERT INTO problem_proposals (author_user_id, title, statement_md, time_limit, memory_limit, kind, sale_tier)
		 VALUES ($1, $2, '', '1초', '128MB', $3, $4)
		 RETURNING id, author_user_id, title, statement_md, time_limit, memory_limit, examples, testcases, reference_solution_source, reference_solution_language, status, review_notes, example_validation_result, published_problem_id, created_at, updated_at, kind, sale_tier`,
		authorID, title, kind, saleTier,
	).Scan(
		&p.ID, &p.AuthorUserID, &p.Title, &p.StatementMd, &p.TimeLimit, &p.MemoryLimit,
		&p.ExamplesJSON, &p.TestcasesGz, &p.ReferenceSolutionSource, &p.ReferenceSolutionLanguage,
		&p.Status, &p.ReviewNotes, &p.ExampleValidationResult, &p.PublishedProblemID,
		&p.CreatedAt, &p.UpdatedAt, &p.Kind, &p.SaleTier,
	)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (r *ProposalRepository) GetByID(ctx context.Context, id int64) (*Proposal, error) {
	var p Proposal
	err := r.pool.QueryRow(ctx,
		`SELECT id, author_user_id, title, statement_md, time_limit, memory_limit, examples, testcases, reference_solution_source, reference_solution_language, status, review_notes, example_validation_result, published_problem_id, created_at, updated_at, kind, sale_tier
		 FROM problem_proposals WHERE id = $1`,
		id,
	).Scan(
		&p.ID, &p.AuthorUserID, &p.Title, &p.StatementMd, &p.TimeLimit, &p.MemoryLimit,
		&p.ExamplesJSON, &p.TestcasesGz, &p.ReferenceSolutionSource, &p.ReferenceSolutionLanguage,
		&p.Status, &p.ReviewNotes, &p.ExampleValidationResult, &p.PublishedProblemID,
		&p.CreatedAt, &p.UpdatedAt, &p.Kind, &p.SaleTier,
	)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &p, nil
}

type ProposalUpdate struct {
	Title                     *string
	StatementMd               *string
	TimeLimit                 *string
	MemoryLimit               *string
	ExamplesJSON              *string
	TestcasesGz               []byte
	ReferenceSolutionSource   *string
	ReferenceSolutionLanguage *string
	SaleTier                  *string
}

func (r *ProposalRepository) Update(ctx context.Context, id int64, u ProposalUpdate) error {
	sets := []string{"updated_at = NOW()"}
	args := []interface{}{id}
	add := func(col string, val interface{}) {
		args = append(args, val)
		sets = append(sets, col+" = $"+itoa(len(args)))
	}
	if u.Title != nil {
		add("title", *u.Title)
	}
	if u.StatementMd != nil {
		add("statement_md", *u.StatementMd)
	}
	if u.TimeLimit != nil {
		add("time_limit", *u.TimeLimit)
	}
	if u.MemoryLimit != nil {
		add("memory_limit", *u.MemoryLimit)
	}
	if u.ExamplesJSON != nil {
		add("examples", *u.ExamplesJSON)
	}
	if u.TestcasesGz != nil {
		add("testcases", u.TestcasesGz)
	}
	if u.ReferenceSolutionSource != nil {
		add("reference_solution_source", *u.ReferenceSolutionSource)
	}
	if u.ReferenceSolutionLanguage != nil {
		add("reference_solution_language", *u.ReferenceSolutionLanguage)
	}
	if u.SaleTier != nil {
		add("sale_tier", *u.SaleTier)
	}
	if len(args) == 1 {
		return nil
	}
	q := "UPDATE problem_proposals SET " + joinComma(sets) + " WHERE id = $1"
	_, err := r.pool.Exec(ctx, q, args...)
	return err
}

func (r *ProposalRepository) UpdateStatus(ctx context.Context, id int64, status string, notes *string, validationResult []byte) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE problem_proposals SET status = $2, review_notes = $3, example_validation_result = $4, updated_at = NOW() WHERE id = $1`,
		id, status, notes, validationResult,
	)
	return err
}

func (r *ProposalRepository) ListByAuthor(ctx context.Context, authorID int64, limit, offset int32) ([]*Proposal, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, author_user_id, title, statement_md, time_limit, memory_limit, examples, testcases, reference_solution_source, reference_solution_language, status, review_notes, example_validation_result, published_problem_id, created_at, updated_at, kind, sale_tier
		 FROM problem_proposals WHERE author_user_id = $1 ORDER BY created_at DESC LIMIT $2 OFFSET $3`,
		authorID, limit, offset,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*Proposal
	for rows.Next() {
		var p Proposal
		if err := rows.Scan(
			&p.ID, &p.AuthorUserID, &p.Title, &p.StatementMd, &p.TimeLimit, &p.MemoryLimit,
			&p.ExamplesJSON, &p.TestcasesGz, &p.ReferenceSolutionSource, &p.ReferenceSolutionLanguage,
			&p.Status, &p.ReviewNotes, &p.ExampleValidationResult, &p.PublishedProblemID,
			&p.CreatedAt, &p.UpdatedAt, &p.Kind, &p.SaleTier,
		); err != nil {
			return nil, err
		}
		out = append(out, &p)
	}
	return out, rows.Err()
}
