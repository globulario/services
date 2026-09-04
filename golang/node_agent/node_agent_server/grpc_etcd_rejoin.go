// @awareness namespace=globular.platform
// @awareness component=platform_node_agent.etcd_rejoin
// @awareness file_role=last_resort_etcd_member_wipe_requires_controller_to_have_rendered_new_config
// @awareness implements=globular.platform:intent.node_agent.destructive_member_wipes_require_controller_prep
// @awareness implements=globular.platform:intent.node_recovery.fence_before_destructive_reseed
// @awareness risk=critical
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/globulario/services/golang/node_agent/node_agent_server/internal/supervisor"
	"github.com/globulario/services/golang/node_agent/node_agentpb"
)

const etcdMemberDir = "/var/lib/globular/etcd/member"
const etcdUnit = "globular-etcd.service"

// etcdConfigPath is the file etcd reads its membership from. For a member that
// has already initialized, etcd takes membership from its own WAL and this file
// is inert. For an UNINITIALIZED member — which is exactly what the wipe below
// creates — this file is the only membership it has.
const etcdConfigPath = "/var/lib/globular/config/etcd.yaml"

// etcdRejoinConfigInput is the RunWorkflow input carrying the membership this
// node must come back with: the value of etcd.yaml's initial-cluster key, as the
// controller reads it from the LIVE RING (etcd's own member list), which is the
// only set etcd validates initial-cluster against.
//
// It is the MEMBERSHIP, not a whole config file, and that is deliberate. The
// controller's renderer and the installer disagree about the rest of etcd.yaml —
// the installer writes client-cert-auth: true with a trusted-ca-file on both
// transports and listens on 0.0.0.0, the renderer writes neither and binds the
// node IP. Shipping the renderer's whole file during a repair would silently drop
// client-certificate authentication on the repaired etcd endpoint. A repair may
// not change a security posture it was not asked to change; it rewrites the one
// field the wipe invalidates and leaves every other line exactly as the node
// already had it.
const etcdRejoinConfigInput = "etcd_initial_cluster"

// runWipeEtcdAndRejoin stops globular-etcd, installs the controller-rendered
// etcd.yaml, wipes the member data directory, and restarts etcd so it can rejoin
// the cluster as the fresh member the controller's MemberAdd created.
//
// WHY THE CONFIG COMES WITH THE REQUEST.
//
// This file's contract has always been
// intent.node_agent.destructive_member_wipes_require_controller_prep: the
// controller must have rendered the new membership before the wipe. That
// precondition was documented, assumed, and never checked — and the path that
// used to satisfy it no longer exists. dispatchPlan, which carried
// renderEtcdConfig's output to the node, is a no-op stub ("plan system removed"),
// so the on-disk etcd.yaml is whatever Day-0 or the join script wrote and is
// never refreshed when ring membership changes.
//
// The consequence is not a degraded rejoin, it is a permanent one. Wiping
// /var/lib/globular/etcd/member makes the node an uninitialized member; an
// uninitialized member re-reads initial-cluster on every start and etcd aborts
// with "error validating peerURLs ...: member count is unequal" whenever that
// list disagrees with the ring. It never converges, and the controller re-issues
// the wipe every etcdRejoinDispatchCooldown forever.
//
// Measured on the 5-node simulation, 1.2.359, 2026-09-03: node-5's etcd.yaml
// listed 4 members (node-2 absent) while the ring held 5; globular-etcd exited
// 1 on every start with that message; wipe-etcd-and-rejoin ran at 15:30:23,
// 15:33:23, 15:36:23 and 15:39:23 and reported SUCCEEDED each time; the scenario
// authority/node-clone-identity-collision failed its restoration postcondition
// with etcd_healthy_endpoints baseline=5 now=4.
//
// So the membership travels WITH the destructive request, from the actor that
// owns it. A request without it is refused before anything is destroyed:
// a wipe we cannot correctly configure is a wipe we must not perform
// (invariant:destructive_actions.require_explicit_guard).
func (srv *NodeAgentServer) runWipeEtcdAndRejoin(ctx context.Context, req *node_agentpb.RunWorkflowRequest) (*node_agentpb.RunWorkflowResponse, error) {
	start := time.Now()
	log.Printf("wipe-etcd-and-rejoin: starting etcd data wipe and rejoin")

	// Step 0: the guard. Refuse before touching anything if the caller did not
	// supply the membership this node must come back with.
	initialCluster := req.GetInputs()[etcdRejoinConfigInput]
	if initialCluster == "" {
		msg := fmt.Sprintf("wipe-etcd-and-rejoin: refused — no %q input. "+
			"Wiping %s makes this an uninitialized member whose initial-cluster must match the live ring exactly; "+
			"without the controller-rendered config the wipe would leave etcd unable to start at all "+
			"(intent.node_agent.destructive_member_wipes_require_controller_prep)",
			etcdRejoinConfigInput, etcdMemberDir)
		log.Print(msg)
		return &node_agentpb.RunWorkflowResponse{Status: "FAILED", Error: msg}, nil
	}

	// Step 1: stop etcd so we don't wipe a running process's data.
	stopCtx, stopCancel := context.WithTimeout(ctx, 30*time.Second)
	defer stopCancel()
	if err := supervisor.Stop(stopCtx, etcdUnit); err != nil {
		msg := fmt.Sprintf("wipe-etcd-and-rejoin: failed to stop %s: %v", etcdUnit, err)
		log.Print(msg)
		return &node_agentpb.RunWorkflowResponse{Status: "FAILED", Error: msg}, nil
	}
	log.Printf("wipe-etcd-and-rejoin: stopped %s", etcdUnit)

	// Step 2: install the controller-supplied membership BEFORE the wipe, so
	// there is no window in which the data dir is gone and the config is stale.
	if err := rewriteEtcdInitialCluster(etcdConfigPath, initialCluster); err != nil {
		msg := fmt.Sprintf("wipe-etcd-and-rejoin: failed to update initial-cluster in %s: %v", etcdConfigPath, err)
		log.Print(msg)
		return &node_agentpb.RunWorkflowResponse{Status: "FAILED", Error: msg}, nil
	}
	log.Printf("wipe-etcd-and-rejoin: set initial-cluster=%s in %s", initialCluster, etcdConfigPath)

	// Step 3: wipe the member directory. This contains the WAL and snapshots
	// that record the old (removed) member identity. etcd will re-create it
	// from scratch when it joins the cluster as a new member.
	if err := os.RemoveAll(etcdMemberDir); err != nil {
		msg := fmt.Sprintf("wipe-etcd-and-rejoin: failed to remove %s: %v", etcdMemberDir, err)
		log.Print(msg)
		return &node_agentpb.RunWorkflowResponse{Status: "FAILED", Error: msg}, nil
	}
	log.Printf("wipe-etcd-and-rejoin: wiped %s", etcdMemberDir)

	// Step 4: start etcd with the membership installed in step 2.
	//
	// systemd's start-limit is reached quickly when a previous episode
	// crash-looped, and Start() then fails with "start request repeated too
	// quickly" on a config that is now correct. Reset the counter first so the
	// repair is judged on this attempt, not on the failures it is fixing.
	startCtx, startCancel := context.WithTimeout(ctx, 90*time.Second)
	defer startCancel()
	if err := supervisor.ResetFailed(startCtx, etcdUnit); err != nil {
		log.Printf("wipe-etcd-and-rejoin: reset-failed %s: %v (continuing)", etcdUnit, err)
	}
	if err := supervisor.Start(startCtx, etcdUnit); err != nil {
		msg := fmt.Sprintf("wipe-etcd-and-rejoin: failed to start %s: %v", etcdUnit, err)
		log.Print(msg)
		return &node_agentpb.RunWorkflowResponse{Status: "FAILED", Error: msg}, nil
	}
	if err := supervisor.WaitActive(startCtx, etcdUnit, 60*time.Second); err != nil {
		msg := fmt.Sprintf("wipe-etcd-and-rejoin: %s did not become active: %v", etcdUnit, err)
		log.Print(msg)
		return &node_agentpb.RunWorkflowResponse{Status: "FAILED", Error: msg}, nil
	}

	elapsed := time.Since(start).Round(time.Millisecond)
	log.Printf("wipe-etcd-and-rejoin: completed successfully in %s", elapsed)
	return &node_agentpb.RunWorkflowResponse{
		Status:         "SUCCEEDED",
		StepsTotal:     4,
		StepsSucceeded: 4,
	}, nil
}

// rewriteEtcdInitialCluster replaces the initial-cluster value in an existing
// etcd.yaml and changes nothing else.
//
// It fails when the file is missing or carries no initial-cluster key: the node
// would then have no membership at all after the wipe, which is worse than the
// stale membership we came to fix. There is no "write a fresh config" fallback
// for the same reason the caller has no --force — a repair that invents the
// parts it cannot read is how a node comes back configured differently from the
// cluster it rejoins.
func rewriteEtcdInitialCluster(path, initialCluster string) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	lines := strings.Split(string(raw), "\n")
	replaced := false
	for i, line := range lines {
		// "initial-cluster:" only — never initial-cluster-state or
		// initial-cluster-token, whose values must survive untouched.
		if strings.HasPrefix(strings.TrimSpace(line), "initial-cluster:") {
			lines[i] = fmt.Sprintf("initial-cluster: %q", initialCluster)
			replaced = true
			break
		}
	}
	if !replaced {
		return fmt.Errorf("%s has no initial-cluster key — refusing to guess the rest of the config", path)
	}
	return writeEtcdConfigTo(path, strings.Join(lines, "\n"))
}

// writeEtcdConfigTo replaces etcd.yaml atomically and restores the ownership and
// mode etcd needs. A partial write here would be read by the very next start.
func writeEtcdConfigTo(path, content string) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".etcd.yaml.*")
	if err != nil {
		return fmt.Errorf("create temp in %s: %w", dir, err)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath) // no-op once the rename succeeds

	if _, err := tmp.WriteString(content); err != nil {
		tmp.Close()
		return fmt.Errorf("write temp: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return fmt.Errorf("sync temp: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp: %w", err)
	}
	if err := os.Chmod(tmpPath, 0o640); err != nil {
		return fmt.Errorf("chmod temp: %w", err)
	}
	applyGlobularOwnership(tmpPath)
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("rename onto %s: %w", path, err)
	}
	return nil
}

// applyGlobularOwnership sets globular:globular on a path, best-effort.
//
// Per CLAUDE.md: never hardcode UIDs/GIDs — always resolve via lookup. A lookup
// failure is logged and tolerated: on a node where the user does not exist the
// agent is not running as root either, and the rename would have failed first.
func applyGlobularOwnership(path string) {
	u, err := user.Lookup("globular")
	if err != nil {
		log.Printf("nodeagent: globular user not found — leaving owner of %s unchanged: %v", path, err)
		return
	}
	uid, err := strconv.Atoi(u.Uid)
	if err != nil {
		log.Printf("nodeagent: invalid globular UID %q: %v", u.Uid, err)
		return
	}
	g, err := user.LookupGroup("globular")
	if err != nil {
		log.Printf("nodeagent: globular group not found — leaving group of %s unchanged: %v", path, err)
		return
	}
	gid, err := strconv.Atoi(g.Gid)
	if err != nil {
		log.Printf("nodeagent: invalid globular GID %q: %v", g.Gid, err)
		return
	}
	if err := os.Chown(path, uid, gid); err != nil {
		log.Printf("nodeagent: cannot set globular:globular ownership on %s: %v", path, err)
	}
}
