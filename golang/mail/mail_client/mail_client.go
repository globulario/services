package mail_client

import (
	"context"
	"time"

	"fmt"
	"io"
	"os"

	globular "github.com/globulario/services/golang/globular_client"
	"github.com/globulario/services/golang/mail/mailpb"
	"github.com/globulario/services/golang/security"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

// //////////////////////////////////////////////////////////////////////////////
// mail Client Service
// //////////////////////////////////////////////////////////////////////////////
type Mail_Client struct {
	cc *grpc.ClientConn
	c  mailpb.MailServiceClient

	// The id of the service
	id string

	// The name of the service
	name string

	// The mac address of the server
	mac string

	// The client domain
	domain string

	//  keep the last connection state of the client.
	state string

	// The address where connection with client can be done. ex: globule0.globular.cloud:10101
	address string

	// The port
	port int

	// is the connection is secure?
	hasTLS bool

	// Link to client key file
	keyFile string

	// Link to client certificate file.
	certFile string

	// certificate authority file
	caFile string

	// The client context
	ctx context.Context
}

// Create a connection to the service.
func NewMailService_Client(address string, id string) (*Mail_Client, error) {
	client := new(Mail_Client)
	err := globular.InitClient(client, address, id)
	if err != nil {
		return nil, err
	}

	err = client.Reconnect()
	if err != nil {
		return nil, err
	}

	return client, nil
}

func (client *Mail_Client) Reconnect() error {
	var err error
	nb_try_connect := 10
	
	for i:=0; i <nb_try_connect; i++ {
		client.cc, err = globular.GetClientConnection(client)
		if err == nil {
			client.c = mailpb.NewMailServiceClient(client.cc)
			break
		}
		
		// wait 500 millisecond before next try
		time.Sleep(500 * time.Millisecond)
	}

	return err
}

// The address where the client can connect.
func (client *Mail_Client) SetAddress(address string) {
	client.address = address
}

func (client *Mail_Client) Invoke(method string, rqst interface{}, ctx context.Context) (interface{}, error) {
	if ctx == nil {
		ctx = client.GetCtx()
	}
	return globular.InvokeClientRequest(client.c, ctx, method, rqst)
}

func (client *Mail_Client) GetCtx() context.Context {
	if client.ctx == nil {
		client.ctx = globular.GetClientContext(client)
	}

	token, err := security.GetLocalToken(client.GetMac())
	if err == nil {
		md := metadata.New(map[string]string{"token": string(token), "domain": client.domain, "mac": client.GetMac()})
		client.ctx = metadata.NewOutgoingContext(context.Background(), md)
	}
	return client.ctx
}

// Return the domain
func (client *Mail_Client) GetDomain() string {
	return client.domain
}

// Return the address
func (client *Mail_Client) GetAddress() string {
	return client.address
}

// Return the last know connection state
func (client *Mail_Client) GetState() string {
	return client.state
}

// Return the id of the service instance
func (client *Mail_Client) GetId() string {
	return client.id
}

// Return the name of the service
func (client *Mail_Client) GetName() string {
	return client.name
}

func (client *Mail_Client) GetMac() string {
	return client.mac
}

// must be close when no more needed.
func (client *Mail_Client) Close() {
	client.cc.Close()
}

// Set grpc_service port.
func (client *Mail_Client) SetPort(port int) {
	client.port = port
}

// Return the grpc port number
func (client *Mail_Client) GetPort() int {
	return client.port
}

// Set the client name.
func (client *Mail_Client) SetName(name string) {
	client.name = name
}

func (client *Mail_Client) SetMac(mac string) {
	client.mac = mac
}

// Set the service instance id
func (client *Mail_Client) SetId(id string) {
	client.id = id
}

// Set the domain.
func (client *Mail_Client) SetDomain(domain string) {
	client.domain = domain
}

func (client *Mail_Client) SetState(state string) {
	client.state = state
}

////////////////// TLS ///////////////////

// Get if the client is secure.
func (client *Mail_Client) HasTLS() bool {
	return client.hasTLS
}

// Get the TLS certificate file path
func (client *Mail_Client) GetCertFile() string {
	return client.certFile
}

// Get the TLS key file path
func (client *Mail_Client) GetKeyFile() string {
	return client.keyFile
}

// Get the TLS key file path
func (client *Mail_Client) GetCaFile() string {
	return client.caFile
}

// Set the client is a secure client.
func (client *Mail_Client) SetTLS(hasTls bool) {
	client.hasTLS = hasTls
}

// Set TLS certificate file path
func (client *Mail_Client) SetCertFile(certFile string) {
	client.certFile = certFile
}

// Set TLS key file path
func (client *Mail_Client) SetKeyFile(keyFile string) {
	client.keyFile = keyFile
}

// Set TLS authority trust certificate file path
func (client *Mail_Client) SetCaFile(caFile string) {
	client.caFile = caFile
}

//////////////////////////////// Api ////////////////////////////////

// Stop the service.
func (client *Mail_Client) StopService() {
	client.c.Stop(client.GetCtx(), &mailpb.StopRequest{})
}

/**
 * Create a connection with a mail server.
 */
func (client *Mail_Client) CreateConnection(id string, user string, pwd string, port int, host string) error {
	rqst := &mailpb.CreateConnectionRqst{
		Connection: &mailpb.Connection{
			Id:       id,
			User:     user,
			Password: pwd,
			Port:     int32(port),
			Host:     host,
		},
	}

	_, err := client.c.CreateConnection(client.GetCtx(), rqst)

	return err
}

/**
 * Delete a connection with a mail server.
 */
func (client *Mail_Client) DeleteConnection(id string) error {

	rqst := &mailpb.DeleteConnectionRqst{
		Id: id,
	}
	_, err := client.c.DeleteConnection(client.GetCtx(), rqst)
	return err
}

/**
 * Send email whiout files.
 */
func (client *Mail_Client) SendEmail(id string, from string, to []string, cc []*mailpb.CarbonCopy, subject string, body string, bodyType int32) error {

	rqst := &mailpb.SendEmailRqst{
		Id: id,
		Email: &mailpb.Email{
			From:     from,
			To:       to,
			Cc:       cc,
			Subject:  subject,
			Body:     body,
			BodyType: mailpb.BodyType(bodyType),
		},
	}

	_, err := client.c.SendEmail(client.GetCtx(), rqst)

	return err
}

/**
 * Send email with files.
 */
/**
 * Here I will make use of a stream
 */
func sendFile(id string, path string, stream mailpb.MailService_SendEmailWithAttachementsClient) {

	file, err := os.Open(path)
	if err != nil {
		fmt.Print("Fail to open file "+path+" with error: %v", err)
	}

	// close the file when done.
	defer file.Close()

	const BufferSize = 1024 * 5 // the chunck size.

	buffer := make([]byte, BufferSize)
	for {
		bytesread, err := file.Read(buffer)
		if bytesread > 0 {
			rqst := &mailpb.SendEmailWithAttachementsRqst{
				Id: id,
				Data: &mailpb.SendEmailWithAttachementsRqst_Attachements{
					Attachements: &mailpb.Attachement{
						FileName: path,
						FileData: buffer[:bytesread],
					},
				},
			}
			err = stream.Send(rqst)
		}

		if err != nil {
			if err != io.EOF {
				fmt.Println(err)
			}
			break
		}
	}
}

/**
 * Test send email with attachements.
 */
func (client *Mail_Client) SendEmailWithAttachements(id string, from string, to []string, cc []*mailpb.CarbonCopy, subject string, body string, bodyType int32, files []string) error {

	// Open the stream...
	stream, err := client.c.SendEmailWithAttachements(client.GetCtx())
	if err != nil {
		fmt.Println("error while TestSendEmailWithAttachements:", err)
	}

	// Send file attachment as a stream, not need to be send first.
	for i := 0; i < len(files); i++ {
		sendFile(id, files[i], stream)
	}

	// Send the email message...
	rqst := &mailpb.SendEmailWithAttachementsRqst{
		Id: id,
		Data: &mailpb.SendEmailWithAttachementsRqst_Email{
			Email: &mailpb.Email{
				From:     from,
				To:       to,
				Cc:       cc,
				Subject:  subject,
				Body:     body,
				BodyType: mailpb.BodyType(bodyType),
			},
		},
	}

	err = stream.Send(rqst)
	if err != nil {
		return err
	}

	_, err = stream.CloseAndRecv()

	return err

}
