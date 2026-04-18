#!/usr/bin/env bash
# check_proto_authz.sh — verify every rpc in every service proto has a
# (globular.auth.authz) annotation.
#
# Exits 0 if all RPCs are annotated.
# Exits 1 and prints filename + rpc name for each violation.
#
# Allowlist: protos that intentionally have no authz because the service
# is unimplemented, is a pure reflection API, or is the auth system itself.
# Add entries here only with an explicit reason.
set -euo pipefail

PROTO_DIR="$(cd "$(dirname "$0")/../proto" && pwd)"

# Protos excluded from the check and the reason why.
declare -A ALLOWLIST
ALLOWLIST["compute.proto"]="not built — Phase 2+ feature, no enforcement surface"
ALLOWLIST["compute_runner.proto"]="not built — Phase 2+ feature, no enforcement surface"
ALLOWLIST["reflection.proto"]="standard gRPC reflection, public by design"
ALLOWLIST["globular_auth.proto"]="internal auth primitives, enforced at the interceptor layer not proto annotations"

failures=0

for proto in "$PROTO_DIR"/*.proto; do
    base="$(basename "$proto")"

    if [[ -n "${ALLOWLIST[$base]+set}" ]]; then
        echo "  skip  $base  (${ALLOWLIST[$base]})"
        continue
    fi

    # Check if the file defines any service (has an rpc keyword).
    if ! grep -q '^\s*rpc ' "$proto"; then
        continue  # No RPCs — nothing to check.
    fi

    # Walk the file line by line, tracking the current rpc block.
    # An rpc block starts at "rpc MethodName(" and ends at the closing "}"
    # or at the next "rpc" keyword. Within the block we look for authz.
    in_rpc=0
    rpc_name=""
    has_authz=0
    brace_depth=0

    while IFS= read -r line; do
        # Detect start of rpc block.
        if echo "$line" | grep -qE '^\s*rpc\s+\w+\s*\('; then
            # If we were already in an rpc block, close it out.
            if [[ $in_rpc -eq 1 && $has_authz -eq 0 ]]; then
                echo "  FAIL  $base  rpc $rpc_name  missing (globular.auth.authz)"
                ((failures++)) || true
            fi
            rpc_name="$(echo "$line" | sed -E 's/.*rpc\s+(\w+)\s*\(.*/\1/')"
            in_rpc=1
            has_authz=0
            brace_depth=0
        fi

        if [[ $in_rpc -eq 1 ]]; then
            # Count braces to know when the rpc block ends.
            opens=$(echo "$line"  | tr -cd '{' | wc -c)
            closes=$(echo "$line" | tr -cd '}' | wc -c)
            brace_depth=$(( brace_depth + opens - closes ))

            if echo "$line" | grep -q 'globular\.auth\.authz'; then
                has_authz=1
            fi

            # rpc block ends when brace_depth returns to 0 after opening.
            if [[ $brace_depth -le 0 && $opens -gt 0 ]]; then
                if [[ $has_authz -eq 0 ]]; then
                    echo "  FAIL  $base  rpc $rpc_name  missing (globular.auth.authz)"
                    ((failures++)) || true
                fi
                in_rpc=0
            fi

            # Short-form rpc (no body braces): ends with semicolon.
            if echo "$line" | grep -qE '^\s*rpc\s+.*\)\s*returns\s*.*\)\s*;'; then
                if [[ $has_authz -eq 0 ]]; then
                    echo "  FAIL  $base  rpc $rpc_name  missing (globular.auth.authz)"
                    ((failures++)) || true
                fi
                in_rpc=0
            fi
        fi
    done < "$proto"

    # Handle last rpc block if file ends while still in one.
    if [[ $in_rpc -eq 1 && $has_authz -eq 0 ]]; then
        echo "  FAIL  $base  rpc $rpc_name  missing (globular.auth.authz)"
        ((failures++)) || true
    fi
done

if [[ $failures -gt 0 ]]; then
    echo ""
    echo "FAIL: $failures RPC(s) missing (globular.auth.authz) annotations."
    echo "Add annotations or extend the allowlist with an explicit reason."
    exit 1
fi

echo "OK: all RPCs have (globular.auth.authz) annotations"
