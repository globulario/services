#!/usr/bin/env python3
"""Enforce the day-0 bootstrap package contract against the canonical registry."""

from __future__ import annotations

import pathlib
import re
import sys
from collections import deque

import yaml


REPO_ROOT = pathlib.Path(__file__).resolve().parents[1]
DEFAULT_INSTALL_DAY0 = REPO_ROOT / "scripts" / "release" / "install-day0.sh"
DEFAULT_ENSURE_BOOTSTRAP = REPO_ROOT / "scripts" / "release" / "ensure-bootstrap-artifacts.sh"
DEFAULT_REGISTRY = REPO_ROOT.parent / "packages" / "registry.yaml"


def die(msg: str) -> int:
    print(f"FAIL: {msg}", file=sys.stderr)
    return 1


def package_name_from_tgz(token: str) -> str:
    stem = token.removesuffix(".tgz")
    parts = stem.rsplit("_", 3)
    if len(parts) != 4:
        raise ValueError(f"cannot parse package token {token!r}")
    return parts[0]


def parse_install_day0_packages(path: pathlib.Path) -> set[str]:
    text = path.read_text(encoding="utf-8")
    tokens = re.findall(r'"([^"\n]+\.tgz)"', text)
    packages: set[str] = set()
    for token in tokens:
        if "*" in token or "/" in token or "$" in token or token.startswith("_"):
            continue
        packages.add(package_name_from_tgz(token))
    return packages


def parse_published_globs(path: pathlib.Path) -> set[str]:
    """Parse the CORE_PACKAGES glob patterns from ensure-bootstrap-artifacts.sh.

    Patterns look like "etcd_*_linux_amd64.tgz"; the package name is the token
    before the first "_*". This is the set of packages the day-0 registration
    step actually publishes into the repository.
    """
    text = path.read_text(encoding="utf-8")
    tokens = re.findall(r'"([^"\n]+_\*_linux_amd64\.tgz)"', text)
    packages: set[str] = set()
    for token in tokens:
        name = token.split("_*", 1)[0]
        if name:
            packages.add(name)
    return packages


def required_day0_packages(registry_path: pathlib.Path) -> tuple[set[str], dict[str, dict]]:
    data = yaml.safe_load(registry_path.read_text(encoding="utf-8"))
    packages = data.get("packages", [])
    by_name = {pkg["name"]: pkg for pkg in packages}

    required: set[str] = set()
    queue: deque[str] = deque()
    for pkg in packages:
        if pkg.get("day0_required"):
            name = pkg["name"]
            required.add(name)
            queue.append(name)

    while queue:
        name = queue.popleft()
        pkg = by_name.get(name, {})
        for dep in pkg.get("hard_deps", []) or []:
            if dep not in by_name:
                raise ValueError(f"registry package {name!r} depends on unknown package {dep!r}")
            if dep not in required:
                required.add(dep)
                queue.append(dep)

    return required, by_name


def main() -> int:
    install_day0 = pathlib.Path(sys.argv[1]) if len(sys.argv) > 1 else DEFAULT_INSTALL_DAY0
    registry = pathlib.Path(sys.argv[2]) if len(sys.argv) > 2 else DEFAULT_REGISTRY
    ensure_bootstrap = pathlib.Path(sys.argv[3]) if len(sys.argv) > 3 else DEFAULT_ENSURE_BOOTSTRAP

    if not install_day0.is_file():
        return die(f"install-day0.sh not found at {install_day0}")
    if not registry.is_file():
        return die(f"registry.yaml not found at {registry}")
    if not ensure_bootstrap.is_file():
        return die(f"ensure-bootstrap-artifacts.sh not found at {ensure_bootstrap}")

    try:
        referenced = parse_install_day0_packages(install_day0)
        published = parse_published_globs(ensure_bootstrap)
        required, by_name = required_day0_packages(registry)
    except ValueError as exc:
        return die(str(exc))

    unknown = sorted(name for name in referenced if name not in by_name)
    missing = sorted(name for name in required if name not in referenced)
    # The registration step (ensure-bootstrap-artifacts.sh) must actually PUBLISH
    # every registry-required day-0 package into the repository — otherwise
    # day-0/day-1 nodes resolve "latest manifest" and get NotFound even though
    # install-day0.sh installed the local artifact. This is the drift that left
    # codex/alertmanager (day0_required: true) out of the catalog.
    unpublished = sorted(name for name in required if name not in published)
    unknown_published = sorted(name for name in published if name not in by_name)

    if unknown:
        print("FAIL: install-day0 references package(s) not present in packages/registry.yaml:", file=sys.stderr)
        for name in unknown:
            print(f"  - {name}", file=sys.stderr)
    if missing:
        print("FAIL: install-day0 is missing registry-required day-0 package(s) or their hard deps:", file=sys.stderr)
        for name in missing:
            print(f"  - {name}", file=sys.stderr)
    if unpublished:
        print("FAIL: ensure-bootstrap-artifacts.sh CORE_PACKAGES does not publish registry-required day-0 package(s):", file=sys.stderr)
        for name in unpublished:
            print(f"  - {name}", file=sys.stderr)
    if unknown_published:
        print("FAIL: ensure-bootstrap-artifacts.sh publishes package(s) not present in packages/registry.yaml:", file=sys.stderr)
        for name in unknown_published:
            print(f"  - {name}", file=sys.stderr)

    if unknown or missing or unpublished or unknown_published:
        return 1

    print(
        "OK: day-0 package contract matches registry "
        f"({len(referenced)} installed, {len(published)} published, {len(required)} required)"
    )
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
