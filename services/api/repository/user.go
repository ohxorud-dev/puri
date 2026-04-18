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
	IsBanned     bool
	BannedAt     *time.Time
	CreatedAt    *time.Time
}

const userColumns = "id, username, email, password_hash, display_name, boj_handle, role, is_banned, banned_at, created_at"

type UserRepository struct {
	pool *pgxpool.Pool
}

func NewUserRepository(pool *pgxpool.Pool) *UserRepository {
	return &UserRepository{pool: pool}
}

func scanUser(row pgx.Row, user *User) error {
	return row.Scan(
		&user.ID, &user.Username, &user.Email, &user.PasswordHash,
		&user.DisplayName, &user.BojHandle, &user.Role,
		&user.IsBanned, &user.BannedAt, &user.CreatedAt,
	)
}

func (r *UserRepository) Create(ctx context.Context, username, email, passwordHash string) (*User, error) {
	var user User
	err := scanUser(r.pool.QueryRow(ctx,
		`INSERT INTO users (username, email, password_hash) VALUES ($1, $2, $3) RETURNING `+userColumns,
		username, email, passwordHash,
	), &user)
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *UserRepository) GetByEmail(ctx context.Context, email string) (*User, error) {
	var user User
	err := scanUser(r.pool.QueryRow(ctx,
		`SELECT `+userColumns+` FROM users WHERE email = $1`,
		email,
	), &user)
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
	err := scanUser(r.pool.QueryRow(ctx,
		`SELECT `+userColumns+` FROM users WHERE username = $1`,
		username,
	), &user)
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
	err := scanUser(r.pool.QueryRow(ctx,
		`SELECT `+userColumns+` FROM users WHERE id = $1`,
		id,
	), &user)
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
	err := scanUser(r.pool.QueryRow(ctx,
		`UPDATE users SET display_name = COALESCE($2, display_name), boj_handle = COALESCE($3, boj_handle) WHERE id = $1 RETURNING `+userColumns,
		id, displayName, bojHandle,
	), &user)
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *UserRepository) SetBanned(ctx context.Context, id int64, banned bool) (*User, error) {
	var user User
	err := scanUser(r.pool.QueryRow(ctx,
		`UPDATE users
		 SET is_banned = $2,
		     banned_at = CASE WHEN $2 THEN NOW() ELSE NULL END
		 WHERE id = $1
		 RETURNING `+userColumns,
		id, banned,
	), &user)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *UserRepository) IsBanned(ctx context.Context, id int64) (bool, error) {
	var banned bool
	err := r.pool.QueryRow(ctx, `SELECT is_banned FROM users WHERE id = $1`, id).Scan(&banned)
	if err == pgx.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return banned, nil
}

func (r *UserRepository) ListUsers(ctx context.Context, limit int32, offset int32) ([]*User, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT `+userColumns+` FROM users ORDER BY id ASC LIMIT $1 OFFSET $2`,
		limit, offset,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []*User
	for rows.Next() {
		var u User
		if err := scanUser(rows, &u); err != nil {
			return nil, err
		}
		users = append(users, &u)
	}
	return users, rows.Err()
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
