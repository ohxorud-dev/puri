package repository

import "context"

type RankEntry struct {
	Rank            int32
	UserID          int64
	Username        string
	SolvedCount     int32
	SubmissionCount int32
}

type UserRepo interface {
	Create(ctx context.Context, username, email, passwordHash string) (*User, error)
	GetByEmail(ctx context.Context, email string) (*User, error)
	GetByUsername(ctx context.Context, username string) (*User, error)
	GetByID(ctx context.Context, id int64) (*User, error)
	UpdateProfile(ctx context.Context, id int64, displayName, bojHandle *string) (*User, error)
	SetBanned(ctx context.Context, id int64, banned bool) (*User, error)
	IsBanned(ctx context.Context, id int64) (bool, error)
	ListUsers(ctx context.Context, limit int32, offset int32) ([]*User, error)
	GetRanking(ctx context.Context, limit int32, offset int32) ([]*RankEntry, error)
}

type CommunityRepo interface {
	CreatePost(ctx context.Context, category, title, content string, authorID int64, authorName string) (*Post, error)
	GetPostByID(ctx context.Context, id int64) (*Post, error)
	IncrementViewCount(ctx context.Context, id int64) error
	ListPosts(ctx context.Context, category string, limit int32, offset int32) ([]*Post, error)
	UpdatePost(ctx context.Context, id int64, title, content string) (*Post, error)
	DeletePost(ctx context.Context, id int64) error
	CreateComment(ctx context.Context, postID int64, content string, authorID int64, authorName string) (*Comment, error)
	ListComments(ctx context.Context, postID int64) ([]*Comment, error)
	GetCommentByID(ctx context.Context, commentID int64) (*Comment, error)
	DeleteComment(ctx context.Context, commentID int64, postID int64) error
}
