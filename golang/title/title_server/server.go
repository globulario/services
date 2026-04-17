// Package main wires the Title gRPC service for Globular with clean logging,
// CLI describe/health handlers, and documented getters/setters matching the
// Globular service contract.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/blevesearch/bleve/v2"
	"context"

	"github.com/globulario/services/golang/backup_hook"
	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/event/event_client"
	"github.com/globulario/services/golang/globular_client"
	globular "github.com/globulario/services/golang/globular_service"
	"github.com/globulario/services/golang/policy"
	"github.com/globulario/services/golang/rbac/rbac_client"
	"github.com/globulario/services/golang/resource/resourcepb"
	"github.com/globulario/services/golang/shared_index"
	"github.com/globulario/services/golang/storage/storage_store"
	"github.com/globulario/services/golang/title/title_client"
	"github.com/globulario/services/golang/title/titlepb"
	Utility "github.com/globulario/utility"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

// Defaults & CORS
var (
	defaultPort       = 10000
	defaultProxy      = defaultPort + 1
	allowAllOrigins   = true
	allowedOriginsStr = ""
)

// Version information (set via ldflags during build)
var (
	Version   = "0.1.0"
	BuildTime = "unknown"
	GitCommit = "unknown"
)

// logger is the service-wide structured logger.
var logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
	Level: slog.LevelInfo,
}))

// server implements Globular plumbing and Title RPC dependencies.
type server struct {
	// Core metadata
	Id           string
	Mac          string
	Name         string
	Domain       string
	Address      string
	Path         string
	Proto        string
	Port         int
	Proxy        int
	Protocol     string
	Version      string
	PublisherID  string
	Description  string
	Keywords     []string
	Repositories []string
	Discoveries  []string

	// Policy / ops
	AllowAllOrigins bool
	AllowedOrigins  string
	KeepUpToDate    bool
	Plaform         string
	Checksum        string
	KeepAlive       bool
	Permissions     []interface{}
	Dependencies    []string
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

	// Runtime
	grpcServer *grpc.Server

	// TMDb API key for title enrichment (stored in config, persists across reinstalls)
	TmdbApiKey string `json:"TmdbApiKey"`

	// Cache for search indices and associations
	CacheAddress           string
	CacheReplicationFactor int
	CacheType              string
	indexs                 map[string]bleve.Index
	associations           *sync.Map
	sharedIndex            *shared_index.SharedIndex
	indexPathsBeforeBackup []string // saved before backup close
}

// ---------------- Globular contract: documented getters/setters ----------------

// GetConfigurationPath returns the path to the service configuration file.
func (srv *server) GetConfigurationPath() string { return srv.ConfigPath }

// SetConfigurationPath sets the path to the service configuration file.
func (srv *server) SetConfigurationPath(path string) { srv.ConfigPath = path }

// GetAddress returns the HTTP address where the /config endpoint is served.
func (srv *server) GetAddress() string { return srv.Address }

// SetAddress sets the HTTP address where the /config endpoint is served.
func (srv *server) SetAddress(address string) { srv.Address = address }

// GetProcess returns the process id of the service (or -1 if not started).
func (srv *server) GetProcess() int { return srv.Process }

// SetProcess records the process id. When pid == -1, it closes indices and stores.
func (srv *server) SetProcess(pid int) {
	if pid == -1 {
		// Close indices
		for _, idx := range srv.indexs {
			_ = idx.Close()
		}
		// Close association stores
		if srv.associations != nil {
			srv.associations.Range(func(_ any, v any) bool {
				v.(storage_store.Store).Close()
				return true
			})
		}
	}
	srv.Process = pid
}

// GetProxyProcess returns the reverse-proxy process id (or -1 if not started).
func (srv *server) GetProxyProcess() int { return srv.ProxyProcess }

// SetProxyProcess records the reverse-proxy process id.
func (srv *server) SetProxyProcess(pid int) { srv.ProxyProcess = pid }

// GetState returns the current service lifecycle state (e.g. "running").
func (srv *server) GetState() string { return srv.State }

// SetState updates the current service lifecycle state.
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

// GetMac returns the MAC address of the host (if provided by the platform).
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

// Dist packages the service into the given path using Globular.
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
func (srv *server) GetPlatform() string { return srv.Plaform } // preserve original field name

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
func (srv *server) SetPermissions(permissions []interface{}) { srv.Permissions = permissions }

// RolesDefault returns an empty set — roles are defined externally in
// cluster-roles.json and per-service policy files.
func (srv *server) RolesDefault() []resourcepb.Role {
	return []resourcepb.Role{}
}

// Init initializes the service configuration and gRPC server.
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

// Save persists the current configuration.
func (srv *server) Save() error { return globular.SaveService(srv) }

// loadCustomConfig reads the service config from etcd and extracts
// service-specific fields that the generic Service interface doesn't cover
// (e.g. TmdbApiKey, CacheAddress, CacheType).
// Returns the extracted values so they can be applied after Init().
func loadCustomConfigFromEtcd(id string) (tmdbKey, cacheAddr, cacheType string, cacheRF int) {
	cfg, err := config.GetServiceConfigurationById(id)
	if err != nil || cfg == nil {
		// Also try by name.
		cfgs, err2 := config.GetServicesConfigurationsByName("title.TitleService")
		if err2 != nil || len(cfgs) == 0 {
			return
		}
		cfg = cfgs[0]
	}
	if v, ok := cfg["TmdbApiKey"].(string); ok {
		tmdbKey = v
	}
	if v, ok := cfg["CacheAddress"].(string); ok {
		cacheAddr = v
	}
	if v, ok := cfg["CacheType"].(string); ok {
		cacheType = v
	}
	if v, ok := cfg["CacheReplicationFactor"].(float64); ok && v > 0 {
		cacheRF = int(v)
	}
	return
}

// StartService begins serving gRPC and starts the shared index for mesh-ready search.
func (srv *server) StartService() error {
	// Scylla hosts come from etcd (Tier-0 — DNS depends on Scylla).
	scyllaHosts, err := config.GetScyllaHosts()
	if err != nil {
		logger.Warn("scylla hosts unavailable, shared index disabled", "err", err)
		return globular.StartService(srv, srv.grpcServer)
	}
	if srv.CacheAddress != "" {
		scyllaHosts = strings.Split(srv.CacheAddress, ",")
	}

	si := shared_index.New(shared_index.Config{
		Group:         "title",
		ScyllaHosts:   scyllaHosts,
		LocalIndexDir: config.GetDataDir(),
		PollInterval:  500 * time.Millisecond,
		SyncInterval:  5 * time.Second,
	}, logger)

	if err := si.Start(context.Background()); err != nil {
		logger.Warn("shared index unavailable, running in standalone mode", "err", err)
	} else {
		srv.sharedIndex = si
		logger.Info("shared index started (mesh-ready)")
	}

	return globular.StartService(srv, srv.grpcServer)
}

// StopService gracefully stops the running gRPC server and shared index.
func (srv *server) StopService() error {
	if srv.sharedIndex != nil {
		srv.sharedIndex.Stop()
	}
	return globular.StopService(srv, srv.grpcServer)
}

// ---------------- Helper clients & events ----------------

// getRbacClient returns a connected RBAC client.
func (srv *server) getRbacClient() (*rbac_client.Rbac_Client, error) {
	Utility.RegisterFunction("NewRbacService_Client", rbac_client.NewRbacService_Client)
	c, err := globular_client.GetClient(srv.Address, "rbac.RbacService", "NewRbacService_Client")
	if err != nil {
		return nil, err
	}
	return c.(*rbac_client.Rbac_Client), nil
}

// getEventClient returns a connected Event client.
func (srv *server) getEventClient() (*event_client.Event_Client, error) {
	Utility.RegisterFunction("NewEventService_Client", event_client.NewEventService_Client)
	c, err := globular_client.GetClient(srv.Address, "event.EventService", "NewEventService_Client")
	if err != nil {
		return nil, err
	}
	return c.(*event_client.Event_Client), nil
}

// publish sends a named event with data on the event bus.
func (srv *server) publish(event string, data []byte) error {
	client, err := srv.getEventClient()
	if err != nil {
		return err
	}
	return client.Publish(event, data)
}

// ---------------- main entrypoint ----------------

// main configures and starts the Title service.
func main() {
	// Define CLI flags (BEFORE any arg parsing)
	var (
		showDescribe = flag.Bool("describe", false, "print service description as JSON and exit")
		showHealth   = flag.Bool("health", false, "print service health status as JSON and exit")
		showVersion  = flag.Bool("version", false, "print version information as JSON and exit")
		showHelp     = flag.Bool("help", false, "show usage information and exit")
		enableDebug  = flag.Bool("debug", false, "enable debug logging")
	)

	flag.Usage = printUsage
	flag.Parse()

	// Apply debug logging if requested
	if *enableDebug {
		logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
		logger.Debug("debug logging enabled")
	}

	// Handle early-exit flags
	if *showHelp {
		printUsage()
		return
	}
	if *showVersion {
		printVersion()
		return
	}

	srv := new(server)

	// Static defaults that do not require etcd reads.
	srv.Name = string(titlepb.File_title_proto.Services().Get(0).FullName())
	srv.Proto = titlepb.File_title_proto.Path()
	srv.Path, _ = filepath.Abs(filepath.Dir(os.Args[0]))
	srv.Port = defaultPort
	srv.Proxy = defaultProxy
	srv.Protocol = "grpc"
	srv.Version = Version // Use build-time version
	srv.PublisherID = "localhost"
	srv.Description = "Media title catalog with metadata enrichment from IMDB and file associations"
	srv.Keywords = []string{"title", "movie", "tv", "episode", "audio", "video", "imdb", "metadata", "catalog"}
	srv.Repositories = make([]string, 0)
	srv.Discoveries = make([]string, 0)
	srv.Dependencies = make([]string, 0)
	srv.Permissions = []interface{}{
		// ---------------- Publishers ----------------

		// CreatePublisher
		map[string]interface{}{
			"action":     "/title.TitleService/CreatePublisher",
			"permission": "write",
			"resources": []interface{}{
				// CreatePublisherRequest.publisher.ID
				map[string]interface{}{"index": 0, "field": "Publisher.ID", "permission": "write"},
				// CreatePublisherRequest.indexPath
				map[string]interface{}{"index": 0, "field": "IndexPath", "permission": "write"},
			},
		},
		// DeletePublisher
		map[string]interface{}{
			"action":     "/title.TitleService/DeletePublisher",
			"permission": "admin",
			"resources": []interface{}{
				// DeletePublisherRequest.PublisherID
				map[string]interface{}{"index": 0, "field": "PublisherID", "permission": "admin"},
				// DeletePublisherRequest.indexPath
				map[string]interface{}{"index": 0, "field": "IndexPath", "permission": "admin"},
			},
		},
		// GetPublisherById
		map[string]interface{}{
			"action":     "/title.TitleService/GetPublisherById",
			"permission": "read",
			"resources": []interface{}{
				// GetPublisherByIdRequest.PublisherID
				map[string]interface{}{"index": 0, "field": "PublisherID", "permission": "read"},
				// GetPublisherByIdRequest.indexPath
				map[string]interface{}{"index": 0, "field": "IndexPath", "permission": "read"},
			},
		},

		// ---------------- Persons ----------------

		// CreatePerson
		map[string]interface{}{
			"action":     "/title.TitleService/CreatePerson",
			"permission": "write",
			"resources": []interface{}{
				// CreatePersonRequest.person.ID
				map[string]interface{}{"index": 0, "field": "Person.ID", "permission": "write"},
				// CreatePersonRequest.indexPath
				map[string]interface{}{"index": 0, "field": "IndexPath", "permission": "write"},
			},
		},
		// DeletePerson
		map[string]interface{}{
			"action":     "/title.TitleService/DeletePerson",
			"permission": "admin",
			"resources": []interface{}{
				// DeletePersonRequest.personId
				map[string]interface{}{"index": 0, "field": "PersonId", "permission": "admin"},
				// DeletePersonRequest.indexPath
				map[string]interface{}{"index": 0, "field": "IndexPath", "permission": "admin"},
			},
		},
		// GetPersonById
		map[string]interface{}{
			"action":     "/title.TitleService/GetPersonById",
			"permission": "read",
			"resources": []interface{}{
				// GetPersonByIdRequest.personId
				map[string]interface{}{"index": 0, "field": "PersonId", "permission": "read"},
				// GetPersonByIdRequest.indexPath
				map[string]interface{}{"index": 0, "field": "IndexPath", "permission": "read"},
			},
		},

		// ---------------- Titles ----------------

		// CreateTitle
		map[string]interface{}{
			"action":     "/title.TitleService/CreateTitle",
			"permission": "write",
			"resources": []interface{}{
				// CreateTitleRequest.title.ID
				map[string]interface{}{"index": 0, "field": "Title.ID", "permission": "write"},
				// CreateTitleRequest.indexPath
				map[string]interface{}{"index": 0, "field": "IndexPath", "permission": "write"},
			},
		},
		// GetTitleById
		map[string]interface{}{
			"action":     "/title.TitleService/GetTitleById",
			"permission": "read",
			"resources": []interface{}{
				// GetTitleByIdRequest.titleId
				map[string]interface{}{"index": 0, "field": "TitleId", "permission": "read"},
				// GetTitleByIdRequest.indexPath
				map[string]interface{}{"index": 0, "field": "IndexPath", "permission": "read"},
			},
		},
		// DeleteTitle
		map[string]interface{}{
			"action":     "/title.TitleService/DeleteTitle",
			"permission": "admin",
			"resources": []interface{}{
				// DeleteTitleRequest.titleId
				map[string]interface{}{"index": 0, "field": "TitleId", "permission": "admin"},
				// DeleteTitleRequest.indexPath
				map[string]interface{}{"index": 0, "field": "IndexPath", "permission": "admin"},
			},
		},
		// UpdateTitleMetadata
		map[string]interface{}{
			"action":     "/title.TitleService/UpdateTitleMetadata",
			"permission": "write",
			"resources": []interface{}{
				// UpdateTitleMetadataRequest.title.ID
				map[string]interface{}{"index": 0, "field": "Title.ID", "permission": "write"},
				// UpdateTitleMetadataRequest.indexPath
				map[string]interface{}{"index": 0, "field": "IndexPath", "permission": "write"},
			},
		},

		// ---------------- Audios / Albums ----------------

		// CreateAudio
		map[string]interface{}{
			"action":     "/title.TitleService/CreateAudio",
			"permission": "write",
			"resources": []interface{}{
				// CreateAudioRequest.audio.ID
				map[string]interface{}{"index": 0, "field": "Audio.ID", "permission": "write"},
				// CreateAudioRequest.indexPath
				map[string]interface{}{"index": 0, "field": "IndexPath", "permission": "write"},
			},
		},
		// GetAudioById
		map[string]interface{}{
			"action":     "/title.TitleService/GetAudioById",
			"permission": "read",
			"resources": []interface{}{
				// GetAudioByIdRequest.audioId
				map[string]interface{}{"index": 0, "field": "AudioId", "permission": "read"},
				// GetAudioByIdRequest.indexPath
				map[string]interface{}{"index": 0, "field": "IndexPath", "permission": "read"},
			},
		},
		// GetAlbum
		map[string]interface{}{
			"action":     "/title.TitleService/GetAlbum",
			"permission": "read",
			"resources": []interface{}{
				// GetAlbumRequest.albumId
				map[string]interface{}{"index": 0, "field": "AlbumId", "permission": "read"},
				// GetAlbumRequest.indexPath
				map[string]interface{}{"index": 0, "field": "IndexPath", "permission": "read"},
			},
		},
		// DeleteAudio
		map[string]interface{}{
			"action":     "/title.TitleService/DeleteAudio",
			"permission": "admin",
			"resources": []interface{}{
				// DeleteAudioRequest.audioId
				map[string]interface{}{"index": 0, "field": "AudioId", "permission": "admin"},
				// DeleteAudioRequest.indexPath
				map[string]interface{}{"index": 0, "field": "IndexPath", "permission": "admin"},
			},
		},
		// DeleteAlbum
		map[string]interface{}{
			"action":     "/title.TitleService/DeleteAlbum",
			"permission": "admin",
			"resources": []interface{}{
				// DeleteAlbumRequest.albumId
				map[string]interface{}{"index": 0, "field": "AlbumId", "permission": "admin"},
				// DeleteAlbumRequest.indexPath
				map[string]interface{}{"index": 0, "field": "IndexPath", "permission": "admin"},
			},
		},

		// ---------------- Videos ----------------

		// CreateVideo
		map[string]interface{}{
			"action":     "/title.TitleService/CreateVideo",
			"permission": "write",
			"resources": []interface{}{
				// CreateVideoRequest.video.ID
				map[string]interface{}{"index": 0, "field": "Video.ID", "permission": "write"},
				// CreateVideoRequest.indexPath
				map[string]interface{}{"index": 0, "field": "IndexPath", "permission": "write"},
			},
		},
		// GetVideoById
		map[string]interface{}{
			"action":     "/title.TitleService/GetVideoById",
			"permission": "read",
			"resources": []interface{}{
				// GetVideoByIdRequest.videoId
				map[string]interface{}{"index": 0, "field": "VideoId", "permission": "read"},
				// GetVideoByIdRequest.indexPath
				map[string]interface{}{"index": 0, "field": "IndexPath", "permission": "read"},
			},
		},
		// DeleteVideo
		map[string]interface{}{
			"action":     "/title.TitleService/DeleteVideo",
			"permission": "admin",
			"resources": []interface{}{
				// DeleteVideoRequest.videoId
				map[string]interface{}{"index": 0, "field": "VideoId", "permission": "admin"},
				// DeleteVideoRequest.indexPath
				map[string]interface{}{"index": 0, "field": "IndexPath", "permission": "admin"},
			},
		},
		// UpdateVideoMetadata
		map[string]interface{}{
			"action":     "/title.TitleService/UpdateVideoMetadata",
			"permission": "write",
			"resources": []interface{}{
				// UpdateVideoMetadataRequest.video.ID
				map[string]interface{}{"index": 0, "field": "Video.ID", "permission": "write"},
				// UpdateVideoMetadataRequest.indexPath
				map[string]interface{}{"index": 0, "field": "IndexPath", "permission": "write"},
			},
		},

		// ---------------- File associations ----------------

		// AssociateFileWithTitle
		map[string]interface{}{
			"action":     "/title.TitleService/AssociateFileWithTitle",
			"permission": "write",
			"resources": []interface{}{
				// AssociateFileWithTitleRequest.titleId
				map[string]interface{}{"index": 0, "field": "TitleId", "permission": "write"},
				// AssociateFileWithTitleRequest.filePath
				map[string]interface{}{"index": 0, "field": "FilePath", "permission": "write"},
				// AssociateFileWithTitleRequest.indexPath
				map[string]interface{}{"index": 0, "field": "IndexPath", "permission": "write"},
			},
		},
		// DissociateFileWithTitle
		map[string]interface{}{
			"action":     "/title.TitleService/DissociateFileWithTitle",
			"permission": "write",
			"resources": []interface{}{
				// DissociateFileWithTitleRequest.titleId
				map[string]interface{}{"index": 0, "field": "TitleId", "permission": "write"},
				// DissociateFileWithTitleRequest.filePath
				map[string]interface{}{"index": 0, "field": "FilePath", "permission": "write"},
				// DissociateFileWithTitleRequest.indexPath
				map[string]interface{}{"index": 0, "field": "IndexPath", "permission": "write"},
			},
		},
		// GetFileTitles
		map[string]interface{}{
			"action":     "/title.TitleService/GetFileTitles",
			"permission": "read",
			"resources": []interface{}{
				// GetFileTitlesRequest.filePath
				map[string]interface{}{"index": 0, "field": "FilePath", "permission": "read"},
				// GetFileTitlesRequest.indexPath
				map[string]interface{}{"index": 0, "field": "IndexPath", "permission": "read"},
			},
		},
		// GetFileVideos
		map[string]interface{}{
			"action":     "/title.TitleService/GetFileVideos",
			"permission": "read",
			"resources": []interface{}{
				// GetFileVideosRequest.filePath
				map[string]interface{}{"index": 0, "field": "FilePath", "permission": "read"},
				// GetFileVideosRequest.indexPath
				map[string]interface{}{"index": 0, "field": "IndexPath", "permission": "read"},
			},
		},
		// GetFileAudios
		map[string]interface{}{
			"action":     "/title.TitleService/GetFileAudios",
			"permission": "read",
			"resources": []interface{}{
				// GetFileAudiosRequest.filePath
				map[string]interface{}{"index": 0, "field": "FilePath", "permission": "read"},
				// GetFileAudiosRequest.indexPath
				map[string]interface{}{"index": 0, "field": "IndexPath", "permission": "read"},
			},
		},
		// GetTitleFiles
		map[string]interface{}{
			"action":     "/title.TitleService/GetTitleFiles",
			"permission": "read",
			"resources": []interface{}{
				// GetTitleFilesRequest.titleId
				map[string]interface{}{"index": 0, "field": "TitleId", "permission": "read"},
				// GetTitleFilesRequest.indexPath
				map[string]interface{}{"index": 0, "field": "IndexPath", "permission": "read"},
			},
		},

		// ---------------- Search ----------------

		// SearchTitles
		map[string]interface{}{
			"action":     "/title.TitleService/SearchTitles",
			"permission": "read",
			"resources": []interface{}{
				// SearchTitlesRequest.indexPath
				map[string]interface{}{"index": 0, "field": "IndexPath", "permission": "read"},
			},
		},
		// SearchPersons
		map[string]interface{}{
			"action":     "/title.TitleService/SearchPersons",
			"permission": "read",
			"resources": []interface{}{
				// SearchPersonsRequest.indexPath
				map[string]interface{}{"index": 0, "field": "IndexPath", "permission": "read"},
			},
		},
	}

	srv.Process = -1
	srv.ProxyProcess = -1
	srv.AllowAllOrigins = allowAllOrigins
	srv.AllowedOrigins = allowedOriginsStr
	srv.KeepAlive = true
	srv.KeepUpToDate = true
	srv.associations = new(sync.Map)
	srv.CacheType = "SCYLLADB"
	srv.CacheAddress = config.GetLocalIP()
	srv.CacheReplicationFactor = 1

	// Register Title client factory (used elsewhere in service).
	Utility.RegisterFunction("NewTitleService_Client", title_client.NewTitleService_Client)

	// Register method→action mappings with the global resolver for interceptor use.
	policy.GlobalResolver().Register([]policy.Permission{
		{Method: "/title.TitleService/GetPublisherById", Action: "title.getpublisher"},
		{Method: "/title.TitleService/GetPersonById", Action: "title.getperson"},
		{Method: "/title.TitleService/GetTitleById", Action: "title.gettitle"},
		{Method: "/title.TitleService/GetAudioById", Action: "title.getaudio"},
		{Method: "/title.TitleService/GetAlbum", Action: "title.getalbum"},
		{Method: "/title.TitleService/GetVideoById", Action: "title.getvideo"},
		{Method: "/title.TitleService/GetFileTitles", Action: "title.getfiletitles"},
		{Method: "/title.TitleService/GetFileVideos", Action: "title.getfilevideos"},
		{Method: "/title.TitleService/GetFileAudios", Action: "title.getfileaudios"},
		{Method: "/title.TitleService/GetTitleFiles", Action: "title.gettitlefiles"},
		{Method: "/title.TitleService/SearchTitles", Action: "title.searchtitles"},
		{Method: "/title.TitleService/SearchPersons", Action: "title.searchpersons"},
		{Method: "/title.TitleService/CreatePublisher", Action: "title.createpublisher"},
		{Method: "/title.TitleService/CreatePerson", Action: "title.createperson"},
		{Method: "/title.TitleService/CreateTitle", Action: "title.createtitle"},
		{Method: "/title.TitleService/UpdateTitleMetadata", Action: "title.updatetitle"},
		{Method: "/title.TitleService/CreateAudio", Action: "title.createaudio"},
		{Method: "/title.TitleService/CreateVideo", Action: "title.createvideo"},
		{Method: "/title.TitleService/UpdateVideoMetadata", Action: "title.updatevideo"},
		{Method: "/title.TitleService/AssociateFileWithTitle", Action: "title.associate"},
		{Method: "/title.TitleService/DissociateFileWithTitle", Action: "title.dissociate"},
		{Method: "/title.TitleService/DeletePublisher", Action: "title.deletepublisher"},
		{Method: "/title.TitleService/DeletePerson", Action: "title.deleteperson"},
		{Method: "/title.TitleService/DeleteTitle", Action: "title.deletetitle"},
		{Method: "/title.TitleService/DeleteAudio", Action: "title.deleteaudio"},
		{Method: "/title.TitleService/DeleteAlbum", Action: "title.deletealbum"},
		{Method: "/title.TitleService/DeleteVideo", Action: "title.deletevideo"},
	})

	// Handle --describe flag (print service metadata and exit)
	if *showDescribe {
		srv.Process = os.Getpid()
		srv.State = "starting"

		// Provide environment-driven defaults without etcd.
		if v, ok := os.LookupEnv("GLOBULAR_DOMAIN"); ok && v != "" {
			srv.Domain = strings.ToLower(v)
		} else {
			srv.Domain = "localhost"
		}
		host := "0.0.0.0"
		if ip, err := Utility.GetPrimaryIPAddress(); err == nil && ip != "" {
			host = ip
		}
		srv.Address = fmt.Sprintf("%s:%d", host, srv.Port)

		b, err := globular.DescribeJSON(srv)
		if err != nil {
			logger.Error("describe error", "service", srv.Name, "id", srv.Id, "err", err)
			os.Exit(2)
		}
		_, _ = os.Stdout.Write(b)
		_, _ = os.Stdout.Write([]byte("\n"))
		return
	}

	// Handle --health flag (print health status and exit)
	if *showHealth {
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
	}

	// ---- Positional arguments (service ID and config path) ----
	args := flag.Args()
	if len(args) == 0 {
		srv.Id = Utility.GenerateUUID(srv.Name + ":" + srv.Version + ":" + srv.Mac)
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
	} else if len(args) == 1 {
		srv.Id = args[0]
	} else if len(args) >= 2 {
		srv.Id = args[0]
		srv.ConfigPath = args[1]
	}

	// Safe to fetch config now (file/etcd as configured).
	if d, err := config.GetDomain(); err == nil {
		srv.Domain = d
	} else {
		srv.Domain = "localhost"
	}
	if a, err := config.GetAddress(); err == nil && strings.TrimSpace(a) != "" {
		srv.Address = a
	}

	logger.Info("starting title service",
		"service", srv.Name,
		"version", srv.Version,
		"domain", srv.Domain,
		"address", srv.Address,
		"port", srv.Port,
		"cache_type", srv.CacheType,
	)

	// Pre-read custom fields from etcd BEFORE Init(), because Init() calls
	// SaveService() which would overwrite them with empty defaults.
	tmdbKey, customCacheAddr, customCacheType, customCacheRF := loadCustomConfigFromEtcd(srv.Id)

	start := time.Now()
	logger.Debug("initializing service", "service", srv.Name, "id", srv.Id)
	if err := srv.Init(); err != nil {
		logger.Error("service init failed", "service", srv.Name, "id", srv.Id, "err", err)
		os.Exit(1)
	}
	logger.Debug("service init completed", "duration_ms", time.Since(start).Milliseconds())

	// Restore custom fields and re-save so they persist in etcd.
	if tmdbKey != "" {
		srv.TmdbApiKey = tmdbKey
		logger.Info("TMDb API key loaded from config")
	}
	if customCacheAddr != "" {
		srv.CacheAddress = customCacheAddr
	}
	if customCacheType != "" {
		srv.CacheType = customCacheType
	}
	if customCacheRF > 0 {
		srv.CacheReplicationFactor = customCacheRF
	}
	// Re-save to persist the restored custom fields.
	if tmdbKey != "" || customCacheAddr != "" || customCacheType != "" || customCacheRF > 0 {
		_ = srv.Save()
	}

	// non-blocking TSV pre-download
	logger.Debug("starting IMDB dataset prewarm (background)")
	go srv.prewarmIMDBDatasets()

	logger.Debug("registering gRPC handlers", "service", srv.Name)
	titlepb.RegisterTitleServiceServer(srv.grpcServer, srv)
	backup_hook.Register(srv.grpcServer, srv.newBackupHookHandler())
	reflection.Register(srv.grpcServer)
	logger.Debug("gRPC handlers registered")

	if srv.CacheAddress == "" || strings.HasPrefix(strings.ToLower(srv.CacheAddress), "localhost") {
		srv.CacheAddress = config.GetLocalIP()
	}

	logger.Info("service ready",
		"service", srv.Name,
		"port", srv.Port,
		"proxy", srv.Proxy,
		"protocol", srv.Protocol,
		"domain", srv.Domain,
		"startup_ms", time.Since(start).Milliseconds(),
		"version", srv.Version,
		"cache_address", srv.CacheAddress,
	)

	if err := srv.StartService(); err != nil {
		logger.Error("service start failed", "service", srv.Name, "id", srv.Id, "err", err)
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("Globular Title Service")
	fmt.Println()
	fmt.Println("USAGE:")
	fmt.Println("  title_server [OPTIONS] [<id> [configPath]]")
	fmt.Println()
	fmt.Println("OPTIONS:")
	fmt.Println("  --debug       Enable debug logging")
	fmt.Println("  --describe    Print service description as JSON and exit")
	fmt.Println("  --health      Print service health status as JSON and exit")
	fmt.Println("  --version     Print version information as JSON and exit")
	fmt.Println("  --help        Show this help message and exit")
	fmt.Println()
	fmt.Println("POSITIONAL ARGUMENTS:")
	fmt.Println("  id          Service instance ID (optional, auto-generated if not provided)")
	fmt.Println("  configPath  Path to service configuration file (optional)")
	fmt.Println()
	fmt.Println("ENVIRONMENT VARIABLES:")
	fmt.Println("  GLOBULAR_DOMAIN      Override service domain")
	fmt.Println("  GLOBULAR_ADDRESS     Override service address")
	fmt.Println()
	fmt.Println("FEATURES:")
	fmt.Println("  • Media title catalog with search and indexing")
	fmt.Println("  • IMDB metadata enrichment for movies, TV shows, and persons")
	fmt.Println("  • Audio/video/album associations with files")
	fmt.Println("  • Publisher and person management")
	fmt.Println("  • Full-text search across titles and persons")
	fmt.Println()
	fmt.Println("EXAMPLES:")
	fmt.Println("  # Start with auto-generated ID and default config")
	fmt.Println("  title_server")
	fmt.Println()
	fmt.Println("  # Start with specific service ID")
	fmt.Println("  title_server my-title-service-id")
	fmt.Println()
	fmt.Println("  # Enable debug logging")
	fmt.Println("  title_server --debug")
	fmt.Println()
	fmt.Println("  # Print service metadata")
	fmt.Println("  title_server --describe")
	fmt.Println()
	fmt.Println("  # Check service health")
	fmt.Println("  title_server --health")
}

func printVersion() {
	info := map[string]string{
		"service":    "title",
		"version":    Version,
		"build_time": BuildTime,
		"git_commit": GitCommit,
	}
	data, _ := json.MarshalIndent(info, "", "  ")
	fmt.Println(string(data))
}
