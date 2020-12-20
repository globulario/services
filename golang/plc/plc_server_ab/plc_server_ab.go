package main

import (
	"context"
	"encoding/json"
	"errors"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"strings"

	"github.com/globulario/Globular/Interceptors"
	globular "github.com/globulario/services/golang/globular_service"

	"github.com/davecourtois/Utility"
	"github.com/globulario/services/golang/plc/plc_client"
	"github.com/globulario/services/golang/plc/plcpb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"

	//	"google.golang.org/grpc/grpclog"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"

	"fmt"
	"math"
	"plctag"
)

// TODO take care of TLS/https
var (
	defaultPort  = 10028
	defaultProxy = 10029

	// By default all origins are allowed.
	allow_all_origins = true

	// comma separeated values.
	allowed_origins string = ""

	domain string = "localhost"
)

type TagType int

const (
	BOOL_TAG_TYPE   TagType = 0
	SINT_TAG_TYPE   TagType = 1
	INT_TAG_TYPE    TagType = 2
	DINT_TAG_TYPE   TagType = 3
	REAL_TAG_TYPE   TagType = 4
	LREAL_TAG_TYPE  TagType = 5
	LINT_TAG_TYPE   TagType = 6
	UNKNOW_TAG_TYPE TagType = 7
)

// Keep connection information here.
type connection struct {
	Id       string // The connection id
	IP       string // can also be ipv4 addresse.
	Port     plcpb.PortType
	Slot     int32
	Rack     int32
	Timeout  int64         // Time out for reading/writing tags
	Cpu      plcpb.CpuType // The cpu type
	Protocol plcpb.ProtocolType

	isOpen bool
}

// Value need by Globular to start the services...
type server struct {
	// The global attribute of the services.
	Id              string
	Name            string
	Path            string
	Proto           string
	Port            int
	Proxy           int
	AllowAllOrigins bool
	AllowedOrigins  string // comma separated string.
	Protocol        string
	Domain          string
	Description     string
	Keywords        []string
	Repositories    []string
	Discoveries     []string
	PublisherId     string

	// self-signed X.509 public keys for distribution
	CertFile string
	// a private RSA key to sign and authenticate the public key
	KeyFile string
	// a private RSA key to sign and authenticate the public key
	CertAuthorityTrust string
	TLS                bool
	Version            string
	KeepUpToDate       bool
	KeepAlive          bool
	Permissions        []interface{} // contains the action permission for the services.

	// The grpc server.
	grpcServer *grpc.Server

	// use only for serialization.
	Connections []connection

	// The list of open tags.
	paths map[string]string
	tags  map[string]int32
}

// Create the configuration file if is not already exist.
func (self *server) init() {

	// That function is use to get access to other server.
	Utility.RegisterFunction("NewPlcService_Client", plc_client.NewPlcService_Client)

	// Here I will retreive the list of connections from file if there are some...
	dir, _ := filepath.Abs(filepath.Dir(os.Args[0]))
	file, err := ioutil.ReadFile(dir + "/config.json")
	if err == nil {
		err := json.Unmarshal([]byte(file), &self)
		if err != nil {
			fmt.Println(err)
		}
	} else {
		// save it the first time to generate the configuratio file.
		if len(self.Id) == 0 {
			// Generate random id for the server instance.
			self.Id = Utility.RandomUUID()
		}
		self.Save()

	}
}

// Globular services implementation...
// The id of a particular service instance.
func (self *server) GetId() string {
	return self.Id
}
func (self *server) SetId(id string) {
	self.Id = id
}

// The name of a service, must be the gRpc Service name.
func (self *server) GetName() string {
	return self.Name
}
func (self *server) SetName(name string) {
	self.Name = name
}

// The description of the service
func (self *server) GetDescription() string {
	return self.Description
}
func (self *server) SetDescription(description string) {
	self.Description = description
}

// The list of keywords of the services.
func (self *server) GetKeywords() []string {
	return self.Keywords
}
func (self *server) SetKeywords(keywords []string) {
	self.Keywords = keywords
}

func (self *server) GetRepositories() []string {
	return self.Repositories
}
func (self *server) SetRepositories(repositories []string) {
	self.Repositories = repositories
}

func (self *server) GetDiscoveries() []string {
	return self.Discoveries
}
func (self *server) SetDiscoveries(discoveries []string) {
	self.Discoveries = discoveries
}

// Dist
func (self *server) Dist(path string) error {
	return globular.Dist(path, self)
}

func (self *server) GetPlatform() string {
	return globular.GetPlatform()
}

// The path of the executable.
func (self *server) GetPath() string {
	return self.Path
}
func (self *server) SetPath(path string) {
	self.Path = path
}

// The path of the .proto file.
func (self *server) GetProto() string {
	return self.Proto
}
func (self *server) SetProto(proto string) {
	self.Proto = proto
}

// The gRpc port.
func (self *server) GetPort() int {
	return self.Port
}
func (self *server) SetPort(port int) {
	self.Port = port
}

// The reverse proxy port (use by gRpc Web)
func (self *server) GetProxy() int {
	return self.Proxy
}
func (self *server) SetProxy(proxy int) {
	self.Proxy = proxy
}

// Can be one of http/https/tls
func (self *server) GetProtocol() string {
	return self.Protocol
}
func (self *server) SetProtocol(protocol string) {
	self.Protocol = protocol
}

// Return true if all Origins are allowed to access the mircoservice.
func (self *server) GetAllowAllOrigins() bool {
	return self.AllowAllOrigins
}
func (self *server) SetAllowAllOrigins(allowAllOrigins bool) {
	self.AllowAllOrigins = allowAllOrigins
}

// If AllowAllOrigins is false then AllowedOrigins will contain the
// list of address that can reach the services.
func (self *server) GetAllowedOrigins() string {
	return self.AllowedOrigins
}

func (self *server) SetAllowedOrigins(allowedOrigins string) {
	self.AllowedOrigins = allowedOrigins
}

// Can be a ip address or domain name.
func (self *server) GetDomain() string {
	return self.Domain
}
func (self *server) SetDomain(domain string) {
	self.Domain = domain
}

// TLS section

// If true the service run with TLS. The
func (self *server) GetTls() bool {
	return self.TLS
}
func (self *server) SetTls(hasTls bool) {
	self.TLS = hasTls
}

// The certificate authority file
func (self *server) GetCertAuthorityTrust() string {
	return self.CertAuthorityTrust
}
func (self *server) SetCertAuthorityTrust(ca string) {
	self.CertAuthorityTrust = ca
}

// The certificate file.
func (self *server) GetCertFile() string {
	return self.CertFile
}
func (self *server) SetCertFile(certFile string) {
	self.CertFile = certFile
}

// The key file.
func (self *server) GetKeyFile() string {
	return self.KeyFile
}
func (self *server) SetKeyFile(keyFile string) {
	self.KeyFile = keyFile
}

// The service version
func (self *server) GetVersion() string {
	return self.Version
}
func (self *server) SetVersion(version string) {
	self.Version = version
}

// The publisher id.
func (self *server) GetPublisherId() string {
	return self.PublisherId
}
func (self *server) SetPublisherId(publisherId string) {
	self.PublisherId = publisherId
}

func (self *server) GetKeepUpToDate() bool {
	return self.KeepUpToDate
}
func (self *server) SetKeepUptoDate(val bool) {
	self.KeepUpToDate = val
}

func (self *server) GetKeepAlive() bool {
	return self.KeepAlive
}
func (self *server) SetKeepAlive(val bool) {
	self.KeepAlive = val
}

func (self *server) GetPermissions() []interface{} {
	return self.Permissions
}
func (self *server) SetPermissions(permissions []interface{}) {
	self.Permissions = permissions
}

// Create the configuration file if is not already exist.
func (self *server) Init() error {

	// That function is use to get access to other server.
	Utility.RegisterFunction("NewPlcService_Client", plc_client.NewPlcService_Client)

	// Get the configuration path.
	dir, _ := filepath.Abs(filepath.Dir(os.Args[0]))

	err := globular.InitService(dir+"/config.json", self)
	if err != nil {
		return err
	}

	// Initialyse GRPC server.
	self.grpcServer, err = globular.InitGrpcServer(self, Interceptors.ServerUnaryInterceptor, Interceptors.ServerStreamInterceptor)
	if err != nil {
		return err
	}

	return nil

}

// Save the configuration values.
func (self *server) Save() error {
	// Create the file...
	dir, _ := filepath.Abs(filepath.Dir(os.Args[0]))
	return globular.SaveService(dir+"/config.json", self)
}

func (self *server) StartService() error {
	return globular.StartService(self, self.grpcServer)
}

func (self *server) StopService() error {
	return globular.StopService(self, self.grpcServer)
}

func (self *server) Stop(context.Context, *plcpb.StopRequest) (*plcpb.StopResponse, error) {
	return &plcpb.StopResponse{}, self.StopService()
}

////////////////// Now the API ////////////////

func (self *server) getCpuType(cpuType plcpb.CpuType) string {
	if cpuType == plcpb.CpuType_PLC {
		return "PLC"
	} else if cpuType == plcpb.CpuType_PLC5 {
		return "PLC5"
	} else if cpuType == plcpb.CpuType_SLC {
		return "SLC"

	} else if cpuType == plcpb.CpuType_SLC500 {
		return "SLC500"

	} else if cpuType == plcpb.CpuType_MICROLOGIX {
		return "MICROLOGIX"

	} else if cpuType == plcpb.CpuType_MLGX {
		return "MLGX"

	} else if cpuType == plcpb.CpuType_COMPACTLOGIX {
		return "COMPACTLOGIX"

	} else if cpuType == plcpb.CpuType_CLGX {
		return "CLGX"

	} else if cpuType == plcpb.CpuType_LGX {
		return "LGX"

	} else if cpuType == plcpb.CpuType_CONTROLLOGIX {
		return "CONTROLLOGIX"

	} else if cpuType == plcpb.CpuType_CONTROLOGIX {
		return "CONTROLOGIX"

	} else if cpuType == plcpb.CpuType_FLEXLOGIX {
		return "FLEXLOGIX"

	} else if cpuType == plcpb.CpuType_FLGX {
		return "FLGX"
	}

	return ""
}

func (self *server) getPortType(portType plcpb.PortType) string {

	if portType == plcpb.PortType_BACKPLANE {
		return "1"
	} else if portType == plcpb.PortType_NET_ETHERNET {
		return "2"
	} else if portType == plcpb.PortType_DH_PLUS_CHANNEL_A {
		return "2"
	} else if portType == plcpb.PortType_DH_PLUS_CHANNEL_B {
		return "2"
	} else if portType == plcpb.PortType_SERIAL {
		return "3"
	}

	return ""
}

func (self *server) getProtocol(protocolType plcpb.ProtocolType) string {
	if protocolType == plcpb.ProtocolType_AB_EIP {
		return "ab_eip"
	} else if protocolType == plcpb.ProtocolType_AB_CIP {
		return "ab_cip"
	}

	return ""
}

func (self *server) getTypeSize(tagType plcpb.TagType) int32 {
	if tagType == plcpb.TagType_SINT {
		return 1
	}

	if tagType == plcpb.TagType_INT {
		return 2
	}

	if tagType == plcpb.TagType_BOOL || tagType == plcpb.TagType_DINT || tagType == plcpb.TagType_REAL {
		return 4
	}

	if tagType == plcpb.TagType_LINT || tagType == plcpb.TagType_LREAL {
		return 8
	}

	return 0
}

func (self *server) getCpuPath(c connection) string {
	return "protocol=" + self.getProtocol(c.Protocol) + "&gateway=" + c.IP + "&path=" + self.getPortType(c.Port) + "," + Utility.ToString(c.Slot) + "&cpu=" + self.getCpuType(c.Cpu)
}

// Create a new connection and store it for futur use. If the connection already
// exist it will be replace by the new one.
func (self *server) CreateConnection(ctx context.Context, rqst *plcpb.CreateConnectionRqst) (*plcpb.CreateConnectionRsp, error) {

	fmt.Println("Try to create a new connection")
	var c connection

	// Set the connection info from the request.
	c.Id = rqst.Connection.Id
	c.IP = rqst.Connection.Ip
	c.Rack = rqst.Connection.Rack
	c.Port = rqst.Connection.PortType
	c.Slot = rqst.Connection.Slot
	c.Timeout = rqst.Connection.Timeout
	c.Cpu = rqst.Connection.Cpu

	// Here I will put all connections in the Connections array.
	exist := false
	for i := 0; i < len(self.Connections); i++ {
		if self.Connections[i].Id == c.Id {
			self.Connections[i] = c
			exist = true
		}
	}

	if !exist {
		self.Connections = append(self.Connections, c)
	}

	// Print the success message here.
	fmt.Println("Connection " + c.Id + " was created with success!")

	return &plcpb.CreateConnectionRsp{
		Result: true,
	}, nil
}

func (self *server) getConnection(id string) (connection, error) {

	var c connection
	for i := 0; i < len(self.Connections); i++ {
		if self.Connections[i].Id == id {
			c = self.Connections[i]
			return c, nil
		}
	}

	return c, errors.New("No connection found with id: " + id)
}

// Retreive a connection from the map of connection.
func (self *server) GetConnection(ctx context.Context, rqst *plcpb.GetConnectionRqst) (*plcpb.GetConnectionRsp, error) {
	id := rqst.GetId()
	c, err := self.getConnection(id)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("Connection with id "+id+" does not exist!")))
	}

	// return success.
	return &plcpb.GetConnectionRsp{
		Connection: &plcpb.Connection{Id: c.Id, Ip: c.IP, Rack: c.Rack, Slot: c.Slot, Timeout: c.Timeout, PortType: plcpb.PortType(c.Port), Cpu: c.Cpu},
	}, nil
}

// Remove a connection from the map and the file.
func (self *server) DeleteConnection(ctx context.Context, rqst *plcpb.DeleteConnectionRqst) (*plcpb.DeleteConnectionRsp, error) {
	id := rqst.GetId()
	c, err := self.getConnection(id)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("Connection with id "+id+" does not exist!")))
	}

	// close the plc connection if it;s open
	self.Connections = make([]connection, 0)
	for i := 0; i < len(self.Connections); i++ {
		c_ := self.Connections[i]
		if c_.Id != c.Id {
			self.Connections = append(self.Connections, c_)
		}
	}

	// close the connection
	self.closeConnection(id)

	// In that case I will save it in file.
	err = self.Save()
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// return success.
	return &plcpb.DeleteConnectionRsp{
		Result: true,
	}, nil
}

func (self *server) closeConnection(id string) {
	// Here I will close all tags for that connection.
	for tag, _ := range self.tags {
		values := strings.Split(tag, ":")
		if values[0] == id {
			self.closeTag(values[0], values[1])
		}
	}

}

// Close a connection
func (self *server) CloseConnection(ctx context.Context, rqst *plcpb.CloseConnectionRqst) (*plcpb.CloseConnectionRsp, error) {
	id := rqst.ConnectionId
	_, err := self.getConnection(id)

	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("Connection with id "+id+" does not exist!")))
	}

	self.closeConnection(id)

	return &plcpb.CloseConnectionRsp{
		Result: true,
	}, nil
}

// Return the size of a tag.
func (self *server) getTagTypeSize(tagType TagType) int {
	if tagType == BOOL_TAG_TYPE || tagType == SINT_TAG_TYPE {
		return 1
	} else if tagType == INT_TAG_TYPE {
		return 2
	} else if tagType == DINT_TAG_TYPE || tagType == REAL_TAG_TYPE {
		return 4
	} else if tagType == LINT_TAG_TYPE || tagType == LREAL_TAG_TYPE {
		return 8
	}

	return -1 // must not be taken.
}

// Open a tag and return the status.
func (self *server) openTag(connectionId string, name string, tagType plcpb.TagType, elementCount int32) (int32, error) {

	c, err := self.getConnection(connectionId)
	if err != nil {
		return 0, err
	}

	tagPath := self.getCpuPath(c) + "&elem_count=" + Utility.ToString(elementCount) + "&elem_size=" + Utility.ToString(self.getTypeSize(tagType)) + "&name=" + name
	hasChange := true

	// get the path
	if p, ok := self.paths[connectionId+":"+name]; ok {
		// The path exist
		hasChange = tagPath != p
	}

	// If the tag path has change or no path exist.
	if hasChange {
		// Keep the path.
		self.paths[connectionId+":"+name] = tagPath

		// Create a connection for the tagPath.
		tag := plctag.Create(tagPath, int(c.Timeout))
		self.tags[tagPath] = tag

		/* everything OK? */
		if rc := plctag.Status(tag); rc != plctag.STATUS_OK {
			err := errors.New("ERROR %s: Error setting up tag internal state. " + plctag.DecodeError(rc))
			plctag.Destroy(tag)
			delete(self.tags, tagPath)
			log.Println("fail to open tag ", err)
			return -1, err
		}
	}

	return self.tags[tagPath], nil
}

func (self *server) closeTag(connectionId string, name string) {
	if tag, ok := self.tags[connectionId+":"+name]; ok {

		// Destroy the connection.
		plctag.Destroy(tag)

		// The path exist
		delete(self.tags, connectionId+":"+name)
	}
}

// Read Tag
func (self *server) ReadTag(ctx context.Context, rqst *plcpb.ReadTagRqst) (*plcpb.ReadTagRsp, error) {

	id := rqst.GetConnectionId()
	c, err := self.getConnection(id)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("Connection with id "+id+" does not exist!")))
	}

	values := make([]interface{}, 0)

	offset := rqst.Offset
	length := rqst.Length
	size := offset + length
	if rqst.Type == plcpb.TagType_BOOL {
		size = int32(math.Ceil(float64(size) / 32.0))
	}

	tag, err := self.openTag(id, rqst.Name, rqst.Type, size)

	if rc := plctag.Read(tag, int(c.Timeout)); rc != plctag.STATUS_OK {
		err := errors.New("ERROR %s: Error setting up tag internal state. " + plctag.DecodeError(rc))
		plctag.Destroy(tag)
		log.Println(err)
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	time.Sleep(time.Millisecond * time.Duration(250))

	// Keep the tag for further use.
	self.tags[id+":"+rqst.Name] = tag

	// Read the tag.
	for i := offset; i < offset+length; i++ {
		index := int(i * self.getTypeSize(rqst.Type))

		if rqst.Type == plcpb.TagType_BOOL {
			values = append(values, plctag.GetUint8(tag, index))
		} else if rqst.Type == plcpb.TagType_DINT {

			if rqst.Unsigned {
				values = append(values, plctag.GetUint32(tag, index))
			} else {
				values = append(values, plctag.GetInt32(tag, index))
			}

		} else if rqst.Type == plcpb.TagType_INT {
			if rqst.Unsigned {
				values = append(values, plctag.GetUint16(tag, index))
			} else {
				values = append(values, plctag.GetInt16(tag, index))
			}
		} else if rqst.Type == plcpb.TagType_SINT {
			if rqst.Unsigned {
				values = append(values, plctag.GetUint8(tag, index))
			} else {
				values = append(values, plctag.GetInt8(tag, index))
			}
		} else if rqst.Type == plcpb.TagType_LINT {
			if rqst.Unsigned {
				values = append(values, plctag.GetUint64(tag, index))
			} else {
				values = append(values, plctag.GetInt64(tag, index))
			}
		} else if rqst.Type == plcpb.TagType_REAL {
			values = append(values, plctag.GetFloat32(tag, index))
		} else if rqst.Type == plcpb.TagType_LREAL {
			values = append(values, plctag.GetFloat64(tag, index))
		}
	}

	// return the values as string.
	jsonStr, _ := Utility.ToJson(values)
	return &plcpb.ReadTagRsp{
		Values: jsonStr,
	}, nil
}

// Write Tag
func (self *server) WriteTag(ctx context.Context, rqst *plcpb.WriteTagRqst) (*plcpb.WriteTagRsp, error) {

	id := rqst.GetConnectionId()
	c, err := self.getConnection(id)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("Connection with id "+id+" does not exist!")))
	}

	offset := rqst.Offset
	length := rqst.Length
	size := offset + length

	if rqst.Type == plcpb.TagType_BOOL {
		size = int32(math.Ceil(float64(size) / 32.0))
	}

	tag, err := self.openTag(id, rqst.Name, rqst.Type, size)
	if rc := plctag.Write(tag, int(c.Timeout)); rc != plctag.STATUS_OK {
		err := errors.New("ERROR %s: Error setting up tag internal state. " + plctag.DecodeError(rc))
		plctag.Destroy(tag)
		log.Println(err)
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// Get the values to write into the plc.
	values := make([]interface{}, 0)
	json.Unmarshal([]byte(rqst.Values), &values)

	// Read the tag.
	i := offset
	j := 0
	for i < offset+length {
		index := int(i * self.getTypeSize(rqst.Type))
		v := values[j]

		if rqst.Type == plcpb.TagType_BOOL {
			plctag.SetUint8(tag, index, uint8(Utility.ToInt(v)))
		} else if rqst.Type == plcpb.TagType_DINT {

			if rqst.Unsigned {
				plctag.SetUint32(tag, index, uint32(Utility.ToInt(v)))
			} else {
				plctag.SetInt32(tag, index, int32(Utility.ToInt(v)))
			}

		} else if rqst.Type == plcpb.TagType_INT {
			if rqst.Unsigned {
				plctag.SetUint16(tag, index, uint16(Utility.ToInt(v)))
			} else {
				plctag.SetInt16(tag, index, int16(Utility.ToInt(v)))
			}
		} else if rqst.Type == plcpb.TagType_SINT {
			if rqst.Unsigned {
				plctag.SetUint8(tag, index, uint8(Utility.ToInt(v)))
			} else {
				plctag.SetInt8(tag, index, int8(Utility.ToInt(v)))
			}
		} else if rqst.Type == plcpb.TagType_LINT {
			if rqst.Unsigned {
				plctag.SetUint64(tag, index, uint64(Utility.ToInt(v)))
			} else {
				plctag.SetInt64(tag, index, int64(Utility.ToInt(v)))
			}
		} else if rqst.Type == plcpb.TagType_REAL {
			plctag.SetFloat32(tag, index, float32(Utility.ToNumeric(v)))
		} else if rqst.Type == plcpb.TagType_LREAL {
			plctag.SetFloat64(tag, index, Utility.ToNumeric(v))
		}

		j++
		i++
	}

	return &plcpb.WriteTagRsp{
		Result: true,
	}, nil
}

// That service is use to give access to SQL.
// port number must be pass as argument.
func main() {

	// The actual server implementation.
	s_impl := new(server)
	s_impl.Name = string(plcpb.File_proto_plc_proto.Services().Get(0).FullName())
	s_impl.Proto = plcpb.File_proto_plc_proto.Path()
	s_impl.Port = defaultPort
	s_impl.Proxy = defaultProxy
	s_impl.Protocol = "grpc"
	s_impl.Domain = domain
	s_impl.PublisherId = "localhost"
	s_impl.Version = "0.0.1"
	s_impl.Connections = make([]connection, 0)
	s_impl.Permissions = make([]interface{}, 0)
	s_impl.Keywords = make([]string, 0)
	s_impl.Repositories = make([]string, 0)
	s_impl.Discoveries = make([]string, 0)

	// The list of paths.
	s_impl.tags = make(map[string]int32, 0)
	s_impl.paths = make(map[string]string, 0)

	// TODO set it from the program arguments...
	s_impl.AllowAllOrigins = allow_all_origins
	s_impl.AllowedOrigins = allowed_origins
	err := s_impl.Init()
	if err != nil {
		log.Fatalf("Fail to initialyse service %s: %s", s_impl.Name, s_impl.Id, err)
	}

	if len(os.Args) == 2 {
		s_impl.Port, _ = strconv.Atoi(os.Args[1]) // The second argument must be the port number
	}

	// Here I will retreive the list of connections from file if there are some...
	err = s_impl.Init()
	if err != nil {
		fmt.Println("Fail to initialyse service %s: %s", s_impl.Name, s_impl.Id, err)
	}

	// Register the echo services
	plcpb.RegisterPlcServiceServer(s_impl.grpcServer, s_impl)
	reflection.Register(s_impl.grpcServer)

	// Start the service.
	s_impl.StartService()

}
