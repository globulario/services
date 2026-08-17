#!/usr/bin/env bash
# Release gate: require proof that the host-level resolver witness was run
# against the DECLARED baseline and passed.
#
# WHY A RECEIPT AND NOT A DIRECT RUN
#
# verify-resolver-without-libnss.sh needs a real host with systemd-resolved and
# an NSS stack. A release build cannot execute it, and a containerized cluster
# cannot substitute for it — containers share the host kernel and run no
# independent systemd-resolved, so the layer this witness covers is invisible
# there. That leaves three options:
#
#   1. skip it in the build        -> a safety check that fails open the moment
#                                     nobody remembers to run it, which is the
#                                     exact defect class this gate exists for
#   2. block every build on a VM   -> a gate so heavy it gets routed around, and
#                                     a routed-around gate produces a false
#                                     record of compliance (see 2026-08-10: the
#                                     bypass IS what caused 2026-08-14)
#   3. verify a receipt            -> run it deliberately, record what it proved,
#                                     and let the build check the binding
#
# This is option 3.
#
# WHAT IT BINDS
#
# The receipt records the baseline IDENTITY, not merely a timestamp, so evidence
# earned against a different target cannot satisfy this gate. If the declared
# baseline moves, every prior receipt stops counting — which is correct: they
# proved something about a platform that is no longer the target.
#
# Usage:
#   check-resolver-witness.sh <baseline.json> [receipt.json]
#
# Produce a receipt with:
#   RECEIPT_OUT=<path> EXPECT_SYSTEMD=<ver> NODE_FQDN=<fqdn> \
#     scripts/release/verify-resolver-without-libnss.sh
#
# Exit: 0 receipt valid, 1 missing/mismatched/failed, 2 usage.

set -uo pipefail

BASELINE="${1:-}"
RECEIPT="${2:-scripts/release/resolver-witness-receipt.json}"
MAX_AGE_DAYS="${RESOLVER_WITNESS_MAX_AGE_DAYS:-90}"

[ -n "${BASELINE}" ] || { echo "usage: $0 <baseline.json> [receipt.json]" >&2; exit 2; }

echo "== resolver witness receipt =="

if [ ! -r "${BASELINE}" ]; then
    echo "  ✗ declared baseline not readable: ${BASELINE}" >&2
    echo "    Cannot verify a witness against a target that is not declared." >&2
    exit 1
fi

want_systemd=$(python3 -c "import json;print(json.load(open('${BASELINE}')).get('expected_systemd',''))" 2>/dev/null)
baseline_id=$(python3 -c "import json;print(json.load(open('${BASELINE}')).get('id',''))" 2>/dev/null)
if [ -z "${want_systemd}" ]; then
    echo "  ✗ declared baseline has no expected_systemd: ${BASELINE}" >&2
    exit 1
fi
echo "  declared baseline: ${baseline_id} (systemd ${want_systemd})"

if [ ! -r "${RECEIPT}" ]; then
    echo "  ✗ RELEASE REJECTED — no resolver-witness receipt at ${RECEIPT}" >&2
    echo "" >&2
    echo "    This is NOT the same as the witness having failed, and it is not a" >&2
    echo "    reason to proceed. It means the host-level resolver behaviour of this" >&2
    echo "    release is unproven, and unproven is refused." >&2
    echo "" >&2
    echo "    Produce one on the declared baseline:" >&2
    echo "      RECEIPT_OUT=${RECEIPT} EXPECT_SYSTEMD=${want_systemd} \\" >&2
    echo "        NODE_FQDN=<a.cluster.fqdn> \\" >&2
    echo "        scripts/release/verify-resolver-without-libnss.sh" >&2
    exit 1
fi

verdict=$(python3 -c "import json;print(json.load(open('${RECEIPT}')).get('verdict',''))" 2>/dev/null)
got_systemd=$(python3 -c "import json;print(json.load(open('${RECEIPT}')).get('expected_systemd',''))" 2>/dev/null)
observed=$(python3 -c "import json;print(json.load(open('${RECEIPT}')).get('observed_systemd',''))" 2>/dev/null)
recorded=$(python3 -c "import json;print(json.load(open('${RECEIPT}')).get('recorded_at_unix',0))" 2>/dev/null)
by=$(python3 -c "import json;print(json.load(open('${RECEIPT}')).get('recorded_by',''))" 2>/dev/null)

if [ "${verdict}" != "PASS" ]; then
    echo "  ✗ RELEASE REJECTED — receipt verdict is '${verdict}', not PASS" >&2
    exit 1
fi

# The binding that matters: evidence earned on a different target does not count.
if [ "${got_systemd}" != "${want_systemd}" ] || [ "${observed}" != "${want_systemd}" ]; then
    echo "  ✗ RELEASE REJECTED — receipt was earned against a different baseline" >&2
    echo "      declared:  systemd ${want_systemd}" >&2
    echo "      receipt:   expected=${got_systemd} observed=${observed}" >&2
    echo "    A witness proves something about the platform it ran on. If the" >&2
    echo "    declared baseline moved, prior receipts stop counting." >&2
    exit 1
fi

age_days=$(( ( $(date +%s) - ${recorded:-0} ) / 86400 ))
echo "  receipt:           PASS on systemd ${observed}"
echo "  recorded:          ${age_days}d ago by ${by}"

if [ "${age_days}" -gt "${MAX_AGE_DAYS}" ]; then
    echo "  ✗ RELEASE REJECTED — receipt is ${age_days}d old (limit ${MAX_AGE_DAYS}d)" >&2
    echo "    A receipt does not age into truth. Re-run the witness, or raise the" >&2
    echo "    limit deliberately with RESOLVER_WITNESS_MAX_AGE_DAYS=<n>." >&2
    exit 1
fi

echo "  ✓ host-level resolver behaviour proven on the declared baseline"
exit 0
