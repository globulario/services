// rbac_index.go: resource type/path indexation utilities.

// @awareness namespace=globular.platform
// @awareness component=platform_rbac
// @awareness file_role=rbac_permission_index
// @awareness implements=globular.platform:intent.security.deny_overrides_allow
// @awareness risk=high
package main

import (
	"encoding/json"
	"errors"
	"github.com/globulario/services/golang/rbac/rbacpb"
	Utility "github.com/globulario/utility"
	"strings"
)

func (srv *server) getResourceTypePathIndexation(resource_type string) ([]*rbacpb.Permissions, error) {

	data, err := srv.getItem(resource_type)
	if err != nil {
		return nil, err
	}
	// Empty payload = no indexed paths for this resource type. Treat as
	// empty list instead of letting json.Unmarshal raise "unexpected end
	// of JSON input" — same root cause as the rbac_actions.go regression.
	if len(data) == 0 {
		return []*rbacpb.Permissions{}, nil
	}

	paths := make([]string, 0)
	err = json.Unmarshal(data, &paths)
	if err != nil {
		return nil, err
	}

	permissions := make([]*rbacpb.Permissions, 0)
	for i := range paths {

		p, err := srv.getResourcePermissions(paths[i])
		if err == nil && p != nil {
			if p.ResourceType == resource_type {
				permissions = append(permissions, p)
			}
		}
	}

	return permissions, nil
}

func (srv *server) setResourceTypePathIndexation(resource_type string, path string) error {

	// logPrintln("setSubjectResourcePermissions", path)
	// Here I will retreive the actual list of paths use by this user.
	data, err := srv.getItem(resource_type)
	paths_ := make([]interface{}, 0)

	if err == nil && len(data) > 0 {
		err := json.Unmarshal(data, &paths_)
		if err != nil {
			return err
		}
	}

	paths := make([]string, len(paths_))
	for i := range paths_ {
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
	return srv.setItem(resource_type, data)
}

func (srv *server) setSubjectResourcePermissions(subject string, path string) error {

	// Here I will retreive the actual list of paths use by this user.
	data, _ := srv.getItem(subject)
	paths_ := make([]any, 0)

	if len(data) > 0 {
		err := json.Unmarshal(data, &paths_)
		if err != nil {
			return err
		}
	}

	paths := make([]string, len(paths_))
	for i := range paths_ {
		paths[i] = paths_[i].(string)
	}

	if !Utility.Contains(paths, path) {
		paths = append(paths, path)
	} else {
		return nil // nothing todo here...
	}

	// simply marshal the permission and put it into the store.
	data, err := json.Marshal(paths)
	if err != nil {
		return err
	}

	err = srv.setItem(subject, data)
	if err != nil {
		return err
	}

	return nil
}

func (srv *server) getSubjectResourcePermissions(subject, resource_type string, subject_type rbacpb.SubjectType) ([]*rbacpb.Permissions, error) {

	// set the key to looking for...
	id := "PERMISSIONS/"
	switch subject_type {
	case rbacpb.SubjectType_ACCOUNT:
		id += "ACCOUNTS/"
		exist, a := srv.accountExist(subject)
		if exist {
			id += a
		} else {
			return nil, errors.New("no account found with id " + subject)
		}
	case rbacpb.SubjectType_APPLICATION:
		id += "APPLICATIONS/"
		exist, a := srv.applicationExist(subject)
		if exist {
			id += a
		} else {
			return nil, errors.New("no application found with id " + subject)
		}
	case rbacpb.SubjectType_GROUP:
		id += "GROUPS/"
		exist, g := srv.groupExist(subject)
		if exist {
			id += g
		} else {
			return nil, errors.New("no group found with id " + subject)
		}
	case rbacpb.SubjectType_ORGANIZATION:
		id += "ORGANIZATIONS/"
		exist, o := srv.organizationExist(subject)
		if exist {
			id += o
		} else {
			return nil, errors.New("no organization found with id " + subject)
		}
	case rbacpb.SubjectType_NODE_IDENTITY:
		id += "NODE_IDENTITIES/"
		id += subject
	}

	// Set the subject.
	data, err := srv.getItem(id)

	// retreive path
	permissions := make([]*rbacpb.Permissions, 0)
	if err != nil {
		return permissions, nil
	}
	// Empty record = no permissions assigned yet. Return empty list
	// instead of failing the Unmarshal below.
	if len(data) == 0 {
		return permissions, nil
	}

	paths := make([]interface{}, 0)
	err = json.Unmarshal(data, &paths)
	if err != nil {
		return nil, err
	}

	for i := range paths {
		p, err := srv.getResourcePermissions(paths[i].(string))
		if err == nil && p != nil {
			if p.ResourceType == resource_type || len(resource_type) == 0 {
				permissions = append(permissions, p)
			}
		}
	}

	return permissions, nil
}

func (srv *server) deleteResourceTypePathIndexation(resource_type string, path string) error {

	data, err := srv.getItem(resource_type)
	if err != nil {
		return err
	}
	// Nothing indexed for this resource type = nothing to delete.
	if len(data) == 0 {
		return nil
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

	return srv.setItem(resource_type, data)
}

func (srv *server) deleteSubjectResourcePermissions(subject string, path string) error {
	srv.cache.RemoveItem(path)
	data, err := srv.getItem(subject)
	if err != nil {
		return err
	}
	// Subject has no resource permissions stored = nothing to delete.
	if len(data) == 0 {
		return nil
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

	return srv.setItem(subject, data)

}
