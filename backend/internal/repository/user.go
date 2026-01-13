package repository

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"example.com/ai-budget-planner/backend/internal/models"
)

type UserRepository struct {
	db *pgxpool.Pool
}

// NewUserRepository создает репозиторий пользователей.
func NewUserRepository(db *pgxpool.Pool) *UserRepository {
	return &UserRepository{db: db}
}

// Create создает пользователя в базе.
func (r *UserRepository) Create(ctx context.Context, email, passwordHash string, name *string) (models.User, error) {
	var user models.User
	var nameValue *string

	err := r.db.QueryRow(ctx,
		`INSERT INTO users (email, password_hash, name)
		 VALUES ($1, $2, $3)
		 RETURNING id, email, password_hash, name, created_at, updated_at`,
		email, passwordHash, name,
	).Scan(&user.ID, &user.Email, &user.PasswordHash, &nameValue, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return user, ErrConflict
		}
		return user, err
	}

	user.Name = nameValue
	return user, nil
}

// GetByEmail возвращает пользователя по email.
func (r *UserRepository) GetByEmail(ctx context.Context, email string) (models.User, error) {
	var user models.User
	var nameValue *string

	err := r.db.QueryRow(ctx,
		`SELECT id, email, password_hash, name, created_at, updated_at
		 FROM users
		 WHERE email = $1`,
		email,
	).Scan(&user.ID, &user.Email, &user.PasswordHash, &nameValue, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return user, ErrNotFound
		}
		return user, err
	}

	user.Name = nameValue
	return user, nil
}

// GetByID возвращает пользователя по идентификатору.
func (r *UserRepository) GetByID(ctx context.Context, id uuid.UUID) (models.User, error) {
	var user models.User
	var nameValue *string

	err := r.db.QueryRow(ctx,
		`SELECT id, email, password_hash, name, created_at, updated_at
		 FROM users
		 WHERE id = $1`,
		id,
	).Scan(&user.ID, &user.Email, &user.PasswordHash, &nameValue, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return user, ErrNotFound
		}
		return user, err
	}

	user.Name = nameValue
	return user, nil
}
