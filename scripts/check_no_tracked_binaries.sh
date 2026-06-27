#!/usr/bin/env bash
#
# check_no_tracked_binaries.sh — fail if any git-tracked file is a compiled binary.
#
# Compiled executables, shared libraries, object files, and static archives are
# build artifacts: platform-specific, history-bloating, and never source. They
# must never be tracked in git. This is the general form of two narrower gates:
#   - the /golang/*_server .gitignore rule, and
#   - `make check-no-misplaced-pb` (generated protobuf placement),
# all born from the same scar: a stale root-level cluster_controller{,_grpc}.pb.go
# and an accidentally-committed golang/node_agent_server_new binary.
#
# Detection is by content (libmagic mime type), NOT by extension or mode bit, so
# it catches binaries regardless of name and does not flag legit binary *source*
# such as PNGs, fonts, or fixtures (those have non-executable mime types).
set -euo pipefail

if ! command -v file >/dev/null 2>&1; then
	echo "FAIL: 'file' command not found — cannot verify the tracked-binary boundary."
	echo "      Install 'file' (libmagic) so this hygiene gate can run."
	exit 1
fi

cd "$(git rev-parse --show-toplevel)"

# Allowlist: tracked binaries that are legitimately source (rare). One repo-
# relative path per alternation. Keep this matching nothing unless there is a
# justified exception (then document why here).
ALLOWLIST_RE='^$'

# Compiled-binary mime types (libmagic). None of these is ever source:
#   x-executable / x-pie-executable  ELF programs (typical Go build output)
#   x-sharedlib                      ELF shared object (.so)
#   x-mach-binary                    macOS Mach-O
#   x-dosexec                        Windows PE (.exe / .dll)
#   x-object                         relocatable object (.o)
#   x-archive                        static archive (.a)
# NOTE on whitespace: `file` pads filenames with alignment spaces when handed
# many files at once, so the separator is ":<one-or-more-spaces>", not ": ".
# Both the match and the strip below tolerate that with [[:space:]]+.
MIME_RE=':[[:space:]]+application/x-(executable|pie-executable|sharedlib|mach-binary|dosexec|object|archive)$'

offenders=$(
	git ls-files -z \
		| xargs -0 file --mime-type -- 2>/dev/null \
		| grep -aE "$MIME_RE" \
		| sed -E 's/:[[:space:]]+application\/x-[a-z-]+$//' \
		| grep -avE "$ALLOWLIST_RE" \
		|| true
)

if [ -n "$offenders" ]; then
	echo "FAIL: compiled binaries tracked in git (build artifacts — never source):"
	echo "$offenders" | sed 's/^/  /'
	echo
	echo "Untrack them:  git rm --cached <path>   then add a .gitignore rule."
	echo "If one is genuinely source, add it to ALLOWLIST_RE in $0 with a reason."
	exit 1
fi

echo "OK: no compiled binaries tracked in git"
