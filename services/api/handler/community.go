package handler

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	communityv1 "github.com/ohxorud-dev/puri/gen/go/community/v1"
	"github.com/ohxorud-dev/puri/services/api/auth"
	"github.com/ohxorud-dev/puri/services/api/repository"
)

var categoryMap = map[communityv1.Category]string{
	communityv1.Category_CATEGORY_NOTICE:  "notice",
	communityv1.Category_CATEGORY_GENERAL: "general",
	communityv1.Category_CATEGORY_QNA:     "qna",
	communityv1.Category_CATEGORY_TIPS:    "tips",
}

func categoryToString(c communityv1.Category) string {
	if s, ok := categoryMap[c]; ok {
		return s
	}
	return "general"
}

type CommunityServiceHandler struct {
	repo     repository.CommunityRepo
	userRepo repository.UserRepo
}

func NewCommunityServiceHandler(repo repository.CommunityRepo, userRepo repository.UserRepo) *CommunityServiceHandler {
	return &CommunityServiceHandler{repo: repo, userRepo: userRepo}
}

func (h *CommunityServiceHandler) CreatePost(ctx context.Context, req *connect.Request[communityv1.CreatePostRequest]) (*connect.Response[communityv1.CreatePostResponse], error) {
	userID, ok := auth.UserIDFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("unauthenticated"))
	}

	user, err := h.userRepo.GetByID(ctx, userID)
	if err != nil || user == nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to get user"))
	}

	if categoryToString(req.Msg.Category) == "notice" && user.Role != "admin" {
		return nil, connect.NewError(connect.CodePermissionDenied, fmt.Errorf("only admins can post notices"))
	}

	post, err := h.repo.CreatePost(ctx, categoryToString(req.Msg.Category), req.Msg.Title, req.Msg.Content, userID, user.Username)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to create post"))
	}

	return connect.NewResponse(&communityv1.CreatePostResponse{Post: toProtoPost(post)}), nil
}

func (h *CommunityServiceHandler) GetPost(ctx context.Context, req *connect.Request[communityv1.GetPostRequest]) (*connect.Response[communityv1.GetPostResponse], error) {
	post, err := h.repo.GetPostByID(ctx, req.Msg.PostId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("database error"))
	}
	if post == nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("post not found"))
	}

	if err := h.repo.IncrementViewCount(ctx, req.Msg.PostId); err != nil {
		log.Printf("[WARN] failed to increment view count for post %d: %v", req.Msg.PostId, err)
	}

	return connect.NewResponse(&communityv1.GetPostResponse{Post: toProtoPost(post)}), nil
}

func (h *CommunityServiceHandler) ListPosts(ctx context.Context, req *connect.Request[communityv1.ListPostsRequest]) (*connect.Response[communityv1.ListPostsResponse], error) {
	limit := req.Msg.PageSize
	if limit == 0 {
		limit = 20
	}

	offset := int32(0)
	if req.Msg.PageToken != "" {
		if o, err := strconv.Atoi(req.Msg.PageToken); err == nil && o >= 0 && o <= 10000 {
			offset = int32(o)
		}
	}

	posts, err := h.repo.ListPosts(ctx, categoryToString(req.Msg.Category), limit, offset)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("database error"))
	}

	var protoPosts []*communityv1.Post
	for _, p := range posts {
		protoPosts = append(protoPosts, toProtoPost(p))
	}

	nextPageToken := ""
	if len(posts) == int(limit) {
		nextPageToken = strconv.Itoa(int(offset) + int(limit))
	}

	return connect.NewResponse(&communityv1.ListPostsResponse{Posts: protoPosts, NextPageToken: nextPageToken}), nil
}

func (h *CommunityServiceHandler) UpdatePost(ctx context.Context, req *connect.Request[communityv1.UpdatePostRequest]) (*connect.Response[communityv1.UpdatePostResponse], error) {
	userID, ok := auth.UserIDFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("unauthenticated"))
	}

	existing, err := h.repo.GetPostByID(ctx, req.Msg.PostId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("database error"))
	}
	if existing == nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("post not found"))
	}
	if existing.AuthorID != userID {
		return nil, connect.NewError(connect.CodePermissionDenied, fmt.Errorf("not authorized"))
	}

	post, err := h.repo.UpdatePost(ctx, req.Msg.PostId, req.Msg.Title, req.Msg.Content)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to update post"))
	}

	return connect.NewResponse(&communityv1.UpdatePostResponse{Post: toProtoPost(post)}), nil
}

func (h *CommunityServiceHandler) DeletePost(ctx context.Context, req *connect.Request[communityv1.DeletePostRequest]) (*connect.Response[communityv1.DeletePostResponse], error) {
	userID, ok := auth.UserIDFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("unauthenticated"))
	}

	existing, err := h.repo.GetPostByID(ctx, req.Msg.PostId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("database error"))
	}
	if existing == nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("post not found"))
	}
	if existing.AuthorID != userID {
		return nil, connect.NewError(connect.CodePermissionDenied, fmt.Errorf("not authorized"))
	}

	if err := h.repo.DeletePost(ctx, req.Msg.PostId); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to delete post"))
	}

	return connect.NewResponse(&communityv1.DeletePostResponse{}), nil
}

func (h *CommunityServiceHandler) CreateComment(ctx context.Context, req *connect.Request[communityv1.CreateCommentRequest]) (*connect.Response[communityv1.CreateCommentResponse], error) {
	userID, ok := auth.UserIDFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("unauthenticated"))
	}

	user, err := h.userRepo.GetByID(ctx, userID)
	if err != nil || user == nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to get user"))
	}

	comment, err := h.repo.CreateComment(ctx, req.Msg.PostId, req.Msg.Content, userID, user.Username)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to create comment"))
	}

	return connect.NewResponse(&communityv1.CreateCommentResponse{Comment: toProtoComment(comment)}), nil
}

func (h *CommunityServiceHandler) ListComments(ctx context.Context, req *connect.Request[communityv1.ListCommentsRequest]) (*connect.Response[communityv1.ListCommentsResponse], error) {
	comments, err := h.repo.ListComments(ctx, req.Msg.PostId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("database error"))
	}

	var protoComments []*communityv1.Comment
	for _, c := range comments {
		protoComments = append(protoComments, toProtoComment(c))
	}

	return connect.NewResponse(&communityv1.ListCommentsResponse{Comments: protoComments}), nil
}

func (h *CommunityServiceHandler) DeleteComment(ctx context.Context, req *connect.Request[communityv1.DeleteCommentRequest]) (*connect.Response[communityv1.DeleteCommentResponse], error) {
	userID, ok := auth.UserIDFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("unauthenticated"))
	}

	existing, err := h.repo.GetCommentByID(ctx, req.Msg.CommentId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("database error"))
	}
	if existing == nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("comment not found"))
	}
	if existing.AuthorID != userID {
		return nil, connect.NewError(connect.CodePermissionDenied, fmt.Errorf("not authorized"))
	}

	if err := h.repo.DeleteComment(ctx, req.Msg.CommentId, existing.PostID); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to delete comment"))
	}

	return connect.NewResponse(&communityv1.DeleteCommentResponse{}), nil
}

func toTimestamp(t *time.Time) *timestamppb.Timestamp {
	if t == nil {
		return nil
	}
	return timestamppb.New(*t)
}

func toProtoPost(p *repository.Post) *communityv1.Post {
	post := &communityv1.Post{
		Id:           p.ID,
		Category:     categoryFromString(p.Category),
		Title:        p.Title,
		Content:      p.Content,
		AuthorId:     p.AuthorID,
		AuthorName:   p.AuthorName,
		ViewCount:    p.ViewCount,
		LikeCount:    p.LikeCount,
		CommentCount: p.CommentCount,
		CreatedAt:    toTimestamp(p.CreatedAt),
		UpdatedAt:    toTimestamp(p.UpdatedAt),
	}
	return post
}

func toProtoComment(c *repository.Comment) *communityv1.Comment {
	comment := &communityv1.Comment{
		Id:         c.ID,
		PostId:     c.PostID,
		AuthorId:   c.AuthorID,
		AuthorName: c.AuthorName,
		Content:    c.Content,
		CreatedAt:  toTimestamp(c.CreatedAt),
	}
	return comment
}

func categoryFromString(s string) communityv1.Category {
	switch s {
	case "notice":
		return communityv1.Category_CATEGORY_NOTICE
	case "qna":
		return communityv1.Category_CATEGORY_QNA
	case "tips":
		return communityv1.Category_CATEGORY_TIPS
	default:
		return communityv1.Category_CATEGORY_GENERAL
	}
}
