package auth

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nikhilbhutani/backendwithai/internal/models"
	"github.com/nikhilbhutani/backendwithai/internal/tenant"
)

type APIKeyMiddleware struct {
	db            *pgxpool.Pool
	headerName    string
	tenantService *tenant.Service
}

func NewAPIKeyMiddleware(db *pgxpool.Pool, headerName string, ts *tenant.Service) *APIKeyMiddleware {
	return &APIKeyMiddleware{
		db:            db,
		headerName:    headerName,
		tenantService: ts,
	}
}

func (m *APIKeyMiddleware) Authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := r.Header.Get(m.headerName)
		if key == "" {
			next.ServeHTTP(w, r)
			return
		}

		hash := HashAPIKey(key)

		var ak models.APIKey
		var scopesJSON json.RawMessage
		err := m.db.QueryRow(r.Context(),
			`SELECT id, tenant_id, user_id, key_hash, name, scopes, expires_at, created_at
			 FROM api_keys WHERE key_hash = $1`, hash,
		).Scan(&ak.ID, &ak.TenantID, &ak.UserID, &ak.KeyHash, &ak.Name, &scopesJSON, &ak.ExpiresAt, &ak.CreatedAt)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "invalid API key")
			return
		}

		if err := json.Unmarshal(scopesJSON, &ak.Scopes); err != nil {
			writeError(w, http.StatusInternalServerError, "invalid scopes")
			return
		}

		if ak.ExpiresAt != nil && ak.ExpiresAt.Before(time.Now()) {
			writeError(w, http.StatusUnauthorized, "API key expired")
			return
		}

		// Constant-time comparison already done via hash lookup
		if subtle.ConstantTimeCompare([]byte(ak.KeyHash), []byte(hash)) != 1 {
			writeError(w, http.StatusUnauthorized, "invalid API key")
			return
		}

		// Update last used
		go func() {
			m.db.Exec(r.Context(), "UPDATE api_keys SET last_used_at = $1 WHERE id = $2", time.Now(), ak.ID)
		}()

		t, err := m.tenantService.GetByID(r.Context(), ak.TenantID)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "tenant not found")
			return
		}

		ctx := tenant.WithTenant(r.Context(), t)

		if ak.UserID != nil {
			user, err := m.tenantService.GetUserByID(r.Context(), *ak.UserID)
			if err == nil {
				ctx = tenant.WithUser(ctx, user)
			}
		}

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func HashAPIKey(key string) string {
	h := sha256.Sum256([]byte(key))
	return hex.EncodeToString(h[:])
}

func GenerateAPIKeyPrefix() string {
	return fmt.Sprintf("bai_%d", time.Now().UnixNano())
}
