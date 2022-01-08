package interceptors

// TODO for the validation, use a map to store valid method/token/resource/access
// the validation will be renew only if the token expire. And when a token expire
// the value in the map will be discard. That way it will put less charge on the server
// side.

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/davecourtois/Utility"
	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/globular_client"
	"github.com/globulario/services/golang/log/log_client"
	"github.com/globulario/services/golang/log/logpb"
	"github.com/globulario/services/golang/persistence/persistence_client"
	"github.com/globulario/services/golang/rbac/rbac_client"
	"github.com/globulario/services/golang/rbac/rbacpb"
	"github.com/globulario/services/golang/resource/resource_client"
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
	cache sync.Map

	// keep map in memory.
	ressourceInfos sync.Map

	// Client connections.
	clients sync.Map
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
 * Get a client
 */
func getClient(name, address string) (globular_client.Client, error) {
	id := Utility.GenerateUUID(name + "|" + address)

	// Here I get existing client if it exist...
	val, ok := clients.Load(id)
	if ok {
		return val.(globular_client.Client), nil
	}

	if name == "persistence.PersistenceService" {
		client, err := persistence_client.NewPersistenceService_Client(address, name)
		if err != nil {
			return nil, err
		}
		clients.Store(id, client)
		return client, nil
	} else if name == "resource.ResourceService" {
		client, err := resource_client.NewResourceService_Client(address, "resource.ResourceService")
		if err != nil {
			return nil, err
		}
		clients.Store(id, client)
		// simply redirect the rquest the the good address and return the result to the caller...
		return client, nil
	}

	return nil, errors.New("no service register with name " + name + " was found at address " + address)
}

/**
 * Invoke a methode on a given client.
 */
func invoke(address, method string, rqst interface{}, ctx context.Context) (interface{}, error) {
	name := strings.Split(method, "/")[1]
	client, err := getClient(name, address)
	if err != nil {
		return nil, err
	}

	// So here I will inject the mac address to give the other peer information about
	// where the request came from...
	// add key-value pairs of metadata to context
	return client.Invoke(method, rqst, ctx)
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
		hasAccess_ := item.(map[string]interface{})
		expiredAt := time.Unix(hasAccess_["expiredAt"].(int64), 0)
		hasAccess__ := hasAccess_["hasAccess"].(bool)
		if time.Now().Before(expiredAt) && hasAccess__ {
			fmt.Println("permission found for method", method, subject)
			return true, nil
		}
		// the token is expire...
		fmt.Println("remove expired cache item")
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
	//log(domain, application, subject, method, Utility.FileLine(), Utility.FunctionName(), "validate action "+method+" for  "+subject+" at domain "+domain, logpb.LogLevel_INFO_MESSAGE)
	cache.Store(uuid, map[string]interface{}{"hasAccess": hasAccess, "expiredAt": time.Now().Add(time.Minute * 15).Unix()})

	return hasAccess, nil

}

func validateActionRequest(token string, application string, organization string, rqst interface{}, method string, subject string, subjectType rbacpb.SubjectType, domain string) (bool, error) {

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
	hasAccess, err := validateAction(token, application, domain, organization, method, subject, subjectType, infos)
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
	var domain string
	var address string // the address if the token issuer
	var organization string

	// Here I will test if the
	method := info.FullMethod
	domain, _ = config.GetDomain()

	if md, ok := metadata.FromIncomingContext(ctx); ok {

		// The application...
		application = strings.Join(md["application"], "")
		token = strings.Join(md["token"], "")

		// origin := strings.Join(md["origin"], "")
		// fmt.Println("-------------------------------> ", origin)
		address, _ = config.GetAddress()
	}

	// If the call come from a local client it has hasAccess
	hasAccess := true

	// Set the list of restricted method here...
	if method == "/services_manager.ServicesManagerServices/GetServicesConfig" {
		hasAccess = true
	}

	var clientId string
	var err error
	var issuer string

	if len(token) > 0 {
		claims, err := security.ValidateToken(token)
		if err != nil && !hasAccess {
			log(domain, application, clientId, method, Utility.FileLine(), Utility.FunctionName(), "fail to validate token for method "+method+" with error "+err.Error(), logpb.LogLevel_ERROR_MESSAGE)
			return nil, err
		}

		clientId = claims.Id
		issuer = claims.Issuer
	}

	// Test if peer has access
	if !hasAccess && len(clientId) > 0 {
		hasAccess, _ = validateActionRequest(token, application, organization, rqst, method, clientId, rbacpb.SubjectType_ACCOUNT, address)
	}

	if !hasAccess && len(application) > 0 {
		hasAccess, _ = validateActionRequest(token, application, organization, rqst, method, application, rbacpb.SubjectType_APPLICATION, address)
	}

	if !hasAccess && len(issuer) > 0 {
		hasAccess, _ = validateActionRequest(token, application, organization, rqst, method, issuer, rbacpb.SubjectType_PEER, address)
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
	inner        grpc.ServerStream // default stream
	method       string
	address      string
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
	hasAccess := true

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
		hasAccess, _ = validateActionRequest(l.token, l.application, l.organization, rqst, l.method, l.clientId, rbacpb.SubjectType_ACCOUNT, l.address)
	}

	if !hasAccess && len(l.application) > 0 {

		hasAccess, _ = validateActionRequest(l.token, l.application, l.organization, rqst, l.method, l.application, rbacpb.SubjectType_APPLICATION, l.address)

	}

	if !hasAccess && len(l.peer) > 0 {
		hasAccess, _ = validateActionRequest(l.token, l.application, l.organization, rqst, l.method, l.peer, rbacpb.SubjectType_PEER, l.address)
	}

	if !hasAccess {
		err := errors.New("Permission denied to execute method " + l.method + " user:" + l.clientId + " address:" + l.address + " application:" + l.application)
		return err
	}

	// I will store the access.
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
	var domain string      // This is the target domain, the one use in TLS certificate.
	var address string     // the address if the token issuer

	method := info.FullMethod
	domain, _ = config.GetDomain()

	if md, ok := metadata.FromIncomingContext(stream.Context()); ok {
		application = strings.Join(md["application"], "")
		token = strings.Join(md["token"], "")
		address, _ = config.GetAddress()

		// In that case the request must be process by other peer so I will redirect
		// the request to that peer and return it response.
		address_ := strings.TrimSpace(strings.Join(md["address"], ""))
		address_ = strings.ToLower(address_)
	}

	var clientId string
	var issuer string
	var err error
	hasAccess := true

	// TODO set method the require access validation here....


	// Here I will get the peer mac address from the list of registered peer...
	if len(token) > 0 {
		claims, err := security.ValidateToken(token)
		if err != nil && !hasAccess{
			log(domain, application, clientId, method, Utility.FileLine(), Utility.FunctionName(), "fail to validate token for method "+method+" with error "+err.Error(), logpb.LogLevel_ERROR_MESSAGE)
			fmt.Println(token)
			return err
		}

		clientId = claims.Id
		issuer = claims.Issuer
	}

	// The uuid will be use to set hasAccess into the cache.
	uuid := Utility.RandomUUID()

	// Start streaming.
	err = handler(srv, ServerStreamInterceptorStream{uuid: uuid, inner: stream, method: method, address: address, token: token, application: application, clientId: clientId, peer: issuer})

	if err != nil {
		log(domain, application, clientId, method, Utility.FileLine(), Utility.FunctionName(), err.Error(), logpb.LogLevel_ERROR_MESSAGE)
	}

	// Remove the uuid from the cache
	cache.Delete(uuid)

	return err
}
