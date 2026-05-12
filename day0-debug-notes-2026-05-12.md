# Day-0 debug notes (2026-05-12)

## Scope
- Debug installer scripts locally first (`/home/dave/globular-1.2.40-linux-amd64`), no cluster-fix actions.

## Observed failures
1. `repo register-upstream` fallback writes invalid enum type (`GIT_INDEXHUB_RELEASE`) causing `repo sync` unmarshal failure.
2. `awareness install` fails with `AWARENESS_BUNDLE_STALE` when script passes short build id (`2f491b35`) while manifest has UUID build id (`2f491b35-57b1-484a-9d32-d30fea0a159b`).

## Reproduction attempts
- Reproduced enum mapping output from current fallback expression:
  - `github -> GIT_INDEXHUB_RELEASE` (invalid)
- Reproduced awareness install behavior using local bundle with `--bundle-root /tmp/*`:
  - with short build-id: fails stale
  - with manifest UUID build-id: succeeds (`AWARENESS_READY`)

## Dead ends (do not retry)
- Retrying `awareness install` with `--build-id 2f491b35` (always stale in this bundle)
- Keeping current chained string replacement for enum mapping

## Candidate local patch validated in /tmp/day0-debug
- `ensure-bootstrap-artifacts.sh`: replaced chained string replacements with explicit provider->enum map.
  - Expected `github -> GITHUB_RELEASE` now deterministic.
- `install-day0.sh`: build-id now prefers manifest `build_id` (UUID), fallback to filename slug only if manifest missing.
  - Local awareness install test passed with `AWARENESS_READY` using manifest UUID.

## Next suggested application order
1. Apply same two changes to `/home/dave/globular-1.2.40-linux-amd64/scripts/*` for local Day-0 rerun tests.
2. After confirmed, port changes into repo scripts under `services/scripts`.
