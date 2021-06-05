package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"
	"github.com/davecourtois/Utility"
	globular "github.com/globulario/services/golang/globular_service"
	"github.com/globulario/services/golang/repository/repository_client"
	"github.com/globulario/services/golang/resource/resource_client"
	"github.com/globulario/services/golang/resource/resourcepb"
	"github.com/globulario/services/golang/services_manager/services_managerpb"
	"github.com/globulario/services/golang/config"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Uninstall a service...
func (server *server) UninstallService(ctx context.Context, rqst *services_managerpb.UninstallServiceRequest) (*services_managerpb.UninstallServiceResponse, error) {
	err := server.uninstallService(rqst.PublisherId, rqst.ServiceId, rqst.Version, rqst.DeletePermissions)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &services_managerpb.UninstallServiceResponse{
		Result: true,
	}, nil
}

// Install/Update a service on globular instance.
// file postinst, postrm, preinst, postinst
func (server *server) installService(descriptor *resourcepb.PackageDescriptor) error {
	// repository must exist...
	if len(descriptor.Repositories) == 0 {
		return errors.New("No service repository was found for service " + descriptor.Id)
	}

	for i := 0; i < len(descriptor.Repositories); i++ {
		services_repository, err := repository_client.NewRepositoryService_Client(descriptor.Repositories[i], "repository.PackageRepository")
		if err != nil {
			return err
		}

		bundle, err := services_repository.DownloadBundle(descriptor, globular.GetPlatform())

		if err == nil {

			previous, _ := config.GetServicesConfigurationsById(descriptor.Id)
			if previous != nil {
				// Uninstall the previous version...
				server.uninstallService(descriptor.PublisherId, descriptor.Id, descriptor.Version, false)
			}

			// Create the file.
			r := bytes.NewReader(bundle.Binairies)
			_extracted_path_, err := Utility.ExtractTarGz(r)
			if err != nil {
				return err
			}

			defer os.RemoveAll(_extracted_path_)

			// I will save the binairy in file...
			Utility.CreateDirIfNotExist(server.Root + "/services/")
			err = Utility.CopyDir(_extracted_path_+"/"+descriptor.PublisherId, server.Root+"/services/")

			if err != nil {
				return err
			}

			path := server.Root + "/services/" + descriptor.PublisherId + "/" + descriptor.Name + "/" + descriptor.Version + "/" + descriptor.Id

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

			protos, _ := Utility.FindFileByName(server.Root+"/services/"+descriptor.PublisherId+"/"+descriptor.Name+"/"+descriptor.Version, ".proto")
			if len(protos) == 0 {
				return errors.New("no configuration file was found")
			}

			// I will replace the path inside the config...
			execName := s["Path"].(string)[strings.LastIndex(s["Path"].(string), "/")+1:]
			s["Path"] = path + "/" + execName
			s["Proto"] = protos[0]

			// Here I will get previous service values...
			if previous != nil {
				s["KeepAlive"] = previous["KeepAlive"].(bool)
				s["KeepUpToDate"] =previous["KeepUpToDate"].(bool)
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
				if !Utility.Contains(server.Discoveries, descriptor.Discoveries[i]) {
					server.Discoveries = append(server.Discoveries, descriptor.Discoveries[i])
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
				server.Save()
			}

			break
		} else {
			return err
		}
	}

	return nil
}

// Install/Update a service on globular instance.
func (server *server) InstallService(ctx context.Context, rqst *services_managerpb.InstallServiceRequest) (*services_managerpb.InstallServiceResponse, error) {

	// Connect to the dicovery services
	resource_client_, err := resource_client.NewResourceService_Client(rqst.DicorveryId, "resource.ResourceService")

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
	err = server.installService(descriptor)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &services_managerpb.InstallServiceResponse{
		Result: true,
	}, nil

}

// Stop a service
func (server *server) StopServiceInstance(ctx context.Context, rqst *services_managerpb.StopServiceInstanceRequest) (*services_managerpb.StopServiceInstanceResponse, error) {

	s, err := config.GetServicesConfigurationsById(rqst.ServiceId)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	
	if s != nil {
		err := server.stopService(s)
		if err != nil {
			return nil, status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
	} else {
		// Close all services with a given name.
		services, err := config.GetServicesConfigurationsByName(rqst.ServiceId)
		if err != nil {
			return nil, status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}

		for i := 0; i < len(services); i++ {
			serviceId := services[i]["Id"].(string)
			s, err := config.GetServicesConfigurationsById(serviceId)
			if err != nil {
				return nil, status.Errorf(
					codes.Internal,
					Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
			}

			if s == nil {
				return nil, status.Errorf(
					codes.Internal,
					Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("No service found with id "+serviceId)))
			}

			err = server.stopService(s)
			if err != nil {
				return nil, status.Errorf(
					codes.Internal,
					Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
			}
		}
	}

	return &services_managerpb.StopServiceInstanceResponse{
		Result: true,
	}, nil
}

// Start a service
func (server *server) StartServiceInstance(ctx context.Context, rqst *services_managerpb.StartServiceInstanceRequest) (*services_managerpb.StartServiceInstanceResponse, error) {
	return nil, errors.New("not implemented")
}

// Restart all Services also the http(s)
func (server *server) RestartAllServices(ctx context.Context, rqst *services_managerpb.RestartAllServicesRequest) (*services_managerpb.RestartAllServicesResponse, error) {
	return nil, errors.New("not implemented")
}
