package auth

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const CookieName = "puri_session"

type contextKey string

const userIDKey contextKey = "user_id"

type Claims struct {
	UserID int64 `json:"user_id"`
	jwt.RegisteredClaims
}

func MakeAuthCookie(userID int64, secret string, isProd bool) (*http.Cookie, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, Claims{
		UserID: userID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(7 * 24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	})

	tokenString, err := token.SignedString([]byte(secret))
	if err != nil {
		return nil, fmt.Errorf("failed to sign token: %w", err)
	}

	cookie := &http.Cookie{
		Name:     CookieName,
		Value:    tokenString,
		Path:     "/",
		HttpOnly: true,
		MaxAge:   int((7 * 24 * time.Hour).Seconds()),
	}
	cookie.SameSite = http.SameSiteLaxMode
	if isProd {
		cookie.Domain = "puri.ac"
		cookie.Secure = true
	}
	return cookie, nil
}

func MakeClearAuthCookie(isProd bool) *http.Cookie {
	cookie := &http.Cookie{
		Name:     CookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		MaxAge:   -1,
	}
	if isProd {
		cookie.Domain = ".puri.ac"
		cookie.Secure = true
	}
	cookie.SameSite = http.SameSiteLaxMode
	return cookie
}

func VerifyTokenFromHeader(header http.Header, secret string) (int64, error) {
	cookies := header.Values("Cookie")
	for _, c := range cookies {
		parsed, err := http.ParseCookie(c)
		if err != nil {
			continue
		}
		for _, cookie := range parsed {
			if cookie.Name == CookieName {
				return verifyTokenString(cookie.Value, secret)
			}
		}
	}
	return 0, fmt.Errorf("auth cookie not found")
}

func verifyTokenString(tokenString string, secret string) (int64, error) {
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(secret), nil
	})
	if err != nil || !token.Valid {
		return 0, fmt.Errorf("invalid token")
	}
	return claims.UserID, nil
}

func WithUserID(ctx context.Context, userID int64) context.Context {
	return context.WithValue(ctx, userIDKey, userID)
}

func UserIDFromContext(ctx context.Context) (int64, bool) {
	userID, ok := ctx.Value(userIDKey).(int64)
	return userID, ok
}
