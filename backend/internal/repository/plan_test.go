package repository

import (
	"strings"
	"testing"
	"unicode/utf8"
)

// TestBuildCopyTitle проверяет ограничение длины заголовка копии.
func TestBuildCopyTitle(t *testing.T) {
	original := strings.Repeat("a", 210)
	result := buildCopyTitle(original, 200)

	if !strings.HasPrefix(result, "Copy of ") {
		t.Fatalf("expected prefix, got %s", result)
	}

	if utf8.RuneCountInString(result) > 200 {
		t.Fatalf("expected result length <= 200, got %d", utf8.RuneCountInString(result))
	}
}
