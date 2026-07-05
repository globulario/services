// @awareness namespace=globular.platform
// @awareness component=platform_node_agent.grpc_workflow
// @awareness file_role=node_local_workflow_dispatch_running_action_handlers_per_step
// @awareness implements=globular.platform:intent.node_agent.is_executor_not_cluster_brain
// @awareness risk=critical
package main

// grpc_workflow.go — the node-agent's workflow execution surface.
// The controller decides the steps; this code runs each step's
// registered action handler in order and reports per-step status
// back to the workflow service.
//
// MUST NOT skip, reorder, or invent steps. A node-agent that
// "optimised" a workflow by collapsing steps would silently
// break the controller's ability to replay or audit the run.
// Step failures MUST surface as such — translating a real
// failure to a benign status to "unstick" the workflow is the
// fastest way to ship a broken upgrade. The workflow service is
// the only authority on whether a stuck run should be skipped;
// see grpc_workflow_skip.go for the explicit operator path.

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/installed_state"
	"github.com/globulario/services/golang/node_agent/node_agent_server/internal/actions"
	"github.com/globulario/services/golang/node_agent/node_agent_server/internal/supervisor"
	"github.com/globulario/services/golang/node_agent/node_agentpb"
	"github.com/globulario/services/golang/workflow/engine"
	"github.com/globulario/services/golang/workflow/v1alpha1"
	"google.golang.org/protobuf/types/known/structpb"
)

var writeConvergenceResult = installed_state.WriteConvergenceResult

func defaultClusterID() string {
	if d, err := config.GetDomain(); err == nil && strings.TrimSpace(d) != "" {
		return strings.TrimSpace(d)
	}
	return "globular.internal"
}

// RunWorkflow implements the gRPC endpoint for workflow execution.
// The controller (or CLI) calls this to trigger a workflow on the node.
func (srv *NodeAgentServer) RunWorkflow(ctx context.Context, req *node_agentpb.RunWorkflowRequest) (*node_agentpb.RunWorkflowResponse, error) {
	name := req.GetWorkflowName()
	if name == "" {
		name = "node.join"
	}

	// Node-identity fence: a node-targeted workflow must execute only on its
	// intended node. The controller stamps target_node_id when it dispatches to
	// a specific node's agent; if present it MUST match this agent's own
	// identity. A package meant for node X reaching node Y's agent (e.g. a
	// mis-resolved or stale endpoint) is rejected here rather than silently
	// installed on the wrong node — the action is attributed pointwise to the
	// intended node (four_layer.workflow_actor_attribution_required;
	// forbidden_fix:cross_instance_coupling_via_single_workflow_run). Absent
	// target_node_id keeps older dispatchers working unchanged.
	if tgt := strings.TrimSpace(req.GetInputs()["target_node_id"]); tgt != "" && srv.nodeID != "" && tgt != srv.nodeID {
		return &node_agentpb.RunWorkflowResponse{
			Status: "FAILED",
			Error: fmt.Sprintf("node-identity fence: workflow %q targeted node %s but this agent is node %s",
				name, tgt, srv.nodeID),
		}, nil
	}

	// Synthetic workflows: simple actions that don't need a YAML definition.
	switch name {
	case "install-package":
		return srv.runInstallPackage(ctx, req)
	case "uninstall-package":
		return srv.runUninstallPackage(ctx, req)
	case "probe-scylla-health":
		return srv.runProbeScyllaHealth(ctx, req)
	case "wipe-scylla-data":
		return srv.runWipeScyllaData(ctx, req)
	case "scylla-remove-node":
		return srv.runScyllaRemoveNode(ctx, req)
	case "probe-etcd-health":
		return srv.runProbeEtcdHealth(ctx, req)
	case "wipe-etcd-and-rejoin":
		return srv.runWipeEtcdAndRejoin(ctx, req)
	case "probe-minio-health":
		return srv.runProbeMinioHealth(ctx, req)
	case "webroot-sync":
		return srv.runWebrootSync(ctx, req)
	case "day0.bootstrap":
		return srv.runDay0Bootstrap(ctx, req)
	}

	// Resolve definition path.
	defPath := req.GetDefinitionPath()
	if defPath == "" {
		defPath = resolveWorkflowPath(name)
	}
	if defPath == "" {
		return nil, fmt.Errorf("workflow definition %q not found", name)
	}

	// Build inputs from request + local state.
	inputs := make(map[string]any)
	for k, v := range req.GetInputs() {
		inputs[k] = v
	}
	// Fill in defaults from local state.
	if _, ok := inputs["cluster_id"]; !ok {
		inputs["cluster_id"] = defaultClusterID()
	}
	if _, ok := inputs["node_id"]; !ok {
		inputs["node_id"] = srv.nodeID
	}
	if _, ok := inputs["node_hostname"]; !ok && srv.state != nil {
		inputs["node_hostname"] = srv.state.NodeName
	}
	if _, ok := inputs["node_ip"]; !ok && srv.state != nil {
		inputs["node_ip"] = srv.state.AdvertiseIP
	}
	if _, ok := inputs["repository_address"]; !ok {
		inputs["repository_address"] = srv.discoverRepositoryAddr()
	}

	log.Printf("grpc-workflow: starting %s (def=%s)", name, defPath)
	start := time.Now()

	run, err := srv.RunWorkflowDefinition(ctx, defPath, inputs)
	elapsed := time.Since(start)

	resp := &node_agentpb.RunWorkflowResponse{
		DurationMs: elapsed.Milliseconds(),
	}

	if run != nil {
		resp.RunId = run.ID
		resp.Status = string(run.Status)
		for _, st := range run.Steps {
			resp.StepsTotal++
			switch st.Status {
			case engine.StepSucceeded:
				resp.StepsSucceeded++
			case engine.StepFailed:
				resp.StepsFailed++
			}
		}
	}

	if err != nil {
		resp.Status = "FAILED"
		resp.Error = err.Error()
	}

	return resp, nil
}

// extractRunInstallPackageHashes returns the two strictly-separate hash schemas
// the install-package workflow carries:
//
//   - convergenceHash: ComputeReleaseDesiredHash output
//     (publisher/name=version+b:N;), used ONLY by canSkipInstallPackage to
//     decide whether the installed package already matches desired, and as
//     LocalHash in the convergence result (stamped as pkg.Checksum so the
//     controller's classifyPackageConvergence stops re-dispatching).
//
//   - expectedSha256: BINARY sha256 from the repository manifest's
//     entrypoint_checksum, used ONLY as ApplyPackageReleaseRequest.ExpectedSha256
//     for the node-agent's verifyInstalledBinaryHashStrict gate.
//
// These two schemas MUST NOT be aliased. The historical fallback
// (`if desired_hash=="" then desired_hash = expected_sha256`) silently routed
// the convergence identity hash into the binary verify gate — the v1.2.119
// regression that produced installed_binary_hash_mismatch on every dispatch.
// See invariant runtime.success_requires_expected_binary_checksum and the
// hash schema documentation in release_resolver.go ResolvedArtifact.
func extractRunInstallPackageHashes(inputs map[string]string) (convergenceHash, expectedSha256 string) {
	convergenceHash = strings.TrimSpace(inputs["desired_hash"])
	expectedSha256 = strings.TrimSpace(inputs["expected_sha256"])
	return
}

// runInstallPackage handles the synthetic "install-package" workflow.
// The controller sends this when it wants a single package installed on this node.
func (srv *NodeAgentServer) runInstallPackage(ctx context.Context, req *node_agentpb.RunWorkflowRequest) (*node_agentpb.RunWorkflowResponse, error) {
	inputs := req.GetInputs()
	pkgName := inputs["package_name"]
	pkgKind := inputs["kind"]
	if pkgName == "" {
		return nil, fmt.Errorf("install-package: missing package_name input")
	}
	if pkgKind == "" {
		pkgKind = "SERVICE"
	}

	// CRITICAL: Protect ScyllaDB from reinstall while serving CQL.
	// Reinstalling ScyllaDB wipes Raft state and corrupts the cluster.
	if pkgName == "scylladb" {
		if conn, err := net.DialTimeout("tcp", "0.0.0.0:9042", 2*time.Second); err == nil {
			conn.Close()
			log.Printf("grpc-workflow: install-package scylladb SKIPPED — CQL port 9042 is active, protecting Raft state")
			return &node_agentpb.RunWorkflowResponse{
				Status:         "SUCCEEDED",
				StepsTotal:     1,
				StepsSucceeded: 1,
			}, nil
		}
		// Also try the node's own IPs.
		addrs, _ := net.InterfaceAddrs()
		for _, a := range addrs {
			if ipNet, ok := a.(*net.IPNet); ok && !ipNet.IP.IsLoopback() && ipNet.IP.To4() != nil {
				if conn, err := net.DialTimeout("tcp", ipNet.IP.String()+":9042", 2*time.Second); err == nil {
					conn.Close()
					log.Printf("grpc-workflow: install-package scylladb SKIPPED — CQL active on %s:9042, protecting Raft state", ipNet.IP)
					return &node_agentpb.RunWorkflowResponse{
						Status:         "SUCCEEDED",
						StepsTotal:     1,
						StepsSucceeded: 1,
					}, nil
				}
			}
		}
	}

	// Check if already installed at the desired version with runtime proof.
	desiredVersion := inputs["version"]
	buildID := inputs["build_id"]
	convergenceHash, expectedSha256 := extractRunInstallPackageHashes(inputs)
	forceReinstall := false // set true when unit file is gone so apply-package bypasses its own idempotency guard
	if desiredVersion != "" {
		existing, _ := installed_state.GetInstalledPackage(ctx, srv.nodeID, pkgKind, pkgName)
		skipResult, reason := canSkipInstallPackage(
			ctx, pkgName, pkgKind, desiredVersion, convergenceHash, buildID, existing,
			supervisor.IsActive, supervisor.IsLoaded,
		)
		wfID := inputs["workflow_id"]
		if wfID == "" {
			wfID = "install-package"
		}
		switch skipResult {
		case installSkipAllowed:
			log.Printf("grpc-workflow: %s", reason)
			// Re-stamp the canonical install receipt FIRST, before the
			// runtime-proof refresh below. canSkipInstallPackage proved
			// version + build_id + checksum + active unit + entrypoint_
			// checksum present — i.e. the on-disk unit content matches
			// the desired version, which is a property of disk state and
			// NOT of the running PID. The receipt must therefore carry
			// canonical provenance (installed_by, unit_file_sha256,
			// binary_sha256), not a stale legacy_sidecar marker left over
			// from a pre-refactor migration.
			//
			// Placed BEFORE the runtime-proof block on purpose: wrapper
			// packages (envoy, keepalived, etc.) whose binary does not
			// match the *_server naming convention fail discoverRunning
			// Binaries lookup and the runtime-proof check returns
			// runtimeProofNoRunningPID — which terminates this branch
			// with FAILED. The receipt should still reflect the canonical
			// state of the disk; otherwise the doctor surfaces
			// unit_receipt_drift.unit_file_drift forever even though
			// disk matches the renderer output. Live regression observed
			// 2026-06-03 on globular-envoy.service.
			//
			// INFRASTRUCTURE wrapper packages hit this case repeatedly
			// because their install short-circuits when the unit content
			// survives sweeps.
			//
			// Best-effort: failures are logged but never affect any
			// subsequent return value.
			srv.restampReceiptOnInstallSkip(ctx, pkgName, pkgKind, desiredVersion, buildID)

			// Phase 27: runtime-proof refresh before claiming SUCCEEDED.
			// canSkipInstallPackage proved on-disk binary + active unit,
			// but the running PID may still be the OLD binary (the binary
			// was swapped without a unit restart). If the running PID's
			// checksum doesn't match expectedSha256, the skip is unsafe:
			// the service is still old AND its /globular/services/<id>/config
			// record (which only refreshes when the service self-registers
			// on startup) is stale too. See invariant
			// node_agent.install_skip_must_refresh_runtime_proof and the
			// matching failure_mode in docs/awareness/failure_modes.yaml.
			//
			// Command packages (unit == "") have no runtime to check — the
			// binary-on-disk proof in canSkipInstallPackage is sufficient.
			// Services without expectedSha256 in the dispatch return
			// runtimeProofMatches (no opinion to enforce) — equivalent to
			// today's behaviour.
			if packageUnit(pkgName) != "" {
				verdict, rtReason := verifyRunningBinaryMatchesExpected(pkgName, expectedSha256)
				switch verdict {
				case runtimeProofStale:
					// Running PID is old — restart the unit so the new
					// binary loads and the service self-registers its
					// updated etcd config. Use the existing Restart +
					// WaitActive helpers (same path used by the
					// installSkipDeniedInactive recovery below).
					log.Printf("grpc-workflow: %s", rtReason)
					unit := packageUnit(pkgName)
					if restartErr := supervisor.Restart(ctx, unit); restartErr != nil {
						log.Printf("grpc-workflow: install-package %s: runtime-proof restart failed: %v", pkgName, restartErr)
						return &node_agentpb.RunWorkflowResponse{
							Status:         "FAILED",
							StepsTotal:     1,
							StepsSucceeded: 0,
							Error:          fmt.Sprintf("runtime-proof restart failed: %v", restartErr),
						}, nil
					}
					if waitErr := supervisor.WaitActive(ctx, unit, 30*time.Second); waitErr != nil {
						log.Printf("grpc-workflow: install-package %s: runtime-proof unit did not become active: %v", pkgName, waitErr)
						return &node_agentpb.RunWorkflowResponse{
							Status:         "FAILED",
							StepsTotal:     1,
							StepsSucceeded: 0,
							Error:          fmt.Sprintf("runtime-proof unit did not become active: %v", waitErr),
						}, nil
					}
					log.Printf("grpc-workflow: install-package %s: runtime-proof restart succeeded (unit=%s)", pkgName, unit)
				case runtimeProofNoRunningPID:
					// No running PID found to verify against. Could be a
					// mid-restart window or a crashed service. Refuse to
					// claim success on weak evidence.
					log.Printf("grpc-workflow: %s", rtReason)
					return &node_agentpb.RunWorkflowResponse{
						Status:         "FAILED",
						StepsTotal:     1,
						StepsSucceeded: 0,
						Error:          rtReason,
					}, nil
				case runtimeProofMatches:
					// Running PID matches expected (or no opinion to enforce).
					log.Printf("grpc-workflow: %s", rtReason)
				}
			}
			srv.emitConvergenceResult(&installed_state.ConvergenceResultV1{
				ActionID:        convergenceActionID(srv.nodeID, pkgKind, pkgName, desiredVersion),
				WorkflowID:      wfID,
				Package:         pkgName,
				NodeID:          srv.nodeID,
				DesiredVersion:  desiredVersion,
				DesiredBuildID:  buildID,
				LocalVersion:    existing.GetVersion(),
				LocalBuildID:    existing.GetBuildId(),
				LocalHash:       convergenceHash,
				Outcome:         installed_state.OutcomeSuccessLocalPendingSync,
				SourceComponent: "node-agent",
				Evidence:        map[string]string{"kind": pkgKind, "skip_reason": "already_converged"},
			})
			return &node_agentpb.RunWorkflowResponse{
				Status:         "SUCCEEDED",
				StepsTotal:     1,
				StepsSucceeded: 1,
			}, nil

		case installSkipInactiveByDesign:
			// The desired layer says this inactive unit is convergent (e.g.
			// keepalived while ingress explicitly disables the VIP). Do NOT
			// Start/repair/reinstall — inactive IS the converged state. Record
			// the runtime proof as inactive_by_design and report success so the
			// reconciler stops looping (four_layer.runtime_expectation_must_be_
			// derived_from_desired_not_installed; install_skip_must_refresh_
			// runtime_proof). node-agent stays an executor: it derives this from
			// the controller-authored ingress spec, it does not invent policy.
			log.Printf("grpc-workflow: %s", reason)
			srv.emitConvergenceResult(&installed_state.ConvergenceResultV1{
				ActionID:        convergenceActionID(srv.nodeID, pkgKind, pkgName, desiredVersion),
				WorkflowID:      wfID,
				Package:         pkgName,
				NodeID:          srv.nodeID,
				DesiredVersion:  desiredVersion,
				DesiredBuildID:  buildID,
				LocalVersion:    existing.GetVersion(),
				LocalBuildID:    existing.GetBuildId(),
				LocalHash:       convergenceHash,
				Outcome:         installed_state.OutcomeSuccessLocalPendingSync,
				SourceComponent: "node-agent",
				Evidence:        map[string]string{"kind": pkgKind, "skip_reason": "inactive_by_design", "runtime_state": "inactive_by_design"},
			})
			return &node_agentpb.RunWorkflowResponse{
				Status:         "SUCCEEDED",
				StepsTotal:     1,
				StepsSucceeded: 1,
			}, nil

		case installSkipDeniedInactive:
			// Unit is loaded but inactive — try a Start before full reinstall.
			log.Printf("grpc-workflow: %s", reason)
			unit := packageUnit(pkgName)
			if startErr := supervisor.Start(ctx, unit); startErr == nil {
				if waitErr := supervisor.WaitActive(ctx, unit, 30*time.Second); waitErr == nil {
					log.Printf("grpc-workflow: install-package %s: repair via Start succeeded", pkgName)
					srv.emitConvergenceResult(&installed_state.ConvergenceResultV1{
						ActionID:        convergenceActionID(srv.nodeID, pkgKind, pkgName, desiredVersion),
						WorkflowID:      wfID,
						Package:         pkgName,
						NodeID:          srv.nodeID,
						DesiredVersion:  desiredVersion,
						DesiredBuildID:  buildID,
						LocalVersion:    existing.GetVersion(),
						LocalBuildID:    existing.GetBuildId(),
						LocalHash:       convergenceHash,
						Outcome:         installed_state.OutcomeSuccessLocalPendingSync,
						SourceComponent: "node-agent",
						Evidence:        map[string]string{"kind": pkgKind, "skip_reason": "repaired_via_start"},
					})
					return &node_agentpb.RunWorkflowResponse{
						Status:         "SUCCEEDED",
						StepsTotal:     1,
						StepsSucceeded: 1,
					}, nil
				}
			}
			log.Printf("grpc-workflow: install-package %s: repair via Start failed, proceeding with full reinstall", pkgName)

		case installSkipDeniedUnitGone:
			log.Printf("grpc-workflow: %s", reason)
			forceReinstall = true // unit file gone: bypass apply-package's build_id idempotency guard

		case installSkipDeniedNoRecord, installSkipDeniedVersion:
			log.Printf("grpc-workflow: %s", reason)
			// fall through to full reinstall
		}
	}

	// Native dependency preflight: if a required shared library is absent on
	// this node, emit OutcomeBlockedMissingNativeDep and return without
	// downloading. The drift suppressor holds re-dispatch indefinitely;
	// the operator installs the library then annotates the release to unblock.
	if missingLib := nativeDepMissing(pkgName); missingLib != "" {
		wfIDForBlock := inputs["workflow_id"]
		if wfIDForBlock == "" {
			wfIDForBlock = "install-package"
		}
		provider := nativeDepProvider(missingLib)
		manualAction := nativeDepManualAction(missingLib)
		evidence := map[string]string{
			"missing_lib": missingLib,
		}
		if provider != "" {
			evidence["provider"] = provider
		}
		if manualAction != "" {
			evidence["manual_action"] = manualAction
		}
		log.Printf("grpc-workflow: install-package %s BLOCKED — native dep %q absent", pkgName, missingLib)
		srv.emitConvergenceResult(&installed_state.ConvergenceResultV1{
			ActionID:        convergenceActionID(srv.nodeID, pkgKind, pkgName, desiredVersion),
			WorkflowID:      wfIDForBlock,
			Package:         pkgName,
			NodeID:          srv.nodeID,
			DesiredVersion:  desiredVersion,
			DesiredBuildID:  buildID,
			Outcome:         installed_state.OutcomeBlockedMissingNativeDep,
			ReasonCode:      "missing_native_dep",
			UnblockPolicy:   "dependency_present|operator_resume|policy_changed",
			Evidence:        evidence,
			SourceComponent: "node-agent",
		})
		return &node_agentpb.RunWorkflowResponse{
			Status:         "SUCCEEDED",
			StepsTotal:     1,
			StepsSucceeded: 1,
		}, nil
	}

	if buildID != "" {
		log.Printf("grpc-workflow: install-package %s (%s) build_id=%s", pkgName, pkgKind, buildID)
	} else {
		log.Printf("grpc-workflow: install-package %s (%s)", pkgName, pkgKind)
	}
	start := time.Now()
	log.Printf("grpc-workflow: install-package %s dispatching apply (build_id=%s expected_sha256_present=%t)",
		pkgName, buildID, expectedSha256 != "")
	applyResp, err := srv.ApplyPackageRelease(ctx, &node_agentpb.ApplyPackageReleaseRequest{
		PackageName:    pkgName,
		PackageKind:    pkgKind,
		Version:        desiredVersion,
		Publisher:      defaultPublisherID,
		ExpectedSha256: expectedSha256, // BINARY hash from manifest.entrypoint_checksum (NOT convergenceHash)
		OperationId:    inputs["workflow_id"],
		BuildId:        buildID,
		Force:          forceReinstall,
	})
	elapsed := time.Since(start)

	wfIDFull := inputs["workflow_id"]
	if wfIDFull == "" {
		wfIDFull = "install-package"
	}

	resp := &node_agentpb.RunWorkflowResponse{
		DurationMs: elapsed.Milliseconds(),
		StepsTotal: 1,
	}
	if err != nil {
		resp.Status = "FAILED"
		resp.Error = err.Error()
		resp.StepsFailed = 1
		log.Printf("grpc-workflow: install-package %s FAILED (%v): %v", pkgName, elapsed, err)
		srv.emitConvergenceResult(&installed_state.ConvergenceResultV1{
			ActionID:        convergenceActionID(srv.nodeID, pkgKind, pkgName, desiredVersion),
			WorkflowID:      wfIDFull,
			Package:         pkgName,
			NodeID:          srv.nodeID,
			DesiredVersion:  desiredVersion,
			DesiredBuildID:  buildID,
			Outcome:         installed_state.OutcomeFailedTransient,
			SourceComponent: "node-agent",
			Evidence:        map[string]string{"kind": pkgKind, "error": err.Error()},
		})
	} else if applyResp == nil {
		resp.Status = "FAILED"
		resp.Error = "apply package returned nil response"
		resp.StepsFailed = 1
		log.Printf("grpc-workflow: install-package %s FAILED (%v): nil apply response", pkgName, elapsed)
		srv.emitConvergenceResult(&installed_state.ConvergenceResultV1{
			ActionID:        convergenceActionID(srv.nodeID, pkgKind, pkgName, desiredVersion),
			WorkflowID:      wfIDFull,
			Package:         pkgName,
			NodeID:          srv.nodeID,
			DesiredVersion:  desiredVersion,
			DesiredBuildID:  buildID,
			Outcome:         installed_state.OutcomeFailedTransient,
			SourceComponent: "node-agent",
			Evidence:        map[string]string{"kind": pkgKind, "error": "nil apply response"},
		})
	} else if !applyResp.GetOk() {
		resp.Status = "FAILED"
		if applyResp.GetErrorDetail() != "" {
			resp.Error = applyResp.GetErrorDetail()
		} else {
			resp.Error = applyResp.GetMessage()
		}
		resp.StepsFailed = 1
		log.Printf("grpc-workflow: install-package %s FAILED (%v): %s", pkgName, elapsed, resp.Error)
		srv.emitConvergenceResult(&installed_state.ConvergenceResultV1{
			ActionID:        convergenceActionID(srv.nodeID, pkgKind, pkgName, desiredVersion),
			WorkflowID:      wfIDFull,
			Package:         pkgName,
			NodeID:          srv.nodeID,
			DesiredVersion:  desiredVersion,
			DesiredBuildID:  buildID,
			Outcome:         installed_state.OutcomeFailedTransient,
			SourceComponent: "node-agent",
			Evidence:        map[string]string{"kind": pkgKind, "error": resp.Error},
		})
	} else {
		resp.Status = "SUCCEEDED"
		resp.StepsSucceeded = 1
		log.Printf("grpc-workflow: install-package %s SUCCEEDED (%v, status=%s)", pkgName, elapsed, applyResp.GetStatus())
		srv.emitConvergenceResult(&installed_state.ConvergenceResultV1{
			ActionID:       convergenceActionID(srv.nodeID, pkgKind, pkgName, desiredVersion),
			WorkflowID:     wfIDFull,
			Package:        pkgName,
			NodeID:         srv.nodeID,
			DesiredVersion: desiredVersion,
			DesiredBuildID: buildID,
			LocalVersion:   desiredVersion,
			LocalBuildID:   buildID,
			// LocalHash tells the controller what artifact digest was installed so
			// it can stamp pkg.Checksum and stop re-dispatching on checksum mismatch.
			LocalHash:       convergenceHash,
			Outcome:         installed_state.OutcomeSuccessLocalPendingSync,
			SourceComponent: "node-agent",
			Evidence:        map[string]string{"kind": pkgKind},
		})

		// Stamp the convergence hash for INFRASTRUCTURE packages. The controller's
		// classifyPackageConvergence compares pkg.Checksum against
		// InfrastructureRelease.Status.DesiredHash (= ComputeInfrastructureDesiredHash
		// output). apply_package_release stamps the binary SHA256 which never matches
		// the convergence hash — causing a perpetual redispatch loop. Overwrite
		// Checksum with the convergence hash received from the workflow dispatcher.
		if strings.ToUpper(pkgKind) == "INFRASTRUCTURE" && convergenceHash != "" {
			if stampErr := StampInfraConvergenceHash(ctx, srv.nodeID, pkgName, convergenceHash); stampErr != nil {
				log.Printf("grpc-workflow: stamp convergence hash for %s failed (non-fatal): %v", pkgName, stampErr)
			} else {
				log.Printf("grpc-workflow: stamped convergence hash for INFRASTRUCTURE/%s", pkgName)
			}
		}
	}
	return resp, nil
}

// stampBuildID updates the installed-state record for a package with the
// build_id so the controller knows this exact build is installed and stops
// re-dispatching the same version.
func (srv *NodeAgentServer) stampBuildID(ctx context.Context, pkgName, pkgKind, buildID string) {
	_ = ctx
	_ = buildID
	log.Printf("grpc-workflow: stampBuildID skipped for %s/%s — controller owns authoritative installed-state commits", pkgKind, pkgName)
}

// writeInstalledRecord does a targeted, single-package etcd write so the
// controller sees the new version/build_id immediately after a successful
// install — without waiting for the batch syncInstalledStateToEtcd round.
// It preserves InstalledUnix, Checksum, Platform, and Metadata from any
// pre-existing record.
func (srv *NodeAgentServer) writeInstalledRecord(ctx context.Context, pkgName, pkgKind, version, buildID string) {
	_ = ctx
	_ = buildID
	log.Printf("grpc-workflow: writeInstalledRecord skipped for %s/%s@%s — controller owns authoritative installed-state commits", pkgKind, pkgName, version)
}

// emitConvergenceResult writes a ConvergenceResultV1 to etcd asynchronously.
// One retry after 5 seconds guards against transient etcd hiccups — a lost
// record means the controller never writes the installed-state entry and keeps
// re-dispatching the install workflow indefinitely.
func (srv *NodeAgentServer) emitConvergenceResult(r *installed_state.ConvergenceResultV1) {
	go func() {
		for attempt := 0; attempt < 2; attempt++ {
			if attempt > 0 {
				time.Sleep(5 * time.Second)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 35*time.Second)
			err := writeConvergenceResult(ctx, r)
			cancel()
			if err == nil {
				return
			}
			log.Printf("grpc-workflow: convergence-result %s/%s (attempt %d): %v", r.NodeID, r.Package, attempt+1, err)
		}
	}()
}

// convergenceActionID returns a deterministic etcd action key for a package
// install outcome. This key is stable across retries so the latest result
// always overwrites older attempts for the same version.
func convergenceActionID(nodeID, kind, name, version string) string {
	return nodeID + "/" + strings.ToUpper(kind) + "/" + name + "/" + version
}

// runUninstallPackage handles the synthetic "uninstall-package" workflow.
// It stops and removes a package from the node, then clears its installed state
// from etcd. The steps are:
//  1. Stop and disable the systemd unit (SERVICE/INFRASTRUCTURE only)
//  2. Remove the binary, unit file, config directory, and version marker
//  3. Daemon-reload systemd (SERVICE/INFRASTRUCTURE only)
//  4. Delete the installed-state record from etcd
//  5. Remove the service config from etcd
//  6. Sync installed state
func (srv *NodeAgentServer) runUninstallPackage(ctx context.Context, req *node_agentpb.RunWorkflowRequest) (*node_agentpb.RunWorkflowResponse, error) {
	inputs := req.GetInputs()
	pkgName := inputs["package_name"]
	pkgKind := inputs["kind"]
	if pkgName == "" {
		return nil, fmt.Errorf("uninstall-package: missing package_name input")
	}
	if pkgKind == "" {
		pkgKind = "SERVICE"
	}
	pkgKind = strings.ToUpper(pkgKind)

	log.Printf("grpc-workflow: uninstall-package %s (%s)", pkgName, pkgKind)
	start := time.Now()

	const totalSteps int32 = 3 // uninstall files, clear etcd state, sync

	// Step 1: Uninstall files (stop systemd, remove binary/unit/config).
	// Delegate to the registered package.uninstall for every package kind so the
	// uninstall contract is centralized in one action.
	var uninstallErr error
	switch pkgKind {
	case "SERVICE", "INFRASTRUCTURE":
		handler := actions.Get("package.uninstall")
		if handler == nil {
			uninstallErr = fmt.Errorf("action package.uninstall not registered")
		} else {
			argsMap := map[string]any{
				"name": pkgName,
				"kind": pkgKind,
			}
			// Allow caller to specify a custom systemd unit name
			// (e.g. "scylla-server.service" instead of "globular-scylladb.service").
			if unit := inputs["unit"]; unit != "" {
				argsMap["unit"] = unit
			}
			args, err := structpb.NewStruct(argsMap)
			if err != nil {
				uninstallErr = fmt.Errorf("build uninstall args: %w", err)
			} else {
				if _, err := handler.Apply(ctx, args); err != nil {
					uninstallErr = fmt.Errorf("uninstall %s: %w", pkgName, err)
				}
			}
		}
	case "COMMAND":
		handler := actions.Get("package.uninstall")
		if handler == nil {
			uninstallErr = fmt.Errorf("action package.uninstall not registered")
		} else {
			argsMap := map[string]any{
				"name": pkgName,
				"kind": pkgKind,
			}
			args, err := structpb.NewStruct(argsMap)
			if err != nil {
				uninstallErr = fmt.Errorf("build uninstall args: %w", err)
			} else if _, err := handler.Apply(ctx, args); err != nil {
				uninstallErr = fmt.Errorf("uninstall %s: %w", pkgName, err)
			}
		}
	default:
		uninstallErr = fmt.Errorf("unsupported package kind %q", pkgKind)
	}

	if uninstallErr != nil {
		elapsed := time.Since(start)
		log.Printf("grpc-workflow: uninstall-package %s FAILED (%v): %v", pkgName, elapsed, uninstallErr)
		return &node_agentpb.RunWorkflowResponse{
			Status:      "FAILED",
			Error:       uninstallErr.Error(),
			DurationMs:  elapsed.Milliseconds(),
			StepsTotal:  totalSteps,
			StepsFailed: 1,
		}, nil
	}

	// Step 2: Clear installed state from etcd.
	if err := installed_state.DeleteInstalledPackage(ctx, srv.nodeID, pkgKind, pkgName); err != nil {
		log.Printf("grpc-workflow: uninstall-package %s: warning: failed to clear installed state: %v", pkgName, err)
		// Non-fatal — the package files are already removed.
	}

	// Also clean up service config from etcd so it no longer appears in admin catalog.
	if err := config.DeleteServiceConfigurationByName(pkgName); err != nil {
		log.Printf("grpc-workflow: uninstall-package %s: warning: failed to clean service config: %v", pkgName, err)
	}

	// Step 3: Sync installed state so the controller sees the change immediately.
	srv.syncInstalledStateToEtcd(ctx)

	elapsed := time.Since(start)
	log.Printf("grpc-workflow: uninstall-package %s SUCCEEDED (%v)", pkgName, elapsed)
	return &node_agentpb.RunWorkflowResponse{
		Status:         "SUCCEEDED",
		DurationMs:     elapsed.Milliseconds(),
		StepsTotal:     totalSteps,
		StepsSucceeded: totalSteps,
	}, nil
}

// runDay0Bootstrap handles the "day0.bootstrap" workflow.
// This uses RunDay0BootstrapWorkflow which wires the installer-specific actions
// (TLS setup, package install, DNS bootstrap, etc.) that are different from
// the generic workflow runner.
func (srv *NodeAgentServer) runDay0Bootstrap(ctx context.Context, req *node_agentpb.RunWorkflowRequest) (*node_agentpb.RunWorkflowResponse, error) {
	defPath := resolveDay0WorkflowPath()
	if defPath == "" {
		// Also try the generic resolver.
		defPath = resolveWorkflowPath("day0.bootstrap")
	}
	if defPath == "" {
		return nil, fmt.Errorf("day0.bootstrap.yaml not found")
	}

	// Build inputs from request + defaults.
	inputs := make(map[string]any)
	for k, v := range req.GetInputs() {
		inputs[k] = v
	}
	if _, ok := inputs["cluster_id"]; !ok {
		inputs["cluster_id"] = defaultClusterID()
	}
	if _, ok := inputs["bootstrap_node_id"]; !ok {
		inputs["bootstrap_node_id"] = srv.nodeID
	}
	if _, ok := inputs["bootstrap_node_hostname"]; !ok && srv.state != nil {
		inputs["bootstrap_node_hostname"] = srv.state.NodeName
	}
	if _, ok := inputs["domain"]; !ok {
		inputs["domain"] = defaultClusterID()
	}
	if _, ok := inputs["repository_address"]; !ok {
		inputs["repository_address"] = ""
	}
	if _, ok := inputs["bootstrap_node_profiles"]; !ok {
		// Mirror foundingNodeProfiles from cluster_controller/profiles_normalize.go.
		// The workflow's verify_profile_install_set step requires this input.
		inputs["bootstrap_node_profiles"] = []string{"core", "control-plane", "storage"}
	}

	log.Printf("grpc-workflow: starting day0.bootstrap (def=%s)", defPath)
	start := time.Now()

	run, err := srv.RunDay0BootstrapWorkflow(ctx, defPath, inputs)
	elapsed := time.Since(start)

	resp := &node_agentpb.RunWorkflowResponse{
		DurationMs: elapsed.Milliseconds(),
	}
	if run != nil {
		resp.RunId = run.ID
		resp.Status = string(run.Status)
		for _, st := range run.Steps {
			resp.StepsTotal++
			switch st.Status {
			case engine.StepSucceeded:
				resp.StepsSucceeded++
			case engine.StepFailed:
				resp.StepsFailed++
			}
		}
	}
	if err != nil {
		resp.Status = "FAILED"
		resp.Error = err.Error()
	}
	return resp, nil
}

var fetchWorkflowDefsOnce sync.Once

// resolveWorkflowPath finds a workflow YAML by name.
// On first miss it attempts to fetch all definitions from MinIO.
func resolveWorkflowPath(name string) string {
	candidates := []string{
		fmt.Sprintf("/var/lib/globular/workflows/%s.yaml", name),
		fmt.Sprintf("/tmp/%s.yaml", name),
		fmt.Sprintf("/usr/lib/globular/workflows/%s.yaml", name),
	}
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}

	// Not found on disk — try fetching from MinIO (once).
	fetchWorkflowDefsOnce.Do(func() {
		fetchWorkflowDefsFromEtcd()
	})

	// Retry after fetch.
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

// fetchWorkflowDefsFromEtcd caches workflow definitions from etcd to
// /var/lib/globular/workflows/ so the local disk fallback in LoadFile works
// on nodes that joined before SeedCoreWorkflows ran.
func fetchWorkflowDefsFromEtcd() {
	if v1alpha1.EtcdFetcher == nil {
		log.Printf("workflow-resolver: etcd fetcher not configured — skipping cache")
		return
	}
	destDir := "/var/lib/globular/workflows"
	os.MkdirAll(destDir, 0o755)

	knownDefs := []string{
		"day0.bootstrap.yaml",
		"node.bootstrap.yaml",
		"node.join.yaml",
		"node.repair.yaml",
		"cluster.reconcile.yaml",
		"release.apply.package.yaml",
		"release.apply.infrastructure.yaml",
		"release.remove.package.yaml",
	}

	fetched := 0
	for _, name := range knownDefs {
		data, err := v1alpha1.EtcdFetcher(name)
		if err != nil {
			log.Printf("workflow-resolver: fetch %s from etcd: %v", name, err)
			continue
		}
		dest := filepath.Join(destDir, name)
		if err := os.WriteFile(dest, data, 0o644); err != nil {
			log.Printf("workflow-resolver: write %s: %v", dest, err)
			continue
		}
		fetched++
	}
	if fetched > 0 {
		log.Printf("workflow-resolver: cached %d workflow definitions from etcd to %s", fetched, destDir)
	} else {
		log.Printf("workflow-resolver: no workflow definitions found in etcd")
	}
}
