# Agent Workflow

1. Name the exact move before asking for admission.
2. Preserve the requested files and operation as the envelope.
3. Run MCP `admit_change` or the CLI fallback.
4. Stop on `waiting`, `refused`, stale, or `uncertifiable`.
5. If admitted, edit only inside the envelope.
6. If the work needs a new file or broader behavior, stop and request new
   admission.
7. Verify the final diff with MCP `verify_admission` or the CLI fallback.
8. Report remaining proof plainly.

CLI verification:

```bash
sensei verify-admission --decision <decision.yaml> --bundle <dir> --repo <checkout> --output <verification.yaml> --format yaml
```

Receipt inspection:

```bash
sensei admission-status --decision <decision.yaml> --verification <verification.yaml>
```

The normal path should load only this skill. Route to `sensei-closure` only when
the admission result says architectural knowledge is incomplete.
