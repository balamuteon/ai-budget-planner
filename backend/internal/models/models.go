package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type CategoryType string

type PriorityColor string

type NoteType string

const (
	CategoryTypeMandatory CategoryType = "mandatory"
	CategoryTypeOptional  CategoryType = "optional"

	PriorityColorRed    PriorityColor = "red"
	PriorityColorYellow PriorityColor = "yellow"
	PriorityColorGreen  PriorityColor = "green"

	NoteTypeAI   NoteType = "ai"
	NoteTypeUser NoteType = "user"
)

type User struct {
	ID           uuid.UUID `json:"id"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"-"`
	Name         *string   `json:"name,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type BudgetPlan struct {
	ID              uuid.UUID `json:"id"`
	UserID          uuid.UUID `json:"user_id"`
	Title           string    `json:"title"`
	BudgetCents     int64     `json:"budget_cents"`
	PeriodStart     time.Time `json:"period_start"`
	PeriodEnd       time.Time `json:"period_end"`
	BackgroundColor string    `json:"background_color"`
	IsAIGenerated   bool      `json:"is_ai_generated"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

type ExpenseCategory struct {
	ID           uuid.UUID    `json:"id"`
	PlanID       uuid.UUID    `json:"plan_id"`
	Title        string       `json:"title"`
	CategoryType CategoryType `json:"category_type"`
	SortOrder    int          `json:"sort_order"`
	CreatedAt    time.Time    `json:"created_at"`
}

type ExpenseItem struct {
	ID            uuid.UUID     `json:"id"`
	CategoryID    uuid.UUID     `json:"category_id"`
	Title         string        `json:"title"`
	AmountCents   int64         `json:"amount_cents"`
	PriorityColor PriorityColor `json:"priority_color"`
	IsCompleted   bool          `json:"is_completed"`
	SortOrder     int           `json:"sort_order"`
	CreatedAt     time.Time     `json:"created_at"`
	UpdatedAt     time.Time     `json:"updated_at"`
}

type Note struct {
	ID        uuid.UUID `json:"id"`
	PlanID    uuid.UUID `json:"plan_id"`
	Content   string    `json:"content"`
	NoteType  NoteType  `json:"note_type"`
	SortOrder int       `json:"sort_order"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type AIInputData struct {
	ID                uuid.UUID       `json:"id"`
	UserID            uuid.UUID       `json:"user_id"`
	Period            *string         `json:"period,omitempty"`
	Income            json.RawMessage `json:"income,omitempty"`
	MandatoryExpenses json.RawMessage `json:"mandatory_expenses,omitempty"`
	OptionalExpenses  json.RawMessage `json:"optional_expenses,omitempty"`
	Assets            json.RawMessage `json:"assets,omitempty"`
	Debts             json.RawMessage `json:"debts,omitempty"`
	AdditionalNotes   *string         `json:"additional_notes,omitempty"`
	CreatedAt         time.Time       `json:"created_at"`
}

type RefreshToken struct {
	ID         uuid.UUID  `json:"id"`
	UserID     uuid.UUID  `json:"user_id"`
	TokenHash  string     `json:"-"`
	ExpiresAt  time.Time  `json:"expires_at"`
	CreatedAt  time.Time  `json:"created_at"`
	RevokedAt  *time.Time `json:"revoked_at,omitempty"`
	ReplacedBy *uuid.UUID `json:"replaced_by,omitempty"`
}
