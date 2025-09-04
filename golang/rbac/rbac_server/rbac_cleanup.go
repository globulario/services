// rbac_cleanup.go: consistency cleanup for permissions.

package main

import (
	"encoding/base64"
	"errors"
	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/rbac/rbacpb"
	Utility "github.com/globulario/utility"
	"google.golang.org/protobuf/proto"
	"strings"
)

func (srv *server) cleanupPermission(permission *rbacpb.Permission) (bool, *rbacpb.Permission) {
	hasChange := false
	if permission == nil {
		return false, nil
	}

	// logPrintln("cleanupPermission")
	// Cleanup owners from deleted subjects...
	accounts_change, accounts := srv.cleanupSubjectPermissions(rbacpb.SubjectType_ACCOUNT, permission.Accounts)
	if accounts_change {
		hasChange = true
		permission.Accounts = accounts
	}

	applications_change, applications := srv.cleanupSubjectPermissions(rbacpb.SubjectType_APPLICATION, permission.Applications)
	if applications_change {
		hasChange = true
		permission.Applications = applications
	}

	groups_change, groups := srv.cleanupSubjectPermissions(rbacpb.SubjectType_GROUP, permission.Groups)
	if groups_change {
		hasChange = true
		permission.Groups = groups
	}

	organizations_change, organizations := srv.cleanupSubjectPermissions(rbacpb.SubjectType_ORGANIZATION, permission.Organizations)
	if organizations_change {
		hasChange = true
		permission.Organizations = organizations
	}

	peers_change, peers := srv.cleanupSubjectPermissions(rbacpb.SubjectType_PEER, permission.Peers)
	if peers_change {
		hasChange = true
		permission.Peers = peers
	}

	return hasChange, permission
}

func (srv *server) cleanupPermissions(permissions *rbacpb.Permissions) (bool, *rbacpb.Permissions, error) {

	// Delete the indexation
	if permissions.ResourceType == "file" {
		deleted := false
		if strings.HasPrefix(permissions.Path, "/users/") || strings.HasPrefix(permissions.Path, "/applications/") {
			if !Utility.Exists(config.GetDataDir() + "/files" + permissions.Path) {
				srv.deleteResourcePermissions(permissions.Path, permissions)
				deleted = true
			}
		} else if !Utility.Exists(permissions.Path) {
			srv.deleteResourcePermissions(permissions.Path, permissions)
			deleted = true
		}

		// Now I will send deleted event...
		if deleted {
			data, err := proto.Marshal(permissions)
			if err != nil {
				return false, nil, err
			}
			encoded := []byte(base64.StdEncoding.EncodeToString(data))
			srv.publish("delete_resources_permissions_event", encoded)
			return false, nil, errors.New("file does not exist " + permissions.Path)
		}
	}

	hasChange := false
	ownersChange, owners := srv.cleanupPermission(permissions.Owners)
	if ownersChange {
		permissions.Owners = owners
		hasChange = true
	}

	// Allowed...
	for i := range permissions.Allowed {
		permissionHasChange, permission := srv.cleanupPermission(permissions.Allowed[i])
		if permissionHasChange {
			permissions.Allowed[i] = permission
			hasChange = true
		}
	}

	for i := range permissions.Denied {
		permissionHasChange, permission := srv.cleanupPermission(permissions.Denied[i])
		if permissionHasChange {
			permissions.Allowed[i] = permission
			hasChange = true
		}
	}

	return hasChange, permissions, nil
}

func (srv *server) cleanupSubjectPermissions(subjectType rbacpb.SubjectType, subjects []string) (bool, []string) {
	// So here I will remove subject that no more exist in the permissions and keep up to date...
	subjects_ := make([]string, 0)
	needSave := false

	switch subjectType {
	case rbacpb.SubjectType_ACCOUNT:
		for i := range subjects {
			exist, a := srv.accountExist(subjects[i])
			if exist {
				subjects_ = append(subjects_, a)
			} else {
				needSave = true
			}
		}
	case rbacpb.SubjectType_APPLICATION:
		for i := range subjects {
			exist, a := srv.applicationExist(subjects[i])
			if exist {
				subjects_ = append(subjects_, a)
			} else {
				needSave = true
			}
		}

	case rbacpb.SubjectType_GROUP:
		for i := range subjects {
			exist, g := srv.groupExist(subjects[i])
			if exist {
				subjects_ = append(subjects_, g)
			} else {
				needSave = true
			}
		}
	case rbacpb.SubjectType_ORGANIZATION:
		for i := range subjects {
			exist, o := srv.organizationExist(subjects[i])
			if exist {
				subjects_ = append(subjects_, o)
			} else {
				needSave = true
			}
		}
	case rbacpb.SubjectType_PEER:
		for i := range subjects {
			if srv.peerExist(subjects[i]) {
				subjects_ = append(subjects_, subjects[i])
			} else {
				needSave = true
			}
		}
	}

	return needSave, subjects_
}
