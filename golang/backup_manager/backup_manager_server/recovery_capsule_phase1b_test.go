package main

import (
	"bytes"
	"strings"
	"testing"
)

// generatePhase1b is a small helper to drive writePhase1bScript and return
// the script text. Centralised so every test consumes the same artefact.
func generatePhase1b(t *testing.T) string {
	t.Helper()
	var buf bytes.Buffer
	// RecoveryInputs is unused by writePhase1bScript (the script is
	// data-driven via the capsule's own manifest at run time). Pass a
	// minimal struct so the generator doesn't crash on field access.
	writePhase1bScript(&buf, &RecoveryInputs{})
	return buf.String()
}

// generateMainRestore renders restore.sh so tests can assert orchestration.
func generateMainRestore(t *testing.T) string {
	t.Helper()
	var buf bytes.Buffer
	writeMainRestoreScript(&buf, &RecoveryInputs{
		BackupID: "test-backup",
		Domain:   "example.test",
	})
	return buf.String()
}

// generateReadme renders README.md.
func generateReadme(t *testing.T) string {
	t.Helper()
	var buf bytes.Buffer
	writeRecoveryREADME(&buf, &RecoveryInputs{
		BackupID: "test-backup",
		Domain:   "example.test",
	})
	return buf.String()
}

// mustContain asserts the script contains every substring in want. Each
// missing substring is reported individually so a failing test pinpoints
// which invariant slipped.
func mustContain(t *testing.T, label, script string, want ...string) {
	t.Helper()
	for _, w := range want {
		if !strings.Contains(script, w) {
			t.Errorf("[%s] missing required substring %q", label, w)
		}
	}
}

// mustNotContain asserts the absence of substrings. Used for safety-side
// invariants (e.g. no secret bytes embedded, no -x trace).
func mustNotContain(t *testing.T, label, script string, banned ...string) {
	t.Helper()
	for _, b := range banned {
		if strings.Contains(script, b) {
			t.Errorf("[%s] contains banned substring %q", label, b)
		}
	}
}

func TestWritePhase1bScript_HasStrictBashHeader(t *testing.T) {
	s := generatePhase1b(t)
	mustContain(t, "strict header", s,
		"#!/bin/bash",
		"set -euo pipefail",
		"set +x",
	)
}

func TestWritePhase1bScript_RequiresRoot(t *testing.T) {
	s := generatePhase1b(t)
	mustContain(t, "root assertion", s,
		`[[ $EUID -eq 0 ]] || die "Run as root (or with sudo)"`,
	)
}

func TestWritePhase1bScript_DetectsLocalNodeID(t *testing.T) {
	s := generatePhase1b(t)
	mustContain(t, "node-id detection", s,
		`LOCAL_NODE_ID="${NODE_ID_OVERRIDE:-}"`,
		`/var/lib/globular/nodeagent/state.json`,
		`jq -r .NodeID`,
		`cannot determine local node_id`,
	)
}

// TestWritePhase1bScript_AvailableNodesJQTolerant pins that the jq
// expression used to list available node IDs from the top-level manifest
// accepts both the current schema (.nodes[]) and a hypothetical alias
// (.cluster_node_set[]). This is forward-compat hardening: today the
// cluster manifest written by Phase 2 uses `.nodes` (see
// secret_collector_cluster.go: ClusterSecretManifest.Nodes), but the
// generated diagnostic should not break if that key is ever renamed.
func TestWritePhase1bScript_AvailableNodesJQTolerant(t *testing.T) {
	s := generatePhase1b(t)
	mustContain(t, "tolerant jq for available nodes", s,
		`jq -r '(.nodes // .cluster_node_set // [])[]?.node_id' "$TOP_MANIFEST"`,
	)
	// Defense in depth: the brittle single-schema expression must NOT
	// appear anywhere in the generated script.
	mustNotContain(t, "no brittle single-schema jq", s,
		`jq -r '.nodes[].node_id' "$TOP_MANIFEST"`,
	)
}

func TestWritePhase1bScript_VerifiesSha256BeforeWrite(t *testing.T) {
	s := generatePhase1b(t)
	mustContain(t, "sha256 verify", s,
		`ACT_SHA=$(sha256sum "$SRC" | awk '{print $1}')`,
		`[[ "$ACT_SHA" == "$EXP_SHA" ]]`,
		`die "sha256 mismatch`,
	)
	// The verify block MUST appear before the mv -f rename in source order.
	verifyIdx := strings.Index(s, "sha256 mismatch")
	renameIdx := strings.Index(s, `mv -f "$TMP" "$ORIG"`)
	if verifyIdx < 0 || renameIdx < 0 {
		t.Fatal("script missing sha-verify or rename anchor")
	}
	if verifyIdx >= renameIdx {
		t.Errorf("sha256 verify must precede the atomic rename; verifyIdx=%d renameIdx=%d", verifyIdx, renameIdx)
	}
}

func TestWritePhase1bScript_RefusesSymlinkDest(t *testing.T) {
	s := generatePhase1b(t)
	mustContain(t, "symlink dest refusal", s,
		`if [[ -L "$path" ]]; then`,
		`die "REFUSE: destination $path is a symlink"`,
	)
}

// TestWritePhase1bScript_RefusesSymlinkAncestor pins the user-requested
// defense-in-depth: walking each ancestor and refusing if any of them is
// a symlink, not just the leaf.
func TestWritePhase1bScript_RefusesSymlinkAncestor(t *testing.T) {
	s := generatePhase1b(t)
	mustContain(t, "symlink ancestor refusal", s,
		`refuse_symlink_path()`,
		`IFS='/' read -ra parts <<< "${dir#/}"`,
		`for part in "${parts[@]}"`,
		`if [[ -L "$cur" ]]; then`,
		`die "REFUSE: destination ancestor $cur is a symlink"`,
	)
	// And the helper must be CALLED before mkdir/install/mv in the loop.
	callIdx := strings.Index(s, `refuse_symlink_path "$ORIG"`)
	mkdirIdx := strings.Index(s, `mkdir -p "$(dirname "$ORIG")"`)
	if callIdx < 0 || mkdirIdx < 0 {
		t.Fatal("missing refuse_symlink_path call or mkdir anchor")
	}
	if callIdx >= mkdirIdx {
		t.Errorf("refuse_symlink_path must be called before mkdir; callIdx=%d mkdirIdx=%d", callIdx, mkdirIdx)
	}
}

func TestWritePhase1bScript_RefusesOutsideGlobular(t *testing.T) {
	s := generatePhase1b(t)
	mustContain(t, "outside-globular refusal", s,
		`canonical="$(realpath -m "$path")"`,
		`[[ "$canonical" == /var/lib/globular/* ]]`,
		`die "REFUSE: destination $path resolves outside /var/lib/globular`,
	)
}

func TestWritePhase1bScript_AppliesModeOwnerGroup(t *testing.T) {
	s := generatePhase1b(t)
	mustContain(t, "mode/owner/group restore", s,
		`MODE=$(echo "$entry" | jq -r '.mode_octal`,
		`OWNER=$(echo "$entry" | jq -r '.owner`,
		`GROUP=$(echo "$entry" | jq -r '.group`,
		`install -m "$MODE" -o "$OWNER" -g "$GROUP"`,
	)
}

func TestWritePhase1bScript_IdempotentMatchingSha(t *testing.T) {
	s := generatePhase1b(t)
	mustContain(t, "idempotent skip", s,
		`LOCAL_SHA=$(sha256sum "$ORIG" | awk '{print $1}')`,
		`if [[ "$LOCAL_SHA" == "$EXP_SHA" ]]; then`,
		`log "OK (idempotent):`,
		`continue`,
	)
}

func TestWritePhase1bScript_RefusesOverwriteWithoutForce(t *testing.T) {
	s := generatePhase1b(t)
	mustContain(t, "overwrite protection", s,
		`if [[ -z "${FORCE_OVERWRITE:-}" ]]; then`,
		`die "$ORIG exists with different content`,
		`set FORCE_OVERWRITE=1 to replace`,
	)
}

func TestWritePhase1bScript_HandlesMissingSecretsDirGracefully(t *testing.T) {
	s := generatePhase1b(t)
	mustContain(t, "missing secrets dir back-compat", s,
		`if [[ ! -d "$SECRETS_ROOT" ]]; then`,
		`log "no payload/secrets/ in capsule (pre-Task-#12 backup); skipping"`,
		`exit 0`,
	)
}

// TestWritePhase1bScript_NoFileContentsEmbedded — the generator must
// never embed payload bytes into the script. The script is purely a
// runtime reader of the capsule.
func TestWritePhase1bScript_NoFileContentsEmbedded(t *testing.T) {
	s := generatePhase1b(t)
	// No heredocs that could carry secrets, no `cat`/`echo` of secret files.
	mustNotContain(t, "no embedded secret bodies", s,
		"BEGIN PRIVATE KEY",
		"-----BEGIN",
		"<<EOF_SECRETS",
		`cat "$SRC"`,
		`cat $SRC`,
	)
	// The script DOES `cat /var/lib/globular/nodeagent/state.json` indirectly
	// via jq — that's the node_id source, not a secret payload. Audit by
	// counting `cat` invocations: should be zero in the loop body.
	loopStart := strings.Index(s, "jq -c '.entries[]")
	loopEnd := strings.Index(s, `log "Phase 1b complete`)
	if loopStart < 0 || loopEnd < 0 {
		t.Fatal("missing loop anchors")
	}
	loopBody := s[loopStart:loopEnd]
	if strings.Contains(loopBody, "cat ") {
		t.Errorf("restore loop body must not invoke `cat` on capsule files; found in:\n%s", loopBody)
	}
}

// TestWriteMainRestoreScript_InsertsPhase1b_BetweenPhase1AndPhase2 — pins
// the orchestration order. phase1b must run AFTER phase1 (restic restored
// the filesystem baseline) and BEFORE phase2 (MinIO needs the contract).
func TestWriteMainRestoreScript_InsertsPhase1b_BetweenPhase1AndPhase2(t *testing.T) {
	s := generateMainRestore(t)

	p1Idx := strings.Index(s, `phase1-restore-files.sh`)
	p1bIdx := strings.Index(s, `phase1b-restore-secrets.sh`)
	p2Idx := strings.Index(s, `phase2-bootstrap-minio.sh`)

	if p1Idx < 0 || p1bIdx < 0 || p2Idx < 0 {
		t.Fatalf("missing phase reference in main restore script (p1=%d p1b=%d p2=%d)", p1Idx, p1bIdx, p2Idx)
	}
	if !(p1Idx < p1bIdx && p1bIdx < p2Idx) {
		t.Errorf("phase1b must appear between phase1 and phase2; offsets p1=%d p1b=%d p2=%d", p1Idx, p1bIdx, p2Idx)
	}

	// Phase filter help text must mention 1b.
	mustContain(t, "usage text mentions 1b", s,
		"--phase 1|1b|2|3|4",
		"Phase 1b",
	)

	// run_phase line for 1b uses the right script name.
	mustContain(t, "run_phase 1b line", s,
		`run_phase 1b "Restore node-local secrets from capsule"                 "phase1b-restore-secrets.sh"`,
	)
}

func TestWriteReadme_DocumentsPhase1b(t *testing.T) {
	s := generateReadme(t)
	mustContain(t, "README documents phase1b", s,
		"`phase1b-restore-secrets.sh`",
		"Restore node-local root-owned secrets collected by backup_manager",
		"../payload/secrets/<node_id>/",
	)
}
