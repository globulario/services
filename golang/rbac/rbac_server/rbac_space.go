// rbac_space.go: disk space accounting and quotas.

package main

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/globulario/services/golang/rbac/rbacpb"
	"github.com/globulario/services/golang/security"
	Utility "github.com/globulario/utility"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ValidateSubjectSpace checks if a subject (such as an account or service account) has sufficient available space.
// For service accounts (identified by "sa@" prefix), space validation is bypassed and always returns true.
// For other subjects, it retrieves the available space and compares it to the required space specified in the request.
// Returns a response indicating whether the subject has enough space, or an error if space retrieval fails.
func (srv *server) ValidateSubjectSpace(ctx context.Context, rqst *rbacpb.ValidateSubjectSpaceRqst) (*rbacpb.ValidateSubjectSpaceRsp, error) {

	// in case of sa not space validation must be done...
	if rqst.Type == rbacpb.SubjectType_ACCOUNT {
		exist, a := srv.accountExist(rqst.Subject)
		if exist {
			if strings.HasPrefix(a, "sa@") {
				return &rbacpb.ValidateSubjectSpaceRsp{HasSpace: true}, nil
			}
		}
	}

	available_space, err := srv.getSubjectAvailableSpace(rqst.Subject, rqst.Type)
	if err != nil {
		return &rbacpb.ValidateSubjectSpaceRsp{HasSpace: false}, status.Errorf(
			codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &rbacpb.ValidateSubjectSpaceRsp{HasSpace: available_space > rqst.RequiredSpace}, nil

}

func (srv *server) getSubjectAllocatedSpace(subject string, subject_type rbacpb.SubjectType) (uint64, error) {

	id := "ALLOCATED_SPACE/"
	switch subject_type {
	case rbacpb.SubjectType_ACCOUNT:
		exist, a := srv.accountExist(subject)
		if !exist {
			return 0, errors.New("no account exist with id " + a)
		}
		id += "ACCOUNT/" + a

	case rbacpb.SubjectType_APPLICATION:
		exist, a := srv.applicationExist(subject)
		if !exist {
			return 0, errors.New("no application exist with id " + a)
		}
		id += "APPLICATION/" + a
	case rbacpb.SubjectType_GROUP:
		exist, g := srv.groupExist(subject)
		if !exist {
			return 0, errors.New("no group exist with id " + g)
		}
		id += "GROUP/" + g
	case rbacpb.SubjectType_ORGANIZATION:
		exist, o := srv.organizationExist(subject)
		if !exist {
			return 0, errors.New("no organization exist with id " + o)
		}
		id += "ORGANIZATION/" + o
	case rbacpb.SubjectType_NODE_IDENTITY:
		if !srv.nodeIdentityExists(subject) {
			return 0, errors.New("no node identity exists with id " + subject)
		}
		id += "PEER/" + subject
	}

	data, err := srv.getItem(id)
	if err != nil {
		logPrintln("fail to get allocated space for ", subject, " with id ", id, " with error ", err)
		return 0, err
	}

	var ret uint64
	buf := bytes.NewBuffer(data)
	err = binary.Read(buf, binary.LittleEndian, &ret)
	if err != nil {
		logPrintln("fail to get allocated space for ", subject, " with id ", id, " with error ", err)
		srv.removeItem(id)
		return 0, err
	}

	return ret, nil
}

func (srv *server) getSubjectOwnedFiles(dir string) ([]string, error) {
	files := make([]string, 0)
	err := filepath.Walk(dir,
		func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if !strings.Contains(path, ".hidden") {
				files = append(files, path)
			}

			return nil
		})
	if err != nil {
		return nil, err
	}

	return files, err
}

func (srv *server) getSubjectUsedSpace(subject string, subject_type rbacpb.SubjectType) (uint64, error) {
	id := "USED_SPACE/"
	switch subject_type {
	case rbacpb.SubjectType_ACCOUNT:
		exist, a := srv.accountExist(subject)
		if !exist {
			return 0, errors.New("no account exist with id " + a)
		}
		id += "ACCOUNT/" + a
	case rbacpb.SubjectType_APPLICATION:
		exist, a := srv.applicationExist(subject)
		if !exist {
			return 0, errors.New("no application exist with id " + a)
		}
		id += "APPLICATION/" + a
	case rbacpb.SubjectType_GROUP:
		exist, g := srv.groupExist(subject)
		if !exist {
			return 0, errors.New("no group exist with id " + g)
		}
		id += "GROUP/" + g
	case rbacpb.SubjectType_ORGANIZATION:
		exist, o := srv.organizationExist(subject)
		if !exist {
			return 0, errors.New("no organization exist with id " + o)
		}
		id += "ORGANIZATION/" + o
	case rbacpb.SubjectType_NODE_IDENTITY:
		if !srv.nodeIdentityExists(subject) {
			return 0, errors.New("no node identity exists with id " + subject)
		}
		id += "PEER/" + subject
	}

	data, err := srv.getItem(id)
	if err != nil {
		return 0, err
	}

	var ret uint64
	buf := bytes.NewBuffer(data)
	err = binary.Read(buf, binary.LittleEndian, &ret)
	if err != nil {
		srv.removeItem(id)
		return 0, err
	}

	return ret, nil
}

func (srv *server) getSubjectAvailableSpace(subject string, subject_type rbacpb.SubjectType) (uint64, error) {

	used_space, err := srv.getSubjectUsedSpace(subject, subject_type)
	if err != nil {
		used_space, err = srv.initSubjectUsedSpace(subject, subject_type)
		if err != nil {
			return 0, err
		}
	}

	allocated_space, err := srv.getSubjectAllocatedSpace(subject, subject_type)
	if err != nil {
		return 0, err
	}

	available_space := int(allocated_space) - int(used_space)
	if available_space < 0 {
		return 0, errors.New("no space available for " + subject)
	}

	return allocated_space - used_space, nil

}

func (srv *server) setSubjectUsedSpace(subject string, subject_type rbacpb.SubjectType, used_space uint64) error {

	id := "USED_SPACE/"
	switch subject_type {
	case rbacpb.SubjectType_ACCOUNT:
		exist, a := srv.accountExist(subject)
		if !exist {
			err := errors.New("no account exist with id " + subject)
			logPrintln(err)
			return err
		}

		id += "ACCOUNT/" + a

	case rbacpb.SubjectType_APPLICATION:
		exist, a := srv.applicationExist(subject)
		if !exist {
			return errors.New("no application exist with id " + subject)
		}
		id += "APPLICATION/" + a
	case rbacpb.SubjectType_GROUP:
		exist, g := srv.groupExist(subject)
		if !exist {
			return errors.New("no group exist with id " + subject)
		}
		id += "GROUP/" + g
	case rbacpb.SubjectType_ORGANIZATION:
		exist, o := srv.organizationExist(subject)
		if !exist {
			return errors.New("no organization exist with id " + subject)
		}

		id += "ORGANIZATION/" + o
	case rbacpb.SubjectType_NODE_IDENTITY:
		if !srv.nodeIdentityExists(subject) {
			return errors.New("no node identity exists with id " + subject)
		}
		id += "PEER/" + subject
	}

	b := make([]byte, 8)
	binary.LittleEndian.PutUint64(b, used_space)
	err := srv.setItem(id, b)
	if err != nil {
		return err
	}

	values, err := srv.getItem("USED_SPACE")
	ids := make([]string, 0)
	if err == nil {
		json.Unmarshal(values, &ids)
	}

	if !Utility.Contains(ids, id) {
		ids = append(ids, id)
		values, err = json.Marshal(ids)
		if err == nil {
			err := srv.setItem("USED_SPACE", values)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func (srv *server) initSubjectUsedSpace(subject string, subject_type rbacpb.SubjectType) (uint64, error) {

	// So the I must get the list of owned file from the subject, and calculate the their total space...
	permissions, err := srv.getSubjectResourcePermissions(subject, "file", subject_type)
	if err != nil {
		return 0, err
	}

	// From permission I will get the list of all file owned by that user.
	owned_files := make([]string, 0)
	for i := range permissions {
		path := permissions[i].Path

		if srv.storageExists(path) {
			fi, err := srv.storageStat(path)
			if err == nil && !fi.IsDir() {
				// get the size

				switch subject_type {
				case rbacpb.SubjectType_ACCOUNT:
					exist, a := srv.accountExist(subject)
					if exist {
						if permissions[i].Owners != nil {
							if permissions[i].Owners.Accounts != nil {
								if Utility.Contains(permissions[i].Owners.Accounts, a) {
									if !Utility.Contains(owned_files, path) {
										owned_files = append(owned_files, path)
									}
								}
							}
						}
					}
				case rbacpb.SubjectType_APPLICATION:
					exist, a := srv.applicationExist(subject)
					if exist {
						if permissions[i].Owners != nil {
							if permissions[i].Owners.Accounts != nil {
								if Utility.Contains(permissions[i].Owners.Accounts, a) {
									if !Utility.Contains(owned_files, path) {
										owned_files = append(owned_files, path)
									}
								}
							}
						}
					}
				case rbacpb.SubjectType_GROUP:
					exist, g := srv.groupExist(subject)
					if exist {
						if permissions[i].Owners != nil {
							if permissions[i].Owners.Groups != nil {
								if Utility.Contains(permissions[i].Owners.Groups, g) {
									if !Utility.Contains(owned_files, path) {
										owned_files = append(owned_files, path)
									}
								}
							}
						}
					}
				case rbacpb.SubjectType_ORGANIZATION:
					exist, o := srv.organizationExist(subject)
					if exist {
						if permissions[i].Owners != nil {
							if permissions[i].Owners.Organizations != nil {
								if Utility.Contains(permissions[i].Owners.Organizations, o) {
									if !Utility.Contains(owned_files, path) {
										owned_files = append(owned_files, path)
									}
								}
							}
						}
					}
				case rbacpb.SubjectType_NODE_IDENTITY:
					if permissions[i].Owners != nil {
						if permissions[i].Owners.NodeIdentities != nil {
							if Utility.Contains(permissions[i].Owners.NodeIdentities, subject) {
								if !Utility.Contains(owned_files, path) {
									owned_files = append(owned_files, path)
								}
							}
						}
					}
				}
			} else if err == nil && fi.IsDir() {
				// In that case I will get all files contain in that directory...
				files, err := srv.getSubjectOwnedFiles(path)
				if err == nil {
					for j := 0; j < len(files); j++ {
						path := files[j]
						if !Utility.Contains(owned_files, path) {
							owned_files = append(owned_files, path)
						}
					}
				}
			}
		} else {
			logPrintln("fail to retreive file at path ", path)
		}
	}

	// Calculate used space.
	used_space := uint64(0)
	for i := range owned_files {
		path := owned_files[i]
		fi, err := srv.storageStat(path)
		if err == nil {
			if !fi.IsDir() {
				used_space += uint64(fi.Size())
			}
		} else {
			logPrintln("fail to get stat for file  ", path, "with error", err)
		}
	}

	// save the value, so no recalculation will be required.
	return used_space, srv.setSubjectUsedSpace(subject, subject_type, used_space)
}

// GetSubjectAvailableSpace retrieves the available space for a given subject and type.
// It calls the internal getSubjectAvailableSpace method and returns the result in a response message.
// If an error occurs during retrieval, it returns an appropriate gRPC error status.
//
// Parameters:
//
//	ctx - The context for the request.
//	rqst - The request containing the subject and type.
//
// Returns:
//
//	*rbacpb.GetSubjectAvailableSpaceRsp - The response containing the available space.
//	error - An error if the operation fails.
func (srv *server) GetSubjectAvailableSpace(ctx context.Context, rqst *rbacpb.GetSubjectAvailableSpaceRqst) (*rbacpb.GetSubjectAvailableSpaceRsp, error) {
	available_space, err := srv.getSubjectAvailableSpace(rqst.Subject, rqst.Type)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	return &rbacpb.GetSubjectAvailableSpaceRsp{AvailableSpace: available_space}, nil
}

// GetSubjectAllocatedSpace retrieves the allocated space for a given subject and type.
// It calls getSubjectAllocatedSpace to obtain the space allocation and returns the result
// in a GetSubjectAllocatedSpaceRsp response. If an error occurs during retrieval, it returns
// an internal gRPC error with detailed information.
//
// Parameters:
//
//	ctx - The context for the request, used for cancellation and deadlines.
//	rqst - The request containing the subject identifier and type.
//
// Returns:
//
//	*rbacpb.GetSubjectAllocatedSpaceRsp - The response containing the allocated space.
//	error - An error if the retrieval fails.
func (srv *server) GetSubjectAllocatedSpace(ctx context.Context, rqst *rbacpb.GetSubjectAllocatedSpaceRqst) (*rbacpb.GetSubjectAllocatedSpaceRsp, error) {

	allocated_space, err := srv.getSubjectAllocatedSpace(rqst.Subject, rqst.Type)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	rsp := &rbacpb.GetSubjectAllocatedSpaceRsp{AllocatedSpace: allocated_space}

	return rsp, nil
}

// SetSubjectAllocatedSpace sets the allocated space for a specified subject (account, application, group, organization, or node identity).
// Only users with admin privileges or members of the admin role are authorized to perform this operation.
// The function validates the subject type and existence, creates necessary directories for accounts,
// and updates the allocated space in the storage. Returns an error if the operation fails or if the user is unauthorized.
//
// Parameters:
//
//	ctx  - The context for the request, used for authentication and tracing.
//	rqst - The request containing the subject type, subject identifier, and the allocated space value.
//
// Returns:
//
//	*rbacpb.SetSubjectAllocatedSpaceRsp - The response object (empty on success).
//	error - An error if the operation fails or the user is not authorized.
func (srv *server) SetSubjectAllocatedSpace(ctx context.Context, rqst *rbacpb.SetSubjectAllocatedSpaceRqst) (*rbacpb.SetSubjectAllocatedSpaceRsp, error) {

	// So here only admin must be abble to set the allocated space, or members of admin role...
	// Here I will add additional validation...
	clientId, _, err := security.GetClientId(ctx)
	if err != nil {
		return nil, err
	}

	if !strings.HasPrefix(clientId, "sa@") {

		account, err := srv.getAccount(clientId)
		if err != nil {
			return nil, err
		}

		admin, err := srv.getRole("admin")
		if err != nil {
			return nil, err
		}

		if !Utility.Contains(admin.Accounts, account.Id+"@"+account.Domain) {
			return nil, errors.New(account.Id + "@" + account.Domain + " must be admin to set Allocated space.")
		}
	}

	subject_type := rqst.Type
	subject := rqst.Subject

	id := "ALLOCATED_SPACE/"
	switch subject_type {
	case rbacpb.SubjectType_ACCOUNT:
		exist, a := srv.accountExist(subject)
		if !exist {
			return nil, errors.New("no account exist with id " + subject)
		}
		id += "ACCOUNT/" + a

	case rbacpb.SubjectType_APPLICATION:
		exist, a := srv.applicationExist(subject)
		if !exist {
			return nil, errors.New("no application exist with id " + subject)
		}
		id += "APPLICATION/" + a
	case rbacpb.SubjectType_GROUP:
		exist, g := srv.groupExist(subject)
		if !exist {
			return nil, errors.New("no group exist with id " + subject)
		}
		id += "GROUP/" + g
	case rbacpb.SubjectType_ORGANIZATION:
		exist, o := srv.organizationExist(subject)
		if !exist {
			return nil, errors.New("no organization exist with id " + subject)
		}

		id += "ORGANIZATION/" + o
	case rbacpb.SubjectType_NODE_IDENTITY:
		if !srv.nodeIdentityExists(subject) {
			return nil, errors.New("no node identity exists with id " + subject)
		}
		id += "PEER/" + subject
	}

	b := make([]byte, 8)
	binary.LittleEndian.PutUint64(b, rqst.AllocatedSpace)
	err = srv.setItem(id, b)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// test if the allocated space is set...
	_, err = srv.getItem(id)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &rbacpb.SetSubjectAllocatedSpaceRsp{}, nil
}
