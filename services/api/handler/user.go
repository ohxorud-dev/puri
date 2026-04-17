package handler

import (
	"context"
	"fmt"
	"strconv"

	"connectrpc.com/connect"
	"golang.org/x/crypto/bcrypt"
	"google.golang.org/protobuf/types/known/timestamppb"

	userv1 "github.com/puri-cp/puri/gen/user/v1"
	"github.com/puri-cp/puri/services/api/auth"
	"github.com/puri-cp/puri/services/api/repository"
)

type UserServiceHandler struct {
	repo   repository.UserRepo
	secret string
	isProd bool
}

func NewUserServiceHandler(repo repository.UserRepo, secret string, isProd bool) *UserServiceHandler {
	return &UserServiceHandler{repo: repo, secret: secret, isProd: isProd}
}

func (h *UserServiceHandler) Register(ctx context.Context, req *connect.Request[userv1.RegisterRequest]) (*connect.Response[userv1.RegisterResponse], error) {
	existing, err := h.repo.GetByEmail(ctx, req.Msg.Email)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("database error"))
	}
	if existing != nil {
		return nil, connect.NewError(connect.CodeAlreadyExists, fmt.Errorf("email already registered"))
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Msg.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to hash password"))
	}

	user, err := h.repo.Create(ctx, req.Msg.Username, req.Msg.Email, string(hash))
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to create user"))
	}

	cookie, err := auth.MakeAuthCookie(user.ID, h.secret, h.isProd)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to create session"))
	}

	resp := connect.NewResponse(&userv1.RegisterResponse{User: toProtoUser(user)})
	resp.Header().Set("Set-Cookie", cookie.String())
	return resp, nil
}

func (h *UserServiceHandler) Login(ctx context.Context, req *connect.Request[userv1.LoginRequest]) (*connect.Response[userv1.LoginResponse], error) {
	user, err := h.repo.GetByUsername(ctx, req.Msg.Username)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("database error"))
	}
	if user == nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("invalid credentials"))
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Msg.Password)); err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("invalid credentials"))
	}

	cookie, err := auth.MakeAuthCookie(user.ID, h.secret, h.isProd)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to create session"))
	}

	resp := connect.NewResponse(&userv1.LoginResponse{User: toProtoUser(user)})
	resp.Header().Set("Set-Cookie", cookie.String())
	return resp, nil
}

func (h *UserServiceHandler) Logout(ctx context.Context, req *connect.Request[userv1.LogoutRequest]) (*connect.Response[userv1.LogoutResponse], error) {
	resp := connect.NewResponse(&userv1.LogoutResponse{})
	resp.Header().Set("Set-Cookie", auth.MakeClearAuthCookie(h.isProd).String())
	return resp, nil
}

func (h *UserServiceHandler) GetProfile(ctx context.Context, req *connect.Request[userv1.GetProfileRequest]) (*connect.Response[userv1.GetProfileResponse], error) {
	userID, ok := auth.UserIDFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("unauthenticated"))
	}

	user, err := h.repo.GetByID(ctx, userID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("database error"))
	}
	if user == nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("user not found"))
	}

	return connect.NewResponse(&userv1.GetProfileResponse{User: toProtoUser(user)}), nil
}

func (h *UserServiceHandler) UpdateProfile(ctx context.Context, req *connect.Request[userv1.UpdateProfileRequest]) (*connect.Response[userv1.UpdateProfileResponse], error) {
	userID, ok := auth.UserIDFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("unauthenticated"))
	}

	var displayName, bojHandle *string
	if req.Msg.DisplayName != nil {
		dn := *req.Msg.DisplayName
		displayName = &dn
	}
	if req.Msg.BojHandle != nil {
		bh := *req.Msg.BojHandle
		bojHandle = &bh
	}

	user, err := h.repo.UpdateProfile(ctx, userID, displayName, bojHandle)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("database error"))
	}

	return connect.NewResponse(&userv1.UpdateProfileResponse{User: toProtoUser(user)}), nil
}

func (h *UserServiceHandler) GetRanking(ctx context.Context, req *connect.Request[userv1.GetRankingRequest]) (*connect.Response[userv1.GetRankingResponse], error) {
	limit := req.Msg.PageSize
	if limit == 0 {
		limit = 50
	}

	offset := int32(0)
	if req.Msg.PageToken != "" {
		if o, err := strconv.Atoi(req.Msg.PageToken); err == nil && o >= 0 {
			offset = int32(o)
		}
	}

	entries, err := h.repo.GetRanking(ctx, limit, offset)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("database error"))
	}

	var protoEntries []*userv1.RankEntry
	for _, e := range entries {
		protoEntries = append(protoEntries, &userv1.RankEntry{
			Rank:            e.Rank,
			UserId:          e.UserID,
			Username:        e.Username,
			SolvedCount:     e.SolvedCount,
			SubmissionCount: e.SubmissionCount,
		})
	}

	nextPageToken := ""
	if len(entries) == int(limit) {
		nextPageToken = strconv.Itoa(int(offset) + int(limit))
	}

	return connect.NewResponse(&userv1.GetRankingResponse{Entries: protoEntries, NextPageToken: nextPageToken}), nil
}

func toProtoUser(u *repository.User) *userv1.User {
	user := &userv1.User{
		Id:       u.ID,
		Username: u.Username,
		Email:    u.Email,
	}
	if u.CreatedAt != nil {
		user.CreatedAt = timestamppb.New(*u.CreatedAt)
	}
	if u.DisplayName != nil {
		user.DisplayName = *u.DisplayName
	}
	if u.BojHandle != nil {
		user.BojHandle = *u.BojHandle
	}
	return user
}
