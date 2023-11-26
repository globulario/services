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
	"path/filepath"
	"strings"

	"github.com/davecourtois/Utility"
	"github.com/globulario/services/golang/applications_manager/applications_managerpb"
	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/globular_client"
	"github.com/globulario/services/golang/rbac/rbacpb"
	"github.com/globulario/services/golang/repository/repository_client"
	"github.com/globulario/services/golang/resource/resource_client"
	"github.com/globulario/services/golang/resource/resourcepb"
	"github.com/globulario/services/golang/security"
	"golang.org/x/net/html"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (srv *server) uninstallApplication(token, id string) error {

	// I will retrieve the application from the database.
	application, err := srv.getApplication(id)
	if err != nil {
		return err
	}

	// Same as delete applicaitons.
	err = srv.deleteApplication(token, id)
	if err != nil {
		return err
	}

	// Remove the application directory... but keep application data...
	return os.RemoveAll(config.GetWebRootDir() + "/" + application.Name)

}

// Uninstall application...
func (srv *server) UninstallApplication(ctx context.Context, rqst *applications_managerpb.UninstallApplicationRequest) (*applications_managerpb.UninstallApplicationResponse, error) {

	// Here I will also remove the application permissions...
	if rqst.DeletePermissions {
		/** TODO remove applicaiton permissions...*/
	}

	_, token, err := security.GetClientId(ctx)
	if err != nil {
		return nil, err
	}

	err = srv.uninstallApplication(token, rqst.ApplicationId)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &applications_managerpb.UninstallApplicationResponse{
		Result: true,
	}, nil
}

// Install local package. found in the local directory...
func (srv *server) installLocalApplicationPackage(token, domain, applicationId, publisherId, version string) error {

	// in case of local package I will try to find the package in the local directory...
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
		if descriptor["roles"] != nil {
			roles_ := descriptor["roles"].([]interface{})
			for i := 0; i < len(roles_); i++ {
				role_ := roles_[i].(map[string]interface{})
				r := new(resourcepb.Role)
				r.Id = role_["id"].(string)
				r.Name = role_["name"].(string)
				r.Domain, _ = config.GetDomain()
				actions := role_["actions"].([]interface{})
				r.Actions = make([]string, len(actions))
				for j := 0; j < len(actions); j++ {
					r.Actions[j] = actions[j].(string)
				}
				roles = append(roles, r)
			}
		}

		// groups
		groups := make([]*resourcepb.Group, 0)
		if descriptor["groups"] != nil {
			groups_ := descriptor["groups"].([]interface{})
			for i := 0; i < len(groups); i++ {
				group_ := groups_[i].(map[string]interface{})
				g := new(resourcepb.Group)
				g.Id = group_["id"].(string)
				g.Domain, _ = config.GetDomain()
				g.Name = group_["name"].(string)
				groups = append(groups, g)
			}
		}

		// Now I will install the applicaiton.
		err = srv.installApplication(token, domain, descriptor["id"].(string), descriptor["name"].(string), descriptor["publisherId"].(string), descriptor["version"].(string), descriptor["description"].(string), descriptor["icon"].(string), descriptor["alias"].(string), r_, actions, keywords, roles, groups, false)
		if err != nil {
			return err
		}

		return nil
	}

	err := errors.New("no application pacakage found with path " + path)
	fmt.Println("fail to get local application package with error: ", err)
	return err
}

// ////////////////////// Resource Client ////////////////////////////////////////////
func GetResourceClient(address string) (*resource_client.Resource_Client, error) {
	Utility.RegisterFunction("NewResourceService_Client", resource_client.NewResourceService_Client)
	client, err := globular_client.GetClient(address, "resource.ResourceService", "NewResourceService_Client")
	if err != nil {
		return nil, err
	}
	return client.(*resource_client.Resource_Client), nil
}

// ////////////////////// Repository Client ////////////////////////////////////////////
func GetRepositoryClient(address string) (*repository_client.Repository_Service_Client, error) {
	Utility.RegisterFunction("NewRepositoryService_Client", repository_client.NewRepositoryService_Client)
	client, err := globular_client.GetClient(address, "repository.PackageRepository", "NewRepositoryService_Client")
	if err != nil {
		return nil, err
	}
	return client.(*repository_client.Repository_Service_Client), nil
}

// Install web Application
func (srv *server) InstallApplication(ctx context.Context, rqst *applications_managerpb.InstallApplicationRequest) (*applications_managerpb.InstallApplicationResponse, error) {

	_, token, err := security.GetClientId(ctx)
	if err != nil {
		return nil, err
	}

	// Here I will try to install the application from the local directory...
	err = srv.installLocalApplicationPackage(token, rqst.Domain, rqst.ApplicationId, rqst.PublisherId, rqst.Version)
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
		err = srv.installApplication(token, rqst.Domain, descriptor.Id, descriptor.Name, descriptor.PublisherId, descriptor.Version, descriptor.Description, descriptor.Icon, descriptor.Alias, r, descriptor.Actions, descriptor.Keywords, descriptor.Roles, descriptor.Groups, rqst.SetAsDefault)
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

func isValidHTML(filePath string) bool {
	file, err := os.Open(filePath)
	if err != nil {
		fmt.Println("Error opening file:", err)
		return false
	}
	defer file.Close()

	// Parse the HTML content
	tokenizer := html.NewTokenizer(file)

	for {
		tokenType := tokenizer.Next()

		switch tokenType {
		case html.ErrorToken:
			// End of the file
			return true
		case html.StartTagToken, html.SelfClosingTagToken:
			// Continue parsing
		default:
			// If an unexpected token is encountered, it's not valid HTML
			return false
		}
	}
}

func appendBaseTag(filePath, base string) error {
	// Read the content of the HTML file
	content, err := ioutil.ReadFile(filePath)
	if err != nil {
		return err
	}

	// Parse the HTML content
	doc, err := html.Parse(bytes.NewReader(content))
	if err != nil {
		return err
	}

	// Check if the base tag is already present
	if !containsBaseTag(doc) {
		// If not present, append the base tag to the head
		baseTag := createBaseTag(base)
		head := findHead(doc)
		if head != nil {
			head.AppendChild(baseTag)
		} else {
			return fmt.Errorf("head tag not found in HTML document")
		}

		// Serialize the modified HTML
		var buffer bytes.Buffer
		if err := html.Render(&buffer, doc); err != nil {
			return err
		}

		// Write the modified content back to the file
		if err := ioutil.WriteFile(filePath, buffer.Bytes(), 0644); err != nil {
			return err
		}
	}

	return nil
}

func containsBaseTag(n *html.Node) bool {
	// Check if the HTML document already contains a base tag
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.ElementNode && c.Data == "base" {
			return true
		}
	}

	return false
}

func createBaseTag(base string) *html.Node {
	// Create a new base tag
	baseTag := &html.Node{
		Type: html.ElementNode,
		Data: "base",
		Attr: []html.Attribute{
			{Key: "href", Val: "/" + base + "/"},
		},
	}

	return baseTag
}

func findHead(n *html.Node) *html.Node {
	// Find the head tag in the HTML document
	var head *html.Node
	var f func(*html.Node)
	f = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "head" {
			head = n
			return
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			f(c)
		}
	}
	f(n)

	return head
}

// Intall
func (srv *server) installApplication(token, domain, id, name, publisherId, version, description, icon, alias string, r io.Reader, actions []string, keywords []string, roles []*resourcepb.Role, groups []*resourcepb.Group, set_as_default bool) error {

	// Here I will extract the file.
	__extracted_path__, err := Utility.ExtractTarGz(r)
	if err != nil {
		return err
	}

	// remove temporary files.
	defer os.RemoveAll(__extracted_path__)

	files, err := Utility.FindFileByName(__extracted_path__, "index.html")
	if err != nil {
		return err
	}

	// Here I will test that the index.html file is not corrupted...

	// The file must contain a linq to a bundle.js file.
	/*if !isValidHTML(files[0])  {
		return errors.New("something wrong append the index.html at path it content is not valid " + files[0])
	}*/

	if err := appendBaseTag(files[0], name); err != nil {
		fmt.Println("Error:", err)
	} else {
		fmt.Println("Operation completed successfully.")
	}

	// Copy the files to it final destination
	abosolutePath := srv.WebRoot + "/" + name

	// Remove the existing files.
	if Utility.Exists(abosolutePath) {
		os.RemoveAll(abosolutePath)
	}

	// Recreate the dir and move file in it.
	err = Utility.CreateDirIfNotExist(abosolutePath)
	if err != nil {
		return err
	}

	
	err = Utility.CopyDir(filepath.Dir(files[0])+"/.", abosolutePath)
	if err != nil {
		return err
	}

	if len(alias) == 0 {
		return errors.New("no application alias was given")
	}

	if len(name) == 0 {
		return errors.New("no application name was given")
	}

	if len(version) == 0 {
		return errors.New("no application version was given")
	}

	err = srv.createApplication(token, id, name, domain, Utility.GenerateUUID(name), "/"+name, publisherId, version, description, alias, icon, actions, keywords)
	if err != nil {
		return err
	}

	// Now I will create/update roles define in the application descriptor...
	for i := 0; i < len(roles); i++ {
		role := roles[i]
		err = srv.createRole(token, role.Id, role.Name, role.Actions)
		if err != nil {
			log.Println("fail to create role "+role.Id, "with error:", err)
		}
	}

	for i := 0; i < len(groups); i++ {
		group := groups[i]
		err = srv.createGroup(token, group.Id, group.Name, group.Description)
		if err != nil {
			log.Println("fail to create group "+group.Id, "with error:", err)
		}
	}

	// Set the path of the directory where the application can store files.
	Utility.CreateDirIfNotExist(config.GetDataDir() + "/files/applications/" + name)
	if len(domain) == 0 {
		return errors.New("no domain given for application")
	}

	owner := id
	if !strings.Contains(owner, "@") {
		owner += "@" + domain
	}

	err = srv.addResourceOwner("/applications/"+name, "file", owner, rbacpb.SubjectType_APPLICATION)
	if err != nil {
		return err
	}

	// here is a little workaround to be sure the bundle.js file will not be cached in the brower...
	_, err = ioutil.ReadFile(abosolutePath + "/index.html")
	if err != nil {
		return err
	}


	if set_as_default {
		// TODO keep track of the starting appliation...
		// srv.IndexApplication = name
	}

	return err
}
