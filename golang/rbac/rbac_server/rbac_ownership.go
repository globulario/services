// rbac_ownership.go: ownership management helpers.

package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"

	"github.com/globulario/services/golang/rbac/rbacpb"
	Utility "github.com/globulario/utility"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

func (srv *server) addResourceOwner(path, resourceType_, subject string, subjectType rbacpb.SubjectType) error {
	if len(path) == 0 {
		return errors.New("no resource path was given")
	}

	if len(subject) == 0 {
		return errors.New("no subject was given")
	}

	if len(subject) == 0 {
		return errors.New("no resource type was given")
	}

	permissions, err := srv.getResourcePermissions(path)

	needSave := false
	if err != nil {
		if strings.Contains(err.Error(), "item not found") {

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
				ResourceType: resourceType_,
				Path:         path,
			}
			needSave = true
		} else {
			return err
		}
	}

	// Owned resources
	owners := permissions.Owners
	if owners == nil {
		owners = &rbacpb.Permission{
			Name:          "owner",
			Accounts:      []string{},
			Applications:  []string{},
			Groups:        []string{},
			Peers:         []string{},
			Organizations: []string{},
		}
	}

	switch subjectType {
	case rbacpb.SubjectType_ACCOUNT:
		exist, a := srv.accountExist(subject)
		if exist {
			if !Utility.Contains(owners.Accounts, a) {
				owners.Accounts = append(owners.Accounts, a)
				needSave = true
			}
		} else {
			return errors.New("account with id " + subject + " does not exist")
		}
	case rbacpb.SubjectType_APPLICATION:
		exist, a := srv.applicationExist(subject)
		if exist {
			if !Utility.Contains(owners.Applications, a) {
				owners.Applications = append(owners.Applications, a)
				needSave = true
			}
		} else {
			return errors.New("application with id " + subject + " does not exist")
		}
	case rbacpb.SubjectType_GROUP:
		exist, g := srv.groupExist(subject)
		if exist {
			if !Utility.Contains(owners.Groups, g) {
				owners.Groups = append(owners.Groups, g)
				needSave = true
			}
		} else {
			return errors.New("group with id " + subject + " does not exist")
		}
	case rbacpb.SubjectType_ORGANIZATION:
		exist, o := srv.organizationExist(subject)
		if exist {
			if !Utility.Contains(owners.Organizations, o) {
				owners.Organizations = append(owners.Organizations, o)
				needSave = true
			}
		} else {
			return errors.New("organisation with id " + subject + " does not exist")
		}
	case rbacpb.SubjectType_PEER:
		if !Utility.Contains(owners.Peers, subject) {
			owners.Peers = append(owners.Peers, subject)
			needSave = true
		}
	}

	// Save permission if it's owner has changed.
	if needSave {
		permissions.Owners = owners
		err = srv.setResourcePermissions(path, permissions.ResourceType, permissions)
		if err != nil {
			return err
		}
	}

	return nil
}

// AddResourceOwner adds an owner to a specified resource in the RBAC system.
// It takes a context and an AddResourceOwnerRqst containing the resource path,
// resource type, subject, and ownership type. If the operation fails, it returns
// an error with details; otherwise, it returns an empty AddResourceOwnerRsp on success.
func (srv *server) AddResourceOwner(ctx context.Context, rqst *rbacpb.AddResourceOwnerRqst) (*rbacpb.AddResourceOwnerRsp, error) {

	err := srv.addResourceOwner(rqst.Path, rqst.ResourceType, rqst.Subject, rqst.Type)

	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	return &rbacpb.AddResourceOwnerRsp{}, nil
}

func (srv *server) removeResourceOwner(owner string, subjectType rbacpb.SubjectType, path string) error {

	permissions, err := srv.getResourcePermissions(path)
	if err != nil {
		return err
	}

	// Owned resources
	owners := permissions.Owners
	switch subjectType {
	case rbacpb.SubjectType_ACCOUNT:
		if Utility.Contains(owners.Accounts, owner) {
			owners.Accounts = Utility.RemoveString(owners.Accounts, owner)
		}
	case rbacpb.SubjectType_APPLICATION:
		if Utility.Contains(owners.Applications, owner) {
			owners.Applications = Utility.RemoveString(owners.Applications, owner)
		}
	case rbacpb.SubjectType_GROUP:
		if Utility.Contains(owners.Groups, owner) {
			owners.Groups = Utility.RemoveString(owners.Groups, owner)
		}
	case rbacpb.SubjectType_ORGANIZATION:
		if Utility.Contains(owners.Organizations, owner) {
			owners.Organizations = Utility.RemoveString(owners.Organizations, owner)
		}
	case rbacpb.SubjectType_PEER:
		if Utility.Contains(owners.Peers, owner) {
			owners.Peers = Utility.RemoveString(owners.Peers, owner)
		}
	}

	permissions.Owners = owners
	err = srv.setResourcePermissions(path, permissions.ResourceType, permissions)
	if err != nil {
		return err
	}

	return nil
}

func (srv *server) removeResourceSubject(subject string, subjectType rbacpb.SubjectType, path string) error {

	permissions, err := srv.getResourcePermissions(path)
	if err != nil {
		return err
	}

	// Allowed resources
	allowed := permissions.Allowed
	for i := range allowed {
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
	for i := range denied {
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

	err = srv.setResourcePermissions(path, permissions.ResourceType, permissions)
	if err != nil {
		return err
	}

	return nil
}

func (srv *server) RemoveResourceOwner(ctx context.Context, rqst *rbacpb.RemoveResourceOwnerRqst) (*rbacpb.RemoveResourceOwnerRsp, error) {
	err := srv.removeResourceOwner(rqst.Subject, rqst.Type, rqst.Path)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &rbacpb.RemoveResourceOwnerRsp{}, nil
}

func (srv *server) DeleteAllAccess(ctx context.Context, rqst *rbacpb.DeleteAllAccessRqst) (*rbacpb.DeleteAllAccessRsp, error) {
	subjectId := ""
	switch rqst.Type {
	case rbacpb.SubjectType_ACCOUNT:
		exist, a := srv.accountExist(rqst.Subject)
		if exist {
			subjectId = "PERMISSIONS/ACCOUNTS/" + a
		} else {
			return nil, errors.New("no account found with id " + rqst.Subject)
		}
	case rbacpb.SubjectType_APPLICATION:
		exist, a := srv.applicationExist(rqst.Subject)
		if exist {
			subjectId = "PERMISSIONS/APPLICATIONS/" + a
		} else {
			return nil, errors.New("2167 no application found with id " + rqst.Subject)
		}
	case rbacpb.SubjectType_GROUP:
		exist, g := srv.groupExist(rqst.Subject)
		if exist {
			subjectId = "PERMISSIONS/GROUPS/" + g
		} else {
			return nil, errors.New("no group found with id " + rqst.Subject)
		}
	case rbacpb.SubjectType_ORGANIZATION:
		exist, o := srv.organizationExist(rqst.Subject)
		if exist {
			subjectId = "PERMISSIONS/ORGANIZATIONS/" + o
		} else {
			return nil, errors.New("no organization found with id " + rqst.Subject)
		}
	case rbacpb.SubjectType_PEER:
		subjectId = "PERMISSIONS/PEERS/" + rqst.Subject
	}

	// Here I must remove the subject from all permissions.
	data, err := srv.getItem(subjectId)
	if err != nil {
		return nil, err
	}

	paths := make([]string, 0)
	err = json.Unmarshal(data, &paths)
	if err != nil {
		return nil, err
	}

	// Remove the suject from all permissions with given paths.
	for i := range paths {

		// Remove from owner
		srv.removeResourceOwner(rqst.Subject, rqst.Type, paths[i])

		// Remove from subject.
		srv.removeResourceSubject(rqst.Subject, rqst.Type, paths[i])

		// Now I will send an update event.
		permissions, err := srv.getResourcePermissions(paths[i])
		if err == nil {
			// That's the way to marshal object as evt data
			data_, err := proto.Marshal(permissions)
			if err == nil {
				encoded := []byte(base64.StdEncoding.EncodeToString(data_))
				srv.publish("set_resources_permissions_event", encoded)
			}
		}
	}

	// remove the indexation...
	err = srv.removeItem(subjectId)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &rbacpb.DeleteAllAccessRsp{}, nil
}
