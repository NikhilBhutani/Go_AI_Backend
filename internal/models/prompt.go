package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type Prompt struct {
	ID             uuid.UUID  `json:"id" db:"id"`
	TenantID       *uuid.UUID `json:"tenant_id,omitempty" db:"tenant_id"`
	Name           string     `json:"name" db:"name"`
	Description    string     `json:"description,omitempty" db:"description"`
	CurrentVersion int        `json:"current_version" db:"current_version"`
	CreatedAt      time.Time  `json:"created_at" db:"created_at"`
}

type PromptVersion struct {
	ID             uuid.UUID       `json:"id" db:"id"`
	PromptID       uuid.UUID       `json:"prompt_id" db:"prompt_id"`
	Version        int             `json:"version" db:"version"`
	SystemTemplate string          `json:"system_template,omitempty" db:"system_template"`
	UserTemplate   string          `json:"user_template,omitempty" db:"user_template"`
	Variables      json.RawMessage `json:"variables" db:"variables"`
	CreatedAt      time.Time       `json:"created_at" db:"created_at"`
}
