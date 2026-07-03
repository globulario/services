# Build & Release — local `dist/` and GitHub CI

Canonical reference for how a Globular release is built, both **locally**
(`scripts/build-release.sh` → `services/dist/`) and in **CI** (GitHub Actions:
`ci.yml`, `docs.yml`, `release.yml`). Both paths produce the same artifact shape:
a versioned bundle directory and a `.tar.gz` + `.sha256`, plus a v2 BOM
`release-index.json`.

> **PRIME RULES that govern this area**
> - `release-index.json` is the platform-release truth; never confuse platform
>   release with per-package version.
> - `services/dist` and `services/generated` are **disposable** output/workspace,
>   never authored source or install-recipe authority.
> - Package **identity** is authored in `packages/registry.yaml`; the **recipe**
>   in `packages/metadata/<name>/specs`. Everything downstream is a projection.
> - Install **execution** authority is `globular-installer/scripts/install-day0.sh`;
>   `services/scripts/install.sh` is only a release wrapper.

---

## 1. Repository layout the build depends on

The build is **multi-repo**. `build-release.sh` and `release.yml` both check out /
expect these siblings next to `services/`:

| Repo | Role in the build |
|------|-------------------|
| `services/` | Go services source, `scripts/build-release.sh`, release wrapper `scripts/install.sh`, Day-0 script `scripts/release/install-day0.sh`, `webroot/`, workflow definitions |
| `packages/` | **`registry.yaml`** (package identity authority) + `metadata/<name>/specs` (recipe authority) + `dist/` prebuilt infra/command package artifacts |
| `globular-installer/` | The `globular-installer` binary (install execution authority) + `internal/packagecatalog` and `internal/assets/webroot` generated mirrors |
| `Globular/` | Source for `xds` and `gateway` binaries (`./cmd/xds`, `./cmd/gateway`) |
| `awareness-graph/` | (CI only) the `awg` CLI used by the CI awareness gates |

---

## 2. Local build — `scripts/build-release.sh`

```bash
cd services
bash scripts/build-release.sh [version] [--bump patch|minor|major] [--full-regenerate]
```

**Output** (into `services/dist/`, which is wiped and recreated each run):

```
dist/globular-<version>-linux-amd64/          # unpacked bundle
  ├── globular                                # CLI
  ├── globular-installer                      # install execution binary
  ├── packages/*.tgz + SHA256SUMS
  ├── release-index.json                      # v2 BOM (platform-release truth)
  ├── scripts/                                # install-day0.sh + installer scripts
  ├── workflows/*.yaml
  ├── webroot/
  └── docs/operational-knowledge/             # ops-knowledge seed (Day-0 ai-memory)
dist/globular-<version>-linux-amd64.tar.gz
dist/globular-<version>-linux-amd64.tar.gz.sha256
dist/.staging/                                # transient assembly area
```

### Version selection
- With an explicit `[version]` → that version.
- Otherwise it refreshes tags from `origin` and bumps the latest `vX.Y.Z` tag
  (default `--bump patch`). e.g. latest `v1.2.266` → `1.2.267`.

### `generated/` workspace and `--full-regenerate`
- `services/generated/` holds regenerated release inputs: per-service package
  **templates** (`generated/<name>_<ver>_linux_amd64.tgz`), policy, specs, and a
  `release-inputs.manifest.json` freshness stamp.
- **`--full-regenerate`** runs `scripts/regenerate-release-inputs.sh` first, which
  wipes and rebuilds those inputs from the authoritative `packages/` metadata.
- Without the flag, the script **validates** the existing `generated/` inputs
  against the release version and reuses them (faster). Binaries are **always**
  recompiled fresh (`-trimpath -ldflags "-X main.Version=<ver> -s -w"`); the
  service package `.tgz` is the fresh binary + the generated template.

### Two known gotchas (both cost real time — read before building)

1. **`METADATA_ROOT` override for the flattened `packages` layout.**
   `globular-installer`'s `make check-specs` (invoked by the build to validate the
   installer mirror) defaults `METADATA_ROOT=../packages/metadata`. On a `packages`
   checkout where metadata has been **flattened to the repo root**
   (`packages/<name>/specs/…`), that path does not exist and the build fails at
   *"metadata root not found"*. Run with the override:
   ```bash
   METADATA_ROOT=/abs/path/to/packages \
     bash scripts/build-release.sh
   ```
   If `check-specs` then reports `STALE`, re-sync the installer mirror:
   `make -C ../globular-installer sync-specs METADATA_ROOT=/abs/path/to/packages`.

2. **`dist/` is disposable — do not run the installer against it in place.**
   The build starts by `rm -rf`-ing `dist/`. If a prior `sudo` install (or
   `globular-installer`) ran **against the extracted bundle inside `dist/`**, it
   creates **root-owned** symlinks/dirs there (`bin/globular-installer`,
   `internal/assets/packages`) that the non-root build cannot remove →
   *"Permission denied"*. Always extract and install from a copy **outside** `dist/`:
   ```bash
   mkdir -p /tmp/globular-test
   tar xzf dist/globular-<ver>-linux-amd64.tar.gz -C /tmp/globular-test
   sudo bash /tmp/globular-test/globular-<ver>-linux-amd64/install.sh
   ```
   Recovery if it already happened: `sudo rm -rf dist/globular-<ver>-linux-amd64`.

### What the bundle ships for Day-0 ops-knowledge
`build-release.sh` copies `services/docs/operational-knowledge` into
`<bundle>/docs/operational-knowledge`. `install-day0.sh` reads
`$INSTALLER_ROOT/docs/operational-knowledge` to seed ai-memory; if it is missing,
Day-0 logs *"operational-knowledge directory not found — skipping ops-knowledge
seed"* and defers to Day-1.

### Shared Day-0 / Day-1 bundle contract
Day-0 and Day-1 must rely on the same release-bundle shape. The release tarball
is not just a Day-0 artifact; it is also the source shape that Day-1 expects
when joining from GitHub or from a controller-local fallback bundle. Minimum
contract:

- `globular-installer` at the bundle root
- `packages/`
- `workflows/`
- `release-index.json`
- `scripts/install-day0.sh`

Day-1 does not ship a bundled join script: the canonical Day-1 path is the
gateway-served join script (`Globular/internal/gateway/handlers/cluster/join_script.go`,
served at `/join`), which consumes the bundle's `globular-installer`, `packages/`,
`workflows/`, and `release-index.json`. There is no `scripts/install-day1.sh`.

The Day-0 wrapper `services/scripts/install.sh` must also persist
`globular-installer` to `/usr/lib/globular/bin/globular-installer`. Without
that, a controller cannot reconstruct an equivalent Day-1 fallback bundle after
the original extracted release tree is gone.

---

## 3. CI — GitHub Actions

### `ci.yml` — build/test gate (`name: ci`)
- **Triggers:** `push` and `pull_request` to `master`/`main`.
- **Jobs:**
  - **`build-test`** — checks out services + siblings (utility, globular-installer,
    packages, awareness-graph), `go build`, then the hard gates:
    - `make check-services` (service security boundaries)
    - Invariant tests, Unit tests
    - **impact-ci** required tests (awareness-driven: tests required by the changed files)
    - **principle-check** state-mutation invariants (hard gate)
    - **behavioral ratchets** (hard gate)
    - **`awg validate`** (awareness YAML validity, hard gate)
    - **`awg audit -check`** (graph self-audit, hard gate)
  - **`proto-check`** — verifies the `.proto` files still compile.
  - **`lint`** — Go lint.

### `docs.yml` — documentation & path/gating checks (`name: Documentation`)
- **Triggers:** `push`/`pull_request` on doc-relevant `paths`.
- **Jobs:**
  - **`build-docs`** — builds the mkdocs-material site.
  - **`validate-cli-commands`** — builds the CLI and asserts every CLI command
    referenced in the docs actually exists.
  - **`check-paths`** — fails on stale filesystem paths referenced in docs.
  - **`break-glass-gating`** — enforces break-glass gating on owner-state-mutating
    scripts.

### `release.yml` — the release build (`name: Release`)
- **Triggers:**
  - `push` of a tag matching **`v*`** (the normal path — tag → release), and
  - **`workflow_dispatch`** with inputs: `version` (required for manual runs),
    `force_full_rebuild` (treat all packages as changed), `force_reason`.
- **`permissions: contents: write`** (needed to create the GitHub Release).
- **Single `release` job**, key stages in order:
  1. Checkout services + pinned siblings; **gates**: installer spec kinds match
     `registry.yaml` (single-source), package-authority layout coherent.
  2. **Extract version** from the tag/input.
  3. **BOM change detection** — build all Go services with a **sentinel version**
     (`0.0.0-detect`) so binary checksums are version-independent, fetch the
     previous release-index, and detect which packages actually **changed**.
  4. **Rebuild only changed** Go services (and xds/gateway) with the **real**
     version; run tests; build `globular-installer`; generate build identity.
  5. **Package changed packages**; **carry-forward unchanged** packages by
     downloading them from their **origin** GitHub release (repairing/re-authoring
     only if a historical package no longer passes current validation).
  6. **Validate** carry-forward `entrypoint_checksum` against the actual tarball
     binaries — refuses to publish a release whose BOM entries don't match.
  7. **Generate `release-index.json` (v2 BOM)** and **assemble the tarball**
     (bundle dir → `SHA256SUMS` → `.tar.gz` + `.sha256`); validate the extracted
     tarball and the BOM delta integrity.
  8. **Create GitHub Release** (`softprops/action-gh-release@v2`) attaching:
     `globular-<ver>-linux-amd64.tar.gz` + `.sha256`, `dist/packages/*.tgz`, and
     `dist/release-index.json`; auto-generated notes.

**BOM model:** source-**changed** packages are uploaded as individual assets;
unchanged packages are **carry-forward** references to their origin release (unless
they had to be repaired for this release). This keeps releases small and makes
`release-index.json` the authoritative BOM across releases.

---

## 4. Local ↔ CI parity and differences

- `build-release.sh` is explicitly a local mirror of what `release.yml` does, and
  produces the **same bundle shape** (`dist/globular-<ver>-linux-amd64{,.tar.gz}`
  + v2 `release-index.json`).
- **CI does BOM change-detection + carry-forward** from prior GitHub releases
  (small releases, cross-release BOM authority). The **local** build packages from
  `services/generated` + `packages/dist` and does **not** carry-forward from
  GitHub — its `--upstream`/BOM story is simpler; a locally tag-not-yet-published
  version is expected ("Tag vX not published on upstream — Day-0 continues with
  locally published artifacts").
- CI runs the **awareness/behavioral hard gates** (`ci.yml`); the local build does
  not. Run `make check-services` and `go test ./... -race` locally before tagging.

---

## 5. Cutting a release (normal path)

1. Land changes on `master` (CI `ci.yml` green).
2. Tag: `git tag vX.Y.Z && git push origin vX.Y.Z`.
3. `release.yml` fires on the `v*` tag, builds the BOM release, and publishes the
   GitHub Release with the tarball, per-package `.tgz`, and `release-index.json`.
4. Day-0/Day-1 installs read **`release-index.json`** as the platform-release truth.

For a **local** dry run of the same bundle: `bash scripts/build-release.sh` (add
`METADATA_ROOT=…` if `packages` metadata is flattened).
