// rbac_sharing.go: share/unshare resources and listing.

package main

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/globulario/services/golang/rbac/rbacpb"
	Utility "github.com/globulario/utility"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
)

func (srv *server) setSubjectSharedResource(subject, resourceUuid string) error {

	shared := make([]string, 0)
	data, err := srv.getItem(subject)
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

		return srv.setItem(subject, data)
	}

	return nil
}

func (srv *server) unsetSubjectSharedResource(subject, resourceUuid string) error {

	shared := make([]string, 0)
	data, err := srv.getItem(subject)
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
		return srv.setItem(subject, data_)
	}

	return nil
}

func (srv *server) shareResource(share *rbacpb.Share) error {

	// the id will be compose of the domain @ path ex. domain@/usr/toto/titi
	uuid := Utility.GenerateUUID(share.Domain + share.Path)

	// remove previous record...
	srv.unshareResource(share.Domain, share.Path)

	// set the new record.
	jsonStr, err := protojson.Marshal(share)
	if err != nil {
		return err
	}

	// Now I will serialyse it and save it in the store.
	err = srv.setItem(uuid, []byte(jsonStr))
	if err != nil {
		return err
	}

	// Now I will set the value in the user share...
	// The list of accounts
	for i := range share.Accounts {
		accountExist, accountId := srv.accountExist(share.Accounts[i])
		if !accountExist {
			return errors.New("no account exist with id " + share.Accounts[i])
		}
		a := "SHARED/ACCOUNTS/" + accountId
		err := srv.setSubjectSharedResource(a, uuid)
		if err != nil {
			return err
		}
	}

	for i := range share.Applications {
		applicationExist, applicationId := srv.applicationExist(share.Applications[i])
		if !applicationExist {
			return errors.New("no application exist with id " + share.Applications[i])
		}
		a := "SHARED/APPLICATIONS/" + applicationId
		err := srv.setSubjectSharedResource(a, uuid)
		if err != nil {
			return err
		}
	}

	for i := range share.Organizations {
		organizationExist, organizationId := srv.organizationExist(share.Organizations[i])
		if !organizationExist {
			return errors.New("no organization exist with id " + share.Organizations[i])
		}
		o := "SHARED/ORGANIZATIONS/" + organizationId
		err := srv.setSubjectSharedResource(o, uuid)
		if err != nil {
			return err
		}
	}

	for i := range share.Groups {
		groupExist, groupId := srv.groupExist(share.Groups[i])
		if !groupExist {
			return errors.New("no group exist with id " + share.Groups[i])
		}
		g := "SHARED/GROUPS/" + groupId
		err := srv.setSubjectSharedResource(g, uuid)
		if err != nil {
			return err
		}
	}

	for i := range share.Peers {
		if !srv.peerExist(share.Peers[i]) {
			return errors.New("no peer exist with id " + share.Peers[i])
		}
		p := "SHARED/PEERS/" + share.Peers[i]
		err := srv.setSubjectSharedResource(p, uuid)
		if err != nil {
			return err
		}
	}

	return nil

}

func (srv *server) unshareResource(domain, path string) error {

	// logPrintln("unshareResource")
	uuid := Utility.GenerateUUID(domain + path)

	var share *rbacpb.Share
	data, err := srv.getItem(uuid)
	if err == nil {
		share = new(rbacpb.Share)
		err := protojson.Unmarshal(data, share)
		if err != nil {
			return err
		}
	} else {
		return nil // nothing to delete here...
	}

	// Now I will set the value in the user share...
	// The list of accounts
	for i := range share.Accounts {
		a := "SHARED/ACCOUNTS/" + share.Accounts[i]
		err := srv.unsetSubjectSharedResource(a, uuid)
		if err != nil {
			return err
		}
	}

	for i := range share.Applications {
		a := "SHARED/APPLICATIONS/" + share.Applications[i]
		err := srv.unsetSubjectSharedResource(a, uuid)
		if err != nil {
			return err
		}
	}

	for i := range share.Organizations {
		o := "SHARED/ORGANIZATIONS/" + share.Organizations[i]
		err := srv.unsetSubjectSharedResource(o, uuid)
		if err != nil {
			return err
		}
	}

	for i := range share.Groups {
		g := "SHARED/GROUPS/" + share.Groups[i]
		err := srv.unsetSubjectSharedResource(g, uuid)
		if err != nil {
			return err
		}
	}

	for i := range share.Peers {
		p := "SHARED/PEERS/" + share.Peers[i]
		err := srv.unsetSubjectSharedResource(p, uuid)
		if err != nil {
			return err
		}
	}

	return srv.removeItem(uuid)
}

func (srv *server) getSharedResource(subject string, subjectType rbacpb.SubjectType) ([]*rbacpb.Share, error) {

	// So here I will get the share resource for a given subject.
	id := "SHARED/"
	switch subjectType {
	case rbacpb.SubjectType_ACCOUNT:
		id += "ACCOUNTS"
		exist, a := srv.accountExist(subject)
		if !exist {
			return nil, errors.New("no account exist with id " + subject)
		}
		id += "/" + a
	case rbacpb.SubjectType_APPLICATION:
		id += "APPLICATIONS"
		exist, a := srv.applicationExist(subject)
		if !exist {
			return nil, errors.New("no application exist with id " + subject)
		}
		id += "/" + a
	case rbacpb.SubjectType_GROUP:
		id += "GROUPS"
		exist, g := srv.groupExist(subject)
		if !exist {
			return nil, errors.New("no group exist with id " + subject)
		}
		id += "/" + g
	case rbacpb.SubjectType_PEER:
		id += "PEERS/" + subject
	case rbacpb.SubjectType_ORGANIZATION:
		id += "ORGANIZATIONS"
		exist, o := srv.organizationExist(subject)
		if !exist {
			return nil, errors.New("no organization exist with id " + subject)
		}
		id += "/" + o
	}

	// Now I will retreive the list of existing path.
	shared := make([]string, 0)
	data, err := srv.getItem(id)
	if err == nil {
		err := json.Unmarshal(data, &shared)
		if err != nil {
			return nil, err
		}
	}

	share_ := make([]*rbacpb.Share, 0)

	// So now I go the list of shared uuid.
	for i := range shared {
		data, err := srv.getItem(shared[i])
		if err == nil {
			share := new(rbacpb.Share)
			err := protojson.Unmarshal(data, share)
			if err == nil {

				if len(share.Accounts) > 0 || len(share.Applications) > 0 || len(share.Groups) > 0 || len(share.Organizations) > 0 {
					share_ = append(share_, share)
				}
			}
		}
	}

	// Here I will get the share for groups.
	if subjectType == rbacpb.SubjectType_ACCOUNT {

		account, err := srv.getAccount(subject)
		if err != nil {
			return nil, err
		}

		for i := range account.Groups {
			share__, err := srv.getSharedResource(account.Groups[i], rbacpb.SubjectType_GROUP)
			if err == nil {
				for j := 0; j < len(share__); j++ {
					// test if the share already exist in the path
					contain := false
					for k := 0; k < len(share_); k++ {
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

		for i := range account.Organizations {
			share__, err := srv.getSharedResource(account.Organizations[i], rbacpb.SubjectType_ORGANIZATION)
			if err == nil {
				for j := 0; j < len(share__); j++ {
					// test if the share already exist in the path
					contain := false
					for k := 0; k < len(share_); k++ {
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
		group, err := srv.getGroup(subject)
		if err != nil {
			return nil, err
		}

		for i := range group.Organizations {
			share__, err := srv.getSharedResource(group.Organizations[i], rbacpb.SubjectType_ORGANIZATION)
			if err == nil {
				for j := range share__ {
					// test if the share already exist in the path
					contain := false
					for k := range share_ {
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

// GetSharedResource retrieves all shared resources for a given subject.
// It optionally filters the shared resources by owner if the Owner field in the request is provided.
// The function checks if the owner is present in the Accounts, Groups, Organizations, or Applications
// of each shared resource and applies ownership logic accordingly.
// Returns a response containing the filtered shared resources or an error if retrieval fails.
func (srv *server) GetSharedResource(ctx context.Context, rqst *rbacpb.GetSharedResourceRqst) (*rbacpb.GetSharedResourceRsp, error) {

	// retreive all shared resource for a given subject.
	share, err := srv.getSharedResource(rqst.Subject, rqst.Type)

	if err != nil {
		return nil, status.Error(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	if len(rqst.Owner) > 0 {
		// logPrintln("get resource share with: ", rqst.Owner)
		share_ := make([]*rbacpb.Share, 0)
		for i := range share {
			path := share[i].Path

			if Utility.Contains(share[i].Accounts, rqst.Owner) {
				if srv.isOwner(rqst.Owner, rbacpb.SubjectType_ACCOUNT, path) {
					share_ = append(share_, share[i])
				}
			} else if Utility.Contains(share[i].Groups, rqst.Owner) {

				// no set file from group where I'm the owner...
				if !srv.isOwner(rqst.Subject, rbacpb.SubjectType_ACCOUNT, path) {
					share_ = append(share_, share[i])
				}

			} else if Utility.Contains(share[i].Organizations, rqst.Owner) {

				// no set file from group where I'm the owner...
				if !srv.isOwner(rqst.Subject, rbacpb.SubjectType_ACCOUNT, path) {
					share_ = append(share_, share[i])
				}

			} else if Utility.Contains(share[i].Applications, rqst.Owner) {

				// no set file from group where I'm the owner...
				if !srv.isOwner(rqst.Subject, rbacpb.SubjectType_ACCOUNT, path) {
					share_ = append(share_, share[i])
				}
			}
		}

		share = share_
	}

	return &rbacpb.GetSharedResourceRsp{SharedResource: share}, nil
}

func (srv *server) removeSubjectFromShare(subject string, subjectType rbacpb.SubjectType, resourceId string) error {

	data, err := srv.getItem(resourceId)
	if err != nil {
		return err
	}

	share := new(rbacpb.Share)
	err = protojson.Unmarshal(data, share)
	if err != nil {
		return err
	}

	// if no permission exist at the path I will return an error.
	permissions, err := srv.getResourcePermissions(share.Path)
	if err != nil {
		return err
	}

	// Remove the subject from the share...
	switch subjectType {
	case rbacpb.SubjectType_ACCOUNT:
		share.Accounts = Utility.RemoveString(share.Accounts, subject)
		exist, a := srv.accountExist(subject)
		if exist {
			share.Accounts = Utility.RemoveString(share.Accounts, a)
		}

		// remove the permission.
		for i := range permissions.Allowed {
			permissions.Allowed[i].Accounts = Utility.RemoveString(permissions.Allowed[i].Accounts, subject)
		}

		for i := range permissions.Denied {
			permissions.Denied[i].Accounts = Utility.RemoveString(permissions.Denied[i].Accounts, subject)
		}

	case rbacpb.SubjectType_APPLICATION:
		share.Applications = Utility.RemoveString(share.Applications, subject)
		exist, a := srv.applicationExist(subject)
		if !exist {
			share.Applications = Utility.RemoveString(share.Applications, a)
		}

		for i := range permissions.Allowed {
			permissions.Allowed[i].Applications = Utility.RemoveString(permissions.Allowed[i].Applications, subject)
		}

		for i := range permissions.Denied {
			permissions.Denied[i].Applications = Utility.RemoveString(permissions.Denied[i].Applications, subject)
		}

	case rbacpb.SubjectType_GROUP:

		share.Groups = Utility.RemoveString(share.Groups, subject)
		exist, g := srv.groupExist(subject)
		if exist {
			share.Groups = Utility.RemoveString(share.Groups, g)
		}

		for i := range permissions.Allowed {
			permissions.Allowed[i].Groups = Utility.RemoveString(permissions.Allowed[i].Groups, subject)
		}

		for i := range permissions.Denied {
			permissions.Denied[i].Groups = Utility.RemoveString(permissions.Denied[i].Groups, subject)
		}

	case rbacpb.SubjectType_PEER:
		share.Peers = Utility.RemoveString(share.Peers, subject)
		for i := range permissions.Allowed {
			permissions.Allowed[i].Peers = Utility.RemoveString(permissions.Allowed[i].Peers, subject)
		}

		for i := range permissions.Denied {
			permissions.Denied[i].Peers = Utility.RemoveString(permissions.Denied[i].Peers, subject)
		}

	case rbacpb.SubjectType_ORGANIZATION:
		share.Organizations = Utility.RemoveString(share.Organizations, subject)
		exist, o := srv.organizationExist(subject)
		if exist {
			share.Organizations = Utility.RemoveString(share.Organizations, o)
		}

		for i := range permissions.Allowed {
			permissions.Allowed[i].Organizations = Utility.RemoveString(permissions.Allowed[i].Organizations, subject)
		}

		for i := range permissions.Denied {
			permissions.Denied[i].Organizations = Utility.RemoveString(permissions.Denied[i].Organizations, subject)
		}
	}

	err = srv.shareResource(share)
	if err != nil {
		return err
	}

	// save the permissions.
	data_, err := json.Marshal(permissions)
	if err != nil {
		return err
	}

	err = srv.setItem(share.Path, data_)
	if err != nil {
		return err
	}

	srv.cache.RemoveItem(share.Path) // remove from the cache

	return nil
}

// RemoveSubjectFromShare removes a subject from a specified share within the RBAC system.
// The share is identified by a combination of domain and path, which is used to generate a unique UUID.
// It takes a RemoveSubjectFromShareRqst containing the subject, type, domain, and path, and returns
// a RemoveSubjectFromShareRsp on success or an error if the operation fails.
//
// Parameters:
//   ctx - The context for the request, used for cancellation and deadlines.
//   rqst - The request containing the subject, type, domain, and path information.
//
// Returns:
//   *rbacpb.RemoveSubjectFromShareRsp - The response indicating successful removal.
//   error - An error if the removal fails.
func (srv *server) RemoveSubjectFromShare(ctx context.Context, rqst *rbacpb.RemoveSubjectFromShareRqst) (*rbacpb.RemoveSubjectFromShareRsp, error) {

	// Here I will get the share and remove the subject from it.
	// the id will be compose of the domain @ path ex. domain@/usr/toto/titi
	uuid := Utility.GenerateUUID(rqst.Domain + rqst.Path)

	err := srv.removeSubjectFromShare(rqst.Subject, rqst.Type, uuid)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &rbacpb.RemoveSubjectFromShareRsp{}, nil
}

func (srv *server) deleteSubjectShare(subject string, subjectType rbacpb.SubjectType) error {
	// logPrintln("deleteSubjectShare")
	// First of all I will get the list of share the subject is part of.
	id := "SHARED/"

	switch subjectType {
	case rbacpb.SubjectType_ACCOUNT:
		id += "ACCOUNTS"
		exist, a := srv.accountExist(subject)
		if !exist {
			return errors.New("no account exist with id " + a)
		}
		id += "/" + a
	case rbacpb.SubjectType_APPLICATION:
		id += "APPLICATIONS"
		exist, a := srv.applicationExist(subject)
		if !exist {
			return errors.New("no application exist with id " + a)
		}
		id += "/" + a
	case rbacpb.SubjectType_GROUP:
		id += "GROUPS"
		exist, g := srv.groupExist(subject)
		if !exist {
			return errors.New("no group exist with id " + g)
		}
		id += "/" + g
	case rbacpb.SubjectType_PEER:
		id += "PEERS/" + subject
	case rbacpb.SubjectType_ORGANIZATION:
		id += "ORGANIZATIONS"
		exist, o := srv.organizationExist(subject)
		if !exist {
			return errors.New("no organization exist with id " + o)
		}
		id += "/" + o
	}

	// id += "/" + subject

	// Now I will retreive the list of existing path.
	shared := make([]string, 0)
	data, err := srv.getItem(id)
	if err == nil {
		err := json.Unmarshal(data, &shared)
		if err != nil {
			return err
		}
	}

	// So now I go the list of shared uuid.
	for i := range shared {
		err := srv.removeSubjectFromShare(subject, subjectType, shared[i])
		if err != nil {
			return err
		}
	}

	// And finaly I will remove the entry with the id...
	err = srv.removeItem(id)
	if err != nil {
		return err
	}

	return nil
}

// DeleteSubjectShare handles the deletion of a subject share based on the provided request.
// It calls the internal deleteSubjectShare method with the subject and type from the request.
// Returns a response indicating success or an error if the deletion fails.
//
// Parameters:
//   ctx - The context for the request, used for cancellation and deadlines.
//   rqst - The request containing the subject and type to be deleted.
//
// Returns:
//   *rbacpb.DeleteSubjectShareRsp - The response object for the deletion operation.
//   error - An error if the deletion fails, otherwise nil.
func (srv *server) DeleteSubjectShare(ctx context.Context, rqst *rbacpb.DeleteSubjectShareRqst) (*rbacpb.DeleteSubjectShareRsp, error) {

	err := srv.deleteSubjectShare(rqst.Subject, rqst.Type)

	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &rbacpb.DeleteSubjectShareRsp{}, nil
}
