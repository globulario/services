#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT="/home/dave/Documents/github.com/globulario/services"
export GLOBULAR_MCP_CONFIG="$REPO_ROOT/mcp.config.json"

# Prefer a local build (always up to date with dev changes); fall back to
# the system-installed binary. The installed binary requires GLOBULAR_MCP_CONFIG
# support (>= v1.0.46) to honour the repo's mcp.config.json via stdio.
MCP_BIN="$REPO_ROOT/mcp"
if [[ ! -x "$MCP_BIN" ]]; then
  MCP_BIN="/usr/lib/globular/bin/mcp"
fi

exec "$MCP_BIN"
