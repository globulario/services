# Awareness Daily Workflow

This is the default workflow for Claude/Codex on Globular changes.

## 1) Run preflight before edits

Always run awareness preflight before changing code, especially for reconcile, package install, restart behavior, desired/installed/runtime drift, dependency cycles, or repeated bugs.

Preferred (MCP tool):
- `awareness.preflight`

CLI fallback:

```bash
globular awareness preflight --task "<task>" --format agent
```

Add files when known:

```bash
globular awareness preflight --task "<task>" --file <path> --format agent
```

## 2) Interpret `UNKNOWN_IMPACT`

`UNKNOWN_IMPACT` means awareness did not find enough matched facts. It is not proof the change is safe.

Required response:
1. Expand file scope (`--file` for touched files).
2. Re-run preflight.
3. Run targeted searches/tests before editing high-risk logic.

## 3) Handle forbidden fixes

If preflight returns `forbidden_fixes`, do not implement those patterns.

Required:
1. Keep the fix at the correct architecture layer.
2. Run required tests listed by preflight.
3. State forbidden fixes avoided in your final response footer.

## 4) Run did-we-fix before repeating old work

Use:
- MCP: `awareness.did_we_fix`
- CLI via preflight output (`did_we_fix` section)

If status is `PARTIAL` or `REGRESSED`, continue from remaining gaps and required tests instead of creating a parallel fix.

## 5) Use runtime evidence when architecture-sensitive

If classification includes `ARCHITECTURE_SENSITIVE` or `CONVERGENCE_RISK`:
1. collect runtime evidence (`awareness.runtime_snapshot` or preflight with runtime include)
2. reconcile Repository -> Desired -> Installed -> Runtime before patching

## 6) Create learning proposal after a new bug

When a new architectural failure is confirmed:
1. produce incident evidence
2. create proposal (`propose-from-incident`)
3. validate proposal (`validate-proposal`)
4. approve proposal (`approve-proposal`)
5. promote proposal via CLI only (`promote-proposal`)

Promotion is intentionally not exposed in MCP.

## 7) Required final response footer

Every code-changing response must end with:

```text
Awareness used:
- preflight: yes/no
- command/tool run:
- invariants touched:
- forbidden fixes avoided:
- did-we-fix status:
- required tests:
- tests run:
- audit result:
```
