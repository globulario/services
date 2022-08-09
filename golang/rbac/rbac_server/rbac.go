package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"strings"

	"encoding/binary"

	"github.com/davecourtois/Utility"
	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/rbac/rbacpb"
	"github.com/golang/protobuf/jsonpb"
	"github.com/golang/protobuf/proto"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Return a resource permission.
func (rbac_server *server) getResourceTypePathIndexation(resource_type string) ([]*rbacpb.Permissions, error) {

	data, err := rbac_server.permissions.GetItem(resource_type)
	if err != nil {
		return nil, err
	}

	paths := make([]string, 0)
	err = json.Unmarshal(data, &paths)
	if err != nil {
		return nil, err
	}

	permissions := make([]*rbacpb.Permissions, 0)
	for i := 0; i < len(paths); i++ {

		p, err := rbac_server.getResourcePermissions(paths[i])
		if err == nil && p != nil {
			if p.ResourceType == resource_type {
				permissions = append(permissions, p)
			}
		} else {
			fmt.Println("path not found: ", paths[i], err)
		}
	}

	return permissions, nil
}

func (rbac_server *server) setResourceTypePathIndexation(resource_type string, path string) error {

	// fmt.Println("setSubjectResourcePermissions", path)
	// Here I will retreive the actual list of paths use by this user.
	data, err := rbac_server.permissions.GetItem(resource_type)
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
	return rbac_server.permissions.SetItem(resource_type, data)
}

func (rbac_server *server) setSubjectResourcePermissions(subject string, path string) error {

	// Here I will retreive the actual list of paths use by this user.
	data, _ := rbac_server.permissions.GetItem(subject)
	paths_ := make([]interface{}, 0)

	if data != nil {
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
	data, err := json.Marshal(paths)
	if err != nil {
		return err
	}

	err = rbac_server.permissions.SetItem(subject, data)
	if err != nil {
		return err
	}

	return nil
}

// The function return the list of permissions associtated with a given subject.
func (rbac_server *server) getSubjectResourcePermissions(subject, resource_type string, subject_type rbacpb.SubjectType) ([]*rbacpb.Permissions, error) {
	// set the key to looking for...
	id := "PERMISSIONS/"
	if subject_type == rbacpb.SubjectType_ACCOUNT {
		id += "ACCOUNTS/"
		exist, a := rbac_server.accountExist(subject)
		if exist {
			id += a
		} else {
			return nil, errors.New("no account found with id " + subject)
		}
	} else if subject_type == rbacpb.SubjectType_APPLICATION {
		id += "APPLICATIONS/"
		exist, a := rbac_server.applicationExist(subject)
		if exist {
			id += a
		} else {
			return nil, errors.New("no application found with id " + subject)
		}
	} else if subject_type == rbacpb.SubjectType_GROUP {
		id += "GROUPS/"
		exist, g := rbac_server.groupExist(subject)
		if exist {
			id += g
		} else {
			return nil, errors.New("no group found with id " + subject)
		}
	} else if subject_type == rbacpb.SubjectType_ORGANIZATION {
		id += "ORGANIZATIONS/"
		exist, o := rbac_server.groupExist(subject)
		if exist {
			id += o
		} else {
			return nil, errors.New("no organization found with id " + subject)
		}
	} else if subject_type == rbacpb.SubjectType_PEER {
		id += "PEERS/"
		id += subject
	}

	// Set the subject.
	data, err := rbac_server.permissions.GetItem(id)

	// retreive path
	permissions := make([]*rbacpb.Permissions, 0)

	if err != nil {
		fmt.Println("fail to retreive item with id ", id, "with error", err)
		return permissions, nil
	}

	paths := make([]interface{}, 0)
	if err == nil {
		err := json.Unmarshal(data, &paths)
		if err != nil {
			return nil, err
		}
	}

	for i := 0; i < len(paths); i++ {
		p, err := rbac_server.getResourcePermissions(paths[i].(string))
		if err == nil && p != nil {
			if p.ResourceType == resource_type || len(resource_type) == 0 {
				permissions = append(permissions, p)
			}
		} else {
			fmt.Println("path not found: ", paths[i], err)
		}
	}

	return permissions, nil
}

//* Validate if the subject has enought space to store a file *
func (rbac_server *server) ValidateSubjectSpace(ctx context.Context, rqst *rbacpb.ValidateSubjectSpaceRqst) (*rbacpb.ValidateSubjectSpaceRsp, error) {

	// in case of sa not space validation must be done...
	if rqst.Type == rbacpb.SubjectType_ACCOUNT {
		exist, a := rbac_server.accountExist(rqst.Subject)
		if exist {
			if strings.HasPrefix(a, "sa@") {
				return &rbacpb.ValidateSubjectSpaceRsp{HasSpace: true}, nil
			}
		}
	}

	available_space, err := rbac_server.getSubjectAvailableSpace(rqst.Subject, rqst.Type)
	if err != nil {
		return &rbacpb.ValidateSubjectSpaceRsp{HasSpace: false}, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &rbacpb.ValidateSubjectSpaceRsp{HasSpace: available_space > rqst.RequiredSpace}, nil

}

/**
 * Return the subject allocated space...
 */
func (rbac_server *server) getSubjectAllocatedSpace(subject string, subject_type rbacpb.SubjectType) (uint64, error) {
	id := "ALLOCATED_SPACE/"
	if subject_type == rbacpb.SubjectType_ACCOUNT {
		exist, a := rbac_server.accountExist(subject)
		if !exist {
			return 0, errors.New("no account exist with id " + a)
		}
		id += "ACCOUNT/" + a
	} else if subject_type == rbacpb.SubjectType_APPLICATION {
		exist, a := rbac_server.applicationExist(subject)
		if !exist {
			return 0, errors.New("no application exist with id " + a)
		}
		id += "APPLICATION/" + a
	} else if subject_type == rbacpb.SubjectType_GROUP {
		exist, g := rbac_server.groupExist(subject)
		if !exist {
			return 0, errors.New("no group exist with id " + g)
		}
		id += "GROUP/" + g
	} else if subject_type == rbacpb.SubjectType_ORGANIZATION {
		exist, o := rbac_server.organizationExist(subject)
		if !exist {
			return 0, errors.New("no organization exist with id " + o)
		}
		id += "ORGANIZATION/" + o
	} else if subject_type == rbacpb.SubjectType_PEER {
		if !rbac_server.peerExist(subject) {
			return 0, errors.New("no peer exist with id " + subject)
		}
		id += "PEER/" + subject
	}

	data, err := rbac_server.permissions.GetItem(id)
	if err != nil {
		return 0, err
	}

	var ret uint64
	buf := bytes.NewBuffer(data)
	binary.Read(buf, binary.LittleEndian, &ret)

	return ret, nil
}

// return all files contain in a given directory recursively...
func (rbac_server *server) getSubjectOwnedFiles(dir string) ([]string, error) {
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
		fmt.Println(err)
		return nil, err
	}

	return files, err
}

func (rbac_server *server) getSubjectAvailableSpace(subject string, subject_type rbacpb.SubjectType) (uint64, error) {

	// So the I must get the list of owned file from the subject, and calculate the their total space...
	permissions, err := rbac_server.getSubjectResourcePermissions(subject, "file", subject_type)
	if err != nil {
		return 0, err
	}

	// From permission I will get the list of all file owned by that user.
	owned_files := make([]string, 0)
	for i := 0; i < len(permissions); i++ {
		path := permissions[i].Path

		// so here I will retreive the local file.
		if !Utility.Exists(path) {
			if Utility.Exists(config.GetDataDir() + "/files" + path) {
				path = config.GetDataDir() + "/files" + path
			}
		}

		if Utility.Exists(path) {
			fi, err := os.Stat(path)
			if !fi.IsDir() && err == nil {
				// get the size

				if subject_type == rbacpb.SubjectType_ACCOUNT {
					exist, a := rbac_server.accountExist(subject)
					if exist {
						if Utility.Contains(permissions[i].Owners.Accounts, a) {
							if !Utility.Contains(owned_files, path) {
								owned_files = append(owned_files, path)
							}
						}
					}
				} else if subject_type == rbacpb.SubjectType_APPLICATION {
					exist, a := rbac_server.applicationExist(subject)
					if exist {
						if Utility.Contains(permissions[i].Owners.Applications, a) {
							if !Utility.Contains(owned_files, path) {
								owned_files = append(owned_files, path)
							}
						}
					}
				} else if subject_type == rbacpb.SubjectType_GROUP {
					exist, g := rbac_server.groupExist(subject)
					if exist {
						if Utility.Contains(permissions[i].Owners.Groups, g) {
							if !Utility.Contains(owned_files, path) {
								owned_files = append(owned_files, path)
							}
						}
					}
				} else if subject_type == rbacpb.SubjectType_ORGANIZATION {
					exist, o := rbac_server.groupExist(subject)
					if exist {
						if Utility.Contains(permissions[i].Owners.Organizations, o) {
							if !Utility.Contains(owned_files, path) {
								owned_files = append(owned_files, path)
							}
						}
					}
				} else if subject_type == rbacpb.SubjectType_PEER {
					if Utility.Contains(permissions[i].Owners.Peers, subject) {
						if !Utility.Contains(owned_files, path) {
							owned_files = append(owned_files, path)
						}
					}
				}
			} else if fi.IsDir() {
				// In that case I will get all files contain in that directory...
				files, err := rbac_server.getSubjectOwnedFiles(path)
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
			fmt.Println("fail to retreive file at path ", path)
		}
	}

	// Calculate used space.
	used_space := uint64(0)
	for i := 0; i < len(owned_files); i++ {
		path := owned_files[i]
		fi, err := os.Stat(path)
		if !fi.IsDir() && err == nil {
			used_space += uint64(fi.Size())
		}
	}

	allocated_space, err := rbac_server.getSubjectAllocatedSpace(subject, subject_type)
	if err != nil {
		return 0, err
	}

	available_space := allocated_space - used_space

	return available_space, nil
}

//* Return the subject available disk space *
func (rbac_server *server) GetSubjectAvailableSpace(ctx context.Context, rqst *rbacpb.GetSubjectAvailableSpaceRqst) (*rbacpb.GetSubjectAvailableSpaceRsp, error) {
	available_space, err := rbac_server.getSubjectAvailableSpace(rqst.Subject, rqst.Type)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	return &rbacpb.GetSubjectAvailableSpaceRsp{AvailableSpace: available_space}, nil
}

//* Return the subject allocated disk space *
func (rbac_server *server) GetSubjectAllocatedSpace(ctx context.Context, rqst *rbacpb.GetSubjectAllocatedSpaceRqst) (*rbacpb.GetSubjectAllocatedSpaceRsp, error) {
	allocated_space, err := rbac_server.getSubjectAllocatedSpace(rqst.Subject, rqst.Type)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	return &rbacpb.GetSubjectAllocatedSpaceRsp{AllocatedSpace: allocated_space}, nil
}

//* Set the user allocated space *
func (rbac_server *server) SetSubjectAllocatedSpace(ctx context.Context, rqst *rbacpb.SetSubjectAllocatedSpaceRqst) (*rbacpb.SetSubjectAllocatedSpaceRsp, error) {
	subject_type := rqst.Type
	subject := rqst.Subject
	id := "ALLOCATED_SPACE/"
	if subject_type == rbacpb.SubjectType_ACCOUNT {
		exist, a := rbac_server.accountExist(subject)
		if !exist {
			return nil, errors.New("no account exist with id " + a)
		}
		id += "ACCOUNT/" + a
	} else if subject_type == rbacpb.SubjectType_APPLICATION {
		exist, a := rbac_server.applicationExist(subject)
		if !exist {
			return nil, errors.New("no application exist with id " + a)
		}
		id += "APPLICATION/" + a
	} else if subject_type == rbacpb.SubjectType_GROUP {
		exist, g := rbac_server.groupExist(subject)
		if !exist {
			return nil, errors.New("no group exist with id " + g)
		}
		id += "GROUP/" + g
	} else if subject_type == rbacpb.SubjectType_ORGANIZATION {
		exist, o := rbac_server.organizationExist(subject)
		if !exist {
			return nil, errors.New("no organization exist with id " + o)
		}

		id += "ORGANIZATION/" + o
	} else if subject_type == rbacpb.SubjectType_PEER {
		if !rbac_server.peerExist(subject) {
			return nil, errors.New("no peer exist with id " + subject)
		}
		id += "PEER/" + subject
	}

	b := make([]byte, 8)
	binary.LittleEndian.PutUint64(b, rqst.AllocatedSpace)
	err := rbac_server.permissions.SetItem(id, b)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &rbacpb.SetSubjectAllocatedSpaceRsp{}, nil
}

// Save the resource permission
func (rbac_server *server) setResourcePermissions(path, resource_type string, permissions *rbacpb.Permissions) error {

	// be sure the path and the resource type are set in the permissions itself.
	permissions.Path = path
	permissions.ResourceType = resource_type

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
					exist, a := rbac_server.accountExist(allowed[i].Accounts[j])
					if exist {
						err := rbac_server.setSubjectResourcePermissions("PERMISSIONS/ACCOUNTS/"+a, path)
						if err != nil {
							return err
						}
						share.Accounts = append(share.Accounts, a)
					}
				}
			}

			// Groups
			if allowed[i].Groups != nil {
				for j := 0; j < len(allowed[i].Groups); j++ {
					exist, g := rbac_server.groupExist(allowed[i].Groups[j])
					if exist {

						err := rbac_server.setSubjectResourcePermissions("PERMISSIONS/GROUPS/"+g, path)
						if err != nil {
							return err
						}
						share.Groups = append(share.Groups, g)
					}
				}
			}

			// Organizations
			if allowed[i].Organizations != nil {
				for j := 0; j < len(allowed[i].Organizations); j++ {
					exist, o := rbac_server.organizationExist(allowed[i].Organizations[j])
					if exist {
						err := rbac_server.setSubjectResourcePermissions("PERMISSIONS/ORGANIZATIONS/"+o, path)
						if err != nil {
							return err
						}
						share.Organizations = append(share.Organizations, o)
					}
				}
			}

			// Applications
			if allowed[i].Applications != nil {
				for j := 0; j < len(allowed[i].Applications); j++ {
					exist, a := rbac_server.applicationExist(allowed[i].Applications[j])
					if exist {
						err := rbac_server.setSubjectResourcePermissions("PERMISSIONS/APPLICATIONS/"+a, path)
						if err != nil {
							return err
						}
						share.Applications = append(share.Applications, a)
					}
				}
			}

			// Peers
			if allowed[i].Peers != nil {
				for j := 0; j < len(allowed[i].Peers); j++ {
					if rbac_server.peerExist(allowed[i].Peers[j]) {
						err := rbac_server.setSubjectResourcePermissions("PERMISSIONS/PEERS/"+allowed[i].Peers[j], path)
						if err != nil {
							return err
						}
						share.Peers = append(share.Peers, allowed[i].Peers[j])
					}
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
					exist, a := rbac_server.accountExist(denied[i].Accounts[j])
					if exist {
						err := rbac_server.setSubjectResourcePermissions("PERMISSIONS/ACCOUNTS/"+a, path)
						if err != nil {
							return err
						}
					}
				}
			}

			// Applications
			if denied[i].Applications != nil {
				for j := 0; j < len(denied[i].Applications); j++ {
					exist, a := rbac_server.applicationExist(denied[i].Applications[j])
					if exist {
						err := rbac_server.setSubjectResourcePermissions("PERMISSIONS/APPLICATIONS/"+a, path)
						if err != nil {
							return err
						}
					}
				}
			}

			// Peers
			if denied[i].Peers != nil {
				for j := 0; j < len(denied[i].Peers); j++ {
					if rbac_server.peerExist(denied[i].Peers[j]) {
						err := rbac_server.setSubjectResourcePermissions("PERMISSIONS/PEERS/"+denied[i].Peers[j], path)
						if err != nil {
							return err
						}
					}
				}
			}

			// Groups
			if denied[i].Groups != nil {
				for j := 0; j < len(denied[i].Groups); j++ {
					exist, g := rbac_server.groupExist(denied[i].Groups[j])
					if exist {
						err := rbac_server.setSubjectResourcePermissions("PERMISSIONS/GROUPS/"+g, path)
						if err != nil {
							return err
						}
					}
				}
			}

			// Organizations
			if denied[i].Organizations != nil {
				for j := 0; j < len(denied[i].Organizations); j++ {
					exist, o := rbac_server.organizationExist(denied[i].Organizations[j])
					if exist {
						err := rbac_server.setSubjectResourcePermissions("PERMISSIONS/ORGANIZATIONS/"+o, path)
						if err != nil {
							return err
						}
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
				exist, a := rbac_server.accountExist(owners.Accounts[j])
				if exist {
					err := rbac_server.setSubjectResourcePermissions("PERMISSIONS/ACCOUNTS/"+a, path)
					if err != nil {
						return err
					}
					share.Accounts = append(share.Accounts, a)
				} else {
					fmt.Println("no account found with id ", owners.Accounts[j])
				}
			}
		}

		// Applications
		if owners.Applications != nil {
			for j := 0; j < len(owners.Applications); j++ {
				exist, a := rbac_server.applicationExist(owners.Applications[j])
				if exist {
					err := rbac_server.setSubjectResourcePermissions("PERMISSIONS/APPLICAITONS/"+a, path)
					if err != nil {
						return err
					}
					share.Applications = append(share.Applications, a)
				} else {
					fmt.Println("no application found with id ", owners.Applications[j])
				}
			}

		}

		// Peers
		if owners.Peers != nil {
			for j := 0; j < len(owners.Peers); j++ {
				if rbac_server.peerExist(owners.Peers[j]) {
					err := rbac_server.setSubjectResourcePermissions("PERMISSIONS/PEERS/"+owners.Peers[j], path)
					if err != nil {
						return err
					}
					share.Peers = append(share.Peers, owners.Peers[j])
				}
			}
		}

		// Groups
		if owners.Groups != nil {
			for j := 0; j < len(owners.Groups); j++ {
				exist, g := rbac_server.groupExist(owners.Groups[j])
				if exist {
					err := rbac_server.setSubjectResourcePermissions("PERMISSIONS/GROUPS/"+g, path)
					if err != nil {
						return err
					}
					share.Groups = append(share.Groups, g)
				} else {
					fmt.Println("no group found with id ", owners.Groups[j])
				}
			}
		}

		// Organizations
		if owners.Organizations != nil {
			for j := 0; j < len(owners.Organizations); j++ {
				exist, o := rbac_server.organizationExist(owners.Organizations[j])
				if exist {
					err := rbac_server.setSubjectResourcePermissions("PERMISSIONS/ORGANIZATIONS/"+o, path)
					if err != nil {
						return err
					}
					share.Organizations = append(share.Organizations, o)
				} else {

					fmt.Println("no organization found with id ", owners.Organizations[j])
				}
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

	if permissions.ResourceType == "file" {
		err = rbac_server.shareResource(share)
		if err != nil {
			return err
		}
	}

	err = rbac_server.setResourceTypePathIndexation(resource_type, path)
	if err != nil {
		return err
	}

	// That's the way to marshal object as evt data
	data_, err := proto.Marshal(permissions)
	if err != nil {
		return err
	}

	encoded := []byte(base64.StdEncoding.EncodeToString(data_))
	rbac_server.publish("set_resources_permissions_event", encoded)

	if err != nil {
		return err
	}

	return nil
}

//* Set resource permissions this method will replace existing permission at once *
func (rbac_server *server) SetResourcePermissions(ctx context.Context, rqst *rbacpb.SetResourcePermissionsRqst) (*rbacpb.SetResourcePermissionsRqst, error) {

	err := rbac_server.setResourcePermissions(rqst.Path, rqst.ResourceType, rqst.Permissions)

	if err != nil {
		fmt.Println("fail to set resource permission with error ", err)
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	return &rbacpb.SetResourcePermissionsRqst{}, nil
}

/**
 * Remove a resource path for a resource path.
 */
func (rbac_server *server) deleteResourceTypePathIndexation(resource_type string, path string) error {

	data, err := rbac_server.permissions.GetItem(resource_type)
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

	return rbac_server.permissions.SetItem(resource_type, data)
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

// Remouve a resource permission
func (rbac_server *server) deleteResourcePermissions(path string, permissions *rbacpb.Permissions) error {

	// Allowed resources
	allowed := permissions.Allowed
	if allowed != nil {
		for i := 0; i < len(allowed); i++ {

			// Accounts
			for j := 0; j < len(allowed[i].Accounts); j++ {
				exist, a := rbac_server.accountExist(allowed[i].Accounts[j])
				if exist {
					err := rbac_server.deleteSubjectResourcePermissions("PERMISSIONS/ACCOUNTS/"+a, path)
					if err != nil {
						fmt.Println(err)
					}
				}
			}

			// Groups
			for j := 0; j < len(allowed[i].Groups); j++ {
				exist, g := rbac_server.groupExist(allowed[i].Groups[j])
				if exist {
					err := rbac_server.deleteSubjectResourcePermissions("PERMISSIONS/GROUPS/"+g, path)
					if err != nil {
						fmt.Println(err)
					}
				}
			}

			// Organizations
			for j := 0; j < len(allowed[i].Organizations); j++ {
				exist, o := rbac_server.organizationExist(allowed[i].Organizations[j])
				if exist {
					err := rbac_server.deleteSubjectResourcePermissions("PERMISSIONS/ORGANIZATIONS/"+o, path)
					if err != nil {
						fmt.Println(err)
					}
				}
			}

			// Applications
			for j := 0; j < len(allowed[i].Applications); j++ {
				exist, a := rbac_server.applicationExist(allowed[i].Applications[j])
				if exist {
					err := rbac_server.deleteSubjectResourcePermissions("PERMISSIONS/APPLICATIONS/"+a, path)
					if err != nil {
						fmt.Println(err)
					}
				}
			}

			// Peers
			for j := 0; j < len(allowed[i].Peers); j++ {
				err := rbac_server.deleteSubjectResourcePermissions("PERMISSIONS/PEERS/"+allowed[i].Peers[j], path)
				if err != nil {
					fmt.Println(err)
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
				exist, a := rbac_server.accountExist(denied[i].Accounts[j])
				if exist {
					err := rbac_server.deleteSubjectResourcePermissions("PERMISSIONS/ACCOUNTS/"+a, path)
					if err != nil {
						fmt.Println(err)
					}
				}
			}
			// Applications
			for j := 0; j < len(denied[i].Applications); j++ {
				exist, a := rbac_server.applicationExist(denied[i].Applications[j])
				if exist {
					err := rbac_server.deleteSubjectResourcePermissions("PERMISSIONS/APPLICATIONS/"+a, path)
					if err != nil {
						fmt.Println(err)
					}
				}
			}

			// Peers
			for j := 0; j < len(denied[i].Peers); j++ {
				err := rbac_server.deleteSubjectResourcePermissions("PERMISSIONS/PEERS/"+denied[i].Peers[j], path)
				if err != nil {
					fmt.Println(err)
				}
			}

			// Groups
			for j := 0; j < len(denied[i].Groups); j++ {
				exist, g := rbac_server.groupExist(denied[i].Groups[j])
				if exist {
					err := rbac_server.deleteSubjectResourcePermissions("PERMISSIONS/GROUPS/"+g, path)
					if err != nil {
						fmt.Println(err)
					}
				}
			}

			// Organizations
			for j := 0; j < len(denied[i].Organizations); j++ {
				exist, o := rbac_server.organizationExist(denied[i].Organizations[j])
				if exist {
					err := rbac_server.deleteSubjectResourcePermissions("PERMISSIONS/ORGANIZATIONS/"+o, path)
					if err != nil {
						fmt.Println(err)
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
				exist, a := rbac_server.accountExist(owners.Accounts[j])
				if exist {
					err := rbac_server.deleteSubjectResourcePermissions("PERMISSIONS/ACCOUNTS/"+a, path)
					if err != nil {
						fmt.Println(err)
					}
				}
			}
		}

		// Applications
		if owners.Applications != nil {
			for j := 0; j < len(owners.Applications); j++ {
				exist, a := rbac_server.applicationExist(owners.Applications[j])
				if exist {
					err := rbac_server.deleteSubjectResourcePermissions("PERMISSIONS/APPLICATIONS/"+a, path)
					if err != nil {
						fmt.Println(err)
					}
				}
			}
		}

		// Peers
		if owners.Peers != nil {
			for j := 0; j < len(owners.Peers); j++ {
				err := rbac_server.deleteSubjectResourcePermissions("PERMISSIONS/PEERS/"+owners.Peers[j], path)
				if err != nil {
					fmt.Println(err)
				}
			}
		}

		// Groups
		if owners.Groups != nil {
			for j := 0; j < len(owners.Groups); j++ {
				exist, g := rbac_server.groupExist(owners.Groups[j])
				if exist {
					err := rbac_server.deleteSubjectResourcePermissions("PERMISSIONS/GROUPS/"+g, path)
					if err != nil {
						fmt.Println(err)
					}

				}
			}
		}

		// Organizations
		if owners.Organizations != nil {
			for j := 0; j < len(owners.Organizations); j++ {
				exist, o := rbac_server.organizationExist(owners.Organizations[j])
				if exist {
					err := rbac_server.deleteSubjectResourcePermissions("PERMISSIONS/ORGANIZATIONS/"+o, path)
					if err != nil {
						fmt.Println(err)
					}
				}
			}
		}
	}

	// Remove the resource type permission
	rbac_server.deleteResourceTypePathIndexation(permissions.ResourceType, path)

	// unshare the resource
	if permissions.ResourceType == "file" {
		rbac_server.unshareResource(rbac_server.Domain, path)
	}

	// Remove the path
	err := rbac_server.permissions.RemoveItem(path)
	if err != nil {
		fmt.Println("fail to remove key ", path)
	}

	data, err := proto.Marshal(permissions)
	if err != nil {
		return err
	}

	encoded := []byte(base64.StdEncoding.EncodeToString(data))
	rbac_server.publish("delete_resources_permissions_event", encoded)
	if err != nil {
		return err
	}

	return rbac_server.permissions.RemoveItem(path)
}

// test if all subject exist...
func (rbac_server *server) cleanupPermission(permission *rbacpb.Permission) (bool, *rbacpb.Permission) {
	hasChange := false

	// fmt.Println("cleanupPermission")
	// Cleanup owners from deleted subjects...
	accounts_change, accounts := rbac_server.cleanupSubjectPermissions(rbacpb.SubjectType_ACCOUNT, permission.Accounts)
	if accounts_change {
		hasChange = true
		permission.Accounts = accounts
	}

	applications_change, applications := rbac_server.cleanupSubjectPermissions(rbacpb.SubjectType_APPLICATION, permission.Applications)
	if applications_change {
		hasChange = true
		permission.Applications = applications
	}

	groups_change, groups := rbac_server.cleanupSubjectPermissions(rbacpb.SubjectType_GROUP, permission.Groups)
	if groups_change {
		hasChange = true
		permission.Groups = groups
	}

	organizations_change, organizations := rbac_server.cleanupSubjectPermissions(rbacpb.SubjectType_ORGANIZATION, permission.Organizations)
	if organizations_change {
		hasChange = true
		permission.Organizations = organizations
	}

	peers_change, peers := rbac_server.cleanupSubjectPermissions(rbacpb.SubjectType_PEER, permission.Peers)
	if peers_change {
		hasChange = true
		permission.Peers = peers
	}

	return hasChange, permission
}

// test if the permission has change...
func (rbac_server *server) cleanupPermissions(permissions *rbacpb.Permissions) (bool, *rbacpb.Permissions, error) {

	// Delete the indexation
	if permissions.ResourceType == "file" {
		deleted := false
		if strings.HasPrefix(permissions.Path, "/users/") || strings.HasPrefix(permissions.Path, "/applications/") {
			if !Utility.Exists(config.GetDataDir() + "/files" + permissions.Path) {
				rbac_server.deleteResourcePermissions(permissions.Path, permissions)
				deleted = true
			}
		} else if !Utility.Exists(permissions.Path) {
			rbac_server.deleteResourcePermissions(permissions.Path, permissions)
			deleted = true
		}

		// Now I will send deleted event...
		if deleted {
			data, err := proto.Marshal(permissions)
			if err != nil {
				return false, nil, err
			}
			encoded := []byte(base64.StdEncoding.EncodeToString(data))
			rbac_server.publish("delete_resources_permissions_event", encoded)
			return false, nil, errors.New("file does not exist " + permissions.Path)
		}
	}

	hasChange := false
	ownersChange, owners := rbac_server.cleanupPermission(permissions.Owners)
	if ownersChange {
		permissions.Owners = owners
		hasChange = true
	}

	// Allowed...
	for i := 0; i < len(permissions.Allowed); i++ {
		permissionHasChange, permission := rbac_server.cleanupPermission(permissions.Allowed[i])
		if permissionHasChange {
			permissions.Allowed[i] = permission
			hasChange = true
		}
	}

	for i := 0; i < len(permissions.Denied); i++ {
		permissionHasChange, permission := rbac_server.cleanupPermission(permissions.Denied[i])
		if permissionHasChange {
			permissions.Allowed[i] = permission
			hasChange = true
		}
	}

	return hasChange, permissions, nil
}

// Remove all deleted subject from permission.
func (rbac_server *server) cleanupSubjectPermissions(subjectType rbacpb.SubjectType, subjects []string) (bool, []string) {
	// So here I will remove subject that no more exist in the permissions and keep up to date...
	subjects_ := make([]string, 0)
	needSave := false

	if subjectType == rbacpb.SubjectType_ACCOUNT {
		for i := 0; i < len(subjects); i++ {
			exist, a := rbac_server.accountExist(subjects[i])
			if exist {
				subjects_ = append(subjects_, a)
			} else {
				needSave = true
			}
		}
	} else if subjectType == rbacpb.SubjectType_APPLICATION {
		for i := 0; i < len(subjects); i++ {
			exist, a := rbac_server.applicationExist(subjects[i])
			if exist {
				subjects_ = append(subjects_, a)
			} else {
				needSave = true
			}
		}

	} else if subjectType == rbacpb.SubjectType_GROUP {
		for i := 0; i < len(subjects); i++ {
			exist, g := rbac_server.groupExist(subjects[i])
			if exist {
				subjects_ = append(subjects_, g)
			} else {
				needSave = true
			}
		}
	} else if subjectType == rbacpb.SubjectType_ORGANIZATION {
		for i := 0; i < len(subjects); i++ {
			exist, o := rbac_server.organizationExist(subjects[i])
			if exist {
				subjects_ = append(subjects_, o)
			} else {
				needSave = true
			}
		}
	} else if subjectType == rbacpb.SubjectType_PEER {
		for i := 0; i < len(subjects); i++ {
			if rbac_server.peerExist(subjects[i]) {
				subjects_ = append(subjects_, subjects[i])
			} else {
				needSave = true
			}
		}
	}

	return needSave, subjects_
}

// Return a resource permission.
func (rbac_server *server) getResourcePermissions(path string) (*rbacpb.Permissions, error) {

	data, err := rbac_server.permissions.GetItem(path)
	if err != nil {
		return nil, err
	}

	permissions := new(rbacpb.Permissions)
	err = json.Unmarshal(data, &permissions)
	if err != nil {
		return nil, err
	}

	// remove deleted subjects
	needSave, permissions, err := rbac_server.cleanupPermissions(permissions)
	if err != nil {
		return nil, err
	}

	// save the value...
	if needSave {
		rbac_server.setResourcePermissions(path, permissions.ResourceType, permissions)
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
	err = rbac_server.setResourcePermissions(rqst.Path, permissions.ResourceType, permissions)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &rbacpb.DeleteResourcePermissionRqst{}, nil
}

//* Get the resource Permission.
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
	err = rbac_server.setResourcePermissions(rqst.Path, permissions.ResourceType, permissions)
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

func (rbac_server *server) addResourceOwner(path, resourceType_, subject string, subjectType rbacpb.SubjectType) error {

	if subjectType == rbacpb.SubjectType_ACCOUNT {
		exist, a := rbac_server.accountExist(subject)
		if !exist {
			return errors.New("no account exist with id " + a)
		}
	} else if subjectType == rbacpb.SubjectType_APPLICATION {
		exist, a := rbac_server.applicationExist(subject)
		if !exist {
			return errors.New("no application exist with id " + a)
		}

	} else if subjectType == rbacpb.SubjectType_GROUP {
		exist, g := rbac_server.groupExist(subject)
		if !exist {
			return errors.New("no group exist with id " + g)
		}

	} else if subjectType == rbacpb.SubjectType_ORGANIZATION {
		exist, o := rbac_server.organizationExist(subject)
		if !exist {
			return errors.New("no organization exist with id " + o)
		}
	} else if subjectType == rbacpb.SubjectType_PEER {
		if !rbac_server.peerExist(subject) {
			return errors.New("no peer exist with id " + subject)
		}
	}

	permissions, err := rbac_server.getResourcePermissions(path)

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
	if subjectType == rbacpb.SubjectType_ACCOUNT {
		exist, a := rbac_server.accountExist(subject)
		if exist {
			if !Utility.Contains(owners.Accounts, a) {
				owners.Accounts = append(owners.Accounts, a)
				needSave = true
			}
		} else {
			return errors.New("account with id " + subject + " donsent exit")
		}
	} else if subjectType == rbacpb.SubjectType_APPLICATION {
		exist, a := rbac_server.applicationExist(subject)
		if exist {
			if !Utility.Contains(owners.Applications, a) {
				owners.Applications = append(owners.Applications, a)
				needSave = true
			}
		} else {
			return errors.New("application with id " + subject + " donsent exit")
		}
	} else if subjectType == rbacpb.SubjectType_GROUP {
		exist, g := rbac_server.groupExist(subject)
		if exist {
			if !Utility.Contains(owners.Groups, g) {
				owners.Groups = append(owners.Groups, g)
				needSave = true
			}
		} else {
			return errors.New("group with id " + subject + " donsent exit")
		}
	} else if subjectType == rbacpb.SubjectType_ORGANIZATION {
		exist, o := rbac_server.organizationExist(subject)
		if exist {
			if !Utility.Contains(owners.Organizations, o) {
				owners.Organizations = append(owners.Organizations, o)
				needSave = true
			}
		} else {
			return errors.New("organisation with id " + subject + " donsent exit")
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
		err = rbac_server.setResourcePermissions(path, permissions.ResourceType, permissions)
		if err != nil {
			return err
		}
	}

	return nil
}

//* Add resource owner do nothing if it already exist
func (rbac_server *server) AddResourceOwner(ctx context.Context, rqst *rbacpb.AddResourceOwnerRqst) (*rbacpb.AddResourceOwnerRsp, error) {

	err := rbac_server.addResourceOwner(rqst.Path, rqst.ResourceType, rqst.Subject, rqst.Type)

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
	err = rbac_server.setResourcePermissions(path, permissions.ResourceType, permissions)
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

	err = rbac_server.setResourcePermissions(path, permissions.ResourceType, permissions)
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

	subjectId := ""
	if rqst.Type == rbacpb.SubjectType_ACCOUNT {
		exist, a := rbac_server.accountExist(rqst.Subject)
		if exist {
			subjectId = "PERMISSIONS/ACCOUNTS/" + a
		} else {
			return nil, errors.New("no account found with id " + rqst.Subject)
		}
	} else if rqst.Type == rbacpb.SubjectType_APPLICATION {
		exist, a := rbac_server.applicationExist(rqst.Subject)
		if exist {
			subjectId = "PERMISSIONS/APPLICATIONS/" + a
		} else {
			return nil, errors.New("no application found with id " + rqst.Subject)
		}
	} else if rqst.Type == rbacpb.SubjectType_GROUP {
		exist, g := rbac_server.groupExist(rqst.Subject)
		if exist {
			subjectId = "PERMISSIONS/GROUPS/" + g
		} else {
			return nil, errors.New("no group found with id " + rqst.Subject)
		}
	} else if rqst.Type == rbacpb.SubjectType_ORGANIZATION {
		exist, o := rbac_server.organizationExist(rqst.Subject)
		if exist {
			subjectId = "PERMISSIONS/ORGANIZATIONS/" + o
		} else {
			return nil, errors.New("no organization found with id " + rqst.Subject)
		}
	} else if rqst.Type == rbacpb.SubjectType_PEER {
		subjectId = "PERMISSIONS/PEERS/" + subjectId
	}

	// Here I must remove the subject from all permissions.
	data, err := rbac_server.permissions.GetItem(subjectId)
	if err != nil {
		return nil, err
	}

	paths := make([]string, 0)
	err = json.Unmarshal(data, &paths)
	if err != nil {
		return nil, err
	}

	// Remove the suject from all permissions with given paths.
	for i := 0; i < len(paths); i++ {

		// Remove from owner
		rbac_server.removeResourceOwner(rqst.Subject, rqst.Type, paths[i])

		// Remove from subject.
		rbac_server.removeResourceSubject(rqst.Subject, rqst.Type, paths[i])

		// Now I will send an update event.
		permissions, err := rbac_server.getResourcePermissions(paths[i])
		if err == nil {
			// That's the way to marshal object as evt data
			data_, _ := proto.Marshal(permissions)
			if err == nil {

				encoded := []byte(base64.StdEncoding.EncodeToString(data_))
				rbac_server.publish("set_resources_permissions_event", encoded)
			}
		}
	}

	// remove the indexation...
	err = rbac_server.permissions.RemoveItem(subjectId)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &rbacpb.DeleteAllAccessRsp{}, nil
}

// Return true if the file is found in the public path...
func isPublic(path string) bool {
	public := config.GetPublicDirs()
	path = strings.ReplaceAll(path, "\\", "/")
	if Utility.Exists(path) {
		for i := 0; i < len(public); i++ {
			if strings.HasPrefix(path, public[i]) {
				return true
			}
		}
	}
	return false
}

// Return  accessAllowed, accessDenied, error
func (rbac_server *server) validateAccess(subject string, subjectType rbacpb.SubjectType, name string, path string) (bool, bool, error) {

	if subjectType == rbacpb.SubjectType_ACCOUNT {
		exist, a := rbac_server.accountExist(subject)
		if !exist {
			return false, false, errors.New("no account exist with id " + a)
		}

		if strings.HasPrefix(a, "sa@") {
			return true, false, nil
		}

	} else if subjectType == rbacpb.SubjectType_APPLICATION {
		exist, a := rbac_server.applicationExist(subject)
		if !exist {
			return false, false, errors.New("no application exist with id " + a)
		}

	} else if subjectType == rbacpb.SubjectType_GROUP {
		exist, g := rbac_server.groupExist(subject)
		if !exist {
			return false, false, errors.New("no group exist with id " + g)
		}

	} else if subjectType == rbacpb.SubjectType_ORGANIZATION {
		exist, o := rbac_server.organizationExist(subject)
		if !exist {
			return false, false, errors.New("no organization exist with id " + o)
		}
	} else if subjectType == rbacpb.SubjectType_PEER {
		if !rbac_server.peerExist(subject) {
			return false, false, errors.New("no peer exist with id " + subject)
		}
	}

	if len(path) == 0 {
		return false, false, errors.New("no path was given to validate access for suject " + subject)
	}

	// .hidden files can be read by all... also file in public directory can be read by all...
	if strings.Contains(path, "/.hidden/") || (isPublic(path) && name == "read") {
		return true, false, nil
	}

	// first I will test if permissions is define
	fmt.Println("-----------------> try to find permission for ", path)
	permissions, err := rbac_server.getResourcePermissions(path)
	if err != nil {
		if permissions == nil {
			fmt.Println("no permission exist for ", path)
			// so here I will recursively get it parent permission...
			if strings.LastIndex(path, "/") > 0 {
				return rbac_server.validateAccess(subject, subjectType, name, path[0:strings.LastIndex(path, "/")])
			} else {
				return true, false, nil // No permission is define for that resource so I need to give access in that case.
			}
		}
		return false, false, err
	} else {
		fmt.Println("---------------------------> permissions found ", permissions)
	}

	// Test if the Subject is owner of the resource in that case I will git him access.
	owners := permissions.Owners
	subjectStr := ""
	if owners != nil {
		if subjectType == rbacpb.SubjectType_ACCOUNT {
			subjectStr = "Account"
			exist, a := rbac_server.accountExist(subject)
			if !exist {
				return false, false, errors.New("no account exist with id " + a)
			}
			if owners.Accounts != nil {
				if Utility.Contains(owners.Accounts, subject) || Utility.Contains(owners.Accounts, a) {
					return true, false, nil
				}
			} else {
				account, err := rbac_server.getAccount(subject)
				if err != nil {
					return false, false, errors.New("no account named " + subject + " exist")
				}

				if account.Groups != nil {
					for i := 0; i < len(account.Groups); i++ {
						groupId := account.Groups[i]
						isOwner, _, _ := rbac_server.validateAccess(groupId, rbacpb.SubjectType_GROUP, name, path)
						if isOwner {
							return true, false, nil
						}
					}
				}

				// from the account I will get the list of group.
				if account.Organizations != nil {
					for i := 0; i < len(account.Organizations); i++ {
						organizationId := account.Organizations[i]
						isOwner, _, _ := rbac_server.validateAccess(organizationId, rbacpb.SubjectType_ORGANIZATION, name, path)
						if isOwner {
							return true, false, nil
						}
					}
				}

			}

		} else if subjectType == rbacpb.SubjectType_APPLICATION {
			subjectStr = "Application"
			exist, a := rbac_server.applicationExist(subject)
			if !exist {
				return false, false, errors.New("no application exist with id " + a)
			}
			if owners.Applications != nil {
				if Utility.Contains(owners.Applications, subject) || Utility.Contains(owners.Applications, subject) {
					return true, false, nil
				}
			}
		} else if subjectType == rbacpb.SubjectType_GROUP {
			subjectStr = "Group"
			exist, g := rbac_server.groupExist(subject)
			if !exist {
				return false, false, errors.New("no group exist with id " + g)
			}
			if owners.Groups != nil {
				if Utility.Contains(owners.Groups, subject) || Utility.Contains(owners.Groups, g) {
					return true, false, nil
				}
			}
		} else if subjectType == rbacpb.SubjectType_ORGANIZATION {
			subjectStr = "Organization"
			exist, o := rbac_server.organizationExist(subject)
			if !exist {
				return false, false, errors.New("no organization exist with id " + o)
			}
			if owners.Organizations != nil {
				if Utility.Contains(owners.Organizations, subject) || Utility.Contains(owners.Organizations, o) {
					return true, false, nil
				}
			}
		} else if subjectType == rbacpb.SubjectType_PEER {
			subjectStr = "Peer"
			if !rbac_server.peerExist(subject) {
				return false, false, errors.New("no peer exist with id " + subject)
			}

			if owners.Peers != nil {
				if Utility.Contains(owners.Peers, subject) {
					return true, false, nil
				}
			}
		}
	}

	// if the permission is owner...
	if name == "owner" {
		return false, false, errors.New("no valid owner found for " + path)
	}

	if len(permissions.Allowed) == 0 && len(permissions.Denied) == 0 {

		// In that case I will try to get parent resource permission.
		if len(strings.Split(path, "/")) > 1 {
			parentPath := path[0:strings.LastIndex(path, "/")]
			// test for it parent.
			return rbac_server.validateAccess(subject, subjectType, name, parentPath)
		}

		// if no permission are define for a resource anyone can access it.
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
			exist, a := rbac_server.applicationExist(subject)
			if !exist {
				return false, false, errors.New("no application exist with id " + a)
			}
			if denied.Applications != nil {
				accessDenied = Utility.Contains(denied.Applications, subject) || Utility.Contains(denied.Applications, a)
			}
		} else if subjectType == rbacpb.SubjectType_GROUP {
			exist, g := rbac_server.groupExist(subject)
			if !exist {
				return false, false, errors.New("no group exist with id " + g)
			}
			// Here the Subject is a group
			if denied.Groups != nil {
				accessDenied = Utility.Contains(denied.Groups, subject) || Utility.Contains(denied.Groups, g)
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
			exist, o := rbac_server.organizationExist(subject)
			if !exist {
				return false, false, errors.New("no organization exist with id " + o)
			}
			// Here the Subject is an organizations.
			if denied.Organizations != nil {
				accessDenied = Utility.Contains(denied.Organizations, subject) || Utility.Contains(denied.Organizations, o)
			}
		} else if subjectType == rbacpb.SubjectType_PEER {
			if !rbac_server.peerExist(subject) {
				return false, false, errors.New("no peer exist with id " + subject)
			}

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
			exist, a := rbac_server.accountExist(subject)
			if !exist {
				return false, false, errors.New("no account exist with id " + a)
			}
			if allowed.Accounts != nil {
				hasAccess = Utility.Contains(allowed.Accounts, subject) || Utility.Contains(allowed.Accounts, a)
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
				exist, g := rbac_server.groupExist(subject)
				if !exist {
					return false, false, errors.New("no group exist with id " + g)
				}
				hasAccess = Utility.Contains(allowed.Groups, subject) || Utility.Contains(allowed.Groups, g)
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
			exist, o := rbac_server.organizationExist(subject)
			if !exist {
				return false, false, errors.New("no organization exist with id " + o)
			}
			if allowed.Organizations != nil {
				hasAccess = Utility.Contains(allowed.Organizations, subject) || Utility.Contains(allowed.Organizations, subject)
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
			exist, a := rbac_server.applicationExist(subject)
			if !exist {
				return false, false, errors.New("no application exist with id " + a)
			}

			if allowed.Applications != nil {
				hasAccess = Utility.Contains(allowed.Applications, subject) || Utility.Contains(allowed.Applications, a)
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

//* Validate if a account can get access to a given resource for a given operation (read, write...) That function is recursive. *
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
When gRPC service methode are called they must validate the resource pass in parameters.
So each service is reponsible to give access permissions requirement.
*/
func (rbac_server *server) setActionResourcesPermissions(permissions map[string]interface{}) error {

	// So here I will keep values in local storage.cap()
	data, err := json.Marshal(permissions["resources"])
	if err != nil {
		return err
	}

	return rbac_server.permissions.SetItem(permissions["action"].(string), data)
}

/**
 * Set Action Resource
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

// Retreive the resource infos from the database.
func (rbac_server *server) getActionResourcesPermissions(action string) ([]*rbacpb.ResourceInfos, error) {

	if len(action) == 0 {
		return nil, errors.New("no action given")
	}
	data, err := rbac_server.permissions.GetItem(action)
	infos_ := make([]*rbacpb.ResourceInfos, 0)
	if err != nil {
		if !strings.Contains(err.Error(), "item not found") {
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
// before calling ValidateAction. In that way the list of resource affected
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

	// test if the subject exist.
	if subjectType == rbacpb.SubjectType_ACCOUNT {
		exist, a := rbac_server.accountExist(subject)
		if !exist {
			return false, errors.New("no account exist with id " + a)
		}
	} else if subjectType == rbacpb.SubjectType_APPLICATION {
		exist, a := rbac_server.applicationExist(subject)
		if !exist {
			return false, errors.New("no application exist with id " + a)
		}

	} else if subjectType == rbacpb.SubjectType_GROUP {
		exist, g := rbac_server.groupExist(subject)
		if !exist {
			return false, errors.New("no group exist with id " + g)
		}

	} else if subjectType == rbacpb.SubjectType_ORGANIZATION {
		exist, o := rbac_server.organizationExist(subject)
		if !exist {
			return false, errors.New("no organization exist with id " + o)
		}
	} else if subjectType == rbacpb.SubjectType_PEER {
		if !rbac_server.peerExist(subject) {
			return false, errors.New("no peer exist with id " + subject)
		}
	}

	var actions []string
	// All action from those services are available...
	if len(resources) == 0 {
		if strings.HasPrefix(action, "/echo.EchoService") || strings.HasPrefix(action, "/resource.ResourceService") || strings.HasPrefix(action, "/event.EventService") {
			return true, nil
		}
	}

	// Validate the access for a given suject...
	hasAccess := false

	// So first of all I will validate the actions itself...
	if subjectType == rbacpb.SubjectType_APPLICATION {
		//rbac_server.logServiceInfo("", Utility.FileLine(), Utility.FunctionName(), "validate action "+action+" for application "+subject)
		application, err := rbac_server.getApplication(subject)
		if err != nil {
			rbac_server.logServiceInfo("", Utility.FileLine(), Utility.FunctionName(), "fail to retreive application "+subject+" from the resource...")
			return false, err
		}
		actions = application.Actions
	} else if subjectType == rbacpb.SubjectType_PEER {
		//rbac_server.logServiceInfo("", Utility.FileLine(), Utility.FunctionName(), "validate action "+action+" for peer "+subject)
		peer, err := rbac_server.getPeer(subject)
		if err != nil {
			rbac_server.logServiceError("", Utility.FileLine(), Utility.FunctionName(), err.Error())
			return false, err
		}
		actions = peer.Actions
	} else if subjectType == rbacpb.SubjectType_ROLE {
		//rbac_server.logServiceInfo("", Utility.FileLine(), Utility.FunctionName(), "validate action "+action+" for role "+subject)
		role, err := rbac_server.getRole(subject)

		// If the role is sa then I will it has all permission...
		domain, _ := config.GetDomain()
		if role.Domain == domain && role.Name == "admin" {
			return true, nil
		}

		if err != nil {
			rbac_server.logServiceError("", Utility.FileLine(), Utility.FunctionName(), err.Error())
			return false, err
		}
		actions = role.Actions
	} else if subjectType == rbacpb.SubjectType_ACCOUNT {
		//rbac_server.logServiceInfo("", Utility.FileLine(), Utility.FunctionName(), "validate action "+action+" for account "+subject)
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
				role, err := rbac_server.getRole(roleId)
				if err == nil {
					if role.Name == "admin"{
						return true, nil
					}

					hasAccess_, _ := rbac_server.validateAction(action, roleId, rbacpb.SubjectType_ROLE, resources)
					if hasAccess_ {
						hasAccess = hasAccess_
						break
					}
				}
			}
		}
	}

	if !hasAccess {
		if actions != nil {
			for i := 0; i < len(actions) && !hasAccess; i++ {
				if actions[i] == action {
					hasAccess = true
					break
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

					//rbac_server.logServiceInfo("", Utility.FileLine(), Utility.FunctionName(), "subject "+subject+" can call the method '"+action+"' and has permission to "+resources[i].Permission+" resource '"+resources[i].Path+"'")
					return true, nil
				}
			}
		}
	}

	//rbac_server.logServiceInfo("", Utility.FileLine(), Utility.FunctionName(), "subject "+subject+" can call the method '"+action)
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

// Set the subject share resource.
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
	// fmt.Println("shareResource")
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
		accountExist, accountId := rbac_server.accountExist(share.Accounts[i])
		if !accountExist {
			return errors.New("no account exist with id " + share.Accounts[i])
		}
		a := "SHARED/ACCOUNTS/" + accountId
		err := rbac_server.setSubjectSharedResource(a, uuid)
		if err != nil {
			return err
		}
	}

	for i := 0; i < len(share.Applications); i++ {
		applicationExist, applicationId := rbac_server.applicationExist(share.Applications[i])
		if !applicationExist {
			return errors.New("no application exist with id " + share.Applications[i])
		}
		a := "SHARED/APPLICATIONS/" + applicationId
		err := rbac_server.setSubjectSharedResource(a, uuid)
		if err != nil {
			return err
		}
	}

	for i := 0; i < len(share.Organizations); i++ {
		organizationExist, organizationId := rbac_server.organizationExist(share.Organizations[i])
		if !organizationExist {
			return errors.New("no organization exist with id " + share.Organizations[i])
		}
		o := "SHARED/ORGANIZATIONS/" + organizationId
		err := rbac_server.setSubjectSharedResource(o, uuid)
		if err != nil {
			return err
		}
	}

	for i := 0; i < len(share.Groups); i++ {
		groupExist, groupId := rbac_server.groupExist(share.Groups[i])
		if !groupExist {
			return errors.New("no group exist with id " + share.Groups[i])
		}
		g := "SHARED/GROUPS/" + groupId
		err := rbac_server.setSubjectSharedResource(g, uuid)
		if err != nil {
			return err
		}
	}

	for i := 0; i < len(share.Peers); i++ {
		if !rbac_server.peerExist(share.Peers[i]) {
			return errors.New("no peer exist with id " + share.Peers[i])
		}
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
	// fmt.Println("unshareResource")
	uuid := Utility.GenerateUUID(domain + path)

	var share *rbacpb.Share
	data, err := rbac_server.permissions.GetItem(uuid)
	if err == nil {
		share = new(rbacpb.Share)
		err := jsonpb.UnmarshalString(string(data), share)
		if err != nil {
			return err
		}
	} else {
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

// Get the list of accessible shared resource.
// TODO if account also get share for groups and organization that the acount is part of...
func (rbac_server *server) getSharedResource(subject string, subjectType rbacpb.SubjectType) ([]*rbacpb.Share, error) {

	// So here I will get the share resource for a given subject.
	id := "SHARED/"
	if subjectType == rbacpb.SubjectType_ACCOUNT {
		id += "ACCOUNTS"
		exist, a := rbac_server.accountExist(subject)
		if !exist {
			return nil, errors.New("no account exist with id " + a)
		}
		id += "/" + a
	} else if subjectType == rbacpb.SubjectType_APPLICATION {
		id += "APPLICATIONS"
		exist, a := rbac_server.applicationExist(subject)
		if !exist {
			return nil, errors.New("no application exist with id " + a)
		}
		id += "/" + a
	} else if subjectType == rbacpb.SubjectType_GROUP {
		id += "GROUPS"
		exist, g := rbac_server.groupExist(subject)
		if !exist {
			return nil, errors.New("no group exist with id " + g)
		}
		id += "/" + g
	} else if subjectType == rbacpb.SubjectType_PEER {
		id += "PEERS/" + subject
	} else if subjectType == rbacpb.SubjectType_ORGANIZATION {
		id += "ORGANIZATIONS"
		exist, o := rbac_server.organizationExist(subject)
		if !exist {
			return nil, errors.New("no organization exist with id " + o)
		}
		id += "/" + o
	}

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

		for i := 0; i < len(account.Groups); i++ {
			share__, err := rbac_server.getSharedResource(account.Groups[i], rbacpb.SubjectType_GROUP)
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

		for i := 0; i < len(account.Organizations); i++ {
			share__, err := rbac_server.getSharedResource(account.Organizations[i], rbacpb.SubjectType_ORGANIZATION)
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
		group, err := rbac_server.getGroup(subject)
		if err != nil {
			return nil, err
		}

		for i := 0; i < len(group.Organizations); i++ {
			share__, err := rbac_server.getSharedResource(group.Organizations[i], rbacpb.SubjectType_ORGANIZATION)
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

	return share_, nil
}

// Get the list of accessible shared resources.
func (rbac_server *server) GetSharedResource(ctx context.Context, rqst *rbacpb.GetSharedResourceRqst) (*rbacpb.GetSharedResourceRsp, error) {
	share, err := rbac_server.getSharedResource(rqst.Subject, rqst.Type)

	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &rbacpb.GetSharedResourceRsp{SharedResource: share}, nil
}

func (rbac_server *server) removeSubjectFromShare(subject string, subjectType rbacpb.SubjectType, resourceId string) error {

	// fmt.Println("removeSubjectFromShare")
	data, err := rbac_server.permissions.GetItem(resourceId)
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
		exist, a := rbac_server.accountExist(subject)
		if exist {
			share.Accounts = Utility.RemoveString(share.Accounts, a)
		}
	} else if subjectType == rbacpb.SubjectType_APPLICATION {
		share.Applications = Utility.RemoveString(share.Applications, subject)
		exist, a := rbac_server.applicationExist(subject)
		if !exist {
			share.Applications = Utility.RemoveString(share.Applications, a)
		}
	} else if subjectType == rbacpb.SubjectType_GROUP {
		share.Groups = Utility.RemoveString(share.Groups, subject)
		exist, g := rbac_server.groupExist(subject)
		if exist {
			share.Groups = Utility.RemoveString(share.Groups, g)
		}
	} else if subjectType == rbacpb.SubjectType_PEER {
		share.Peers = Utility.RemoveString(share.Peers, subject)
	} else if subjectType == rbacpb.SubjectType_ORGANIZATION {
		share.Organizations = Utility.RemoveString(share.Organizations, subject)
		exist, o := rbac_server.organizationExist(subject)
		if exist {
			share.Organizations = Utility.RemoveString(share.Organizations, o)
		}
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

func (rbac_server *server) deleteSubjectShare(subject string, subjectType rbacpb.SubjectType) error {
	// fmt.Println("deleteSubjectShare")
	// First of all I will get the list of share the subject is part of.
	id := "SHARED/"

	if subjectType == rbacpb.SubjectType_ACCOUNT {
		id += "ACCOUNTS"
		exist, a := rbac_server.accountExist(subject)
		if !exist {
			return errors.New("no account exist with id " + a)
		}
		id += "/" + a
	} else if subjectType == rbacpb.SubjectType_APPLICATION {
		id += "APPLICATIONS"
		exist, a := rbac_server.applicationExist(subject)
		if !exist {
			return errors.New("no application exist with id " + a)
		}
		id += "/" + a
	} else if subjectType == rbacpb.SubjectType_GROUP {
		id += "GROUPS"
		exist, g := rbac_server.groupExist(subject)
		if !exist {
			return errors.New("no group exist with id " + g)
		}
		id += "/" + g
	} else if subjectType == rbacpb.SubjectType_PEER {
		id += "PEERS/" + subject
	} else if subjectType == rbacpb.SubjectType_ORGANIZATION {
		id += "ORGANIZATIONS"
		exist, o := rbac_server.organizationExist(subject)
		if !exist {
			return errors.New("no organization exist with id " + o)
		}
		id += "/" + o
	}

	// id += "/" + subject

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

//* Get the list of all resource permission for a given resource type ex. blog or file...
func (server *server) GetResourcePermissionsByResourceType(rqst *rbacpb.GetResourcePermissionsByResourceTypeRqst, stream rbacpb.RbacService_GetResourcePermissionsByResourceTypeServer) error {
	permissions, err := server.getResourceTypePathIndexation(rqst.ResourceType)

	if err != nil {
		status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// Now I will return the list of permissions for a given permission type ex. blog, file, application, package etc.
	nb := 25 // the number of object to be send a each iteration...
	for i := 0; i < len(permissions); i += nb {
		if i+nb < len(permissions) {
			stream.Send(&rbacpb.GetResourcePermissionsByResourceTypeRsp{Permissions: permissions[i : i+nb]})
		} else {
			// send the remaining.
			stream.Send(&rbacpb.GetResourcePermissionsByResourceTypeRsp{Permissions: permissions[i:]})
		}
	}

	return nil
}

//* Return the list of permissions for a given subject. If no resource type was given all resource will be return. *
func (server *server) GetResourcePermissionsBySubject(rqst *rbacpb.GetResourcePermissionsBySubjectRqst, stream rbacpb.RbacService_GetResourcePermissionsBySubjectServer) error {

	permissions, err := server.getSubjectResourcePermissions(rqst.Subject, rqst.ResourceType, rqst.SubjectType)

	if err != nil {
		status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// Now I will return the list of permissions for a given permission type ex. blog, file, application, package etc.
	nb := 25 // the number of object to be send a each iteration...
	for i := 0; i < len(permissions); i += nb {
		if i+nb < len(permissions) {
			stream.Send(&rbacpb.GetResourcePermissionsBySubjectRsp{Permissions: permissions[i : i+nb]})
		} else {
			// send the remaining.
			stream.Send(&rbacpb.GetResourcePermissionsBySubjectRsp{Permissions: permissions[i:]})
		}
	}

	return nil
}
