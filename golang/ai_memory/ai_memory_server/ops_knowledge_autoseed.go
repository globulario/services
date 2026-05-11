// Day-1 auto-seeding of operational-knowledge into ai-memory.
//
// On a fresh cluster, ai-memory comes online before anyone has run
// `globular ops-knowledge seed` — yet AI agents that consume ai-memory
// expect a baseline of Day-0/Day-1 knowledge to already be there. Rather
// than depend on an external workflow or operator action, ai-memory
// self-populates from the operational-knowledge payload that ships
// inside the awareness bundle (see docs/operational-knowledge/).
//
// The bundle is symlinked at /var/lib/globular/awareness/current/ on
// every node by the node-agent's awareness-bundle install loop. If the
// symlink resolves and contains an ops-knowledge/ tree, this seeder
// runs. Otherwise it stays silent — the bundle may not have been
// installed yet (pre-Day-1, or pre-bundle-feature builds), and other
// rules surface bundle absence.
//
// Idempotency: each entry's row in scylla carries metadata.seed_sha256.
// The seeder skips entries whose stored hash already matches the
// canonical hash of the on-disk YAML, so re-running is cheap and safe.
//
// Auth: this writes directly to scylla, bypassing the gRPC handlers
// that would normally enforce the immutability layer. That is correct
// here — the immutability layer protects against EXTERNAL callers
// rewriting seed entries; the seeder IS the authority that establishes
// what the seed entries are. There is no service principal to
// authenticate against.
package main

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"github.com/globulario/services/golang/ai_memory/ai_memorypb"
	"github.com/globulario/services/golang/opsknowledge"
)

// opsKnowledgeBundlePath is where node-agent symlinks the active
// awareness bundle. The seed payload lives under <path>/ops-knowledge/.
// var (not const) so tests can swap it.
var opsKnowledgeBundlePath = "/var/lib/globular/awareness/current"

const (
	// opsKnowledgeProject is the project key under which seed entries
	// are stored — must match what `globular ops-knowledge seed` uses.
	opsKnowledgeProject = "globular-services"

	// opsKnowledgeSeedTag is the tag every seed entry carries (in
	// addition to its own tags), used by `ops-knowledge list` to
	// scope queries.
	opsKnowledgeSeedTag = "seed"

	// opsKnowledgeSeederAgent is the agent_id stamped on rows the
	// auto-seeder writes; matches the CLI seeder for consistency so
	// queries on agent_id collapse both paths to one identity.
	opsKnowledgeSeederAgent = "ops-knowledge-seeder"
)

// setBundlePathForTest is a tiny shim used only by tests to swap
// opsKnowledgeBundlePath without exposing the variable's mutability
// to production callers.
func setBundlePathForTest(path string) { opsKnowledgeBundlePath = path }

// startOpsKnowledgeAutoSeed runs one pass at startup (after scylla is
// connected) and then re-checks every hour. The hourly cadence catches
// bundle updates pushed by node-agent without requiring a service
// restart.
func (srv *server) startOpsKnowledgeAutoSeed(ctx context.Context) {
	go func() {
		// Initial pass — best-effort, do not block service startup.
		srv.runOpsKnowledgeAutoSeed(ctx)
		t := time.NewTicker(1 * time.Hour)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				srv.runOpsKnowledgeAutoSeed(ctx)
			}
		}
	}()
}

// opsKnowledgeAutoSeedResult is logged after each pass for observability.
type opsKnowledgeAutoSeedResult struct {
	Skipped int
	Stored  int
	Updated int
	Failed  int
	Total   int
}

func (srv *server) runOpsKnowledgeAutoSeed(ctx context.Context) opsKnowledgeAutoSeedResult {
	var res opsKnowledgeAutoSeedResult

	if srv.session == nil {
		logger.Debug("ops-knowledge autoseed: scylla session not ready, skipping")
		return res
	}

	opsDir := filepath.Join(opsKnowledgeBundlePath, "ops-knowledge")
	if _, err := os.Stat(opsDir); err != nil {
		// Bundle not installed yet (pre-Day-1) or built without
		// ops-knowledge — silent skip; doctor surfaces bundle issues.
		return res
	}

	files, err := opsknowledge.LoadDir(opsDir)
	if err != nil {
		logger.Warn("ops-knowledge autoseed: load failed", "dir", opsDir, "err", err)
		return res
	}

	// Determine the seed_version stamp from the bundle's manifest.json
	// when readable; fall back to "auto-seeded" so the value is never
	// empty. This is informational only — the canonical hash is what
	// the doctor uses for drift detection.
	seedVersion := readBundleSeedVersion()

	for _, f := range files {
		for _, e := range f.Entries {
			res.Total++
			hash, err := opsknowledge.HashEntry(e)
			if err != nil {
				res.Failed++
				logger.Warn("ops-knowledge autoseed: hash failed", "id", e.ID, "err", err)
				continue
			}
			outcome, err := srv.upsertOpsKnowledgeEntry(ctx, e, hash, seedVersion)
			if err != nil {
				res.Failed++
				logger.Warn("ops-knowledge autoseed: upsert failed", "id", e.ID, "err", err)
				continue
			}
			switch outcome {
			case "skipped":
				res.Skipped++
			case "stored":
				res.Stored++
			case "updated":
				res.Updated++
			}
		}
	}

	if res.Stored > 0 || res.Updated > 0 || res.Failed > 0 {
		logger.Info("ops-knowledge autoseed pass complete",
			"total", res.Total, "stored", res.Stored, "updated", res.Updated,
			"skipped", res.Skipped, "failed", res.Failed,
			"seed_version", seedVersion)
	}
	return res
}

// upsertOpsKnowledgeEntry writes one seed entry to scylla via the
// internal Store/Update path so the immutability layer's bypass for
// id-less Stores does not apply (we always pass the explicit id).
//
// Returns one of "skipped" | "stored" | "updated".
func (srv *server) upsertOpsKnowledgeEntry(ctx context.Context, e opsknowledge.Entry, hash, seedVersion string) (string, error) {
	mem := opsKnowledgeEntryToMemory(e, hash, seedVersion, srv.Domain)

	// Read current row directly via gocql to compare the stored hash.
	// We cannot call srv.Get() because its auth interceptors reject
	// internal callers without a JWT/mTLS context. Using session.Query
	// keeps this an in-process write.
	var (
		storedMetadata map[string]string
		exists         bool
	)
	iter := srv.session.Query(
		`SELECT metadata FROM memories WHERE project = ? AND id = ? ALLOW FILTERING`,
		opsKnowledgeProject, e.ID,
	).WithContext(ctx).Iter()
	if iter.Scan(&storedMetadata) {
		exists = true
	}
	if err := iter.Close(); err != nil {
		return "", err
	}

	if exists && storedMetadata != nil && storedMetadata["seed_sha256"] == hash {
		return "skipped", nil
	}

	cql := `INSERT INTO memories (id, project, type, tags, title, content, created_at, updated_at, agent_id, conversation_id, cluster_id, metadata, related_ids, reference_count) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	now := time.Now().Unix()
	if mem.GetCreatedAt() == 0 {
		mem.CreatedAt = now
	}
	mem.UpdatedAt = now

	args := []interface{}{
		mem.GetId(), mem.GetProject(), memoryTypeToString(mem.GetType()),
		mem.GetTags(), mem.GetTitle(), mem.GetContent(),
		mem.GetCreatedAt(), mem.GetUpdatedAt(), mem.GetAgentId(),
		mem.GetConversationId(), mem.GetClusterId(),
		mem.GetMetadata(), mem.GetRelatedIds(), int(mem.GetReferenceCount()),
	}
	if err := srv.session.Query(cql, args...).WithContext(ctx).Exec(); err != nil {
		return "", err
	}
	if exists {
		return "updated", nil
	}
	return "stored", nil
}

// opsKnowledgeEntryToMemory builds a Memory from an opsknowledge.Entry,
// applying the same field mapping the CLI seeder uses so both paths
// produce identical rows.
func opsKnowledgeEntryToMemory(e opsknowledge.Entry, hash, seedVersion, clusterID string) *ai_memorypb.Memory {
	memType := ai_memorypb.MemoryType_REFERENCE
	if v, ok := ai_memorypb.MemoryType_value[e.Type]; ok {
		memType = ai_memorypb.MemoryType(v)
	}
	tags := append([]string{}, e.Tags...)
	if !containsString(tags, opsKnowledgeSeedTag) {
		tags = append(tags, opsKnowledgeSeedTag)
	}
	return &ai_memorypb.Memory{
		Id:         e.ID,
		Project:    opsKnowledgeProject,
		Type:       memType,
		Tags:       tags,
		Title:      e.Title,
		Content:    e.Content,
		AgentId:    opsKnowledgeSeederAgent,
		ClusterId:  clusterID,
		RelatedIds: append([]string{}, e.RelatedIDs...),
		Metadata: map[string]string{
			"source":       opsknowledge.ProvenanceSourceSeed,
			"seed_version": seedVersion,
			"seed_sha256":  hash,
			"immutable":    "true",
		},
	}
}

func containsString(haystack []string, needle string) bool {
	for _, s := range haystack {
		if s == needle {
			return true
		}
	}
	return false
}
