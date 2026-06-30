# Artifact Authority Matrix

A mirror may support offline install.
A staging directory may support build flow.
An output directory may support release publication.
None of those may become source authority.

| Directory | Role |
|---|---|
| `packages/registry.yaml` | package identity authority |
| `packages/metadata/<name>/specs/*.yaml` | package recipe authority |
| `packages/bin` | package binary staging |
| `packages/dist` | package artifact output |
| `services/webroot` | authored release webroot source |
| `services/generated` | disposable generated workspace |
| `services/dist` | disposable release output |
| `services/dist/.staging` | temporary release assembly staging |
| `globular-installer/scripts/install-day0.sh` | install execution authority |
| `globular-installer/internal/packagecatalog` | generated embedded package mirror |
| `globular-installer/internal/assets/webroot` | generated fallback mirror |
| `globular-installer/internal/assets/bin` | installer-private helper assets only |

Ownership split:

- `packages` defines what packages exist and how they are packaged.
- `services` owns generated service outputs, release webroot source, and release assembly/publication.
- `globular-installer` owns installation execution and embedded fallback mirrors only.

Rules:

- Never infer package identity from `packages/bin`, `packages/dist`, `services/generated`, or `services/dist`.
- Inside a built release bundle, `release-index.json` is the BOM authority for that release; the surrounding `services/dist` directory is still only disposable output.
- `services/generated` may be deleted only at a regeneration boundary.
- Development/incremental mode may reuse `services/generated`.
- Full regenerate-and-release mode must wipe and rebuild release-input subtrees under `services/generated` before assembly.
- `services/dist` may be deleted and recreated by the release build.
- `services/scripts/build-release.sh` should start by removing and recreating `services/dist`.
- `services/scripts/regenerate-release-inputs.sh` is the explicit regeneration boundary for release-input subtrees under `services/generated`.
- Mirrors must be generated from authority and checked for drift.
- Staging directories must be disposable.
- Outputs must not become inputs unless explicitly validated against authority.
