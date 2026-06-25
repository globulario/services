# break-glass.sh — shared guard for break-glass recovery scripts.
#
# Some recovery scripts mutate owner-owned cluster state DIRECTLY — deleting etcd
# keys under /globular/resources, /globular/plans, /globular/nodes, or editing the
# controller's state.json — while the cluster-controller (the owner) is stopped.
# This bypasses the owner path on purpose: it is the break-glass recovery route
# for stuck states (ghost nodes, stale plans) that the typed API cannot resolve
# because the controller is down or the state is corrupt. The controller
# re-derives state from etcd when it restarts (post-reconciled).
#
# These scripts are SANCTIONED break-glass, NOT normal operations. This guard
# makes that explicit and gated. Source it at the top of such a script, right
# after `set -euo pipefail`, and call break_glass_guard before any mutation:
#
#   source "$(dirname "${BASH_SOURCE[0]}")/lib/break-glass.sh"
#   break_glass_guard "reset-all-plans" "deletes ALL plan + release keys, then restarts the controller"
#
# Confirmation: set BREAK_GLASS_CONFIRM=1 in the environment for automation, or
# answer "yes" at the interactive prompt. On a non-TTY without the env var the
# guard refuses (so a stray pipe can never silently wipe cluster state).

# break_glass_guard <name> [action-description]
break_glass_guard() {
	local name="${1:?break_glass_guard: script name required}"
	local action="${2:-mutates owner-owned cluster state directly}"

	{
		echo "############################################################"
		echo "##  ⚠  BREAK-GLASS: ${name}"
		echo "##"
		echo "##  This bypasses the live cluster-controller (the owner)"
		echo "##  and ${action}."
		echo "##"
		echo "##  Owner-owned state is mutated DIRECTLY; the controller"
		echo "##  re-derives state from etcd on restart (post-reconciled)."
		echo "##  Use ONLY for recovery from a stuck cluster."
		echo "############################################################"
	} >&2

	if [ "${BREAK_GLASS_CONFIRM:-}" = "1" ]; then
		echo "BREAK_GLASS_CONFIRM=1 set — proceeding non-interactively." >&2
	elif [ -t 0 ]; then
		printf 'Type "yes" to proceed with break-glass %s: ' "${name}" >&2
		local reply
		read -r reply
		if [ "${reply}" != "yes" ]; then
			echo "Aborted." >&2
			exit 1
		fi
	else
		echo "Refusing: stdin is not a TTY and BREAK_GLASS_CONFIRM is not set." >&2
		echo "Re-run with BREAK_GLASS_CONFIRM=1 to confirm in automation." >&2
		exit 1
	fi

	# Durable audit record of the invocation.
	local who stamp msg
	who="$(id -un 2>/dev/null || echo unknown)"
	stamp="$(date -u +%Y-%m-%dT%H:%M:%SZ 2>/dev/null || echo unknown)"
	msg="break-glass invoked: script=${name} user=${who} at=${stamp} action=${action}"
	if command -v logger >/dev/null 2>&1; then
		logger -t globular-break-glass "${msg}" || true
	fi
	if mkdir -p /var/log/globular 2>/dev/null; then
		echo "${msg}" >>/var/log/globular/break-glass.log 2>/dev/null || true
	fi
	echo "${msg}" >&2
}
