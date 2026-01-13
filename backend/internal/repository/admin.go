package repository

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type AdminRepository struct {
	db *pgxpool.Pool
}

type AdminUser struct {
	ID        uuid.UUID
	Email     string
	Name      *string
	CreatedAt time.Time
	UpdatedAt time.Time
}

type AIRequestFilter struct {
	UserID      *uuid.UUID
	Success     *bool
	RequestType *string
}

type AIRequestRecord struct {
	ID              uuid.UUID
	UserID          uuid.UUID
	RequestType     string
	Provider        string
	Model           string
	Prompt          *string
	RequestPayload  []byte
	ResponsePayload []byte
	RawResponse     *string
	Success         bool
	ErrorMessage    *string
	CreatedAt       time.Time
}

type DailyCount struct {
	Day   time.Time
	Count int
}

type UsageStats struct {
	Users           int
	Plans           int
	AIRequests      int
	AISuccess       int
	AIFail          int
	AIRequestsByDay []DailyCount
}

// NewAdminRepository создает репозиторий для админских запросов.
func NewAdminRepository(db *pgxpool.Pool) *AdminRepository {
	return &AdminRepository{db: db}
}

// ListUsers возвращает список пользователей с пагинацией.
func (r *AdminRepository) ListUsers(ctx context.Context, limit, offset int) ([]AdminUser, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, email, name, created_at, updated_at
		 FROM users
		 ORDER BY created_at DESC
		 LIMIT $1 OFFSET $2`,
		limit, offset,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	users := make([]AdminUser, 0)
	for rows.Next() {
		var user AdminUser
		var name *string
		if err := rows.Scan(&user.ID, &user.Email, &name, &user.CreatedAt, &user.UpdatedAt); err != nil {
			return nil, err
		}
		user.Name = name
		users = append(users, user)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return users, nil
}

// CountUsers возвращает общее количество пользователей.
func (r *AdminRepository) CountUsers(ctx context.Context) (int, error) {
	var count int
	if err := r.db.QueryRow(ctx, `SELECT COUNT(*) FROM users`).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

// ListAIRequests возвращает логи AI-запросов с фильтрацией.
func (r *AdminRepository) ListAIRequests(ctx context.Context, filter AIRequestFilter, limit, offset int, includePayloads bool) ([]AIRequestRecord, error) {
	where, args := buildAIRequestWhere(filter)

	columns := "id, user_id, request_type, provider, model, success, error_message, created_at"
	if includePayloads {
		columns = "id, user_id, request_type, provider, model, prompt, request_payload, response_payload, raw_response, success, error_message, created_at"
	}

	limitParam := len(args) + 1
	offsetParam := len(args) + 2
	query := fmt.Sprintf("SELECT %s FROM ai_requests%s ORDER BY created_at DESC LIMIT $%d OFFSET $%d", columns, where, limitParam, offsetParam)
	args = append(args, limit, offset)

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	requests := make([]AIRequestRecord, 0)
	for rows.Next() {
		var record AIRequestRecord
		if includePayloads {
			var prompt *string
			var requestPayload []byte
			var responsePayload []byte
			var rawResponse *string
			if err := rows.Scan(
				&record.ID,
				&record.UserID,
				&record.RequestType,
				&record.Provider,
				&record.Model,
				&prompt,
				&requestPayload,
				&responsePayload,
				&rawResponse,
				&record.Success,
				&record.ErrorMessage,
				&record.CreatedAt,
			); err != nil {
				return nil, err
			}
			record.Prompt = prompt
			record.RequestPayload = requestPayload
			record.ResponsePayload = responsePayload
			record.RawResponse = rawResponse
		} else {
			if err := rows.Scan(
				&record.ID,
				&record.UserID,
				&record.RequestType,
				&record.Provider,
				&record.Model,
				&record.Success,
				&record.ErrorMessage,
				&record.CreatedAt,
			); err != nil {
				return nil, err
			}
		}
		requests = append(requests, record)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return requests, nil
}

// CountAIRequests возвращает количество AI-запросов по фильтру.
func (r *AdminRepository) CountAIRequests(ctx context.Context, filter AIRequestFilter) (int, error) {
	where, args := buildAIRequestWhere(filter)

	query := fmt.Sprintf("SELECT COUNT(*) FROM ai_requests%s", where)
	var count int
	if err := r.db.QueryRow(ctx, query, args...).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

// UsageStats возвращает агрегированную статистику за N дней.
func (r *AdminRepository) UsageStats(ctx context.Context, days int) (UsageStats, error) {
	stats := UsageStats{}
	if days <= 0 {
		return stats, ErrInvalid
	}

	if err := r.db.QueryRow(ctx, `SELECT COUNT(*) FROM users`).Scan(&stats.Users); err != nil {
		return stats, err
	}

	if err := r.db.QueryRow(ctx, `SELECT COUNT(*) FROM budget_plans`).Scan(&stats.Plans); err != nil {
		return stats, err
	}

	if err := r.db.QueryRow(ctx,
		`SELECT COUNT(*),
		        COUNT(*) FILTER (WHERE success),
		        COUNT(*) FILTER (WHERE NOT success)
		 FROM ai_requests`,
	).Scan(&stats.AIRequests, &stats.AISuccess, &stats.AIFail); err != nil {
		return stats, err
	}

	start := time.Now().UTC().AddDate(0, 0, -days+1)
	rows, err := r.db.Query(ctx,
		`SELECT date_trunc('day', created_at)::date AS day,
		        COUNT(*)
		 FROM ai_requests
		 WHERE created_at >= $1
		 GROUP BY day
		 ORDER BY day DESC`,
		start,
	)
	if err != nil {
		return stats, err
	}
	defer rows.Close()

	stats.AIRequestsByDay = make([]DailyCount, 0)
	for rows.Next() {
		var row DailyCount
		if err := rows.Scan(&row.Day, &row.Count); err != nil {
			return stats, err
		}
		stats.AIRequestsByDay = append(stats.AIRequestsByDay, row)
	}

	if err := rows.Err(); err != nil {
		return stats, err
	}

	return stats, nil
}

func buildAIRequestWhere(filter AIRequestFilter) (string, []interface{}) {
	clauses := make([]string, 0)
	args := make([]interface{}, 0)

	if filter.UserID != nil {
		args = append(args, *filter.UserID)
		clauses = append(clauses, fmt.Sprintf("user_id = $%d", len(args)))
	}

	if filter.Success != nil {
		args = append(args, *filter.Success)
		clauses = append(clauses, fmt.Sprintf("success = $%d", len(args)))
	}

	if filter.RequestType != nil {
		args = append(args, *filter.RequestType)
		clauses = append(clauses, fmt.Sprintf("request_type = $%d", len(args)))
	}

	if len(clauses) == 0 {
		return "", args
	}

	return " WHERE " + strings.Join(clauses, " AND "), args
}
