package repository

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"example.com/ai-budget-planner/backend/internal/models"
)

type RefreshTokenRepository struct {
	db *pgxpool.Pool
}

// NewRefreshTokenRepository создает репозиторий refresh-токенов.
func NewRefreshTokenRepository(db *pgxpool.Pool) *RefreshTokenRepository {
	return &RefreshTokenRepository{db: db}
}

// Create сохраняет refresh-токен.
func (r *RefreshTokenRepository) Create(ctx context.Context, token models.RefreshToken) error {
	_, err := r.db.Exec(ctx,
		`INSERT INTO refresh_tokens (id, user_id, token_hash, expires_at)
		 VALUES ($1, $2, $3, $4)`,
		token.ID, token.UserID, token.TokenHash, token.ExpiresAt,
	)
	return err
}

// GetByID возвращает refresh-токен по идентификатору.
func (r *RefreshTokenRepository) GetByID(ctx context.Context, id uuid.UUID) (models.RefreshToken, error) {
	var token models.RefreshToken
	var revokedAt *time.Time
	var replacedBy *uuid.UUID

	err := r.db.QueryRow(ctx,
		`SELECT id, user_id, token_hash, expires_at, created_at, revoked_at, replaced_by
		 FROM refresh_tokens
		 WHERE id = $1`,
		id,
	).Scan(&token.ID, &token.UserID, &token.TokenHash, &token.ExpiresAt, &token.CreatedAt, &revokedAt, &replacedBy)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return token, ErrNotFound
		}
		return token, err
	}

	token.RevokedAt = revokedAt
	token.ReplacedBy = replacedBy
	return token, nil
}

// Revoke помечает refresh-токен отозванным.
func (r *RefreshTokenRepository) Revoke(ctx context.Context, id uuid.UUID, replacedBy *uuid.UUID) error {
	cmd, err := r.db.Exec(ctx,
		`UPDATE refresh_tokens
		 SET revoked_at = NOW(), replaced_by = $2
		 WHERE id = $1 AND revoked_at IS NULL`,
		id, replacedBy,
	)
	if err != nil {
		return err
	}

	if cmd.RowsAffected() == 0 {
		return ErrNotFound
	}

	return nil
}

// Rotate заменяет старый refresh-токен на новый.
func (r *RefreshTokenRepository) Rotate(ctx context.Context, oldID uuid.UUID, newToken models.RefreshToken) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	_, err = tx.Exec(ctx,
		`INSERT INTO refresh_tokens (id, user_id, token_hash, expires_at)
		 VALUES ($1, $2, $3, $4)`,
		newToken.ID, newToken.UserID, newToken.TokenHash, newToken.ExpiresAt,
	)
	if err != nil {
		return err
	}

	cmd, err := tx.Exec(ctx,
		`UPDATE refresh_tokens
		 SET revoked_at = NOW(), replaced_by = $2
		 WHERE id = $1 AND revoked_at IS NULL`,
		oldID, newToken.ID,
	)
	if err != nil {
		return err
	}

	if cmd.RowsAffected() == 0 {
		return ErrNotFound
	}

	return tx.Commit(ctx)
}
