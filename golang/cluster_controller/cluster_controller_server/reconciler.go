package main

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/globulario/services/golang/installed_state"
	"github.com/globulario/services/golang/repository/repositorypb"
	"github.com/globulario/services/golang/versionutil"
)

// driftReconciler periodically compares desired state against installed state
// on each node and dispatches ApplyPackageRelease for any drift detected.
// Only the leader runs reconciliation. This is the convergence backstop that
// ensures desired == installed without relying on manual intervention.
type driftReconciler struct {
	srv      *server
	interval time.Duration
	timeout  time.Duration // per-apply RPC timeout

	mu          sync.Mutex
	inflight    map[string]time.Time // "nodeID/KIND/name" -> dispatch time
	inflightTTL time.Duration
	sem         chan struct{} // bounds concurrent dispatches
}

func newDriftReconciler(srv *server, interval time.Duration) *driftReconciler {
	return &driftReconciler{
		srv:         srv,
		interval:    interval,
		timeout:     5 * time.Minute,
		inflight:    make(map[string]time.Time),
		inflightTTL: 10 * time.Minute,
		sem:         make(chan struct{}, 2), // max 2 concurrent applies
	}
}

// Start launches the reconciler as a background goroutine.
func (r *driftReconciler) Start(ctx context.Context) {
	safeGo("drift-reconciler", func() {
		ticker := time.NewTicker(r.interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if r.srv.isLeader() {
					r.reconcileOnce(ctx)
				}
			}
		}
	})
}

func (r *driftReconciler) reconcileOnce(ctx context.Context) {
	r.expireStale()

	desired := r.srv.collectDesiredVersions(ctx)
	if len(desired) == 0 {
		return
	}

	repo := resolveRepositoryInfo()

	// Snapshot node list under lock.
	r.srv.lock("drift-reconciler:snapshot")
	nodes := make(map[string]*nodeState, len(r.srv.state.Nodes))
	for id, n := range r.srv.state.Nodes {
		nodes[id] = n
	}
	r.srv.unlock()

	for nodeID, node := range nodes {
		if node.Status != "ready" {
			continue
		}
		if node.AgentEndpoint == "" {
			continue
		}

		pkgs, err := installed_state.ListInstalledPackages(ctx, nodeID, "")
		if err != nil {
			log.Printf("drift-reconciler: list installed for node %s: %v", nodeID, err)
			continue
		}

		// Index installed by "KIND/name".
		installedMap := make(map[string]installedInfo, len(pkgs))
		for _, pkg := range pkgs {
			key := strings.ToUpper(pkg.GetKind()) + "/" + canonicalServiceName(pkg.GetName())
			installedMap[key] = installedInfo{
				version:     pkg.GetVersion(),
				buildNumber: pkg.GetBuildNumber(),
				status:      pkg.GetStatus(),
			}
		}

		for desiredKey, dv := range desired {
			pkg, found := installedMap[desiredKey]

			// Already aligned.
			if found && versionutil.EqualFull(dv.version, dv.buildNumber, pkg.version, pkg.buildNumber) {
				continue
			}
			// Node-agent is already applying this package.
			if found && pkg.status == "updating" {
				continue
			}

			inflightKey := nodeID + "/" + desiredKey
			if r.isInflight(inflightKey) {
				continue
			}

			parts := strings.SplitN(desiredKey, "/", 2)
			if len(parts) != 2 {
				continue
			}
			kind, name := parts[0], parts[1]

			// Only reconcile SERVICE packages; infrastructure is managed by bootstrap.
			if kind != "SERVICE" {
				continue
			}

			// Verify the desired artifact is PUBLISHED before dispatching.
			// This prevents installing artifacts that are still VERIFIED (not yet
			// through the publish pipeline), maintaining the truth chain:
			// Repository → Desired → Installed → Runtime.
			if !r.isArtifactPublished(ctx, name, dv.version, dv.buildNumber, repo.Address) {
				log.Printf("drift-reconciler: skipping node=%s pkg=%s@%s-b%d — artifact not PUBLISHED",
					nodeID, name, dv.version, dv.buildNumber)
				continue
			}

			installedVer := "<none>"
			if found {
				installedVer = fmt.Sprintf("%s-b%d", pkg.version, pkg.buildNumber)
			}
			log.Printf("drift-reconciler: node=%s pkg=%s desired=%s-b%d installed=%s — dispatching",
				node.Identity.Hostname, name, dv.version, dv.buildNumber, installedVer)

			r.markInflight(inflightKey)

			// Capture loop variables for goroutine.
			nID, endpoint := nodeID, node.AgentEndpoint
			pkgName, pkgKind := name, kind
			version, buildNumber := dv.version, dv.buildNumber

			go func() {
				r.sem <- struct{}{}        // acquire
				defer func() { <-r.sem }() // release
				defer r.clearInflight(inflightKey)

				rctx, cancel := context.WithTimeout(ctx, r.timeout)
				defer cancel()

				err := r.srv.remoteApplyPackageRelease(rctx, nID, endpoint,
					pkgName, pkgKind, version, "", repo.Address, buildNumber, false)
				if err != nil {
					log.Printf("drift-reconciler: apply failed node=%s pkg=%s: %v", nID, pkgName, err)
				} else {
					log.Printf("drift-reconciler: apply succeeded node=%s pkg=%s@%s", nID, pkgName, version)
				}
			}()
		}
	}
}

type installedInfo struct {
	version     string
	buildNumber int64
	status      string
}

func (r *driftReconciler) isInflight(key string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	_, ok := r.inflight[key]
	return ok
}

func (r *driftReconciler) markInflight(key string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.inflight[key] = time.Now()
}

func (r *driftReconciler) clearInflight(key string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.inflight, key)
}

// isArtifactPublished checks whether the artifact for the given service/version/build
// is in PUBLISHED state in the repository. Returns false if the artifact cannot be
// verified (repository unreachable, artifact not found, or not PUBLISHED).
func (r *driftReconciler) isArtifactPublished(ctx context.Context, serviceName, version string, buildNumber int64, repoAddr string) bool {
	resolver := &ReleaseResolver{RepositoryAddr: repoAddr}
	client, cc, err := resolver.dialRepositoryDirect(ctx, repoAddr)
	if err != nil {
		log.Printf("drift-reconciler: cannot dial repository for publish check pkg=%s: %v", serviceName, err)
		return false
	}
	defer cc.Close()

	authCtx := resolver.buildAuthContext(ctx)
	ref := &repositorypb.ArtifactRef{
		PublisherId: defaultPublisherID(),
		Name:        serviceName,
		Version:     version,
		Platform:    "linux_amd64",
		Kind:        repositorypb.ArtifactKind_SERVICE,
	}
	resp, err := client.GetArtifactManifest(authCtx, &repositorypb.GetArtifactManifestRequest{
		Ref:         ref,
		BuildNumber: buildNumber,
	})
	if err != nil {
		log.Printf("drift-reconciler: artifact publish check failed pkg=%s@%s-b%d: %v",
			serviceName, version, buildNumber, err)
		return false
	}
	manifest := resp.GetManifest()
	if manifest == nil {
		return false
	}
	ps := manifest.GetPublishState()
	// Only PUBLISHED is acceptable. UNSPECIFIED is no longer tolerated —
	// the migration promotes legacy manifests to PUBLISHED on first startup.
	return ps == repositorypb.PublishState_PUBLISHED
}

func (r *driftReconciler) expireStale() {
	r.mu.Lock()
	defer r.mu.Unlock()
	cutoff := time.Now().Add(-r.inflightTTL)
	for key, t := range r.inflight {
		if t.Before(cutoff) {
			delete(r.inflight, key)
		}
	}
}
