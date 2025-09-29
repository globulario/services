// Package main implements the Media gRPC service wired for Globular.
// It provides minimal, well-structured plumbing (slog logging, --describe,
// --health, clean getters/setters) and preserves the existing media-specific
// hooks (video preview/timeline generation, scheduling, etc.).
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/globulario/services/golang/authentication/authentication_client"
	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/event/event_client"
	"github.com/globulario/services/golang/event/eventpb"
	"github.com/globulario/services/golang/file/file_client"
	"github.com/globulario/services/golang/file/filepb"
	"github.com/globulario/services/golang/globular_client"
	globular "github.com/globulario/services/golang/globular_service"
	"github.com/globulario/services/golang/media/media_client"
	"github.com/globulario/services/golang/media/mediapb"
	"github.com/globulario/services/golang/rbac/rbac_client"
	"github.com/globulario/services/golang/rbac/rbacpb"
	"github.com/globulario/services/golang/resource/resourcepb"
	"github.com/globulario/services/golang/security"
	"github.com/globulario/services/golang/storage/storage_store"
	"github.com/globulario/services/golang/title/title_client"
	Utility "github.com/globulario/utility"
	"github.com/jasonlvhit/gocron"

	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
	"google.golang.org/protobuf/encoding/protojson"
)

// -----------------------------------------------------------------------------
// Defaults & Logger
// -----------------------------------------------------------------------------

var (
	defaultPort       = 10029
	defaultProxy      = 10030
	allowAllOrigins   = true
	allowedOriginsStr = ""

	// in-process cache backend handle
	cache storage_store.Store

	logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
)

const MAX_FFMPEG_INSTANCE = 3

// -----------------------------------------------------------------------------
// Service type (public getters/setters preserved)
// -----------------------------------------------------------------------------

// server implements the Globular service, plus media-specific fields.
type server struct {
	// Core metadata
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
	AllowedOrigins  string
	Protocol        string
	Version         string
	PublisherID     string
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

	// TLS
	TLS                bool
	CertFile           string
	KeyFile            string
	CertAuthorityTrust string

	// Permissions & deps
	Permissions  []interface{}
	Dependencies []string

	// runtime
	grpcServer *grpc.Server

	// Media service config
	Root string

	CacheType              string
	CacheAddress           string
	CacheReplicationFactor int
	HasEnableGPU           bool

	videoConversionErrors *sync.Map
	videoConversionLogs   *sync.Map
	scheduler             *gocron.Scheduler

	isProcessing                bool
	isProcessingAudio           bool
	AutomaticVideoConversion    bool
	AutomaticStreamConversion   bool
	StartVideoConversionHour    string
	MaximumVideoConversionDelay string
}

// --- Globular getters/setters (unchanged public prototypes) ---

// GetConfigurationPath returns the path to the service configuration file.
func (srv *server) GetConfigurationPath() string { return srv.ConfigPath }

// SetConfigurationPath sets the path to the service configuration file.
func (srv *server) SetConfigurationPath(path string) { srv.ConfigPath = path }

// GetAddress returns the HTTP address where /config can be reached.
func (srv *server) GetAddress() string { return srv.Address }

// SetAddress sets the HTTP address where /config can be reached.
func (srv *server) SetAddress(address string) { srv.Address = address }

// GetProcess returns the process id of the service, or -1 if not started.
func (srv *server) GetProcess() int { return srv.Process }

// SetProcess records the process id of the service.
func (srv *server) SetProcess(pid int) { srv.Process = pid }

// GetProxyProcess returns the reverse-proxy process id, or -1 if not started.
func (srv *server) GetProxyProcess() int { return srv.ProxyProcess }

// SetProxyProcess records the reverse-proxy process id.
func (srv *server) SetProxyProcess(pid int) { srv.ProxyProcess = pid }

// GetState returns the current service state (e.g., "running").
func (srv *server) GetState() string { return srv.State }

// SetState updates the current service state.
func (srv *server) SetState(state string) { srv.State = state }

// GetLastError returns the last error message recorded by the service.
func (srv *server) GetLastError() string { return srv.LastError }

// SetLastError records the last error message.
func (srv *server) SetLastError(err string) { srv.LastError = err }

// SetModTime sets the last modification time (unix seconds).
func (srv *server) SetModTime(modtime int64) { srv.ModTime = modtime }

// GetModTime returns the last modification time (unix seconds).
func (srv *server) GetModTime() int64 { return srv.ModTime }

// GetId returns the unique id of this service instance.
func (srv *server) GetId() string { return srv.Id }

// SetId sets the unique id of this service instance.
func (srv *server) SetId(id string) { srv.Id = id }

// GetName returns the gRPC service name.
func (srv *server) GetName() string { return srv.Name }

// SetName sets the gRPC service name.
func (srv *server) SetName(name string) { srv.Name = name }

// GetDescription returns the service description.
func (srv *server) GetDescription() string { return srv.Description }

// SetDescription sets the service description.
func (srv *server) SetDescription(description string) { srv.Description = description }

// GetMac returns the MAC address of the host (if set by the platform).
func (srv *server) GetMac() string { return srv.Mac }

// SetMac sets the MAC address of the host.
func (srv *server) SetMac(mac string) { srv.Mac = mac }

// GetKeywords returns the service keywords.
func (srv *server) GetKeywords() []string { return srv.Keywords }

// SetKeywords sets the service keywords.
func (srv *server) SetKeywords(keywords []string) { srv.Keywords = keywords }

// GetRepositories returns repositories associated with the service.
func (srv *server) GetRepositories() []string { return srv.Repositories }

// SetRepositories sets repositories associated with the service.
func (srv *server) SetRepositories(repositories []string) { srv.Repositories = repositories }

// GetDiscoveries returns discovery endpoints for the service.
func (srv *server) GetDiscoveries() []string { return srv.Discoveries }

// SetDiscoveries sets discovery endpoints for the service.
func (srv *server) SetDiscoveries(discoveries []string) { srv.Discoveries = discoveries }

// Dist packages (distributes) the service into the given path using Globular.
func (srv *server) Dist(path string) (string, error) { return globular.Dist(path, srv) }

// GetDependencies returns the list of dependent services.
func (srv *server) GetDependencies() []string {
	if srv.Dependencies == nil {
		srv.Dependencies = make([]string, 0)
	}
	return srv.Dependencies
}

// SetDependency appends a dependency if it is not already present.
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

// GetPlatform returns the service platform (e.g., "linux/amd64").
func (srv *server) GetPlatform() string { return srv.Plaform }

// SetPlatform sets the service platform (e.g., "linux/amd64").
func (srv *server) SetPlatform(platform string) { srv.Plaform = platform }

// GetPath returns the executable path.
func (srv *server) GetPath() string { return srv.Path }

// SetPath sets the executable path.
func (srv *server) SetPath(path string) { srv.Path = path }

// GetProto returns the path to the .proto file.
func (srv *server) GetProto() string { return srv.Proto }

// SetProto sets the path to the .proto file.
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

// SetAllowAllOrigins toggles whether all origins are allowed.
func (srv *server) SetAllowAllOrigins(allowAllOrigins bool) { srv.AllowAllOrigins = allowAllOrigins }

// GetAllowedOrigins returns the comma-separated list of allowed origins.
func (srv *server) GetAllowedOrigins() string { return srv.AllowedOrigins }

// SetAllowedOrigins sets the comma-separated list of allowed origins.
func (srv *server) SetAllowedOrigins(allowedOrigins string) { srv.AllowedOrigins = allowedOrigins }

// GetDomain returns the configured domain (ip or DNS name).
func (srv *server) GetDomain() string { return srv.Domain }

// SetDomain sets the configured domain (ip or DNS name).
func (srv *server) SetDomain(domain string) { srv.Domain = domain }

// GetTls returns true when TLS is enabled.
func (srv *server) GetTls() bool { return srv.TLS }

// SetTls enables or disables TLS.
func (srv *server) SetTls(hasTls bool) { srv.TLS = hasTls }

// GetCertAuthorityTrust returns the CA bundle path.
func (srv *server) GetCertAuthorityTrust() string { return srv.CertAuthorityTrust }

// SetCertAuthorityTrust sets the CA bundle path.
func (srv *server) SetCertAuthorityTrust(ca string) { srv.CertAuthorityTrust = ca }

// GetCertFile returns the TLS certificate path.
func (srv *server) GetCertFile() string { return srv.CertFile }

// SetCertFile sets the TLS certificate path.
func (srv *server) SetCertFile(certFile string) { srv.CertFile = certFile }

// GetKeyFile returns the TLS private key path.
func (srv *server) GetKeyFile() string { return srv.KeyFile }

// SetKeyFile sets the TLS private key path.
func (srv *server) SetKeyFile(keyFile string) { srv.KeyFile = keyFile }

// GetVersion returns the service version.
func (srv *server) GetVersion() string { return srv.Version }

// SetVersion sets the service version.
func (srv *server) SetVersion(version string) { srv.Version = version }

// GetPublisherID returns the publisher ID.
func (srv *server) GetPublisherID() string { return srv.PublisherID }

// SetPublisherID sets the publisher ID.
func (srv *server) SetPublisherID(PublisherID string) { srv.PublisherID = PublisherID }

// GetKeepUpToDate returns whether auto-updates are enabled.
func (srv *server) GetKeepUpToDate() bool { return srv.KeepUpToDate }

// SetKeepUptoDate toggles auto-updates.
func (srv *server) SetKeepUptoDate(val bool) { srv.KeepUpToDate = val }

// GetKeepAlive returns whether the service should be kept alive by the supervisor.
func (srv *server) GetKeepAlive() bool { return srv.KeepAlive }

// SetKeepAlive toggles keep-alive behavior.
func (srv *server) SetKeepAlive(val bool) { srv.KeepAlive = val }

// GetPermissions returns the action permissions configured for this service.
func (srv *server) GetPermissions() []interface{} { return srv.Permissions }

// SetPermissions sets the action permissions for this service.
func (srv *server) SetPermissions(p []interface{}) { srv.Permissions = p }

// Init initializes config and gRPC server.
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

// Save persists configuration.
func (srv *server) Save() error { return globular.SaveService(srv) }

// StartService launches gRPC service (+ proxy if configured).
func (srv *server) StartService() error { return globular.StartService(srv, srv.grpcServer) }

// StopService gracefully stops gRPC.
func (srv *server) StopService() error { return globular.StopService(srv, srv.grpcServer) }

// Stop RPC.
func (srv *server) Stop(ctx context.Context, _ *mediapb.StopRequest) (*mediapb.StopResponse, error) {
	return &mediapb.StopResponse{}, srv.StopService()
}

// RolesDefault returns the default roles for this service.
func (srv *server) RolesDefault() []resourcepb.Role {
	domain, _ := config.GetDomain()

	return []resourcepb.Role{
		{
			Id:          "role:media.viewer",
			Name:        "Media Viewer",
			Domain:      domain,
			Description: "Can inspect conversion logs and errors.",
			Actions: []string{
				"/media.MediaService/GetVideoConversionErrors",
				"/media.MediaService/GetVideoConversionLogs",
				"/media.MediaService/IsProcessVideo",
			},
			TypeName: "resource.Role",
		},
		{
			Id:          "role:media.uploader",
			Name:        "Media Uploader",
			Domain:      domain,
			Description: "Can upload videos and trigger playlist/VTT generation.",
			Actions: []string{
				"/media.MediaService/UploadVideo",
				"/media.MediaService/GeneratePlaylist",
				"/media.MediaService/CreateVttFile",
			},
			TypeName: "resource.Role",
		},
		{
			Id:          "role:media.converter",
			Name:        "Media Converter",
			Domain:      domain,
			Description: "Can initiate conversions and previews.",
			Actions: []string{
				"/media.MediaService/CreateVideoPreview",
				"/media.MediaService/CreateVideoTimeLine",
				"/media.MediaService/ConvertVideoToMpeg4H264",
				"/media.MediaService/ConvertVideoToHls",
				"/media.MediaService/StartProcessVideo",
				"/media.MediaService/StartProcessAudio",
				"/media.MediaService/StopProcessVideo",
			},
			TypeName: "resource.Role",
		},
		{
			Id:          "role:media.admin",
			Name:        "Media Admin",
			Domain:      domain,
			Description: "Full control over MediaService, including settings and stop.",
			Actions: []string{
				"/media.MediaService/Stop",
				"/media.MediaService/UploadVideo",
				"/media.MediaService/CreateVideoPreview",
				"/media.MediaService/CreateVideoTimeLine",
				"/media.MediaService/ConvertVideoToMpeg4H264",
				"/media.MediaService/ConvertVideoToHls",
				"/media.MediaService/StartProcessVideo",
				"/media.MediaService/StartProcessAudio",
				"/media.MediaService/StopProcessVideo",
				"/media.MediaService/IsProcessVideo",
				"/media.MediaService/SetVideoConversion",
				"/media.MediaService/SetVideoStreamConversion",
				"/media.MediaService/SetStartVideoConversionHour",
				"/media.MediaService/SetMaximumVideoConversionDelay",
				"/media.MediaService/GetVideoConversionErrors",
				"/media.MediaService/ClearVideoConversionErrors",
				"/media.MediaService/ClearVideoConversionError",
				"/media.MediaService/ClearVideoConversionLogs",
				"/media.MediaService/GetVideoConversionLogs",
				"/media.MediaService/GeneratePlaylist",
				"/media.MediaService/CreateVttFile",
			},
			TypeName: "resource.Role",
		},
	}
}

// -----------------------------------------------------------------------------
// Helpers (unchanged prototypes; logging via slog)
// -----------------------------------------------------------------------------

func (srv *server) isPublic(path string) bool {
	path = strings.ReplaceAll(path, "\\", "/")
	publics, err := srv.getPublicDirs()
	if err != nil {
		return false
	}
	if Utility.Exists(path) {
		for _, p := range publics {
			if strings.HasPrefix(path, p) {
				return true
			}
		}
	}
	return false
}

func (srv *server) formatPath(path string) string {
	path, _ = url.PathUnescape(path)
	path = strings.ReplaceAll(path, "\\", "/")

	if strings.HasPrefix(path, "/") {
		if len(path) > 1 {
			if strings.HasPrefix(path, "/") {
				if !srv.isPublic(path) {
					if strings.HasPrefix(path, "/users/") || strings.HasPrefix(path, "/applications/") {
						path = config.GetDataDir() + "/files" + path
					} else if Utility.Exists(config.GetWebRootDir() + path) {
						path = config.GetWebRootDir() + path
					} else if Utility.Exists(srv.Root + path) {
						path = srv.Root + path
					} else if Utility.Exists("/" + path) {
						path = "/" + path
					} else {
						path = srv.Root + "/" + path
					}
				}
			} else {
				path = srv.Root + "/" + path
			}
		} else {
			path = srv.Root
		}
	}
	return strings.ReplaceAll(path, "//", "/")
}

func getAuticationClient(address string) (*authentication_client.Authentication_Client, error) {
	Utility.RegisterFunction("NewAuthenticationService_Client", authentication_client.NewAuthenticationService_Client)
	client, err := globular_client.GetClient(address, "authentication.AuthenticationService", "NewAuthenticationService_Client")
	if err != nil {
		return nil, err
	}
	return client.(*authentication_client.Authentication_Client), nil
}

func (srv *server) getFileClient() (*file_client.File_Client, error) {
	address, _ := config.GetAddress()
	Utility.RegisterFunction("NewFileService_Client", file_client.NewFileService_Client)
	client, err := globular_client.GetClient(address, "file.FileService", "NewFileService_Client")
	if err != nil {
		logger.Error("file client connect failed", "err", err)
		return nil, err
	}
	return client.(*file_client.File_Client), nil
}

func (srv *server) getPublicDirs() ([]string, error) {
	client, err := srv.getFileClient()
	if err != nil {
		return nil, err
	}
	public, err := client.GetPublicDirs()
	if err != nil {
		return nil, err
	}
	return public, nil
}

func (srv *server) getFileInfo(token, path string) (*filepb.FileInfo, error) {
	if data_, err := cache.GetItem(path); err == nil {
		fi := new(filepb.FileInfo)
		if err := protojson.Unmarshal(data_, fi); err == nil {
			return fi, nil
		}
	}
	fc, err := srv.getFileClient()
	if err != nil {
		return nil, err
	}
	fi, err := fc.GetFileInfo(token, path, false, -1, -1)
	if err != nil {
		return nil, err
	}
	if data_, err := protojson.Marshal(fi); err == nil {
		_ = cache.SetItem(path, data_)
	}
	return fi, nil
}

func getEventClient() (*event_client.Event_Client, error) {
	address, _ := config.GetAddress()
	Utility.RegisterFunction("NewEventService_Client", event_client.NewEventService_Client)
	client, err := globular_client.GetClient(address, "event.EventService", "NewEventService_Client")
	if err != nil {
		logger.Error("event client connect failed", "err", err)
		return nil, err
	}
	return client.(*event_client.Event_Client), nil
}

func (srv *server) publishReloadDirEvent(path string) {
	client, err := getEventClient()
	path = strings.ReplaceAll(path, "\\", "/")
	path = strings.ReplaceAll(path, config.GetDataDir()+"/files", "")
	if err == nil {
		client.Publish("reload_dir_event", []byte(path))
	}
}

func getTitleClient() (*title_client.Title_Client, error) {
	address, _ := config.GetAddress()
	Utility.RegisterFunction("NewTitleService_Client", title_client.NewTitleService_Client)
	client, err := globular_client.GetClient(address, "title.TitleService", "NewTitleService_Client")
	if err != nil {
		return nil, err
	}
	return client.(*title_client.Title_Client), nil
}

func getRbacClient() (*rbac_client.Rbac_Client, error) {
	address, _ := config.GetAddress()
	Utility.RegisterFunction("NewRbacService_Client", rbac_client.NewRbacService_Client)
	client, err := globular_client.GetClient(address, "rbac.RbacService", "NewRbacService_Client")
	if err != nil {
		return nil, err
	}
	return client.(*rbac_client.Rbac_Client), nil
}

func (srv *server) setOwner(token, path string) error {
	var clientId string
	if len(token) > 0 {
		claims, err := security.ValidateToken(token)
		if err != nil {
			return err
		}
		if len(claims.UserDomain) == 0 {
			return errors.New("no user domain was found in the token")
		}
		clientId = claims.Id + "@" + claims.UserDomain
	} else {
		return errors.New("CreateBlogPost no token was given")
	}
	rbac_client_, err := getRbacClient()
	if err != nil {
		return err
	}
	if strings.Contains(path, "/files/users/") {
		path = path[strings.Index(path, "/users/"):]
	}
	return rbac_client_.AddResourceOwner(token, path, "file", clientId, rbacpb.SubjectType_ACCOUNT)
}

func subscribeWithRetry(
	channel string,
	id string,
	handler func(*eventpb.Event),
) error {
	var lastErr error
	for i := 0; i < 30; i++ { // ~60s total
		evtClient, err := getEventClient()
		if err == nil {
			// If your environment requires a token to subscribe, set it here:
			/*if mac := os.Getenv("GLOBULAR_MAC"); mac != "" {
			    if tok, e := security.GetLocalToken(mac); e == nil {
			        evtClient.SetToken(tok) // no-op if your client ignores it
			    }
			}*/
			if err = evtClient.Subscribe(channel, id, handler); err == nil {
				logger.Info("subscribed to event", "event", channel)
				return nil
			}
			lastErr = err
		} else {
			lastErr = err
		}
		time.Sleep(2 * time.Second)
	}
	return fmt.Errorf("subscribe %s failed after retries: %w", channel, lastErr)
}

// -----------------------------------------------------------------------------
// main with --describe / --health
// -----------------------------------------------------------------------------

func main() {
	srv := new(server)

	// Fill ONLY fields that do NOT call into config/etcd yet.
	srv.Name = string(mediapb.File_media_proto.Services().Get(0).FullName())
	srv.Proto = mediapb.File_media_proto.Path()
	srv.Path, _ = filepath.Abs(filepath.Dir(os.Args[0]))
	srv.Port = defaultPort
	srv.Proxy = defaultProxy
	srv.Protocol = "grpc"
	srv.Version = "0.0.1"
	srv.PublisherID = "localhost"
	srv.Description = "Media service with previews and scheduled conversions."
	srv.Keywords = []string{"Example", "media", "Test", "Service"}
	srv.Repositories = make([]string, 0)
	srv.Discoveries = make([]string, 0)
	srv.Dependencies = []string{
		"rbac.RbacService",
		"event.EventService",
		"authentication.AuthenticationService", 
		"log.LogService",
	}

	// srv.Permissions for media.MediaService
	srv.Permissions = []interface{}{
		// ---- Upload video (writes a new file)
		map[string]interface{}{
			"action":     "/media.MediaService/UploadVideo",
			"permission": "write",
			"resources": []interface{}{
				// UploadVideoRequest.dest
				map[string]interface{}{"index": 0, "field": "Dest", "permission": "write"},
			},
		},

		// ---- Create preview (writes artifacts near file)
		map[string]interface{}{
			"action":     "/media.MediaService/CreateVideoPreview",
			"permission": "write",
			"resources": []interface{}{
				// CreateVideoPreviewRequest.path
				map[string]interface{}{"index": 0, "field": "Path", "permission": "write"},
			},
		},

		// ---- Create timeline
		map[string]interface{}{
			"action":     "/media.MediaService/CreateVideoTimeLine",
			"permission": "write",
			"resources": []interface{}{
				// CreateVideoTimeLineRequest.path
				map[string]interface{}{"index": 0, "field": "Path", "permission": "write"},
			},
		},

		// ---- Convert to H.264
		map[string]interface{}{
			"action":     "/media.MediaService/ConvertVideoToMpeg4H264",
			"permission": "write",
			"resources": []interface{}{
				// ConvertVideoToMpeg4H264Request.path
				map[string]interface{}{"index": 0, "field": "Path", "permission": "write"},
			},
		},

		// ---- Convert to HLS
		map[string]interface{}{
			"action":     "/media.MediaService/ConvertVideoToHls",
			"permission": "write",
			"resources": []interface{}{
				// ConvertVideoToHlsRequest.path
				map[string]interface{}{"index": 0, "field": "Path", "permission": "write"},
			},
		},

		// ---- Bulk process (video)
		map[string]interface{}{
			"action":     "/media.MediaService/StartProcessVideo",
			"permission": "write",
			"resources": []interface{}{
				// StartProcessVideoRequest.path
				map[string]interface{}{"index": 0, "field": "Path", "permission": "write"},
			},
		},

		// ---- Bulk process (audio)
		map[string]interface{}{
			"action":     "/media.MediaService/StartProcessAudio",
			"permission": "write",
			"resources": []interface{}{
				// StartProcessAudioRequest.path
				map[string]interface{}{"index": 0, "field": "Path", "permission": "write"},
			},
		},

		// ---- Clear single conversion error
		map[string]interface{}{
			"action":     "/media.MediaService/ClearVideoConversionError",
			"permission": "delete",
			"resources": []interface{}{
				// ClearVideoConversionErrorRequest.path
				map[string]interface{}{"index": 0, "field": "Path", "permission": "delete"},
			},
		},

		// ---- Generate playlist (writes m3u in folder)
		map[string]interface{}{
			"action":     "/media.MediaService/GeneratePlaylist",
			"permission": "write",
			"resources": []interface{}{
				// GeneratePlaylistRequest.dir
				map[string]interface{}{"index": 0, "field": "Dir", "permission": "write"},
			},
		},

		// ---- Create VTT file (writes subtitle file)
		map[string]interface{}{
			"action":     "/media.MediaService/CreateVttFile",
			"permission": "write",
			"resources": []interface{}{
				// CreateVttFileRequest.path
				map[string]interface{}{"index": 0, "field": "Path", "permission": "write"},
			},
		},
	}

	srv.Process = -1
	srv.ProxyProcess = -1
	srv.AllowAllOrigins = allowAllOrigins
	srv.AllowedOrigins = allowedOriginsStr
	srv.KeepAlive = true
	srv.KeepUpToDate = true

	// Media defaults
	srv.Root = config.GetDataDir() + "/files"
	srv.HasEnableGPU = false
	srv.scheduler = gocron.NewScheduler()
	srv.videoConversionErrors = new(sync.Map)
	srv.videoConversionLogs = new(sync.Map)
	srv.AutomaticStreamConversion = false
	srv.AutomaticVideoConversion = false
	srv.MaximumVideoConversionDelay = "00:00"
	srv.StartVideoConversionHour = "00:00"

	// ---- CLI handling BEFORE config access ----
	args := os.Args[1:]
	if len(args) == 0 {
		srv.Id = Utility.GenerateUUID(srv.Name + ":" + srv.Address)
		allocator, err := config.NewDefaultPortAllocator()
		if err != nil {
			logger.Error("fail to create port allocator", "error", err)
			os.Exit(1)
		}
		p, err := allocator.Next(srv.Id)
		if err != nil {
			logger.Error("fail to allocate port", "error", err)
			os.Exit(1)
		}
		srv.Port = p
	}

	// Optional positional overrides (id, config path) if they don't start with '-'
	if len(args) == 1 && !strings.HasPrefix(args[0], "-") {
		srv.Id = args[0]
	} else if len(args) == 2 && !strings.HasPrefix(args[0], "-") && !strings.HasPrefix(args[1], "-") {
		srv.Id = args[0]
		srv.ConfigPath = args[1]
	}

	// Handle flags first (no etcd/config access)
	for _, a := range args {
		switch strings.ToLower(a) {
		case "--describe":
			// best-effort runtime fields without hitting etcd
			srv.Process = os.Getpid()
			srv.State = "starting"

			// Provide harmless defaults for Domain/Address that don’t need etcd
			if v, ok := os.LookupEnv("GLOBULAR_DOMAIN"); ok && v != "" {
				srv.Domain = strings.ToLower(v)
			} else {
				srv.Domain = "localhost"
			}
			if v, ok := os.LookupEnv("GLOBULAR_ADDRESS"); ok && v != "" {
				srv.Address = strings.ToLower(v)
			} else {
				// address here is informational; using grpc port keeps it truthful
				srv.Address = "localhost:" + Utility.ToString(srv.Port)
			}

			b, err := globular.DescribeJSON(srv)
			if err != nil {
				logger.Error("describe error", "service", srv.Name, "id", srv.Id, "err", err)
				os.Exit(2)
			}
			_, _ = os.Stdout.Write(b)
			_, _ = os.Stdout.Write([]byte("\n"))
			return

		case "--health":
			if srv.Port == 0 || srv.Name == "" {
				logger.Error("health error: uninitialized", "service", srv.Name, "port", srv.Port)
				os.Exit(2)
			}
			b, err := globular.HealthJSON(srv, &globular.HealthOptions{
				Timeout:     1500 * time.Millisecond,
				ServiceName: "",
			})
			if err != nil {
				logger.Error("health error", "service", srv.Name, "id", srv.Id, "err", err)
				os.Exit(2)
			}
			_, _ = os.Stdout.Write(b)
			_, _ = os.Stdout.Write([]byte("\n"))
			return
		case "--help", "-h", "/?":
			printUsage()
			return
		case "--version", "-v":
			fmt.Println(srv.Version)
			return
		}
	}

	// Now safe to use config (may read etcd / file fallback)
	if d, err := config.GetDomain(); err == nil && d != "" {
		srv.Domain = d
	}
	if a, err := config.GetAddress(); err == nil && strings.TrimSpace(a) != "" {
		srv.Address = a
	}

	// Register client ctor
	Utility.RegisterFunction("NewMediaService_Client", media_client.NewMediaService_Client)

	start := time.Now()
	if err := srv.Init(); err != nil {
		logger.Error("service init failed", "service", srv.Name, "id", srv.Id, "err", err)
		os.Exit(1)
	}

	// Register RPCs
	mediapb.RegisterMediaServiceServer(srv.grpcServer, srv)
	reflection.Register(srv.grpcServer)

	// Cache backend selection
	switch strings.ToUpper(srv.CacheType) {
	case "BADGER":
		cache = storage_store.NewBadger_store()
	case "SCYLLA":
		cache = storage_store.NewScylla_store(srv.CacheAddress, "files", srv.CacheReplicationFactor)
	case "LEVELDB":
		cache = storage_store.NewLevelDB_store()
	default:
		cache = storage_store.NewBigCache_store()
	}

	if srv.MaximumVideoConversionDelay == "" {
		srv.MaximumVideoConversionDelay = "00:00"
	}
	if srv.StartVideoConversionHour == "" {
		srv.StartVideoConversionHour = "00:00"
	}

	Utility.CreateDirIfNotExist(srv.Root + "/cache")
	if err := cache.Open(`{"path":"` + srv.Root + `", "name":"files"}`); err != nil {
		logger.Warn("cache open failed", "root", srv.Root, "err", err)
	}

	go func() {
		if titleCli, err := getTitleClient(); err == nil {
			ch1 := make(chan string)

			// worker (unchanged) …

			if err := subscribeWithRetry("generate_video_preview_event", Utility.RandomUUID(), func(e *eventpb.Event) {
				ch1 <- string(e.Data)
			}); err != nil {
				logger.Warn("subscribe failed", "channel", "generate_video_preview_event", "err", err)
			}

			if err := subscribeWithRetry("cancel_upload_event", srv.GetId(), cancelUploadVideoHandeler(srv, titleCli)); err != nil {
				logger.Warn("subscribe failed", "channel", "cancel_upload_event", "err", err)
			}
		}
	}()

	// Schedule automatic video conversions
	srv.scheduler.Every(1).Day().At(srv.StartVideoConversionHour).Do(processVideos, srv)
	if srv.AutomaticVideoConversion {
		srv.scheduler.Start()
	}

	// Audio processing bootstrap
	srv.startProcessAudios()

	logger.Info("service ready",
		"service", srv.Name,
		"port", srv.Port,
		"proxy", srv.Proxy,
		"protocol", srv.Protocol,
		"domain", srv.Domain,
		"listen_ms", time.Since(start).Milliseconds(),
	)

	if err := srv.StartService(); err != nil {
		logger.Error("service start failed", "service", srv.Name, "id", srv.Id, "err", err)
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("Usage:")
	fmt.Println("  media_server [id] [config_path] [--describe] [--health]")
	fmt.Println()
	fmt.Println("Options:")
	fmt.Println("  --describe   Print service metadata as JSON and exit")
	fmt.Println("  --health     Print health check JSON and exit")
	fmt.Println()
	fmt.Println("Arguments:")
	fmt.Println("  id           Optional service instance ID")
	fmt.Println("  config_path  Optional path to configuration file")
}
