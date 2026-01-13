-- +goose Up
CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email VARCHAR(255) UNIQUE NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    name VARCHAR(100),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE budget_plans (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    title VARCHAR(200) NOT NULL,
    budget_cents BIGINT NOT NULL,
    period_start DATE NOT NULL,
    period_end DATE NOT NULL,
    background_color VARCHAR(7) NOT NULL DEFAULT '#FDF7F7',
    is_ai_generated BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE expense_categories (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    plan_id UUID NOT NULL REFERENCES budget_plans(id) ON DELETE CASCADE,
    title VARCHAR(100) NOT NULL,
    category_type VARCHAR(20) NOT NULL CHECK (category_type IN ('mandatory', 'optional')),
    sort_order INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE expense_items (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    category_id UUID NOT NULL REFERENCES expense_categories(id) ON DELETE CASCADE,
    title VARCHAR(200) NOT NULL,
    amount_cents BIGINT NOT NULL,
    priority_color VARCHAR(20) NOT NULL CHECK (priority_color IN ('red', 'yellow', 'green')),
    is_completed BOOLEAN NOT NULL DEFAULT FALSE,
    sort_order INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE notes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    plan_id UUID NOT NULL REFERENCES budget_plans(id) ON DELETE CASCADE,
    content TEXT NOT NULL,
    note_type VARCHAR(20) NOT NULL CHECK (note_type IN ('ai', 'user')),
    sort_order INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE ai_input_data (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    period VARCHAR(50),
    income JSONB,
    mandatory_expenses JSONB,
    optional_expenses JSONB,
    assets JSONB,
    debts JSONB,
    additional_notes TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_budget_plans_user_id ON budget_plans (user_id);
CREATE INDEX idx_expense_categories_plan_id ON expense_categories (plan_id);
CREATE INDEX idx_expense_items_category_id ON expense_items (category_id);
CREATE INDEX idx_notes_plan_id ON notes (plan_id);
CREATE INDEX idx_ai_input_data_user_id ON ai_input_data (user_id);

-- +goose Down
DROP TABLE IF EXISTS ai_input_data;
DROP TABLE IF EXISTS notes;
DROP TABLE IF EXISTS expense_items;
DROP TABLE IF EXISTS expense_categories;
DROP TABLE IF EXISTS budget_plans;
DROP TABLE IF EXISTS users;

DROP EXTENSION IF EXISTS pgcrypto;
