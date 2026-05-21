#!/usr/bin/env python3
"""detect-changes.py — BOM change detection for Globular platform releases.

Computes a package_contract_digest for each package and compares it against
the previous release index. The contract digest is version-independent: it
covers binary checksum (built with a fixed sentinel version), normalized
package metadata (excluding mutable fields), specs, systemd units, profiles,
hard_deps, provides, requires, and defaults.

CRITICAL: Go service binaries MUST be built with a fixed sentinel version
(e.g. 0.0.0-detect) BEFORE running this script. This ensures that version
string changes alone do not produce false positives in change detection.

Outputs:
  1. version-overrides.txt  — for gen-version.sh (unchanged Go services keep old version)
  2. change-manifest.json   — full change report for the release builder

Usage:
  python3 detect-changes.py \\
    --prev-index dist/prev-release-index.json \\
    --metadata-dir packages/metadata \\
    --bin-dir dist/bin \\
    --pkg-map-json services/golang/build/pkg-map.json \\
    --version 1.0.85 \\
    --tag v1.0.85 \\
    --output-overrides dist/version-overrides.txt \\
    --output-manifest dist/change-manifest.json \\
    [--force-full-rebuild] [--force-reason "reason"]

Exit codes:
  0 — success
  1 — error (missing required files, invalid input)
"""

import argparse
import hashlib
import json
import os
import sys


# ── Hashing helpers ────────────────────────────────────────────────────────

def sha256_file(path):
    """SHA256 of a file. Returns 'sha256:<hex>' or '' if missing."""
    try:
        h = hashlib.sha256()
        with open(path, "rb") as f:
            for chunk in iter(lambda: f.read(8192), b""):
                h.update(chunk)
        return f"sha256:{h.hexdigest()}"
    except FileNotFoundError:
        return ""


def sha256_dir(dirpath):
    """Combined SHA256 of all files in a directory (sorted by name).

    Returns '' if the directory doesn't exist or contains no files.
    """
    if not os.path.isdir(dirpath):
        return ""
    h = hashlib.sha256()
    found = False
    for fname in sorted(os.listdir(dirpath)):
        fpath = os.path.join(dirpath, fname)
        if not os.path.isfile(fpath):
            continue
        found = True
        h.update(fname.encode())
        h.update(b"\x00")
        with open(fpath, "rb") as f:
            for chunk in iter(lambda: f.read(8192), b""):
                h.update(chunk)
        h.update(b"\x00")
    return f"sha256:{h.hexdigest()}" if found else ""


def sha256_source_tree(dirpath, extensions=(".go",)):
    """Combined SHA256 of all source files in a directory tree (recursive).

    Used for Go services instead of binary checksums: CGO makes binaries
    non-reproducible across CI runs (different linker, glibc, runner image).
    Source hashing is deterministic regardless of build environment.

    Returns '' if no matching files found.
    """
    if not os.path.isdir(dirpath):
        return ""
    h = hashlib.sha256()
    found = False
    for root, _dirs, files in sorted(os.walk(dirpath)):
        for fname in sorted(files):
            if not any(fname.endswith(ext) for ext in extensions):
                continue
            fpath = os.path.join(root, fname)
            relpath = os.path.relpath(fpath, dirpath)
            found = True
            h.update(relpath.encode())
            h.update(b"\x00")
            with open(fpath, "rb") as f:
                for chunk in iter(lambda: f.read(8192), b""):
                    h.update(chunk)
            h.update(b"\x00")
    return f"sha256:{h.hexdigest()}" if found else ""


# Shared Go packages that affect all consumers. Changes here mark ALL
# Go services as changed.
#
# The list is conservative — only include packages that are imported by
# multiple service binaries AND whose changes are not detectable by
# hashing each service's own source tree. The v1.2.59 incident: a fix
# to golang/verifier/ silently shipped only into cluster_controller
# (own source changed for unrelated reasons) and was skipped for
# cluster_doctor (own source unchanged), leaving the doctor stuck
# emitting old bootstrap_ordering_skew severities and the UI red on
# every fresh install. The correct long-term fix is to walk
# `go list -deps` per service; until then, keep this list current with
# any package consumed by more than one binary.
SHARED_GO_PACKAGES = [
    "globular_service",
    "interceptors",
    "config",
    "security",
    "dephealth",
    "subsystem",
    # Phase 9 (Diagnostic Honesty Refactor) shared evidence + verdict
    # packages — consumed by cluster_doctor (sweep) and cluster_controller
    # (release pipeline + version health). Forgetting these caused the
    # v1.2.59 regression above.
    "verifier",
    "crossnodedrift",
    "fallback",
    "installed_state",
]


def normalized_manifest_sha256(pkg_json_path):
    """Hash package.json excluding fields overwritten at package time.

    Excluded: version, build_id, entrypoint_checksum.
    These change with every release and must not trigger change detection.
    """
    try:
        with open(pkg_json_path) as f:
            m = json.load(f)
    except (FileNotFoundError, json.JSONDecodeError):
        return ""

    stable = {k: v for k, v in m.items()
              if k not in ("version", "build_id", "entrypoint_checksum")}
    canonical = json.dumps(stable, sort_keys=True, separators=(",", ":"))
    return f"sha256:{hashlib.sha256(canonical.encode()).hexdigest()}"


def contract_digest(c):
    """Deterministic digest of all contract-relevant fields."""
    h = hashlib.sha256()

    def w(label, value):
        h.update(f"{label}\x00{value}\x00".encode())

    w("entrypoint", c.get("entrypoint_checksum", ""))
    w("manifest",   c.get("manifest_sha256", ""))
    w("spec",       c.get("spec_sha256", ""))
    w("systemd",    c.get("systemd_sha256", ""))

    for p in sorted(c.get("profiles", [])):
        w("profile", p)
    for d in sorted(c.get("hard_deps", [])):
        w("hard_dep", d)
    for p in sorted(c.get("provides", [])):
        w("provides", p)
    for r in sorted(c.get("requires", [])):
        w("requires", r)
    for k in sorted(c.get("defaults", {})):
        w(f"default:{k}", json.dumps(c["defaults"][k], sort_keys=True))

    return f"sha256:{h.hexdigest()}"


# ── Previous index loader ─────────────────────────────────────────────────

def load_prev_index(path):
    """Load previous release index, return {name: entry}."""
    try:
        with open(path) as f:
            idx = json.load(f)
    except (FileNotFoundError, json.JSONDecodeError):
        return {}
    return {p["name"]: p for p in idx.get("packages", []) if p.get("name")}


# ── Main ───────────────────────────────────────────────────────────────────

def main():
    ap = argparse.ArgumentParser(description="BOM change detection for Globular releases")
    ap.add_argument("--prev-index",        required=True, help="Path to previous release-index.json")
    ap.add_argument("--metadata-dir",      required=True, help="Path to packages/metadata/")
    ap.add_argument("--bin-dir",           required=True, help="Path to dist/bin/")
    ap.add_argument("--pkg-map-json",      required=True, help="Path to pkg-map.json")
    ap.add_argument("--go-src-dir",       default="",    help="Path to golang/ source root (for source-based change detection)")
    ap.add_argument("--version",           required=True, help="Current platform version (e.g. 1.0.85)")
    ap.add_argument("--tag",               required=True, help="Current release tag (e.g. v1.0.85)")
    ap.add_argument("--output-overrides",  required=True, help="Output: version-overrides.txt")
    ap.add_argument("--output-manifest",   required=True, help="Output: change-manifest.json")
    ap.add_argument("--force-full-rebuild", action="store_true", help="Treat all packages as changed")
    ap.add_argument("--force-reason",      default="",    help="Reason for force rebuild")
    args = ap.parse_args()

    prev = load_prev_index(args.prev_index)

    try:
        with open(args.pkg_map_json) as f:
            pkg_map = json.load(f)
    except (FileNotFoundError, json.JSONDecodeError) as e:
        print(f"ERROR: cannot load pkg-map-json: {e}", file=sys.stderr)
        sys.exit(1)

    results = []

    # Precompute shared Go package hashes. Changes to these mark ALL Go
    # services as changed (they're imported by most/all services).
    go_src = args.go_src_dir
    shared_hash = ""
    if go_src:
        sh = hashlib.sha256()
        for pkg in sorted(SHARED_GO_PACKAGES):
            pkg_dir = os.path.join(go_src, pkg)
            h = sha256_source_tree(pkg_dir)
            sh.update(f"{pkg}\x00{h}\x00".encode())
        # Also include go.sum — dependency changes affect all binaries.
        gosum = sha256_file(os.path.join(go_src, "go.sum"))
        sh.update(f"go.sum\x00{gosum}\x00".encode())
        shared_hash = f"sha256:{sh.hexdigest()}"

    for name, info in sorted(pkg_map.items()):
        binary     = info["binary"]
        kind       = info.get("kind", "service")
        go_target  = info.get("go_target", "")
        plat_ver   = info.get("platform_version", True)

        meta = os.path.join(args.metadata_dir, name)
        if not os.path.isdir(meta):
            print(f"  SKIP {name}: no metadata dir at {meta}", file=sys.stderr)
            continue

        bin_path = os.path.join(args.bin_dir, binary)
        if not os.path.isfile(bin_path):
            print(f"  SKIP {name}: binary {binary} not found in {args.bin_dir}", file=sys.stderr)
            continue

        # Load manifest for contract fields.
        pj_path = os.path.join(meta, "package.json")
        try:
            with open(pj_path) as f:
                manifest = json.load(f)
        except (FileNotFoundError, json.JSONDecodeError):
            manifest = {}

        # Compute contract components.
        #
        # For Go services (go_target set): use SOURCE file hashing instead of
        # binary checksums. CGO_ENABLED=1 makes Go binaries non-reproducible
        # across CI runs (different linker, glibc, runner image). Source hashing
        # is deterministic regardless of build environment.
        #
        # For third-party packages (no go_target): use binary hash as before.
        # Downloaded binaries are reproducible (same URL = same file).
        if go_target and go_src:
            src_dir = os.path.join(go_src, go_target)
            src_hash = sha256_source_tree(src_dir)
            # Combine service source + shared packages + go.sum
            combined = hashlib.sha256()
            combined.update(f"src\x00{src_hash}\x00".encode())
            combined.update(f"shared\x00{shared_hash}\x00".encode())
            cksum = f"sha256:{combined.hexdigest()}"
        else:
            cksum = sha256_file(bin_path)

        mhash = normalized_manifest_sha256(pj_path)
        shash = sha256_dir(os.path.join(meta, "specs"))
        uhash = sha256_dir(os.path.join(meta, "systemd"))

        cd = contract_digest({
            "entrypoint_checksum": cksum,
            "manifest_sha256":    mhash,
            "spec_sha256":        shash,
            "systemd_sha256":     uhash,
            "profiles":  manifest.get("profiles", []),
            "hard_deps": manifest.get("hard_deps", []),
            "provides":  manifest.get("provides_capabilities", []),
            "requires":  manifest.get("requires", []),
            "defaults":  manifest.get("defaults", {}),
        })

        # Compare against previous release.
        p       = prev.get(name, {})
        prev_cd = p.get("package_contract_digest", "")

        if prev_cd and prev_cd == cd and not args.force_full_rebuild:
            # ── Unchanged: carry forward from previous release ──
            results.append({
                "name":                    name,
                "kind":                    p.get("kind", kind),
                "go_target":               go_target,
                "platform_version":        plat_ver,
                "changed":                 False,
                "version":                 p.get("version", ""),
                "origin_release":          p.get("origin_release", p.get("release_tag", "")),
                "package_contract_digest": cd,
                "entrypoint_checksum":     cksum,
                "asset_url":               p.get("asset_url", ""),
                "package_digest":          p.get("package_digest", ""),
                "build_number":            p.get("build_number", 0),
                "build_id":                str(p.get("build_id", "")),
                "filename":                p.get("filename", ""),
                "profiles":                manifest.get("profiles", []),
                "publisher":               manifest.get("publisher", ""),
            })
        else:
            # ── Changed: assign version based on version_source ──
            if plat_ver:
                pkg_version = args.version  # platform version
            else:
                pkg_version = manifest.get("version", "")  # upstream (from metadata)

            results.append({
                "name":                    name,
                "kind":                    kind,
                "go_target":               go_target,
                "platform_version":        plat_ver,
                "changed":                 True,
                "version":                 pkg_version,
                "origin_release":          args.tag,
                "package_contract_digest": cd,
                "entrypoint_checksum":     cksum,
                "asset_url":               "",
                "package_digest":          "",
                "build_number":            0,
                "build_id":                "",
                "filename":                "",
                "profiles":                manifest.get("profiles", []),
                "publisher":               manifest.get("publisher", ""),
            })

    # ── Write version-overrides.txt for gen-version.sh ──
    # Only unchanged Go services need overrides (they keep their old version).
    # Changed services use gen-version.sh's default (the platform version).
    override_lines = [
        "# Version overrides for gen-version.sh (BOM release model)",
        f"# Platform release: {args.tag}",
        f"# Default version (for changed packages): {args.version}",
    ]
    for r in results:
        if not r["changed"] and r["go_target"]:
            override_lines.append(f"{r['go_target']}={r['version']}")

    with open(args.output_overrides, "w") as f:
        f.write("\n".join(override_lines) + "\n")

    # ── Write change-manifest.json ──
    changed_n   = sum(1 for r in results if r["changed"])
    unchanged_n = len(results) - changed_n
    refs = sorted(set(
        r["origin_release"] for r in results
        if not r["changed"] and r["origin_release"]
    ))

    manifest_out = {
        "platform_release":         args.version,
        "release_tag":              args.tag,
        "force_full_rebuild":       args.force_full_rebuild,
        "force_full_rebuild_reason": args.force_reason if args.force_full_rebuild else "",
        "changed_count":            changed_n,
        "unchanged_count":          unchanged_n,
        "referenced_releases":      refs,
        "packages":                 results,
    }
    with open(args.output_manifest, "w") as f:
        json.dump(manifest_out, f, indent=2)

    # ── Summary ──
    print(f"\nBOM change detection: {changed_n} changed, {unchanged_n} unchanged, {len(results)} total")
    if args.force_full_rebuild:
        print(f"  FORCED: {args.force_reason}")
    for r in results:
        if r["changed"]:
            print(f"  CHANGED  {r['name']:30s} -> v{r['version']}")
        else:
            print(f"  same     {r['name']:30s}    v{r['version']:20s} (origin: {r['origin_release']})")


if __name__ == "__main__":
    main()
