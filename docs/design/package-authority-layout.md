# Package Authority Layout

See also [Artifact Authority Matrix](./artifact-authority-matrix.md) for the
cross-repo source/mirror/staging/output split.

`packages/registry.yaml` is the only authored package identity and package-index authority.

Everything else is downstream of that:

- `packages/metadata/<name>/specs/*.yaml`
  Canonical install-recipe source for one registered package.
- `globular-installer/internal/packagecatalog/specs/*.yaml`
  Embedded installer mirror generated from the registry-backed metadata specs.
- `packages/bin/`
  Binaries only.
- `packages/dist/`
  Built package artifacts only.
- `services/generated/`
  Generated outputs only; never a package identity authority. Safe to delete and
  recreate as part of a fresh build/publish flow, provided the generation steps
  are rerun first. For full release hygiene, use
  `scripts/regenerate-release-inputs.sh` or `scripts/build-release.sh --full-regenerate`
  to wipe and rebuild the release-input subtrees before release assembly.
- `services/dist/`
  Disposable release output only. Temporary release assembly state belongs under
  `services/dist/.staging/`, never as authored source.
- `services/scripts/build-release.sh`
  Recreates `services/dist/` from scratch at the start of each release build.
- `services/dist/`
  Release artifacts only.

Forbidden layouts:

- A second consumed spec forest such as `specs/specs/`
- Unregistered package specs in any consumed path
- Installer embedded specs that drift from `packages/registry.yaml`
- Artifact directories being used to infer package identity

Validation:

- `make check-package-authority`
  Fails on duplicate consumed spec roots, registry/metadata mismatches, stale
  installer mirrors, or a missing/stale installer packagecatalog manifest.

Operational rule:

- If package identity changes, edit `packages/registry.yaml` first.
- If install behavior changes, edit `packages/metadata/<name>/specs/*.yaml`.
- Never hand-edit `globular-installer/internal/packagecatalog/specs/*.yaml`.
