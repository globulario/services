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
func (svr *server) GetConfigurationPath() string {
	return svr.ConfigPath
}

func (svr *server) SetConfigurationPath(path string) {
	svr.ConfigPath = path
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
func (sql_server *server) GetId() string {
	return sql_server.Id
}
func (sql_server *server) SetId(id string) {
	sql_server.Id = id
}

// The name of a service, must be the gRpc Service name.
func (sql_server *server) GetName() string {
	return sql_server.Name
}
func (sql_server *server) SetName(name string) {
	sql_server.Name = name
}

func (svr *server) GetMac() string {
	return svr.Mac
}

func (svr *server) SetMac(mac string) {
	svr.Mac = mac
}

// The description of the service
func (sql_server *server) GetDescription() string {
	return sql_server.Description
}
func (sql_server *server) SetDescription(description string) {
	sql_server.Description = description
}

func (sql_server *server) GetRepositories() []string {
	return sql_server.Repositories
}
func (sql_server *server) SetRepositories(repositories []string) {
	sql_server.Repositories = repositories
}

func (sql_server *server) GetDiscoveries() []string {
	return sql_server.Discoveries
}
func (sql_server *server) SetDiscoveries(discoveries []string) {
	sql_server.Discoveries = discoveries
}

// The list of keywords of the services.
func (sql_server *server) GetKeywords() []string {
	return sql_server.Keywords
}
func (sql_server *server) SetKeywords(keywords []string) {
	sql_server.Keywords = keywords
}

// Dist
func (sql_server *server) Dist(path string) (string, error) {

	return globular.Dist(path, sql_server)
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

func (svr *server) GetChecksum() string {

	return svr.Checksum
}

func (svr *server) SetChecksum(checksum string) {
	svr.Checksum = checksum
}

func (svr *server) GetPlatform() string {
	return svr.Plaform
}

func (svr *server) SetPlatform(platform string) {
	svr.Plaform = platform
}

// The path of the executable.
func (sql_server *server) GetPath() string {
	return sql_server.Path
}
func (sql_server *server) SetPath(path string) {
	sql_server.Path = path
}

// The path of the .proto file.
func (sql_server *server) GetProto() string {
	return sql_server.Proto
}
func (sql_server *server) SetProto(proto string) {
	sql_server.Proto = proto
}

// The gRpc port.
func (sql_server *server) GetPort() int {
	return sql_server.Port
}
func (sql_server *server) SetPort(port int) {
	sql_server.Port = port
}

// The reverse proxy port (use by gRpc Web)
func (sql_server *server) GetProxy() int {
	return sql_server.Proxy
}
func (sql_server *server) SetProxy(proxy int) {
	sql_server.Proxy = proxy
}

// Can be one of http/https/tls
func (sql_server *server) GetProtocol() string {
	return sql_server.Protocol
}
func (sql_server *server) SetProtocol(protocol string) {
	sql_server.Protocol = protocol
}

// Return true if all Origins are allowed to access the mircoservice.
func (sql_server *server) GetAllowAllOrigins() bool {
	return sql_server.AllowAllOrigins
}
func (sql_server *server) SetAllowAllOrigins(allowAllOrigins bool) {
	sql_server.AllowAllOrigins = allowAllOrigins
}

// If AllowAllOrigins is false then AllowedOrigins will contain the
// list of address that can reach the services.
func (sql_server *server) GetAllowedOrigins() string {
	return sql_server.AllowedOrigins
}

func (sql_server *server) SetAllowedOrigins(allowedOrigins string) {
	sql_server.AllowedOrigins = allowedOrigins
}

// Can be a ip address or domain name.
func (sql_server *server) GetDomain() string {
	return sql_server.Domain
}
func (sql_server *server) SetDomain(domain string) {
	sql_server.Domain = domain
}

// TLS section

// If true the service run with TLS. The
func (sql_server *server) GetTls() bool {
	return sql_server.TLS
}
func (sql_server *server) SetTls(hasTls bool) {
	sql_server.TLS = hasTls
}

// The certificate authority file
func (sql_server *server) GetCertAuthorityTrust() string {
	return sql_server.CertAuthorityTrust
}
func (sql_server *server) SetCertAuthorityTrust(ca string) {
	sql_server.CertAuthorityTrust = ca
}

// The certificate file.
func (sql_server *server) GetCertFile() string {
	return sql_server.CertFile
}
func (sql_server *server) SetCertFile(certFile string) {
	sql_server.CertFile = certFile
}

// The key file.
func (sql_server *server) GetKeyFile() string {
	return sql_server.KeyFile
}
func (sql_server *server) SetKeyFile(keyFile string) {
	sql_server.KeyFile = keyFile
}

// The service version
func (sql_server *server) GetVersion() string {
	return sql_server.Version
}
func (sql_server *server) SetVersion(version string) {
	sql_server.Version = version
}

// The publisher id.
func (sql_server *server) GetPublisherId() string {
	return sql_server.PublisherId
}
func (sql_server *server) SetPublisherId(publisherId string) {
	sql_server.PublisherId = publisherId
}

func (sql_server *server) GetKeepUpToDate() bool {
	return sql_server.KeepUpToDate
}
func (sql_server *server) SetKeepUptoDate(val bool) {
	sql_server.KeepUpToDate = val
}

func (sql_server *server) GetKeepAlive() bool {
	return sql_server.KeepAlive
}
func (sql_server *server) SetKeepAlive(val bool) {
	sql_server.KeepAlive = val
}

func (sql_server *server) GetPermissions() []interface{} {
	return sql_server.Permissions
}
func (sql_server *server) SetPermissions(permissions []interface{}) {
	sql_server.Permissions = permissions
}

// Create the configuration file if is not already exist.
func (sql_server *server) Init() error {

	err := globular.InitService(sql_server)
	if err != nil {
		return err
	}

	// Initialyse GRPC server.
	sql_server.grpcServer, err = globular.InitGrpcServer(sql_server, interceptors.ServerUnaryInterceptor, interceptors.ServerStreamInterceptor)
	if err != nil {
		return err
	}

	return nil

}

// Save the configuration values.
func (sql_server *server) Save() error {
	// Create the file...
	return globular.SaveService(sql_server)
}

func (sql_server *server) StartService() error {
	return globular.StartService(sql_server, sql_server.grpcServer)
}

func (sql_server *server) StopService() error {
	return globular.StopService(sql_server, sql_server.grpcServer)
}

func (sql_server *server) Stop(context.Context, *sqlpb.StopRequest) (*sqlpb.StopResponse, error) {
	return &sqlpb.StopResponse{}, sql_server.StopService()
}

//////////////////////// SQL Specific services /////////////////////////////////

// Create a new SQL connection and store it for futur use. If the connection already
// exist it will be replace by the new one.
func (sql_server *server) CreateConnection(ctx context.Context, rqst *sqlpb.CreateConnectionRqst) (*sqlpb.CreateConnectionRsp, error) {

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
	sql_server.Connections[c.Id] = c

	// In that case I will save it in file.
	err = sql_server.Save()
	if err != nil {
		fmt.Println("fail to save connection ", err)
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// test if the connection is reacheable.
	_, err = sql_server.ping(ctx, c.Id)

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
func (sql_server *server) DeleteConnection(ctx context.Context, rqst *sqlpb.DeleteConnectionRqst) (*sqlpb.DeleteConnectionRsp, error) {
	id := rqst.GetId()
	if _, ok := sql_server.Connections[id]; !ok {
		return &sqlpb.DeleteConnectionRsp{
			Result: true,
		}, nil
	}

	delete(sql_server.Connections, id)

	// In that case I will save it in file.
	err := sql_server.Save()
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
func (sql_server *server) ping(ctx context.Context, id string) (string, error) {
	if _, ok := sql_server.Connections[id]; !ok {
		return "", errors.New("connection with id " + id + " dosent exist.")
	}

	c := sql_server.Connections[id]

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
func (sql_server *server) Ping(ctx context.Context, rqst *sqlpb.PingConnectionRqst) (*sqlpb.PingConnectionRsp, error) {
	pong, err := sql_server.ping(ctx, rqst.GetId())

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
func (sql_server *server) QueryContext(rqst *sqlpb.QueryContextRqst, stream sqlpb.SqlService_QueryContextServer) error {
	// Get the connection charset...
	var charset string
	// Be sure the connection is there.
	if conn, ok := sql_server.Connections[rqst.Query.ConnectionId]; !ok {
		return errors.New("connection with id " + rqst.Query.ConnectionId + " dosent exist.")
	} else {
		charset = conn.Charset
	}

	// Now I will open the connection.
	c := sql_server.Connections[rqst.Query.ConnectionId]

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
func (sql_server *server) ExecContext(ctx context.Context, rqst *sqlpb.ExecContextRqst) (*sqlpb.ExecContextRsp, error) {

	// Be sure the connection is there.
	if _, ok := sql_server.Connections[rqst.Query.ConnectionId]; !ok {
		return nil, errors.New("connection with id " + rqst.Query.ConnectionId + " dosent exist.")
	}

	// Now I will open the connection.
	c := sql_server.Connections[rqst.Query.ConnectionId]

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
	s_impl.PublisherId = "globulario@globule-dell.globular.cloud"
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

	// Register the echo services
	sqlpb.RegisterSqlServiceServer(s_impl.grpcServer, s_impl)
	reflection.Register(s_impl.grpcServer)

	// Start the service.
	s_impl.StartService()

}
