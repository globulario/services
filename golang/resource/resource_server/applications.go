package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/rbac/rbacpb"
	"github.com/globulario/services/golang/resource/resourcepb"
	"github.com/globulario/services/golang/security"
	Utility "github.com/globulario/utility"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
)

// AddApplicationActions adds new actions to an existing application identified by ApplicationId.
// It retrieves the application from the persistence store, checks if the actions already exist,
// and appends any new actions provided in the request. If changes are made, the application is
// updated in the persistence store. An update event is published upon successful modification.
//
// Parameters:
//
//	ctx - The context for the request.
//	rqst - The request containing the ApplicationId and the list of actions to add.
//
// Returns:
//
//	*resourcepb.AddApplicationActionsRsp - The response indicating the result of the operation.
//	error - An error if the operation fails.
func (srv *server) AddApplicationActions(ctx context.Context, rqst *resourcepb.AddApplicationActionsRqst) (*resourcepb.AddApplicationActionsRsp, error) {
	// That service made user of persistence service.
	p, err := srv.getPersistenceStore()
	if err != nil {
		return nil, err
	}

	q := `{"_id":"` + rqst.ApplicationId + `"}`

	values, err := p.FindOne(context.Background(), "local_resource", "local_resource", "Applications", q, ``)
	if err != nil {

		return nil, status.Errorf(
			codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	application := values.(map[string]interface{})
	needSave := false
	var actions_ []interface{}

	if application["actions"] == nil {
		application["actions"] = rqst.Actions
		needSave = true
	} else {

		switch application["actions"].(type) {
		case primitive.A:
			actions_ = []interface{}(application["actions"].(primitive.A))
		case []interface{}:
			actions_ = []interface{}(application["actions"].([]interface{}))
		default:
			logger.Warn("unknown type", "value", application["actions"])
		}

		for j := 0; j < len(rqst.Actions); j++ {
			exist := false
			for i := 0; i < len(actions_); i++ {
				if actions_[i].(string) == rqst.Actions[j] {
					exist = true
					break
				}
			}
			if !exist {
				actions_ = append(actions_, rqst.Actions[j])
				needSave = true
			}
		}
	}

	if needSave {
		application["actions"] = actions_
		jsonStr := serialyseObject(application)
		err := p.ReplaceOne(context.Background(), "local_resource", "local_resource", "Applications", q, string(jsonStr), ``)
		if err != nil {
			return nil, status.Errorf(
				codes.Internal,
				"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
	}

	srv.publishEvent("update_application_"+rqst.ApplicationId+"_evt", []byte{}, srv.Address)

	return &resourcepb.AddApplicationActionsRsp{Result: true}, nil
}

// CreateApplication handles the creation of a new application resource.
// It retrieves the client ID from the context, saves the application data,
// publishes a creation event if successful, and returns an appropriate response.
// Returns an error if the client ID cannot be retrieved or if saving the application fails.
func (srv *server) CreateApplication(ctx context.Context, rqst *resourcepb.CreateApplicationRqst) (*resourcepb.CreateApplicationRsp, error) {

	clientId, _, err := security.GetClientId(ctx)
	if err != nil {
		return nil, err
	}

	err = srv.save_application(ctx, rqst.Application, clientId)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	jsonStr, err := protojson.Marshal(rqst.Application)
	if err == nil {
		srv.publishEvent("create_application_evt", jsonStr, srv.GetAddress())
	}

	return &resourcepb.CreateApplicationRsp{}, nil
}

/**
 * Delete application Data from the backend.
 */
func (srv *server) deleteApplication(ctx context.Context, applicationId string) error {

	_, token, err := security.GetClientId(ctx)
	if err != nil {
		return err
	}

	if strings.Contains(applicationId, "@") {
		domain := strings.Split(applicationId, "@")[1]
		applicationId = strings.Split(applicationId, "@")[0]

		localDomain, err := config.GetDomain()
		if err != nil {
			return err
		}

		if localDomain != domain {
			return errors.New("i cant's delete object from domain " + domain + " from domain " + localDomain)
		}
	}

	// That service made user of persistence service.
	p, err := srv.getPersistenceStore()
	if err != nil {
		return err
	}

	q := `{"_id":"` + applicationId + `"}`

	values, err := p.FindOne(context.Background(), "local_resource", "local_resource", "Applications", q, ``)
	if err != nil {
		return err
	}

	// I will remove all the access to the application, before removing the application.
	srv.deleteAllAccess(token, applicationId, rbacpb.SubjectType_APPLICATION)
	srv.deleteResourcePermissions(token,applicationId)

	application := values.(map[string]interface{})

	// I will remove it from organization...
	if application["organizations"] != nil {

		var organizations []interface{}
		switch values.(map[string]interface{})["organizations"].(type) {
		case primitive.A:
			organizations = []interface{}(values.(map[string]interface{})["organizations"].(primitive.A))
		case []interface{}:
			organizations = values.(map[string]interface{})["organizations"].([]interface{})
		}
		for i := 0; i < len(organizations); i++ {
			organizationId := organizations[i].(map[string]interface{})["$id"].(string)
			srv.deleteReference(p, applicationId, organizationId, "applications", "Organizations")
		}
	}

	// I will remove the directory.
	err = os.RemoveAll(application["path"].(string))
	if err != nil {
		return err
	}

	// Finaly I will remove the entry in  the table.
	err = p.DeleteOne(context.Background(), "local_resource", "local_resource", "Applications", q, "")
	if err != nil {
		return err
	}

	// Drop the application user.
	// Here I will drop the db user.
	var dropUserScript string
	if p.GetStoreType() == "MONGO" {
		dropUserScript = fmt.Sprintf(
			`db=db.getSiblingDB('admin');db.dropUser('%s', {w: 'majority', wtimeout: 4000})`,
			applicationId)
	} else if p.GetStoreType() == "SCYLLA" {
		dropUserScript = fmt.Sprintf("DROP KEYSPACE IF EXISTS %s;", applicationId)
	} else if p.GetStoreType() == "SQL" {
		dropUserScript = fmt.Sprintf("DROP DATABASE IF EXISTS %s;", applicationId)
	} else {
		return errors.New("unknown backend type " + p.GetStoreType())
	}

	// I will execute the sript with the admin function.
	// TODO implement drop user for scylla and sql
	if p.GetStoreType() == "MONGO" {
		err = p.RunAdminCmd(context.Background(), "local_resource", srv.Backend_user, srv.Backend_password, dropUserScript)
		if err != nil {
			return err
		}
	}

	// set back the domain part
	applicationId = application["_id"].(string) + "@" + application["domain"].(string)

	srv.publishEvent("delete_application_"+applicationId+"_evt", []byte{}, application["domain"].(string))
	srv.publishEvent("delete_application_evt", []byte(applicationId), application["domain"].(string))

	return nil
}

// DeleteApplication deletes an application identified by the given ApplicationId.
// It interacts with the persistence service to remove the application data.
// Returns a response indicating the result of the deletion or an error if the operation fails.
// TODO: Also deletes directory permissions associated with the application.
func (srv *server) DeleteApplication(ctx context.Context, rqst *resourcepb.DeleteApplicationRqst) (*resourcepb.DeleteApplicationRsp, error) {

	// That service made user of persistence service.
	err := srv.deleteApplication(ctx, rqst.ApplicationId)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// TODO delete dir permission associate with the application.

	return &resourcepb.DeleteApplicationRsp{
		Result: true,
	}, nil
}

// GetApplicationAlias retrieves the alias of an application by its ID.
// It queries the persistence store for the application document and returns the alias field.
// Returns a GetApplicationAliasRsp containing the alias, or an error if the operation fails.
func (srv *server) GetApplicationAlias(ctx context.Context, rqst *resourcepb.GetApplicationAliasRqst) (*resourcepb.GetApplicationAliasRsp, error) {
	// That service made user of persistence service.
	p, err := srv.getPersistenceStore()
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	q := `{"_id":"` + rqst.Id + `"}`

	// Now I will retrieve the application icon...
	data, err := p.FindOne(context.Background(), "local_resource", "local_resource", "Applications", q, `[{"Projection":{"alias":1}}]`)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &resourcepb.GetApplicationAliasRsp{
		Alias: data.(string),
	}, nil
}

// GetApplicationIcon retrieves the icon associated with a specific application.
// It takes a context and a GetApplicationIconRqst containing the application's ID.
// The function queries the persistence store for the application's icon and returns it
// in a GetApplicationIconRsp. If an error occurs during retrieval, an appropriate gRPC
// status error is returned.
func (srv *server) GetApplicationIcon(ctx context.Context, rqst *resourcepb.GetApplicationIconRqst) (*resourcepb.GetApplicationIconRsp, error) {
	p, err := srv.getPersistenceStore()
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	q := `{"_id":"` + rqst.Id + `"}`

	// Now I will retrieve the application icon...
	data, err := p.FindOne(context.Background(), "local_resource", "local_resource", "Applications", q, `[{"Projection":{"icon":1}}]`)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &resourcepb.GetApplicationIconRsp{
		Icon: data.(string),
	}, nil
}

func (srv *server) getApplications(query string, options string) ([]*resourcepb.Application, error) {

	p, err := srv.getPersistenceStore()
	if err != nil {
		return nil, err
	}

	if len(query) == 0 {
		query = "{}"
	}

	// So here I will get the list of retrieved permission.
	values, err := p.Find(context.Background(), "local_resource", "local_resource", "Applications", query, options)
	if err != nil {
		return nil, err
	}

	applications := make([]*resourcepb.Application, 0)

	// Convert to Application.
	for i := 0; i < len(values); i++ {
		values_ := values[i].(map[string]interface{})

		if values_["icon"] == nil {
			values_["icon"] = ""
		}

		if values_["alias"] == nil {
			values_["alias"] = ""
		}

		// Set the date
		creationDate := int64(Utility.ToInt(values_["creation_date"]))
		lastDeployed := int64(Utility.ToInt(values_["last_deployed"]))

		// Here I will also append the list of actions.
		actions := make([]string, 0)

		if values_["actions"] != nil {

			var actions_ []interface{}
			switch values_["actions"].(type) {
			case primitive.A:
				actions_ = []interface{}(values_["actions"].(primitive.A))
			case []interface{}:
				actions_ = []interface{}(values_["actions"].([]interface{}))
			default:
				logger.Warn("unknown type", "value", values_["actions"])
			}

			for i := 0; i < len(actions_); i++ {
				actions = append(actions, actions_[i].(string))
			}
		}

		// Now the list of keywords.
		keywords := make([]string, 0)

		if values_["keywords"] != nil {

			var keywords_ []interface{}
			switch values_["keywords"].(type) {
			case primitive.A:
				keywords_ = []interface{}(values_["keywords"].(primitive.A))
			case []interface{}:
				keywords_ = []interface{}(values_["keywords"].([]interface{}))
			default:
				logger.Warn("unknown type", "value", values_["keywords"])
			}

			for i := 0; i < len(keywords_); i++ {
				keywords = append(keywords, keywords_[i].(string))
			}
		}

		application := &resourcepb.Application{Id: values_["_id"].(string), Name: values_["name"].(string), Domain: values_["domain"].(string), Path: values_["path"].(string), CreationDate: creationDate, LastDeployed: lastDeployed, Alias: values_["alias"].(string), Icon: values_["icon"].(string), Description: values_["description"].(string), PublisherID: values_["PublisherID"].(string), Version: values_["version"].(string), Actions: actions, Keywords: keywords}

		// TODO validate token...
		application.Password = values_["password"].(string)

		applications = append(applications, application)
	}

	return applications, nil
}

// GetApplications streams application data matching the specified query and options.
// It retrieves the list of applications using srv.getApplications and sends each application
// individually to the client via the provided gRPC stream. If an error occurs during retrieval
// or streaming, the error is returned.
//
// Parameters:
//
//	rqst   - The request containing query and options for filtering applications.
//	stream - The gRPC server stream to send application responses.
//
// Returns:
//
//	error - An error if retrieval or streaming fails, otherwise nil.
func (srv *server) GetApplications(rqst *resourcepb.GetApplicationsRqst, stream resourcepb.ResourceService_GetApplicationsServer) error {

	applications, err := srv.getApplications(rqst.Query, rqst.Options)

	if err != nil {
		return err
	}

	for i := 0; i < len(applications); i++ {
		err := stream.Send(&resourcepb.GetApplicationsRsp{
			Applications: []*resourcepb.Application{applications[i]},
		})
		if err != nil {
			return err
		}
	}

	return nil
}

// GetApplicationVersion retrieves the version of an application specified by its ID.
// It determines the underlying persistence store type (MongoDB, ScyllaDB, or SQL) and constructs
// the appropriate query to fetch the application's version from the "Applications" collection/table.
// Returns a GetApplicationVersionRsp containing the version string, or an error if the operation fails.
//
// Parameters:
//
//	ctx - The context for the request.
//	rqst - The request containing the application's ID.
//
// Returns:
//
//	*resourcepb.GetApplicationVersionRsp - The response containing the application's version.
//	error - An error if the operation fails or the database type is unknown.
func (srv *server) GetApplicationVersion(ctx context.Context, rqst *resourcepb.GetApplicationVersionRqst) (*resourcepb.GetApplicationVersionRsp, error) {

	p, err := srv.getPersistenceStore()
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	var q string
	if p.GetStoreType() == "MONGO" {
		q = `{"name":"` + rqst.Id + `"}`
	} else if p.GetStoreType() == "SCYLLA" || p.GetStoreType() == "SQL" {
		q = `SELECT * FROM Applications WHERE name='` + rqst.Id + `'`
	} else {
		return nil, errors.New("unknown database type " + p.GetStoreType())
	}

	var previousVersion string
	previous, err := p.FindOne(context.Background(), "local_resource", "local_resource", "Applications", q, `[{"Projection":{"version":1}}]`)
	if err == nil {
		if previous != nil {
			if previous.(map[string]interface{})["version"] != nil {
				previousVersion = previous.(map[string]interface{})["version"].(string)
			}
		}
	} else {
		return nil, status.Errorf(
			codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &resourcepb.GetApplicationVersionRsp{
		Version: previousVersion,
	}, nil

}

// RemoveApplicationAction removes a specified action from an application's list of actions.
// It retrieves the application by its ID, checks if the action exists, and removes it if present.
// If the action does not exist, an error is returned. The updated application is saved back to the persistence store.
// An event is published to notify about the update.
//
// Parameters:
//
//	ctx - The context for the request.
//	rqst - The request containing the ApplicationId and Action to be removed.
//
// Returns:
//
//	*resourcepb.RemoveApplicationActionRsp - The response indicating the result of the operation.
//	error - An error if the operation fails.
func (srv *server) RemoveApplicationAction(ctx context.Context, rqst *resourcepb.RemoveApplicationActionRqst) (*resourcepb.RemoveApplicationActionRsp, error) {

	// That service made user of persistence service.
	p, err := srv.getPersistenceStore()
	if err != nil {
		return nil, err
	}

	q := `{"_id":"` + rqst.ApplicationId + `"}`

	values, err := p.FindOne(context.Background(), "local_resource", "local_resource", "Applications", q, ``)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	application := values.(map[string]interface{})

	needSave := false
	if application["actions"] == nil {
		application["actions"] = []string{rqst.Action}
		needSave = true
	} else {
		exist := false
		actions := make([]interface{}, 0)

		var actions_ []interface{}
		switch application["actions"].(type) {
		case primitive.A:
			actions_ = []interface{}(application["actions"].(primitive.A))
		case []interface{}:
			actions_ = []interface{}(application["actions"].([]interface{}))
		default:
			logger.Warn("unknown type", "value", application["actions"])
		}

		for i := 0; i < len(actions_); i++ {
			if actions_[i].(string) == rqst.Action {
				exist = true
			} else {
				actions = append(actions, actions_[i])
			}
		}

		if exist {
			application["actions"] = actions
			needSave = true
		} else {
			return nil, status.Errorf(
				codes.Internal,
				"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("Application named "+rqst.ApplicationId+" not contain actions named "+rqst.Action+"!")))
		}
	}

	if needSave {
		jsonStr := serialyseObject(application)
		err := p.ReplaceOne(context.Background(), "local_resource", "local_resource", "Applications", q, string(jsonStr), ``)
		if err != nil {
			return nil, status.Errorf(
				codes.Internal,
				"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
	}

	srv.publishEvent("update_application_"+rqst.ApplicationId+"_evt", []byte{}, srv.Address)

	return &resourcepb.RemoveApplicationActionRsp{Result: true}, nil
}

// RemoveApplicationsAction removes a specified action from the "actions" field of all application records
// in the persistence store. It supports multiple database types (MongoDB, ScyllaDB, SQL) and updates each
// application by removing the given action if present. If the action is removed, the application record is
// updated in the database and an update event is published. Returns a response indicating success or an error
// if any operation fails.
//
// Parameters:
//
//	ctx - The context for the request.
//	rqst - The request containing the action to be removed.
//
// Returns:
//
//	*resourcepb.RemoveApplicationsActionRsp - The response indicating the result of the operation.
//	error - An error if the operation fails.
func (srv *server) RemoveApplicationsAction(ctx context.Context, rqst *resourcepb.RemoveApplicationsActionRqst) (*resourcepb.RemoveApplicationsActionRsp, error) {

	// That service made user of persistence service.
	p, err := srv.getPersistenceStore()
	if err != nil {
		return nil, err
	}

	var q string
	if p.GetStoreType() == "MONGO" {
		q = `{}`
	} else if p.GetStoreType() == "SCYLLA" {
		q = `` // TODO scylla db query.
	} else if p.GetStoreType() == "SQL" {
		q = `SELECT * FROM Applications` // TODO sql query string here...
	} else {
		return nil, errors.New("unknown database type " + p.GetStoreType())
	}

	values, err := p.Find(context.Background(), "local_resource", "local_resource", "Applications", q, ``)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	for i := range values {
		application := values[i].(map[string]interface{})

		needSave := false
		if application["actions"] == nil {
			application["actions"] = []string{rqst.Action}
			needSave = true
		} else {
			exist := false
			actions := make([]interface{}, 0)

			var actions_ []interface{}
			switch application["actions"].(type) {
			case primitive.A:
				actions_ = []interface{}(application["actions"].(primitive.A))
			case []interface{}:
				actions_ = []interface{}(application["actions"].([]interface{}))
			default:
				logger.Warn("unknown type", "value", application["actions"])
			}

			for i := 0; i < len(actions_); i++ {
				if actions_[i].(string) == rqst.Action {
					exist = true
				} else {
					actions = append(actions, actions_[i])
				}
			}
			if exist {
				application["actions"] = actions
				needSave = true
			}
		}

		if needSave {
			jsonStr := serialyseObject(application)
			q = `{"_id":"` + application["_id"].(string) + `"}`
			err := p.ReplaceOne(context.Background(), "local_resource", "local_resource", "Applications", q, string(jsonStr), ``)
			srv.publishEvent("update_application_"+application["_id"].(string)+"_evt", []byte{}, srv.Address)
			if err != nil {
				return nil, status.Errorf(
					codes.Internal,
					"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
			}
		}
	}

	return &resourcepb.RemoveApplicationsActionRsp{Result: true}, nil
}

func (srv *server) save_application(ctx context.Context, app *resourcepb.Application, owner string) error {

	_, token, err := security.GetClientId(ctx)
	if err != nil {
		return err
	}

	p, err := srv.getPersistenceStore()
	if err != nil {
		return err
	}

	if app == nil {
		return errors.New("no application object was given in the request")
	}

	q := `{"_id":"` + app.Id + `"}`

	_, err = p.Count(context.Background(), "local_resource", "local_resource", "Applications", q, "")

	application := make(map[string]interface{}, 0)
	application["_id"] = app.Id
	application["name"] = app.Name
	application["path"] = "/" + app.Name // The path must be the same as the application name.
	application["PublisherID"] = app.PublisherID
	application["version"] = app.Version
	application["domain"] = srv.Domain // the domain where the application is save...
	application["description"] = app.Description
	application["actions"] = app.Actions
	application["keywords"] = app.Keywords
	application["icon"] = app.Icon
	application["alias"] = app.Alias

	// be sure the domain is set correctly
	if len(app.Domain) == 0 {
		app.Domain, _ = config.GetDomain()
	}

	application["domain"] = app.Domain
	application["password"] = app.Password

	if len(application["password"].(string)) == 0 {
		application["password"] = app.Id
	}
	application["store"] = p.GetStoreType()

	// Save the actual time.
	application["last_deployed"] = time.Now().Unix() // save it as unix time.


	// Here I will set the resource to manage the applicaiton access permission.
	if err != nil {
		application["creation_date"] = time.Now().Unix() // save it as unix time.
		_, err := p.InsertOne(context.Background(), "local_resource", "local_resource", "Applications", application, "")
		if err != nil {
			logger.Info("log", "args", []interface{}{"error while inserting application ", err})
			return err
		}

	} else {
		if app.CreationDate == 0 {
			application["creation_date"] = time.Now().Unix() // save it as unix time.
		} else {
			application["creation_date"] = app.CreationDate
		}

		jsonStr, _ := Utility.ToJson(application)

		err := p.ReplaceOne(context.Background(), "local_resource", "local_resource", "Applications", q, jsonStr, "")

		if err != nil {
			return err
		}
	}

	// Create the application file directory.
	path := "/applications/" + app.Name
	Utility.CreateDirIfNotExist(config.GetDataDir() + "/files" + path)

	// Add resource owner
	srv.addResourceOwner(token, path, "file", app.Id+"@"+app.Domain, rbacpb.SubjectType_APPLICATION)

	// Add application owner
	srv.addResourceOwner(token, app.Id+"@"+app.Domain, "application", owner, rbacpb.SubjectType_ACCOUNT)

	// Publish application.
	srv.publishEvent("update_application_"+app.Id+"@"+app.Domain+"_evt", []byte{}, srv.Address)

	return nil
}

// UpdateApplication updates an existing application in the persistence store with the provided values.
// It retrieves the persistence store, constructs a query to find the application by its ID, and updates
// the application's fields. If successful, it publishes an update event. Returns an UpdateApplicationRsp
// response or an error if the operation fails.
//
// Parameters:
//
//	ctx - The context for the request.
//	rqst - The request containing the application ID and values to update.
//
// Returns:
//
//	*resourcepb.UpdateApplicationRsp - The response indicating the result of the update operation.
//	error - An error if the update fails.
func (srv *server) UpdateApplication(ctx context.Context, rqst *resourcepb.UpdateApplicationRqst) (*resourcepb.UpdateApplicationRsp, error) {
	p, err := srv.getPersistenceStore()
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	q := `{"_id":"` + rqst.ApplicationId + `"}`

	err = p.UpdateOne(context.Background(), "local_resource", "local_resource", "Applications", q, rqst.Values, "")
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	srv.publishEvent("update_application_"+rqst.ApplicationId+"_evt", []byte{}, srv.Address)

	return &resourcepb.UpdateApplicationRsp{}, nil
}
