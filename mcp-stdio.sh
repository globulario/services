#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT="/home/dave/Documents/github.com/globulario/services"
export GLOBULAR_MCP_CONFIG="$REPO_ROOT/mcp.config.json"

exec "$REPO_ROOT/mcp"
