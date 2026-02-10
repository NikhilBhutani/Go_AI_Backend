package auth

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nikhilbhutani/backendwithai/internal/tenant"
)

type Permission string

const (
	PermDocumentsRead   Permission = "documents:read"
	PermDocumentsWrite  Permission = "documents:write"
	PermPromptsRead     Permission = "prompts:read"
	PermPromptsWrite    Permission = "prompts:write"
	PermFinetuneManage  Permission = "finetune:manage"
	PermLLMChat         Permission = "llm:chat"
	PermLLMEmbed        Permission = "llm:embed"
	PermWebhooksManage  Permission = "webhooks:manage"
	PermAdminRead       Permission = "admin:read"
	PermAdminWrite      Permission = "admin:write"
	PermWildcard        Permission = "*"
)

type RBAC struct {
	db *pgxpool.Pool
}

func NewRBAC(db *pgxpool.Pool) *RBAC {
	return &RBAC{db: db}
}

func (r *RBAC) RequirePermission(perm Permission) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			user := tenant.UserFromContext(req.Context())
			if user == nil {
				writeError(w, http.StatusForbidden, "no user in context")
				return
			}

			if user.RoleID == nil {
				writeError(w, http.StatusForbidden, "no role assigned")
				return
			}

			has, err := r.userHasPermission(req.Context(), user.RoleID.String(), perm)
			if err != nil {
				writeError(w, http.StatusInternalServerError, "permission check failed")
				return
			}
			if !has {
				writeError(w, http.StatusForbidden, "insufficient permissions")
				return
			}

			next.ServeHTTP(w, req)
		})
	}
}

func (r *RBAC) userHasPermission(ctx context.Context, roleID string, perm Permission) (bool, error) {
	var permJSON json.RawMessage
	err := r.db.QueryRow(ctx,
		"SELECT permissions FROM roles WHERE id = $1", roleID,
	).Scan(&permJSON)
	if err != nil {
		return false, err
	}

	var perms []string
	if err := json.Unmarshal(permJSON, &perms); err != nil {
		return false, err
	}

	for _, p := range perms {
		if Permission(p) == PermWildcard || Permission(p) == perm {
			return true, nil
		}
	}
	return false, nil
}
