package main

import (
	"context"
	"encoding/gob"
	"errors"
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
	"google.golang.org/grpc/metadata"

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

	// Get the configuration path.
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

func (svr *server) saveTorrentLnks(lnks []TorrentLnk) error {
	// create a file
	dataFile, err := os.Create(svr.DownloadDir + "/lnks.gob")
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
func (svr *server) readTorrentLnks() ([]TorrentLnk, error) {
	fmt.Println("readTorrentLnks ", svr.DownloadDir+"/lnks.gob")

	// open data file
	dataFile, err := os.Open(svr.DownloadDir + "/lnks.gob")
	lnks := make([]TorrentLnk, 0)

	if err != nil {
		fmt.Println("readTorrentLnks: no file found...", err)
		return lnks, err
	}
	defer dataFile.Close()

	dataDecoder := gob.NewDecoder(dataFile)
	err = dataDecoder.Decode(&lnks)

	fmt.Println("readTorrentLnks: decode done...")
	if err != nil {
		fmt.Println("readTorrentLnks: decode error...", err)
		return lnks, err
	}

	fmt.Println("readTorrentLnks: decode done...")
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

func (svr *server) addResourceOwner(path, resourceType, subject string, subjectType rbacpb.SubjectType) error {
	rbac_client_, err := GetRbacClient(svr.Address)
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

func (svr *server) processTorrent() {

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
			case a := <-svr.actions:
				if a["action"] == "setTorrentTransfer" {
					fmt.Println("setTorrentTransfer")
					t := a["torrentTransfer"].(*TorrentTransfer)
					pending = append(pending, t)

					// Keep the link...
					lnks, err := svr.readTorrentLnks()
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
						err := svr.saveTorrentLnks(lnks)
						if err == nil {
							fmt.Println("lnk ", lnks[len(lnks)-1], " was set")
						}
					}

				} else if a["action"] == "getTorrentsInfo" {

					fmt.Println("getTorrentsInfo")
					getTorrentsInfo_actions = append(getTorrentsInfo_actions, a)

				} else if a["action"] == "dropTorrent" {
					fmt.Println("dropTorrent", a["name"].(string))
					// remove it from the map...
					delete(infos, a["name"].(string))
					pending_ := make([]*TorrentTransfer, 0)
					for _, p := range pending {
						if p.tor.Name() == a["name"].(string) {
							p.tor.Drop()
							os.RemoveAll(svr.DownloadDir + "/" + p.tor.Name())
						} else {
							pending_ = append(pending_, p)
						}
					}

					// Set the pending
					pending = pending_

					// I will remove the lnk from the list...
					lnks, err := svr.readTorrentLnks()
					if err == nil {
						lnks_ := make([]TorrentLnk, 0)
						for _, lnk := range lnks {
							if lnk.Name != a["name"].(string) {
								lnks_ = append(lnks_, lnk)
							}
						}
						svr.saveTorrentLnks(lnks_)
					}

				} else if a["action"] == "getTorrentLnks" {
					fmt.Println("execute getTorrentLnks...")
					lnks, err := svr.readTorrentLnks()
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
							src := svr.DownloadDir + "/" + file.Path
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
									svr.addResourceOwner(dir, "file", pending[i].owner, rbacpb.SubjectType_ACCOUNT)
									// so here the dir will be the parent of that dir
									dir = filepath.Dir(dir) // this will be the dir to reload...
								}

								err = Utility.CopyFile(src, dst)

								if err != nil {
									fmt.Println("fail to copy torrent file with error ", err)
								} else {
									fmt.Println("copy ", src, " to ", dst, " successfull")
									svr.addResourceOwner(dst, "file", pending[i].owner, rbacpb.SubjectType_ACCOUNT)

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

			case <-svr.done:

				// Stop the torrent client
				svr.torrent_client_.Close()

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
func (svr *server) setTorrentTransfer(t *torrent.Torrent, seed bool, lnk, dest string, owner string) {
	fmt.Println("set torrent transfer ", t.Name())
	a := make(map[string]interface{})
	a["action"] = "setTorrentTransfer"
	a["torrentTransfer"] = &TorrentTransfer{dst: dest, lnk: lnk, tor: t, seed: seed, owner: owner}

	// Set the action.
	svr.actions <- a
}

func (svr *server) dropTorrent(name string) {
	fmt.Println("drop torrent ", name)
	a := make(map[string]interface{})
	a["action"] = "dropTorrent"
	a["name"] = name

	// Set the action.
	svr.actions <- a
}

// get torrents infos...
func (svr *server) getTorrentsInfo(stream torrentpb.TorrentService_GetTorrentInfosServer) chan bool {

	// Return the torrent infos...
	a := make(map[string]interface{})
	a["action"] = "getTorrentsInfo"
	a["stream"] = stream

	exit := make(chan bool)

	a["exit"] = exit

	// Set the action.
	svr.actions <- a

	// wait for the result and return
	return exit // that channel will be call when the stream will be unavailable...
}

// get torrents infos...
func (svr *server) getTorrentLnks() []*torrentpb.TorrentLnk {
	fmt.Println("get torrents info")

	// Return the torrent infos...
	a := make(map[string]interface{})
	a["action"] = "getTorrentLnks"

	// set the output channel for the retreived link's
	a["lnks"] = make(chan []*torrentpb.TorrentLnk)

	// Set the action and wait for it result...
	svr.actions <- a

	// read the lnks...
	lnks := <-a["lnks"].(chan []*torrentpb.TorrentLnk) // that channel will be call when the stream will be unavailable...

	// wait for the result and return
	return lnks
}

// NewClient creates a new torrent client based on a magnet or a torrent file.
// If the torrent file is on http, we try downloading it.
func (svr *server) downloadTorrent(link, dest string, seed bool, owner string) error {
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
		fmt.Println("wait for torrent info...")

		<-t.GotInfo()

		fmt.Println("torrent info received...")

		// Start download...
		fmt.Println("Download torrent...")
		t.DownloadAll()

		svr.setTorrentTransfer(t, seed, link, dest, owner)

	}()

	return nil
}

// * Get the torrent links
func (svr *server) GetTorrentLnks(ctx context.Context, rqst *torrentpb.GetTorrentLnksRequest) (*torrentpb.GetTorrentLnksResponse, error) {
	// simply return the list of lnks found on the server...
	lnks := svr.getTorrentLnks()
	return &torrentpb.GetTorrentLnksResponse{Lnks: lnks}, nil
}

// * Return all torrent info... *
func (svr *server) GetTorrentInfos(rqst *torrentpb.GetTorrentInfosRequest, stream torrentpb.TorrentService_GetTorrentInfosServer) error {

	// I will get all torrents from all clients...
	<-svr.getTorrentsInfo(stream)

	// wait until the stream is close by the client...
	return nil

}

// * Download a torrent file
func (svr *server) DownloadTorrent(ctx context.Context, rqst *torrentpb.DownloadTorrentRequest) (*torrentpb.DownloadTorrentResponse, error) {

	var clientId string
	var err error

	// Now I will index the conversation to be retreivable for it creator...
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		token := strings.Join(md["token"], "")
		if len(token) > 0 {
			claims, err := security.ValidateToken(token)
			if err != nil {
				return nil, status.Errorf(
					codes.Internal,
					Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
			}
			clientId = claims.Id + "@" + claims.Domain
		} else {
			return nil, status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("no token was given for download torrent")))
		}
	}

	// So here I will create a new client...
	err = svr.downloadTorrent(rqst.Link, rqst.Dest, rqst.Seed, clientId)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &torrentpb.DownloadTorrentResponse{}, nil
}

// * Trop the torrent...
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
		if err != nil {
			fmt.Println("fail to read torrent links with error ", err)
		} else {
			if len(lnks) > 0 {
				// Now I will download all the torrent...
				for i := 0; i < len(lnks); i++ {
					lnk := lnks[i]
					fmt.Println("open torrent ", lnk.Name)
					s_impl.downloadTorrent(lnk.Lnk, lnk.Dir, lnk.Seed, lnk.Owner)
				}
			} else {
				fmt.Println("no torrent to download")
			}
		}
	}()

	// Start the service.
	s_impl.StartService()
}
