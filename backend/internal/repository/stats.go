package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"example.com/ai-budget-planner/backend/internal/models"
)

type StatsRepository struct {
	db *pgxpool.Pool
}

type OverviewStats struct {
	TotalPlans       int
	ActivePlans      int
	ArchivedPlans    int
	TotalBudgetCents int64
	TotalSpentCents  int64
}

type CategorySpend struct {
	CategoryID   uuid.UUID
	Title        string
	CategoryType models.CategoryType
	SpentCents   int64
}

type MonthlyComparison struct {
	Month       time.Time
	BudgetCents int64
	SpentCents  int64
}

// NewStatsRepository создает репозиторий статистики.
func NewStatsRepository(db *pgxpool.Pool) *StatsRepository {
	return &StatsRepository{db: db}
}

// Overview возвращает сводную статистику по планам пользователя.
func (r *StatsRepository) Overview(ctx context.Context, userID uuid.UUID) (OverviewStats, error) {
	var stats OverviewStats

	err := r.db.QueryRow(ctx,
		`SELECT COUNT(*) AS total_plans,
		        COUNT(*) FILTER (WHERE period_end >= CURRENT_DATE) AS active_plans,
		        COUNT(*) FILTER (WHERE period_end < CURRENT_DATE) AS archived_plans,
		        COALESCE(SUM(budget_cents), 0) AS total_budget_cents
		 FROM budget_plans
		 WHERE user_id = $1`,
		userID,
	).Scan(&stats.TotalPlans, &stats.ActivePlans, &stats.ArchivedPlans, &stats.TotalBudgetCents)
	if err != nil {
		return stats, err
	}

	err = r.db.QueryRow(ctx,
		`SELECT COALESCE(SUM(CASE WHEN i.is_completed THEN i.amount_cents ELSE 0 END), 0)
		 FROM budget_plans p
		 LEFT JOIN expense_categories c ON c.plan_id = p.id
		 LEFT JOIN expense_items i ON i.category_id = c.id
		 WHERE p.user_id = $1`,
		userID,
	).Scan(&stats.TotalSpentCents)
	if err != nil {
		return stats, err
	}

	return stats, nil
}

// SpendingByCategory возвращает траты по категориям в плане.
func (r *StatsRepository) SpendingByCategory(ctx context.Context, userID, planID uuid.UUID) ([]CategorySpend, error) {
	var exists bool
	err := r.db.QueryRow(ctx,
		`SELECT EXISTS (
			SELECT 1 FROM budget_plans WHERE id = $1 AND user_id = $2
		 )`,
		planID, userID,
	).Scan(&exists)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, ErrNotFound
	}

	rows, err := r.db.Query(ctx,
		`SELECT c.id, c.title, c.category_type,
		        COALESCE(SUM(CASE WHEN i.is_completed THEN i.amount_cents ELSE 0 END), 0) AS spent_cents
		 FROM expense_categories c
		 LEFT JOIN expense_items i ON i.category_id = c.id
		 WHERE c.plan_id = $1
		 GROUP BY c.id, c.title, c.category_type, c.sort_order
		 ORDER BY c.sort_order, c.created_at`,
		planID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	spending := make([]CategorySpend, 0)
	for rows.Next() {
		var row CategorySpend
		err := rows.Scan(&row.CategoryID, &row.Title, &row.CategoryType, &row.SpentCents)
		if err != nil {
			return nil, err
		}
		spending = append(spending, row)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return spending, nil
}

// MonthlyComparison возвращает сравнение бюджетов по месяцам.
func (r *StatsRepository) MonthlyComparison(ctx context.Context, userID uuid.UUID, months int) ([]MonthlyComparison, error) {
	if months <= 0 {
		return nil, ErrInvalid
	}

	rows, err := r.db.Query(ctx,
		`WITH plan_spent AS (
			SELECT p.id,
			       date_trunc('month', p.period_start)::date AS month,
			       p.budget_cents,
			       COALESCE(SUM(CASE WHEN i.is_completed THEN i.amount_cents ELSE 0 END), 0) AS spent_cents
			FROM budget_plans p
			LEFT JOIN expense_categories c ON c.plan_id = p.id
			LEFT JOIN expense_items i ON i.category_id = c.id
			WHERE p.user_id = $1
			GROUP BY p.id, month, p.budget_cents
		)
		SELECT month,
		       COALESCE(SUM(budget_cents), 0) AS budget_cents,
		       COALESCE(SUM(spent_cents), 0) AS spent_cents
		FROM plan_spent
		GROUP BY month
		ORDER BY month DESC
		LIMIT $2`,
		userID, months,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]MonthlyComparison, 0)
	for rows.Next() {
		var row MonthlyComparison
		var month time.Time
		err := rows.Scan(&month, &row.BudgetCents, &row.SpentCents)
		if err != nil {
			return nil, err
		}
		row.Month = month
		items = append(items, row)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return items, nil
}
