package main

import (
	"reflect"
	"testing"

	clustercontrollerpb "github.com/globulario/services/golang/clustercontroller/clustercontrollerpb"
)

func TestNormalizeProfiles(t *testing.T) {
	tests := []struct {
		name string
		raw  []string
		want []string
	}{
		{
			name: "dedup, lowercase, trim, sort",
			raw:  []string{"Core", " gateway", "core"},
			want: []string{"core", "gateway"},
		},
		{
			name: "empty list",
			raw:  []string{},
			want: []string{},
		},
		{
			name: "nil list",
			raw:  nil,
			want: []string{},
		},
		{
			name: "single profile",
			raw:  []string{"storage"},
			want: []string{"storage"},
		},
		{
			name: "sort order",
			raw:  []string{"storage", "core", "gateway"},
			want: []string{"core", "gateway", "storage"},
		},
		{
			name: "whitespace only",
			raw:  []string{"  ", "\t"},
			want: []string{},
		},
		{
			name: "uppercase dedup",
			raw:  []string{"CORE", "Core", "core"},
			want: []string{"core"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeProfiles(tt.raw)
			// Treat nil and empty slice as equivalent.
			if len(got) == 0 && len(tt.want) == 0 {
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("normalizeProfiles(%v) = %v, want %v", tt.raw, got, tt.want)
			}
		})
	}
}

// TestPlanHashOrderIndependent verifies that plan hashes are equal for the
// same profile set regardless of input order.
func TestPlanHashOrderIndependent(t *testing.T) {
	profiles1 := []string{"core", "gateway"}
	profiles2 := []string{"gateway", "core"}

	actions1, err1 := buildPlanActions(profiles1)
	actions2, err2 := buildPlanActions(profiles2)
	if err1 != nil || err2 != nil {
		t.Fatalf("unexpected errors: %v, %v", err1, err2)
	}

	plan1 := &clustercontrollerpb.NodePlan{UnitActions: actions1}
	plan2 := &clustercontrollerpb.NodePlan{UnitActions: actions2}

	h1 := planHash(plan1)
	h2 := planHash(plan2)
	if h1 != h2 {
		t.Errorf("planHash should be equal for same profiles in different order: %q != %q", h1, h2)
	}
}

