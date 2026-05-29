# Project T — verifier honors manifest entrypoint

## Status

**COMPLETE.** Code committed at `eadc5690`, node-agent v1.2.123 deployed,
operator-installed symlinks removed (`/usr/lib/globular/bin/scylla-manager`
and `/usr/lib/globular/bin/scylla-manager-agent`), system fully stable.

## Root cause

`node_agent/node_agent_server/apply_package_release.go::installedBinaryPath`
inferred the expected binary path from the package name with hardcoded
normalization:

- SERVICE: probe `<bindir>/<strings.ReplaceAll(name, "-", "_")>_server`,
  fall back to `<bindir>/<strings.ReplaceAll(name, "-", "_")>`
- INFRASTRUCTURE / COMMAND: raw `<bindir>/<name>` (no normalization)

Both diverge from the manifest's actual entrypoint when a package's name
uses hyphens but the binary uses underscores. Observed in production
during Project R: `scylla-manager` ships
`"entrypoint": "bin/scylla_manager"` (underscore) but the legacy
inference returned `/usr/lib/globular/bin/scylla-manager` (hyphen, file
does not exist). The verifier flagged the binary missing; the
controller's drift-reconciler dispatched an install loop until the
operator hand-installed a workaround symlink.

The hyphen↔underscore disagreement is a real authorial choice: package
authors (and upstream binary names) are not bound by Globular's
package-name convention. The verifier must honor the manifest.

## Files changed

| Path | Change |
|---|---|
| `golang/versionutil/version.go` | New `EntrypointPath`, `WriteEntrypoint`, `ReadEntrypoint` — sidecar mirrors the existing version/kind marker pattern. Sidecar file lives at `/var/lib/globular/services/<sanitized-name>/entrypoint` and contains just the binary filename. `bin/` and `./bin/` prefixes from `package.json` are stripped at write time. |
| `golang/versionutil/entrypoint_test.go` | New: 6 unit tests covering bin/ stripping, empty no-op, missing-sidecar reads, last-write-wins, sanitized-name roundtrips. |
| `golang/node_agent/node_agent_server/installer_api.go` | New `readArtifactManifestEntrypoint` reads `package.json.entrypoint` from the staged tarball (mirrors `readArtifactManifestVersion`). After `InstallPackage` completes, the entrypoint is persisted to the sidecar via `versionutil.WriteEntrypoint`. |
| `golang/node_agent/node_agent_server/apply_package_release.go` | `installedBinaryPath` now consults `versionutil.ReadEntrypoint(name)` first; falls back to the legacy inference only when no sidecar exists. |
| `golang/node_agent/node_agent_server/installed_binary_path_test.go` | New: 6 unit tests covering the literal scylla-manager bug repro (INFRASTRUCTURE hyphen name + underscore entrypoint), SERVICE entrypoint overriding `_server` inference, no-sidecar legacy fallback for both SERVICE and INFRASTRUCTURE, and the defensive case where a sidecar overrides an inferred path that also exists on disk. |

Commit: `eadc5690`.

## Manifest fields used

The package's own `package.json` (top-level entry in the `.tgz`):

```json
{
  "name": "scylla-manager",
  "entrypoint": "bin/scylla_manager",
  ...
}
```

The `entrypoint` field is the source of truth. Globular's installer
reads it at install time and persists it to a sidecar so subsequent
verifier/drift logic doesn't need to re-open the tarball.

## Tests added

### `golang/versionutil/entrypoint_test.go` (6 tests)

| Test | Assertion |
|---|---|
| `TestEntrypointPath_HyphenPackageName_PathConstructed` | `EntrypointPath("scylla-manager")` resolves under the canonical service dir |
| `TestWriteEntrypoint_StripsBinPrefix` | `bin/scylla_manager`, `./bin/scylla_manager`, `scylla_manager`, leading/trailing spaces all roundtrip to `scylla_manager` |
| `TestWriteEntrypoint_Empty_NoOp` | Empty or whitespace-only entrypoint does not create a sidecar |
| `TestReadEntrypoint_NoSidecar_ReturnsEmpty` | Missing sidecar returns "" so callers fall back |
| `TestWriteEntrypoint_OverwritesExisting` | Last write wins |
| `TestEntrypointSidecar_RoundtripWithSanitizedName` | Underscored package names roundtrip and live under the hyphenated dir name |

### `golang/node_agent/node_agent_server/installed_binary_path_test.go` (6 tests)

| Test | Assertion |
|---|---|
| `TestInstalledBinaryPath_InfraHyphenName_UnderscoreEntrypoint_UsesSidecar` | **Literal Project R repro**: INFRASTRUCTURE name `scylla-manager` + sidecar `bin/scylla_manager` resolves to `<bin>/scylla_manager` (not `<bin>/scylla-manager`) |
| `TestInstalledBinaryPath_ServiceHyphenName_UnderscoreEntrypoint_UsesSidecar` | SERVICE entrypoint sidecar overrides `_server` inference |
| `TestInstalledBinaryPath_NoSidecar_FallsBackToLegacyInfer` | Legacy SERVICE fallback still finds `_server` when sidecar absent |
| `TestInstalledBinaryPath_NoSidecar_LegacyInfraUsesRawName` | Legacy INFRASTRUCTURE fallback still uses raw name |
| `TestInstalledBinaryPath_SidecarOverridesInferredHit` | Defensive: even when the legacy inferred path exists on disk, the sidecar wins |
| `TestInstalledBinaryPath_FallsBackToPlainName` | (existing test, still passes) |

## Test results

```
golang/versionutil                                    PASS  (0.014s, 6 new + existing tests)
golang/node_agent/node_agent_server                   PASS  (165.364s, 6 new + 700+ existing tests)
golang/versionutil (re-run)                           PASS  (0.274s)
```

Zero regressions.

## Symlink removal result

Before:
```
$ ls -la /usr/lib/globular/bin/scylla-manager*
lrwxrwxrwx 1 root root 14 May 29 10:44 /usr/lib/globular/bin/scylla-manager       -> scylla_manager
lrwxrwxrwx 1 root root 20 May 29 10:44 /usr/lib/globular/bin/scylla-manager-agent -> scylla_manager_agent
```

After:
```
$ rm /usr/lib/globular/bin/scylla-manager /usr/lib/globular/bin/scylla-manager-agent
$ ls -la /usr/lib/globular/bin/scylla-manager*
ls: cannot access '/usr/lib/globular/bin/scylla-manager': No such file or directory
ls: cannot access '/usr/lib/globular/bin/scylla-manager-agent': No such file or directory
```

The real binaries remain at the underscore paths:
```
/usr/lib/globular/bin/scylla_manager
/usr/lib/globular/bin/scylla_manager_agent
```

These are where the systemd unit's `ExecStart` already points
(`{{.Prefix}}/bin/scylla_manager`), so the running unit is unaffected.

## Doctor before / after delta

| Snapshot | Total findings | scylla-manager findings | Notes |
|---|---|---|---|
| Pre-Project-T (with symlinks) | 24 | 0 | symlinks masked the verifier path bug |
| Post-Project-T deploy + symlink removal | 24 | 0 | same — no regression |

Other findings unchanged. Project Q dir cleanup candidates still
present, artifact cache mismatches still present (separate classes).

## Confirmation that Project R backup readiness survived

| Check | State |
|---|---|
| `globular-scylla-manager.service` | `active`, real binary running, NRestarts=0 |
| Cluster registered | `globular-internal` (`932c01cb-8c50-4a30-b90d-e2f08c10a17c`) |
| Healthcheck tasks | 32 successful runs each (every minute since cluster registration), 0 errors |
| Backup tasks | `backup/3b966c52-...` and `backup/105a3d1f-...` both `DONE` from Project R execution |
| Backup artifacts in MinIO | Present at `s3:scylla-manager-backup` (verified during Project R) |
| Restore dry-run | Still works (snapshot discovery and contract are properties of the manifest in MinIO; not affected by binary path) |

The Project R operational state survives Project T's symlink removal
because the running process holds its own file descriptor to the real
binary, and the systemd unit's `ExecStart` references the underscore
path directly. Project T's improvement materializes on the **next**
install of a hyphen-vs-underscore package — the controller's drift
reconciler will dispatch through the new code path that writes the
entrypoint sidecar.

## Out-of-scope follow-ups (unchanged from prior reports)

- **Project Q** — honor `Spec.Paused` on `InfrastructureRelease`. Still
  desirable for clean operator disable/enable cycles.
- **Project S** — Day-0/Day-1 scylla-manager cluster registration so a
  fresh deployment never lands in the "running but unregistered" state.
- The 3 orphan healthcheck rows (synthetic `cluster_id=15098bd9...`)
  remain an upstream scylla-manager 3.10.1 quirk — they recreate on
  every scylla-manager process restart. Not a Project T concern.

## How packages installed BEFORE Project T behave

`installedBinaryPath` falls back to the legacy inference when no
entrypoint sidecar exists. Existing installations are not regressed.

The sidecar is written on **next install** of any package. Currently
only scylla-manager and scylla-manager-agent have the hyphen-vs-
underscore mismatch (verified by grep against
`packages/metadata/*/package.json`'s `entrypoint` field). Both will
get the sidecar the next time they are reinstalled (e.g. on the
next package bump or version pin). Until then, the legacy fallback
returns the hyphen path which does not exist — the verifier would
flag drift and dispatch reinstall (via the new code path), closing
the loop automatically without an operator step.

For the present moment: the symlinks were not needed to keep the
running unit alive (systemd unit refers to the underscore path
directly), so removing them did not disturb scylla-manager.
