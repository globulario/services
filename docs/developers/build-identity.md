# Build Identity Fields

Every Globular package artifact has multiple identity fields. They serve different purposes and must not be confused.

## Fields

| Field | Type | Source | Purpose |
|-------|------|--------|---------|
| `build_number` | integer | `github.run_number` | Sequential CI counter. Monotonically increasing per repo. Used for ordering: "build 109 is newer than build 108." |
| `build_id` | UUID (v7) | Generated per release | Unique artifact identity. Time-sortable. Used by repository to index and fetch specific artifacts. Never reused. |
| `version` | semver string | Release tag or `--bump` | Package version. Platform services use the release tag (e.g., `1.0.90`). Self-versioned binaries (etcd, envoy) use their own version. |
| `artifact_sha256` | hex string | SHA256 of `.tgz` | Content hash of the package archive. Used for cache validation and integrity verification during fetch. |
| `entrypoint_checksum` | `sha256:hex` | SHA256 of binary | Content hash of the main executable inside the package. Used for process fingerprinting and drift detection. |
| `package_contract_digest` | `sha256:hex` | SHA256 of contract inputs | Hash of the package "contract" (unit files, config structure, metadata). Changes when the package shape changes, even if the binary is identical. Used for BOM change detection. |

## How They Relate

```
Release v1.0.90 (platform release)
├── build_number = 110 (CI run #110)
├── build_id = 019ddb0a-bc0a-7537-... (unique to this release)
│
├── Package: echo
│   ├── version = 1.0.90 (platform version)
│   ├── artifact_sha256 = 361cd28f... (of echo_1.0.90_linux_amd64.tgz)
│   ├── entrypoint_checksum = sha256:a1b2c3... (of bin/echo_server)
│   └── package_contract_digest = sha256:d4e5f6... (unit + config shape)
│
├── Package: etcd (self-versioned)
│   ├── version = 3.5.14 (etcd's own version, NOT 1.0.90)
│   └── ...
```

## Rules

1. **build_id is a UUID, build_number is an integer.** Never use one where the other is expected.
2. **Version comes from the release tag for platform services.** Self-versioned packages (etcd, envoy, prometheus, etc.) keep their upstream version.
3. **artifact_sha256 is for the archive, entrypoint_checksum is for the binary.** The archive includes unit files, config, and metadata — a different binary produces a different archive hash even if the contract is the same.
4. **package_contract_digest is for change detection.** If only the binary changed but the unit files and config stayed the same, the contract digest stays the same — but the archive hash changes because the binary is inside it.

## Environment Variables (CI)

| Variable | Value | Used For |
|----------|-------|----------|
| `BUILD_NUMBER` | `${{ github.run_number }}` | Stamped into package.json, release-index.json |
| `BUILD_ID` | UUIDv7 (generated) | Stamped into package.json, release-index.json |
| `VERSION` | Tag without `v` prefix | Package version for platform-versioned packages |

Do NOT use `BUILD_ID` for the sequential counter. Do NOT use `BUILD_NUMBER` for the unique identity.
