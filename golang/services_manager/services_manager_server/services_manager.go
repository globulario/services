package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"strings"
	"sync"

	"github.com/davecourtois/Utility"
	"github.com/globulario/services/golang/discovery/discovery_client"
	globular "github.com/globulario/services/golang/globular_service"
	"github.com/globulario/services/golang/repository/repository_client"
	"github.com/globulario/services/golang/resource/resourcepb"
	"github.com/globulario/services/golang/services_manager/services_managerpb"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/structpb"
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
	log.Println("step 2: try to dowload service bundle")
	if len(descriptor.Repositories) == 0 {
		return errors.New("No service repository was found for service " + descriptor.Id)
	}

	for i := 0; i < len(descriptor.Repositories); i++ {
		services_repository, err := repository_client.NewRepositoryService_Client(descriptor.Repositories[i], "packages.PackageRepository")
		if err != nil {
			return err
		}

		log.Println("try to download service from ", descriptor.Repositories[i])
		bundle, err := services_repository.DownloadBundle(descriptor, globular.GetPlatform())

		if err == nil {

			previous := server.getService(descriptor.Id)
			if previous != nil {
				// Uninstall the previous version...
				server.uninstallService(descriptor.PublisherId, descriptor.Id, descriptor.Version, false)
			}

			// Create the file.
			r := bytes.NewReader(bundle.Binairies)
			_extracted_path_, err := Utility.ExtractTarGz(r)
			defer os.RemoveAll(_extracted_path_)
			if err != nil {
				if err.Error() != "EOF" {
					// report the error and try to continue...
					log.Println(err)
				}
			}

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
				log.Println("run preinst script please wait...")
				script := exec.Command("/bin/sh", path+"/preinst")
				err := script.Run()
				if err != nil {
					log.Println("error with script ", err.Error())
					defer os.RemoveAll(path)
					return err
				}
				log.Println("preinst script was execute with success! ")
			}

			configs, _ := Utility.FindFileByName(path, "config.json")

			if len(configs) == 0 {
				log.Println("No configuration file was found at at path ", path)
				return errors.New("no configuration file was found")
			}

			s := make(map[string]interface{}, 0)
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
				log.Println("No prototype file was found at at path ", server.Root+"/services/"+descriptor.PublisherId+"/"+descriptor.Name+"/"+descriptor.Version)
				return errors.New("no configuration file was found")
			}

			// I will replace the path inside the config...
			execName := s["Path"].(string)[strings.LastIndex(s["Path"].(string), "/")+1:]
			s["Path"] = path + "/" + execName
			s["Proto"] = protos[0]

			// Here I will get previous service values...
			if previous != nil {
				s["KeepAlive"] = getBoolVal(previous, "KeepAlive")
				s["KeepUpToDate"] = getBoolVal(previous, "KeepUpToDate")
			}

			err = os.Chmod(s["Path"].(string), 0755)
			if err != nil {
				log.Println(err)
			}

			jsonStr, _ := Utility.ToJson(s)
			ioutil.WriteFile(configs[0], []byte(jsonStr), 0644)

			// set the service in the map.
			s_ := new(sync.Map)
			setValues(s_, s)
			log.Println("Service is install successfully!")

			// initialyse the new service.
			err = server.initService(s_)
			if err != nil {
				return err
			}

			// Here I will set the service method...
			server.setServiceMethods(s["Name"].(string), s["Proto"].(string))
			server.registerMethods()

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
					log.Println("error with script ", err.Error())
					defer os.RemoveAll(path)
					return err
				}
				log.Println("running script ", path)
			}

			if needSave {
				server.saveConfig()
			}

			break
		} else {
			log.Println("fail to download error with error ", err)
			return err
		}
	}

	return nil

}

// Install/Update a service on globular instance.
func (server *server) InstallService(ctx context.Context, rqst *services_managerpb.InstallServiceRequest) (*services_managerpb.InstallServiceResponse, error) {
	log.Println("Try to install new service ", rqst.ServiceId, "from", rqst.DicorveryId)

	// Connect to the dicovery services
	services_discovery, err := discovery_client.NewDiscoveryService_Client(rqst.DicorveryId, "packages.PackageDiscovery")

	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("fail to connect to "+rqst.DicorveryId)))
	}

	descriptors, err := services_discovery.GetPackageDescriptor(rqst.ServiceId, rqst.PublisherId)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	log.Println("step 1: get service descriptor")
	// The first element in the array is the most recent descriptor
	// so if no version is given the most recent will be taken.
	descriptor := descriptors[0]
	for i := 0; i < len(descriptors); i++ {
		if descriptors[i].Version == rqst.Version {
			descriptor = descriptors[i]
			break
		}
	}

	err = server.installService(descriptor)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	log.Println("Service was install!")
	return &services_managerpb.InstallServiceResponse{
		Result: true,
	}, nil

}

// Stop a service
func (server *server) StopServiceInstance(ctx context.Context, rqst *services_managerpb.StopServiceInstanceRequest) (*services_managerpb.StopServiceInstanceResponse, error) {

	s := server.getService(rqst.ServiceId)
	if s != nil {
		err := server.stopService(s)
		if err != nil {
			return nil, status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
	} else {
		// Close all services with a given name.
		services := server.getServiceConfigByName(rqst.ServiceId)
		for i := 0; i < len(services); i++ {
			serviceId := services[i]["Id"].(string)
			s := server.getService(serviceId)
			if s == nil {
				return nil, status.Errorf(
					codes.Internal,
					Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("No service found with id "+serviceId)))
			}
			err := server.stopService(s)
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

// Return the list of services configurations.
func (server *server) GetServicesConfig(ctx context.Context, rqst *services_managerpb.GetServicesConfigRequest) (*services_managerpb.GetServicesConfigResponse, error) {
	configs := server.getServicesConfig()

	configs_ := make([]*structpb.Struct, 0)
	for i := 0; i < len(configs); i++ {
		config, err := structpb.NewStruct(configs[i])
		if err != nil {
			return nil, status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
		configs_ = append(configs_, config)
	}

	return &services_managerpb.GetServicesConfigResponse{
		Configs: configs_,
	}, nil
}

// Return the config of a particular service from it id.
func (server *server) GetServiceConfig(ctx context.Context, rqst *services_managerpb.GetServiceConfigRequest) (*services_managerpb.GetServiceConfigResponse, error) {
	return nil, errors.New("not implemented")
}

// Save a service config.
func (server *server) SaveServiceConfig(ctx context.Context, rqst *services_managerpb.SaveServiceConfigRequest) (*services_managerpb.SaveServiceConfigResponse, error) {
	if rqst.Config == nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("no service configuration to save")))

	}

	// TODO

	// - 1 test if the service is local

	// if is not local dispatch the request to it service manager

	// if is local
	//  init the service with it new configuration.

	// Save the configuration...

	return &services_managerpb.SaveServiceConfigResponse{
		/** Nothing here **/
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
