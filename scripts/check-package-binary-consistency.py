#!/usr/bin/env python3
"""check-package-binary-consistency.py — guard against package payload-identity drift.

A package's entrypoint binary is declared in THREE independent places:

  1. services/golang/build/pkg-map.json   ["<pkg>"]["binary"]   ← AUTHORITATIVE
     (release.yml and build-local-release.sh read this to decide which binary to
      copy into the package payload and to compute entrypoint_checksum)
  2. packages/registry.yaml                 binary: <name>
  3. packages/<pkg>/specs/<pkg>_{service,cmd}.yaml  metadata.entrypoint

When these drift, the build SILENTLY ships the wrong payload: in v1.2.216 the
claude package shipped a 12.3 MB Claude CLI v2.1.80 instead of the `noop`
sentinel its spec/verifier expected, because pkg-map.json still said
binary=claude while registry/spec had been moved to noop. entrypoint_checksum is
computed from whatever was packaged, so the verifier never catches the mismatch.

This script fails CI loud when the three declaration sites disagree.

Documented exceptions (build-time rename, not drift):
  - globular-cli: built as `globularcli` (go target), distributed as `globular`.
  - mcp: built from `./mcp`, distributed as `mcp_server` to avoid colliding with
    the package directory name.

Usage: python3 scripts/check-package-binary-consistency.py
       (run from the services repo root; expects ../packages alongside)
Exit 0 = consistent, 1 = drift found.
"""
import json, os, re, sys

SVC = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
PKG = os.path.normpath(os.path.join(SVC, "..", "packages"))

# pkg-map binary -> expected installed/entrypoint binary, when a documented
# build-time rename applies. Anything not listed must match verbatim.
RENAME_EXCEPTIONS = {
    "globularcli": "globular",
    "mcp": "mcp_server",
}


def load_registry_binaries(path):
    reg, name = {}, None
    for line in open(path):
        m = re.match(r"\s*-\s*name:\s*(\S+)", line)
        if m:
            name = m.group(1).strip()
            continue
        m = re.match(r"\s*binary:\s*(\S+)", line)
        if m and name and name not in reg:
            reg[name] = m.group(1).strip()
    return reg


def spec_entrypoint(name):
    base = name.replace('-', '_')
    for cand in (f"{PKG}/{name}/specs/{base}_service.yaml",
                 f"{PKG}/{name}/specs/{base}_cmd.yaml"):
        if not os.path.exists(cand):
            continue
        inmeta = False
        for line in open(cand):
            if re.match(r"^metadata:", line):
                inmeta = True
                continue
            if inmeta and re.match(r"^\S", line):
                inmeta = False
            m = re.match(r"\s+entrypoint:\s*(\S+)", line)
            if m and inmeta:
                return re.sub(r"^bin/", "", m.group(1).strip().strip('"')), cand
        # legacy: top-level entrypoint
        for line in open(cand):
            m = re.match(r"^entrypoint:\s*(\S+)", line)
            if m:
                return re.sub(r"^bin/", "", m.group(1).strip().strip('"')), cand
        return None, cand
    return None, None


def norm(binary):
    """Map a build artifact name to the distributed/entrypoint name."""
    return RENAME_EXCEPTIONS.get(binary, binary)


def main():
    pkgmap = json.load(open(f"{SVC}/golang/build/pkg-map.json"))
    reg = load_registry_binaries(f"{PKG}/registry.yaml")

    drift = []
    for name in sorted(pkgmap):
        pm = pkgmap[name].get("binary", "")
        if not pm:
            continue
        expected = norm(pm)  # what the package should actually ship/declare
        rb = reg.get(name)
        ep, specf = spec_entrypoint(name)

        problems = []
        if rb is not None and rb != expected:
            problems.append(f"registry.yaml binary={rb!r} != expected {expected!r}")
        if ep is not None and ep != expected:
            problems.append(f"spec entrypoint={ep!r} != expected {expected!r}")
        if problems:
            drift.append((name, pm, expected, problems))

    if drift:
        print("PACKAGE BINARY-IDENTITY DRIFT (build will ship the wrong payload):\n")
        for name, pm, expected, problems in drift:
            print(f"  ✗ {name}: pkg-map binary={pm!r} (expects {expected!r})")
            for p in problems:
                print(f"      - {p}")
        print("\nFix: make pkg-map.json / registry.yaml / specs entrypoint agree, "
              "or add a documented rename to RENAME_EXCEPTIONS.")
        return 1

    print(f"OK: {len(pkgmap)} packages — pkg-map / registry / spec entrypoint all consistent.")
    return 0


if __name__ == "__main__":
    sys.exit(main())
