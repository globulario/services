package main

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"github.com/davecourtois/Utility"
	"github.com/globulario/services/golang/rbac/rbacpb"
	"github.com/golang/protobuf/jsonpb"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (rbac_server *server) setSubjectResourcePermissions(subject string, path string) error {

	// Here I will retreive the actual list of paths use by this user.
	data, err := rbac_server.permissions.GetItem(subject)
	paths_ := make([]interface{}, 0)

	if err == nil {
		err := json.Unmarshal(data, &paths_)
		if err != nil {
			return err
		}
	}

	paths := make([]string, len(paths_))
	for i := 0; i < len(paths_); i++ {
		paths[i] = paths_[i].(string)
	}

	if !Utility.Contains(paths, path) {
		paths = append(paths, path)
	} else {
		return nil // nothing todo here...
	}

	// simply marshal the permission and put it into the store.
	data, err = json.Marshal(paths)
	if err != nil {
		return err
	}
	return rbac_server.permissions.SetItem(subject, data)
}

// Save the ressource permission
func (rbac_server *server) setResourcePermissions(path string, permissions *rbacpb.Permissions) error {

	// First of all I need to remove the existing permission.
	rbac_server.deleteResourcePermissions(path, permissions)

	// Each permissions object has a share objet associated with it so I will create it...
	share := new(rbacpb.Share)
	share.Path = path
	share.Domain = rbac_server.Domain // The domain of the permissions manager...

	// set aggregations...
	share.Accounts = make([]string, 0)
	share.Applications = make([]string, 0)
	share.Groups = make([]string, 0)
	share.Organizations = make([]string, 0)
	share.Peers = make([]string, 0)

	// Allowed resources
	allowed := permissions.Allowed
	if allowed != nil {

		for i := 0; i < len(allowed); i++ {

			// Accounts
			if allowed[i].Accounts != nil {
				for j := 0; j < len(allowed[i].Accounts); j++ {
					err := rbac_server.setSubjectResourcePermissions("PERMISSIONS/ACCOUNTS/" + allowed[i].Accounts[j], path)
					if err != nil {
						return err
					}
					share.Accounts = append(share.Accounts, allowed[i].Accounts[j])
				}
			}

			// Groups
			if allowed[i].Groups != nil {
				for j := 0; j < len(allowed[i].Groups); j++ {
					err := rbac_server.setSubjectResourcePermissions("PERMISSIONS/GROUPS/" + allowed[i].Groups[j], path)
					if err != nil {
						return err
					}
					share.Groups = append(share.Groups, allowed[i].Groups[j])
				}
			}

			// Organizations
			if allowed[i].Organizations != nil {
				for j := 0; j < len(allowed[i].Organizations); j++ {
					err := rbac_server.setSubjectResourcePermissions("PERMISSIONS/ORGANIZATIONS/" + allowed[i].Organizations[j], path)
					if err != nil {
						return err
					}
					share.Organizations = append(share.Organizations, allowed[i].Organizations[j])
				}
			}

			// Applications
			if allowed[i].Applications != nil {
				for j := 0; j < len(allowed[i].Applications); j++ {
					err := rbac_server.setSubjectResourcePermissions("PERMISSIONS/APPLICATIONS/" + allowed[i].Applications[j], path)
					if err != nil {
						return err
					}
					share.Applications = append(share.Applications, allowed[i].Applications[j])
				}
			}

			// Peers
			if allowed[i].Peers != nil {
				for j := 0; j < len(allowed[i].Peers); j++ {
					err := rbac_server.setSubjectResourcePermissions("PERMISSIONS/PEERS/" + allowed[i].Peers[j], path)
					if err != nil {
						return err
					}
					share.Peers = append(share.Peers, allowed[i].Peers[j])
				}
			}
		}
	}

	// Denied resources
	denied := permissions.Denied
	if denied != nil {
		for i := 0; i < len(denied); i++ {
			// Acccounts
			if denied[i].Accounts != nil {
				for j := 0; j < len(denied[i].Accounts); j++ {
					err := rbac_server.setSubjectResourcePermissions("PERMISSIONS/ACCOUNTS/" + denied[i].Accounts[j], path)
					if err != nil {
						return err
					}
				}
			}

			// Applications
			if denied[i].Applications != nil {
				for j := 0; j < len(denied[i].Applications); j++ {
					err := rbac_server.setSubjectResourcePermissions("PERMISSIONS/APPLICATIONS/" + denied[i].Applications[j], path)
					if err != nil {
						return err
					}
				}
			}

			// Peers
			if denied[i].Peers != nil {
				for j := 0; j < len(denied[i].Peers); j++ {
					err := rbac_server.setSubjectResourcePermissions("PERMISSIONS/PEERS/" + denied[i].Peers[j], path)
					if err != nil {
						return err
					}
				}
			}

			// Groups
			if denied[i].Groups != nil {
				for j := 0; j < len(denied[i].Groups); j++ {
					err := rbac_server.setSubjectResourcePermissions("PERMISSIONS/GROUPS/" + denied[i].Groups[j], path)
					if err != nil {
						return err
					}
				}
			}

			// Organizations
			if denied[i].Organizations != nil {
				for j := 0; j < len(denied[i].Organizations); j++ {
					err := rbac_server.setSubjectResourcePermissions("PERMISSIONS/ORGANIZATIONS/" + denied[i].Organizations[j], path)
					if err != nil {
						return err
					}
				}
			}

		}
	}

	// Owned resources
	owners := permissions.Owners
	if owners != nil {
		// Acccounts
		if owners.Accounts != nil {
			for j := 0; j < len(owners.Accounts); j++ {
				err := rbac_server.setSubjectResourcePermissions("PERMISSIONS/ACCOUNTS/" + owners.Accounts[j], path)
				if err != nil {
					return err
				}
			}
		}

		// Applications
		if owners.Applications != nil {
			for j := 0; j < len(owners.Applications); j++ {
				err := rbac_server.setSubjectResourcePermissions("PERMISSIONS/APPLICAITONS/" + owners.Applications[j], path)
				if err != nil {
					return err
				}
				share.Applications = append(share.Applications, owners.Applications[j])
			}
			
		}

		// Peers
		if owners.Peers != nil {
			for j := 0; j < len(owners.Peers); j++ {
				err := rbac_server.setSubjectResourcePermissions("PERMISSIONS/PEERS/" + owners.Peers[j], path)
				if err != nil {
					return err
				}
				share.Peers = append(share.Peers, owners.Peers[j])
			}
		}

		// Groups
		if owners.Groups != nil {
			for j := 0; j < len(owners.Groups); j++ {
				err := rbac_server.setSubjectResourcePermissions("PERMISSIONS/GROUPS/" + owners.Groups[j], path)
				if err != nil {
					return err
				}
				share.Groups = append(share.Groups, owners.Groups[j])
			}
		}

		// Organizations
		if owners.Organizations != nil {
			for j := 0; j < len(owners.Organizations); j++ {
				err := rbac_server.setSubjectResourcePermissions("PERMISSIONS/ORGANIZATIONS/" + owners.Organizations[j], path)
				if err != nil {
					return err
				}
				share.Organizations = append(share.Organizations, owners.Organizations[j])
			}
		}
	}

	// simply marshal the permission and put it into the store.
	data, err := json.Marshal(permissions)
	if err != nil {
		return err
	}

	err = rbac_server.permissions.SetItem(path, data)
	if err != nil {
		return err
	}

	err = rbac_server.shareResource(share)
	if err != nil {
		return err
	}

	return nil
}

//* Set resource permissions this method will replace existing permission at once *
func (rbac_server *server) SetResourcePermissions(ctx context.Context, rqst *rbacpb.SetResourcePermissionsRqst) (*rbacpb.SetResourcePermissionsRqst, error) {
	err := rbac_server.setResourcePermissions(rqst.Path, rqst.Permissions)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	return &rbacpb.SetResourcePermissionsRqst{}, nil
}

/**
 * Remove a resource path for an entity.
 */
func (rbac_server *server) deleteSubjectResourcePermissions(subject string, path string) error {

	data, err := rbac_server.permissions.GetItem(subject)
	if err != nil {
		return err
	}

	paths := make([]string, 0)
	err = json.Unmarshal(data, &paths)
	if err != nil {
		return err
	}

	// Here I will remove the path itself.
	paths = Utility.RemoveString(paths, path)

	// Now I will remove all other path that start with this one...
	for i := 0; i < len(paths); {
		if strings.HasPrefix(paths[i], path) {
			paths = Utility.RemoveString(paths, paths[i])
		} else {
			i++
		}
	}

	data, err = json.Marshal(paths)
	if err != nil {
		return err
	}

	return rbac_server.permissions.SetItem(subject, data)

}

// Remouve a ressource permission
func (rbac_server *server) deleteResourcePermissions(path string, permissions *rbacpb.Permissions) error {

	// Allowed resources
	allowed := permissions.Allowed
	if allowed != nil {
		for i := 0; i < len(allowed); i++ {

			// Accounts
			for j := 0; j < len(allowed[i].Accounts); j++ {
				err := rbac_server.deleteSubjectResourcePermissions("PERMISSIONS/ACCOUNTS/" + allowed[i].Accounts[j], path)
				if err != nil {
					return err
				}
			}

			// Groups
			for j := 0; j < len(allowed[i].Groups); j++ {
				err := rbac_server.deleteSubjectResourcePermissions("PERMISSIONS/GROUPS/" +allowed[i].Groups[j], path)
				if err != nil {
					return err
				}
			}

			// Organizations
			for j := 0; j < len(allowed[i].Organizations); j++ {
				err := rbac_server.deleteSubjectResourcePermissions("PERMISSIONS/ORGANIZATIONS/" +allowed[i].Organizations[j], path)
				if err != nil {
					return err
				}
			}

			// Applications
			for j := 0; j < len(allowed[i].Applications); j++ {
				err := rbac_server.deleteSubjectResourcePermissions("PERMISSIONS/APPLICATIONS/" +allowed[i].Applications[j], path)
				if err != nil {
					return err
				}
			}

			// Peers
			for j := 0; j < len(allowed[i].Peers); j++ {
				err := rbac_server.deleteSubjectResourcePermissions("PERMISSIONS/PEERS/" +allowed[i].Peers[j], path)
				if err != nil {
					return err
				}
			}
		}
	}

	// Denied resources
	denied := permissions.Denied
	if denied != nil {
		for i := 0; i < len(denied); i++ {
			// Acccounts
			for j := 0; j < len(denied[i].Accounts); j++ {
				err := rbac_server.deleteSubjectResourcePermissions("PERMISSIONS/ACCOUNTS/" + denied[i].Accounts[j], path)
				if err != nil {
					return err
				}
			}
			// Applications
			for j := 0; j < len(denied[i].Applications); j++ {
				err := rbac_server.deleteSubjectResourcePermissions("PERMISSIONS/APPLICATIONS/" + denied[i].Applications[j], path)
				if err != nil {
					return err
				}
			}

			// Peers
			for j := 0; j < len(denied[i].Peers); j++ {
				err := rbac_server.deleteSubjectResourcePermissions("PERMISSIONS/PEERS/" + denied[i].Peers[j], path)
				if err != nil {
					return err
				}
			}

			// Groups
			for j := 0; j < len(denied[i].Groups); j++ {
				err := rbac_server.deleteSubjectResourcePermissions("PERMISSIONS/GROUPS/" + denied[i].Groups[j], path)
				if err != nil {
					return err
				}
			}

			// Organizations
			for j := 0; j < len(denied[i].Organizations); j++ {
				err := rbac_server.deleteSubjectResourcePermissions("PERMISSIONS/ORGANIZATIONS/" + denied[i].Organizations[j], path)
				if err != nil {
					return err
				}
			}
		}
	}

	// Owned resources
	owners := permissions.Owners

	if owners != nil {
		// Acccounts
		if owners.Accounts != nil {
			for j := 0; j < len(owners.Accounts); j++ {
				err := rbac_server.deleteSubjectResourcePermissions("PERMISSIONS/ACCOUNTS/" + owners.Accounts[j], path)
				if err != nil {
					return err
				}
			}
		}

		// Applications
		if owners.Applications != nil {
			for j := 0; j < len(owners.Applications); j++ {
				err := rbac_server.deleteSubjectResourcePermissions("PERMISSIONS/APPLICATIONS/" + owners.Applications[j], path)
				if err != nil {
					return err
				}
			}
		}

		// Peers
		if owners.Peers != nil {
			for j := 0; j < len(owners.Peers); j++ {
				err := rbac_server.deleteSubjectResourcePermissions("PERMISSIONS/PEERS/" + owners.Peers[j], path)
				if err != nil {
					return err
				}
			}
		}

		// Groups
		if owners.Groups != nil {
			for j := 0; j < len(owners.Groups); j++ {
				err := rbac_server.deleteSubjectResourcePermissions("PERMISSIONS/GROUPS/" + owners.Groups[j], path)
				if err != nil {
					return err
				}
			}
		}

		// Organizations
		if owners.Organizations != nil {
			for j := 0; j < len(owners.Organizations); j++ {
				err := rbac_server.deleteSubjectResourcePermissions("PERMISSIONS/ORGANIZATIONS/" + owners.Organizations[j], path)
				if err != nil {
					return err
				}
			}
		}
	}

	// unshare the resource
	rbac_server.unshareResource(rbac_server.Domain, path)

	// Remove sub-permissions...
	rbac_server.permissions.RemoveItem(path + "*")

	return rbac_server.permissions.RemoveItem(path)

}

func (rbac_server *server) getResourcePermissions(path string) (*rbacpb.Permissions, error) {

	data, err := rbac_server.permissions.GetItem(path)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	permissions := new(rbacpb.Permissions)
	err = json.Unmarshal(data, &permissions)
	if err != nil {
		return nil, err
	}
	return permissions, nil
}

//* Delete a resource permissions (when a resource is deleted) *
func (rbac_server *server) DeleteResourcePermissions(ctx context.Context, rqst *rbacpb.DeleteResourcePermissionsRqst) (*rbacpb.DeleteResourcePermissionsRqst, error) {
	permissions, err := rbac_server.getResourcePermissions(rqst.Path)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	err = rbac_server.deleteResourcePermissions(rqst.Path, permissions)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &rbacpb.DeleteResourcePermissionsRqst{}, nil
}

//* Delete a specific resource permission *
func (rbac_server *server) DeleteResourcePermission(ctx context.Context, rqst *rbacpb.DeleteResourcePermissionRqst) (*rbacpb.DeleteResourcePermissionRqst, error) {

	permissions, err := rbac_server.getResourcePermissions(rqst.Path)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	if rqst.Type == rbacpb.PermissionType_ALLOWED {
		// Remove the permission from the allowed permission
		allowed := make([]*rbacpb.Permission, 0)
		for i := 0; i < len(permissions.Allowed); i++ {
			if permissions.Allowed[i].Name != rqst.Name {
				allowed = append(allowed, permissions.Allowed[i])
			}
		}
		permissions.Allowed = allowed
	} else if rqst.Type == rbacpb.PermissionType_DENIED {
		// Remove the permission from the allowed permission.
		denied := make([]*rbacpb.Permission, 0)
		for i := 0; i < len(permissions.Denied); i++ {
			if permissions.Denied[i].Name != rqst.Name {
				denied = append(denied, permissions.Denied[i])
			}
		}
		permissions.Denied = denied
	}
	err = rbac_server.setResourcePermissions(rqst.Path, permissions)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &rbacpb.DeleteResourcePermissionRqst{}, nil
}

//* Get the ressource Permission.
func (rbac_server *server) GetResourcePermission(ctx context.Context, rqst *rbacpb.GetResourcePermissionRqst) (*rbacpb.GetResourcePermissionRsp, error) {
	permissions, err := rbac_server.getResourcePermissions(rqst.Path)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// Search on allowed permission
	if rqst.Type == rbacpb.PermissionType_ALLOWED {
		for i := 0; i < len(permissions.Allowed); i++ {
			if permissions.Allowed[i].Name == rqst.Name {
				return &rbacpb.GetResourcePermissionRsp{Permission: permissions.Allowed[i]}, nil
			}
		}
	} else if rqst.Type == rbacpb.PermissionType_DENIED { // search in denied permissions.

		for i := 0; i < len(permissions.Denied); i++ {
			if permissions.Denied[i].Name == rqst.Name {
				return &rbacpb.GetResourcePermissionRsp{Permission: permissions.Allowed[i]}, nil
			}
		}
	}

	return nil, status.Errorf(
		codes.Internal,
		Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("No permission found with name "+rqst.Name)))
}

//* Set specific resource permission  ex. read permission... *
func (rbac_server *server) SetResourcePermission(ctx context.Context, rqst *rbacpb.SetResourcePermissionRqst) (*rbacpb.SetResourcePermissionRsp, error) {
	permissions, err := rbac_server.getResourcePermissions(rqst.Path)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// Remove the permission from the allowed permission
	if rqst.Type == rbacpb.PermissionType_ALLOWED {
		allowed := make([]*rbacpb.Permission, 0)
		for i := 0; i < len(permissions.Allowed); i++ {
			if permissions.Allowed[i].Name == rqst.Permission.Name {
				allowed = append(allowed, permissions.Allowed[i])
			} else {
				allowed = append(allowed, rqst.Permission)
			}
		}
		permissions.Allowed = allowed
	} else if rqst.Type == rbacpb.PermissionType_DENIED {

		// Remove the permission from the allowed permission.
		denied := make([]*rbacpb.Permission, 0)
		for i := 0; i < len(permissions.Denied); i++ {
			if permissions.Denied[i].Name == rqst.Permission.Name {
				denied = append(denied, permissions.Denied[i])
			} else {
				denied = append(denied, rqst.Permission)
			}
		}
		permissions.Denied = denied
	}
	err = rbac_server.setResourcePermissions(rqst.Path, permissions)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &rbacpb.SetResourcePermissionRsp{}, nil
}

//* Get resource permissions *
func (rbac_server *server) GetResourcePermissions(ctx context.Context, rqst *rbacpb.GetResourcePermissionsRqst) (*rbacpb.GetResourcePermissionsRsp, error) {
	permissions, err := rbac_server.getResourcePermissions(rqst.Path)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &rbacpb.GetResourcePermissionsRsp{Permissions: permissions}, nil
}

func (rbac_server *server) addResourceOwner(path string, subject string, subjectType rbacpb.SubjectType) error {
	permissions, err := rbac_server.getResourcePermissions(path)

	if err != nil {
		if strings.Contains(err.Error(), "leveldb: not found"){

			// So here I will create the permissions object...
			permissions = &rbacpb.Permissions{
				Allowed: []*rbacpb.Permission{},
				Denied:  []*rbacpb.Permission{},
				Owners: &rbacpb.Permission{
					Name:          "owner",
					Accounts:      []string{},
					Applications:  []string{},
					Groups:        []string{},
					Peers:         []string{},
					Organizations: []string{},
				},
			}
		} else {
			return err
		}
	}

	needSave := false
	// Owned resources
	owners := permissions.Owners
	if subjectType == rbacpb.SubjectType_ACCOUNT {
		if !Utility.Contains(owners.Accounts, subject) {
			owners.Accounts = append(owners.Accounts, subject)
			needSave = true
		}
	} else if subjectType == rbacpb.SubjectType_APPLICATION {
		if !Utility.Contains(owners.Applications, subject) {
			owners.Applications = append(owners.Applications, subject)
			needSave = true
		}
	} else if subjectType == rbacpb.SubjectType_GROUP {
		if !Utility.Contains(owners.Groups, subject) {
			owners.Groups = append(owners.Groups, subject)
			needSave = true
		}
	} else if subjectType == rbacpb.SubjectType_ORGANIZATION {
		if !Utility.Contains(owners.Organizations, subject) {
			owners.Organizations = append(owners.Organizations, subject)
			needSave = true
		}
	} else if subjectType == rbacpb.SubjectType_PEER {
		if !Utility.Contains(owners.Peers, subject) {
			owners.Peers = append(owners.Peers, subject)
			needSave = true
		}
	}

	// Save permission if ti owner has changed.
	if needSave {
		permissions.Owners = owners
		err = rbac_server.setResourcePermissions(path, permissions)
		if err != nil {
			return err
		}
	}
	return nil
}

//* Add resource owner do nothing if it already exist
func (rbac_server *server) AddResourceOwner(ctx context.Context, rqst *rbacpb.AddResourceOwnerRqst) (*rbacpb.AddResourceOwnerRsp, error) {

	err := rbac_server.addResourceOwner(rqst.Path, rqst.Subject, rqst.Type)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &rbacpb.AddResourceOwnerRsp{}, nil
}

func (rbac_server *server) removeResourceOwner(owner string, subjectType rbacpb.SubjectType, path string) error {
	permissions, err := rbac_server.getResourcePermissions(path)
	if err != nil {
		return err
	}

	// Owned resources
	owners := permissions.Owners
	if subjectType == rbacpb.SubjectType_ACCOUNT {
		if Utility.Contains(owners.Accounts, owner) {
			owners.Accounts = Utility.RemoveString(owners.Accounts, owner)
		}
	} else if subjectType == rbacpb.SubjectType_APPLICATION {
		if Utility.Contains(owners.Applications, owner) {
			owners.Applications = Utility.RemoveString(owners.Applications, owner)
		}
	} else if subjectType == rbacpb.SubjectType_GROUP {
		if Utility.Contains(owners.Groups, owner) {
			owners.Groups = Utility.RemoveString(owners.Groups, owner)
		}
	} else if subjectType == rbacpb.SubjectType_ORGANIZATION {
		if Utility.Contains(owners.Organizations, owner) {
			owners.Organizations = Utility.RemoveString(owners.Organizations, owner)
		}
	} else if subjectType == rbacpb.SubjectType_PEER {
		if Utility.Contains(owners.Peers, owner) {
			owners.Peers = Utility.RemoveString(owners.Peers, owner)
		}
	}

	permissions.Owners = owners
	err = rbac_server.setResourcePermissions(path, permissions)
	if err != nil {
		return err
	}

	return nil
}

// Remove a Subject from denied list and allowed list.
func (rbac_server *server) removeResourceSubject(subject string, subjectType rbacpb.SubjectType, path string) error {
	permissions, err := rbac_server.getResourcePermissions(path)
	if err != nil {
		return err
	}

	// Allowed resources
	allowed := permissions.Allowed
	for i := 0; i < len(allowed); i++ {
		// Accounts
		if subjectType == rbacpb.SubjectType_ACCOUNT {
			accounts := make([]string, 0)
			for j := 0; j < len(allowed[i].Accounts); j++ {
				if subject != allowed[i].Accounts[j] {
					accounts = append(accounts, allowed[i].Accounts[j])
				}

			}
			allowed[i].Accounts = accounts
		}

		// Groups
		if subjectType == rbacpb.SubjectType_GROUP {
			groups := make([]string, 0)
			for j := 0; j < len(allowed[i].Groups); j++ {
				if subject != allowed[i].Groups[j] {
					groups = append(groups, allowed[i].Groups[j])
				}
			}
			allowed[i].Groups = groups
		}

		// Organizations
		if subjectType == rbacpb.SubjectType_ORGANIZATION {
			organizations := make([]string, 0)
			for j := 0; j < len(allowed[i].Organizations); j++ {
				if subject != allowed[i].Organizations[j] {
					organizations = append(organizations, allowed[i].Organizations[j])
				}
			}
			allowed[i].Organizations = organizations
		}

		// Applications
		if subjectType == rbacpb.SubjectType_APPLICATION {
			applications := make([]string, 0)
			for j := 0; j < len(allowed[i].Applications); j++ {
				if subject != allowed[i].Applications[j] {
					applications = append(applications, allowed[i].Applications[j])
				}
			}
			allowed[i].Applications = applications
		}

		// Peers
		if subjectType == rbacpb.SubjectType_PEER {
			peers := make([]string, 0)
			for j := 0; j < len(allowed[i].Peers); j++ {
				if subject != allowed[i].Peers[j] {
					peers = append(peers, allowed[i].Peers[j])
				}
			}
			allowed[i].Peers = peers
		}
	}

	// Denied resources
	denied := permissions.Denied
	for i := 0; i < len(denied); i++ {
		// Accounts
		if subjectType == rbacpb.SubjectType_ACCOUNT {
			accounts := make([]string, 0)
			for j := 0; j < len(denied[i].Accounts); j++ {
				if subject != denied[i].Accounts[j] {
					accounts = append(accounts, denied[i].Accounts[j])
				}

			}
			denied[i].Accounts = accounts
		}

		// Groups
		if subjectType == rbacpb.SubjectType_GROUP {
			groups := make([]string, 0)
			for j := 0; j < len(denied[i].Groups); j++ {
				if subject != denied[i].Groups[j] {
					groups = append(groups, denied[i].Groups[j])
				}
			}
			denied[i].Groups = groups
		}

		// Organizations
		if subjectType == rbacpb.SubjectType_ORGANIZATION {
			organizations := make([]string, 0)
			for j := 0; j < len(denied[i].Organizations); j++ {
				if subject != denied[i].Organizations[j] {
					organizations = append(organizations, denied[i].Organizations[j])
				}
			}
			denied[i].Organizations = organizations
		}

		// Applications
		if subjectType == rbacpb.SubjectType_APPLICATION {
			applications := make([]string, 0)
			for j := 0; j < len(denied[i].Applications); j++ {
				if subject != denied[i].Applications[j] {
					applications = append(applications, denied[i].Applications[j])
				}
			}
			denied[i].Applications = applications
		}

		// Peers
		if subjectType == rbacpb.SubjectType_PEER {
			peers := make([]string, 0)
			for j := 0; j < len(denied[i].Peers); j++ {
				if subject != denied[i].Peers[j] {
					peers = append(peers, denied[i].Peers[j])
				}
			}
			denied[i].Peers = peers
		}
	}

	err = rbac_server.setResourcePermissions(path, permissions)
	if err != nil {
		return err
	}

	return nil
}

//* Remove resource owner
func (rbac_server *server) RemoveResourceOwner(ctx context.Context, rqst *rbacpb.RemoveResourceOwnerRqst) (*rbacpb.RemoveResourceOwnerRsp, error) {
	err := rbac_server.removeResourceOwner(rqst.Subject, rqst.Type, rqst.Path)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &rbacpb.RemoveResourceOwnerRsp{}, nil
}

//* That function must be call when a subject is removed to clean up permissions.
func (rbac_server *server) DeleteAllAccess(ctx context.Context, rqst *rbacpb.DeleteAllAccessRqst) (*rbacpb.DeleteAllAccessRsp, error) {

	// Here I must remove the subject from all permissions.
	data, err := rbac_server.permissions.GetItem(rqst.Subject)
	if err != nil {
		return nil, err
	}

	paths := make([]string, 0)
	err = json.Unmarshal(data, &paths)
	if err != nil {
		return nil, err
	}

	for i := 0; i < len(paths); i++ {

		// Remove from owner
		rbac_server.removeResourceOwner(rqst.Subject, rqst.Type, paths[i])

		// Remove from subject.
		rbac_server.removeResourceSubject(rqst.Subject, rqst.Type, paths[i])

	}

	err = rbac_server.permissions.RemoveItem(rqst.Subject)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &rbacpb.DeleteAllAccessRsp{}, nil
}

// Return  accessAllowed, accessDenied, error
func (rbac_server *server) validateAccess(subject string, subjectType rbacpb.SubjectType, name string, path string) (bool, bool, error) {
	if len(path) == 0 {
		return false, false, errors.New("no path was given to validate access for suject " + subject)
	}

	// .hidden files can be read by all...
	if strings.Contains(path, ".hidden/") {
		 return true, false, nil
	}

	// first I will test if permissions is define
	permissions, err := rbac_server.getResourcePermissions(path)
	if err != nil {
		// In that case I will try to get parent ressource permission.
		if len(strings.Split(path, "/")) > 1 {
			// test for it parent.
			return rbac_server.validateAccess(subject, subjectType, name, path[0:strings.LastIndex(path, "/")])
		}

		if strings.Contains(err.Error(), "leveldb: not found") {
			return true, false, err
		}

		// if no permission are define for a ressource anyone can access it.
		return false, false, err
	}

	// Test if the Subject is owner of the ressource in that case I will git him access.
	owners := permissions.Owners
	isOwner := false
	subjectStr := ""
	if owners != nil {
		if subjectType == rbacpb.SubjectType_ACCOUNT {
			subjectStr = "Account"
			if owners.Accounts != nil {
				if Utility.Contains(owners.Accounts, subject) {
					isOwner = true
				}
			}
		} else if subjectType == rbacpb.SubjectType_APPLICATION {
			subjectStr = "Application"
			if owners.Applications != nil {
				if Utility.Contains(owners.Applications, subject) {
					isOwner = true
				}
			}
		} else if subjectType == rbacpb.SubjectType_GROUP {
			subjectStr = "Group"
			if owners.Groups != nil {
				if Utility.Contains(owners.Groups, subject) {
					isOwner = true
				}
			}
		} else if subjectType == rbacpb.SubjectType_ORGANIZATION {
			subjectStr = "Organization"
			if owners.Organizations != nil {
				if Utility.Contains(owners.Organizations, subject) {
					isOwner = true
				}
			}
		} else if subjectType == rbacpb.SubjectType_PEER {
			subjectStr = "Peer"
			if owners.Peers != nil {
				if Utility.Contains(owners.Peers, subject) {
					isOwner = true
				}
			}
		}
	}

	// If the user is the owner no other validation are required.
	if isOwner {
		return true, false, nil
	}

	// First I will validate that the permission is not denied...
	var denied *rbacpb.Permission
	for i := 0; i < len(permissions.Denied); i++ {
		if permissions.Denied[i].Name == name {
			denied = permissions.Denied[i]
			break
		}
	}

	////////////////////// Test if the access is denied first. /////////////////////
	accessDenied := false

	// Here the Subject is not the owner...
	if denied != nil {
		if subjectType == rbacpb.SubjectType_ACCOUNT {

			// Here the subject is an account.
			if denied.Accounts != nil {
				accessDenied = Utility.Contains(denied.Accounts, subject)
			}

			// The access is not denied for the account itself, I will validate
			// that the account is not part of denied group.
			if !accessDenied {

				// from the account I will get the list of group.
				account, err := rbac_server.getAccount(subject)
				if err != nil {
					return false, false, errors.New("no account named " + subject + " exist")
				}

				if account.Groups != nil {
					for i := 0; i < len(account.Groups); i++ {
						groupId := account.Groups[i]
						_, accessDenied_, _ := rbac_server.validateAccess(groupId, rbacpb.SubjectType_GROUP, name, path)
						if accessDenied_ {
							return false, true, errors.New("access denied for " + subjectStr + " " + subject + " " + name + " " + path)
						}
					}
				}

				// from the account I will get the list of group.
				if account.Organizations != nil {
					for i := 0; i < len(account.Organizations); i++ {
						organizationId := account.Organizations[i]
						_, accessDenied_, _ := rbac_server.validateAccess(organizationId, rbacpb.SubjectType_ORGANIZATION, name, path)
						if accessDenied_ {
							return false, true, errors.New("access denied for " + subjectStr + " " + subject + " " + name + " " + path)
						}
					}
				}

			}

		} else if subjectType == rbacpb.SubjectType_APPLICATION {
			// Here the Subject is an application.
			if denied.Applications != nil {
				accessDenied = Utility.Contains(denied.Applications, subject)
			}
		} else if subjectType == rbacpb.SubjectType_GROUP {
			// Here the Subject is a group
			if denied.Groups != nil {
				accessDenied = Utility.Contains(denied.Groups, subject)
			}

			// The access is not denied for the account itself, I will validate
			// that the account is not part of denied group.
			if !accessDenied {

				// from the account I will get the list of group.
				group, err := rbac_server.getGroup(subject)
				if err != nil {
					return false, false, errors.New("no account named " + subject + " exist")
				}

				if group.Organizations != nil {
					for i := 0; i < len(group.Organizations); i++ {
						organizationId := group.Organizations[i]
						_, accessDenied_, _ := rbac_server.validateAccess(organizationId, rbacpb.SubjectType_ORGANIZATION, name, path)
						if accessDenied_ {
							return false, true, errors.New("access denied for " + subjectStr + " " + organizationId + " " + name + " " + path)
						}
					}
				}

			}

		} else if subjectType == rbacpb.SubjectType_ORGANIZATION {
			// Here the Subject is an Organisations.
			if denied.Organizations != nil {
				accessDenied = Utility.Contains(denied.Organizations, subject)
			}
		} else if subjectType == rbacpb.SubjectType_PEER {
			// Here the Subject is a Peer.
			if denied.Peers != nil {
				accessDenied = Utility.Contains(denied.Peers, subject)
			}
		}
	}

	if accessDenied {
		err := errors.New("access denied for " + subjectStr + " " + subject + " " + name + " " + path)
		return false, true, err
	}

	var allowed *rbacpb.Permission
	for i := 0; i < len(permissions.Allowed); i++ {
		if permissions.Allowed[i].Name == name {
			allowed = permissions.Allowed[i]
			break
		}
	}

	hasAccess := false
	if allowed != nil {
		// Test if the access is allowed
		if subjectType == rbacpb.SubjectType_ACCOUNT {
			if allowed.Accounts != nil {
				hasAccess = Utility.Contains(allowed.Accounts, subject)
				if hasAccess {
					return true, false, nil
				}
			}
			if !hasAccess {

				account, err := rbac_server.getAccount(subject)
				if err == nil {
					// from the account I will get the list of group.
					if account.Groups != nil {
						for i := 0; i < len(account.Groups); i++ {
							groupId := account.Groups[i]
							hasAccess_, _, _ := rbac_server.validateAccess(groupId, rbacpb.SubjectType_GROUP, name, path)
							if hasAccess_ {
								return true, false, nil
							}
						}
					}

					// from the account I will get the list of group.
					if account.Organizations != nil {
						for i := 0; i < len(account.Organizations); i++ {
							organizationId := account.Organizations[i]
							hasAccess_, _, _ := rbac_server.validateAccess(organizationId, rbacpb.SubjectType_ORGANIZATION, name, path)
							if hasAccess_ {
								return true, false, nil
							}
						}

					}
				}
			}

		} else if subjectType == rbacpb.SubjectType_GROUP {
			// validate the group access
			if allowed.Groups != nil {
				hasAccess = Utility.Contains(allowed.Groups, subject)
				if hasAccess {
					return true, false, nil
				}
			}

			if !hasAccess {
				group, err := rbac_server.getGroup(subject)
				if err == nil {
					if group.Organizations != nil {
						for i := 0; i < len(group.Organizations); i++ {
							organizationId := group.Organizations[i]
							hasAccess_, _, _ := rbac_server.validateAccess(organizationId, rbacpb.SubjectType_ORGANIZATION, name, path)
							if hasAccess_ {
								return true, false, nil
							}
						}
					}
				}
			}
		} else if subjectType == rbacpb.SubjectType_ORGANIZATION {
			if allowed.Organizations != nil {
				hasAccess = Utility.Contains(allowed.Organizations, subject)
				if hasAccess {
					return true, false, nil
				}
			}
		} else if subjectType == rbacpb.SubjectType_PEER {
			// Here the Subject is an application.
			if allowed.Peers != nil {
				hasAccess = Utility.Contains(allowed.Peers, subject)
				if hasAccess {
					return true, false, nil
				}
			}
		} else if subjectType == rbacpb.SubjectType_APPLICATION {
			// Here the Subject is an application.
			if allowed.Applications != nil {
				hasAccess = Utility.Contains(allowed.Applications, subject)
				if hasAccess {
					return true, false, nil
				}
			}
		}
	}

	if !hasAccess {
		err := errors.New("access denied for " + subjectStr + " " + subject + " " + name + " " + path)
		return false, false, err
	}

	// The permission is set.
	return true, false, nil
}

//* Validate if a account can get access to a given ressource for a given operation (read, write...) That function is recursive. *
func (rbac_server *server) ValidateAccess(ctx context.Context, rqst *rbacpb.ValidateAccessRqst) (*rbacpb.ValidateAccessRsp, error) {
	// Here I will get information from context.
	hasAccess, accessDenied, err := rbac_server.validateAccess(rqst.Subject, rqst.Type, rqst.Permission, rqst.Path)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// The permission is set.
	return &rbacpb.ValidateAccessRsp{HasAccess: hasAccess, AccessDenied: accessDenied}, nil
}

/** Set action permissions.
When gRPC service methode are called they must validate the ressource pass in parameters.
So each service is reponsible to give access permissions requirement.
*/
func (rbac_server *server) setActionResourcesPermissions(permissions map[string]interface{}) error {
	// So here I will keep values in local storage.cap()
	data, err := json.Marshal(permissions["resources"])
	if err != nil {
		return err
	}
	rbac_server.permissions.SetItem(permissions["action"].(string), data)
	return nil
}

/**
 * Set Action Ressource
 */
func (rbac_server *server) SetActionResourcesPermissions(ctx context.Context, rqst *rbacpb.SetActionResourcesPermissionsRqst) (*rbacpb.SetActionResourcesPermissionsRsp, error) {

	err := rbac_server.setActionResourcesPermissions(rqst.Permissions.AsMap())
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &rbacpb.SetActionResourcesPermissionsRsp{}, nil
}

// Retreive the ressource infos from the database.
func (rbac_server *server) getActionResourcesPermissions(action string) ([]*rbacpb.ResourceInfos, error) {
	if len(action) == 0 {
		return nil, errors.New("no action given")
	}
	data, err := rbac_server.permissions.GetItem(action)
	infos_ := make([]*rbacpb.ResourceInfos, 0)
	if err != nil {
		if !strings.Contains(err.Error(), "leveldb: not found") {
			return nil, err
		} else {
			// no infos_ found...
			return infos_, nil
		}
	}
	infos := make([]interface{}, 0)
	err = json.Unmarshal(data, &infos)

	for i := 0; i < len(infos); i++ {
		info := infos[i].(map[string]interface{})
		infos_ = append(infos_, &rbacpb.ResourceInfos{Index: int32(Utility.ToInt(info["index"])), Permission: info["permission"].(string)})
	}

	return infos_, err
}

//* Return the action resource informations. That function must be called
// before calling ValidateAction. In that way the list of ressource affected
// by the rpc method will be given and resource access validated.
// ex. CopyFile(src, dest) -> src and dest are resource path and must be validated
// for read and write access respectivly.
func (rbac_server *server) GetActionResourceInfos(ctx context.Context, rqst *rbacpb.GetActionResourceInfosRqst) (*rbacpb.GetActionResourceInfosRsp, error) {
	infos, err := rbac_server.getActionResourcesPermissions(rqst.Action)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &rbacpb.GetActionResourceInfosRsp{Infos: infos}, nil
}

/**
 * Validate an action and also validate it resources
 */
func (rbac_server *server) validateAction(action string, subject string, subjectType rbacpb.SubjectType, resources []*rbacpb.ResourceInfos) (bool, error) {

	var actions []string
	if strings.HasPrefix(action, "/echo.EchoService") || strings.HasPrefix(action, "/resource.ResourceService") || strings.HasPrefix(action, "/event.EventService") {
		return true, nil
	}

	// Validate the access for a given suject...
	hasAccess := false

	// So first of all I will validate the actions itself...
	if subjectType == rbacpb.SubjectType_APPLICATION {
		rbac_server.logServiceInfo("", Utility.FileLine(), Utility.FunctionName(), "validate action "+action+" for application "+subject)
		application, err := rbac_server.getApplication(subject)
		if err != nil {
			rbac_server.logServiceInfo("", Utility.FileLine(), Utility.FunctionName(), "fail to retreive application "+subject+" from the resource...")
			return false, err
		}
		actions = application.Actions
	} else if subjectType == rbacpb.SubjectType_PEER {
		rbac_server.logServiceInfo("", Utility.FileLine(), Utility.FunctionName(), "validate action "+action+" for peer "+subject)
		peer, err := rbac_server.getPeer(subject)
		if err != nil {
			rbac_server.logServiceError("", Utility.FileLine(), Utility.FunctionName(), err.Error())
			return false, err
		}
		actions = peer.Actions
	} else if subjectType == rbacpb.SubjectType_ROLE {
		rbac_server.logServiceInfo("", Utility.FileLine(), Utility.FunctionName(), "validate action "+action+" for role "+subject)
		role, err := rbac_server.getRole(subject)
		if err != nil {
			rbac_server.logServiceError("", Utility.FileLine(), Utility.FunctionName(), err.Error())
			return false, err
		}
		actions = role.Actions
	} else if subjectType == rbacpb.SubjectType_ACCOUNT {
		rbac_server.logServiceInfo("", Utility.FileLine(), Utility.FunctionName(), "validate action "+action+" for account "+subject)
		// If the user is the super admin i will return true.
		if subject == "sa" {
			return true, nil
		}

		account, err := rbac_server.getAccount(subject)
		if err != nil {
			rbac_server.logServiceError("", Utility.FileLine(), Utility.FunctionName(), err.Error())
			return false, err
		}

		// call the rpc method.
		if account.Roles != nil {
			for i := 0; i < len(account.Roles); i++ {
				roleId := account.Roles[i]
				hasAccess_, _ := rbac_server.validateAction(action, roleId, rbacpb.SubjectType_ROLE, resources)
				if hasAccess_ {
					hasAccess = hasAccess_
					break
				}
			}
		}
	}

	if !hasAccess {
		if actions != nil {
			for i := 0; i < len(actions); i++ {
				if actions[i] == action {
					hasAccess = true
				}
			}
		}
	}

	if !hasAccess {
		err := errors.New("Access denied for " + subject + " to call method " + action)
		return false, err
	} else if subjectType == rbacpb.SubjectType_ROLE {
		return true, nil
	}

	// Now I will validate the resource access.
	// infos
	permissions_, _ := rbac_server.getActionResourcesPermissions(action)
	if len(resources) > 0 {
		if permissions_ == nil {
			err := errors.New("no resources path are given for validations")
			return false, err
		}
		for i := 0; i < len(resources); i++ {
			if len(resources[i].Path) > 0 { // Here if the path is empty i will simply not validate it.
				hasAccess, accessDenied, _ := rbac_server.validateAccess(subject, subjectType, resources[i].Permission, resources[i].Path)
				if !hasAccess || accessDenied {
					err := errors.New("subject " + subject + " can call the method '" + action + "' but has not the permission to " + resources[i].Permission + " resource '" + resources[i].Path + "'")
					return false, err
				} else if hasAccess {

					rbac_server.logServiceInfo("", Utility.FileLine(), Utility.FunctionName(), "subject "+subject+" can call the method '"+action+"' and has permission to "+resources[i].Permission+" resource '"+resources[i].Path+"'")
					return true, nil
				}
			}
		}
	}
	rbac_server.logServiceInfo("", Utility.FileLine(), Utility.FunctionName(), "subject "+subject+" can call the method '"+action)
	return true, nil
}

//* Validate the actions...
func (rbac_server *server) ValidateAction(ctx context.Context, rqst *rbacpb.ValidateActionRqst) (*rbacpb.ValidateActionRsp, error) {

	// So here From the context I will validate if the application can execute the action...
	var err error
	if len(rqst.Action) == 0 {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("no action was given to validate")))
	}

	// If the address is local I will give the permission.
	//log.Println("validate action ", rqst.Action, rqst.Subject, rqst.Type, rqst.Infos)
	hasAccess, err := rbac_server.validateAction(rqst.Action, rqst.Subject, rqst.Type, rqst.Infos)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &rbacpb.ValidateActionRsp{
		Result: hasAccess,
	}, nil
}

// Set the subject share ressource.
func (rbac_server *server) setSubjectSharedResource(subject, resourceUuid string) error {
	shared := make([]string, 0)
	data, err := rbac_server.permissions.GetItem(subject)
	if err == nil {
		err := json.Unmarshal(data, &shared)
		if err != nil {
			return err
		}
	}

	if !Utility.Contains(shared, resourceUuid) {
		shared = append(shared, resourceUuid)
		// Now I will set back the values in the store.
		data, err := json.Marshal(shared)
		if err != nil {
			return err
		}

		return rbac_server.permissions.SetItem(subject, data)
	}

	return nil
}

func (rbac_server *server) unsetSubjectSharedResource(subject, resourceUuid string) error {
	shared := make([]string, 0)
	data, err := rbac_server.permissions.GetItem(subject)
	if err == nil {
		err := json.Unmarshal(data, &shared)
		if err != nil {
			return err
		}
	}

	if Utility.Contains(shared, resourceUuid) {

		shared = Utility.RemoveString(shared, resourceUuid)
		if err != nil {
			return err
		}

		// set back to db
		data_, err := json.Marshal(shared)
		if err != nil {
			return err
		}
		return rbac_server.permissions.SetItem(subject, data_)
	}

	return nil
}

// Save / Create a Share.
func (rbac_server *server) shareResource(share *rbacpb.Share) error {
	// the id will be compose of the domain @ path ex. domain@/usr/toto/titi
	uuid := Utility.GenerateUUID(share.Domain + share.Path)

	// remove previous record...
	rbac_server.unshareResource(share.Domain, share.Path)

	// set the new record.
	var marshaler jsonpb.Marshaler
	jsonStr, err := marshaler.MarshalToString(share)
	if err != nil {
		return err
	}

	// Now I will serialyse it and save it in the store.
	err = rbac_server.permissions.SetItem(uuid, []byte(jsonStr))
	if err != nil {
		return err
	}

	// Now I will set the value in the user share...
	// The list of accounts
	for i := 0; i < len(share.Accounts); i++ {
		a := "SHARED/ACCOUNTS/" + share.Accounts[i]
		err := rbac_server.setSubjectSharedResource(a, uuid)
		if err != nil {
			return err
		}
	}

	for i := 0; i < len(share.Applications); i++ {
		a := "SHARED/APPLICATIONS/" + share.Applications[i]
		err := rbac_server.setSubjectSharedResource(a, uuid)
		if err != nil {
			return err
		}
	}

	for i := 0; i < len(share.Organizations); i++ {
		o := "SHARED/ORGANIZATIONS/" + share.Organizations[i]
		err := rbac_server.setSubjectSharedResource(o, uuid)
		if err != nil {
			return err
		}
	}

	for i := 0; i < len(share.Groups); i++ {
		g := "SHARED/GROUPS/" + share.Groups[i]
		err := rbac_server.setSubjectSharedResource(g, uuid)
		if err != nil {
			return err
		}
	}

	for i := 0; i < len(share.Peers); i++ {
		p := "SHARED/PEERS/" + share.Peers[i]
		err := rbac_server.setSubjectSharedResource(p, uuid)
		if err != nil {
			return err
		}
	}

	return nil

}

// That function will set a share or update existing share... ex. add/delete account, group
func (rbac_server *server) ShareResource(ctx context.Context, rqst *rbacpb.ShareResourceRqst) (*rbacpb.ShareResourceRsp, error) {

	err := rbac_server.shareResource(rqst.Share)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &rbacpb.ShareResourceRsp{}, nil
}

func (rbac_server *server) unshareResource(domain, path string) error {

	uuid := Utility.GenerateUUID(domain + path)

	var share *rbacpb.Share
	data, err := rbac_server.permissions.GetItem(uuid)
	if err == nil {
		share = new(rbacpb.Share)
		err := jsonpb.UnmarshalString(string(data), share)
		if err != nil {
			return err
		}
	}else{
		return nil // nothing to delete here...
	}

	// Now I will set the value in the user share...
	// The list of accounts
	for i := 0; i < len(share.Accounts); i++ {
		a := "SHARED/ACCOUNTS/" + share.Accounts[i]
		err := rbac_server.unsetSubjectSharedResource(a, uuid)
		if err != nil {
			return err
		}
	}

	for i := 0; i < len(share.Applications); i++ {
		a := "SHARED/APPLICATIONS/" + share.Applications[i]
		err := rbac_server.unsetSubjectSharedResource(a, uuid)
		if err != nil {
			return err
		}
	}

	for i := 0; i < len(share.Organizations); i++ {
		o := "SHARED/ORGANIZATIONS/" + share.Organizations[i]
		err := rbac_server.unsetSubjectSharedResource(o, uuid)
		if err != nil {
			return err
		}
	}

	for i := 0; i < len(share.Groups); i++ {
		g := "SHARED/GROUPS/" + share.Groups[i]
		err := rbac_server.unsetSubjectSharedResource(g, uuid)
		if err != nil {
			return err
		}
	}

	for i := 0; i < len(share.Peers); i++ {
		p := "SHARED/PEERS/" + share.Peers[i]
		err := rbac_server.unsetSubjectSharedResource(p, uuid)
		if err != nil {
			return err
		}
	}

	return rbac_server.permissions.RemoveItem(uuid)
}

// Remove the share
func (rbac_server *server) UshareResource(ctx context.Context, rqst *rbacpb.UnshareResourceRqst) (*rbacpb.UnshareResourceRsp, error) {

	err := rbac_server.unshareResource(rqst.Share.Domain, rqst.Share.Path)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &rbacpb.UnshareResourceRsp{}, nil
}

// Get the list of accessible shared ressource.
// TODO if account also get share for groups and organization that the acount is part of...
func (rbac_server *server) getSharedResource(subject string, subjectType rbacpb.SubjectType) ([]*rbacpb.Share, error) {

	// So here I will get the share resource for a given subject.
	id := "SHARED/"
	if subjectType == rbacpb.SubjectType_ACCOUNT {
		id += "ACCOUNTS"
	} else if subjectType == rbacpb.SubjectType_APPLICATION {
		id += "APPLICATIONS"
	} else if subjectType == rbacpb.SubjectType_GROUP {
		id += "GROUPS"
	} else if subjectType == rbacpb.SubjectType_PEER {
		id += "PEERS"
	} else if subjectType == rbacpb.SubjectType_ORGANIZATION {
		id += "ORGANIZATIONS"
	}
	id += "/" + subject

	// Now I will retreive the list of existing path.
	shared := make([]string, 0)
	data, err := rbac_server.permissions.GetItem(id)
	if err == nil {
		err := json.Unmarshal(data, &shared)
		if err != nil {
			return nil, err
		}
	}
	share_ := make([]*rbacpb.Share, 0)

	// So now I go the list of shared uuid.
	for i := 0; i < len(shared); i++ {
		data, err := rbac_server.permissions.GetItem(shared[i])
		if err == nil {
			share := new(rbacpb.Share)
			err := jsonpb.UnmarshalString(string(data), share)
			if err == nil {
				share_ = append(share_, share)
			}
		}
	}

	// Here I will get the share for groups.
	if subjectType == rbacpb.SubjectType_ACCOUNT {
		account, err := rbac_server.getAccount(subject)
		if err != nil {
			return nil, err
		}

		for i:=0; i < len(account.Groups); i++ {
			share__, err := rbac_server.getSharedResource(account.Groups[i], rbacpb.SubjectType_GROUP)
			if err == nil {
				for j:=0; j < len(share__); j++ {
					// test if the share already exist in the path
					contain := false
					for k:=0; k < len(share_); k++ {
						if share__[j].Path == share_[k].Path {
							contain = true
							break
						}
					}
					if !contain {
						// append the list of values...
						share_ = append(share_, share__[j])
					}
					
				}
			}
		}

		for i:=0; i < len(account.Organizations); i++ {
			share__, err := rbac_server.getSharedResource(account.Organizations[i], rbacpb.SubjectType_ORGANIZATION)
			if err == nil {
				for j:=0; j < len(share__); j++ {
					// test if the share already exist in the path
					contain := false
					for k:=0; k < len(share_); k++ {
						if share__[j].Path == share_[k].Path {
							contain = true
							break
						}
					}
					if !contain {
						// append the list of values...
						share_ = append(share_, share__[j])
					}
					
				}
			}
		}
	}

	// Now for groups I will also take organizations shares...
	if subjectType == rbacpb.SubjectType_GROUP {
		group, err := rbac_server.getGroup(subject)
		if err != nil {
			return nil, err
		}

		for i:=0; i < len(group.Organizations); i++ {
			share__, err := rbac_server.getSharedResource(group.Organizations[i], rbacpb.SubjectType_ORGANIZATION)
			if err == nil {
				for j:=0; j < len(share__); j++ {
					// test if the share already exist in the path
					contain := false
					for k:=0; k < len(share_); k++ {
						if share__[j].Path == share_[k].Path {
							contain = true
							break
						}
					}
					if !contain {
						// append the list of values...
						share_ = append(share_, share__[j])
					}
					
				}
			}
		}
	}

	return share_, nil
}

// Get the list of accessible shared ressources.
func (rbac_server *server) GetSharedResource(ctx context.Context, rqst *rbacpb.GetSharedResourceRqst) (*rbacpb.GetSharedResourceRsp, error) {
	share, err := rbac_server.getSharedResource(rqst.Subject, rqst.Type)

	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &rbacpb.GetSharedResourceRsp{SharedResource: share}, nil
}

func (rbac_server *server) removeSubjectFromShare(subject string, subjectType rbacpb.SubjectType, ressourceId string) error {
	data, err := rbac_server.permissions.GetItem(ressourceId)
	if err != nil {
		return err
	}

	share := new(rbacpb.Share)
	err = jsonpb.UnmarshalString(string(data), share)
	if err != nil {
		return err
	}

	// Remove the subject from the share...
	if subjectType == rbacpb.SubjectType_ACCOUNT {
		share.Accounts = Utility.RemoveString(share.Accounts, subject)
	} else if subjectType == rbacpb.SubjectType_APPLICATION {
		share.Applications = Utility.RemoveString(share.Applications, subject)
	} else if subjectType == rbacpb.SubjectType_GROUP {
		share.Groups = Utility.RemoveString(share.Groups, subject)
	} else if subjectType == rbacpb.SubjectType_PEER {
		share.Peers = Utility.RemoveString(share.Peers, subject)
	} else if subjectType == rbacpb.SubjectType_ORGANIZATION {
		share.Organizations = Utility.RemoveString(share.Organizations, subject)
	}

	err = rbac_server.shareResource(share)
	if err != nil {
		return err
	}

	return nil
}

// Remove a subject from a share.
func (rbac_server *server) RemoveSubjectFromShare(ctx context.Context, rqst *rbacpb.RemoveSubjectFromShareRqst) (*rbacpb.RemoveSubjectFromShareRsp, error) {

	// Here I will get the share and remove the subject from it.
	// the id will be compose of the domain @ path ex. domain@/usr/toto/titi
	uuid := Utility.GenerateUUID(rqst.Domain + rqst.Path)

	err := rbac_server.removeSubjectFromShare(rqst.Subject, rqst.Type, uuid)

	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &rbacpb.RemoveSubjectFromShareRsp{}, nil
}

func (rbac_server *server) deleteSubjectShare(subject string, subjectType rbacpb.SubjectType) error{
	// First of all I will get the list of share the subject is part of.
	id := "SHARED/"

	if subjectType == rbacpb.SubjectType_ACCOUNT {
		id += "ACCOUNTS"
	} else if subjectType == rbacpb.SubjectType_APPLICATION {
		id += "APPLICATIONS"
	} else if subjectType == rbacpb.SubjectType_GROUP {
		id += "GROUPS"
	} else if subjectType == rbacpb.SubjectType_PEER {
		id += "PEERS"
	} else if subjectType == rbacpb.SubjectType_ORGANIZATION {
		id += "ORGANIZATIONS"
	}

	id += "/" + subject

	// Now I will retreive the list of existing path.
	shared := make([]string, 0)
	data, err := rbac_server.permissions.GetItem(id)
	if err == nil {
		err := json.Unmarshal(data, &shared)
		if err != nil {
			return err
		}
	}

	// So now I go the list of shared uuid.
	for i := 0; i < len(shared); i++ {
		err := rbac_server.removeSubjectFromShare(subject, subjectType, shared[i])
		if err != nil {
			return err
		}
	}

	// And finaly I will remove the entry with the id...
	err = rbac_server.permissions.RemoveItem(id)
	if err != nil {
		return err
	}

	return nil
}


// Delete the subject
func (rbac_server *server) DeleteSubjectShare(ctx context.Context, rqst *rbacpb.DeleteSubjectShareRqst) (*rbacpb.DeleteSubjectShareRsp, error) {

	err := rbac_server.deleteSubjectShare(rqst.Subject, rqst.Type)

	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &rbacpb.DeleteSubjectShareRsp{}, nil
}
