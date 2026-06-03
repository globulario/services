# Deterministic Systemd Unit Rendering

## Status

**Open architectural bug.** Sibling concern to [retire-systemd-sidecars.md](retire-systemd-sidecars.md). Not blocking the sidecar retirement, but the sidecar retirement does not address this root cause and would re-surface it the moment installed_state proofs need to be regenerated.

## Invariant

> **Same package + same desired state + same render inputs MUST produce byte-identical systemd unit output.**

Two `apply-package` invocations for the same `(publisher, name, version, build_id)` against the same install path must produce a unit file whose sha256 is identical.

## Evidence this is currently violated

Captured live on globule-ryzen 2026-06-03:

- `node-agent@1.2.143` (build_id `019e8da6-...`) was installed at 13:01 EDT.
  - Resulting `/etc/systemd/system/globular-node-agent.service`: sha256 `63670501a2f8882527c2243282c24a45844570ff72599a8f6153c70ac39e9a45`.
- The same package was reinstalled at 15:20 EDT by `globular services apply-desired` (no version change, no build_id change, no desired-state change).
  - Resulting unit file: sha256 `0fe9aafc3cea0ebc86494d862c44a6de8e803f10c927ddf4a60fb7edd308c751`.
- Diff between the two renderings was substantive, not whitespace:
  - `After=` line: `globular-etcd.service` → `globular-event.service globular-cluster-controller.service`
  - `Requires=` line: present in v1 → removed in v2
  - `WorkingDirectory=` / `Environment=` lines: absent in v1 → present in v2
  - `ExecStartPre=`: one line in v1 → two lines in v2 (mkdir+chown injected)

The package was identical. The renderer's inputs (template variables, ambient config, derived dependency declarations) had silently changed between the two invocations.

## Why this matters

The sidecar retirement (see sibling doc) moves expected-unit-sha into `installed_state.metadata`. That fixes "the sidecar doesn't get updated when the unit does." It does NOT fix "the unit content differs every time we render it."

After sidecar retirement, the failure mode becomes:

1. Initial install at T₀ → renders unit, stamps `installed_state.metadata.unit_file_sha256 = SHA_T0`.
2. Re-install at T₁ (e.g. unrelated `apply-desired`, recovery action, repair workflow) → renders the SAME package, but produces unit with `SHA_T1 ≠ SHA_T0`.
3. The new install correctly stamps `installed_state.metadata.unit_file_sha256 = SHA_T1`.
4. But: between step 2's write and step 3's stamp, the heartbeat may compute `sha256(unit) = SHA_T1`, read `installed_state.metadata.unit_file_sha256 = SHA_T0` (not yet updated) → report `unit_file_drift`.

Even atomically, the broader problem persists: any time TWO install paths render the same package non-deterministically, they will write different unit files and "fix" each other's drift.

The sidecar pattern masked this by also being non-deterministic — each install rewrote both the unit AND the sidecar, so the local cache stayed self-consistent even if it disagreed with what other installs would have produced. The retirement to etcd-authoritative state exposes that the renderer itself is the source of variability.

## Likely causes (to investigate)

Not yet fully diagnosed. Candidates:

1. **Template variable expansion changes between renders** — e.g., dependency lists computed at render time from the current installed-state snapshot (which evolves over the cluster's lifetime).
2. **Conditional injections based on package config** — e.g., `mkdir+chown ExecStartPre` only emitted when certain metadata is present, and that metadata gets added in a later install path.
3. **WorkingDirectory normalisation** (`normalizeUnitWorkingDirectory`) being applied non-idempotently — runs only when input has a particular shape, so first install with shape A produces output X, second install (input now shape X) skips normalisation and produces output X (idempotent in this case, but other normalisers may not be).
4. **External rendering rules** evolving via `globular_service`/spec scanner/installer template helpers — fixes applied between releases that change rendered output without bumping the package version.

## What the fix looks like

A `unit_renderer_version` field stamped into `installed_state.metadata` (proposed in [retire-systemd-sidecars.md](retire-systemd-sidecars.md#installed-state-should-become-an-installation-receipt)) gives the runtime a way to distinguish "different bytes, intentional renderer evolution" from "different bytes, same renderer = bug." That field is necessary but not sufficient.

The architectural fix is:

```text
Render(pkg_manifest, ambient_inputs) → deterministic bytes
```

where `ambient_inputs` is an explicit, declared input set — not "whatever the renderer happens to read at the moment of invocation." Anything outside the declared input set must not affect output. Reproducible-build discipline applied to systemd units.

Concrete steps (deferred for separate implementation):

1. Inventory every reader in the unit-rendering code path (`render_template_vars`, `normalize_unit_working_directory`, spec scanner, installer template helpers).
2. Each reader becomes an explicit `ambient_inputs` field — declared, versioned, and recorded in `installed_state.metadata.unit_renderer_version`.
3. Snapshot the `ambient_inputs` at install time and stamp them in `installed_state.metadata.renderer_inputs_sha256`. Drift detection becomes: `sha(unit_now) ≠ stamped` is a real drift; `sha(ambient_now) ≠ stamped` is a render-input change (separate, expected, requires bump).
4. Test: render the same package against the same `ambient_inputs` twice → assert byte-identical output.

## Why this is NOT being fixed in the same commit as sidecar retirement

The sidecar retirement is a contained architectural cleanup: stop writing to one path, start writing to another, retire the legacy reader. Bounded blast radius. The renderer-determinism fix is investigation-driven: we don't yet know how many template-variable readers exist, what `ambient_inputs` should be, or whether the existing render path can be made deterministic without rewriting it.

Combining the two would either:
- Block sidecar retirement on an unbounded investigation, OR
- Quietly ship renderer changes alongside a different refactor, breaking review.

Tracking it as a sibling open bug keeps both honest.

## Required tests (when this is addressed)

```text
1. render(pkg, inputs) is pure: same args → same bytes, across multiple invocations.
2. ambient inputs are declared: rendering MUST NOT silently read state outside its declared input set.
3. renderer version is stamped: every install records unit_renderer_version in installed_state.metadata.
4. renderer-version changes are observable: drift detection distinguishes "renderer changed" from "unit changed."
```

## References

- Sidecar retirement spec: [retire-systemd-sidecars.md](retire-systemd-sidecars.md)
- Install action: `golang/node_agent/node_agent_server/internal/actions/artifact.go`
- Template renderer: `renderTemplateVars` in the same file
- Unit normaliser: `normalizeUnitWorkingDirectory`
- Spec scanner: `golang/globularcli/pkgpack/specscan.go`
