package handlers

import (
	"testing"

	"example.com/ai-budget-planner/backend/internal/models"
)

// TestMapCategoryType проверяет маппинг типов категорий.
func TestMapCategoryType(t *testing.T) {
	value, ok := mapCategoryType("mandatory")
	if !ok || value != models.CategoryTypeMandatory {
		t.Fatalf("expected mandatory, got %v (ok=%v)", value, ok)
	}

	value, ok = mapCategoryType("optional")
	if !ok || value != models.CategoryTypeOptional {
		t.Fatalf("expected optional, got %v (ok=%v)", value, ok)
	}

	if _, ok := mapCategoryType("other"); ok {
		t.Fatal("expected invalid category type")
	}
}

// TestMapPriority проверяет маппинг приоритетов.
func TestMapPriority(t *testing.T) {
	value, ok := mapPriority("red")
	if !ok || value != models.PriorityColorRed {
		t.Fatalf("expected red, got %v (ok=%v)", value, ok)
	}

	value, ok = mapPriority("yellow")
	if !ok || value != models.PriorityColorYellow {
		t.Fatalf("expected yellow, got %v (ok=%v)", value, ok)
	}

	value, ok = mapPriority("green")
	if !ok || value != models.PriorityColorGreen {
		t.Fatalf("expected green, got %v (ok=%v)", value, ok)
	}

	if _, ok := mapPriority("blue"); ok {
		t.Fatal("expected invalid priority")
	}
}

// TestMapNoteType проверяет маппинг типов заметок.
func TestMapNoteType(t *testing.T) {
	value, ok := mapNoteType("ai")
	if !ok || value != models.NoteTypeAI {
		t.Fatalf("expected ai, got %v (ok=%v)", value, ok)
	}

	value, ok = mapNoteType("user")
	if !ok || value != models.NoteTypeUser {
		t.Fatalf("expected user, got %v (ok=%v)", value, ok)
	}

	if _, ok := mapNoteType("system"); ok {
		t.Fatal("expected invalid note type")
	}
}
