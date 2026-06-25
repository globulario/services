package config

// globular:tested_by critical_state_registry_ownership

import "testing"

// TestRegistryKeyHasCompletePolicy verifies that every key in CriticalEtcdKeys
// and every prefix in CriticalEtcdPrefixes has a corresponding entry in
// CriticalKeyPolicies with all required governance fields populated.
//
// This test acts as a CI regression guard: adding a key to the live-check list
// without a policy entry will fail the build immediately.
//
// Invariant: critical_state.registry_ownership_required
func TestRegistryKeyHasCompletePolicy(t *testing.T) {
	gaps := PolicyGapsForKeys(CriticalEtcdKeys, CriticalEtcdPrefixes)
	if len(gaps) != 0 {
		t.Errorf("critical keys have no ownership policy — add CriticalKeyPolicy entries for: %v", gaps)
	}
	for _, p := range CriticalKeyPolicies {
		if p.Key == "" {
			t.Error("CriticalKeyPolicy entry has empty Key")
		}
		if p.Owner == "" {
			t.Errorf("CriticalKeyPolicy %q: Owner must be non-empty", p.Key)
		}
		if p.SchemaVersion == "" {
			t.Errorf("CriticalKeyPolicy %q: SchemaVersion must be non-empty", p.Key)
		}
		if p.DeletePolicyName == "" {
			t.Errorf("CriticalKeyPolicy %q: DeletePolicyName must be non-empty", p.Key)
		}
		if p.DoctorInvariant == "" {
			t.Errorf("CriticalKeyPolicy %q: DoctorInvariant must be non-empty", p.Key)
		}
	}
}

// TestUnknownWriterRejected verifies that ValidateCriticalKeyOwner returns an
// error when the writer is not the registered owner of a critical key, and
// returns nil for the correct owner and for unregistered keys.
//
// Invariant: critical_state.registry_ownership_required
func TestUnknownWriterRejected(t *testing.T) {
	cases := []struct {
		key     string
		writer  string
		wantErr bool
	}{
		// Exact key matches — correct owner.
		{"/globular/ingress/v1/spec", "cluster-controller", false},
		{"/globular/system/config", "cluster-controller", false},
		{"/globular/pki/ca", "cluster-controller", false},
		{"/globular/ingress/v1/spec_backup", "cluster-controller", false},
		{"/globular/objectstore/config", "cluster-controller", false},
		// Exact key matches — wrong owner.
		{"/globular/ingress/v1/spec", "node-agent", true},
		{"/globular/system/config", "rogue-process", true},
		{"/globular/pki/ca", "dns-service", true},
		{"/globular/objectstore/config", "workflow-service", true},
		// Prefix matches — correct owner.
		{"/globular/nodes/node-abc/status", "node-agent", false},
		{"/globular/resources/DesiredService/my-svc", "cluster-controller", false},
		{"/globular/scylla/schema_guard/globular", "cluster-controller", false},
		// Prefix matches — wrong owner.
		// cluster-controller IS an authorized writer of /globular/nodes/ (it commits
		// installed-state there during release/convergence — AuthorizedWriters), so a
		// /globular/nodes/ write by cluster-controller is permitted, not rejected.
		{"/globular/nodes/node-abc/status", "cluster-controller", false},
		{"/globular/resources/ServiceRelease/svc", "node-agent", true},
		// A writer that is neither owner nor authorized is still rejected.
		{"/globular/nodes/node-abc/packages/svc/echo", "rogue-process", true},
		// Unregistered key — no restriction (returns nil regardless of writer).
		{"/globular/unknown/key", "any-writer", false},
		{"/custom/not/registered", "rogue-process", false},
	}
	for _, tc := range cases {
		err := ValidateCriticalKeyOwner(tc.key, tc.writer)
		if tc.wantErr && err == nil {
			t.Errorf("key=%q writer=%q: expected ownership error, got nil", tc.key, tc.writer)
		}
		if !tc.wantErr && err != nil {
			t.Errorf("key=%q writer=%q: expected no error, got %v", tc.key, tc.writer, err)
		}
	}
}

// TestDeleteCriticalKeyTriggerOwnerRestore verifies that OwnerForKey returns
// the correct owner for all registered critical keys (including prefix-based
// lookups), and returns an error for keys with no registered owner.
//
// The semantic intent: when a critical key is deleted, the system must be able
// to identify the authoritative owner who is responsible for restoring it.
//
// Invariant: critical_state.registry_ownership_required
func TestDeleteCriticalKeyTriggerOwnerRestore(t *testing.T) {
	// All exact keys must resolve to a known, non-empty owner.
	for _, key := range CriticalEtcdKeys {
		owner, err := OwnerForKey(key)
		if err != nil {
			t.Errorf("OwnerForKey(%q): expected owner, got error: %v", key, err)
			continue
		}
		if owner == "" {
			t.Errorf("OwnerForKey(%q): returned empty owner", key)
		}
	}

	// Prefix-based lookups: a concrete key under the prefix must return the
	// prefix owner so the restore guardian can be identified.
	prefixCases := []struct {
		key       string
		wantOwner string
	}{
		{"/globular/resources/DesiredService/my-svc", "cluster-controller"},
		{"/globular/nodes/node-abc/packages/globular-envoy", "node-agent"},
		{"/globular/scylla/schema_guard/globular_keyspace", "cluster-controller"},
	}
	for _, tc := range prefixCases {
		owner, err := OwnerForKey(tc.key)
		if err != nil {
			t.Errorf("OwnerForKey(%q): expected %q, got error: %v", tc.key, tc.wantOwner, err)
			continue
		}
		if owner != tc.wantOwner {
			t.Errorf("OwnerForKey(%q) = %q, want %q", tc.key, owner, tc.wantOwner)
		}
	}

	// Keys with no policy must return an error — the restore path cannot be
	// inferred for unregistered keys.
	unknowns := []string{
		"/globular/unknown/key",
		"/custom/not/registered",
		"/globular/ingress/v1/delete_approval/42", // sub-key, not in CriticalEtcdKeys
	}
	for _, key := range unknowns {
		_, err := OwnerForKey(key)
		if err == nil {
			t.Errorf("OwnerForKey(%q): expected error for unregistered key, got nil", key)
		}
	}
}
