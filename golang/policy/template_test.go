package policy

import (
	"testing"
)

func TestExpandTemplate_Success(t *testing.T) {
	result, err := ExpandTemplate(
		"/catalog/connections/{connectionId}/item-definitions/{itemDefinitionId}",
		map[string]string{"connectionId": "conn-123", "itemDefinitionId": "item-456"},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := "/catalog/connections/conn-123/item-definitions/item-456"
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestExpandTemplate_CollectionTemplate(t *testing.T) {
	result, err := ExpandTemplate(
		"/catalog/connections/{connectionId}/items",
		map[string]string{"connectionId": "conn-123"},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "/catalog/connections/conn-123/items" {
		t.Errorf("unexpected result: %q", result)
	}
}

func TestExpandTemplate_MissingField(t *testing.T) {
	_, err := ExpandTemplate(
		"/catalog/connections/{connectionId}/items/{itemId}",
		map[string]string{"connectionId": "conn-123"},
	)
	if err == nil {
		t.Fatal("expected error for missing field")
	}
	if !contains(err.Error(), "missing required field") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestExpandTemplate_EmptyField(t *testing.T) {
	_, err := ExpandTemplate(
		"/catalog/connections/{connectionId}",
		map[string]string{"connectionId": ""},
	)
	if err == nil {
		t.Fatal("expected error for empty field value")
	}
}

func TestExpandTemplate_EmptyTemplate(t *testing.T) {
	result, err := ExpandTemplate("", map[string]string{"a": "b"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "" {
		t.Errorf("expected empty string, got %q", result)
	}
}

func TestExpandTemplate_NoPlaceholders(t *testing.T) {
	result, err := ExpandTemplate("/catalog/connections", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "/catalog/connections" {
		t.Errorf("unexpected: %q", result)
	}
}

func TestExtractTemplatePlaceholders(t *testing.T) {
	fields := ExtractTemplatePlaceholders("/catalog/connections/{connectionId}/items/{itemId}")
	if len(fields) != 2 {
		t.Fatalf("expected 2 fields, got %d", len(fields))
	}
	if fields[0] != "connectionId" || fields[1] != "itemId" {
		t.Errorf("unexpected fields: %v", fields)
	}
}

func TestExtractTemplatePlaceholders_None(t *testing.T) {
	fields := ExtractTemplatePlaceholders("/catalog/connections")
	if len(fields) != 0 {
		t.Errorf("expected 0 fields, got %d", len(fields))
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstr(s, substr))
}

func containsSubstr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
