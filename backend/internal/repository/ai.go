package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type AIRepository struct {
	db *pgxpool.Pool
}

type AIRequestLog struct {
	UserID          uuid.UUID
	RequestType     string
	Provider        string
	Model           string
	Prompt          string
	RequestPayload  []byte
	ResponsePayload []byte
	RawResponse     string
	Success         bool
	ErrorMessage    *string
}

// NewAIRepository создает репозиторий для AI-запросов.
func NewAIRepository(db *pgxpool.Pool) *AIRepository {
	return &AIRepository{db: db}
}

// LogRequest сохраняет лог AI-запроса.
func (r *AIRepository) LogRequest(ctx context.Context, log AIRequestLog) error {
	_, err := r.db.Exec(ctx,
		`INSERT INTO ai_requests
		 (user_id, request_type, provider, model, prompt, request_payload, response_payload, raw_response, success, error_message)
		 VALUES ($1, $2, $3, $4, $5, NULLIF($6, '')::jsonb, NULLIF($7, '')::jsonb, $8, $9, $10)`,
		log.UserID,
		log.RequestType,
		log.Provider,
		log.Model,
		log.Prompt,
		string(log.RequestPayload),
		string(log.ResponsePayload),
		log.RawResponse,
		log.Success,
		log.ErrorMessage,
	)
	return err
}

// SaveInputData сохраняет входные данные AI-формы пользователя.
func (r *AIRepository) SaveInputData(ctx context.Context, userID uuid.UUID, period *string, income, mandatory, optional, assets, debts []byte, additionalNotes *string) error {
	_, err := r.db.Exec(ctx,
		`INSERT INTO ai_input_data
		 (user_id, period, income, mandatory_expenses, optional_expenses, assets, debts, additional_notes)
		 VALUES ($1, $2, NULLIF($3, '')::jsonb, NULLIF($4, '')::jsonb, NULLIF($5, '')::jsonb, NULLIF($6, '')::jsonb, NULLIF($7, '')::jsonb, $8)`,
		userID,
		period,
		string(income),
		string(mandatory),
		string(optional),
		string(assets),
		string(debts),
		additionalNotes,
	)
	return err
}
