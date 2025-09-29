// rbac_permissions.go: get/set/delete resource permissions.

package main

import (
	"context"
	"encoding/base64"
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
				exist, _ := srv.accountExist(denied[i].Accounts[j])
				if exist {
					// Do not index denied subjects under allowed index space
					has_denied = true
				}
			}
		}

		// Applications
		if denied[i].Applications != nil {
			for j := range denied[i].Applications {
				exist, _ := srv.applicationExist(denied[i].Applications[j])
				if exist {
					// Do not index denied subjects under allowed index space
					has_denied = true
				}
			}
		}

		// Peers
		if denied[i].Peers != nil {
			for j := range denied[i].Peers {
				if srv.peerExist(denied[i].Peers[j]) {
					// Do not index denied subjects under allowed index space
					has_denied = true
				}
			}
		}

		// Groups
		if denied[i].Groups != nil {
			for j := range denied[i].Groups {
				exist, _ := srv.groupExist(denied[i].Groups[j])
				if exist {
					// Do not index denied subjects under allowed index space
					has_denied = true
				}
			}
		}

		// Organizations
		if denied[i].Organizations != nil {
			for j := range denied[i].Organizations {
				exist, _ := srv.organizationExist(denied[i].Organizations[j])
				if exist {
					// Do not index denied subjects under allowed index space
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
	data, err := protojson.Marshal(permissions)
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
//
//	ctx - The context for the request, used for authentication and tracing.
//	rqst - The request containing the resource path, resource type, and permissions to set.
//
// Returns:
//
//	*rbacpb.SetResourcePermissionsRsp - The response indicating success.
//	error - An error if validation fails, the client is not authorized, or the operation fails.
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
	// NOTE: We no longer index denied subjects in the allowed index space,
	// so there is nothing to clean up here for denied entries.

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

	// Remove the path once
	if err := srv.removeItem(path); err != nil {
		logPrintln("fail to remove key ", path, ": ", err)
	}

	data, err := proto.Marshal(permissions)
	if err != nil {
		return err
	}

	encoded := []byte(base64.StdEncoding.EncodeToString(data))
	srv.publish("delete_resources_permissions_event", encoded)

	// already removed above
	return nil
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
	err = protojson.Unmarshal(data, permissions)
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
		if err := srv.setResourcePermissions(path, permissions.ResourceType, permissions); err != nil {
			logPrintln("cleanupPermissions resave failed for ", path, ": ", err)
		}
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
//
//	ctx - The context for the request, used for cancellation and deadlines.
//	rqst - The request containing the resource path whose permissions are to be deleted.
//
// Returns:
//
//	A response indicating the result of the delete operation, or an error if the operation fails.
func (srv *server) DeleteResourcePermissions(ctx context.Context, rqst *rbacpb.DeleteResourcePermissionsRqst) (*rbacpb.DeleteResourcePermissionsRsp, error) {

	permissions, err := srv.getResourcePermissions(rqst.Path)
	if err != nil {
		if strings.Contains(err.Error(), "item not found") || strings.Contains(err.Error(), "Key not found") {
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
// DeleteResourcePermission removes the whole action entry (e.g. "execute")
// from Allowed or Denied, then persists.
func (srv *server) DeleteResourcePermission(ctx context.Context, rqst *rbacpb.DeleteResourcePermissionRqst) (*rbacpb.DeleteResourcePermissionRsp, error) {
	path := rqst.Path
	action := rqst.Name
	kind := rqst.Type

	perms, err := srv.getResourcePermissions(path)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "permissions not found for path %q", path)
	}

	// 1) Collect subjects tied to the action we’re deleting (so we can drop their index entries)
	var rmAcc, rmGrp, rmOrg, rmApp, rmPeer []string
	switch kind {
	case rbacpb.PermissionType_ALLOWED:
		for _, p := range perms.Allowed {
			if p != nil && p.Name == action {
				rmAcc = append(rmAcc, p.Accounts...)
				rmGrp = append(rmGrp, p.Groups...)
				rmOrg = append(rmOrg, p.Organizations...)
				rmApp = append(rmApp, p.Applications...)
				rmPeer = append(rmPeer, p.Peers...)
			}
		}
	case rbacpb.PermissionType_DENIED:
		for _, p := range perms.Denied {
			if p != nil && p.Name == action {
				rmAcc = append(rmAcc, p.Accounts...)
				rmGrp = append(rmGrp, p.Groups...)
				rmOrg = append(rmOrg, p.Organizations...)
				rmApp = append(rmApp, p.Applications...)
				rmPeer = append(rmPeer, p.Peers...)
			}
		}
	}

	// 2) Drop index entries *before* we persist, so setResourcePermissions can re-add for any subjects
	//    that still have other actions or owner rights.
	for _, a := range rmAcc {
		if ok, id := srv.accountExist(a); ok {
			_ = srv.deleteSubjectResourcePermissions("PERMISSIONS/ACCOUNTS/"+id, path)
		}
	}
	for _, g := range rmGrp {
		if ok, id := srv.groupExist(g); ok {
			_ = srv.deleteSubjectResourcePermissions("PERMISSIONS/GROUPS/"+id, path)
		}
	}
	for _, o := range rmOrg {
		if ok, id := srv.organizationExist(o); ok {
			_ = srv.deleteSubjectResourcePermissions("PERMISSIONS/ORGANIZATIONS/"+id, path)
		}
	}
	for _, a := range rmApp {
		if ok, id := srv.applicationExist(a); ok {
			_ = srv.deleteSubjectResourcePermissions("PERMISSIONS/APPLICATIONS/"+id, path)
		}
	}
	for _, p := range rmPeer {
		_ = srv.deleteSubjectResourcePermissions("PERMISSIONS/PEERS/"+p, path)
	}

	// 3) Remove the action from the in-memory perms
	filter := func(in []*rbacpb.Permission) []*rbacpb.Permission {
		if len(in) == 0 {
			return in
		}
		out := in[:0]
		for _, p := range in {
			if p == nil || p.Name != action {
				out = append(out, p)
			}
		}
		if len(out) == 0 {
			return []*rbacpb.Permission{}
		}
		return out
	}

	switch kind {
	case rbacpb.PermissionType_ALLOWED:
		perms.Allowed = filter(perms.Allowed)
	case rbacpb.PermissionType_DENIED:
		perms.Denied = filter(perms.Denied)
	default:
		return nil, status.Errorf(codes.InvalidArgument, "unknown PermissionType: %v", kind)
	}

	// 4) Persist; this will (re)index whatever remains allowed/owner for this path
	if err := srv.setResourcePermissions(path, perms.ResourceType, perms); err != nil {
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// 5) Invalidate the per-path cache
	srv.cache.RemoveItem(path)

	return &rbacpb.DeleteResourcePermissionRsp{}, nil
}

// GetResourcePermission retrieves a specific permission for a resource based on the provided request.
// It searches for the permission by name in either the allowed or denied permissions list, depending on the request type.
// If the permission is found, it returns the corresponding permission in the response.
// If the permission is not found or an error occurs during retrieval, it returns an appropriate error.
//
// Parameters:
//
//	ctx - The context for the request.
//	rqst - The request containing the resource path, permission name, and permission type.
//
// Returns:
//
//	*rbacpb.GetResourcePermissionRsp - The response containing the found permission.
//	error - An error if the permission is not found or if there is an internal issue.
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
				return &rbacpb.GetResourcePermissionRsp{Permission: permissions.Allowed[i]}, nil
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

func upsertPermission(list []*rbacpb.Permission, p *rbacpb.Permission) []*rbacpb.Permission {
	for i := range list {
		if list[i].Name == p.Name {
			list[i] = p
			return list
		}
	}
	return append(list, p)
}

// SetResourcePermission sets a permission for a resource, either as allowed or denied, based on the request type.
// It retrieves the current permissions for the specified resource path, updates the allowed or denied permissions
// by replacing or adding the specified permission, and then saves the updated permissions.
// Returns an error if retrieving or setting permissions fails.
//
// Parameters:
//
//	ctx - The context for the request.
//	rqst - The request containing the resource path, permission type (allowed or denied), and the permission to set.
//
// Returns:
//
//	*rbacpb.SetResourcePermissionRsp - The response indicating success.
//	error - An error if the operation fails.
func (srv *server) SetResourcePermission(ctx context.Context, rqst *rbacpb.SetResourcePermissionRqst) (*rbacpb.SetResourcePermissionRsp, error) {
	// Try to fetch existing permissions
	permissions, err := srv.getResourcePermissions(rqst.Path)

	if rqst.Permission == nil {
		return nil, errors.New("no permission given")
	}

	if len(rqst.Path) == 0 {
		return nil, errors.New("no resource path given")
	}

	if len(rqst.ResourceType) == 0 {
		return nil, errors.New("no resource type given")
	}

	// If none exist yet, bootstrap a new record instead of failing
	if err != nil {
		errStr := ""
		if err != nil {
			errStr = err.Error()
		}
		if strings.Contains(errStr, "item not found") ||
			strings.Contains(errStr, "Key not found") ||
			strings.Contains(errStr, "not found") {
			permissions = &rbacpb.Permissions{
				// Default to "file" for single resources; adjust if your system uses something else
				ResourceType: rqst.ResourceType,
				Allowed:      []*rbacpb.Permission{},
				Denied:       []*rbacpb.Permission{},
				Owners:       nil,
				Path:         rqst.Path,
			}
		} else {
			return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
	}

	// Ensure slices are non-nil
	if permissions.Allowed == nil {
		permissions.Allowed = make([]*rbacpb.Permission, 0, 1)
	}
	if permissions.Denied == nil {
		permissions.Denied = make([]*rbacpb.Permission, 0, 1)
	}

	// Upsert the requested permission into the right bucket
	switch rqst.Type {
	case rbacpb.PermissionType_ALLOWED:
		permissions.Allowed = upsertPermission(permissions.Allowed, rqst.Permission)
	case rbacpb.PermissionType_DENIED:
		permissions.Denied = upsertPermission(permissions.Denied, rqst.Permission)
	default:
		return nil, status.Errorf(codes.InvalidArgument, "unknown PermissionType: %v", rqst.Type)
	}

	// Persist the updated (or bootstrapped) permission set
	if err := srv.setResourcePermissions(rqst.Path, permissions.ResourceType, permissions); err != nil {
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
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
