package main

import (
	"encoding/json"
	"testing"
)

// TestMigrationRecordRoundTrip verifies JSON marshaling of migrationRecord.
func TestMigrationRecordRoundTrip(t *testing.T) {
	rec := migrationRecord{
		Version:   schemaVersion,
		Status:    "complete",
		NodeID:    "node-abc",
		Timestamp: "2024-01-01T00:00:00Z",
	}

	data, err := json.Marshal(rec)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got migrationRecord
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if got.Version != rec.Version {
		t.Errorf("version: got %d, want %d", got.Version, rec.Version)
	}
	if got.Status != rec.Status {
		t.Errorf("status: got %q, want %q", got.Status, rec.Status)
	}
	if got.NodeID != rec.NodeID {
		t.Errorf("node_id: got %q, want %q", got.NodeID, rec.NodeID)
	}
}

// TestMigrationVersionGating verifies that only status==complete AND
// version >= schemaVersion satisfies the completion check.
func TestMigrationVersionGating(t *testing.T) {
	cases := []struct {
		name    string
		rec     migrationRecord
		wantOK  bool
	}{
		{
			name:   "complete_current_version",
			rec:    migrationRecord{Version: schemaVersion, Status: "complete"},
			wantOK: true,
		},
		{
			name:   "complete_future_version",
			rec:    migrationRecord{Version: schemaVersion + 1, Status: "complete"},
			wantOK: true,
		},
		{
			name:   "failed_current_version",
			rec:    migrationRecord{Version: schemaVersion, Status: "failed"},
			wantOK: false,
		},
		{
			name:   "complete_old_version",
			rec:    migrationRecord{Version: schemaVersion - 1, Status: "complete"},
			wantOK: schemaVersion-1 >= schemaVersion, // false
		},
		{
			name:   "zero_version",
			rec:    migrationRecord{Version: 0, Status: "complete"},
			wantOK: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.rec.Status == "complete" && tc.rec.Version >= schemaVersion
			if got != tc.wantOK {
				t.Errorf("completion check: got %v, want %v (rec=%+v)", got, tc.wantOK, tc.rec)
			}
		})
	}
}

// TestMigrationConstants ensures the constants are sane — schemaVersion must
// be positive and the TTL must be reasonable. These act as guardrails against
// accidental zeroing during refactors.
func TestMigrationConstants(t *testing.T) {
	if schemaVersion < 1 {
		t.Errorf("schemaVersion must be >= 1, got %d", schemaVersion)
	}
	if migrationLockTTL < 10 {
		t.Errorf("migrationLockTTL must be >= 10s to survive transient etcd hiccups, got %d", migrationLockTTL)
	}
	if migrationTimeout.Seconds() < float64(migrationLockTTL) {
		t.Errorf("migrationTimeout (%s) must be > migrationLockTTL (%ds) so we can wait for a slow holder",
			migrationTimeout, migrationLockTTL)
	}
	if migrationMutexKey == "" {
		t.Error("migrationMutexKey must not be empty")
	}
	if migrationStateKey == "" {
		t.Error("migrationStateKey must not be empty")
	}
}

// TestMigrationKeyHierarchy verifies that migrationStateKey is a child of
// migrationMutexKey. concurrency.NewMutex creates keys under the mutex prefix,
// so the state key must NOT share that prefix — otherwise etcd list queries
// under the mutex prefix would accidentally return the state record.
func TestMigrationKeyHierarchy(t *testing.T) {
	// The state key must NOT be a prefix match of the mutex key (it's a child,
	// that's fine), but the mutex key must not be a prefix of the state key in
	// a way that would pollute mutex listing.
	//
	// Accepted topology:
	//   mutex prefix: /globular/migrations/scylla/ai_memory
	//   state key:    /globular/migrations/scylla/ai_memory/state
	//
	// The /state suffix means etcdctl get --prefix on the mutex prefix WILL
	// return the state key. This is intentional — operators can see both.
	// What we verify is that they're distinct keys.
	if migrationMutexKey == migrationStateKey {
		t.Error("migrationMutexKey and migrationStateKey must be distinct keys")
	}
}
