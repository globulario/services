package main

import (
	"testing"

	"github.com/globulario/services/golang/versionutil"
)

// entrypointProofOptional must be driven by the DECLARED identity proof mode, not
// inferred from entrypoint=="none". The old inference wrongly exempted a
// binary_sha256 noop package (e.g. claude) from sha verification.
func TestEntrypointProofOptional_DrivenByDeclaredProofMode(t *testing.T) {
	versionutil.SetBaseDir(t.TempDir())

	// proof=version (codex/scylladb/keepalived): identity is the version → optional.
	if err := versionutil.WriteIdentityProof("codex", "version"); err != nil {
		t.Fatal(err)
	}
	if !entrypointProofOptional("codex") {
		t.Error("proof=version must be entrypoint-proof-optional (version is the identity)")
	}

	// proof=binary_sha256 (claude): a pinned binary sha IS the identity → NOT
	// optional, even though the entrypoint is none — the installed binary must be
	// hashed against the declared checksum.
	if err := versionutil.WriteIdentityProof("claude", "binary_sha256"); err != nil {
		t.Fatal(err)
	}
	if entrypointProofOptional("claude") {
		t.Error("proof=binary_sha256 must NOT be optional — the old entrypoint==none inference wrongly exempted it")
	}

	// Legacy package: no proof sidecar → fall back to the entrypoint sidecar.
	if err := versionutil.WriteEntrypoint("legacy-noop", "none"); err != nil {
		t.Fatal(err)
	}
	if !entrypointProofOptional("legacy-noop") {
		t.Error("legacy noop (entrypoint none, no declared proof) must fall back to optional")
	}
	if err := versionutil.WriteEntrypoint("legacy-bin", "mybin"); err != nil {
		t.Fatal(err)
	}
	if entrypointProofOptional("legacy-bin") {
		t.Error("legacy shipped-binary (entrypoint mybin, no declared proof) must NOT be optional")
	}
}
