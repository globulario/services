package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/anacrolix/torrent"
	"github.com/davecourtois/Utility"
	globular "github.com/globulario/services/golang/globular_service"
	"github.com/globulario/services/golang/torrent/torrent_client"
	"github.com/globulario/services/golang/torrent/torrentpb"

	//"github.com/globulario/services/golang/interceptors"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"

	//"google.golang.org/grpc/grpclog"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"
)

// The default values.
var (
	defaultPort  = 10029
	defaultProxy = 10030

	// By default all origins are allowed.
	allow_all_origins = true

	// comma separeated values.
	allowed_origins string = ""

	domain string = "localhost"
)

// Value need by Globular to start the services...
type server struct {
	// The global attribute of the services.
	Id              string
	Mac             string
	Name            string
	Domain          string
	Address         string
	Path            string
	Proto           string
	Port            int
	Proxy           int
	AllowAllOrigins bool
	AllowedOrigins  string // comma separated string.
	Protocol        string
	Version         string
	PublisherId     string
	KeepUpToDate    bool
	KeepAlive       bool
	Description     string
	Keywords        []string
	Repositories    []string
	Discoveries     []string
	Process         int
	ProxyProcess    int
	ConfigPath      string
	LastError       string
	State           string
	ModTime         int64

	TLS bool

	// svr-signed X.509 public keys for distribution
	CertFile string

	// a private RSA key to sign and authenticate the public key
	KeyFile string

	// a private RSA key to sign and authenticate the public key
	CertAuthorityTrust string

	Permissions []interface{} // contains the action permission for the services.

	Dependencies []string // The list of services needed by this services.

	DownloadDir string // where the files will be downloaded...

	Seed bool // if true torrent will be seeded...

	// The grpc server.
	grpcServer *grpc.Server

	// The torrent client.
	torrent_client_ *torrent.Client

	// The actions channel
	actions chan map[string]interface{}

	// The ticker that get track of downloads...
	ticker *time.Ticker

	// The done channel to exit loop...
	done chan bool
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

// The path of the configuration.
func (svr *server) GetConfigurationPath() string {
	return svr.ConfigPath
}

func (svr *server) SetConfigurationPath(path string) {
	svr.ConfigPath = path
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
	Utility.RegisterFunction("NewtorrentService_Client", torrent_client.NewTorrentService_Client)

	// Get the configuration path.
	err := globular.InitService(svr)
	if err != nil {
		return err
	}

	// Initialyse GRPC server.
	svr.grpcServer, err = globular.InitGrpcServer(svr /*interceptors.ServerUnaryInterceptor, interceptors.ServerStreamIntercepto*/, nil, nil)
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

func IsUrl(str string) bool {
	u, err := url.Parse(str)
	return err == nil && u.Scheme != "" && u.Host != ""
}

// Keep track where to copy the torrent files when transfert is completed...
type TorrentTransfer struct {
	dst string
	tor *torrent.Torrent
}

/**
 * Manage torrents here...
 */
func (svr *server) processTorrent() {

	pending := make([]*TorrentTransfer, 0)

	// Update the torrent information each second...
	ticker := time.NewTicker(1 * time.Second)
	previousInfos := make(map[string]*torrentpb.TorrentInfo)

	go func() {
		for {
			select {
			case a := <-svr.actions:
				if a["action"] == "setTorrentTransfer" {
					t := a["torrentTransfer"].(*TorrentTransfer)
					pending = append(pending, t)
				} else if a["action"] == "getTorrentsInfo" {
					// get back current torrent infos...
					infos := make([]*torrentpb.TorrentInfo, 0)
					for i := 0; i < len(pending); i++ {
						// get the previous information to calculate the rate...
						previousInfo := previousInfos[pending[i].tor.Name()]
						var info *torrentpb.TorrentInfo
						if previousInfo != nil {
							info = getTorrentInfo(pending[i].tor, previousInfo.UpdatedAt, previousInfo.Downloaded)
						} else {
							info = getTorrentInfo(pending[i].tor, 0, 0)
						}
						
						info.Destination = pending[i].dst
						infos = append(infos, info)
					}
					a["infos"].(chan []*torrentpb.TorrentInfo) <- infos
				} else if a["action"] == "dropTorrent" {
					pending_ := make([]*TorrentTransfer, 0)
					for i := 0; i < len(pending); i++ {
						p := pending_[i]
						if p.tor.Name() == a["name"].(string) {
							p.tor.Drop()
							os.RemoveAll(svr.DownloadDir + "/" + p.tor.Name())
							delete(previousInfos, p.tor.Name())
						} else {
							pending_ = append(pending_, p)
						}
					}

					// Set the pending
					pending = pending_
				}

			case <-ticker.C:
				for i := 0; i < len(pending); i++ {
					previousInfo := previousInfos[pending[i].tor.Name()]
					var info *torrentpb.TorrentInfo
					if previousInfo != nil {
						info = getTorrentInfo(pending[i].tor, previousInfo.UpdatedAt, previousInfo.Downloaded)
					} else {
						info = getTorrentInfo(pending[i].tor, 0, 0)
					}
					previousInfos[pending[i].tor.Name()] = info
					if info.Downloaded == info.Size {
						// The torrent is completed...
						// so I will copy the files to destination...
						for j := 0; j < len(info.Files); j++ {
							src := svr.DownloadDir + "/" + info.Files[j].Path
							dst := pending[i].dst + "/" + info.Files[j].Path
							if Utility.Exists(src) && !Utility.Exists(dst) {
								fmt.Println("copy file ", src, " to ", dst)
								// Create the dir...
								Utility.CreateDirIfNotExist(dst[0:strings.LastIndex(dst, "/")])
								err := Utility.CopyFile(src, dst)
								if err != nil {
									fmt.Println("fail to copy torrent file with error ", err)
								}
							}
						}
					} else {
						fmt.Println(info.Name, info.Downloaded, " of ", info.Size, " or ", info.Percent, "% rate ", info.DownloadRate)
					}
				}

			case <-svr.done:
				// Stop the torrent client
				svr.torrent_client_.Close()
				return // exit the loop...
			}
		}
	}()
}

/**
 * Download file to a given folder...
 */
func downloadFile(file_url, dest string) (string, error) {

	// Build fileName from fullPath
	fileURL, err := url.Parse(file_url)
	if err != nil {
		return "", err
	}

	path := fileURL.Path
	segments := strings.Split(path, "/")
	fileName := dest + "/" + segments[len(segments)-1]

	// Create blank file
	file, err := os.Create(fileName)
	if err != nil {
		return "", err
	}

	client := http.Client{
		CheckRedirect: func(r *http.Request, via []*http.Request) error {
			r.URL.Opaque = r.URL.Path
			return nil
		},
	}

	// Put content on file
	resp, err := client.Get(file_url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	size, err := io.Copy(file, resp.Body)
	if err != nil {
		return "", err
	}

	defer file.Close()

	fmt.Printf("Downloaded a file %s with size %d", fileName, size)

	return "", nil

}

func percent(actual, total int64) float64 {
	return float64(actual)/float64(total) * 100
}

func getTorrentInfo(t *torrent.Torrent, updatedAt, downloaded int64) *torrentpb.TorrentInfo {

	torrentInfo := new(torrentpb.TorrentInfo)
	torrentInfo.Name = t.Name()
	torrentInfo.Loaded = t.Info() != nil
	if torrentInfo.Loaded {
		torrentInfo.Size = t.Length()
	}

	totalChunks := int64(0)
	totalCompleted := int64(0)
	<-t.GotInfo()

	tfiles := t.Files()
	if len(tfiles) > 0 && torrentInfo.Files == nil {
		torrentInfo.Files = make([]*torrentpb.TorrentFileInfo, len(tfiles))
	}
	//merge in files
	for i, f := range tfiles {
		path := f.Path()
		file := torrentInfo.Files[i]
		if file == nil {
			file = &torrentpb.TorrentFileInfo{Path: path}
			torrentInfo.Files[i] = file
		}
		chunks := f.State()

		file.Size = f.Length()
		file.Chunks = int64(len(chunks))
		completed := 0
		for _, p := range chunks {
			if p.Complete {
				completed++
			}
		}
		file.Completed = int64(completed)
		file.Percent = percent(file.Completed, file.Chunks)

		totalChunks += file.Chunks
		totalCompleted += file.Completed
	}

	//cacluate rate
	now := time.Now()

	if updatedAt != 0 {
		dt := float32(now.Sub(time.Unix(updatedAt, 0)))
		db := float32(t.BytesCompleted() - downloaded)
		rate := db * (float32(time.Second) / dt)
		if rate >= 0 {
			torrentInfo.DownloadRate = rate
		}
	}

	torrentInfo.Downloaded = t.BytesCompleted()
	torrentInfo.UpdatedAt = now.Unix()
	
	torrentInfo.Percent = percent(torrentInfo.Downloaded, torrentInfo.Size)

	return torrentInfo
}

/////////////////////// torrent specific function /////////////////////////////////

// Set the torrent files... the torrent will be download in the
// DownloadDir and moved to it destination when done.
func (svr *server) setTorrentTransfer(t *torrent.Torrent, dest string) {
	a := make(map[string]interface{})
	a["action"] = "setTorrentTransfer"
	a["torrentTransfer"] = &TorrentTransfer{dst: dest, tor: t}

	// Set the action.
	svr.actions <- a
}

func (svr *server) dropTorrent(name string) {
	a := make(map[string]interface{})
	a["action"] = "dropTorrent"
	a["name"] = name

	// Set the action.
	svr.actions <- a
}

// get torrents infos...
func (svr *server) getTorrentsInfo() []*torrentpb.TorrentInfo {

	// Return the torrent infos...
	a := make(map[string]interface{})
	a["action"] = "getTorrentsInfo"
	a["infos"] = make(chan []*torrentpb.TorrentInfo)

	// Set the action.
	svr.actions <- a

	// wait for the result and return
	return <-a["infos"].(chan []*torrentpb.TorrentInfo)
}

// NewClient creates a new torrent client based on a magnet or a torrent file.
// If the torrent file is on http, we try downloading it.
func (svr *server) downloadTorrent(link, dest string, seed bool) error {
	var t *torrent.Torrent
	var err error

	// Add as magnet url.
	if strings.HasPrefix(link, "magnet:") {
		if t, err = svr.torrent_client_.AddMagnet(link); err != nil {
			return err
		}
	} else {
		// Otherwise add as a torrent file.

		// If it's online, we try downloading the file.
		if IsUrl(link) {
			if link, err = downloadFile(link, dest); err != nil {
				return err
			}
		}

		// Check if the file exists.
		if _, err = os.Stat(link); err != nil {
			return err
		}

		if t, err = svr.torrent_client_.AddTorrentFromFile(link); err != nil {
			return err
		}
	}

	// Start download the torrent...
	go func() {
		// Wait to get the info...
		<-t.GotInfo()

		// Start download...
		t.DownloadAll()

		// Set the torrent
		svr.setTorrentTransfer(t, dest)

	}()

	return nil
}

//* Return all torrent info... *
func (svr *server) GetTorrentInfos(ctx context.Context, rqst *torrentpb.GetTorrentInfosRequest) (*torrentpb.GetTorrentInfosResponse, error) {

	// I will get all torrents from all clients...
	infos := svr.getTorrentsInfo()

	return &torrentpb.GetTorrentInfosResponse{Infos: infos}, nil
}

//* Download a torrent file
func (svr *server) DownloadTorrent(ctx context.Context, rqst *torrentpb.DownloadTorrentRequest) (*torrentpb.DownloadTorrentResponse, error) {

	// So here I will create a new client...
	err := svr.downloadTorrent(rqst.Link, rqst.Dest, rqst.Seed)

	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &torrentpb.DownloadTorrentResponse{}, nil
}

//* Trop the torrent...
func (svr *server) DropTorrent(ctx context.Context, rqst *torrentpb.DropTorrentRequest) (*torrentpb.DropTorrentResponse, error) {

	// simply trop the torrent file...
	svr.dropTorrent(rqst.Name)

	return &torrentpb.DropTorrentResponse{}, nil
}

// That service is use to give access to SQL.
// port number must be pass as argument.
func main() {

	// set the logger.
	//grpclog.SetLogger(log.New(os.Stdout, "torrent_service: ", log.LstdFlags))

	// Set the log information in case of crash...
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	// Initialyse service with default values.
	s_impl := new(server)
	s_impl.Name = string(torrentpb.File_torrent_proto.Services().Get(0).FullName())
	s_impl.Proto = torrentpb.File_torrent_proto.Path()
	s_impl.Port = defaultPort
	s_impl.Proxy = defaultProxy
	s_impl.Protocol = "grpc"
	s_impl.Domain = domain
	s_impl.Version = "0.0.1"
	s_impl.PublisherId = "globulario"
	s_impl.Description = "The Hello world of gRPC service!"
	s_impl.Keywords = []string{"Example", "torrent", "Test", "Service"}
	s_impl.Repositories = make([]string, 0)
	s_impl.Discoveries = make([]string, 0)
	s_impl.Dependencies = make([]string, 0)
	s_impl.Permissions = make([]interface{}, 0)
	s_impl.Process = -1
	s_impl.ProxyProcess = -1
	s_impl.AllowAllOrigins = allow_all_origins
	s_impl.AllowedOrigins = allowed_origins
	s_impl.KeepAlive = true
	s_impl.DownloadDir = os.TempDir()
	s_impl.Seed = false

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
		log.Fatalf("fail to initialyse service %s: %s", s_impl.Name, s_impl.Id)
	}

	// Register the torrent services
	torrentpb.RegisterTorrentServiceServer(s_impl.grpcServer, s_impl)
	reflection.Register(s_impl.grpcServer)

	// Create client.
	config := torrent.NewDefaultClientConfig()
	config.DataDir = s_impl.DownloadDir
	config.Seed = s_impl.Seed

	// The actions
	s_impl.actions = make(chan map[string]interface{})

	// When the music over turn off the ligth...
	s_impl.done = make(chan bool)

	// One client per directory...
	s_impl.torrent_client_, err = torrent.NewClient(config)
	if err != nil {
		fmt.Println("fail to start torrent client with error ", err)
		return
	}

	// Start process torrent...
	s_impl.processTorrent()

	// Start the service.
	s_impl.StartService()

}