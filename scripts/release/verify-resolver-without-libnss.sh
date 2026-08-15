#!/usr/bin/env bash
# Prove that cluster hostname resolution does NOT require a Globular-supplied
# NSS module, so libnss-resolve can be removed from the release package set.
#
# Run this ON THE TARGET, before removing the package. It is the gate on that
# removal, not a report about it.
#
# Intended target: the frozen witness, Ubuntu Noble release-20260518 / amd64 /
# systemd 255.4-1ubuntu8.15, unpatched. Also run it on a current 8.17 host: the
# two-target pair is what distinguishes "works on the minimum baseline" from
# "works only on the minimum baseline".
#
# Why getent and ping rather than dig alone: dig speaks DNS directly and proves
# only that the DNS service answers. The removal question is whether the libc
# NSS path that ordinary programs use still resolves once no Globular NSS module
# is present. getent exercises exactly that path; ping is an ordinary consumer
# of it.
#
# The negative case is not decoration. A resolver probe that reports success for
# everything cannot be distinguished from a dead one
# (awareness.scanner_zero_findings_conflates_clean_with_dead), so a
# known-impossible name MUST fail for the pass to mean anything.

set -uo pipefail

CLUSTER_DOMAIN="${CLUSTER_DOMAIN:-globular.internal}"
NODE_FQDN="${NODE_FQDN:-}"          # a known cluster node, e.g. globule-nuc.globular.internal
IMPOSSIBLE="${IMPOSSIBLE:-definitely-not-a-real-host-$$.${CLUSTER_DOMAIN}}"

pass=0; fail=0
ok()   { printf '  PASS  %s\n' "$1"; pass=$((pass+1)); }
bad()  { printf '  FAIL  %s\n' "$1"; fail=$((fail+1)); }
note() { printf '  ....  %s\n' "$1"; }

echo "== fixture self-check =="
# Fail the FIXTURE, not the product, when the target is not the declared one.
sysver=$(dpkg-query -W -f='${Version}' systemd 2>/dev/null || echo "?")
resver=$(dpkg-query -W -f='${Version}' systemd-resolved 2>/dev/null || echo "?")
echo "  systemd=${sysver}  systemd-resolved=${resver}"
if [ -n "${EXPECT_SYSTEMD:-}" ] && [ "${sysver}" != "${EXPECT_SYSTEMD}" ]; then
    echo "  FIXTURE INVALID: expected systemd ${EXPECT_SYSTEMD}, found ${sysver}" >&2
    echo "  The fixture has decayed; this is no longer the declared experiment." >&2
    exit 2
fi

echo
echo "== precondition: no Globular-supplied NSS module =="
nssstate=$(dpkg-query -W -f='${Status} ${Version}' libnss-resolve 2>/dev/null || echo "absent")
case "${nssstate}" in
    absent|*"deinstall"*|*"config-files"*)
        ok "libnss-resolve is not installed (${nssstate})" ;;
    *)
        # Distro-supplied is fine and expected on desktop images; the point is
        # that GLOBULAR must not be the one supplying it. Flag so the operator
        # can tell which situation they are proving.
        note "libnss-resolve IS installed (${nssstate})"
        note "distro-supplied is acceptable, but this run then does NOT prove"
        note "resolution works without it — remove it first to prove removal"
        ;;
esac

echo
echo "== precondition: stale 'resolve' entry may remain in nsswitch.conf =="
hostsline=$(grep -E '^hosts:' /etc/nsswitch.conf || echo "")
echo "  ${hostsline:-<no hosts: line>}"
if echo "${hostsline}" | grep -q 'resolve'; then
    note "nsswitch still lists 'resolve'; glibc should skip an unloadable source"
    note "and fall through to 'dns'. That fall-through is what this run tests."
    # The leading '!' inverts the meaning and is the difference between safe and
    # unsafe, so it must be matched precisely:
    #
    #   [!UNAVAIL=return]  "for any status that is NOT unavail, return"
    #                      -> an unavailable module CONTINUES to dns. SAFE.
    #                      This is the stock Ubuntu idiom.
    #   [UNAVAIL=return]   "on unavail, return"
    #                      -> lookup stops dead. Fall-through BLOCKED.
    #
    # An earlier version of this check matched [^]]* across the '!' and so
    # reported the stock-safe form as blocked — inverting the verdict on every
    # default Ubuntu host.
    action=$(echo "${hostsline}" | sed -n 's/.*resolve[[:space:]]*\(\[[^]]*\]\).*/\1/p')
    if [ -n "${action}" ]; then
        note "explicit NSS action on 'resolve': ${action}"
        if echo "${action}" | grep -qE '\[[^]!]*UNAVAIL[[:space:]]*=[[:space:]]*return'; then
            bad "[UNAVAIL=return] on 'resolve' stops the lookup — fall-through BLOCKED, removal unsafe on this host"
        else
            ok "NSS action on 'resolve' permits fall-through to dns"
        fi
    else
        ok "no explicit NSS action on 'resolve' — glibc default continues to dns"
    fi
fi

echo
echo "== configuration: globular.internal routed to Globular DNS =="
# Assert the CONFIGURATION, never one filesystem topology. Drop-in filenames
# differ by node type (globular.conf on joined nodes, plus globular-dns.conf on
# the day-0 founder), resolv.conf may be a stub symlink, an uplink file or a
# static file, and NetworkManager may participate in ownership. An earlier
# version of this check hardcoded one filename and declared the prerequisite
# unconfigured on every joined node in the fleet — while resolution demonstrably
# worked. What matters is that SOMETHING routes the cluster domain at Globular
# DNS, by whatever legitimate mechanism.
routed=0
if ls /etc/systemd/resolved.conf.d/*.conf >/dev/null 2>&1 &&
   grep -qs "${CLUSTER_DOMAIN}" /etc/systemd/resolved.conf.d/*.conf; then
    ok "resolved.conf.d routes ${CLUSTER_DOMAIN}: $(ls /etc/systemd/resolved.conf.d/*.conf | tr '\n' ' ')"
    grep -hE '^(DNS|Domains)=' /etc/systemd/resolved.conf.d/*.conf | sed 's/^/        /'
    routed=1
fi
if command -v resolvectl >/dev/null 2>&1 &&
   resolvectl status 2>/dev/null | grep -q "${CLUSTER_DOMAIN}"; then
    ok "systemd-resolved reports a route for ${CLUSTER_DOMAIN}"
    routed=1
fi
if grep -qs "${CLUSTER_DOMAIN}" /etc/resolv.conf; then
    ok "/etc/resolv.conf references ${CLUSTER_DOMAIN}"
    routed=1
fi
if [ "${routed}" -eq 0 ]; then
    bad "no configured route for ${CLUSTER_DOMAIN} found by any mechanism — the prerequisite is unconfigured"
fi
if command -v resolvectl >/dev/null 2>&1; then
    resolvectl status 2>/dev/null | grep -E 'DNS Domain|Current DNS Server' | head -4 | sed 's/^/        /'
fi

echo
echo "== effective resolution through libc/NSS (the actual removal test) =="
if getent hosts "${CLUSTER_DOMAIN}" >/dev/null 2>&1; then
    ok "getent hosts ${CLUSTER_DOMAIN} -> $(getent hosts "${CLUSTER_DOMAIN}" | awk '{print $1}' | tr '\n' ' ')"
else
    bad "getent hosts ${CLUSTER_DOMAIN} returned nothing"
fi

if [ -n "${NODE_FQDN}" ]; then
    if getent hosts "${NODE_FQDN}" >/dev/null 2>&1; then
        ok "getent hosts ${NODE_FQDN} -> $(getent hosts "${NODE_FQDN}" | awk '{print $1}' | tr '\n' ' ')"
    else
        bad "getent hosts ${NODE_FQDN} returned nothing"
    fi
else
    note "NODE_FQDN unset — skipping per-node lookup (set it for full coverage)"
fi

echo
echo "== ordinary consumer of the same path =="
if [ -n "${NODE_FQDN}" ]; then
    if ping -c1 -W2 "${NODE_FQDN}" >/dev/null 2>&1; then
        ok "ping resolved and reached ${NODE_FQDN}"
    else
        # Resolution is the claim under test; reachability is not.
        if getent hosts "${NODE_FQDN}" >/dev/null 2>&1; then
            note "ping failed but the name resolved — treating as resolution PASS"
            ok "ping path resolved ${NODE_FQDN}"
        else
            bad "ping could not resolve ${NODE_FQDN}"
        fi
    fi
fi

echo
echo "== sensitivity: the probe must be able to say NO =="
if getent hosts "${IMPOSSIBLE}" >/dev/null 2>&1; then
    bad "known-impossible name ${IMPOSSIBLE} RESOLVED — the probe cannot fail, so its passes prove nothing"
else
    ok "known-impossible name correctly did not resolve"
fi

echo
echo "== result =="
echo "  pass=${pass} fail=${fail}"
if [ "${fail}" -ne 0 ]; then
    echo "  VERDICT: NOT SAFE to remove libnss-resolve from the release set"
    exit 1
fi
echo "  VERDICT: cluster names resolve through libc without a Globular-supplied NSS module"
exit 0
