// Package main implements the Mail gRPC service used by Globular.
// It provides SMTP sending helpers, IMAP/SMTP embedded servers bootstrap,
// Globular lifecycle (Init/Save/Start/Stop), connection management, and
// --describe/--health CLI utilities.
//
// Notes:
// - All public (exported) methods keep their original prototypes.
// - Logging uses slog with structured fields.
// - Errors returned via gRPC are enriched with context and call site metadata.
package main

import (
	"context"
	"crypto/tls"
	"errors"
	"io"
	"log/slog"
	"strings"
	"time"

	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/globular_client"
	globular "github.com/globulario/services/golang/globular_service"
	"github.com/globulario/services/golang/mail/mail_server/imap"
	"github.com/globulario/services/golang/mail/mail_server/smtp"
	"github.com/globulario/services/golang/mail/mailpb"
	"github.com/globulario/services/golang/persistence/persistence_client"
	"github.com/globulario/services/golang/resource/resourcepb"
	Utility "github.com/globulario/utility"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	smtp_ "net/smtp"

	gomail "gopkg.in/gomail.v2"
)

// -----------------------------------------------------------------------------
// Defaults & CORS
// -----------------------------------------------------------------------------

// -----------------------------------------------------------------------------
// Types
// -----------------------------------------------------------------------------

// connection holds credentials and target host for SMTP relaying.
type connection struct {
	Id       string // Connection id
	Host     string // Hostname or IPv4
	User     string
	Password string
	Port     int32
}

// server is the concrete Mail service and Globular plumbing.
type server struct {
	// Core metadata
	Id                 string
	Name               string
	Mac                string
	Path               string
	Proto              string
	Port               int
	Proxy              int
	Protocol           string
	AllowAllOrigins    bool
	AllowedOrigins     string // comma separated
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
	PublisherID        string
	KeepUpToDate       bool
	Plaform            string
	Checksum           string
	KeepAlive          bool
	Permissions        []interface{}
	Dependencies       []string
	Process            int
	ProxyProcess       int
	ConfigPath         string
	LastError          string
	ModTime            int64
	State              string

	// Runtime
	grpcServer *grpc.Server

	// SMTP/IMAP
	Connections         map[string]connection
	Persistence_address string
	SMTP_Port           int
	SMTPS_Port          int // SSL
	SMTP_ALT_Port       int // Alternate
	IMAP_Port           int
	IMAPS_Port          int // SSL
	IMAP_ALT_Port       int // Alternate

	// Backend auth for IMAP/SMTP store
	Password string

	// Persistence DB address (e.g., 0.0.0.0:27017)
	DbIpV4 string

	logger *slog.Logger
}

// -----------------------------------------------------------------------------
// Globular Service Contract (Exported Getters/Setters)
// -----------------------------------------------------------------------------

// GetConfigurationPath returns the path where the service configuration is stored.
func (srv *server) GetConfigurationPath() string { return srv.ConfigPath }

// SetConfigurationPath sets the path where the service configuration is stored.
func (srv *server) SetConfigurationPath(path string) { srv.ConfigPath = path }

// GetAddress returns the HTTP address where /config is served.
func (srv *server) GetAddress() string { return srv.Address }

// SetAddress sets the HTTP address where /config is served.
func (srv *server) SetAddress(address string) { srv.Address = address }

// GetProcess returns the process id of the service or -1 if not started.
func (srv *server) GetProcess() int { return srv.Process }

// SetProcess records the process id.
func (srv *server) SetProcess(pid int) { srv.Process = pid }

// GetProxyProcess returns the reverse proxy pid or -1 if not started.
func (srv *server) GetProxyProcess() int { return srv.ProxyProcess }

// SetProxyProcess records the reverse proxy pid.
func (srv *server) SetProxyProcess(pid int) { srv.ProxyProcess = pid }

// GetState returns the current service state (e.g., "running").
func (srv *server) GetState() string { return srv.State }

// SetState sets the current service state.
func (srv *server) SetState(state string) { srv.State = state }

// GetLastError returns the last recorded error message.
func (srv *server) GetLastError() string { return srv.LastError }

// SetLastError records the last error message.
func (srv *server) SetLastError(err string) { srv.LastError = err }

// SetModTime sets the last modification time (unix seconds).
func (srv *server) SetModTime(modtime int64) { srv.ModTime = modtime }

// GetModTime returns the last modification time (unix seconds).
func (srv *server) GetModTime() int64 { return srv.ModTime }

// GetId returns this service instance id.
func (srv *server) GetId() string { return srv.Id }

// SetId sets this service instance id.
func (srv *server) SetId(id string) { srv.Id = id }

// GetName returns the gRPC service name.
func (srv *server) GetName() string { return srv.Name }

// SetName sets the gRPC service name.
func (srv *server) SetName(name string) { srv.Name = name }

// GetDescription returns the service description.
func (srv *server) GetDescription() string { return srv.Description }

// SetDescription sets the service description.
func (srv *server) SetDescription(description string) { srv.Description = description }

// GetMac returns the host MAC address string (if provided by platform).
func (srv *server) GetMac() string { return srv.Mac }

// SetMac sets the host MAC address string.
func (srv *server) SetMac(mac string) { srv.Mac = mac }

// GetKeywords returns the service keywords.
func (srv *server) GetKeywords() []string { return srv.Keywords }

// SetKeywords sets the service keywords.
func (srv *server) SetKeywords(keywords []string) { srv.Keywords = keywords }

// Dist packages the service binary/artifacts to the given path using Globular.
func (srv *server) Dist(path string) (string, error) { return globular.Dist(path, srv) }

// GetDependencies returns the list of dependent services.
func (srv *server) GetDependencies() []string {
	if srv.Dependencies == nil {
		srv.Dependencies = make([]string, 0)
	}
	return srv.Dependencies
}

// SetDependency appends a dependency if not already present.
func (srv *server) SetDependency(dependency string) {
	if srv.Dependencies == nil {
		srv.Dependencies = make([]string, 0)
	}
	if !Utility.Contains(srv.Dependencies, dependency) {
		srv.Dependencies = append(srv.Dependencies, dependency)
	}
}

// GetChecksum returns the binary checksum.
func (srv *server) GetChecksum() string { return srv.Checksum }

// SetChecksum sets the binary checksum.
func (srv *server) SetChecksum(checksum string) { srv.Checksum = checksum }

// GetPlatform returns the platform string (e.g., "linux/amd64").
func (srv *server) GetPlatform() string { return srv.Plaform }

// SetPlatform sets the platform string (e.g., "linux/amd64").
func (srv *server) SetPlatform(platform string) { srv.Plaform = platform }

// GetPath returns the executable path.
func (srv *server) GetPath() string { return srv.Path }

// SetPath sets the executable path.
func (srv *server) SetPath(path string) { srv.Path = path }

// GetRepositories returns associated repositories.
func (srv *server) GetRepositories() []string { return srv.Repositories }

// SetRepositories sets associated repositories.
func (srv *server) SetRepositories(repositories []string) { srv.Repositories = repositories }

// GetDiscoveries returns discovery endpoints.
func (srv *server) GetDiscoveries() []string { return srv.Discoveries }

// SetDiscoveries sets discovery endpoints.
func (srv *server) SetDiscoveries(discoveries []string) { srv.Discoveries = discoveries }

// GetProto returns the .proto path.
func (srv *server) GetProto() string { return srv.Proto }

// SetProto sets the .proto path.
func (srv *server) SetProto(proto string) { srv.Proto = proto }

// GetPort returns the gRPC port.
func (srv *server) GetPort() int { return srv.Port }

// SetPort sets the gRPC port.
func (srv *server) SetPort(port int) { srv.Port = port }

// GetProxy returns the reverse proxy port (for gRPC-Web).
func (srv *server) GetProxy() int { return srv.Proxy }

// SetProxy sets the reverse proxy port (for gRPC-Web).
func (srv *server) SetProxy(proxy int) { srv.Proxy = proxy }

// GetProtocol returns the network protocol (e.g., "grpc", "tls", "https").
func (srv *server) GetProtocol() string { return srv.Protocol }

// SetProtocol sets the network protocol (e.g., "grpc", "tls", "https").
func (srv *server) SetProtocol(protocol string) { srv.Protocol = protocol }

// GetAllowAllOrigins returns whether all origins are allowed.
func (srv *server) GetAllowAllOrigins() bool { return srv.AllowAllOrigins }

// SetAllowAllOrigins sets whether all origins are allowed.
func (srv *server) SetAllowAllOrigins(allowAllOrigins bool) { srv.AllowAllOrigins = allowAllOrigins }

// GetAllowedOrigins returns the comma-separated allowed origins list.
func (srv *server) GetAllowedOrigins() string { return srv.AllowedOrigins }

// SetAllowedOrigins sets the comma-separated allowed origins list.
func (srv *server) SetAllowedOrigins(allowedOrigins string) { srv.AllowedOrigins = allowedOrigins }

// GetDomain returns domain (ip/DNS).
func (srv *server) GetDomain() string { return srv.Domain }

// SetDomain sets domain (ip/DNS).
func (srv *server) SetDomain(domain string) { srv.Domain = domain }

// GetTls returns true if TLS is enabled.
func (srv *server) GetTls() bool { return srv.TLS }

// SetTls enables/disables TLS.
func (srv *server) SetTls(hasTls bool) { srv.TLS = hasTls }

// GetCertAuthorityTrust returns path to CA trust bundle.
func (srv *server) GetCertAuthorityTrust() string { return srv.CertAuthorityTrust }

// SetCertAuthorityTrust sets path to CA trust bundle.
func (srv *server) SetCertAuthorityTrust(ca string) { srv.CertAuthorityTrust = ca }

// GetCertFile returns path to TLS certificate.
func (srv *server) GetCertFile() string { return srv.CertFile }

// SetCertFile sets path to TLS certificate.
func (srv *server) SetCertFile(certFile string) { srv.CertFile = certFile }

// GetKeyFile returns path to TLS private key.
func (srv *server) GetKeyFile() string { return srv.KeyFile }

// SetKeyFile sets path to TLS private key.
func (srv *server) SetKeyFile(keyFile string) { srv.KeyFile = keyFile }

// GetVersion returns service version.
func (srv *server) GetVersion() string { return srv.Version }

// SetVersion sets service version.
func (srv *server) SetVersion(version string) { srv.Version = version }

// GetPublisherID returns publisher id.
func (srv *server) GetPublisherID() string { return srv.PublisherID }

// SetPublisherID sets publisher id.
func (srv *server) SetPublisherID(PublisherID string) { srv.PublisherID = PublisherID }

// GetKeepUpToDate returns whether auto-update is enabled.
func (srv *server) GetKeepUpToDate() bool { return srv.KeepUpToDate }

// SetKeepUptoDate toggles auto-update.
func (srv *server) SetKeepUptoDate(val bool) { srv.KeepUpToDate = val }

// GetKeepAlive returns whether the supervisor should keep the service alive.
func (srv *server) GetKeepAlive() bool { return srv.KeepAlive }

// SetKeepAlive toggles keep-alive behavior.
func (srv *server) SetKeepAlive(val bool) { srv.KeepAlive = val }

// GetPermissions returns action permissions.
func (srv *server) GetPermissions() []interface{} { return srv.Permissions }

// SetPermissions sets action permissions.
func (srv *server) SetPermissions(permissions []interface{}) { srv.Permissions = permissions }
func (srv *server) GetGrpcServer() *grpc.Server              { return srv.grpcServer }

func (srv *server) RolesDefault() []resourcepb.Role {
	domain, _ := config.GetDomain()

	return []resourcepb.Role{
		{
			Id:          "role:mail.sender",
			Name:        "Mail Sender",
			Domain:      domain,
			Description: "Can send emails (simple and with attachments) using existing connections.",
			Actions: []string{
				"/mail.MailService/SendEmail",
				"/mail.MailService/SendEmailWithAttachements",
			},
			TypeName: "resource.Role",
		},
		{
			Id:          "role:mail.connector.admin",
			Name:        "Mail Connector Admin",
			Domain:      domain,
			Description: "Manage SMTP connection profiles (create/delete).",
			Actions: []string{
				"/mail.MailService/CreateConnection",
				"/mail.MailService/DeleteConnection",
			},
			TypeName: "resource.Role",
		},
		{
			Id:          "role:mail.admin",
			Name:        "Mail Service Admin",
			Domain:      domain,
			Description: "Full control over MailService, including stop and connection/profile management.",
			Actions: []string{
				"/mail.MailService/Stop",
				"/mail.MailService/CreateConnection",
				"/mail.MailService/DeleteConnection",
				"/mail.MailService/SendEmail",
				"/mail.MailService/SendEmailWithAttachements",
			},
			TypeName: "resource.Role",
		},
	}
}

// Init loads/creates configuration and initializes the gRPC server.
func (srv *server) Init() error {
	if err := globular.InitService(srv); err != nil {
		return err
	}
	gs, err := globular.InitGrpcServer(srv)
	if err != nil {
		return err
	}
	srv.grpcServer = gs
	return nil
}

// Save persists the current configuration to disk.
func (srv *server) Save() error { return globular.SaveService(srv) }

// StopService gracefully stops gRPC serving.
func (srv *server) StopService() error { return globular.StopService(srv, srv.grpcServer) }

// Stop stops the service via gRPC call.
func (srv *server) Stop(ctx context.Context, _ *mailpb.StopRequest) (*mailpb.StopResponse, error) {
	return &mailpb.StopResponse{}, srv.StopService()
}

// CarbonCopy represents a single CC target.
type CarbonCopy struct {
	EMail string
	Name  string
}

// Attachment wraps a filename and data buffer. If FileData is nil/empty, the file
// is assumed to be on disk at FileName and gomail will read it from there.
type Attachment struct {
	FileName string
	FileData []byte
}

// sendEmail relays an email via the provided SMTP host/user/password/port.
// bodyType should be "text/plain" or "text/html".
func (srv *server) sendEmail(
	host, user, pwd string, port int,
	from string,
	to []string,
	cc []*CarbonCopy,
	subject, body string,
	attachs []*Attachment,
	bodyType string,
) error {
	msg := gomail.NewMessage()
	msg.SetHeader("From", from)
	msg.SetHeader("To", to...)

	if len(cc) > 0 {
		ccList := make([]string, 0, len(cc))
		for _, c := range cc {
			ccList = append(ccList, msg.FormatAddress(c.EMail, c.Name))
		}
		msg.SetHeader("Cc", ccList...)
	}

	msg.SetHeader("Subject", subject)
	msg.SetBody(bodyType, body)

	for _, a := range attachs {
		fn := a.FileName
		if len(a.FileData) == 0 {
			msg.Attach(fn)
			continue
		}
		msg.Attach(fn, gomail.SetCopyFunc(func(w io.Writer) error {
			_, err := w.Write(a.FileData)
			return err
		}))
	}

	dialer := gomail.NewDialer(host, port, user, pwd)
	dialer.Auth = smtp_.PlainAuth("", dialer.Username, dialer.Password, dialer.Host)

	if port != 25 {
		cer, err := tls.LoadX509KeyPair(srv.CertFile, srv.KeyFile)
		if err != nil {
			return status.Errorf(codes.Internal, "smtp tls keypair load failed (cert=%s key=%s): %v", srv.CertFile, srv.KeyFile, err)
		}
		dialer.TLSConfig = &tls.Config{ServerName: host, Certificates: []tls.Certificate{cer}}
	}

	if err := dialer.DialAndSend(msg); err != nil {
		return status.Errorf(codes.Internal, "smtp dial/send failed (host=%s port=%d user=%s to=%d): %v", host, port, user, len(to), err)
	}
	return nil
}

// CreateConnection creates or updates an SMTP connection profile and persists it.
func (srv *server) CreateConnection(ctx context.Context, rqst *mailpb.CreateConnectionRqst) (*mailpb.CreateConnectionRsp, error) {
	if rqst == nil || rqst.Connection == nil {
		return nil, status.Errorf(codes.InvalidArgument, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("missing connection payload")))
	}

	c := connection{
		Id:       rqst.Connection.Id,
		Host:     rqst.Connection.Host,
		Port:     rqst.Connection.Port,
		User:     rqst.Connection.User,
		Password: rqst.Connection.Password,
	}

	if c.Id == "" || c.Host == "" || c.Port == 0 {
		return nil, status.Errorf(codes.InvalidArgument, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("id, host and port are required")))
	}

	if srv.Connections == nil {
		srv.Connections = make(map[string]connection)
	}
	srv.Connections[c.Id] = c

	if err := srv.Save(); err != nil {
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &mailpb.CreateConnectionRsp{Result: true}, nil
}

// DeleteConnection removes an SMTP connection profile and persists changes.
func (srv *server) DeleteConnection(ctx context.Context, rqst *mailpb.DeleteConnectionRqst) (*mailpb.DeleteConnectionRsp, error) {
	if rqst == nil {
		return nil, status.Errorf(codes.InvalidArgument, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("missing request")))
	}
	id := strings.TrimSpace(rqst.GetId())
	if id == "" {
		return nil, status.Errorf(codes.InvalidArgument, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("missing id")))
	}

	if _, ok := srv.Connections[id]; ok {
		delete(srv.Connections, id)
		if err := srv.Save(); err != nil {
			return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
	}

	return &mailpb.DeleteConnectionRsp{Result: true}, nil
}

// SendEmail sends a simple email without attachments using a stored connection.
func (srv *server) SendEmail(ctx context.Context, rqst *mailpb.SendEmailRqst) (*mailpb.SendEmailRsp, error) {
	if rqst == nil {
		return nil, status.Errorf(codes.InvalidArgument, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("missing request")))
	}
	conn, ok := srv.Connections[rqst.Id]
	if !ok {
		return nil, status.Errorf(codes.NotFound, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("no connection with id "+rqst.Id)))
	}
	if rqst.Email == nil {
		return nil, status.Errorf(codes.InvalidArgument, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("missing email payload")))
	}

	ccs := make([]*CarbonCopy, len(rqst.Email.Cc))
	for i := range rqst.Email.Cc {
		ccs[i] = &CarbonCopy{Name: rqst.Email.Cc[i].Name, EMail: rqst.Email.Cc[i].Address}
	}

	bodyType := "text/html"
	if rqst.Email.BodyType != mailpb.BodyType_HTML {
		bodyType = "text/plain"
	}

	if err := srv.sendEmail(
		conn.Host, conn.User, conn.Password, int(conn.Port),
		rqst.Email.From, rqst.Email.To, ccs, rqst.Email.Subject, rqst.Email.Body,
		nil, bodyType); err != nil {
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &mailpb.SendEmailRsp{Result: true}, nil
}

// SendEmailWithAttachements streams an email with attachments, buffering parts,
// then sends via a stored connection when the stream completes.
func (srv *server) SendEmailWithAttachements(stream mailpb.MailService_SendEmailWithAttachementsServer) error {
	attachments := make([]*Attachment, 0)
	var (
		bodyType string = "text/plain"
		body     string
		subject  string
		from     string
		to       []string
		cc       []*CarbonCopy
		id       string
	)

	for {
		rqst, err := stream.Recv()
		if err == io.EOF {
			conn, ok := srv.Connections[id]
			if !ok {
				return status.Errorf(codes.NotFound, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("no connection with id "+id)))
			}
			if err := srv.sendEmail(conn.Host, conn.User, conn.Password, int(conn.Port), from, to, cc, subject, body, attachments, bodyType); err != nil {
				return status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
			}
			if err := stream.SendAndClose(&mailpb.SendEmailWithAttachementsRsp{Result: true}); err != nil {
				return status.Errorf(codes.Internal, "stream close failed: %v", err)
			}
			return nil
		}
		if err != nil {
			return status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}

		id = rqst.Id

		switch msg := rqst.Data.(type) {
		case *mailpb.SendEmailWithAttachementsRqst_Email:
			cc = make([]*CarbonCopy, len(msg.Email.Cc))
			for i := range msg.Email.Cc {
				cc[i] = &CarbonCopy{Name: msg.Email.Cc[i].Name, EMail: msg.Email.Cc[i].Address}
			}
			if msg.Email.BodyType == mailpb.BodyType_HTML {
				bodyType = "text/html"
			} else {
				bodyType = "text/plain"
			}
			from = msg.Email.From
			to = msg.Email.To
			body = msg.Email.Body
			subject = msg.Email.Subject

		case *mailpb.SendEmailWithAttachementsRqst_Attachements:
			var last *Attachment
			if len(attachments) > 0 {
				last = attachments[len(attachments)-1]
				if last.FileName != msg.Attachements.FileName {
					last = &Attachment{FileName: msg.Attachements.FileName, FileData: make([]byte, 0)}
					attachments = append(attachments, last)
				}
			} else {
				last = &Attachment{FileName: msg.Attachements.FileName, FileData: make([]byte, 0)}
				attachments = append(attachments, last)
			}
			last.FileData = append(last.FileData, msg.Attachements.FileData...)
		}
	}
}

// getPersistenceClient returns a connected persistence client for the given address.
func getPersistenceClient(address string) (*persistence_client.Persistence_Client, error) {
	Utility.RegisterFunction("NewPersistenceService_Client", persistence_client.NewPersistenceService_Client)
	client, err := globular_client.GetClient(address, "persistence.PersistenceService", "NewPersistenceService_Client")
	if err != nil {
		return nil, err
	}
	return client.(*persistence_client.Persistence_Client), nil
}

// StartService starts serving gRPC and launches embedded IMAP/SMTP servers.
func (srv *server) StartService() error {
	if srv.logger == nil {
		srv.logger = logger
	}

	// Start embedded IMAP/SMTP in a goroutine
	go func() {
		certFile := config.GetLocalCertificate()
		address, _ := config.GetHostname()
		port := 27018
		imap.Backend_address = address
		smtp.Backend_address = address
		imap.Backend_port = port
		smtp.Backend_port = port
		imap.Backend_password = srv.Password

		store, err := getPersistenceClient(srv.Persistence_address)
		if err != nil {
			srv.logger.Error("persistence connect failed", "address", srv.Persistence_address, "err", err)
			return
		}

		imap.Store = store
		smtp.Store = store

		nbTry := 10
		for ; nbTry > 0; nbTry-- {
			err = store.CreateConnection("local_resource", "local_resource", address, float64(port), 1, "sa", srv.Password, 500, "", false)
			if err == nil {
				break
			}
			time.Sleep(300 * time.Millisecond)
		}
		if nbTry == 0 {
			srv.logger.Warn("persistence default connection failed", "address", address, "port", port, "err", err)
			return
		}

		imap.StartImap(store, address, port, srv.Password, srv.KeyFile, certFile, srv.IMAP_Port, srv.IMAPS_Port, srv.IMAP_ALT_Port)
		smtp.StartSmtp(store, address, port, srv.Password, srv.Domain, srv.KeyFile, certFile, srv.SMTP_Port, srv.SMTPS_Port, srv.SMTP_ALT_Port)
	}()

	return globular.StartService(srv, srv.grpcServer)
}
