# Convergence Workflow

Convergence is advanced one deterministic iteration at a time.

Inspect status:

```bash
sensei convergence-status --session <session.yaml> --verify-bundle <dir>
```

Advance one iteration:

```bash
sensei advance-convergence --closure-request <request.yaml> --claims <claims.yaml> --dialogue <dialogue.yaml> --evidence-state <state.yaml> --graph-nt <graph.nt> --repo <checkout> --question-created-at <RFC3339> --output-dir <dir>
```

After advancement, inspect status again before taking another closure action.
Stall, oscillation, and budget exhaustion are real outcomes and must be reported
instead of hidden behind another retry.
