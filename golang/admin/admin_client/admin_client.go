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
	"strconv"

	"github.com/davecourtois/Utility"
	"github.com/globulario/services/golang/admin/adminpb"
	globular "github.com/globulario/services/golang/globular_client"

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

	// The port
	port int

	// The client domain
	domain string

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
func NewAdminService_Client(address string, id string) (*Admin_Client, error) {

	client := new(Admin_Client)
	err := globular.InitClient(client, address, id)
	if err != nil {
		return nil, err
	}

	client.cc, err = globular.GetClientConnection(client)
	if err != nil {
		return nil, err
	}

	client.c = adminpb.NewAdminServiceClient(client.cc)

	return client, nil
}

func (admin_client *Admin_Client) Invoke(method string, rqst interface{}, ctx context.Context) (interface{}, error) {
	if ctx == nil {
		ctx = globular.GetClientContext(admin_client)
	}
	return globular.InvokeClientRequest(admin_client.c, ctx, method, rqst)
}

// Return the address
func (admin_client *Admin_Client) GetAddress() string {
	return admin_client.domain + ":" + strconv.Itoa(admin_client.port)
}

// Return the domain
func (admin_client *Admin_Client) GetDomain() string {
	return admin_client.domain
}

// Return the id of the service instance
func (admin_client *Admin_Client) GetId() string {
	return admin_client.id
}

// Return the mac address
func (admin_client *Admin_Client) GetMac() string {
	return admin_client.mac
}

// Return the name of the service
func (admin_client *Admin_Client) GetName() string {
	return admin_client.name
}

// must be close when no more needed.
func (admin_client *Admin_Client) Close() {
	admin_client.cc.Close()
}

// Set grpc_service port.
func (admin_client *Admin_Client) SetPort(port int) {
	admin_client.port = port
}

// Set the client service instance id.
func (admin_client *Admin_Client) SetId(id string) {
	admin_client.id = id
}

// Set the client name.
func (admin_client *Admin_Client) SetName(name string) {
	admin_client.name = name
}

func (admin_client *Admin_Client) SetMac(mac string) {
	admin_client.mac = mac
}

// Set the domain.
func (admin_client *Admin_Client) SetDomain(domain string) {
	admin_client.domain = domain
}

////////////////// TLS ///////////////////

// Get if the client is secure.
func (admin_client *Admin_Client) HasTLS() bool {
	return admin_client.hasTLS
}

// Get the TLS certificate file path
func (admin_client *Admin_Client) GetCertFile() string {
	return admin_client.certFile
}

// Get the TLS key file path
func (admin_client *Admin_Client) GetKeyFile() string {
	return admin_client.keyFile
}

// Get the TLS key file path
func (admin_client *Admin_Client) GetCaFile() string {
	return admin_client.caFile
}

// Set the client is a secure client.
func (admin_client *Admin_Client) SetTLS(hasTls bool) {
	admin_client.hasTLS = hasTls
}

// Set TLS certificate file path
func (admin_client *Admin_Client) SetCertFile(certFile string) {
	admin_client.certFile = certFile
}

// Set TLS key file path
func (admin_client *Admin_Client) SetKeyFile(keyFile string) {
	admin_client.keyFile = keyFile
}

// Set TLS authority trust certificate file path
func (admin_client *Admin_Client) SetCaFile(caFile string) {
	admin_client.caFile = caFile
}

/////////////////////// API /////////////////////

/** Create a service package **/
func (admin_client *Admin_Client) createServicePackage(publisherId string, serviceName string, serviceId string, version string, platform string, servicePath string) (string, error) {
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

	if _, err := io.Copy(fileToWrite, &buf); err != nil {
		return "", err
	}

	// close the file.
	fileToWrite.Close()

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
func (admin_client *Admin_Client) GetCertificates(domain string, port int, path string) (string, string, string, error) {

	rqst := &adminpb.GetCertificatesRequest{
		Domain: domain,
		Path:   path,
		Port:   int32(port),
	}

	rsp, err := admin_client.c.GetCertificates(globular.GetClientContext(admin_client), rqst)

	if err != nil {
		return "", "", "", err
	}

	return rsp.Certkey, rsp.Cert, rsp.Cacert, nil
}

/**
 * Push update to a give globular server.
 */
func (admin_client *Admin_Client) Update(path string, platform string, token string, domain string) (int, error) {

	// Set the token into the context and send the request.
	md := metadata.New(map[string]string{"token": string(token), "domain": domain})
	ctx := metadata.NewOutgoingContext(context.Background(), md)

	// Open the stream...
	stream, err := admin_client.c.Update(ctx)
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
func (admin_client *Admin_Client) DownloadGlobular(source, platform, path string) error {

	// Retreive a single value...
	rqst := &adminpb.DownloadGlobularRequest{
		Source:   source,
		Platform: platform,
	}

	stream, err := admin_client.c.DownloadGlobular(globular.GetClientContext(admin_client), rqst)
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
func (admin_client *Admin_Client) SetEnvironmentVariable(token, name, value string) error {
	rqst := &adminpb.SetEnvironmentVariableRequest{
		Name:  name,
		Value: value,
	}

	ctx := globular.GetClientContext(admin_client)
	if len(token) > 0 {
		md, _ := metadata.FromOutgoingContext(ctx)

		if len(md.Get("token")) != 0 {
			md.Set("token", token)
		}
		ctx = metadata.NewOutgoingContext(context.Background(), md)
	}

	_, err := admin_client.c.SetEnvironmentVariable(ctx, rqst)
	return err

}

// UnsetEnvironement variable.
func (admin_client *Admin_Client) UnsetEnvironmentVariable(token, name string) error {
	rqst := &adminpb.UnsetEnvironmentVariableRequest{
		Name: name,
	}

	ctx := globular.GetClientContext(admin_client)
	if len(token) > 0 {
		md, _ := metadata.FromOutgoingContext(ctx)

		if len(md.Get("token")) != 0 {
			md.Set("token", token)
		}
		ctx = metadata.NewOutgoingContext(context.Background(), md)
	}
	_, err := admin_client.c.UnsetEnvironmentVariable(ctx, rqst)
	return err
}

func (admin_client *Admin_Client) GetEnvironmentVariable(token, name string) (string, error) {
	rqst := &adminpb.GetEnvironmentVariableRequest{
		Name: name,
	}

	ctx := globular.GetClientContext(admin_client)
	if len(token) > 0 {
		md, _ := metadata.FromOutgoingContext(ctx)

		if len(md.Get("token")) != 0 {
			md.Set("token", token)
		}
		ctx = metadata.NewOutgoingContext(context.Background(), md)
	}
	rsp, err := admin_client.c.GetEnvironmentVariable(ctx, rqst)
	if err != nil {

		return "", err
	}
	return rsp.Value, nil
}

// Run a command.
func (admin_client *Admin_Client) RunCmd(token, cmd string, args []string, blocking bool) (string, error) {
	rqst := &adminpb.RunCmdRequest{
		Cmd:      cmd,
		Args:     args,
		Blocking: blocking,
	}

	ctx := globular.GetClientContext(admin_client)
	if len(token) > 0 {
		md, _ := metadata.FromOutgoingContext(ctx)

		if len(md.Get("token")) != 0 {
			md.Set("token", token)
		}
		ctx = metadata.NewOutgoingContext(context.Background(), md)
	}

	stream, err := admin_client.c.RunCmd(ctx, rqst)
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

func (admin_client *Admin_Client) KillProcess(token string, pid int) error {
	rqst := &adminpb.KillProcessRequest{
		Pid: int64(pid),
	}

	ctx := globular.GetClientContext(admin_client)
	if len(token) > 0 {
		md, _ := metadata.FromOutgoingContext(ctx)

		if len(md.Get("token")) != 0 {
			md.Set("token", token)
		}
		ctx = metadata.NewOutgoingContext(context.Background(), md)
	}

	_, err := admin_client.c.KillProcess(ctx, rqst)
	return err
}

func (admin_client *Admin_Client) KillProcesses(token string, name string) error {
	rqst := &adminpb.KillProcessesRequest{
		Name: name,
	}

	ctx := globular.GetClientContext(admin_client)
	if len(token) > 0 {
		md, _ := metadata.FromOutgoingContext(ctx)

		if len(md.Get("token")) != 0 {
			md.Set("token", token)
		}
		ctx = metadata.NewOutgoingContext(context.Background(), md)
	}

	_, err := admin_client.c.KillProcesses(ctx, rqst)
	return err
}

func (admin_client *Admin_Client) GetPids(token string, name string) ([]int32, error) {
	rqst := &adminpb.GetPidsRequest{
		Name: name,
	}
	ctx := globular.GetClientContext(admin_client)
	if len(token) > 0 {
		md, _ := metadata.FromOutgoingContext(ctx)

		if len(md.Get("token")) != 0 {
			md.Set("token", token)
		}
		ctx = metadata.NewOutgoingContext(context.Background(), md)
	}

	rsp, err := admin_client.c.GetPids(ctx, rqst)
	if err != nil {
		return nil, err
	}
	return rsp.Pids, err
}
