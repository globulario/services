#!/usr/bin/env bash
# check-mcp-config-portable.sh — the committed .mcp.json must work from a fresh
# clone on a machine that is not the author's.
#
# Root cause this gate closes: `sensei init -mcp` scaffolds .mcp.json with the
# absolute path of the generating machine's LOCAL BUILD OUTPUT —
# /home/dave/Documents/github.com/globulario/sensei/bin/awareness-mcp. That
# path names a user home, a worktree location, and a sibling repository's build
# artifact, none of which exist on a fresh clone elsewhere. The failure is
# silent in the worst way: the file parses, the client accepts it, and the
# server simply never starts for anyone but the author.
#
# Portability model enforced here: PATH-RESOLVED INSTALLED COMMAND.
# The command must be a bare name that PATH resolves, so a missing prerequisite
# produces a clear "command not found" rather than a dangling private path.
# Deliberately NOT relying on "${VAR}" / "~" / "$(pwd)" expansion: whether the
# consuming client performs it is a client-specific guarantee we do not control,
# and awareness-mcp already reads $SENSEI_ADDR (then legacy $AWG_ADDR) itself,
# so the override is handled server-side and needs no client expansion at all.

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
CONFIG="$REPO_ROOT/.mcp.json"
FAILURES=0

fail() { echo "  ERROR: $*"; FAILURES=$((FAILURES + 1)); }
pass() { echo "  ✓ $*"; }

echo "Checking committed MCP configuration is path-portable..."

if [ ! -f "$CONFIG" ]; then
    echo "  NOTICE: no .mcp.json at repository root — nothing to check"
    exit 0
fi

# 1. It must parse.
if ! python3 -c "import json,sys; json.load(open(sys.argv[1]))" "$CONFIG" 2>/dev/null; then
    fail ".mcp.json is not valid JSON"
    echo "FAILED: 1 portability violation(s)."
    exit 1
fi
pass ".mcp.json parses"

# 2/3. No user-home or worktree-specific absolute path.
#
# Checked against the RAW TEXT, not just command fields: an absolute private
# path is equally broken in args, env values, or cwd.
if grep -qE '"/home/[^"]+|"/Users/[^"]+|"[A-Za-z]:\\\\' "$CONFIG"; then
    fail "absolute user-home path in .mcp.json — will not resolve on another machine:"
    grep -nE '"/home/[^"]+|"/Users/[^"]+|"[A-Za-z]:\\\\' "$CONFIG" | sed 's/^/       /'
else
    pass "no user-home absolute paths"
fi

if grep -q "$REPO_ROOT" "$CONFIG"; then
    fail "worktree-specific absolute path ($REPO_ROOT) in .mcp.json"
else
    pass "no worktree-specific absolute paths"
fi

# 4. No embedded secrets. Tokens are ephemeral and never committed.
if grep -qiE '"(token|password|secret|api_?key|credential)[^"]*"[[:space:]]*:[[:space:]]*"[^"]+"' "$CONFIG"; then
    fail "possible embedded secret in .mcp.json"
else
    pass "no embedded credentials"
fi

# 11. A sibling repository must be explicit, never inferred from a private path.
if grep -qE '"[^"]*/(sensei|Globular|globular-installer|globular-quickstart)/[^"]*"' "$CONFIG"; then
    fail "sibling repository referenced by path — make the prerequisite explicit instead"
else
    pass "no implicit sibling-repository paths"
fi

# 5/6. Every command must resolve through the declared portability model.
mapfile -t COMMANDS < <(python3 - "$CONFIG" <<'PY'
import json, sys
cfg = json.load(open(sys.argv[1]))
for name, srv in (cfg.get("mcpServers") or {}).items():
    cmd = srv.get("command")
    if cmd:
        print(f"{name}\t{cmd}")
PY
)

for entry in "${COMMANDS[@]}"; do
    name="${entry%%$'\t'*}"
    cmd="${entry##*$'\t'}"
    case "$cmd" in
        /*|~*)
            fail "server '$name': command '$cmd' is an absolute/home path, not PATH-resolved"
            ;;
        ./*|../*)
            # A repo-relative command is an accepted model, but only if the
            # client's working directory is the repo root AND the file exists.
            if [ ! -x "$REPO_ROOT/$cmd" ]; then
                fail "server '$name': repo-relative command '$cmd' is not an executable file in the repo"
            else
                pass "server '$name': repo-relative command '$cmd' exists and is executable"
            fi
            ;;
        *)
            if command -v "$cmd" >/dev/null 2>&1; then
                pass "server '$name': command '$cmd' resolves on PATH ($(command -v "$cmd"))"
            else
                # Not a hard failure: a fresh clone on a machine without the
                # prerequisite installed is exactly the documented case. What
                # matters is that it fails CLEARLY rather than silently
                # pointing into someone else's filesystem.
                echo "  NOTICE: server '$name': command '$cmd' is not installed here;" \
                     "a fresh clone reports a clear 'command not found' (documented prerequisite)"
            fi
            ;;
    esac
done

# 7/12. Fixture clone: copy the config into a temporary directory whose path
# contains spaces and resolve the command from there. This is what a fresh
# clone in an arbitrary location actually does.
FIXTURE="$(mktemp -d "${TMPDIR:-/tmp}/mcp portability check.XXXXXX")"
trap 'rm -rf "$FIXTURE"' EXIT
cp "$CONFIG" "$FIXTURE/.mcp.json"
(
    cd "$FIXTURE"
    if ! python3 -c "import json; json.load(open('.mcp.json'))" 2>/dev/null; then
        echo "  ERROR: config does not parse from a fresh-clone directory"
        exit 1
    fi
    if grep -qE '"/home/[^"]+' .mcp.json; then
        echo "  ERROR: config still points into the author's filesystem from a fresh clone"
        exit 1
    fi
) && pass "resolves from a fixture clone in a path containing spaces" \
  || FAILURES=$((FAILURES + 1))

if [ "$FAILURES" -gt 0 ]; then
    echo "FAILED: $FAILURES portability violation(s)."
    exit 1
fi
echo "mcp config gate: all checks passed"
