package infra_truth

import "testing"

func TestBuildScyllaDesiredState_Provenance(t *testing.T) {
	ds, err := BuildScyllaDesiredState(ScyllaDesiredInputs{
		NodeID:      "globule-ryzen",
		ClusterID:   "test-cluster",
		LocalIP:     "10.0.0.63",
		ClusterName: "Globular",
		Peers:       []string{"10.0.0.63", "10.0.0.8"},
		Seeds:       []string{"10.0.0.8"},
		Now:         1700000000,
	})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if ds.Source != SourceComputedFromMembership {
		t.Errorf("source=%q want %q", ds.Source, SourceComputedFromMembership)
	}
	if ds.GeneratedAt != 1700000000 {
		t.Errorf("generated_at=%d", ds.GeneratedAt)
	}
	if len(ds.ExpectedListenAddresses) != 1 || ds.ExpectedListenAddresses[0] != "10.0.0.63" {
		t.Errorf("expected_listen=%v", ds.ExpectedListenAddresses)
	}
	// provenance must be visible in the projected map
	if ds.desiredMap()["source"] != SourceComputedFromMembership {
		t.Errorf("desiredMap missing provenance: %+v", ds.desiredMap())
	}
}

func TestBuildScyllaDesiredState_MissingFactsErrors(t *testing.T) {
	if _, err := BuildScyllaDesiredState(ScyllaDesiredInputs{LocalIP: "10.0.0.63"}); err == nil {
		t.Error("expected error when node id is empty")
	}
	if _, err := BuildScyllaDesiredState(ScyllaDesiredInputs{NodeID: "n1"}); err == nil {
		t.Error("expected error when local IP is empty")
	}
}

func TestDeriveBootstrapIntent(t *testing.T) {
	cases := []struct {
		name string
		in   ScyllaDesiredInputs
		want string
	}{
		{"first node, no peers", ScyllaDesiredInputs{NodeID: "n1", LocalIP: "10.0.0.63"}, BootstrapFirstNode},
		{"first node, only self peer", ScyllaDesiredInputs{NodeID: "n1", LocalIP: "10.0.0.63", Peers: []string{"10.0.0.63"}}, BootstrapFirstNode},
		{"joining via peer", ScyllaDesiredInputs{NodeID: "n1", LocalIP: "10.0.0.63", Peers: []string{"10.0.0.63", "10.0.0.8"}}, BootstrapJoining},
		{"joining via seed", ScyllaDesiredInputs{NodeID: "n1", LocalIP: "10.0.0.63", Seeds: []string{"10.0.0.8"}}, BootstrapJoining},
		{"override wins", ScyllaDesiredInputs{NodeID: "n1", LocalIP: "10.0.0.63", Peers: []string{"10.0.0.63", "10.0.0.8"}, BootstrapIntentOverride: BootstrapFirstNode}, BootstrapFirstNode},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := deriveBootstrapIntent(c.in); got != c.want {
				t.Errorf("got %q want %q", got, c.want)
			}
		})
	}
}
