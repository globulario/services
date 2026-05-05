package ingress

import (
	"encoding/json"
	"testing"
)

// TestSpecExplicitDisabledField verifies that Spec round-trips ExplicitDisabled
// correctly for both true and false values.
func TestSpecExplicitDisabledField(t *testing.T) {
	t.Run("explicit_disabled=true", func(t *testing.T) {
		s := Spec{
			Version:          "v1",
			Mode:             ModeDisabled,
			ExplicitDisabled: true,
		}
		b, err := json.Marshal(s)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		var s2 Spec
		if err := json.Unmarshal(b, &s2); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if !s2.ExplicitDisabled {
			t.Error("ExplicitDisabled=true must survive JSON round-trip")
		}
	})

	t.Run("explicit_disabled=false (omitempty — field absent in JSON)", func(t *testing.T) {
		s := Spec{
			Version:          "v1",
			Mode:             ModeDisabled,
			ExplicitDisabled: false,
		}
		b, err := json.Marshal(s)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		// With omitempty, false is omitted from JSON.
		// Deserializing must produce ExplicitDisabled=false, not true.
		var s2 Spec
		if err := json.Unmarshal(b, &s2); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if s2.ExplicitDisabled {
			t.Error("ExplicitDisabled must be false when field is absent in JSON")
		}
	})
}

// TestIsExplicitDisable verifies the shared guard for Case 11
// (UNGUARDED_RUNTIME_DESTRUCTIVE_ACTION). Ambiguous disables must not stop
// keepalived — only fully-qualified intents are allowed to be destructive.
func TestIsExplicitDisable(t *testing.T) {
	cases := []struct {
		name string
		spec Spec
		want bool
	}{
		{
			name: "fully qualified disable returns true",
			spec: Spec{Mode: ModeDisabled, ExplicitDisabled: true, Reason: "operator: maintenance", Generation: 1},
			want: true,
		},
		{
			name: "missing reason returns false",
			spec: Spec{Mode: ModeDisabled, ExplicitDisabled: true, Reason: "", Generation: 1},
			want: false,
		},
		{
			name: "zero generation returns false",
			spec: Spec{Mode: ModeDisabled, ExplicitDisabled: true, Reason: "maintenance", Generation: 0},
			want: false,
		},
		{
			name: "explicit_disabled=false returns false",
			spec: Spec{Mode: ModeDisabled, ExplicitDisabled: false, Reason: "maintenance", Generation: 1},
			want: false,
		},
		{
			name: "vip_failover mode returns false",
			spec: Spec{Mode: ModeVIPFailover, ExplicitDisabled: true, Reason: "maintenance", Generation: 1},
			want: false,
		},
		{
			name: "empty spec returns false",
			spec: Spec{},
			want: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.spec.IsExplicitDisable()
			if got != tc.want {
				t.Errorf("IsExplicitDisable() = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestSpecModeHoldSafe verifies that a spec with mode=disabled and
// explicit_disabled=false deserializes without ExplicitDisabled becoming true.
// This exercises the hold-safe vs stop-keepalived distinction.
func TestSpecModeHoldSafe(t *testing.T) {
	// Simulate what the controller publishes when no backup exists:
	// mode=disabled, explicit_disabled absent (omitempty).
	raw := `{"version":"v1","mode":"disabled"}`
	var spec Spec
	if err := json.Unmarshal([]byte(raw), &spec); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if spec.Mode != ModeDisabled {
		t.Errorf("mode: got %v, want %v", spec.Mode, ModeDisabled)
	}
	if spec.ExplicitDisabled {
		t.Error("ExplicitDisabled must be false when field is absent — this spec should trigger hold-safe, not stop-keepalived")
	}
}
