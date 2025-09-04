package main

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"sort"
	"strings"

	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/resource/resourcepb"
	Utility "github.com/globulario/utility"
	"github.com/txn2/txeh"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ClearAllNotifications removes all notifications for a specified recipient.
// It validates the recipient, normalizes the database name, and constructs a query
// based on the underlying persistence store type (MongoDB, ScyllaDB, or SQL).
// After deleting the notifications, it publishes a "clear_notification_evt" event.
// Returns a ClearAllNotificationsRsp on success, or an error if the operation fails.
//
// Parameters:
//   ctx - The context for the request.
//   rqst - The request containing the recipient whose notifications should be cleared.
//
// Returns:
//   *resourcepb.ClearAllNotificationsRsp - The response indicating success.
//   error - An error if the operation fails.
func (srv *server) ClearAllNotifications(ctx context.Context, rqst *resourcepb.ClearAllNotificationsRqst) (*resourcepb.ClearAllNotificationsRsp, error) {

	if len(rqst.Recipient) == 0 {
		return nil, status.Errorf(
			codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("recipient is empty")))
	}

	db := rqst.Recipient
	if strings.Contains(db, "@") {
		db = strings.Split(db, "@")[0]
	}

	db = strings.ReplaceAll(db, "-", "_")
	db = strings.ReplaceAll(db, ".", "_")
	db = strings.ReplaceAll(db, " ", "_")

	db += "_db"

	if !strings.Contains(rqst.Recipient, "@") {
		rqst.Recipient += "@" + srv.Domain
	}

	p, err := srv.getPersistenceStore()
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	var query string
	if p.GetStoreType() == "MONGO" {
		query = `{}`
	} else if p.GetStoreType() == "SCYLLA" {
		query = `SELECT * FROM Notifications WHERE recipient='` + rqst.Recipient + `' ALLOW FILTERING`
	} else if p.GetStoreType() == "SQL" {
		query = `SELECT * FROM Notifications WHERE recipient='` + rqst.Recipient + `'`
	} else {
		return nil, errors.New("unknown database type " + p.GetStoreType())
	}

	err = p.Delete(context.Background(), "local_resource", db, "Notifications", query, ``)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	srv.publishEvent("clear_notification_evt", []byte{}, srv.Address)

	return &resourcepb.ClearAllNotificationsRsp{}, nil
}

// ClearNotificationsByType removes all notifications of a specified type for a given recipient.
// It determines the appropriate database name based on the recipient, constructs a query depending
// on the underlying persistence store type (MONGO, SCYLLA, or SQL), and deletes matching notifications.
// After successful deletion, it publishes an event to notify concerned clients about the cleared notifications.
// Returns an empty ClearNotificationsByTypeRsp on success, or an error if the operation fails.
func (srv *server) ClearNotificationsByType(ctx context.Context, rqst *resourcepb.ClearNotificationsByTypeRqst) (*resourcepb.ClearNotificationsByTypeRsp, error) {
	p, err := srv.getPersistenceStore()
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	notificationType := int32(rqst.NotificationType)

	db := rqst.Recipient
	if strings.Contains(db, "@") {
		db = strings.Split(db, "@")[0]
	}

	db = strings.ReplaceAll(db, "-", "_")
	db = strings.ReplaceAll(db, ".", "_")
	db = strings.ReplaceAll(db, " ", "_")

	db += "_db"

	var query string
	if p.GetStoreType() == "MONGO" {
		query = `{ "notificationtype":` + Utility.ToString(notificationType) + `}`
	} else if p.GetStoreType() == "SCYLLA" || p.GetStoreType() == "SQL" {
		query = `SELECT * FROM Notifications`
	} else {
		return nil, errors.New("unknown database type " + p.GetStoreType())
	}

	err = p.Delete(context.Background(), "local_resource", db, "Notifications", query, ``)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// Send event to all concern client.
	domain, _ := config.GetDomain()
	evt_client, err := getEventClient(domain)
	if err == nil {
		evt := rqst.Recipient + "_clear_user_notifications_evt"
		evt_client.Publish(evt, []byte{})
	}

	return &resourcepb.ClearNotificationsByTypeRsp{}, nil
}

// CreateNotification handles the creation of a notification for a user.
// It checks if a notification with the given ID already exists for the recipient.
// If the recipient belongs to a different domain, the request is redirected to the appropriate resource client.
// Otherwise, the notification is inserted into the recipient's database and an event is published.
// Returns a CreateNotificationRsp on success, or an error if the operation fails.
func (srv *server) CreateNotification(ctx context.Context, rqst *resourcepb.CreateNotificationRqst) (*resourcepb.CreateNotificationRsp, error) {

	p, err := srv.getPersistenceStore()
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	q := `{"_id":"` + rqst.Notification.Id + `"}`

	// so the recipient here is the id of the user...
	recipient := strings.Split(rqst.Notification.Recipient, "@")[0]

	name := strings.ReplaceAll(recipient, "-", "_")
	name = strings.ReplaceAll(name, ".", "_")
	name = strings.ReplaceAll(name, " ", "_")

	count, _ := p.Count(context.Background(), "local_resource", name+"_db", "Notifications", q, "")
	if count > 0 {
		return nil, status.Errorf(
			codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("Notification with id "+rqst.Notification.Id+" already exist")))
	}

	// if the account is not on the domain will redirect the request...
	if rqst.Notification.NotificationType == resourcepb.NotificationType_USER_NOTIFICATION {
		recipient := rqst.Notification.Recipient
		localDomain, _ := config.GetDomain()
		if strings.Contains(recipient, "@") {
			domain := strings.Split(recipient, "@")[1]

			if localDomain != domain {
				client, err := getResourceClient(domain)
				if err != nil {
					return nil, status.Errorf(
						codes.Internal,
						"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
				}

				err = client.CreateNotification(rqst.Notification)
				if err != nil {
					return nil, status.Errorf(
						codes.Internal,
						"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
				}

				return &resourcepb.CreateNotificationRsp{}, nil
			}
		} else {
			recipient += "@" + localDomain
		}
	}

	// insert notification into recipient database
	_, err = p.InsertOne(context.Background(), "local_resource", name+"_db", "Notifications", rqst.Notification, "")
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	jsonStr, err := json.Marshal(rqst.Notification)
	if err == nil {
		srv.publishEvent("create_notification_evt", jsonStr, srv.GetAddress())
	}

	return &resourcepb.CreateNotificationRsp{}, nil
}

// DeleteNotification deletes a notification for a specified recipient.
// It validates the recipient, normalizes the database name, and ensures the recipient
// has a domain suffix. The function then attempts to delete the notification from the
// persistence store and publishes relevant events upon successful deletion.
//
// Parameters:
//   ctx - The context for the request.
//   rqst - The request containing the notification ID and recipient.
//
// Returns:
//   *resourcepb.DeleteNotificationRsp - The response object for the deletion operation.
//   error - An error if the deletion fails or if input validation fails.
func (srv *server) DeleteNotification(ctx context.Context, rqst *resourcepb.DeleteNotificationRqst) (*resourcepb.DeleteNotificationRsp, error) {

	if len(rqst.Recipient) == 0 {
		return nil, status.Errorf(
			codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("recipient is empty")))
	}

	db := rqst.Recipient
	if strings.Contains(db, "@") {
		db = strings.Split(db, "@")[0]
	}

	db = strings.ReplaceAll(db, "-", "_")
	db = strings.ReplaceAll(db, ".", "_")
	db = strings.ReplaceAll(db, " ", "_")

	db += "_db"

	if !strings.Contains(rqst.Recipient, "@") {
		rqst.Recipient += "@" + srv.Domain
	}

	p, err := srv.getPersistenceStore()
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	q := `{"_id":"` + rqst.Id + `"}`

	err = p.DeleteOne(context.Background(), "local_resource", db, "Notifications", q, ``)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	srv.publishEvent("delete_notification_"+rqst.Id+"_evt", []byte{}, srv.Address)
	srv.publishEvent("delete_notification_evt", []byte(rqst.Id), srv.Address)

	return &resourcepb.DeleteNotificationRsp{}, nil
}

// FindPackages searches for package descriptors in the persistence store based on the provided keywords.
// It constructs a query depending on the underlying database type (MongoDB, ScyllaDB, or SQL) and retrieves matching packages.
// The function returns a FindPackagesDescriptorResponse containing the list of found package descriptors or an error if the operation fails.
//
// Parameters:
//   ctx - The context for the request.
//   rqst - The request containing the keywords to search for.
//
// Returns:
//   *resourcepb.FindPackagesDescriptorResponse - The response containing the found package descriptors.
//   error - An error if the operation fails.
func (srv *server) FindPackages(ctx context.Context, rqst *resourcepb.FindPackagesDescriptorRequest) (*resourcepb.FindPackagesDescriptorResponse, error) {
	// That service made user of persistence service.
	p, err := srv.getPersistenceStore()
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	kewordsStr, err := Utility.ToJson(rqst.Keywords)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	var query string
	if p.GetStoreType() == "MONGO" {
		query = `{"keywords": { "$all" : ` + kewordsStr + `}}`
	} else if p.GetStoreType() == "SCYLLA" {
		query = `SELECT * FROM Packages WHERE keywords='` + kewordsStr + `' ALLOW FILTERING`
	} else if p.GetStoreType() == "SQL" {
		query = `SELECT * FROM Packages WHERE keywords='` + kewordsStr + `'`
	} else {
		return nil, errors.New("unknown database type " + p.GetStoreType())
	}

	data, err := p.Find(context.Background(), "local_resource", "local_resource", "Packages", query, "")
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	descriptors := make([]*resourcepb.PackageDescriptor, len(data))
	for i := range data {
		descriptor := data[i].(map[string]interface{})
		descriptors[i] = new(resourcepb.PackageDescriptor)
		descriptors[i].TypeName = "PackageDescriptor"
		descriptors[i].Id = descriptor["_id"].(string)
		descriptors[i].Name = descriptor["name"].(string)
		descriptors[i].Description = descriptor["description"].(string)
		descriptors[i].PublisherID = descriptor["PublisherID"].(string)
		descriptors[i].Version = descriptor["version"].(string)
		descriptors[i].Icon = descriptor["icon"].(string)
		descriptors[i].Alias = descriptor["alias"].(string)
		if descriptor["keywords"] != nil {

			var keywords []interface{}
			switch descriptor["keywords"].(type) {
			case primitive.A:
				keywords = []interface{}(descriptor["keywords"].(primitive.A))
			case []interface{}:
				keywords = descriptor["keywords"].([]interface{})
			}

			descriptors[i].Keywords = make([]string, len(keywords))
			for j := 0; j < len(keywords); j++ {
				descriptors[i].Keywords[j] = keywords[j].(string)
			}
		}
		if descriptor["actions"] != nil {

			var actions []interface{}
			switch descriptor["actions"].(type) {
			case primitive.A:
				actions = []interface{}(descriptor["actions"].(primitive.A))
			case []interface{}:
				actions = descriptor["actions"].([]interface{})
			}

			descriptors[i].Actions = make([]string, len(actions))
			for j := 0; j < len(actions); j++ {
				descriptors[i].Actions[j] = actions[j].(string)
			}
		}
		if descriptor["discoveries"] != nil {

			var discoveries []interface{}
			switch descriptor["discoveries"].(type) {
			case primitive.A:
				discoveries = []interface{}(descriptor["discoveries"].(primitive.A))
			case []interface{}:
				discoveries = descriptor["discoveries"].([]interface{})
			}

			descriptors[i].Discoveries = make([]string, len(discoveries))
			for j := 0; j < len(discoveries); j++ {
				descriptors[i].Discoveries[j] = discoveries[j].(string)
			}
		}

		if descriptor["repositories"] != nil {

			var repositories []interface{}
			switch descriptor["repositories"].(type) {
			case primitive.A:
				repositories = []interface{}(descriptor["repositories"].(primitive.A))
			case []interface{}:
				repositories = descriptor["repositories"].([]interface{})
			}

			descriptors[i].Repositories = make([]string, len(repositories))
			for j := 0; j < len(repositories); j++ {
				descriptors[i].Repositories[j] = repositories[j].(string)
			}
		}
	}

	// Return the list of Service Descriptor.
	return &resourcepb.FindPackagesDescriptorResponse{
		Results: descriptors,
	}, nil
}

// GetNotifications streams notifications for a specified recipient to the client.
// It validates the recipient, constructs the appropriate database name, and queries
// the persistence store for notifications matching the recipient. The results are
// streamed in batches to the client using the provided gRPC stream. If any error
// occurs during processing or streaming, an appropriate gRPC status error is returned.
//
// Parameters:
//   rqst   - The request containing the recipient for whom notifications are to be fetched.
//   stream - The gRPC server stream used to send notifications to the client.
//
// Returns:
//   error - Returns an error if the recipient is empty, if there is a problem with
//           the persistence store, or if streaming notifications fails.
func (srv *server) GetNotifications(rqst *resourcepb.GetNotificationsRqst, stream resourcepb.ResourceService_GetNotificationsServer) error {

	if len(rqst.Recipient) == 0 {
		return status.Errorf(
			codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("recipient is empty")))
	}

	db := rqst.Recipient
	if strings.Contains(db, "@") {
		db = strings.Split(db, "@")[0]
	}

	db = strings.ReplaceAll(db, "-", "_")
	db = strings.ReplaceAll(db, ".", "_")
	db = strings.ReplaceAll(db, " ", "_")

	db += "_db"

	if !strings.Contains(rqst.Recipient, "@") {
		rqst.Recipient += "@" + srv.Domain
	}

	// Get the persistence connection
	p, err := srv.getPersistenceStore()
	if err != nil {
		return err
	}

	query := `{"recipient":"` + rqst.Recipient + `"}`
	if len(query) == 0 {
		query = "{}"
	}

	notifications, err := p.Find(context.Background(), "local_resource", db, "Notifications", query, "")
	if err != nil {
		return status.Errorf(
			codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// No I will stream the result over the networks.
	maxSize := 50
	values := make([]*resourcepb.Notification, 0)
	for i := range notifications {
		n_ := notifications[i].(map[string]interface{})
		notificationType := resourcepb.NotificationType(int32(Utility.ToInt(n_["notificationtype"])))
		noticationDate := Utility.ToInt(n_["date"])

		values = append(values, &resourcepb.Notification{Id: n_["_id"].(string), Mac: n_["mac"].(string), Sender: n_["sender"].(string), Date: int64(noticationDate), Recipient: n_["recipient"].(string), Message: n_["message"].(string), NotificationType: notificationType})
		if len(values) >= maxSize {
			err := stream.Send(
				&resourcepb.GetNotificationsRsp{
					Notifications: values,
				},
			)
			if err != nil {
				return status.Errorf(
					codes.Internal,
					"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
			}
			values = make([]*resourcepb.Notification, 0)
		}
	}

	// Send reminding values.
	err = stream.Send(
		&resourcepb.GetNotificationsRsp{
			Notifications: values,
		},
	)

	if err != nil {
		return status.Errorf(
			codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return nil
}

// GetPackageBundleChecksum retrieves the checksum of a package bundle identified by the given request ID.
// It queries the persistence store for the bundle and returns its checksum in the response.
// Returns an error if the bundle cannot be found or if there is a problem accessing the persistence store.
func (srv *server) GetPackageBundleChecksum(ctx context.Context, rqst *resourcepb.GetPackageBundleChecksumRequest) (*resourcepb.GetPackageBundleChecksumResponse, error) {
	p, err := srv.getPersistenceStore()
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	q := `{"_id":"` + rqst.Id + `"}`

	values, err := p.FindOne(context.Background(), "local_resource", "local_resource", "Bundles", q, "")
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// Here I will retrieve the values from the db and
	return &resourcepb.GetPackageBundleChecksumResponse{
		Checksum: values.(map[string]interface{})["checksum"].(string),
	}, nil

}

// GetPackageDescriptor retrieves package descriptors based on the provided service ID and publisher ID.
// It queries the underlying persistence store (MongoDB, ScyllaDB, or SQL) for matching package records.
// The function constructs the appropriate query for the store type, fetches the results, and maps them
// to resourcepb.PackageDescriptor objects, including associated groups and roles. If multiple descriptors
// are found, they are sorted by version in descending order. Returns a GetPackageDescriptorResponse containing
// the descriptors or an error if the operation fails.
func (srv *server) GetPackageDescriptor(ctx context.Context, rqst *resourcepb.GetPackageDescriptorRequest) (*resourcepb.GetPackageDescriptorResponse, error) {

	p, err := srv.getPersistenceStore()
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	var query string
	if p.GetStoreType() == "MONGO" {
		query = `{"name":"` + rqst.ServiceId + `", "PublisherID":"` + rqst.PublisherID + `"}`
	} else if p.GetStoreType() == "SCYLLA" {
		query = `SELECT * FROM Packages WHERE name='` + rqst.ServiceId + `' AND PublisherID='` + rqst.PublisherID + `' ALLOW FILTERING`
	} else if p.GetStoreType() == "SQL" {
		query = `SELECT * FROM Packages WHERE name='` + rqst.ServiceId + `' AND PublisherID='` + rqst.PublisherID + `'`
	} else {
		return nil, errors.New("unknown database type " + p.GetStoreType())
	}

	values, err := p.Find(context.Background(), "local_resource", "local_resource", "Packages", query, "")
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	if len(values) == 0 {
		return nil, status.Errorf(
			codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("No package descriptor with id "+rqst.ServiceId+" was found for publisher id "+rqst.PublisherID)))
	}

	descriptors := make([]*resourcepb.PackageDescriptor, len(values))
	for i := 0; i < len(values); i++ {
		descriptor := values[i].(map[string]interface{})
		descriptors[i] = new(resourcepb.PackageDescriptor)
		descriptors[i].TypeName = "PackageDescriptor"
		descriptors[i].Id = descriptor["_id"].(string)
		descriptors[i].Name = descriptor["name"].(string)
		if descriptor["alias"] != nil {
			descriptors[i].Alias = descriptor["alias"].(string)
		} else {
			descriptors[i].Alias = descriptors[i].Name
		}
		if descriptor["icon"] != nil {
			descriptors[i].Icon = descriptor["icon"].(string)
		}
		if descriptor["description"] != nil {
			descriptors[i].Description = descriptor["description"].(string)
		}
		if descriptor["PublisherID"] != nil {
			descriptors[i].PublisherID = descriptor["PublisherID"].(string)
		}
		if descriptor["version"] != nil {
			descriptors[i].Version = descriptor["version"].(string)
		}
		descriptors[i].Type = resourcepb.PackageType(Utility.ToInt(descriptor["type"]))

		if descriptor["keywords"] != nil {

			var keywords []interface{}
			switch descriptor["keywords"].(type) {
			case primitive.A:
				keywords = []interface{}(descriptor["keywords"].(primitive.A))
			case []interface{}:
				keywords = descriptor["keywords"].([]interface{})
			}

			descriptors[i].Keywords = make([]string, len(keywords))
			for j := 0; j < len(keywords); j++ {
				descriptors[i].Keywords[j] = keywords[j].(string)
			}
		}

		if descriptor["actions"] != nil {
			var actions []interface{}
			switch descriptor["actions"].(type) {
			case primitive.A:
				actions = []interface{}(descriptor["actions"].(primitive.A))
			case []interface{}:
				actions = descriptor["actions"].([]interface{})
			}

			descriptors[i].Actions = make([]string, len(actions))
			for j := 0; j < len(actions); j++ {
				descriptors[i].Actions[j] = actions[j].(string)
			}
		}

		if descriptor["discoveries"] != nil {

			var discoveries []interface{}
			switch descriptor["discoveries"].(type) {
			case primitive.A:
				discoveries = []interface{}(descriptor["discoveries"].(primitive.A))
			case []interface{}:
				discoveries = descriptor["discoveries"].([]interface{})
			}

			descriptors[i].Discoveries = make([]string, len(discoveries))
			for j := 0; j < len(discoveries); j++ {
				descriptors[i].Discoveries[j] = discoveries[j].(string)
			}
		}

		if descriptor["repositories"] != nil {

			var repositories []interface{}
			switch descriptor["repositories"].(type) {
			case primitive.A:
				repositories = []interface{}(descriptor["repositories"].(primitive.A))
			case []interface{}:
				repositories = descriptor["repositories"].([]interface{})
			}
			descriptors[i].Repositories = make([]string, len(repositories))
			for j := 0; j < len(repositories); j++ {
				descriptors[i].Repositories[j] = repositories[j].(string)
			}
		}

		if descriptor["groups"] != nil {
			var groups []interface{}
			switch descriptor["groups"].(type) {
			case primitive.A:
				groups = []interface{}(descriptor["groups"].(primitive.A))
			case []interface{}:
				groups = descriptor["groups"].([]interface{})
			}

			descriptors[i].Groups = make([]*resourcepb.Group, len(groups))

			for j := 0; j < len(groups); j++ {
				groupId := groups[j].(map[string]interface{})["$id"].(string)
				g, err := srv.getGroup(groupId)
				if err == nil {
					descriptors[i].Groups[j] = g
				}
			}
		}

		if descriptor["roles"] != nil {

			var roles []interface{}
			switch descriptor["roles"].(type) {
			case primitive.A:
				roles = []interface{}(descriptor["roles"].(primitive.A))
			case []interface{}:
				roles = descriptor["roles"].([]interface{})
			}

			descriptors[i].Roles = make([]*resourcepb.Role, len(roles))

			for j := 0; j < len(roles); j++ {

				// Get the role id.
				roleId := roles[j].(map[string]interface{})["$id"].(string)

				// Get the role.
				role_, err := srv.getRole(roleId)
				if err == nil {
					// set it back in the package descriptor.
					descriptors[i].Roles[j] = role_
				}
			}
		}
	}
	if len(descriptors) > 1 {
		sort.Slice(descriptors[:], func(i, j int) bool {
			return descriptors[i].Version > descriptors[j].Version
		})
	}

	// Return the list of Service Descriptor.
	return &resourcepb.GetPackageDescriptorResponse{
		Results: descriptors,
	}, nil
}

// GetPackagesDescriptor streams package descriptors matching the provided query to the client.
// It retrieves package data from the persistence store, constructs PackageDescriptor objects,
// and sends them in batches of 20 via the gRPC stream. The method supports filtering by query
// and options, and handles various fields such as keywords, actions, discoveries, and repositories.
// Returns an error if data retrieval or streaming fails.
func (srv *server) GetPackagesDescriptor(rqst *resourcepb.GetPackagesDescriptorRequest, stream resourcepb.ResourceService_GetPackagesDescriptorServer) error {
	p, err := srv.getPersistenceStore()
	if err != nil {
		return status.Errorf(
			codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	query := rqst.Query
	if len(query) == 0 {
		query = "{}"
	}

	data, err := p.Find(context.Background(), "local_resource", "local_resource", "Packages", query, rqst.Options)
	if err != nil {
		return status.Errorf(
			codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	descriptors := make([]*resourcepb.PackageDescriptor, 0)
	for i := 0; i < len(data); i++ {
		descriptor := new(resourcepb.PackageDescriptor)
		descriptor.TypeName = "PackageDescriptor"
		descriptor.Id = data[i].(map[string]interface{})["_id"].(string)
		descriptor.Name = data[i].(map[string]interface{})["name"].(string)
		descriptor.Description = data[i].(map[string]interface{})["description"].(string)
		descriptor.PublisherID = data[i].(map[string]interface{})["PublisherID"].(string)
		descriptor.Version = data[i].(map[string]interface{})["version"].(string)
		if data[i].(map[string]interface{})["icon"] != nil {
			descriptor.Icon = data[i].(map[string]interface{})["icon"].(string)
		}

		if data[i].(map[string]interface{})["alias"] != nil {
			descriptor.Alias = data[i].(map[string]interface{})["alias"].(string)
		}

		descriptor.Type = resourcepb.PackageType(Utility.ToInt(data[i].(map[string]interface{})["type"]))

		if data[i].(map[string]interface{})["keywords"] != nil {

			var keywords []interface{}
			switch data[i].(map[string]interface{})["keywords"].(type) {
			case primitive.A:
				keywords = []interface{}(data[i].(map[string]interface{})["keywords"].(primitive.A))
			case []interface{}:
				keywords = data[i].(map[string]interface{})["keywords"].([]interface{})
			}

			descriptor.Keywords = make([]string, len(keywords))
			for j := 0; j < len(keywords); j++ {
				descriptor.Keywords[j] = keywords[j].(string)
			}
		}

		if data[i].(map[string]interface{})["actions"] != nil {

			var actions []interface{}
			switch data[i].(map[string]interface{})["actions"].(type) {
			case primitive.A:
				actions = []interface{}(data[i].(map[string]interface{})["actions"].(primitive.A))
			case []interface{}:
				actions = data[i].(map[string]interface{})["actions"].([]interface{})
			}

			descriptor.Actions = make([]string, len(actions))
			for j := 0; j < len(actions); j++ {
				descriptor.Actions[j] = actions[j].(string)
			}
		}

		if data[i].(map[string]interface{})["discoveries"] != nil {

			var discoveries []interface{}
			switch data[i].(map[string]interface{})["discoveries"].(type) {
			case primitive.A:
				discoveries = []interface{}(data[i].(map[string]interface{})["discoveries"].(primitive.A))
			case []interface{}:
				discoveries = data[i].(map[string]interface{})["discoveries"].([]interface{})
			}

			descriptor.Discoveries = make([]string, len(discoveries))
			for j := 0; j < len(discoveries); j++ {
				descriptor.Discoveries[j] = discoveries[j].(string)
			}
		}

		if data[i].(map[string]interface{})["repositories"] != nil {

			var repositories []interface{}
			switch data[i].(map[string]interface{})["repositories"].(type) {
			case primitive.A:
				repositories = []interface{}(data[i].(map[string]interface{})["repositories"].(primitive.A))
			case []interface{}:
				repositories = data[i].(map[string]interface{})["repositories"].([]interface{})
			}

			descriptor.Repositories = make([]string, len(repositories))
			for j := 0; j < len(repositories); j++ {
				descriptor.Repositories[j] = repositories[j].(string)
			}
		}

		descriptors = append(descriptors, descriptor)
		// send at each 20
		if i%20 == 0 {
			stream.Send(&resourcepb.GetPackagesDescriptorResponse{
				Results: descriptors,
			})
			descriptors = make([]*resourcepb.PackageDescriptor, 0)
		}
	}

	if len(descriptors) > 0 {
		stream.Send(&resourcepb.GetPackagesDescriptorResponse{
			Results: descriptors,
		})
	}

	// Return the list of Service Descriptor.
	return nil
}

// SetPackageBundle stores or updates a package bundle in the persistence store.
// It generates a unique bundle ID based on the package descriptor and platform,
// serializes the bundle information to JSON, and replaces or upserts the bundle
// record in the "Bundles" collection of the "local_resource" database.
// Returns a SetPackageBundleResponse with Result=true on success, or an error
// if any operation fails.
//
// Parameters:
//   ctx  - context for request cancellation and deadlines
//   rqst - SetPackageBundleRequest containing the bundle to store
//
// Returns:
//   *resourcepb.SetPackageBundleResponse - response indicating success
//   error                                - error if the operation fails
func (srv *server) SetPackageBundle(ctx context.Context, rqst *resourcepb.SetPackageBundleRequest) (*resourcepb.SetPackageBundleResponse, error) {
	bundle := rqst.Bundle

	p, err := srv.getPersistenceStore()
	if err != nil {
		slog.Error("SetPackageBundle: getPersistenceStore failed", "file", Utility.FileLine(), "func", Utility.FunctionName(), "error", err.Error())
		return nil, status.Errorf(
			codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// Generate the bundle id....
	id := Utility.GenerateUUID(bundle.PackageDescriptor.PublisherID + "%" + bundle.PackageDescriptor.Name + "%" + bundle.PackageDescriptor.Version + "%" + bundle.PackageDescriptor.Id + "%" + bundle.Plaform)

	jsonStr, err := Utility.ToJson(map[string]interface{}{"_id": id, "checksum": bundle.Checksum, "platform": bundle.Plaform, "PublisherID": bundle.PackageDescriptor.PublisherID, "servicename": bundle.PackageDescriptor.Name, "serviceid": bundle.PackageDescriptor.Id, "modified": bundle.Modified, "size": bundle.Size})
	if err != nil {
		slog.Error("SetPackageBundle: ToJson failed", "file", Utility.FileLine(), "func", Utility.FunctionName(), "error", err.Error())
		return nil, status.Errorf(
			codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	q := `{"_id":"` + id + `"}`

	err = p.ReplaceOne(context.Background(), "local_resource", "local_resource", "Bundles", q, jsonStr, `[{"upsert": true}]`)
	if err != nil {
		slog.Error("SetPackageBundle: ReplaceOne failed", "file", Utility.FileLine(), "func", Utility.FunctionName(), "error", err.Error())
		return nil, err
	}
	return &resourcepb.SetPackageBundleResponse{Result: true}, nil
}

// SetPackageDescriptor sets or updates a package descriptor in the persistence store.
// It generates a unique ID for the descriptor and ensures the TypeName fields are set
// for the descriptor, its groups, and roles. The function supports multiple database
// types (MongoDB, ScyllaDB, SQL) and constructs the appropriate query for each.
// The descriptor is upserted into the "Packages" collection/table. If the operation
// fails or the descriptor is not created, an error is returned.
//
// Parameters:
//   ctx - The context for the request.
//   rqst - The request containing the PackageDescriptor to set.
//
// Returns:
//   *resourcepb.SetPackageDescriptorResponse - The response indicating success.
//   error - An error if the operation fails.
func (srv *server) SetPackageDescriptor(ctx context.Context, rqst *resourcepb.SetPackageDescriptorRequest) (*resourcepb.SetPackageDescriptorResponse, error) {

	p, err := srv.getPersistenceStore()
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	var q string
	if p.GetStoreType() == "MONGO" {
		q = `{"name":"` + rqst.PackageDescriptor.Name + `", "PublisherID":"` + rqst.PackageDescriptor.PublisherID + `", "version":"` + rqst.PackageDescriptor.Version + `"}`
	} else if p.GetStoreType() == "SCYLLA" {
		q = `SELECT * FROM Packages WHERE name='` + rqst.PackageDescriptor.Name + `' AND PublisherID='` + rqst.PackageDescriptor.PublisherID + `' AND version='` + rqst.PackageDescriptor.Version + `' ALLOW FILTERING`
	} else if p.GetStoreType() == "SQL" {
		q = `SELECT * FROM Packages WHERE name='` + rqst.PackageDescriptor.Name + `' AND PublisherID='` + rqst.PackageDescriptor.PublisherID + `' AND version='` + rqst.PackageDescriptor.Version + `'`
	} else {
		return nil, errors.New("unknown database type " + p.GetStoreType())
	}

	rqst.PackageDescriptor.TypeName = "PackageDescriptor"
	rqst.PackageDescriptor.Id = Utility.GenerateUUID(rqst.PackageDescriptor.PublisherID + "%" + rqst.PackageDescriptor.Name + "%" + rqst.PackageDescriptor.Version)

	for i := 0; i < len(rqst.PackageDescriptor.Groups); i++ {
		rqst.PackageDescriptor.Groups[i].TypeName = "Group"
	}

	for i := 0; i < len(rqst.PackageDescriptor.Roles); i++ {
		rqst.PackageDescriptor.Roles[i].TypeName = "Role"
	}

	jsonStr, err := json.Marshal(rqst.PackageDescriptor)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// little fix...
	jsonStr_ := strings.ReplaceAll(string(jsonStr), "PublisherID", "PublisherID")

	// Always create a new if not already exist.
	err = p.ReplaceOne(context.Background(), "local_resource", "local_resource", "Packages", q, jsonStr_, `[{"upsert": true}]`)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	count, err := p.Count(context.Background(), "local_resource", "local_resource", "Packages", q, "")
	if count == 0 || err != nil {
		return nil, status.Errorf(
			codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("unable to create the package descriptor")))

	}

	return &resourcepb.SetPackageDescriptorResponse{
		Result: true,
	}, nil
}

/** Set the host if it's part of the same local network. */
func (srv *server) setLocalHosts(peer *resourcepb.Peer) error {

	// Finaly I will set the domain in the hosts file...
	hosts, err := txeh.NewHostsDefault()
	address := peer.GetHostname()
	if peer.GetDomain() != "localhost" {
		address = address + "." + peer.GetDomain()
	}

	if err != nil {
		logger.Error("fail to set host entry", "address", address, "error", err)
		return err
	}

	if peer.ExternalIpAddress == Utility.MyIP() {
		hosts.AddHost(peer.LocalIpAddress, address)
	}

	err = hosts.Save()
	if err != nil {
		logger.Error("fail to save hosts", "ip", peer.LocalIpAddress, "address", address, "error", err)
		return err
	}

	return nil
}

/** Set the host if it's part of the same local network. */
func (srv *server) removeFromLocalHosts(peer *resourcepb.Peer) error {
	// Finaly I will set the domain in the hosts file...
	hosts, err := txeh.NewHostsDefault()
	if err != nil {
		return err
	}

	domain := peer.GetDomain()

	if peer.ExternalIpAddress == Utility.MyIP() {
		hosts.RemoveHost(domain)
	} else {
		return errors.New("the peer is not on the same local network")
	}

	err = hosts.Save()
	if err != nil {
		logger.Error("fail to save hosts file", "error", err)
	}

	return err
}
