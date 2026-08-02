#!/usr/bin/env python3
"""Harden and execute the one-shot PR #221 review fixer."""

from __future__ import annotations

import subprocess
from pathlib import Path

root = Path(__file__).resolve().parents[1]
script = root / "scripts/pr221-review-fixes.py"
text = script.read_text(encoding="utf-8")
old = '''    for message in ("RepairIndexRequest", "RepairIndexItem", "RepairIndexResponse"):
        text = regex_once(
            text,
            rf'''\\nmessage {message} \\{{.*?\\n\\}}\\n''',
            "\\n",
            f"remove unwired {message}",
        )
'''
new = '''    for message in ("RepairIndexRequest", "RepairIndexItem", "RepairIndexResponse"):
        marker = f"message {message} {{"
        start = text.find(marker)
        if start < 0:
            # Some branches never carried the message declarations; the
            # contract is already absent and therefore aligned.
            continue
        line_start = text.rfind("\\n", 0, start) + 1
        depth = 0
        end = None
        for index in range(start, len(text)):
            char = text[index]
            if char == "{":
                depth += 1
            elif char == "}":
                depth -= 1
                if depth == 0:
                    end = index + 1
                    break
        if end is None:
            raise RuntimeError(f"unterminated proto message {message}")
        while end < len(text) and text[end] in " \\t":
            end += 1
        if end < len(text) and text[end] == "\\n":
            end += 1
        text = text[:line_start] + text[end:]
'''
if text.count(old) != 1:
    raise SystemExit(f"expected one proto-removal block, found {text.count(old)}")
script.write_text(text.replace(old, new, 1), encoding="utf-8")
subprocess.run(["python3", str(script)], cwd=root, check=True)
