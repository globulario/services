#!/usr/bin/env python3
"""Refine and execute the one-shot PR #222 fixer."""

from __future__ import annotations

import subprocess
from pathlib import Path

root = Path(__file__).resolve().parents[1]
script = root / "scripts/pr222-review-fixes.py"
text = script.read_text(encoding="utf-8")
old = '''    if 'srv.resources.Apply(ctx, resourceType, rel)' in text or 'srv.resources.Apply(ctx, resourceType, obj)' in text:
        raise RuntimeError("workflow release writer still bypasses applyWorkflowRelease")
'''
new = '''    if 'srv.resources.Apply(ctx, resourceType, rel)' in text:
        raise RuntimeError("workflow release writer still bypasses applyWorkflowRelease for a typed release")
    # The one remaining generic Apply is intentionally inside
    # applyWorkflowRelease for non-ServiceRelease resources.
    if text.count('srv.resources.Apply(ctx, resourceType, obj)') != 1:
        raise RuntimeError("generic workflow release persistence escaped its choke point")
'''
if text.count(old) != 1:
    raise SystemExit(f"writer assertion block: expected 1, found {text.count(old)}")
text = text.replace(old, new, 1)
old = '''	for _, forbidden := range []string{
		"srv.resources.Apply(ctx, resourceType, rel)",
		"srv.resources.Apply(ctx, resourceType, obj)",
	} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("workflow callback bypasses canonical ServiceRelease validation: %s", forbidden)
		}
	}
'''
new = '''	if strings.Contains(text, "srv.resources.Apply(ctx, resourceType, rel)") {
		t.Fatalf("typed workflow callback bypasses canonical ServiceRelease validation")
	}
	if got := strings.Count(text, "srv.resources.Apply(ctx, resourceType, obj)"); got != 1 {
		t.Fatalf("generic workflow persistence must exist only inside applyWorkflowRelease, got %d occurrences", got)
	}
'''
if text.count(old) != 1:
    raise SystemExit(f"writer ratchet block: expected 1, found {text.count(old)}")
script.write_text(text.replace(old, new, 1), encoding="utf-8")
subprocess.run(["python3", str(script)], cwd=root, check=True)
