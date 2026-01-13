package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"example.com/ai-budget-planner/backend/internal/models"
)

type PlanRepository struct {
	db *pgxpool.Pool
}

type defaultCategory struct {
	Title        string
	CategoryType models.CategoryType
}

type PlanCategoryInput struct {
	Title        string
	CategoryType models.CategoryType
	Items        []PlanItemInput
}

type PlanItemInput struct {
	Title         string
	AmountCents   int64
	PriorityColor models.PriorityColor
	IsCompleted   bool
}

type PlanNoteInput struct {
	Content  string
	NoteType models.NoteType
}

var defaultCategories = []defaultCategory{
	{Title: "Жилье", CategoryType: models.CategoryTypeMandatory},
	{Title: "Коммунальные услуги", CategoryType: models.CategoryTypeMandatory},
	{Title: "Еда", CategoryType: models.CategoryTypeMandatory},
	{Title: "Транспорт", CategoryType: models.CategoryTypeMandatory},
	{Title: "Развлечения", CategoryType: models.CategoryTypeOptional},
	{Title: "Другое", CategoryType: models.CategoryTypeOptional},
}

type PlanWithSpent struct {
	Plan       models.BudgetPlan
	SpentCents int64
}

// NewPlanRepository создает репозиторий планов бюджета.
func NewPlanRepository(db *pgxpool.Pool) *PlanRepository {
	return &PlanRepository{db: db}
}

// Create создает план и базовые категории.
func (r *PlanRepository) Create(ctx context.Context, userID uuid.UUID, title string, budgetCents int64, periodStart, periodEnd time.Time, backgroundColor string, isAIGenerated bool) (models.BudgetPlan, error) {
	var plan models.BudgetPlan

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return plan, err
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	err = tx.QueryRow(ctx,
		`INSERT INTO budget_plans (user_id, title, budget_cents, period_start, period_end, background_color, is_ai_generated)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)
		 RETURNING id, user_id, title, budget_cents, period_start, period_end, background_color, is_ai_generated, created_at, updated_at`,
		userID, title, budgetCents, periodStart, periodEnd, backgroundColor, isAIGenerated,
	).Scan(&plan.ID, &plan.UserID, &plan.Title, &plan.BudgetCents, &plan.PeriodStart, &plan.PeriodEnd, &plan.BackgroundColor, &plan.IsAIGenerated, &plan.CreatedAt, &plan.UpdatedAt)
	if err != nil {
		return plan, err
	}

	for idx, category := range defaultCategories {
		_, err = tx.Exec(ctx,
			`INSERT INTO expense_categories (id, plan_id, title, category_type, sort_order)
			 VALUES ($1, $2, $3, $4, $5)`,
			uuid.New(), plan.ID, category.Title, category.CategoryType, idx,
		)
		if err != nil {
			return plan, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return plan, err
	}

	return plan, nil
}

// CreateWithDetails создает план вместе с категориями и заметками.
func (r *PlanRepository) CreateWithDetails(ctx context.Context, userID uuid.UUID, title string, budgetCents int64, periodStart, periodEnd time.Time, backgroundColor string, isAIGenerated bool, categories []PlanCategoryInput, notes []PlanNoteInput) (models.BudgetPlan, error) {
	var plan models.BudgetPlan

	if len(categories) == 0 {
		return plan, ErrInvalid
	}

	var total int64
	for _, category := range categories {
		for _, item := range category.Items {
			total += item.AmountCents
		}
	}

	if total > budgetCents {
		return plan, ErrBudgetExceeded
	}

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return plan, err
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	err = tx.QueryRow(ctx,
		`INSERT INTO budget_plans (user_id, title, budget_cents, period_start, period_end, background_color, is_ai_generated)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)
		 RETURNING id, user_id, title, budget_cents, period_start, period_end, background_color, is_ai_generated, created_at, updated_at`,
		userID, title, budgetCents, periodStart, periodEnd, backgroundColor, isAIGenerated,
	).Scan(&plan.ID, &plan.UserID, &plan.Title, &plan.BudgetCents, &plan.PeriodStart, &plan.PeriodEnd, &plan.BackgroundColor, &plan.IsAIGenerated, &plan.CreatedAt, &plan.UpdatedAt)
	if err != nil {
		return plan, err
	}

	for idx, category := range categories {
		if strings.TrimSpace(category.Title) == "" {
			return plan, ErrInvalid
		}

		categoryID := uuid.New()
		_, err = tx.Exec(ctx,
			`INSERT INTO expense_categories (id, plan_id, title, category_type, sort_order)
			 VALUES ($1, $2, $3, $4, $5)`,
			categoryID, plan.ID, category.Title, category.CategoryType, idx,
		)
		if err != nil {
			return plan, err
		}

		for itemIdx, item := range category.Items {
			if strings.TrimSpace(item.Title) == "" || item.AmountCents <= 0 {
				return plan, ErrInvalid
			}

			_, err = tx.Exec(ctx,
				`INSERT INTO expense_items (id, category_id, title, amount_cents, priority_color, is_completed, sort_order)
				 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
				uuid.New(), categoryID, item.Title, item.AmountCents, item.PriorityColor, item.IsCompleted, itemIdx,
			)
			if err != nil {
				return plan, err
			}
		}
	}

	for idx, note := range notes {
		if strings.TrimSpace(note.Content) == "" {
			return plan, ErrInvalid
		}

		_, err = tx.Exec(ctx,
			`INSERT INTO notes (id, plan_id, content, note_type, sort_order)
			 VALUES ($1, $2, $3, $4, $5)`,
			uuid.New(), plan.ID, note.Content, note.NoteType, idx,
		)
		if err != nil {
			return plan, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return plan, err
	}

	return plan, nil
}

// Update обновляет план бюджета.
func (r *PlanRepository) Update(ctx context.Context, userID, planID uuid.UUID, title string, budgetCents int64, periodStart, periodEnd time.Time, backgroundColor *string, isAIGenerated *bool) (models.BudgetPlan, error) {
	var plan models.BudgetPlan

	err := r.db.QueryRow(ctx,
		`UPDATE budget_plans
		 SET title = $3,
		     budget_cents = $4,
		     period_start = $5,
		     period_end = $6,
		     background_color = COALESCE($7, background_color),
		     is_ai_generated = COALESCE($8, is_ai_generated),
		     updated_at = NOW()
		 WHERE id = $1 AND user_id = $2
		 RETURNING id, user_id, title, budget_cents, period_start, period_end, background_color, is_ai_generated, created_at, updated_at`,
		planID, userID, title, budgetCents, periodStart, periodEnd, backgroundColor, isAIGenerated,
	).Scan(&plan.ID, &plan.UserID, &plan.Title, &plan.BudgetCents, &plan.PeriodStart, &plan.PeriodEnd, &plan.BackgroundColor, &plan.IsAIGenerated, &plan.CreatedAt, &plan.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return plan, ErrNotFound
		}
		return plan, err
	}

	return plan, nil
}

// Delete удаляет план бюджета.
func (r *PlanRepository) Delete(ctx context.Context, userID, planID uuid.UUID) error {
	cmd, err := r.db.Exec(ctx,
		`DELETE FROM budget_plans
		 WHERE id = $1 AND user_id = $2`,
		planID, userID,
	)
	if err != nil {
		return err
	}

	if cmd.RowsAffected() == 0 {
		return ErrNotFound
	}

	return nil
}

// GetByID возвращает план пользователя по идентификатору.
func (r *PlanRepository) GetByID(ctx context.Context, userID, planID uuid.UUID) (models.BudgetPlan, error) {
	var plan models.BudgetPlan

	err := r.db.QueryRow(ctx,
		`SELECT id, user_id, title, budget_cents, period_start, period_end, background_color, is_ai_generated, created_at, updated_at
		 FROM budget_plans
		 WHERE id = $1 AND user_id = $2`,
		planID, userID,
	).Scan(&plan.ID, &plan.UserID, &plan.Title, &plan.BudgetCents, &plan.PeriodStart, &plan.PeriodEnd, &plan.BackgroundColor, &plan.IsAIGenerated, &plan.CreatedAt, &plan.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return plan, ErrNotFound
		}
		return plan, err
	}

	return plan, nil
}

// ListByUser возвращает список планов пользователя.
func (r *PlanRepository) ListByUser(ctx context.Context, userID uuid.UUID) ([]PlanWithSpent, error) {
	rows, err := r.db.Query(ctx,
		`SELECT p.id, p.user_id, p.title, p.budget_cents, p.period_start, p.period_end,
		        p.background_color, p.is_ai_generated, p.created_at, p.updated_at,
		        COALESCE(SUM(CASE WHEN i.is_completed THEN i.amount_cents ELSE 0 END), 0) AS spent_cents
		 FROM budget_plans p
		 LEFT JOIN expense_categories c ON c.plan_id = p.id
		 LEFT JOIN expense_items i ON i.category_id = c.id
		 WHERE p.user_id = $1 AND p.period_end >= CURRENT_DATE
		 GROUP BY p.id, p.user_id, p.title, p.budget_cents, p.period_start, p.period_end,
		          p.background_color, p.is_ai_generated, p.created_at, p.updated_at
		 ORDER BY p.created_at DESC`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	plans := make([]PlanWithSpent, 0)
	for rows.Next() {
		var plan models.BudgetPlan
		var spent int64

		err := rows.Scan(&plan.ID, &plan.UserID, &plan.Title, &plan.BudgetCents, &plan.PeriodStart, &plan.PeriodEnd, &plan.BackgroundColor, &plan.IsAIGenerated, &plan.CreatedAt, &plan.UpdatedAt, &spent)
		if err != nil {
			return nil, err
		}

		plans = append(plans, PlanWithSpent{Plan: plan, SpentCents: spent})
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return plans, nil
}

// ListArchivedByUser возвращает архивные планы пользователя.
func (r *PlanRepository) ListArchivedByUser(ctx context.Context, userID uuid.UUID) ([]PlanWithSpent, error) {
	rows, err := r.db.Query(ctx,
		`SELECT p.id, p.user_id, p.title, p.budget_cents, p.period_start, p.period_end,
		        p.background_color, p.is_ai_generated, p.created_at, p.updated_at,
		        COALESCE(SUM(CASE WHEN i.is_completed THEN i.amount_cents ELSE 0 END), 0) AS spent_cents
		 FROM budget_plans p
		 LEFT JOIN expense_categories c ON c.plan_id = p.id
		 LEFT JOIN expense_items i ON i.category_id = c.id
		 WHERE p.user_id = $1 AND p.period_end < CURRENT_DATE
		 GROUP BY p.id, p.user_id, p.title, p.budget_cents, p.period_start, p.period_end,
		          p.background_color, p.is_ai_generated, p.created_at, p.updated_at
		 ORDER BY p.period_end DESC`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	plans := make([]PlanWithSpent, 0)
	for rows.Next() {
		var plan models.BudgetPlan
		var spent int64

		err := rows.Scan(&plan.ID, &plan.UserID, &plan.Title, &plan.BudgetCents, &plan.PeriodStart, &plan.PeriodEnd, &plan.BackgroundColor, &plan.IsAIGenerated, &plan.CreatedAt, &plan.UpdatedAt, &spent)
		if err != nil {
			return nil, err
		}

		plans = append(plans, PlanWithSpent{Plan: plan, SpentCents: spent})
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return plans, nil
}

// GetSpentCents считает фактические траты по плану.
func (r *PlanRepository) GetSpentCents(ctx context.Context, planID uuid.UUID) (int64, error) {
	var spent int64

	err := r.db.QueryRow(ctx,
		`SELECT COALESCE(SUM(CASE WHEN i.is_completed THEN i.amount_cents ELSE 0 END), 0)
		 FROM expense_categories c
		 LEFT JOIN expense_items i ON i.category_id = c.id
		 WHERE c.plan_id = $1`,
		planID,
	).Scan(&spent)
	if err != nil {
		return 0, err
	}

	return spent, nil
}

// ListCategories возвращает категории плана.
func (r *PlanRepository) ListCategories(ctx context.Context, planID uuid.UUID) ([]models.ExpenseCategory, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, plan_id, title, category_type, sort_order, created_at
		 FROM expense_categories
		 WHERE plan_id = $1
		 ORDER BY sort_order, created_at`,
		planID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	categories := make([]models.ExpenseCategory, 0)
	for rows.Next() {
		var category models.ExpenseCategory

		err := rows.Scan(&category.ID, &category.PlanID, &category.Title, &category.CategoryType, &category.SortOrder, &category.CreatedAt)
		if err != nil {
			return nil, err
		}

		categories = append(categories, category)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return categories, nil
}

// ListItemsByCategoryIDs возвращает расходы по списку категорий.
func (r *PlanRepository) ListItemsByCategoryIDs(ctx context.Context, categoryIDs []uuid.UUID) ([]models.ExpenseItem, error) {
	if len(categoryIDs) == 0 {
		return []models.ExpenseItem{}, nil
	}

	rows, err := r.db.Query(ctx,
		`SELECT id, category_id, title, amount_cents, priority_color, is_completed, sort_order, created_at, updated_at
		 FROM expense_items
		 WHERE category_id = ANY($1)
		 ORDER BY sort_order, created_at`,
		categoryIDs,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]models.ExpenseItem, 0)
	for rows.Next() {
		var item models.ExpenseItem

		err := rows.Scan(&item.ID, &item.CategoryID, &item.Title, &item.AmountCents, &item.PriorityColor, &item.IsCompleted, &item.SortOrder, &item.CreatedAt, &item.UpdatedAt)
		if err != nil {
			return nil, err
		}

		items = append(items, item)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return items, nil
}

// ListNotes возвращает заметки плана.
func (r *PlanRepository) ListNotes(ctx context.Context, planID uuid.UUID) ([]models.Note, error) {
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

// ReorderCategories меняет порядок категорий в плане.
func (r *PlanRepository) ReorderCategories(ctx context.Context, userID, planID uuid.UUID, categoryIDs []uuid.UUID) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	var exists bool
	err = tx.QueryRow(ctx,
		`SELECT EXISTS (
			SELECT 1 FROM budget_plans WHERE id = $1 AND user_id = $2
		 )`,
		planID, userID,
	).Scan(&exists)
	if err != nil {
		return err
	}
	if !exists {
		return ErrNotFound
	}

	var count int
	err = tx.QueryRow(ctx,
		`SELECT COUNT(*)
		 FROM expense_categories
		 WHERE plan_id = $1 AND id = ANY($2)`,
		planID, categoryIDs,
	).Scan(&count)
	if err != nil {
		return err
	}

	if count != len(categoryIDs) {
		return ErrInvalid
	}

	var total int
	err = tx.QueryRow(ctx,
		`SELECT COUNT(*)
		 FROM expense_categories
		 WHERE plan_id = $1`,
		planID,
	).Scan(&total)
	if err != nil {
		return err
	}

	if total != len(categoryIDs) {
		return ErrInvalid
	}

	cmd, err := tx.Exec(ctx,
		`UPDATE expense_categories AS c
		 SET sort_order = v.ord - 1
		 FROM unnest($1::uuid[]) WITH ORDINALITY AS v(id, ord)
		 WHERE c.id = v.id AND c.plan_id = $2`,
		categoryIDs, planID,
	)
	if err != nil {
		return err
	}

	if cmd.RowsAffected() != int64(len(categoryIDs)) {
		return ErrNotFound
	}

	return tx.Commit(ctx)
}

// Duplicate создает полную копию плана с категориями и заметками.
func (r *PlanRepository) Duplicate(ctx context.Context, userID, planID uuid.UUID) (models.BudgetPlan, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return models.BudgetPlan{}, err
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	var original models.BudgetPlan
	err = tx.QueryRow(ctx,
		`SELECT id, user_id, title, budget_cents, period_start, period_end, background_color, is_ai_generated, created_at, updated_at
		 FROM budget_plans
		 WHERE id = $1 AND user_id = $2`,
		planID, userID,
	).Scan(&original.ID, &original.UserID, &original.Title, &original.BudgetCents, &original.PeriodStart, &original.PeriodEnd, &original.BackgroundColor, &original.IsAIGenerated, &original.CreatedAt, &original.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return models.BudgetPlan{}, ErrNotFound
		}
		return models.BudgetPlan{}, err
	}

	newTitle := buildCopyTitle(original.Title, 200)

	var newPlan models.BudgetPlan
	err = tx.QueryRow(ctx,
		`INSERT INTO budget_plans (id, user_id, title, budget_cents, period_start, period_end, background_color, is_ai_generated)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		 RETURNING id, user_id, title, budget_cents, period_start, period_end, background_color, is_ai_generated, created_at, updated_at`,
		uuid.New(), userID, newTitle, original.BudgetCents, original.PeriodStart, original.PeriodEnd, original.BackgroundColor, original.IsAIGenerated,
	).Scan(&newPlan.ID, &newPlan.UserID, &newPlan.Title, &newPlan.BudgetCents, &newPlan.PeriodStart, &newPlan.PeriodEnd, &newPlan.BackgroundColor, &newPlan.IsAIGenerated, &newPlan.CreatedAt, &newPlan.UpdatedAt)
	if err != nil {
		return models.BudgetPlan{}, err
	}

	categoryRows, err := tx.Query(ctx,
		`SELECT id, title, category_type, sort_order
		 FROM expense_categories
		 WHERE plan_id = $1
		 ORDER BY sort_order, created_at`,
		planID,
	)
	if err != nil {
		return models.BudgetPlan{}, err
	}
	defer categoryRows.Close()

	categoryMap := make(map[uuid.UUID]uuid.UUID)
	oldCategoryIDs := make([]uuid.UUID, 0)

	for categoryRows.Next() {
		var oldID uuid.UUID
		var title string
		var categoryType models.CategoryType
		var sortOrder int

		err = categoryRows.Scan(&oldID, &title, &categoryType, &sortOrder)
		if err != nil {
			return models.BudgetPlan{}, err
		}

		newID := uuid.New()
		categoryMap[oldID] = newID
		oldCategoryIDs = append(oldCategoryIDs, oldID)

		_, err = tx.Exec(ctx,
			`INSERT INTO expense_categories (id, plan_id, title, category_type, sort_order)
			 VALUES ($1, $2, $3, $4, $5)`,
			newID, newPlan.ID, title, categoryType, sortOrder,
		)
		if err != nil {
			return models.BudgetPlan{}, err
		}
	}

	if err = categoryRows.Err(); err != nil {
		return models.BudgetPlan{}, err
	}

	if len(oldCategoryIDs) > 0 {
		var itemRows pgx.Rows
		itemRows, err = tx.Query(ctx,
			`SELECT category_id, title, amount_cents, priority_color, is_completed, sort_order
			 FROM expense_items
			 WHERE category_id = ANY($1)
			 ORDER BY sort_order, created_at`,
			oldCategoryIDs,
		)
		if err != nil {
			return models.BudgetPlan{}, err
		}
		defer itemRows.Close()

		for itemRows.Next() {
			var oldCategoryID uuid.UUID
			var title string
			var amountCents int64
			var priorityColor models.PriorityColor
			var isCompleted bool
			var sortOrder int

			err = itemRows.Scan(&oldCategoryID, &title, &amountCents, &priorityColor, &isCompleted, &sortOrder)
			if err != nil {
				return models.BudgetPlan{}, err
			}

			newCategoryID, ok := categoryMap[oldCategoryID]
			if !ok {
				return models.BudgetPlan{}, fmt.Errorf("missing category mapping")
			}

			_, err = tx.Exec(ctx,
				`INSERT INTO expense_items (id, category_id, title, amount_cents, priority_color, is_completed, sort_order)
				 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
				uuid.New(), newCategoryID, title, amountCents, priorityColor, isCompleted, sortOrder,
			)
			if err != nil {
				return models.BudgetPlan{}, err
			}
		}

		if err = itemRows.Err(); err != nil {
			return models.BudgetPlan{}, err
		}
	}

	noteRows, err := tx.Query(ctx,
		`SELECT content, note_type, sort_order
		 FROM notes
		 WHERE plan_id = $1
		 ORDER BY sort_order, created_at`,
		planID,
	)
	if err != nil {
		return models.BudgetPlan{}, err
	}
	defer noteRows.Close()

	for noteRows.Next() {
		var content string
		var noteType models.NoteType
		var sortOrder int

		err := noteRows.Scan(&content, &noteType, &sortOrder)
		if err != nil {
			return models.BudgetPlan{}, err
		}

		_, err = tx.Exec(ctx,
			`INSERT INTO notes (id, plan_id, content, note_type, sort_order)
			 VALUES ($1, $2, $3, $4, $5)`,
			uuid.New(), newPlan.ID, content, noteType, sortOrder,
		)
		if err != nil {
			return models.BudgetPlan{}, err
		}
	}

	if err := noteRows.Err(); err != nil {
		return models.BudgetPlan{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return models.BudgetPlan{}, err
	}

	return newPlan, nil
}

func buildCopyTitle(title string, maxRunes int) string {
	copyTitle := fmt.Sprintf("Copy of %s", title)
	if len([]rune(copyTitle)) <= maxRunes {
		return copyTitle
	}

	runes := []rune(copyTitle)
	return string(runes[:maxRunes])
}
