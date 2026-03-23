// Package main implements the AI Memory gRPC service backed by ScyllaDB.
// It provides cluster-scoped, persistent memory for AI agents.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/globulario/services/golang/ai_memory/ai_memorypb"
	"github.com/globulario/services/golang/config"
	globular "github.com/globulario/services/golang/globular_service"
	"github.com/globulario/services/golang/resource/resourcepb"
	"github.com/gocql/gocql"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

// Version information (set via ldflags during build).
var (
	Version   = "0.0.1"
	BuildTime = "unknown"
	GitCommit = "unknown"
)

var (
	defaultPort  = 10200
	defaultProxy = 10201
)

var logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))

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

func (srv *server) connectScylla() error {
	if srv.session != nil {
		return nil
	}

	hosts := srv.ScyllaHosts
	if len(hosts) == 0 {
		hosts = []string{"127.0.0.1"}
	}
	port := srv.ScyllaPort
	if port == 0 {
		port = 9042
	}
	rf := srv.ScyllaReplicationFactor
	if rf == 0 {
		rf = 1
	}

	// Connect without keyspace first to create keyspace + tables.
	cluster := gocql.NewCluster(hosts...)
	cluster.Port = port
	cluster.Consistency = gocql.Quorum
	cluster.Timeout = 10 * time.Second
	cluster.ConnectTimeout = 10 * time.Second

	session, err := cluster.CreateSession()
	if err != nil {
		return fmt.Errorf("scylla connect: %w", err)
	}

	// Create keyspace with configured replication factor.
	cql := fmt.Sprintf(
		`CREATE KEYSPACE IF NOT EXISTS %s WITH replication = {'class': 'SimpleStrategy', 'replication_factor': %d}`,
		keyspace, rf,
	)
	if err := session.Query(cql).Exec(); err != nil {
		session.Close()
		return fmt.Errorf("create keyspace: %w", err)
	}

	// Create tables and indexes.
	for _, stmt := range []string{createMemoriesTableCQL, createSessionsTableCQL, createTagsIndexCQL} {
		if err := session.Query(stmt).Exec(); err != nil {
			session.Close()
			return fmt.Errorf("schema init: %w", err)
		}
	}
	session.Close()

	// Reconnect with keyspace set.
	cluster.Keyspace = keyspace
	srv.session, err = cluster.CreateSession()
	if err != nil {
		return fmt.Errorf("scylla connect (keyspace): %w", err)
	}

	logger.Info("ScyllaDB connected", "hosts", hosts, "keyspace", keyspace)
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

	// Read ScyllaDB hosts from environment or use defaults.
	if h := os.Getenv("SCYLLA_HOSTS"); h != "" {
		srv.ScyllaHosts = strings.Split(h, ",")
	}
	if len(srv.ScyllaHosts) == 0 || (len(srv.ScyllaHosts) == 1 && srv.ScyllaHosts[0] == "127.0.0.1") {
		// Try to get from persistence service config (ScyllaDB connection).
		if cfg, err := config.GetServiceConfigurationById("persistence.PersistenceService"); err == nil {
			for _, key := range []string{"Host", "host", "Address", "address"} {
				if host, ok := cfg[key].(string); ok && host != "" {
					srv.ScyllaHosts = []string{host}
					break
				}
			}
		}
	}
	if len(srv.ScyllaHosts) == 0 || (len(srv.ScyllaHosts) == 1 && srv.ScyllaHosts[0] == "127.0.0.1") {
		// Fall back to the service's own advertise address (node IP).
		if srv.Address != "" {
			host := srv.Address
			if h, _, ok := strings.Cut(host, ":"); ok {
				host = h
			}
			if host != "" && host != "127.0.0.1" && host != "localhost" {
				srv.ScyllaHosts = []string{host}
			}
		}
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

	now := time.Now().Unix()
	if m.GetCreatedAt() == 0 {
		m.CreatedAt = now
	}
	m.UpdatedAt = now

	// Use domain as cluster identity.
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

	// Over-fetch if text search is needed.
	fetchLimit := limit
	if needsFiltering {
		fetchLimit = limit * 5
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

	lowerSearch := strings.ToLower(textSearch)

	for iter.Scan(&id, &proj, &memType, &tags, &title, &content, &createdAt, &updatedAt, &agentID, &convID, &clusterID, &metadata, &relatedIDs, &refCount) {
		// Client-side text filter.
		if textSearch != "" {
			if !strings.Contains(strings.ToLower(title), lowerSearch) &&
				!strings.Contains(strings.ToLower(content), lowerSearch) {
				continue
			}
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

func (srv *server) Get(ctx context.Context, req *ai_memorypb.GetRqst) (*ai_memorypb.GetRsp, error) {
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
		return nil, fmt.Errorf("get memory %s: %w", id, err)
	}

	// Increment reference count — this memory was accessed.
	go func() {
		_ = srv.session.Query(
			`UPDATE memories SET reference_count = ? WHERE project = ? AND type = ? AND created_at = ? AND id = ?`,
			refCount+1, proj, memType, createdAt, memID,
		).Exec()
	}()

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

func (srv *server) Update(ctx context.Context, req *ai_memorypb.UpdateRqst) (*ai_memorypb.UpdateRsp, error) {
	m := req.GetMemory()
	if m == nil || m.GetId() == "" || m.GetProject() == "" {
		return nil, fmt.Errorf("memory with id and project is required")
	}

	// First, fetch the existing memory to merge.
	existing, err := srv.Get(ctx, &ai_memorypb.GetRqst{Id: m.GetId(), Project: m.GetProject()})
	if err != nil {
		return nil, fmt.Errorf("update: fetch existing: %w", err)
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
	existing, err := srv.Get(ctx, &ai_memorypb.GetRqst{Id: id, Project: project})
	if err != nil {
		return nil, fmt.Errorf("delete: fetch existing: %w", err)
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
	// Over-fetch for client-side topic matching.
	fetchLimit := limit * 10
	if fetchLimit < 20 {
		fetchLimit = 20
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
		Permissions:     make([]interface{}, 0),
		ScyllaHosts:     []string{"127.0.0.1"},
		ScyllaPort:      9042,
		ScyllaReplicationFactor: 1,
	}
}

func setupGrpcService(s *server) {
	ai_memorypb.RegisterAiMemoryServiceServer(s.grpcServer, s)
	reflection.Register(s.grpcServer)
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

ENVIRONMENT:
  SCYLLA_HOSTS  Comma-separated ScyllaDB hosts (default: 127.0.0.1)

EXAMPLES:
  %s --version
  %s --debug
  SCYLLA_HOSTS=10.0.0.10,10.0.0.11 %s

`, exe, exe, exe, exe, exe)
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
