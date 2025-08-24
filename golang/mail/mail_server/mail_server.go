package main

import (
	"context"
	"crypto/tls"

	//"crypto/tls"
	"errors"
	"fmt"
	"path/filepath"

	//"fmt"
	"io"
	"log"
	"os"
	"strings"

	Utility "github.com/davecourtois/!utility"
	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/globular_client"
	globular "github.com/globulario/services/golang/globular_service"
	"github.com/globulario/services/golang/interceptors"
	"github.com/globulario/services/golang/persistence/persistence_client"

	"github.com/globulario/services/golang/mail/mail_client"
	"github.com/globulario/services/golang/mail/mailpb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"

	//"google.golang.org/grpc/grpclog"

	smtp_ "net/smtp"

	"github.com/globulario/services/golang/mail/mail_server/imap"
	"github.com/globulario/services/golang/mail/mail_server/smtp"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"

	gomail "gopkg.in/gomail.v2"
)

var (
	defaultPort  = 10067
	defaultProxy = 10068

	// By default all origins are allowed.
	allow_all_origins = true

	// comma separeated values.
	allowed_origins string = ""
)

// Keep connection information here.
type connection struct {
	Id       string // The connection id
	Host     string // can also be ipv4 addresse.
	User     string
	Password string
	Port     int32
}

type server struct {
	// The global attribute of the services.
	Id                 string
	Name               string
	Mac                string
	Path               string
	Proto              string
	Port               int
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
	CertFile           string
	KeyFile            string
	CertAuthorityTrust string
	Version            string
	TLS                bool
	PublisherId        string
	KeepUpToDate       bool
	Plaform            string
	Checksum           string
	KeepAlive          bool
	Permissions        []interface{} // contains the action permission for the services.
	Dependencies       []string      // The list of services needed by this services.
	Process            int
	ProxyProcess       int
	ConfigPath         string
	LastError          string
	ModTime            int64
	State              string

	// The grpc server.
	grpcServer *grpc.Server

	// The map of connection...
	Connections map[string]connection

	// The persistence service...
	Persistence_address string

	// The smtp server port
	SMTP_Port     int
	SMTPS_Port    int // ssl port
	SMTP_ALT_Port int // alternate port

	// The imap server port
	IMAP_Port     int
	IMAPS_Port    int // ssl port
	IMAP_ALT_Port int // alternate port

	// The backend admin password necessary to validate email address and
	// store incomming message.
	Password string
	DbIpV4   string // The address of the databe ex 0.0.0.0:27017

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

// The description of the service
func (srv *server) GetDescription() string {
	return srv.Description
}
func (srv *server) SetDescription(description string) {
	srv.Description = description
}

func (srv *server) GetMac() string {
	return srv.Mac
}

func (srv *server) SetMac(mac string) {
	srv.Mac = mac
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

func (srv *server) Stop(context.Context, *mailpb.StopRequest) (*mailpb.StopResponse, error) {
	return &mailpb.StopResponse{}, srv.StopService()
}

//////////////////////////// SMPT specific functions ///////////////////////////

// Create a new connection and store it for futur use. If the connection already
// exist it will be replace by the new one.
func (srv *server) CreateConnection(ctx context.Context, rsqt *mailpb.CreateConnectionRqst) (*mailpb.CreateConnectionRsp, error) {

	var c connection
	var err error

	// Set the connection info from the request.
	c.Id = rsqt.Connection.Id
	c.Host = rsqt.Connection.Host
	c.Port = rsqt.Connection.Port
	c.User = rsqt.Connection.User
	c.Password = rsqt.Connection.Password

	// set or update the connection and save it in json file.
	srv.Connections[c.Id] = c

	// In that case I will save it in file.
	err = srv.Save()
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// test if the connection is reacheable.
	// _, err = srv.ping(ctx, c.Id)

	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &mailpb.CreateConnectionRsp{
		Result: true,
	}, nil
}

// Remove a connection from the map and the file.
func (srv *server) DeleteConnection(ctx context.Context, rqst *mailpb.DeleteConnectionRqst) (*mailpb.DeleteConnectionRsp, error) {
	id := rqst.GetId()
	if _, ok := srv.Connections[id]; !ok {
		return &mailpb.DeleteConnectionRsp{
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
	return &mailpb.DeleteConnectionRsp{
		Result: true,
	}, nil
}

////////////////////////////////////////////////////////////////////////////////
// SMTP functions.
////////////////////////////////////////////////////////////////////////////////

/**
 * Carbon copy list...
 */
type CarbonCopy struct {
	EMail string
	Name  string
}

/**
 * Attachment file, if the data is empty or nil
 * that means the file is on the server a the given path.
 */
type Attachment struct {
	FileName string
	FileData []byte
}

/**
 * Send mail... The server id is the authentication id...
 */
func (srv *server) sendEmail(host string, user string, pwd string, port int, from string, to []string, cc []*CarbonCopy, subject string, body string, attachs []*Attachment, bodyType string) error {

	fmt.Println("mail_server sendEmail ", host, user, pwd, port)

	msg := gomail.NewMessage()
	msg.SetHeader("From", from)
	msg.SetHeader("To", to...)

	// Attach the multiple carbon copy...
	var cc_ []string
	for i := 0; i < len(cc); i++ {
		cc_ = append(cc_, msg.FormatAddress(cc[i].EMail, cc[i].Name))
	}

	if len(cc_) > 0 {
		msg.SetHeader("Cc", cc_...)
	}

	msg.SetHeader("Subject", subject)
	msg.SetBody(bodyType, body)
	for i := 0; i < len(attachs); i++ {
		msg.Attach(attachs[i].FileName, gomail.SetCopyFunc(func(w io.Writer) error {
			_, err := w.Write(attachs[i].FileData)
			return err
		}))
	}

	dialer := gomail.NewDialer(host, port, user, pwd)
	dialer.Auth = smtp_.PlainAuth("", dialer.Username, dialer.Password, dialer.Host)

	if port != 25 {
		cer, err := tls.LoadX509KeyPair(srv.CertFile, srv.KeyFile)
		if err != nil {
			return err
		}

		dialer.TLSConfig = &tls.Config{ServerName: host, Certificates: []tls.Certificate{cer}}
	}

	if err := dialer.DialAndSend(msg); err != nil {
		return err
	}

	return nil
}

// Send a simple email whitout file.
func (srv *server) SendEmail(ctx context.Context, rqst *mailpb.SendEmailRqst) (*mailpb.SendEmailRsp, error) {

	if _, ok := srv.Connections[rqst.Id]; !ok {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("No connection found with id "+rqst.Id)))
	}
	if rqst.Email == nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("No email message was given!")))
	}

	cc := make([]*CarbonCopy, len(rqst.Email.Cc))
	for i := 0; i < len(rqst.Email.Cc); i++ {
		cc[i] = &CarbonCopy{Name: rqst.Email.Cc[i].Name, EMail: rqst.Email.Cc[i].Address}
	}

	bodyType := "text/html"
	if rqst.Email.BodyType != mailpb.BodyType_HTML {
		bodyType = "text/html"
	}

	err := srv.sendEmail(srv.Connections[rqst.Id].Host, srv.Connections[rqst.Id].User, srv.Connections[rqst.Id].Password, int(srv.Connections[rqst.Id].Port), rqst.Email.From, rqst.Email.To, cc, rqst.Email.Subject, rqst.Email.Body, []*Attachment{}, bodyType)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &mailpb.SendEmailRsp{
		Result: true,
	}, nil
}

// Send email with file attachement attachements.
func (srv *server) SendEmailWithAttachements(stream mailpb.MailService_SendEmailWithAttachementsServer) error {

	// that buffer will contain the file attachement data while data is transfert.
	attachements := make([]*Attachment, 0)
	var bodyType string
	var body string
	var subject string
	var from string
	var to []string
	var cc []*CarbonCopy
	var id string

	// So here I will read the stream until it end...
	for {
		rqst, err := stream.Recv()
		if _, ok := srv.Connections[rqst.Id]; !ok {
			return status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("No connection found with id "+rqst.Id)))
		}
		if err == io.EOF {

			// Here all data is read...
			c := srv.Connections[id]
			err := srv.sendEmail(c.Host, c.User, c.Password, int(c.Port), from, to, cc, subject, body, attachements, bodyType)

			if err != nil {
				return status.Errorf(
					codes.Internal,
					Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
			}

			// Close the stream...
			err_ := stream.SendAndClose(&mailpb.SendEmailWithAttachementsRsp{
				Result: true,
			})

			if err_ != nil {
				fmt.Println("fail send response and close stream with error ", err_)
				return err_
			}
		}

		if err != nil {
			return status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}

		id = rqst.Id

		// Receive message informations.
		switch msg := rqst.Data.(type) {
		case *mailpb.SendEmailWithAttachementsRqst_Email:
			cc = make([]*CarbonCopy, len(msg.Email.Cc))

			// The email itsrv.
			for i := 0; i < len(msg.Email.Cc); i++ {
				cc[i] = &CarbonCopy{Name: msg.Email.Cc[i].Name, EMail: msg.Email.Cc[i].Address}
			}
			bodyType = "text"
			if msg.Email.BodyType == mailpb.BodyType_HTML {
				bodyType = "html"
			}
			from = msg.Email.From
			to = msg.Email.To
			body = msg.Email.Body
			subject = msg.Email.Subject

		case *mailpb.SendEmailWithAttachementsRqst_Attachements:
			var lastAttachement *Attachment
			if len(attachements) > 0 {
				lastAttachement = attachements[len(attachements)-1]
				if lastAttachement.FileName != msg.Attachements.FileName {
					lastAttachement = new(Attachment)
					lastAttachement.FileData = make([]byte, 0)
					lastAttachement.FileName = msg.Attachements.FileName
					attachements = append(attachements, lastAttachement)
				}
			} else {
				lastAttachement = new(Attachment)
				lastAttachement.FileData = make([]byte, 0)
				lastAttachement.FileName = msg.Attachements.FileName
				attachements = append(attachements, lastAttachement)
			}

			// Append the data in the file attachement.
			lastAttachement.FileData = append(lastAttachement.FileData, msg.Attachements.FileData...)
		}

	}
}

func GetPersistenceClient(address string) (*persistence_client.Persistence_Client, error) {
	Utility.RegisterFunction("NewPersistenceService_Client", persistence_client.NewPersistenceService_Client)
	client, err := globular_client.GetClient(address, "persistence.PersistenceService", "NewPersistenceService_Client")
	if err != nil {
		return nil, err
	}
	return client.(*persistence_client.Persistence_Client), nil
}

////////////////////////////////////////////////////////////////////////////////
// IMAP functions.
////////////////////////////////////////////////////////////////////////////////

// That service is use to give access to SQL.
// port number must be pass as argument.
func main() {

	// Give base info to retreive it configuration.

	// set the logger.
	//grpclog.SetLogger(log.New(os.Stdout, "smtp_service: ", log.LstdFlags))

	// Set the log information in case of crash...
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	// The actual server implementation.
	s_impl := new(server)
	s_impl.Name = string(mailpb.File_mail_proto.Services().Get(0).FullName())
	s_impl.Proto = mailpb.File_mail_proto.Path()
	s_impl.Path, _ = filepath.Abs(filepath.Dir(os.Args[0]))
	s_impl.Domain, _ = config.GetDomain()
	s_impl.Address, _ = config.GetAddress()
	s_impl.Proxy = defaultProxy
	s_impl.Protocol = "grpc"
	s_impl.Version = "0.0.1"
	s_impl.AllowAllOrigins = allow_all_origins
	s_impl.AllowedOrigins = allowed_origins
	s_impl.PublisherId = "localhost"
	s_impl.Permissions = make([]interface{}, 0)
	s_impl.SMTP_Port = 25      // non encrypted
	s_impl.SMTPS_Port = 465    // encrypted
	s_impl.SMTP_ALT_Port = 587 // This is the default smtp port (25 is almost alway's blocked by isp).
	s_impl.IMAP_Port = 143     // non
	s_impl.IMAPS_Port = 993
	s_impl.IMAP_ALT_Port = 1043 // non official
	s_impl.Keywords = make([]string, 0)
	s_impl.Repositories = make([]string, 0)
	s_impl.Discoveries = make([]string, 0)
	s_impl.Dependencies = []string{"log.LogService", "persistence.PersistenceService"}
	s_impl.Connections = make(map[string]connection)
	s_impl.DbIpV4 = "0.0.0.0:27018" // default MONGO port.
	s_impl.Process = -1
	s_impl.ProxyProcess = -1
	s_impl.Password = "adminadmin" // The default password for the admin.
	s_impl.KeepAlive = true
	s_impl.KeepUpToDate = true
	s_impl.Persistence_address, _ = config.GetAddress() // default set to the same srv...

	// register new client creator.
	Utility.RegisterFunction("NewMailService_Client", mail_client.NewMailService_Client)

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
	mailpb.RegisterMailServiceServer(s_impl.grpcServer, s_impl)
	reflection.Register(s_impl.grpcServer)

	// Here I will start the local smtp srv.
	go func() {
		certFile := config.GetLocalCertificate()
		domain, _ := config.GetDomain()
		name, _ := config.GetName()

		certFile = config.GetConfigDir() + "/tls/" + name + "." + domain + "/" + certFile

		// The backend connection.
		address := string(strings.Split(s_impl.DbIpV4, ":")[0])
		port := Utility.ToInt(strings.Split(s_impl.DbIpV4, ":")[1])

		// set variable for imap and smtp
		imap.Backend_address = address
		smtp.Backend_address = address
		imap.Backend_port = port
		smtp.Backend_port = port
		imap.Backend_password = s_impl.Password

		// if not connection withe the persistence service can be made...
		store, err := GetPersistenceClient(s_impl.Persistence_address)
		if err != nil {
			fmt.Println("fail to connect to persistence service", s_impl.Persistence_address, "with error with error: ", err)
			os.Exit(0)
		}

		imap.Store = store
		smtp.Store = store

		// Open the backend main connection
		nbTry := 10
		for ; nbTry > 0; nbTry-- {
			err = store.CreateConnection("local_resource", "local_resource", address, float64(port), 1, "sa", s_impl.Password, 500, "", false)
			if err == nil {
				break
			}
		}

		if nbTry == 0 {
			fmt.Println("fail to create connection local_resource ", address, err)
			os.Exit(0)
		}

		// start imap srv.
		fmt.Println("Start imap server on port ", s_impl.IMAP_Port)
		imap.StartImap(store, address, port, s_impl.Password, s_impl.KeyFile, certFile, s_impl.IMAP_Port, s_impl.IMAPS_Port, s_impl.IMAP_ALT_Port)

		fmt.Println("Start smtp server")
		// start smtp server
		smtp.StartSmtp(store, address, port, s_impl.Password, s_impl.Domain, s_impl.KeyFile, certFile, s_impl.SMTP_Port, s_impl.SMTPS_Port, s_impl.SMTP_ALT_Port)

	}()
	// Start the service.
	s_impl.StartService()

}
