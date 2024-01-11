package interceptors

// TODO for the validation, use a map to store valid method/token/resource/access
// the validation will be renew only if the token expire. And when a token expire
// the value in the map will be discard. That way it will put less charge on the server
// side.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/davecourtois/Utility"
	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/globular_client"
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

	// That will contain the permission in memory to limit the number
	// of resource request...
	// TODO made use of real cache instead of a memory map to limit the memory usage...
	cache sync.Map

	// That will contain the permission in memory to limit the number
	resourceInfos sync.Map
)

func GetLogClient(address string) (*log_client.Log_Client, error) {
	Utility.RegisterFunction("NewLogService_Client", log_client.NewLogService_Client)
	client, err := globular_client.GetClient(address, "log.LogService", "NewLogService_Client")
	if err != nil {
		return nil, err
	}
	return client.(*log_client.Log_Client), nil
}

/**
 * Get the rbac client.
 */
func GetRbacClient(address string) (*rbac_client.Rbac_Client, error) {
	Utility.RegisterFunction("NewRbacService_Client", rbac_client.NewRbacService_Client)

	client, err := globular_client.GetClient(address, "rbac.RbacService", "NewRbacService_Client")
	if err != nil {
		return nil, err
	}
	return client.(*rbac_client.Rbac_Client), nil
}

/**
 * Keep method info in memory.
 */
func getActionResourceInfos(address, method string) ([]*rbacpb.ResourceInfos, error) {

	// init the resourceInfos
	val, ok := resourceInfos.Load(method)
	if ok {
		return val.([]*rbacpb.ResourceInfos), nil
	}

	rbac_client_, err := GetRbacClient(address)
	if err != nil {
		return nil, err
	}

	//do something here
	infos, err := rbac_client_.GetActionResourceInfos(method)
	if err != nil {
		return nil, err
	}

	resourceInfos.Store(method, infos)

	return infos, nil

}

func ValidateSubjectSpace(subject, address string, subjectType rbacpb.SubjectType, required_space uint64) (bool, error) {
	rbac_client_, err := GetRbacClient(address)
	if err != nil {
		return false, err
	}
	hasSpace, err := rbac_client_.ValidateSubjectSpace(subject, subjectType, required_space)
	return hasSpace, err
}

func validateAction(token, application, address, organization, method, subject string, subjectType rbacpb.SubjectType, infos []*rbacpb.ResourceInfos) (bool, bool, error) {

	// Here I will test if the subject is the super admin...
	domain, _ := config.GetDomain()
	if !strings.Contains(subject, "@") {
		subject = subject + "@" + domain
	}

	if subject == "sa@"+domain {
		return true, false, nil
	}

	id := address + method + token
	for i := 0; i < len(infos); i++ {
		id += infos[i].Permission + infos[i].Path
	}

	// generate a uuid for the action and it's resource permissions.
	uuid := Utility.GenerateUUID(id)
	item, ok := cache.Load(uuid)
	if ok {
		// Here I will test if the permission has expired...
		hasAccess_ := item.(map[string]interface{})
		expiredAt := time.Unix(hasAccess_["expiredAt"].(int64), 0)
		hasAccess__ := hasAccess_["hasAccess"].(bool)
		if time.Now().Before(expiredAt) && hasAccess__ {
			return true, false, nil
		}
		// the token is expire...
		cache.Delete(uuid)
	}

	rbac_client_, err := GetRbacClient(address)
	if err != nil {
		fmt.Println("fail to connecto the the rbac service!")
		return false, false, err
	}

	hasAccess, accessDenied, err := rbac_client_.ValidateAction(method, subject, subjectType, infos)
	if err != nil {
		return hasAccess, accessDenied, err
	}

	// Here I will set the access in the cache.
	cache.Store(uuid, map[string]interface{}{"hasAccess": hasAccess, "expiredAt": time.Now().Add(time.Minute * 15).Unix()})

	return hasAccess, accessDenied, nil

}

func validateActionRequest(token string, application string, organization string, rqst interface{}, method string, subject string, subjectType rbacpb.SubjectType, domain string) (bool, bool, error) {

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
				if param.Kind() == protoreflect.MessageKind && len(infos[i].Field) > 0 {
					infos[i].Path, _ = url.PathUnescape(val.Message().Get(param.Message().Fields().ByTextName(infos[i].Field)).String())
				} else if param.IsList() {
					infos_ := make([]*rbacpb.ResourceInfos, val.List().Len())
					for j := 0; j < val.List().Len(); j++ {
						val_ := val.List().Get(j).String()
						infos_[j] = new(rbacpb.ResourceInfos)
						infos_[j].Path, _ = url.PathUnescape(val_)
						infos_[j].Index = infos[i].Index
						infos_[j].Permission = infos[i].Permission
					}

					hasAccess, accessDenied, err := validateAction(token, application, domain, organization, method, subject, subjectType, infos_)

					if err != nil {
						return hasAccess, accessDenied, err
					}

					return hasAccess, accessDenied, nil
				} else {
					infos[i].Path, _ = url.PathUnescape(val.String())
				}

			}
		}
	}

	// TODO keep to value in cache for keep speed.
	hasAccess, accessDenied, err := validateAction(token, application, domain, organization, method, subject, subjectType, infos)

	if err != nil {
		return hasAccess, accessDenied, err
	}

	// Here I will store the permission for further use...
	return hasAccess, accessDenied, nil
}

// Log the error...
func log(domain, application, user, method, fileLine, functionName string, msg string, level logpb.LogLevel) {
	logger, _ := GetLogClient(domain)
	if logger != nil {
		logger.Log(application, user, method, level, msg, fileLine, functionName)
	}
}

// Get the client.
func getClient(address, serviceName string) (globular_client.Client, error) {

	uuid := Utility.GenerateUUID(address + serviceName)

	// Here I will test if the client is already in the cache.
	item, ok := cache.Load(uuid)
	if ok {
		// Here I will test if the permission has expired...
		client := item.(globular_client.Client)
		return client, nil
	}

	fct := "New" + serviceName[strings.Index(serviceName, ".")+1:] + "_Client"

	client, err := globular_client.GetClient(address, serviceName, fct)
	if err != nil {
		return nil, err
	}

	// Here I will set the client in the cache.
	cache.Store(uuid, client)

	return client, nil
}

// The round robin policy unary method handler.
func roundRobinUnaryMethodHandler(ctx context.Context, method string, rqst interface{}) (interface{}, error) {

	config_, err := config.GetLocalConfig(true)
	if err != nil {
		return nil, err
	}

	// Here I will get the list of peers...
	peers := config_["Peers"].([]interface{})
	if len(peers) == 0 {
		return nil, errors.New("no peers found")
	}

	// I will get the round robin index for the method.
	index, ok := cache.Load("roundRobinIndex_" + method)
	if !ok {
		index = 0
	}

	// Here I will test if the index is -1, if it is I will force the method to be call locally.
	if index.(int) == -1 {
		index = 0
		cache.Store("roundRobinIndex_"+method, index)
		return nil, errors.New("force method to be cal locally")
	}

	// display the peers information...
	peer := peers[index.(int)].(map[string]interface{})
	address := peer["Hostname"].(string) + "." + peer["Domain"].(string) + ":" + Utility.ToString(peer["Port"])
	client, err := getClient(address, method[1:][0:strings.Index(method[1:], "/")])
	if err != nil {
		return nil, err
	}

	// Here I will call the method on the peer.
	rsp, err := client.Invoke(method, rqst, ctx)
	if err != nil {
		return nil, err
	}

	// Here I will increment the index.
	index = index.(int) + 1
	if index.(int) >= len(peers) {
		index = -1
	}

	// Here I will set the index in the cache.
	cache.Store("roundRobinIndex_"+method, index)

	return rsp, nil
}

// That interceptor is use by all services to apply the dynamic method routing.
func handleUnaryMethod(routing string, ctx context.Context, method string, rqst interface{}) (interface{}, error) {

	// Here I will apply the policy.
	if routing == "round-robin" {
		return roundRobinUnaryMethodHandler(ctx, method, rqst)
	}

	return nil, errors.New("fail to invoke method " + method + " not implemented")
}

// That interceptor is use by all services except the resource service who has
// it own interceptor.
func ServerUnaryInterceptor(ctx context.Context, rqst interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {

	// The token and the application id.
	var token string
	var application string
	var organization string

	// The peer domain.
	address, err := config.GetAddress()
	if err != nil {
		return nil, err
	}

	if len(address) == 0 {
		return nil, errors.New("fail to get the address")
	}

	// Here I will test if the
	method := info.FullMethod

	if md, ok := metadata.FromIncomingContext(ctx); ok {
		// The application...
		application = strings.Join(md["application"], "")
		token = strings.Join(md["token"], "")
	}

	// If the call come from a local client it has hasAccess
	hasAccess := true
	accessDenied := false

	// Set the list of restricted method here...
	if method == "/services_manager.ServicesManagerServices/GetServicesConfig" || method == "/rbac.RbacService/SetSubjectAllocatedSpace/" {
		hasAccess = false
	}

	var clientId string
	var issuer string

	if len(token) > 0 {
		claims, err := security.ValidateToken(token)
		if err != nil && !hasAccess {
			log(address, application, clientId, method, Utility.FileLine(), Utility.FunctionName(), "fail to validate token for method "+method+" with error "+err.Error(), logpb.LogLevel_ERROR_MESSAGE)
			return nil, err
		}

		if len(claims.Domain) == 0 {
			return nil, errors.New("fail to validate token for method " + method + " with error " + err.Error())
		}
		clientId = claims.Id + "@" + claims.UserDomain
		issuer = claims.Issuer
	}

	if method != "/rbac.RbacService/GetActionResourceInfos" {
		infos, err := getActionResourceInfos(address, method)
		if err == nil && infos != nil {
			hasAccess = false
		}
	}

	if !hasAccess && len(clientId) > 0 {
		hasAccess, accessDenied, _ = validateActionRequest(token, application, organization, rqst, method, clientId, rbacpb.SubjectType_ACCOUNT, address)
		/** Append other method that need disk space here **/
		if method == "/torrent.TorrentService/DownloadTorrent" {
			// Test if the space is not already bust...
			ValidateSubjectSpace(clientId, address, rbacpb.SubjectType_ACCOUNT, 0)
		}
	}

	if !hasAccess && len(application) > 0 && accessDenied {
		hasAccess, accessDenied, _ = validateActionRequest(token, application, organization, rqst, method, application, rbacpb.SubjectType_APPLICATION, address)
	}

	if !hasAccess && len(issuer) > 0 && !accessDenied {
		macAddress, _ := config.GetMacAddress()
		if issuer != macAddress {
			hasAccess, accessDenied, _ = validateActionRequest(token, application, organization, rqst, method, issuer, rbacpb.SubjectType_PEER, address)
		}
	}

	if !hasAccess || accessDenied {
		err := errors.New("Permission denied to execute method " + method + " user:" + clientId + " address:" + address + " application:" + application)

		log(address, application, clientId, method, Utility.FileLine(), Utility.FunctionName(), err.Error(), logpb.LogLevel_ERROR_MESSAGE)
		return nil, err
	}

	var result interface{}

	// Here I will try to call the method dynamically.
	result, err = handleUnaryMethod(ctx, method, rqst)
	if err == nil {
		return result, err
	}

	// I will call the real method.
	result, err = handler(ctx, rqst)

	// Send log message.
	if err != nil {
		log(address, application, clientId, method, Utility.FileLine(), Utility.FunctionName(), err.Error(), logpb.LogLevel_ERROR_MESSAGE)
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
	hasAccess := false
	accessDenied := false

	// Here I will test if the method is in the list of method that don't need access validation.
	if l.method == "/resource.ResourceService/GetRoles" ||
		l.method == "/resource.ResourceService/GetAccounts" ||
		l.method == "/resource.ResourceService/GetOrganizations" ||
		l.method == "/resource.ResourceService/GetApplications" ||
		l.method == "/resource.ResourceService/GetPeers" ||
		l.method == "/resource.ResourceService/GetGroups" ||
		l.method == "/admin.AdminService/DownloadGlobular" ||
		l.method == "/admin.AdminService/GetProcessInfos" ||
		l.method == "/repository.PackageRepository/DownloadBundle" {
		return nil
	}

	//fmt.Print("ServerStreamInterceptorStream.RecvMsg ", l.method, " ", l.application, " ", l.token, " ", l.address, " ", l.clientId, " ", l.peer, " ", l.uuid, "\n")

	// if the cache contain the uuid it means permission is allowed
	_, ok := cache.Load(l.uuid)
	if ok {
		//fmt.Println("permission found in cache user " + l.clientId + " has permission to execute method: " + l.method + " domain:" + l.address + " application:" + l.application)
		return nil
	}

	// fmt.Println("validate permission cache user " + l.clientId + " has permission to execute method: " + l.method + " domain:" + l.address + " application:" + l.application)
	// Test if peer has access
	if !hasAccess && len(l.clientId) > 0 {
		hasAccess, accessDenied, _ = validateActionRequest(l.token, l.application, l.organization, rqst, l.method, l.clientId, rbacpb.SubjectType_ACCOUNT, l.address)
	}

	if !hasAccess && len(l.application) > 0 && !accessDenied {
		hasAccess, accessDenied, _ = validateActionRequest(l.token, l.application, l.organization, rqst, l.method, l.application, rbacpb.SubjectType_APPLICATION, l.address)
	}

	if !hasAccess && len(l.peer) > 0 && !accessDenied {
		hasAccess, accessDenied, _ = validateActionRequest(l.token, l.application, l.organization, rqst, l.method, l.peer, rbacpb.SubjectType_PEER, l.address)
	}

	if !hasAccess || accessDenied {
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
	address, err := config.GetAddress()
	if err != nil {
		return err
	}

	if len(address) == 0 {
		return errors.New("fail to get the address")
	}

	method := info.FullMethod

	if md, ok := metadata.FromIncomingContext(stream.Context()); ok {
		application = strings.Join(md["application"], "")
		token = strings.Join(md["token"], "")

		// In that case the request must be process by other peer so I will redirect
		// the request to that peer and return it response.
		//address = strings.ToLower(strings.TrimSpace(strings.Join(md["domain"], "")))
	}

	var clientId string
	var issuer string
	hasAccess := true

	// TODO set method the require access validation here....

	// Here I will get the peer mac address from the list of registered peer...
	if len(token) > 0 {
		claims, err := security.ValidateToken(token)
		if err != nil && !hasAccess {
			log(address, application, clientId, method, Utility.FileLine(), Utility.FunctionName(), "fail to validate token for method "+method+" with error "+err.Error(), logpb.LogLevel_ERROR_MESSAGE)
			fmt.Println(token)
			return err
		}

		if len(claims.Domain) == 0 {
			return errors.New("fail to validate token for method " + method + " with error " + err.Error())
		}

		clientId = claims.Id + "@" + claims.UserDomain
		issuer = claims.Issuer
	}

	//fmt.Println("---------> ServerStreamInterceptor", method, application, token, address)

	// The uuid will be use to set hasAccess into the cache.
	uuid := Utility.RandomUUID()

	// Start streaming.
	err = handler(srv, ServerStreamInterceptorStream{uuid: uuid, inner: stream, method: method, address: address, token: token, application: application, clientId: clientId, peer: issuer})

	if err != nil {
		log(address, application, clientId, method, Utility.FileLine(), Utility.FunctionName(), err.Error(), logpb.LogLevel_ERROR_MESSAGE)
	}

	// Remove the uuid from the cache
	cache.Delete(uuid)

	return err
}
