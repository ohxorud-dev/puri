package handler

import (
	"context"
	"fmt"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	communityv1 "github.com/ohxorud-dev/puri/gen/go/community/v1"
	"github.com/ohxorud-dev/puri/services/api/repository"
)

func newCommunityHandler(repo *MockCommunityRepo, userRepo *MockUserRepo) *CommunityServiceHandler {
	return NewCommunityServiceHandler(repo, userRepo)
}

func testPost(authorID int64) *repository.Post {
	ts := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	return &repository.Post{
		ID:         1,
		Category:   "general",
		Title:      "Test Post",
		Content:    "Test Content",
		AuthorID:   authorID,
		AuthorName: "testuser",
		CreatedAt:  &ts,
	}
}

func testComment(authorID int64) *repository.Comment {
	ts := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	return &repository.Comment{
		ID:         1,
		PostID:     1,
		AuthorID:   authorID,
		AuthorName: "testuser",
		Content:    "Test comment",
		CreatedAt:  &ts,
	}
}

// --- CreatePost ---

func TestCreatePost_Success(t *testing.T) {
	repo := new(MockCommunityRepo)
	userRepo := new(MockUserRepo)
	h := newCommunityHandler(repo, userRepo)

	userRepo.On("GetByID", mock.Anything, int64(1)).Return(testUser(), nil)
	repo.On("CreatePost", mock.Anything, "general", "Title", "Content", int64(1), "testuser").Return(testPost(1), nil)

	resp, err := h.CreatePost(authedCtx(1), connect.NewRequest(&communityv1.CreatePostRequest{
		Category: communityv1.Category_CATEGORY_GENERAL,
		Title:    "Title",
		Content:  "Content",
	}))

	require.NoError(t, err)
	assert.Equal(t, "Test Post", resp.Msg.Post.Title)
	repo.AssertExpectations(t)
	userRepo.AssertExpectations(t)
}

func TestCreatePost_Unauthenticated(t *testing.T) {
	repo := new(MockCommunityRepo)
	userRepo := new(MockUserRepo)
	h := newCommunityHandler(repo, userRepo)

	_, err := h.CreatePost(context.Background(), connect.NewRequest(&communityv1.CreatePostRequest{
		Title:   "Title",
		Content: "Content",
	}))

	require.Error(t, err)
	assert.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
}

func TestCreatePost_UserNotFound(t *testing.T) {
	repo := new(MockCommunityRepo)
	userRepo := new(MockUserRepo)
	h := newCommunityHandler(repo, userRepo)

	userRepo.On("GetByID", mock.Anything, int64(1)).Return(nil, nil)

	_, err := h.CreatePost(authedCtx(1), connect.NewRequest(&communityv1.CreatePostRequest{
		Title:   "Title",
		Content: "Content",
	}))

	require.Error(t, err)
	assert.Equal(t, connect.CodeInternal, connect.CodeOf(err))
}

func TestCreatePost_DBError(t *testing.T) {
	repo := new(MockCommunityRepo)
	userRepo := new(MockUserRepo)
	h := newCommunityHandler(repo, userRepo)

	userRepo.On("GetByID", mock.Anything, int64(1)).Return(testUser(), nil)
	repo.On("CreatePost", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, fmt.Errorf("db error"))

	_, err := h.CreatePost(authedCtx(1), connect.NewRequest(&communityv1.CreatePostRequest{
		Category: communityv1.Category_CATEGORY_GENERAL,
		Title:    "Title",
		Content:  "Content",
	}))

	require.Error(t, err)
	assert.Equal(t, connect.CodeInternal, connect.CodeOf(err))
}

// --- GetPost ---

func TestGetPost_Success(t *testing.T) {
	repo := new(MockCommunityRepo)
	userRepo := new(MockUserRepo)
	h := newCommunityHandler(repo, userRepo)

	repo.On("GetPostByID", mock.Anything, int64(1)).Return(testPost(1), nil)
	repo.On("IncrementViewCount", mock.Anything, int64(1)).Return(nil)

	resp, err := h.GetPost(context.Background(), connect.NewRequest(&communityv1.GetPostRequest{PostId: 1}))

	require.NoError(t, err)
	assert.Equal(t, "Test Post", resp.Msg.Post.Title)
	repo.AssertExpectations(t)
}

func TestGetPost_NotFound(t *testing.T) {
	repo := new(MockCommunityRepo)
	userRepo := new(MockUserRepo)
	h := newCommunityHandler(repo, userRepo)

	repo.On("GetPostByID", mock.Anything, int64(99)).Return(nil, nil)

	_, err := h.GetPost(context.Background(), connect.NewRequest(&communityv1.GetPostRequest{PostId: 99}))

	require.Error(t, err)
	assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}

func TestGetPost_DBError(t *testing.T) {
	repo := new(MockCommunityRepo)
	userRepo := new(MockUserRepo)
	h := newCommunityHandler(repo, userRepo)

	repo.On("GetPostByID", mock.Anything, int64(1)).Return(nil, fmt.Errorf("db error"))

	_, err := h.GetPost(context.Background(), connect.NewRequest(&communityv1.GetPostRequest{PostId: 1}))

	require.Error(t, err)
	assert.Equal(t, connect.CodeInternal, connect.CodeOf(err))
}

func TestGetPost_IncrementViewCountFails(t *testing.T) {
	repo := new(MockCommunityRepo)
	userRepo := new(MockUserRepo)
	h := newCommunityHandler(repo, userRepo)

	repo.On("GetPostByID", mock.Anything, int64(1)).Return(testPost(1), nil)
	repo.On("IncrementViewCount", mock.Anything, int64(1)).Return(fmt.Errorf("view count error"))

	resp, err := h.GetPost(context.Background(), connect.NewRequest(&communityv1.GetPostRequest{PostId: 1}))

	require.NoError(t, err)
	assert.Equal(t, "Test Post", resp.Msg.Post.Title)
}

// --- ListPosts ---

func TestListPosts_Success(t *testing.T) {
	repo := new(MockCommunityRepo)
	userRepo := new(MockUserRepo)
	h := newCommunityHandler(repo, userRepo)

	posts := []*repository.Post{testPost(1)}
	repo.On("ListPosts", mock.Anything, "general", int32(20), int32(0)).Return(posts, nil)

	resp, err := h.ListPosts(context.Background(), connect.NewRequest(&communityv1.ListPostsRequest{
		Category: communityv1.Category_CATEGORY_GENERAL,
	}))

	require.NoError(t, err)
	assert.Len(t, resp.Msg.Posts, 1)
}

func TestListPosts_DefaultPageSize(t *testing.T) {
	repo := new(MockCommunityRepo)
	userRepo := new(MockUserRepo)
	h := newCommunityHandler(repo, userRepo)

	repo.On("ListPosts", mock.Anything, "general", int32(20), int32(0)).Return([]*repository.Post{}, nil)

	_, err := h.ListPosts(context.Background(), connect.NewRequest(&communityv1.ListPostsRequest{
		Category: communityv1.Category_CATEGORY_GENERAL,
		PageSize: 0,
	}))

	require.NoError(t, err)
	repo.AssertCalled(t, "ListPosts", mock.Anything, "general", int32(20), int32(0))
}

func TestListPosts_WithPageToken(t *testing.T) {
	repo := new(MockCommunityRepo)
	userRepo := new(MockUserRepo)
	h := newCommunityHandler(repo, userRepo)

	repo.On("ListPosts", mock.Anything, "general", int32(10), int32(40)).Return([]*repository.Post{}, nil)

	_, err := h.ListPosts(context.Background(), connect.NewRequest(&communityv1.ListPostsRequest{
		Category:  communityv1.Category_CATEGORY_GENERAL,
		PageSize:  10,
		PageToken: "40",
	}))

	require.NoError(t, err)
	repo.AssertCalled(t, "ListPosts", mock.Anything, "general", int32(10), int32(40))
}

func TestListPosts_NextPageToken(t *testing.T) {
	repo := new(MockCommunityRepo)
	userRepo := new(MockUserRepo)
	h := newCommunityHandler(repo, userRepo)

	// Return exactly `limit` posts → next page exists
	posts := make([]*repository.Post, 10)
	for i := range posts {
		posts[i] = testPost(1)
	}
	repo.On("ListPosts", mock.Anything, "general", int32(10), int32(0)).Return(posts, nil)

	resp, err := h.ListPosts(context.Background(), connect.NewRequest(&communityv1.ListPostsRequest{
		Category: communityv1.Category_CATEGORY_GENERAL,
		PageSize: 10,
	}))

	require.NoError(t, err)
	assert.Equal(t, "10", resp.Msg.NextPageToken)
}

func TestListPosts_NoNextPage(t *testing.T) {
	repo := new(MockCommunityRepo)
	userRepo := new(MockUserRepo)
	h := newCommunityHandler(repo, userRepo)

	// Return fewer than limit → no next page
	posts := []*repository.Post{testPost(1)}
	repo.On("ListPosts", mock.Anything, "general", int32(10), int32(0)).Return(posts, nil)

	resp, err := h.ListPosts(context.Background(), connect.NewRequest(&communityv1.ListPostsRequest{
		Category: communityv1.Category_CATEGORY_GENERAL,
		PageSize: 10,
	}))

	require.NoError(t, err)
	assert.Empty(t, resp.Msg.NextPageToken)
}

func TestListPosts_DBError(t *testing.T) {
	repo := new(MockCommunityRepo)
	userRepo := new(MockUserRepo)
	h := newCommunityHandler(repo, userRepo)

	repo.On("ListPosts", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, fmt.Errorf("db error"))

	_, err := h.ListPosts(context.Background(), connect.NewRequest(&communityv1.ListPostsRequest{
		Category: communityv1.Category_CATEGORY_GENERAL,
	}))

	require.Error(t, err)
	assert.Equal(t, connect.CodeInternal, connect.CodeOf(err))
}

// --- UpdatePost ---

func TestUpdatePost_Success(t *testing.T) {
	repo := new(MockCommunityRepo)
	userRepo := new(MockUserRepo)
	h := newCommunityHandler(repo, userRepo)

	repo.On("GetPostByID", mock.Anything, int64(1)).Return(testPost(1), nil)
	updated := testPost(1)
	updated.Title = "Updated"
	repo.On("UpdatePost", mock.Anything, int64(1), "Updated", "New content").Return(updated, nil)

	resp, err := h.UpdatePost(authedCtx(1), connect.NewRequest(&communityv1.UpdatePostRequest{
		PostId:  1,
		Title:   "Updated",
		Content: "New content",
	}))

	require.NoError(t, err)
	assert.Equal(t, "Updated", resp.Msg.Post.Title)
}

func TestUpdatePost_Unauthenticated(t *testing.T) {
	repo := new(MockCommunityRepo)
	userRepo := new(MockUserRepo)
	h := newCommunityHandler(repo, userRepo)

	_, err := h.UpdatePost(context.Background(), connect.NewRequest(&communityv1.UpdatePostRequest{PostId: 1}))

	require.Error(t, err)
	assert.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
}

func TestUpdatePost_NotFound(t *testing.T) {
	repo := new(MockCommunityRepo)
	userRepo := new(MockUserRepo)
	h := newCommunityHandler(repo, userRepo)

	repo.On("GetPostByID", mock.Anything, int64(1)).Return(nil, nil)

	_, err := h.UpdatePost(authedCtx(1), connect.NewRequest(&communityv1.UpdatePostRequest{PostId: 1}))

	require.Error(t, err)
	assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}

func TestUpdatePost_PermissionDenied(t *testing.T) {
	repo := new(MockCommunityRepo)
	userRepo := new(MockUserRepo)
	h := newCommunityHandler(repo, userRepo)

	repo.On("GetPostByID", mock.Anything, int64(1)).Return(testPost(99), nil) // author is 99

	_, err := h.UpdatePost(authedCtx(1), connect.NewRequest(&communityv1.UpdatePostRequest{PostId: 1}))

	require.Error(t, err)
	assert.Equal(t, connect.CodePermissionDenied, connect.CodeOf(err))
}

func TestUpdatePost_DBError(t *testing.T) {
	repo := new(MockCommunityRepo)
	userRepo := new(MockUserRepo)
	h := newCommunityHandler(repo, userRepo)

	repo.On("GetPostByID", mock.Anything, int64(1)).Return(testPost(1), nil)
	repo.On("UpdatePost", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, fmt.Errorf("db error"))

	_, err := h.UpdatePost(authedCtx(1), connect.NewRequest(&communityv1.UpdatePostRequest{
		PostId:  1,
		Title:   "Updated",
		Content: "Content",
	}))

	require.Error(t, err)
	assert.Equal(t, connect.CodeInternal, connect.CodeOf(err))
}

// --- DeletePost ---

func TestDeletePost_Success(t *testing.T) {
	repo := new(MockCommunityRepo)
	userRepo := new(MockUserRepo)
	h := newCommunityHandler(repo, userRepo)

	repo.On("GetPostByID", mock.Anything, int64(1)).Return(testPost(1), nil)
	repo.On("DeletePost", mock.Anything, int64(1)).Return(nil)

	_, err := h.DeletePost(authedCtx(1), connect.NewRequest(&communityv1.DeletePostRequest{PostId: 1}))

	require.NoError(t, err)
	repo.AssertExpectations(t)
}

func TestDeletePost_Unauthenticated(t *testing.T) {
	repo := new(MockCommunityRepo)
	userRepo := new(MockUserRepo)
	h := newCommunityHandler(repo, userRepo)

	_, err := h.DeletePost(context.Background(), connect.NewRequest(&communityv1.DeletePostRequest{PostId: 1}))

	require.Error(t, err)
	assert.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
}

func TestDeletePost_NotFound(t *testing.T) {
	repo := new(MockCommunityRepo)
	userRepo := new(MockUserRepo)
	h := newCommunityHandler(repo, userRepo)

	repo.On("GetPostByID", mock.Anything, int64(1)).Return(nil, nil)

	_, err := h.DeletePost(authedCtx(1), connect.NewRequest(&communityv1.DeletePostRequest{PostId: 1}))

	require.Error(t, err)
	assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}

func TestDeletePost_PermissionDenied(t *testing.T) {
	repo := new(MockCommunityRepo)
	userRepo := new(MockUserRepo)
	h := newCommunityHandler(repo, userRepo)

	repo.On("GetPostByID", mock.Anything, int64(1)).Return(testPost(99), nil)

	_, err := h.DeletePost(authedCtx(1), connect.NewRequest(&communityv1.DeletePostRequest{PostId: 1}))

	require.Error(t, err)
	assert.Equal(t, connect.CodePermissionDenied, connect.CodeOf(err))
}

func TestDeletePost_DBError(t *testing.T) {
	repo := new(MockCommunityRepo)
	userRepo := new(MockUserRepo)
	h := newCommunityHandler(repo, userRepo)

	repo.On("GetPostByID", mock.Anything, int64(1)).Return(testPost(1), nil)
	repo.On("DeletePost", mock.Anything, int64(1)).Return(fmt.Errorf("db error"))

	_, err := h.DeletePost(authedCtx(1), connect.NewRequest(&communityv1.DeletePostRequest{PostId: 1}))

	require.Error(t, err)
	assert.Equal(t, connect.CodeInternal, connect.CodeOf(err))
}

// --- CreateComment ---

func TestCreateComment_Success(t *testing.T) {
	repo := new(MockCommunityRepo)
	userRepo := new(MockUserRepo)
	h := newCommunityHandler(repo, userRepo)

	userRepo.On("GetByID", mock.Anything, int64(1)).Return(testUser(), nil)
	repo.On("CreateComment", mock.Anything, int64(1), "Nice post", int64(1), "testuser").Return(testComment(1), nil)

	resp, err := h.CreateComment(authedCtx(1), connect.NewRequest(&communityv1.CreateCommentRequest{
		PostId:  1,
		Content: "Nice post",
	}))

	require.NoError(t, err)
	assert.Equal(t, "Test comment", resp.Msg.Comment.Content)
	repo.AssertExpectations(t)
}

func TestCreateComment_Unauthenticated(t *testing.T) {
	repo := new(MockCommunityRepo)
	userRepo := new(MockUserRepo)
	h := newCommunityHandler(repo, userRepo)

	_, err := h.CreateComment(context.Background(), connect.NewRequest(&communityv1.CreateCommentRequest{
		PostId:  1,
		Content: "Hello",
	}))

	require.Error(t, err)
	assert.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
}

func TestCreateComment_UserNotFound(t *testing.T) {
	repo := new(MockCommunityRepo)
	userRepo := new(MockUserRepo)
	h := newCommunityHandler(repo, userRepo)

	userRepo.On("GetByID", mock.Anything, int64(1)).Return(nil, nil)

	_, err := h.CreateComment(authedCtx(1), connect.NewRequest(&communityv1.CreateCommentRequest{
		PostId:  1,
		Content: "Hello",
	}))

	require.Error(t, err)
	assert.Equal(t, connect.CodeInternal, connect.CodeOf(err))
}

// --- ListComments ---

func TestListComments_Success(t *testing.T) {
	repo := new(MockCommunityRepo)
	userRepo := new(MockUserRepo)
	h := newCommunityHandler(repo, userRepo)

	comments := []*repository.Comment{testComment(1)}
	repo.On("ListComments", mock.Anything, int64(1)).Return(comments, nil)

	resp, err := h.ListComments(context.Background(), connect.NewRequest(&communityv1.ListCommentsRequest{PostId: 1}))

	require.NoError(t, err)
	assert.Len(t, resp.Msg.Comments, 1)
}

func TestListComments_DBError(t *testing.T) {
	repo := new(MockCommunityRepo)
	userRepo := new(MockUserRepo)
	h := newCommunityHandler(repo, userRepo)

	repo.On("ListComments", mock.Anything, int64(1)).Return(nil, fmt.Errorf("db error"))

	_, err := h.ListComments(context.Background(), connect.NewRequest(&communityv1.ListCommentsRequest{PostId: 1}))

	require.Error(t, err)
	assert.Equal(t, connect.CodeInternal, connect.CodeOf(err))
}

// --- DeleteComment ---

func TestDeleteComment_Success(t *testing.T) {
	repo := new(MockCommunityRepo)
	userRepo := new(MockUserRepo)
	h := newCommunityHandler(repo, userRepo)

	repo.On("GetCommentByID", mock.Anything, int64(1)).Return(testComment(1), nil)
	repo.On("DeleteComment", mock.Anything, int64(1), int64(1)).Return(nil)

	_, err := h.DeleteComment(authedCtx(1), connect.NewRequest(&communityv1.DeleteCommentRequest{CommentId: 1}))

	require.NoError(t, err)
	repo.AssertExpectations(t)
}

func TestDeleteComment_Unauthenticated(t *testing.T) {
	repo := new(MockCommunityRepo)
	userRepo := new(MockUserRepo)
	h := newCommunityHandler(repo, userRepo)

	_, err := h.DeleteComment(context.Background(), connect.NewRequest(&communityv1.DeleteCommentRequest{CommentId: 1}))

	require.Error(t, err)
	assert.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
}

func TestDeleteComment_NotFound(t *testing.T) {
	repo := new(MockCommunityRepo)
	userRepo := new(MockUserRepo)
	h := newCommunityHandler(repo, userRepo)

	repo.On("GetCommentByID", mock.Anything, int64(1)).Return(nil, nil)

	_, err := h.DeleteComment(authedCtx(1), connect.NewRequest(&communityv1.DeleteCommentRequest{CommentId: 1}))

	require.Error(t, err)
	assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}

func TestDeleteComment_PermissionDenied(t *testing.T) {
	repo := new(MockCommunityRepo)
	userRepo := new(MockUserRepo)
	h := newCommunityHandler(repo, userRepo)

	repo.On("GetCommentByID", mock.Anything, int64(1)).Return(testComment(99), nil) // author is 99

	_, err := h.DeleteComment(authedCtx(1), connect.NewRequest(&communityv1.DeleteCommentRequest{CommentId: 1}))

	require.Error(t, err)
	assert.Equal(t, connect.CodePermissionDenied, connect.CodeOf(err))
}

func TestDeleteComment_DBError(t *testing.T) {
	repo := new(MockCommunityRepo)
	userRepo := new(MockUserRepo)
	h := newCommunityHandler(repo, userRepo)

	repo.On("GetCommentByID", mock.Anything, int64(1)).Return(testComment(1), nil)
	repo.On("DeleteComment", mock.Anything, int64(1), int64(1)).Return(fmt.Errorf("db error"))

	_, err := h.DeleteComment(authedCtx(1), connect.NewRequest(&communityv1.DeleteCommentRequest{CommentId: 1}))

	require.Error(t, err)
	assert.Equal(t, connect.CodeInternal, connect.CodeOf(err))
}
