package tenant

import (
	"context"

	"github.com/google/uuid"
	"github.com/nikhilbhutani/backendwithai/internal/models"
)

type contextKey string

const (
	tenantKey contextKey = "tenant"
	userKey   contextKey = "user"
)

func WithTenant(ctx context.Context, t *models.Tenant) context.Context {
	return context.WithValue(ctx, tenantKey, t)
}

func FromContext(ctx context.Context) *models.Tenant {
	t, _ := ctx.Value(tenantKey).(*models.Tenant)
	return t
}

func IDFromContext(ctx context.Context) uuid.UUID {
	if t := FromContext(ctx); t != nil {
		return t.ID
	}
	return uuid.Nil
}

func WithUser(ctx context.Context, u *models.User) context.Context {
	return context.WithValue(ctx, userKey, u)
}

func UserFromContext(ctx context.Context) *models.User {
	u, _ := ctx.Value(userKey).(*models.User)
	return u
}
