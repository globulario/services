package main

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"os"
	"strings"

	"github.com/davecourtois/Utility"
	"github.com/globulario/services/golang/catalog/catalog_client"
	"github.com/globulario/services/golang/catalog/catalogpb"
	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/event/event_client"
	globular "github.com/globulario/services/golang/globular_service"
	"github.com/globulario/services/golang/interceptors"
	"github.com/globulario/services/golang/persistence/persistence_client"
	"github.com/golang/protobuf/jsonpb"
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

	// The default address.
	domain string = "localhost"
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
	ModTime            int64

	// Contain the list of service use by the catalog server.
	Services     map[string]interface{}
	Permissions  []interface{}
	Dependencies []string // The list of services needed by this services.

	// Here I will create client to services use by the catalog server.
	persistenceClient *persistence_client.Persistence_Client
	eventClient       *event_client.Event_Client
	// The grpc server.
	grpcServer *grpc.Server
}

// The http address where the configuration can be found /config
func (svr *server) GetAddress() string {
	return svr.Address
}

func (svr *server) SetAddress(address string) {
	svr.Address = address
}

func (svr *server) GetProcess() int {
	return svr.Process
}

func (svr *server) SetProcess(pid int) {
	svr.Process = pid
}

func (svr *server) GetProxyProcess() int {
	return svr.ProxyProcess
}

func (svr *server) SetProxyProcess(pid int) {
	svr.ProxyProcess = pid
}

// The path of the configuration.
func (svr *server) GetConfigurationPath() string {
	return svr.ConfigPath
}

func (svr *server) SetConfigurationPath(path string) {
	svr.ConfigPath = path
}

// The current service state
func (svr *server) GetState() string {
	return svr.State
}

func (svr *server) SetState(state string) {
	svr.State = state
}

// The last error
func (svr *server) GetLastError() string {
	return svr.LastError
}

func (svr *server) SetLastError(err string) {
	svr.LastError = err
}

// The modeTime
func (svr *server) SetModTime(modtime int64) {
	svr.ModTime = modtime
}
func (svr *server) GetModTime() int64 {
	return svr.ModTime
}

// Globular services implementation...
// The id of a particular service instance.
func (svr *server) GetId() string {
	return svr.Id
}

func (svr *server) SetId(id string) {
	svr.Id = id
}

// The name of a service, must be the gRpc Service name.
func (svr *server) GetName() string {
	return svr.Name
}
func (svr *server) SetName(name string) {
	svr.Name = name
}

func (svr *server) GetMac() string {
	return svr.Mac
}

func (svr *server) SetMac(mac string) {
	svr.Mac = mac
}

// The description of the service
func (svr *server) GetDescription() string {
	return svr.Description
}
func (svr *server) SetDescription(description string) {
	svr.Description = description
}

// The list of keywords of the services.
func (svr *server) GetKeywords() []string {
	return svr.Keywords
}
func (svr *server) SetKeywords(keywords []string) {
	svr.Keywords = keywords
}

// Dist
func (svr *server) Dist(path string) (string, error) {

	return globular.Dist(path, svr)
}

func (server *server) GetDependencies() []string {

	if server.Dependencies == nil {
		server.Dependencies = make([]string, 0)
	}

	return server.Dependencies
}

func (server *server) SetDependency(dependency string) {
	if server.Dependencies == nil {
		server.Dependencies = make([]string, 0)
	}

	// Append the depency to the list.
	if !Utility.Contains(server.Dependencies, dependency) {
		server.Dependencies = append(server.Dependencies, dependency)
	}
}

func (svr *server) GetPlatform() string {
	return globular.GetPlatform()
}

func (svr *server) GetRepositories() []string {
	return svr.Repositories
}
func (svr *server) SetRepositories(repositories []string) {
	svr.Repositories = repositories
}

func (svr *server) GetDiscoveries() []string {
	return svr.Discoveries
}
func (svr *server) SetDiscoveries(discoveries []string) {
	svr.Discoveries = discoveries
}

// The path of the executable.
func (svr *server) GetPath() string {
	return svr.Path
}
func (svr *server) SetPath(path string) {
	svr.Path = path
}

// The path of the .proto file.
func (svr *server) GetProto() string {
	return svr.Proto
}
func (svr *server) SetProto(proto string) {
	svr.Proto = proto
}

// The gRpc port.
func (svr *server) GetPort() int {
	return svr.Port
}
func (svr *server) SetPort(port int) {
	svr.Port = port
}

// The reverse proxy port (use by gRpc Web)
func (svr *server) GetProxy() int {
	return svr.Proxy
}
func (svr *server) SetProxy(proxy int) {
	svr.Proxy = proxy
}

// Can be one of http/https/tls
func (svr *server) GetProtocol() string {
	return svr.Protocol
}
func (svr *server) SetProtocol(protocol string) {
	svr.Protocol = protocol
}

// Return true if all Origins are allowed to access the mircoservice.
func (svr *server) GetAllowAllOrigins() bool {
	return svr.AllowAllOrigins
}
func (svr *server) SetAllowAllOrigins(allowAllOrigins bool) {
	svr.AllowAllOrigins = allowAllOrigins
}

// If AllowAllOrigins is false then AllowedOrigins will contain the
// list of address that can reach the services.
func (svr *server) GetAllowedOrigins() string {
	return svr.AllowedOrigins
}

func (svr *server) SetAllowedOrigins(allowedOrigins string) {
	svr.AllowedOrigins = allowedOrigins
}

// Can be a ip address or domain name.
func (svr *server) GetDomain() string {
	return svr.Domain
}
func (svr *server) SetDomain(domain string) {
	svr.Domain = domain
}

// TLS section

// If true the service run with TLS. The
func (svr *server) GetTls() bool {
	return svr.TLS
}
func (svr *server) SetTls(hasTls bool) {
	svr.TLS = hasTls
}

// The certificate authority file
func (svr *server) GetCertAuthorityTrust() string {
	return svr.CertAuthorityTrust
}
func (svr *server) SetCertAuthorityTrust(ca string) {
	svr.CertAuthorityTrust = ca
}

// The certificate file.
func (svr *server) GetCertFile() string {
	return svr.CertFile
}
func (svr *server) SetCertFile(certFile string) {
	svr.CertFile = certFile
}

// The key file.
func (svr *server) GetKeyFile() string {
	return svr.KeyFile
}
func (svr *server) SetKeyFile(keyFile string) {
	svr.KeyFile = keyFile
}

// The service version
func (svr *server) GetVersion() string {
	return svr.Version
}
func (svr *server) SetVersion(version string) {
	svr.Version = version
}

// The publisher id.
func (svr *server) GetPublisherId() string {
	return svr.PublisherId
}
func (svr *server) SetPublisherId(publisherId string) {
	svr.PublisherId = publisherId
}

func (svr *server) GetKeepUpToDate() bool {
	return svr.KeepUpToDate
}
func (svr *server) SetKeepUptoDate(val bool) {
	svr.KeepUpToDate = val
}

func (svr *server) GetKeepAlive() bool {
	return svr.KeepAlive
}
func (svr *server) SetKeepAlive(val bool) {
	svr.KeepAlive = val
}

func (svr *server) GetPermissions() []interface{} {
	return svr.Permissions
}
func (svr *server) SetPermissions(permissions []interface{}) {
	svr.Permissions = permissions
}

// Create the configuration file if is not already exist.
func (svr *server) Init() error {

	// That function is use to get access to other server.
	Utility.RegisterFunction("NewCatalogService_Client", catalog_client.NewCatalogService_Client)

	// Get the configuration path.
	err := globular.InitService(svr)
	if err != nil {
		return err
	}

	// Initialyse GRPC server.
	svr.grpcServer, err = globular.InitGrpcServer(svr, interceptors.ServerUnaryInterceptor, interceptors.ServerStreamInterceptor)
	if err != nil {
		return err
	}

	// Connect to the persistence service.
	if svr.Services["Persistence"] != nil {
		persistence_service := svr.Services["Persistence"].(map[string]interface{})
		address := persistence_service["Address"].(string)
		svr.persistenceClient, err = persistence_client.NewPersistenceService_Client(address, "persistence.PersistenceService")
		if err != nil {
			log.Println("fail to connect to persistence service ", err)
		}
	}

	if svr.Services["Event"] != nil {
		event_service := svr.Services["Event"].(map[string]interface{})
		address := event_service["Address"].(string)
		svr.eventClient, err = event_client.NewEventService_Client(address, "event.EventService")
		if err != nil {
			log.Println("fail to connect to event service ", err)
		}
	}

	return nil

}

// Save the configuration values.
func (svr *server) Save() error {
	// Create the file...
	return globular.SaveService(svr)
}

func (svr *server) StartService() error {
	return globular.StartService(svr, svr.grpcServer)
}

func (svr *server) StopService() error {
	return globular.StopService(svr, svr.grpcServer)
}

func (svr *server) Stop(context.Context, *catalogpb.StopRequest) (*catalogpb.StopResponse, error) {
	return &catalogpb.StopResponse{}, svr.StopService()
}

// Create a new connection.
func (svr *server) CreateConnection(ctx context.Context, rqst *catalogpb.CreateConnectionRqst) (*catalogpb.CreateConnectionRsp, error) {
	if rqst.Connection == nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("no connection information found in the request!")))

	}
	// So here I will call the function on the client.
	if svr.persistenceClient != nil {
		persistence := svr.Services["Persistence"].(map[string]interface{})
		if persistence["Connections"] == nil {
			persistence["Connections"] = make(map[string]interface{})
		}

		connections := persistence["Connections"].(map[string]interface{})

		storeType := int32(rqst.Connection.GetStore())
		err := svr.persistenceClient.CreateConnection(rqst.Connection.GetId(), rqst.Connection.GetName(), rqst.Connection.GetHost(), Utility.ToNumeric(rqst.Connection.Port), Utility.ToNumeric(storeType), rqst.Connection.GetUser(), rqst.Connection.GetPassword(), Utility.ToNumeric(rqst.Connection.GetTimeout()), rqst.Connection.GetOptions(), true)
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

		svr.Save()

	}

	return &catalogpb.CreateConnectionRsp{
		Result: true,
	}, nil
}

// Delete a connection.
func (svr *server) DeleteConnection(ctx context.Context, rqst *catalogpb.DeleteConnectionRqst) (*catalogpb.DeleteConnectionRsp, error) {
	return nil, nil
}

// Create unit of measure exemple inch
func (svr *server) SaveUnitOfMeasure(ctx context.Context, rqst *catalogpb.SaveUnitOfMeasureRequest) (*catalogpb.SaveUnitOfMeasureResponse, error) {
	unitOfMeasure := rqst.GetUnitOfMeasure()

	persistence := svr.Services["Persistence"].(map[string]interface{})

	if persistence["Connections"].(map[string]interface{})[rqst.ConnectionId] == nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("no connection found with id "+rqst.ConnectionId)))
	}

	connection := persistence["Connections"].(map[string]interface{})[rqst.ConnectionId].(map[string]interface{})

	// Here I will generate the _id key
	_id := Utility.GenerateUUID(unitOfMeasure.Id + unitOfMeasure.LanguageCode)
	svr.persistenceClient.DeleteOne(connection["Id"].(string), connection["Name"].(string), "UnitOfMeasure", `{ "_id" : "`+_id+`" }`, "")

	var marshaler jsonpb.Marshaler
	jsonStr, err := marshaler.MarshalToString(unitOfMeasure)
	jsonStr = `{ "_id" : "` + _id + `",` + jsonStr[1:]

	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	id, err := svr.persistenceClient.InsertOne(connection["Id"].(string), connection["Name"].(string), "UnitOfMeasure", jsonStr, "")

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
func (svr *server) SavePropertyDefinition(ctx context.Context, rqst *catalogpb.SavePropertyDefinitionRequest) (*catalogpb.SavePropertyDefinitionResponse, error) {
	propertyDefinition := rqst.PropertyDefinition

	persistence := svr.Services["Persistence"].(map[string]interface{})

	if persistence["Connections"].(map[string]interface{})[rqst.ConnectionId] == nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("no connection found with id "+rqst.ConnectionId)))
	}

	connection := persistence["Connections"].(map[string]interface{})[rqst.ConnectionId].(map[string]interface{})

	// Here I will generate the _id key
	_id := Utility.GenerateUUID(propertyDefinition.Id + propertyDefinition.LanguageCode)

	var marshaler jsonpb.Marshaler
	jsonStr, err := marshaler.MarshalToString(propertyDefinition)
	jsonStr = `{ "_id" : "` + _id + `",` + jsonStr[1:]

	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	err = svr.persistenceClient.ReplaceOne(connection["Id"].(string), connection["Name"].(string), "PropertyDefinition", `{ "_id" : "`+_id+`"}`, jsonStr, `[{"upsert": true}]`)

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
func (svr *server) SaveItemDefinition(ctx context.Context, rqst *catalogpb.SaveItemDefinitionRequest) (*catalogpb.SaveItemDefinitionResponse, error) {
	itemDefinition := rqst.ItemDefinition

	persistence := svr.Services["Persistence"].(map[string]interface{})

	if persistence["Connections"].(map[string]interface{})[rqst.ConnectionId] == nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("no connection found with id "+rqst.ConnectionId)))
	}

	connection := persistence["Connections"].(map[string]interface{})[rqst.ConnectionId].(map[string]interface{})

	// Here I will generate the _id key
	_id := Utility.GenerateUUID(itemDefinition.Id + itemDefinition.LanguageCode)

	var marshaler jsonpb.Marshaler
	jsonStr, err := marshaler.MarshalToString(itemDefinition)
	jsonStr = `{ "_id" : "` + _id + `",` + jsonStr[1:]

	// Set the db reference.
	jsonStr = strings.Replace(jsonStr, "refColId", "$ref", -1)
	jsonStr = strings.Replace(jsonStr, "refObjId", "$id", -1)
	jsonStr = strings.Replace(jsonStr, "refDbName", "$db", -1)

	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	err = svr.persistenceClient.ReplaceOne(connection["Id"].(string), connection["Name"].(string), "ItemDefinition", `{ "_id" : "`+_id+`"}`, jsonStr, `[{"upsert": true}]`)

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
func (svr *server) SaveInventory(ctx context.Context, rqst *catalogpb.SaveInventoryRequest) (*catalogpb.SaveInventoryResponse, error) {
	inventory := rqst.Inventory
	persistence := svr.Services["Persistence"].(map[string]interface{})

	if persistence["Connections"].(map[string]interface{})[rqst.ConnectionId] == nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("no connection found with id "+rqst.ConnectionId)))
	}

	connection := persistence["Connections"].(map[string]interface{})[rqst.ConnectionId].(map[string]interface{})

	var marshaler jsonpb.Marshaler

	jsonStr, err := marshaler.MarshalToString(inventory)

	// Here I will generate the _id key
	_id := Utility.GenerateUUID(inventory.LocalisationId + inventory.PacakgeId)
	jsonStr = `{ "_id" : "` + _id + `",` + jsonStr[1:]

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
	err = svr.persistenceClient.ReplaceOne(connection["Id"].(string), connection["Name"].(string), "Inventory", `{ "_id" : "`+_id+`"}`, jsonStr, `[{"upsert": true}]`)
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
func (svr *server) SaveItemInstance(ctx context.Context, rqst *catalogpb.SaveItemInstanceRequest) (*catalogpb.SaveItemInstanceResponse, error) {
	instance := rqst.ItemInstance
	persistence := svr.Services["Persistence"].(map[string]interface{})

	if persistence["Connections"].(map[string]interface{})[rqst.ConnectionId] == nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("no connection found with id "+rqst.ConnectionId)))
	}

	connection := persistence["Connections"].(map[string]interface{})[rqst.ConnectionId].(map[string]interface{})

	var marshaler jsonpb.Marshaler

	jsonStr, err := marshaler.MarshalToString(instance)
	if len(instance.Id) == 0 {
		instance.Id = Utility.RandomUUID()
	}

	// Here I will generate the _id key
	_id := Utility.GenerateUUID(instance.Id)
	jsonStr = `{ "_id" : "` + _id + `",` + jsonStr[1:]

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
	err = svr.persistenceClient.ReplaceOne(connection["Id"].(string), connection["Name"].(string), "ItemInstance", `{ "_id" : "`+_id+`"}`, jsonStr, `[{"upsert": true}]`)
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
func (svr *server) SaveManufacturer(ctx context.Context, rqst *catalogpb.SaveManufacturerRequest) (*catalogpb.SaveManufacturerResponse, error) {

	persistence := svr.Services["Persistence"].(map[string]interface{})

	if persistence["Connections"].(map[string]interface{})[rqst.ConnectionId] == nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("no connection found with id "+rqst.ConnectionId)))
	}

	connection := persistence["Connections"].(map[string]interface{})[rqst.ConnectionId].(map[string]interface{})

	manufacturer := rqst.Manufacturer

	// Here I will generate the _id key
	_id := Utility.GenerateUUID(manufacturer.Id)

	var marshaler jsonpb.Marshaler
	jsonStr, err := marshaler.MarshalToString(manufacturer)
	jsonStr = `{ "_id" : "` + _id + `",` + jsonStr[1:]

	// Always create a new
	err = svr.persistenceClient.ReplaceOne(connection["Id"].(string), connection["Name"].(string), "Manufacturer", `{ "_id" : "`+_id+`"}`, jsonStr, `[{"upsert": true}]`)
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
func (svr *server) SaveSupplier(ctx context.Context, rqst *catalogpb.SaveSupplierRequest) (*catalogpb.SaveSupplierResponse, error) {
	persistence := svr.Services["Persistence"].(map[string]interface{})

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

	var marshaler jsonpb.Marshaler
	jsonStr, err := marshaler.MarshalToString(supplier)
	jsonStr = `{ "_id" : "` + _id + `",` + jsonStr[1:]

	// Always create a new
	err = svr.persistenceClient.ReplaceOne(connection["Id"].(string), connection["Name"].(string), "Supplier", `{ "_id" : "`+_id+`"}`, jsonStr, `[{"upsert": true}]`)
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
func (svr *server) SaveLocalisation(ctx context.Context, rqst *catalogpb.SaveLocalisationRequest) (*catalogpb.SaveLocalisationResponse, error) {
	persistence := svr.Services["Persistence"].(map[string]interface{})

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

	var marshaler jsonpb.Marshaler
	jsonStr, err := marshaler.MarshalToString(localisation)
	jsonStr = `{ "_id" : "` + _id + `",` + jsonStr[1:]

	// set the object references...
	jsonStr = strings.Replace(jsonStr, "refObjId", "$id", -1)
	jsonStr = strings.Replace(jsonStr, "refColId", "$ref", -1)
	jsonStr = strings.Replace(jsonStr, "refDbName", "$db", -1)

	// Always create a new
	err = svr.persistenceClient.ReplaceOne(connection["Id"].(string), connection["Name"].(string), "Localisation", `{ "_id" : "`+_id+`"}`, jsonStr, `[{"upsert": true}]`)
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
func (svr *server) SavePackage(ctx context.Context, rqst *catalogpb.SavePackageRequest) (*catalogpb.SavePackageResponse, error) {
	persistence := svr.Services["Persistence"].(map[string]interface{})

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

	var marshaler jsonpb.Marshaler
	jsonStr, err := marshaler.MarshalToString(package_)
	jsonStr = `{ "_id" : "` + _id + `",` + jsonStr[1:]

	// set the object references...
	jsonStr = strings.Replace(jsonStr, "refObjId", "$id", -1)
	jsonStr = strings.Replace(jsonStr, "refColId", "$ref", -1)
	jsonStr = strings.Replace(jsonStr, "refDbName", "$db", -1)

	// Always create a new
	err = svr.persistenceClient.ReplaceOne(connection["Id"].(string), connection["Name"].(string), "Package", `{ "_id" : "`+_id+`"}`, jsonStr, `[{"upsert": true}]`)
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
func (svr *server) SavePackageSupplier(ctx context.Context, rqst *catalogpb.SavePackageSupplierRequest) (*catalogpb.SavePackageSupplierResponse, error) {
	persistence := svr.Services["Persistence"].(map[string]interface{})

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
	_, err := svr.persistenceClient.FindOne(connection["Id"].(string), rqst.PackageSupplier.Package.RefDbName, rqst.PackageSupplier.Package.RefColId, `{"_id":"`+rqst.PackageSupplier.Package.RefObjId+`"}`, "")
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// Test if the supplier exist.
	_, err = svr.persistenceClient.FindOne(connection["Id"].(string), rqst.PackageSupplier.Supplier.RefDbName, rqst.PackageSupplier.Supplier.RefColId, `{"_id":"`+rqst.PackageSupplier.Supplier.RefObjId+`"}`, "")
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// Here I will generate the _id key
	_id := Utility.GenerateUUID(packageSupplier.Id)

	var marshaler jsonpb.Marshaler
	jsonStr, err := marshaler.MarshalToString(packageSupplier)
	jsonStr = `{ "_id" : "` + _id + `",` + jsonStr[1:]

	// set the object references...
	jsonStr = strings.Replace(jsonStr, "refObjId", "$id", -1)
	jsonStr = strings.Replace(jsonStr, "refColId", "$ref", -1)
	jsonStr = strings.Replace(jsonStr, "refDbName", "$db", -1)

	// Always create a new
	err = svr.persistenceClient.ReplaceOne(connection["Id"].(string), connection["Name"].(string), "PackageSupplier", `{ "_id" : "`+_id+`"}`, jsonStr, `[{"upsert": true}]`)
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
func (svr *server) SaveItemManufacturer(ctx context.Context, rqst *catalogpb.SaveItemManufacturerRequest) (*catalogpb.SaveItemManufacturerResponse, error) {
	persistence := svr.Services["Persistence"].(map[string]interface{})

	if persistence["Connections"].(map[string]interface{})[rqst.ConnectionId] == nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("no connection found with id "+rqst.ConnectionId)))
	}

	connection := persistence["Connections"].(map[string]interface{})[rqst.ConnectionId].(map[string]interface{})

	// Test if the item exist
	_, err := svr.persistenceClient.FindOne(connection["Id"].(string), rqst.ItemManafacturer.Item.RefDbName, rqst.ItemManafacturer.Item.RefColId, `{"_id":"`+rqst.ItemManafacturer.Item.RefObjId+`"}`, "")
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// Test if the supplier exist.
	_, err = svr.persistenceClient.FindOne(connection["Id"].(string), rqst.ItemManafacturer.Manufacturer.RefDbName, rqst.ItemManafacturer.Manufacturer.RefColId, `{"_id":"`+rqst.ItemManafacturer.Manufacturer.RefObjId+`"}`, "")
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

	var marshaler jsonpb.Marshaler
	jsonStr, err := marshaler.MarshalToString(itemManafacturer)
	jsonStr = `{ "_id" : "` + _id + `",` + jsonStr[1:]

	// set the object references...
	jsonStr = strings.Replace(jsonStr, "refObjId", "$id", -1)
	jsonStr = strings.Replace(jsonStr, "refColId", "$ref", -1)
	jsonStr = strings.Replace(jsonStr, "refDbName", "$db", -1)

	// Always create a new
	err = svr.persistenceClient.ReplaceOne(connection["Id"].(string), connection["Name"].(string), "ItemManufacturer", `{ "_id" : "`+_id+`"}`, jsonStr, `[{"upsert": true}]`)
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
func (svr *server) SaveCategory(ctx context.Context, rqst *catalogpb.SaveCategoryRequest) (*catalogpb.SaveCategoryResponse, error) {
	persistence := svr.Services["Persistence"].(map[string]interface{})

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

	var marshaler jsonpb.Marshaler
	jsonStr, err := marshaler.MarshalToString(category)
	jsonStr = `{ "_id" : "` + _id + `",` + jsonStr[1:]

	// Always create a new
	err = svr.persistenceClient.ReplaceOne(connection["Id"].(string), connection["Name"].(string), "Category", `{ "_id" : "`+_id+`"}`, jsonStr, `[{"upsert": true}]`)
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
func (svr *server) AppendItemDefinitionCategory(ctx context.Context, rqst *catalogpb.AppendItemDefinitionCategoryRequest) (*catalogpb.AppendItemDefinitionCategoryResponse, error) {

	persistence := svr.Services["Persistence"].(map[string]interface{})

	if persistence["Connections"].(map[string]interface{})[rqst.ConnectionId] == nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("no connection found with id "+rqst.ConnectionId)))
	}

	connection := persistence["Connections"].(map[string]interface{})[rqst.ConnectionId].(map[string]interface{})

	var marshaler jsonpb.Marshaler
	jsonStr, err := marshaler.MarshalToString(rqst.Category)

	// Set the db reference.
	jsonStr = strings.Replace(jsonStr, "refObjId", "$id", -1)
	jsonStr = strings.Replace(jsonStr, "refColId", "$ref", -1)
	jsonStr = strings.Replace(jsonStr, "refDbName", "$db", -1)

	// Now I will modify the jsonStr to insert the value in the array.
	jsonStr = `{ "$push": { "categories":` + jsonStr + `}}`

	// Always create a new
	err = svr.persistenceClient.UpdateOne(connection["Id"].(string), connection["Name"].(string), rqst.ItemDefinition.RefColId, `{ "_id" : "`+rqst.ItemDefinition.RefObjId+`"}`, jsonStr, `[]`)
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
func (svr *server) RemoveItemDefinitionCategory(ctx context.Context, rqst *catalogpb.RemoveItemDefinitionCategoryRequest) (*catalogpb.RemoveItemDefinitionCategoryResponse, error) {

	persistence := svr.Services["Persistence"].(map[string]interface{})

	if persistence["Connections"].(map[string]interface{})[rqst.ConnectionId] == nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("no connection found with id "+rqst.ConnectionId)))
	}

	connection := persistence["Connections"].(map[string]interface{})[rqst.ConnectionId].(map[string]interface{})

	var marshaler jsonpb.Marshaler
	jsonStr, err := marshaler.MarshalToString(rqst.Category)

	// Set the db reference.
	jsonStr = strings.Replace(jsonStr, "refObjId", "$id", -1)
	jsonStr = strings.Replace(jsonStr, "refColId", "$ref", -1)
	jsonStr = strings.Replace(jsonStr, "refDbName", "$db", -1)

	// Now I will modify the jsonStr to insert the value in the array.
	jsonStr = `{ "$pull": { "categories":` + jsonStr + `}}` // remove a particular item.

	// Always create a new
	err = svr.persistenceClient.UpdateOne(connection["Id"].(string), connection["Name"].(string), rqst.ItemDefinition.RefColId, `{ "_id" : "`+rqst.ItemDefinition.RefObjId+`"}`, jsonStr, `[]`)
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
func (svr *server) GetItemInstance(ctx context.Context, rqst *catalogpb.GetItemInstanceRequest) (*catalogpb.GetItemInstanceResponse, error) {
	persistence := svr.Services["Persistence"].(map[string]interface{})

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

	obj, err := svr.persistenceClient.FindOne(connection["Id"].(string), connection["Name"].(string), "ItemInstance", query, `[{"Projection":{"_id":0}}]`)
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
	err = jsonpb.UnmarshalString(jsonStr, instance)
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
func (svr *server) GetItemInstances(ctx context.Context, rqst *catalogpb.GetItemInstancesRequest) (*catalogpb.GetItemInstancesResponse, error) {
	persistence := svr.Services["Persistence"].(map[string]interface{})

	if persistence["Connections"].(map[string]interface{})[rqst.ConnectionId] == nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("no connection found with id "+rqst.ConnectionId)))
	}

	connection := persistence["Connections"].(map[string]interface{})[rqst.ConnectionId].(map[string]interface{})

	options, err := svr.getOptionsString(rqst.Options)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	if len(rqst.Query) == 0 {
		rqst.Query = `{}`
	}

	values, err := svr.persistenceClient.Find(connection["Id"].(string), connection["Name"].(string), "ItemInstance", rqst.Query, options)
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
	err = jsonpb.UnmarshalString(string(jsonStr), instances)
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
func (svr *server) GetItemDefinition(ctx context.Context, rqst *catalogpb.GetItemDefinitionRequest) (*catalogpb.GetItemDefinitionResponse, error) {
	persistence := svr.Services["Persistence"].(map[string]interface{})

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

	obj, err := svr.persistenceClient.FindOne(connection["Id"].(string), connection["Name"].(string), "ItemDefinition", query, `[{"Projection":{"_id":0}}]`)
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
	err = jsonpb.UnmarshalString(jsonStr, definition)
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
func (svr *server) GetInventories(ctx context.Context, rqst *catalogpb.GetInventoriesRequest) (*catalogpb.GetInventoriesResponse, error) {
	persistence := svr.Services["Persistence"].(map[string]interface{})

	if persistence["Connections"].(map[string]interface{})[rqst.ConnectionId] == nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("no connection found with id "+rqst.ConnectionId)))
	}

	connection := persistence["Connections"].(map[string]interface{})[rqst.ConnectionId].(map[string]interface{})

	options, err := svr.getOptionsString(rqst.Options)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	if len(rqst.Query) == 0 {
		rqst.Query = `{}`
	}
	values, err := svr.persistenceClient.Find(connection["Id"].(string), connection["Name"].(string), "Inventory", rqst.Query, options)
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
	err = jsonpb.UnmarshalString(string(jsonStr), inventories)
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
func (svr *server) GetItemDefinitions(ctx context.Context, rqst *catalogpb.GetItemDefinitionsRequest) (*catalogpb.GetItemDefinitionsResponse, error) {
	persistence := svr.Services["Persistence"].(map[string]interface{})

	if persistence["Connections"].(map[string]interface{})[rqst.ConnectionId] == nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("no connection found with id "+rqst.ConnectionId)))
	}

	connection := persistence["Connections"].(map[string]interface{})[rqst.ConnectionId].(map[string]interface{})

	options, err := svr.getOptionsString(rqst.Options)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	if len(rqst.Query) == 0 {
		rqst.Query = `{}`
	}

	values, err := svr.persistenceClient.Find(connection["Id"].(string), connection["Name"].(string), "ItemDefinition", rqst.Query, options)
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
	err = jsonpb.UnmarshalString(string(jsonStr), definitions)
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
func (svr *server) GetSupplier(ctx context.Context, rqst *catalogpb.GetSupplierRequest) (*catalogpb.GetSupplierResponse, error) {
	persistence := svr.Services["Persistence"].(map[string]interface{})

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

	obj, err := svr.persistenceClient.FindOne(connection["Id"].(string), connection["Name"].(string), "Supplier", query, `[{"Projection":{"_id":0}}]`)
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
	err = jsonpb.UnmarshalString(jsonStr, supplier)
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
func (svr *server) GetSuppliers(ctx context.Context, rqst *catalogpb.GetSuppliersRequest) (*catalogpb.GetSuppliersResponse, error) {
	persistence := svr.Services["Persistence"].(map[string]interface{})

	if persistence["Connections"].(map[string]interface{})[rqst.ConnectionId] == nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("no connection found with id "+rqst.ConnectionId)))
	}

	connection := persistence["Connections"].(map[string]interface{})[rqst.ConnectionId].(map[string]interface{})

	options, err := svr.getOptionsString(rqst.Options)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	if len(rqst.Query) == 0 {
		rqst.Query = `{}`
	}
	values, err := svr.persistenceClient.Find(connection["Id"].(string), connection["Name"].(string), "Supplier", rqst.Query, options)
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
	err = jsonpb.UnmarshalString(string(jsonStr), suppliers)
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
func (svr *server) GetSupplierPackages(ctx context.Context, rqst *catalogpb.GetSupplierPackagesRequest) (*catalogpb.GetSupplierPackagesResponse, error) {
	persistence := svr.Services["Persistence"].(map[string]interface{})

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

	values, err := svr.persistenceClient.Find(connection["Id"].(string), connection["Name"].(string), "PackageSupplier", query, `[{"Projection":{"_id":1}}]`)
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
		obj, err := svr.persistenceClient.FindOne(connection["Id"].(string), connection["Name"].(string), "PackageSupplier", `{"_id":"`+results[i]["_id"].(string)+`"}`, `[{"Projection":{"_id":0}}]`)
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
		err = jsonpb.UnmarshalString(jsonStr, packageSupplier)
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
func (svr *server) GetPackage(ctx context.Context, rqst *catalogpb.GetPackageRequest) (*catalogpb.GetPackageResponse, error) {
	persistence := svr.Services["Persistence"].(map[string]interface{})

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

	obj, err := svr.persistenceClient.FindOne(connection["Id"].(string), connection["Name"].(string), "Package", query, `[{"Projection":{"_id":0}}]`)
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
	err = jsonpb.UnmarshalString(jsonStr, package_)
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
func (svr *server) GetPackages(ctx context.Context, rqst *catalogpb.GetPackagesRequest) (*catalogpb.GetPackagesResponse, error) {
	persistence := svr.Services["Persistence"].(map[string]interface{})

	if persistence["Connections"].(map[string]interface{})[rqst.ConnectionId] == nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("no connection found with id "+rqst.ConnectionId)))
	}

	connection := persistence["Connections"].(map[string]interface{})[rqst.ConnectionId].(map[string]interface{})
	options, err := svr.getOptionsString(rqst.Options)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	values, err := svr.persistenceClient.Find(connection["Id"].(string), connection["Name"].(string), "Package", rqst.Query, options)
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
	err = jsonpb.UnmarshalString(jsonStr, packages)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &catalogpb.GetPackagesResponse{
		Packages: packages.Packages,
	}, nil

}

func (svr *server) getLocalisation(localisationId string, connectionId string) (*catalogpb.Localisation, error) {
	persistence := svr.Services["Persistence"].(map[string]interface{})

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

	obj, err := svr.persistenceClient.FindOne(connection["Id"].(string), connection["Name"].(string), "Localisation", query, `[{"Projection":{"_id":0}}]`)
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
	err = jsonpb.UnmarshalString(jsonStr, localisation)
	if err != nil {
		return nil, err
	}

	return localisation, nil
}

// Get Localisation
func (svr *server) GetLocalisation(ctx context.Context, rqst *catalogpb.GetLocalisationRequest) (*catalogpb.GetLocalisationResponse, error) {

	localisation, err := svr.getLocalisation(rqst.LocalisationId, rqst.ConnectionId)

	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &catalogpb.GetLocalisationResponse{
		Localisation: localisation,
	}, nil

}

func (svr *server) getLocalisations(query string, options string, connectionId string) ([]*catalogpb.Localisation, error) {
	persistence := svr.Services["Persistence"].(map[string]interface{})

	if persistence["Connections"].(map[string]interface{})[connectionId] == nil {
		return nil, errors.New("no connection found with id " + connectionId)
	}

	connection := persistence["Connections"].(map[string]interface{})[connectionId].(map[string]interface{})
	options, err := svr.getOptionsString(options)
	if err != nil {
		return nil, err
	}

	if len(query) == 0 {
		query = `{}`
	}

	values, err := svr.persistenceClient.Find(connection["Id"].(string), connection["Name"].(string), "Localisation", query, options)
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
	err = jsonpb.UnmarshalString(jsonStr, localisations)

	if err != nil {
		return nil, err
	}

	return localisations.Localisations, nil

}

// Get Packages
func (svr *server) GetLocalisations(ctx context.Context, rqst *catalogpb.GetLocalisationsRequest) (*catalogpb.GetLocalisationsResponse, error) {

	// unmarshall object
	localisations, err := svr.getLocalisations(rqst.Query, rqst.Options, rqst.ConnectionId)
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
func (svr *server) getCategory(categoryId string, connectionId string) (*catalogpb.Category, error) {
	persistence := svr.Services["Persistence"].(map[string]interface{})

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

	obj, err := svr.persistenceClient.FindOne(connection["Id"].(string), connection["Name"].(string), "Category", query, `[{"Projection":{"_id":0}}]`)
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
	err = jsonpb.UnmarshalString(jsonStr, category)
	if err != nil {
		return nil, err
	}

	return category, nil
}

func (svr *server) GetCategory(ctx context.Context, rqst *catalogpb.GetCategoryRequest) (*catalogpb.GetCategoryResponse, error) {

	category, err := svr.getCategory(rqst.CategoryId, rqst.ConnectionId)

	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &catalogpb.GetCategoryResponse{
		Category: category,
	}, nil

}

func (svr *server) getCategories(query string, options string, connectionId string) ([]*catalogpb.Category, error) {
	persistence := svr.Services["Persistence"].(map[string]interface{})

	if persistence["Connections"].(map[string]interface{})[connectionId] == nil {
		return nil, errors.New("no connection found with id " + connectionId)
	}

	connection := persistence["Connections"].(map[string]interface{})[connectionId].(map[string]interface{})
	options, err := svr.getOptionsString(options)
	if err != nil {
		return nil, err
	}

	if len(query) == 0 {
		query = `{}`
	}

	values, err := svr.persistenceClient.Find(connection["Id"].(string), connection["Name"].(string), "Category", query, options)
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
	err = jsonpb.UnmarshalString(jsonStr, categories)

	if err != nil {
		return nil, err
	}

	return categories.Categories, nil

}

// Get Packages
func (svr *server) GetCategories(ctx context.Context, rqst *catalogpb.GetCategoriesRequest) (*catalogpb.GetCategoriesResponse, error) {

	// unmarshall object
	categories, err := svr.getCategories(rqst.Query, rqst.Options, rqst.ConnectionId)
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
func (svr *server) GetManufacturer(ctx context.Context, rqst *catalogpb.GetManufacturerRequest) (*catalogpb.GetManufacturerResponse, error) {
	persistence := svr.Services["Persistence"].(map[string]interface{})

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

	obj, err := svr.persistenceClient.FindOne(connection["Id"].(string), connection["Name"].(string), "Package", query, `[{"Projection":{"_id":0}}]`)
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
	err = jsonpb.UnmarshalString(jsonStr, manufacturer)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &catalogpb.GetManufacturerResponse{
		Manufacturer: manufacturer,
	}, nil

}

func (svr *server) getOptionsString(options string) (string, error) {
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

	optionsStr, err := json.Marshal(options_)
	return string(optionsStr), err
}

// Get Manufacturers
func (svr *server) GetManufacturers(ctx context.Context, rqst *catalogpb.GetManufacturersRequest) (*catalogpb.GetManufacturersResponse, error) {
	persistence := svr.Services["Persistence"].(map[string]interface{})

	if persistence["Connections"].(map[string]interface{})[rqst.ConnectionId] == nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("no connection found with id "+rqst.ConnectionId)))
	}

	connection := persistence["Connections"].(map[string]interface{})[rqst.ConnectionId].(map[string]interface{})

	options, err := svr.getOptionsString(rqst.Options)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	if len(rqst.Query) == 0 {
		rqst.Query = `{}`
	}

	values, err := svr.persistenceClient.Find(connection["Id"].(string), connection["Name"].(string), "Manufacturer", rqst.Query, options)
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
	err = jsonpb.UnmarshalString(jsonStr, manufacturers)
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
func (svr *server) GetUnitOfMeasures(ctx context.Context, rqst *catalogpb.GetUnitOfMeasuresRequest) (*catalogpb.GetUnitOfMeasuresResponse, error) {
	persistence := svr.Services["Persistence"].(map[string]interface{})

	if persistence["Connections"].(map[string]interface{})[rqst.ConnectionId] == nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("no connection found with id "+rqst.ConnectionId)))
	}

	connection := persistence["Connections"].(map[string]interface{})[rqst.ConnectionId].(map[string]interface{})
	options, err := svr.getOptionsString(rqst.Options)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	if len(rqst.Query) == 0 {
		rqst.Query = `{}`
	}

	values, err := svr.persistenceClient.Find(connection["Id"].(string), connection["Name"].(string), "UnitOfMeasure", rqst.Query, options)
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
	err = jsonpb.UnmarshalString(string(jsonStr), unitOfMeasures)
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
func (svr *server) GetUnitOfMeasure(ctx context.Context, rqst *catalogpb.GetUnitOfMeasureRequest) (*catalogpb.GetUnitOfMeasureResponse, error) {
	persistence := svr.Services["Persistence"].(map[string]interface{})

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

	obj, err := svr.persistenceClient.FindOne(connection["Id"].(string), connection["Name"].(string), "UnitOfMeasure", query, `[{"Projection":{"_id":0}}]`)
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
	err = jsonpb.UnmarshalString(jsonStr, unitOfMeasure)
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
func (svr *server) DeletePackage(ctx context.Context, rqst *catalogpb.DeletePackageRequest) (*catalogpb.DeletePackageResponse, error) {
	persistence := svr.Services["Persistence"].(map[string]interface{})
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

	err := svr.persistenceClient.DeleteOne(connection["Id"].(string), connection["Name"].(string), "Package", `{"_id":"`+_id+`"}`, "")

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
func (svr *server) DeletePackageSupplier(ctx context.Context, rqst *catalogpb.DeletePackageSupplierRequest) (*catalogpb.DeletePackageSupplierResponse, error) {
	persistence := svr.Services["Persistence"].(map[string]interface{})
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

	err := svr.persistenceClient.DeleteOne(connection["Id"].(string), connection["Name"].(string), "PackageSupplier", `{"_id":"`+_id+`"}`, "")

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
func (svr *server) DeleteSupplier(ctx context.Context, rqst *catalogpb.DeleteSupplierRequest) (*catalogpb.DeleteSupplierResponse, error) {
	persistence := svr.Services["Persistence"].(map[string]interface{})
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
	err := svr.persistenceClient.DeleteOne(connection["Id"].(string), connection["Name"].(string), "Supplier", `{"_id":"`+_id+`"}`, "")

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
func (svr *server) DeletePropertyDefinition(ctx context.Context, rqst *catalogpb.DeletePropertyDefinitionRequest) (*catalogpb.DeletePropertyDefinitionResponse, error) {
	return nil, nil
}

// Delete unit of measure
func (svr *server) DeleteUnitOfMeasure(ctx context.Context, rqst *catalogpb.DeleteUnitOfMeasureRequest) (*catalogpb.DeleteUnitOfMeasureResponse, error) {
	return nil, nil
}

// Delete Item Instance
func (svr *server) DeleteItemInstance(ctx context.Context, rqst *catalogpb.DeleteItemInstanceRequest) (*catalogpb.DeleteItemInstanceResponse, error) {
	return nil, nil
}

// Delete Manufacturer
func (svr *server) DeleteManufacturer(ctx context.Context, rqst *catalogpb.DeleteManufacturerRequest) (*catalogpb.DeleteManufacturerResponse, error) {
	return nil, nil
}

// Delete Item Manufacturer
func (svr *server) DeleteItemManufacturer(ctx context.Context, rqst *catalogpb.DeleteItemManufacturerRequest) (*catalogpb.DeleteItemManufacturerResponse, error) {
	return nil, nil
}

// Delete Category
func (svr *server) DeleteCategory(ctx context.Context, rqst *catalogpb.DeleteCategoryRequest) (*catalogpb.DeleteCategoryResponse, error) {
	return nil, nil
}

func (svr *server) deleteLocalisation(localisation *catalogpb.Localisation, connectionId string) error {
	persistence := svr.Services["Persistence"].(map[string]interface{})
	if persistence["Connections"].(map[string]interface{})[connectionId] == nil {
		return errors.New("no connection found with id " + connectionId)
	}

	connection := persistence["Connections"].(map[string]interface{})[connectionId].(map[string]interface{})

	// I will remove referencing object...
	referenced, err := svr.getLocalisations(`{"subLocalisations.values.$id":"`+Utility.GenerateUUID(localisation.GetId()+localisation.GetLanguageCode())+`"}`, "", connectionId)
	if err == nil {
		refStr := `{"$id":"` + Utility.GenerateUUID(localisation.GetId()+localisation.GetLanguageCode()) + `","$ref":"Localisation","$db":"` + connection["Name"].(string) + `"}`
		for i := 0; i < len(referenced); i++ {
			// Now I will modify the jsonStr to insert the value in the array.
			query := `{"$pull":{"subLocalisations.values":` + refStr + `}}` // remove a particular item.
			_id := Utility.GenerateUUID(referenced[i].Id + referenced[i].LanguageCode)
			// Always create a new
			err = svr.persistenceClient.UpdateOne(connection["Id"].(string), connection["Name"].(string), "Localisation", `{"_id" : "`+_id+`"}`, query, `[]`)
			if err != nil {
				return err
			}
		}
	}

	// So here I will delete all sub-localisation to...
	if localisation.GetSubLocalisations() != nil {
		for i := 0; i < len(localisation.GetSubLocalisations().GetValues()); i++ {
			subLocalisation, err := svr.getLocalisation(localisation.GetSubLocalisations().GetValues()[i].GetRefObjId(), connectionId)
			if err == nil {
				err := svr.deleteLocalisation(subLocalisation, connectionId)
				if err != nil {
					return err
				}
			}
		}
	}

	// Here I will generate the _id key
	_id := Utility.GenerateUUID(localisation.Id + localisation.LanguageCode)

	return svr.persistenceClient.DeleteOne(connection["Id"].(string), connection["Name"].(string), "Localisation", `{"_id":"`+_id+`"}`, "")

}

// Delete Localisation
func (svr *server) DeleteLocalisation(ctx context.Context, rqst *catalogpb.DeleteLocalisationRequest) (*catalogpb.DeleteLocalisationResponse, error) {

	err := svr.deleteLocalisation(rqst.Localisation, rqst.ConnectionId)
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
func (svr *server) DeleteInventory(ctx context.Context, rqst *catalogpb.DeleteInventoryRequest) (*catalogpb.DeleteInventoryResponse, error) {

	persistence := svr.Services["Persistence"].(map[string]interface{})
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
	err := svr.persistenceClient.DeleteOne(connection["Id"].(string), connection["Name"].(string), "Inventory", `{"_id":"`+_id+`"}`, "")

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
	s_impl.Name = Utility.GetExecName(os.Args[0])
	s_impl.Port = defaultPort
	s_impl.Proxy = defaultProxy
	s_impl.Protocol = "grpc"
	s_impl.Domain = domain
	s_impl.Version = "0.0.1"
	s_impl.Keywords = make([]string, 0)
	s_impl.Repositories = make([]string, 0)
	s_impl.Discoveries = make([]string, 0)
	s_impl.Dependencies = make([]string, 0)
	s_impl.Process = -1
	s_impl.ProxyProcess = -1
	s_impl.KeepAlive = true

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

	// Register the echo services
	catalogpb.RegisterCatalogServiceServer(s_impl.grpcServer, s_impl)
	reflection.Register(s_impl.grpcServer)

	// Start the service.
	s_impl.StartService()

}
