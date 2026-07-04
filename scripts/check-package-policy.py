#!/usr/bin/env python3
"""check-package-policy.py — enforce that RBAC-enforced service packages ship
their generated authorization vocabulary.

Invariant: rbac.enforced_service_requires_packaged_policy_vocabulary.

A service that participates in post-bootstrap RBAC enforcement MUST ship its
generated permission/action vocabulary (permissions.generated.json +
roles.generated.json) INSIDE its package, so the node can register method->action
mappings after the bootstrap gate closes. A package that carries executable code
but not its governance vocabulary can run but cannot be governed
(policy_version=unknown, coarse action keys, role_binding_denied post-bootstrap).

authzgen emits services/generated/policy/<svc>/permissions.generated.json for
every RBAC-enforced service. This gate asserts: for every such service that has a
built .tgz, the archive contains policy/permissions.generated.json.

This is the CI-visible mirror of the in-build guard pkgpack.assertPackageGuards.
It is best-effort about WHICH packages are built: it only checks packages that
actually exist under the search dirs, and skips cleanly when none are built (so a
fresh checkout does not fail spuriously). It is strict about packages it DOES
find: a policy-declaring service whose .tgz omits policy is a hard failure.
"""

import glob
import json
import os
import pathlib
import re
import sys
import tarfile

SERVICES_ROOT = pathlib.Path(__file__).resolve().parent.parent
POLICY_ROOT = SERVICES_ROOT / "generated" / "policy"
PROTO_ROOT = SERVICES_ROOT / "proto"

# Proto files intentionally without authz vocabulary (mirror of
# scripts/check_proto_authz.sh ALLOWLIST). A new service proto is NOT allowed to
# silently join this list — adding it here is an explicit, reviewed opt-out, not
# a silent bypass.
PROTO_AUTHZ_ALLOWLIST = {
    "compute.proto",
    "compute_runner.proto",
    "reflection.proto",
    "globular_auth.proto",
}

# Directories that may hold built service packages.
SEARCH_DIRS = [
    SERVICES_ROOT / "generated",
    SERVICES_ROOT / "dist",
]


def rbac_enforced_services() -> dict[str, pathlib.Path]:
    """Map package-name (hyphenated) -> policy dir for every service whose
    authzgen output includes permissions.generated.json."""
    services: dict[str, pathlib.Path] = {}
    if not POLICY_ROOT.is_dir():
        return services
    for entry in sorted(POLICY_ROOT.iterdir()):
        if not entry.is_dir():
            continue
        if not (entry / "permissions.generated.json").is_file():
            continue
        # generated/policy dirs use underscores (node_agent); packages use
        # hyphens (node-agent).
        pkg_name = entry.name.replace("_", "-")
        services[pkg_name] = entry
    return services


def find_package_tgzs(pkg_name: str) -> list[pathlib.Path]:
    found: list[pathlib.Path] = []
    for d in SEARCH_DIRS:
        if not d.is_dir():
            continue
        # generated/<pkg>_<ver>_<platform>.tgz and dist/*/packages/<pkg>_...tgz
        found += [pathlib.Path(p) for p in glob.glob(str(d / f"{pkg_name}_*.tgz"))]
        found += [pathlib.Path(p) for p in glob.glob(str(d / "*" / "packages" / f"{pkg_name}_*.tgz"))]
    return found


def tgz_has_policy(tgz: pathlib.Path) -> bool:
    try:
        with tarfile.open(tgz, "r:gz") as tf:
            for name in tf.getnames():
                norm = name.lstrip("./")
                if norm == "policy/permissions.generated.json":
                    return True
    except (tarfile.TarError, OSError) as exc:
        print(f"  WARN: cannot read {tgz}: {exc}", file=sys.stderr)
    return False


def enforced_services_from_protos() -> dict[str, str]:
    """Every gRPC service declared in a proto that carries authz vocabulary MUST
    have generated policy. Returns fully-qualified {package.Service: proto-file}.
    This is the authoritative 'a new service cannot silently bypass the rule'
    set: authz-annotated → must appear in generated policy."""
    should: dict[str, str] = {}
    if not PROTO_ROOT.is_dir():
        return should
    for proto in sorted(PROTO_ROOT.glob("*.proto")):
        if proto.name in PROTO_AUTHZ_ALLOWLIST:
            continue
        txt = proto.read_text()
        if "globular.auth.authz" not in txt:
            continue  # no authz options → not an RBAC-enforced service proto
        pkg_m = re.search(r"^package\s+([\w.]+)\s*;", txt, re.M)
        if not pkg_m:
            continue
        pkg = pkg_m.group(1)
        for svc_m in re.finditer(r"^service\s+(\w+)", txt, re.M):
            should[f"{pkg}.{svc_m.group(1)}"] = proto.name
    return should


def services_with_generated_policy() -> set[str]:
    """Fully-qualified gRPC service names that HAVE generated policy. The
    'service' field may be a comma-separated list for multi-service packages."""
    have: set[str] = set()
    for f in glob.glob(str(POLICY_ROOT / "*" / "permissions.generated.json")):
        try:
            svc = json.load(open(f)).get("service", "")
        except (json.JSONDecodeError, OSError):
            continue
        for part in svc.split(","):
            part = part.strip()
            if part:
                have.add(part)
    return have


def check_authz_services_have_policy() -> list[str]:
    """Seam gate: every authz-annotated service proto must have generated
    policy. Catches a new service (or an authzgen regression) that would ship
    with RBAC-enforced RPCs but no action vocabulary — the silent bypass."""
    should = enforced_services_from_protos()
    if not should:
        return []  # no proto tree here (e.g. partial checkout) — skip
    have = services_with_generated_policy()
    return sorted(f"{svc}  (declared in {proto})" for svc, proto in should.items() if svc not in have)


def main() -> int:
    # Seam 1: every authz-annotated gRPC service must HAVE generated policy.
    # This runs from source (proto/ + generated/policy/) and needs no built
    # packages, so it always guards against a new service bypassing the rule.
    missing_vocab = check_authz_services_have_policy()
    if missing_vocab:
        print(
            "FAIL: authz-annotated gRPC service(s) have NO generated policy vocabulary "
            "(invariant rbac.enforced_service_requires_packaged_policy_vocabulary):",
            file=sys.stderr,
        )
        for m in missing_vocab:
            print(f"  {m}", file=sys.stderr)
        print(
            "\nEvery service proto with (globular.auth.authz) options must produce "
            "generated/policy/<svc>/permissions.generated.json (run authzgen). To opt a "
            "proto out, add it to PROTO_AUTHZ_ALLOWLIST here AND in check_proto_authz.sh "
            "with a documented reason — never leave it silently unpoliced.",
            file=sys.stderr,
        )
        return 1

    # Seam 2: every BUILT RBAC-service package must SHIP its policy vocabulary.
    services = rbac_enforced_services()
    if not services:
        print("check-package-policy: no generated/policy/* found — skipping (nothing built)")
        return 0

    checked = 0
    failures: list[str] = []
    for pkg_name in sorted(services):
        tgzs = find_package_tgzs(pkg_name)
        if not tgzs:
            continue  # package not built in this tree — not this gate's job
        for tgz in tgzs:
            checked += 1
            if not tgz_has_policy(tgz):
                rel = os.path.relpath(tgz, SERVICES_ROOT)
                failures.append(
                    f"{rel}: RBAC-enforced service '{pkg_name}' ships no "
                    f"policy/permissions.generated.json"
                )

    if failures:
        print(
            "FAIL: RBAC-enforced service package(s) missing packaged policy "
            "vocabulary (invariant rbac.enforced_service_requires_packaged_policy_vocabulary):",
            file=sys.stderr,
        )
        for f in failures:
            print(f"  {f}", file=sys.stderr)
        print(
            "\nFix: the package build must copy generated/policy/<svc>/ into the "
            "package's policy/ dir (see pkgpack.BuildPackage / assertPackageGuards).",
            file=sys.stderr,
        )
        return 1

    if checked == 0:
        print("check-package-policy: no built RBAC-service packages found — skipping")
        return 0

    print(f"OK: all {checked} built RBAC-service package(s) ship their policy vocabulary")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
