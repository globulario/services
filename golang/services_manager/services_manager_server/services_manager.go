package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"

	//"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"

	"github.com/davecourtois/Utility"
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

	err = srv.uninstallService(token, rqst.PublisherId, rqst.ServiceId, rqst.Version, rqst.DeletePermissions)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &services_managerpb.UninstallServiceResponse{
		Result: true,
	}, nil
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
// file postinst, postrm, preinst, postinst
func (srv *server) installService(token string, descriptor *resourcepb.PackageDescriptor) error {
	// repository must exist...
	if len(descriptor.Repositories) == 0 {
		return errors.New("No service repository was found for service " + descriptor.Id)
	}

	for i := 0; i < len(descriptor.Repositories); i++ {
		services_repository, err := GetRepositoryClient(descriptor.Repositories[i])
		if err != nil {
			return err
		}

		bundle, err := services_repository.DownloadBundle(descriptor, globular.GetPlatform())

		if err == nil {

			previous, _ := config.GetServiceConfigurationById(descriptor.Id)
			if previous != nil {
				// Uninstall the previous version...
				srv.uninstallService(token, descriptor.PublisherId, descriptor.Id, descriptor.Version, false)
			}

			// Create the file.
			r := bytes.NewReader(bundle.Binairies)
			_extracted_path_, err := Utility.ExtractTarGz(r)
			if err != nil {
				return err
			}

			defer os.RemoveAll(_extracted_path_)

			// I will save the binairy in file...
			Utility.CreateDirIfNotExist(srv.Root + "/services/")
			err = Utility.CopyDir(_extracted_path_+"/"+descriptor.PublisherId, srv.Root+"/services/")

			if err != nil {
				return err
			}

			path := srv.Root + "/services/" + descriptor.PublisherId + "/" + descriptor.Name + "/" + descriptor.Version + "/" + descriptor.Id

			// before I will start the service I will get a look if preinst script must be run...
			if Utility.Exists(path + "/preinst") {
				// I that case I will run it...
				script := exec.Command("/bin/sh", path+"/preinst")
				err := script.Run()
				if err != nil {
					defer os.RemoveAll(path)
					return err
				}
			}

			configs, _ := Utility.FindFileByName(path, "config.json")

			if len(configs) == 0 {
				return errors.New("no configuration file was found")
			}

			s := make(map[string]interface{})
			data, err := ioutil.ReadFile(configs[0])
			if err != nil {
				return err
			}
			err = json.Unmarshal(data, &s)
			if err != nil {
				return err
			}

			protos, _ := Utility.FindFileByName(srv.Root+"/services/"+descriptor.PublisherId+"/"+descriptor.Name+"/"+descriptor.Version, ".proto")
			if len(protos) == 0 {
				return errors.New("no service was found")
			}

			// I will replace the path inside the config...
			execName := s["Path"].(string)[strings.LastIndex(s["Path"].(string), "/")+1:]
			s["Path"] = path + "/" + execName
			s["Proto"] = protos[0]

			// Here I will get previous service values...
			if previous != nil {
				s["KeepAlive"] = previous["KeepAlive"].(bool)
				s["KeepUpToDate"] = previous["KeepUpToDate"].(bool)
			}

			err = os.Chmod(s["Path"].(string), 0755)
			if err != nil {
				return err
			}

			jsonStr, _ := Utility.ToJson(s)
			ioutil.WriteFile(configs[0], []byte(jsonStr), 0644)

			// Append to the list of service discoveries.
			needSave := false
			for i := 0; i < len(descriptor.Discoveries); i++ {
				if !Utility.Contains(srv.Discoveries, descriptor.Discoveries[i]) {
					srv.Discoveries = append(srv.Discoveries, descriptor.Discoveries[i])
					needSave = true
				}
			}

			if Utility.Exists(path + "/postinst") {
				// I that case I will run it...
				script := exec.Command("/bin/sh", path+"/postinst")
				err := script.Run()
				if err != nil {
					defer os.RemoveAll(path)
					return err
				}
			}

			if needSave {
				// save the service manager configuration itself.
				srv.Save()
			}

			break
		} else {
			return err
		}
	}

	return nil
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

	// Connect to the dicovery services
	resource_client_, err := GetResourceClient(rqst.DicorveryId)

	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("fail to connect to "+rqst.DicorveryId)))
	}

	descriptor, err := resource_client_.GetPackageDescriptor(rqst.ServiceId, rqst.PublisherId, rqst.Version)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// The first element in the array is the most recent descriptor
	// so if no version is given the most recent will be taken.
	err = srv.installService(token, descriptor)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &services_managerpb.InstallServiceResponse{
		Result: true,
	}, nil

}

func (srv *server) stopServiceInstance(serviceId string) error {
	if serviceId == srv.GetId() {
		return errors.New("The service manager could not stop itself!")
	}
	s, err := config.GetServiceConfigurationById(serviceId)
	if err != nil {
		return err
	}

	if s != nil {
		err := srv.stopService(s)
		if err != nil {
			return err
		}
	} else {
		// Close all services with a given name.
		services, err := config.GetServicesConfigurationsByName(serviceId)
		if err != nil {
			return err
		}

		for i := 0; i < len(services); i++ {
			serviceId := services[i]["Id"].(string)
			s, err := config.GetServiceConfigurationById(serviceId)
			if err != nil {
				return err
			}

			if s == nil {
				return errors.New("No service found with id " + serviceId)
			}

			err = srv.stopService(s)
			if err != nil {
				return err
			}

		}
	}

	return nil
}

// Stop a service
func (srv *server) StopServiceInstance(ctx context.Context, rqst *services_managerpb.StopServiceInstanceRequest) (*services_managerpb.StopServiceInstanceResponse, error) {

	err := srv.stopServiceInstance(rqst.ServiceId)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &services_managerpb.StopServiceInstanceResponse{
		Result: true,
	}, nil
}

func (srv *server) startServiceInstance(serviceId string) error {
	if serviceId == srv.GetId() {
		return errors.New("the service manager could not start itself")
	}

	// here I will read the server configuration file...
	globular := make(map[string]interface{})
	file, err := ioutil.ReadFile(config.GetConfigDir() + "/config.json")
	// Init the service with the default port address
	if err == nil {
		err := json.Unmarshal(file, &globular)
		if err != nil {
			return err
		}
	} else {
		return err
	}

	s, err := config.GetServiceConfigurationById(serviceId)
	if err != nil {
		return err
	}

	port := Utility.ToInt(s["Port"])
	proxyPort := Utility.ToInt(s["ProxyPort"])

	processPid, err := process.StartServiceProcess(s, port, proxyPort)
	if err != nil {
		return err
	}

	s["Process"] = processPid

	// save the service configuration
	s["ProxyProcess"], err = Utility.GetProcessIdsByName("envoy")
	if err != nil {
		return err
	}

	return srv.publishUpdateServiceConfigEvent(s)
}

// Start a service
func (srv *server) StartServiceInstance(ctx context.Context, rqst *services_managerpb.StartServiceInstanceRequest) (*services_managerpb.StartServiceInstanceResponse, error) {
	err := srv.startServiceInstance(rqst.ServiceId)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &services_managerpb.StartServiceInstanceResponse{}, nil
}

// Restart all Services also the http(s)
func (srv *server) RestartAllServices(ctx context.Context, rqst *services_managerpb.RestartAllServicesRequest) (*services_managerpb.RestartAllServicesResponse, error) {
	services, err := config.GetServicesConfigurations()
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// stop all serives...
	for i := 0; i < len(services); i++ {
		if services[i]["Id"].(string) != srv.GetId() {
			err := srv.stopServiceInstance(services[i]["Id"].(string))
			if err != nil {
				return nil, status.Errorf(
					codes.Internal,
					Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
			}
		}
	}

	for i := 0; i < len(services); i++ {
		if services[i]["Id"].(string) != srv.GetId() {
			err := srv.startServiceInstance(services[i]["Id"].(string))
			if err != nil {
				return nil, status.Errorf(
					codes.Internal,
					Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
			}
		}
	}

	return &services_managerpb.RestartAllServicesResponse{}, nil
}

func (srv *server) GetServicesConfiguration(ctx context.Context, rqst *services_managerpb.GetServicesConfigurationRequest) (*services_managerpb.GetServicesConfigurationResponse, error) {
	services, err := config.GetServicesConfigurations()
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	rsp := &services_managerpb.GetServicesConfigurationResponse{}

	rsp.Services = make([]*structpb.Struct, len(services))

	// Now I will set the value in the results array...
	for i := 0; i < len(services); i++ {
		rsp.Services[i], _ = structpb.NewStruct(services[i])
	}

	return rsp, nil
}

/**
 * Save service configuration.
 */
func (srv *server) SaveServiceConfig(ctx context.Context, rqst *services_managerpb.SaveServiceConfigRequest) (*services_managerpb.SaveServiceConfigResponse, error) {

	s := make(map[string]interface{})
	err := json.Unmarshal([]byte(rqst.Config), &s)

	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// Save the service configuration
	err = config.SaveServiceConfiguration(s)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// Stop and start the services
	// here I will use brut force by restarting the service itself...
	err = srv.stopServiceInstance(s["Id"].(string))
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	err = srv.startServiceInstance(s["Id"].(string))
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	err = srv.publishUpdateServiceConfigEvent(s)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &services_managerpb.SaveServiceConfigResponse{}, nil
}

/**
 * That function return the list of all actions.
 */
func (srv *server) GetAllActions(ctx context.Context, rqst *services_managerpb.GetAllActionsRequest) (*services_managerpb.GetAllActionsResponse, error) {

	// first of all I will retreive the list of all services configuration.
	services, err := config.GetServicesConfigurations()
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// the actions retreived...
	actions := make([]string, 0)

	// Now I will all protofile and extract methods names.
	for i := 0; i < len(services); i++ {
		path := services[i]["Proto"].(string)

		// here I will parse the service defintion file to extract the
		// service difinition.
		reader, _ := os.Open(path)
		defer reader.Close()

		parser := proto.NewParser(reader)
		definition, _ := parser.Parse()

		// Stack values from walking tree
		stack := make([]interface{}, 0)

		handlePackage := func(stack *[]interface{}) func(*proto.Package) {
			return func(p *proto.Package) {
				*stack = append(*stack, p)
			}
		}(&stack)

		handleService := func(stack *[]interface{}) func(*proto.Service) {
			return func(s *proto.Service) {
				*stack = append(*stack, s)
			}
		}(&stack)

		handleRpc := func(stack *[]interface{}) func(*proto.RPC) {
			return func(r *proto.RPC) {
				*stack = append(*stack, r)
			}
		}(&stack)

		// Walk this way
		proto.Walk(definition,
			proto.WithPackage(handlePackage),
			proto.WithService(handleService),
			proto.WithRPC(handleRpc))

		var packageName string
		var serviceName string
		var methodName string

		for len(stack) > 0 {
			var x interface{}
			x, stack = stack[0], stack[1:]
			switch v := x.(type) {
			case *proto.Package:
				packageName = v.Name
			case *proto.Service:
				serviceName = v.Name
			case *proto.RPC:
				methodName = v.Name
				path := "/" + packageName + "." + serviceName + "/" + methodName
				// So here I will register the method into the backend.
				actions = append(actions, path)
			}
		}
	}

	return &services_managerpb.GetAllActionsResponse{
		Actions: actions,
	}, nil
}
