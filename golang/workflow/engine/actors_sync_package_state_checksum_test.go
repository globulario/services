package engine

// Project J — actor-level regression tests for the
// installed_package_checksum_must_be_binary_sha256 invariant.
//
// nodeSyncPackageState used to read req.With["desired_hash"] (the synthetic
// convergence-identity hash) and pass it through SyncInstalledPackage →
// CommitInstalledPackage → etcd InstalledPackage.Checksum. Every other writer
// (apply_package_release.go, self_hosted_runtime_proof_writer.go) treats
// Checksum as the BINARY sha256 (= manifest entrypoint_checksum). The
// committer was the only writer aliasing the synthetic identity into the
// binary-sha256 slot. INC-2026-0014's invariant
// install_package.hash_schemas_must_not_alias forbids that aliasing.
//
// The fix:
//   - actor reads req.With["resolved_entrypoint_checksum"]
//   - falls back to "" when the value is absent or the synthesized "<nil>"
//   - NEVER falls back to desired_hash
//   - the heartbeat / self-hosted proof writer fills the Checksum field
//     from on-disk truth on the next cycle when the committer leaves it empty

import (
	"context"
	"testing"
)

// 1. Happy path: the actor reads resolved_entrypoint_checksum from req.With
// and passes it verbatim to SyncInstalledPackage as the hash argument.
// The forbidden outcome is reading any other key, especially desired_hash.
func TestNodeSyncPackageState_WritesEntrypointChecksumNotDesiredHash(t *testing.T) {
	const wantBinaryHash = "1ddbdc2cf2f4cca415f5b2b3e22a5001ff142b636043e4f350ab7bdaa04fb7bf"
	const desiredHash = "de2b04ff64ce4489000000000000000000000000000000000000000000000000"

	var seen string
	cfg := NodeDirectApplyConfig{
		SyncInstalledPackage: func(ctx context.Context, name, version, hash, kind, buildID string) error {
			seen = hash
			return nil
		},
	}
	handler := nodeSyncPackageState(cfg)

	req := ActionRequest{
		With: map[string]any{
			"package_name":                 "dns",
			"version":                      "1.2.113",
			"package_kind":                 "SERVICE",
			"build_id":                     "019e7136-d4da-780d-a2ee-32647d327797",
			"desired_hash":                 desiredHash,
			"resolved_entrypoint_checksum": wantBinaryHash,
		},
	}
	if _, err := handler(context.Background(), req); err != nil {
		t.Fatalf("handler returned err: %v", err)
	}
	if seen == desiredHash {
		t.Fatalf("regression: actor read desired_hash %q into Checksum slot (INC-2026-0014 violation)", desiredHash)
	}
	if seen != wantBinaryHash {
		t.Fatalf("Checksum hash = %q, want resolved_entrypoint_checksum=%q", seen, wantBinaryHash)
	}
}

// 2. Missing key fallback: when resolved_entrypoint_checksum is absent (older
// workflow definitions / pre-manifest dispatch paths), the actor must pass
// an empty string — NOT silently fall back to desired_hash. The heartbeat
// self-hosted proof writer fills the field from on-disk truth on the next
// cycle.
func TestNodeSyncPackageState_FallsBackToEmptyWhenMissingChecksum(t *testing.T) {
	var seen string
	var called bool
	cfg := NodeDirectApplyConfig{
		SyncInstalledPackage: func(ctx context.Context, name, version, hash, kind, buildID string) error {
			seen = hash
			called = true
			return nil
		},
	}
	handler := nodeSyncPackageState(cfg)

	// resolved_entrypoint_checksum omitted; desired_hash present (legacy).
	req := ActionRequest{
		With: map[string]any{
			"package_name": "legacy-svc",
			"version":      "1.0.0",
			"package_kind": "SERVICE",
			"desired_hash": "de2b04ff64ce4489synthetic-identity-hash-do-not-write",
		},
	}
	if _, err := handler(context.Background(), req); err != nil {
		t.Fatalf("handler returned err: %v", err)
	}
	if !called {
		t.Fatalf("SyncInstalledPackage callback not invoked")
	}
	if seen != "" {
		t.Fatalf("hash = %q, want empty; actor must NOT fall back to desired_hash", seen)
	}
}

// 3. The synthesized "<nil>" string (Go's fmt.Sprint of nil map value) MUST
// also fall back to empty. Without this guard, the actor would write the
// literal string "<nil>" into Checksum.
func TestNodeSyncPackageState_NilStringFallsBackToEmpty(t *testing.T) {
	var seen string
	cfg := NodeDirectApplyConfig{
		SyncInstalledPackage: func(ctx context.Context, name, version, hash, kind, buildID string) error {
			seen = hash
			return nil
		},
	}
	handler := nodeSyncPackageState(cfg)

	req := ActionRequest{
		With: map[string]any{
			"package_name": "x",
			"version":      "1.0",
			"package_kind": "SERVICE",
			// resolved_entrypoint_checksum: nil -> fmt.Sprint => "<nil>"
			"resolved_entrypoint_checksum": nil,
		},
	}
	if _, err := handler(context.Background(), req); err != nil {
		t.Fatalf("handler returned err: %v", err)
	}
	if seen != "" {
		t.Fatalf("hash = %q, want empty (the synthesized \"<nil>\" must be treated as missing)", seen)
	}
}

// 4. Regression — install_package.hash_schemas_must_not_alias. With BOTH
// desired_hash and resolved_entrypoint_checksum present, the actor MUST
// pick the binary-sha256 (resolved_entrypoint_checksum), never the
// synthetic identity (desired_hash). Asserts the invariant rule from
// INC-2026-0014 at the actor boundary.
func TestNodeSyncPackageState_HashSchemasMustNotAlias(t *testing.T) {
	const binaryHash = "879e841827e74446259b878c354de4f92d9a48859546ee4ae0021b618f00a79a"
	const phantomIdentity = "de2b04ff64ce4489abcdef0123456789abcdef0123456789abcdef0123456789"

	var seen string
	cfg := NodeDirectApplyConfig{
		SyncInstalledPackage: func(ctx context.Context, name, version, hash, kind, buildID string) error {
			seen = hash
			return nil
		},
	}
	handler := nodeSyncPackageState(cfg)

	req := ActionRequest{
		With: map[string]any{
			"package_name":                 "repository",
			"version":                      "1.2.122",
			"package_kind":                 "SERVICE",
			"build_id":                     "019e717b-eceb-7878-b601-ee1631a4230b",
			"desired_hash":                 phantomIdentity,
			"resolved_entrypoint_checksum": binaryHash,
		},
	}
	if _, err := handler(context.Background(), req); err != nil {
		t.Fatalf("handler returned err: %v", err)
	}
	if seen != binaryHash {
		t.Fatalf("invariant install_package.hash_schemas_must_not_alias violated: hash = %q, want binary %q", seen, binaryHash)
	}
}

// 5. Non-string types in with: map: ensure fmt.Sprint conversion doesn't
// produce a garbage value. Some workflow loaders deliver int / int64 for
// numeric fields. resolved_entrypoint_checksum is always a hex string in
// practice, but the test pins the contract.
func TestNodeSyncPackageState_StringValueRoundTripped(t *testing.T) {
	const want = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

	var seen string
	cfg := NodeDirectApplyConfig{
		SyncInstalledPackage: func(ctx context.Context, name, version, hash, kind, buildID string) error {
			seen = hash
			return nil
		},
	}
	handler := nodeSyncPackageState(cfg)

	req := ActionRequest{
		With: map[string]any{
			"package_name":                 "x",
			"version":                      "1.0",
			"package_kind":                 "SERVICE",
			"resolved_entrypoint_checksum": want,
		},
	}
	if _, err := handler(context.Background(), req); err != nil {
		t.Fatalf("handler returned err: %v", err)
	}
	if seen != want {
		t.Fatalf("hash = %q, want %q", seen, want)
	}
}
