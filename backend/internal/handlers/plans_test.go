package handlers

import "testing"

// TestParsePeriodValid проверяет корректный разбор периода.
func TestParsePeriodValid(t *testing.T) {
	start, end, err := parsePeriod("2024-01-01", "2024-01-31")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if start.Format(dateLayout) != "2024-01-01" {
		t.Fatalf("unexpected start: %s", start.Format(dateLayout))
	}
	if end.Format(dateLayout) != "2024-01-31" {
		t.Fatalf("unexpected end: %s", end.Format(dateLayout))
	}
}

// TestParsePeriodInvalid проверяет ошибки при неверном периоде.
func TestParsePeriodInvalid(t *testing.T) {
	if _, _, err := parsePeriod("2024/01/01", "2024-01-31"); err == nil {
		t.Fatal("expected error for invalid start format")
	}

	if _, _, err := parsePeriod("2024-02-01", "2024-01-31"); err == nil {
		t.Fatal("expected error for end before start")
	}
}

// TestValidateHexColor проверяет валидацию hex-цвета.
func TestValidateHexColor(t *testing.T) {
	if _, err := validateHexColor("#AABBCC"); err != nil {
		t.Fatalf("expected valid color, got %v", err)
	}

	if _, err := validateHexColor("AABBCC"); err == nil {
		t.Fatal("expected error for missing #")
	}

	if _, err := validateHexColor("#XYZ123"); err == nil {
		t.Fatal("expected error for invalid hex")
	}
}
