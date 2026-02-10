package models

import (
	"time"

	"github.com/google/uuid"
)

type User struct {
	ID        uuid.UUID  `json:"id" db:"id"`
	TenantID  uuid.UUID  `json:"tenant_id" db:"tenant_id"`
	RoleID    *uuid.UUID `json:"role_id,omitempty" db:"role_id"`
	Email     string     `json:"email" db:"email"`
	FullName  string     `json:"full_name,omitempty" db:"full_name"`
	CreatedAt time.Time  `json:"created_at" db:"created_at"`
}

type APIKey struct {
	ID         uuid.UUID  `json:"id" db:"id"`
	TenantID   uuid.UUID  `json:"tenant_id" db:"tenant_id"`
	UserID     *uuid.UUID `json:"user_id,omitempty" db:"user_id"`
	KeyHash    string     `json:"-" db:"key_hash"`
	Name       string     `json:"name" db:"name"`
	Scopes     []string   `json:"scopes" db:"scopes"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty" db:"last_used_at"`
	ExpiresAt  *time.Time `json:"expires_at,omitempty" db:"expires_at"`
	CreatedAt  time.Time  `json:"created_at" db:"created_at"`
}
