// @awareness namespace=globular.platform
// @awareness component=platform_ai_memory
// @awareness file_role=persistent_ai_knowledge_store_not_cluster_authority
// @awareness implements=globular.platform:intent.ai.memory.outcome_history_not_cluster_authority
// @awareness implements=globular.platform:intent.ai.supplementary_not_required
// @awareness risk=low
//
// Package main implements the AI Memory gRPC service backed by ScyllaDB.
// It provides cluster-scoped, persistent memory for AI agents.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/globulario/services/golang/ai_memory/ai_memorypb"
	"github.com/globulario/services/golang/ai_memory/behavioral/domain"
	"github.com/globulario/services/golang/ai_memory/behavioral/store"
	cluster_operator "github.com/globulario/services/golang/ai_memory/domains/cluster_operator"
	"github.com/globulario/services/golang/config"
	globular "github.com/globulario/services/golang/globular_service"
	"github.com/globulario/services/golang/resource/resourcepb"
	"github.com/gocql/gocql"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"
)

// Version information (set via ldflags during build).
var (
	BuildTime = "unknown"
	GitCommit = "unknown"
)

var (
	defaultPort  = 10200
	defaultProxy = 10201
)

var logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))

// clusterIDWarnOnce guards a one-time warning when srv.Domain is used as
// cluster identity and looks like it could vary (e.g. contains "localhost"
// or is an IP address). Known gap: Domain is not the canonical cluster ID.
var clusterIDWarnOnce sync.Once

// -----------------------------------------------------------------------------
// Service definition
// -----------------------------------------------------------------------------

type server struct {
	// Globular service metadata
	Id                 string
	Mac                string
	Name               string
	Path               string
	Proto              string
	Port               int
	Proxy              int
	AllowAllOrigins    bool
	AllowedOrigins     string
	Protocol           string
	Domain             string
	Address            string
	Description        string
	Keywords           []string
	Repositories       []string
	Discoveries        []string
	State              string
	CertFile           string
	KeyFile            string
	CertAuthorityTrust string
	TLS                bool
	Version            string
	PublisherID        string
	KeepUpToDate       bool
	Checksum           string
	Plaform            string // kept for API compatibility
	KeepAlive          bool
	Permissions        []interface{}
	Dependencies       []string
	Process            int
	ProxyProcess       int
	ConfigPath         string
	LastError          string
	ModTime            int64

	// HostedServices lists additional gRPC service names served on this same
	// process/port beyond Name. The xDS route builder generates one gateway
	// route per (Name + HostedServices) entry, all pointing at this service's
	// cluster. Without this, a co-hosted service (e.g.
	// behavioral_memory.BehavioralMemoryService, registered on the same gRPC
	// server) has no gateway route and its requests fall through to the HTML
	// catch-all (HTTP 200 text/html) even though the backend serves it.
	HostedServices []string

	// gRPC
	grpcServer *grpc.Server

	// ScyllaDB
	ScyllaHosts          []string // e.g. ["127.0.0.1"]
	ScyllaPort           int      // default 9042
	ScyllaReplicationFactor int   // default 1
	session              *gocql.Session
}

// -----------------------------------------------------------------------------
// Globular service contract (getters/setters)
// -----------------------------------------------------------------------------

func (srv *server) GetConfigurationPath() string        { return srv.ConfigPath }
func (srv *server) SetConfigurationPath(path string)    { srv.ConfigPath = path }
func (srv *server) GetAddress() string                  { return srv.Address }
func (srv *server) SetAddress(address string)           { srv.Address = address }
func (srv *server) GetProcess() int                     { return srv.Process }
func (srv *server) SetProcess(pid int) {
	if pid == -1 {
		srv.closeScylla()
	}
	srv.Process = pid
}
func (srv *server) GetProxyProcess() int                { return srv.ProxyProcess }
func (srv *server) SetProxyProcess(pid int)             { srv.ProxyProcess = pid }
func (srv *server) GetState() string                    { return srv.State }
func (srv *server) SetState(state string)               { srv.State = state }
func (srv *server) GetLastError() string                { return srv.LastError }
func (srv *server) SetLastError(err string)             { srv.LastError = err }
func (srv *server) SetModTime(modtime int64)            { srv.ModTime = modtime }
func (srv *server) GetModTime() int64                   { return srv.ModTime }
func (srv *server) GetId() string                       { return srv.Id }
func (srv *server) SetId(id string)                     { srv.Id = id }
func (srv *server) GetName() string                     { return srv.Name }
func (srv *server) SetName(name string)                 { srv.Name = name }
func (srv *server) GetMac() string                      { return srv.Mac }
func (srv *server) SetMac(mac string)                   { srv.Mac = mac }
func (srv *server) GetDescription() string              { return srv.Description }
func (srv *server) SetDescription(description string)   { srv.Description = description }
func (srv *server) GetKeywords() []string               { return srv.Keywords }
func (srv *server) SetKeywords(keywords []string)       { srv.Keywords = keywords }
func (srv *server) Dist(path string) (string, error)    { return globular.Dist(path, srv) }
func (srv *server) GetDependencies() []string {
	if srv.Dependencies == nil {
		srv.Dependencies = []string{}
	}
	return srv.Dependencies
}
func (srv *server) SetDependency(dep string) {
	if srv.Dependencies == nil {
		srv.Dependencies = []string{}
	}
	for _, d := range srv.Dependencies {
		if d == dep {
			return
		}
	}
	srv.Dependencies = append(srv.Dependencies, dep)
}
func (srv *server) GetChecksum() string                      { return srv.Checksum }
func (srv *server) SetChecksum(checksum string)              { srv.Checksum = checksum }
func (srv *server) GetPlatform() string                      { return srv.Plaform }
func (srv *server) SetPlatform(platform string)              { srv.Plaform = platform }
func (srv *server) GetRepositories() []string                { return srv.Repositories }
func (srv *server) SetRepositories(v []string)               { srv.Repositories = v }
func (srv *server) GetDiscoveries() []string                 { return srv.Discoveries }
func (srv *server) SetDiscoveries(v []string)                { srv.Discoveries = v }
func (srv *server) GetPath() string                          { return srv.Path }
func (srv *server) SetPath(path string)                      { srv.Path = path }
func (srv *server) GetProto() string                         { return srv.Proto }
func (srv *server) SetProto(proto string)                    { srv.Proto = proto }
func (srv *server) GetPort() int                             { return srv.Port }
func (srv *server) SetPort(port int)                         { srv.Port = port }
func (srv *server) GetProxy() int                            { return srv.Proxy }
func (srv *server) SetProxy(proxy int)                       { srv.Proxy = proxy }
func (srv *server) GetProtocol() string                      { return srv.Protocol }
func (srv *server) SetProtocol(protocol string)              { srv.Protocol = protocol }
func (srv *server) GetAllowAllOrigins() bool                 { return srv.AllowAllOrigins }
func (srv *server) SetAllowAllOrigins(b bool)                { srv.AllowAllOrigins = b }
func (srv *server) GetAllowedOrigins() string                { return srv.AllowedOrigins }
func (srv *server) SetAllowedOrigins(s string)               { srv.AllowedOrigins = s }
func (srv *server) GetDomain() string                        { return srv.Domain }
func (srv *server) SetDomain(domain string)                  { srv.Domain = domain }
func (srv *server) GetTls() bool                             { return srv.TLS }
func (srv *server) SetTls(hasTls bool)                       { srv.TLS = hasTls }
func (srv *server) GetCertAuthorityTrust() string            { return srv.CertAuthorityTrust }
func (srv *server) SetCertAuthorityTrust(ca string)          { srv.CertAuthorityTrust = ca }
func (srv *server) GetCertFile() string                      { return srv.CertFile }
func (srv *server) SetCertFile(certFile string)              { srv.CertFile = certFile }
func (srv *server) GetKeyFile() string                       { return srv.KeyFile }
func (srv *server) SetKeyFile(keyFile string)                { srv.KeyFile = keyFile }
func (srv *server) GetVersion() string                       { return srv.Version }
func (srv *server) SetVersion(version string)                { srv.Version = version }
func (srv *server) GetPublisherID() string                   { return srv.PublisherID }
func (srv *server) SetPublisherID(p string)                  { srv.PublisherID = p }
func (srv *server) GetKeepUpToDate() bool                    { return srv.KeepUpToDate }
func (srv *server) SetKeepUptoDate(val bool)                 { srv.KeepUpToDate = val }
func (srv *server) GetKeepAlive() bool                       { return srv.KeepAlive }
func (srv *server) SetKeepAlive(val bool)                    { srv.KeepAlive = val }
func (srv *server) GetPermissions() []interface{}            { return srv.Permissions }
func (srv *server) SetPermissions(permissions []interface{}) { srv.Permissions = permissions }
func (srv *server) GetGrpcServer() *grpc.Server              { return srv.grpcServer }

func (srv *server) RolesDefault() []resourcepb.Role {
	return []resourcepb.Role{}
}

// -----------------------------------------------------------------------------
// ScyllaDB connection
// -----------------------------------------------------------------------------

// connectScylla initialises the ScyllaDB connection and ensures the schema is
// up to date. Schema DDL is run under an etcd-backed distributed mutex so
// that multiple nodes starting simultaneously cannot race on schema creation.
func (srv *server) connectScylla() error {
	if srv.session != nil {
		return nil
	}

	hosts := srv.ScyllaHosts
	if len(hosts) == 0 {
		return fmt.Errorf("scylla connect: no hosts configured (resolve from etcd first)")
	}

	// Run schema DDL under etcd coordination — only one node applies DDL at a time.
	ctx, cancel := context.WithTimeout(context.Background(), migrationTimeout+30*time.Second)
	defer cancel()

	if err := srv.runSchemaWithCoordination(ctx); err != nil {
		return fmt.Errorf("scylla schema: %w", err)
	}

	// Behavioral-memory keyspace migrates under its OWN etcd lock, independent of
	// the ai_memory keyspace above. Failure here is NON-FATAL: behavioral-memory
	// is a supplementary surface, and the established AiMemoryService (memory
	// CRUD + sessions, plus consumers like ai_watcher and the MCP memory tools)
	// must not be taken down by a behavioral-only schema problem ("AI is
	// supplementary, never required"). On failure we log and continue; the
	// behavioral RPCs then degrade (they error until the keyspace exists) while
	// AiMemoryService keeps working. The DDL is idempotent, so a later restart or
	// a node that wins the migration lock will create the tables.
	if err := srv.runBehavioralSchemaWithCoordination(ctx); err != nil {
		logger.Error("behavioral_memory schema unavailable — behavioral RPCs will degrade; AiMemoryService unaffected", "err", err)
	}

	// Connect with keyspace set for normal operation.
	port := srv.ScyllaPort
	if port == 0 {
		port = 9042
	}
	rf := len(hosts)
	if rf > 3 {
		rf = 3
	}
	consistency := gocql.Quorum
	if rf < 2 {
		consistency = gocql.One
	}

	cluster := gocql.NewCluster(hosts...)
	cluster.Port = port
	cluster.Consistency = consistency
	cluster.Timeout = 10 * time.Second
	cluster.ConnectTimeout = 10 * time.Second
	cluster.Keyspace = keyspace

	var err error
	srv.session, err = cluster.CreateSession()
	if err != nil {
		return fmt.Errorf("scylla connect (keyspace): %w", err)
	}

	logger.Info("ScyllaDB connected", "hosts", hosts, "keyspace", keyspace)
	return nil
}

// applySchema runs all schema DDL statements against ScyllaDB.
// All statements use IF NOT EXISTS and are safe to re-run (idempotent).
// Called only by the node that wins the migration coordinator lock.
func (srv *server) applySchema(_ context.Context) error {
	hosts := srv.ScyllaHosts
	port := srv.ScyllaPort
	if port == 0 {
		port = 9042
	}
	rf := len(hosts)
	if rf > 3 {
		rf = 3
	}
	consistency := gocql.Quorum
	if rf < 2 {
		consistency = gocql.One
	}

	// Connect without keyspace to run CREATE KEYSPACE.
	cluster := gocql.NewCluster(hosts...)
	cluster.Port = port
	cluster.Consistency = consistency
	cluster.Timeout = 10 * time.Second
	cluster.ConnectTimeout = 10 * time.Second

	session, err := cluster.CreateSession()
	if err != nil {
		return fmt.Errorf("scylla connect (schema): %w", err)
	}
	defer session.Close()

	cql := fmt.Sprintf(
		`CREATE KEYSPACE IF NOT EXISTS %s WITH replication = {'class': 'SimpleStrategy', 'replication_factor': %d}`,
		keyspace, rf,
	)
	if err := session.Query(cql).Exec(); err != nil {
		return fmt.Errorf("create keyspace: %w", err)
	}

	for _, stmt := range []string{createMemoriesTableCQL, createSessionsTableCQL, createTagsIndexCQL} {
		if err := session.Query(stmt).Exec(); err != nil {
			return fmt.Errorf("schema DDL: %w", err)
		}
	}
	return nil
}

func (srv *server) closeScylla() {
	if srv.session != nil {
		srv.session.Close()
		srv.session = nil
	}
}

// -----------------------------------------------------------------------------
// Lifecycle
// -----------------------------------------------------------------------------

func (srv *server) Init() error {
	if err := globular.InitService(srv); err != nil {
		return err
	}

	gs, err := globular.InitGrpcServer(srv)
	if err != nil {
		return err
	}
	srv.grpcServer = gs

	// Scylla hosts MUST come from etcd (Tier-0 cluster key). The service config
	// in etcd may contain stale/incorrect hosts from a previous boot; the cluster
	// key is the sole source of truth for infrastructure addresses.
	if hosts, err := config.GetScyllaHosts(); err == nil && len(hosts) > 0 {
		srv.ScyllaHosts = hosts
	} else {
		return fmt.Errorf("scylla hosts unavailable (etcd key %s): %w",
			"/globular/cluster/scylla/hosts", err)
	}

	if err := srv.connectScylla(); err != nil {
		return fmt.Errorf("init ScyllaDB: %w", err)
	}

	return nil
}

func (srv *server) Save() error { return globular.SaveService(srv) }

func (srv *server) StartService() error {
	return globular.StartService(srv, srv.grpcServer)
}

func (srv *server) StopService() error {
	return globular.StopService(srv, srv.grpcServer)
}

// -----------------------------------------------------------------------------
// gRPC Handlers
// -----------------------------------------------------------------------------

func (srv *server) Store(ctx context.Context, req *ai_memorypb.StoreRqst) (*ai_memorypb.StoreRsp, error) {
	m := req.GetMemory()
	if m == nil {
		return nil, fmt.Errorf("memory is required")
	}

	id := m.GetId()
	if id == "" {
		id = gocql.TimeUUID().String()
	}

	project := m.GetProject()
	if project == "" {
		return nil, fmt.Errorf("project is required")
	}

	// If the caller supplied an explicit id, ScyllaDB INSERT will upsert
	// against any existing row with the same partition key. Block that
	// from silently overwriting a protected seed entry — only the seed
	// admin may rewrite seed rows. Storing with a server-allocated UUID
	// (id == "" on the request) cannot collide, so we skip the check.
	if req.GetMemory().GetId() != "" && !authorizedSeedMutator(ctx) {
		existing, err := srv.getInternal(ctx, &ai_memorypb.GetRqst{Id: id, Project: project})
		if err != nil {
			// gocql.ErrNotFound means row doesn't exist — safe to INSERT.
			// Any other error (connection failure, timeout) must be surfaced
			// so the caller doesn't silently overwrite a seed entry.
			if !errors.Is(err, gocql.ErrNotFound) {
				return nil, fmt.Errorf("store: pre-check existing row: %w", err)
			}
		} else {
			if err := guardSeedMutation(ctx, existing.GetMemory(), "store"); err != nil {
				return nil, err
			}
		}
	}

	now := time.Now().Unix()
	if m.GetCreatedAt() == 0 {
		m.CreatedAt = now
	}
	m.UpdatedAt = now

	// Known gap: uses srv.Domain as cluster identity, not the canonical
	// cluster ID from etcd. See startup warning in main().
	clusterID := srv.Domain

	memType := memoryTypeToString(m.GetType())
	tags := m.GetTags()

	// Convert proto metadata map.
	metadata := m.GetMetadata()
	relatedIDs := m.GetRelatedIds()
	refCount := int(m.GetReferenceCount())

	cql := `INSERT INTO memories (id, project, type, tags, title, content, created_at, updated_at, agent_id, conversation_id, cluster_id, metadata, related_ids, reference_count) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	args := []interface{}{
		id, project, memType, tags, m.GetTitle(), m.GetContent(),
		m.GetCreatedAt(), m.GetUpdatedAt(), m.GetAgentId(), m.GetConversationId(), clusterID,
		metadata, relatedIDs, refCount,
	}

	q := srv.session.Query(cql, args...)
	if ttl := m.GetTtlSeconds(); ttl > 0 {
		q = q.DefaultTimestamp(false)
		q = srv.session.Query(
			`INSERT INTO memories (id, project, type, tags, title, content, created_at, updated_at, agent_id, conversation_id, cluster_id, metadata, related_ids, reference_count) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?) USING TTL ?`,
			append(args, int(ttl))...,
		)
	}

	if err := q.Exec(); err != nil {
		return nil, fmt.Errorf("store memory: %w", err)
	}

	logger.Info("memory stored", "id", id, "project", project, "type", memType, "title", m.GetTitle())
	return &ai_memorypb.StoreRsp{Id: id}, nil
}

// memoryMatchesAllTerms reports whether an entry's title+content contains EVERY
// term (AND semantics; substring per term). lowerTerms must already be
// lowercased (strings.Fields(strings.ToLower(query))). An empty term list
// matches everything. This replaced a whole-query strings.Contains check that
// treated the entire text_search string as one contiguous substring, so any
// multi-word query returned zero results even when each term was present —
// silently masking the seeded ops.* corpus from semantic recall.
func memoryMatchesAllTerms(title, content string, lowerTerms []string) bool {
	if len(lowerTerms) == 0 {
		return true
	}
	haystack := strings.ToLower(title) + "\n" + strings.ToLower(content)
	for _, term := range lowerTerms {
		if !strings.Contains(haystack, term) {
			return false
		}
	}
	return true
}

func (srv *server) Query(ctx context.Context, req *ai_memorypb.QueryRqst) (*ai_memorypb.QueryRsp, error) {
	project := req.GetProject()
	if project == "" {
		return nil, fmt.Errorf("project is required")
	}

	limit := int(req.GetLimit())
	if limit <= 0 {
		limit = 20
	}

	// Build query dynamically based on filters.
	var conditions []string
	var args []interface{}

	conditions = append(conditions, "project = ?")
	args = append(args, project)

	if req.GetType() != ai_memorypb.MemoryType_MEMORY_UNSPECIFIED {
		conditions = append(conditions, "type = ?")
		args = append(args, memoryTypeToString(req.GetType()))
	}

	if len(req.GetTags()) > 0 {
		for _, tag := range req.GetTags() {
			conditions = append(conditions, "tags CONTAINS ?")
			args = append(args, tag)
		}
	}

	needsFiltering := false
	textSearch := req.GetTextSearch()
	if textSearch != "" {
		// Text search is done client-side after fetching; we over-fetch.
		needsFiltering = true
	}

	cql := "SELECT id, project, type, tags, title, content, created_at, updated_at, agent_id, conversation_id, cluster_id, metadata, related_ids, reference_count FROM memories WHERE " +
		strings.Join(conditions, " AND ")

	// Over-fetch if text search is needed, but cap to avoid unbounded scans.
	fetchLimit := limit
	if needsFiltering {
		fetchLimit = limit * 5
	}
	if fetchLimit > 500 {
		fetchLimit = 500
	}
	cql += fmt.Sprintf(" LIMIT %d", fetchLimit)

	if needsFiltering || len(req.GetTags()) > 0 {
		cql += " ALLOW FILTERING"
	}

	iter := srv.session.Query(cql, args...).Iter()

	var memories []*ai_memorypb.Memory
	var (
		id, proj, memType, title, content, agentID, convID, clusterID string
		tags, relatedIDs                                              []string
		createdAt, updatedAt                                          int64
		metadata                                                      map[string]string
		refCount                                                      int
	)

	// Tokenize the query: an entry matches when EVERY whitespace-separated term
	// appears somewhere in title+content (AND semantics). The previous code
	// matched the entire query as one contiguous substring, so any multi-word
	// query (e.g. "rbac sa superadmin") returned zero results even when each
	// term was present — silently masking the whole corpus from semantic recall.
	searchTerms := strings.Fields(strings.ToLower(textSearch))

	for iter.Scan(&id, &proj, &memType, &tags, &title, &content, &createdAt, &updatedAt, &agentID, &convID, &clusterID, &metadata, &relatedIDs, &refCount) {
		// Client-side text filter: every term must be present in title+content.
		if !memoryMatchesAllTerms(title, content, searchTerms) {
			continue
		}

		memories = append(memories, &ai_memorypb.Memory{
			Id:             id,
			Project:        proj,
			Type:           stringToMemoryType(memType),
			Tags:           tags,
			Title:          title,
			Content:        content,
			CreatedAt:      createdAt,
			UpdatedAt:      updatedAt,
			AgentId:        agentID,
			ConversationId: convID,
			ClusterId:      clusterID,
			Metadata:       metadata,
			RelatedIds:     relatedIDs,
			ReferenceCount: int32(refCount),
		})

		if len(memories) >= limit {
			break
		}

		// Reset slices for next scan.
		tags = nil
	}

	if err := iter.Close(); err != nil {
		return nil, fmt.Errorf("query memories: %w", err)
	}

	return &ai_memorypb.QueryRsp{
		Memories: memories,
		Total:    int32(len(memories)),
	}, nil
}

// getInternal fetches a memory entry without incrementing its reference count.
// Used by Update and Delete to avoid inflating ref counts on internal lookups,
// and by the seed mutation guard in Store.
func (srv *server) getInternal(_ context.Context, req *ai_memorypb.GetRqst) (*ai_memorypb.GetRsp, error) {
	id := req.GetId()
	project := req.GetProject()
	if id == "" || project == "" {
		return nil, fmt.Errorf("id and project are required")
	}

	// We need to scan across types since id alone doesn't identify the row.
	cql := `SELECT id, project, type, tags, title, content, created_at, updated_at, agent_id, conversation_id, cluster_id, metadata, related_ids, reference_count
		FROM memories WHERE project = ? AND id = ? ALLOW FILTERING`

	var (
		memID, proj, memType, title, content, agentID, convID, clusterID string
		tags, relatedIDs                                                  []string
		createdAt, updatedAt                                              int64
		metadata                                                          map[string]string
		refCount                                                          int
	)

	if err := srv.session.Query(cql, project, id).Scan(
		&memID, &proj, &memType, &tags, &title, &content,
		&createdAt, &updatedAt, &agentID, &convID, &clusterID,
		&metadata, &relatedIDs, &refCount,
	); err != nil {
		// Distinguish not-found from connection/transport errors.
		if errors.Is(err, gocql.ErrNotFound) {
			return nil, err // caller can check with errors.Is
		}
		return nil, fmt.Errorf("get memory %s: %w", id, err)
	}

	return &ai_memorypb.GetRsp{
		Memory: &ai_memorypb.Memory{
			Id:             memID,
			Project:        proj,
			Type:           stringToMemoryType(memType),
			Tags:           tags,
			Title:          title,
			Content:        content,
			CreatedAt:      createdAt,
			UpdatedAt:      updatedAt,
			AgentId:        agentID,
			ConversationId: convID,
			ClusterId:      clusterID,
			Metadata:       metadata,
			RelatedIds:     relatedIDs,
			ReferenceCount: int32(refCount),
		},
	}, nil
}

func (srv *server) Get(ctx context.Context, req *ai_memorypb.GetRqst) (*ai_memorypb.GetRsp, error) {
	rsp, err := srv.getInternal(ctx, req)
	if err != nil {
		// Map gocql.ErrNotFound → gRPC NotFound; other errors → Unavailable.
		if errors.Is(err, gocql.ErrNotFound) {
			return nil, status.Errorf(codes.NotFound, "memory %s not found", req.GetId())
		}
		return nil, status.Errorf(codes.Unavailable, "get memory %s: %v", req.GetId(), err)
	}

	// Increment reference count — this memory was accessed by a caller.
	refCtx, refCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer refCancel()
	m := rsp.GetMemory()
	if err := srv.session.Query(
		`UPDATE memories SET reference_count = ? WHERE project = ? AND type = ? AND created_at = ? AND id = ?`,
		m.GetReferenceCount()+1, m.GetProject(), memoryTypeToString(m.GetType()), m.GetCreatedAt(), m.GetId(),
	).WithContext(refCtx).Exec(); err != nil {
		logger.Warn("failed to increment reference_count", "id", m.GetId(), "err", err)
	}

	return rsp, nil
}

func (srv *server) Update(ctx context.Context, req *ai_memorypb.UpdateRqst) (*ai_memorypb.UpdateRsp, error) {
	m := req.GetMemory()
	if m == nil || m.GetId() == "" || m.GetProject() == "" {
		return nil, fmt.Errorf("memory with id and project is required")
	}

	// Fetch the existing memory to merge — use getInternal to avoid
	// inflating reference_count on internal lookups.
	existing, err := srv.getInternal(ctx, &ai_memorypb.GetRqst{Id: m.GetId(), Project: m.GetProject()})
	if err != nil {
		return nil, fmt.Errorf("update: fetch existing: %w", err)
	}

	// Reject mutations against protected operational-knowledge seed
	// entries unless the caller is the seed admin.
	if err := guardSeedMutation(ctx, existing.GetMemory(), "update"); err != nil {
		return nil, err
	}

	merged := existing.GetMemory()

	// Merge non-empty fields.
	if m.GetTitle() != "" {
		merged.Title = m.GetTitle()
	}
	if m.GetContent() != "" {
		merged.Content = m.GetContent()
	}
	if len(m.GetTags()) > 0 {
		merged.Tags = m.GetTags()
	}
	if len(m.GetMetadata()) > 0 {
		if merged.Metadata == nil {
			merged.Metadata = make(map[string]string)
		}
		for k, v := range m.GetMetadata() {
			merged.Metadata[k] = v
		}
	}
	if len(m.GetRelatedIds()) > 0 {
		// Append new related IDs, dedup.
		seen := make(map[string]bool)
		for _, rid := range merged.GetRelatedIds() {
			seen[rid] = true
		}
		for _, rid := range m.GetRelatedIds() {
			if !seen[rid] {
				merged.RelatedIds = append(merged.RelatedIds, rid)
			}
		}
	}
	merged.UpdatedAt = time.Now().Unix()

	// Re-insert (ScyllaDB upserts).
	cql := `INSERT INTO memories (id, project, type, tags, title, content, created_at, updated_at, agent_id, conversation_id, cluster_id, metadata, related_ids, reference_count) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	args := []interface{}{
		merged.GetId(), merged.GetProject(), memoryTypeToString(merged.GetType()),
		merged.GetTags(), merged.GetTitle(), merged.GetContent(),
		merged.GetCreatedAt(), merged.GetUpdatedAt(), merged.GetAgentId(),
		merged.GetConversationId(), merged.GetClusterId(),
		merged.GetMetadata(), merged.GetRelatedIds(), int(merged.GetReferenceCount()),
	}

	if ttl := m.GetTtlSeconds(); ttl > 0 {
		cql = `INSERT INTO memories (id, project, type, tags, title, content, created_at, updated_at, agent_id, conversation_id, cluster_id, metadata, related_ids, reference_count) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?) USING TTL ?`
		args = append(args, int(ttl))
	}

	if err := srv.session.Query(cql, args...).Exec(); err != nil {
		return nil, fmt.Errorf("update memory: %w", err)
	}

	logger.Info("memory updated", "id", m.GetId(), "project", m.GetProject())
	return &ai_memorypb.UpdateRsp{Success: true}, nil
}

func (srv *server) Delete(ctx context.Context, req *ai_memorypb.DeleteRqst) (*ai_memorypb.DeleteRsp, error) {
	id := req.GetId()
	project := req.GetProject()
	if id == "" || project == "" {
		return nil, fmt.Errorf("id and project are required")
	}

	// Need to know type and created_at to delete from the clustered table.
	// Use getInternal to avoid inflating reference_count on internal lookups.
	existing, err := srv.getInternal(ctx, &ai_memorypb.GetRqst{Id: id, Project: project})
	if err != nil {
		return nil, fmt.Errorf("delete: fetch existing: %w", err)
	}

	// Reject deletes against protected operational-knowledge seed
	// entries unless the caller is the seed admin.
	if err := guardSeedMutation(ctx, existing.GetMemory(), "delete"); err != nil {
		return nil, err
	}

	m := existing.GetMemory()
	cql := `DELETE FROM memories WHERE project = ? AND type = ? AND created_at = ? AND id = ?`
	if err := srv.session.Query(cql, project, memoryTypeToString(m.GetType()), m.GetCreatedAt(), id).Exec(); err != nil {
		return nil, fmt.Errorf("delete memory: %w", err)
	}

	logger.Info("memory deleted", "id", id, "project", project)
	return &ai_memorypb.DeleteRsp{Success: true}, nil
}

func (srv *server) List(ctx context.Context, req *ai_memorypb.ListRqst) (*ai_memorypb.ListRsp, error) {
	project := req.GetProject()
	if project == "" {
		return nil, fmt.Errorf("project is required")
	}

	limit := int(req.GetLimit())
	if limit <= 0 {
		limit = 20
	}

	var conditions []string
	var args []interface{}

	conditions = append(conditions, "project = ?")
	args = append(args, project)

	if req.GetType() != ai_memorypb.MemoryType_MEMORY_UNSPECIFIED {
		conditions = append(conditions, "type = ?")
		args = append(args, memoryTypeToString(req.GetType()))
	}

	if len(req.GetTags()) > 0 {
		for _, tag := range req.GetTags() {
			conditions = append(conditions, "tags CONTAINS ?")
			args = append(args, tag)
		}
	}

	cql := "SELECT id, type, tags, title, created_at, updated_at, agent_id FROM memories WHERE " +
		strings.Join(conditions, " AND ")

	cql += fmt.Sprintf(" LIMIT %d", limit)
	if len(req.GetTags()) > 0 {
		cql += " ALLOW FILTERING"
	}

	iter := srv.session.Query(cql, args...).Iter()

	var summaries []*ai_memorypb.MemorySummary
	var (
		id, memType, title, agentID string
		tags                        []string
		createdAt, updatedAt        int64
	)

	for iter.Scan(&id, &memType, &tags, &title, &createdAt, &updatedAt, &agentID) {
		summaries = append(summaries, &ai_memorypb.MemorySummary{
			Id:        id,
			Type:      stringToMemoryType(memType),
			Tags:      tags,
			Title:     title,
			CreatedAt: createdAt,
			UpdatedAt: updatedAt,
			AgentId:   agentID,
		})
		tags = nil
	}

	if err := iter.Close(); err != nil {
		return nil, fmt.Errorf("list memories: %w", err)
	}

	return &ai_memorypb.ListRsp{
		Memories: summaries,
		Total:    int32(len(summaries)),
	}, nil
}

func (srv *server) SaveSession(ctx context.Context, req *ai_memorypb.SaveSessionRqst) (*ai_memorypb.SaveSessionRsp, error) {
	s := req.GetSession()
	if s == nil {
		return nil, fmt.Errorf("session is required")
	}

	id := s.GetId()
	if id == "" {
		id = gocql.TimeUUID().String()
	}

	project := s.GetProject()
	if project == "" {
		return nil, fmt.Errorf("project is required")
	}

	now := time.Now().Unix()
	if s.GetCreatedAt() == 0 {
		s.CreatedAt = now
	}

	clusterID := srv.Domain

	cql := `INSERT INTO sessions (id, project, topic, summary, decisions, open_questions, related_memories, created_at, agent_id, cluster_id) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	if err := srv.session.Query(cql,
		id, project, s.GetTopic(), s.GetSummary(),
		s.GetDecisions(), s.GetOpenQuestions(), s.GetRelatedMemories(),
		s.GetCreatedAt(), s.GetAgentId(), clusterID,
	).Exec(); err != nil {
		return nil, fmt.Errorf("save session: %w", err)
	}

	logger.Info("session saved", "id", id, "project", project, "topic", s.GetTopic())
	return &ai_memorypb.SaveSessionRsp{Id: id}, nil
}

func (srv *server) ResumeSession(ctx context.Context, req *ai_memorypb.ResumeSessionRqst) (*ai_memorypb.ResumeSessionRsp, error) {
	project := req.GetProject()
	if project == "" {
		return nil, fmt.Errorf("project is required")
	}

	limit := int(req.GetLimit())
	if limit <= 0 {
		limit = 1
	}

	topic := req.GetTopic()
	// Over-fetch for client-side topic matching, but cap to avoid unbounded scans.
	fetchLimit := limit * 10
	if fetchLimit < 20 {
		fetchLimit = 20
	}
	if fetchLimit > 500 {
		fetchLimit = 500
	}

	cql := fmt.Sprintf(
		`SELECT id, project, topic, summary, decisions, open_questions, related_memories, created_at, agent_id, cluster_id FROM sessions WHERE project = ? LIMIT %d`,
		fetchLimit,
	)

	iter := srv.session.Query(cql, project).Iter()

	var sessions []*ai_memorypb.Session
	var (
		id, proj, sTopic, summary, agentID, clusterID string
		decisions, openQuestions, relatedMemories      []string
		createdAt                                      int64
	)

	lowerTopic := strings.ToLower(topic)

	for iter.Scan(&id, &proj, &sTopic, &summary, &decisions, &openQuestions, &relatedMemories, &createdAt, &agentID, &clusterID) {
		// Fuzzy match on topic and summary.
		if topic != "" {
			if !strings.Contains(strings.ToLower(sTopic), lowerTopic) &&
				!strings.Contains(strings.ToLower(summary), lowerTopic) {
				continue
			}
		}

		sessions = append(sessions, &ai_memorypb.Session{
			Id:              id,
			Project:         proj,
			Topic:           sTopic,
			Summary:         summary,
			Decisions:       decisions,
			OpenQuestions:    openQuestions,
			RelatedMemories: relatedMemories,
			CreatedAt:       createdAt,
			AgentId:         agentID,
			ClusterId:       clusterID,
		})

		if len(sessions) >= limit {
			break
		}
	}

	if err := iter.Close(); err != nil {
		return nil, fmt.Errorf("resume session: %w", err)
	}

	return &ai_memorypb.ResumeSessionRsp{Sessions: sessions}, nil
}

// Stop handles the gRPC Stop endpoint.
func (srv *server) Stop(ctx context.Context, req *ai_memorypb.StopRequest) (*ai_memorypb.StopResponse, error) {
	return &ai_memorypb.StopResponse{}, srv.StopService()
}

// -----------------------------------------------------------------------------
// Type conversion helpers
// -----------------------------------------------------------------------------

func memoryTypeToString(t ai_memorypb.MemoryType) string {
	switch t {
	case ai_memorypb.MemoryType_FEEDBACK:
		return "feedback"
	case ai_memorypb.MemoryType_ARCHITECTURE:
		return "architecture"
	case ai_memorypb.MemoryType_DECISION:
		return "decision"
	case ai_memorypb.MemoryType_DEBUG:
		return "debug"
	case ai_memorypb.MemoryType_SESSION:
		return "session"
	case ai_memorypb.MemoryType_USER:
		return "user"
	case ai_memorypb.MemoryType_PROJECT:
		return "project"
	case ai_memorypb.MemoryType_REFERENCE:
		return "reference"
	case ai_memorypb.MemoryType_SCRATCH:
		return "scratch"
	case ai_memorypb.MemoryType_SKILL:
		return "skill"
	default:
		return "unspecified"
	}
}

func stringToMemoryType(s string) ai_memorypb.MemoryType {
	switch strings.ToLower(s) {
	case "feedback":
		return ai_memorypb.MemoryType_FEEDBACK
	case "architecture":
		return ai_memorypb.MemoryType_ARCHITECTURE
	case "decision":
		return ai_memorypb.MemoryType_DECISION
	case "debug":
		return ai_memorypb.MemoryType_DEBUG
	case "session":
		return ai_memorypb.MemoryType_SESSION
	case "user":
		return ai_memorypb.MemoryType_USER
	case "project":
		return ai_memorypb.MemoryType_PROJECT
	case "reference":
		return ai_memorypb.MemoryType_REFERENCE
	case "scratch":
		return ai_memorypb.MemoryType_SCRATCH
	case "skill":
		return ai_memorypb.MemoryType_SKILL
	default:
		return ai_memorypb.MemoryType_MEMORY_UNSPECIFIED
	}
}

// -----------------------------------------------------------------------------
// Main
// -----------------------------------------------------------------------------

func initializeServerDefaults() *server {
	return &server{
		Name:            "ai_memory.AiMemoryService",
		Proto:           "ai_memory.proto",
		Path:            func() string { p, _ := filepath.Abs(filepath.Dir(os.Args[0])); return p }(),
		Port:            defaultPort,
		Proxy:           defaultProxy,
		Protocol:        "grpc",
		Version:         Version,
		PublisherID:     "localhost",
		Description:     "AI memory service — cluster-scoped persistent knowledge for AI agents",
		Keywords:        []string{"ai", "memory", "conversation", "knowledge", "context", "scylladb"},
		AllowAllOrigins: true,
		KeepAlive:       true,
		KeepUpToDate:    true,
		Process:         -1,
		ProxyProcess:    -1,
		Repositories:    make([]string, 0),
		Discoveries:     make([]string, 0),
		Dependencies:    []string{"persistence.PersistenceService"},
		// behavioral_memory.BehavioralMemoryService is registered on the same
		// gRPC server (see behavioral_handlers.go). Declare it so xDS builds a
		// gateway route for it; otherwise its requests hit the HTML catch-all.
		HostedServices:  []string{"behavioral_memory.BehavioralMemoryService"},
		Permissions:     make([]interface{}, 0),
		ScyllaHosts:     nil, // resolved from etcd at Init() — never hardcode
		ScyllaPort:      9042,
		ScyllaReplicationFactor: 1,
	}
}

func setupGrpcService(s *server) {
	ai_memorypb.RegisterAiMemoryServiceServer(s.grpcServer, s)
	// Behavioral-memory: a second, protocol-shaped gRPC service hosted in the
	// same binary. PR-2 wires the Scylla-backed store (shared session) for the
	// ingestion-half RPCs; the rest stay dark. Does not touch AiMemoryService.
	var behavioralBackend store.Store = store.Unconfigured{}
	if s.session != nil {
		behavioralBackend = store.NewScyllaStore(s.session)
	}
	registerBehavioralService(s.grpcServer, behavioralBackend)
	// Load the cluster_operator domain pack's catalogs + proposed principles into
	// the store (idempotent, non-destructive, NEVER auto-promotes). Best-effort:
	// the seed is supplementary, so a failure logs and the service continues.
	loadBehavioralSeed(behavioralBackend)
	// Self-seed the flat operational-knowledge recall into the memories table.
	// Auth-free (writes the local store directly), so unlike the day-0/day-1 CLI
	// seed it does not depend on auth timing and self-heals on every restart.
	s.loadOpsKnowledgeRecallSeed()
	reflection.Register(s.grpcServer)
}

// recallSeedProject is the project under which flat operational-knowledge recall
// memories are stored — matches the `ops-knowledge seed` CLI so the two paths
// converge on the same rows.
const recallSeedProject = "globular-services"

// loadOpsKnowledgeRecallSeed self-seeds the embedded operational-knowledge recall
// (compiled from docs/operational-knowledge) into the memories table. It is
// idempotent — entries whose seed_sha256 is unchanged are skipped, changed
// entries are replaced (the PK ((project), type, created_at, id) clusters on
// created_at, so a changed entry is deleted then re-inserted to avoid a
// duplicate row). Strictly best-effort: any failure logs and the service
// continues (the seed is supplementary, never required).
func (srv *server) loadOpsKnowledgeRecallSeed() {
	if srv.session == nil {
		return
	}
	entries, err := cluster_operator.RecallSeedEntries()
	if err != nil {
		logger.Warn("ops-knowledge recall seed: load failed (non-fatal)", "err", err)
		return
	}
	if len(entries) == 0 {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	now := time.Now().Unix()
	stored, skipped, failed := 0, 0, 0
	for _, e := range entries {
		var exType string
		var exCreated int64
		exMeta := map[string]string{}
		scanErr := srv.session.Query(
			`SELECT type, created_at, metadata FROM memories WHERE project = ? AND id = ? LIMIT 1 ALLOW FILTERING`,
			recallSeedProject, e.ID).WithContext(ctx).Scan(&exType, &exCreated, &exMeta)
		exists := scanErr == nil
		if exists && exMeta["seed_sha256"] == e.SeedSHA256 {
			skipped++
			continue
		}
		tags := append([]string{}, e.Tags...)
		if !containsRecallTag(tags, "seed") {
			tags = append(tags, "seed")
		}
		meta := map[string]string{
			"source":      "seed",
			"immutable":   "true",
			"seed_sha256": e.SeedSHA256,
		}
		// Replace a changed row (created_at is part of the clustering key, so a
		// plain insert with a new created_at would leave a duplicate behind).
		if exists {
			_ = srv.session.Query(
				`DELETE FROM memories WHERE project = ? AND type = ? AND created_at = ? AND id = ?`,
				recallSeedProject, exType, exCreated, e.ID).WithContext(ctx).Exec()
		}
		if err := srv.session.Query(
			`INSERT INTO memories (id, project, type, tags, title, content, created_at, updated_at, agent_id, conversation_id, cluster_id, metadata, related_ids, reference_count) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			e.ID, recallSeedProject, recallMemType(e.Type), tags, e.Title, e.Content,
			now, now, "ops-knowledge-seeder", "", "", meta, e.RelatedIDs, 0,
		).WithContext(ctx).Exec(); err != nil {
			logger.Debug("ops-knowledge recall seed: upsert failed", "id", e.ID, "err", err)
			failed++
			continue
		}
		stored++
	}
	logger.Info("ops-knowledge recall seed loaded",
		"stored", stored, "skipped", skipped, "failed", failed, "total", len(entries))
}

// recallMemType maps a recall entry type to the canonical stored type string,
// matching the ops-knowledge CLI (defaults to REFERENCE for an unknown type).
func recallMemType(t string) string {
	if v, ok := ai_memorypb.MemoryType_value[t]; ok {
		return memoryTypeToString(ai_memorypb.MemoryType(v))
	}
	return memoryTypeToString(ai_memorypb.MemoryType_REFERENCE)
}

func containsRecallTag(tags []string, want string) bool {
	for _, t := range tags {
		if t == want {
			return true
		}
	}
	return false
}

// loadBehavioralSeed loads the cluster_operator domain pack into the behavioral
// store under the canonical seed project. Guarded — never fatal.
func loadBehavioralSeed(st store.Store) {
	if _, ok := st.(store.Unconfigured); ok {
		return // no persistence backend wired
	}
	pack, err := cluster_operator.New()
	if err != nil {
		logger.Error("behavioral seed: cluster_operator pack invalid", "err", err)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	res, err := domain.LoadCatalogs(ctx, st, behavioralSeedProject, pack)
	if err != nil {
		logger.Warn("behavioral seed: cluster_operator load failed (non-fatal)", "err", err)
		return
	}
	logger.Info("behavioral seed: cluster_operator loaded",
		"authorities", res.Authorities, "conditions", res.Conditions,
		"principles_seeded", res.PrinciplesSeeded, "principles_skipped", res.PrinciplesSkipped)
}

func main() {
	srv := initializeServerDefaults()

	var (
		enableDebug  = flag.Bool("debug", false, "enable debug logging")
		showVersion  = flag.Bool("version", false, "print version information as JSON and exit")
		showHelp     = flag.Bool("help", false, "show usage information and exit")
		showDescribe = flag.Bool("describe", false, "print service description as JSON and exit")
		showHealth   = flag.Bool("health", false, "print service health status as JSON and exit")
	)

	flag.Usage = printUsage
	flag.Parse()

	if *enableDebug {
		logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
	}

	if *showHelp {
		printUsage()
		return
	}
	if *showVersion {
		printVersion()
		return
	}
	if *showDescribe {
		globular.HandleDescribeFlag(srv, logger)
		return
	}
	if *showHealth {
		health := map[string]interface{}{
			"service": srv.Name,
			"status":  "healthy",
			"version": srv.Version,
		}
		data, _ := json.MarshalIndent(health, "", "  ")
		fmt.Println(string(data))
		return
	}

	args := flag.Args()
	if err := globular.AllocatePortIfNeeded(srv, args); err != nil {
		logger.Error("port allocation failed", "error", err)
		os.Exit(1)
	}

	globular.ParsePositionalArgs(srv, args)
	globular.LoadRuntimeConfig(srv)

	logger.Info("starting ai_memory service", "service", srv.Name, "version", srv.Version, "domain", srv.Domain)

	start := time.Now()
	if err := srv.Init(); err != nil {
		logger.Error("init failed", "service", srv.Name, "id", srv.Id, "err", err)
		os.Exit(1)
	}
	logger.Info("service initialized", "duration_ms", time.Since(start).Milliseconds())

	// Known gap: clusterID is derived from srv.Domain, not the canonical
	// cluster identity from etcd. Log a one-time warning if Domain looks
	// like it could vary across nodes (e.g. "localhost", IP address, empty).
	clusterIDWarnOnce.Do(func() {
		d := srv.Domain
		if d == "" || d == "localhost" || strings.HasPrefix(d, "127.") || strings.HasPrefix(d, "10.") || strings.HasPrefix(d, "192.168.") {
			logger.Warn("cluster_id derived from Domain, which may vary across nodes — not canonical cluster identity",
				"domain", d)
		}
	})

	setupGrpcService(srv)

	logger.Info("service ready",
		"service", srv.Name,
		"version", srv.Version,
		"port", srv.Port,
		"domain", srv.Domain,
		"startup_ms", time.Since(start).Milliseconds(),
	)

	lm := globular.NewLifecycleManager(srv, logger)
	if err := lm.Start(); err != nil {
		logger.Error("service start failed", "service", srv.Name, "id", srv.Id, "err", err)
		os.Exit(1)
	}
}

func printUsage() {
	exe := filepath.Base(os.Args[0])
	fmt.Fprintf(os.Stdout, `%s - AI Memory Service

Cluster-scoped persistent memory for AI agents, backed by ScyllaDB.
Stores knowledge, decisions, debugging insights, and session context
that persists across conversations, machines, and agent instances.

USAGE:
  %s [OPTIONS] [<id>] [<configPath>]

OPTIONS:
  --debug       Enable debug logging
  --version     Print version information as JSON and exit
  --help        Show this help message and exit
  --describe    Print service description as JSON and exit
  --health      Print health status as JSON and exit

ScyllaDB hosts are resolved from etcd at startup — no environment variables.

EXAMPLES:
  %s --version
  %s --debug

`, exe, exe, exe, exe)
}

func printVersion() {
	data := map[string]string{
		"service":    "ai_memory",
		"version":    Version,
		"build_time": BuildTime,
		"git_commit": GitCommit,
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(data)
}
