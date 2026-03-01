package rules

import "testing"

func TestNormalizeUnitState(t *testing.T) {
	cases := []struct {
		input    string
		expected UnitStateEnum
	}{
		{"active", UnitStateActive},
		{"ACTIVE", UnitStateActive},
		{"running", UnitStateActive},
		{"inactive", UnitStateInactive},
		{"dead", UnitStateInactive},
		{"INACTIVE", UnitStateInactive},
		{"failed", UnitStateFailed},
		{"FAILED", UnitStateFailed},
		{"disabled", UnitStateDisabled},
		{"not-found", UnitStateNotFound},
		{"not_found", UnitStateNotFound},
		{"notfound", UnitStateNotFound},
		{"missing", UnitStateNotFound},
		{"activating", UnitStateActivating},
		{"deactivating", UnitStateDeactivating},
		{"", UnitStateUnknown},
		{"bogus-state", UnitStateUnknown},
		{"  active  ", UnitStateActive}, // leading/trailing whitespace
	}

	for _, tc := range cases {
		got := NormalizeUnitState(tc.input)
		if got != tc.expected {
			t.Errorf("NormalizeUnitState(%q) = %v, want %v", tc.input, got, tc.expected)
		}
	}
}
