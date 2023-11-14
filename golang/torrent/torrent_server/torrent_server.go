package main

import (
	"context"
	"encoding/gob"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/anacrolix/torrent"
	"github.com/davecourtois/Utility"
	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/event/event_client"
	"github.com/globulario/services/golang/globular_client"
	globular "github.com/globulario/services/golang/globular_service"
	"github.com/globulario/services/golang/interceptors"
	"github.com/globulario/services/golang/rbac/rbac_client"
	"github.com/globulario/services/golang/rbac/rbacpb"
	"github.com/globulario/services/golang/security"
	"github.com/globulario/services/golang/torrent/torrentpb"

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
	Plaform         string
	Checksum        string
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

	// The done channel to exit loop...
	done chan bool
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

	// Get the configuration path.
	err := globular.InitService(srv)
	if err != nil {
		return err
	}

	// Initialyse GRPC srv.
	srv.grpcServer, err = globular.InitGrpcServer(srv, interceptors.ServerUnaryInterceptor, interceptors.ServerStreamInterceptor)
	if err != nil {
		return err
	}

	if srv.Address == "" {
		srv.Address, _ = config.GetAddress()
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

func IsUrl(str string) bool {
	u, err := url.Parse(str)
	return err == nil && u.Scheme != "" && u.Host != ""
}

// Keep track where to copy the torrent files when transfert is completed...
type TorrentTransfer struct {
	dst   string
	lnk   string
	seed  bool
	owner string
	tor   *torrent.Torrent
}

// Keep track of torrent dowload...
type TorrentLnk struct {
	Name  string // The name of the torrent.
	Dir   string // The dir where to copy the torrent when the download is complete.
	Lnk   string // Can be a file path or a Url...
	Seed  bool   // if true the link will be seed...
	Owner string // the owner of this torrent
}

func (srv *server) saveTorrentLnks(lnks []TorrentLnk) error {
	// create a file
	dataFile, err := os.Create(srv.DownloadDir + "/lnks.gob")
	if err != nil {
		fmt.Println(err)
		return err
	}
	defer dataFile.Close()

	// serialize the data
	dataEncoder := gob.NewEncoder(dataFile)
	dataEncoder.Encode(lnks)

	return nil
}

/**
 * Read previous link's
 */
func (srv *server) readTorrentLnks() ([]TorrentLnk, error) {

	// open data file
	dataFile, err := os.Open(srv.DownloadDir + "/lnks.gob")
	lnks := make([]TorrentLnk, 0)

	if err != nil {
		return lnks, err
	}
	defer dataFile.Close()

	dataDecoder := gob.NewDecoder(dataFile)
	err = dataDecoder.Decode(&lnks)

	if err != nil {
		fmt.Println("readTorrentLnks: decode error...", err)
		return lnks, err
	}

	return lnks, err
}

//////////////////////// RBAC function //////////////////////////////////////////////
/**
 * Get the rbac client.
 */
func GetRbacClient(address string) (*rbac_client.Rbac_Client, error) {
	Utility.RegisterFunction("NewRbacService_Client", rbac_client.NewRbacService_Client)
	client, err := globular_client.GetClient(address, "rbac.RbacService", "NewRbacService_Client")
	if err != nil {
		return nil, err
	}
	return client.(*rbac_client.Rbac_Client), nil
}

func (srv *server) addResourceOwner(path, resourceType, subject string, subjectType rbacpb.SubjectType) error {
	rbac_client_, err := GetRbacClient(srv.Address)
	if err != nil {
		return err
	}
	return rbac_client_.AddResourceOwner(path, resourceType, subject, subjectType)
}

func getEventClient() (*event_client.Event_Client, error) {
	address, _ := config.GetAddress()
	Utility.RegisterFunction("NewEventService_Client", event_client.NewEventService_Client)
	client, err := globular_client.GetClient(address, "event.EventService", "NewEventService_Client")
	if err != nil {
		return nil, err
	}
	return client.(*event_client.Event_Client), nil
}

/**
 * Manage torrents here...
 */

func (srv *server) processTorrent() {

	pending := make([]*TorrentTransfer, 0)

	// Update the torrent information each second...
	infos := make(map[string]*torrentpb.TorrentInfo)

	// Client streams...
	ticker := time.NewTicker(1 * time.Second)

	// Here I will keep the open streams and the bool chan to exit...
	getTorrentsInfo_actions := make([]map[string]interface{}, 0)

	go func() {
		for {
			select {
			case a := <-srv.actions:
				if a["action"] == "setTorrentTransfer" {
					t := a["torrentTransfer"].(*TorrentTransfer)
					pending = append(pending, t)

					// Keep the link...
					lnks, err := srv.readTorrentLnks()
					exist := false
					if err != nil {
						lnks = make([]TorrentLnk, 0)
					} else {
						for _, lnk := range lnks {
							if lnk.Lnk == t.lnk {
								exist = true
								break
							}
						}
					}

					if !exist {
						lnks = append(lnks, TorrentLnk{Dir: t.dst, Lnk: t.lnk, Seed: t.seed, Name: t.tor.Name(), Owner: t.owner})
						err := srv.saveTorrentLnks(lnks)
						if err != nil {
							fmt.Println("fail to save torrent lnks with error ", err)
						}
					}

				} else if a["action"] == "getTorrentsInfo" {
					getTorrentsInfo_actions = append(getTorrentsInfo_actions, a)
				} else if a["action"] == "dropTorrent" {
					// remove it from the map...
					delete(infos, a["name"].(string))
					pending_ := make([]*TorrentTransfer, 0)
					for _, p := range pending {
						if p.tor.Name() == a["name"].(string) {
							p.tor.Drop()
							os.RemoveAll(srv.DownloadDir + "/" + p.tor.Name())
						} else {
							pending_ = append(pending_, p)
						}
					}

					// Set the pending
					pending = pending_

					// I will remove the lnk from the list...
					lnks, err := srv.readTorrentLnks()
					if err == nil {
						lnks_ := make([]TorrentLnk, 0)
						for _, lnk := range lnks {
							if lnk.Name != a["name"].(string) {
								lnks_ = append(lnks_, lnk)
							}
						}
						srv.saveTorrentLnks(lnks_)
					}

				} else if a["action"] == "getTorrentLnks" {
					lnks, err := srv.readTorrentLnks()
					lnks_ := make([]*torrentpb.TorrentLnk, 0)

					if err == nil {
						for i := 0; i < len(lnks); i++ {
							lnk := lnks[i]
							lnks_ = append(lnks_, &torrentpb.TorrentLnk{Name: lnk.Name, Lnk: lnk.Lnk, Dest: lnk.Dir, Seed: lnk.Seed, Owner: lnk.Owner})
						}
					}
					// return the list of link's in the channel.
					a["lnks"].(chan []*torrentpb.TorrentLnk) <- lnks_

				}
			case <-ticker.C:
				// get back current torrent infos...
				infos_ := make([]*torrentpb.TorrentInfo, 0)
				for i := 0; i < len(pending); i++ {
					infos[pending[i].tor.Name()] = getTorrentInfo(pending[i].tor, infos[pending[i].tor.Name()])
					infos[pending[i].tor.Name()].Destination = pending[i].dst
					infos_ = append(infos_, infos[pending[i].tor.Name()])
					for _, file := range infos[pending[i].tor.Name()].Files {
						// I will copy files when they are completes...
						if file.Percent == 100 {
							src := srv.DownloadDir + "/" + file.Path
							dst := pending[i].dst + "/" + file.Path
							if Utility.Exists(src) && !Utility.Exists(dst) {

								// Create the dir...
								dir := filepath.Dir(dst)
								err := Utility.CreateDirIfNotExist(dir)
								if err == nil {
									if strings.Contains(dir, "/files/users/") {
										dir = dir[strings.Index(dir, "/users/"):]
									}
									// add owner to the directory itself.
									srv.addResourceOwner(dir, "file", pending[i].owner, rbacpb.SubjectType_ACCOUNT)
									// so here the dir will be the parent of that dir
									dir = filepath.Dir(dir) // this will be the dir to reload...
								}

								err = Utility.CopyFile(src, dst)

								if err != nil {
									fmt.Println("fail to copy torrent file with error ", err)
								} else {
									srv.addResourceOwner(dst, "file", pending[i].owner, rbacpb.SubjectType_ACCOUNT)

									// publish reload dir event.
									// force client to reload their informations.
									event_client, err := getEventClient()
									if err == nil {
										event_client.Publish("reload_dir_event", []byte(dir))
									}
								}
							}
						}
					}
				}

				// Now I will send info_ to connected client...
				for index := 0; index < len(getTorrentsInfo_actions); index++ {
					action := getTorrentsInfo_actions[index]
					stream := action["stream"].(torrentpb.TorrentService_GetTorrentInfosServer)
					err := stream.Send(&torrentpb.GetTorrentInfosResponse{Infos: infos_})
					if err != nil {
						fmt.Println("exit torrent  ", err)
						action["exit"].(chan bool) <- true
						getTorrentsInfo_actions = append(getTorrentsInfo_actions[:index], getTorrentsInfo_actions[index+1:]...)
					}
				}

			case <-srv.done:

				// Stop the torrent client
				srv.torrent_client_.Close()

				// Close the action...
				for _, action := range getTorrentsInfo_actions {
					action["exit"].(chan bool) <- true
				}

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
	return float64(actual) / float64(total) * 100
}

func getTorrentInfo(t *torrent.Torrent, torrentInfo *torrentpb.TorrentInfo) *torrentpb.TorrentInfo {

	var updatedAt, downloaded int64
	if torrentInfo == nil {
		torrentInfo = new(torrentpb.TorrentInfo)
		torrentInfo.Name = t.Name()
		torrentInfo.Loaded = t.Info() != nil
		if torrentInfo.Loaded {
			torrentInfo.Size = t.Length()
		}
		// return the values without file informations at first...
		go func() {
			<-t.GotInfo()
			torrentInfo.Files = make([]*torrentpb.TorrentFileInfo, len(t.Files()))
		}()
	} else {
		// get previous values.
		updatedAt = torrentInfo.UpdatedAt
		downloaded = torrentInfo.Downloaded
	}

	// This will be append when the file will be initialyse...
	if torrentInfo.Files != nil {
		for i, f := range t.Files() {
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
	}

	return torrentInfo
}

/////////////////////// torrent specific function /////////////////////////////////

// Set the torrent files... the torrent will be download in the
// DownloadDir and moved to it destination when done.
func (srv *server) setTorrentTransfer(t *torrent.Torrent, seed bool, lnk, dest string, owner string) {

	a := make(map[string]interface{})
	a["action"] = "setTorrentTransfer"
	a["torrentTransfer"] = &TorrentTransfer{dst: dest, lnk: lnk, tor: t, seed: seed, owner: owner}

	// Set the action.
	srv.actions <- a
}

func (srv *server) dropTorrent(name string) {

	a := make(map[string]interface{})
	a["action"] = "dropTorrent"
	a["name"] = name

	// Set the action.
	srv.actions <- a
}

// get torrents infos...
func (srv *server) getTorrentsInfo(stream torrentpb.TorrentService_GetTorrentInfosServer) chan bool {

	// Return the torrent infos...
	a := make(map[string]interface{})
	a["action"] = "getTorrentsInfo"
	a["stream"] = stream

	exit := make(chan bool)

	a["exit"] = exit

	// Set the action.
	srv.actions <- a

	// wait for the result and return
	return exit // that channel will be call when the stream will be unavailable...
}

// get torrents infos...
func (srv *server) getTorrentLnks() []*torrentpb.TorrentLnk {

	// Return the torrent infos...
	a := make(map[string]interface{})
	a["action"] = "getTorrentLnks"

	// set the output channel for the retreived link's
	a["lnks"] = make(chan []*torrentpb.TorrentLnk)

	// Set the action and wait for it result...
	srv.actions <- a

	// read the lnks...
	lnks := <-a["lnks"].(chan []*torrentpb.TorrentLnk) // that channel will be call when the stream will be unavailable...

	// wait for the result and return
	return lnks
}

// NewClient creates a new torrent client based on a magnet or a torrent file.
// If the torrent file is on http, we try downloading it.
func (srv *server) downloadTorrent(link, dest string, seed bool, owner string) error {
	var t *torrent.Torrent
	var err error

	// Add as magnet url.
	if strings.HasPrefix(link, "magnet:") {
		if t, err = srv.torrent_client_.AddMagnet(link); err != nil {
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

		if t, err = srv.torrent_client_.AddTorrentFromFile(link); err != nil {
			return err
		}
	}

	// Start download the torrent...
	go func() {
		// Wait to get the info...
		<-t.GotInfo()

		// Start download...
		t.DownloadAll()

		srv.setTorrentTransfer(t, seed, link, dest, owner)

	}()

	return nil
}

// * Get the torrent links
func (srv *server) GetTorrentLnks(ctx context.Context, rqst *torrentpb.GetTorrentLnksRequest) (*torrentpb.GetTorrentLnksResponse, error) {
	// simply return the list of lnks found on the server...
	lnks := srv.getTorrentLnks()
	return &torrentpb.GetTorrentLnksResponse{Lnks: lnks}, nil
}

// * Return all torrent info... *
func (srv *server) GetTorrentInfos(rqst *torrentpb.GetTorrentInfosRequest, stream torrentpb.TorrentService_GetTorrentInfosServer) error {

	// I will get all torrents from all clients...
	<-srv.getTorrentsInfo(stream)

	// wait until the stream is close by the client...
	return nil

}

// * Download a torrent file
func (srv *server) DownloadTorrent(ctx context.Context, rqst *torrentpb.DownloadTorrentRequest) (*torrentpb.DownloadTorrentResponse, error) {

	clientId, _, err := security.GetClientId(ctx)
	if err != nil {
		return nil, err
	}

	// So here I will create a new client...
	err = srv.downloadTorrent(rqst.Link, rqst.Dest, rqst.Seed, clientId)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &torrentpb.DownloadTorrentResponse{}, nil
}

// * Trop the torrent...
func (srv *server) DropTorrent(ctx context.Context, rqst *torrentpb.DropTorrentRequest) (*torrentpb.DropTorrentResponse, error) {

	// simply trop the torrent file...
	srv.dropTorrent(rqst.Name)

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
	s_impl.Path, _ = filepath.Abs(filepath.Dir(os.Args[0]))
	s_impl.Protocol = "grpc"
	s_impl.Domain, _ = config.GetDomain()
	s_impl.Address, _ = config.GetAddress()
	s_impl.Version = "0.0.1"
	s_impl.PublisherId = "globulario@globule-dell.globular.cloud"
	s_impl.Description = "The Hello world of gRPC service!"
	s_impl.Keywords = []string{"Example", "torrent", "Test", "Service"}
	s_impl.Repositories = make([]string, 0)
	s_impl.Discoveries = make([]string, 0)
	s_impl.Dependencies = make([]string, 0)
	s_impl.Permissions = make([]interface{}, 1)
	s_impl.Process = -1
	s_impl.ProxyProcess = -1
	s_impl.AllowAllOrigins = allow_all_origins
	s_impl.AllowedOrigins = allowed_origins
	s_impl.KeepAlive = true
	s_impl.KeepUpToDate = true
	s_impl.DownloadDir = config.GetDataDir() + "/torrents"
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

	if s_impl.Address == "" {
		s_impl.Address, _ = config.GetAddress()
	}

	// Create the dowload directory if it not already exist...
	Utility.CreateDirIfNotExist(s_impl.DownloadDir)

	// Register the torrent services
	torrentpb.RegisterTorrentServiceServer(s_impl.grpcServer, s_impl)
	reflection.Register(s_impl.grpcServer)

	// The actions
	s_impl.actions = make(chan map[string]interface{})

	// Now the permissions
	s_impl.Permissions[0] = map[string]interface{}{"action": "/torrent.TorrentService/DownloadTorrentRequest", "resources": []interface{}{map[string]interface{}{"index": 1, "permission": "write"}}}

	// When the music over turn off the ligth...
	s_impl.done = make(chan bool)

	// One client per directory...

	// Create client.
	config := torrent.NewDefaultClientConfig()
	config.DataDir = s_impl.DownloadDir
	config.Seed = s_impl.Seed

	s_impl.torrent_client_, err = torrent.NewClient(config)
	if err != nil {
		fmt.Println("fail to start torrent client with error ", err)
		return
	}

	// Start process torrent...
	s_impl.processTorrent()

	// download links...
	go func() {
		lnks, err := s_impl.readTorrentLnks()
		if err == nil {
			if len(lnks) > 0 {
				// Now I will download all the torrent...
				for i := 0; i < len(lnks); i++ {
					lnk := lnks[i]
					fmt.Println("open torrent ", lnk.Name)
					s_impl.downloadTorrent(lnk.Lnk, lnk.Dir, lnk.Seed, lnk.Owner)
				}
			}
		}
	}()

	// Start the service.
	s_impl.StartService()
}
