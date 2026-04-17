package repository

import (
	"context"
	"strconv"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Submission struct {
	ID              int64
	UserID          int64
	Username        string
	ProblemID       int32
	Language        string
	SourceCode      string
	Status          string
	Result          *string
	ExecutionTimeMs *int32
	MemoryUsageKb   *int32
}

type SubmissionRepository struct {
	pool *pgxpool.Pool
}

func NewSubmissionRepository(pool *pgxpool.Pool) *SubmissionRepository {
	return &SubmissionRepository{pool: pool}
}

func (r *SubmissionRepository) Create(ctx context.Context, userID int64, problemID int32, language, sourceCode string) (*Submission, error) {
	var s Submission
	err := r.pool.QueryRow(ctx,
		`INSERT INTO submissions (user_id, problem_id, language, source_code, status) VALUES ($1, $2, $3, $4, 'PENDING') RETURNING id, user_id, problem_id, language, source_code, status, result, execution_time_ms, memory_usage_kb`,
		userID, problemID, language, sourceCode,
	).Scan(
		&s.ID, &s.UserID, &s.ProblemID, &s.Language, &s.SourceCode,
		&s.Status, &s.Result, &s.ExecutionTimeMs, &s.MemoryUsageKb,
	)
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *SubmissionRepository) GetByID(ctx context.Context, id int64) (*Submission, error) {
	var s Submission
	err := r.pool.QueryRow(ctx,
		`SELECT id, user_id, problem_id, language, source_code, status, result, execution_time_ms, memory_usage_kb FROM submissions WHERE id = $1`,
		id,
	).Scan(
		&s.ID, &s.UserID, &s.ProblemID, &s.Language, &s.SourceCode,
		&s.Status, &s.Result, &s.ExecutionTimeMs, &s.MemoryUsageKb,
	)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *SubmissionRepository) UpdateStatus(ctx context.Context, id int64, status string, result string, executionTimeMs int32, memoryUsageKb int32) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE submissions SET status = $2, result = $3, execution_time_ms = $4, memory_usage_kb = $5 WHERE id = $1`,
		id, status, result, executionTimeMs, memoryUsageKb,
	)
	return err
}

func (r *SubmissionRepository) List(ctx context.Context, userID *int64, problemID *int32, limit int32, offset int32) ([]*Submission, error) {
	query := `SELECT s.id, s.user_id, COALESCE(u.username, ''), s.problem_id, s.language, s.source_code, s.status, s.result, s.execution_time_ms, s.memory_usage_kb FROM submissions s LEFT JOIN users u ON s.user_id = u.id`
	var args []interface{}
	var conditions []string

	if userID != nil {
		conditions = append(conditions, "s.user_id = $"+strconv.Itoa(len(args)+1))
		args = append(args, *userID)
	}
	if problemID != nil {
		conditions = append(conditions, "s.problem_id = $"+strconv.Itoa(len(args)+1))
		args = append(args, *problemID)
	}

	if len(conditions) > 0 {
		query += " WHERE " + conditions[0]
		for i := 1; i < len(conditions); i++ {
			query += " AND " + conditions[i]
		}
	}

	query += " ORDER BY s.created_at DESC LIMIT $" + strconv.Itoa(len(args)+1) + " OFFSET $" + strconv.Itoa(len(args)+2)
	args = append(args, limit, offset)

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var submissions []*Submission
	for rows.Next() {
		var s Submission
		if err := rows.Scan(
			&s.ID, &s.UserID, &s.Username, &s.ProblemID, &s.Language, &s.SourceCode,
			&s.Status, &s.Result, &s.ExecutionTimeMs, &s.MemoryUsageKb,
		); err != nil {
			return nil, err
		}
		submissions = append(submissions, &s)
	}
	return submissions, rows.Err()
}
