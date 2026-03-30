# Implementation Plan: Reuse globular-installer as Shared Infra Install Engine

## 1. Summary Recommendation

**Import the installer's spec execution engine as a Go library into node-agent.** The installer is already architecturally perfect for this: stateless runner, clean interfaces, zero imports from the services repo, spec-driven execution. The `internal/` packages need to be made public (renamed to a public path), and node-agent's `infrastructure.install` action should delegate to the installer's `Install()` function instead of reimplementing extraction logic.

The migration is low-risk because:
- Day 0 continues using the same installer binary (unchanged)
- etcd join state machine stays in the controller (untouched)
- Only the *execution* of infra package install is redirected through shared code
- Rollback is trivial: revert the import and keep the old `infrastructure.install` code behind a feature flag

---

## 2. Current State Analysis

### What's duplicated

| Behavior | Installer (`internal/installer/`) | Node-Agent (`internal/actions/`) |
|----------|-----------------------------------|----------------------------------|
| Archive extraction | `install_package_payload_step.go` — manifest-driven, validates `package.json`, copies bin/config/systemd/spec with template rendering | `infrastructure_actions.go` — hardcoded `bin/`→binDir, `systemd/`→systemdDir, `config/`→configDir/{name} |
| Lifecycle scripts | `run_script_step.go` — spec-driven, named script, configurable timeout | `infrastructure_actions.go` — hardcoded pre-install.sh, pre-start.sh, post-install.sh |
| systemd reload | `install_services_step.go` + `start_services_step.go` — Check/Apply idempotent | `infrastructure_actions.go` — always `daemon-reload` if units written |
| Service start | `start_services_step.go` — with restart-on-file-change, binary tracking | `service.go` — simple `systemctl start` |
| Health checks | `health_checks_step.go` — poll systemd active + optional port check | Plan probes — `probe.tcp`, `probe.exec`, `probe.service_config_tcp` |
| Template rendering | `text/template` on spec YAML, `{{.Prefix}}`, `{{.NodeIP}}`, etc. | None — raw file copy |
| Idempotency | `Check()` method on every step — `StatusOK` means skip | None — always overwrites |
| Config validation | `ensure_service_config_step.go` — port allocation, address validation | None |

### What's NOT duplicated (correctly separate)

- **Plan compilation**: Only in controller (`infrastructure_compiler.go`)
- **Plan execution**: Only in node-agent (`plan_runner.go` → `planexec`)
- **Artifact fetch**: Only in node-agent (`artifact.go`)
- **State reporting**: Only in node-agent (`package_state.go`)
- **etcd join**: Only in controller (`etcd_members.go`)
- **Service probes**: Only in controller (`operator/*.go`)

### Key insight

The installer's `Install()` function = `Load spec → Build plan → Run steps`. The node-agent's `infrastructure.install` = `Extract tar → Run scripts → Reload systemd`. They do the same thing with different quality levels. The installer does it **better** (idempotent, Check/Apply pattern, template rendering, manifest tracking).

---

## 3. Target Architecture

```
                     ┌──────────────────┐
                     │  Cluster         │
                     │  Controller      │
                     │                  │
                     │  • Resolves      │
                     │    profiles      │
                     │  • Compiles      │
                     │    plans         │
                     │  • Gates         │
                     │    workloads     │
                     │  • etcd join     │
                     │    state machine │
                     └────────┬─────────┘
                              │ PutCurrentPlan(nodeID, plan)
                              ▼
                     ┌──────────────────┐
                     │  Node Agent      │
                     │                  │
                     │  • Polls plans   │
                     │  • Runs steps    │
                     │  • Reports state │
                     │                  │
                     │  infrastructure. │
                     │  install action: │
                     │  ┌──────────────┐│
                     │  │ Shared       ││
                     │  │ Installer    ││  ← imported as Go library
                     │  │ Engine       ││
                     │  │              ││
                     │  │ • Load spec  ││
                     │  │ • Build plan ││
                     │  │ • Run steps  ││
                     │  │ • Report     ││
                     │  └──────────────┘│
                     └──────────────────┘
```

**Responsibilities:**

| Actor | Resolves metadata | Chooses order | Chooses mode | Executes steps | Reports state |
|-------|------------------|---------------|--------------|----------------|---------------|
| Controller | ✓ (versions, digests, profiles) | ✓ (plan steps) | ✓ (install/upgrade/rollback) | | |
| Node-agent | | | | ✓ (action dispatch) | ✓ (package.report_state) |
| Installer engine | | ✓ (spec step order) | | ✓ (Check/Apply per step) | |

The controller decides **what** and **when**. The installer engine decides **how** (reading the spec embedded in the package).

---

## 4. Recommended Reuse Model

### Recommendation: Go library import (not CLI subprocess)

**Why library, not subprocess:**
- The installer's `Install()` function returns structured `RunReport` — perfect for node-agent to inspect and report
- No serialization overhead
- Error handling is Go-native (no exit code parsing)
- Template vars can be computed in-process (node IP, state dir, etc.)
- Single binary — no need to ship separate installer binary to joining nodes

**Why NOT extracting to a new module:**
- The installer is already a separate Go module (`github.com/globulario/globular-installer`)
- Just need to rename `internal/installer` → `pkg/installer` (or `installer/`) to make it importable
- The services repo's `go.work` or `go.mod` can reference the installer module directly

### Concrete model

```go
// In node-agent's infrastructure_actions.go:

import (
    "github.com/globulario/globular-installer/pkg/installer"
    "github.com/globulario/globular-installer/pkg/installer/spec"
)

func (a infrastructureInstallAction) Apply(ctx context.Context, args *structpb.Struct) (string, error) {
    component := args.Fields["name"].GetStringValue()
    version := args.Fields["version"].GetStringValue()
    artifactPath := args.Fields["artifact_path"].GetStringValue()

    // 1. Stage the package (extract to temp dir)
    stagingDir, err := stagePackage(artifactPath)
    if err != nil {
        return "", err
    }
    defer os.RemoveAll(stagingDir)

    // 2. Create installer context with node-specific values
    opts := installer.Options{
        Version:    version,
        Prefix:     installPrefix(),
        StateDir:   stateDir(),
        ConfigDir:  configDir(),
        StagingDir: stagingDir,
        Force:      true,
        Verbose:    true,
    }
    ictx, err := installer.NewContext(opts)
    if err != nil {
        return "", fmt.Errorf("create installer context: %w", err)
    }

    // 3. Run the installer (spec from package)
    report, err := installer.Install(ictx)
    if err != nil {
        return "", fmt.Errorf("installer failed: %w", err)
    }

    // 4. Summarize results
    return summarizeReport(component, version, report), nil
}
```

---

## 5. Exact Code Changes

### Phase 1: Export installer API (globular-installer repo)

**Rename** `internal/installer/` → `pkg/installer/`

This makes the following types importable:
- `installer.Step`, `installer.StepStatus`
- `installer.Plan`, `installer.NewPlan`
- `installer.Context`, `installer.Options`, `installer.NewContext`
- `installer.Runner`, `installer.NewRunner`, `installer.RunMode`
- `installer.RunReport`, `installer.StepResult`
- `installer.Install`, `installer.Uninstall`
- `installer.BuildInstallPlan`, `installer.BuildUninstallPlan`
- `installer.Logger`

Also rename:
- `internal/installer/spec/` → `pkg/installer/spec/`
- `internal/installer/manifest/` → `pkg/installer/manifest/`
- `internal/platform/` → `pkg/platform/`

**Update** all internal imports in the installer repo (find-replace `internal/` → `pkg/`).

**Update** `cmd/globular-installer/main.go` to import from new paths.

No behavioral changes — just path renames.

### Phase 2: Add installer dependency to services (services repo)

Add to `golang/go.mod`:
```
require github.com/globulario/globular-installer v0.0.0-...
```

Or in `go.work`:
```
use ../globular-installer
```

### Phase 3: Rewrite infrastructure.install (services/node-agent)

Replace the current `infrastructure_actions.go` Apply method with installer delegation:

**Keep:**
- `stagePackage()` helper (extract .tgz to temp dir — reuse existing extraction loop but route to staging dir)
- Lifecycle script execution (pre-install, pre-start, post-install) — these now happen inside the installer's `run_script` step
- `detectNodeIP()`, `installPaths()` helpers

**Remove:**
- Manual bin/systemd/config routing (lines 96-156)
- Manual daemon-reload (lines 177-185)
- Manual data dir creation (lines 187-198)

**Add:**
- `installer.NewContext()` with node-specific options
- `installer.Install()` call
- Report summarization

### Phase 4: Handle packages without specs (backward compat)

Old packages don't have specs embedded. The installer needs a fallback:

```go
// If no spec in staging dir, fall back to legacy extraction
if _, err := os.Stat(filepath.Join(stagingDir, "specs")); os.IsNotExist(err) {
    return legacyInfraInstall(ctx, args) // old code path
}
```

This allows gradual migration: rebuild packages with specs, they use the new path; old packages use the old path.

### Phase 5: etcd handling (no change)

etcd join state machine stays in the controller (`etcd_members.go`). The only change is that etcd *package installation* (binary + systemd unit + config) goes through the shared installer instead of raw extraction. The cluster-level join (MemberAdd, config rendering, phase transitions) remains controller-owned.

---

## 6. Phased Migration Plan

### Phase A: Export installer API (1 day, zero risk)
- **Code**: Rename `internal/` → `pkg/` in globular-installer
- **Behavior**: Zero change — same binary, same tests
- **Test**: `go build ./...` and `go test ./...` in installer repo
- **Rollback**: `git revert`

### Phase B: Import installer into services, add feature flag (1 day, zero risk)
- **Code**: Add `go.mod` dependency, create `useInstallerEngine` bool flag in infrastructure_actions.go
- **Behavior**: Flag defaults to `false` — no behavior change
- **Test**: `go build ./...` in services repo
- **Rollback**: Remove dependency

### Phase C: Implement installer-backed infrastructure.install (2 days, low risk)
- **Code**: New `installerInfraInstall()` function that delegates to `installer.Install()`
- **Behavior**: When flag is `true`, uses installer; when `false`, uses old code
- **Test**: Run Day 1 join test with flag=true on a test node
- **Rollback**: Set flag to `false`

### Phase D: Rebuild infra packages with specs (1 day, zero risk)
- **Code**: Ensure all infra packages (etcd, minio, scylladb, envoy, xds, gateway, dns, prometheus) have specs with `run_script` steps
- **Behavior**: Packages are now self-contained
- **Test**: Build each package, verify spec and scripts in archive
- **Rollback**: Use old packages

### Phase E: Enable by default, remove legacy code (1 day, medium risk)
- **Code**: Set flag to `true`, keep legacy code behind `false` path
- **Behavior**: All infra installs use installer engine
- **Test**: Full Day 0 + Day 1 test cycle
- **Rollback**: Set flag to `false`

### Phase F: Remove legacy code (1 day, cleanup)
- **Code**: Delete old extraction code, remove feature flag
- **Behavior**: Only installer engine path exists
- **Test**: Same as Phase E
- **Rollback**: `git revert` to Phase E

---

## 7. Test Plan

| Test | Phase | Method |
|------|-------|--------|
| Installer repo builds after rename | A | `go build ./...` |
| Installer tests pass after rename | A | `go test ./...` |
| Services repo builds with installer dep | B | `go build ./...` |
| MinIO package installs via installer engine | C | Manual: install minio on test node |
| ScyllaDB package installs via installer engine | C | Manual: install scylladb on test node |
| Day 0 bootstrap unchanged | C | Full Day 0 install on fresh node |
| Day 1 join with installer engine | D+E | `globular cluster join` on fresh node |
| etcd expansion works | D+E | Verify etcd `member list` shows 2 nodes |
| Workloads blocked until infra healthy | D+E | Watch controller logs during join |
| Rollback to old code path | E | Set flag=false, re-run join |

---

## 8. Risks and Safeguards

### Risk 1: Circular dependency
**Mitigation**: The installer has ZERO imports from the services repo. Verified. The dependency is one-way: services → installer.

### Risk 2: Installer assumes Day 0 environment
**Mitigation**: The installer's `NewContext()` builds template vars including `NodeIP`, `Prefix`, `StateDir`. On Day 1, the node-agent provides these. The installer doesn't hardcode paths — everything comes from `Options`.

**One concern**: `NewContext()` reads embedded assets (`assets.ReadConfigAsset`) for xds/gateway configs. On Day 1, these assets won't be available (they're compiled into the installer binary). Fix: the installer should gracefully skip missing assets (it already does: `if b, err := assets.ReadConfigAsset(...); err == nil`).

### Risk 3: Port allocator conflict
**Mitigation**: Day 1 installs use `ensure_service_config` step which allocates ports. If the node-agent already manages ports, there could be double-allocation. Fix: pass the node-agent's known ports to the installer context, or skip port allocation for infra packages (they use fixed ports).

### Risk 4: Breaking existing packages
**Mitigation**: Phase 4's backward-compat check: if no spec in archive, fall back to legacy code. This means old packages still work.

### Risk 5: Manifest collision
**Mitigation**: The installer writes `install-manifest.json`. Node-agent doesn't use this manifest. They can coexist. Long-term, node-agent should read the manifest to detect what the installer installed.

---

## 9. Final Recommendation

**Do it in 3 real steps:**

1. **Export the installer API** (rename `internal/` → `pkg/`). This is a 30-minute mechanical change with zero risk.

2. **Wire it into node-agent behind a flag**. This lets you test on a single node without affecting the cluster. The flag defaults to off.

3. **Rebuild packages with specs**, enable the flag, and test Day 0 + Day 1. If anything breaks, flip the flag off.

The self-contained packages work we just completed (lifecycle scripts, `run_script` step) was the **prerequisite** for this. Now that packages carry their own setup logic, the installer engine can execute them correctly on any node — Day 0 or Day 1 — without external scripts or hardcoded behavior.

The duplication will be gone. The installer becomes a shared engine. The controller stays the brain. The node-agent stays the executor. Clean boundaries.
