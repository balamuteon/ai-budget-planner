package repository

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"example.com/ai-budget-planner/backend/internal/models"
)

type NoteRepository struct {
	db *pgxpool.Pool
}

// NewNoteRepository создает репозиторий заметок.
func NewNoteRepository(db *pgxpool.Pool) *NoteRepository {
	return &NoteRepository{db: db}
}

// ListByPlan возвращает заметки плана.
func (r *NoteRepository) ListByPlan(ctx context.Context, userID, planID uuid.UUID) ([]models.Note, error) {
	var exists bool
	if err := r.db.QueryRow(ctx,
		`SELECT EXISTS (
			SELECT 1 FROM budget_plans WHERE id = $1 AND user_id = $2
		 )`,
		planID, userID,
	).Scan(&exists); err != nil {
		return nil, err
	}

	if !exists {
		return nil, ErrNotFound
	}

	rows, err := r.db.Query(ctx,
		`SELECT id, plan_id, content, note_type, sort_order, created_at, updated_at
		 FROM notes
		 WHERE plan_id = $1
		 ORDER BY sort_order, created_at`,
		planID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	notes := make([]models.Note, 0)
	for rows.Next() {
		var note models.Note

		err := rows.Scan(&note.ID, &note.PlanID, &note.Content, &note.NoteType, &note.SortOrder, &note.CreatedAt, &note.UpdatedAt)
		if err != nil {
			return nil, err
		}

		notes = append(notes, note)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return notes, nil
}

// ListByPlanAndType возвращает заметки плана по типу.
func (r *NoteRepository) ListByPlanAndType(ctx context.Context, userID, planID uuid.UUID, noteType models.NoteType) ([]models.Note, error) {
	var exists bool
	if err := r.db.QueryRow(ctx,
		`SELECT EXISTS (
			SELECT 1 FROM budget_plans WHERE id = $1 AND user_id = $2
		 )`,
		planID, userID,
	).Scan(&exists); err != nil {
		return nil, err
	}

	if !exists {
		return nil, ErrNotFound
	}

	rows, err := r.db.Query(ctx,
		`SELECT id, plan_id, content, note_type, sort_order, created_at, updated_at
		 FROM notes
		 WHERE plan_id = $1 AND note_type = $2
		 ORDER BY sort_order, created_at`,
		planID, noteType,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	notes := make([]models.Note, 0)
	for rows.Next() {
		var note models.Note

		err := rows.Scan(&note.ID, &note.PlanID, &note.Content, &note.NoteType, &note.SortOrder, &note.CreatedAt, &note.UpdatedAt)
		if err != nil {
			return nil, err
		}

		notes = append(notes, note)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return notes, nil
}

// Create добавляет заметку к плану.
func (r *NoteRepository) Create(ctx context.Context, userID, planID uuid.UUID, content string, noteType models.NoteType) (models.Note, error) {
	var note models.Note

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return note, err
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	var exists bool
	if err := tx.QueryRow(ctx,
		`SELECT EXISTS (
			SELECT 1 FROM budget_plans WHERE id = $1 AND user_id = $2
		 )`,
		planID, userID,
	).Scan(&exists); err != nil {
		return note, err
	}

	if !exists {
		return note, ErrNotFound
	}

	var maxOrder int
	err = tx.QueryRow(ctx,
		`SELECT COALESCE(MAX(sort_order), -1)
		 FROM notes
		 WHERE plan_id = $1`,
		planID,
	).Scan(&maxOrder)
	if err != nil {
		return note, err
	}

	sortOrder := maxOrder + 1

	err = tx.QueryRow(ctx,
		`INSERT INTO notes (id, plan_id, content, note_type, sort_order)
		 VALUES ($1, $2, $3, $4, $5)
		 RETURNING id, plan_id, content, note_type, sort_order, created_at, updated_at`,
		uuid.New(), planID, content, noteType, sortOrder,
	).Scan(&note.ID, &note.PlanID, &note.Content, &note.NoteType, &note.SortOrder, &note.CreatedAt, &note.UpdatedAt)
	if err != nil {
		return note, err
	}

	if err := tx.Commit(ctx); err != nil {
		return note, err
	}

	return note, nil
}

// DeleteByPlanAndType удаляет заметки плана по типу.
func (r *NoteRepository) DeleteByPlanAndType(ctx context.Context, userID, planID uuid.UUID, noteType models.NoteType) error {
	cmd, err := r.db.Exec(ctx,
		`DELETE FROM notes n
		 USING budget_plans p
		 WHERE n.plan_id = p.id
		   AND p.id = $1
		   AND p.user_id = $2
		   AND n.note_type = $3`,
		planID, userID, noteType,
	)
	if err != nil {
		return err
	}

	if cmd.RowsAffected() == 0 {
		return ErrNotFound
	}

	return nil
}

// Update изменяет заметку.
func (r *NoteRepository) Update(ctx context.Context, userID, noteID uuid.UUID, content string, noteType models.NoteType) (models.Note, error) {
	var note models.Note

	err := r.db.QueryRow(ctx,
		`UPDATE notes n
		 SET content = $2,
		     note_type = $3,
		     updated_at = NOW()
		 FROM budget_plans p
		 WHERE n.id = $1
		   AND n.plan_id = p.id
		   AND p.user_id = $4
		 RETURNING n.id, n.plan_id, n.content, n.note_type, n.sort_order, n.created_at, n.updated_at`,
		noteID, content, noteType, userID,
	).Scan(&note.ID, &note.PlanID, &note.Content, &note.NoteType, &note.SortOrder, &note.CreatedAt, &note.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return note, ErrNotFound
		}
		return note, err
	}

	return note, nil
}

// Delete удаляет заметку.
func (r *NoteRepository) Delete(ctx context.Context, userID, noteID uuid.UUID) error {
	cmd, err := r.db.Exec(ctx,
		`DELETE FROM notes n
		 USING budget_plans p
		 WHERE n.id = $1
		   AND n.plan_id = p.id
		   AND p.user_id = $2`,
		noteID, userID,
	)
	if err != nil {
		return err
	}

	if cmd.RowsAffected() == 0 {
		return ErrNotFound
	}

	return nil
}

// Reorder меняет порядок заметок в плане.
func (r *NoteRepository) Reorder(ctx context.Context, userID, noteID uuid.UUID, noteIDs []uuid.UUID) error {
	if len(noteIDs) == 0 {
		return ErrInvalid
	}

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	var planID uuid.UUID
	err = tx.QueryRow(ctx,
		`SELECT n.plan_id
		 FROM notes n
		 JOIN budget_plans p ON p.id = n.plan_id
		 WHERE n.id = $1 AND p.user_id = $2`,
		noteID, userID,
	).Scan(&planID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return err
	}

	var count int
	err = tx.QueryRow(ctx,
		`SELECT COUNT(*)
		 FROM notes
		 WHERE plan_id = $1 AND id = ANY($2)`,
		planID, noteIDs,
	).Scan(&count)
	if err != nil {
		return err
	}

	if count != len(noteIDs) {
		return ErrInvalid
	}

	var total int
	err = tx.QueryRow(ctx,
		`SELECT COUNT(*)
		 FROM notes
		 WHERE plan_id = $1`,
		planID,
	).Scan(&total)
	if err != nil {
		return err
	}

	if total != len(noteIDs) {
		return ErrInvalid
	}

	cmd, err := tx.Exec(ctx,
		`UPDATE notes AS n
		 SET sort_order = v.ord - 1
		 FROM unnest($1::uuid[]) WITH ORDINALITY AS v(id, ord)
		 WHERE n.id = v.id AND n.plan_id = $2`,
		noteIDs, planID,
	)
	if err != nil {
		return err
	}

	if cmd.RowsAffected() != int64(len(noteIDs)) {
		return ErrInvalid
	}

	return tx.Commit(ctx)
}
