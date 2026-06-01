// Package workflow provides a client-side recorder for emitting workflow
// runs, steps, artifacts, and events to the WorkflowService.
//
// Usage:
//
//	rec := workflow.NewRecorder("localhost:10220", "cluster-id")
//	defer rec.Close()
//
//	run, _ := rec.StartRun(ctx, &workflow.RunParams{...})
//	rec.RecordStep(ctx, run.Id, 1, &workflow.StepParams{...})
//	rec.FinishRun(ctx, run.Id, workflow.Succeeded, "all good", "", workflow.NoFailure)
package workflow

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/workflow/workflowpb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// Convenience aliases for enum values.
var (
	Succeeded  = workflowpb.RunStatus_RUN_STATUS_SUCCEEDED
	Failed     = workflowpb.RunStatus_RUN_STATUS_FAILED
	Executing  = workflowpb.RunStatus_RUN_STATUS_EXECUTING
	Pending    = workflowpb.RunStatus_RUN_STATUS_PENDING
	Blocked    = workflowpb.RunStatus_RUN_STATUS_BLOCKED
	RolledBack = workflowpb.RunStatus_RUN_STATUS_ROLLED_BACK

	StepRunning   = workflowpb.StepStatus_STEP_STATUS_RUNNING
	StepSucceeded = workflowpb.StepStatus_STEP_STATUS_SUCCEEDED
	StepFailed    = workflowpb.StepStatus_STEP_STATUS_FAILED
	StepSkipped   = workflowpb.StepStatus_STEP_STATUS_SKIPPED
	StepBlocked   = workflowpb.StepStatus_STEP_STATUS_BLOCKED

	NoFailure = workflowpb.FailureClass_FAILURE_CLASS_UNKNOWN

	ActorController = workflowpb.WorkflowActor_ACTOR_CLUSTER_CONTROLLER
	ActorNodeAgent  = workflowpb.WorkflowActor_ACTOR_NODE_AGENT
	ActorInstaller  = workflowpb.WorkflowActor_ACTOR_INSTALLER
	ActorRuntime    = workflowpb.WorkflowActor_ACTOR_RUNTIME
	ActorRepository = workflowpb.WorkflowActor_ACTOR_REPOSITORY

	PhaseDecision  = workflowpb.WorkflowPhaseKind_PHASE_DECISION
	PhaseFetch     = workflowpb.WorkflowPhaseKind_PHASE_FETCH
	PhaseInstall   = workflowpb.WorkflowPhaseKind_PHASE_INSTALL
	PhaseConfigure = workflowpb.WorkflowPhaseKind_PHASE_CONFIGURE
	PhaseStart     = workflowpb.WorkflowPhaseKind_PHASE_START
	PhaseVerify    = workflowpb.WorkflowPhaseKind_PHASE_VERIFY
	PhasePublish   = workflowpb.WorkflowPhaseKind_PHASE_PUBLISH

	KindInfra   = workflowpb.ComponentKind_COMPONENT_KIND_INFRASTRUCTURE
	KindService = workflowpb.ComponentKind_COMPONENT_KIND_SERVICE

	TriggerRepair    = workflowpb.TriggerReason_TRIGGER_REASON_REPAIR
	TriggerBootstrap = workflowpb.TriggerReason_TRIGGER_REASON_BOOTSTRAP
)

// RunParams holds the parameters for starting a workflow run.
type RunParams struct {
	NodeID           string
	NodeHostname     string
	ComponentName    string
	ComponentKind    workflowpb.ComponentKind
	ComponentVersion string
	ReleaseKind      string
	ReleaseObjectID  string
	TriggerReason    workflowpb.TriggerReason
	CorrelationID    string
	WorkflowName     string // workflow definition name (e.g. "day0.bootstrap", "node.repair")
}

// StepParams holds the parameters for recording a workflow step.
type StepParams struct {
	StepKey     string
	Title       string
	Actor       workflowpb.WorkflowActor
	Phase       workflowpb.WorkflowPhaseKind
	Status      workflowpb.StepStatus
	SourceActor workflowpb.WorkflowActor
	TargetActor workflowpb.WorkflowActor
	Message     string
	DetailsJSON string
}

// AddrResolver is a function that returns the gRPC address for the workflow
// service. It is called lazily on each connect attempt so the address can
// change at runtime (e.g. local port becomes available after install, or
// gateway is discovered after bootstrap).
type AddrResolver func() string

// Recorder is a fire-and-forget client for the WorkflowService.
// All methods log errors but never return them — the workflow trace
// must never block the reconciliation pipeline.
//
// The recorder connects lazily on first use and reconnects automatically
// if the connection is lost, following the same pattern as eventPublisher.
type Recorder struct {
	clusterID    string
	addrResolver AddrResolver
	client       workflowpb.WorkflowServiceClient
	conn         *grpc.ClientConn
	mu           sync.Mutex
	seqMap       map[string]int32 // run_id → next seq number
}

// Default certificate paths for Globular service mTLS.
var (
	DefaultCertFile  = "/var/lib/globular/pki/issued/services/service.crt"
	DefaultKeyFile   = "/var/lib/globular/pki/issued/services/service.key"
	DefaultCAFile    = "/var/lib/globular/pki/ca.crt"
	DefaultTokenFile = "/var/lib/globular/tokens/node_token"
)

// NewRecorder creates a lazy recorder that connects on first use.
// The addr parameter is used as a static address. For dynamic discovery
// (e.g. routing through Envoy gateway), use NewRecorderWithResolver instead.
func NewRecorder(addr, clusterID string) *Recorder {
	return NewRecorderWithResolver(func() string { return addr }, clusterID)
}

// NewRecorderWithResolver creates a lazy recorder that calls resolver()
// to obtain the workflow service address on each connection attempt.
// This allows the address to change at runtime — for example, the service
// may start locally after install, or be discovered via the gateway.
func NewRecorderWithResolver(resolver AddrResolver, clusterID string) *Recorder {
	return &Recorder{
		clusterID:    clusterID,
		addrResolver: resolver,
		seqMap:       make(map[string]int32),
	}
}

// ensureConnected lazily dials the workflow service. Returns true if a
// live client is available. On failure it logs and returns false — the
// caller should silently drop the event (fire-and-forget).
func (r *Recorder) ensureConnected() bool {
	if r == nil {
		return false
	}
	if r.client != nil {
		return true
	}

	rawAddr := ""
	if r.addrResolver != nil {
		rawAddr = r.addrResolver()
	}
	if rawAddr == "" {
		return false
	}

	dt := resolveRecorderTarget(rawAddr)

	creds, err := loadRecorderTLS(dt.serverName)
	if err != nil {
		// TLS not ready yet (certs may not be installed) — try again later.
		return false
	}

	token := loadNodeToken()

	dialOpts := []grpc.DialOption{
		grpc.WithTransportCredentials(creds),
		grpc.WithBlock(),
	}
	if token != "" {
		dialOpts = append(dialOpts, grpc.WithUnaryInterceptor(tokenInjector(token, r.clusterID)))
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, err := grpc.DialContext(ctx, dt.address, dialOpts...)
	if err != nil {
		log.Printf("workflow recorder: connect to %s failed: %v", dt.address, err)
		return false
	}
	r.conn = conn
	r.client = workflowpb.NewWorkflowServiceClient(conn)
	authMethod := "mTLS"
	if token != "" {
		authMethod = "mTLS+token"
	}
	log.Printf("workflow recorder: connected to %s (%s)", dt.address, authMethod)
	return true
}

// disconnect tears down the connection so ensureConnected will re-dial
// on the next call. Used after RPC failures.
func (r *Recorder) disconnect() {
	if r == nil {
		return
	}
	if r.conn != nil {
		r.conn.Close()
	}
	r.conn = nil
	r.client = nil
}

// loadNodeToken reads the node's identity token (JWT).
//
// Resolution order:
//  1. DefaultTokenFile (/var/lib/globular/tokens/node_token) — preferred
//     explicit handle used on Day-1 nodes — but only if the token is not
//     already expired.
//  2. MAC-matched filename: any non-loopback interface MAC on this host is
//     converted to "<mac>_token" and loaded. This prevents picking up a
//     token that was issued for a DIFFERENT node's MAC (which would cause
//     Unauthenticated on every cluster-internal call because audience
//     doesn't match this node).
//  3. Directory scan fallback: parse exp claims from every *_token file,
//     skip expired ones, and return the token with the latest exp. This
//     keeps Day-0 nodes working while preventing stale tokens from older
//     node identities from being preferred just because they sort first.
//
// Returns empty string if no usable token is found.
func loadNodeToken() string {
	// 1. Explicit Day-1 node token.
	if data, err := os.ReadFile(DefaultTokenFile); err == nil {
		if t := strings.TrimSpace(string(data)); t != "" && !jwtExpired(t) {
			return t
		}
	}

	dir := "/var/lib/globular/tokens"

	// 2. MAC-matched token for this node.
	for _, mac := range localNodeMACs() {
		candidate := dir + "/" + mac + "_token"
		if data, err := os.ReadFile(candidate); err == nil {
			if t := strings.TrimSpace(string(data)); t != "" && !jwtExpired(t) {
				return t
			}
		}
	}

	// 3. Fallback scan: pick the non-expired token with the latest exp.
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	var best string
	var bestExp int64
	now := time.Now().Unix()
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), "_token") {
			continue
		}
		data, err := os.ReadFile(dir + "/" + e.Name())
		if err != nil {
			continue
		}
		t := strings.TrimSpace(string(data))
		if t == "" {
			continue
		}
		exp, ok := jwtExp(t)
		if !ok {
			// Unparsable payload — skip; we can't risk picking an expired
			// one without knowing.
			continue
		}
		if exp <= now {
			continue
		}
		if exp > bestExp {
			bestExp = exp
			best = t
		}
	}
	return best
}

// localNodeMACs returns MAC addresses of this host's non-loopback network
// interfaces, formatted with underscores (e.g. "e0_d4_64_f0_86_f6") to match
// the token filename convention used by the PKI issuer.
func localNodeMACs() []string {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil
	}
	out := make([]string, 0, len(ifaces))
	for _, iface := range ifaces {
		if iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		mac := iface.HardwareAddr.String()
		if mac == "" {
			continue
		}
		out = append(out, strings.ReplaceAll(mac, ":", "_"))
	}
	return out
}

// jwtExp extracts the "exp" claim (Unix seconds) from an unverified JWT.
// Signature is NOT validated — we only read the payload to filter out
// tokens that have already expired. Returns (0, false) if the token is
// malformed or has no exp claim.
func jwtExp(tok string) (int64, bool) {
	parts := strings.Split(tok, ".")
	if len(parts) != 3 {
		return 0, false
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		// Some issuers emit with padding; try standard base64.
		payload, err = base64.StdEncoding.DecodeString(parts[1])
		if err != nil {
			return 0, false
		}
	}
	var claims struct {
		Exp int64 `json:"exp"`
	}
	if err := json.Unmarshal(payload, &claims); err != nil {
		return 0, false
	}
	if claims.Exp <= 0 {
		return 0, false
	}
	return claims.Exp, true
}

// jwtExpired reports whether the token's exp claim is in the past. Returns
// false when the claim cannot be parsed, so unknown-format tokens are still
// attempted — the server will make the final decision.
func jwtExpired(tok string) bool {
	exp, ok := jwtExp(tok)
	if !ok {
		return false
	}
	return time.Now().Unix() >= exp
}

// tokenInjector returns a gRPC unary interceptor that attaches the token
// and cluster_id as metadata on every outgoing call.
func tokenInjector(token, clusterID string) grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply interface{},
		cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		md, ok := metadata.FromOutgoingContext(ctx)
		if !ok {
			md = metadata.New(nil)
		} else {
			md = md.Copy()
		}
		md.Set("token", token)
		if clusterID != "" {
			md.Set("cluster_id", clusterID)
		}
		return invoker(metadata.NewOutgoingContext(ctx, md), method, req, reply, cc, opts...)
	}
}

// loadRecorderTLS loads the node's service certificates for mTLS.
func loadRecorderTLS(serverName string) (credentials.TransportCredentials, error) {
	cert, err := tls.LoadX509KeyPair(DefaultCertFile, DefaultKeyFile)
	if err != nil {
		return nil, fmt.Errorf("load client cert: %w", err)
	}

	caCert, err := os.ReadFile(DefaultCAFile)
	if err != nil {
		return nil, fmt.Errorf("read CA: %w", err)
	}

	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(caCert) {
		return nil, fmt.Errorf("failed to parse CA cert")
	}

	return credentials.NewTLS(&tls.Config{
		Certificates: []tls.Certificate{cert},
		RootCAs:      pool,
		ServerName:   serverName,
	}), nil
}

// recorderDialTarget mirrors config.ResolveDialTarget without importing config
// (the workflow package must not depend on config to avoid circular imports).
type recorderDialTarget struct {
	address    string
	serverName string
}

func resolveRecorderTarget(endpoint string) recorderDialTarget {
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		return recorderDialTarget{}
	}
	host, port, err := net.SplitHostPort(endpoint)
	if err != nil {
		// No port — treat entire string as host.
		host = endpoint
		port = ""
	}
	if host == "127.0.0.1" || host == "::1" || host == "localhost" {
		if routable := config.GetRoutableIPv4(); routable != "" {
			host = routable
		}
	}
	addr := host
	if port != "" {
		addr = net.JoinHostPort(host, port)
	}
	return recorderDialTarget{address: addr, serverName: host}
}

// Close releases the gRPC connection.
func (r *Recorder) Close() {
	if r != nil && r.conn != nil {
		r.conn.Close()
	}
}

// Available returns true if the recorder can connect to the workflow service.
func (r *Recorder) Available() bool {
	return r != nil && r.ensureConnected()
}

// StartRun begins a new workflow run. Returns the run ID (even on failure, returns "").
func (r *Recorder) StartRun(ctx context.Context, p *RunParams) string {
	if !r.ensureConnected() {
		return ""
	}

	now := timestamppb.Now()
	run := &workflowpb.WorkflowRun{
		CorrelationId: p.CorrelationID,
		Context: &workflowpb.WorkflowContext{
			ClusterId:        r.clusterID,
			NodeId:           p.NodeID,
			NodeHostname:     p.NodeHostname,
			ComponentName:    p.ComponentName,
			ComponentKind:    p.ComponentKind,
			ComponentVersion: p.ComponentVersion,
			ReleaseKind:      p.ReleaseKind,
			ReleaseObjectId:  p.ReleaseObjectID,
			// plan_id/plan_generation retired (plan-era fields)
		},
		TriggerReason: p.TriggerReason,
		Status:        workflowpb.RunStatus_RUN_STATUS_PENDING,
		CurrentActor:  ActorController,
		StartedAt:     now,
		WorkflowName:  p.WorkflowName,
	}

	resp, err := r.client.StartRun(ctx, &workflowpb.StartRunRequest{Run: run})
	if err != nil {
		log.Printf("workflow recorder: StartRun failed: %v", err)
		return ""
	}
	return resp.GetId()
}

// RecordStep records a step in a workflow run. Returns the step seq.
func (r *Recorder) RecordStep(ctx context.Context, runID string, p *StepParams) int32 {
	if runID == "" || !r.ensureConnected() {
		return 0
	}

	r.mu.Lock()
	seq := r.seqMap[runID] + 1
	r.seqMap[runID] = seq
	r.mu.Unlock()

	now := timestamppb.Now()
	step := &workflowpb.WorkflowStep{
		RunId:       runID,
		Seq:         seq,
		StepKey:     p.StepKey,
		Title:       p.Title,
		Actor:       p.Actor,
		Phase:       p.Phase,
		Status:      p.Status,
		SourceActor: p.SourceActor,
		TargetActor: p.TargetActor,
		CreatedAt:   now,
		StartedAt:   now,
		Message:     p.Message,
		DetailsJson: p.DetailsJSON,
	}

	if _, err := r.client.RecordStep(ctx, &workflowpb.RecordStepRequest{
		ClusterId: r.clusterID,
		Step:      step,
	}); err != nil {
		log.Printf("workflow recorder: RecordStep failed: %v", err)
	}
	return seq
}

// CompleteStep marks a step as succeeded.
func (r *Recorder) CompleteStep(ctx context.Context, runID string, seq int32, msg string, durationMs int64) {
	if runID == "" || !r.ensureConnected() {
		return
	}
	if _, err := r.client.UpdateStep(ctx, &workflowpb.UpdateStepRequest{
		ClusterId:  r.clusterID,
		RunId:      runID,
		Seq:        seq,
		Status:     StepSucceeded,
		Message:    msg,
		DurationMs: durationMs,
	}); err != nil {
		log.Printf("workflow recorder: CompleteStep failed: %v", err)
	}
}

// FailStep marks a step as failed with classification.
func (r *Recorder) FailStep(ctx context.Context, runID string, seq int32, errorCode, errorMsg, actionHint string, failClass workflowpb.FailureClass, retryable bool) {
	if runID == "" || !r.ensureConnected() {
		return
	}
	if _, err := r.client.FailStep(ctx, &workflowpb.FailStepRequest{
		ClusterId:               r.clusterID,
		RunId:                   runID,
		Seq:                     seq,
		ErrorCode:               errorCode,
		ErrorMessage:            errorMsg,
		ActionHint:              actionHint,
		FailureClass:            failClass,
		Retryable:               retryable,
		OperatorActionRequired:  !retryable,
	}); err != nil {
		log.Printf("workflow recorder: FailStep failed: %v", err)
	}
}

// UpdateRunStatus updates the run status and summary.
func (r *Recorder) UpdateRunStatus(ctx context.Context, runID string, status workflowpb.RunStatus, summary string, actor workflowpb.WorkflowActor) {
	if runID == "" || !r.ensureConnected() {
		return
	}
	if _, err := r.client.UpdateRun(ctx, &workflowpb.UpdateRunRequest{
		Id:           runID,
		ClusterId:    r.clusterID,
		Status:       status,
		Summary:      summary,
		CurrentActor: actor,
	}); err != nil {
		log.Printf("workflow recorder: UpdateRunStatus failed: %v", err)
	}
}

// FinishRun completes a workflow run.
func (r *Recorder) FinishRun(ctx context.Context, runID string, status workflowpb.RunStatus, summary, errorMsg string, failClass workflowpb.FailureClass) {
	if runID == "" || !r.ensureConnected() {
		return
	}
	if _, err := r.client.FinishRun(ctx, &workflowpb.FinishRunRequest{
		Id:           runID,
		ClusterId:    r.clusterID,
		Status:       status,
		Summary:      summary,
		ErrorMessage: errorMsg,
		FailureClass: failClass,
	}); err != nil {
		log.Printf("workflow recorder: FinishRun failed: %v", err)
	}

	// Clean up seq counter.
	r.mu.Lock()
	delete(r.seqMap, runID)
	r.mu.Unlock()
}

// RecordOutcome updates only the workflow summary table (no individual run row).
// Use for periodic workflows (e.g. cluster.reconcile firing every 30s) to keep
// the runs table bounded. The summary carries last success/failure for the
// dashboard view.
func (r *Recorder) RecordOutcome(ctx context.Context, workflowName, runID string,
	status workflowpb.RunStatus, startedAt, finishedAt time.Time, failureReason string) {
	if workflowName == "" || !r.ensureConnected() {
		return
	}
	durationMs := int64(0)
	if !startedAt.IsZero() && !finishedAt.IsZero() {
		durationMs = finishedAt.Sub(startedAt).Milliseconds()
	}
	if _, err := r.client.RecordOutcome(ctx, &workflowpb.RecordOutcomeRequest{
		ClusterId:     r.clusterID,
		WorkflowName:  workflowName,
		RunId:         runID,
		Status:        status,
		StartedAt:     timestamppb.New(startedAt),
		FinishedAt:    timestamppb.New(finishedAt),
		DurationMs:    durationMs,
		FailureReason: failureReason,
	}); err != nil {
		log.Printf("workflow recorder: RecordOutcome failed: %v", err)
	}
}

// RecordStepOutcome upserts per-step aggregate counters. Fire-and-forget.
func (r *Recorder) RecordStepOutcome(ctx context.Context, workflowName, stepID string,
	status workflowpb.StepStatus, startedAt, finishedAt time.Time,
	errorCode, errorMsg string) {
	if workflowName == "" || stepID == "" || !r.ensureConnected() {
		return
	}
	durationMs := int64(0)
	if !startedAt.IsZero() && !finishedAt.IsZero() {
		durationMs = finishedAt.Sub(startedAt).Milliseconds()
	}
	if _, err := r.client.RecordStepOutcome(ctx, &workflowpb.RecordStepOutcomeRequest{
		ClusterId:    r.clusterID,
		WorkflowName: workflowName,
		StepId:       stepID,
		Status:       status,
		StartedAt:    timestamppb.New(startedAt),
		FinishedAt:   timestamppb.New(finishedAt),
		DurationMs:   durationMs,
		ErrorCode:    errorCode,
		ErrorMessage: errorMsg,
	}); err != nil {
		log.Printf("workflow recorder: RecordStepOutcome failed: %v", err)
	}
}

// RecordPhaseTransition appends a phase transition event (TTL 7 days).
func (r *Recorder) RecordPhaseTransition(ctx context.Context, resourceType, resourceName,
	fromPhase, toPhase, reason, caller string, blocked bool) {
	if resourceType == "" || resourceName == "" || !r.ensureConnected() {
		return
	}
	if _, err := r.client.RecordPhaseTransition(ctx, &workflowpb.RecordPhaseTransitionRequest{
		ClusterId:    r.clusterID,
		ResourceType: resourceType,
		ResourceName: resourceName,
		FromPhase:    fromPhase,
		ToPhase:      toPhase,
		Reason:       reason,
		Caller:       caller,
		Blocked:      blocked,
	}); err != nil {
		log.Printf("workflow recorder: RecordPhaseTransition failed: %v", err)
	}
}

// RecordDriftObservation increments consecutive_cycles for a drift item.
func (r *Recorder) RecordDriftObservation(ctx context.Context, driftType, entityRef, chosenWorkflow, remediationID string) {
	if driftType == "" || entityRef == "" || !r.ensureConnected() {
		return
	}
	if _, err := r.client.RecordDriftObservation(ctx, &workflowpb.RecordDriftObservationRequest{
		ClusterId:      r.clusterID,
		DriftType:      driftType,
		EntityRef:      entityRef,
		ChosenWorkflow: chosenWorkflow,
		RemediationId:  remediationID,
	}); err != nil {
		log.Printf("workflow recorder: RecordDriftObservation failed: %v", err)
	}
}

// ClearDriftObservation removes a drift item once it's no longer observed.
func (r *Recorder) ClearDriftObservation(ctx context.Context, driftType, entityRef string) {
	if driftType == "" || entityRef == "" || !r.ensureConnected() {
		return
	}
	if _, err := r.client.ClearDriftObservation(ctx, &workflowpb.ClearDriftObservationRequest{
		ClusterId: r.clusterID,
		DriftType: driftType,
		EntityRef: entityRef,
	}); err != nil {
		log.Printf("workflow recorder: ClearDriftObservation failed: %v", err)
	}
}

// AddArtifact attaches an artifact reference to a run/step.
func (r *Recorder) AddArtifact(ctx context.Context, runID string, stepSeq int32, kind workflowpb.ArtifactKind, name, version, path string) {
	if runID == "" || !r.ensureConnected() {
		return
	}
	if _, err := r.client.AddArtifactRef(ctx, &workflowpb.AddArtifactRefRequest{
		ClusterId: r.clusterID,
		Artifact: &workflowpb.WorkflowArtifactRef{
			RunId:   runID,
			StepSeq: stepSeq,
			Kind:    kind,
			Name:    name,
			Version: version,
			Path:    path,
		},
	}); err != nil {
		log.Printf("workflow recorder: AddArtifact failed: %v", err)
	}
}

// CorrelationID builds a stable correlation ID for a reconciliation lineage.
func CorrelationID(releaseKind, component, nodeID string) string {
	return fmt.Sprintf("%s/%s/%s", releaseKind, component, nodeID)
}
