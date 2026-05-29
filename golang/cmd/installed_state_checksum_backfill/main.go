// installed_state_checksum_backfill — Project K Phase 1 / 2 / 3 helper.
//
// Reads one installed_state record from etcd, applies the Project K safety
// predicate, and (when --apply) writes the corrected InstalledPackage.Checksum
// + Metadata["entrypoint_checksum"] via the proper writer
// (installed_state.WriteInstalledPackage). All other fields are preserved.
//
// This is NOT a long-running RPC. It's a one-record-at-a-time CLI tool the
// operator drives via a script. Each invocation logs the verdict to stdout
// and a structured record to the audit ring (if reachable).
//
// Predicate (per loads/checksum_backfill_inventory_impact.md):
//
//  1. The expected binary path exists and is readable.
//  2. The on-disk sha256 of the binary equals the
//     repository.manifests[name,version].entrypointChecksum recorded in
//     the local CAS manifest file.
//  3. The current Checksum is empty OR is not equal to the on-disk sha256.
//  4. The record's proof_source is NOT self_hosted_runtime_proof.
//  5. The record's Status is "installed".
//
// If any clause fails, the tool exits non-zero with a structured reason
// printed to stderr and writes nothing.
//
// Forbidden by design:
//   - no raw cqlsh / etcdctl put
//   - no Status mutation
//   - no Version / BuildId / BuildNumber mutation
//   - no proof_source forgery
package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/globulario/services/golang/installed_state"
	node_agentpb "github.com/globulario/services/golang/node_agent/node_agentpb"
)

const (
	repositoryCASDir       = "/var/lib/globular/repository/artifacts"
	defaultBinaryDir       = "/usr/lib/globular/bin"
	selfHostedProofSource  = "self_hosted_runtime_proof"
	statusInstalled        = "installed"
)

func main() {
	var (
		nodeID                = flag.String("node", "", "node_id (required)")
		kind                  = flag.String("kind", "", "package kind (SERVICE|INFRASTRUCTURE|COMMAND) (required)")
		name                  = flag.String("name", "", "package name (required)")
		apply                 = flag.Bool("apply", false, "set to actually write; default is dry-run")
		repairUpdatedUnixOnly = flag.Bool("repair-updated-unix-to-installed-unix", false,
			"recovery mode: ONLY rewrites UpdatedUnix to InstalledUnix when Checksum is "+
				"already correct. Use to undo a previous run that bumped UpdatedUnix and "+
				"falsely tripped the cluster_doctor service.old_pid_after_upgrade rule. "+
				"Implies --apply.")
	)
	flag.Parse()

	if *nodeID == "" || *kind == "" || *name == "" {
		fmt.Fprintln(os.Stderr, "usage: installed_state_checksum_backfill --node <id> --kind <SERVICE|INFRASTRUCTURE|COMMAND> --name <pkg> [--apply | --repair-updated-unix-to-installed-unix]")
		os.Exit(2)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var verdict Verdict
	var err error
	if *repairUpdatedUnixOnly {
		verdict, err = runUpdatedUnixRepair(ctx, *nodeID, *kind, *name)
	} else {
		verdict, err = runPredicateAndMaybeWrite(ctx, *nodeID, *kind, *name, *apply)
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(verdict)
	if err != nil {
		os.Exit(1)
	}
}

// Verdict is the structured per-record outcome.
type Verdict struct {
	NodeID                 string `json:"node_id"`
	Kind                   string `json:"kind"`
	Name                   string `json:"name"`
	Version                string `json:"version"`
	StatusBefore           string `json:"status_before"`
	ChecksumBefore         string `json:"checksum_before"`
	OnDiskBinaryPath       string `json:"on_disk_binary_path"`
	OnDiskSHA256           string `json:"on_disk_sha256"`
	ManifestChecksum       string `json:"manifest_entrypoint_checksum"`
	OnDiskMatchesManifest  bool   `json:"on_disk_matches_manifest"`
	ProofSource            string `json:"proof_source"`
	PredicateClause1Binary bool   `json:"predicate_1_binary_exists"`
	PredicateClause2Match  bool   `json:"predicate_2_on_disk_matches_manifest"`
	PredicateClause3NeedsWrite bool `json:"predicate_3_needs_write"`
	PredicateClause4NotSelfHosted bool `json:"predicate_4_not_self_hosted"`
	PredicateClause5Installed bool `json:"predicate_5_status_installed"`
	Verdict                string `json:"verdict"` // would_repair | repaired | skipped
	SkipReason             string `json:"skip_reason,omitempty"`
	Applied                bool   `json:"applied"`
	ChecksumAfter          string `json:"checksum_after,omitempty"`
}

func runPredicateAndMaybeWrite(ctx context.Context, nodeID, kind, name string, apply bool) (Verdict, error) {
	v := Verdict{NodeID: nodeID, Kind: kind, Name: name}

	// Read existing record.
	existing, err := installed_state.GetInstalledPackage(ctx, nodeID, kind, name)
	if err != nil {
		v.Verdict = "skipped"
		v.SkipReason = fmt.Sprintf("read existing record: %v", err)
		return v, fmt.Errorf("get installed package: %w", err)
	}
	if existing == nil {
		v.Verdict = "skipped"
		v.SkipReason = "no installed_state record at the expected key"
		return v, fmt.Errorf("record not found")
	}
	v.Version = existing.GetVersion()
	v.StatusBefore = existing.GetStatus()
	v.ChecksumBefore = existing.GetChecksum()
	if md := existing.GetMetadata(); md != nil {
		v.ProofSource = md["proof_source"]
	}

	// Predicate clause 1: binary path exists and is readable.
	binPath := candidateBinaryPath(name, kind)
	v.OnDiskBinaryPath = binPath
	if binPath == "" {
		v.Verdict = "skipped"
		v.SkipReason = "predicate_1_binary_missing: no binary at any candidate path"
		return v, fmt.Errorf("no candidate binary path")
	}
	fi, err := os.Stat(binPath)
	if err != nil || fi.IsDir() {
		v.Verdict = "skipped"
		v.SkipReason = fmt.Sprintf("predicate_1_binary_missing: stat %s: %v", binPath, err)
		return v, fmt.Errorf("binary stat: %w", err)
	}
	onDisk, err := hashFile(binPath)
	if err != nil {
		v.Verdict = "skipped"
		v.SkipReason = fmt.Sprintf("predicate_1_hash_failed: %v", err)
		return v, fmt.Errorf("hash binary: %w", err)
	}
	v.OnDiskSHA256 = onDisk
	v.PredicateClause1Binary = true

	// Predicate clause 2: on-disk sha256 == repository manifest entrypoint_checksum
	// for the recorded version.
	mfst, err := readCASManifestEntrypointChecksum(name, v.Version)
	if err != nil {
		v.Verdict = "skipped"
		v.SkipReason = fmt.Sprintf("predicate_2_manifest_missing: %v", err)
		return v, fmt.Errorf("read CAS manifest: %w", err)
	}
	v.ManifestChecksum = mfst
	v.OnDiskMatchesManifest = strings.EqualFold(onDisk, mfst)
	if !v.OnDiskMatchesManifest {
		v.Verdict = "skipped"
		v.SkipReason = fmt.Sprintf("predicate_2_drift: on-disk %s != manifest %s (genuine hash_drift requires fresh install, not backfill)", onDisk[:16], mfst[:16])
		return v, fmt.Errorf("on-disk vs manifest mismatch")
	}
	v.PredicateClause2Match = true

	// Predicate clause 3: Checksum needs write (empty OR different from on-disk).
	cur := strings.ToLower(strings.TrimSpace(existing.GetChecksum()))
	if cur != "" && cur == onDisk {
		v.Verdict = "skipped"
		v.SkipReason = "predicate_3_already_correct: Checksum already equals on-disk sha256"
		return v, fmt.Errorf("idempotent skip")
	}
	v.PredicateClause3NeedsWrite = true

	// Predicate clause 4: not Project B self-hosted record.
	if v.ProofSource == selfHostedProofSource {
		v.Verdict = "skipped"
		v.SkipReason = "predicate_4_self_hosted: Project B owns this record (proof_source=self_hosted_runtime_proof)"
		return v, fmt.Errorf("self-hosted, skip")
	}
	v.PredicateClause4NotSelfHosted = true

	// Predicate clause 5: status == installed.
	if v.StatusBefore != statusInstalled {
		v.Verdict = "skipped"
		v.SkipReason = fmt.Sprintf("predicate_5_status: status=%q, want %q", v.StatusBefore, statusInstalled)
		return v, fmt.Errorf("not installed, skip")
	}
	v.PredicateClause5Installed = true

	// All predicates pass.
	if !apply {
		v.Verdict = "would_repair"
		return v, nil
	}

	// Patch the record. Copy the existing struct; mutate only Checksum and
	// Metadata["entrypoint_checksum"]. Preserve everything else.
	patched := &node_agentpb.InstalledPackage{}
	*patched = *existing
	patched.Checksum = onDisk
	if patched.Metadata == nil {
		patched.Metadata = map[string]string{}
	} else {
		md := make(map[string]string, len(existing.Metadata)+1)
		for k, vv := range existing.Metadata {
			md[k] = vv
		}
		patched.Metadata = md
	}
	patched.Metadata["entrypoint_checksum"] = onDisk
	// IMPORTANT: do NOT advance UpdatedUnix. The backfill corrects recorded
	// metadata that should have been right from the original install — the
	// binary on disk has not changed. Advancing UpdatedUnix tricks the
	// cluster_doctor `service.old_pid_after_upgrade` rule into firing
	// because it compares UpdatedUnix to the running process start time
	// and assumes a newer record means a newer binary. Preserve the
	// existing UpdatedUnix so the runtime-vs-record comparison stays
	// honest.

	if err := installed_state.WriteInstalledPackage(ctx, patched); err != nil {
		v.Verdict = "skipped"
		v.SkipReason = fmt.Sprintf("write_failed: %v", err)
		return v, fmt.Errorf("write installed package: %w", err)
	}
	v.Verdict = "repaired"
	v.Applied = true
	v.ChecksumAfter = onDisk
	return v, nil
}

// runUpdatedUnixRepair handles the recovery case for the first invocation
// of this tool (which incorrectly advanced UpdatedUnix). It refuses to act
// unless:
//   - The record exists.
//   - Checksum is non-empty and matches Metadata["entrypoint_checksum"]
//     and matches the on-disk binary sha256 — i.e. the previous backfill
//     succeeded for the Checksum field and the only damage is a wrong
//     UpdatedUnix.
//   - InstalledUnix is non-zero (we have a sane target value to revert to).
//
// The write rewrites only UpdatedUnix = InstalledUnix; every other field is
// preserved byte-for-byte.
func runUpdatedUnixRepair(ctx context.Context, nodeID, kind, name string) (Verdict, error) {
	v := Verdict{NodeID: nodeID, Kind: kind, Name: name}
	existing, err := installed_state.GetInstalledPackage(ctx, nodeID, kind, name)
	if err != nil || existing == nil {
		v.Verdict = "skipped"
		v.SkipReason = fmt.Sprintf("read existing record: %v", err)
		return v, fmt.Errorf("get installed package: %w", err)
	}
	v.Version = existing.GetVersion()
	v.StatusBefore = existing.GetStatus()
	v.ChecksumBefore = existing.GetChecksum()

	if existing.GetChecksum() == "" {
		v.Verdict = "skipped"
		v.SkipReason = "checksum_empty: Checksum is empty, this is a fresh-backfill case not a repair"
		return v, fmt.Errorf("checksum empty")
	}
	mdCk := existing.GetMetadata()["entrypoint_checksum"]
	if !strings.EqualFold(existing.GetChecksum(), mdCk) {
		v.Verdict = "skipped"
		v.SkipReason = fmt.Sprintf("checksum_metadata_disagree: Checksum=%s vs Metadata.entrypoint_checksum=%s",
			existing.GetChecksum()[:16], mdCk[:16])
		return v, fmt.Errorf("checksum metadata disagree")
	}
	binPath := candidateBinaryPath(name, kind)
	if binPath == "" {
		v.Verdict = "skipped"
		v.SkipReason = "binary_missing"
		return v, fmt.Errorf("no binary")
	}
	onDisk, err := hashFile(binPath)
	if err != nil {
		v.Verdict = "skipped"
		v.SkipReason = fmt.Sprintf("hash_file_failed: %v", err)
		return v, fmt.Errorf("hash file: %w", err)
	}
	v.OnDiskBinaryPath = binPath
	v.OnDiskSHA256 = onDisk
	if !strings.EqualFold(existing.GetChecksum(), onDisk) {
		v.Verdict = "skipped"
		v.SkipReason = fmt.Sprintf("checksum_does_not_match_disk: %s vs %s (the prior backfill didn't take or the binary has changed)",
			existing.GetChecksum()[:16], onDisk[:16])
		return v, fmt.Errorf("checksum does not match disk")
	}
	if existing.GetInstalledUnix() == 0 {
		v.Verdict = "skipped"
		v.SkipReason = "installed_unix_zero: cannot determine sane revert value"
		return v, fmt.Errorf("installed_unix zero")
	}
	if existing.GetInstalledUnix() == existing.GetUpdatedUnix() {
		v.Verdict = "skipped"
		v.SkipReason = "no_repair_needed: InstalledUnix already equals UpdatedUnix"
		return v, fmt.Errorf("idempotent skip")
	}

	patched := &node_agentpb.InstalledPackage{}
	*patched = *existing
	patched.UpdatedUnix = existing.GetInstalledUnix()
	// Copy metadata to avoid aliasing.
	if existing.GetMetadata() != nil {
		md := make(map[string]string, len(existing.Metadata))
		for k, vv := range existing.Metadata {
			md[k] = vv
		}
		patched.Metadata = md
	}
	if err := installed_state.WriteInstalledPackage(ctx, patched); err != nil {
		v.Verdict = "skipped"
		v.SkipReason = fmt.Sprintf("write_failed: %v", err)
		return v, fmt.Errorf("write: %w", err)
	}
	v.Verdict = "repaired"
	v.Applied = true
	v.ChecksumAfter = onDisk
	return v, nil
}

// candidateBinaryPath returns the first existing binary at one of the
// conventional paths for this package, or "" if none exists.
//
// The convention:
//
//	SERVICE → /usr/lib/globular/bin/<name>_server (hyphens → underscores)
//	         → /usr/lib/globular/bin/<name>       (fallback, e.g. mcp)
//	INFRASTRUCTURE / COMMAND → /usr/lib/globular/bin/<name>
//	                          → /usr/lib/globular/bin/<name_with_underscores>
func candidateBinaryPath(name, kind string) string {
	underscored := strings.ReplaceAll(name, "-", "_")
	candidates := []string{
		filepath.Join(defaultBinaryDir, underscored+"_server"),
		filepath.Join(defaultBinaryDir, name+"_server"),
		filepath.Join(defaultBinaryDir, name),
		filepath.Join(defaultBinaryDir, underscored),
	}
	if strings.EqualFold(kind, "SERVICE") {
		// SERVICE prefers _server first.
	} else {
		// INFRASTRUCTURE/COMMAND prefer plain name first.
		candidates = []string{
			filepath.Join(defaultBinaryDir, name),
			filepath.Join(defaultBinaryDir, underscored),
			filepath.Join(defaultBinaryDir, underscored+"_server"),
			filepath.Join(defaultBinaryDir, name+"_server"),
		}
	}
	for _, p := range candidates {
		if fi, err := os.Stat(p); err == nil && !fi.IsDir() {
			return p
		}
	}
	return ""
}

// hashFile returns the lowercase hex sha256 of the file at path.
func hashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// readCASManifestEntrypointChecksum finds the CAS manifest file for the given
// package + version on linux_amd64 and returns the entrypointChecksum value
// stripped of the sha256: prefix.
func readCASManifestEntrypointChecksum(name, version string) (string, error) {
	entries, err := os.ReadDir(repositoryCASDir)
	if err != nil {
		return "", fmt.Errorf("read %s: %w", repositoryCASDir, err)
	}
	prefix := fmt.Sprintf("core@globular.io%%%s%%%s%%linux_amd64%%", name, version)
	for _, e := range entries {
		n := e.Name()
		if !strings.HasPrefix(n, prefix) || !strings.HasSuffix(n, ".manifest.json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(repositoryCASDir, n))
		if err != nil {
			return "", fmt.Errorf("read manifest %s: %w", n, err)
		}
		var m struct {
			EntrypointChecksum string `json:"entrypointChecksum"`
		}
		if err := json.Unmarshal(data, &m); err != nil {
			return "", fmt.Errorf("parse manifest %s: %w", n, err)
		}
		ck := m.EntrypointChecksum
		if i := strings.Index(ck, ":"); i >= 0 {
			ck = ck[i+1:]
		}
		return strings.ToLower(strings.TrimSpace(ck)), nil
	}
	return "", fmt.Errorf("no CAS manifest found for %s@%s linux_amd64", name, version)
}

// init silences global log noise from imported packages so the tool's JSON
// output is clean on stdout.
func init() {
	log.SetOutput(io.Discard)
}
