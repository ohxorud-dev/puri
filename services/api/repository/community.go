package repository

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Post struct {
	ID           int64
	Category     string
	Title        string
	Content      string
	AuthorID     int64
	AuthorName   string
	CreatedAt    *time.Time
	UpdatedAt    *time.Time
	ViewCount    int32
	LikeCount    int32
	CommentCount int32
}

type Comment struct {
	ID         int64
	PostID     int64
	AuthorID   int64
	AuthorName string
	Content    string
	CreatedAt  *time.Time
}

type CommunityRepository struct {
	pool *pgxpool.Pool
}

func NewCommunityRepository(pool *pgxpool.Pool) *CommunityRepository {
	return &CommunityRepository{pool: pool}
}

func (r *CommunityRepository) CreatePost(ctx context.Context, category, title, content string, authorID int64, authorName string) (*Post, error) {
	var post Post
	err := r.pool.QueryRow(ctx,
		`INSERT INTO posts (category, title, content, author_id, author_name) VALUES ($1, $2, $3, $4, $5) RETURNING id, category, title, content, author_id, author_name, created_at, updated_at, view_count, like_count, comment_count`,
		category, title, content, authorID, authorName,
	).Scan(
		&post.ID, &post.Category, &post.Title, &post.Content,
		&post.AuthorID, &post.AuthorName, &post.CreatedAt, &post.UpdatedAt,
		&post.ViewCount, &post.LikeCount, &post.CommentCount,
	)
	if err != nil {
		return nil, err
	}
	return &post, nil
}

func (r *CommunityRepository) GetPostByID(ctx context.Context, id int64) (*Post, error) {
	var post Post
	err := r.pool.QueryRow(ctx,
		`SELECT id, category, title, content, author_id, author_name, created_at, updated_at, view_count, like_count, comment_count FROM posts WHERE id = $1`,
		id,
	).Scan(
		&post.ID, &post.Category, &post.Title, &post.Content,
		&post.AuthorID, &post.AuthorName, &post.CreatedAt, &post.UpdatedAt,
		&post.ViewCount, &post.LikeCount, &post.CommentCount,
	)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &post, nil
}

func (r *CommunityRepository) IncrementViewCount(ctx context.Context, id int64) error {
	_, err := r.pool.Exec(ctx, `UPDATE posts SET view_count = view_count + 1 WHERE id = $1`, id)
	return err
}

func (r *CommunityRepository) ListPosts(ctx context.Context, category string, limit int32, offset int32) ([]*Post, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, category, title, content, author_id, author_name, created_at, updated_at, view_count, like_count, comment_count FROM posts WHERE category = $1 ORDER BY created_at DESC LIMIT $2 OFFSET $3`,
		category, limit, offset,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var posts []*Post
	for rows.Next() {
		var post Post
		if err := rows.Scan(
			&post.ID, &post.Category, &post.Title, &post.Content,
			&post.AuthorID, &post.AuthorName, &post.CreatedAt, &post.UpdatedAt,
			&post.ViewCount, &post.LikeCount, &post.CommentCount,
		); err != nil {
			return nil, err
		}
		posts = append(posts, &post)
	}
	return posts, rows.Err()
}

func (r *CommunityRepository) UpdatePost(ctx context.Context, id int64, title, content string) (*Post, error) {
	var post Post
	err := r.pool.QueryRow(ctx,
		`UPDATE posts SET title = $2, content = $3, updated_at = NOW() WHERE id = $1 RETURNING id, category, title, content, author_id, author_name, created_at, updated_at, view_count, like_count, comment_count`,
		id, title, content,
	).Scan(
		&post.ID, &post.Category, &post.Title, &post.Content,
		&post.AuthorID, &post.AuthorName, &post.CreatedAt, &post.UpdatedAt,
		&post.ViewCount, &post.LikeCount, &post.CommentCount,
	)
	if err != nil {
		return nil, err
	}
	return &post, nil
}

func (r *CommunityRepository) DeletePost(ctx context.Context, id int64) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM posts WHERE id = $1`, id)
	return err
}

func (r *CommunityRepository) CreateComment(ctx context.Context, postID int64, content string, authorID int64, authorName string) (*Comment, error) {
	var comment Comment
	err := r.pool.QueryRow(ctx,
		`INSERT INTO comments (post_id, author_id, author_name, content) VALUES ($1, $2, $3, $4) RETURNING id, post_id, author_id, author_name, content, created_at`,
		postID, authorID, authorName, content,
	).Scan(
		&comment.ID, &comment.PostID, &comment.AuthorID,
		&comment.AuthorName, &comment.Content, &comment.CreatedAt,
	)
	if err != nil {
		return nil, err
	}

	_, _ = r.pool.Exec(ctx, `UPDATE posts SET comment_count = comment_count + 1 WHERE id = $1`, postID)
	return &comment, nil
}

func (r *CommunityRepository) ListComments(ctx context.Context, postID int64) ([]*Comment, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, post_id, author_id, author_name, content, created_at FROM comments WHERE post_id = $1 ORDER BY created_at ASC`,
		postID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var comments []*Comment
	for rows.Next() {
		var comment Comment
		if err := rows.Scan(
			&comment.ID, &comment.PostID, &comment.AuthorID,
			&comment.AuthorName, &comment.Content, &comment.CreatedAt,
		); err != nil {
			return nil, err
		}
		comments = append(comments, &comment)
	}
	return comments, rows.Err()
}

func (r *CommunityRepository) GetCommentByID(ctx context.Context, commentID int64) (*Comment, error) {
	var comment Comment
	err := r.pool.QueryRow(ctx,
		`SELECT id, post_id, author_id, author_name, content, created_at FROM comments WHERE id = $1`,
		commentID,
	).Scan(
		&comment.ID, &comment.PostID, &comment.AuthorID,
		&comment.AuthorName, &comment.Content, &comment.CreatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &comment, nil
}

func (r *CommunityRepository) DeleteComment(ctx context.Context, commentID int64, postID int64) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM comments WHERE id = $1`, commentID)
	if err != nil {
		return err
	}
	_, _ = r.pool.Exec(ctx, `UPDATE posts SET comment_count = comment_count - 1 WHERE id = $1 AND comment_count > 0`, postID)
	return nil
}
