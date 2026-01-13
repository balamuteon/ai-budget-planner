package repository

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"example.com/ai-budget-planner/backend/internal/models"
)

type ItemRepository struct {
	db *pgxpool.Pool
}

// NewItemRepository создает репозиторий расходов.
func NewItemRepository(db *pgxpool.Pool) *ItemRepository {
	return &ItemRepository{db: db}
}

// GetPlanIDByItemID возвращает план, которому принадлежит расход.
func (r *ItemRepository) GetPlanIDByItemID(ctx context.Context, userID, itemID uuid.UUID) (uuid.UUID, error) {
	var planID uuid.UUID

	err := r.db.QueryRow(ctx,
		`SELECT p.id
		 FROM expense_items i
		 JOIN expense_categories c ON c.id = i.category_id
		 JOIN budget_plans p ON p.id = c.plan_id
		 WHERE i.id = $1 AND p.user_id = $2`,
		itemID, userID,
	).Scan(&planID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return uuid.Nil, ErrNotFound
		}
		return uuid.Nil, err
	}

	return planID, nil
}

// Create добавляет новый расход с проверкой бюджета.
func (r *ItemRepository) Create(ctx context.Context, userID, planID, categoryID uuid.UUID, title string, amountCents int64, priorityColor models.PriorityColor, isCompleted bool) (models.ExpenseItem, error) {
	var item models.ExpenseItem

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return item, err
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	budgetCents, err := lockPlanBudget(ctx, tx, userID, planID)
	if err != nil {
		return item, err
	}

	if err = ensureCategoryInPlan(ctx, tx, categoryID, planID); err != nil {
		return item, err
	}

	currentTotal, err := sumPlanAmount(ctx, tx, planID)
	if err != nil {
		return item, err
	}

	if currentTotal+amountCents > budgetCents {
		return item, ErrBudgetExceeded
	}

	var maxOrder int
	err = tx.QueryRow(ctx,
		`SELECT COALESCE(MAX(sort_order), -1)
		 FROM expense_items
		 WHERE category_id = $1`,
		categoryID,
	).Scan(&maxOrder)
	if err != nil {
		return item, err
	}

	sortOrder := maxOrder + 1

	err = tx.QueryRow(ctx,
		`INSERT INTO expense_items (id, category_id, title, amount_cents, priority_color, is_completed, sort_order)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)
		 RETURNING id, category_id, title, amount_cents, priority_color, is_completed, sort_order, created_at, updated_at`,
		uuid.New(), categoryID, title, amountCents, priorityColor, isCompleted, sortOrder,
	).Scan(&item.ID, &item.CategoryID, &item.Title, &item.AmountCents, &item.PriorityColor, &item.IsCompleted, &item.SortOrder, &item.CreatedAt, &item.UpdatedAt)
	if err != nil {
		return item, err
	}

	if err := tx.Commit(ctx); err != nil {
		return item, err
	}

	return item, nil
}

// Update изменяет расход с проверкой бюджета.
func (r *ItemRepository) Update(ctx context.Context, userID, itemID uuid.UUID, title string, amountCents int64, priorityColor models.PriorityColor) (models.ExpenseItem, error) {
	var item models.ExpenseItem

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return item, err
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	var planID uuid.UUID
	var budgetCents int64
	var currentAmount int64

	err = tx.QueryRow(ctx,
		`SELECT p.id, p.budget_cents, i.amount_cents
		 FROM expense_items i
		 JOIN expense_categories c ON c.id = i.category_id
		 JOIN budget_plans p ON p.id = c.plan_id
		 WHERE i.id = $1 AND p.user_id = $2
		 FOR UPDATE OF p`,
		itemID, userID,
	).Scan(&planID, &budgetCents, &currentAmount)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return item, ErrNotFound
		}
		return item, err
	}

	currentTotal, err := sumPlanAmount(ctx, tx, planID)
	if err != nil {
		return item, err
	}

	newTotal := currentTotal - currentAmount + amountCents
	if newTotal > budgetCents {
		return item, ErrBudgetExceeded
	}

	err = tx.QueryRow(ctx,
		`UPDATE expense_items
		 SET title = $2,
		     amount_cents = $3,
		     priority_color = $4,
		     updated_at = NOW()
		 WHERE id = $1
		 RETURNING id, category_id, title, amount_cents, priority_color, is_completed, sort_order, created_at, updated_at`,
		itemID, title, amountCents, priorityColor,
	).Scan(&item.ID, &item.CategoryID, &item.Title, &item.AmountCents, &item.PriorityColor, &item.IsCompleted, &item.SortOrder, &item.CreatedAt, &item.UpdatedAt)
	if err != nil {
		return item, err
	}

	if err := tx.Commit(ctx); err != nil {
		return item, err
	}

	return item, nil
}

// Delete удаляет расход пользователя.
func (r *ItemRepository) Delete(ctx context.Context, userID, itemID uuid.UUID) error {
	cmd, err := r.db.Exec(ctx,
		`DELETE FROM expense_items i
		 USING expense_categories c, budget_plans p
		 WHERE i.id = $1
		   AND i.category_id = c.id
		   AND c.plan_id = p.id
		   AND p.user_id = $2`,
		itemID, userID,
	)
	if err != nil {
		return err
	}

	if cmd.RowsAffected() == 0 {
		return ErrNotFound
	}

	return nil
}

// Toggle переключает статус выполнения расхода.
func (r *ItemRepository) Toggle(ctx context.Context, userID, itemID uuid.UUID, isCompleted *bool) (models.ExpenseItem, error) {
	var item models.ExpenseItem

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return item, err
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	var current bool
	err = tx.QueryRow(ctx,
		`SELECT i.is_completed
		 FROM expense_items i
		 JOIN expense_categories c ON c.id = i.category_id
		 JOIN budget_plans p ON p.id = c.plan_id
		 WHERE i.id = $1 AND p.user_id = $2
		 FOR UPDATE`,
		itemID, userID,
	).Scan(&current)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return item, ErrNotFound
		}
		return item, err
	}

	newValue := current
	if isCompleted == nil {
		newValue = !current
	} else {
		newValue = *isCompleted
	}

	err = tx.QueryRow(ctx,
		`UPDATE expense_items
		 SET is_completed = $2,
		     updated_at = NOW()
		 WHERE id = $1
		 RETURNING id, category_id, title, amount_cents, priority_color, is_completed, sort_order, created_at, updated_at`,
		itemID, newValue,
	).Scan(&item.ID, &item.CategoryID, &item.Title, &item.AmountCents, &item.PriorityColor, &item.IsCompleted, &item.SortOrder, &item.CreatedAt, &item.UpdatedAt)
	if err != nil {
		return item, err
	}

	if err := tx.Commit(ctx); err != nil {
		return item, err
	}

	return item, nil
}

// UpdateColor обновляет цвет приоритета расхода.
func (r *ItemRepository) UpdateColor(ctx context.Context, userID, itemID uuid.UUID, priorityColor models.PriorityColor) (models.ExpenseItem, error) {
	var item models.ExpenseItem

	err := r.db.QueryRow(ctx,
		`UPDATE expense_items i
		 SET priority_color = $3,
		     updated_at = NOW()
		 FROM expense_categories c
		 JOIN budget_plans p ON p.id = c.plan_id
		 WHERE i.id = $1
		   AND i.category_id = c.id
		   AND p.user_id = $2
		 RETURNING i.id, i.category_id, i.title, i.amount_cents, i.priority_color, i.is_completed, i.sort_order, i.created_at, i.updated_at`,
		itemID, userID, priorityColor,
	).Scan(&item.ID, &item.CategoryID, &item.Title, &item.AmountCents, &item.PriorityColor, &item.IsCompleted, &item.SortOrder, &item.CreatedAt, &item.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return item, ErrNotFound
		}
		return item, err
	}

	return item, nil
}

// Reorder меняет порядок расходов внутри категории.
func (r *ItemRepository) Reorder(ctx context.Context, userID, itemID uuid.UUID, itemIDs []uuid.UUID) error {
	if len(itemIDs) == 0 {
		return ErrInvalid
	}

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	var categoryID uuid.UUID
	err = tx.QueryRow(ctx,
		`SELECT c.id
		 FROM expense_items i
		 JOIN expense_categories c ON c.id = i.category_id
		 JOIN budget_plans p ON p.id = c.plan_id
		 WHERE i.id = $1 AND p.user_id = $2`,
		itemID, userID,
	).Scan(&categoryID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return err
	}

	var count int
	err = tx.QueryRow(ctx,
		`SELECT COUNT(*)
		 FROM expense_items
		 WHERE category_id = $1 AND id = ANY($2)`,
		categoryID, itemIDs,
	).Scan(&count)
	if err != nil {
		return err
	}

	if count != len(itemIDs) {
		return ErrInvalid
	}

	var total int
	err = tx.QueryRow(ctx,
		`SELECT COUNT(*)
		 FROM expense_items
		 WHERE category_id = $1`,
		categoryID,
	).Scan(&total)
	if err != nil {
		return err
	}

	if total != len(itemIDs) {
		return ErrInvalid
	}

	cmd, err := tx.Exec(ctx,
		`UPDATE expense_items AS i
		 SET sort_order = v.ord - 1
		 FROM unnest($1::uuid[]) WITH ORDINALITY AS v(id, ord)
		 WHERE i.id = v.id AND i.category_id = $2`,
		itemIDs, categoryID,
	)
	if err != nil {
		return err
	}

	if cmd.RowsAffected() != int64(len(itemIDs)) {
		return ErrInvalid
	}

	return tx.Commit(ctx)
}

func lockPlanBudget(ctx context.Context, tx pgx.Tx, userID, planID uuid.UUID) (int64, error) {
	var budgetCents int64
	if err := tx.QueryRow(ctx,
		`SELECT budget_cents
		 FROM budget_plans
		 WHERE id = $1 AND user_id = $2
		 FOR UPDATE`,
		planID, userID,
	).Scan(&budgetCents); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, ErrNotFound
		}
		return 0, err
	}

	return budgetCents, nil
}

func ensureCategoryInPlan(ctx context.Context, tx pgx.Tx, categoryID, planID uuid.UUID) error {
	var exists bool
	err := tx.QueryRow(ctx,
		`SELECT EXISTS (
			SELECT 1 FROM expense_categories WHERE id = $1 AND plan_id = $2
		 )`,
		categoryID, planID,
	).Scan(&exists)
	if err != nil {
		return err
	}

	if !exists {
		return ErrNotFound
	}

	return nil
}

func sumPlanAmount(ctx context.Context, tx pgx.Tx, planID uuid.UUID) (int64, error) {
	var total int64
	if err := tx.QueryRow(ctx,
		`SELECT COALESCE(SUM(i.amount_cents), 0)
		 FROM expense_items i
		 JOIN expense_categories c ON c.id = i.category_id
		 WHERE c.plan_id = $1`,
		planID,
	).Scan(&total); err != nil {
		return 0, err
	}

	return total, nil
}
