package configcanon

import (
	"encoding/json"
	"math"
	"testing"
)

func TestMapOrderDeterminism(t *testing.T) {
	a := map[string]any{"b": 2, "a": 1}
	b := map[string]any{"a": 1, "b": 2}
	canonA, digestA, err := NormalizeConfig(a)
	if err != nil {
		t.Fatalf("NormalizeConfig(a): %v", err)
	}
	canonB, digestB, err := NormalizeConfig(b)
	if err != nil {
		t.Fatalf("NormalizeConfig(b): %v", err)
	}
	if string(canonA) != string(canonB) {
		t.Fatalf("canonical mismatch: %s vs %s", canonA, canonB)
	}
	if digestA != digestB {
		t.Fatalf("digest mismatch: %s vs %s", digestA, digestB)
	}
}

func TestNestedObjectSorting(t *testing.T) {
	input := map[string]any{
		"z": map[string]any{"b": 2, "a": 1},
		"a": []any{3, 2, 1},
	}
	canon, _, err := NormalizeConfig(input)
	if err != nil {
		t.Fatalf("NormalizeConfig: %v", err)
	}
	expected := `{"a":[3,2,1],"z":{"a":1,"b":2}}`
	if string(canon) != expected {
		t.Fatalf("expected %s got %s", expected, canon)
	}
}

func TestArrayOrderingPreserved(t *testing.T) {
	input := []any{1, 2, 3}
	canon, _, err := NormalizeConfig(input)
	if err != nil {
		t.Fatalf("NormalizeConfig: %v", err)
	}
	if string(canon) != "[1,2,3]" {
		t.Fatalf("unexpected canonical array: %s", canon)
	}
}

func TestTypeStabilityStringVsNumber(t *testing.T) {
	a := map[string]any{"v": "1"}
	b := map[string]any{"v": 1}
	_, digestA, err := NormalizeConfig(a)
	if err != nil {
		t.Fatalf("NormalizeConfig(a): %v", err)
	}
	_, digestB, err := NormalizeConfig(b)
	if err != nil {
		t.Fatalf("NormalizeConfig(b): %v", err)
	}
	if digestA == digestB {
		t.Fatalf("expected different digests for string vs number")
	}
}

func TestInvalidFloatRejected(t *testing.T) {
	if _, _, err := NormalizeConfig(math.NaN()); err == nil {
		t.Fatalf("expected error for NaN")
	}
}

func TestMapStringStringConvenience(t *testing.T) {
	m := map[string]string{"b": "2", "a": "1"}
	canon, _, err := NormalizeConfig(m)
	if err != nil {
		t.Fatalf("NormalizeConfig: %v", err)
	}
	expected := `{"a":"1","b":"2"}`
	if string(canon) != expected {
		t.Fatalf("expected %s got %s", expected, canon)
	}
}

func TestExampleCanonicalJSON(t *testing.T) {
	input := map[string]any{
		"name":     "gateway",
		"replicas": 2,
		"tags":     []any{"blue", "prod"},
		"config": map[string]any{
			"port": 80,
			"tls":  map[string]any{"enabled": false},
		},
	}
	canon, _, err := NormalizeConfig(input)
	if err != nil {
		t.Fatalf("NormalizeConfig: %v", err)
	}
	expected := `{"config":{"port":80,"tls":{"enabled":false}},"name":"gateway","replicas":2,"tags":["blue","prod"]}`
	if string(canon) != expected {
		t.Fatalf("expected %s got %s", expected, canon)
	}
}

func TestInvalidJSONNumber(t *testing.T) {
	num := json.Number("notanumber")
	if _, _, err := NormalizeConfig(num); err == nil {
		t.Fatalf("expected error for invalid json.Number")
	}
}
