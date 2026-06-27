package main

import (
	"log"
	"os"
	"path/filepath"
	"syscall"

	"github.com/globulario/services/golang/runtimedirs"
)

// defaultStateRoot is the runtime state root every service materializes its
// per-service directory under. Canonicalization sweeps this tree.
const defaultStateRoot = "/var/lib/globular"

// CanonicalizeRuntimeDirsOnce permanently retires legacy underscore (and
// no-separator) runtime-dir aliases under stateRoot by folding their contents
// into the canonical hyphenated directory, then removing the now-empty legacy
// dir. cluster-doctor is the DETECTOR of this drift; node-agent is the
// SELF-HEALER — running this on every startup.
//
// The alias model is the SHARED one owned by runtimedirs (no second
// handwritten list). For each canonical→legacy pair:
//
//  1. If the legacy dir does not exist → nothing to do.
//  2. Ensure the canonical dir exists (created preserving the legacy dir's
//     mode + ownership when it must be created).
//  3. Move each legacy entry into the canonical dir.
//  4. Never overwrite an existing canonical entry — on conflict, log it and
//     leave the legacy dir intact for operator review.
//  5. Remove the legacy dir only if it ends up empty, using a non-recursive
//     rmdir (os.Remove on a dir fails rather than deleting non-empty content).
//
// Idempotent and safe to run on every node-agent start: once a legacy dir is
// gone, its pair is skipped; conflicts are left in place rather than retried
// destructively. Honors invariants service.runtime_dir_name_must_be_canonical
// and migration.state_path_upgrade_must_be_idempotent.
func CanonicalizeRuntimeDirsOnce(stateRoot string) {
	if stateRoot == "" {
		return
	}
	for canonical, legacies := range runtimedirs.CanonicalToLegacy() {
		canonPath := filepath.Join(stateRoot, canonical)
		for _, legacy := range legacies {
			legacyPath := filepath.Join(stateRoot, legacy)
			if legacyPath == canonPath {
				continue
			}
			canonicalizeOneRuntimeDir(canonPath, legacyPath)
		}
	}
}

// canonicalizeOneRuntimeDir folds a single legacy alias dir into its canonical
// counterpart. Best-effort: every failure is logged and left non-fatal so a
// stuck pair can never block node-agent startup.
func canonicalizeOneRuntimeDir(canonPath, legacyPath string) {
	li, err := os.Lstat(legacyPath)
	if err != nil || !li.IsDir() {
		// Legacy dir absent (or a non-dir we must not touch) → nothing to do.
		return
	}

	// Ensure the canonical dir exists, preserving the legacy dir's perms and
	// ownership when we have to create it (the service that owns it runs as a
	// non-root user; a root-created dir would be unwritable to it).
	if _, err := os.Stat(canonPath); os.IsNotExist(err) {
		if err := os.MkdirAll(canonPath, li.Mode().Perm()); err != nil {
			log.Printf("runtime-dir-canonicalize: WARN create canonical %s: %v — legacy %s left in place", canonPath, err, legacyPath)
			return
		}
		chownLikeRuntimeDir(canonPath, li)
		log.Printf("runtime-dir-canonicalize: created canonical %s for legacy alias %s", canonPath, legacyPath)
	} else if err != nil {
		log.Printf("runtime-dir-canonicalize: WARN stat canonical %s: %v — legacy %s left in place", canonPath, err, legacyPath)
		return
	}

	entries, err := os.ReadDir(legacyPath)
	if err != nil {
		log.Printf("runtime-dir-canonicalize: WARN read legacy %s: %v", legacyPath, err)
		return
	}

	unresolved := false
	for _, e := range entries {
		src := filepath.Join(legacyPath, e.Name())
		dst := filepath.Join(canonPath, e.Name())
		if _, err := os.Lstat(dst); err == nil {
			// Canonical already has this entry — never overwrite silently.
			log.Printf("runtime-dir-canonicalize: CONFLICT %s already exists; leaving legacy copy %s and dir %s intact", dst, src, legacyPath)
			unresolved = true
			continue
		} else if !os.IsNotExist(err) {
			log.Printf("runtime-dir-canonicalize: WARN stat %s: %v — leaving legacy %s intact", dst, err, legacyPath)
			unresolved = true
			continue
		}
		if err := os.Rename(src, dst); err != nil {
			log.Printf("runtime-dir-canonicalize: WARN move %s -> %s: %v — leaving legacy %s intact", src, dst, err, legacyPath)
			unresolved = true
		}
	}

	if unresolved {
		// At least one entry could not be migrated — leave the legacy dir for
		// the operator rather than removing a non-empty dir or losing data.
		return
	}

	// Safe rmdir: os.Remove on a directory removes it ONLY if empty (it returns
	// an error rather than deleting content), giving us non-recursive semantics.
	if err := os.Remove(legacyPath); err != nil {
		log.Printf("runtime-dir-canonicalize: WARN rmdir legacy %s: %v", legacyPath, err)
		return
	}
	log.Printf("runtime-dir-canonicalize: removed legacy alias dir %s (canonical %s is authoritative)", legacyPath, canonPath)
}

// chownLikeRuntimeDir best-effort applies fi's uid/gid to path so a freshly
// created canonical dir keeps the same ownership as the legacy dir it replaces.
// Failures are intentionally silent — ownership is a refinement, not a gate.
func chownLikeRuntimeDir(path string, fi os.FileInfo) {
	if st, ok := fi.Sys().(*syscall.Stat_t); ok {
		_ = os.Chown(path, int(st.Uid), int(st.Gid))
	}
}
