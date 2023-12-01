package main

import (
	"context"
	"encoding/json"
	"errors"

	//"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/davecourtois/Utility"
	"github.com/globulario/services/golang/catalog/catalogpb"
	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/event/event_client"
	"github.com/globulario/services/golang/globular_client"
	globular "github.com/globulario/services/golang/globular_service"
	"github.com/globulario/services/golang/interceptors"
	"github.com/globulario/services/golang/persistence/persistence_client"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"

	// "google.golang.org/grpc/grpclog"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"
)

// TODO take care of TLS/https
var (
	defaultPort  = 10017
	defaultProxy = 10018

	// By default all origins are allowed.
	allow_all_origins = true

	// comma separeated values.
	allowed_origins string = ""
)

// Value need by Globular to start the services...
type server struct {
	// The global attribute of the services.
	Id              string
	Name            string
	Mac             string
	Port            int
	Proxy           int
	Path            string
	Proto           string
	AllowAllOrigins bool
	AllowedOrigins  string // comma separated string.
	Protocol        string
	Domain          string
	Address         string
	Description     string
	Keywords        []string
	Repositories    []string
	Discoveries     []string
	Process         int
	ProxyProcess    int
	ConfigPath      string
	State           string
	LastError       string

	// svr-signed X.509 public keys for distribution
	CertFile string
	// a private RSA key to sign and authenticate the public key
	KeyFile string
	// a private RSA key to sign and authenticate the public key
	CertAuthorityTrust string
	TLS                bool
	Version            string
	PublisherId        string
	KeepUpToDate       bool
	KeepAlive          bool
	Checksum           string
	Plaform            string
	ModTime            int64

	// Contain the list of service use by the catalog srv.
	Services map[string]interface{}

	Permissions  []interface{}
	Dependencies []string // The list of services needed by this services.

	// Here I will create client to services use by the catalog srv.
	persistenceClient *persistence_client.Persistence_Client
	eventClient       *event_client.Event_Client
	// The grpc server.
	grpcServer *grpc.Server
}

// The path of the configuration.
func (srv *server) GetConfigurationPath() string {
	return srv.ConfigPath
}

func (srv *server) SetConfigurationPath(path string) {
	srv.ConfigPath = path
}

// The http address where the configuration can be found /config
func (srv *server) GetAddress() string {
	return srv.Address
}

func (srv *server) SetAddress(address string) {
	srv.Address = address
}

func (srv *server) GetProcess() int {
	return srv.Process
}

func (srv *server) SetProcess(pid int) {
	srv.Process = pid
}

func (srv *server) GetProxyProcess() int {
	return srv.ProxyProcess
}

func (srv *server) SetProxyProcess(pid int) {
	srv.ProxyProcess = pid
}

// The current service state
func (srv *server) GetState() string {
	return srv.State
}

func (srv *server) SetState(state string) {
	srv.State = state
}

// The last error
func (srv *server) GetLastError() string {
	return srv.LastError
}

func (srv *server) SetLastError(err string) {
	srv.LastError = err
}

// The modeTime
func (srv *server) SetModTime(modtime int64) {
	srv.ModTime = modtime
}
func (srv *server) GetModTime() int64 {
	return srv.ModTime
}

// Globular services implementation...
// The id of a particular service instance.
func (srv *server) GetId() string {
	return srv.Id
}

func (srv *server) SetId(id string) {
	srv.Id = id
}

// The name of a service, must be the gRpc Service name.
func (srv *server) GetName() string {
	return srv.Name
}
func (srv *server) SetName(name string) {
	srv.Name = name
}

func (srv *server) GetMac() string {
	return srv.Mac
}

func (srv *server) SetMac(mac string) {
	srv.Mac = mac
}

func (srv *server) GetChecksum() string {

	return srv.Checksum
}

func (srv *server) SetChecksum(checksum string) {
	srv.Checksum = checksum
}

func (srv *server) GetPlatform() string {
	return srv.Plaform
}

func (srv *server) SetPlatform(platform string) {
	srv.Plaform = platform
}

// The description of the service
func (srv *server) GetDescription() string {
	return srv.Description
}
func (srv *server) SetDescription(description string) {
	srv.Description = description
}

// The list of keywords of the services.
func (srv *server) GetKeywords() []string {
	return srv.Keywords
}
func (srv *server) SetKeywords(keywords []string) {
	srv.Keywords = keywords
}

// Dist
func (srv *server) Dist(path string) (string, error) {

	return globular.Dist(path, srv)
}

func (srv *server) GetDependencies() []string {

	if srv.Dependencies == nil {
		srv.Dependencies = make([]string, 0)
	}

	return srv.Dependencies
}

func (srv *server) SetDependency(dependency string) {
	if srv.Dependencies == nil {
		srv.Dependencies = make([]string, 0)
	}

	// Append the depency to the list.
	if !Utility.Contains(srv.Dependencies, dependency) {
		srv.Dependencies = append(srv.Dependencies, dependency)
	}
}

func (srv *server) GetRepositories() []string {
	return srv.Repositories
}
func (srv *server) SetRepositories(repositories []string) {
	srv.Repositories = repositories
}

func (srv *server) GetDiscoveries() []string {
	return srv.Discoveries
}
func (srv *server) SetDiscoveries(discoveries []string) {
	srv.Discoveries = discoveries
}

// The path of the executable.
func (srv *server) GetPath() string {
	return srv.Path
}
func (srv *server) SetPath(path string) {
	srv.Path = path
}

// The path of the .proto file.
func (srv *server) GetProto() string {
	return srv.Proto
}
func (srv *server) SetProto(proto string) {
	srv.Proto = proto
}

// The gRpc port.
func (srv *server) GetPort() int {
	return srv.Port
}
func (srv *server) SetPort(port int) {
	srv.Port = port
}

// The reverse proxy port (use by gRpc Web)
func (srv *server) GetProxy() int {
	return srv.Proxy
}
func (srv *server) SetProxy(proxy int) {
	srv.Proxy = proxy
}

// Can be one of http/https/tls
func (srv *server) GetProtocol() string {
	return srv.Protocol
}
func (srv *server) SetProtocol(protocol string) {
	srv.Protocol = protocol
}

// Return true if all Origins are allowed to access the mircoservice.
func (srv *server) GetAllowAllOrigins() bool {
	return srv.AllowAllOrigins
}
func (srv *server) SetAllowAllOrigins(allowAllOrigins bool) {
	srv.AllowAllOrigins = allowAllOrigins
}

// If AllowAllOrigins is false then AllowedOrigins will contain the
// list of address that can reach the services.
func (srv *server) GetAllowedOrigins() string {
	return srv.AllowedOrigins
}

func (srv *server) SetAllowedOrigins(allowedOrigins string) {
	srv.AllowedOrigins = allowedOrigins
}

// Can be a ip address or domain name.
func (srv *server) GetDomain() string {
	return srv.Domain
}
func (srv *server) SetDomain(domain string) {
	srv.Domain = domain
}

// TLS section

// If true the service run with TLS. The
func (srv *server) GetTls() bool {
	return srv.TLS
}
func (srv *server) SetTls(hasTls bool) {
	srv.TLS = hasTls
}

// The certificate authority file
func (srv *server) GetCertAuthorityTrust() string {
	return srv.CertAuthorityTrust
}

func (srv *server) SetCertAuthorityTrust(ca string) {
	srv.CertAuthorityTrust = ca
}

// The certificate file.
func (srv *server) GetCertFile() string {
	return srv.CertFile
}
func (srv *server) SetCertFile(certFile string) {
	srv.CertFile = certFile
}

// The key file.
func (srv *server) GetKeyFile() string {
	return srv.KeyFile
}
func (srv *server) SetKeyFile(keyFile string) {
	srv.KeyFile = keyFile
}

// The service version
func (srv *server) GetVersion() string {
	return srv.Version
}
func (srv *server) SetVersion(version string) {
	srv.Version = version
}

// The publisher id.
func (srv *server) GetPublisherId() string {
	return srv.PublisherId
}
func (srv *server) SetPublisherId(publisherId string) {
	srv.PublisherId = publisherId
}

func (srv *server) GetKeepUpToDate() bool {
	return srv.KeepUpToDate
}
func (srv *server) SetKeepUptoDate(val bool) {
	srv.KeepUpToDate = val
}

func (srv *server) GetKeepAlive() bool {
	return srv.KeepAlive
}
func (srv *server) SetKeepAlive(val bool) {
	srv.KeepAlive = val
}

func (srv *server) GetPermissions() []interface{} {
	return srv.Permissions
}
func (srv *server) SetPermissions(permissions []interface{}) {
	srv.Permissions = permissions
}

func GetPersistenceClient(address string) (*persistence_client.Persistence_Client, error) {
	Utility.RegisterFunction("NewPersistenceService_Client", persistence_client.NewPersistenceService_Client)
	client, err := globular_client.GetClient(address, "persistence.PersistenceService", "NewPersistenceService_Client")
	if err != nil {
		return nil, err
	}
	return client.(*persistence_client.Persistence_Client), nil
}

func GetEventClient(address string) (*event_client.Event_Client, error) {
	Utility.RegisterFunction("NewEventService_Client", event_client.NewEventService_Client)
	client, err := globular_client.GetClient(address, "event.EventService", "NewEventService_Client")
	if err != nil {
		return nil, err
	}
	return client.(*event_client.Event_Client), nil
}

// Create the configuration file if is not already exist.
func (srv *server) Init() error {

	// Get the configuration path.
	err := globular.InitService(srv)
	if err != nil {
		return err
	}

	// Initialyse GRPC srv.
	srv.grpcServer, err = globular.InitGrpcServer(srv, interceptors.ServerUnaryInterceptor, interceptors.ServerStreamInterceptor)
	if err != nil {
		return err
	}

	// Connect to the persistence service.
	if srv.Services["Persistence"] != nil {
		persistence_service := srv.Services["Persistence"].(map[string]interface{})
		address := persistence_service["Address"].(string)
		srv.persistenceClient, err = GetPersistenceClient(address)
		if err != nil {
			log.Println("fail to connect to persistence service ", err)
		}
	}

	if srv.Services["Event"] != nil {
		event_service := srv.Services["Event"].(map[string]interface{})
		address := event_service["Address"].(string)
		srv.eventClient, err = GetEventClient(address)
		if err != nil {
			log.Println("fail to connect to event service ", err)
		}
	}

	return nil

}

// Save the configuration values.
func (srv *server) Save() error {
	// Create the file...
	return globular.SaveService(srv)
}

func (srv *server) StartService() error {
	return globular.StartService(srv, srv.grpcServer)
}

func (srv *server) StopService() error {
	return globular.StopService(srv, srv.grpcServer)
}

func (srv *server) Stop(context.Context, *catalogpb.StopRequest) (*catalogpb.StopResponse, error) {
	return &catalogpb.StopResponse{}, srv.StopService()
}

// Create a new connection.
func (srv *server) CreateConnection(ctx context.Context, rqst *catalogpb.CreateConnectionRqst) (*catalogpb.CreateConnectionRsp, error) {
	if rqst.Connection == nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("no connection information found in the request!")))

	}
	// So here I will call the function on the client.
	if srv.persistenceClient != nil {
		persistence := srv.Services["Persistence"].(map[string]interface{})
		if persistence["Connections"] == nil {
			persistence["Connections"] = make(map[string]interface{})
		}

		connections := persistence["Connections"].(map[string]interface{})

		storeType := int32(rqst.Connection.GetStore())
		err := srv.persistenceClient.CreateConnection(rqst.Connection.GetId(), rqst.Connection.GetName(), rqst.Connection.GetHost(), Utility.ToNumeric(rqst.Connection.Port), Utility.ToNumeric(storeType), rqst.Connection.GetUser(), rqst.Connection.GetPassword(), Utility.ToNumeric(rqst.Connection.GetTimeout()), rqst.Connection.GetOptions(), false)
		if err != nil {
			return nil, status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}

		connection := make(map[string]interface{})
		connection["Id"] = rqst.Connection.GetId()
		connection["Name"] = rqst.Connection.GetName()
		connection["Host"] = rqst.Connection.GetHost()
		connection["Store"] = rqst.Connection.GetStore()
		connection["User"] = rqst.Connection.GetUser()
		connection["Password"] = rqst.Connection.GetPassword()
		connection["Port"] = rqst.Connection.GetPort()
		connection["Timeout"] = rqst.Connection.GetTimeout()
		connection["Options"] = rqst.Connection.GetOptions()

		connections[rqst.Connection.GetId()] = connection

		srv.Save()

	}

	return &catalogpb.CreateConnectionRsp{
		Result: true,
	}, nil
}

// Delete a connection.
func (srv *server) DeleteConnection(ctx context.Context, rqst *catalogpb.DeleteConnectionRqst) (*catalogpb.DeleteConnectionRsp, error) {
	return nil, nil
}

// Create unit of measure exemple inch
func (srv *server) SaveUnitOfMeasure(ctx context.Context, rqst *catalogpb.SaveUnitOfMeasureRequest) (*catalogpb.SaveUnitOfMeasureResponse, error) {
	unitOfMeasure := rqst.GetUnitOfMeasure()

	persistence := srv.Services["Persistence"].(map[string]interface{})

	if persistence["Connections"].(map[string]interface{})[rqst.ConnectionId] == nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("no connection found with id "+rqst.ConnectionId)))
	}

	connection := persistence["Connections"].(map[string]interface{})[rqst.ConnectionId].(map[string]interface{})

	// Here I will generate the _id key
	_id := Utility.GenerateUUID(unitOfMeasure.Id + unitOfMeasure.LanguageCode)
	srv.persistenceClient.DeleteOne(connection["Id"].(string), connection["Name"].(string), "UnitOfMeasure", `{ "_id" : "`+_id+`" }`, "")

	data, err := protojson.Marshal(unitOfMeasure)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))

	}

	jsonStr := `{ "_id" : "` + _id + `",` + string(data)[1:]

	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	id, err := srv.persistenceClient.InsertOne(connection["Id"].(string), connection["Name"].(string), "UnitOfMeasure", jsonStr, "")

	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &catalogpb.SaveUnitOfMeasureResponse{
		Id: id,
	}, nil
}

// Create property definition return the uuid of the created property
func (srv *server) SavePropertyDefinition(ctx context.Context, rqst *catalogpb.SavePropertyDefinitionRequest) (*catalogpb.SavePropertyDefinitionResponse, error) {
	propertyDefinition := rqst.PropertyDefinition

	persistence := srv.Services["Persistence"].(map[string]interface{})

	if persistence["Connections"].(map[string]interface{})[rqst.ConnectionId] == nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("no connection found with id "+rqst.ConnectionId)))
	}

	connection := persistence["Connections"].(map[string]interface{})[rqst.ConnectionId].(map[string]interface{})

	// Here I will generate the _id key
	_id := Utility.GenerateUUID(propertyDefinition.Id + propertyDefinition.LanguageCode)

	data, err := protojson.Marshal(propertyDefinition)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))

	}

	jsonStr := `{ "_id" : "` + _id + `",` + string(data)[1:]

	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	err = srv.persistenceClient.ReplaceOne(connection["Id"].(string), connection["Name"].(string), "PropertyDefinition", `{ "_id" : "`+_id+`"}`, jsonStr, `[{"upsert": true}]`)

	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &catalogpb.SavePropertyDefinitionResponse{
		Id: _id,
	}, nil
}

// Create item definition.
func (srv *server) SaveItemDefinition(ctx context.Context, rqst *catalogpb.SaveItemDefinitionRequest) (*catalogpb.SaveItemDefinitionResponse, error) {
	itemDefinition := rqst.ItemDefinition

	persistence := srv.Services["Persistence"].(map[string]interface{})

	if persistence["Connections"].(map[string]interface{})[rqst.ConnectionId] == nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("no connection found with id "+rqst.ConnectionId)))
	}

	connection := persistence["Connections"].(map[string]interface{})[rqst.ConnectionId].(map[string]interface{})

	// Here I will generate the _id key
	_id := Utility.GenerateUUID(itemDefinition.Id + itemDefinition.LanguageCode)

	data, err := protojson.Marshal(itemDefinition)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))

	}

	jsonStr := `{ "_id" : "` + _id + `",` + string(data)[1:]

	// Set the db reference.
	jsonStr = strings.Replace(jsonStr, "refColId", "$ref", -1)
	jsonStr = strings.Replace(jsonStr, "refObjId", "$id", -1)
	jsonStr = strings.Replace(jsonStr, "refDbName", "$db", -1)

	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	err = srv.persistenceClient.ReplaceOne(connection["Id"].(string), connection["Name"].(string), "ItemDefinition", `{ "_id" : "`+_id+`"}`, jsonStr, `[{"upsert": true}]`)

	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &catalogpb.SaveItemDefinitionResponse{
		Id: _id,
	}, nil
}

// Create item response request.
func (srv *server) SaveInventory(ctx context.Context, rqst *catalogpb.SaveInventoryRequest) (*catalogpb.SaveInventoryResponse, error) {
	inventory := rqst.Inventory
	persistence := srv.Services["Persistence"].(map[string]interface{})

	if persistence["Connections"].(map[string]interface{})[rqst.ConnectionId] == nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("no connection found with id "+rqst.ConnectionId)))
	}

	connection := persistence["Connections"].(map[string]interface{})[rqst.ConnectionId].(map[string]interface{})

	data, err := protojson.Marshal(inventory)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))

	}
	_id := Utility.GenerateUUID(inventory.LocalisationId + inventory.PacakgeId)
	jsonStr := `{ "_id" : "` + _id + `",` + string(data)[1:]

	// Set the db reference.
	jsonStr = strings.Replace(jsonStr, "refColId", "$ref", -1)
	jsonStr = strings.Replace(jsonStr, "refObjId", "$id", -1)
	jsonStr = strings.Replace(jsonStr, "refDbName", "$db", -1)

	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// Always create a new
	err = srv.persistenceClient.ReplaceOne(connection["Id"].(string), connection["Name"].(string), "Inventory", `{ "_id" : "`+_id+`"}`, jsonStr, `[{"upsert": true}]`)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &catalogpb.SaveInventoryResponse{
		Id: _id,
	}, nil
}

// Create item response request.
func (srv *server) SaveItemInstance(ctx context.Context, rqst *catalogpb.SaveItemInstanceRequest) (*catalogpb.SaveItemInstanceResponse, error) {
	instance := rqst.ItemInstance
	persistence := srv.Services["Persistence"].(map[string]interface{})

	if persistence["Connections"].(map[string]interface{})[rqst.ConnectionId] == nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("no connection found with id "+rqst.ConnectionId)))
	}

	connection := persistence["Connections"].(map[string]interface{})[rqst.ConnectionId].(map[string]interface{})

	data, err := protojson.Marshal(instance)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))

	}

	// Here I will generate the _id key
	_id := Utility.GenerateUUID(instance.Id)
	jsonStr := `{ "_id" : "` + _id + `",` + string(data)[1:]


	// Set the db reference.
	jsonStr = strings.Replace(jsonStr, "refColId", "$ref", -1)
	jsonStr = strings.Replace(jsonStr, "refObjId", "$id", -1)
	jsonStr = strings.Replace(jsonStr, "refDbName", "$db", -1)

	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// Always create a new
	err = srv.persistenceClient.ReplaceOne(connection["Id"].(string), connection["Name"].(string), "ItemInstance", `{ "_id" : "`+_id+`"}`, jsonStr, `[{"upsert": true}]`)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &catalogpb.SaveItemInstanceResponse{
		Id: _id,
	}, nil
}

// Save Manufacturer
func (srv *server) SaveManufacturer(ctx context.Context, rqst *catalogpb.SaveManufacturerRequest) (*catalogpb.SaveManufacturerResponse, error) {

	persistence := srv.Services["Persistence"].(map[string]interface{})

	if persistence["Connections"].(map[string]interface{})[rqst.ConnectionId] == nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("no connection found with id "+rqst.ConnectionId)))
	}

	connection := persistence["Connections"].(map[string]interface{})[rqst.ConnectionId].(map[string]interface{})

	manufacturer := rqst.Manufacturer

	// Here I will generate the _id key
	_id := Utility.GenerateUUID(manufacturer.Id)

	data, err := protojson.Marshal(manufacturer)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))

	}

	// Here I will generate the _id key
	jsonStr := `{ "_id" : "` + _id + `",` + string(data)[1:]

	// Always create a new
	err = srv.persistenceClient.ReplaceOne(connection["Id"].(string), connection["Name"].(string), "Manufacturer", `{ "_id" : "`+_id+`"}`, jsonStr, `[{"upsert": true}]`)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// Append manafacturer item response.
	return &catalogpb.SaveManufacturerResponse{
		Id: _id,
	}, nil
}

// Save Supplier
func (srv *server) SaveSupplier(ctx context.Context, rqst *catalogpb.SaveSupplierRequest) (*catalogpb.SaveSupplierResponse, error) {
	persistence := srv.Services["Persistence"].(map[string]interface{})

	if persistence["Connections"].(map[string]interface{})[rqst.ConnectionId] == nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("no connection found with id "+rqst.ConnectionId)))
	}

	connection := persistence["Connections"].(map[string]interface{})[rqst.ConnectionId].(map[string]interface{})

	// save the supplier id
	supplier := rqst.Supplier

	// Here I will generate the _id key
	_id := Utility.GenerateUUID(supplier.Id)

	data, err := protojson.Marshal(supplier)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))

	}

	// Here I will generate the _id key
	jsonStr := `{ "_id" : "` + _id + `",` + string(data)[1:]

	// Always create a new
	err = srv.persistenceClient.ReplaceOne(connection["Id"].(string), connection["Name"].(string), "Supplier", `{ "_id" : "`+_id+`"}`, jsonStr, `[{"upsert": true}]`)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// Append manafacturer item response.
	return &catalogpb.SaveSupplierResponse{
		Id: _id,
	}, nil
}

// Save localisation
func (srv *server) SaveLocalisation(ctx context.Context, rqst *catalogpb.SaveLocalisationRequest) (*catalogpb.SaveLocalisationResponse, error) {
	persistence := srv.Services["Persistence"].(map[string]interface{})

	if persistence["Connections"].(map[string]interface{})[rqst.ConnectionId] == nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("no connection found with id "+rqst.ConnectionId)))
	}

	connection := persistence["Connections"].(map[string]interface{})[rqst.ConnectionId].(map[string]interface{})

	// save the supplier id
	localisation := rqst.Localisation

	// Set the reference correctly.
	if localisation.SubLocalisations != nil {
		for i := 0; i < len(localisation.SubLocalisations.Values); i++ {
			if !Utility.IsUuid(localisation.SubLocalisations.Values[i].GetRefObjId()) {
				localisation.SubLocalisations.Values[i].RefObjId = Utility.GenerateUUID(localisation.SubLocalisations.Values[i].GetRefObjId())
			}
		}
	}

	// Here I will generate the _id key
	_id := Utility.GenerateUUID(localisation.Id + localisation.LanguageCode)

	data, err := protojson.Marshal(localisation)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))

	}

	// Here I will generate the _id key
	jsonStr := `{ "_id" : "` + _id + `",` + string(data)[1:]

	// set the object references...
	jsonStr = strings.Replace(jsonStr, "refObjId", "$id", -1)
	jsonStr = strings.Replace(jsonStr, "refColId", "$ref", -1)
	jsonStr = strings.Replace(jsonStr, "refDbName", "$db", -1)

	// Always create a new
	err = srv.persistenceClient.ReplaceOne(connection["Id"].(string), connection["Name"].(string), "Localisation", `{ "_id" : "`+_id+`"}`, jsonStr, `[{"upsert": true}]`)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// Append manafacturer item response.
	return &catalogpb.SaveLocalisationResponse{
		Id: _id,
	}, nil
}

// Save Package
func (srv *server) SavePackage(ctx context.Context, rqst *catalogpb.SavePackageRequest) (*catalogpb.SavePackageResponse, error) {
	persistence := srv.Services["Persistence"].(map[string]interface{})

	if persistence["Connections"].(map[string]interface{})[rqst.ConnectionId] == nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("no connection found with id "+rqst.ConnectionId)))
	}

	connection := persistence["Connections"].(map[string]interface{})[rqst.ConnectionId].(map[string]interface{})

	// save the supplier id
	package_ := rqst.Package

	// Set the reference with _id value if not already set.
	for i := 0; i < len(package_.Subpackages); i++ {
		subPackage := package_.Subpackages[i]
		if subPackage.UnitOfMeasure != nil {
			if !Utility.IsUuid(subPackage.UnitOfMeasure.RefObjId) {
				subPackage.UnitOfMeasure.RefObjId = Utility.GenerateUUID(subPackage.UnitOfMeasure.RefObjId)
			}
		}
		if subPackage.Package != nil {
			if !Utility.IsUuid(subPackage.Package.RefObjId) {
				subPackage.Package.RefObjId = Utility.GenerateUUID(subPackage.Package.RefObjId)
			}
		}
	}

	for i := 0; i < len(package_.ItemInstances); i++ {
		itemInstance := package_.ItemInstances[i]
		if itemInstance.UnitOfMeasure != nil {
			if !Utility.IsUuid(itemInstance.UnitOfMeasure.RefObjId) {
				itemInstance.UnitOfMeasure.RefObjId = Utility.GenerateUUID(itemInstance.UnitOfMeasure.RefObjId)
			}
		}
		if !Utility.IsUuid(itemInstance.ItemInstance.RefObjId) {
			itemInstance.ItemInstance.RefObjId = Utility.GenerateUUID(itemInstance.ItemInstance.RefObjId)
		}
	}

	// Here I will generate the _id key
	_id := Utility.GenerateUUID(package_.Id + package_.LanguageCode)

	data, err := protojson.Marshal(package_)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))

	}

	// Here I will generate the _id key
	jsonStr := `{ "_id" : "` + _id + `",` + string(data)[1:]

	// set the object references...
	jsonStr = strings.Replace(jsonStr, "refObjId", "$id", -1)
	jsonStr = strings.Replace(jsonStr, "refColId", "$ref", -1)
	jsonStr = strings.Replace(jsonStr, "refDbName", "$db", -1)

	// Always create a new
	err = srv.persistenceClient.ReplaceOne(connection["Id"].(string), connection["Name"].(string), "Package", `{ "_id" : "`+_id+`"}`, jsonStr, `[{"upsert": true}]`)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// Append manafacturer item response.
	return &catalogpb.SavePackageResponse{
		Id: _id,
	}, nil

}

// Save Package Supplier
func (srv *server) SavePackageSupplier(ctx context.Context, rqst *catalogpb.SavePackageSupplierRequest) (*catalogpb.SavePackageSupplierResponse, error) {
	persistence := srv.Services["Persistence"].(map[string]interface{})

	if persistence["Connections"].(map[string]interface{})[rqst.ConnectionId] == nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("no connection found with id "+rqst.ConnectionId)))
	}

	connection := persistence["Connections"].(map[string]interface{})[rqst.ConnectionId].(map[string]interface{})
	// Now I will create the PackageSupplier.
	// save the supplier id
	packageSupplier := rqst.PackageSupplier

	if !Utility.IsUuid(packageSupplier.Supplier.RefObjId) {
		packageSupplier.Supplier.RefObjId = Utility.GenerateUUID(packageSupplier.Supplier.RefObjId)
	}

	if !Utility.IsUuid(packageSupplier.Package.RefObjId) {
		packageSupplier.Package.RefObjId = Utility.GenerateUUID(packageSupplier.Package.RefObjId)
	}

	// Test if the pacakge exist
	_, err := srv.persistenceClient.FindOne(connection["Id"].(string), rqst.PackageSupplier.Package.RefDbName, rqst.PackageSupplier.Package.RefColId, `{"_id":"`+rqst.PackageSupplier.Package.RefObjId+`"}`, "")
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// Test if the supplier exist.
	_, err = srv.persistenceClient.FindOne(connection["Id"].(string), rqst.PackageSupplier.Supplier.RefDbName, rqst.PackageSupplier.Supplier.RefColId, `{"_id":"`+rqst.PackageSupplier.Supplier.RefObjId+`"}`, "")
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// Here I will generate the _id key
	_id := Utility.GenerateUUID(packageSupplier.Id)

	data, err := protojson.Marshal(packageSupplier)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))

	}

	// Here I will generate the _id key
	jsonStr := `{ "_id" : "` + _id + `",` + string(data)[1:]
	

	// set the object references...
	jsonStr = strings.Replace(jsonStr, "refObjId", "$id", -1)
	jsonStr = strings.Replace(jsonStr, "refColId", "$ref", -1)
	jsonStr = strings.Replace(jsonStr, "refDbName", "$db", -1)

	// Always create a new
	err = srv.persistenceClient.ReplaceOne(connection["Id"].(string), connection["Name"].(string), "PackageSupplier", `{ "_id" : "`+_id+`"}`, jsonStr, `[{"upsert": true}]`)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &catalogpb.SavePackageSupplierResponse{
		Id: _id,
	}, nil
}

// Save Item Manufacturer
func (srv *server) SaveItemManufacturer(ctx context.Context, rqst *catalogpb.SaveItemManufacturerRequest) (*catalogpb.SaveItemManufacturerResponse, error) {
	persistence := srv.Services["Persistence"].(map[string]interface{})

	if persistence["Connections"].(map[string]interface{})[rqst.ConnectionId] == nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("no connection found with id "+rqst.ConnectionId)))
	}

	connection := persistence["Connections"].(map[string]interface{})[rqst.ConnectionId].(map[string]interface{})

	// Test if the item exist
	_, err := srv.persistenceClient.FindOne(connection["Id"].(string), rqst.ItemManafacturer.Item.RefDbName, rqst.ItemManafacturer.Item.RefColId, `{"_id":"`+rqst.ItemManafacturer.Item.RefObjId+`"}`, "")
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// Test if the supplier exist.
	_, err = srv.persistenceClient.FindOne(connection["Id"].(string), rqst.ItemManafacturer.Manufacturer.RefDbName, rqst.ItemManafacturer.Manufacturer.RefColId, `{"_id":"`+rqst.ItemManafacturer.Manufacturer.RefObjId+`"}`, "")
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// Now I will create the PackageSupplier.
	// save the supplier id
	itemManafacturer := rqst.ItemManafacturer

	// Here I will generate the _id key
	_id := Utility.GenerateUUID(itemManafacturer.Id)

	data, err := protojson.Marshal(itemManafacturer)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))

	}

	// Here I will generate the _id key
	jsonStr := `{ "_id" : "` + _id + `",` + string(data)[1:]

	// set the object references...
	jsonStr = strings.Replace(jsonStr, "refObjId", "$id", -1)
	jsonStr = strings.Replace(jsonStr, "refColId", "$ref", -1)
	jsonStr = strings.Replace(jsonStr, "refDbName", "$db", -1)

	// Always create a new
	err = srv.persistenceClient.ReplaceOne(connection["Id"].(string), connection["Name"].(string), "ItemManufacturer", `{ "_id" : "`+_id+`"}`, jsonStr, `[{"upsert": true}]`)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &catalogpb.SaveItemManufacturerResponse{
		Id: _id,
	}, nil
}

// Save Item Category
func (srv *server) SaveCategory(ctx context.Context, rqst *catalogpb.SaveCategoryRequest) (*catalogpb.SaveCategoryResponse, error) {
	persistence := srv.Services["Persistence"].(map[string]interface{})

	if persistence["Connections"].(map[string]interface{})[rqst.ConnectionId] == nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("no connection found with id "+rqst.ConnectionId)))
	}

	connection := persistence["Connections"].(map[string]interface{})[rqst.ConnectionId].(map[string]interface{})

	// save the supplier id
	category := rqst.Category

	// Here I will generate the _id key
	_id := Utility.GenerateUUID(category.Id + category.LanguageCode)


	data, err := protojson.Marshal(category)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))

	}

	// Here I will generate the _id key
	jsonStr := `{ "_id" : "` + _id + `",` + string(data)[1:]

	// Always create a new
	err = srv.persistenceClient.ReplaceOne(connection["Id"].(string), connection["Name"].(string), "Category", `{ "_id" : "`+_id+`"}`, jsonStr, `[{"upsert": true}]`)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// Append manafacturer item response.
	return &catalogpb.SaveCategoryResponse{
		Id: _id,
	}, nil
}

// Append a new Item to manufacturer
func (srv *server) AppendItemDefinitionCategory(ctx context.Context, rqst *catalogpb.AppendItemDefinitionCategoryRequest) (*catalogpb.AppendItemDefinitionCategoryResponse, error) {

	persistence := srv.Services["Persistence"].(map[string]interface{})

	if persistence["Connections"].(map[string]interface{})[rqst.ConnectionId] == nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("no connection found with id "+rqst.ConnectionId)))
	}

	connection := persistence["Connections"].(map[string]interface{})[rqst.ConnectionId].(map[string]interface{})

	data, err := protojson.Marshal(rqst.Category)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))

	}

	// Here I will generate the _id key
	jsonStr := string(data)

	// Set the db reference.
	jsonStr = strings.Replace(jsonStr, "refObjId", "$id", -1)
	jsonStr = strings.Replace(jsonStr, "refColId", "$ref", -1)
	jsonStr = strings.Replace(jsonStr, "refDbName", "$db", -1)

	// Now I will modify the jsonStr to insert the value in the array.
	jsonStr = `{ "$push": { "categories":` + jsonStr + `}}`

	// Always create a new
	err = srv.persistenceClient.UpdateOne(connection["Id"].(string), connection["Name"].(string), rqst.ItemDefinition.RefColId, `{ "_id" : "`+rqst.ItemDefinition.RefObjId+`"}`, jsonStr, `[]`)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &catalogpb.AppendItemDefinitionCategoryResponse{
		Result: true,
	}, nil
}

// Remove Item from manufacturer
func (srv *server) RemoveItemDefinitionCategory(ctx context.Context, rqst *catalogpb.RemoveItemDefinitionCategoryRequest) (*catalogpb.RemoveItemDefinitionCategoryResponse, error) {

	persistence := srv.Services["Persistence"].(map[string]interface{})

	if persistence["Connections"].(map[string]interface{})[rqst.ConnectionId] == nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("no connection found with id "+rqst.ConnectionId)))
	}

	connection := persistence["Connections"].(map[string]interface{})[rqst.ConnectionId].(map[string]interface{})

	data, err := protojson.Marshal(rqst.Category)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))

	}

	// Here I will generate the _id key
	jsonStr := string(data)

	// Set the db reference.
	jsonStr = strings.Replace(jsonStr, "refObjId", "$id", -1)
	jsonStr = strings.Replace(jsonStr, "refColId", "$ref", -1)
	jsonStr = strings.Replace(jsonStr, "refDbName", "$db", -1)

	// Now I will modify the jsonStr to insert the value in the array.
	jsonStr = `{ "$pull": { "categories":` + jsonStr + `}}` // remove a particular item.

	// Always create a new
	err = srv.persistenceClient.UpdateOne(connection["Id"].(string), connection["Name"].(string), rqst.ItemDefinition.RefColId, `{ "_id" : "`+rqst.ItemDefinition.RefObjId+`"}`, jsonStr, `[]`)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &catalogpb.RemoveItemDefinitionCategoryResponse{
		Result: true,
	}, nil
}

//////////////////////////////// Getter function ///////////////////////////////

// Getter function.

// Getter Item instance.
func (srv *server) GetItemInstance(ctx context.Context, rqst *catalogpb.GetItemInstanceRequest) (*catalogpb.GetItemInstanceResponse, error) {
	persistence := srv.Services["Persistence"].(map[string]interface{})

	if persistence["Connections"].(map[string]interface{})[rqst.ConnectionId] == nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("no connection found with id "+rqst.ConnectionId)))
	}

	connection := persistence["Connections"].(map[string]interface{})[rqst.ConnectionId].(map[string]interface{})
	var query string
	if Utility.IsUuid(rqst.ItemInstanceId) {
		query = `{"_id":"` + rqst.ItemInstanceId + `"}`
	} else {
		query = `{"_id":"` + Utility.GenerateUUID(rqst.ItemInstanceId) + `"}`
	}

	obj, err := srv.persistenceClient.FindOne(connection["Id"].(string), connection["Name"].(string), "ItemInstance", query, `[{"Projection":{"_id":0}}]`)
	if err != nil {
		return nil, err
	}
	jsonStr, _ := Utility.ToJson(obj)

	// replace the reference.
	jsonStr = strings.Replace(jsonStr, "$id", "refObjId", -1)
	jsonStr = strings.Replace(jsonStr, "$ref", "refColId", -1)
	jsonStr = strings.Replace(jsonStr, "$db", "refDbName", -1)

	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	instance := new(catalogpb.ItemInstance)
	err = protojson.Unmarshal([]byte(jsonStr), instance)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &catalogpb.GetItemInstanceResponse{
		ItemInstance: instance,
	}, nil

}

// Get Item Instances.
func (srv *server) GetItemInstances(ctx context.Context, rqst *catalogpb.GetItemInstancesRequest) (*catalogpb.GetItemInstancesResponse, error) {
	persistence := srv.Services["Persistence"].(map[string]interface{})

	if persistence["Connections"].(map[string]interface{})[rqst.ConnectionId] == nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("no connection found with id "+rqst.ConnectionId)))
	}

	connection := persistence["Connections"].(map[string]interface{})[rqst.ConnectionId].(map[string]interface{})

	options, err := srv.getOptionsString(rqst.Options)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	if len(rqst.Query) == 0 {
		rqst.Query = ``
	}

	values, err := srv.persistenceClient.Find(connection["Id"].(string), connection["Name"].(string), "ItemInstance", rqst.Query, options)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	jsonStr, err := Utility.ToJson(&values)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// replace the reference.
	jsonStr = strings.Replace(jsonStr, "$id", "refObjId", -1)
	jsonStr = strings.Replace(jsonStr, "$ref", "refColId", -1)
	jsonStr = strings.Replace(jsonStr, "$db", "refDbName", -1)

	// set the Packages properties...
	jsonStr = `{ "itemInstances":` + jsonStr + `}`

	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// unmarshall object
	instances := new(catalogpb.ItemInstances)
	err = protojson.Unmarshal([]byte(jsonStr), instances)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &catalogpb.GetItemInstancesResponse{
		ItemInstances: instances.ItemInstances,
	}, nil
}

// Getter Item defintion.
func (srv *server) GetItemDefinition(ctx context.Context, rqst *catalogpb.GetItemDefinitionRequest) (*catalogpb.GetItemDefinitionResponse, error) {
	persistence := srv.Services["Persistence"].(map[string]interface{})

	if persistence["Connections"].(map[string]interface{})[rqst.ConnectionId] == nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("no connection found with id "+rqst.ConnectionId)))
	}

	connection := persistence["Connections"].(map[string]interface{})[rqst.ConnectionId].(map[string]interface{})
	var query string
	if Utility.IsUuid(rqst.ItemDefinitionId) {
		query = `{"_id":"` + rqst.ItemDefinitionId + `"}`
	} else {
		query = `{"_id":"` + Utility.GenerateUUID(rqst.ItemDefinitionId) + `"}`
	}

	obj, err := srv.persistenceClient.FindOne(connection["Id"].(string), connection["Name"].(string), "ItemDefinition", query, `[{"Projection":{"_id":0}}]`)
	if err != nil {
		return nil, err
	}

	jsonStr, _ := Utility.ToJson(obj)

	// replace the reference.
	jsonStr = strings.Replace(jsonStr, "$id", "refObjId", -1)
	jsonStr = strings.Replace(jsonStr, "$ref", "refColId", -1)
	jsonStr = strings.Replace(jsonStr, "$db", "refDbName", -1)

	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	definition := new(catalogpb.ItemDefinition)
	err = protojson.Unmarshal([]byte(jsonStr), definition)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &catalogpb.GetItemDefinitionResponse{
		ItemDefinition: definition,
	}, nil

}

// Get Inventories.
func (srv *server) GetInventories(ctx context.Context, rqst *catalogpb.GetInventoriesRequest) (*catalogpb.GetInventoriesResponse, error) {
	persistence := srv.Services["Persistence"].(map[string]interface{})

	if persistence["Connections"].(map[string]interface{})[rqst.ConnectionId] == nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("no connection found with id "+rqst.ConnectionId)))
	}

	connection := persistence["Connections"].(map[string]interface{})[rqst.ConnectionId].(map[string]interface{})

	options, err := srv.getOptionsString(rqst.Options)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	if len(rqst.Query) == 0 {
		rqst.Query = ``
	}
	values, err := srv.persistenceClient.Find(connection["Id"].(string), connection["Name"].(string), "Inventory", rqst.Query, options)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	jsonStr, err := Utility.ToJson(&values)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// replace the reference.
	jsonStr = strings.Replace(jsonStr, "$id", "refObjId", -1)
	jsonStr = strings.Replace(jsonStr, "$ref", "refColId", -1)
	jsonStr = strings.Replace(jsonStr, "$db", "refDbName", -1)

	// set the Packages properties...
	jsonStr = `{ "inventories":` + jsonStr + `}`

	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// unmarshall object
	inventories := new(catalogpb.Inventories)
	err = protojson.Unmarshal([]byte(jsonStr), inventories)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &catalogpb.GetInventoriesResponse{
		Inventories: inventories.Inventories,
	}, nil
}

// Get Item Definitions.
func (srv *server) GetItemDefinitions(ctx context.Context, rqst *catalogpb.GetItemDefinitionsRequest) (*catalogpb.GetItemDefinitionsResponse, error) {
	persistence := srv.Services["Persistence"].(map[string]interface{})

	if persistence["Connections"].(map[string]interface{})[rqst.ConnectionId] == nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("no connection found with id "+rqst.ConnectionId)))
	}

	connection := persistence["Connections"].(map[string]interface{})[rqst.ConnectionId].(map[string]interface{})

	options, err := srv.getOptionsString(rqst.Options)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	if len(rqst.Query) == 0 {
		rqst.Query = ``
	}

	values, err := srv.persistenceClient.Find(connection["Id"].(string), connection["Name"].(string), "ItemDefinition", rqst.Query, options)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	jsonStr, err := Utility.ToJson(&values)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// replace the reference.
	jsonStr = strings.Replace(jsonStr, "$id", "refObjId", -1)
	jsonStr = strings.Replace(jsonStr, "$ref", "refColId", -1)
	jsonStr = strings.Replace(jsonStr, "$db", "refDbName", -1)

	// set the Packages properties...
	jsonStr = `{ "itemDefinitions":` + jsonStr + `}`

	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// unmarshall object
	definitions := new(catalogpb.ItemDefinitions)
	err = protojson.Unmarshal([]byte(jsonStr), definitions)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &catalogpb.GetItemDefinitionsResponse{
		ItemDefinitions: definitions.ItemDefinitions,
	}, nil
}

// Getter Supplier.
func (srv *server) GetSupplier(ctx context.Context, rqst *catalogpb.GetSupplierRequest) (*catalogpb.GetSupplierResponse, error) {
	persistence := srv.Services["Persistence"].(map[string]interface{})

	if persistence["Connections"].(map[string]interface{})[rqst.ConnectionId] == nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("no connection found with id "+rqst.ConnectionId)))
	}

	connection := persistence["Connections"].(map[string]interface{})[rqst.ConnectionId].(map[string]interface{})
	var query string
	if Utility.IsUuid(rqst.SupplierId) {
		query = `{"_id":"` + rqst.SupplierId + `"}`
	} else {
		query = `{"_id":"` + Utility.GenerateUUID(rqst.SupplierId) + `"}`
	}

	obj, err := srv.persistenceClient.FindOne(connection["Id"].(string), connection["Name"].(string), "Supplier", query, `[{"Projection":{"_id":0}}]`)
	if err != nil {
		return nil, err
	}

	jsonStr, _ := Utility.ToJson(obj)
	// replace the reference.
	jsonStr = strings.Replace(jsonStr, "$id", "refObjId", -1)
	jsonStr = strings.Replace(jsonStr, "$ref", "refColId", -1)
	jsonStr = strings.Replace(jsonStr, "$db", "refDbName", -1)

	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	supplier := new(catalogpb.Supplier)
	err = protojson.Unmarshal([]byte(jsonStr), supplier)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &catalogpb.GetSupplierResponse{
		Supplier: supplier,
	}, nil

}

// Get Suppliers
func (srv *server) GetSuppliers(ctx context.Context, rqst *catalogpb.GetSuppliersRequest) (*catalogpb.GetSuppliersResponse, error) {
	persistence := srv.Services["Persistence"].(map[string]interface{})

	if persistence["Connections"].(map[string]interface{})[rqst.ConnectionId] == nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("no connection found with id "+rqst.ConnectionId)))
	}

	connection := persistence["Connections"].(map[string]interface{})[rqst.ConnectionId].(map[string]interface{})

	options, err := srv.getOptionsString(rqst.Options)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	if len(rqst.Query) == 0 {
		rqst.Query = ``
	}
	values, err := srv.persistenceClient.Find(connection["Id"].(string), connection["Name"].(string), "Supplier", rqst.Query, options)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	jsonStr, err := Utility.ToJson(&values)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// replace the reference.
	jsonStr = strings.Replace(jsonStr, "$id", "refObjId", -1)
	jsonStr = strings.Replace(jsonStr, "$ref", "refColId", -1)
	jsonStr = strings.Replace(jsonStr, "$db", "refDbName", -1)

	// set the Packages properties...
	jsonStr = `{ "suppliers":` + jsonStr + `}`

	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// unmarshall object
	suppliers := new(catalogpb.Suppliers)
	err = protojson.Unmarshal([]byte(jsonStr), suppliers)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &catalogpb.GetSuppliersResponse{
		Suppliers: suppliers.Suppliers,
	}, nil
}

// Get Package supplier.
func (srv *server) GetSupplierPackages(ctx context.Context, rqst *catalogpb.GetSupplierPackagesRequest) (*catalogpb.GetSupplierPackagesResponse, error) {
	persistence := srv.Services["Persistence"].(map[string]interface{})

	if persistence["Connections"].(map[string]interface{})[rqst.ConnectionId] == nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("no connection found with id "+rqst.ConnectionId)))
	}

	connection := persistence["Connections"].(map[string]interface{})[rqst.ConnectionId].(map[string]interface{})
	var query string
	if Utility.IsUuid(rqst.SupplierId) {
		query = `{"supplier.$id":"` + rqst.SupplierId + `"}`
	} else {
		query = `{"supplier.$id":"` + Utility.GenerateUUID(rqst.SupplierId) + `"}`
	}

	values, err := srv.persistenceClient.Find(connection["Id"].(string), connection["Name"].(string), "PackageSupplier", query, `[{"Projection":{"_id":1}}]`)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	jsonStr, err := Utility.ToJson(&values)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	results := make([]map[string]interface{}, 0)
	err = json.Unmarshal([]byte(jsonStr), &results)

	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	packagesSupplier := make([]*catalogpb.PackageSupplier, 0)

	for i := 0; i < len(results); i++ {
		obj, err := srv.persistenceClient.FindOne(connection["Id"].(string), connection["Name"].(string), "PackageSupplier", `{"_id":"`+results[i]["_id"].(string)+`"}`, `[{"Projection":{"_id":0}}]`)
		if err != nil {
			return nil, status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}

		jsonStr, _ := Utility.ToJson(obj)

		// replace the reference.
		jsonStr = strings.Replace(jsonStr, "$id", "refObjId", -1)
		jsonStr = strings.Replace(jsonStr, "$ref", "refColId", -1)
		jsonStr = strings.Replace(jsonStr, "$db", "refDbName", -1)

		if err != nil {
			return nil, status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}

		packageSupplier := new(catalogpb.PackageSupplier)
		err = protojson.Unmarshal([]byte(jsonStr), packageSupplier)
		if err != nil {
			return nil, status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}

		packagesSupplier = append(packagesSupplier, packageSupplier)
	}

	return &catalogpb.GetSupplierPackagesResponse{
		PackagesSupplier: packagesSupplier,
	}, nil

}

// Get Package
func (srv *server) GetPackage(ctx context.Context, rqst *catalogpb.GetPackageRequest) (*catalogpb.GetPackageResponse, error) {
	persistence := srv.Services["Persistence"].(map[string]interface{})

	if persistence["Connections"].(map[string]interface{})[rqst.ConnectionId] == nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("no connection found with id "+rqst.ConnectionId)))
	}

	connection := persistence["Connections"].(map[string]interface{})[rqst.ConnectionId].(map[string]interface{})
	var query string
	if Utility.IsUuid(rqst.PackageId) {
		query = `{"_id":"` + rqst.PackageId + `"}`
	} else {
		query = `{"_id":"` + Utility.GenerateUUID(rqst.PackageId) + `"}`
	}

	obj, err := srv.persistenceClient.FindOne(connection["Id"].(string), connection["Name"].(string), "Package", query, `[{"Projection":{"_id":0}}]`)
	jsonStr, err := Utility.ToJson(obj)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// replace the reference.
	jsonStr = strings.Replace(jsonStr, "$id", "refObjId", -1)
	jsonStr = strings.Replace(jsonStr, "$ref", "refColId", -1)
	jsonStr = strings.Replace(jsonStr, "$db", "refDbName", -1)

	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	package_ := new(catalogpb.Package)
	err = protojson.Unmarshal([]byte(jsonStr), package_)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &catalogpb.GetPackageResponse{
		Pacakge: package_,
	}, nil

}

// Get Packages
func (srv *server) GetPackages(ctx context.Context, rqst *catalogpb.GetPackagesRequest) (*catalogpb.GetPackagesResponse, error) {
	persistence := srv.Services["Persistence"].(map[string]interface{})

	if persistence["Connections"].(map[string]interface{})[rqst.ConnectionId] == nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("no connection found with id "+rqst.ConnectionId)))
	}

	connection := persistence["Connections"].(map[string]interface{})[rqst.ConnectionId].(map[string]interface{})
	options, err := srv.getOptionsString(rqst.Options)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	values, err := srv.persistenceClient.Find(connection["Id"].(string), connection["Name"].(string), "Package", rqst.Query, options)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	jsonStr, err := Utility.ToJson(&values)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// replace the reference.
	jsonStr = strings.Replace(jsonStr, "$id", "refObjId", -1)
	jsonStr = strings.Replace(jsonStr, "$ref", "refColId", -1)
	jsonStr = strings.Replace(jsonStr, "$db", "refDbName", -1)

	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// set the Packages properties...
	jsonStr = `{ "packages":` + jsonStr + `}`

	// unmarshall object
	packages := new(catalogpb.Packages)
	err = protojson.Unmarshal([]byte(jsonStr), packages)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &catalogpb.GetPackagesResponse{
		Packages: packages.Packages,
	}, nil

}

func (srv *server) getLocalisation(localisationId string, connectionId string) (*catalogpb.Localisation, error) {
	persistence := srv.Services["Persistence"].(map[string]interface{})

	if persistence["Connections"].(map[string]interface{})[connectionId] == nil {
		return nil, errors.New("no connection found with id " + connectionId)
	}

	connection := persistence["Connections"].(map[string]interface{})[connectionId].(map[string]interface{})

	var query string
	if Utility.IsUuid(localisationId) {
		query = `{"_id":"` + localisationId + `"}`
	} else {
		query = `{"_id":"` + Utility.GenerateUUID(localisationId) + `"}`
	}

	obj, err := srv.persistenceClient.FindOne(connection["Id"].(string), connection["Name"].(string), "Localisation", query, `[{"Projection":{"_id":0}}]`)
	jsonStr, err := Utility.ToJson(obj)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	// replace the reference.
	jsonStr = strings.Replace(jsonStr, "$id", "refObjId", -1)
	jsonStr = strings.Replace(jsonStr, "$ref", "refColId", -1)
	jsonStr = strings.Replace(jsonStr, "$db", "refDbName", -1)

	if err != nil {
		return nil, err
	}

	localisation := new(catalogpb.Localisation)
	err = protojson.Unmarshal([]byte(jsonStr), localisation)
	if err != nil {
		return nil, err
	}

	return localisation, nil
}

// Get Localisation
func (srv *server) GetLocalisation(ctx context.Context, rqst *catalogpb.GetLocalisationRequest) (*catalogpb.GetLocalisationResponse, error) {

	localisation, err := srv.getLocalisation(rqst.LocalisationId, rqst.ConnectionId)

	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &catalogpb.GetLocalisationResponse{
		Localisation: localisation,
	}, nil

}

func (srv *server) getLocalisations(query string, options string, connectionId string) ([]*catalogpb.Localisation, error) {
	persistence := srv.Services["Persistence"].(map[string]interface{})

	if persistence["Connections"].(map[string]interface{})[connectionId] == nil {
		return nil, errors.New("no connection found with id " + connectionId)
	}

	connection := persistence["Connections"].(map[string]interface{})[connectionId].(map[string]interface{})
	options, err := srv.getOptionsString(options)
	if err != nil {
		return nil, err
	}

	if len(query) == 0 {
		query = ``
	}

	values, err := srv.persistenceClient.Find(connection["Id"].(string), connection["Name"].(string), "Localisation", query, options)
	if err != nil {
		return nil, err
	}

	jsonStr, err := Utility.ToJson(&values)
	if err != nil {
		return nil, err
	}

	// replace the reference.
	jsonStr = strings.Replace(jsonStr, "$id", "refObjId", -1)
	jsonStr = strings.Replace(jsonStr, "$ref", "refColId", -1)
	jsonStr = strings.Replace(jsonStr, "$db", "refDbName", -1)

	if err != nil {
		return nil, err
	}

	// set the Packages properties...
	jsonStr = `{ "localisations":` + jsonStr + `}`

	// unmarshall object
	localisations := new(catalogpb.Localisations)
	err = protojson.Unmarshal([]byte(jsonStr), localisations)

	if err != nil {
		return nil, err
	}

	return localisations.Localisations, nil

}

// Get Packages
func (srv *server) GetLocalisations(ctx context.Context, rqst *catalogpb.GetLocalisationsRequest) (*catalogpb.GetLocalisationsResponse, error) {

	// unmarshall object
	localisations, err := srv.getLocalisations(rqst.Query, rqst.Options, rqst.ConnectionId)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &catalogpb.GetLocalisationsResponse{
		Localisations: localisations,
	}, nil

}

/**
 * Get the category.
 */
func (srv *server) getCategory(categoryId string, connectionId string) (*catalogpb.Category, error) {
	persistence := srv.Services["Persistence"].(map[string]interface{})

	if persistence["Connections"].(map[string]interface{})[connectionId] == nil {
		return nil, errors.New("no connection found with id " + connectionId)
	}

	connection := persistence["Connections"].(map[string]interface{})[connectionId].(map[string]interface{})

	var query string
	if Utility.IsUuid(categoryId) {
		query = `{"_id":"` + categoryId + `"}`
	} else {
		query = `{"_id":"` + Utility.GenerateUUID(categoryId) + `"}`
	}

	obj, err := srv.persistenceClient.FindOne(connection["Id"].(string), connection["Name"].(string), "Category", query, `[{"Projection":{"_id":0}}]`)
	jsonStr, err := Utility.ToJson(obj)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	// replace the reference.
	jsonStr = strings.Replace(jsonStr, "$id", "refObjId", -1)
	jsonStr = strings.Replace(jsonStr, "$ref", "refColId", -1)
	jsonStr = strings.Replace(jsonStr, "$db", "refDbName", -1)

	if err != nil {
		return nil, err
	}

	category := new(catalogpb.Category)
	err = protojson.Unmarshal([]byte(jsonStr), category)
	if err != nil {
		return nil, err
	}

	return category, nil
}

func (srv *server) GetCategory(ctx context.Context, rqst *catalogpb.GetCategoryRequest) (*catalogpb.GetCategoryResponse, error) {

	category, err := srv.getCategory(rqst.CategoryId, rqst.ConnectionId)

	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &catalogpb.GetCategoryResponse{
		Category: category,
	}, nil

}

func (srv *server) getCategories(query string, options string, connectionId string) ([]*catalogpb.Category, error) {
	persistence := srv.Services["Persistence"].(map[string]interface{})

	if persistence["Connections"].(map[string]interface{})[connectionId] == nil {
		return nil, errors.New("no connection found with id " + connectionId)
	}

	connection := persistence["Connections"].(map[string]interface{})[connectionId].(map[string]interface{})
	options, err := srv.getOptionsString(options)
	if err != nil {
		return nil, err
	}

	if len(query) == 0 {
		query = ``
	}

	values, err := srv.persistenceClient.Find(connection["Id"].(string), connection["Name"].(string), "Category", query, options)
	if err != nil {
		return nil, err
	}

	jsonStr, err := Utility.ToJson(&values)
	if err != nil {
		return nil, err
	}

	// replace the reference.
	jsonStr = strings.Replace(jsonStr, "$id", "refObjId", -1)
	jsonStr = strings.Replace(jsonStr, "$ref", "refColId", -1)
	jsonStr = strings.Replace(jsonStr, "$db", "refDbName", -1)

	if err != nil {
		return nil, err
	}

	// set the Packages properties...
	jsonStr = `{ "categories":` + jsonStr + `}`

	// unmarshall object
	categories := new(catalogpb.Categories)
	err = protojson.Unmarshal([]byte(jsonStr), categories)

	if err != nil {
		return nil, err
	}

	return categories.Categories, nil

}

// Get Packages
func (srv *server) GetCategories(ctx context.Context, rqst *catalogpb.GetCategoriesRequest) (*catalogpb.GetCategoriesResponse, error) {

	// unmarshall object
	categories, err := srv.getCategories(rqst.Query, rqst.Options, rqst.ConnectionId)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &catalogpb.GetCategoriesResponse{
		Categories: categories,
	}, nil

}

// Get Manufacturer
func (srv *server) GetManufacturer(ctx context.Context, rqst *catalogpb.GetManufacturerRequest) (*catalogpb.GetManufacturerResponse, error) {
	persistence := srv.Services["Persistence"].(map[string]interface{})

	if persistence["Connections"].(map[string]interface{})[rqst.ConnectionId] == nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("no connection found with id "+rqst.ConnectionId)))
	}

	connection := persistence["Connections"].(map[string]interface{})[rqst.ConnectionId].(map[string]interface{})
	var query string
	if Utility.IsUuid(rqst.ManufacturerId) {
		query = `{"_id":"` + rqst.ManufacturerId + `"}`
	} else {
		query = `{"_id":"` + Utility.GenerateUUID(rqst.ManufacturerId) + `"}`
	}

	obj, err := srv.persistenceClient.FindOne(connection["Id"].(string), connection["Name"].(string), "Package", query, `[{"Projection":{"_id":0}}]`)
	jsonStr, err := Utility.ToJson(obj)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	// replace the reference.
	jsonStr = strings.Replace(jsonStr, "$id", "refObjId", -1)
	jsonStr = strings.Replace(jsonStr, "$ref", "refColId", -1)
	jsonStr = strings.Replace(jsonStr, "$db", "refDbName", -1)

	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	manufacturer := new(catalogpb.Manufacturer)
	err = protojson.Unmarshal([]byte(jsonStr), manufacturer)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &catalogpb.GetManufacturerResponse{
		Manufacturer: manufacturer,
	}, nil

}

func (srv *server) getOptionsString(options string) (string, error) {
	options_ := make([]map[string]interface{}, 0)
	if len(options) > 0 {
		err := json.Unmarshal([]byte(options), &options_)
		if err != nil {
			return "", err
		}

		var projections map[string]interface{}

		for i := 0; i < len(options_); i++ {
			if options_[i]["Projection"] != nil {
				projections = options_[i]["Projection"].(map[string]interface{})
				break
			}
		}

		if projections != nil {
			projections["_id"] = 0
		} else {
			options_ = append(options_, map[string]interface{}{"Projection": map[string]interface{}{"_id": 0}})
		}

	} else {
		options_ = append(options_, map[string]interface{}{"Projection": map[string]interface{}{"_id": 0}})
	}

	optionsStr, err := Utility.ToJson(options_)
	return string(optionsStr), err
}

// Get Manufacturers
func (srv *server) GetManufacturers(ctx context.Context, rqst *catalogpb.GetManufacturersRequest) (*catalogpb.GetManufacturersResponse, error) {
	persistence := srv.Services["Persistence"].(map[string]interface{})

	if persistence["Connections"].(map[string]interface{})[rqst.ConnectionId] == nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("no connection found with id "+rqst.ConnectionId)))
	}

	connection := persistence["Connections"].(map[string]interface{})[rqst.ConnectionId].(map[string]interface{})

	options, err := srv.getOptionsString(rqst.Options)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	if len(rqst.Query) == 0 {
		rqst.Query = ``
	}

	values, err := srv.persistenceClient.Find(connection["Id"].(string), connection["Name"].(string), "Manufacturer", rqst.Query, options)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	jsonStr, err := Utility.ToJson(&values)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// replace the reference.
	jsonStr = strings.Replace(jsonStr, "$id", "refObjId", -1)
	jsonStr = strings.Replace(jsonStr, "$ref", "refColId", -1)
	jsonStr = strings.Replace(jsonStr, "$db", "refDbName", -1)

	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	// set the Packages properties...
	jsonStr = `{ "manufacturers":` + jsonStr + `}`

	// unmarshall object
	manufacturers := new(catalogpb.Manufacturers)
	err = protojson.Unmarshal([]byte(jsonStr), manufacturers)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &catalogpb.GetManufacturersResponse{
		Manufacturers: manufacturers.Manufacturers,
	}, nil

}

// Get Package
func (srv *server) GetUnitOfMeasures(ctx context.Context, rqst *catalogpb.GetUnitOfMeasuresRequest) (*catalogpb.GetUnitOfMeasuresResponse, error) {
	persistence := srv.Services["Persistence"].(map[string]interface{})

	if persistence["Connections"].(map[string]interface{})[rqst.ConnectionId] == nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("no connection found with id "+rqst.ConnectionId)))
	}

	connection := persistence["Connections"].(map[string]interface{})[rqst.ConnectionId].(map[string]interface{})
	options, err := srv.getOptionsString(rqst.Options)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	if len(rqst.Query) == 0 {
		rqst.Query = ``
	}

	values, err := srv.persistenceClient.Find(connection["Id"].(string), connection["Name"].(string), "UnitOfMeasure", rqst.Query, options)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	jsonStr, err := Utility.ToJson(&values)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// replace the reference.
	jsonStr = strings.Replace(jsonStr, "$id", "refObjId", -1)
	jsonStr = strings.Replace(jsonStr, "$ref", "refColId", -1)
	jsonStr = strings.Replace(jsonStr, "$db", "refDbName", -1)

	// set the Packages properties...
	jsonStr = `{ "unitOfMeasures":` + jsonStr + `}`

	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// unmarshall object
	unitOfMeasures := new(catalogpb.UnitOfMeasures)
	err = protojson.Unmarshal([]byte(jsonStr), unitOfMeasures)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &catalogpb.GetUnitOfMeasuresResponse{
		UnitOfMeasures: unitOfMeasures.UnitOfMeasures,
	}, nil

}

// Get Unit of measure.
func (srv *server) GetUnitOfMeasure(ctx context.Context, rqst *catalogpb.GetUnitOfMeasureRequest) (*catalogpb.GetUnitOfMeasureResponse, error) {
	persistence := srv.Services["Persistence"].(map[string]interface{})

	if persistence["Connections"].(map[string]interface{})[rqst.ConnectionId] == nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("no connection found with id "+rqst.ConnectionId)))
	}

	connection := persistence["Connections"].(map[string]interface{})[rqst.ConnectionId].(map[string]interface{})
	var query string
	if Utility.IsUuid(rqst.UnitOfMeasureId) {
		query = `{"_id":"` + rqst.UnitOfMeasureId + `"}`
	} else {
		query = `{"_id":"` + Utility.GenerateUUID(rqst.UnitOfMeasureId) + `"}`
	}

	obj, err := srv.persistenceClient.FindOne(connection["Id"].(string), connection["Name"].(string), "UnitOfMeasure", query, `[{"Projection":{"_id":0}}]`)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	jsonStr, err := Utility.ToJson(obj)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// replace the reference.
	jsonStr = strings.Replace(jsonStr, "$id", "refObjId", -1)
	jsonStr = strings.Replace(jsonStr, "$ref", "refColId", -1)
	jsonStr = strings.Replace(jsonStr, "$db", "refDbName", -1)

	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	unitOfMeasure := new(catalogpb.UnitOfMeasure)
	err = protojson.Unmarshal([]byte(jsonStr), unitOfMeasure)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &catalogpb.GetUnitOfMeasureResponse{
		UnitOfMeasure: unitOfMeasure,
	}, nil

}

////// Delete function //////

// Delete a package.
func (srv *server) DeletePackage(ctx context.Context, rqst *catalogpb.DeletePackageRequest) (*catalogpb.DeletePackageResponse, error) {
	persistence := srv.Services["Persistence"].(map[string]interface{})
	if persistence["Connections"].(map[string]interface{})[rqst.ConnectionId] == nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("no connection found with id "+rqst.ConnectionId)))
	}

	connection := persistence["Connections"].(map[string]interface{})[rqst.ConnectionId].(map[string]interface{})

	// save the supplier id
	package_ := rqst.Package

	// Here I will generate the _id key
	_id := Utility.GenerateUUID(package_.Id + package_.LanguageCode)

	err := srv.persistenceClient.DeleteOne(connection["Id"].(string), connection["Name"].(string), "Package", `{"_id":"`+_id+`"}`, "")

	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &catalogpb.DeletePackageResponse{
		Result: true,
	}, nil
}

// Delete a package supplier
func (srv *server) DeletePackageSupplier(ctx context.Context, rqst *catalogpb.DeletePackageSupplierRequest) (*catalogpb.DeletePackageSupplierResponse, error) {
	persistence := srv.Services["Persistence"].(map[string]interface{})
	if persistence["Connections"].(map[string]interface{})[rqst.ConnectionId] == nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("no connection found with id "+rqst.ConnectionId)))
	}

	connection := persistence["Connections"].(map[string]interface{})[rqst.ConnectionId].(map[string]interface{})

	// save the supplier id
	packageSupplier := rqst.PackageSupplier

	// Here I will generate the _id key
	_id := Utility.GenerateUUID(packageSupplier.Id)

	err := srv.persistenceClient.DeleteOne(connection["Id"].(string), connection["Name"].(string), "PackageSupplier", `{"_id":"`+_id+`"}`, "")

	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &catalogpb.DeletePackageSupplierResponse{
		Result: true,
	}, nil
}

// Delete a supplier
func (srv *server) DeleteSupplier(ctx context.Context, rqst *catalogpb.DeleteSupplierRequest) (*catalogpb.DeleteSupplierResponse, error) {
	persistence := srv.Services["Persistence"].(map[string]interface{})
	if persistence["Connections"].(map[string]interface{})[rqst.ConnectionId] == nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("no connection found with id "+rqst.ConnectionId)))
	}

	connection := persistence["Connections"].(map[string]interface{})[rqst.ConnectionId].(map[string]interface{})

	// save the supplier id
	supplier := rqst.Supplier

	// Here I will generate the _id key
	_id := Utility.GenerateUUID(supplier.Id)
	err := srv.persistenceClient.DeleteOne(connection["Id"].(string), connection["Name"].(string), "Supplier", `{"_id":"`+_id+`"}`, "")

	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &catalogpb.DeleteSupplierResponse{
		Result: true,
	}, nil
}

// Delete propertie definition
func (srv *server) DeletePropertyDefinition(ctx context.Context, rqst *catalogpb.DeletePropertyDefinitionRequest) (*catalogpb.DeletePropertyDefinitionResponse, error) {
	return nil, nil
}

// Delete unit of measure
func (srv *server) DeleteUnitOfMeasure(ctx context.Context, rqst *catalogpb.DeleteUnitOfMeasureRequest) (*catalogpb.DeleteUnitOfMeasureResponse, error) {
	return nil, nil
}

// Delete Item Instance
func (srv *server) DeleteItemInstance(ctx context.Context, rqst *catalogpb.DeleteItemInstanceRequest) (*catalogpb.DeleteItemInstanceResponse, error) {
	return nil, nil
}

// Delete Manufacturer
func (srv *server) DeleteManufacturer(ctx context.Context, rqst *catalogpb.DeleteManufacturerRequest) (*catalogpb.DeleteManufacturerResponse, error) {
	return nil, nil
}

// Delete Item Manufacturer
func (srv *server) DeleteItemManufacturer(ctx context.Context, rqst *catalogpb.DeleteItemManufacturerRequest) (*catalogpb.DeleteItemManufacturerResponse, error) {
	return nil, nil
}

// Delete Category
func (srv *server) DeleteCategory(ctx context.Context, rqst *catalogpb.DeleteCategoryRequest) (*catalogpb.DeleteCategoryResponse, error) {
	return nil, nil
}

func (srv *server) deleteLocalisation(localisation *catalogpb.Localisation, connectionId string) error {

	persistence := srv.Services["Persistence"].(map[string]interface{})
	if persistence["Connections"].(map[string]interface{})[connectionId] == nil {
		return errors.New("no connection found with id " + connectionId)
	}

	connection := persistence["Connections"].(map[string]interface{})[connectionId].(map[string]interface{})

	// I will remove referencing object...
	referenced, err := srv.getLocalisations(`{"subLocalisations.values.$id":"`+Utility.GenerateUUID(localisation.GetId()+localisation.GetLanguageCode())+`"}`, "", connectionId)
	if err == nil {
		refStr := `{"$id":"` + Utility.GenerateUUID(localisation.GetId()+localisation.GetLanguageCode()) + `","$ref":"Localisation","$db":"` + connection["Name"].(string) + `"}`
		for i := 0; i < len(referenced); i++ {
			// Now I will modify the jsonStr to insert the value in the array.
			query := `{"$pull":{"subLocalisations.values":` + refStr + `}}` // remove a particular item.
			_id := Utility.GenerateUUID(referenced[i].Id + referenced[i].LanguageCode)
			// Always create a new
			err = srv.persistenceClient.UpdateOne(connection["Id"].(string), connection["Name"].(string), "Localisation", `{"_id" : "`+_id+`"}`, query, `[]`)
			if err != nil {
				return err
			}
		}
	}

	// So here I will delete all sub-localisation to...
	if localisation.GetSubLocalisations() != nil {
		for i := 0; i < len(localisation.GetSubLocalisations().GetValues()); i++ {
			subLocalisation, err := srv.getLocalisation(localisation.GetSubLocalisations().GetValues()[i].GetRefObjId(), connectionId)
			if err == nil {
				err := srv.deleteLocalisation(subLocalisation, connectionId)
				if err != nil {
					return err
				}
			}
		}
	}

	// Here I will generate the _id key
	_id := Utility.GenerateUUID(localisation.Id + localisation.LanguageCode)

	return srv.persistenceClient.DeleteOne(connection["Id"].(string), connection["Name"].(string), "Localisation", `{"_id":"`+_id+`"}`, "")

}

// Delete Localisation
func (srv *server) DeleteLocalisation(ctx context.Context, rqst *catalogpb.DeleteLocalisationRequest) (*catalogpb.DeleteLocalisationResponse, error) {

	err := srv.deleteLocalisation(rqst.Localisation, rqst.ConnectionId)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &catalogpb.DeleteLocalisationResponse{
		Result: true,
	}, nil
}

// Delete Localisation
func (srv *server) DeleteInventory(ctx context.Context, rqst *catalogpb.DeleteInventoryRequest) (*catalogpb.DeleteInventoryResponse, error) {

	persistence := srv.Services["Persistence"].(map[string]interface{})
	if persistence["Connections"].(map[string]interface{})[rqst.ConnectionId] == nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("no connection found with id "+rqst.ConnectionId)))
	}

	connection := persistence["Connections"].(map[string]interface{})[rqst.ConnectionId].(map[string]interface{})

	// save the supplier id
	inventory := rqst.Inventory

	// Here I will generate the _id key
	_id := Utility.GenerateUUID(inventory.LocalisationId + inventory.PacakgeId)
	err := srv.persistenceClient.DeleteOne(connection["Id"].(string), connection["Name"].(string), "Inventory", `{"_id":"`+_id+`"}`, "")

	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &catalogpb.DeleteInventoryResponse{
		Result: true,
	}, nil

}

// That service is use to give access to SQL.
// port number must be pass as argument.
func main() {

	// set the logger.
	//grpclog.SetLogger(log.New(os.Stdout, "catalog_service: ", log.LstdFlags))

	// Set the log information in case of crash...
	//log.SetFlags(log.LstdFlags | log.Lshortfile)

	// The actual server implementation.
	s_impl := new(server)
	s_impl.Name = string(catalogpb.File_catalog_proto.Services().Get(0).FullName())
	s_impl.Proto = catalogpb.File_catalog_proto.Path()
	s_impl.Path, _ = filepath.Abs(filepath.Dir(os.Args[0]))
	s_impl.Port = defaultPort
	s_impl.Proxy = defaultProxy
	s_impl.Path, _ = filepath.Abs(filepath.Dir(os.Args[0]))
	s_impl.Protocol = "grpc"
	s_impl.Domain, _ = config.GetDomain()
	s_impl.Address, _ = config.GetAddress()
	s_impl.Version = "0.0.1"
	s_impl.Keywords = make([]string, 0)
	s_impl.Repositories = make([]string, 0)
	s_impl.Discoveries = make([]string, 0)
	s_impl.Dependencies = make([]string, 0)
	s_impl.Process = -1
	s_impl.ProxyProcess = -1
	s_impl.KeepAlive = true
	s_impl.KeepUpToDate = true

	// Read service information.
	s_impl.Services = make(map[string]interface{})
	s_impl.Services["Persistence"] = make(map[string]interface{})
	s_impl.Services["Persistence"].(map[string]interface{})["Address"], _ = config.GetAddress()
	s_impl.Services["Event"] = make(map[string]interface{})
	s_impl.Services["Event"].(map[string]interface{})["Address"], _ = config.GetAddress()

	// TODO set it from the program arguments...
	s_impl.AllowAllOrigins = allow_all_origins
	s_impl.AllowedOrigins = allowed_origins

	// Give base info to retreive it configuration.
	if len(os.Args) == 2 {
		s_impl.Id = os.Args[1] // The second argument must be the port number
	} else if len(os.Args) == 3 {
		s_impl.Id = os.Args[1]         // The second argument must be the port number
		s_impl.ConfigPath = os.Args[2] // The second argument must be the port number
	}

	// Here I will retreive the list of connections from file if there are some...
	err := s_impl.Init()
	if err != nil {
		log.Fatalf("Fail to initialyse service %s: %s", s_impl.Name, s_impl.Id)
	}

	if s_impl.Address == "" {
		s_impl.Address, _ = config.GetAddress()
	}

	// Register the echo services
	catalogpb.RegisterCatalogServiceServer(s_impl.grpcServer, s_impl)
	reflection.Register(s_impl.grpcServer)

	// Start the service.
	s_impl.StartService()

}
