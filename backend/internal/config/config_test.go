package config

import (
	"reflect"
	"testing"
)

// TestParseCSVEnv проверяет разбор списка email из ENV.
func TestParseCSVEnv(t *testing.T) {
	t.Setenv("ADMIN_EMAILS", " Admin@example.com, ,USER@Example.com ")

	got := parseCSVEnv("ADMIN_EMAILS")
	want := []string{"admin@example.com", "user@example.com"}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("expected %v, got %v", want, got)
	}
}

// TestParseCSVEnvMissing проверяет поведение при отсутствии переменной.
func TestParseCSVEnvMissing(t *testing.T) {
	got := parseCSVEnv("MISSING_ENV")
	if got != nil {
		t.Fatalf("expected nil, got %v", got)
	}
}
