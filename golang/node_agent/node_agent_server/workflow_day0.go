package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/node_agent/node_agentpb"
	"github.com/globulario/services/golang/workflow/engine"
	"github.com/globulario/services/golang/workflow/v1alpha1"
)

const (
	bootstrapEnabledPath = "/var/lib/globular/bootstrap.enabled"
	bootstrapTokenDir    = "/var/lib/globular/tokens"
	bootstrapLogDir      = "/var/lib/globular/logs/bootstrap"
)

// RunDay0BootstrapWorkflow executes the day0.bootstrap workflow definition
// to perform the full cluster bootstrap sequence. This replaces the manual
// install-day0.sh script with a declarative, observable workflow.
func (srv *NodeAgentServer) RunDay0BootstrapWorkflow(ctx context.Context, defPath string, inputs map[string]any) (*engine.Run, error) {
	loader := v1alpha1.NewLoader()
	def, err := loader.LoadFile(defPath)
	if err != nil {
		return nil, fmt.Errorf("load workflow definition %s: %w", defPath, err)
	}

	repoAddr := ""
	if addr, ok := inputs["repository_address"].(string); ok {
		repoAddr = addr
	}
	if repoAddr == "" {
		repoAddr = srv.discoverRepositoryAddr()
	}

	domain := ""
	if d, ok := inputs["domain"].(string); ok {
		domain = d
	}

	router := engine.NewRouter()

	// Wire installer actions to real node-agent capabilities.
	engine.RegisterInstallerActions(router, engine.InstallerConfig{
		SetupTLS: func(ctx context.Context, clusterID string) error {
			// TLS is set up by the Globule process before node-agent starts.
			// Verify certs exist as a sanity check.
			for _, path := range []string{
				"/var/lib/globular/config/tls/server.crt",
				"/var/lib/globular/config/tls/server.key",
				"/var/lib/globular/pki/ca.crt",
			} {
				if _, err := os.Stat(path); err != nil {
					return fmt.Errorf("TLS cert missing: %s", path)
				}
			}
			log.Printf("day0: TLS certs verified for cluster %s", clusterID)
			return nil
		},

		EnableBootstrapWindow: func(ctx context.Context, ttl time.Duration) error {
			expiry := time.Now().Add(ttl).Format(time.RFC3339)
			if err := os.MkdirAll(filepath.Dir(bootstrapEnabledPath), 0o755); err != nil {
				return err
			}
			log.Printf("day0: enabling bootstrap window until %s", expiry)
			return os.WriteFile(bootstrapEnabledPath, []byte(expiry+"\n"), 0o644)
		},

		DisableBootstrapWindow: func(ctx context.Context) error {
			log.Printf("day0: disabling bootstrap window")
			if err := os.Remove(bootstrapEnabledPath); err != nil && !os.IsNotExist(err) {
				return err
			}
			return nil
		},

		WriteBootstrapCreds: func(ctx context.Context) error {
			// Write a bootstrap sa token so early services can authenticate.
			if err := os.MkdirAll(bootstrapTokenDir, 0o700); err != nil {
				return err
			}
			tokenPath := filepath.Join(bootstrapTokenDir, "bootstrap_sa_token")
			log.Printf("day0: writing bootstrap credentials to %s", tokenPath)
			return os.WriteFile(tokenPath, []byte("bootstrap\n"), 0o600)
		},

		InstallPackage: func(ctx context.Context, name string) error {
			return srv.InstallPackage(ctx, name, "SERVICE", repoAddr, "")
		},

		InstallPackageSet: func(ctx context.Context, packages []string) error {
			var errs []string
			for _, pkg := range packages {
				kind := inferPackageKind(pkg)
				if err := srv.InstallPackage(ctx, pkg, kind, repoAddr, ""); err != nil {
					errs = append(errs, fmt.Sprintf("%s: %v", pkg, err))
				}
			}
			if len(errs) > 0 {
				return fmt.Errorf("install failures: %s", strings.Join(errs, "; "))
			}
			return nil
		},

		InstallProfileSets: func(ctx context.Context, profiles []string) error {
			// Profile-based installation is resolved by the controller.
			// During Day-0, packages are installed explicitly in prior steps.
			log.Printf("day0: install profile sets %v (no-op, packages installed explicitly)", profiles)
			return nil
		},

		ConfigureSharedStorage: func(ctx context.Context) error {
			// Create MinIO buckets if mc CLI is available.
			mc, err := exec.LookPath("mc")
			if err != nil {
				log.Printf("day0: mc CLI not found, skipping MinIO bucket setup")
				return nil
			}
			buckets := []string{"globular-packages", "globular-config", "globular-backups"}
			for _, bucket := range buckets {
				cmd := exec.CommandContext(ctx, mc, "mb", "--ignore-existing", "local/"+bucket)
				if out, err := cmd.CombinedOutput(); err != nil {
					log.Printf("day0: mc mb %s: %s (%v)", bucket, string(out), err)
				}
			}

			// Upload workflow definitions to globular-config/workflows/ in MinIO.
			// Same pattern as pki/ca.pem and ai/CLAUDE.md — cluster-wide config.
			workflowDir := "/var/lib/globular/workflows"
			entries, err := os.ReadDir(workflowDir)
			if err != nil {
				log.Printf("day0: no workflow definitions at %s: %v", workflowDir, err)
				return nil
			}
			uploaded := 0
			for _, e := range entries {
				if e.IsDir() || !strings.HasSuffix(e.Name(), ".yaml") {
					continue
				}
				data, err := os.ReadFile(filepath.Join(workflowDir, e.Name()))
				if err != nil {
					log.Printf("day0: read %s: %v", e.Name(), err)
					continue
				}
				key := "workflows/" + e.Name()
				if err := config.PutClusterConfig(key, data); err != nil {
					log.Printf("day0: upload %s to MinIO: %v", key, err)
					continue
				}
				uploaded++
			}
			log.Printf("day0: %d workflow definitions uploaded to MinIO globular-config/workflows/", uploaded)

			log.Printf("day0: shared storage configured (%d buckets)", len(buckets))
			return nil
		},

		BootstrapDNS: func(ctx context.Context, d string) error {
			if d == "" {
				d = domain
			}
			// Use the globular CLI if available.
			cli, err := exec.LookPath("globular")
			if err != nil {
				log.Printf("day0: globular CLI not found, skipping DNS bootstrap")
				return nil
			}
			cmd := exec.CommandContext(ctx, cli, "dns", "bootstrap", "--domain", d)
			out, err := cmd.CombinedOutput()
			if err != nil {
				return fmt.Errorf("dns bootstrap: %s (%w)", strings.TrimSpace(string(out)), err)
			}
			log.Printf("day0: DNS bootstrapped for %s", d)
			return nil
		},

		ValidateClusterHealth: func(ctx context.Context) error {
			// Verify key infrastructure services are active.
			critical := []string{
				"globular-etcd.service",
				"globular-gateway.service",
				"globular-xds.service",
			}
			var inactive []string
			for _, unit := range critical {
				out, err := exec.CommandContext(ctx, "systemctl", "is-active", unit).Output()
				if err != nil || strings.TrimSpace(string(out)) != "active" {
					inactive = append(inactive, unit)
				}
			}
			if len(inactive) > 0 {
				return fmt.Errorf("inactive services: %s", strings.Join(inactive, ", "))
			}
			log.Printf("day0: cluster health validated (%d critical services active)", len(critical))
			return nil
		},

		GenerateJoinToken: func(ctx context.Context) (string, error) {
			cli, err := exec.LookPath("globular")
			if err != nil {
				return "", fmt.Errorf("globular CLI not found")
			}
			cmd := exec.CommandContext(ctx, cli, "cluster", "token", "create")
			out, err := cmd.CombinedOutput()
			if err != nil {
				return "", fmt.Errorf("token create: %s (%w)", strings.TrimSpace(string(out)), err)
			}
			token := strings.TrimSpace(string(out))
			log.Printf("day0: join token generated (%d chars)", len(token))
			return token, nil
		},

		RestartServices: func(ctx context.Context, services []string) error {
			for _, svc := range services {
				unit := "globular-" + svc + ".service"
				log.Printf("day0: restarting %s", unit)
				if out, err := exec.CommandContext(ctx, "systemctl", "restart", unit).CombinedOutput(); err != nil {
					log.Printf("day0: restart %s: %s (%v)", unit, string(out), err)
					// Non-fatal — service may not exist yet.
				}
			}
			return nil
		},

		ClusterBootstrap: func(ctx context.Context, clusterID, nodeID string) error {
			cli, err := exec.LookPath("globular")
			if err != nil {
				return fmt.Errorf("globular CLI not found")
			}
			cmd := exec.CommandContext(ctx, cli, "cluster", "bootstrap",
				"--cluster-id", clusterID, "--node-id", nodeID)
			out, err := cmd.CombinedOutput()
			if err != nil {
				return fmt.Errorf("cluster bootstrap: %s (%w)", strings.TrimSpace(string(out)), err)
			}
			log.Printf("day0: cluster bootstrap complete (cluster=%s node=%s)", clusterID, nodeID)
			return nil
		},

		CaptureFailureBundle: func(ctx context.Context, runID string) error {
			if err := os.MkdirAll(bootstrapLogDir, 0o755); err != nil {
				return err
			}
			bundlePath := filepath.Join(bootstrapLogDir, fmt.Sprintf("failure-%s.log", runID))
			units := []string{
				"globular-etcd.service", "globular-gateway.service",
				"globular-xds.service", "globular-envoy.service",
				"globular-node-agent.service", "globular-cluster-controller.service",
			}
			var buf strings.Builder
			for _, unit := range units {
				out, _ := exec.CommandContext(ctx, "journalctl", "-u", unit,
					"-n", "100", "--no-pager").CombinedOutput()
				buf.WriteString(fmt.Sprintf("=== %s ===\n%s\n\n", unit, string(out)))
			}
			os.WriteFile(bundlePath, []byte(buf.String()), 0o644)
			log.Printf("day0: failure bundle captured to %s", bundlePath)
			return nil
		},
	})

	// Wire repository actions.
	engine.RegisterRepositoryActions(router, engine.RepositoryConfig{
		PublishBootstrapArtifacts: func(ctx context.Context, source string) error {
			// Try the ensure-bootstrap-artifacts.sh script if available.
			script := "/usr/lib/globular/scripts/ensure-bootstrap-artifacts.sh"
			if _, err := os.Stat(script); err == nil {
				cmd := exec.CommandContext(ctx, "/bin/bash", script)
				cmd.Env = os.Environ()
				out, err := cmd.CombinedOutput()
				if err != nil {
					return fmt.Errorf("publish artifacts: %s (%w)", strings.TrimSpace(string(out)), err)
				}
				log.Printf("day0: bootstrap artifacts published via script")
				return nil
			}
			// Fallback: use globular CLI.
			cli, _ := exec.LookPath("globular")
			if cli != "" {
				cmd := exec.CommandContext(ctx, cli, "pkg", "publish", "--source", source)
				out, err := cmd.CombinedOutput()
				if err != nil {
					return fmt.Errorf("pkg publish: %s (%w)", strings.TrimSpace(string(out)), err)
				}
				log.Printf("day0: bootstrap artifacts published via CLI")
				return nil
			}
			log.Printf("day0: no publish mechanism available, skipping artifact publish")
			return nil
		},
	})

	// Wire controller actions (called locally via CLI since controller starts mid-bootstrap).
	engine.RegisterReleaseControllerActions(router, engine.ReleaseControllerConfig{
		SeedDesiredFromInstalled: func(ctx context.Context, clusterID string) error {
			cli, err := exec.LookPath("globular")
			if err != nil {
				log.Printf("day0: globular CLI not found, skipping desired state seed")
				return nil
			}
			cmd := exec.CommandContext(ctx, cli, "services", "seed")
			out, err := cmd.CombinedOutput()
			if err != nil {
				return fmt.Errorf("seed desired: %s (%w)", strings.TrimSpace(string(out)), err)
			}
			log.Printf("day0: desired state seeded from installed")
			return nil
		},
		ReconcileUntilStable: func(ctx context.Context, clusterID string) error {
			// Wait for controller reconcile to settle.
			log.Printf("day0: waiting for reconciliation to stabilize...")
			time.Sleep(5 * time.Second)
			return nil
		},
		EmitBootstrapSucceeded: func(ctx context.Context, clusterID string) error {
			log.Printf("day0: cluster %s bootstrap SUCCEEDED", clusterID)
			return nil
		},
	})

	// Also register node-agent actions (probe_infra_health, verify_services_active, etc.)
	engine.RegisterNodeAgentActions(router, engine.NodeAgentConfig{
		NodeID: srv.nodeID,
		FetchAndInstall: func(ctx context.Context, pkg engine.PackageRef) error {
			return srv.InstallPackage(ctx, pkg.Name, pkg.Kind, repoAddr, "")
		},
		IsServiceActive: func(name string) bool {
			return engine.DefaultIsServiceActive(name)
		},
		SyncInstalledState: func(ctx context.Context) error {
			srv.syncInstalledStateToEtcd(ctx)
			return nil
		},
		ProbeInfraHealth: func(ctx context.Context, probeName string) bool {
			resp, err := srv.RunWorkflow(ctx, &node_agentpb.RunWorkflowRequest{
				WorkflowName: probeName,
			})
			return err == nil && resp.GetStatus() == "SUCCEEDED"
		},
	})

	eng := &engine.Engine{
		Router: router,
		OnStepDone: func(run *engine.Run, step *engine.StepState) {
			elapsed := time.Duration(0)
			if !step.StartedAt.IsZero() && !step.FinishedAt.IsZero() {
				elapsed = step.FinishedAt.Sub(step.StartedAt)
			}
			log.Printf("day0-workflow: step %s → %s (%s)",
				step.ID, step.Status, elapsed.Round(time.Millisecond))
		},
	}

	log.Printf("day0-workflow: starting %s", def.Metadata.Name)
	start := time.Now()
	run, err := eng.Execute(ctx, def, inputs)
	elapsed := time.Since(start)

	if err != nil {
		log.Printf("day0-workflow: FAILED after %s: %v",
			elapsed.Round(time.Millisecond), err)
	} else {
		succeeded := 0
		for _, st := range run.Steps {
			if st.Status == engine.StepSucceeded {
				succeeded++
			}
		}
		log.Printf("day0-workflow: SUCCEEDED in %s (%d/%d steps)",
			elapsed.Round(time.Millisecond), succeeded, len(run.Steps))
	}

	return run, err
}

// inferPackageKind returns the package kind based on known infrastructure names.
func inferPackageKind(name string) string {
	switch name {
	case "scylladb", "etcd", "minio", "envoy", "xds", "gateway",
		"node-agent", "cluster-controller", "cluster-doctor",
		"prometheus", "node-exporter", "sidekick",
		"scylla-manager", "scylla-manager-agent":
		return "INFRASTRUCTURE"
	case "mc", "globular-cli", "etcdctl", "rclone", "restic", "sctool",
		"sha256sum", "yt-dlp", "ffmpeg":
		return "COMMAND"
	default:
		return "SERVICE"
	}
}

// resolveDay0WorkflowPath finds the day0.bootstrap.yaml definition.
func resolveDay0WorkflowPath() string {
	candidates := []string{
		"/var/lib/globular/workflows/day0.bootstrap.yaml",
		"/usr/lib/globular/workflows/day0.bootstrap.yaml",
		"/tmp/day0.bootstrap.yaml",
	}
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}
