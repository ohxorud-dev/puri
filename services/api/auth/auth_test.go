package auth

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testSecret = "test-secret-key-for-unit-tests"

func TestMakeAuthCookie(t *testing.T) {
	cookie, err := MakeAuthCookie(42, testSecret, false)
	require.NoError(t, err)
	require.NotNil(t, cookie)

	assert.Equal(t, CookieName, cookie.Name)
	assert.Equal(t, "/", cookie.Path)
	assert.True(t, cookie.HttpOnly)
	assert.Equal(t, http.SameSiteLaxMode, cookie.SameSite)
	assert.Equal(t, int((7 * 24 * time.Hour).Seconds()), cookie.MaxAge)
	assert.NotEmpty(t, cookie.Value)
}

func TestMakeAuthCookie_RoundTrip(t *testing.T) {
	userID := int64(123)
	cookie, err := MakeAuthCookie(userID, testSecret, false)
	require.NoError(t, err)

	header := http.Header{}
	header.Set("Cookie", cookie.Name+"="+cookie.Value)

	got, err := VerifyTokenFromHeader(header, testSecret)
	require.NoError(t, err)
	assert.Equal(t, userID, got)
}

func TestMakeClearAuthCookie(t *testing.T) {
	cookie := MakeClearAuthCookie(false)
	assert.Equal(t, CookieName, cookie.Name)
	assert.Empty(t, cookie.Value)
	assert.Equal(t, -1, cookie.MaxAge)
}

func TestVerifyTokenFromHeader_Valid(t *testing.T) {
	cookie, err := MakeAuthCookie(99, testSecret, false)
	require.NoError(t, err)

	header := http.Header{}
	header.Set("Cookie", cookie.Name+"="+cookie.Value)

	userID, err := VerifyTokenFromHeader(header, testSecret)
	require.NoError(t, err)
	assert.Equal(t, int64(99), userID)
}

func TestVerifyTokenFromHeader_NoCookie(t *testing.T) {
	header := http.Header{}
	_, err := VerifyTokenFromHeader(header, testSecret)
	assert.Error(t, err)
}

func TestVerifyTokenFromHeader_InvalidToken(t *testing.T) {
	header := http.Header{}
	header.Set("Cookie", CookieName+"=garbage-token-value")

	_, err := VerifyTokenFromHeader(header, testSecret)
	assert.Error(t, err)
}

func TestVerifyTokenFromHeader_WrongSecret(t *testing.T) {
	cookie, err := MakeAuthCookie(42, "secret-A", false)
	require.NoError(t, err)

	header := http.Header{}
	header.Set("Cookie", cookie.String())

	_, err = VerifyTokenFromHeader(header, "secret-B")
	assert.Error(t, err)
}

func TestVerifyTokenFromHeader_Expired(t *testing.T) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, Claims{
		UserID: 42,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(-1 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now().Add(-2 * time.Hour)),
		},
	})
	tokenString, err := token.SignedString([]byte(testSecret))
	require.NoError(t, err)

	header := http.Header{}
	header.Set("Cookie", CookieName+"="+tokenString)

	_, err = VerifyTokenFromHeader(header, testSecret)
	assert.Error(t, err)
}

func TestUserIDContext_RoundTrip(t *testing.T) {
	ctx := WithUserID(context.Background(), 55)
	userID, ok := UserIDFromContext(ctx)
	assert.True(t, ok)
	assert.Equal(t, int64(55), userID)
}

func TestUserIDFromContext_Missing(t *testing.T) {
	userID, ok := UserIDFromContext(context.Background())
	assert.False(t, ok)
	assert.Equal(t, int64(0), userID)
}
