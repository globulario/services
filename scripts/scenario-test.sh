#!/bin/bash
set -euo pipefail

# ── scenario-test.sh ────────────────────────────────────────────────────────
# Awareness/remediation validation suite for the Globular quickstart cluster.
#
# Runs 5 scenarios against a Docker-based test node, observing event
# propagation through the pipeline:
#   node_agent (event_publisher) → event_service → ai_watcher → ai_executor
#
# Each scenario gets a unique correlation marker (RUN_ID + scenario name)
# embedded in log timestamps, making it easy to trace events across runs.
#
# Usage:
#   ./scripts/scenario-test.sh                    # defaults: node-1, discovery
#   ./scripts/scenario-test.sh node-2 log         # custom node + service
#   SCENARIOS="crash,clean-stop" ./scripts/scenario-test.sh  # subset
# ─────────────────────────────────────────────────────────────────────────────

# ── Configuration ─────────────────────────────────────────────────────────
TARGET_NODE="${1:-node-1}"
TARGET_SERVICE="${2:-discovery}"
TARGET_CONTAINER="globular-${TARGET_NODE}"
TARGET_UNIT="globular-${TARGET_SERVICE}.service"

# Correlation ID: ties all events/logs from this run together.
RUN_ID="scentest-$(date +%s)-$$"
RUN_START=$(date -u +"%Y-%m-%d %H:%M:%S")

# Which scenarios to run (comma-separated, or "all").
SCENARIOS="${SCENARIOS:-all}"

# Timing: node_agent polls every 5s, watcher batch window is 10s.
# Override via environment: EVENT_WAIT=10 ./scripts/scenario-test.sh
EVENT_WAIT="${EVENT_WAIT:-20}"
RECOVERY_WAIT="${RECOVERY_WAIT:-30}"
COOLDOWN_WAIT="${COOLDOWN_WAIT:-5}"

# ── Colors ────────────────────────────────────────────────────────────────
RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[0;33m'
CYAN='\033[0;36m'; BOLD='\033[1m'; NC='\033[0m'

# ── Counters ──────────────────────────────────────────────────────────────
PASS=0; FAIL=0; SKIP=0
REPORT=""

# ── Helpers ───────────────────────────────────────────────────────────────
log()  { echo -e "${CYAN}[$RUN_ID]${NC} $*"; }
ok()   { echo -e "  ${GREEN}✓${NC} $*"; }
fail() { echo -e "  ${RED}✗${NC} $*"; }
warn() { echo -e "  ${YELLOW}!${NC} $*"; }

dexec() {
    docker exec "$TARGET_CONTAINER" "$@" 2>/dev/null
}

unit_state() {
    dexec systemctl show "$TARGET_UNIT" --property=ActiveState,SubState --no-pager 2>/dev/null \
        | awk -F= '{printf "%s", $2; if(NR==1) printf "/"}' || echo "unknown/unknown"
}

# Count occurrences of a pattern in a string. Returns a clean integer.
count_in() {
    local text="$1" pattern="$2" n
    if [ -z "$text" ]; then echo "0"; return; fi
    n=$(printf '%s\n' "$text" | grep -c "$pattern" 2>/dev/null) || true
    echo "${n:-0}"
}

wait_for_state() {
    local target="$1" timeout="$2"
    local deadline=$((SECONDS + timeout))
    while [ $SECONDS -lt $deadline ]; do
        local state
        state=$(unit_state)
        [[ "$state" == *"$target"* ]] && return 0
        sleep 1
    done
    return 1
}

# Collect event-publisher logs for the target unit since a given timestamp.
collect_events() {
    local since="$1"
    docker exec "$TARGET_CONTAINER" journalctl -u globular-node-agent.service \
        --since="$since" --no-pager 2>/dev/null \
        | grep "event-publisher:.*${TARGET_UNIT%%.*}" 2>/dev/null || true
}

find_watcher_node() {
    for node in globular-node-1 globular-node-2 globular-node-3; do
        if docker exec "$node" systemctl is-active globular-ai-watcher.service 2>/dev/null | grep -q "^active$"; then
            echo "$node"
            return 0
        fi
    done
    return 1
}

collect_incidents() {
    local watcher_node="$1"
    [ -z "$watcher_node" ] && return
    docker exec "$watcher_node" journalctl -u globular-ai-watcher.service \
        --since="$RUN_START" --no-pager 2>/dev/null \
        | grep -E "incident|batch.*fired|rule.*matched" 2>/dev/null || true
}

collect_remediations() {
    local watcher_node="$1"
    [ -z "$watcher_node" ] && return
    for node in globular-node-1 globular-node-2 globular-node-3; do
        docker exec "$node" journalctl -u globular-ai-executor.service \
            --since="$RUN_START" --no-pager 2>/dev/null \
            | grep -E "remediat|ProcessIncident|action.*executed" 2>/dev/null || true
    done
}

record_result() {
    local scenario="$1" status="$2" expected="$3" actual="$4"
    local incidents="$5" remediations="$6" final_state="$7" notes="$8"
    REPORT+="$(printf '\n%-22s %-6s %-18s %-18s %-10s %-12s %-18s %s' \
        "$scenario" "$status" "$expected" "$actual" \
        "$incidents" "$remediations" "$final_state" "$notes")"
    case "$status" in
        PASS) PASS=$((PASS + 1)) ;; FAIL) FAIL=$((FAIL + 1)) ;; SKIP) SKIP=$((SKIP + 1)) ;;
    esac
}

should_run() {
    [[ "$SCENARIOS" == "all" ]] && return 0
    echo ",$SCENARIOS," | grep -q ",$1," && return 0
    return 1
}

# ── Pre-flight checks ────────────────────────────────────────────────────
echo ""
echo -e "${BOLD}═══════════════════════════════════════════════════════════════${NC}"
echo -e "${BOLD}  Awareness / Remediation Scenario Test Suite${NC}"
echo -e "${BOLD}═══════════════════════════════════════════════════════════════${NC}"
echo -e "  Run ID:     ${CYAN}${RUN_ID}${NC}"
echo -e "  Target:     ${TARGET_CONTAINER} / ${TARGET_UNIT}"
echo -e "  Started:    ${RUN_START}"
echo ""

if ! docker inspect "$TARGET_CONTAINER" --format '{{.State.Running}}' 2>/dev/null | grep -q true; then
    echo -e "${RED}FATAL: container $TARGET_CONTAINER is not running${NC}"
    exit 1
fi

if ! dexec systemctl cat "$TARGET_UNIT" >/dev/null 2>&1; then
    echo -e "${RED}FATAL: unit $TARGET_UNIT not found in $TARGET_CONTAINER${NC}"
    exit 1
fi

EVENT_NODE=""
for node in globular-node-1 globular-node-2 globular-node-3; do
    if docker exec "$node" systemctl is-active globular-event.service 2>/dev/null | grep -q "^active$"; then
        EVENT_NODE="$node"; break
    fi
done
[ -n "$EVENT_NODE" ] && ok "Event service: $EVENT_NODE" || warn "Event service not running"

WATCHER_NODE=""
if WATCHER_NODE=$(find_watcher_node); then
    ok "AI watcher: $WATCHER_NODE"
else
    warn "AI watcher not running — incident detection untested"
    WATCHER_NODE=""
fi

dexec systemctl is-active globular-node-agent.service 2>/dev/null | grep -q "^active$" \
    && ok "Node agent: $TARGET_CONTAINER" \
    || warn "Node agent not running — events won't publish"

# Ensure target service is running and observed by node-agent.
INITIAL_STATE=$(unit_state)
log "Initial state: $INITIAL_STATE"
if [[ "$INITIAL_STATE" != *"running"* ]]; then
    log "Starting $TARGET_UNIT..."
    dexec systemctl start "$TARGET_UNIT" || true
    wait_for_state "running" 15 || { echo -e "${RED}FATAL: could not start $TARGET_UNIT${NC}"; exit 1; }
fi
log "Stabilizing 12s (node-agent polls every 5s)..."
sleep 12

echo ""
echo -e "${BOLD}── Scenarios ───────────────────────────────────────────────────${NC}"

# ══════════════════════════════════════════════════════════════════════════
# SCENARIO 1: Clean Stop
# ══════════════════════════════════════════════════════════════════════════
if should_run "clean-stop"; then
    echo ""
    log "${BOLD}[1/5] CLEAN STOP${NC}"
    SCENARIO_START=$(date -u +"%Y-%m-%d %H:%M:%S")
    log "Stopping $TARGET_UNIT cleanly..."
    dexec systemctl stop "$TARGET_UNIT"

    state=$(unit_state)
    [[ "$state" == *"dead"* ]] || [[ "$state" == *"inactive"* ]] && ok "Stopped: $state" || fail "Not stopped: $state"

    log "Waiting ${EVENT_WAIT}s for events..."
    sleep "$EVENT_WAIT"

    events=$(collect_events "$SCENARIO_START")
    event_count=$(count_in "$events" "service\.")
    stopped_count=$(count_in "$events" "service\.stopped")
    state_changed=$(count_in "$events" "service\.state_changed")
    exited_count=$(count_in "$events" "service\.exited")

    incident_count=0; remediation_count=0
    if [ -n "$WATCHER_NODE" ]; then
        incident_count=$(count_in "$(collect_incidents "$WATCHER_NODE")" "incident|batch.*fired")
        remediation_count=$(count_in "$(collect_remediations "$WATCHER_NODE")" "remediat|action.*executed")
    fi

    # Clean stop must NOT produce service.exited or incidents.
    # service.stopped OR service.state_changed are both acceptable
    # (agent may catch transient deactivating state).
    status="PASS"; notes=""
    [ "$exited_count" -gt 0 ] && { status="FAIL"; notes="false service.exited"; fail "False crash event on clean stop"; }
    [ "$incident_count" -gt 0 ] && { status="FAIL"; notes="${notes:+$notes; }false incident"; fail "False incident on clean stop"; }
    if [ "$stopped_count" -gt 0 ] || [ "$state_changed" -gt 0 ]; then
        ok "Event published (stopped=$stopped_count, state_changed=$state_changed)"
    elif [ "$event_count" -eq 0 ]; then
        notes="${notes:+$notes; }no events observed"
        warn "No events — timing or node-agent not connected"
    fi

    record_result "clean-stop" "$status" "stopped|changed" \
        "stop=$stopped_count,chg=$state_changed,exit=$exited_count" \
        "$incident_count" "$remediation_count" "$(unit_state)" "$notes"

    dexec systemctl start "$TARGET_UNIT" || true
    wait_for_state "running" 15 || true
    sleep 10
fi

# ══════════════════════════════════════════════════════════════════════════
# SCENARIO 2: Crash (SIGKILL)
# ══════════════════════════════════════════════════════════════════════════
if should_run "crash"; then
    echo ""
    log "${BOLD}[2/5] CRASH (SIGKILL)${NC}"
    pid=$(dexec systemctl show "$TARGET_UNIT" --property=MainPID --value 2>/dev/null || echo "0")

    if [ "$pid" = "0" ] || [ -z "$pid" ]; then
        warn "No PID — skipping"
        record_result "crash" "SKIP" "service.exited" "n/a" "n/a" "n/a" "$(unit_state)" "no PID"
    else
        SCENARIO_START=$(date -u +"%Y-%m-%d %H:%M:%S")
        log "Killing PID $pid..."
        dexec kill -9 "$pid" || true
        sleep 2
        log "State after kill: $(unit_state)"

        log "Waiting ${EVENT_WAIT}s for events..."
        sleep "$EVENT_WAIT"

        events=$(collect_events "$SCENARIO_START")
        exited_count=$(count_in "$events" "service\.exited")
        started_count=$(count_in "$events" "service\.started")
        state_changed=$(count_in "$events" "service\.state_changed")

        incident_count=0; remediation_count=0
        if [ -n "$WATCHER_NODE" ]; then
            incident_count=$(count_in "$(collect_incidents "$WATCHER_NODE")" "incident|batch.*fired")
            remediation_count=$(count_in "$(collect_remediations "$WATCHER_NODE")" "remediat|action.*executed")
        fi

        log "Waiting ${RECOVERY_WAIT}s for recovery..."
        wait_for_state "running" "$RECOVERY_WAIT" || true
        final_state=$(unit_state)

        status="PASS"; notes=""
        if [ "$exited_count" -gt 0 ] || [ "$state_changed" -gt 0 ]; then
            ok "Crash detected (exited=$exited_count, state_changed=$state_changed)"
        else
            status="FAIL"; notes="no crash event detected"
            fail "Missing crash event after SIGKILL"
        fi
        [[ "$final_state" == *"running"* ]] && ok "Recovered: $final_state" \
            || { notes="${notes:+$notes; }not recovered ($final_state)"; warn "Not recovered: $final_state"; }

        record_result "crash" "$status" "service.exited" \
            "exit=$exited_count,chg=$state_changed,start=$started_count" \
            "$incident_count" "$remediation_count" "$final_state" "$notes"
    fi

    dexec systemctl start "$TARGET_UNIT" 2>/dev/null || true
    wait_for_state "running" 15 || true
    sleep 10
fi

# ══════════════════════════════════════════════════════════════════════════
# SCENARIO 3: Controller Restart
# ══════════════════════════════════════════════════════════════════════════
if should_run "controller-restart"; then
    echo ""
    log "${BOLD}[3/5] CONTROLLER RESTART${NC}"

    CTRL_NODE=""
    for node in globular-node-1 globular-node-2 globular-node-3; do
        if docker exec "$node" systemctl is-active globular-cluster-controller.service 2>/dev/null | grep -q "^active$"; then
            CTRL_NODE="$node"; break
        fi
    done

    if [ -z "$CTRL_NODE" ]; then
        warn "No controller — skipping"
        record_result "controller-restart" "SKIP" "ctrl:stopped+started" "n/a" "n/a" "n/a" "n/a" "no controller"
    else
        SCENARIO_START=$(date -u +"%Y-%m-%d %H:%M:%S")
        log "Restarting controller on $CTRL_NODE..."
        docker exec "$CTRL_NODE" systemctl restart globular-cluster-controller.service 2>/dev/null || true

        log "Waiting ${EVENT_WAIT}s for events..."
        sleep "$EVENT_WAIT"

        ctrl_state=$(docker exec "$CTRL_NODE" systemctl show globular-cluster-controller.service \
            --property=ActiveState,SubState --no-pager 2>/dev/null \
            | awk -F= '{printf "%s", $2; if(NR==1) printf "/"}')

        target_events=$(collect_events "$SCENARIO_START")
        false_exited=$(count_in "$target_events" "service\.exited")

        ctrl_events=$(docker exec "$CTRL_NODE" journalctl -u globular-node-agent.service \
            --since="$SCENARIO_START" --no-pager 2>/dev/null \
            | grep "event-publisher:.*cluster-controller" 2>/dev/null || true)
        ctrl_stopped=$(count_in "$ctrl_events" "service\.stopped")
        ctrl_changed=$(count_in "$ctrl_events" "service\.state_changed")

        status="PASS"; notes=""
        [[ "$ctrl_state" == *"running"* ]] && ok "Controller recovered: $ctrl_state" \
            || { status="FAIL"; notes="controller not recovered ($ctrl_state)"; fail "Controller not recovered"; }
        [ "$false_exited" -gt 0 ] \
            && { status="FAIL"; notes="${notes:+$notes; }false exited on $TARGET_UNIT"; fail "False crash on $TARGET_UNIT"; } \
            || ok "No false crash events on $TARGET_UNIT"

        record_result "controller-restart" "$status" "ctrl:stopped+started" \
            "ctrl_stop=$ctrl_stopped,ctrl_chg=$ctrl_changed,tgt_exit=$false_exited" \
            "0" "0" "$ctrl_state" "$notes"
    fi
    sleep "$COOLDOWN_WAIT"
fi

# ══════════════════════════════════════════════════════════════════════════
# SCENARIO 4: Crash Loop (3 rapid kills)
# ══════════════════════════════════════════════════════════════════════════
if should_run "crash-loop"; then
    echo ""
    log "${BOLD}[4/5] CRASH LOOP (3 rapid kills)${NC}"
    SCENARIO_START=$(date -u +"%Y-%m-%d %H:%M:%S")

    for i in 1 2 3; do
        pid=$(dexec systemctl show "$TARGET_UNIT" --property=MainPID --value 2>/dev/null || echo "0")
        [ "$pid" = "0" ] || [ -z "$pid" ] && { sleep 3; pid=$(dexec systemctl show "$TARGET_UNIT" --property=MainPID --value 2>/dev/null || echo "0"); }
        if [ "$pid" != "0" ] && [ -n "$pid" ]; then
            log "  Kill $i: PID=$pid"
            dexec kill -9 "$pid" 2>/dev/null || true
        else
            log "  Kill $i: no PID"
        fi
        sleep 3
    done

    log "Waiting ${EVENT_WAIT}s for events..."
    sleep "$EVENT_WAIT"

    events=$(collect_events "$SCENARIO_START")
    exited_count=$(count_in "$events" "service\.exited")
    stopped_count=$(count_in "$events" "service\.stopped")
    started_count=$(count_in "$events" "service\.started")
    state_changed=$(count_in "$events" "service\.state_changed")

    incident_count=0
    [ -n "$WATCHER_NODE" ] && incident_count=$(count_in "$(collect_incidents "$WATCHER_NODE")" "incident|batch.*fired")

    wait_for_state "running" "$RECOVERY_WAIT" || true
    final_state=$(unit_state)

    status="PASS"; notes=""
    total_crash=$((exited_count + state_changed))
    if [ "$total_crash" -ge 2 ]; then
        ok "Multiple crash events: exited=$exited_count, state_changed=$state_changed"
    elif [ "$total_crash" -ge 1 ]; then
        ok "At least 1 crash event (some deduped by cooldown)"
    else
        status="FAIL"; notes="no crash events"
        fail "No crash events after 3 kills"
    fi
    [ "$stopped_count" -gt 0 ] && { warn "CONTRADICTORY: $stopped_count service.stopped in crash loop"; notes="${notes:+$notes; }contradictory stopped=$stopped_count"; }
    [ -n "$WATCHER_NODE" ] && [ "$incident_count" -gt 3 ] && { warn "DUPLICATE: $incident_count incidents (should be ≤3)"; notes="${notes:+$notes; }dup=$incident_count"; }

    record_result "crash-loop" "$status" "≥2 crash events" \
        "exit=$exited_count,chg=$state_changed,stop=$stopped_count,start=$started_count" \
        "$incident_count" "0" "$final_state" "$notes"

    dexec systemctl start "$TARGET_UNIT" 2>/dev/null || true
    wait_for_state "running" 15 || true
    sleep 10
fi

# ══════════════════════════════════════════════════════════════════════════
# SCENARIO 5: Fast Stop/Start
# ══════════════════════════════════════════════════════════════════════════
if should_run "fast-stop-start"; then
    echo ""
    log "${BOLD}[5/5] FAST STOP/START${NC}"
    SCENARIO_START=$(date -u +"%Y-%m-%d %H:%M:%S")

    log "Stopping and immediately restarting $TARGET_UNIT..."
    dexec systemctl stop "$TARGET_UNIT" 2>/dev/null || true
    sleep 1  # 1s gap — shorter than node-agent's 5s poll
    dexec systemctl start "$TARGET_UNIT" 2>/dev/null || true
    wait_for_state "running" 15 || warn "Service did not restart"

    log "Waiting ${EVENT_WAIT}s for events..."
    sleep "$EVENT_WAIT"

    events=$(collect_events "$SCENARIO_START")
    exited_count=$(count_in "$events" "service\.exited")
    stopped_count=$(count_in "$events" "service\.stopped")
    started_count=$(count_in "$events" "service\.started")
    state_changed=$(count_in "$events" "service\.state_changed")

    incident_count=0
    [ -n "$WATCHER_NODE" ] && incident_count=$(count_in "$(collect_incidents "$WATCHER_NODE")" "incident|batch.*fired")
    final_state=$(unit_state)

    status="PASS"; notes=""
    [ "$exited_count" -gt 0 ] && { status="FAIL"; notes="false service.exited"; fail "False crash on fast stop/start"; } || ok "No false crash events"
    [ "$incident_count" -gt 0 ] && { warn "Incident on intentional restart ($incident_count)"; notes="${notes:+$notes; }incident=$incident_count"; }
    [[ "$final_state" == *"running"* ]] && ok "Running: $final_state" || notes="${notes:+$notes; }not recovered ($final_state)"

    record_result "fast-stop-start" "$status" "stopped+started" \
        "exit=$exited_count,chg=$state_changed,stop=$stopped_count,start=$started_count" \
        "$incident_count" "0" "$final_state" "$notes"
fi

# ── Final Report ──────────────────────────────────────────────────────────
echo ""
echo -e "${BOLD}═══════════════════════════════════════════════════════════════${NC}"
echo -e "${BOLD}  RESULTS — Run ${CYAN}${RUN_ID}${NC}"
echo -e "${BOLD}═══════════════════════════════════════════════════════════════${NC}"
echo ""
printf "${BOLD}%-22s %-6s %-18s %-18s %-10s %-12s %-18s %s${NC}\n" \
    "SCENARIO" "STATUS" "EXPECTED" "ACTUAL" "INCIDENTS" "REMEDIATION" "FINAL STATE" "NOTES"
printf '%.0s─' {1..140}; echo ""
echo "$REPORT"
echo ""
printf '%.0s─' {1..140}; echo ""
echo ""

echo -e "${BOLD}Pipeline status:${NC}"
[ -n "$EVENT_NODE" ] && echo -e "  ${GREEN}✓${NC} Event service:  $EVENT_NODE" \
                     || echo -e "  ${RED}✗${NC} Event service:  NOT RUNNING"
[ -n "$WATCHER_NODE" ] && echo -e "  ${GREEN}✓${NC} AI watcher:     $WATCHER_NODE" \
                       || echo -e "  ${YELLOW}!${NC} AI watcher:     NOT RUNNING (incident detection untested)"
echo ""

TOTAL=$((PASS + FAIL + SKIP))
echo -e "${BOLD}Summary:${NC} ${GREEN}${PASS} passed${NC}, ${RED}${FAIL} failed${NC}, ${YELLOW}${SKIP} skipped${NC} (${TOTAL} total)"
echo -e "Correlation: grep for ${CYAN}${RUN_ID}${NC} or timestamps after ${RUN_START}"
echo ""

[ "$FAIL" -eq 0 ]
