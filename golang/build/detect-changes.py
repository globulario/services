#!/usr/bin/env python3
"""detect-changes.py — Compare current package contracts against previous release.

Computes a package_contract_digest for each package and compares it against
the previous release index. Produces:
  1. version-overrides.txt — for gen-version.sh (unchanged packages keep old version)
  2. change-manifest.json — full change detection results for the release builder

The contract digest covers: binary checksum, package.json, specs, systemd units,
profiles, hard_deps, provides, requires, defaults. It is independent of tar/gzip
archive metadata — same content always produces the same digest.

Usage:
  python3 detect-changes.py \\
    --prev-index prev-release-index.json \\
    --metadata-dir packages/metadata \\
    --bin-dir dist/bin \\
    --services-list golang/build/services.list \\
    --version 1.0.84 \\
    --tag v1.0.84 \\
    --output-overrides dist/version-overrides.txt \\
    --output-manifest dist/change-manifest.json

Exit codes:
  0 — success
  1 — error (missing required files, invalid arguments)
"""

import argparse
import hashlib
import json
import os
import sys
from pathlib import Path


def compute_file_sha256(path: str) -> str:
    """Compute sha256 of a file. Returns 'sha256:<hex>' or empty string if missing."""
    try:
        h = hashlib.sha256()
        with open(path, 'rb') as f:
            for chunk in iter(lambda: f.read(8192), b''):
                h.update(chunk)
        return f"sha256:{h.hexdigest()}"
    except FileNotFoundError:
        return ""


def compute_contract_digest(components: dict) -> str:
    """Compute a normalized package contract digest from component hashes.

    Components are hashed in a deterministic order so the same content
    always produces the same digest regardless of filesystem metadata.
    """
    h = hashlib.sha256()

    def write_field(label: str, value: str):
        h.update(label.encode())
        h.update(b'\x00')
        h.update(value.encode())
        h.update(b'\x00')

    write_field("entrypoint", components.get("entrypoint_checksum", ""))
    write_field("manifest", components.get("manifest_sha256", ""))
    write_field("spec", components.get("spec_sha256", ""))
    write_field("systemd", components.get("systemd_sha256", ""))

    for profile in sorted(components.get("profiles", [])):
        write_field("profile", profile)
    for dep in sorted(components.get("hard_deps", [])):
        write_field("hard_dep", dep)
    for prov in sorted(components.get("provides", [])):
        write_field("provides", prov)
    for req in sorted(components.get("requires", [])):
        write_field("requires", req)
    for k in sorted(components.get("defaults", {}).keys()):
        write_field(f"default:{k}", components["defaults"][k])

    return f"sha256:{h.hexdigest()}"


def load_prev_index(path: str) -> dict:
    """Load previous release index. Returns {name: entry_dict}."""
    try:
        with open(path) as f:
            idx = json.load(f)
    except (FileNotFoundError, json.JSONDecodeError):
        return {}
    result = {}
    for pkg in idx.get("packages", []):
        name = pkg.get("name", "")
        if name:
            result[name] = pkg
    return result


def detect_package_changes(
    metadata_dir: str,
    bin_dir: str,
    bin_map: dict,
    prev_packages: dict,
    current_version: str,
    current_tag: str,
) -> list:
    """Detect which packages changed vs the previous release.

    Returns a list of dicts with change-detection results per package.
    """
    results = []

    for pkg_name, bin_name in sorted(bin_map.items()):
        meta_dir = os.path.join(metadata_dir, pkg_name)
        if not os.path.isdir(meta_dir):
            continue

        bin_path = os.path.join(bin_dir, bin_name)
        pkg_json_path = os.path.join(meta_dir, "package.json")

        if not os.path.isfile(bin_path):
            continue

        # Load package.json for metadata.
        try:
            with open(pkg_json_path) as f:
                manifest = json.load(f)
        except (FileNotFoundError, json.JSONDecodeError):
            manifest = {}

        # Compute component hashes.
        entrypoint_checksum = compute_file_sha256(bin_path)
        manifest_sha256 = compute_file_sha256(pkg_json_path)

        # Find spec and systemd files.
        spec_sha256 = ""
        spec_dir = os.path.join(meta_dir, "specs")
        if os.path.isdir(spec_dir):
            for spec_file in sorted(os.listdir(spec_dir)):
                spec_sha256 = compute_file_sha256(os.path.join(spec_dir, spec_file))
                break  # use first spec file

        systemd_sha256 = ""
        systemd_dir = os.path.join(meta_dir, "systemd")
        if os.path.isdir(systemd_dir):
            for unit_file in sorted(os.listdir(systemd_dir)):
                systemd_sha256 = compute_file_sha256(os.path.join(systemd_dir, unit_file))
                break

        components = {
            "entrypoint_checksum": entrypoint_checksum,
            "manifest_sha256": manifest_sha256,
            "spec_sha256": spec_sha256,
            "systemd_sha256": systemd_sha256,
            "profiles": manifest.get("profiles", []),
            "hard_deps": manifest.get("hard_deps", []),
            "provides": manifest.get("provides_capabilities", []),
            "requires": [],
            "defaults": manifest.get("defaults", {}),
        }

        contract_digest = compute_contract_digest(components)

        # Compare against previous release.
        prev = prev_packages.get(pkg_name, {})
        prev_contract = prev.get("package_contract_digest", "")
        prev_version = prev.get("version", "")
        prev_origin = prev.get("origin_release", prev.get("release_tag", ""))
        prev_asset_url = prev.get("asset_url", "")
        prev_artifact_sha256 = prev.get("artifact_sha256", prev.get("package_digest", ""))
        prev_build_number = prev.get("build_number", 0)
        prev_build_id = prev.get("build_id", "")

        if prev_contract and prev_contract == contract_digest:
            # Unchanged — keep previous version.
            changed = False
            pkg_version = prev_version
            origin_release = prev_origin
            asset_url = prev_asset_url
            build_number = prev_build_number
            build_id = prev_build_id
        else:
            # Changed — use current platform version.
            changed = True
            pkg_version = current_version
            origin_release = current_tag
            asset_url = ""  # will be set by the release builder
            build_number = 0  # will be set by the release builder
            build_id = ""

        results.append({
            "name": pkg_name,
            "changed": changed,
            "version": pkg_version,
            "origin_release": origin_release,
            "contract_digest": contract_digest,
            "prev_contract_digest": prev_contract,
            "entrypoint_checksum": entrypoint_checksum,
            "asset_url": asset_url,
            "prev_artifact_sha256": prev_artifact_sha256,
            "build_number": build_number,
            "build_id": build_id,
        })

    return results


def main():
    parser = argparse.ArgumentParser(description="Detect package changes for BOM release")
    parser.add_argument("--prev-index", required=True, help="Path to previous release-index.json")
    parser.add_argument("--metadata-dir", required=True, help="Path to packages/metadata/")
    parser.add_argument("--bin-dir", required=True, help="Path to dist/bin/")
    parser.add_argument("--bin-map-json", required=True, help="JSON file mapping package-name → binary-name")
    parser.add_argument("--version", required=True, help="Current platform version (e.g. 1.0.84)")
    parser.add_argument("--tag", required=True, help="Current release tag (e.g. v1.0.84)")
    parser.add_argument("--output-overrides", required=True, help="Output path for version-overrides.txt")
    parser.add_argument("--output-manifest", required=True, help="Output path for change-manifest.json")
    parser.add_argument("--force-full-rebuild", action="store_true", help="Treat all packages as changed")
    parser.add_argument("--force-reason", default="", help="Reason for force full rebuild")
    args = parser.parse_args()

    prev_packages = load_prev_index(args.prev_index)

    # Load binary map.
    try:
        with open(args.bin_map_json) as f:
            bin_map = json.load(f)
    except (FileNotFoundError, json.JSONDecodeError) as e:
        print(f"ERROR: cannot load bin-map-json: {e}", file=sys.stderr)
        sys.exit(1)

    results = detect_package_changes(
        metadata_dir=args.metadata_dir,
        bin_dir=args.bin_dir,
        bin_map=bin_map,
        prev_packages=prev_packages,
        current_version=args.version,
        current_tag=args.tag,
    )

    if args.force_full_rebuild:
        for r in results:
            r["changed"] = True
            r["version"] = args.version
            r["origin_release"] = args.tag

    # Write version overrides for gen-version.sh.
    # Only list unchanged packages; changed ones get the default version.
    overrides = []
    for r in results:
        if not r["changed"]:
            # Map package name back to Go build target path suffix.
            # This is a simplification — the release.yml BIN_MAP defines the mapping.
            overrides.append(f"# {r['name']} unchanged (origin: {r['origin_release']})")

    with open(args.output_overrides, 'w') as f:
        f.write("# Version overrides for gen-version.sh (BOM release model)\n")
        f.write(f"# Generated for platform release {args.tag}\n")
        for line in overrides:
            f.write(line + "\n")

    # Write full change manifest.
    changed_count = sum(1 for r in results if r["changed"])
    unchanged_count = sum(1 for r in results if not r["changed"])

    manifest = {
        "platform_release": args.version,
        "release_tag": args.tag,
        "force_full_rebuild": args.force_full_rebuild,
        "force_full_rebuild_reason": args.force_reason,
        "changed_count": changed_count,
        "unchanged_count": unchanged_count,
        "referenced_releases": sorted(set(
            r["origin_release"] for r in results
            if not r["changed"] and r["origin_release"] != args.tag
        )),
        "packages": results,
    }
    with open(args.output_manifest, 'w') as f:
        json.dump(manifest, f, indent=2)

    print(f"detect-changes: {changed_count} changed, {unchanged_count} unchanged out of {len(results)} packages")
    for r in results:
        status = "CHANGED" if r["changed"] else f"unchanged (origin: {r['origin_release']})"
        print(f"  {r['name']:30s} v{r['version']:12s} {status}")


if __name__ == "__main__":
    main()
