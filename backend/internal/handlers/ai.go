package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"example.com/ai-budget-planner/backend/internal/ai"
	"example.com/ai-budget-planner/backend/internal/auth"
	"example.com/ai-budget-planner/backend/internal/models"
	"example.com/ai-budget-planner/backend/internal/notifications"
	"example.com/ai-budget-planner/backend/internal/repository"
)

const (
	aiRequestGeneratePlan    = "generate_plan"
	aiRequestAnalyzeSpending = "analyze_spending"
)

type AIHandler struct {
	Service  *ai.Service
	Plans    *repository.PlanRepository
	Notes    *repository.NoteRepository
	AIRepo   *repository.AIRepository
	Notifier *notifications.Hub
	Provider string
	Model    string
}

// NewAIHandler создает обработчик AI-запросов.
func NewAIHandler(service *ai.Service, plans *repository.PlanRepository, notes *repository.NoteRepository, aiRepo *repository.AIRepository, notifier *notifications.Hub, provider, model string) *AIHandler {
	return &AIHandler{
		Service:  service,
		Plans:    plans,
		Notes:    notes,
		AIRepo:   aiRepo,
		Notifier: notifier,
		Provider: provider,
		Model:    model,
	}
}

type GeneratePlanRequest struct {
	PeriodStart string            `json:"period_start" validate:"required"`
	PeriodEnd   string            `json:"period_end" validate:"required"`
	BudgetCents int64             `json:"budget_cents" validate:"gt=0"`
	Currency    string            `json:"currency"`
	UserData    AIUserDataRequest `json:"user_data"`
}

type AIUserDataRequest struct {
	Period            string           `json:"period"`
	Income            []AIIncomeSource `json:"income"`
	MandatoryExpenses []AIExpenseItem  `json:"mandatory_expenses"`
	OptionalExpenses  []AIExpenseItem  `json:"optional_expenses"`
	Assets            []AIAssetItem    `json:"assets"`
	Debts             []AIDebtItem     `json:"debts"`
	Notes             string           `json:"additional_notes"`
}

type AIIncomeSource struct {
	Source      string `json:"source"`
	AmountCents int64  `json:"amount_cents"`
}

type AIExpenseItem struct {
	Title       string `json:"title"`
	AmountCents int64  `json:"amount_cents"`
}

type AIAssetItem struct {
	Title       string `json:"title"`
	AmountCents int64  `json:"amount_cents"`
}

type AIDebtItem struct {
	Title       string `json:"title"`
	AmountCents int64  `json:"amount_cents"`
}

type AnalyzeSpendingRequest struct {
	PlanID   string `json:"plan_id" validate:"required"`
	Currency string `json:"currency"`
}

// GeneratePlan создает план бюджета на основе данных пользователя.
func (h *AIHandler) GeneratePlan(c echo.Context) error {
	userID, ok := auth.UserIDFromContext(c)
	if !ok {
		return unauthorized(c)
	}

	var req GeneratePlanRequest
	if err := c.Bind(&req); err != nil {
		return badRequest(c, "invalid payload")
	}
	if err := c.Validate(&req); err != nil {
		return badRequest(c, "validation failed")
	}

	periodStart, periodEnd, err := parsePeriod(req.PeriodStart, req.PeriodEnd)
	if err != nil {
		return badRequest(c, err.Error())
	}

	currency := strings.TrimSpace(req.Currency)
	if currency == "" {
		currency = "RUB"
	}

	input := ai.GeneratePlanInput{
		PeriodStart: req.PeriodStart,
		PeriodEnd:   req.PeriodEnd,
		BudgetCents: req.BudgetCents,
		Currency:    currency,
		UserData: ai.UserData{
			Period:            req.UserData.Period,
			Income:            toAIIncome(req.UserData.Income),
			MandatoryExpenses: toAIExpenses(req.UserData.MandatoryExpenses),
			OptionalExpenses:  toAIExpenses(req.UserData.OptionalExpenses),
			Assets:            toAIAssets(req.UserData.Assets),
			Debts:             toAIDebts(req.UserData.Debts),
			Notes:             req.UserData.Notes,
		},
	}

	inputPayload, _ := json.Marshal(input)
	if err := h.storeInputData(c.Request().Context(), userID, req); err != nil {
		return serverError(c)
	}

	aiResponse, prompt, raw, err := h.Service.GeneratePlan(c.Request().Context(), input)
	responsePayload := []byte(nil)
	if err == nil {
		responsePayload, _ = json.Marshal(aiResponse)
	}

	if err != nil {
		h.logAIRequest(c.Request().Context(), userID, aiRequestGeneratePlan, prompt, inputPayload, responsePayload, raw, err)

		plan, fallbackErr := h.createFallbackPlan(c.Request().Context(), userID, periodStart, periodEnd, req.BudgetCents)
		if fallbackErr != nil {
			return serverError(c)
		}
		logPlanSource("fallback", plan.ID, userID)

		response, buildErr := buildPlanDetailResponse(c.Request().Context(), h.Plans, plan)
		if buildErr != nil {
			return serverError(c)
		}

		publishBudgetUpdate(h.Notifier, userID, plan.ID, response.Plan.SpentCents, response.Plan.RemainingCents)
		return c.JSON(http.StatusCreated, response)
	}

	categories, notes, mapErr := mapAIPlan(aiResponse)
	if mapErr != nil {
		h.logAIRequest(c.Request().Context(), userID, aiRequestGeneratePlan, prompt, inputPayload, responsePayload, raw, mapErr)

		plan, fallbackErr := h.createFallbackPlan(c.Request().Context(), userID, periodStart, periodEnd, req.BudgetCents)
		if fallbackErr != nil {
			return serverError(c)
		}
		logPlanSource("fallback", plan.ID, userID)

		response, buildErr := buildPlanDetailResponse(c.Request().Context(), h.Plans, plan)
		if buildErr != nil {
			return serverError(c)
		}

		publishBudgetUpdate(h.Notifier, userID, plan.ID, response.Plan.SpentCents, response.Plan.RemainingCents)
		return c.JSON(http.StatusCreated, response)
	}

	plan, err := h.Plans.CreateWithDetails(c.Request().Context(), userID, aiResponse.Plan.Title, req.BudgetCents, periodStart, periodEnd, defaultBackgroundColor, true, categories, notes)
	if err != nil {
		h.logAIRequest(c.Request().Context(), userID, aiRequestGeneratePlan, prompt, inputPayload, responsePayload, raw, err)

		if errors.Is(err, repository.ErrBudgetExceeded) || errors.Is(err, repository.ErrInvalid) {
			plan, fallbackErr := h.createFallbackPlan(c.Request().Context(), userID, periodStart, periodEnd, req.BudgetCents)
			if fallbackErr != nil {
				return serverError(c)
			}
			logPlanSource("fallback", plan.ID, userID)

			response, buildErr := buildPlanDetailResponse(c.Request().Context(), h.Plans, plan)
			if buildErr != nil {
				return serverError(c)
			}

			publishBudgetUpdate(h.Notifier, userID, plan.ID, response.Plan.SpentCents, response.Plan.RemainingCents)
			return c.JSON(http.StatusCreated, response)
		}
		return serverError(c)
	}

	h.logAIRequest(c.Request().Context(), userID, aiRequestGeneratePlan, prompt, inputPayload, responsePayload, raw, nil)
	logPlanSource("ai", plan.ID, userID)

	response, err := buildPlanDetailResponse(c.Request().Context(), h.Plans, plan)
	if err != nil {
		return serverError(c)
	}

	publishBudgetUpdate(h.Notifier, userID, plan.ID, response.Plan.SpentCents, response.Plan.RemainingCents)
	return c.JSON(http.StatusCreated, response)
}

// AnalyzeSpending запрашивает у AI советы по расходам.
func (h *AIHandler) AnalyzeSpending(c echo.Context) error {
	userID, ok := auth.UserIDFromContext(c)
	if !ok {
		return unauthorized(c)
	}

	var req AnalyzeSpendingRequest
	if err := c.Bind(&req); err != nil {
		return badRequest(c, "invalid payload")
	}
	if err := c.Validate(&req); err != nil {
		return badRequest(c, "validation failed")
	}

	planID, err := uuid.Parse(req.PlanID)
	if err != nil {
		return badRequest(c, "invalid plan id")
	}

	plan, err := h.Plans.GetByID(c.Request().Context(), userID, planID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return notFound(c, "plan not found")
		}
		return serverError(c)
	}

	currency := strings.TrimSpace(req.Currency)
	if currency == "" {
		currency = "RUB"
	}

	categories, err := h.Plans.ListCategories(c.Request().Context(), plan.ID)
	if err != nil {
		return serverError(c)
	}

	categoryIDs := make([]uuid.UUID, 0, len(categories))
	for _, category := range categories {
		categoryIDs = append(categoryIDs, category.ID)
	}

	items, err := h.Plans.ListItemsByCategoryIDs(c.Request().Context(), categoryIDs)
	if err != nil {
		return serverError(c)
	}

	categoryIndex := make(map[uuid.UUID]int, len(categories))
	categorySnapshots := make([]ai.CategorySnapshot, 0, len(categories))
	for _, category := range categories {
		categoryIndex[category.ID] = len(categorySnapshots)
		categorySnapshots = append(categorySnapshots, ai.CategorySnapshot{
			Title: category.Title,
			Type:  string(category.CategoryType),
			Items: []ai.ItemSnapshot{},
		})
	}

	for _, item := range items {
		index, ok := categoryIndex[item.CategoryID]
		if !ok {
			continue
		}
		categorySnapshots[index].Items = append(categorySnapshots[index].Items, ai.ItemSnapshot{
			Title:       item.Title,
			AmountCents: item.AmountCents,
			Priority:    string(item.PriorityColor),
			IsCompleted: item.IsCompleted,
		})
	}

	input := ai.AnalyzeSpendingInput{
		PlanTitle:   plan.Title,
		BudgetCents: plan.BudgetCents,
		Currency:    currency,
		Categories:  categorySnapshots,
	}

	inputPayload, _ := json.Marshal(input)
	aiResponse, prompt, raw, err := h.Service.AnalyzeSpending(c.Request().Context(), input)
	responsePayload := []byte(nil)
	if err == nil {
		responsePayload, _ = json.Marshal(aiResponse)
	}

	h.logAIRequest(c.Request().Context(), userID, aiRequestAnalyzeSpending, prompt, inputPayload, responsePayload, raw, err)

	advices := aiResponse.Advices
	if err != nil {
		advices = fallbackAdvices()
		slog.Warn("ai advices fallback used", slog.String("plan_id", plan.ID.String()), slog.String("user_id", userID.String()))
	}
	if err == nil {
		slog.Info("ai advices generated", slog.String("plan_id", plan.ID.String()), slog.String("user_id", userID.String()))
	}

	if err := h.Notes.DeleteByPlanAndType(c.Request().Context(), userID, plan.ID, models.NoteTypeAI); err != nil && !errors.Is(err, repository.ErrNotFound) {
		return serverError(c)
	}

	noteResponses := make([]NoteResponse, 0, len(advices))
	for _, advice := range advices {
		noteType := models.NoteTypeAI
		if strings.TrimSpace(advice.Type) == string(models.NoteTypeUser) {
			noteType = models.NoteTypeUser
		}

		note, err := h.Notes.Create(c.Request().Context(), userID, plan.ID, advice.Content, noteType)
		if err != nil {
			return serverError(c)
		}
		noteResponses = append(noteResponses, toNoteResponse(note))
	}

	publishAdviceUpdate(h.Notifier, userID, plan.ID, len(noteResponses))
	return c.JSON(http.StatusOK, map[string][]NoteResponse{"advices": noteResponses})
}

// GetAdvices возвращает сохраненные AI-советы по плану.
func (h *AIHandler) GetAdvices(c echo.Context) error {
	userID, ok := auth.UserIDFromContext(c)
	if !ok {
		return unauthorized(c)
	}

	planID, err := uuid.Parse(c.Param("planId"))
	if err != nil {
		return badRequest(c, "invalid plan id")
	}

	notes, err := h.Notes.ListByPlanAndType(c.Request().Context(), userID, planID, models.NoteTypeAI)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return notFound(c, "plan not found")
		}
		return serverError(c)
	}

	response := make([]NoteResponse, 0, len(notes))
	for _, note := range notes {
		response = append(response, toNoteResponse(note))
	}

	return c.JSON(http.StatusOK, map[string][]NoteResponse{"advices": response})
}

func (h *AIHandler) storeInputData(ctx context.Context, userID uuid.UUID, req GeneratePlanRequest) error {
	income, err := json.Marshal(req.UserData.Income)
	if err != nil {
		return err
	}

	mandatory, err := json.Marshal(req.UserData.MandatoryExpenses)
	if err != nil {
		return err
	}

	optional, err := json.Marshal(req.UserData.OptionalExpenses)
	if err != nil {
		return err
	}

	assets, err := json.Marshal(req.UserData.Assets)
	if err != nil {
		return err
	}

	debts, err := json.Marshal(req.UserData.Debts)
	if err != nil {
		return err
	}

	period := strings.TrimSpace(req.UserData.Period)
	if period == "" {
		period = fmt.Sprintf("%s - %s", req.PeriodStart, req.PeriodEnd)
	}

	notes := strings.TrimSpace(req.UserData.Notes)
	var notesPtr *string
	if notes != "" {
		notesPtr = &notes
	}

	return h.AIRepo.SaveInputData(ctx, userID, &period, income, mandatory, optional, assets, debts, notesPtr)
}

func (h *AIHandler) logAIRequest(ctx context.Context, userID uuid.UUID, requestType string, prompt string, requestPayload, responsePayload []byte, raw []byte, err error) {
	log := repository.AIRequestLog{
		UserID:          userID,
		RequestType:     requestType,
		Provider:        h.Provider,
		Model:           h.Model,
		Prompt:          prompt,
		RequestPayload:  requestPayload,
		ResponsePayload: responsePayload,
		RawResponse:     string(raw),
		Success:         err == nil,
	}
	if err != nil {
		errMsg := err.Error()
		log.ErrorMessage = &errMsg
	}

	_ = h.AIRepo.LogRequest(ctx, log)
}

func (h *AIHandler) createFallbackPlan(ctx context.Context, userID uuid.UUID, periodStart, periodEnd time.Time, budgetCents int64) (models.BudgetPlan, error) {
	title := fmt.Sprintf("Бюджетный план %s - %s", periodStart.Format(dateLayout), periodEnd.Format(dateLayout))
	plan, err := h.Plans.Create(ctx, userID, title, budgetCents, periodStart, periodEnd, defaultBackgroundColor, false)
	if err != nil {
		return plan, err
	}

	_, _ = h.Notes.Create(ctx, userID, plan.ID, "Ошибка AI генерации. Создан шаблонный план.", models.NoteTypeAI)
	return plan, nil
}

func mapAIPlan(response ai.PlanResponse) ([]repository.PlanCategoryInput, []repository.PlanNoteInput, error) {
	categories := make([]repository.PlanCategoryInput, 0, len(response.Plan.Categories))
	for _, category := range response.Plan.Categories {
		categoryType, ok := mapCategoryType(category.Type)
		if !ok {
			return nil, nil, errors.New("invalid category type")
		}

		items := make([]repository.PlanItemInput, 0, len(category.Items))
		for _, item := range category.Items {
			priority, ok := mapPriority(item.Priority)
			if !ok {
				return nil, nil, errors.New("invalid priority")
			}

			items = append(items, repository.PlanItemInput{
				Title:         item.Title,
				AmountCents:   item.AmountCents,
				PriorityColor: priority,
				IsCompleted:   false,
			})
		}

		categories = append(categories, repository.PlanCategoryInput{
			Title:        category.Title,
			CategoryType: categoryType,
			Items:        items,
		})
	}

	notes := make([]repository.PlanNoteInput, 0, len(response.Plan.Notes))
	for _, note := range response.Plan.Notes {
		noteType, ok := mapNoteType(note.Type)
		if !ok {
			return nil, nil, errors.New("invalid note type")
		}

		notes = append(notes, repository.PlanNoteInput{
			Content:  note.Content,
			NoteType: noteType,
		})
	}

	return categories, notes, nil
}

func mapCategoryType(value string) (models.CategoryType, bool) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case string(models.CategoryTypeMandatory):
		return models.CategoryTypeMandatory, true
	case string(models.CategoryTypeOptional):
		return models.CategoryTypeOptional, true
	default:
		return "", false
	}
}

func mapPriority(value string) (models.PriorityColor, bool) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case string(models.PriorityColorRed):
		return models.PriorityColorRed, true
	case string(models.PriorityColorYellow):
		return models.PriorityColorYellow, true
	case string(models.PriorityColorGreen):
		return models.PriorityColorGreen, true
	default:
		return "", false
	}
}

func mapNoteType(value string) (models.NoteType, bool) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case string(models.NoteTypeAI):
		return models.NoteTypeAI, true
	case string(models.NoteTypeUser):
		return models.NoteTypeUser, true
	default:
		return "", false
	}
}

func toAIIncome(values []AIIncomeSource) []ai.IncomeSource {
	out := make([]ai.IncomeSource, 0, len(values))
	for _, value := range values {
		out = append(out, ai.IncomeSource{Source: value.Source, AmountCents: value.AmountCents})
	}
	return out
}

func toAIExpenses(values []AIExpenseItem) []ai.Expense {
	out := make([]ai.Expense, 0, len(values))
	for _, value := range values {
		out = append(out, ai.Expense{Title: value.Title, AmountCents: value.AmountCents})
	}
	return out
}

func toAIAssets(values []AIAssetItem) []ai.Asset {
	out := make([]ai.Asset, 0, len(values))
	for _, value := range values {
		out = append(out, ai.Asset{Title: value.Title, AmountCents: value.AmountCents})
	}
	return out
}

func toAIDebts(values []AIDebtItem) []ai.Debt {
	out := make([]ai.Debt, 0, len(values))
	for _, value := range values {
		out = append(out, ai.Debt{Title: value.Title, AmountCents: value.AmountCents})
	}
	return out
}

func fallbackAdvices() []ai.Note {
	return []ai.Note{
		{Content: "Пересмотрите обязательные и необязательные расходы и ограничьте лишние траты.", Type: string(models.NoteTypeAI)},
		{Content: "Держите резерв 5-10% бюджета на непредвиденные расходы.", Type: string(models.NoteTypeAI)},
		{Content: "Отмечайте выполненные расходы еженедельно, чтобы вовремя заметить перерасход.", Type: string(models.NoteTypeAI)},
	}
}

func logPlanSource(source string, planID, userID uuid.UUID) {
	switch source {
	case "fallback":
		slog.Warn("ai plan fallback used", slog.String("plan_id", planID.String()), slog.String("user_id", userID.String()))
	default:
		slog.Info("ai plan generated", slog.String("plan_id", planID.String()), slog.String("user_id", userID.String()))
	}
}
