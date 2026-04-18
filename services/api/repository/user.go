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

type Ban struct {
	ID               int64
	UserID           int64
	Reason           string
	BannedBy         int64
	BannedByUsername string
	BannedAt         time.Time
	UnbannedAt       *time.Time
	UnbannedBy       *int64
}

func (r *UserRepository) BanUser(ctx context.Context, id int64, reason string, bannedBy int64) (*User, *Ban, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, nil, err
	}
	defer tx.Rollback(ctx)

	var ban Ban
	if reason == "" {
		err = tx.QueryRow(ctx,
			`INSERT INTO bans (user_id, banned_by)
			 VALUES ($1, $2)
			 RETURNING id, user_id, reason, banned_by, banned_at, unbanned_at, unbanned_by`,
			id, bannedBy,
		).Scan(&ban.ID, &ban.UserID, &ban.Reason, &ban.BannedBy, &ban.BannedAt, &ban.UnbannedAt, &ban.UnbannedBy)
	} else {
		err = tx.QueryRow(ctx,
			`INSERT INTO bans (user_id, reason, banned_by)
			 VALUES ($1, $2, $3)
			 RETURNING id, user_id, reason, banned_by, banned_at, unbanned_at, unbanned_by`,
			id, reason, bannedBy,
		).Scan(&ban.ID, &ban.UserID, &ban.Reason, &ban.BannedBy, &ban.BannedAt, &ban.UnbannedAt, &ban.UnbannedBy)
	}
	if err != nil {
		return nil, nil, err
	}

	var user User
	err = scanUser(tx.QueryRow(ctx,
		`UPDATE users SET is_banned = true, banned_at = NOW() WHERE id = $1 RETURNING `+userColumns,
		id,
	), &user)
	if err == pgx.ErrNoRows {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, nil, err
	}
	return &user, &ban, nil
}

func (r *UserRepository) UnbanUser(ctx context.Context, id int64, unbannedBy int64) (*User, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx,
		`UPDATE bans SET unbanned_at = NOW(), unbanned_by = $2
		 WHERE user_id = $1 AND unbanned_at IS NULL`,
		id, unbannedBy,
	); err != nil {
		return nil, err
	}

	var user User
	err = scanUser(tx.QueryRow(ctx,
		`UPDATE users SET is_banned = false, banned_at = NULL WHERE id = $1 RETURNING `+userColumns,
		id,
	), &user)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *UserRepository) UpdateBanReason(ctx context.Context, userID int64, reason string) (*Ban, error) {
	var b Ban
	var err error
	if reason == "" {
		err = r.pool.QueryRow(ctx,
			`UPDATE bans SET reason = DEFAULT
			 WHERE user_id = $1 AND unbanned_at IS NULL
			 RETURNING id, user_id, reason, banned_by, banned_at, unbanned_at, unbanned_by`,
			userID,
		).Scan(&b.ID, &b.UserID, &b.Reason, &b.BannedBy, &b.BannedAt, &b.UnbannedAt, &b.UnbannedBy)
	} else {
		err = r.pool.QueryRow(ctx,
			`UPDATE bans SET reason = $2
			 WHERE user_id = $1 AND unbanned_at IS NULL
			 RETURNING id, user_id, reason, banned_by, banned_at, unbanned_at, unbanned_by`,
			userID, reason,
		).Scan(&b.ID, &b.UserID, &b.Reason, &b.BannedBy, &b.BannedAt, &b.UnbannedAt, &b.UnbannedBy)
	}
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &b, nil
}

func (r *UserRepository) SetRole(ctx context.Context, id int64, role string) (*User, error) {
	var user User
	err := scanUser(r.pool.QueryRow(ctx,
		`UPDATE users SET role = $2 WHERE id = $1 RETURNING `+userColumns,
		id, role,
	), &user)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *UserRepository) GetActiveBan(ctx context.Context, userID int64) (*Ban, error) {
	var b Ban
	err := r.pool.QueryRow(ctx,
		`SELECT b.id, b.user_id, b.reason, b.banned_by, COALESCE(u.username, ''), b.banned_at, b.unbanned_at, b.unbanned_by
		 FROM bans b LEFT JOIN users u ON u.id = b.banned_by
		 WHERE b.user_id = $1 AND b.unbanned_at IS NULL`,
		userID,
	).Scan(&b.ID, &b.UserID, &b.Reason, &b.BannedBy, &b.BannedByUsername, &b.BannedAt, &b.UnbannedAt, &b.UnbannedBy)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &b, nil
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
