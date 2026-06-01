// @awareness namespace=globular.platform
// @awareness component=platform_node_agent.pki
// @awareness file_role=node_secret_and_certificate_collector
// @awareness implements=globular.platform:intent.dns_pki.explicit_identity_over_convenient_routing
// @awareness risk=high
package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/globulario/services/golang/node_agent/node_agentpb"
)

// secretCollectorBackupRoot is the only directory under which CollectBackupSecrets
// will write. Any capsule_dir not canonically inside this root is rejected.
const secretCollectorBackupRoot = "/var/lib/globular/backups"

// secretCollectorAllowlistEntry describes one file that the collector will
// try to copy. The whole list is hardcoded in this file — no caller can
// extend it, no globs, no patterns, no symlink traversal. Auditable as
// source code.
type secretCollectorAllowlistEntry struct {
	// Absolute path on the local filesystem.
	Path string
	// CapsuleRelpath is the file name under <node_id>/. Slashes in the
	// original path are flattened to "__" so the directory stays flat.
	CapsuleRelpath string
	// Required: when true, missing-required => failure (unless OptionalWhenAbsent).
	Required bool
	// OptionalWhenAbsent: when true, source-not-present is tolerated even when Required.
	// Use case: .bootstrap-sa-password only exists on the bootstrap node; on
	// the other 4 nodes the absence is normal.
	OptionalWhenAbsent bool
	// ProducedBy: free-text label of the writer process — informational only.
	ProducedBy string
}

// secretCollectorAllowlist is the canonical, strict set of files the
// privileged collector may read. Constraints (enforced by tests):
//   - exactly 4 entries
//   - every Path is absolute and contains no glob metacharacters
//   - every Path lives under /var/lib/globular/
var secretCollectorAllowlist = []secretCollectorAllowlistEntry{
	{
		Path:               "/var/lib/globular/.bootstrap-sa-password",
		CapsuleRelpath:     "bootstrap-sa-password",
		Required:           false,
		OptionalWhenAbsent: true,
		ProducedBy:         "install-day0.sh / ensure-bootstrap-artifacts.sh",
	},
	{
		Path:               "/var/lib/globular/ingress/spec-last-known-good.json",
		CapsuleRelpath:     "ingress__spec-last-known-good.json",
		Required:           true,
		OptionalWhenAbsent: true,
		ProducedBy:         "cluster_controller / xds reconcile (root-side write)",
	},
	{
		Path:               "/var/lib/globular/objectstore/minio_contract-last-known-good.json",
		CapsuleRelpath:     "objectstore__minio_contract-last-known-good.json",
		Required:           true,
		OptionalWhenAbsent: false,
		ProducedBy:         "node_agent minio reconcile (root-side write)",
	},
	{
		Path:               "/var/lib/globular/xds/config-last-known-good.json",
		CapsuleRelpath:     "xds__config-last-known-good.json",
		Required:           true,
		OptionalWhenAbsent: true,
		ProducedBy:         "xds / cluster_controller (root-side write)",
	},
}

// secretCollectorClock is overridable in tests for deterministic
// collected_at_unix values.
var secretCollectorClock = func() time.Time { return time.Now() }

// secretCollectorPrimaryIPFn is overridable in tests so the result doesn't
// depend on host network state.
var secretCollectorPrimaryIPFn = nodeRoutableIP

// validateCapsuleDir checks that the caller-supplied capsule_dir is safe to
// write to:
//   - absolute
//   - canonical (Clean(x) == x — rejects "..", duplicate "/", etc.)
//   - resolves (after Lstat on the chain) to a path under
//     secretCollectorBackupRoot — no symlink escape
//
// Returns (canonicalDir, nil) on success or an error describing the failure.
// Does NOT create the directory; the caller does that under controlled mode.
func validateCapsuleDir(capsuleDir string) (string, error) {
	if capsuleDir == "" {
		return "", errors.New("capsule_dir must not be empty")
	}
	if !filepath.IsAbs(capsuleDir) {
		return "", fmt.Errorf("capsule_dir %q must be absolute", capsuleDir)
	}
	cleaned := filepath.Clean(capsuleDir)
	if cleaned != capsuleDir {
		return "", fmt.Errorf("capsule_dir %q is not canonical (clean=%q); reject to prevent path-traversal", capsuleDir, cleaned)
	}
	if strings.Contains(capsuleDir, "..") {
		return "", fmt.Errorf("capsule_dir %q contains '..' — rejected", capsuleDir)
	}
	// Walk each parent: if any component along the path is a symlink, the
	// directory could resolve outside our allowed root. We accept that the
	// final directory may not exist yet (the backup_manager creates it just
	// before the call), but no ancestor may be a symlink that escapes.
	if err := assertNoSymlinkEscape(cleaned, secretCollectorBackupRoot); err != nil {
		return "", err
	}
	// String-prefix check is correct because both sides have been cleaned and
	// confirmed symlink-free along the chain.
	if cleaned != secretCollectorBackupRoot && !strings.HasPrefix(cleaned, secretCollectorBackupRoot+"/") {
		return "", fmt.Errorf("capsule_dir %q is outside %s", cleaned, secretCollectorBackupRoot)
	}
	return cleaned, nil
}

// assertNoSymlinkEscape walks each ancestor of capsuleDir (starting from
// allowedRoot, descending towards capsuleDir) and refuses if any segment
// is a symlink. We don't require the final directory to exist.
func assertNoSymlinkEscape(capsuleDir, allowedRoot string) error {
	// Both inputs are already cleaned. If capsuleDir is not under allowedRoot
	// the string-prefix check after this will catch it; here we just verify
	// the realpath of each existing ancestor stays inside allowedRoot.
	// Walk components: /var, /var/lib, /var/lib/globular, /var/lib/globular/backups, ...
	parts := strings.Split(capsuleDir, string(os.PathSeparator))
	cur := ""
	for _, p := range parts {
		if p == "" {
			cur = "/"
			continue
		}
		if cur == "/" {
			cur = "/" + p
		} else {
			cur = cur + "/" + p
		}
		info, err := os.Lstat(cur)
		if err != nil {
			// Ancestor doesn't exist — fine. The final dir may be created by
			// the caller; we only enforce the existing-ancestor chain.
			if errors.Is(err, os.ErrNotExist) {
				return nil
			}
			return fmt.Errorf("lstat %s: %w", cur, err)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			target, _ := os.Readlink(cur)
			return fmt.Errorf("ancestor %s is a symlink (target=%q); refusing to write through symlink chain", cur, target)
		}
	}
	return nil
}

// flattenForCapsule maps an absolute source path to a flat capsule filename.
// Currently identical to the allowlist's CapsuleRelpath but exposed for
// symmetry and future use.
func flattenForCapsule(originalPath string) string {
	// Stripped of leading "/var/lib/globular/" and slashes replaced with "__".
	const prefix = "/var/lib/globular/"
	if strings.HasPrefix(originalPath, prefix) {
		return strings.ReplaceAll(originalPath[len(prefix):], "/", "__")
	}
	return strings.ReplaceAll(strings.TrimPrefix(originalPath, "/"), "/", "__")
}

// statOwnerGroup looks up the owner and group names from a Stat_t.
// Falls back to numeric strings when /etc/passwd or /etc/group lookups fail
// (e.g. inside containerized tests where uids don't resolve).
func statOwnerGroup(st *syscall.Stat_t) (owner, group string) {
	if u, err := user.LookupId(strconv.FormatUint(uint64(st.Uid), 10)); err == nil {
		owner = u.Username
	} else {
		owner = strconv.FormatUint(uint64(st.Uid), 10)
	}
	if g, err := user.LookupGroupId(strconv.FormatUint(uint64(st.Gid), 10)); err == nil {
		group = g.Name
	} else {
		group = strconv.FormatUint(uint64(st.Gid), 10)
	}
	return owner, group
}

// chownToGlobularUser sets owner/group to globular:globular when the user
// exists on the system. Best-effort: when the user is missing (e.g. unit
// test sandbox), the file ownership is left at the caller's identity. The
// mode bits are still applied so dest files are at least mode-0640.
func chownToGlobularUser(path string) error {
	u, err := user.Lookup("globular")
	if err != nil {
		return nil // best-effort; missing user is acceptable in tests
	}
	uid, err := strconv.Atoi(u.Uid)
	if err != nil {
		return fmt.Errorf("parse globular uid: %w", err)
	}
	gid, err := strconv.Atoi(u.Gid)
	if err != nil {
		return fmt.Errorf("parse globular gid: %w", err)
	}
	return os.Chown(path, uid, gid)
}

// copyOneSecretFile reads src, writes to <destDir>/<destName> via tmp+rename,
// computes sha256 from the BYTES THAT WERE WRITTEN (not from a re-read of
// the source), and returns the entry metadata. Refuses to follow symlinks
// on the source side (Lstat + O_NOFOLLOW).
func copyOneSecretFile(src, destDir, destName string) (*node_agentpb.SecretFileEntry, error) {
	entry := &node_agentpb.SecretFileEntry{
		OriginalPath:   src,
		CapsuleRelpath: destName,
	}
	li, err := os.Lstat(src)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			entry.Found = false
			entry.Reason = "source file not present at collection time"
			return entry, nil
		}
		entry.Found = false
		entry.Reason = "lstat: " + err.Error()
		return entry, err
	}
	if li.Mode()&os.ModeSymlink != 0 {
		entry.Found = false
		entry.Reason = "source is a symlink; refused for safety"
		return entry, errors.New("symlink source refused: " + src)
	}
	if !li.Mode().IsRegular() {
		entry.Found = false
		entry.Reason = "source is not a regular file (" + li.Mode().String() + ")"
		return entry, errors.New("non-regular source refused: " + src)
	}
	st, ok := li.Sys().(*syscall.Stat_t)
	if ok {
		entry.ModeOctal = fmt.Sprintf("%04o", li.Mode().Perm())
		entry.Owner, entry.Group = statOwnerGroup(st)
	} else {
		entry.ModeOctal = fmt.Sprintf("%04o", li.Mode().Perm())
		entry.Owner, entry.Group = "?", "?"
	}

	// O_NOFOLLOW: if a symlink slipped in between Lstat and open, the open
	// fails with ELOOP rather than dereferencing.
	f, err := os.OpenFile(src, os.O_RDONLY|syscall.O_NOFOLLOW, 0)
	if err != nil {
		entry.Found = false
		entry.Reason = "open: " + err.Error()
		return entry, err
	}
	defer f.Close()

	tmpPath := filepath.Join(destDir, destName+".tmp")
	out, err := os.OpenFile(tmpPath, os.O_RDWR|os.O_CREATE|os.O_TRUNC|os.O_EXCL, 0o640)
	if err != nil {
		entry.Found = false
		entry.Reason = "create-tmp: " + err.Error()
		return entry, err
	}
	hash := sha256.New()
	written, err := io.Copy(io.MultiWriter(out, hash), f)
	if cerr := out.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		_ = os.Remove(tmpPath)
		entry.Found = false
		entry.Reason = "copy: " + err.Error()
		return entry, err
	}
	if err := os.Rename(tmpPath, filepath.Join(destDir, destName)); err != nil {
		_ = os.Remove(tmpPath)
		entry.Found = false
		entry.Reason = "rename: " + err.Error()
		return entry, err
	}
	// Force mode 0640 explicitly (umask may have downgraded it).
	if err := os.Chmod(filepath.Join(destDir, destName), 0o640); err != nil {
		return entry, fmt.Errorf("chmod 0640: %w", err)
	}
	// Best-effort chown to globular so backup_manager (user=globular) can
	// read the file. Failure is logged-and-tolerated.
	if err := chownToGlobularUser(filepath.Join(destDir, destName)); err != nil {
		slog.Warn("secret-collector: chown to globular failed (best-effort)",
			"path", filepath.Join(destDir, destName), "error", err.Error())
	}
	entry.Found = true
	entry.SizeBytes = uint64(written)
	entry.Sha256 = hex.EncodeToString(hash.Sum(nil))
	return entry, nil
}

// CollectBackupSecrets is the gRPC handler. Local-only: reads from the
// node's own filesystem, writes to a node-scoped subdirectory inside the
// caller-supplied capsule_dir. Does NOT touch peers.
func (srv *NodeAgentServer) CollectBackupSecrets(ctx context.Context, req *node_agentpb.CollectBackupSecretsRequest) (*node_agentpb.CollectBackupSecretsResponse, error) {
	if req == nil {
		return nil, errors.New("nil request")
	}

	capsuleDir, err := validateCapsuleDir(req.GetCapsuleDir())
	if err != nil {
		return nil, fmt.Errorf("capsule_dir validation failed: %w", err)
	}
	if srv.nodeID == "" {
		return nil, errors.New("local node_id is empty; cannot scope capsule destination")
	}
	dest := filepath.Join(capsuleDir, "payload", "secrets", srv.nodeID)
	if err := os.MkdirAll(dest, 0o750); err != nil {
		return nil, fmt.Errorf("mkdir capsule secrets dest: %w", err)
	}
	// Best-effort chown the destination dir so the backup_manager user can
	// walk it. If the user doesn't exist (test env), we leave it.
	if err := chownToGlobularUser(dest); err != nil {
		slog.Warn("secret-collector: chown dest dir to globular failed (best-effort)",
			"path", dest, "error", err.Error())
	}

	hostname, _ := os.Hostname()
	primaryIP := secretCollectorPrimaryIPFn()

	resp := &node_agentpb.CollectBackupSecretsResponse{
		NodeId:           srv.nodeID,
		Hostname:         hostname,
		PrimaryIp:        primaryIP,
		NodeAgentVersion: srv.agentVersion,
		CollectedAtUnix:  fmt.Sprintf("%d", secretCollectorClock().Unix()),
		PerNodeManifest:  filepath.Join("payload", "secrets", srv.nodeID, "manifest.json"),
	}

	for _, item := range secretCollectorAllowlist {
		entry, copyErr := copyOneSecretFile(item.Path, dest, item.CapsuleRelpath)
		entry.Required = item.Required
		entry.OptionalWhenAbsent = item.OptionalWhenAbsent
		entry.ProducedBy = item.ProducedBy

		// Log at INFO; never include file content.
		slog.Info("secret-collector: entry processed",
			"original_path", entry.OriginalPath,
			"found", entry.Found,
			"required", entry.Required,
			"size_bytes", entry.SizeBytes,
			"sha256_prefix", sha256Prefix(entry.Sha256),
			"reason", entry.Reason,
		)

		// Per-entry classification:
		//   - found=true              → entry succeeds
		//   - found=false + optional  → missing_optional
		//   - found=false + required + OptionalWhenAbsent → missing_optional
		//   - found=false + required + NOT OptionalWhenAbsent → missing_required
		//   - copyErr on a required path that exists but failed → missing_required
		if !entry.Found {
			if item.Required && !item.OptionalWhenAbsent {
				resp.MissingRequired = append(resp.MissingRequired, item.Path)
			} else {
				resp.MissingOptional = append(resp.MissingOptional, item.Path)
			}
		} else if copyErr != nil {
			// Found but copy failed mid-way — treat as missing.
			if item.Required {
				resp.MissingRequired = append(resp.MissingRequired, item.Path)
			} else {
				resp.MissingOptional = append(resp.MissingOptional, item.Path)
			}
		}
		resp.Entries = append(resp.Entries, entry)
	}

	// Write the per-node manifest.json. We use a small inline JSON encoder
	// to keep the schema stable and to avoid a runtime dependency on
	// protojson's wire-format defaults (the file is meant to be read by
	// hand and by the restore script).
	manifestPath := filepath.Join(dest, "manifest.json")
	if err := writePerNodeManifest(manifestPath, resp); err != nil {
		return nil, fmt.Errorf("write per-node manifest: %w", err)
	}
	if err := chownToGlobularUser(manifestPath); err != nil {
		slog.Warn("secret-collector: chown manifest to globular failed (best-effort)",
			"path", manifestPath, "error", err.Error())
	}
	// Sidecar hostname file — human-readable inspection aid.
	_ = os.WriteFile(filepath.Join(dest, "hostname.txt"), []byte(hostname+"\n"), 0o640)
	_ = os.WriteFile(filepath.Join(dest, "node_agent_version.txt"), []byte(srv.agentVersion+"\n"), 0o640)

	return resp, nil
}

// sha256Prefix returns the first 12 chars of the hex digest for logging.
// The full digest goes in the manifest; logs only need an identifier.
func sha256Prefix(full string) string {
	if len(full) <= 12 {
		return full
	}
	return full[:12]
}
