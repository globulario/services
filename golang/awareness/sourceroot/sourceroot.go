// Package sourceroot is the single source of truth for "is a real source
// tree available to scan from this process?"
//
// Awareness has several consumers that all need to walk the Go source —
// test discovery, Go-file coverage, integrity check, scan_violations,
// future scanners. Before this package, each consumer invented its own
// degraded signal (cwd fallback, empty string, "unverified" string,
// 0/0 percent, …) and each downstream report collapsed those signals
// into different — usually critical — buckets. The composed result on
// a production MCP host (no source on disk) was a wall of false
// "missing tests" / "0% coverage critical" alerts.
//
// This package fixes the shape by replacing every per-call sentinel
// with one discriminated state:
//
//	Found(path)           — a verified, walkable source tree
//	Absent                — no source tree at all (production host)
//	Inaccessible(err)     — a path exists but isn't readable
//	WrongContext(p,reason) — a path exists but is not what was asked for
//
// Consumers switch on the typed state. The package is intentionally
// strict: there is no fallback to os.Getwd() and no string sentinel.
// Non-Found states are *telemetry signals*, never evidence of missing
// code.
//
// See docs/awareness/composed_path_failures.md (2026-05-14 entry) for
// the incident log that justifies this package, and the invariant
// awareness.source_scan_requires_verified_repo_root for the enforcement
// contract.
package sourceroot

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// State enumerates the discriminated result of a source-root resolution.
type State int

const (
	// Found means we have a verified, walkable source tree.
	Found State = iota
	// Absent means no source tree is available from this process.
	// On a production MCP host this is the expected, calm state — not
	// an error. Consumers should report "telemetry unavailable" at
	// info severity, never "missing code" at critical severity.
	Absent
	// Inaccessible means a candidate path exists but cannot be read
	// (permission denied, broken symlink, etc.). Distinguished from
	// Absent because the operator may want to chmod or run as a
	// different user.
	Inaccessible
	// WrongContext means a candidate path exists but is not the right
	// tree — for example, an explicit hint pointed at an install dir
	// instead of a checkout, or a git rev-parse from inside a vendored
	// tree returned a sub-repo. Reason explains why.
	WrongContext
)

// String returns a stable, lower-case label for the state. Suitable for
// JSON output, log fields, and switch statements.
func (s State) String() string {
	switch s {
	case Found:
		return "found"
	case Absent:
		return "absent"
	case Inaccessible:
		return "inaccessible"
	case WrongContext:
		return "wrong_context"
	default:
		return "unknown"
	}
}

// Result is the typed answer to "give me a source root."
//
// Callers MUST switch on State before reading Path. A non-Found result
// must never be mapped to a critical-severity finding about missing
// code; see forbidden_fix collapse_source_absent_into_critical_missing_evidence.
type Result struct {
	State  State
	Path   string // valid only when State == Found
	Err    error  // populated when State == Inaccessible
	Reason string // populated when State == WrongContext or as extra context
}

// IsAvailable returns true iff Result.State == Found. Convenience helper
// for the common case where a caller only needs a yes/no.
func (r Result) IsAvailable() bool { return r.State == Found }

// Options control how Resolve looks for a source root.
type Options struct {
	// ExplicitPath, if non-empty, is tried first. Useful for MCP
	// configurations that set awareness.repo_path explicitly. If the
	// path is not a directory, Resolve returns Inaccessible — it does
	// NOT silently fall back to git discovery, since the caller's
	// explicit intent should never be overridden by ambient state.
	ExplicitPath string

	// AllowGitDiscovery, when true, lets Resolve try `git rev-parse
	// --show-toplevel` in the current process directory. Defaults to
	// true; set to false for callers that want strictly-explicit
	// resolution.
	AllowGitDiscovery bool
}

// DefaultOptions is the convenience value for the common case: no
// explicit path, git discovery allowed.
var DefaultOptions = Options{AllowGitDiscovery: true}

// Resolve returns a typed Result describing whether a source tree is
// available. It never panics, never falls back to os.Getwd(), and never
// returns Found unless the path is a directory the calling process can
// stat.
func Resolve(opts Options) Result {
	if path := strings.TrimSpace(opts.ExplicitPath); path != "" {
		return resolveExplicit(path)
	}
	if opts.AllowGitDiscovery {
		if r, ok := resolveGit(); ok {
			return r
		}
	}
	return Result{State: Absent, Reason: "no explicit path and no enclosing git checkout"}
}

func resolveExplicit(path string) Result {
	info, err := os.Stat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Result{State: Inaccessible, Path: path, Err: fmt.Errorf("path does not exist: %w", err)}
		}
		return Result{State: Inaccessible, Path: path, Err: err}
	}
	if !info.IsDir() {
		return Result{State: WrongContext, Path: path, Reason: "explicit path is a file, not a directory"}
	}
	abs, absErr := filepath.Abs(path)
	if absErr != nil {
		abs = path
	}
	return Result{State: Found, Path: abs}
}

func resolveGit() (Result, bool) {
	out, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		// Not in a git checkout, or git not installed. Caller falls
		// through to Absent.
		return Result{}, false
	}
	root := strings.TrimSpace(string(out))
	if root == "" {
		return Result{}, false
	}
	info, err := os.Stat(root)
	if err != nil {
		return Result{State: Inaccessible, Path: root, Err: err}, true
	}
	if !info.IsDir() {
		return Result{State: WrongContext, Path: root, Reason: "git toplevel is not a directory"}, true
	}
	return Result{State: Found, Path: root}, true
}
