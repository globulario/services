package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/emicklei/proto"
	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/event/eventpb"
	"github.com/globulario/services/golang/globular_client"
	globular "github.com/globulario/services/golang/globular_service"
	"github.com/globulario/services/golang/process"
	"github.com/globulario/services/golang/repository/repository_client"
	"github.com/globulario/services/golang/resource/resource_client"
	"github.com/globulario/services/golang/resource/resourcepb"
	"github.com/globulario/services/golang/security"
	"github.com/globulario/services/golang/services_manager/services_managerpb"
	Utility "github.com/globulario/utility"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/structpb"
)

func (srv *server) uninstallService(token, PublisherID, serviceId, version string, deletePermissions bool) error {
	services, err := config.GetServicesConfigurations()
	if err != nil {
		return err
	}

	for _, s := range services {
		pub := s["PublisherID"].(string)
		id := s["Id"].(string)
		ver := s["Version"].(string)
		name := s["Name"].(string)

		if pub == PublisherID && id == serviceId && ver == version {
			// Stop service
			_ = srv.stopService(s)

			// Get actions to delete
			toDelete, err := config.GetServiceMethods(name, PublisherID, version)
			if err != nil {
				return err
			}

			if deletePermissions {
				for _, act := range toDelete {
					_ = srv.removeRolesAction(act)
					_ = srv.removeApplicationsAction(token, act)
					_ = srv.removePeersAction(act)
				}
			}

			// refresh local methods set
			methods := make([]string, 0, len(srv.methods))
			for _, m := range srv.methods {
				if !Utility.Contains(toDelete, m) {
					methods = append(methods, m)
				}
			}
			srv.methods = methods
			if err := srv.registerMethods(); err != nil {
				logger.Warn("register methods after uninstall failed", "err", err)
			}

			// Remove files
			path := filepath.ToSlash(filepath.Join(srv.Root, "services", PublisherID, name, version, serviceId))
			if Utility.Exists(path) {
				if err := os.RemoveAll(path); err != nil {
					return err
				}
			}
		}
	}
	return nil
}


// updateService reacts to repo events and updates a service if KeepUpToDate is true.
func updateService(srv *server, service map[string]interface{}) func(evt *eventpb.Event) {
	return func(evt *eventpb.Event) {
		logger.Info("update service event received", "event", string(evt.Name))

		kup, _ := service["KeepUpToDate"].(bool)
		if !kup {
			return
		}

		descriptor := new(resourcepb.PackageDescriptor)
		if err := protojson.Unmarshal(evt.Data, descriptor); err != nil {
			logger.Error("parse package descriptor failed", "err", err)
			return
		}

		logger.Info("updating service",
			"name", descriptor.Name,
			"PublisherID", descriptor.PublisherID,
			"id", descriptor.Id,
			"version", descriptor.Version)

		token, err := security.GetLocalToken(srv.Mac)
		if err != nil {
			logger.Error("get local token failed", "err", err)
			return
		}

		if srv.stopService(service) == nil {
			if srv.uninstallService(token, descriptor.PublisherID, descriptor.Id, service["Version"].(string), true) == nil {
				if err := srv.installService(token, descriptor); err != nil {
					logger.Error("service update failed", "err", err)
				} else {
					logger.Info("service updated", "name", service["Name"])
				}
			}
		}
	}
}

// UninstallService removes a service from the current node.
// It authenticates with the token extracted from ctx and delegates to uninstallService.
// Public signature preserved.
func (srv *server) UninstallService(ctx context.Context, rqst *services_managerpb.UninstallServiceRequest) (*services_managerpb.UninstallServiceResponse, error) {
	_, token, err := security.GetClientId(ctx)
	if err != nil {
		logger.Error("failed to get client token for uninstall", "serviceId", rqst.ServiceId, "err", err)
		return nil, err
	}

	if err := srv.uninstallService(token, rqst.PublisherID, rqst.ServiceId, rqst.Version, rqst.DeletePermissions); err != nil {
		logger.Error("uninstall service failed",
			"serviceId", rqst.ServiceId,
			"PublisherID", rqst.PublisherID,
			"version", rqst.Version,
			"err", err,
		)
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	logger.Info("service uninstalled", "serviceId", rqst.ServiceId, "PublisherID", rqst.PublisherID, "version", rqst.Version)
	return &services_managerpb.UninstallServiceResponse{Result: true}, nil
}

// GetRepositoryClient returns a repository client for the given address.
// Public signature preserved.
func GetRepositoryClient(address string) (*repository_client.Repository_Service_Client, error) {
	Utility.RegisterFunction("NewRepositoryService_Client", repository_client.NewRepositoryService_Client)
	client, err := globular_client.GetClient(address, "repository.PackageRepository", "NewRepositoryService_Client")
	if err != nil {
		return nil, fmt.Errorf("connect repository at %s: %w", address, err)
	}
	return client.(*repository_client.Repository_Service_Client), nil
}

// installService downloads, expands and installs a service bundle, then writes its config to etcd.
// It preserves relevant previous config flags if a previous version exists.
func (srv *server) installService(token string, descriptor *resourcepb.PackageDescriptor) error {
	if len(descriptor.Repositories) == 0 {
		return fmt.Errorf("no service repository found for service %s", descriptor.Id)
	}

	for _, repoAddr := range descriptor.Repositories {
		repoClient, err := GetRepositoryClient(repoAddr)
		if err != nil {
			return fmt.Errorf("get repository client (%s): %w", repoAddr, err)
		}

		bundle, err := repoClient.DownloadBundle(descriptor, globular.GetPlatform())
		if err != nil {
			return fmt.Errorf("download bundle (%s@%s) from %s: %w", descriptor.Id, descriptor.Version, repoAddr, err)
		}

		// Best-effort uninstall of previous version.
		if prev, _ := config.GetServiceConfigurationById(descriptor.Id); prev != nil {
			if err := srv.uninstallService(token, descriptor.PublisherID, descriptor.Id, Utility.ToString(prev["Version"]), false); err != nil {
				logger.Warn("failed to uninstall previous version (continuing)",
					"serviceId", descriptor.Id, "prevVersion", Utility.ToString(prev["Version"]), "err", err)
			}
		}

		// Extract bundle to a temp dir.
		extractedPath, err := Utility.ExtractTarGz(bytes.NewReader(bundle.Binairies))
		if err != nil {
			return fmt.Errorf("extract bundle (%s@%s): %w", descriptor.Id, descriptor.Version, err)
		}
		defer func() {
			if rmErr := os.RemoveAll(extractedPath); rmErr != nil {
				logger.Warn("cleanup extracted bundle failed", "path", extractedPath, "err", rmErr)
			}
		}()

		// Copy to <root>/services/<PublisherID>/...
		base := filepath.Join(srv.Root, "services")
		if err := Utility.CreateDirIfNotExist(base); err != nil {
			return fmt.Errorf("ensure services dir: %w", err)
		}
		if err := Utility.CopyDir(filepath.Join(extractedPath, descriptor.PublisherID), base); err != nil {
			return fmt.Errorf("copy bundle to services tree: %w", err)
		}

		installRoot := filepath.Join(base, descriptor.PublisherID, descriptor.Name, descriptor.Version, descriptor.Id)

		// preinst hook
		if Utility.Exists(filepath.Join(installRoot, "preinst")) {
			if err := exec.Command("/bin/sh", filepath.Join(installRoot, "preinst")).Run(); err != nil {
				_ = os.RemoveAll(installRoot)
				return fmt.Errorf("preinst failed: %w", err)
			}
		}

		// Locate executable
		execPath, err := findServiceExecutable(installRoot, descriptor.Name, descriptor.Id)
		if err != nil {
			return fmt.Errorf("find service executable: %w", err)
		}
		if err := os.Chmod(execPath, 0o755); err != nil {
			return fmt.Errorf("chmod executable: %w", err)
		}

		// Locate proto (prefer published path; fallback to local)
		protoPath, err := findProtoPath(srv.Root, descriptor.PublisherID, descriptor.Name, descriptor.Version, installRoot)
		if err != nil {
			return err
		}

		// Preserve previous runtime prefs if any
		prevCfg, _ := config.GetServiceConfigurationById(descriptor.Id)

		// Build desired config
		cfg := map[string]interface{}{
			"Id":           descriptor.Id,
			"Name":         descriptor.Name,
			"PublisherID":  descriptor.PublisherID,
			"Version":      descriptor.Version,
			"Description":  descriptor.Description,
			"Keywords":     descriptor.Keywords,
			"Path":         toSlash(execPath),
			"Proto":        toSlash(protoPath),
			"Repositories": toIfaceSlice(descriptor.Repositories),
			"Discoveries":  toIfaceSlice(descriptor.Discoveries),

			// Defaults / runtime
			"KeepAlive":       true,
			"KeepUpToDate":    false,
			"AllowAllOrigins": true,
			"AllowedOrigins":  "",
			"TLS":             false,
			"Port":            0,  // to be assigned at start
			"Proxy":           0,  // to be assigned at start
			"Process":         -1, // runtime
			"ProxyProcess":    -1, // runtime
			"State":           "stopped",
			"LastError":       "",
		}

		// Preserve selected fields from previous config
		if prevCfg != nil {
			copyIfPresent(prevCfg, cfg, "KeepAlive", "KeepUpToDate", "TLS", "AllowAllOrigins", "AllowedOrigins")
		}

		// Save config to etcd
		if err := config.SaveServiceConfiguration(cfg); err != nil {
			return fmt.Errorf("save service configuration (%s): %w", descriptor.Id, err)
		}

		// postinst hook
		if Utility.Exists(filepath.Join(installRoot, "postinst")) {
			if err := exec.Command("/bin/sh", filepath.Join(installRoot, "postinst")).Run(); err != nil {
				_ = os.RemoveAll(installRoot)
				return fmt.Errorf("postinst failed: %w", err)
			}
		}

		// Merge discoveries
		if mergeDiscoveries(&srv.Discoveries, descriptor.Discoveries) {
			if err := srv.Save(); err != nil {
				logger.Warn("failed to persist service-manager after discoveries merge", "err", err)
			}
		}

		logger.Info("service installed",
			"serviceId", descriptor.Id,
			"PublisherID", descriptor.PublisherID,
			"version", descriptor.Version,
			"exec", execPath,
			"proto", protoPath)
		return nil
	}

	return nil
}

func toIfaceSlice(in []string) []interface{} {
	out := make([]interface{}, 0, len(in))
	for _, v := range in {
		out = append(out, v)
	}
	return out
}

func toSlash(s string) string { return strings.ReplaceAll(s, "\\", "/") }

func copyIfPresent(src, dst map[string]interface{}, keys ...string) {
	for _, k := range keys {
		if v, ok := src[k]; ok {
			dst[k] = v
		}
	}
}

func mergeDiscoveries(dst *[]string, src []string) bool {
	changed := false
	for _, d := range src {
		if !Utility.Contains(*dst, d) {
			*dst = append(*dst, d)
			changed = true
		}
	}
	return changed
}

func findProtoPath(root, publisher, name, version, fallbackRoot string) (string, error) {
	// First look under <root>/services/<publisher>/<name>/<version>/
	serviceRoot := filepath.Join(root, "services", publisher, name, version)
	protos, _ := Utility.FindFileByName(serviceRoot, ".proto")
	if len(protos) == 0 {
		// fallback: search inside installRoot
		protos, _ = Utility.FindFileByName(fallbackRoot, ".proto")
		if len(protos) == 0 {
			return "", fmt.Errorf("no .proto file found for service under %s or %s", serviceRoot, fallbackRoot)
		}
	}
	return protos[0], nil
}

func findServiceExecutable(dir, name, id string) (string, error) {
	// Prefer exact name/id, then first executable file.
	candidates := []string{
		filepath.Join(dir, name),
		filepath.Join(dir, id),
		filepath.Join(dir, name+".exe"),
		filepath.Join(dir, id+".exe"),
	}
	for _, c := range candidates {
		if Utility.Exists(c) {
			return c, nil
		}
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", fmt.Errorf("read dir %s: %w", dir, err)
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		full := filepath.Join(dir, e.Name())
		fi, err := os.Stat(full)
		if err == nil && (fi.Mode()&0o111) != 0 { // any exec bit
			return full, nil
		}
		// Windows: fall back to .exe
		if strings.HasSuffix(strings.ToLower(e.Name()), ".exe") {
			return full, nil
		}
	}
	return "", fmt.Errorf("no executable found in %s", dir)
}

func getResourceClient(address string) (*resource_client.Resource_Client, error) {
	Utility.RegisterFunction("NewResourceService_Client", resource_client.NewResourceService_Client)
	client, err := globular_client.GetClient(address, "resource.ResourceService", "NewResourceService_Client")
	if err != nil {
		return nil, fmt.Errorf("connect resource at %s: %w", address, err)
	}
	return client.(*resource_client.Resource_Client), nil
}

// InstallService installs (or updates) a service by fetching its descriptor from a resource service
// identified by rqst.DicorveryId, then delegates to installService.
// Public signature preserved.
func (srv *server) InstallService(ctx context.Context, rqst *services_managerpb.InstallServiceRequest) (*services_managerpb.InstallServiceResponse, error) {
	_, token, err := security.GetClientId(ctx)
	if err != nil {
		logger.Error("failed to get client token for install", "serviceId", rqst.ServiceId, "err", err)
		return nil, err
	}

	resourceClient, err := getResourceClient(rqst.DicorveryId)
	if err != nil {
		logger.Error("failed to connect to discovery/resource", "discoveryId", rqst.DicorveryId, "err", err)
		return nil, status.Errorf(codes.Internal, Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), fmt.Errorf("connect to %s: %w", rqst.DicorveryId, err)))
	}

	descriptor, err := resourceClient.GetPackageDescriptor(rqst.ServiceId, rqst.PublisherID, rqst.Version)
	if err != nil {
		logger.Error("get package descriptor failed",
			"serviceId", rqst.ServiceId, "PublisherID", rqst.PublisherID, "version", rqst.Version, "err", err)
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	if err := srv.installService(token, descriptor); err != nil {
		logger.Error("install service failed",
			"serviceId", rqst.ServiceId, "PublisherID", rqst.PublisherID, "version", rqst.Version, "err", err)
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &services_managerpb.InstallServiceResponse{Result: true}, nil
}

func (srv *server) stopServiceInstance(serviceId string) error {
	if serviceId == srv.GetId() {
		return errors.New("service manager cannot stop itself")
	}

	s, err := config.GetServiceConfigurationById(serviceId)
	if err != nil {
		return fmt.Errorf("get service config by id (%s): %w", serviceId, err)
	}
	if s != nil {
		return srv.stopService(s)
	}

	services, err := config.GetServicesConfigurationsByName(serviceId)
	if err != nil {
		return fmt.Errorf("get services by name (%s): %w", serviceId, err)
	}
	for _, sc := range services {
		id := sc["Id"].(string)
		cfg, err := config.GetServiceConfigurationById(id)
		if err != nil {
			return fmt.Errorf("get service config by id (%s): %w", id, err)
		}
		if cfg == nil {
			return fmt.Errorf("no service found with id %s", id)
		}
		if err := srv.stopService(cfg); err != nil {
			return err
		}
	}
	return nil
}

// StopServiceInstance stops a running service instance by ID or Name.
// Public signature preserved.
func (srv *server) StopServiceInstance(ctx context.Context, rqst *services_managerpb.StopServiceInstanceRequest) (*services_managerpb.StopServiceInstanceResponse, error) {
	if err := srv.stopServiceInstance(rqst.ServiceId); err != nil {
		logger.Error("stop service failed", "serviceId", rqst.ServiceId, "err", err)
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	logger.Info("service stopped", "serviceId", rqst.ServiceId)
	return &services_managerpb.StopServiceInstanceResponse{Result: true}, nil
}

func (srv *server) startServiceInstance(serviceId string) error {
	if serviceId == srv.GetId() {
		return errors.New("service manager cannot start itself")
	}

	// Load global server config file (legacy)
	data, err := os.ReadFile(filepath.Join(config.GetConfigDir(), "config.json"))
	if err != nil {
		return fmt.Errorf("read global config: %w", err)
	}
	var _globularCfg map[string]interface{}
	if err := json.Unmarshal(data, &_globularCfg); err != nil {
		return fmt.Errorf("parse global config: %w", err)
	}

	s, err := config.GetServiceConfigurationById(serviceId)
	if err != nil {
		return fmt.Errorf("get service config by id (%s): %w", serviceId, err)
	}
	if s == nil {
		return fmt.Errorf("no service configuration found for id %s", serviceId)
	}

	port := Utility.ToInt(s["Port"])
	pid, err := process.StartServiceProcess(s, port)
	if err != nil {
		return fmt.Errorf("start service process: %w", err)
	}

	s["Process"] = pid
	s["State"] = "running"

	if err := srv.publishUpdateServiceConfigEvent(s); err != nil {
		logger.Warn("failed to publish update config event after start", "serviceId", serviceId, "err", err)
	}
	return nil
}

// StartServiceInstance starts a service instance by ID or Name.
// Public signature preserved.
func (srv *server) StartServiceInstance(ctx context.Context, rqst *services_managerpb.StartServiceInstanceRequest) (*services_managerpb.StartServiceInstanceResponse, error) {
	if err := srv.startServiceInstance(rqst.ServiceId); err != nil {
		logger.Error("start service failed", "serviceId", rqst.ServiceId, "err", err)
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	logger.Info("service started", "serviceId", rqst.ServiceId)
	return &services_managerpb.StartServiceInstanceResponse{}, nil
}

// RestartAllServices stops and then restarts all services except the service-manager itself.
// Public signature preserved.
func (srv *server) RestartAllServices(ctx context.Context, rqst *services_managerpb.RestartAllServicesRequest) (*services_managerpb.RestartAllServicesResponse, error) {
	services, err := config.GetServicesConfigurations()
	if err != nil {
		logger.Error("get services configurations failed", "err", err)
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	for _, s := range services {
		id := Utility.ToString(s["Id"])
		if id == srv.GetId() {
			continue
		}
		if err := srv.stopServiceInstance(id); err != nil {
			logger.Error("stop service during restart-all failed", "serviceId", id, "err", err)
			return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
	}

	for _, s := range services {
		id := Utility.ToString(s["Id"])
		if id == srv.GetId() {
			continue
		}
		if err := srv.startServiceInstance(id); err != nil {
			logger.Error("start service during restart-all failed", "serviceId", id, "err", err)
			return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
	}

	logger.Info("all services restarted")
	return &services_managerpb.RestartAllServicesResponse{}, nil
}

// GetServicesConfiguration returns the list of all service configurations (as Struct).
// Public signature preserved.
func (srv *server) GetServicesConfiguration(ctx context.Context, rqst *services_managerpb.GetServicesConfigurationRequest) (*services_managerpb.GetServicesConfigurationResponse, error) {
	services, err := config.GetServicesConfigurations()
	if err != nil {
		logger.Error("get services configurations failed", "err", err)
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	resp := &services_managerpb.GetServicesConfigurationResponse{Services: make([]*structpb.Struct, len(services))}
	for i := range services {
		resp.Services[i], _ = structpb.NewStruct(services[i])
	}
	return resp, nil
}

// SaveServiceConfig persists a given service configuration (JSON), restarts the service,
// and broadcasts the update event.
// Public signature preserved.
func (srv *server) SaveServiceConfig(ctx context.Context, rqst *services_managerpb.SaveServiceConfigRequest) (*services_managerpb.SaveServiceConfigResponse, error) {
	cfg := make(map[string]interface{})
	if err := json.Unmarshal([]byte(rqst.Config), &cfg); err != nil {
		logger.Error("invalid service config JSON", "err", err)
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	if err := config.SaveServiceConfiguration(cfg); err != nil {
		logger.Error("save service configuration failed", "serviceId", cfg["Id"], "err", err)
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	id := Utility.ToString(cfg["Id"])
	if err := srv.stopServiceInstance(id); err != nil {
		logger.Error("stop service after save config failed", "serviceId", id, "err", err)
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	if err := srv.startServiceInstance(id); err != nil {
		logger.Error("start service after save config failed", "serviceId", id, "err", err)
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	if err := srv.publishUpdateServiceConfigEvent(cfg); err != nil {
		logger.Warn("publish update service config event failed", "serviceId", id, "err", err)
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	logger.Info("service configuration saved and service restarted", "serviceId", id)
	return &services_managerpb.SaveServiceConfigResponse{}, nil
}

// GetAllActions parses all known .proto files referenced by service configs,
// and returns the list of fully-qualified gRPC method paths (e.g. /pkg.Service/Method).
// Public signature preserved.
func (srv *server) GetAllActions(ctx context.Context, rqst *services_managerpb.GetAllActionsRequest) (*services_managerpb.GetAllActionsResponse, error) {
	services, err := config.GetServicesConfigurations()
	if err != nil {
		logger.Error("get services configurations failed", "err", err)
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	actions := make([]string, 0, 64)

	for _, svc := range services {
		path := Utility.ToString(svc["Proto"])
		if path == "" || !Utility.Exists(path) {
			// skip services without on-disk proto
			continue
		}

		reader, err := os.Open(path)
		if err != nil {
			logger.Warn("open proto failed", "path", path, "err", err)
			continue
		}

		def, perr := proto.NewParser(reader).Parse()
		_ = reader.Close()
		if perr != nil {
			logger.Warn("parse proto failed", "path", path, "err", perr)
			continue
		}

		var pkg, svcName string
		proto.Walk(def,
			proto.WithPackage(func(p *proto.Package) { pkg = p.Name }),
			proto.WithService(func(s *proto.Service) { svcName = s.Name }),
			proto.WithRPC(func(r *proto.RPC) {
				if pkg != "" && svcName != "" && r != nil {
					actions = append(actions, "/"+pkg+"."+svcName+"/"+r.Name)
				}
			}),
		)
	}

	return &services_managerpb.GetAllActionsResponse{Actions: actions}, nil
}
