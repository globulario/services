package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/emicklei/proto"
	"github.com/globulario/services/golang/config"
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
	"google.golang.org/protobuf/types/known/structpb"
)

// Uninstall a service...
func (srv *server) UninstallService(ctx context.Context, rqst *services_managerpb.UninstallServiceRequest) (*services_managerpb.UninstallServiceResponse, error) {
	_, token, err := security.GetClientId(ctx)
	if err != nil {
		return nil, err
	}
	if err := srv.uninstallService(token, rqst.PublisherID, rqst.ServiceId, rqst.Version, rqst.DeletePermissions); err != nil {
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	return &services_managerpb.UninstallServiceResponse{Result: true}, nil
}

func GetRepositoryClient(address string) (*repository_client.Repository_Service_Client, error) {
	Utility.RegisterFunction("NewRepositoryService_Client", repository_client.NewRepositoryService_Client)
	client, err := globular_client.GetClient(address, "repository.PackageRepository", "NewRepositoryService_Client")
	if err != nil {
		return nil, err
	}
	return client.(*repository_client.Repository_Service_Client), nil
}

// Install/Update a service on globular instance.
func (srv *server) installService(token string, descriptor *resourcepb.PackageDescriptor) error {
	if len(descriptor.Repositories) == 0 {
		return errors.New("no service repository was found for service " + descriptor.Id)
	}

	for i := 0; i < len(descriptor.Repositories); i++ {
		repoClient, err := GetRepositoryClient(descriptor.Repositories[i])
		if err != nil {
			return err
		}

		bundle, err := repoClient.DownloadBundle(descriptor, globular.GetPlatform())
		if err != nil {
			return err
		}

		// Uninstall any existing version first (best-effort)
		if prev, _ := config.GetServiceConfigurationById(descriptor.Id); prev != nil {
			_ = srv.uninstallService(token, descriptor.PublisherID, descriptor.Id, Utility.ToString(prev["Version"]), false)
		}

		// Extract bundle
		r := bytes.NewReader(bundle.Binairies)
		extractedPath, err := Utility.ExtractTarGz(r)
		if err != nil {
			return err
		}
		defer os.RemoveAll(extractedPath)

		// Copy into services tree
		base := srv.Root + "/services/"
		if err := Utility.CreateDirIfNotExist(base); err != nil {
			return err
		}
		if err := Utility.CopyDir(extractedPath+"/"+descriptor.PublisherID, base); err != nil {
			return err
		}

		installRoot := base + descriptor.PublisherID + "/" + descriptor.Name + "/" + descriptor.Version + "/" + descriptor.Id

		// preinst
		if Utility.Exists(installRoot + "/preinst") {
			if err := exec.Command("/bin/sh", installRoot+"/preinst").Run(); err != nil {
				defer os.RemoveAll(installRoot)
				return err
			}
		}

		// Locate executable
		execPath, err := findServiceExecutable(installRoot, descriptor.Name, descriptor.Id)
		if err != nil {
			return err
		}
		if err := os.Chmod(execPath, 0o755); err != nil {
			return err
		}

		// Locate proto (we copy proto under <pub>/<name>/<version>/ by packaging code)
		protos, _ := Utility.FindFileByName(srv.Root+"/services/"+descriptor.PublisherID+"/"+descriptor.Name+"/"+descriptor.Version, ".proto")
		if len(protos) == 0 {
			// fallback: search inside installRoot
			protos, _ = Utility.FindFileByName(installRoot, ".proto")
			if len(protos) == 0 {
				return errors.New("no .proto file found for service " + descriptor.Id)
			}
		}
		protoPath := protos[0]

		// Preserve previous runtime prefs if any
		prev, _ := config.GetServiceConfigurationById(descriptor.Id)

		// Build desired config to save in etcd
		s := map[string]interface{}{
			"Id":           descriptor.Id,
			"Name":         descriptor.Name,
			"PublisherID":  descriptor.PublisherID,
			"Version":      descriptor.Version,
			"Description":  descriptor.Description,
			"Keywords":     descriptor.Keywords,
			"Path":         strings.ReplaceAll(execPath, "\\", "/"),
			"Proto":        strings.ReplaceAll(protoPath, "\\", "/"),
			"Repositories": toIfaceSlice(descriptor.Repositories),
			"Discoveries":  toIfaceSlice(descriptor.Discoveries),
			// defaults / preserved
			"KeepAlive":       true,
			"KeepUpToDate":    false,
			"AllowAllOrigins": true,
			"AllowedOrigins":  "",
			"TLS":             false,
			"Port":            0,  // will be assigned at start
			"Proxy":           0,  // will be assigned at start
			"Process":         -1, // runtime
			"ProxyProcess":    -1, // runtime
			"State":           "stopped",
			"LastError":       "",
		}

		if prev != nil {
			if v, ok := prev["KeepAlive"]; ok {
				s["KeepAlive"] = v
			}
			if v, ok := prev["KeepUpToDate"]; ok {
				s["KeepUpToDate"] = v
			}
			// preserve custom CORS/TLS if they existed
			if v, ok := prev["TLS"]; ok {
				s["TLS"] = v
			}
			if v, ok := prev["AllowAllOrigins"]; ok {
				s["AllowAllOrigins"] = v
			}
			if v, ok := prev["AllowedOrigins"]; ok {
				s["AllowedOrigins"] = v
			}
		}

		// Save desired/runtime into etcd
		if err := config.SaveServiceConfiguration(s); err != nil {
			return err
		}

		// Post-install hook
		if Utility.Exists(installRoot + "/postinst") {
			if err := exec.Command("/bin/sh", installRoot+"/postinst").Run(); err != nil {
				defer os.RemoveAll(installRoot)
				return err
			}
		}

		// Merge server discoveries with descriptor ones and persist service-manager if changed
		needSave := false
		for _, d := range descriptor.Discoveries {
			if !Utility.Contains(srv.Discoveries, d) {
				srv.Discoveries = append(srv.Discoveries, d)
				needSave = true
			}
		}
		if needSave {
			_ = srv.Save()
		}

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

func findServiceExecutable(dir string, name string, id string) (string, error) {
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
		return "", err
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
	return "", errors.New("no executable found in " + dir)
}

func GetResourceClient(address string) (*resource_client.Resource_Client, error) {
	Utility.RegisterFunction("NewResourceService_Client", resource_client.NewResourceService_Client)
	client, err := globular_client.GetClient(address, "resource.ResourceService", "NewResourceService_Client")
	if err != nil {
		return nil, err
	}
	return client.(*resource_client.Resource_Client), nil
}

// Install/Update a service on globular instance.
func (srv *server) InstallService(ctx context.Context, rqst *services_managerpb.InstallServiceRequest) (*services_managerpb.InstallServiceResponse, error) {
	_, token, err := security.GetClientId(ctx)
	if err != nil {
		return nil, err
	}
	resourceClient, err := GetResourceClient(rqst.DicorveryId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("fail to connect to "+rqst.DicorveryId)))
	}
	descriptor, err := resourceClient.GetPackageDescriptor(rqst.ServiceId, rqst.PublisherID, rqst.Version)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	if err := srv.installService(token, descriptor); err != nil {
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	return &services_managerpb.InstallServiceResponse{Result: true}, nil
}

func (srv *server) stopServiceInstance(serviceId string) error {
	if serviceId == srv.GetId() {
		return errors.New("the service manager could not stop itself")
	}
	s, err := config.GetServiceConfigurationById(serviceId)
	if err != nil {
		return err
	}
	if s != nil {
		return srv.stopService(s)
	}

	services, err := config.GetServicesConfigurationsByName(serviceId)
	if err != nil {
		return err
	}
	for _, sc := range services {
		id := sc["Id"].(string)
		cfg, err := config.GetServiceConfigurationById(id)
		if err != nil {
			return err
		}
		if cfg == nil {
			return errors.New("no service found with id " + id)
		}
		if err := srv.stopService(cfg); err != nil {
			return err
		}
	}
	return nil
}

// Stop a service
func (srv *server) StopServiceInstance(ctx context.Context, rqst *services_managerpb.StopServiceInstanceRequest) (*services_managerpb.StopServiceInstanceResponse, error) {
	if err := srv.stopServiceInstance(rqst.ServiceId); err != nil {
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	return &services_managerpb.StopServiceInstanceResponse{Result: true}, nil
}

func (srv *server) startServiceInstance(serviceId string) error {
	if serviceId == srv.GetId() {
		return errors.New("the service manager could not start itself")
	}

	// Read global server config (still file-based)
	globularCfg := make(map[string]interface{})
	data, err := ioutil.ReadFile(config.GetConfigDir() + "/config.json")
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, &globularCfg); err != nil {
		return err
	}

	s, err := config.GetServiceConfigurationById(serviceId)
	if err != nil {
		return err
	}

	port := Utility.ToInt(s["Port"])
	pid, err := process.StartServiceProcess(s, port)
	if err != nil {
		return err
	}

	s["Process"] = pid
	s["State"] = "running"
	return srv.publishUpdateServiceConfigEvent(s)
}

// Start a service
func (srv *server) StartServiceInstance(ctx context.Context, rqst *services_managerpb.StartServiceInstanceRequest) (*services_managerpb.StartServiceInstanceResponse, error) {
	if err := srv.startServiceInstance(rqst.ServiceId); err != nil {
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	return &services_managerpb.StartServiceInstanceResponse{}, nil
}

// Restart all Services also the http(s)
func (srv *server) RestartAllServices(ctx context.Context, rqst *services_managerpb.RestartAllServicesRequest) (*services_managerpb.RestartAllServicesResponse, error) {
	services, err := config.GetServicesConfigurations()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	for _, s := range services {
		if s["Id"].(string) != srv.GetId() {
			if err := srv.stopServiceInstance(s["Id"].(string)); err != nil {
				return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
			}
		}
	}
	for _, s := range services {
		if s["Id"].(string) != srv.GetId() {
			if err := srv.startServiceInstance(s["Id"].(string)); err != nil {
				return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
			}
		}
	}
	return &services_managerpb.RestartAllServicesResponse{}, nil
}

func (srv *server) GetServicesConfiguration(ctx context.Context, rqst *services_managerpb.GetServicesConfigurationRequest) (*services_managerpb.GetServicesConfigurationResponse, error) {
	services, err := config.GetServicesConfigurations()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	rsp := &services_managerpb.GetServicesConfigurationResponse{Services: make([]*structpb.Struct, len(services))}
	for i := range services {
		rsp.Services[i], _ = structpb.NewStruct(services[i])
	}
	return rsp, nil
}

func (srv *server) SaveServiceConfig(ctx context.Context, rqst *services_managerpb.SaveServiceConfigRequest) (*services_managerpb.SaveServiceConfigResponse, error) {
	s := make(map[string]interface{})
	if err := json.Unmarshal([]byte(rqst.Config), &s); err != nil {
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	if err := config.SaveServiceConfiguration(s); err != nil {
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	if err := srv.stopServiceInstance(s["Id"].(string)); err != nil {
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	if err := srv.startServiceInstance(s["Id"].(string)); err != nil {
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	if err := srv.publishUpdateServiceConfigEvent(s); err != nil {
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	return &services_managerpb.SaveServiceConfigResponse{}, nil
}

// Collect all gRPC action paths by parsing .proto files from etcd-provided configs.
func (srv *server) GetAllActions(ctx context.Context, rqst *services_managerpb.GetAllActionsRequest) (*services_managerpb.GetAllActionsResponse, error) {
	services, err := config.GetServicesConfigurations()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	actions := make([]string, 0)

	for _, svc := range services {
		path := Utility.ToString(svc["Proto"])
		if path == "" || !Utility.Exists(path) {
			// skip services without proto on disk
			continue
		}
		reader, _ := os.Open(path)
		defer reader.Close()

		parser := proto.NewParser(reader)
		definition, _ := parser.Parse()

		stack := make([]interface{}, 0)
		proto.Walk(definition,
			proto.WithPackage(func(p *proto.Package) { stack = append(stack, p) }),
			proto.WithService(func(s *proto.Service) { stack = append(stack, s) }),
			proto.WithRPC(func(r *proto.RPC) { stack = append(stack, r) }),
		)

		var pkg, svcName string
		for len(stack) > 0 {
			var x interface{}
			x, stack = stack[0], stack[1:]
			switch v := x.(type) {
			case *proto.Package:
				pkg = v.Name
			case *proto.Service:
				svcName = v.Name
			case *proto.RPC:
				actions = append(actions, "/"+pkg+"."+svcName+"/"+v.Name)
			}
		}
	}

	return &services_managerpb.GetAllActionsResponse{Actions: actions}, nil
}
