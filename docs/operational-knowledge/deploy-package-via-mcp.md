# How to Deploy a Package — MCP Tool Playbook

This document is the canonical procedure for deploying or updating a Globular service package using MCP tools. It is written for AI agents and operators who have MCP access to the cluster.

**Read this before touching any binary on a node.**

---

## Why this matters

Globular enforces a strict 4-layer truth model:

```
Layer 1: Repository   — "Does this version + checksum exist?"
Layer 2: Desired      — "What should be running?"
Layer 3: Installed    — "What is actually on this node?"
Layer 4: Runtime      — "Is it running and healthy?"
```

The reconciler, verifier, and cluster-doctor all watch these four layers continuously. A deployment is only complete when all four layers agree. **Any shortcut that bypasses even one layer will produce findings, break convergence, and leave other nodes out of sync.**

---

## Versioning rule — bump the BUILD NUMBER, never the version

**This is the most common mistake. Read it before you build.**

For a local / manual redeploy of an *already-released* service (a hotfix, a debug build, an unreleased fix), you MUST keep the service at its **current version** and increment the **build number** — never invent a higher version.

```
CORRECT:   ai-memory 1.2.269+b1  →  1.2.269+b2  →  1.2.269+b3   (version fixed, build bumped)
WRONG:     ai-memory 1.2.269     →  1.2.270      →  1.2.271       (version raced ahead of the platform)
```

**Why:** package versions are allocated by the release pipeline and recorded in the BOM (`release-index.json`). If you publish a service at a version *ahead* of the platform, then when the platform is officially released on GitHub at that version, your service is already "ahead" of it — the version-immutability and convergence checks reject it (e.g. *"ai-memory 1.2.271 — want 1.2.270"*), and the cluster-doctor raises `VERSION_MISMATCH` drift. The build number is exactly the escape hatch for "same version, new binary": it distinguishes rebuilds without advancing the release line.

**Get the service's current version from the authoritative source (never guess, never use a stale file):**

```
mcp__globular__repository_active_release()   # read the version for your <service> from the BOM
```

Then reuse **that** version for `--version`, and set `--build-number` to (current build + 1).

## Use the ACTUAL publisherID + a bumped build number — never a local publisher/channel

**This is THE correct way to publish a local rebuild. Everything else is a trap.**

The publish identity is exactly three fields:

| Field | Value | Rule |
|-------|-------|------|
| `--publisher` | `core@globular.io` | the **actual** publisher the package ships under (read it from the package's own `package.json` / the BOM). **Never invent a local publisher.** |
| `--version` | `<current>` | the version already in the BOM — **unchanged**. |
| `--build-number` | `<current + 1>` | the **only** field that advances. |

That triple `(core@globular.io, <version>, <build+1>)` is a **new, distinct build identity** — and that is precisely why it works with sealing:

> Official stable artifacts are **sealed**: a specific `(publisher, name, version, build)` at a specific digest is immutable. Re-uploading the **same build** with different bytes is rejected — `official identity conflict … is SEALED … incoming artifact has a different digest`. `--force` does **NOT** bypass the seal.

A **build-number bump sidesteps the seal cleanly**: build `N+1` is an identity nothing has sealed yet, so it publishes normally on the STABLE channel. You therefore **never** need `--force`, `--unseal-official`, `--channel local/dev`, or a `+local.<host>.N` version suffix.

Those are the WRONG tools (each was tried and failed 2026-07-12):

- ❌ `--channel local` / `--publisher local@<cluster>` / `1.2.272+local.host.1` suffix → creates a **DEV-lane** artifact that the cluster's STABLE desired-state never resolves, and it lands `CHANNEL_UNSET` — invisible to `explain-package`/desired resolution.
- ❌ `--force` on the same build → hits the seal.
- ✅ `--publisher core@globular.io` + `--build-number <current+1>` → clean new build on STABLE, no seal conflict, no version race.

**Note:** `--build-number` is a **`pkg build`** flag (it is baked into the `.tgz` at build time). `pkg publish` has **no** `--build-number` flag. Set it when you BUILD the package, then publish the resulting tgz.

## Publish as root (or the `globular` user) — or you create a PHANTOM

The publish uploads the blob to the repository's **local POSIX CAS** over an authenticated mesh RPC. That RPC needs a **service token**, minted from `/var/lib/globular/keys/…_private`, which only `root`/`globular` can read. Publish as an ordinary user (e.g. `dave`) and you get a split failure:

- the **manifest upsert** (authenticated by your `sa` JWT) **succeeds**, but
- the **blob upload** (needs the unreadable service-token key) **fails silently**,

leaving a **published manifest with no blob — a phantom**. Nodes resolve the manifest, try to fetch the blob, it is missing → `repository.published_missing_blob` → `missing_package` → the release workflow retries forever → CRITICAL `workflow.drift_stuck`. (This is exactly what happened 2026-07-12; re-publishing **as root** put the blob in the CAS and cleared it.)

Correct CLI flow (when MCP tools are unavailable), run the publish as root:

```bash
# 1. Build the package with the bumped build number and the real publisher.
globular pkg build --spec <spec.yaml> --root <payload-dir> \
  --publisher core@globular.io --version <current> --build-number <current+1> \
  --platform linux_amd64 --out /tmp/out
#    (Go services: build the stripped binary first — see "stripped" below — into <payload-dir>/bin/.)

# 2. Publish AS ROOT so the service token mints and the blob lands in the CAS.
TOKEN=$(globular auth login --user sa --password <pw> 2>/dev/null | grep '^Token:' | sed 's/^Token: //')
echo "$TOKEN" | sudo tee /tmp/sa.tok >/dev/null
sudo bash -c 'globular pkg publish --file /tmp/out/<name>_<version>_linux_amd64.tgz \
  --token "$(cat /tmp/sa.tok)" --repository 127.0.0.1:10007 --ca /var/lib/globular/pki/ca.crt --output json'

# 3. Point desired state at the new build, then reconcile.
globular services desired set <name> <current> --build-number <current+1> --token "$TOKEN"
globular services repair
```

> **Storage model:** package blobs live in each repository instance's **local POSIX CAS — never MinIO**. A joined node materializes a PUBLISHED blob into its own CAS from its **staged join packages**, digest-verified against the manifest (`blob_seed.go`). Consequence: a rebuild changes the checksum, so its blob must actually be published to the instance the nodes resolve from — it will **not** materialize from an older staged package whose digest differs. Do not re-publish an already-healthy external package "for cleanliness"; a checksum change with no matching staged blob re-introduces `published_missing_blob`.

## Release-channel builds MUST be stripped

The repository rejects a release-channel artifact that carries debug sections:
`release artifact carries debug section ".debug_aranges" — release-channel builds must be stripped`.
Always build with `-trimpath -ldflags "… -s -w"` (`-s` strips the symbol table, `-w` strips DWARF).

---

## The forbidden pattern — never do this

```bash
# WRONG — never do this
sudo cp /tmp/my_binary /usr/lib/globular/bin/my_service
sudo systemctl restart globular-my-service
```

**Why it breaks:**

| What breaks | How |
|---|---|
| Layer 3 (Installed) | etcd installed-state record still points to the old `build_id` and old `entrypoint_checksum`. The node-agent doesn't know the binary changed. |
| Layer 4 (Runtime) | Verifier computes `sha256(/usr/lib/globular/bin/<binary>)` and compares against the repository manifest's `entrypoint_checksum`. The local `go build` binary has a different checksum → `package.installed_binary_hash_mismatch` fires immediately. |
| Other nodes | The binary lives on one node only. Every other node keeps the old version. The cluster is now split. The reconciler will eventually overwrite your change anyway. |
| leader election | If the binary is a control-plane service, checksum drift can break the leader-election heartbeat that compares `/proc/<pid>/exe` hashes across peers. |

There is **no safe context** for a direct `cp` deploy. Not for a "quick test," not for a "hotfix," not for anything.

---

## The correct flow — 5 steps via MCP

### Step 1 — Authenticate

The publish step requires an authenticated token. Get one with:

```bash
globular auth login --user sa --password <sa-password>
```

Token handling is ephemeral by default (login prints token; no implicit disk cache).
If the MCP publish step fails with "authentication required", pass the token explicitly.
Only if you must use a token file for the MCP service account, do it explicitly:

```bash
TOKEN=$(globular auth login --user sa --password <sa-password> 2>&1 | grep "^Token:" | awk '{print $2}')
sudo mkdir -p /var/lib/globular/.config/globular
echo "$TOKEN" | sudo tee /var/lib/globular/.config/globular/token > /dev/null
sudo chown -R globular:globular /var/lib/globular/.config
```

---

### Step 2 — Build the binary with ldflags

`go build` without ldflags produces a binary with `Version=""` and a different SHA256 than the CI artifact. Inject the service's **current** version (from `repository_active_release` — do NOT bump it) and strip the binary (`-s -w`, required by the release channel):

```bash
go build \
  -trimpath \
  -ldflags "-X main.Version=<current-version> -s -w" \
  -o /tmp/<service>_server \
  ./<service>/<service>_server/
```

`<current-version>` is the version already recorded for this service in the BOM — the same one it is running now. You are shipping a new **build** of that version, not a new version.

Then move the binary to a location the MCP server (running as `globular`) can read:

```bash
sudo mkdir -p /var/lib/globular/packages/out/<service>-build/bin
sudo cp /tmp/<service>_server /var/lib/globular/packages/out/<service>-build/bin/<service>_server
sudo chown -R globular:globular /var/lib/globular/packages/out
```

---

### Step 3 — Build the package artifact

Use the `mcp__globular__package_build` tool (or `globular pkg build`):

```
mcp__globular__package_build(
  spec         = "/var/lib/globular/packages/out/<service>-build/<service>_service.yaml",
  root         = "/var/lib/globular/packages/out/<service>-build",
  version      = "<current-version>",   # SAME version the service already runs — do NOT bump
  build_number = <current-build + 1>,   # THIS is what you increment (1.2.269+b1 → +b2)
  publisher    = "core@globular.io",
  out          = "/var/lib/globular/packages/out"
)
```

The version stays fixed; the build number advances. If publish returns `AlreadyExists`, bump the build number again — never `--force` (a forced re-publish of the same version+build mints a new `build_id` for identical bytes and causes build_id drift across the 4 layers).

The spec YAML can be extracted from the currently installed package:

```bash
# Find the installed spec
find /var/lib/globular -name "*<service>*.yaml" -path "*/specs/*" 2>/dev/null | head -3
# Or extract from the GitHub release artifact
gh release download v<current-version> --repo globulario/services \
  --pattern "<service>_<version>_linux_amd64.tgz" --output /tmp/<service>.tgz
tar -xzf /tmp/<service>.tgz -C /tmp/<service>-extract/
```

**What `package_build` does:**

1. Validates the spec
2. Stages binary + spec + systemd unit into a tarball
3. Computes `entrypoint_checksum = sha256(bin/<exec>)` — this becomes the verifier's ground truth
4. Writes an authoritative `package.json` manifest into the tarball

---

### Step 4 — Publish to the repository

Use the `mcp__globular__package_publish` tool:

```
mcp__globular__package_publish(
  file = "/var/lib/globular/packages/out/<service>_<new-version>_linux_amd64.tgz"
)
```

This uploads the artifact to MinIO and registers it in ScyllaDB. The repository assigns a `build_id` (UUIDv7) — this becomes the convergence identity. After this step, Layer 1 (Repository) is satisfied.

---

### Step 5 — Update desired state and trigger reconciliation

Update Layer 2 (Desired) via:

```
mcp__globular__globular_cli_execute(
  command = "globular services desired set <service> <current-version> --build-number <new-build>",
  approved = true
)
```

Note the **same `<current-version>`** and the `--build-number` pointing at the build you just published. `services desired set` accepts `--build-number` precisely so you can move desired state to a new build of the same version. (`--build-number 0` = latest.) Do not raise the version here.

Then trigger the reconciler:

```
mcp__globular__globular_cli_execute(
  command = "globular services repair",
  approved = true
)
```

The reconciler dispatches a workflow per node that:
1. Downloads the artifact from MinIO
2. Verifies SHA256 and `build_id`
3. Runs the spec steps (install binary, restart unit)
4. Writes the new `installed.build_id` and `installed.entrypoint_checksum` to etcd
5. Node-agent confirms `sha256(/proc/<pid>/exe)` matches manifest

When all nodes converge, `services repair` output shows `installed` for every node. The cluster-doctor will report zero hash-mismatch findings.

---

## Verification

After triggering reconciliation, confirm convergence:

```bash
globular services repair    # check STATUS column: "installed" for the target service
```

Or via cluster-doctor:

```
mcp__globular__cluster_get_doctor_report(freshness = "fresh")
```

Expect zero `package.installed_binary_hash_mismatch` findings. If a node shows `drifted`, re-run `services repair` and check the workflow logs on that node:

```
mcp__globular__nodeagent_get_service_logs(
  unit = "globular-<service>.service",
  node_id = "<node-id>"
)
```

---

## Recovering the correct binary from a GitHub release

If you need to restore a binary that was accidentally overwritten, always use the GitHub release artifact — never a local build:

```bash
gh release download v<version> --repo globulario/services \
  --pattern "<service>_<version>_linux_amd64.tgz" \
  --output /tmp/<service>_<version>.tgz

tar -xzf /tmp/<service>_<version>.tgz -C /tmp/<service>-extract/

# Verify the checksum against the repository manifest
sha256sum /tmp/<service>-extract/bin/<service>_server
```

The SHA256 must match `entrypoint_checksum` in the repository manifest:

```
mcp__globular__repository_get_artifact_manifest(
  publisher_id = "core@globular.io",
  name = "<service>",
  version = "<version>"
)
```

Then publish the extracted binary as a new version using the 5-step flow above. Do not write it directly to the node.

---

## Quick reference

| Step | Tool | What it satisfies |
|---|---|---|
| Build binary | `go build -trimpath -ldflags "-X main.Version=<current-v> -s -w"` | Correct checksum, stripped |
| Build artifact | `mcp__globular__package_build(version=<current-v>, build_number=<n+1>)` | Immutable `.tgz`; **build bumped, version fixed** |
| Publish | `mcp__globular__package_publish` | Layer 1 — Repository |
| Set desired | `globular services desired set <svc> <current-v> --build-number <n+1>` | Layer 2 — Desired |
| Reconcile | `globular services repair` | Layer 3 + 4 — Installed + Runtime |
| Verify | `mcp__globular__cluster_get_doctor_report` | All 4 layers agree |
