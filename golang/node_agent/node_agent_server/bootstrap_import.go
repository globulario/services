package main

// bootstrap_import.go — Phase G: Day-0 bootstrap hardening.
//
// After the repository becomes available, the node-agent scans its installed
// packages for provisional records (packages installed during day-0 bootstrap
// before the repository was running). For each provisional package, it calls
// ImportProvisionalArtifact to confirm the package identity and receive a
// repository-issued build_id.

import (
	"context"
	"crypto/sha256"
	"fmt"
	"log"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/installed_state"
	node_agentpb "github.com/globulario/services/golang/node_agent/node_agentpb"
	"github.com/globulario/services/golang/repository/repository_client"
	repopb "github.com/globulario/services/golang/repository/repositorypb"
)

// provisionalImportDone tracks whether we've already successfully imported
// all provisional packages. Once done, we stop checking.
var provisionalImportDone sync.Once
var provisionalImportCompleted bool

// importProvisionalPackages scans installed packages for provisional records
// and imports them into the repository. Called periodically from the heartbeat
// loop until all provisional packages are imported or none remain.
func (srv *NodeAgentServer) importProvisionalPackages(ctx context.Context) {
	if provisionalImportCompleted {
		return
	}

	if srv.nodeID == "" {
		return
	}

	// List all installed packages on this node.
	allKinds := []string{"SERVICE", "INFRASTRUCTURE", "COMMAND"}
	var provisional []*provisionalEntry

	for _, kind := range allKinds {
		pkgs, err := installed_state.ListInstalledPackages(ctx, srv.nodeID, kind)
		if err != nil {
			continue
		}
		for _, pkg := range pkgs {
			if pkg.GetProvisional() {
				provisional = append(provisional, &provisionalEntry{
					pkg:  pkg,
					kind: kind,
				})
			}
		}
	}

	if len(provisional) == 0 {
		provisionalImportCompleted = true
		return
	}

	// Resolve repository address.
	repoAddr := config.ResolveServiceAddr("repository.PackageRepository", "")
	if repoAddr == "" {
		// Repository not available yet — try again next cycle.
		return
	}

	slog.Info("bootstrap-import: found provisional packages",
		"count", len(provisional), "repo", repoAddr)

	client, err := repository_client.NewRepositoryService_Client(repoAddr, "repository.PackageRepository")
	if err != nil {
		slog.Warn("bootstrap-import: cannot connect to repository", "err", err)
		return
	}
	defer client.Close()

	imported := 0
	failed := 0

	for _, entry := range provisional {
		pkg := entry.pkg
		name := pkg.GetName()
		version := pkg.GetVersion()
		platform := pkg.GetPlatform()
		if platform == "" {
			platform = "linux_amd64"
		}
		publisher := pkg.GetPublisherId()
		if publisher == "" {
			publisher = "core@globular.io"
		}

		// Compute digest from the installed binary (if it exists).
		digest := computeInstalledDigest(name)

		resp, err := client.ImportProvisionalArtifact(&repopb.ImportProvisionalRequest{
			PublisherId:        publisher,
			Name:               name,
			Version:            version,
			Platform:           platform,
			Digest:             digest,
			ProvisionalBuildId: pkg.GetBuildId(),
			Kind:               entry.kind,
		})
		if err != nil {
			slog.Warn("bootstrap-import: RPC failed",
				"name", name, "version", version, "err", err)
			failed++
			continue
		}

		if resp.GetOk() {
			// Success: update installed-state with confirmed build_id
			// and clear provisional flag.
			pkg.BuildId = resp.GetConfirmedBuildId()
			pkg.Provisional = false
			pkg.UpdatedUnix = time.Now().Unix()
			if err := installed_state.WriteInstalledPackage(ctx, pkg); err != nil {
				log.Printf("bootstrap-import: write installed-state %s: %v", name, err)
			} else {
				slog.Info("bootstrap-import: confirmed",
					"name", name, "version", version,
					"build_id", resp.GetConfirmedBuildId())
			}
			imported++
		} else {
			// Conflict or rejection — log for operator resolution.
			slog.Warn("bootstrap-import: rejected",
				"name", name, "version", version,
				"message", resp.GetMessage())
			failed++
		}
	}

	slog.Info("bootstrap-import: cycle complete",
		"imported", imported, "failed", failed,
		"remaining", len(provisional)-imported)

	if failed == 0 {
		provisionalImportCompleted = true
	}
}

type provisionalEntry struct {
	pkg  *node_agentpb.InstalledPackage
	kind string
}

// computeInstalledDigest computes SHA256 of the installed binary for a service.
func computeInstalledDigest(name string) string {
	// Try common binary locations.
	paths := []string{
		fmt.Sprintf("/usr/lib/globular/bin/%s_server", name),
		fmt.Sprintf("/usr/lib/globular/bin/%s", name),
	}
	for _, p := range paths {
		data, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		h := sha256.Sum256(data)
		return fmt.Sprintf("sha256:%x", h)
	}
	return ""
}
