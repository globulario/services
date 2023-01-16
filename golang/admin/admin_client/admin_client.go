package admin_client

import (
	"bufio"
	"bytes"
	"context"
	"io"
	"io/ioutil"
	"log"
	"runtime"

	//	"log"
	"os"

	"github.com/davecourtois/Utility"
	"github.com/globulario/services/golang/admin/adminpb"
	"github.com/globulario/services/golang/config/config_client"
	"github.com/globulario/services/golang/globular_client"
	globular "github.com/globulario/services/golang/globular_client"
	"github.com/globulario/services/golang/security"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

////////////////////////////////////////////////////////////////////////////////
// admin Client Service
////////////////////////////////////////////////////////////////////////////////

type Admin_Client struct {
	cc *grpc.ClientConn
	c  adminpb.AdminServiceClient

	// The id of the service
	id string

	// The mac address of the server
	mac string

	// The name of the service
	name string

	//  keep the last connection state of the client.
	state string

	// The port
	port int

	// The client domain
	domain string

	// The address where connection with client can be done. ex: globule0.globular.cloud:10101
	address string

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
func NewAdminService_Client(address string, id string) (*Admin_Client, error) {

	client := new(Admin_Client)
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

func (client *Admin_Client) Reconnect() error {
	var err error

	client.cc, err = globular.GetClientConnection(client)
	if err != nil {
		return err
	}

	client.c = adminpb.NewAdminServiceClient(client.cc)
	return nil
}

// The address where the client can connect.
func (client *Admin_Client) SetAddress(address string) {
	client.address = address
}

// Return the configuration from the configuration server.
func (client *Admin_Client) GetConfiguration(address, id string) (map[string]interface{}, error) {
	Utility.RegisterFunction("NewConfigService_Client", config_client.NewConfigService_Client)
	client_, err := globular_client.GetClient(address, "config.ConfigService", "NewConfigService_Client")
	if err != nil {
		return nil, err
	}
	return client_.(*config_client.Config_Client).GetServiceConfiguration(id)
}

func (client *Admin_Client) Invoke(method string, rqst interface{}, ctx context.Context) (interface{}, error) {
	if ctx == nil {
		ctx = client.GetCtx()
	}
	return globular.InvokeClientRequest(client.c, ctx, method, rqst)
}

func (client *Admin_Client) GetCtx() context.Context {
	if client.ctx == nil {
		client.ctx = globular.GetClientContext(client)
	}

	// refresh the client as needed...
	token, err := security.GetLocalToken(client.GetMac())
	if err == nil {
		md := metadata.New(map[string]string{"token": string(token), "domain": client.domain, "mac": client.GetMac()})
		client.ctx = metadata.NewOutgoingContext(context.Background(), md)
	}

	return client.ctx
}

// Return the address
func (client *Admin_Client) GetAddress() string {
	return client.address
}

// Return the domain
func (client *Admin_Client) GetDomain() string {
	return client.domain
}

// Return the id of the service instance
func (client *Admin_Client) GetId() string {
	return client.id
}

// Return the mac address
func (client *Admin_Client) GetMac() string {
	return client.mac
}

// Return the name of the service
func (client *Admin_Client) GetName() string {
	return client.name
}

// Return the last know connection state.
func (client *Admin_Client) GetState() string {
	return client.state
}

// must be close when no more needed.
func (client *Admin_Client) Close() {
	client.cc.Close()
}

// Set grpc_service port.
func (client *Admin_Client) SetPort(port int) {
	client.port = port
}

// Return the grpc port number
func (client *Admin_Client) GetPort() int {
	return client.port
}

// Set the client service instance id.
func (client *Admin_Client) SetId(id string) {
	client.id = id
}

// Set the client name.
func (client *Admin_Client) SetName(name string) {
	client.name = name
}

func (client *Admin_Client) SetState(state string) {
	client.state = state
}

func (client *Admin_Client) SetMac(mac string) {
	client.mac = mac
}

// Set the domain.
func (client *Admin_Client) SetDomain(domain string) {
	client.domain = domain
}

////////////////// TLS ///////////////////

// Get if the client is secure.
func (client *Admin_Client) HasTLS() bool {
	return client.hasTLS
}

// Get the TLS certificate file path
func (client *Admin_Client) GetCertFile() string {
	return client.certFile
}

// Get the TLS key file path
func (client *Admin_Client) GetKeyFile() string {
	return client.keyFile
}

// Get the TLS key file path
func (client *Admin_Client) GetCaFile() string {
	return client.caFile
}

// Set the client is a secure client.
func (client *Admin_Client) SetTLS(hasTls bool) {
	client.hasTLS = hasTls
}

// Set TLS certificate file path
func (client *Admin_Client) SetCertFile(certFile string) {
	client.certFile = certFile
}

// Set TLS key file path
func (client *Admin_Client) SetKeyFile(keyFile string) {
	client.keyFile = keyFile
}

// Set TLS authority trust certificate file path
func (client *Admin_Client) SetCaFile(caFile string) {
	client.caFile = caFile
}

/////////////////////// API /////////////////////

/** Create a service package **/
func (client *Admin_Client) createServicePackage(publisherId string, serviceName string, serviceId string, version string, platform string, servicePath string) (string, error) {
	// Take the information from the configuration...
	id := publisherId + "%" + serviceName + "%" + version + "%" + serviceId + "%" + platform

	// tar + gzip
	var buf bytes.Buffer
	Utility.CompressDir(servicePath, &buf)

	// write the .tar.gzip
	fileToWrite, err := os.OpenFile(os.TempDir()+string(os.PathSeparator)+id+".tar.gz", os.O_CREATE|os.O_RDWR, os.FileMode(0755))
	if err != nil {
		return "", err
	}
	defer fileToWrite.Close()
	if _, err := io.Copy(fileToWrite, &buf); err != nil {
		return "", err
	}

	if err != nil {
		return "", err
	}
	return os.TempDir() + string(os.PathSeparator) + id + ".tar.gz", nil
}

/**
 * Generate the certificates for a given domain. The port is the http port use
 * to get the configuration (80 by default). The path is where the file will be
 * written. The return values are the path to tree certicate path.
 */
func (client *Admin_Client) GetCertificates(address string, port int, path string) (string, string, string, error) {

	rqst := &adminpb.GetCertificatesRequest{
		Domain: address,
		Path:   path,
		Port:   int32(port),
	}

	rsp, err := client.c.GetCertificates(client.GetCtx(), rqst)

	if err != nil {
		return "", "", "", err
	}

	return rsp.Certkey, rsp.Cert, rsp.Cacert, nil
}

/**
 * Push update to a give globular server.
 */
func (client *Admin_Client) Update(path string, platform string, token string, domain string) (int, error) {

	// Set the token into the context and send the request.
	md := metadata.New(map[string]string{"token": string(token), "domain": domain})
	ctx := metadata.NewOutgoingContext(context.Background(), md)

	// Open the stream...
	stream, err := client.c.Update(ctx)
	if err != nil {
		return -1, err
	}

	data, err := os.Open(path)
	if err != nil {
		log.Fatal(err)
	}
	defer data.Close()

	reader := bufio.NewReader(data)

	const BufferSize = 1024 * 5 // the chunck size.
	var size int

	for {
		var data [BufferSize]byte
		bytesread, err := reader.Read(data[0:BufferSize])
		if bytesread > 0 {
			rqst := &adminpb.UpdateRequest{
				Data:     data[0:bytesread],
				Platform: platform,
			}
			// send the data to the server.
			err = stream.Send(rqst)
			if err != nil {
				return -1, err
			}
		}
		size += bytesread
		if err == io.EOF {
			err = nil
			break
		} else if err != nil {
			return -1, err
		}
	}

	_, err = stream.CloseAndRecv()
	if err != nil && err != io.EOF {
		return -1, err
	}

	return size, nil

}

/**
 * Download globular executable from a given source...
 */
func (client *Admin_Client) DownloadGlobular(source, platform, path string) error {

	// Retreive a single value...
	rqst := &adminpb.DownloadGlobularRequest{
		Source:   source,
		Platform: platform,
	}

	stream, err := client.c.DownloadGlobular(client.GetCtx(), rqst)
	if err != nil {
		return err
	}

	// Here I will create the final array
	var buffer bytes.Buffer
	for {
		msg, err := stream.Recv()
		if err == io.EOF {
			// end of stream...
			break
		}

		if err != nil {
			return err
		}

		_, err = buffer.Write(msg.Data)
		if err != nil {
			return err
		}
	}

	Utility.CreateDirIfNotExist(path)

	path += "/Globular"
	if runtime.GOOS == "windows" {
		path += ".exe"
	}

	// The buffer that contain the
	return ioutil.WriteFile(path, buffer.Bytes(), 0755)
}

/**
 * Set environement variable.
 */
func (client *Admin_Client) SetEnvironmentVariable(token, name, value string) error {
	rqst := &adminpb.SetEnvironmentVariableRequest{
		Name:  name,
		Value: value,
	}

	ctx := client.GetCtx()
	if len(token) > 0 {
		md, _ := metadata.FromOutgoingContext(ctx)

		if len(md.Get("token")) != 0 {
			md.Set("token", token)
		}
		ctx = metadata.NewOutgoingContext(context.Background(), md)
	}

	_, err := client.c.SetEnvironmentVariable(ctx, rqst)
	return err

}

// UnsetEnvironement variable.
func (client *Admin_Client) UnsetEnvironmentVariable(token, name string) error {
	rqst := &adminpb.UnsetEnvironmentVariableRequest{
		Name: name,
	}

	ctx := client.GetCtx()
	if len(token) > 0 {
		md, _ := metadata.FromOutgoingContext(ctx)

		if len(md.Get("token")) != 0 {
			md.Set("token", token)
		}
		ctx = metadata.NewOutgoingContext(context.Background(), md)
	}
	_, err := client.c.UnsetEnvironmentVariable(ctx, rqst)
	return err
}

func (client *Admin_Client) GetEnvironmentVariable(token, name string) (string, error) {
	rqst := &adminpb.GetEnvironmentVariableRequest{
		Name: name,
	}

	ctx := client.GetCtx()
	if len(token) > 0 {
		md, _ := metadata.FromOutgoingContext(ctx)

		if len(md.Get("token")) != 0 {
			md.Set("token", token)
		}
		ctx = metadata.NewOutgoingContext(context.Background(), md)
	}
	rsp, err := client.c.GetEnvironmentVariable(ctx, rqst)
	if err != nil {

		return "", err
	}
	return rsp.Value, nil
}

// Run a command.
func (client *Admin_Client) RunCmd(token, cmd, path string, args []string, blocking bool) (string, error) {
	rqst := &adminpb.RunCmdRequest{
		Cmd:      cmd,
		Args:     args,
		Path:     path,
		Blocking: blocking,
	}

	ctx := client.GetCtx()
	if len(token) > 0 {
		md, _ := metadata.FromOutgoingContext(ctx)

		if len(md.Get("token")) != 0 {
			md.Set("token", token)
		}
		ctx = metadata.NewOutgoingContext(context.Background(), md)
	}

	stream, err := client.c.RunCmd(ctx, rqst)
	if err != nil {
		return "", err
	}

	// Here I will create the final array
	var buffer bytes.Buffer
	for {
		msg, err := stream.Recv()
		if err == io.EOF {
			// end of stream...
			break
		}
		if err != nil {
			return "", err
		}

		_, err = buffer.Write([]byte(msg.Result))
		if err != nil {
			return "", err
		}
	}

	return buffer.String(), nil

}

func (client *Admin_Client) KillProcess(token string, pid int) error {
	rqst := &adminpb.KillProcessRequest{
		Pid: int64(pid),
	}

	ctx := client.GetCtx()
	if len(token) > 0 {
		md, _ := metadata.FromOutgoingContext(ctx)

		if len(md.Get("token")) != 0 {
			md.Set("token", token)
		}
		ctx = metadata.NewOutgoingContext(context.Background(), md)
	}

	_, err := client.c.KillProcess(ctx, rqst)
	return err
}

func (client *Admin_Client) KillProcesses(token string, name string) error {
	rqst := &adminpb.KillProcessesRequest{
		Name: name,
	}

	ctx := client.GetCtx()
	if len(token) > 0 {
		md, _ := metadata.FromOutgoingContext(ctx)

		if len(md.Get("token")) != 0 {
			md.Set("token", token)
		}
		ctx = metadata.NewOutgoingContext(context.Background(), md)
	}

	_, err := client.c.KillProcesses(ctx, rqst)
	return err
}

func (client *Admin_Client) GetPids(token string, name string) ([]int32, error) {
	rqst := &adminpb.GetPidsRequest{
		Name: name,
	}
	ctx := client.GetCtx()
	if len(token) > 0 {
		md, _ := metadata.FromOutgoingContext(ctx)

		if len(md.Get("token")) != 0 {
			md.Set("token", token)
		}
		ctx = metadata.NewOutgoingContext(context.Background(), md)
	}

	rsp, err := client.c.GetPids(ctx, rqst)
	if err != nil {
		return nil, err
	}
	return rsp.Pids, err
}
