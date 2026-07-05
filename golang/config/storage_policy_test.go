package config

import "testing"

func TestStoragePolicy_DefaultIsDurableAndSafe(t *testing.T) {
	p := DefaultStoragePolicy()
	if p.Profile != StorageProfileDurable {
		t.Fatalf("default profile = %q, want durable", p.Profile)
	}
	if p.AllowDegraded {
		t.Fatal("default must NOT allow degraded")
	}
	if p.IsDegraded() {
		t.Fatal("default must not be degraded")
	}
	if got := p.MinStorageNodes(); got != DurableMinStorageNodes {
		t.Fatalf("default MinStorageNodes = %d, want %d", got, DurableMinStorageNodes)
	}
	if p.MinioStandalone() {
		t.Fatal("durable must use distributed MinIO, not standalone")
	}
}

func TestStoragePolicy_NilTreatedAsDurable(t *testing.T) {
	var p *StoragePolicy
	if p.IsDegraded() {
		t.Fatal("nil policy must not be degraded")
	}
	if got := p.MinStorageNodes(); got != DurableMinStorageNodes {
		t.Fatalf("nil MinStorageNodes = %d, want %d (durable floor)", got, DurableMinStorageNodes)
	}
	if p.MinioStandalone() {
		t.Fatal("nil policy must not run standalone MinIO")
	}
}

func TestStoragePolicy_MinStorageNodes(t *testing.T) {
	cases := []struct {
		name    string
		policy  *StoragePolicy
		wantMin int
		wantDeg bool
	}{
		{"durable", &StoragePolicy{Profile: StorageProfileDurable}, 3, false},
		{"two-node declared", &StoragePolicy{Profile: StorageProfileTwoNodeDegraded, AllowDegraded: true}, 2, true},
		{"single-node declared", &StoragePolicy{Profile: StorageProfileSingleNode, AllowDegraded: true}, 1, true},
		// The critical no-silent-fallback cases: a degraded profile WITHOUT the
		// explicit opt-in must NOT relax the floor.
		{"two-node without opt-in stays durable", &StoragePolicy{Profile: StorageProfileTwoNodeDegraded, AllowDegraded: false}, 3, false},
		{"single-node without opt-in stays durable", &StoragePolicy{Profile: StorageProfileSingleNode, AllowDegraded: false}, 3, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := c.policy.MinStorageNodes(); got != c.wantMin {
				t.Errorf("MinStorageNodes = %d, want %d", got, c.wantMin)
			}
			if got := c.policy.IsDegraded(); got != c.wantDeg {
				t.Errorf("IsDegraded = %v, want %v", got, c.wantDeg)
			}
			if got := c.policy.MinioStandalone(); got != c.wantDeg {
				t.Errorf("MinioStandalone = %v, want %v", got, c.wantDeg)
			}
		})
	}
}

func TestStoragePolicy_ScyllaReplicationFactor(t *testing.T) {
	p := DefaultStoragePolicy()
	cases := []struct{ nodes, wantRF int }{
		{0, 1}, {1, 1}, {2, 2}, {3, 3}, {5, 3},
	}
	for _, c := range cases {
		if got := p.ScyllaReplicationFactor(c.nodes); got != c.wantRF {
			t.Errorf("ScyllaReplicationFactor(%d) = %d, want %d", c.nodes, got, c.wantRF)
		}
	}
}

func TestStoragePolicy_Validate(t *testing.T) {
	cases := []struct {
		name    string
		policy  *StoragePolicy
		wantErr bool
	}{
		{"durable ok", &StoragePolicy{Profile: StorageProfileDurable}, false},
		{"two-node with opt-in ok", &StoragePolicy{Profile: StorageProfileTwoNodeDegraded, AllowDegraded: true}, false},
		{"single-node with opt-in ok", &StoragePolicy{Profile: StorageProfileSingleNode, AllowDegraded: true}, false},
		{"unknown profile rejected", &StoragePolicy{Profile: "quad_node"}, true},
		{"degraded without opt-in rejected", &StoragePolicy{Profile: StorageProfileTwoNodeDegraded, AllowDegraded: false}, true},
		{"single without opt-in rejected", &StoragePolicy{Profile: StorageProfileSingleNode, AllowDegraded: false}, true},
		{"durable with opt-in rejected (contradiction)", &StoragePolicy{Profile: StorageProfileDurable, AllowDegraded: true}, true},
		{"nil rejected", nil, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := c.policy.Validate()
			if (err != nil) != c.wantErr {
				t.Errorf("Validate() err = %v, wantErr = %v", err, c.wantErr)
			}
		})
	}
}
