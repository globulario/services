package mail_client

import (
	"context"
	"log"

	"fmt"
	"io"
	"os"
	"strconv"

	globular "github.com/globulario/services/golang/globular_client"
	"github.com/globulario/services/golang/mail/mailpb"
	"google.golang.org/grpc"
)

////////////////////////////////////////////////////////////////////////////////
// mail Client Service
////////////////////////////////////////////////////////////////////////////////
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
}

// Create a connection to the service.
func NewMailService_Client(address string, id string) (*Mail_Client, error) {
	client := new(Mail_Client)
	err := globular.InitClient(client, address, id)
	if err != nil {
		return nil, err
	}
	client.cc, err = globular.GetClientConnection(client)
	if err != nil {
		return nil, err
	}
	client.c = mailpb.NewMailServiceClient(client.cc)

	return client, nil
}

func (mail_client *Mail_Client) Invoke(method string, rqst interface{}, ctx context.Context) (interface{}, error) {
	if ctx == nil {
		ctx = globular.GetClientContext(mail_client)
	}
	return globular.InvokeClientRequest(mail_client.c, ctx, method, rqst)
}

// Return the domain
func (mail_client *Mail_Client) GetDomain() string {
	return mail_client.domain
}

// Return the address
func (mail_client *Mail_Client) GetAddress() string {
	return mail_client.domain + ":" + strconv.Itoa(mail_client.port)
}

// Return the id of the service instance
func (mail_client *Mail_Client) GetId() string {
	return mail_client.id
}

// Return the name of the service
func (mail_client *Mail_Client) GetName() string {
	return mail_client.name
}

func (mail_client *Mail_Client) GetMac() string {
	return mail_client.mac
}

// must be close when no more needed.
func (mail_client *Mail_Client) Close() {
	mail_client.cc.Close()
}

// Set grpc_service port.
func (mail_client *Mail_Client) SetPort(port int) {
	mail_client.port = port
}

// Set the client name.
func (mail_client *Mail_Client) SetName(name string) {
	mail_client.name = name
}

func (mail_client *Mail_Client) SetMac(mac string) {
	mail_client.mac = mac
}


// Set the service instance id
func (mail_client *Mail_Client) SetId(id string) {
	mail_client.id = id
}

// Set the domain.
func (mail_client *Mail_Client) SetDomain(domain string) {
	mail_client.domain = domain
}

////////////////// TLS ///////////////////

// Get if the client is secure.
func (mail_client *Mail_Client) HasTLS() bool {
	return mail_client.hasTLS
}

// Get the TLS certificate file path
func (mail_client *Mail_Client) GetCertFile() string {
	return mail_client.certFile
}

// Get the TLS key file path
func (mail_client *Mail_Client) GetKeyFile() string {
	return mail_client.keyFile
}

// Get the TLS key file path
func (mail_client *Mail_Client) GetCaFile() string {
	return mail_client.caFile
}

// Set the client is a secure client.
func (mail_client *Mail_Client) SetTLS(hasTls bool) {
	mail_client.hasTLS = hasTls
}

// Set TLS certificate file path
func (mail_client *Mail_Client) SetCertFile(certFile string) {
	mail_client.certFile = certFile
}

// Set TLS key file path
func (mail_client *Mail_Client) SetKeyFile(keyFile string) {
	mail_client.keyFile = keyFile
}

// Set TLS authority trust certificate file path
func (mail_client *Mail_Client) SetCaFile(caFile string) {
	mail_client.caFile = caFile
}

//////////////////////////////// Api ////////////////////////////////

// Stop the service.
func (mail_client *Mail_Client) StopService() {
	mail_client.c.Stop(globular.GetClientContext(mail_client), &mailpb.StopRequest{})
}

/**
 * Create a connection with a mail server.
 */
func (mail_client *Mail_Client) CreateConnection(id string, user string, pwd string, port int, host string) error {
	rqst := &mailpb.CreateConnectionRqst{
		Connection: &mailpb.Connection{
			Id:       id,
			User:     user,
			Password: pwd,
			Port:     int32(port),
			Host:     host,
		},
	}

	_, err := mail_client.c.CreateConnection(globular.GetClientContext(mail_client), rqst)

	return err
}

/**
 * Delete a connection with a mail server.
 */
func (mail_client *Mail_Client) DeleteConnection(id string) error {

	rqst := &mailpb.DeleteConnectionRqst{
		Id: id,
	}
	_, err := mail_client.c.DeleteConnection(globular.GetClientContext(mail_client), rqst)
	return err
}

/**
 * Send email whiout files.
 */
func (mail_client *Mail_Client) SendEmail(id string, from string, to []string, cc []*mailpb.CarbonCopy, subject string, body string, bodyType int32) error {

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

	_, err := mail_client.c.SendEmail(globular.GetClientContext(mail_client), rqst)

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
		log.Fatalf("Fail to open file "+path+" with error: %v", err)
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
 * Test send email whit attachements.
 */
func (mail_client *Mail_Client) SendEmailWithAttachements(id string, from string, to []string, cc []*mailpb.CarbonCopy, subject string, body string, bodyType int32, files []string) error {

	// Open the stream...
	stream, err := mail_client.c.SendEmailWithAttachements(globular.GetClientContext(mail_client))
	if err != nil {
		log.Fatalf("error while TestSendEmailWithAttachements: %v", err)
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
