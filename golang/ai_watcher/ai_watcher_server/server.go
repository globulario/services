// Package main implements the AI Watcher service — a lightweight event-driven
// daemon that subscribes to cluster events, filters them through configurable
// rules, and triggers AI-assisted diagnosis and remediation.
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
	"sync"
	"time"

	"github.com/globulario/services/golang/ai_watcher/ai_watcherpb"
	"github.com/globulario/services/golang/event/event_client"
	"github.com/globulario/services/golang/event/eventpb"
	globular "github.com/globulario/services/golang/globular_service"
	"github.com/globulario/services/golang/resource/resourcepb"
	"github.com/gocql/gocql"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

var (
	Version   = "0.0.1"
	BuildTime = "unknown"
	GitCommit = "unknown"
)

var (
	defaultPort  = 10210
	defaultProxy = 10211
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
	Plaform            string
	KeepAlive          bool
	Permissions        []interface{}
	Dependencies       []string
	Process            int
	ProxyProcess       int
	ConfigPath         string
	LastError          string
	ModTime            int64

	grpcServer *grpc.Server

	// Watcher state
	config      *ai_watcherpb.WatcherConfig
	configMu    sync.RWMutex
	eventClient *event_client.Event_Client

	// Runtime stats
	stats      watcherStats
	statsMu    sync.Mutex
	startedAt  time.Time

	// Incident tracking
	incidents   map[string]*ai_watcherpb.Incident
	incidentsMu sync.RWMutex

	// Cooldown tracking
	lastTrigger   map[string]time.Time // rule ID -> last trigger time
	lastTriggerMu sync.Mutex

	// Batch window
	eventBatch   map[string][]string // rule ID -> batched event names
	eventBatchMu sync.Mutex
	batchTimers  map[string]*time.Timer
}

type watcherStats struct {
	EventsReceived   int64
	EventsFiltered   int64
	IncidentsCreated int64
	AutoRemediations int64
	ApprovalsPending int64
	LastEventAt      time.Time
	LastIncidentAt   time.Time
}

// -----------------------------------------------------------------------------
// Globular service contract (getters/setters)
// -----------------------------------------------------------------------------

func (srv *server) GetConfigurationPath() string        { return srv.ConfigPath }
func (srv *server) SetConfigurationPath(path string)    { srv.ConfigPath = path }
func (srv *server) GetAddress() string                  { return srv.Address }
func (srv *server) SetAddress(address string)           { srv.Address = address }
func (srv *server) GetProcess() int                     { return srv.Process }
func (srv *server) SetProcess(pid int)                  { srv.Process = pid }
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

func (srv *server) RolesDefault() []resourcepb.Role { return []resourcepb.Role{} }

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

	// Initialize watcher state.
	srv.config = defaultWatcherConfig()
	srv.incidents = make(map[string]*ai_watcherpb.Incident)
	srv.lastTrigger = make(map[string]time.Time)
	srv.eventBatch = make(map[string][]string)
	srv.batchTimers = make(map[string]*time.Timer)
	srv.startedAt = time.Now()

	return nil
}

func (srv *server) Save() error { return globular.SaveService(srv) }

func (srv *server) StartService() error {
	// Start event subscription in background.
	go srv.eventLoop()

	return globular.StartService(srv, srv.grpcServer)
}

func (srv *server) StopService() error {
	return globular.StopService(srv, srv.grpcServer)
}

// -----------------------------------------------------------------------------
// Event Loop — the core of the watcher
// -----------------------------------------------------------------------------

func (srv *server) eventLoop() {
	// Wait for the service to be fully started before connecting to events.
	time.Sleep(5 * time.Second)

	for {
		srv.configMu.RLock()
		cfg := srv.config
		srv.configMu.RUnlock()

		if !cfg.GetEnabled() {
			time.Sleep(10 * time.Second)
			continue
		}

		// Connect to event service.
		eventAddr := srv.resolveEventEndpoint()
		client, err := event_client.NewEventService_Client(eventAddr, srv.Id)
		if err != nil {
			logger.Error("event service connection failed, retrying in 10s", "err", err)
			time.Sleep(10 * time.Second)
			continue
		}
		srv.eventClient = client

		logger.Info("connected to event service", "address", eventAddr)

		// Subscribe to configured topics.
		for _, topic := range cfg.GetSubscribeTopics() {
			subscriberID := fmt.Sprintf("watcher_%s_%s", srv.Id, topic)
			if err := client.Subscribe(topic, subscriberID, func(evt *eventpb.Event) {
				srv.handleEvent(evt)
			}); err != nil {
				logger.Error("subscribe failed", "topic", topic, "err", err)
				continue
			}
			logger.Info("subscribed to topic", "topic", topic)
		}

		// Block until connection lost.
		// The event client handles reconnection internally,
		// but we check periodically if config changed.
		for {
			time.Sleep(30 * time.Second)

			srv.configMu.RLock()
			paused := cfg.GetPaused()
			srv.configMu.RUnlock()

			if paused {
				logger.Info("watcher paused, waiting...")
			}
		}
	}
}

func (srv *server) resolveEventEndpoint() string {
	if ep := os.Getenv("GLOBULAR_EVENT_ENDPOINT"); ep != "" {
		return ep
	}
	return "localhost:10102"
}

// handleEvent processes a single event from the event service.
func (srv *server) handleEvent(evt *eventpb.Event) {
	srv.statsMu.Lock()
	srv.stats.EventsReceived++
	srv.stats.LastEventAt = time.Now()
	srv.statsMu.Unlock()

	srv.configMu.RLock()
	cfg := srv.config
	srv.configMu.RUnlock()

	if cfg.GetPaused() {
		return
	}

	eventName := evt.GetName()

	// Match against rules.
	for _, rule := range cfg.GetRules() {
		if !rule.GetEnabled() {
			continue
		}

		if !matchPattern(rule.GetEventPattern(), eventName) {
			continue
		}

		// Check cooldown.
		srv.lastTriggerMu.Lock()
		if last, ok := srv.lastTrigger[rule.GetId()]; ok {
			if time.Since(last) < time.Duration(rule.GetCooldownSeconds())*time.Second {
				srv.lastTriggerMu.Unlock()
				continue
			}
		}
		srv.lastTriggerMu.Unlock()

		srv.statsMu.Lock()
		srv.stats.EventsFiltered++
		srv.statsMu.Unlock()

		// Add to batch window.
		srv.addToBatch(rule, eventName, evt)
	}
}

// addToBatch collects events within the batch window before triggering.
func (srv *server) addToBatch(rule *ai_watcherpb.EventRule, eventName string, evt *eventpb.Event) {
	ruleID := rule.GetId()
	batchWindow := time.Duration(rule.GetBatchWindowSeconds()) * time.Second
	if batchWindow <= 0 {
		batchWindow = 10 * time.Second
	}

	srv.eventBatchMu.Lock()
	defer srv.eventBatchMu.Unlock()

	srv.eventBatch[ruleID] = append(srv.eventBatch[ruleID], eventName)

	// If this is the first event in the batch, start a timer.
	if _, exists := srv.batchTimers[ruleID]; !exists {
		srv.batchTimers[ruleID] = time.AfterFunc(batchWindow, func() {
			srv.fireBatch(rule)
		})
	}
}

// fireBatch triggers after the batch window expires.
func (srv *server) fireBatch(rule *ai_watcherpb.EventRule) {
	ruleID := rule.GetId()

	srv.eventBatchMu.Lock()
	events := srv.eventBatch[ruleID]
	delete(srv.eventBatch, ruleID)
	delete(srv.batchTimers, ruleID)
	srv.eventBatchMu.Unlock()

	if len(events) == 0 {
		return
	}

	// Check repeat threshold.
	if int32(len(events)) < rule.GetRepeatThreshold() {
		return
	}

	// Update cooldown.
	srv.lastTriggerMu.Lock()
	srv.lastTrigger[ruleID] = time.Now()
	srv.lastTriggerMu.Unlock()

	// Create incident.
	incident := &ai_watcherpb.Incident{
		Id:           gocql.TimeUUID().String(),
		TriggerEvent: events[0],
		EventBatch:   events,
		Status:       ai_watcherpb.IncidentStatus_INCIDENT_DETECTED,
		Tier:         rule.GetTier(),
		DetectedAt:   time.Now().Unix(),
		Metadata: map[string]string{
			"rule_id":     ruleID,
			"event_count": fmt.Sprintf("%d", len(events)),
		},
	}

	srv.incidentsMu.Lock()
	srv.incidents[incident.Id] = incident
	srv.incidentsMu.Unlock()

	srv.statsMu.Lock()
	srv.stats.IncidentsCreated++
	srv.stats.LastIncidentAt = time.Now()
	srv.statsMu.Unlock()

	logger.Info("incident created",
		"id", incident.Id,
		"rule", ruleID,
		"tier", rule.GetTier().String(),
		"events", len(events),
		"trigger", events[0],
	)

	// Process based on tier.
	go srv.processIncident(incident, rule)
}

// processIncident handles the incident according to its permission tier.
func (srv *server) processIncident(incident *ai_watcherpb.Incident, rule *ai_watcherpb.EventRule) {
	switch rule.GetTier() {
	case ai_watcherpb.PermissionTier_OBSERVE:
		// Tier 1: diagnose and record only.
		srv.updateIncidentStatus(incident.Id, ai_watcherpb.IncidentStatus_INCIDENT_DIAGNOSING)
		srv.diagnoseAndRecord(incident)

	case ai_watcherpb.PermissionTier_AUTO_REMEDIATE:
		// Tier 2: diagnose, check auto-remediation whitelist, act if approved.
		srv.updateIncidentStatus(incident.Id, ai_watcherpb.IncidentStatus_INCIDENT_DIAGNOSING)
		srv.diagnoseAndAutoRemediate(incident)

	case ai_watcherpb.PermissionTier_REQUIRE_APPROVAL:
		// Tier 3: diagnose, propose action, wait for approval.
		srv.updateIncidentStatus(incident.Id, ai_watcherpb.IncidentStatus_INCIDENT_DIAGNOSING)
		srv.diagnoseAndAwaitApproval(incident)
	}
}

// diagnoseAndRecord performs Tier 1 observation: gather context and store to memory.
func (srv *server) diagnoseAndRecord(incident *ai_watcherpb.Incident) {
	// TODO: Call MCP tools to gather diagnosis:
	//   - cluster_get_health
	//   - cluster_get_doctor_report
	//   - nodeagent_get_service_logs (for the affected service)
	//   - memory_query (for similar past incidents)
	//
	// Then store the diagnosis in ai_memory via memory_store.

	incident.Diagnosis = fmt.Sprintf("Observed %d events matching rule %s. Awaiting full MCP integration for automated diagnosis.",
		len(incident.EventBatch), incident.Metadata["rule_id"])

	srv.updateIncidentStatus(incident.Id, ai_watcherpb.IncidentStatus_INCIDENT_RESOLVED)
	logger.Info("incident recorded (observe only)", "id", incident.Id, "events", len(incident.EventBatch))
}

// diagnoseAndAutoRemediate performs Tier 2: diagnose + auto-fix if whitelisted.
func (srv *server) diagnoseAndAutoRemediate(incident *ai_watcherpb.Incident) {
	// TODO: Same diagnosis as Tier 1, plus:
	//   - Check auto_remediation whitelist for matching action
	//   - If whitelisted: execute remediation via governor
	//   - If not whitelisted: downgrade to Tier 1 (observe only)

	incident.Diagnosis = "Auto-remediation pending MCP integration"
	srv.updateIncidentStatus(incident.Id, ai_watcherpb.IncidentStatus_INCIDENT_RESOLVED)
	logger.Info("incident auto-remediated (placeholder)", "id", incident.Id)
}

// diagnoseAndAwaitApproval performs Tier 3: diagnose, propose, wait.
func (srv *server) diagnoseAndAwaitApproval(incident *ai_watcherpb.Incident) {
	// TODO: Same diagnosis as Tier 1, plus:
	//   - Formulate proposed action
	//   - Set status to AWAITING_APPROVAL
	//   - Publish notification event
	//   - Wait for ApproveAction/DenyAction RPC

	incident.Diagnosis = "Approval-gated remediation pending MCP integration"
	incident.ProposedAction = "Pending diagnosis"
	srv.updateIncidentStatus(incident.Id, ai_watcherpb.IncidentStatus_INCIDENT_AWAITING_APPROVAL)

	srv.statsMu.Lock()
	srv.stats.ApprovalsPending++
	srv.statsMu.Unlock()

	logger.Info("incident awaiting approval", "id", incident.Id)
}

func (srv *server) updateIncidentStatus(id string, status ai_watcherpb.IncidentStatus) {
	srv.incidentsMu.Lock()
	defer srv.incidentsMu.Unlock()
	if inc, ok := srv.incidents[id]; ok {
		inc.Status = status
		if status == ai_watcherpb.IncidentStatus_INCIDENT_RESOLVED ||
			status == ai_watcherpb.IncidentStatus_INCIDENT_FAILED {
			inc.ResolvedAt = time.Now().Unix()
		}
	}
}

// matchPattern checks if eventName matches a pattern (supports trailing "*" wildcard).
func matchPattern(pattern, eventName string) bool {
	if pattern == "*" {
		return true
	}
	if strings.HasSuffix(pattern, "*") {
		prefix := strings.TrimSuffix(pattern, "*")
		return strings.HasPrefix(eventName, prefix)
	}
	return pattern == eventName
}

// -----------------------------------------------------------------------------
// gRPC Handlers
// -----------------------------------------------------------------------------

func (srv *server) GetConfig(ctx context.Context, req *ai_watcherpb.GetConfigRqst) (*ai_watcherpb.GetConfigRsp, error) {
	srv.configMu.RLock()
	defer srv.configMu.RUnlock()
	return &ai_watcherpb.GetConfigRsp{Config: srv.config}, nil
}

func (srv *server) SetConfig(ctx context.Context, req *ai_watcherpb.SetConfigRqst) (*ai_watcherpb.SetConfigRsp, error) {
	srv.configMu.Lock()
	defer srv.configMu.Unlock()
	srv.config = req.GetConfig()
	logger.Info("watcher config updated")
	return &ai_watcherpb.SetConfigRsp{Success: true}, nil
}

func (srv *server) GetStatus(ctx context.Context, req *ai_watcherpb.GetStatusRqst) (*ai_watcherpb.GetStatusRsp, error) {
	srv.configMu.RLock()
	cfg := srv.config
	srv.configMu.RUnlock()

	srv.statsMu.Lock()
	stats := srv.stats
	srv.statsMu.Unlock()

	lastEvent := ""
	if !stats.LastEventAt.IsZero() {
		lastEvent = stats.LastEventAt.Format(time.RFC3339)
	}
	lastIncident := ""
	if !stats.LastIncidentAt.IsZero() {
		lastIncident = stats.LastIncidentAt.Format(time.RFC3339)
	}

	return &ai_watcherpb.GetStatusRsp{
		Running:          cfg.GetEnabled() && !cfg.GetPaused(),
		Paused:           cfg.GetPaused(),
		StartedAt:        srv.startedAt.Unix(),
		EventsReceived:   stats.EventsReceived,
		EventsFiltered:   stats.EventsFiltered,
		IncidentsCreated: stats.IncidentsCreated,
		AutoRemediations: stats.AutoRemediations,
		ApprovalsPending: stats.ApprovalsPending,
		LastEventAt:      lastEvent,
		LastIncidentAt:   lastIncident,
	}, nil
}

func (srv *server) Pause(ctx context.Context, req *ai_watcherpb.PauseRqst) (*ai_watcherpb.PauseRsp, error) {
	srv.configMu.Lock()
	srv.config.Paused = true
	srv.configMu.Unlock()
	logger.Info("watcher paused")
	return &ai_watcherpb.PauseRsp{Success: true}, nil
}

func (srv *server) Resume(ctx context.Context, req *ai_watcherpb.ResumeRqst) (*ai_watcherpb.ResumeRsp, error) {
	srv.configMu.Lock()
	srv.config.Paused = false
	srv.configMu.Unlock()
	logger.Info("watcher resumed")
	return &ai_watcherpb.ResumeRsp{Success: true}, nil
}

func (srv *server) GetIncidents(ctx context.Context, req *ai_watcherpb.GetIncidentsRqst) (*ai_watcherpb.GetIncidentsRsp, error) {
	srv.incidentsMu.RLock()
	defer srv.incidentsMu.RUnlock()

	limit := int(req.GetLimit())
	if limit <= 0 {
		limit = 20
	}

	var result []*ai_watcherpb.Incident
	for _, inc := range srv.incidents {
		if req.GetStatus() != ai_watcherpb.IncidentStatus_INCIDENT_DETECTED || inc.Status == req.GetStatus() {
			result = append(result, inc)
		}
		if len(result) >= limit {
			break
		}
	}

	return &ai_watcherpb.GetIncidentsRsp{
		Incidents: result,
		Total:     int32(len(result)),
	}, nil
}

func (srv *server) GetIncident(ctx context.Context, req *ai_watcherpb.GetIncidentRqst) (*ai_watcherpb.GetIncidentRsp, error) {
	srv.incidentsMu.RLock()
	defer srv.incidentsMu.RUnlock()

	inc, ok := srv.incidents[req.GetId()]
	if !ok {
		return nil, fmt.Errorf("incident %s not found", req.GetId())
	}
	return &ai_watcherpb.GetIncidentRsp{Incident: inc}, nil
}

func (srv *server) ApproveAction(ctx context.Context, req *ai_watcherpb.ApproveActionRqst) (*ai_watcherpb.ApproveActionRsp, error) {
	srv.incidentsMu.Lock()
	inc, ok := srv.incidents[req.GetIncidentId()]
	if !ok {
		srv.incidentsMu.Unlock()
		return nil, fmt.Errorf("incident %s not found", req.GetIncidentId())
	}
	if inc.Status != ai_watcherpb.IncidentStatus_INCIDENT_AWAITING_APPROVAL {
		srv.incidentsMu.Unlock()
		return nil, fmt.Errorf("incident %s is not awaiting approval (status: %s)", req.GetIncidentId(), inc.Status.String())
	}
	inc.Status = ai_watcherpb.IncidentStatus_INCIDENT_REMEDIATING
	inc.Metadata["approved_by"] = req.GetApprover()
	inc.Metadata["approved_at"] = time.Now().Format(time.RFC3339)
	srv.incidentsMu.Unlock()

	srv.statsMu.Lock()
	srv.stats.ApprovalsPending--
	srv.statsMu.Unlock()

	logger.Info("action approved", "incident", req.GetIncidentId(), "approver", req.GetApprover())

	// TODO: Execute the proposed remediation.
	return &ai_watcherpb.ApproveActionRsp{Success: true}, nil
}

func (srv *server) DenyAction(ctx context.Context, req *ai_watcherpb.DenyActionRqst) (*ai_watcherpb.DenyActionRsp, error) {
	srv.incidentsMu.Lock()
	inc, ok := srv.incidents[req.GetIncidentId()]
	if !ok {
		srv.incidentsMu.Unlock()
		return nil, fmt.Errorf("incident %s not found", req.GetIncidentId())
	}
	inc.Status = ai_watcherpb.IncidentStatus_INCIDENT_IGNORED
	inc.Metadata["denied_reason"] = req.GetReason()
	inc.ResolvedAt = time.Now().Unix()
	srv.incidentsMu.Unlock()

	srv.statsMu.Lock()
	if srv.stats.ApprovalsPending > 0 {
		srv.stats.ApprovalsPending--
	}
	srv.statsMu.Unlock()

	logger.Info("action denied", "incident", req.GetIncidentId(), "reason", req.GetReason())
	return &ai_watcherpb.DenyActionRsp{Success: true}, nil
}

func (srv *server) GetPendingApprovals(ctx context.Context, req *ai_watcherpb.GetPendingApprovalsRqst) (*ai_watcherpb.GetPendingApprovalsRsp, error) {
	srv.incidentsMu.RLock()
	defer srv.incidentsMu.RUnlock()

	var pending []*ai_watcherpb.Incident
	for _, inc := range srv.incidents {
		if inc.Status == ai_watcherpb.IncidentStatus_INCIDENT_AWAITING_APPROVAL {
			pending = append(pending, inc)
		}
	}
	return &ai_watcherpb.GetPendingApprovalsRsp{Pending: pending}, nil
}

func (srv *server) Stop(ctx context.Context, req *ai_watcherpb.StopRequest) (*ai_watcherpb.StopResponse, error) {
	return &ai_watcherpb.StopResponse{}, srv.StopService()
}

// -----------------------------------------------------------------------------
// Main
// -----------------------------------------------------------------------------

func initializeServerDefaults() *server {
	return &server{
		Name:            "ai_watcher.AiWatcherService",
		Proto:           "ai_watcher.proto",
		Path:            func() string { p, _ := filepath.Abs(filepath.Dir(os.Args[0])); return p }(),
		Port:            defaultPort,
		Proxy:           defaultProxy,
		Protocol:        "grpc",
		Version:         Version,
		PublisherID:     "localhost",
		Description:     "AI Watcher — event-driven autonomous cluster operations with tiered permissions",
		Keywords:        []string{"ai", "watcher", "events", "autonomous", "remediation", "operations"},
		AllowAllOrigins: true,
		KeepAlive:       true,
		KeepUpToDate:    true,
		Process:         -1,
		ProxyProcess:    -1,
		Repositories:    make([]string, 0),
		Discoveries:     make([]string, 0),
		Dependencies:    []string{"event.EventService", "ai_memory.AiMemoryService"},
		Permissions:     make([]interface{}, 0),
	}
}

func setupGrpcService(s *server) {
	ai_watcherpb.RegisterAiWatcherServiceServer(s.grpcServer, s)
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

	logger.Info("starting ai_watcher service", "service", srv.Name, "version", srv.Version)

	start := time.Now()
	if err := srv.Init(); err != nil {
		logger.Error("init failed", "service", srv.Name, "id", srv.Id, "err", err)
		os.Exit(1)
	}

	setupGrpcService(srv)

	logger.Info("service ready",
		"service", srv.Name,
		"version", srv.Version,
		"port", srv.Port,
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
	fmt.Fprintf(os.Stdout, `%s - AI Watcher Service

Event-driven autonomous cluster operations with tiered permissions.
Subscribes to cluster events, detects problems, and triggers AI-assisted
diagnosis and remediation.

Permission Tiers:
  Tier 1 (observe)       Always allowed — read logs, query doctor, store findings
  Tier 2 (auto-fix)      Pre-approved — restart services, clear corrupted storage
  Tier 3 (approval)      Hold and notify — config changes, desired state changes

USAGE:
  %s [OPTIONS] [<id>] [<configPath>]

OPTIONS:
  --debug       Enable debug logging
  --version     Print version information as JSON and exit
  --help        Show this help message and exit
  --describe    Print service description as JSON and exit
  --health      Print health status as JSON and exit

`, exe, exe)
}

func printVersion() {
	data := map[string]string{
		"service":    "ai_watcher",
		"version":    Version,
		"build_time": BuildTime,
		"git_commit": GitCommit,
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(data)
}
