package ai

type IncomeSource struct {
	Source      string `json:"source"`
	AmountCents int64  `json:"amount_cents"`
}

type Expense struct {
	Title       string `json:"title"`
	AmountCents int64  `json:"amount_cents"`
}

type Asset struct {
	Title       string `json:"title"`
	AmountCents int64  `json:"amount_cents"`
}

type Debt struct {
	Title       string `json:"title"`
	AmountCents int64  `json:"amount_cents"`
}

type UserData struct {
	Period            string         `json:"period,omitempty"`
	Income            []IncomeSource `json:"income,omitempty"`
	MandatoryExpenses []Expense      `json:"mandatory_expenses,omitempty"`
	OptionalExpenses  []Expense      `json:"optional_expenses,omitempty"`
	Assets            []Asset        `json:"assets,omitempty"`
	Debts             []Debt         `json:"debts,omitempty"`
	Notes             string         `json:"additional_notes,omitempty"`
}

type GeneratePlanInput struct {
	PeriodStart string   `json:"period_start"`
	PeriodEnd   string   `json:"period_end"`
	BudgetCents int64    `json:"budget_cents"`
	Currency    string   `json:"currency"`
	UserData    UserData `json:"user_data"`
}

type PlanResponse struct {
	Plan Plan `json:"plan"`
}

type Plan struct {
	Title      string     `json:"title"`
	Categories []Category `json:"categories"`
	Notes      []Note     `json:"notes,omitempty"`
}

type Category struct {
	Title string `json:"title"`
	Type  string `json:"type"`
	Items []Item `json:"items"`
}

type Item struct {
	Title       string `json:"title"`
	AmountCents int64  `json:"amount_cents"`
	Priority    string `json:"priority"`
}

type Note struct {
	Content string `json:"content"`
	Type    string `json:"type"`
}

type AnalyzeSpendingInput struct {
	PlanTitle   string             `json:"plan_title"`
	BudgetCents int64              `json:"budget_cents"`
	Currency    string             `json:"currency"`
	Categories  []CategorySnapshot `json:"categories"`
}

type CategorySnapshot struct {
	Title string         `json:"title"`
	Type  string         `json:"type"`
	Items []ItemSnapshot `json:"items"`
}

type ItemSnapshot struct {
	Title       string `json:"title"`
	AmountCents int64  `json:"amount_cents"`
	Priority    string `json:"priority"`
	IsCompleted bool   `json:"is_completed"`
}

type AdviceResponse struct {
	Advices []Note `json:"advices"`
}
