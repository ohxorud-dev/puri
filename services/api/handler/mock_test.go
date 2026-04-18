package handler

import (
	"context"

	"github.com/ohxorud-dev/puri/services/api/repository"
	"github.com/stretchr/testify/mock"
)

// MockUserRepo implements repository.UserRepo for testing.
type MockUserRepo struct {
	mock.Mock
}

func (m *MockUserRepo) Create(ctx context.Context, username, email, passwordHash string) (*repository.User, error) {
	args := m.Called(ctx, username, email, passwordHash)
	return userOrNil(args, 0), args.Error(1)
}

func (m *MockUserRepo) GetByEmail(ctx context.Context, email string) (*repository.User, error) {
	args := m.Called(ctx, email)
	return userOrNil(args, 0), args.Error(1)
}

func (m *MockUserRepo) GetByUsername(ctx context.Context, username string) (*repository.User, error) {
	args := m.Called(ctx, username)
	return userOrNil(args, 0), args.Error(1)
}

func (m *MockUserRepo) GetByID(ctx context.Context, id int64) (*repository.User, error) {
	args := m.Called(ctx, id)
	return userOrNil(args, 0), args.Error(1)
}

func (m *MockUserRepo) UpdateProfile(ctx context.Context, id int64, displayName, bojHandle *string) (*repository.User, error) {
	args := m.Called(ctx, id, displayName, bojHandle)
	return userOrNil(args, 0), args.Error(1)
}

func (m *MockUserRepo) GetRanking(ctx context.Context, limit int32, offset int32) ([]*repository.RankEntry, error) {
	args := m.Called(ctx, limit, offset)
	v := args.Get(0)
	if v == nil {
		return nil, args.Error(1)
	}
	return v.([]*repository.RankEntry), args.Error(1)
}

// MockCommunityRepo implements repository.CommunityRepo for testing.
type MockCommunityRepo struct {
	mock.Mock
}

func (m *MockCommunityRepo) CreatePost(ctx context.Context, category, title, content string, authorID int64, authorName string) (*repository.Post, error) {
	args := m.Called(ctx, category, title, content, authorID, authorName)
	return postOrNil(args, 0), args.Error(1)
}

func (m *MockCommunityRepo) GetPostByID(ctx context.Context, id int64) (*repository.Post, error) {
	args := m.Called(ctx, id)
	return postOrNil(args, 0), args.Error(1)
}

func (m *MockCommunityRepo) IncrementViewCount(ctx context.Context, id int64) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockCommunityRepo) ListPosts(ctx context.Context, category string, limit int32, offset int32) ([]*repository.Post, error) {
	args := m.Called(ctx, category, limit, offset)
	return postsOrNil(args, 0), args.Error(1)
}

func (m *MockCommunityRepo) UpdatePost(ctx context.Context, id int64, title, content string) (*repository.Post, error) {
	args := m.Called(ctx, id, title, content)
	return postOrNil(args, 0), args.Error(1)
}

func (m *MockCommunityRepo) DeletePost(ctx context.Context, id int64) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockCommunityRepo) CreateComment(ctx context.Context, postID int64, content string, authorID int64, authorName string) (*repository.Comment, error) {
	args := m.Called(ctx, postID, content, authorID, authorName)
	return commentOrNil(args, 0), args.Error(1)
}

func (m *MockCommunityRepo) ListComments(ctx context.Context, postID int64) ([]*repository.Comment, error) {
	args := m.Called(ctx, postID)
	return commentsOrNil(args, 0), args.Error(1)
}

func (m *MockCommunityRepo) GetCommentByID(ctx context.Context, commentID int64) (*repository.Comment, error) {
	args := m.Called(ctx, commentID)
	return commentOrNil(args, 0), args.Error(1)
}

func (m *MockCommunityRepo) DeleteComment(ctx context.Context, commentID int64, postID int64) error {
	args := m.Called(ctx, commentID, postID)
	return args.Error(0)
}

// Nil-safe helpers for type assertions on mock return values.

func userOrNil(args mock.Arguments, index int) *repository.User {
	v := args.Get(index)
	if v == nil {
		return nil
	}
	return v.(*repository.User)
}

func postOrNil(args mock.Arguments, index int) *repository.Post {
	v := args.Get(index)
	if v == nil {
		return nil
	}
	return v.(*repository.Post)
}

func postsOrNil(args mock.Arguments, index int) []*repository.Post {
	v := args.Get(index)
	if v == nil {
		return nil
	}
	return v.([]*repository.Post)
}

func commentOrNil(args mock.Arguments, index int) *repository.Comment {
	v := args.Get(index)
	if v == nil {
		return nil
	}
	return v.(*repository.Comment)
}

func commentsOrNil(args mock.Arguments, index int) []*repository.Comment {
	v := args.Get(index)
	if v == nil {
		return nil
	}
	return v.([]*repository.Comment)
}
