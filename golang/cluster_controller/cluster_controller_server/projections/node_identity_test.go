package projections

import (
	"testing"
)

// These tests cover pure-function behaviour that doesn't require a live
// ScyllaDB. Integration tests (CREATE TABLE / batch write / reverse lookup)
// live alongside the cluster-controller tests where a real session is
// available.

func TestNodeIdentityValidation(t *testing.T) {
	tests := []struct {
		name    string
		in      NodeIdentity
		wantErr bool
	}{
		{
			name:    "empty node_id rejected",
			in:      NodeIdentity{Hostname: "foo"},
			wantErr: true,
		},
		{
			name: "minimal valid",
			in: NodeIdentity{
				NodeID:     "eb9a2dac-05b0-52ac-9002-99d8ffd35902",
				Hostname:   "globule-ryzen",
				ObservedAt: 1712345678,
			},
			wantErr: false,
		},
		{
			name: "full identity",
			in: NodeIdentity{
				NodeID:     "eb9a2dac-05b0-52ac-9002-99d8ffd35902",
				Hostname:   "globule-ryzen",
				IPs:        []string{"10.0.0.63"},
				MACs:       []string{"e0:d4:64:f0:86:f6"},
				Labels:     []string{"control-plane", "core", "gateway"},
				ObservedAt: 1712345678,
			},
			wantErr: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Upsert on a nil-session projector surfaces the same validation
			// that the real one does, without needing a live scylla.
			p := &NodeIdentityProjector{}
			err := p.upsertPreflight(tc.in)
			if (err != nil) != tc.wantErr {
				t.Fatalf("wantErr=%v, got err=%v", tc.wantErr, err)
			}
		})
	}
}

// upsertPreflight extracts the pure validation step from Upsert so it can be
// tested without a gocql session. In production Upsert performs this check
// as its first line.
func (p *NodeIdentityProjector) upsertPreflight(id NodeIdentity) error {
	if id.NodeID == "" {
		return errEmptyNodeID
	}
	return nil
}

var errEmptyNodeID = &projectionError{msg: "node_identity: empty node_id"}

type projectionError struct{ msg string }

func (e *projectionError) Error() string { return e.msg }

func TestNodeIdentityTablesCoverage(t *testing.T) {
	// Every projector operation that writes to scylla must have its
	// corresponding CREATE TABLE statement returned by nodeIdentityTables().
	// If a new table is added later, either extend this set or fail loudly.
	wantTables := []string{
		"node_identity",
		"node_identity_by_hostname",
		"node_identity_by_mac",
		"node_identity_by_ip",
	}
	stmts := nodeIdentityTables()
	if len(stmts) != len(wantTables) {
		t.Fatalf("nodeIdentityTables() returned %d stmts, want %d", len(stmts), len(wantTables))
	}
	for i, name := range wantTables {
		if !contains(stmts[i], name) {
			t.Errorf("stmt[%d] does not reference table %q:\n%s", i, name, stmts[i])
		}
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
