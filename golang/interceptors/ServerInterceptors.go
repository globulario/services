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
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// ---- lazy loader (no side-effects at import time) ---------------------------

var (
	initOnce sync.Once
	loadErr  error
)

// lazyInit is intentionally a no-op for now. Keep it for future one-time setup.
// IMPORTANT: Do NOT call config/etcd here; this must stay side-effect free
// so that importing this package doesn't touch external systems.
func lazyInit() {
	// no-op (reserved for future one-time prep)
}

// Load returns the unary and stream interceptors, ensuring any one-time
// initialization has completed. It never does heavy work at import time.
func Load() (grpc.UnaryServerInterceptor, grpc.StreamServerInterceptor, error) {
	initOnce.Do(lazyInit)
	return ServerUnaryInterceptor, ServerStreamInterceptor, loadErr
}


var (
	// cache is a generic, process-wide sync map for:
	//  - permission TTL entries
	//  - round-robin indices
	//  - client instances
	cache sync.Map

	// resourceInfos memoizes ResourceInfos for a given gRPC method.
	resourceInfos sync.Map
)


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
		// Safety: unexpected type, drop it
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
	if val, ok := resourceInfos.Load(method); ok {
		return val.([]*rbacpb.ResourceInfos), nil
	}

	rbacClient, err := GetRbacClient(address)
	if err != nil {
		return nil, err
	}

	infos, err := rbacClient.GetActionResourceInfos(method)
	if err != nil {
		return nil, err
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

	// Treat super admin as allowed.
	domain, _ := config.GetDomain()
	if !strings.Contains(subject, "@") {
		subject += "@" + domain
	}
	if subject == "sa@"+domain {
		return true, false, nil
	}

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
// log writes a structured log entry using slog instead of the remote log client.
// The signature is kept identical so existing call sites continue to work.
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
	// Map common levels to slog. Unknown values fall back to Debug.
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

// ---- dynamic client + routing -----------------------------------------------

func getClient(address, serviceName string) (globular_client.Client, error) {
	uuid := Utility.GenerateUUID(address + serviceName)

	if item, ok := cache.Load(uuid); ok {
		if c, ok := item.(globular_client.Client); ok {
			return c, nil
		}
		// Bad type in cache, evict
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

// ServerUnaryInterceptor enforces authZ, optional routing, and logging.
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

	// Default to allowed for local/internal methods, restrict listed ones.
	hasAccess, accessDenied := true, false
	if method == "/services_manager.ServicesManagerServices/GetServicesConfig" ||
		method == "/rbac.RbacService/SetSubjectAllocatedSpace" ||
		method == "/rbac.RbacService/SetSubjectAllocatedSpace/" { // tolerate old caller
		hasAccess = false
	}

	var (
		clientId string
		issuer   string
	)

	if token != "" {
		claims, vErr := security.ValidateToken(token)
		if vErr != nil && !hasAccess {
			log(address, application, clientId, method, Utility.FileLine(), Utility.FunctionName(),
				"token validation failed: "+vErr.Error(), logpb.LogLevel_ERROR_MESSAGE)
			return nil, vErr
		}
		if len(claims.Domain) == 0 {
			return nil, errors.New("token validation failed: empty domain")
		}
		clientId = claims.Id + "@" + claims.UserDomain
		issuer = claims.Issuer
	}

	// If RBAC defines resources for this method, require validation.
	if method != "/rbac.RbacService/GetActionResourceInfos" {
		if infos, e := getActionResourceInfos(address, method); e == nil && infos != nil {
			hasAccess = false
		}
	}

	// Validate by ACCOUNT
	if !hasAccess && clientId != "" {
		hasAccess, accessDenied, _ = validateActionRequest(token, application, organization, rqst, method, clientId, rbacpb.SubjectType_ACCOUNT, address)
		// Additional quota validations:
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
			hasAccess, accessDenied, _ = validateActionRequest(token, application, organization, rqst, method, issuer, rbacpb.SubjectType_PEER, address)
		}
	}

	if !hasAccess || accessDenied {
		err := errors.New("permission denied: method=" + method + " user=" + clientId + " address=" + address + " application=" + application)
		log(address, application, clientId, method, Utility.FileLine(), Utility.FunctionName(), err.Error(), logpb.LogLevel_ERROR_MESSAGE)
		return nil, err
	}

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
}

func (l ServerStreamInterceptorStream) SetHeader(m metadata.MD) error { return l.inner.SetHeader(m) }
func (l ServerStreamInterceptorStream) SendHeader(m metadata.MD) error { return l.inner.SendHeader(m) }
func (l ServerStreamInterceptorStream) SetTrailer(m metadata.MD)      { l.inner.SetTrailer(m) }
func (l ServerStreamInterceptorStream) Context() context.Context      { return l.inner.Context() }
func (l ServerStreamInterceptorStream) SendMsg(rqst interface{}) error {
	return l.inner.SendMsg(rqst)
}

// RecvMsg intercepts inbound messages to apply per-message authorization.
func (l ServerStreamInterceptorStream) RecvMsg(rqst interface{}) error {
	if err := l.inner.RecvMsg(rqst); err != nil {
		return err
	}

	// Methods that require no validation
	if l.method == "/resource.ResourceService/GetRoles" ||
		l.method == "/resource.ResourceService/GetAccounts" ||
		l.method == "/resource.ResourceService/GetOrganizations" ||
		l.method == "/resource.ResourceService/GetApplications" ||
		l.method == "/resource.ResourceService/GetPeers" ||
		l.method == "/resource.ResourceService/GetGroups" ||
		l.method == "/admin.AdminService/DownloadGlobular" ||
		l.method == "/admin.AdminService/GetProcessInfos" ||
		l.method == "/rbac.RbacService/GetResourcePermissionsByResourceType" ||
		l.method == "/repository.PackageRepository/DownloadBundle" ||
		l.method == "/title.TitleService/SearchTitles" ||
		l.method == "/blog.BlogService/SearchBlogPosts" ||
		l.method == "/log.LogService/Log" ||
		l.method == "/log.LogService/GetLog" ||
		l.method == "/file.FileService/ReadDir" {
		return nil
	}

	// Quick allow if we already decided in this stream.
	if _, ok := cache.Load(l.uuid); ok {
		return nil
	}

	allowed, denied := false, false

	if !allowed && l.clientId != "" {
		allowed, denied, _ = validateActionRequest(l.token, l.application, l.organization, rqst, l.method, l.clientId, rbacpb.SubjectType_ACCOUNT, l.address)
	}
	if !allowed && l.application != "" && !denied {
		allowed, denied, _ = validateActionRequest(l.token, l.application, l.organization, rqst, l.method, l.application, rbacpb.SubjectType_APPLICATION, l.address)
	}
	if !allowed && l.peer != "" && !denied {
		allowed, denied, _ = validateActionRequest(l.token, l.application, l.organization, rqst, l.method, l.peer, rbacpb.SubjectType_PEER, l.address)
	}

	if !allowed || denied {
		return errors.New("permission denied: method=" + l.method + " user=" + l.clientId + " address=" + l.address + " application=" + l.application)
	}

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
func (b ServerStreamInterceptorBroadcastStream) SetHeader(md metadata.MD) error { return b.ServerStream.SetHeader(md) }
func (b ServerStreamInterceptorBroadcastStream) SendHeader(md metadata.MD) error { return b.ServerStream.SendHeader(md) }
func (b ServerStreamInterceptorBroadcastStream) SetTrailer(md metadata.MD)       { b.ServerStream.SetTrailer(md) }
func (b ServerStreamInterceptorBroadcastStream) SendMsg(m interface{}) error     { return b.ServerStream.SendMsg(m) }

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

// ServerStreamInterceptor enforces authZ per-stream/per-message, supports
// optional broadcast routing, and logs errors.
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

	if md, ok := metadata.FromIncomingContext(stream.Context()); ok {
		application = strings.Join(md["application"], "")
		token = strings.Join(md["token"], "")
		routing = strings.Join(md["routing"], "")
	}

	var (
		clientId string
		issuer   string
	)

	if token != "" {
		claims, vErr := security.ValidateToken(token)
		if vErr != nil {
			log(address, application, clientId, method, Utility.FileLine(), Utility.FunctionName(),
				"token validation failed: "+vErr.Error(), logpb.LogLevel_ERROR_MESSAGE)
			return vErr
		}
		if len(claims.Domain) == 0 {
			return errors.New("token validation failed: empty domain")
		}
		clientId = claims.Id + "@" + claims.UserDomain
		issuer = claims.Issuer
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
			log(address, application, clientId, method, Utility.FileLine(), Utility.FunctionName(), err.Error(), logpb.LogLevel_ERROR_MESSAGE)
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
		clientId:    clientId,
		peer:        issuer,
	})
	if err != nil {
		log(address, application, clientId, method, Utility.FileLine(), Utility.FunctionName(), err.Error(), logpb.LogLevel_ERROR_MESSAGE)
	}

	cache.Delete(uuid)
	return err
}
