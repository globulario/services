// Package interceptors centralizes server-side validation, authorization,
// routing (round-robin / broadcasting), and logging for gRPC services.
package interceptors

// NOTE: We intentionally keep the exported API intact. Internal helpers were
// added to improve clarity, error handling, and logging.
//
// Caching strategy:
//   - `cache` (sync.Map) keeps small, short-lived items:
//       * permission decisions keyed by (address, method, token, resources)
//         with a TTL
//       * round-robin indexes keyed by "roundRobinIndex_<method>"
//       * client instances keyed by <address+serviceName>
//   - `resourceInfos` (sync.Map) memoizes rbac.ResourceInfos per method

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	Utility "github.com/globulario/utility"

	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/globular_client"
	"github.com/globulario/services/golang/log/logpb"
	"github.com/globulario/services/golang/rbac/rbac_client"
	"github.com/globulario/services/golang/rbac/rbacpb"
	"github.com/globulario/services/golang/security"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// ---- lazy loader (no side-effects at import time) ---------------------------

var (
	initOnce sync.Once
	loadErr  error
)

// lazyInit is intentionally a no-op for now. Keep it for future one-time setup.
func lazyInit() {}

// Load returns the unary and stream interceptors.
func Load() (grpc.UnaryServerInterceptor, grpc.StreamServerInterceptor, error) {
	initOnce.Do(lazyInit)
	return ServerUnaryInterceptor, ServerStreamInterceptor, loadErr
}

// ---- bootstrap mode ---------------------------------------------------------

// Phase 2: isBootstrapMode() removed - replaced with security.BootstrapGate
// which enforces 4-level security (enablement, time-bounded, loopback-only, method allowlist)

var (
	// cache is a generic, process-wide sync map for:
	//  - permission TTL entries
	//  - round-robin indices
	//  - client instances
	cache sync.Map

	// resourceInfos memoizes ResourceInfos for a given gRPC method.
	resourceInfos sync.Map

	// Phase 4: Deny-by-default enforcement for unmapped methods
	// When true, methods without RBAC mappings are DENIED (secure default)
	// When false, methods without RBAC mappings are ALLOWED with warning (permissive mode)
	// Set via GLOBULAR_DENY_UNMAPPED=1 environment variable
	DenyUnmappedMethods = false
)

// init configures interceptor behavior from environment variables
func init() {
	// Phase 4: Check if deny-by-default should be enforced
	if v := strings.TrimSpace(os.Getenv("GLOBULAR_DENY_UNMAPPED")); v != "" {
		if v == "1" || strings.EqualFold(v, "true") || strings.EqualFold(v, "yes") {
			DenyUnmappedMethods = true
			slog.Info("deny-by-default mode enabled for unmapped methods", "env", "GLOBULAR_DENY_UNMAPPED="+v)
		}
	}
}

// ---- unauthenticated allowlist ----------------------------------------------

// allowSet holds exact methods (full RPC) that bypass authz.
// allowPrefix holds service or method prefixes (e.g. "/log.LogService/").
var (
	allowSet    sync.Map // key: string method, val: struct{}{}
	allowPrefix sync.Map // key: string prefix, val: struct{}{}
)

// Defaults: infra endpoints must always be reachable.
func init() {
	AllowUnauthenticated(
		"/grpc.health.v1.Health/Check",
		"/grpc.reflection.v1alpha.ServerReflection/ServerReflectionInfo",
	)
}

// AllowUnauthenticated registers exact fully-qualified methods that bypass RBAC.
func AllowUnauthenticated(methods ...string) {
	for _, m := range methods {
		if m == "" {
			continue
		}
		allowSet.Store(m, struct{}{})
	}
}

// AllowUnauthenticatedPrefix registers prefixes (service or method) to bypass RBAC.
// Examples: "/log.LogService/", "/file.FileService/Read"
func AllowUnauthenticatedPrefix(prefixes ...string) {
	for _, p := range prefixes {
		if p == "" {
			continue
		}
		allowPrefix.Store(p, struct{}{})
	}
}

func isUnauthenticated(method string) bool {
	if _, ok := allowSet.Load(method); ok {
		return true
	}
	match := false
	allowPrefix.Range(func(k, _ any) bool {
		if strings.HasPrefix(method, k.(string)) {
			match = true
			return false
		}
		return true
	})
	return match
}

// ---- helpers ----------------------------------------------------------------

type permCacheEntry struct {
	hasAccess bool
	expiresAt int64 // unix seconds
}

func nowUnix() int64 { return time.Now().Unix() }

// buildPermCacheKey returns a stable cache key for a permission decision.
func buildPermCacheKey(address, method, token string, infos []*rbacpb.ResourceInfos) string {
	sb := strings.Builder{}
	sb.WriteString(address)
	sb.WriteString("|")
	sb.WriteString(method)
	sb.WriteString("|")
	sb.WriteString(token)
	for _, ri := range infos {
		// Path and Permission determine the decision edge.
		sb.WriteString("|")
		sb.WriteString(ri.GetPermission())
		sb.WriteString("|")
		sb.WriteString(ri.GetPath())
	}
	return Utility.GenerateUUID(sb.String())
}

func putPermCache(key string, allowed bool, ttl time.Duration) {
	cache.Store(key, permCacheEntry{
		hasAccess: allowed,
		expiresAt: nowUnix() + int64(ttl.Seconds()),
	})
}

func getPermCache(key string) (bool, bool) {
	val, ok := cache.Load(key)
	if !ok {
		return false, false
	}
	entry, ok := val.(permCacheEntry)
	if !ok {
		cache.Delete(key)
		return false, false
	}
	if nowUnix() <= entry.expiresAt {
		return entry.hasAccess, true
	}
	cache.Delete(key)
	return false, false
}

// ---- clients ----------------------------------------------------------------

// GetRbacClient returns (and caches) an RBAC client.
func GetRbacClient(address string) (*rbac_client.Rbac_Client, error) {
	Utility.RegisterFunction("NewRbacService_Client", rbac_client.NewRbacService_Client)
	client, err := globular_client.GetClient(address, "rbac.RbacService", "NewRbacService_Client")
	if err != nil {
		return nil, err
	}
	return client.(*rbac_client.Rbac_Client), nil
}

// ---- resource info memoization ----------------------------------------------

// getActionResourceInfos loads and caches ResourceInfos for a given method.
func getActionResourceInfos(address, method string) ([]*rbacpb.ResourceInfos, error) {
	// Never consult RBAC for allowlisted methods.
	if isUnauthenticated(method) {
		resourceInfos.Store(method, []*rbacpb.ResourceInfos{})
		return []*rbacpb.ResourceInfos{}, nil
	}

	if val, ok := resourceInfos.Load(method); ok {
		return val.([]*rbacpb.ResourceInfos), nil
	}

	rbacClient, err := GetRbacClient(address)
	if err != nil {
		return nil, err
	}

	infos, err := rbacClient.GetActionResourceInfos(method)
	if err != nil {
		// Treat "not found" as "no mapping" (permissive by default).
		msg := strings.ToLower(err.Error())
		if strings.Contains(msg, "not found") || strings.Contains(msg, "key not found") {
			infos = []*rbacpb.ResourceInfos{}
			err = nil
		} else {
			return nil, err
		}
	}

	resourceInfos.Store(method, infos)
	return infos, nil
}

// ---- quotas -----------------------------------------------------------------

// ValidateSubjectSpace validates that subject has required available space.
func ValidateSubjectSpace(subject, address string, subjectType rbacpb.SubjectType, requiredSpace uint64) (bool, error) {
	rbacClient, err := GetRbacClient(address)
	if err != nil {
		return false, err
	}
	return rbacClient.ValidateSubjectSpace(subject, subjectType, requiredSpace)
}

// ---- authorization -----------------------------------------------------------

func validateAction(
	token, application, address, organization, method, subject string,
	subjectType rbacpb.SubjectType,
	infos []*rbacpb.ResourceInfos,
) (bool, bool, error) {

	// Phase 3: Removed hardcoded "sa" bypass
	// Admin access now enforced via RBAC globular-admin role
	// No more magic subjects - all authorization goes through RBAC

	cacheKey := buildPermCacheKey(address, method, token, infos)
	if allowed, ok := getPermCache(cacheKey); ok && allowed {
		return true, false, nil
	}

	rbacClient, err := GetRbacClient(address)
	if err != nil {
		slog.Error("rbac client unavailable", "address", address, "err", err)
		return false, false, err
	}

	allowed, accessDenied, err := rbacClient.ValidateAction(method, subject, subjectType, infos)
	if err != nil {
		return allowed, accessDenied, err
	}

	// Cache positive permission for a short TTL.
	putPermCache(cacheKey, allowed, 15*time.Minute)
	return allowed, accessDenied, nil
}

func validateActionRequest(
	token, application, organization string,
	rqst interface{},
	method, subject string,
	subjectType rbacpb.SubjectType,
	domain string,
) (bool, bool, error) {

	infos, err := getActionResourceInfos(domain, method)
	if err != nil {
		infos = make([]*rbacpb.ResourceInfos, 0)
	} else {
		// Reflect request to bind dynamic resource paths.
		val, _ := Utility.CallMethod(rqst, "ProtoReflect", []interface{}{})
		msg := val.(protoreflect.Message)
		if msg.Descriptor().Fields().Len() > 0 {
			for i := 0; i < len(infos); i++ {
				field := msg.Descriptor().Fields().Get(Utility.ToInt(infos[i].Index))
				v := msg.Get(field)

				// list path binding
				if field.IsList() {
					expanded := make([]*rbacpb.ResourceInfos, v.List().Len())
					for j := 0; j < v.List().Len(); j++ {
						ri := &rbacpb.ResourceInfos{
							Index:      infos[i].Index,
							Permission: infos[i].Permission,
						}
						ri.Path, _ = url.PathUnescape(v.List().Get(j).String())
						expanded[j] = ri
					}
					return validateAction(token, application, domain, organization, method, subject, subjectType, expanded)
				}

				// message subfield or scalar
				if field.Kind() == protoreflect.MessageKind && len(infos[i].Field) > 0 {
					riField := field.Message().Fields().ByTextName(infos[i].Field)
					infos[i].Path, _ = url.PathUnescape(v.Message().Get(riField).String())
				} else {
					infos[i].Path, _ = url.PathUnescape(v.String())
				}
			}
		}
	}

	return validateAction(token, application, domain, organization, method, subject, subjectType, infos)
}

// ---- logging ----------------------------------------------------------------
func log(address, application, user, method, fileLine, functionName, msg string, level logpb.LogLevel) {
	attrs := []any{
		"domain", address,
		"application", application,
		"user", user,
		"method", method,
		"function", functionName,
		"file", fileLine,
	}

	switch level {
	case logpb.LogLevel_ERROR_MESSAGE:
		slog.Error(msg, attrs...)
	case logpb.LogLevel_WARN_MESSAGE:
		slog.Warn(msg, attrs...)
	case logpb.LogLevel_INFO_MESSAGE:
		slog.Info(msg, attrs...)
	default:
		slog.Debug(msg, attrs...)
	}
}

func shouldLogError(method string, err error) bool {
	if err == nil {
		return false
	}
	if strings.Contains(method, "/file.FileService/ReadFile") && strings.Contains(err.Error(), "/.hidden/") {
		return false
	}
	return true
}

// ---- dynamic client + routing -----------------------------------------------

func getClient(address, serviceName string) (globular_client.Client, error) {
	uuid := Utility.GenerateUUID(address + serviceName)

	if item, ok := cache.Load(uuid); ok {
		if c, ok := item.(globular_client.Client); ok {
			return c, nil
		}
		cache.Delete(uuid)
	}

	fct := "New" + serviceName[strings.Index(serviceName, ".")+1:] + "_Client"
	client, err := globular_client.GetClient(address, serviceName, fct)
	if err != nil {
		return nil, err
	}
	cache.Store(uuid, client)
	return client, nil
}

// roundRobinUnaryMethodHandler forwards unary calls to peers in a round-robin fashion.
func roundRobinUnaryMethodHandler(ctx context.Context, method string, rqst interface{}) (interface{}, error) {
	cfg, err := config.GetLocalConfig(true)
	if err != nil {
		return nil, err
	}
	peers := cfg["Peers"].([]interface{})
	if len(peers) == 0 {
		return nil, errors.New("no peers found")
	}

	key := "roundRobinIndex_" + method
	idxAny, ok := cache.Load(key)
	if !ok {
		idxAny = 0
	}

	// -1 means "force local"
	if idx, _ := idxAny.(int); idx == -1 {
		cache.Store(key, 0)
		return nil, errors.New("force method to be called locally")
	}

	idx := idxAny.(int)
	peer := peers[idx].(map[string]interface{})
	address := peer["Hostname"].(string) + "." + peer["Domain"].(string) + ":" + Utility.ToString(peer["Port"])

	service := method[1:][:strings.Index(method[1:], "/")]
	client, err := getClient(address, service)
	if err != nil {
		return nil, err
	}

	resp, err := client.Invoke(method, rqst, ctx)
	if err != nil {
		return nil, err
	}

	idx++
	if idx >= len(peers) {
		idx = -1 // after last peer, force next call to be local
	}
	cache.Store(key, idx)
	return resp, nil
}

func handleUnaryMethod(routing, token string, ctx context.Context, method string, rqst interface{}) (interface{}, error) {
	switch routing {
	case "round-robin":
		outCtx := metadata.AppendToOutgoingContext(context.Background(), "token", token)
		return roundRobinUnaryMethodHandler(outCtx, method, rqst)
	case "", "local":
		return nil, errors.New("no dynamic routing for method")
	default:
		return nil, errors.New("unsupported routing: " + routing)
	}
}

// ---- interceptors (unary) ---------------------------------------------------

// ServerUnaryInterceptor is now NORMALLY PERMISSIVE:
// - If the method is allowlisted â†’ pass through
// - If RBAC has NO resource mapping for the method â†’ pass through
// - Only if there IS a mapping, we parse token and enforce RBAC.
func ServerUnaryInterceptor(ctx context.Context, rqst interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	var (
		token        string
		application  string
		organization string // kept for API parity (unused here)
	)

	address, err := config.GetAddress()
	if err != nil || len(address) == 0 {
		if err == nil {
			err = errors.New("empty address")
		}
		return nil, err
	}

	method := info.FullMethod
	var routing string

	if md, ok := metadata.FromIncomingContext(ctx); ok {
		application = strings.Join(md["application"], "")
		token = strings.Join(md["token"], "")
		routing = strings.Join(md["routing"], "")
	}

	// Phase 1: Create AuthContext for audit logging and future authorization
	// This adds observability without changing behavior
	authCtx, err := security.NewAuthContext(ctx, method)
	if err != nil {
		// Log error but continue - auth context is for observability only in Phase 1
		slog.Warn("failed to create auth context", "error", err, "method", method)
	}
	// Store in context for propagation to handlers
	if authCtx != nil {
		ctx = authCtx.ToContext(ctx)
	}

	// 1) Bypass for bootstrap mode (Day-0 installation).
	//    Phase 2: Use secure BootstrapGate with 4-level protection:
	//    - Explicit enablement (flag file or env var)
	//    - Time-bounded (< 30 minutes)
	//    - Loopback-only (127.0.0.1/::1)
	//    - Method allowlisted (essential Day-0 methods only)
	if authCtx.IsBootstrap {
		allowed, reason := security.DefaultBootstrapGate.ShouldAllow(authCtx)
		if allowed {
			// Security Fix #9: During bootstrap, cluster_id can be empty
			// After cluster init, cluster_id enforcement applies (below)
			LogAuthzDecisionSimple(authCtx, true, reason)
			return handler(ctx, rqst)
		}
		// Bootstrap gate denied - log and fall through to normal authorization
		LogAuthzDecisionSimple(authCtx, false, reason)
	}

	// Security Fix #9: Cluster ID Enforcement
	// Once cluster is initialized, all non-bootstrap requests MUST have
	// verified cluster_id matching local cluster_id (prevents cross-cluster attacks)
	if !authCtx.IsBootstrap {
		// Check if cluster is initialized (has local cluster_id)
		localClusterID, err := security.GetLocalClusterID()
		if err == nil && localClusterID != "" {
			// Cluster is initialized - enforce cluster_id matching
			if authCtx.ClusterID == "" {
				// Missing cluster_id after initialization
				LogAuthzDecisionSimple(authCtx, false, "cluster_id_missing")
				return nil, status.Errorf(codes.Unauthenticated,
					"cluster_id required after cluster initialization")
			}
			// Validate cluster_id matches local cluster
			if err := security.ValidateClusterID(ctx, authCtx.ClusterID); err != nil {
				LogAuthzDecisionSimple(authCtx, false, "cluster_id_mismatch")
				return nil, status.Errorf(codes.Unauthenticated,
					"cluster_id validation failed: %v", err)
			}
		}
		// Cluster not initialized yet OR cluster_id valid - continue
	}

	// 2) Bypass for allowlisted infra/public methods.
	if isUnauthenticated(method) {
		LogAuthzDecisionSimple(authCtx, true, "allowlist")
		return handler(ctx, rqst)
	}

	// Post-Day-0: after cluster is initialized, all mutating RPCs MUST be authenticated.
	// Anonymous callers (no token AND no mTLS cert) receive Unauthenticated.
	// This check applies regardless of whether an RBAC mapping exists.
	clusterInitialized, _ := security.IsClusterInitialized(ctx)

	// Degraded-mode detection: a token is present but the cluster gate returned false.
	// In normal post-Day-0 operation this should never happen because the local cluster
	// ID is cached in-memory after first use. Log a warning so it is visible in audit
	// trails if it ever occurs (e.g. mis-configured or first-boot race).
	if !clusterInitialized && authCtx != nil && authCtx.AuthMethod == "jwt" {
		slog.Warn("clusterInitialized=false while JWT token present; authentication enforcement skipped",
			"method", method,
			"subject", authCtx.Subject,
		)
	}

	if clusterInitialized && security.IsMutatingRPC(method) && authCtx.Subject == "" {
		LogAuthzDecisionSimple(authCtx, false, "authentication_required")
		return nil, status.Errorf(codes.Unauthenticated,
			"authentication required: provide --token or configure client certificates")
	}

	// Role-binding check: applies to all explicitly role-mapped gRPC methods.
	// Skip the RBAC service itself (would cause a circular RPC call).
	// Only fires for authenticated subjects post-cluster-initialization.
	if clusterInitialized && security.IsRoleBasedMethod(method) &&
		!strings.HasPrefix(method, "/rbac.RbacService/") &&
		authCtx != nil && authCtx.Subject != "" {

		allowed, _ := checkRoleBinding(authCtx.Subject, method, address)
		if !allowed {
			LogAuthzDecisionSimple(authCtx, false, "role_binding_denied")
			return nil, status.Errorf(codes.PermissionDenied,
				"permission denied: %s — assign a role with 'globular rbac bind'", method)
		}
		LogAuthzDecisionSimple(authCtx, true, "role_binding_granted")
		return handler(ctx, rqst)
	}

	// 3) Only consult RBAC if there are resource mappings for this method.
	needAuthz := false
	if method != "/rbac.RbacService/GetActionResourceInfos" {
		if infos, e := getActionResourceInfos(address, method); e == nil && len(infos) > 0 {
			needAuthz = true
		}
	}
	if !needAuthz {
		// Post-Day-0: mutating RPCs with an identity but no RBAC mapping are denied.
		// This prevents new methods from silently bypassing access control after cluster init.
		if clusterInitialized && security.IsMutatingRPC(method) {
			LogAuthzDecisionSimple(authCtx, false, "no_rbac_mapping_post_day0")
			return nil, status.Errorf(codes.PermissionDenied,
				"method %s requires an explicit RBAC permission (cluster is secured)", method)
		}
		// Phase 4: Conditional deny-by-default for unmapped methods
		if DenyUnmappedMethods {
			// Enforcement mode: DENY unmapped methods
			LogAuthzDecisionSimple(authCtx, false, "no_rbac_mapping_denied")
			return nil, status.Errorf(codes.PermissionDenied,
				"method %s has no RBAC mapping (deny-by-default enforced)", method)
		}
		// Warning mode: ALLOW but log for detection
		LogAuthzDecisionSimple(authCtx, true, "no_rbac_mapping_warning")
		return handler(ctx, rqst)
	}

	// 4) We need auth → use AuthContext as single source of truth
	// Security Fix #10: Use AuthContext identity instead of re-extracting/re-validating token
	// This ensures consistent identity between AuthContext and authorization decisions
	var clientId, issuer string
	if authCtx != nil && authCtx.Subject != "" {
		// Use identity from AuthContext (already validated during NewAuthContext)
		// This respects both md["token"] AND Authorization header (consistent extraction)
		clientId = authCtx.Subject // Canonical identity (PrincipalID with fallbacks)
		issuer = authCtx.GetIssuer() // Issuer for NODE_IDENTITY authorization
	}

	// Validate by ACCOUNT
	hasAccess, accessDenied, _ := false, false, error(nil)
	if clientId != "" {
		hasAccess, accessDenied, _ = validateActionRequest(token, application, organization, rqst, method, clientId, rbacpb.SubjectType_ACCOUNT, address)
		// Quota example
		if method == "/torrent.TorrentService/DownloadTorrent" {
			_, _ = ValidateSubjectSpace(clientId, address, rbacpb.SubjectType_ACCOUNT, 0)
		}
	}

	// Validate by APPLICATION
	if !hasAccess && application != "" && !accessDenied {
		hasAccess, accessDenied, _ = validateActionRequest(token, application, organization, rqst, method, application, rbacpb.SubjectType_APPLICATION, address)
	}

	// Validate by PEER
	if !hasAccess && issuer != "" && !accessDenied {
		mac, _ := config.GetMacAddress()
		if issuer != mac {
			hasAccess, accessDenied, _ = validateActionRequest(token, application, organization, rqst, method, issuer, rbacpb.SubjectType_NODE_IDENTITY, address)
		}
	}

	if !hasAccess || accessDenied {
		// Phase 1: Log RBAC denial
		LogAuthzDecisionSimple(authCtx, false, "rbac_denied")
		st := status.Errorf(codes.PermissionDenied,
			"permission denied: method=%s user=%s address=%s application=%s",
			method, clientId, address, application,
		)
		log(address, application, clientId, method, Utility.FileLine(), Utility.FunctionName(), st.Error(), logpb.LogLevel_ERROR_MESSAGE)
		return nil, st
	}

	// Phase 1: Log RBAC grant (implicit success - handler will be called)
	LogAuthzDecisionSimple(authCtx, true, "rbac_granted")

	// Optional dynamic routing
	if res, rErr := handleUnaryMethod(routing, token, ctx, method, rqst); rErr == nil {
		return res, nil
	}

	// Call the actual handler
	res, hErr := handler(ctx, rqst)
	if hErr != nil {
		log(address, application, clientId, method, Utility.FileLine(), Utility.FunctionName(), hErr.Error(), logpb.LogLevel_ERROR_MESSAGE)
		return nil, hErr
	}

	return res, nil
}

// ---- interceptors (stream) --------------------------------------------------

// ServerStreamInterceptorStream wraps a ServerStream to authorize each message.
type ServerStreamInterceptorStream struct {
	inner        grpc.ServerStream
	method       string
	address      string
	organization string
	peer         string
	token        string
	application  string
	clientId     string
	uuid         string // cache slot for this stream
	authCtx      *security.AuthContext // Phase 1: for audit logging
}

func (l ServerStreamInterceptorStream) SetHeader(m metadata.MD) error  { return l.inner.SetHeader(m) }
func (l ServerStreamInterceptorStream) SendHeader(m metadata.MD) error { return l.inner.SendHeader(m) }
func (l ServerStreamInterceptorStream) SetTrailer(m metadata.MD)       { l.inner.SetTrailer(m) }
func (l ServerStreamInterceptorStream) Context() context.Context       { return l.inner.Context() }
func (l ServerStreamInterceptorStream) SendMsg(rqst interface{}) error {
	return l.inner.SendMsg(rqst)
}

// RecvMsg is now NORMALLY PERMISSIVE:
// - Allowlisted â†’ pass
// - No RBAC mapping â†’ pass
// - Only if mapping exists, parse token and enforce RBAC.
func (l ServerStreamInterceptorStream) RecvMsg(rqst interface{}) error {
	if err := l.inner.RecvMsg(rqst); err != nil {
		return err
	}

	// 1) Bypass for bootstrap mode (Phase 2: secure gate).
	if l.authCtx != nil && l.authCtx.IsBootstrap {
		allowed, reason := security.DefaultBootstrapGate.ShouldAllow(l.authCtx)
		if allowed {
			LogAuthzDecisionSimple(l.authCtx, true, reason)
			return nil
		}
		// Bootstrap gate denied - log and fall through to normal authorization
		LogAuthzDecisionSimple(l.authCtx, false, reason)
	}

	// 2) Allowlisted methods require no validation.
	if isUnauthenticated(l.method) {
		LogAuthzDecisionSimple(l.authCtx, true, "allowlist")
		return nil
	}

	// 3) If we've already validated this stream, skip (already logged).
	if _, ok := cache.Load(l.uuid); ok {
		return nil
	}

	// 4) Only consult RBAC if there are resource mappings for this method.
	needAuthz := false
	if infos, e := getActionResourceInfos(l.address, l.method); e == nil && len(infos) > 0 {
		needAuthz = true
	}
	if !needAuthz {
		// Phase 4: Conditional deny-by-default for unmapped methods (streaming)
		if DenyUnmappedMethods {
			// Enforcement mode: DENY unmapped methods
			LogAuthzDecisionSimple(l.authCtx, false, "no_rbac_mapping_denied")
			return status.Errorf(codes.PermissionDenied,
				"method %s has no RBAC mapping (deny-by-default enforced)", l.method)
		} else {
			// Warning mode: ALLOW but log for detection (first message only)
			LogAuthzDecisionSimple(l.authCtx, true, "no_rbac_mapping_warning")
			cache.Store(l.uuid, struct{}{}) // mark authorized for rest of the stream
			return nil
		}
	}

	// 5) We need auth → use AuthContext as single source of truth
	// Security Fix #10: Use AuthContext identity instead of re-extracting/re-validating token
	var clientId, issuer string
	if l.authCtx != nil && l.authCtx.Subject != "" {
		// Use identity from AuthContext (already validated during stream setup)
		// This respects both md["token"] AND Authorization header (consistent extraction)
		clientId = l.authCtx.Subject // Canonical identity (PrincipalID with fallbacks)
		issuer = l.authCtx.GetIssuer() // Issuer for NODE_IDENTITY authorization
	}

	allowed, denied := false, false

	if !allowed && clientId != "" {
		allowed, denied, _ = validateActionRequest(l.token, l.application, l.organization, rqst, l.method, clientId, rbacpb.SubjectType_ACCOUNT, l.address)
	}
	if !allowed && l.application != "" && !denied {
		allowed, denied, _ = validateActionRequest(l.token, l.application, l.organization, rqst, l.method, l.application, rbacpb.SubjectType_APPLICATION, l.address)
	}
	if !allowed && issuer != "" && !denied {
		allowed, denied, _ = validateActionRequest(l.token, l.application, l.organization, rqst, l.method, issuer, rbacpb.SubjectType_NODE_IDENTITY, l.address)
	}

	if !allowed || denied {
		// Phase 1: Log RBAC denial
		LogAuthzDecisionSimple(l.authCtx, false, "rbac_denied")
		return status.Errorf(codes.PermissionDenied,
			"permission denied: method=%s user=%s address=%s application=%s",
			l.method, clientId, l.address, l.application,
		)
	}

	// Phase 1: Log RBAC grant (first message only, not every RecvMsg)
	LogAuthzDecisionSimple(l.authCtx, true, "rbac_granted")

	// Mark this stream as authorized for subsequent messages.
	cache.Store(l.uuid, struct{}{})
	return nil
}

// ServerStreamInterceptorBroadcastStream fans out inbound messages to peers.
type ServerStreamInterceptorBroadcastStream struct {
	grpc.ServerStream
	addresses []string
	method    string
	token     string
}

func (b ServerStreamInterceptorBroadcastStream) Context() context.Context {
	return metadata.AppendToOutgoingContext(context.Background(), "token", b.token)
}
func (b ServerStreamInterceptorBroadcastStream) SetHeader(md metadata.MD) error {
	return b.ServerStream.SetHeader(md)
}
func (b ServerStreamInterceptorBroadcastStream) SendHeader(md metadata.MD) error {
	return b.ServerStream.SendHeader(md)
}
func (b ServerStreamInterceptorBroadcastStream) SetTrailer(md metadata.MD) {
	b.ServerStream.SetTrailer(md)
}
func (b ServerStreamInterceptorBroadcastStream) SendMsg(m interface{}) error {
	return b.ServerStream.SendMsg(m)
}

func (b ServerStreamInterceptorBroadcastStream) RecvMsg(m interface{}) error {
	if err := b.ServerStream.RecvMsg(m); err != nil {
		return err
	}
	b.Broadcast(m)
	return nil
}

func (b *ServerStreamInterceptorBroadcastStream) Broadcast(req interface{}) {
	var wg sync.WaitGroup
	for _, addr := range b.addresses {
		wg.Add(1)
		go func(address string) {
			defer wg.Done()
			serviceName := b.method[1:][:strings.Index(b.method[1:], "/")]
			if err := b.sendRequestToServer(b.Context(), serviceName, b.method, address, req); err != nil {
				slog.Warn("broadcast send failed", "address", address, "method", b.method, "err", err)
			}
		}(addr)
	}
	wg.Wait()
}

func (b *ServerStreamInterceptorBroadcastStream) sendRequestToServer(ctx context.Context, serviceName, method, address string, rqst interface{}) error {
	client, err := getClient(address, serviceName)
	if err != nil {
		return err
	}

	stream, err := client.Invoke(method, rqst, ctx)
	if err != nil {
		return err
	}

	for {
		resp, recvErr := Utility.CallMethod(stream, "Recv", []interface{}{})
		if recvErr != nil {
			if recvErr.(error) == io.EOF {
				break
			}
			return recvErr.(error)
		}
		if err := b.SendMsg(resp); err != nil {
			return err
		}
	}
	return nil
}

// ServerStreamInterceptor is now NORMALLY PERMISSIVE at the outer layer too:
// it does NOT pre-validate tokens; RecvMsg decides based on RBAC mappings.
func ServerStreamInterceptor(srv interface{}, stream grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
	var (
		token       string
		application string
	)

	address, err := config.GetAddress()
	if err != nil || len(address) == 0 {
		if err == nil {
			err = errors.New("empty address")
		}
		return err
	}

	method := info.FullMethod
	routing := ""

	ctx := stream.Context()
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		application = strings.Join(md["application"], "")
		token = strings.Join(md["token"], "")
		routing = strings.Join(md["routing"], "")
	}

	// Phase 1: Create AuthContext for audit logging
	authCtx, err := security.NewAuthContext(ctx, method)
	if err != nil {
		slog.Warn("failed to create auth context (stream)", "error", err, "method", method)
	}
	// Store in context for propagation
	if authCtx != nil {
		ctx = authCtx.ToContext(ctx)
		// Note: We don't modify the stream here, just track for audit
	}

	// Bypass RBAC for bootstrap mode (Day-0 installation).
	// Phase 2: Use secure BootstrapGate with 4-level protection.
	if authCtx != nil && authCtx.IsBootstrap {
		allowed, reason := security.DefaultBootstrapGate.ShouldAllow(authCtx)
		if allowed {
			LogAuthzDecisionSimple(authCtx, true, reason)
			return handler(srv, stream)
		}
		// Bootstrap gate denied - log and fall through to normal authorization
		LogAuthzDecisionSimple(authCtx, false, reason)
	}

	// Bypass RBAC entirely for allowlisted infra/public methods.
	if isUnauthenticated(method) {
		LogAuthzDecisionSimple(authCtx, true, "allowlist")
		return handler(srv, stream)
	}

	// Security Fix #9: Cluster ID enforcement for streaming RPCs
	// Once cluster is initialized, ALL non-bootstrap requests must have matching cluster_id
	streamInitialized := false
	if authCtx != nil && !authCtx.IsBootstrap {
		// Check if cluster is initialized (has local cluster ID)
		if localClusterID, err := security.GetLocalClusterID(); err == nil && localClusterID != "" {
			streamInitialized = true
			// Cluster initialized - enforce cluster_id matching
			if authCtx.ClusterID == "" {
				LogAuthzDecisionSimple(authCtx, false, "cluster_id_missing")
				return status.Errorf(codes.Unauthenticated,
					"cluster_id required after cluster initialization")
			}
			// Validate cluster_id matches local cluster
			if err := security.ValidateClusterID(ctx, authCtx.ClusterID); err != nil {
				LogAuthzDecisionSimple(authCtx, false, "cluster_id_mismatch")
				return status.Errorf(codes.Unauthenticated,
					"cluster_id validation failed: %v", err)
			}
		}
		// Cluster not initialized yet OR cluster_id valid - continue
	}

	// Post-Day-0: mutating streaming RPCs must be authenticated.
	if streamInitialized && security.IsMutatingRPC(method) && authCtx != nil && authCtx.Subject == "" {
		LogAuthzDecisionSimple(authCtx, false, "authentication_required")
		return status.Errorf(codes.Unauthenticated,
			"authentication required: provide --token or configure client certificates")
	}

	// Role-binding check for streaming RPCs: mirrors the unary interceptor check.
	// Skip the RBAC service itself (would cause a circular RPC call).
	if streamInitialized && security.IsRoleBasedMethod(method) &&
		!strings.HasPrefix(method, "/rbac.RbacService/") &&
		authCtx != nil && authCtx.Subject != "" {

		allowed, _ := checkRoleBinding(authCtx.Subject, method, address)
		if !allowed {
			LogAuthzDecisionSimple(authCtx, false, "role_binding_denied")
			return status.Errorf(codes.PermissionDenied,
				"permission denied: %s — assign a role with 'globular rbac bind'", method)
		}
		LogAuthzDecisionSimple(authCtx, true, "role_binding_granted")
		return handler(srv, stream)
	}

	uuid := Utility.RandomUUID()

	if routing == "broadcasting" {
		cfg, err := config.GetLocalConfig(true)
		if err != nil {
			return err
		}
		peers := cfg["Peers"].([]interface{})
		if len(peers) == 0 {
			return errors.New("no peers found")
		}
		addrs := make([]string, 0, len(peers)+1)
		for _, p := range peers {
			pm := p.(map[string]interface{})
			addrs = append(addrs, pm["Hostname"].(string)+"."+pm["Domain"].(string)+":"+Utility.ToString(pm["Port"]))
		}
		// include local
		addrs = append(addrs, address)

		if err := handler(srv, ServerStreamInterceptorBroadcastStream{
			ServerStream: stream,
			addresses:    addrs,
			method:       method,
			token:        token,
		}); err != nil {
			if shouldLogError(method, err) {
				log(address, application, "", method, Utility.FileLine(), Utility.FunctionName(), err.Error(), logpb.LogLevel_ERROR_MESSAGE)
			}
			return err
		}
		return nil
	}

	err = handler(srv, ServerStreamInterceptorStream{
		uuid:        uuid,
		inner:       stream,
		method:      method,
		address:     address,
		token:       token,
		application: application,
		authCtx:     authCtx, // Phase 1: pass for audit logging
		// clientId/peer computed lazily in RecvMsg only if RBAC is needed
	})
	if err != nil && shouldLogError(method, err) {
		log(address, application, "", method, Utility.FileLine(), Utility.FunctionName(), err.Error(), logpb.LogLevel_ERROR_MESSAGE)
	}

	cache.Delete(uuid)
	return err
}
