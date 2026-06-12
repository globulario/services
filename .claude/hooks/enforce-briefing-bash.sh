#!/usr/bin/env bash
# PreToolUse hook for Bash.
# Closes the briefing-hook gap that enforce-briefing.sh (Edit|Write|MultiEdit
# matcher) doesn't cover: Bash commands that mutate files under high-risk
# directories via redirects, sed -i, cp/mv, tee, or inline python -c scripts.
#
# Best-effort: catches the common write patterns. The remaining bypass is
# `python3 /path/to/script.py` where the script (not the command line)
# performs the write — that requires script-content analysis, which is out
# of scope. The hook is a deterrent against accidental bypass, not a
# guarantee against intentional one.
#
# High-risk prefixes must stay in sync with enforce-briefing.sh.

set -euo pipefail

input="$(cat)"
sid="$(printf '%s' "$input" | jq -r '.session_id // "unknown"')"
cmd="$(printf '%s' "$input" | jq -r '.tool_input.command // empty')"

[ -z "$cmd" ] && exit 0

# Carve-out: git commands operate on .git/, never on the working tree
# directly (except for `git restore`, `git checkout --`, etc., which we
# don't catch here). Commit messages may legitimately mention high-risk
# paths in prose — that's not a write. Skip.
case "$cmd" in
  "git "*|"git"$'\t'*) exit 0 ;;
esac
# Same for go commands — go build / go test / go mod don't write to
# docs/awareness/ or other high-risk dirs.
case "$cmd" in
  "go "*|"go"$'\t'*) exit 0 ;;
esac

PROJECT_ROOT="/home/dave/Documents/github.com/globulario/services"

# High-risk path prefixes (relative to repo root). Mirror of enforce-briefing.sh.
HIGH_RISK_RELS=(
  "golang/node_agent/"
  "golang/cluster_controller/"
  "golang/repository/"
  "golang/rbac/"
  "golang/security/"
  "golang/cluster_doctor/"
  "golang/mcp/"
  "golang/ai_executor/"
  "golang/services_manager/"
  "docs/awareness/"
  "docs/intent/"
)
NARRATIVE_CARVEOUTS=(
  "docs/awareness/reports/"
  "docs/awareness/decisions/"
  "docs/intent/reports/"
  "docs/intent/decisions/"
)

# is_high_risk_path REL_OR_ABS_PATH → echoes the matched rel path on stdout
# (with high_risk=1) or nothing (high_risk=0).
is_high_risk_path() {
  local p="$1"
  # Strip leading project root if absolute under it.
  case "$p" in
    "$PROJECT_ROOT"/*) p="${p#$PROJECT_ROOT/}" ;;
    /*) return 0 ;;  # absolute path outside project — not our concern
  esac
  # Strip leading ./ if any.
  p="${p#./}"
  # Check narrative carve-outs first.
  for narrative in "${NARRATIVE_CARVEOUTS[@]}"; do
    case "$p" in "$narrative"*) return 0 ;; esac
  done
  for prefix in "${HIGH_RISK_RELS[@]}"; do
    case "$p" in
      "$prefix"*) printf '%s' "$p"; return 1 ;;
    esac
  done
  return 0
}

# Extract candidate write-target paths from the command string. The set of
# patterns covers shell redirects, in-place editors, copy/move targets,
# tee, heredocs, and inline python -c writes that name a path literally.
#
# Each candidate is a path that appears on the right-hand side of a write
# operation. We then check each against the high-risk set.
declare -a candidates=()

# Pattern 1: shell redirect targets — > path / >> path / 2> path / &> path
# Use grep with extended regex; emit one path per line.
while IFS= read -r p; do
  [ -n "$p" ] && candidates+=("$p")
done < <(printf '%s\n' "$cmd" | grep -oE '(>{1,2}|&>|2>)[[:space:]]*[^[:space:]|&;<>]+' | sed -E 's/^(>{1,2}|&>|2>)[[:space:]]*//' || true)

# Pattern 2: tee targets — `tee path` or `tee -a path` or `tee --append path`
while IFS= read -r p; do
  [ -n "$p" ] && candidates+=("$p")
done < <(printf '%s\n' "$cmd" | grep -oE '\btee([[:space:]]+(-a|--append))?[[:space:]]+[^[:space:]|&;<>]+' | sed -E 's/^tee([[:space:]]+(-a|--append))?[[:space:]]+//' || true)

# Pattern 3: sed -i / sed --in-place / perl -i / ruby -i / awk -i inplace
# Target is the last positional argument of the command.
# Limited to commands that begin with sed/perl/ruby/awk to keep the scan tight.
# Note: \b before "-" doesn't match because both space and "-" are non-word;
# use (^|[[:space:]]) instead to anchor on whitespace boundary.
while IFS= read -r line; do
  # Extract the last whitespace-delimited token (the path).
  last="$(printf '%s' "$line" | awk '{print $NF}')"
  [ -n "$last" ] && candidates+=("$last")
done < <(printf '%s\n' "$cmd" | grep -oE '\b(sed|perl|ruby|awk)[[:space:]]+[^;&|]*(^|[[:space:]])(-i|--in-place)([[:space:]]+inplace)?[[:space:]]+[^;&|]*' || true)
# Fallback simpler matcher for `sed -i ARGS PATH` and `perl -i ARGS PATH`:
while IFS= read -r line; do
  last="$(printf '%s' "$line" | awk '{print $NF}')"
  [ -n "$last" ] && candidates+=("$last")
done < <(printf '%s\n' "$cmd" | grep -oE '\b(sed|perl|ruby)[[:space:]]+-i([[:space:]]+[^;&|]+)+' || true)

# Pattern 4: cp / mv / install / rsync — last positional path is the target.
# Restrict to lines starting with these commands. Very heuristic, but
# covers the common idioms.
while IFS= read -r line; do
  last="$(printf '%s' "$line" | awk '{print $NF}')"
  [ -n "$last" ] && candidates+=("$last")
done < <(printf '%s\n' "$cmd" | grep -oE '\b(cp|mv|install|rsync)[[:space:]]+[^;&|]+' || true)

# Pattern 5: heredoc / inline python that mentions a high-risk path literally.
# `python -c "...open('docs/awareness/foo.yaml', 'w')..."` etc. We scan the
# raw command for any high-risk-prefixed string token regardless of context;
# if the command also contains a write verb (open with 'w', write_text,
# Path.write_text, with open ... as f), we block.
mentions_high_risk_literal() {
  local cmdstr="$1"
  for prefix in "${HIGH_RISK_RELS[@]}"; do
    if [[ "$cmdstr" == *"$prefix"* ]]; then
      # Check narrative carve-out — skip if all mentions are under narrative.
      local skip=1
      for narrative in "${NARRATIVE_CARVEOUTS[@]}"; do
        if [[ "$cmdstr" == *"$narrative"* ]]; then
          # At least one narrative mention; need to verify ALL mentions are
          # narrative. Simpler heuristic: if any non-narrative mention exists,
          # don't skip. Use grep -o to enumerate.
          while IFS= read -r mention; do
            local is_nar=0
            for n in "${NARRATIVE_CARVEOUTS[@]}"; do
              if [[ "$mention" == "$n"* ]]; then is_nar=1; break; fi
            done
            if [ "$is_nar" = "0" ]; then skip=0; break; fi
          done < <(printf '%s' "$cmdstr" | grep -oE "$prefix[A-Za-z0-9_./-]*" || true)
        fi
      done
      if [ "$skip" = "1" ]; then
        # No narrative match — definitely high-risk.
        printf '%s' "$prefix"
        return 1
      fi
    fi
  done
  return 0
}

# Write-verb patterns. The character class for the quote char around mode
# is built in a separate variable to avoid the bash-single-quote escaping
# nightmare. We want to match either:
#   open(..., 'w')   — single-quoted mode
#   open(..., "w")   — double-quoted mode
#   open(..., \"w\") — escaped (when the command arrives via JSON/shell)
#   open(..., 'wb')  — same with extra mode chars
# Plus standalone write functions in various languages.
# \\? at the start of the quote class allows an optional escape backslash.
Q='['"'"'"]'   # character class: single OR double quote
WRITE_VERB_RE="open\\([^)]*,[[:space:]]*\\\\?${Q}w|write_text|Path\\([^)]+\\)\\.write|with[[:space:]]+open|fs\\.writeFile|os\\.WriteFile|ioutil\\.WriteFile|ToFile|\\.writelines\\(|\\.dump\\(|json\\.dump|yaml\\.dump"

if printf '%s' "$cmd" | grep -qE "$WRITE_VERB_RE"; then
  if prefix="$(mentions_high_risk_literal "$cmd")"; then
    : # no high-risk mention with write verb
  else
    # mentions_high_risk_literal returned non-zero with the matched prefix
    # echoed to stdout. Recapture stdout via different path.
    prefix=""
  fi
  # Re-run capture cleanly. For each high-risk prefix that appears in
  # the command, find the first non-narrative mention that looks like a
  # specific file path (ends in .yaml/.yml/.md/.go etc., not just the
  # bare directory prefix). A bare directory mention is ambiguous and
  # commonly appears in prose / commit messages; only flag the call when
  # we have evidence of a specific file write target alongside a write
  # verb in the same command.
  hit=""
  for prefix in "${HIGH_RISK_RELS[@]}"; do
    if [[ "$cmd" == *"$prefix"* ]]; then
      while IFS= read -r mention; do
        # Skip if narrative.
        is_nar=0
        for n in "${NARRATIVE_CARVEOUTS[@]}"; do
          if [[ "$mention" == "$n"* ]]; then is_nar=1; break; fi
        done
        [ "$is_nar" = "1" ] && continue
        # Require mention to look like a specific file (has an extension).
        # Bare "docs/awareness/" doesn't qualify; "docs/awareness/foo.yaml" does.
        if [[ "$mention" =~ \.[A-Za-z0-9]+$ ]]; then
          hit="$mention"
          break 2
        fi
      done < <(printf '%s' "$cmd" | grep -oE "$prefix[A-Za-z0-9_./-]*" || true)
    fi
  done
  if [ -n "$hit" ]; then
    candidates+=("$hit")
  fi
fi

# Now check each candidate path against the high-risk set + briefing markers.
dir="/tmp/claude-awareness-briefings/$sid"

for cand in "${candidates[@]}"; do
  # Trim trailing punctuation/quotes that might attach from regex extraction.
  cand="$(printf '%s' "$cand" | sed -E 's/^["'"'"']+|["'"'"',;)]+$//g')"
  [ -z "$cand" ] && continue

  # Resolve relative paths assuming repo root.
  case "$cand" in
    /*) abs="$cand" ;;
    *)  abs="$PROJECT_ROOT/$cand" ;;
  esac

  if rel="$(is_high_risk_path "$abs")"; then
    # Path is not high-risk — continue.
    continue
  fi
  rel="$(is_high_risk_path "$abs" || true)"
  # Re-derive rel deterministically.
  case "$abs" in
    "$PROJECT_ROOT"/*) rel="${abs#$PROJECT_ROOT/}" ;;
    *) rel="$cand" ;;
  esac

  hash="$(printf '%s' "$abs" | sha256sum | awk '{print $1}')"
  if [ -e "$dir/$hash" ]; then
    continue
  fi

  cat <<EOF
{
  "hookSpecificOutput": {
    "hookEventName": "PreToolUse",
    "permissionDecision": "deny",
    "permissionDecisionReason": "CLAUDE.md hard rule #7 (Bash path): this Bash command appears to mutate \"$rel\" — a high-risk file — but mcp__awg__awareness_briefing has not been called for it in this session. Call awareness_briefing first, then retry. Hook coverage is best-effort for Bash; if the detection is a false positive (e.g. command only reads the file), call briefing once to acknowledge the path before retrying. To narrow your scope, name only the file you intend to mutate."
  }
}
EOF
  exit 0
done

exit 0
