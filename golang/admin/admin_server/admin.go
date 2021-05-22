package main

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"io"
	"io/ioutil"
	"log"
	"os"
	"runtime"
	"time"

	"golang.org/x/net/html"

	// "golang.org/x/sys/windows/registry"
	"path/filepath"

	"os/exec"
	"regexp"
	"strings"

	"github.com/globulario/services/golang/rbac/rbacpb"

	"github.com/globulario/services/golang/packages/packagespb"
	"github.com/globulario/services/golang/security"

	"github.com/davecourtois/Utility"
	"github.com/globulario/services/golang/admin/adminpb"
	"github.com/globulario/services/golang/packages/packages_client"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

/**
 * Test if a process with a given name is Running on the server.
 * By default that function is accessible by sa only.
 */
func (admin_server *server) HasRunningProcess(ctx context.Context, rqst *adminpb.HasRunningProcessRequest) (*adminpb.HasRunningProcessResponse, error) {
	ids, err := Utility.GetProcessIdsByName(rqst.Name)

	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	if len(ids) == 0 {
		return &adminpb.HasRunningProcessResponse{
			Result: false,
		}, nil
	}

	return &adminpb.HasRunningProcessResponse{
		Result: true,
	}, nil
}

// Uninstall application...
func (admin_server *server) UninstallApplication(ctx context.Context, rqst *adminpb.UninstallApplicationRequest) (*adminpb.UninstallApplicationResponse, error) {

	// Here I will also remove the application permissions...
	if rqst.DeletePermissions {
		log.Println("remove applicaiton permissions...")
	}

	log.Println("remove applicaiton ", rqst.ApplicationId)

	// Same as delete applicaitons.
	err := admin_server.deleteApplication(rqst.ApplicationId)
	if err != nil {
		return nil,
			status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &adminpb.UninstallApplicationResponse{
		Result: true,
	}, nil
}

// Install web Application
func (admin_server *server) InstallApplication(ctx context.Context, rqst *adminpb.InstallApplicationRequest) (*adminpb.InstallApplicationResponse, error) {
	// Get the package bundle from the repository and install it on the server.
	log.Println("Try to install application " + rqst.ApplicationId)

	// Connect to the dicovery services
	package_discovery, err := packages_client.NewPackagesDiscoveryService_Client(rqst.DicorveryId, "packages.PackageDiscovery")

	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("Fail to connect to "+rqst.DicorveryId)))
	}

	descriptors, err := package_discovery.GetPackageDescriptor(rqst.ApplicationId, rqst.PublisherId)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	log.Println("step 1: get application descriptor")
	// The first element in the array is the most recent descriptor
	// so if no version is given the most recent will be taken.
	descriptor := descriptors[0]
	for i := 0; i < len(descriptors); i++ {
		if descriptors[i].Version == rqst.Version {
			descriptor = descriptors[i]
			break
		}
	}

	log.Println("step 2: try to dowload application bundle")
	if len(descriptor.Repositories) == 0 {

		if err != nil {
			return nil, status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("No service repository was found for application "+descriptor.Id)))
		}

	}

	for i := 0; i < len(descriptor.Repositories); i++ {

		package_repository, err := packages_client.NewServicesRepositoryService_Client(descriptor.Repositories[i], "packages.PackageRepository")
		if err != nil {
			return nil, status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}

		log.Println("--> try to download application bundle from ", descriptor.Repositories[i])
		bundle, err := package_repository.DownloadBundle(descriptor, "webapp")
		if err != nil {
			return nil, status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}

		// Create the file.
		r := bytes.NewReader(bundle.Binairies)

		// Now I will install the applicaiton.
		err = admin_server.installApplication(rqst.Domain, descriptor.Id, descriptor.PublisherId, descriptor.Version, descriptor.Description, descriptor.Icon, descriptor.Alias, r, descriptor.Actions, descriptor.Keywords, descriptor.Roles, descriptor.Groups, rqst.SetAsDefault)
		if err != nil {
			return nil, status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}

	}

	log.Println("application was install!")
	return &adminpb.InstallApplicationResponse{
		Result: true,
	}, nil

}

// Intall
func (admin_server *server) installApplication(domain, name, publisherId, version, description string, icon string, alias string, r io.Reader, actions []string, keywords []string, roles []*packagespb.Role, groups []*packagespb.Group, set_as_default bool) error {

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
		return errors.New("539 something wrong append the index.html file does not contain the bundle.js file... " + string(__indexHtml__))
	}

	// Copy the files to it final destination
	abosolutePath := admin_server.WebRoot

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

	err = admin_server.createApplication(name, Utility.GenerateUUID(name), "/"+name, publisherId, version, description, alias, icon, actions, keywords)
	if err != nil {
		return err
	}

	// Now I will create/update roles define in the application descriptor...
	for i := 0; i < len(roles); i++ {
		role := roles[i]
		err = admin_server.createRole(role.Id, role.Name, role.Actions)
		if err != nil {
			log.Println("fail to create role "+role.Id, "with error:", err)
		}
	}

	for i := 0; i < len(groups); i++ {
		group := groups[i]
		err = admin_server.createGroup(group.Id, group.Name)
		if err != nil {
			log.Println("fail to create group "+group.Id, "with error:", err)
		}
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
		// admin_server.IndexApplication = name
	}

	return err
}

/**
 * Publish an application on globular server.
 */
func (admin_server *server) publishApplication(user, organization, path, name, domain, version, description, icon, alias, repositoryId, discoveryId string, actions, keywords []string, roles []*adminpb.Role) error {

	publisherId := user
	if len(organization) > 0 {
		publisherId = organization
		if !admin_server.isOrganizationMemeber(user, organization) {
			return errors.New(user + " is not a member of " + organization + "!")
		}
	}

	descriptor := &packagespb.PackageDescriptor{
		Id:           name,
		Name:         name,
		PublisherId:  publisherId,
		Version:      version,
		Description:  description,
		Repositories: []string{repositoryId},
		Discoveries:  []string{discoveryId},
		Icon:         icon,
		Alias:        alias,
		Actions:      []string{},
		Type:         packagespb.PackageType_APPLICATION_TYPE,
		Roles:        []*packagespb.Role{},
	}

	if len(repositoryId) > 0 {
		descriptor.Repositories = append(descriptor.Repositories, repositoryId)
	}

	if len(discoveryId) > 0 {
		descriptor.Discoveries = append(descriptor.Discoveries, discoveryId)
	}

	if len(keywords) > 0 {
		descriptor.Keywords = keywords
	}

	if len(actions) > 0 {
		descriptor.Actions = actions
	}

	err := admin_server.publishPackage(user, organization, discoveryId, repositoryId, "webapp", path, descriptor)

	// Set the path of the directory where the application can store date.
	Utility.CreateDirIfNotExist(admin_server.ApplicationsRoot + "/" + name)
	if err != nil {
		return err
	}

	err = admin_server.addResourceOwner("/applications/"+name, name, rbacpb.SubjectType_APPLICATION)
	if err != nil {
		return err
	}

	return nil
}

/**
 * Update Globular itself with a new version.
 */
func (admin_server *server) Update(stream adminpb.AdminService_UpdateServer) error {
	// The buffer that will receive the service executable.
	var buffer bytes.Buffer
	var platform string
	for {
		msg, err := stream.Recv()
		if err == io.EOF || msg == nil {
			// end of stream...
			stream.SendAndClose(&adminpb.UpdateResponse{})
			err = nil
			break
		} else if err != nil {
			return err
		} else if len(msg.Data) == 0 {
			break
		} else {
			buffer.Write(msg.Data)
		}

		if len(msg.Platform) > 0 {
			platform = msg.Platform
		}

	}

	if len(platform) == 0 {
		return errors.New("no platform was given")
	}

	platform_ := runtime.GOOS + ":" + runtime.GOARCH
	if platform != platform_ {
		return errors.New("Wrong executable platform to update from! wants " + platform_ + " not " + platform)
	}

	ex, err := os.Executable()
	if err != nil {
		return err
	}

	path := filepath.Dir(ex)

	path += "/Globular"
	if runtime.GOOS == "windows" {
		path += ".exe"
	}

	existing_checksum := Utility.CreateFileChecksum(path)
	checksum := Utility.CreateDataChecksum(buffer.Bytes())
	if existing_checksum == checksum {
		return errors.New("no update needed")
	}

	// Move the actual file to other file...
	err = os.Rename(path, path+"_"+checksum)
	if err != nil {
		return err
	}

	/** So here I will change the current server path and save the new executable **/
	err = ioutil.WriteFile(path, buffer.Bytes(), 0755)
	if err != nil {
		return err
	}

	// exit
	log.Println("stop globular made use systemctl to restart globular automaticaly")

	// TODO restart Globular exec...
	// Utility.TerminateProcess(pid, 0); // send signal to globular...

	return nil
}

// Download the actual globular exec file.
func (admin_server *server) DownloadGlobular(rqst *adminpb.DownloadGlobularRequest, stream adminpb.AdminService_DownloadGlobularServer) error {
	platform := rqst.Platform

	if len(platform) == 0 {
		return errors.New("no platform was given")
	}

	platform_ := runtime.GOOS + ":" + runtime.GOARCH
	if platform != platform_ {
		return errors.New("Wrong executable platform to update from get " + platform + " want " + platform_)
	}

	ex, err := os.Executable()
	if err != nil {
		return err
	}

	path := filepath.Dir(ex)
	path += "/Globular"
	if runtime.GOOS == "windows" {
		path += ".exe"
	}

	// No I will stream the result over the networks.
	data, err := os.Open(path)
	if err != nil {
		log.Fatal(err)
	}
	defer data.Close()

	reader := bufio.NewReader(data)
	const BufferSize = 1024 * 5 // the chunck size.

	for {
		var data [BufferSize]byte
		bytesread, err := reader.Read(data[0:BufferSize])
		if bytesread > 0 {
			rqst := &adminpb.DownloadGlobularResponse{
				Data: data[0:bytesread],
			}
			// send the data to the server.
			err = stream.Send(rqst)
		}

		if err == io.EOF {
			break
		} else if err != nil {
			return err
		}
	}

	return nil
}

// Deloyed a web application to a globular node. Mostly use a develeopment time.
func (admin_server *server) DeployApplication(stream adminpb.AdminService_DeployApplicationServer) error {

	// - Get the information from the package.json (npm package, the version, the keywords and set the package descriptor with it.

	// The bundle will cantain the necessary information to install the service.
	var buffer bytes.Buffer

	// Here is necessary information to publish an application.
	var name string
	var domain string
	var user string
	var organization string
	var version string
	var description string
	var repositoryId string
	var discoveryId string
	var keywords []string
	var actions []string
	var icon string
	var alias string
	var roles []*adminpb.Role
	var groups []*adminpb.Group
	var set_as_default bool

	for {
		msg, err := stream.Recv()
		if err == io.EOF || msg == nil {
			// end of stream...
			stream.SendAndClose(&adminpb.DeployApplicationResponse{
				Result: true,
			})
			err = nil
			break
		} else if err != nil {
			return err
		} else if len(msg.Data) == 0 {
			break
		} else {
			buffer.Write(msg.Data)
		}

		if len(msg.Name) > 0 {
			name = msg.Name
		}

		if len(msg.Alias) > 0 {
			alias = msg.Alias
		}

		if len(msg.Domain) > 0 {
			domain = msg.Domain
		}
		if len(msg.Organization) > 0 {
			organization = msg.Organization
		}
		if len(msg.User) > 0 {
			user = msg.User
		}
		if len(msg.Version) > 0 {
			version = msg.Version
		}
		if len(msg.Description) > 0 {
			description = msg.Description
		}
		if msg.Keywords != nil {
			keywords = msg.Keywords
		}
		if len(msg.Repository) > 0 {
			repositoryId = msg.Repository
		}
		if len(msg.Discovery) > 0 {
			discoveryId = msg.Discovery
		}

		if len(msg.Roles) > 0 {
			roles = msg.Roles
		}

		if len(msg.Groups) > 0 {
			groups = msg.Groups
		}

		if len(msg.Actions) > 0 {
			actions = msg.Actions
		}

		if len(msg.Icon) > 0 {
			icon = msg.Icon
		}

		if msg.SetAsDefault {
			set_as_default = true
		}

	}

	if len(repositoryId) == 0 {
		repositoryId = domain
	}

	if len(discoveryId) == 0 {
		discoveryId = domain
	}

	// Retreive the actual application installed version.
	previousVersion, err := admin_server.getApplicationVersion(name)

	// Now I will save the bundle into a file in the temp directory.
	path := os.TempDir() + "/" + Utility.RandomUUID()
	defer os.RemoveAll(path)

	err = ioutil.WriteFile(path, buffer.Bytes(), 0644)
	if err != nil {
		return err
	}

	err = admin_server.publishApplication(user, organization, path, name, domain, version, description, icon, alias, repositoryId, discoveryId, actions, keywords, roles)

	if err != nil {
		return err
	}

	// convert struct...
	roles_ := make([]*packagespb.Role, len(roles))
	for i := 0; i < len(roles); i++ {
		roles_[i] = new(packagespb.Role)
		roles_[i].Id = roles[i].Id
		roles_[i].Name = roles[i].Name
		roles_[i].Actions = roles[i].Actions
	}

	groups_ := make([]*packagespb.Group, len(groups))
	for i := 0; i < len(groups); i++ {
		groups_[i] = new(packagespb.Group)
		groups_[i].Id = groups[i].Id
		groups_[i].Name = groups[i].Name
	}

	// Read bytes and extract it in the current directory.
	r := bytes.NewReader(buffer.Bytes())
	err = admin_server.installApplication(domain, name, organization, version, description, icon, alias, r, actions, keywords, roles_, groups_, set_as_default)
	if err != nil {
		return err
	}

	// If the version has change I will notify current users and undate the applications.
	if previousVersion != version {

		// Send application notification...
		admin_server.publish("update_"+strings.Split(domain, ":")[0]+"_"+name+"_evt", []byte(version))

		message := `<div style="display: flex; flex-direction: column">
              <div>A new version of <span style="font-weight: 500;">` + alias + `</span> (v.` + version + `) is available.
              </div>
              <div>
                Press <span style="font-weight: 500;">f5</span> to refresh the page.
              </div>
            </div>
            `

		return admin_server.sendApplicationNotification(name, message)
	}
	return nil
}

// Upload a service package.
func (admin_server *server) UploadServicePackage(stream adminpb.AdminService_UploadServicePackageServer) error {
	// The bundle will cantain the necessary information to install the service.
	path := os.TempDir() + "/" + Utility.RandomUUID()

	fo, err := os.Create(path)
	if err != nil {
		return status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	defer fo.Close()

	for {
		msg, err := stream.Recv()
		if err == nil {
			if len(msg.Organization) > 0 {
				if !admin_server.isOrganizationMemeber(msg.User, msg.Organization) {
					return status.Errorf(
						codes.Internal,
						Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New(msg.User+" is not a member of "+msg.Organization)))
				}
			}
		}

		if msg == nil {
			stream.SendAndClose(&adminpb.UploadServicePackageResponse{
				Path: path,
			})
			err = nil
			break
		} else if err == io.EOF || len(msg.Data) == 0 {
			// end of stream...
			stream.SendAndClose(&adminpb.UploadServicePackageResponse{
				Path: path,
			})
			err = nil
			break
		} else if err != nil {
			return status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		} else {
			fo.Write(msg.Data)
		}
	}
	return nil
}

// Publish a package, the package can contain an application or a services.
func (admin_server *server) publishPackage(user string, organization string, discovery string, repository string, platform string, path string, descriptor *packagespb.PackageDescriptor) error {
	log.Println("Publish package for user: ", user, "organization: ", organization, "discovery: ", discovery, "repository: ", repository, "platform: ", platform)

	// Connect to the dicovery services
	services_discovery, err := packages_client.NewPackagesDiscoveryService_Client(discovery, "packages.PackageDiscovery")
	if err != nil {
		return errors.New("Fail to connect to package discovery at " + discovery)
	}

	// Connect to the repository packagespb.
	services_repository, err := packages_client.NewServicesRepositoryService_Client(repository, "packages.PackageRepository")
	if err != nil {
		return errors.New("Fail to connect to package repository at " + repository)
	}

	// Ladies and Gentlemans After one year after tow years services as resource!
	path_ := descriptor.PublisherId + "/" + descriptor.Name + "/" + descriptor.Id + "/" + descriptor.Version

	// So here I will set the permissions
	var permissions *rbacpb.Permissions
	permissions, err = admin_server.getResourcePermissions(path_)
	if err != nil {
		// Create the permission...
		permissions = &rbacpb.Permissions{
			Allowed: []*rbacpb.Permission{
				//  Exemple of possible permission values.
				&rbacpb.Permission{
					Name:          "publish", // member of the organization can publish the service.
					Applications:  []string{},
					Accounts:      []string{},
					Groups:        []string{},
					Peers:         []string{},
					Organizations: []string{organization},
				},
			},
			Denied: []*rbacpb.Permission{},
			Owners: &rbacpb.Permission{
				Name:          "owner",
				Accounts:      []string{user},
				Applications:  []string{},
				Groups:        []string{},
				Peers:         []string{},
				Organizations: []string{},
			},
		}

		// Set the permissions.
		err = admin_server.setResourcePermissions(path_, permissions)
		if err != nil {
			log.Println("fail to publish package with error: ", err.Error())
			return err
		}
	}

	// Test the permission before actualy publish the package.
	hasAccess, isDenied, err := admin_server.validateAccess(user, rbacpb.SubjectType_ACCOUNT, "publish", path_)
	if !hasAccess || isDenied || err != nil {
		log.Println(err)
		return err
	}

	// Append the user into the list of owner if is not already part of it.
	if !Utility.Contains(permissions.Owners.Accounts, user) {
		permissions.Owners.Accounts = append(permissions.Owners.Accounts, user)
	}

	// Save the permissions.
	err = admin_server.setResourcePermissions(path_, permissions)
	if err != nil {
		log.Println(err)
		return err
	}

	// Fist of all publish the package descriptor.
	err = services_discovery.PublishPackageDescriptor(descriptor)
	if err != nil {
		log.Println(err)
		return err
	}

	// Upload the service to the repository.
	return services_repository.UploadBundle(discovery, descriptor.Id, descriptor.PublisherId, platform, path)
}

// Publish a service. The service must be install localy on the server.
func (admin_server *server) PublishService(ctx context.Context, rqst *adminpb.PublishServiceRequest) (*adminpb.PublishServiceResponse, error) {
	log.Println("try to publish service ", rqst.ServiceName, "...")
	// Make sure the user is part of the organization if one is given.
	publisherId := rqst.User
	if len(rqst.Organization) > 0 {
		publisherId = rqst.Organization
		if !admin_server.isOrganizationMemeber(rqst.User, rqst.Organization) {
			err := errors.New(rqst.User + " is not member of " + rqst.Organization)
			log.Println(err.Error())
			return nil, err
		}
	}

	// Now I will upload the service to the repository...
	descriptor := &packagespb.PackageDescriptor{
		Id:           rqst.ServiceId,
		Name:         rqst.ServiceName,
		PublisherId:  publisherId,
		Version:      rqst.Version,
		Description:  rqst.Description,
		Keywords:     rqst.Keywords,
		Repositories: []string{rqst.RepositoryId},
		Discoveries:  []string{rqst.DicorveryId},
		Type:         packagespb.PackageType_SERVICE_TYPE,
	}

	err := admin_server.publishPackage(rqst.User, rqst.Organization, rqst.DicorveryId, rqst.RepositoryId, rqst.Platform, rqst.Path, descriptor)

	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &adminpb.PublishServiceResponse{
		Result: true,
	}, nil
}

// Kill process by id
func (admin_server *server) KillProcess(ctx context.Context, rqst *adminpb.KillProcessRequest) (*adminpb.KillProcessResponse, error) {
	pid := int(rqst.Pid)
	err := Utility.TerminateProcess(pid, 0)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &adminpb.KillProcessResponse{}, nil
}

// Kill process by name
func (admin_server *server) KillProcesses(ctx context.Context, rqst *adminpb.KillProcessesRequest) (*adminpb.KillProcessesResponse, error) {
	err := Utility.KillProcessByName(rqst.Name)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &adminpb.KillProcessesResponse{}, nil
}

// Return the list of process id with a given name.
func (admin_server *server) GetPids(ctx context.Context, rqst *adminpb.GetPidsRequest) (*adminpb.GetPidsResponse, error) {
	pids_, err := Utility.GetProcessIdsByName(rqst.Name)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	pids := make([]int32, len(pids_))
	for i := 0; i < len(pids_); i++ {
		pids[i] = int32(pids_[i])
	}

	return &adminpb.GetPidsResponse{
		Pids: pids,
	}, nil
}

/**
 * Read output and send it to a channel.
 */
func ReadOutput(output chan string, rc io.ReadCloser) {

	cutset := "\r\n"
	for {
		buf := make([]byte, 3000)
		n, err := rc.Read(buf)
		if err != nil {
			if err != io.EOF {
				log.Println(err)
			}
			if n == 0 {
				break
			}
		}
		text := strings.TrimSpace(string(buf[:n]))
		for {
			// Take the index of any of the given cutset
			n := strings.IndexAny(text, cutset)
			if n == -1 {
				// If not found, but still have data, send it
				if len(text) > 0 {
					output <- text
				}
				break
			}
			// Send data up to the found cutset
			output <- text[:n]
			// If cutset is last element, stop there.
			if n == len(text) {
				break
			}
			// Shift the text and start again.
			text = text[n+1:]
		}
	}
}

// Run an external command must be use with care.
func (admin_server *server) RunCmd(rqst *adminpb.RunCmdRequest, stream adminpb.AdminService_RunCmdServer) error {

	baseCmd := rqst.Cmd
	cmdArgs := rqst.Args
	isBlocking := rqst.Blocking
	pid := -1
	cmd := exec.Command(baseCmd, cmdArgs...)
	if isBlocking {

		stdout, err := cmd.StdoutPipe()
		if err != nil {
			return status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}

		output := make(chan string)
		done := make(chan bool)

		// Process message util the command is done.
		go func() {
			for {
				select {
				case <-done:
					break

				case result := <-output:
					if cmd.Process != nil {
						pid = cmd.Process.Pid
					}

					stream.Send(
						&adminpb.RunCmdResponse{
							Pid:    int32(pid),
							Result: result,
						},
					)
				}
			}

		}()

		// Start reading the output
		go ReadOutput(output, stdout)

		cmd.Run()

		cmd.Wait()

		// Close the output.
		stdout.Close()
		done <- true

	} else {
		err := cmd.Start()
		if err != nil {
			return status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
		if cmd.Process != nil {
			pid = cmd.Process.Pid
		}

		stream.Send(
			&adminpb.RunCmdResponse{
				Pid:    int32(pid),
				Result: "",
			},
		)

	}

	return nil
}

// Set environement variable.
func (admin_server *server) SetEnvironmentVariable(ctx context.Context, rqst *adminpb.SetEnvironmentVariableRequest) (*adminpb.SetEnvironmentVariableResponse, error) {
	err := Utility.SetEnvironmentVariable(rqst.Name, rqst.Value)

	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	return &adminpb.SetEnvironmentVariableResponse{}, nil
}

// Get environement variable.
func (admin_server *server) GetEnvironmentVariable(ctx context.Context, rqst *adminpb.GetEnvironmentVariableRequest) (*adminpb.GetEnvironmentVariableResponse, error) {
	value, err := Utility.GetEnvironmentVariable(rqst.Name)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	return &adminpb.GetEnvironmentVariableResponse{
		Value: value,
	}, nil
}

// Delete environement variable.
func (admin_server *server) UnsetEnvironmentVariable(ctx context.Context, rqst *adminpb.UnsetEnvironmentVariableRequest) (*adminpb.UnsetEnvironmentVariableResponse, error) {

	err := Utility.UnsetEnvironmentVariable(rqst.Name)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	return &adminpb.UnsetEnvironmentVariableResponse{}, nil
}

///////////////////////////////////// API //////////////////////////////////////

// Get certificates from the server and copy them into the the a given directory.
// path: The path where to copy the certificates
// port: The server configuration port the default is 80.
//
// ex. Here is an exemple of the command run from the shell,
//
// Globular certificates -domain=globular.cloud -path=/tmp -port=80
//
// The command can return
func (admin_server *server) GetCertificates(ctx context.Context, rqst *adminpb.GetCertificatesRequest) (*adminpb.GetCertificatesResponse, error) {
	path := rqst.Path
	if len(path) == 0 {
		path = os.TempDir()
	}

	port := 80
	if rqst.Port != 0 {
		port = int(rqst.Port)
	}

	// Create the certificate at the given path.
	key, cert, ca, err := security.InstallCertificates(rqst.Domain, port, path)

	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &adminpb.GetCertificatesResponse{
		Certkey: key,
		Cert:    cert,
		Cacert:  ca,
	}, nil
}
