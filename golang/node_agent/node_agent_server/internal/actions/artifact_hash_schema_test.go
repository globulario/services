// Regression test for invariant install_package.hash_schemas_must_not_alias.
//
// Previously DownloadArtifactToDir treated any non-empty expectedSHA256 as
// the BUNDLE digest. The InstallPackage caller always passed manifest's
// entrypoint_checksum (the BINARY hash inside the bundle), so the verifier
// compared bundle bytes against an entrypoint hash and produced a
// false-positive "artifact digest mismatch" on every fresh download.
//
// The fix: DownloadArtifactToDir ALWAYS resolves the bundle digest from
// the manifest, regardless of what the caller passes. This test pins that
// behaviour by calling the function with an unreachable repo and asserting
// the failure path goes through manifest resolution first (the new code
// path) rather than skipping it (the old buggy path).
package actions_test

import (
	"context"
	"strings"
	"testing"

	"github.com/globulario/services/golang/node_agent/node_agent_server/internal/actions"
)

func TestDownloadArtifactToDir_AlwaysResolvesManifestEvenIfEntrypointHashProvided(t *testing.T) {
	bogusEntrypointHash := "deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef"
	dest := t.TempDir()

	_, err := actions.DownloadArtifactToDir(
		context.Background(),
		"127.0.0.1:1", // unreachable — manifest resolution must fail HERE
		"test@globular.io",
		"nonexistent-pkg",
		"0.0.0",
		"linux_amd64",
		"SERVICE",
		bogusEntrypointHash, // alias-bug bait: non-empty entrypoint hash
		dest,
		0,
	)
	if err == nil {
		t.Fatalf("expected error from unreachable repo, got nil")
	}

	// In the BUGGY path the function would skip resolveArtifactDigest
	// (because expectedSHA256 was non-empty) and proceed to download,
	// failing later with a download / connection error.
	//
	// In the FIXED path it ALWAYS calls resolveArtifactDigest first and
	// fails with "cannot resolve manifest bundle checksum". That phrase
	// is the regression anchor.
	if !strings.Contains(err.Error(), "manifest bundle checksum") {
		t.Errorf("expected manifest-resolve-first behaviour (alias-bug fix)\n"+
			"  got error: %q\n"+
			"  this likely means DownloadArtifactToDir regressed to using the\n"+
			"  caller's expectedSHA256 (entrypoint hash) as the bundle hash,\n"+
			"  which violates invariant install_package.hash_schemas_must_not_alias.",
			err.Error())
	}
}
