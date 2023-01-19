package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/davecourtois/Utility"
	"github.com/globulario/services/golang/applications_manager/applications_managerpb"
	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/globular_client"
	"github.com/globulario/services/golang/rbac/rbacpb"
	"github.com/globulario/services/golang/repository/repository_client"
	"github.com/globulario/services/golang/resource/resource_client"
	"github.com/globulario/services/golang/resource/resourcepb"
	"github.com/golang/protobuf/jsonpb"
	"golang.org/x/net/html"
	"google.golang.org/grpc/codes"

	//"google.golang.org/grpc/grpclog"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// Uninstall application...
func (server *server) UninstallApplication(ctx context.Context, rqst *applications_managerpb.UninstallApplicationRequest) (*applications_managerpb.UninstallApplicationResponse, error) {

	// Here I will also remove the application permissions...
	if rqst.DeletePermissions {
		/** TODO remove applicaiton permissions...*/
	}

	var token string
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		token = strings.Join(md["token"], "")
		if len(token) == 0 {
			return nil, errors.New("application manager UninstallApplication no token was given")
		}
	}

	// Same as delete applicaitons.
	err := server.deleteApplication(token, rqst.ApplicationId)
	if err != nil {
		return nil,
			status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// Remove the application directory... but keep application data...
	os.RemoveAll(config.GetWebRootDir() + "/" + strings.Split(rqst.ApplicationId, "@")[0] )

	return &applications_managerpb.UninstallApplicationResponse{
		Result: true,
	}, nil
}

// Install local package. found in the local directory...
func (server *server) installLocalApplicationPackage(token, domain, applicationId, publisherId, version string) error {

	path := config.GetGlobularExecPath() + "/applications/" + applicationId + "_" + publisherId + "_" + version + ".tar.gz"

	fmt.Println("try to get package ", path)

	// so here I will try find package from the directory
	if len(version) == 0 {
		files, err := ioutil.ReadDir(config.GetGlobularExecPath() + "/applications")
		if err != nil {
			return err
		}

		for _, file := range files {
			if strings.Contains(file.Name(), applicationId) && strings.Contains(file.Name(), publisherId) {
				path = config.GetGlobularExecPath() + "/applications/" + file.Name()
			}
		}
	}

	if Utility.Exists(path) {
		file, err := os.Open(path)

		if err != nil {
			return nil
		}

		defer file.Close()

		r := bufio.NewReader(file)
		_extracted_path_, err := Utility.ExtractTarGz(r)
		if err != nil {
			return err
		}
		defer os.RemoveAll(_extracted_path_)

		// So now I will get the application descriptor
		descriptor := make(map[string]interface{})
		jsonStr, err := ioutil.ReadFile(_extracted_path_ + "/descriptor.json")
		if err != nil {
			return err
		}

		// read the content
		err = json.Unmarshal(jsonStr, &descriptor)
		if err != nil {
			return err
		}

		bundle, err := ioutil.ReadFile(_extracted_path_ + "/bundle.tar.gz")
		if err != nil {
			return err
		}

		// Now I will read the bundle...
		r_ := bytes.NewReader(bundle)

		// actions
		actions := make([]string, 0)
		if descriptor["actions"] != nil {
			actions_ := descriptor["actions"].([]interface{})
			for i := 0; i < len(actions_); i++ {
				actions = append(actions, actions_[i].(string))
			}
		}

		// keywords
		keywords := make([]string, 0)
		if descriptor["keywords"] != nil {
			keywords_ := descriptor["keywords"].([]interface{})
			for i := 0; i < len(keywords_); i++ {
				keywords = append(keywords, keywords_[i].(string))
			}
		}

		// roles
		roles := make([]*resourcepb.Role, 0)

		// groups
		groups := make([]*resourcepb.Group, 0)

		// Now I will install the applicaiton.
		err = server.installApplication(token, domain, descriptor["id"].(string), descriptor["publisherId"].(string), descriptor["version"].(string), descriptor["description"].(string), descriptor["icon"].(string), descriptor["alias"].(string), r_, actions, keywords, roles, groups, false)
		if err != nil {
			return err
		}

		return nil
	}

	err :=errors.New("no application pacakage found with path " + path)
	fmt.Println("fail to get local application package with error: ", err)
	return err
}

// ////////////////////// Resource Client ////////////////////////////////////////////
func GetResourceClient(domain string) (*resource_client.Resource_Client, error) {
	Utility.RegisterFunction("NewResourceService_Client", resource_client.NewResourceService_Client)
	client, err := globular_client.GetClient(domain, "resource.ResourceService", "NewResourceService_Client")
	if err != nil {
		return nil, err
	}
	return client.(*resource_client.Resource_Client), nil
}

// ////////////////////// Repository Client ////////////////////////////////////////////
func GetRepositoryClient(domain string) (*repository_client.Repository_Service_Client, error) {
	Utility.RegisterFunction("NewRepositoryService_Client", repository_client.NewRepositoryService_Client)
	client, err := globular_client.GetClient(domain, "repository.PackageRepository", "NewRepositoryService_Client")
	if err != nil {
		return nil, err
	}
	return client.(*repository_client.Repository_Service_Client), nil
}

// Install web Application
func (server *server) InstallApplication(ctx context.Context, rqst *applications_managerpb.InstallApplicationRequest) (*applications_managerpb.InstallApplicationResponse, error) {
	var token string
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		token = strings.Join(md["token"], "")
		if len(token) == 0 {
			return nil, errors.New("InstallApplication no token was given")
		}
	}

	err := server.installLocalApplicationPackage(token, rqst.Domain, rqst.ApplicationId, rqst.PublisherId, rqst.Version)
	if err == nil {
		fmt.Println("application", rqst.ApplicationId, "was install localy...")
		return &applications_managerpb.InstallApplicationResponse{
			Result: true,
		}, nil
	}

	// Connect to the dicovery services
	resource_client_, err := GetResourceClient(rqst.DicorveryId)

	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("Fail to connect to "+rqst.DicorveryId)))
	}

	descriptor, err := resource_client_.GetPackageDescriptor(rqst.ApplicationId, rqst.PublisherId, rqst.Version)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	if len(descriptor.Repositories) == 0 {

		if err != nil {
			return nil, status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("No service repository was found for application "+descriptor.Id)))
		}

	}

	for i := 0; i < len(descriptor.Repositories); i++ {

		package_repository, err := GetRepositoryClient(descriptor.Repositories[i])
		if err != nil {
			return nil, status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}

		bundle, err := package_repository.DownloadBundle(descriptor, "webapp")
		if err != nil {
			return nil, status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}

		// Create the file.
		r := bytes.NewReader(bundle.Binairies)

		// Now I will install the applicaiton.
		err = server.installApplication(token, rqst.Domain, descriptor.Id, descriptor.PublisherId, descriptor.Version, descriptor.Description, descriptor.Icon, descriptor.Alias, r, descriptor.Actions, descriptor.Keywords, descriptor.Roles, descriptor.Groups, rqst.SetAsDefault)
		if err != nil {
			return nil, status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}

	}

	return &applications_managerpb.InstallApplicationResponse{
		Result: true,
	}, nil

}

// Intall
func (server *server) installApplication(token, domain, name, publisherId, version, description, icon, alias string, r io.Reader, actions []string, keywords []string, roles []*resourcepb.Role, groups []*resourcepb.Group, set_as_default bool) error {

	// Here I will extract the file.
	__extracted_path__, err := Utility.ExtractTarGz(r)
	if err != nil {
		return err
	}

	// remove temporary files.
	defer os.RemoveAll(__extracted_path__)

	// Here I will test that the index.html file is not corrupted...
	__indexHtml__, err := ioutil.ReadFile(__extracted_path__ + "/index.html")
	if err != nil {
		return err
	}

	// The file must contain a linq to a bundle.js file.
	if !strings.Contains(string(__indexHtml__), "./bundle.js") {
		return errors.New("something wrong append the index.html file does not contain the bundle.js file... " + string(__indexHtml__))
	}

	// Copy the files to it final destination
	abosolutePath := server.WebRoot

	// If a domain is given.
	if len(domain) > 0 {
		if Utility.Exists(abosolutePath + "/" + domain) {
			abosolutePath += "/" + domain
		}
	}

	// set the absolute application domain.
	abosolutePath += "/" + name

	// Remove the existing files.
	if Utility.Exists(abosolutePath) {
		os.RemoveAll(abosolutePath)
	}

	// Recreate the dir and move file in it.
	Utility.CreateDirIfNotExist(abosolutePath)
	Utility.CopyDir(__extracted_path__+"/.", abosolutePath)
	if len(alias) == 0 {
		return errors.New("no application alias was given")
	}

	if len(name) == 0 {
		return errors.New("no application name was given")
	}

	if len(version) == 0 {
		return errors.New("no application version was given")
	}

	err = server.createApplication(token, name, domain, Utility.GenerateUUID(name), "/"+name, publisherId, version, description, alias, icon, actions, keywords)
	if err != nil {
		return err
	}

	// Now I will create/update roles define in the application descriptor...
	for i := 0; i < len(roles); i++ {
		role := roles[i]
		err = server.createRole(token, role.Id, role.Name, role.Actions)
		if err != nil {
			log.Println("fail to create role "+role.Id, "with error:", err)
		}
	}

	for i := 0; i < len(groups); i++ {
		group := groups[i]
		err = server.createGroup(token, group.Id, group.Name, group.Description)
		if err != nil {
			log.Println("fail to create group "+group.Id, "with error:", err)
		}
	}

	// Set the path of the directory where the application can store files.
	Utility.CreateDirIfNotExist(config.GetDataDir() + "/files/applications/" + name)
	if len(domain) == 0 {
		return errors.New("no domain given for application")
	}

	err = server.addResourceOwner("/applications/"+name, "file", name+"@"+domain, rbacpb.SubjectType_APPLICATION)
	if err != nil {
		return err
	}

	// here is a little workaround to be sure the bundle.js file will not be cached in the brower...
	indexHtml, err := ioutil.ReadFile(abosolutePath + "/index.html")
	if err != nil {
		return err
	}

	// Parse the index html file to be sure the file is valid.
	_, err = html.Parse(strings.NewReader(string(indexHtml)))
	if err != nil {
		return err
	}

	if err == nil {
		var re = regexp.MustCompile(`\/bundle\.js(\?updated=\d*)?`)
		indexHtml_ := re.ReplaceAllString(string(indexHtml), "/bundle.js?updated="+Utility.ToString(time.Now().Unix()))
		if !strings.Contains(indexHtml_, "/bundle.js?updated=") {
			return errors.New("651 something wrong append the index.html file does not contain the bundle.js file... " + indexHtml_)
		}
		// save it back.
		ioutil.WriteFile(abosolutePath+"/index.html", []byte(indexHtml_), 0644)
	}

	if set_as_default {
		// TODO keep track of the starting appliation...
		// server.IndexApplication = name
	}

	return err
}

/**
 * Send a application notification.
 * That function will send notification to all connected user of that application.
 */
func (server *server) sendApplicationNotification(application string, message string) error {

	// That service made user of persistence service.
	notification := new(resourcepb.Notification)
	notification.Id = Utility.RandomUUID()
	notification.NotificationType = resourcepb.NotificationType_APPLICATION_NOTIFICATION
	notification.Message = message
	notification.Recipient = application
	notification.Date = time.Now().Unix()

	// here I will get infos from the datastore.
	application_, err := server.getApplication(application)
	if err != nil {
		return err
	}

	notification.Sender = `{"_id":"` + application_.Id + `", "name":"` + application_.Name + `","icon":"` + application_.Icon + `", "alias":"` + application_.Alias + `"}`

	err = server.createNotification(notification)
	if err != nil {
		return err
	}

	var marshaler jsonpb.Marshaler
	jsonStr, err := marshaler.MarshalToString(notification)
	if err != nil {
		return err
	}

	return server.publish(application+"_notification_event", []byte(jsonStr))
}
