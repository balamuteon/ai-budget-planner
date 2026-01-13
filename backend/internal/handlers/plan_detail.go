package handlers

import (
	"context"

	"github.com/google/uuid"

	"example.com/ai-budget-planner/backend/internal/models"
	"example.com/ai-budget-planner/backend/internal/repository"
)

func buildPlanDetailResponse(ctx context.Context, plans *repository.PlanRepository, plan models.BudgetPlan) (PlanDetailResponse, error) {
	spent, err := plans.GetSpentCents(ctx, plan.ID)
	if err != nil {
		return PlanDetailResponse{}, err
	}

	categories, err := plans.ListCategories(ctx, plan.ID)
	if err != nil {
		return PlanDetailResponse{}, err
	}

	categoryIDs := make([]uuid.UUID, 0, len(categories))
	for _, category := range categories {
		categoryIDs = append(categoryIDs, category.ID)
	}

	items, err := plans.ListItemsByCategoryIDs(ctx, categoryIDs)
	if err != nil {
		return PlanDetailResponse{}, err
	}

	notes, err := plans.ListNotes(ctx, plan.ID)
	if err != nil {
		return PlanDetailResponse{}, err
	}

	categoryResponses := make([]CategoryResponse, 0, len(categories))
	categoryIndex := make(map[uuid.UUID]int, len(categories))

	for _, category := range categories {
		categoryIndex[category.ID] = len(categoryResponses)
		categoryResponses = append(categoryResponses, CategoryResponse{
			ID:           category.ID,
			Title:        category.Title,
			CategoryType: category.CategoryType,
			SortOrder:    category.SortOrder,
			Items:        []ItemResponse{},
		})
	}

	for _, item := range items {
		index, ok := categoryIndex[item.CategoryID]
		if !ok {
			continue
		}
		categoryResponses[index].Items = append(categoryResponses[index].Items, ItemResponse{
			ID:            item.ID,
			Title:         item.Title,
			AmountCents:   item.AmountCents,
			PriorityColor: item.PriorityColor,
			IsCompleted:   item.IsCompleted,
			SortOrder:     item.SortOrder,
		})
	}

	noteResponses := make([]NoteResponse, 0, len(notes))
	for _, note := range notes {
		noteResponses = append(noteResponses, NoteResponse{
			ID:        note.ID,
			Content:   note.Content,
			NoteType:  note.NoteType,
			SortOrder: note.SortOrder,
			CreatedAt: note.CreatedAt,
			UpdatedAt: note.UpdatedAt,
		})
	}

	return PlanDetailResponse{
		Plan:       toPlanResponse(plan, spent),
		Categories: categoryResponses,
		Notes:      noteResponses,
	}, nil
}
