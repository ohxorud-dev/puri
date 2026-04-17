package repository

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type User struct {
	ID           int64
	Username     string
	Email        string
	PasswordHash string
	DisplayName  *string
	BojHandle    *string
	Role         string
	CreatedAt    *time.Time
}

type UserRepository struct {
	pool *pgxpool.Pool
}

func NewUserRepository(pool *pgxpool.Pool) *UserRepository {
	return &UserRepository{pool: pool}
}

func (r *UserRepository) Create(ctx context.Context, username, email, passwordHash string) (*User, error) {
	var user User
	err := r.pool.QueryRow(ctx,
		`INSERT INTO users (username, email, password_hash) VALUES ($1, $2, $3) RETURNING id, username, email, password_hash, display_name, boj_handle, role, created_at`,
		username, email, passwordHash,
	).Scan(
		&user.ID, &user.Username, &user.Email, &user.PasswordHash,
		&user.DisplayName, &user.BojHandle, &user.Role, &user.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *UserRepository) GetByEmail(ctx context.Context, email string) (*User, error) {
	var user User
	err := r.pool.QueryRow(ctx,
		`SELECT id, username, email, password_hash, display_name, boj_handle, role, created_at FROM users WHERE email = $1`,
		email,
	).Scan(
		&user.ID, &user.Username, &user.Email, &user.PasswordHash,
		&user.DisplayName, &user.BojHandle, &user.Role, &user.CreatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *UserRepository) GetByUsername(ctx context.Context, username string) (*User, error) {
	var user User
	err := r.pool.QueryRow(ctx,
		`SELECT id, username, email, password_hash, display_name, boj_handle, role, created_at FROM users WHERE username = $1`,
		username,
	).Scan(
		&user.ID, &user.Username, &user.Email, &user.PasswordHash,
		&user.DisplayName, &user.BojHandle, &user.Role, &user.CreatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *UserRepository) GetByID(ctx context.Context, id int64) (*User, error) {
	var user User
	err := r.pool.QueryRow(ctx,
		`SELECT id, username, email, password_hash, display_name, boj_handle, role, created_at FROM users WHERE id = $1`,
		id,
	).Scan(
		&user.ID, &user.Username, &user.Email, &user.PasswordHash,
		&user.DisplayName, &user.BojHandle, &user.Role, &user.CreatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *UserRepository) UpdateProfile(ctx context.Context, id int64, displayName, bojHandle *string) (*User, error) {
	var user User
	err := r.pool.QueryRow(ctx,
		`UPDATE users SET display_name = COALESCE($2, display_name), boj_handle = COALESCE($3, boj_handle) WHERE id = $1 RETURNING id, username, email, password_hash, display_name, boj_handle, role, created_at`,
		id, displayName, bojHandle,
	).Scan(
		&user.ID, &user.Username, &user.Email, &user.PasswordHash,
		&user.DisplayName, &user.BojHandle, &user.Role, &user.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *UserRepository) GetRanking(ctx context.Context, limit int32, offset int32) ([]*RankEntry, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT u.id, u.username,
			COUNT(DISTINCT CASE WHEN s.status = 'ACCEPTED' THEN s.problem_id END) AS solved,
			COUNT(s.id) AS submissions
		FROM users u
		LEFT JOIN submissions s ON u.id = s.user_id
		GROUP BY u.id, u.username
		ORDER BY solved DESC, submissions ASC, u.id ASC
		LIMIT $1 OFFSET $2`,
		limit, offset,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []*RankEntry
	rank := int32(offset) + 1
	for rows.Next() {
		var e RankEntry
		if err := rows.Scan(&e.UserID, &e.Username, &e.SolvedCount, &e.SubmissionCount); err != nil {
			return nil, err
		}
		e.Rank = rank
		rank++
		entries = append(entries, &e)
	}
	return entries, rows.Err()
}
