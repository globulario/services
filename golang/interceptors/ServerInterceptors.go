package interceptors

// TODO for the validation, use a map to store valid method/token/resource/access
// the validation will be renew only if the token expire. And when a token expire
// the value in the map will be discard. That way it will put less charge on the server
// side.

import (
	"context"
	"errors"
	//"fmt"
	"strings"
	"sync"
	"time"

	"github.com/davecourtois/Utility"
	"github.com/globulario/services/golang/log/log_client"
	"github.com/globulario/services/golang/log/logpb"
	"github.com/globulario/services/golang/rbac/rbac_client"
	"github.com/globulario/services/golang/rbac/rbacpb"
	"github.com/globulario/services/golang/security"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/reflect/protoreflect"

)

var (

	// The rbac client
	rbac_client_ *rbac_client.Rbac_Client

	// The logger.
	log_client_ *log_client.Log_Client

	// That will contain the permission in memory to limit the number
	// of resource request...
	cache  sync.Map

	// keep map in memory.
	ressourceInfos sync.Map
)

func GetLogClient(domain string) (*log_client.Log_Client, error) {
	var err error
	if log_client_ == nil {
		log_client_, err = log_client.NewLogService_Client(domain, "log.LogService")
		if err != nil {
			return nil, err
		}
	}

	return log_client_, nil
}

/**
 * Get the rbac client.
 */
func GetRbacClient(domain string) (*rbac_client.Rbac_Client, error) {
	var err error
	if rbac_client_ == nil {
		rbac_client_, err = rbac_client.NewRbacService_Client(domain, "rbac.RbacService")
		if err != nil {
			return nil, err
		}

	}
	return rbac_client_, nil
}


/**
 * Keep method info in memory.
 */
func getActionResourceInfos(domain, method string) ([]*rbacpb.ResourceInfos, error) {

	// init the ressourceInfos
	val, ok := ressourceInfos.Load(method)
	if ok {
		return val.([]*rbacpb.ResourceInfos), nil
	}

	rbac_client_, err := GetRbacClient(domain)
	if err != nil {
		return nil, err
	}

	//do something here
	infos, err := rbac_client_.GetActionResourceInfos(method)
	if err != nil {
		return nil, err
	}

	ressourceInfos.Store(method, infos)

	return infos, nil

}

func validateAction(token, application, domain, organization, method, subject string, subjectType rbacpb.SubjectType, infos []*rbacpb.ResourceInfos) (bool, error) {

	id := domain + method + token
	for i := 0; i < len(infos); i++ {
		id += infos[i].Permission + infos[i].Path
	}

	// generate a uuid for the action and it's ressource permissions.
	uuid := Utility.GenerateUUID(id)
	item, ok := cache.Load(uuid)

	if ok {
		// Here I will test if the permission has expired...
		hasAccess_ :=item.(map[string]interface{})
		expiredAt := time.Unix(hasAccess_["expiredAt"].(int64), 0)
		hasAccess__ := hasAccess_["hasAccess"].(bool)
		if time.Now().Before(expiredAt) && hasAccess__ {
			return true, nil
		}
		// the token is expire...
		cache.Delete(uuid)
	}

	rbac_client_, err := GetRbacClient(domain)
	if err != nil {
		return false, err
	}

	hasAccess, err := rbac_client_.ValidateAction(method, subject, subjectType, infos)
	if err != nil {
		return false, err
	}

	// Here I will set the access in the cache.
	log(domain, application, subject, method, Utility.FileLine(), Utility.FunctionName(), "validate action "+method+" for  "+subject+" at domain "+domain, logpb.LogLevel_INFO_MESSAGE)
	cache.Store(uuid, map[string]interface{}{"hasAccess": hasAccess, "expiredAt": time.Now().Add(time.Minute * 15).Unix()})

	return hasAccess, nil

}

func validateActionRequest(token string, application string, organization string, rqst interface{}, method string, subject string, subjectType rbacpb.SubjectType, domain string) (bool, error) {

	hasAccess := false

	infos, err := getActionResourceInfos(domain, method)

	if err != nil {
		infos = make([]*rbacpb.ResourceInfos, 0)
	} else {
		// Here I will get the params...
		val, _ := Utility.CallMethod(rqst, "ProtoReflect", []interface{}{})
		rqst_ := val.(protoreflect.Message)
		if rqst_.Descriptor().Fields().Len() > 0 {
			for i := 0; i < len(infos); i++ {
				// Get the path value from retreive infos.
				param := rqst_.Descriptor().Fields().Get(Utility.ToInt(infos[i].Index))
				val := rqst_.Get(param)
				infos[i].Path = val.String()
			}
		}
	}

	// TODO keep to value in cache for keep speed.
	hasAccess, err = validateAction(token, application, domain, organization, method, subject, subjectType, infos)
	if err != nil {
		return false, err
	}

	// Here I will store the permission for further use...
	return hasAccess, nil
}

// Log the error...
func log(domain, application, user, method, fileLine, functionName string, msg string, level logpb.LogLevel) {
	logger, _ := GetLogClient(domain)
	if logger != nil {
		logger.Log(application, user, method, level, msg, fileLine, functionName)
	}
}

// That interceptor is use by all services except the resource service who has
// it own interceptor.
func ServerUnaryInterceptor(ctx context.Context, rqst interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {

	// The token and the application id.
	var token string
	var application string
	var domain string // This is the target domain, the one use in TLS certificate.
	var organization string

	if md, ok := metadata.FromIncomingContext(ctx); ok {

		application = strings.Join(md["application"], "")
		token = strings.Join(md["token"], "")

		// in case of resource path.
		domain = strings.Join(md["domain"], "")
		if strings.Index(domain, ":") == 0 {
			port := strings.Join(md["port"], "")
			if len(port) > 0 {
				// set the address in that particular case.
				domain += ":" + port
			}
		}
	}


	// Here I will test if the
	method := info.FullMethod

	// If the call come from a local client it has hasAccess
	hasAccess := false
	// needed to get access to the system.
	if method == "/services_manager.ServicesManagerServices/GetServicesConfig" ||
		method == "/services_manager.ServicesManagerServices/GetServiceConfig" ||
		method == "/admin.AdminService/HasRunningProcess" ||
		method == "/admin.AdminService/DownloadGlobular" ||
		method == "/admin.AdminService/GetCertificates" ||
		method == "/authentication.AuthenticationService/Authenticate" ||
		method == "/authentication.AuthenticationService/RefreshToken" ||
		method == "/authentication.AuthenticationService/SetPassword" ||
		method == "/authentication.AuthenticationService/SetRootPassword" ||
		method == "/authentication.AuthenticationService/SetRootEmail" ||
		method == "/discovery.PackageDiscovery/FindPackages" ||
		method == "/discovery.PackageDiscovery/GetPackagesDescriptor" ||
		method == "/discovery.PackageDiscovery/GetPackageDescriptor" ||
		method == "/dns.DnsService/GetA" ||
		method == "/dns.DnsService/GetAAAA" ||
		method == "/resource.ResourceService/UpdateSession" ||
		method == "/resource.ResourceService/GetSession" ||
		method == "/resource.ResourceService/RemoveSession" ||
		method == "/resource.ResourceService/GetSessions" ||
		method == "/resource.ResourceService/GetAccount" ||
		method == "/resource.ResourceService/RegisterPeer" ||
		method == "/resource.ResourceService/AccountExist" ||
		method == "/resource.ResourceService/SetAccountPassword" ||
		method == "/resource.ResourceService/RegisterAccount" ||
		method == "/rbac.RbacService/GetActionResourceInfos" ||
		method == "/rbac.RbacService/ValidateAction" ||
		method == "/rbac.RbacService/ValidateAccess" ||
		method == "/rbac.RbacService/GetResourcePermissions" ||
		method == "/rbac.RbacService/GetResourcePermission" ||
		strings.HasPrefix(method, "/log.LogService/") ||
		strings.HasPrefix(method, "/authentication.AuthenticationService/") ||
		strings.HasPrefix(method, "/event.EventService/") {
		hasAccess = true
	}

	var clientId string
	var err error
	var issuer string

	if len(token) > 0 {
		clientId, _, _, issuer, _, err = security.ValidateToken(token)
		if err != nil && !hasAccess {
			log(domain, application, clientId, method, Utility.FileLine(), Utility.FunctionName(), "fail to validate token for method " + method + " with error " + err.Error(), logpb.LogLevel_ERROR_MESSAGE)
			return nil, err
		}
		if clientId == "sa" {
			hasAccess = true
		}
	}

	// Test if peer has access
	if !hasAccess && len(clientId) > 0 {
		hasAccess, _ = validateActionRequest(token, application, organization, rqst, method, clientId, rbacpb.SubjectType_ACCOUNT, domain)
	}

	if !hasAccess && len(application) > 0 {
		hasAccess, _ = validateActionRequest(token, application, organization, rqst, method, application, rbacpb.SubjectType_APPLICATION, domain)
	}

	if !hasAccess && len(issuer) > 0 {
		hasAccess, _ = validateActionRequest(token, application, organization, rqst, method, issuer, rbacpb.SubjectType_PEER, domain)
	}

	if !hasAccess {
		err := errors.New("Permission denied to execute method " + method + " user:" + clientId + " domain:" + domain + " application:" + application)
		log(domain, application, clientId, method, Utility.FileLine(), Utility.FunctionName(), err.Error(), logpb.LogLevel_ERROR_MESSAGE)
		return nil, err
	}

	var result interface{}
	result, err = handler(ctx, rqst)

	// Send log message.
	if err != nil {
		log(domain, application, clientId, method, Utility.FileLine(), Utility.FunctionName(), err.Error(), logpb.LogLevel_ERROR_MESSAGE)
	}

	return result, err

}

// A wrapper for the real grpc.ServerStream
type ServerStreamInterceptorStream struct {
	inner        grpc.ServerStream
	method       string
	domain       string
	organization string
	peer         string
	token        string
	application  string
	clientId     string
	uuid         string
}

func (l ServerStreamInterceptorStream) SetHeader(m metadata.MD) error {
	return l.inner.SetHeader(m)
}

func (l ServerStreamInterceptorStream) SendHeader(m metadata.MD) error {
	return l.inner.SendHeader(m)
}

func (l ServerStreamInterceptorStream) SetTrailer(m metadata.MD) {
	l.inner.SetTrailer(m)
}

func (l ServerStreamInterceptorStream) Context() context.Context {
	return l.inner.Context()
}

func (l ServerStreamInterceptorStream) SendMsg(rqst interface{}) error {
	return l.inner.SendMsg(rqst)
}

/**
 * Here I will wrap the original stream into this one to get access to the original
 * rqst, so I can validate it resources.
 */
func (l ServerStreamInterceptorStream) RecvMsg(rqst interface{}) error {
	// First of all i will get the message.
	l.inner.RecvMsg(rqst)

	hasAccess := l.clientId == "sa" ||
		l.method == "/grpc.reflection.v1alpha.ServerReflection/ServerReflectionInfo" ||
		l.method == "/admin.AdminService/DownloadGlobular" ||
		l.method == "/repository.PackageRepository/DownloadBundle" ||
		l.method == "/resource.ResourceService/GetApplications" ||
		l.method == "/resource.ResourceService/GetSessions" ||
		l.method == "/resource.ResourceService/GetAccounts" ||
		l.method == "/resource.ResourceService/GetPeers" ||
		l.method == "/resource.ResourceService/GetRoles" ||
		l.method == "/resource.ResourceService/GetGroups" ||
		l.method == "/resource.ResourceService/GetNotifications"

	if hasAccess {
		//fmt.Println("user " + l.clientId + " has permission to execute method: " + l.method + " domain:" + l.domain + " application:" + l.application)
		return nil
	}

	// if the cache contain the uuid it means permission is allowed
	_, ok := cache.Load(l.uuid)
	if ok {
		//fmt.Println("user " + l.clientId + " has permission to execute method: " + l.method + " domain:" + l.domain + " application:" + l.application)
		return nil
	}

	// Test if peer has access
	if !hasAccess && len(l.clientId) > 0 {
		hasAccess, _ = validateActionRequest(l.token, l.application, l.organization, rqst, l.method, l.clientId, rbacpb.SubjectType_ACCOUNT, l.domain)
	}

	if !hasAccess && len(l.application) > 0 {
		hasAccess, _ = validateActionRequest(l.token, l.application, l.organization, rqst, l.method, l.application, rbacpb.SubjectType_APPLICATION, l.domain)
	}

	if !hasAccess && len(l.peer) > 0 {
		hasAccess, _ = validateActionRequest(l.token, l.application, l.organization, rqst, l.method, l.peer, rbacpb.SubjectType_PEER, l.domain)
	}

	if !hasAccess {
		err := errors.New("Permission denied to execute method " + l.method + " user:" + l.clientId + " domain:" + l.domain + " application:" + l.application)
		return err
	}

	cache.Store(l.uuid, []byte{})

	// set empty item to set haAccess.
	return nil
}

// Stream interceptor.
func ServerStreamInterceptor(srv interface{}, stream grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {

	// The token and the application id.
	var token string
	var application string

	// The peer domain.
	var domain string

	if md, ok := metadata.FromIncomingContext(stream.Context()); ok {
		application = strings.Join(md["application"], "")
		token = strings.Join(md["token"], "")
		domain = strings.Join(md["domain"], "")

		if strings.Index(domain, ":") == 0 {
			port := strings.Join(md["port"], "")
			if len(port) > 0 {
				// set the address in that particular case.
				domain += ":" + port
			}
		}
	}

	method := info.FullMethod
	var clientId string
	var issuer string
	var err error

	// Here I will get the peer mac address from the list of registered peer...
	if len(token) > 0 {
		clientId, _, _, issuer, _, err = security.ValidateToken(token)
		if err != nil {
			return err
		}
	}

	// The uuid will be use to set hasAccess into the cache.
	uuid := Utility.RandomUUID()

	// Start streaming.
	err = handler(srv, ServerStreamInterceptorStream{uuid: uuid, inner: stream, method: method, domain: domain, token: token, application: application, clientId: clientId, peer: issuer})

	if err != nil {
		log(domain, application, clientId, method, Utility.FileLine(), Utility.FunctionName(), err.Error(), logpb.LogLevel_ERROR_MESSAGE)
	}

	// Remove the uuid from the cache
	cache.Delete(uuid)

	return err
}
