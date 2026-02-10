package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/nikhilbhutani/backendwithai/internal/tenant"
)

type Claims struct {
	Sub      string `json:"sub"`
	Email    string `json:"email"`
	TenantID string `json:"tenant_id"`
	Role     string `json:"role"`
	jwt.RegisteredClaims
}

type JWTMiddleware struct {
	secret        []byte
	tenantService *tenant.Service
}

func NewJWTMiddleware(secret string, ts *tenant.Service) *JWTMiddleware {
	return &JWTMiddleware{
		secret:        []byte(secret),
		tenantService: ts,
	}
}

func (m *JWTMiddleware) Authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tokenStr := extractBearerToken(r)
		if tokenStr == "" {
			writeError(w, http.StatusUnauthorized, "missing authorization token")
			return
		}

		claims := &Claims{}
		token, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (interface{}, error) {
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
			}
			return m.secret, nil
		})
		if err != nil || !token.Valid {
			writeError(w, http.StatusUnauthorized, "invalid token")
			return
		}

		if claims.ExpiresAt != nil && claims.ExpiresAt.Time.Before(time.Now()) {
			writeError(w, http.StatusUnauthorized, "token expired")
			return
		}

		userID, err := uuid.Parse(claims.Sub)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "invalid user ID in token")
			return
		}

		ctx := r.Context()

		user, err := m.tenantService.GetUserByID(ctx, userID)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "user not found")
			return
		}

		t, err := m.tenantService.GetByID(ctx, user.TenantID)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "tenant not found")
			return
		}

		ctx = tenant.WithTenant(ctx, t)
		ctx = tenant.WithUser(ctx, user)
		ctx = context.WithValue(ctx, claimsKey, claims)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

type ctxKey string

const claimsKey ctxKey = "claims"

func ClaimsFromContext(ctx context.Context) *Claims {
	c, _ := ctx.Value(claimsKey).(*Claims)
	return c
}

func extractBearerToken(r *http.Request) string {
	auth := r.Header.Get("Authorization")
	if strings.HasPrefix(auth, "Bearer ") {
		return strings.TrimPrefix(auth, "Bearer ")
	}
	return ""
}

func writeError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
