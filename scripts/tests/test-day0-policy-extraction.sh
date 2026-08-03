#!/usr/bin/env bash
# Regression tests for install-day0.sh policy extraction (F7).
#
# The previous form used a FIXED --strip-components=2, which only works for one
# member shape. `policy/permissions.generated.json` has 2 components, so strip-2
# emptied the name: tar matched, wrote nothing, exited 0, and _POLICY_DEPLOYED
# incremented for a service whose policy never landed. Same silent-success class
# the script's own comment already records from an earlier incident.
set -uo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
SUT="${ROOT}/scripts/release/install-day0.sh"
PASS=0; FAIL=0
ok(){ echo "  ok   $1"; PASS=$((PASS+1)); }
bad(){ echo "  FAIL $1"; FAIL=$((FAIL+1)); }

# Load only the function under test — the script body performs a real install.
FN="$(sed -n '/^deploy_pkg_policy()/,/^}/p' "$SUT")"
[[ -n "$FN" ]] || { echo "FAIL: deploy_pkg_policy not found"; exit 1; }
eval "$FN"

WORK="$(mktemp -d)"; trap 'rm -rf "$WORK"' EXIT
GOOD='{"permissions":[{"action":"/x.Y/Z","resource":"r"}]}'

mkpkg() { # mkpkg <name> <prefix|none> [json]
  local n="$1"
  local prefix="$2"
  local json="${3-$GOOD}"
  local d="$WORK/src_$n"
  rm -rf "$d"; mkdir -p "$d/policy"
  printf '%s' "$json" > "$d/policy/permissions.generated.json"
  printf '%s' '{"roles":[]}' > "$d/policy/roles.generated.json"
  ( cd "$d" && if [[ "$prefix" == "dot" ]]; then tar czf "$WORK/$n.tgz" ./policy; else tar czf "$WORK/$n.tgz" policy; fi )
  printf '%s' "$WORK/$n.tgz"
}

# 1. bare `policy/...` member — the case the fixed strip count silently dropped
p=$(mkpkg bare none); dest="$WORK/d1"
if deploy_pkg_policy "$p" "$dest" && [[ -s "$dest/permissions.generated.json" ]]; then ok "bare policy/ member installs"; else bad "bare policy/ member installs"; fi

# 2. `./policy/...` member
p=$(mkpkg dot dot); dest="$WORK/d2"
if deploy_pkg_policy "$p" "$dest" && [[ -s "$dest/permissions.generated.json" ]]; then ok "./policy/ member installs"; else bad "./policy/ member installs"; fi

# 3. archive with neither supported member
d="$WORK/src_none"; mkdir -p "$d/other"; printf '{}' > "$d/other/x.json"
( cd "$d" && tar czf "$WORK/none.tgz" other )
dest="$WORK/d3"
if deploy_pkg_policy "$WORK/none.tgz" "$dest"; then bad "missing member must fail"; else ok "missing member fails"; fi

# 4. ambiguous: both shapes present
d="$WORK/src_amb"; rm -rf "$d"; mkdir -p "$d/policy"
printf '%s' "$GOOD" > "$d/policy/permissions.generated.json"
( cd "$d" && tar czf "$WORK/amb.tgz" policy ./policy 2>/dev/null )
dest="$WORK/d4"
if [[ "$(tar tzf "$WORK/amb.tgz" | sed 's#^\./##' | grep -cx 'policy/permissions.generated.json')" -gt 1 ]]; then
  if deploy_pkg_policy "$WORK/amb.tgz" "$dest"; then bad "ambiguous member must fail"; else ok "ambiguous member fails"; fi
else ok "ambiguous member fails (archive not ambiguous on this tar; skipped)"; fi

# 5. malformed JSON
p=$(mkpkg malformed none '{"permissions": [')
dest="$WORK/d5"
if deploy_pkg_policy "$p" "$dest"; then bad "malformed JSON must fail"; else ok "malformed JSON fails"; fi

# 6. zero-byte JSON
p=$(mkpkg empty none '')
dest="$WORK/d6"
if deploy_pkg_policy "$p" "$dest"; then bad "zero-byte JSON must fail"; else ok "zero-byte JSON fails"; fi

# 7. tar exits zero but extracts nothing (mocked)
p=$(mkpkg mock none); dest="$WORK/d7"
tar() { if [[ "${1:-}" == "xzf" ]]; then return 0; fi; command tar "$@"; }
if deploy_pkg_policy "$p" "$dest"; then bad "tar success without file must fail"; else ok "tar success without extracted file fails"; fi
unset -f tar

# 8. destination not overwritten when validation fails
dest="$WORK/d8"; mkdir -p "$dest"; printf '%s' "$GOOD" > "$dest/permissions.generated.json"
before="$(sha256sum "$dest/permissions.generated.json" | awk '{print $1}')"
p=$(mkpkg bad2 none '{"broken"')
deploy_pkg_policy "$p" "$dest" >/dev/null 2>&1
after="$(sha256sum "$dest/permissions.generated.json" | awk '{print $1}')"
[[ "$before" == "$after" ]] && ok "good destination preserved on validation failure" || bad "destination clobbered by invalid policy"

# 9/10. counter increments once on success, never on failure
_POLICY_DEPLOYED=0
p=$(mkpkg cnt none); dest="$WORK/d9"
deploy_pkg_policy "$p" "$dest" && _POLICY_DEPLOYED=$((_POLICY_DEPLOYED+1))
[[ "$_POLICY_DEPLOYED" -eq 1 ]] && ok "counter increments once on success" || bad "counter increments once on success"
p=$(mkpkg cnt2 none '{"nope"')
deploy_pkg_policy "$p" "$WORK/d10" && _POLICY_DEPLOYED=$((_POLICY_DEPLOYED+1))
[[ "$_POLICY_DEPLOYED" -eq 1 ]] && ok "counter unchanged on failure" || bad "counter unchanged on failure"

echo "  ---- passed=$PASS failed=$FAIL"
[[ $FAIL -eq 0 ]]
