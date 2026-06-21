package main

import (
	"errors"
	"strings"
	"testing"
)

// TestIsExistingColumnError covers the idempotency guard for the additive
// schema migration (inputs_json). An ALTER TABLE ADD for a column that already
// exists must be tolerated so a restart / fresh cluster does not fail schema
// init, while unrelated errors must still propagate.
func TestIsExistingColumnError(t *testing.T) {
	cases := []struct {
		err  error
		want bool
	}{
		{nil, false},
		{errors.New("Invalid column name inputs_json because it conflicts with an existing column"), true},
		{errors.New("column already exists"), true},
		{errors.New("Duplicate column name inputs_json"), true},
		{errors.New("connection refused"), false},
		{errors.New("keyspace workflow does not exist"), false},
	}
	for _, c := range cases {
		if got := isExistingColumnError(c.err); got != c.want {
			t.Errorf("isExistingColumnError(%v) = %v, want %v", c.err, got, c.want)
		}
	}
}

// TestSchemaAlterStatements_AddsInputsJson pins that the inputs_json column
// migration is wired so upgraded clusters (whose workflow_runs table predates
// the column) actually get it — CREATE TABLE IF NOT EXISTS would not add it.
func TestSchemaAlterStatements_AddsInputsJson(t *testing.T) {
	found := false
	for _, stmt := range schemaAlterStatements {
		s := strings.ToLower(stmt)
		if strings.Contains(s, "alter table") && strings.Contains(s, "workflow_runs") && strings.Contains(s, "inputs_json") {
			found = true
		}
	}
	if !found {
		t.Fatal("schemaAlterStatements must include an ALTER adding inputs_json to workflow_runs")
	}
}
