package policy

import (
	"fmt"
	"strings"
)

// ExpandTemplate replaces {field} placeholders in a resource template with
// values from a field map. Returns the expanded path and any error.
//
// Example:
//
//	ExpandTemplate("/catalog/connections/{connectionId}/items/{itemId}",
//	    map[string]string{"connectionId": "conn-123", "itemId": "item-456"})
//	→ "/catalog/connections/conn-123/items/item-456", nil
//
// If a required placeholder field is missing or empty, returns an error.
// This is intentionally strict — missing fields must not silently produce
// incomplete resource paths.
func ExpandTemplate(template string, fields map[string]string) (string, error) {
	if template == "" {
		return "", nil
	}

	result := template
	for {
		start := strings.Index(result, "{")
		if start == -1 {
			break
		}
		end := strings.Index(result[start:], "}")
		if end == -1 {
			return "", fmt.Errorf("unclosed placeholder in template %q", template)
		}
		end += start

		fieldName := result[start+1 : end]
		value, ok := fields[fieldName]
		if !ok || value == "" {
			return "", fmt.Errorf("missing required field %q for resource template %q", fieldName, template)
		}

		result = result[:start] + value + result[end+1:]
	}

	return result, nil
}

// ExtractTemplatePlaceholders returns the field names from a resource template.
// Example: "/catalog/connections/{connectionId}/items/{itemId}" → ["connectionId", "itemId"]
func ExtractTemplatePlaceholders(template string) []string {
	var fields []string
	for {
		start := strings.Index(template, "{")
		if start == -1 {
			break
		}
		end := strings.Index(template[start:], "}")
		if end == -1 {
			break
		}
		end += start
		fields = append(fields, template[start+1:end])
		template = template[end+1:]
	}
	return fields
}
