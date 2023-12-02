package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"

	//"strings"
	"time"

	//	"net/http"
	"reflect"
	"runtime"

	"github.com/davecourtois/Utility"
	"github.com/globulario/services/golang/config"
	globular "github.com/globulario/services/golang/globular_service"
	"github.com/globulario/services/golang/interceptors"
	"github.com/globulario/services/golang/sql/sqlpb"

	iconv "github.com/djimenez/iconv-go"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"

	//"google.golang.org/grpc/grpclog"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"

	// The list of available drivers...
	// feel free to append the driver you need.
	// dont forgot the set correction string if you do so.

	_ "github.com/alexbrainman/odbc"
	_ "github.com/denisenkom/go-mssqldb"
	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
	_ "github.com/mattn/go-sqlite3"
)

var (
	defaultPort  = 10039
	defaultProxy = 10040

	// By default all origins are allowed.
	allow_all_origins = true

	// comma separeated values.
	allowed_origins string = ""
)

// Keep connection information here.
type connection struct {
	Id       string // The connection id
	Name     string // The database name
	Host     string // can also be ipv4 addresse.
	Charset  string
	Driver   string // The name of the driver.
	User     string
	Password string
	Port     int32
	Path     string // The path of the sqlite3 database.
}

func (c *connection) getConnectionString() string {
	var connectionString string

	if c.Driver == "mssql" {
		/** Connect to Microsoft Sql server here... **/
		// So I will create the connection string from info...
		connectionString += "server=" + c.Host + ";"
		connectionString += "user=" + c.User + ";"
		connectionString += "password=" + c.Password + ";"
		connectionString += "port=" + strconv.Itoa(int(c.Port)) + ";"
		connectionString += "database=" + c.Name + ";"
		connectionString += "driver=mssql"
		connectionString += "charset=" + c.Charset + ";"
	} else if c.Driver == "mysql" {
		/** Connect to oracle MySql server here... **/
		connectionString += c.User + ":"
		connectionString += c.Password + "@tcp("
		connectionString += c.Host + ":" + strconv.Itoa(int(c.Port)) + ")"
		connectionString += "/" + c.Name
		//connectionString += "encrypt=false;"
		connectionString += "?"
		connectionString += "charset=" + c.Charset + ";"

	} else if c.Driver == "postgres" {
		connectionString += c.User + ":"
		connectionString += c.Password + "@tcp("
		connectionString += c.Host + ":" + strconv.Itoa(int(c.Port)) + ")"
		connectionString += "/" + c.Name
		//connectionString += "encrypt=false;"
		connectionString += "?"
		connectionString += "charset=" + c.Charset + ";"
	} else if c.Driver == "odbc" {
		/** Connect with ODBC here... **/
		if runtime.GOOS == "windows" {
			connectionString += "driver=sql server;"
		} else {
			connectionString += "driver=freetds;"
		}
		connectionString += "server=" + c.Host + ";"
		connectionString += "database=" + c.Name + ";"
		connectionString += "uid=" + c.User + ";"
		connectionString += "pwd=" + c.Password + ";"
		connectionString += "port=" + strconv.Itoa(int(c.Port)) + ";"
		connectionString += "charset=" + c.Charset + ";"

	} else if c.Driver == "sqlite3" {
		connectionString += c.Path + string(os.PathSeparator) + c.Name // The directory...
	}

	return connectionString
}

type server struct {

	// The global attribute of the services.
	Id                 string
	Name               string
	Mac                string
	Proto              string
	Port               int
	Path               string
	Proxy              int
	Protocol           string
	AllowAllOrigins    bool
	AllowedOrigins     string // comma separated string.
	Domain             string
	Address            string
	Description        string
	Keywords           []string
	Repositories       []string
	Discoveries        []string
	CertAuthorityTrust string
	CertFile           string
	KeyFile            string
	TLS                bool
	Version            string
	PublisherId        string
	KeepUpToDate       bool
	Plaform            string
	Checksum           string
	KeepAlive          bool
	Permissions        []interface{} // contains the action permission for the services.
	Dependencies       []string      // The list of services needed by this services.
	Process            int
	ProxyProcess       int
	LastError          string
	ModTime            int64
	State              string
	ConfigPath         string
	// The grpc server.
	grpcServer *grpc.Server

	// The map of connection...
	Connections map[string]connection
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

// The description of the service
func (srv *server) GetDescription() string {
	return srv.Description
}
func (srv *server) SetDescription(description string) {
	srv.Description = description
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

// Create the configuration file if is not already exist.
func (srv *server) Init() error {

	err := globular.InitService(srv)
	if err != nil {
		return err
	}

	// Initialyse GRPC srv.
	srv.grpcServer, err = globular.InitGrpcServer(srv, interceptors.ServerUnaryInterceptor, interceptors.ServerStreamInterceptor)
	if err != nil {
		return err
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

func (srv *server) Stop(context.Context, *sqlpb.StopRequest) (*sqlpb.StopResponse, error) {
	return &sqlpb.StopResponse{}, srv.StopService()
}

//////////////////////// SQL Specific services /////////////////////////////////

// Create a new SQL connection and store it for futur use. If the connection already
// exist it will be replace by the new one.
func (srv *server) CreateConnection(ctx context.Context, rqst *sqlpb.CreateConnectionRqst) (*sqlpb.CreateConnectionRsp, error) {

	// sqlpb
	var c connection

	// Set the connection info from the request.
	c.Id = rqst.Connection.Id
	c.Name = rqst.Connection.Name
	c.Host = rqst.Connection.Host
	c.Port = rqst.Connection.Port
	c.User = rqst.Connection.User
	c.Password = rqst.Connection.Password
	c.Driver = rqst.Connection.Driver
	c.Charset = rqst.Connection.Charset
	c.Path = rqst.Connection.Path

	if c.Driver == "sqlite3" && len(c.Path) == 0 {
		return nil, status.Errorf(
			codes.InvalidArgument,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("path is empty for sqlite3 connection.")))
	}

	db, err := sql.Open(c.Driver, c.getConnectionString())

	if err != nil {
		fmt.Println("fail to create connection with error ", err)
		// codes.
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// close the connection when done.
	defer db.Close()

	// set or update the connection and save it in json file.
	srv.Connections[c.Id] = c

	// In that case I will save it in file.
	err = srv.Save()
	if err != nil {
		fmt.Println("fail to save connection ", err)
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// test if the connection is reacheable.
	_, err = srv.ping(ctx, c.Id)

	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))

	}

	return &sqlpb.CreateConnectionRsp{
		Result: true,
	}, nil
}

// Remove a connection from the map and the file.
func (srv *server) DeleteConnection(ctx context.Context, rqst *sqlpb.DeleteConnectionRqst) (*sqlpb.DeleteConnectionRsp, error) {
	id := rqst.GetId()
	if _, ok := srv.Connections[id]; !ok {
		return &sqlpb.DeleteConnectionRsp{
			Result: true,
		}, nil
	}

	delete(srv.Connections, id)

	// In that case I will save it in file.
	err := srv.Save()
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// return success.
	return &sqlpb.DeleteConnectionRsp{
		Result: true,
	}, nil
}

// local implementation.
func (srv *server) ping(ctx context.Context, id string) (string, error) {
	if _, ok := srv.Connections[id]; !ok {
		return "", errors.New("connection with id " + id + " dosent exist.")
	}

	c := srv.Connections[id]

	// First of all I will try to
	db, err := sql.Open(c.Driver, c.getConnectionString())
	if err != nil {
		return "", status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	defer db.Close()

	ctx, cancel := context.WithTimeout(ctx, 1*time.Second)
	defer cancel()

	// If there is no answer from the database after one second
	if err := db.PingContext(ctx); err != nil {
		return "", err
	}

	return "pong", nil
}

// Ping a sql connection.
func (srv *server) Ping(ctx context.Context, rqst *sqlpb.PingConnectionRqst) (*sqlpb.PingConnectionRsp, error) {
	pong, err := srv.ping(ctx, rqst.GetId())

	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &sqlpb.PingConnectionRsp{
		Result: pong,
	}, nil
}

// The maximum results size before send it over the network.
// if the number is to big network fragmentation will slow down the transfer
// if is to low the serialisation cost will be very hight...
var maxSize = uint(16000) // Value in bytes...

// Now the execute query.
func (srv *server) QueryContext(rqst *sqlpb.QueryContextRqst, stream sqlpb.SqlService_QueryContextServer) error {
	// Get the connection charset...
	var charset string
	// Be sure the connection is there.
	if conn, ok := srv.Connections[rqst.Query.ConnectionId]; !ok {
		return errors.New("connection with id " + rqst.Query.ConnectionId + " dosent exist.")
	} else {
		charset = conn.Charset
	}

	// Now I will open the connection.
	c := srv.Connections[rqst.Query.ConnectionId]

	// First of all I will try to
	db, err := sql.Open(c.Driver, c.getConnectionString())

	if err != nil {
		return status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	defer db.Close()

	// The query
	query := rqst.Query.Query

	// The list of parameters
	parameters := make([]interface{}, 0)
	if len(rqst.Query.Parameters) > 0 {
		err = json.Unmarshal([]byte(rqst.Query.Parameters), &parameters)
		if err != nil {
			return status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))

		}
	}

	// Here I the sql works.
	rows, err := db.QueryContext(stream.Context(), query, parameters...)

	if err != nil {
		return status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	defer rows.Close()

	// First of all I will get the information about columns
	columns, err := rows.Columns()
	if err != nil {
		return status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// The columns type.
	columnsType, err := rows.ColumnTypes()
	if err != nil {
		return status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// In header is not guaranty to contain a column type.
	header := make([]interface{}, len(columns))

	for i := 0; i < len(columnsType); i++ {
		column := columns[i]

		// So here I will extract type information.
		typeInfo := make(map[string]interface{})
		typeInfo["DatabaseTypeName"] = columnsType[i].DatabaseTypeName()
		typeInfo["Name"] = columnsType[i].DatabaseTypeName()

		// If the type is decimal.
		precision, scale, isDecimal := columnsType[i].DecimalSize()
		if isDecimal {
			typeInfo["Scale"] = scale
			typeInfo["Precision"] = precision
		}

		length, hasLength := columnsType[i].Length()
		if hasLength {
			typeInfo["Precision"] = length
		}

		isNull, isNullable := columnsType[i].Nullable()
		typeInfo["IsNullable"] = isNullable
		if isNullable {
			typeInfo["IsNull"] = isNull
		}

		header[i] = map[string]interface{}{"name": column, "typeInfo": typeInfo}
	}

	// serialyse the header in json and send it as first message.
	headerStr, _ := Utility.ToJson(header)

	// So the first message I will send will alway be the header...
	stream.Send(&sqlpb.QueryContextRsp{
		Result: &sqlpb.QueryContextRsp_Header{
			Header: headerStr,
		},
	})

	count := len(columns)
	values := make([]interface{}, count)
	scanArgs := make([]interface{}, count)
	for i := range values {
		scanArgs[i] = &values[i]
	}

	rows_ := make([]interface{}, 0)

	for rows.Next() {
		row := make([]interface{}, count)
		err := rows.Scan(scanArgs...)
		if err != nil {
			return err
		}

		for i, v := range values {
			// So here I will convert the values to Number, Boolean or String
			if v == nil {
				row[i] = nil // NULL value.
			} else {
				if Utility.IsNumeric(v) {
					row[i] = Utility.ToNumeric(v)
				} else if Utility.IsBool(v) {
					row[i] = Utility.ToBool(v)
				} else {
					str := Utility.ToString(v)
					// here I will simply return the sting value.
					if len(charset) > 0 && len(rqst.Query.Charset) > 0 {
						if charset != rqst.Query.Charset {
							// It wil convert for exemple from windows-1252 to utf-8...
							str_, err := iconv.ConvertString(str, charset, rqst.Query.Charset)
							if err == nil {
								str = str_
							}
						}
					}
					row[i] = str
				}
			}
		}

		rows_ = append(rows_, row)
		size := uint(uintptr(len(rows_)) * reflect.TypeOf(rows_).Elem().Size())

		if size > maxSize {
			rowStr, _ := Utility.ToJson(rows_)
			stream.Send(&sqlpb.QueryContextRsp{
				Result: &sqlpb.QueryContextRsp_Rows{
					Rows: string(rowStr),
				},
			})
			rows_ = make([]interface{}, 0)
		}
	}

	if len(rows_) > 0 {
		rowStr, _ := Utility.ToJson(rows_)
		stream.Send(&sqlpb.QueryContextRsp{
			Result: &sqlpb.QueryContextRsp_Rows{
				Rows: string(rowStr),
			},
		})
	}

	return nil
}

// Exec Query SQL CREATE and INSERT. Return the affected rows.
// Now the execute query.
func (srv *server) ExecContext(ctx context.Context, rqst *sqlpb.ExecContextRqst) (*sqlpb.ExecContextRsp, error) {

	// Be sure the connection is there.
	if _, ok := srv.Connections[rqst.Query.ConnectionId]; !ok {
		return nil, errors.New("connection with id " + rqst.Query.ConnectionId + " dosent exist.")
	}

	// Now I will open the connection.
	c := srv.Connections[rqst.Query.ConnectionId]

	// First of all I will try to
	db, err := sql.Open(c.Driver, c.getConnectionString())
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	defer db.Close()

	// The query
	query := rqst.Query.Query

	// The list of parameters
	parameters := make([]interface{}, 0)
	json.Unmarshal([]byte(rqst.Query.Parameters), &parameters)

	// Execute the query here.
	var lastId, affectedRows int64
	var result sql.Result

	if rqst.Tx {
		// with transaction
		tx, err := db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
		if err != nil {
			return nil, status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}

		var execErr error
		result, execErr = tx.ExecContext(ctx, query, parameters...)
		if execErr != nil {
			if rollbackErr := tx.Rollback(); rollbackErr != nil {
				err = errors.New(fmt.Sprint("update failed: %v, unable to rollback: %v\n", execErr, rollbackErr))
				return nil, status.Errorf(
					codes.Internal,
					Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
			}

			err = errors.New(fmt.Sprint("update failed: %v", execErr))
			return nil, status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
		if err := tx.Commit(); err != nil {
			return nil, status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
	} else {
		// without transaction
		result, err = db.ExecContext(ctx, query, parameters...)
	}

	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// So here I will stream affected row if there one.
	affectedRows, err = result.RowsAffected()
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// I will send back the last id and the number of affected rows to the caller.
	lastId, _ = result.LastInsertId()

	return &sqlpb.ExecContextRsp{
		LastId:       lastId,
		AffectedRows: affectedRows,
	}, nil
}

// That service is use to give access to SQL.
// port number must be pass as argument.
func main() {

	// set the logger.
	//grpclog.SetLogger(log.New(os.Stdout, "sql_service: ", log.LstdFlags))

	// Set the log information in case of crash...
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	// The actual server implementation.
	s_impl := new(server)
	s_impl.Connections = make(map[string]connection)
	s_impl.Name = string(sqlpb.File_sql_proto.Services().Get(0).FullName())
	s_impl.Proto = sqlpb.File_sql_proto.Path()
	s_impl.Port = defaultPort
	s_impl.Proxy = defaultProxy
	s_impl.Path, _ = filepath.Abs(filepath.Dir(os.Args[0]))
	s_impl.Protocol = "grpc"
	s_impl.Domain, _ = config.GetDomain()
	s_impl.Address, _ = config.GetAddress()
	s_impl.Version = "0.0.1"
	// TODO set it from the program arguments...
	s_impl.AllowAllOrigins = allow_all_origins
	s_impl.AllowedOrigins = allowed_origins
	s_impl.PublisherId = "localhost"
	s_impl.Permissions = make([]interface{}, 0)
	s_impl.Keywords = make([]string, 0)
	s_impl.Repositories = make([]string, 0)
	s_impl.Discoveries = make([]string, 0)
	s_impl.Dependencies = make([]string, 0)
	s_impl.Process = -1
	s_impl.ProxyProcess = -1
	s_impl.KeepAlive = true
	s_impl.KeepUpToDate = true

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
	sqlpb.RegisterSqlServiceServer(s_impl.grpcServer, s_impl)
	reflection.Register(s_impl.grpcServer)

	// Start the service.
	s_impl.StartService()

}
