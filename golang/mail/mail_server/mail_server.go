package main

import (
	"context"
	"errors"

	//"fmt"
	"io"
	"log"
	"os"
	"strconv"
	"strings"

	globular "github.com/globulario/services/golang/globular_service"
	"github.com/globulario/services/golang/interceptors"
	"github.com/globulario/services/golang/persistence/persistence_client"

	"github.com/davecourtois/Utility"
	"github.com/globulario/services/golang/mail/mail_client"
	"github.com/globulario/services/golang/mail/mailpb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"

	//"google.golang.org/grpc/grpclog"

	"github.com/globulario/services/golang/mail/mail_server/imap"
	"github.com/globulario/services/golang/mail/mail_server/smtp"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"

	gomail "gopkg.in/gomail.v1"
)

var (
	defaultPort  = 10067
	defaultProxy = 10068

	// By default all origins are allowed.
	allow_all_origins = true

	// comma separeated values.
	allowed_origins string = ""

	// the domain of the server.
	domain string = "localhost"
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
	KeepAlive          bool
	Permissions        []interface{} // contains the action permission for the services.
	Dependencies       []string      // The list of services needed by this services.
	Process            int
	ProxyProcess       int
	ConfigPath         string
	LastError          string
	ModTime 		int64

	// The grpc server.
	grpcServer *grpc.Server

	// The map of connection...
	Connections map[string]connection

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

func (svr *server) GetProcess() int {
	return svr.Process
}

func (svr *server) SetProcess(pid int) {
	svr.SetProcess(pid)
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

// The description of the service
func (svr *server) GetDescription() string {
	return svr.Description
}
func (svr *server) SetDescription(description string) {
	svr.Description = description
}

func (svr *server) GetMac() string {
	return svr.Mac
}

func (svr *server) SetMac(mac string) {
	svr.Mac = mac
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

// The path of the executable.
func (svr *server) GetPath() string {
	return svr.Path
}
func (svr *server) SetPath(path string) {
	svr.Path = path
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
	Utility.RegisterFunction("NewMailService_Client", mail_client.NewMailService_Client)

	err := globular.InitService(svr)
	if err != nil {
		return err
	}

	// Initialyse GRPC server.
	svr.grpcServer, err = globular.InitGrpcServer(svr, interceptors.ServerUnaryInterceptor, interceptors.ServerStreamInterceptor)
	if err != nil {
		return err
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

func (svr *server) Stop(context.Context, *mailpb.StopRequest) (*mailpb.StopResponse, error) {
	return &mailpb.StopResponse{}, svr.StopService()
}

//////////////////////////// SMPT specific functions ///////////////////////////

// Create a new connection and store it for futur use. If the connection already
// exist it will be replace by the new one.
func (svr *server) CreateConnection(ctx context.Context, rsqt *mailpb.CreateConnectionRqst) (*mailpb.CreateConnectionRsp, error) {

	var c connection
	var err error

	// Set the connection info from the request.
	c.Id = rsqt.Connection.Id
	c.Host = rsqt.Connection.Host
	c.Port = rsqt.Connection.Port
	c.User = rsqt.Connection.User
	c.Password = rsqt.Connection.Password

	// set or update the connection and save it in json file.
	svr.Connections[c.Id] = c

	// In that case I will save it in file.
	err = svr.Save()
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// test if the connection is reacheable.
	// _, err = svr.ping(ctx, c.Id)

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
func (svr *server) DeleteConnection(ctx context.Context, rqst *mailpb.DeleteConnectionRqst) (*mailpb.DeleteConnectionRsp, error) {
	id := rqst.GetId()
	if _, ok := svr.Connections[id]; !ok {
		return &mailpb.DeleteConnectionRsp{
			Result: true,
		}, nil
	}

	delete(svr.Connections, id)

	// In that case I will save it in file.
	err := svr.Save()
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
 * Send mail... The server id is the authentification id...
 */
func (svr *server) sendEmail(host string, user string, pwd string, port int, from string, to []string, cc []*CarbonCopy, subject string, body string, attachs []*Attachment, bodyType string) error {

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
		f := gomail.CreateFile(attachs[i].FileName, attachs[i].FileData)
		msg.Attach(f)
	}

	mailer := gomail.NewMailer(host, user, pwd, port)

	if err := mailer.Send(msg); err != nil {
		return err
	}
	return nil
}

// Send a simple email whitout file.
func (svr *server) SendEmail(ctx context.Context, rqst *mailpb.SendEmailRqst) (*mailpb.SendEmailRsp, error) {

	if _, ok := svr.Connections[rqst.Id]; !ok {
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

	err := svr.sendEmail(svr.Connections[rqst.Id].Host, svr.Connections[rqst.Id].User, svr.Connections[rqst.Id].Password, int(svr.Connections[rqst.Id].Port), rqst.Email.From, rqst.Email.To, cc, rqst.Email.Subject, rqst.Email.Body, []*Attachment{}, bodyType)
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
func (svr *server) SendEmailWithAttachements(stream mailpb.MailService_SendEmailWithAttachementsServer) error {

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
		if _, ok := svr.Connections[rqst.Id]; !ok {
			return status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("No connection found with id "+rqst.Id)))
		}
		if err == io.EOF {

			// Here all data is read...
			c := svr.Connections[id]
			err := svr.sendEmail(c.Host, c.User, c.Password, int(c.Port), from, to, cc, subject, body, attachements, bodyType)

			if err != nil {
				return status.Errorf(
					codes.Internal,
					Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
			}

			// Close the stream...
			stream.SendAndClose(&mailpb.SendEmailWithAttachementsRsp{
				Result: true,
			})

			return nil
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

			// The email itsvr.
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

	return nil
}

////////////////////////////////////////////////////////////////////////////////
// IMAP functions.
////////////////////////////////////////////////////////////////////////////////

// That service is use to give access to SQL.
// port number must be pass as argument.
func main() {

	log.Println("---> start mail server")
	port := defaultPort // the default value.
	if len(os.Args) == 2 {
		port, _ = strconv.Atoi(os.Args[1]) // The second argument must be the port number
	}
	// set the logger.
	//grpclog.SetLogger(log.New(os.Stdout, "smtp_service: ", log.LstdFlags))

	// Set the log information in case of crash...
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	// The actual server implementation.
	s_impl := new(server)
	s_impl.Name = string(mailpb.File_mail_proto.Services().Get(0).FullName())
	s_impl.Proto = mailpb.File_mail_proto.Path()
	s_impl.Port = port
	s_impl.Domain = domain
	s_impl.Proxy = defaultProxy
	s_impl.Protocol = "grpc"
	s_impl.Version = "0.0.1"
	s_impl.AllowAllOrigins = allow_all_origins
	s_impl.AllowedOrigins = allowed_origins
	s_impl.PublisherId = "globulario"
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
	s_impl.Dependencies = make([]string, 0)
	s_impl.Connections = make(map[string]connection)
	s_impl.DbIpV4 = "0.0.0.0:27017" // default mongodb port.
	s_impl.Process = -1
	s_impl.ProxyProcess = -1
	s_impl.Password = "adminadmin" // The default password for the admin.
	s_impl.KeepAlive = true
	
	// Here I will retreive the list of connections from file if there are some...
	err := s_impl.Init()
	if err != nil {
		log.Fatalf("Fail to initialyse service %s: %s", s_impl.Name, s_impl.Id)
	}

	// Register the echo services
	mailpb.RegisterMailServiceServer(s_impl.grpcServer, s_impl)
	reflection.Register(s_impl.grpcServer)

	// Here I will start the local smtp server.
	go func() {
		certFile := s_impl.CertFile

		// Here in case of tls connection I will use the domain certificate instead of the server certificate.
		if s_impl.TLS == true {
			certFile = certFile[0:strings.Index(certFile, "server.crt")] + s_impl.Domain + ".crt"
		}

		address := string(strings.Split(s_impl.DbIpV4, ":")[0])
		port := Utility.ToInt(strings.Split(s_impl.DbIpV4, ":")[1])

		// The backend connection.
		store, err := persistence_client.NewPersistenceService_Client(address, "persistence.PersistenceService")
		if err != nil {
			return
		}

		// set variable for imap and smtp
		imap.Backend_address = address
		smtp.Backend_address = address
		imap.Backend_port = port
		smtp.Backend_port = port
		imap.Backend_password = s_impl.Password
		imap.Store = store
		smtp.Store = store

		// Open the backend main connection
		err = store.CreateConnection("local_ressource", "local_ressource", address, float64(port), 0, "sa", s_impl.Password, 5000, "", false)
		if err != nil {
			return
		}
		// start imap server.
		imap.StartImap(store, address, port, s_impl.Password, s_impl.KeyFile, certFile, s_impl.IMAP_Port, s_impl.IMAPS_Port, s_impl.IMAP_ALT_Port)

		// start smtp server
		smtp.StartSmtp(store, address, port, s_impl.Domain, s_impl.KeyFile, certFile, s_impl.SMTP_Port, s_impl.SMTPS_Port, s_impl.SMTP_ALT_Port)

	}()
	// Start the service.
	s_impl.StartService()

}
