#!/usr/bin/env python3
"""Apply the bounded review fixes required before PR #221 can merge.

This script is intentionally assertion-heavy. Every replacement must match the
reviewed source exactly once, or the run fails without committing a partial
rewrite.
"""

from __future__ import annotations

import re
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]


def read(path: str) -> str:
    return (ROOT / path).read_text(encoding="utf-8")


def write(path: str, content: str) -> None:
    (ROOT / path).write_text(content, encoding="utf-8")


def replace_once(text: str, old: str, new: str, label: str) -> str:
    count = text.count(old)
    if count != 1:
        raise RuntimeError(f"{label}: expected exactly one match, found {count}")
    return text.replace(old, new, 1)


def regex_once(text: str, pattern: str, replacement: str, label: str) -> str:
    updated, count = re.subn(pattern, replacement, text, count=1, flags=re.S)
    if count != 1:
        raise RuntimeError(f"{label}: expected exactly one regex match, found {count}")
    return updated


def patch_release_workflow() -> None:
    path = ".github/workflows/release.yml"
    text = read(path)

    text = replace_once(
        text,
        '''      - name: Rebuild changed Go services with real version
        working-directory: services/golang
        env:
          VERSION: ${{ steps.version.outputs.version }}
        run: |
''',
        '''      - name: Rebuild changed Go services with committed package versions
        working-directory: services/golang
        run: |
''',
        "Go rebuild step header",
    )
    text = replace_once(
        text,
        '''          # Re-generate version files: changed services get platform version,
          # unchanged services get their previous version via overrides.
          bash build/gen-version.sh "${VERSION}" ../../dist/version-overrides.txt
''',
        '''          # Restore the committed per-package source authority after the
          # sentinel build rewrote the workspace-only generated files.
          bash build/gen-version.sh "0.0.0-dev" ../../dist/package-versions.txt
''',
        "Go rebuild version authority",
    )
    text = replace_once(
        text,
        '            echo "  -> Rebuilding ${bin_name} (v${VERSION})"\n',
        '            echo "  -> Rebuilding ${bin_name} from committed package version"\n',
        "Go rebuild log",
    )

    text = replace_once(
        text,
        '''      - name: Rebuild xds/gateway if changed
        working-directory: Globular
        env:
          VERSION: ${{ steps.version.outputs.version }}
        run: |
          LDFLAGS="-X main.Version=${VERSION} -X main.BuildVersion=${VERSION} -s -w"
''',
        '''      - name: Rebuild xds/gateway from committed versions if changed
        working-directory: Globular
        run: |
          LDFLAGS="-s -w"
''',
        "xds/gateway source authority",
    )
    text = replace_once(
        text,
        '            echo "  -> Rebuilding ${name} (v${VERSION})"\n',
        '            echo "  -> Rebuilding ${name} from committed source version"\n',
        "xds/gateway rebuild log",
    )

    text = regex_once(
        text,
        r'''\n      - name: Generate build identity\n.*?(?=\n      - name: Package changed packages\n)''',
        "\n",
        "remove local build-identity minting step",
    )

    text = replace_once(
        text,
        '''      - name: Package changed packages
        env:
          BUILD_NUMBER: ${{ steps.build_identity.outputs.build_number }}
          BUILD_ID: ${{ steps.build_identity.outputs.build_id }}
          VERSION: ${{ steps.version.outputs.version }}
          TAG: ${{ steps.version.outputs.tag }}
        run: |
''',
        '''      - name: Package changed packages
        run: |
''',
        "package step environment",
    )
    text = replace_once(
        text,
        '''          manifest = json.load(open("dist/change-manifest.json"))
          pkg_map  = json.load(open("services/golang/build/pkg-map.json"))
          version  = os.environ["VERSION"]
          build_number = os.environ["BUILD_NUMBER"]
          build_id = os.environ["BUILD_ID"]
''',
        '''          manifest = json.load(open("dist/change-manifest.json"))
          pkg_map  = json.load(open("services/golang/build/pkg-map.json"))
          package_versions = {}
          with open("dist/package-versions.txt", encoding="utf-8") as versions_file:
              for raw in versions_file:
                  line = raw.strip()
                  if not line or line.startswith("#"):
                      continue
                  name, sep, value = line.partition("=")
                  if not sep or not name.strip() or not value.strip():
                      raise RuntimeError(f"invalid package version row: {raw.rstrip()!r}")
                  package_versions[name.strip()] = value.strip()
''',
        "package version map loader",
    )
    text = replace_once(
        text,
        '''              # Determine version
              if plat_ver:
                  pkg_version = version
              else:
                  pkg_version = detect_infra_version(name, bin_path, pj.get("version", ""))
''',
        '''              # Determine version from the package's declared authority.
              if plat_ver:
                  pkg_version = package_versions.get(name, "")
              else:
                  pkg_version = detect_infra_version(name, bin_path, pj.get("version", ""))
''',
        "package version selection",
    )
    text = replace_once(
        text,
        '''              # Stamp package.json with real version, checksum, build_id.
              # Each package gets its own unique build_id (UUIDv4) so the
              # repository can distinguish artifacts that share a release.
              import uuid as _uuid
              per_pkg_build_id = str(_uuid.uuid4())
              pj_out = json.load(open(f"{tmpdir}/package.json"))
              pj_out["version"] = pkg_version
              pj_out["entrypoint_checksum"] = checksum
              pj_out["build_id"] = per_pkg_build_id
              pj_out["build_number"] = int(build_number)
''',
        '''              # Stamp content facts only. build_id/build_number are allocated
              # by repository admission and must not be minted by release assembly.
              pj_out = json.load(open(f"{tmpdir}/package.json"))
              pj_out["version"] = pkg_version
              pj_out["entrypoint_checksum"] = checksum
              pj_out.pop("build_id", None)
              pj_out.pop("build_number", None)
''',
        "package identity stamping",
    )

    text = replace_once(
        text,
        '''      - name: Download unchanged packages from origin releases
        env:
          GH_TOKEN: ${{ github.token }}
          BUILD_NUMBER: ${{ steps.build_identity.outputs.build_number }}
''',
        '''      - name: Download unchanged packages from origin releases
        env:
          GH_TOKEN: ${{ github.token }}
''',
        "unchanged-package environment",
    )

    text = replace_once(
        text,
        '''      - name: Repair unstripped carry-forward packages for this release
        env:
          BUILD_NUMBER: ${{ steps.build_identity.outputs.build_number }}
        run: |
''',
        '''      - name: Repair unstripped carry-forward packages for this release
        run: |
''',
        "carry-forward repair environment",
    )
    text = replace_once(
        text,
        '          import hashlib, uuid\n',
        '          import hashlib\n',
        "carry-forward uuid import",
    )
    text = replace_once(
        text,
        '          build_number = int(os.environ["BUILD_NUMBER"])\n\n',
        "",
        "carry-forward build number",
    )
    text = replace_once(
        text,
        '''                  pkg["build_id"] = str(uuid.uuid4())
                  pkg["build_number"] = build_number
''',
        '''                  # Repaired bytes are re-admitted as a new repository build;
                  # release assembly must not allocate repository-owned identity.
                  pkg.pop("build_id", None)
                  pkg.pop("build_number", None)
''',
        "carry-forward identity minting",
    )
    text = replace_once(
        text,
        '                      "build_id": pkg["build_id"],\n',
        "",
        "carry-forward report build id",
    )
    text = replace_once(
        text,
        '                  print(f"  - {item[\'name\']}: {item[\'filename\']}{where} -> build_id={item[\'build_id\']}")\n',
        '                  print(f"  - {item[\'name\']}: {item[\'filename\']}{where} -> repository re-admission required")\n',
        "carry-forward report log",
    )

    text = replace_once(
        text,
        '''      - name: Generate release index (v2 BOM)
        env:
          RELEASE_VERSION: ${{ steps.version.outputs.version }}
          RELEASE_TAG: ${{ steps.version.outputs.tag }}
          BUILD_NUMBER: ${{ steps.build_identity.outputs.build_number }}
          BUILD_ID: ${{ steps.build_identity.outputs.build_id }}
''',
        '''      - name: Generate release index (v2 BOM)
        env:
          RELEASE_VERSION: ${{ steps.version.outputs.version }}
          RELEASE_TAG: ${{ steps.version.outputs.tag }}
''',
        "release-index environment",
    )
    text = replace_once(
        text,
        '''          build_number = os.environ["BUILD_NUMBER"]
          build_id     = os.environ["BUILD_ID"]
''',
        "",
        "release-index local identity",
    )
    text = replace_once(
        text,
        '''                      "build_number":             int(pj.get("build_number", build_number)),
                      "build_id":                 str(pj.get("build_id", build_id)),
''',
        "",
        "changed-entry identity pins",
    )
    text = replace_once(
        text,
        '                  "build_number":             int(build_number),\n',
        "",
        "awareness global build number",
    )

    text = replace_once(
        text,
        '''      #   - Changed packages get the current platform version (e.g. 1.0.85)
      #     and are uploaded to this GitHub release.
''',
        '''      #   - Changed code-owned packages keep their committed package version
      #     and are uploaded to this GitHub release.
''',
        "BOM version-authority comment",
    )

    forbidden = {
        "local identity step": "- name: Generate build identity",
        "stack identity reference": "steps.build_identity",
        "per-package UUID": "per_pkg_build_id",
        "UUID minting": "uuid.uuid4",
        "platform package stamp": "pkg_version = version",
        "platform Go regeneration": 'build/gen-version.sh "${VERSION}"',
        "platform xds ldflag": "-X main.Version=${VERSION}",
    }
    for label, needle in forbidden.items():
        if needle in text:
            raise RuntimeError(f"release workflow still contains {label}: {needle}")

    write(path, text)


def patch_identity_gate() -> None:
    path = "scripts/check-identity-authority.sh"
    text = read(path)
    text = replace_once(
        text,
        '''RELEASE_SCRIPTS=(
  "${SERVICES_ROOT}/scripts/build-release.sh"
  "${SERVICES_ROOT}/scripts/regenerate-release-inputs.sh"
)
''',
        '''RELEASE_SCRIPTS=(
  "${SERVICES_ROOT}/scripts/build-release.sh"
  "${SERVICES_ROOT}/scripts/regenerate-release-inputs.sh"
  "${SERVICES_ROOT}/.github/workflows/release.yml"
)
''',
        "identity-gate production workflow scope",
    )
    text = replace_once(
        text,
        "if grep -nE 'uuid\\.uuid4|uuid\\.uuid1|uuidgen' \"$f\" >/dev/null; then",
        "if grep -nE 'uuid\\.uuid[147]|_uuid\\.uuid4|uuidgen|per_pkg_build_id' \"$f\" >/dev/null; then",
        "identity UUID pattern",
    )
    text = replace_once(
        text,
        "if grep -nE 'BUILD_NUMBER=.*date \\+%s|build_number.*date \\+%s' \"$f\" >/dev/null; then",
        "if grep -nE 'BUILD_NUMBER=.*date \\+%s|build_number.*date \\+%s|BUILD_NUMBER=.*github\\.run_number' \"$f\" >/dev/null; then",
        "identity build-number pattern",
    )
    text = replace_once(
        text,
        '''# 3. No platform-version override of package versions in the release build.
if grep -nE -- '-X main\\.Version=\\$\\{VERSION\\}' "${SERVICES_ROOT}/scripts/build-release.sh" >/dev/null; then
  err "build-release.sh injects the PLATFORM version into binaries — service versions come from committed zz files"
else
  ok "no platform-version ldflags override in build-release.sh"
fi
''',
        '''# 3. No platform-version override of package versions in any release path.
for f in "${RELEASE_SCRIPTS[@]}"; do
  if grep -nE -- '-X main\\.Version=\\$\\{VERSION\\}|pkg_version[[:space:]]*=[[:space:]]*version|gen-version\\.sh "\\$\\{VERSION\\}"' "$f" >/dev/null; then
    err "$(basename "$f") stamps the platform version into package identity — package versions come from committed source authority"
  fi
done
if (( ! FAIL )); then
  ok "release paths preserve committed per-package versions"
fi
''',
        "identity platform-version check",
    )
    write(path, text)


def patch_restore_safety() -> None:
    path = "golang/substrate/restore.go"
    text = read(path)
    text = replace_once(
        text,
        '''type RestoreOptions struct {
	// Force overrides the cluster-UID guard and overwrites keys that already
''',
        '''type RestoreOptions struct {
	// ControllerGateEstablished is intentionally not exposed by the CLI. It is
	// used by internal tests until a controller/workflow reader enforces the
	// RESTORED_UNVERIFIED marker before any convergence mutation. The default
	// false value keeps non-dry-run dump restore fail-closed.
	ControllerGateEstablished bool
	// Force overrides the cluster-UID guard and overwrites keys that already
''',
        "restore options gate",
    )
    text = replace_once(
        text,
        '''	if liveUID != nil && d.Manifest.ClusterUID != "" {
		live := strings.TrimSpace(string(liveUID.Value))
		if live != d.Manifest.ClusterUID && !opts.Force {
			return nil, fmt.Errorf("REFUSED: dump is from cluster %s but the live store belongs to cluster %s — importing it would graft one cluster's desired state onto another (override requires --force)",
				d.Manifest.ClusterUID, live)
		}
	}

	res := &RestoreResult{
''',
        '''	if liveUID != nil && d.Manifest.ClusterUID != "" {
		live := strings.TrimSpace(string(liveUID.Value))
		if live != d.Manifest.ClusterUID && !opts.Force {
			return nil, fmt.Errorf("REFUSED: dump is from cluster %s but the live store belongs to cluster %s — importing it would graft one cluster's desired state onto another (override requires --force)",
				d.Manifest.ClusterUID, live)
		}
	}
	if !opts.DryRun && !opts.ControllerGateEstablished {
		return nil, fmt.Errorf("REFUSED: non-dry-run dump restore is locked until controller/workflow mutation paths enforce %s=%s; use --dry-run to inspect the plan", RestoreMarkerKey, StatusRestoredUnverified)
	}

	res := &RestoreResult{
''',
        "restore fail-closed guard",
    )
    write(path, text)

    test_path = "golang/substrate/substrate_test.go"
    tests = read(test_path)
    tests = tests.replace("RestoreOptions{}", "RestoreOptions{ControllerGateEstablished: true}")
    tests = tests.replace(
        "RestoreOptions{Force: true}",
        "RestoreOptions{Force: true, ControllerGateEstablished: true}",
    )
    anchor = "func TestRestore_DryRunWritesNothing(t *testing.T) {\n"
    if tests.count(anchor) != 1:
        raise RuntimeError("restore refusal test anchor missing or duplicated")
    refusal_test = '''func TestRestore_DefaultRefusesMutationWithoutControllerGate(t *testing.T) {
	ctx := context.Background()
	src := newFakeKV()
	seedRepresentativeCluster(src)
	dump, err := TakeDump(ctx, src, false)
	if err != nil {
		t.Fatalf("TakeDump: %v", err)
	}

	dst := newFakeKV()
	before := len(dst.data)
	_, err = RestoreDump(ctx, dst, dump, RestoreOptions{})
	if err == nil || !strings.Contains(err.Error(), "non-dry-run dump restore is locked") {
		t.Fatalf("default restore must fail closed on missing controller gate, got %v", err)
	}
	if len(dst.data) != before {
		t.Fatalf("refused restore wrote %d keys", len(dst.data)-before)
	}
}

'''
    tests = tests.replace(anchor, refusal_test + anchor, 1)
    write(test_path, tests)

    cli_path = "golang/globularcli/substrate_cmds.go"
    cli = read(cli_path)
    cli = replace_once(
        cli,
        '''  --from-dump [file]    rung 3: import a classified dump into a fresh etcd.
                        Without a file, selects the best dump in --dir by
                        desired epoch (not timestamp).
''',
        '''  --from-dump [file]    rung 3: inspect a classified dump for a fresh etcd.
                        Without a file, selects the best dump in --dir by
                        desired epoch (not timestamp). Apply is fail-closed
                        until controller/workflow marker enforcement exists;
                        use --dry-run to inspect the restore plan.
''',
        "CLI restore safety text",
    )
    write(cli_path, cli)


def patch_repair_index_contract() -> None:
    path = "proto/repository.proto"
    text = read(path)
    text = regex_once(
        text,
        r'''\n  // RepairIndex reconstitutes PUBLISHED index authority.*?\n  rpc RepairIndex\(RepairIndexRequest\) returns\(RepairIndexResponse\) \{.*?\n  \}\n''',
        "\n",
        "remove unwired RepairIndex RPC",
    )
    for message in ("RepairIndexRequest", "RepairIndexItem", "RepairIndexResponse"):
        text = regex_once(
            text,
            rf'''\nmessage {message} \{{.*?\n\}}\n''',
            "\n",
            f"remove unwired {message}",
        )
    if "RepairIndex(" in text or "message RepairIndex" in text:
        raise RuntimeError("RepairIndex remains advertised in source proto")
    write(path, text)

    helper_path = "golang/repository/repository_server/artifact_repair_index.go"
    helper = read(helper_path)
    helper = regex_once(
        helper,
        r'''// ── POST-PROTO-REGEN WIRING \(documentation only; not compiled here\) ──────────.*?(?=// Note on the repository_artifact_lifecycle_stuck repair plan)''',
        '''// RepairIndex remains an owner-internal, test-proven operation. No RPC is
// advertised until the source proto, generated bindings, server handler, CLI,
// and MCP surface can land atomically. This prevents source/generated contract
// drift from claiming an operation the running service cannot expose.
//
''',
        "RepairIndex deployment status",
    )
    write(helper_path, helper)


if __name__ == "__main__":
    patch_release_workflow()
    patch_identity_gate()
    patch_restore_safety()
    patch_repair_index_contract()
    print("PR #221 review fixes applied")
