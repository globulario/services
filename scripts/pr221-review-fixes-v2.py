#!/usr/bin/env python3
"""Harden and execute the one-shot PR #221 review fixer."""

from __future__ import annotations

import subprocess
from pathlib import Path

root = Path(__file__).resolve().parents[1]
script = root / "scripts/pr221-review-fixes.py"
text = script.read_text(encoding="utf-8")
start_marker = '    for message in ("RepairIndexRequest", "RepairIndexItem", "RepairIndexResponse"):\n'
end_marker = '    if "RepairIndex(" in text or "message RepairIndex" in text:\n'
start = text.find(start_marker)
end = text.find(end_marker, start)
if start < 0 or end < 0:
    raise SystemExit(f"proto-removal block markers not found: start={start}, end={end}")
replacement = '''    for message in ("RepairIndexRequest", "RepairIndexItem", "RepairIndexResponse"):
        marker = f"message {message} {{"
        message_start = text.find(marker)
        if message_start < 0:
            # Some branches never carried the message declarations; the
            # contract is already absent and therefore aligned.
            continue
        line_start = text.rfind("\\n", 0, message_start) + 1
        depth = 0
        message_end = None
        for index in range(message_start, len(text)):
            char = text[index]
            if char == "{":
                depth += 1
            elif char == "}":
                depth -= 1
                if depth == 0:
                    message_end = index + 1
                    break
        if message_end is None:
            raise RuntimeError(f"unterminated proto message {message}")
        while message_end < len(text) and text[message_end] in " \\t":
            message_end += 1
        if message_end < len(text) and text[message_end] == "\\n":
            message_end += 1
        text = text[:line_start] + text[message_end:]
'''
script.write_text(text[:start] + replacement + text[end:], encoding="utf-8")
subprocess.run(["python3", str(script)], cwd=root, check=True)
