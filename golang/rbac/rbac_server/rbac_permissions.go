// rbac_permissions.go: get/set/delete resource permissions.

package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"strings"

	"github.com/globulario/services/golang/rbac/rbacpb"
	"github.com/globulario/services/golang/security"
	Utility "github.com/globulario/utility"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

func (srv *server) setResourcePermissions(path, resource_type string, permissions *rbacpb.Permissions) error {

	// remove it from the cache...
	srv.cache.RemoveItem(path)

	// be sure the path and the resource type are set in the permissions itself.
	permissions.Path = path
	permissions.ResourceType = resource_type

	// Each permissions object has a share objet associated with it so I will create it...
	share := new(rbacpb.Share)
	share.Path = path
	share.Domain = srv.Domain // The domain of the permissions manager...

	// set aggregations...
	share.Accounts = make([]string, 0)
	share.Applications = make([]string, 0)
	share.Groups = make([]string, 0)
	share.Organizations = make([]string, 0)
	share.Peers = make([]string, 0)

	// Allowed resources
	allowed := permissions.Allowed
	has_allowed := false

	for i := range allowed {
		// Accounts
		if allowed[i].Accounts != nil {
			for j := 0; j < len(allowed[i].Accounts); j++ {
				exist, a := srv.accountExist(allowed[i].Accounts[j])
				if exist {

					err := srv.setSubjectResourcePermissions("PERMISSIONS/ACCOUNTS/"+a, path)
					if err != nil {
						return err
					}
					share.Accounts = append(share.Accounts, a)
					has_allowed = true
				}
			}
		}

		// Groups
		if allowed[i].Groups != nil {
			for j := range allowed[i].Groups {

				exist, g := srv.groupExist(allowed[i].Groups[j])
				if exist {

					err := srv.setSubjectResourcePermissions("PERMISSIONS/GROUPS/"+g, path)
					if err != nil {
						return err
					}
					share.Groups = append(share.Groups, g)
					has_allowed = true
				}
			}
		}

		// Organizations
		if allowed[i].Organizations != nil {
			for j := range allowed[i].Organizations {
				exist, o := srv.organizationExist(allowed[i].Organizations[j])
				if exist {

					err := srv.setSubjectResourcePermissions("PERMISSIONS/ORGANIZATIONS/"+o, path)
					if err != nil {
						return err
					}
					share.Organizations = append(share.Organizations, o)
					has_allowed = true
				}
			}
		}

		// Applications
		if allowed[i].Applications != nil {
			for j := range allowed[i].Applications {
				exist, a := srv.applicationExist(allowed[i].Applications[j])
				if exist {

					err := srv.setSubjectResourcePermissions("PERMISSIONS/APPLICATIONS/"+a, path)
					if err != nil {
						return err
					}
					share.Applications = append(share.Applications, a)
					has_allowed = true
				}
			}
		}

		// Peers
		if allowed[i].Peers != nil {
			for j := range allowed[i].Peers {
				if srv.peerExist(allowed[i].Peers[j]) {

					err := srv.setSubjectResourcePermissions("PERMISSIONS/PEERS/"+allowed[i].Peers[j], path)
					if err != nil {
						return err
					}
					has_allowed = true
					share.Peers = append(share.Peers, allowed[i].Peers[j])
				}
			}
		}
	}

	// remove the allowed resources if no allowed resources are set...
	if !has_allowed {
		permissions.Allowed = nil
	}

	// Denied resources
	denied := permissions.Denied
	has_denied := false

	for i := range denied {
		// Acccounts
		if denied[i].Accounts != nil {
			for j := range denied[i].Accounts {
				exist, a := srv.accountExist(denied[i].Accounts[j])
				if exist {

					err := srv.setSubjectResourcePermissions("PERMISSIONS/ACCOUNTS/"+a, path)
					if err != nil {
						return err
					}
					has_denied = true
				}
			}
		}

		// Applications
		if denied[i].Applications != nil {
			for j := range denied[i].Applications {
				exist, a := srv.applicationExist(denied[i].Applications[j])
				if exist {

					err := srv.setSubjectResourcePermissions("PERMISSIONS/APPLICATIONS/"+a, path)
					if err != nil {
						return err
					}
					has_denied = true
				}
			}
		}

		// Peers
		if denied[i].Peers != nil {
			for j := range denied[i].Peers {
				if srv.peerExist(denied[i].Peers[j]) {

					err := srv.setSubjectResourcePermissions("PERMISSIONS/PEERS/"+denied[i].Peers[j], path)
					if err != nil {
						return err
					}
					has_denied = true
				}
			}
		}

		// Groups
		if denied[i].Groups != nil {
			for j := range denied[i].Groups {
				exist, g := srv.groupExist(denied[i].Groups[j])
				if exist {

					err := srv.setSubjectResourcePermissions("PERMISSIONS/GROUPS/"+g, path)
					if err != nil {
						return err
					}
					has_denied = true
				}
			}
		}

		// Organizations
		if denied[i].Organizations != nil {
			for j := range denied[i].Organizations {
				exist, o := srv.organizationExist(denied[i].Organizations[j])
				if exist {

					err := srv.setSubjectResourcePermissions("PERMISSIONS/ORGANIZATIONS/"+o, path)
					if err != nil {
						return err
					}
					has_denied = true
				}
			}
		}

	}

	// remove the denied resources if no denied resources are set...
	if !has_denied {
		permissions.Denied = nil
	}

	// Owned resources
	owners := permissions.Owners
	has_owners := false
	if owners != nil {
		// Acccounts
		if owners.Accounts != nil {
			for j := range owners.Accounts {
				exist, a := srv.accountExist(owners.Accounts[j])
				if exist {

					err := srv.setSubjectResourcePermissions("PERMISSIONS/ACCOUNTS/"+a, path)
					if err != nil {
						return err
					}
					share.Accounts = append(share.Accounts, a)
					has_owners = true
					// Here I will set the used space.
					if permissions.ResourceType == "file" {
						used_space, err := srv.getSubjectUsedSpace(owners.Accounts[j], rbacpb.SubjectType_ACCOUNT)
						if err != nil {
							used_space, err = srv.initSubjectUsedSpace(owners.Accounts[j], rbacpb.SubjectType_ACCOUNT)
							if err != nil {
								return err
							}
						}

						fi, err := os.Stat(srv.formatPath(path))
						if err == nil {
							if !fi.IsDir() {
								used_space += uint64(fi.Size())
								srv.setSubjectUsedSpace(owners.Accounts[j], rbacpb.SubjectType_ACCOUNT, used_space)
							}
						}
					}

				} else {
					logPrintln("no account found with id ", owners.Accounts[j])
				}
			}
		}

		// Applications
		if owners.Applications != nil {
			for j := range owners.Applications {
				exist, a := srv.applicationExist(owners.Applications[j])
				if exist {

					err := srv.setSubjectResourcePermissions("PERMISSIONS/APPLICATIONS/"+a, path)
					if err != nil {
						return err
					}
					share.Applications = append(share.Applications, a)
					has_owners = true
					if permissions.ResourceType == "file" {

						used_space, err := srv.getSubjectUsedSpace(owners.Applications[j], rbacpb.SubjectType_APPLICATION)
						if err != nil {
							used_space, err = srv.initSubjectUsedSpace(owners.Applications[j], rbacpb.SubjectType_APPLICATION)
							if err != nil {
								return err
							}
						}

						fi, err := os.Stat(srv.formatPath(path))
						if err == nil {
							if !fi.IsDir() {
								used_space += uint64(fi.Size())
								srv.setSubjectUsedSpace(owners.Applications[j], rbacpb.SubjectType_APPLICATION, used_space)
							}
						}
					}
				}
			}

		}

		// Peers
		if owners.Peers != nil {
			for j := range owners.Peers {
				if srv.peerExist(owners.Peers[j]) {

					err := srv.setSubjectResourcePermissions("PERMISSIONS/PEERS/"+owners.Peers[j], path)
					if err != nil {
						return err
					}
					share.Peers = append(share.Peers, owners.Peers[j])
					has_owners = true
					if permissions.ResourceType == "file" {
						used_space, err := srv.getSubjectUsedSpace(owners.Peers[j], rbacpb.SubjectType_PEER)
						if err != nil {
							used_space, err = srv.initSubjectUsedSpace(owners.Peers[j], rbacpb.SubjectType_PEER)
							if err != nil {
								return err
							}
						}

						fi, err := os.Stat(srv.formatPath(path))
						if err == nil {
							if !fi.IsDir() {
								used_space += uint64(fi.Size())
								srv.setSubjectUsedSpace(owners.Peers[j], rbacpb.SubjectType_PEER, used_space)
							}
						}
					}
				}
			}
		}

		// Groups
		if owners.Groups != nil {
			for j := range owners.Groups {
				exist, g := srv.groupExist(owners.Groups[j])
				if exist {

					err := srv.setSubjectResourcePermissions("PERMISSIONS/GROUPS/"+g, path)
					if err != nil {
						return err
					}
					share.Groups = append(share.Groups, g)
					has_owners = true
					if permissions.ResourceType == "file" {
						used_space, err := srv.getSubjectUsedSpace(owners.Groups[j], rbacpb.SubjectType_GROUP)
						if err != nil {
							used_space, err = srv.initSubjectUsedSpace(owners.Groups[j], rbacpb.SubjectType_GROUP)
							if err != nil {
								return err
							}
						}

						fi, err := os.Stat(srv.formatPath(path))
						if err == nil {
							if !fi.IsDir() {
								used_space += uint64(fi.Size())
								srv.setSubjectUsedSpace(owners.Groups[j], rbacpb.SubjectType_GROUP, used_space)
							}
						}
					}
				} else {
					logPrintln("no group found with id ", owners.Groups[j])
				}
			}
		}

		// Organizations
		if owners.Organizations != nil {
			for j := range owners.Organizations {
				exist, o := srv.organizationExist(owners.Organizations[j])
				if exist {

					err := srv.setSubjectResourcePermissions("PERMISSIONS/ORGANIZATIONS/"+o, path)
					if err != nil {
						return err
					}
					share.Organizations = append(share.Organizations, o)
					has_owners = true
					if permissions.ResourceType == "file" {
						used_space, err := srv.getSubjectUsedSpace(owners.Organizations[j], rbacpb.SubjectType_ORGANIZATION)
						if err != nil {
							used_space, err = srv.initSubjectUsedSpace(owners.Organizations[j], rbacpb.SubjectType_ORGANIZATION)
							if err != nil {
								return err
							}
						}

						fi, err := os.Stat(srv.formatPath(path))
						if err == nil {
							if !fi.IsDir() {
								used_space += uint64(fi.Size())
								srv.setSubjectUsedSpace(owners.Organizations[j], rbacpb.SubjectType_ORGANIZATION, used_space)
							}
						}
					}
				} else {

					logPrintln("no organization found with id ", owners.Organizations[j])
				}
			}
		}
	}

	// remove the owners resources if no owners resources are set...
	if !has_owners {
		permissions.Owners = nil
	}

	// simply marshal the permission and put it into the store.
	data, err := json.Marshal(permissions)
	if err != nil {
		return err
	}

	err = srv.setItem(path, data)
	if err != nil {
		return err
	}

	if permissions.ResourceType == "file" {
		err = srv.shareResource(share)
		if err != nil {
			return err
		}
	}

	err = srv.setResourceTypePathIndexation(resource_type, path)
	if err != nil {
		return err
	}

	// That's the way to marshal object as evt data
	data_, err := proto.Marshal(permissions)
	if err != nil {
		return err
	}

	encoded := []byte(base64.StdEncoding.EncodeToString(data_))
	srv.publish("set_resources_permissions_event", encoded)

	return nil
}


// SetResourcePermissions sets the permissions for a specified resource.
// It validates the request parameters, checks if the client is authorized to set permissions,
// and applies the permissions to the resource. Only the owner of the resource or a service account
// (client ID starting with "sa@") is allowed to set permissions.
//
// Parameters:
//   ctx - The context for the request, used for authentication and tracing.
//   rqst - The request containing the resource path, resource type, and permissions to set.
//
// Returns:
//   *rbacpb.SetResourcePermissionsRsp - The response indicating success.
//   error - An error if validation fails, the client is not authorized, or the operation fails.
func (srv *server) SetResourcePermissions(ctx context.Context, rqst *rbacpb.SetResourcePermissionsRqst) (*rbacpb.SetResourcePermissionsRsp, error) {

	if len(rqst.Path) == 0 {
		return nil, errors.New("no resource path given")
	}

	if len(rqst.ResourceType) == 0 {
		return nil, errors.New("no resource type given")
	}

	if rqst.Permissions == nil {
		return nil, errors.New("no permissions given")
	}

	// Here I will add additional validation...
	clientId, _, err := security.GetClientId(ctx)
	if err != nil {
		return nil, err
	}

	if !strings.HasPrefix(clientId, "sa@") {
		if !srv.isOwner(clientId, rbacpb.SubjectType_ACCOUNT, rqst.Path) {
			return nil, errors.New(clientId + " must be owner of " + rqst.Path + " to set permission")
		}
	}

	// Now I will validate the access...
	err = srv.setResourcePermissions(rqst.Path, rqst.ResourceType, rqst.Permissions)
	if err != nil {
		logPrintln("fail to set resource permission with error ", err)
		return nil, status.Errorf(
			codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &rbacpb.SetResourcePermissionsRsp{}, nil
}

func (srv *server) deleteResourcePermissions(path string, permissions *rbacpb.Permissions) error {

	// simply remove it from the cache...
	defer srv.cache.RemoveItem(path)

	// Allowed resources
	allowed := permissions.Allowed
	for i := range allowed {

		// Accounts
		for j := 0; j < len(allowed[i].Accounts); j++ {
			exist, a := srv.accountExist(allowed[i].Accounts[j])
			if exist {
				err := srv.deleteSubjectResourcePermissions("PERMISSIONS/ACCOUNTS/"+a, path)
				if err != nil {
					logPrintln(err)
				}
			}
		}

		// Groups
		for j := 0; j < len(allowed[i].Groups); j++ {
			exist, g := srv.groupExist(allowed[i].Groups[j])
			if exist {
				err := srv.deleteSubjectResourcePermissions("PERMISSIONS/GROUPS/"+g, path)
				if err != nil {
					logPrintln(err)
				}
			}
		}

		// Organizations
		for j := 0; j < len(allowed[i].Organizations); j++ {
			exist, o := srv.organizationExist(allowed[i].Organizations[j])
			if exist {
				err := srv.deleteSubjectResourcePermissions("PERMISSIONS/ORGANIZATIONS/"+o, path)
				if err != nil {
					logPrintln(err)
				}
			}
		}

		// Applications
		for j := 0; j < len(allowed[i].Applications); j++ {
			exist, a := srv.applicationExist(allowed[i].Applications[j])
			if exist {
				err := srv.deleteSubjectResourcePermissions("PERMISSIONS/APPLICATIONS/"+a, path)
				if err != nil {
					logPrintln(err)
				}
			}
		}

		// Peers
		for j := 0; j < len(allowed[i].Peers); j++ {
			err := srv.deleteSubjectResourcePermissions("PERMISSIONS/PEERS/"+allowed[i].Peers[j], path)
			if err != nil {
				logPrintln(err)
			}
		}
	}

	// Denied resources
	denied := permissions.Denied

	for i := range denied {
		// Acccounts
		for j := range denied[i].Accounts {
			exist, a := srv.accountExist(denied[i].Accounts[j])
			if exist {
				err := srv.deleteSubjectResourcePermissions("PERMISSIONS/ACCOUNTS/"+a, path)
				if err != nil {
					logPrintln(err)
				}
			}
		}
		// Applications
		for j := range denied[i].Applications {
			exist, a := srv.applicationExist(denied[i].Applications[j])
			if exist {
				err := srv.deleteSubjectResourcePermissions("PERMISSIONS/APPLICATIONS/"+a, path)
				if err != nil {
					logPrintln(err)
				}
			}
		}

		// Peers
		for j := range denied[i].Peers {
			err := srv.deleteSubjectResourcePermissions("PERMISSIONS/PEERS/"+denied[i].Peers[j], path)
			if err != nil {
				logPrintln(err)
			}
		}

		// Groups
		for j := range denied[i].Groups {
			exist, g := srv.groupExist(denied[i].Groups[j])
			if exist {
				err := srv.deleteSubjectResourcePermissions("PERMISSIONS/GROUPS/"+g, path)
				if err != nil {
					logPrintln(err)
				}
			}
		}

		// Organizations
		for j := range denied[i].Organizations {
			exist, o := srv.organizationExist(denied[i].Organizations[j])
			if exist {
				err := srv.deleteSubjectResourcePermissions("PERMISSIONS/ORGANIZATIONS/"+o, path)
				if err != nil {
					logPrintln(err)
				}
			}
		}
	}

	// Owned resources
	owners := permissions.Owners

	if owners != nil {
		// Acccounts
		if owners.Accounts != nil {

			for j := range owners.Accounts {
				exist, a := srv.accountExist(owners.Accounts[j])
				if exist {
					err := srv.deleteSubjectResourcePermissions("PERMISSIONS/ACCOUNTS/"+a, path)
					if err != nil {
						logPrintln(err)
					}

					if permissions.ResourceType == "file" {
						used_space, err := srv.getSubjectUsedSpace(owners.Accounts[j], rbacpb.SubjectType_ACCOUNT)
						if err != nil {
							used_space, err = srv.initSubjectUsedSpace(owners.Accounts[j], rbacpb.SubjectType_ACCOUNT)
							if err != nil {
								return err
							}
						}

						fi, err := os.Stat(srv.formatPath(path))
						if err == nil {
							if !fi.IsDir() {
								used_space -= uint64(fi.Size())
								srv.setSubjectUsedSpace(owners.Accounts[j], rbacpb.SubjectType_ACCOUNT, used_space)
							}
						} else {
							logPrintln("no path found ", path, err)
						}
					}
				}
			}
		}

		// Applications
		if owners.Applications != nil {
			for j := 0; j < len(owners.Applications); j++ {
				exist, a := srv.applicationExist(owners.Applications[j])
				if exist {
					err := srv.deleteSubjectResourcePermissions("PERMISSIONS/APPLICATIONS/"+a, path)
					if err != nil {
						logPrintln(err)
					}
					if permissions.ResourceType == "file" {
						used_space, err := srv.getSubjectUsedSpace(owners.Applications[j], rbacpb.SubjectType_APPLICATION)
						if err != nil {
							used_space, err = srv.initSubjectUsedSpace(owners.Applications[j], rbacpb.SubjectType_APPLICATION)
							if err != nil {
								return err
							}
						}

						fi, err := os.Stat(srv.formatPath(path))
						if err == nil {
							if !fi.IsDir() {
								used_space -= uint64(fi.Size())
								srv.setSubjectUsedSpace(owners.Applications[j], rbacpb.SubjectType_APPLICATION, used_space)
							}
						}
					}
				}
			}
		}

		// Peers
		if owners.Peers != nil {
			for j := 0; j < len(owners.Peers); j++ {
				err := srv.deleteSubjectResourcePermissions("PERMISSIONS/PEERS/"+owners.Peers[j], path)
				if err != nil {
					logPrintln(err)
				}
				if permissions.ResourceType == "file" {
					used_space, err := srv.getSubjectUsedSpace(owners.Peers[j], rbacpb.SubjectType_PEER)
					if err != nil {
						used_space, err = srv.initSubjectUsedSpace(owners.Peers[j], rbacpb.SubjectType_PEER)
						if err != nil {
							return err
						}
					}

					fi, err := os.Stat(srv.formatPath(path))
					if err == nil {
						if !fi.IsDir() {
							used_space -= uint64(fi.Size())
							srv.setSubjectUsedSpace(owners.Peers[j], rbacpb.SubjectType_PEER, used_space)
						}
					}
				}
			}
		}

		// Groups
		if owners.Groups != nil {
			for j := 0; j < len(owners.Groups); j++ {
				exist, g := srv.groupExist(owners.Groups[j])
				if exist {
					err := srv.deleteSubjectResourcePermissions("PERMISSIONS/GROUPS/"+g, path)
					if err != nil {
						logPrintln(err)
					}

					if permissions.ResourceType == "file" {
						used_space, err := srv.getSubjectUsedSpace(owners.Groups[j], rbacpb.SubjectType_GROUP)
						if err != nil {
							used_space, err = srv.initSubjectUsedSpace(owners.Groups[j], rbacpb.SubjectType_GROUP)
							if err != nil {
								return err
							}
						}

						fi, err := os.Stat(srv.formatPath(path))
						if err == nil {
							if !fi.IsDir() {
								used_space -= uint64(fi.Size())
								srv.setSubjectUsedSpace(owners.Groups[j], rbacpb.SubjectType_GROUP, used_space)
							}
						}
					}

				}
			}
		}

		// Organizations
		if owners.Organizations != nil {
			for j := 0; j < len(owners.Organizations); j++ {
				exist, o := srv.organizationExist(owners.Organizations[j])
				if exist {
					err := srv.deleteSubjectResourcePermissions("PERMISSIONS/ORGANIZATIONS/"+o, path)
					if err != nil {
						logPrintln(err)
					}
					if permissions.ResourceType == "file" {
						used_space, err := srv.getSubjectUsedSpace(owners.Organizations[j], rbacpb.SubjectType_ORGANIZATION)
						if err != nil {
							used_space, err = srv.initSubjectUsedSpace(owners.Organizations[j], rbacpb.SubjectType_ORGANIZATION)
							if err != nil {
								return err
							}
						}

						fi, err := os.Stat(srv.formatPath(path))
						if err == nil {
							if !fi.IsDir() {
								used_space -= uint64(fi.Size())
								srv.setSubjectUsedSpace(owners.Organizations[j], rbacpb.SubjectType_ORGANIZATION, used_space)
							}
						}
					}
				}
			}
		}
	}

	// Remove the resource type permission
	srv.deleteResourceTypePathIndexation(permissions.ResourceType, path)

	// unshare the resource
	if permissions.ResourceType == "file" {
		srv.unshareResource(srv.Domain, path)
	}

	// Remove the path
	err := srv.removeItem(path)
	if err != nil {
		logPrintln("fail to remove key ", path)
	}

	data, err := proto.Marshal(permissions)
	if err != nil {
		return err
	}

	encoded := []byte(base64.StdEncoding.EncodeToString(data))
	srv.publish("delete_resources_permissions_event", encoded)

	return srv.removeItem(path)
}

func (srv *server) getResourcePermissions(path string) (*rbacpb.Permissions, error) {

	chached, err := srv.cache.GetItem(path)
	if err == nil && chached != nil {
		permissions := new(rbacpb.Permissions)
		err := protojson.Unmarshal(chached, permissions)
		if err == nil {
			return permissions, nil
		}
	}

	data, err := srv.getItem(path)
	if err != nil {
		return nil, err
	}

	permissions := new(rbacpb.Permissions)
	err = json.Unmarshal(data, &permissions)
	if err != nil {
		return nil, err
	}

	// remove deleted subjects
	needSave, permissions, err := srv.cleanupPermissions(permissions)
	if err != nil {
		return nil, err
	}

	// save the value...
	if needSave {
		srv.setResourcePermissions(path, permissions.ResourceType, permissions)
	}

	jsonStr, err := protojson.Marshal(permissions)
	if err == nil {
		srv.cache.SetItem(path, []byte(jsonStr))
	}

	return permissions, nil
}


// DeleteResourcePermissions deletes all permissions associated with a specified resource path.
// It first retrieves the current permissions for the resource. If the resource is not found,
// it returns an empty response without error. If any other error occurs during retrieval or
// deletion of permissions, it returns an internal error with detailed information.
// Parameters:
//   ctx - The context for the request, used for cancellation and deadlines.
//   rqst - The request containing the resource path whose permissions are to be deleted.
// Returns:
//   A response indicating the result of the delete operation, or an error if the operation fails.
func (srv *server) DeleteResourcePermissions(ctx context.Context, rqst *rbacpb.DeleteResourcePermissionsRqst) (*rbacpb.DeleteResourcePermissionsRsp, error) {

	permissions, err := srv.getResourcePermissions(rqst.Path)
	if err != nil {
		if strings.Contains(err.Error(), "item not found") {
			return &rbacpb.DeleteResourcePermissionsRsp{}, nil
		}
		return nil, status.Errorf(
			codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	err = srv.deleteResourcePermissions(rqst.Path, permissions)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &rbacpb.DeleteResourcePermissionsRsp{}, nil
}

// DeleteResourcePermission removes a specific permission from a resource's allowed or denied permissions list.
// It takes a context and a DeleteResourcePermissionRqst containing the resource path, permission name, and type (allowed or denied).
// The function retrieves the current permissions for the resource, removes the specified permission from the appropriate list,
// updates the resource permissions, and returns a DeleteResourcePermissionRsp on success or an error if any operation fails.
func (srv *server) DeleteResourcePermission(ctx context.Context, rqst *rbacpb.DeleteResourcePermissionRqst) (*rbacpb.DeleteResourcePermissionRsp, error) {

	permissions, err := srv.getResourcePermissions(rqst.Path)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	switch rqst.Type {
	case rbacpb.PermissionType_ALLOWED:
		// Remove the permission from the allowed permission
		allowed := make([]*rbacpb.Permission, 0)
		for i := range permissions.Allowed {
			if permissions.Allowed[i].Name != rqst.Name {
				allowed = append(allowed, permissions.Allowed[i])
			}
		}
		permissions.Allowed = allowed
	case rbacpb.PermissionType_DENIED:
		// Remove the permission from the allowed permission.
		denied := make([]*rbacpb.Permission, 0)
		for i := range permissions.Denied {
			if permissions.Denied[i].Name != rqst.Name {
				denied = append(denied, permissions.Denied[i])
			}
		}
		permissions.Denied = denied
	}
	err = srv.setResourcePermissions(rqst.Path, permissions.ResourceType, permissions)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &rbacpb.DeleteResourcePermissionRsp{}, nil
}


// GetResourcePermission retrieves a specific permission for a resource based on the provided request.
// It searches for the permission by name in either the allowed or denied permissions list, depending on the request type.
// If the permission is found, it returns the corresponding permission in the response.
// If the permission is not found or an error occurs during retrieval, it returns an appropriate error.
//
// Parameters:
//   ctx - The context for the request.
//   rqst - The request containing the resource path, permission name, and permission type.
//
// Returns:
//   *rbacpb.GetResourcePermissionRsp - The response containing the found permission.
//   error - An error if the permission is not found or if there is an internal issue.
func (srv *server) GetResourcePermission(ctx context.Context, rqst *rbacpb.GetResourcePermissionRqst) (*rbacpb.GetResourcePermissionRsp, error) {

	permissions, err := srv.getResourcePermissions(rqst.Path)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// Search on allowed permission
	switch rqst.Type {
	case rbacpb.PermissionType_ALLOWED:
		for i := range permissions.Allowed {
			if permissions.Allowed[i].Name == rqst.Name {
				return &rbacpb.GetResourcePermissionRsp{Permission: permissions.Denied[i]}, nil
			}
		}
	case rbacpb.PermissionType_DENIED: // search in denied permissions.

		for i := range permissions.Denied {
			if permissions.Denied[i].Name == rqst.Name {
				return &rbacpb.GetResourcePermissionRsp{Permission: permissions.Denied[i]}, nil
			}
		}
	}

	return nil, status.Errorf(
		codes.Internal,
		"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("No permission found with name "+rqst.Name)))
}

// SetResourcePermission sets a permission for a resource, either as allowed or denied, based on the request type.
// It retrieves the current permissions for the specified resource path, updates the allowed or denied permissions
// by replacing or adding the specified permission, and then saves the updated permissions.
// Returns an error if retrieving or setting permissions fails.
//
// Parameters:
//   ctx - The context for the request.
//   rqst - The request containing the resource path, permission type (allowed or denied), and the permission to set.
//
// Returns:
//   *rbacpb.SetResourcePermissionRsp - The response indicating success.
//   error - An error if the operation fails.
func (srv *server) SetResourcePermission(ctx context.Context, rqst *rbacpb.SetResourcePermissionRqst) (*rbacpb.SetResourcePermissionRsp, error) {

	permissions, err := srv.getResourcePermissions(rqst.Path)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// Remove the permission from the allowed permission
	switch rqst.Type {
	case rbacpb.PermissionType_ALLOWED:
		allowed := make([]*rbacpb.Permission, 0)
		for i := range permissions.Allowed {
			if permissions.Allowed[i].Name == rqst.Permission.Name {
				allowed = append(allowed, permissions.Allowed[i])
			} else {
				allowed = append(allowed, rqst.Permission)
			}
		}
		permissions.Allowed = allowed
	case rbacpb.PermissionType_DENIED:

		// Remove the permission from the allowed permission.
		denied := make([]*rbacpb.Permission, 0)
		for i := range permissions.Denied {
			if permissions.Denied[i].Name == rqst.Permission.Name {
				denied = append(denied, permissions.Denied[i])
			} else {
				denied = append(denied, rqst.Permission)
			}
		}
		permissions.Denied = denied
	}
	err = srv.setResourcePermissions(rqst.Path, permissions.ResourceType, permissions)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &rbacpb.SetResourcePermissionRsp{}, nil
}

// GetResourcePermissions retrieves the permissions associated with a specific resource path.
// It takes a context and a GetResourcePermissionsRqst containing the resource path.
// Returns a GetResourcePermissionsRsp with the permissions or an error if retrieval fails.
func (srv *server) GetResourcePermissions(ctx context.Context, rqst *rbacpb.GetResourcePermissionsRqst) (*rbacpb.GetResourcePermissionsRsp, error) {

	permissions, err := srv.getResourcePermissions(rqst.Path)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &rbacpb.GetResourcePermissionsRsp{Permissions: permissions}, nil
}
