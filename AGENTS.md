# Globular Agent Instructions

Before editing Globular code, always run:

```
globular awareness preflight --task "<task>" --format agent
```

Preferred when available:
- MCP tool `awareness.preflight`

CLI fallback:
- `globular awareness preflight --task "<task>" --format agent`

For high-risk files, strict hook mode may block unsafe edits:

```bash
globular awareness hook --strict --watchlist docs/awareness/high_risk_files.yaml --file "<file>" --task "<task>"
```

If editing files, include:

```
--file <file>
```

If changing a package, include:

```
--package <package-path>
```

If touching recovery, bootstrap, package install, or reconcile, include:

```
--phase recovery
--phase package_install
--phase reconcile
```

You must not implement a local retry/restart fix until preflight has identified:

- impacted invariants
- known failure modes
- forbidden fixes
- did-we-fix status
- required tests

## Required output at end of every answer

Every final answer must include:

```
Awareness used:
- preflight: yes/no
- command/tool run: <globular awareness preflight ... | awareness.preflight>
- invariants touched: <list or none>
- forbidden fixes avoided: <list or none>
- did-we-fix status: <DONE|PARTIAL|REGRESSED|UNKNOWN>
- required tests: <list or none>
- tests run: <list or none>
- audit result: <PASS|FAIL|skipped>
```

## Why this matters

Globular has a 4-layer state model (Repository → Desired → Installed → Runtime) that is
non-negotiable. Local restart fixes, checksum patches, and retry workarounds at the wrong
layer cause cascading failures that are hard to diagnose.

The preflight command is the lightweight gate that surfaces the architectural context
before any code is changed. It takes seconds. Skipping it costs hours.

## Quick reference

| Flag | Purpose |
|------|---------|
| `--task` | What you intend to do (required) |
| `--file` | File(s) you plan to edit (repeatable) |
| `--package` | Package directory with awareness.yaml |
| `--phase` | recovery / bootstrap / package_install / reconcile |
| `--format` | markdown (default) / json / agent |

JSON output (`--format json`) is MCP-compatible and can be passed directly to an
orchestrating agent for tool-based workflows.
