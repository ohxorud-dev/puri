package handler

import (
	"context"
	"fmt"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"

	userv1 "github.com/puri-cp/puri/gen/user/v1"
	"github.com/puri-cp/puri/services/api/auth"
	"github.com/puri-cp/puri/services/api/repository"
)

const testSecret = "test-secret-key"

func newUserHandler(repo *MockUserRepo) *UserServiceHandler {
	return NewUserServiceHandler(repo, testSecret, false)
}

func authedCtx(userID int64) context.Context {
	return auth.WithUserID(context.Background(), userID)
}

func testUser() *repository.User {
	return &repository.User{
		ID:       1,
		Username: "testuser",
		Email:    "test@example.com",
	}
}

// --- Register ---

func TestRegister_Success(t *testing.T) {
	repo := new(MockUserRepo)
	h := newUserHandler(repo)

	repo.On("GetByEmail", mock.Anything, "test@example.com").Return(nil, nil)
	repo.On("Create", mock.Anything, "testuser", "test@example.com", mock.AnythingOfType("string")).Return(testUser(), nil)

	resp, err := h.Register(context.Background(), connect.NewRequest(&userv1.RegisterRequest{
		Username: "testuser",
		Email:    "test@example.com",
		Password: "password123",
	}))

	require.NoError(t, err)
	assert.Equal(t, "testuser", resp.Msg.User.Username)
	assert.Equal(t, "test@example.com", resp.Msg.User.Email)
	assert.NotEmpty(t, resp.Header().Get("Set-Cookie"))
	repo.AssertExpectations(t)
}

func TestRegister_DuplicateEmail(t *testing.T) {
	repo := new(MockUserRepo)
	h := newUserHandler(repo)

	repo.On("GetByEmail", mock.Anything, "taken@example.com").Return(testUser(), nil)

	_, err := h.Register(context.Background(), connect.NewRequest(&userv1.RegisterRequest{
		Username: "new",
		Email:    "taken@example.com",
		Password: "password123",
	}))

	require.Error(t, err)
	assert.Equal(t, connect.CodeAlreadyExists, connect.CodeOf(err))
	repo.AssertExpectations(t)
}

func TestRegister_GetByEmailDBError(t *testing.T) {
	repo := new(MockUserRepo)
	h := newUserHandler(repo)

	repo.On("GetByEmail", mock.Anything, "test@example.com").Return(nil, fmt.Errorf("db down"))

	_, err := h.Register(context.Background(), connect.NewRequest(&userv1.RegisterRequest{
		Username: "testuser",
		Email:    "test@example.com",
		Password: "password123",
	}))

	require.Error(t, err)
	assert.Equal(t, connect.CodeInternal, connect.CodeOf(err))
}

func TestRegister_CreateDBError(t *testing.T) {
	repo := new(MockUserRepo)
	h := newUserHandler(repo)

	repo.On("GetByEmail", mock.Anything, "test@example.com").Return(nil, nil)
	repo.On("Create", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, fmt.Errorf("insert failed"))

	_, err := h.Register(context.Background(), connect.NewRequest(&userv1.RegisterRequest{
		Username: "testuser",
		Email:    "test@example.com",
		Password: "password123",
	}))

	require.Error(t, err)
	assert.Equal(t, connect.CodeInternal, connect.CodeOf(err))
}

// --- Login ---

func TestLogin_Success(t *testing.T) {
	repo := new(MockUserRepo)
	h := newUserHandler(repo)

	hash, _ := bcrypt.GenerateFromPassword([]byte("correct-password"), bcrypt.MinCost)
	user := &repository.User{
		ID:           1,
		Username:     "testuser",
		Email:        "test@example.com",
		PasswordHash: string(hash),
	}
	repo.On("GetByUsername", mock.Anything, "testuser").Return(user, nil)

	resp, err := h.Login(context.Background(), connect.NewRequest(&userv1.LoginRequest{
		Username: "testuser",
		Password: "correct-password",
	}))

	require.NoError(t, err)
	assert.Equal(t, "testuser", resp.Msg.User.Username)
	assert.NotEmpty(t, resp.Header().Get("Set-Cookie"))
	repo.AssertExpectations(t)
}

func TestLogin_UserNotFound(t *testing.T) {
	repo := new(MockUserRepo)
	h := newUserHandler(repo)

	repo.On("GetByUsername", mock.Anything, "noone").Return(nil, nil)

	_, err := h.Login(context.Background(), connect.NewRequest(&userv1.LoginRequest{
		Username: "noone",
		Password: "password123",
	}))

	require.Error(t, err)
	assert.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
}

func TestLogin_WrongPassword(t *testing.T) {
	repo := new(MockUserRepo)
	h := newUserHandler(repo)

	hash, _ := bcrypt.GenerateFromPassword([]byte("correct-password"), bcrypt.MinCost)
	user := &repository.User{
		ID:           1,
		Username:     "testuser",
		Email:        "test@example.com",
		PasswordHash: string(hash),
	}
	repo.On("GetByUsername", mock.Anything, "testuser").Return(user, nil)

	_, err := h.Login(context.Background(), connect.NewRequest(&userv1.LoginRequest{
		Username: "testuser",
		Password: "wrong-password",
	}))

	require.Error(t, err)
	assert.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
}

func TestLogin_DBError(t *testing.T) {
	repo := new(MockUserRepo)
	h := newUserHandler(repo)

	repo.On("GetByUsername", mock.Anything, "testuser").Return(nil, fmt.Errorf("db down"))

	_, err := h.Login(context.Background(), connect.NewRequest(&userv1.LoginRequest{
		Username: "testuser",
		Password: "password123",
	}))

	require.Error(t, err)
	assert.Equal(t, connect.CodeInternal, connect.CodeOf(err))
}

// --- Logout ---

func TestLogout(t *testing.T) {
	repo := new(MockUserRepo)
	h := newUserHandler(repo)

	resp, err := h.Logout(context.Background(), connect.NewRequest(&userv1.LogoutRequest{}))

	require.NoError(t, err)
	cookie := resp.Header().Get("Set-Cookie")
	assert.Contains(t, cookie, "Max-Age=0") // Go serializes MaxAge:-1 as Max-Age=0
}

// --- GetProfile ---

func TestGetProfile_Success(t *testing.T) {
	repo := new(MockUserRepo)
	h := newUserHandler(repo)

	repo.On("GetByID", mock.Anything, int64(1)).Return(testUser(), nil)

	resp, err := h.GetProfile(authedCtx(1), connect.NewRequest(&userv1.GetProfileRequest{}))

	require.NoError(t, err)
	assert.Equal(t, "testuser", resp.Msg.User.Username)
	repo.AssertExpectations(t)
}

func TestGetProfile_Unauthenticated(t *testing.T) {
	repo := new(MockUserRepo)
	h := newUserHandler(repo)

	_, err := h.GetProfile(context.Background(), connect.NewRequest(&userv1.GetProfileRequest{}))

	require.Error(t, err)
	assert.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
}

func TestGetProfile_NotFound(t *testing.T) {
	repo := new(MockUserRepo)
	h := newUserHandler(repo)

	repo.On("GetByID", mock.Anything, int64(1)).Return(nil, nil)

	_, err := h.GetProfile(authedCtx(1), connect.NewRequest(&userv1.GetProfileRequest{}))

	require.Error(t, err)
	assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}

func TestGetProfile_DBError(t *testing.T) {
	repo := new(MockUserRepo)
	h := newUserHandler(repo)

	repo.On("GetByID", mock.Anything, int64(1)).Return(nil, fmt.Errorf("db down"))

	_, err := h.GetProfile(authedCtx(1), connect.NewRequest(&userv1.GetProfileRequest{}))

	require.Error(t, err)
	assert.Equal(t, connect.CodeInternal, connect.CodeOf(err))
}

// --- UpdateProfile ---

func TestUpdateProfile_Success(t *testing.T) {
	repo := new(MockUserRepo)
	h := newUserHandler(repo)

	dn := "New Name"
	updated := &repository.User{
		ID:          1,
		Username:    "testuser",
		Email:       "test@example.com",
		DisplayName: &dn,
	}
	repo.On("UpdateProfile", mock.Anything, int64(1), mock.Anything, mock.Anything).Return(updated, nil)

	displayName := "New Name"
	resp, err := h.UpdateProfile(authedCtx(1), connect.NewRequest(&userv1.UpdateProfileRequest{
		DisplayName: &displayName,
	}))

	require.NoError(t, err)
	assert.Equal(t, "New Name", resp.Msg.User.DisplayName)
	repo.AssertExpectations(t)
}

func TestUpdateProfile_Unauthenticated(t *testing.T) {
	repo := new(MockUserRepo)
	h := newUserHandler(repo)

	_, err := h.UpdateProfile(context.Background(), connect.NewRequest(&userv1.UpdateProfileRequest{}))

	require.Error(t, err)
	assert.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
}

func TestUpdateProfile_DBError(t *testing.T) {
	repo := new(MockUserRepo)
	h := newUserHandler(repo)

	repo.On("UpdateProfile", mock.Anything, int64(1), mock.Anything, mock.Anything).Return(nil, fmt.Errorf("db down"))

	_, err := h.UpdateProfile(authedCtx(1), connect.NewRequest(&userv1.UpdateProfileRequest{}))

	require.Error(t, err)
	assert.Equal(t, connect.CodeInternal, connect.CodeOf(err))
}
