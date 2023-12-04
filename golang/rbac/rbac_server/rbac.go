package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/davecourtois/Utility"
	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/rbac/rbacpb"
	"github.com/globulario/services/golang/security"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (srv *server) formatPath(path string) string {
	path, _ = url.PathUnescape(path)
	path = strings.ReplaceAll(path, "\\", "/")
	if strings.HasPrefix(path, "/") {
		if len(path) > 1 {
			if strings.HasPrefix(path, "/") {
				// Must be in the root path if it's not in public path.
				if Utility.Exists(config.GetGlobularExecPath() + path) {
					path = config.GetGlobularExecPath() + path
				} else if Utility.Exists(config.GetWebRootDir() + path) {
					path = config.GetWebRootDir() + path
				} else if strings.HasPrefix(path, "/users/") || strings.HasPrefix(path, "/applications/") {
					path = config.GetDataDir() + "/files" + path
				}

			} else {
				path = config.GetGlobularExecPath() + "/" + path
			}
		}
	}

	return path
}

// Return a resource permission.
func (srv *server) getResourceTypePathIndexation(resource_type string) ([]*rbacpb.Permissions, error) {

	data, err := srv.getItem(resource_type)
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

		p, err := srv.getResourcePermissions(paths[i])
		if err == nil && p != nil {
			if p.ResourceType == resource_type {
				permissions = append(permissions, p)
			}
		} else {
			fmt.Println("74 path not found: ", paths[i], err)
		}
	}

	return permissions, nil
}

func (srv *server) setResourceTypePathIndexation(resource_type string, path string) error {

	// fmt.Println("setSubjectResourcePermissions", path)
	// Here I will retreive the actual list of paths use by this user.
	data, err := srv.getItem(resource_type)
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
	return srv.setItem(resource_type, data)
}

func (srv *server) setSubjectResourcePermissions(subject string, path string) error {

	// Here I will retreive the actual list of paths use by this user.
	data, _ := srv.getItem(subject)
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

	err = srv.setItem(subject, data)
	if err != nil {
		return err
	}

	return nil
}

// The function return the list of permissions associtated with a given subject.
func (srv *server) getSubjectResourcePermissions(subject, resource_type string, subject_type rbacpb.SubjectType) ([]*rbacpb.Permissions, error) {

	// set the key to looking for...
	id := "PERMISSIONS/"
	if subject_type == rbacpb.SubjectType_ACCOUNT {
		id += "ACCOUNTS/"
		exist, a := srv.accountExist(subject)
		if exist {
			id += a
		} else {
			return nil, errors.New("no account found with id " + subject)
		}
	} else if subject_type == rbacpb.SubjectType_APPLICATION {
		id += "APPLICATIONS/"
		exist, a := srv.applicationExist(subject)
		if exist {
			id += a
		} else {
			return nil, errors.New("no application found with id " + subject)
		}
	} else if subject_type == rbacpb.SubjectType_GROUP {
		id += "GROUPS/"
		exist, g := srv.groupExist(subject)
		if exist {
			id += g
		} else {
			return nil, errors.New("no group found with id " + subject)
		}
	} else if subject_type == rbacpb.SubjectType_ORGANIZATION {
		id += "ORGANIZATIONS/"
		exist, o := srv.groupExist(subject)
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
	data, err := srv.getItem(id)

	// retreive path
	permissions := make([]*rbacpb.Permissions, 0)

	if err != nil {
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
		p, err := srv.getResourcePermissions(paths[i].(string))
		if err == nil && p != nil {
			if p.ResourceType == resource_type || len(resource_type) == 0 {
				permissions = append(permissions, p)
			}
		}
	}

	return permissions, nil
}

// * Validate if the subject has enought space to store a file *
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
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &rbacpb.ValidateSubjectSpaceRsp{HasSpace: available_space > rqst.RequiredSpace}, nil

}

/**
 * Return the subject allocated space...
 */
func (srv *server) getSubjectAllocatedSpace(subject string, subject_type rbacpb.SubjectType) (uint64, error) {
	id := "ALLOCATED_SPACE/"
	if subject_type == rbacpb.SubjectType_ACCOUNT {
		exist, a := srv.accountExist(subject)
		if !exist {
			return 0, errors.New("no account exist with id " + a)
		}
		id += "ACCOUNT/" + a

	} else if subject_type == rbacpb.SubjectType_APPLICATION {
		exist, a := srv.applicationExist(subject)
		if !exist {
			return 0, errors.New("no application exist with id " + a)
		}
		id += "APPLICATION/" + a
	} else if subject_type == rbacpb.SubjectType_GROUP {
		exist, g := srv.groupExist(subject)
		if !exist {
			return 0, errors.New("no group exist with id " + g)
		}
		id += "GROUP/" + g
	} else if subject_type == rbacpb.SubjectType_ORGANIZATION {
		exist, o := srv.organizationExist(subject)
		if !exist {
			return 0, errors.New("no organization exist with id " + o)
		}
		id += "ORGANIZATION/" + o
	} else if subject_type == rbacpb.SubjectType_PEER {
		if !srv.peerExist(subject) {
			return 0, errors.New("no peer exist with id " + subject)
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

// return all files contain in a given directory recursively...
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

// Keep the subject used space in store...
func (srv *server) getSubjectUsedSpace(subject string, subject_type rbacpb.SubjectType) (uint64, error) {
	id := "USED_SPACE/"
	if subject_type == rbacpb.SubjectType_ACCOUNT {
		exist, a := srv.accountExist(subject)
		if !exist {
			return 0, errors.New("no account exist with id " + a)
		}
		id += "ACCOUNT/" + a
	} else if subject_type == rbacpb.SubjectType_APPLICATION {
		exist, a := srv.applicationExist(subject)
		if !exist {
			return 0, errors.New("no application exist with id " + a)
		}
		id += "APPLICATION/" + a
	} else if subject_type == rbacpb.SubjectType_GROUP {
		exist, g := srv.groupExist(subject)
		if !exist {
			return 0, errors.New("no group exist with id " + g)
		}
		id += "GROUP/" + g
	} else if subject_type == rbacpb.SubjectType_ORGANIZATION {
		exist, o := srv.organizationExist(subject)
		if !exist {
			return 0, errors.New("no organization exist with id " + o)
		}
		id += "ORGANIZATION/" + o
	} else if subject_type == rbacpb.SubjectType_PEER {
		if !srv.peerExist(subject) {
			return 0, errors.New("no peer exist with id " + subject)
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
	if subject_type == rbacpb.SubjectType_ACCOUNT {
		exist, a := srv.accountExist(subject)
		if !exist {
			errors.New("no account exist with id " + subject)
		}
		id += "ACCOUNT/" + a

		// So Here I will create the user directory if it not already exist.
		// Create the user file directory.
		dataPath := config.GetDataDir()
		if !Utility.Exists(dataPath + "/files/users/" + a) {
			Utility.CreateDirIfNotExist(dataPath + "/files/users/" + a)

			// be sure the user is the owner of that directory...
			srv.addResourceOwner("/users/"+a, "file", a, rbacpb.SubjectType_ACCOUNT)
		}

	} else if subject_type == rbacpb.SubjectType_APPLICATION {
		exist, a := srv.applicationExist(subject)
		if !exist {
			return errors.New("no application exist with id " + subject)
		}
		id += "APPLICATION/" + a
	} else if subject_type == rbacpb.SubjectType_GROUP {
		exist, g := srv.groupExist(subject)
		if !exist {
			return errors.New("no group exist with id " + subject)
		}
		id += "GROUP/" + g
	} else if subject_type == rbacpb.SubjectType_ORGANIZATION {
		exist, o := srv.organizationExist(subject)
		if !exist {
			return errors.New("no organization exist with id " + subject)
		}

		id += "ORGANIZATION/" + o
	} else if subject_type == rbacpb.SubjectType_PEER {
		if !srv.peerExist(subject) {
			return errors.New("no peer exist with id " + subject)
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

// Init the used space value and save it in the store.
func (srv *server) initSubjectUsedSpace(subject string, subject_type rbacpb.SubjectType) (uint64, error) {

	// So the I must get the list of owned file from the subject, and calculate the their total space...
	permissions, err := srv.getSubjectResourcePermissions(subject, "file", subject_type)
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
				} else if subject_type == rbacpb.SubjectType_APPLICATION {
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
				} else if subject_type == rbacpb.SubjectType_GROUP {
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
				} else if subject_type == rbacpb.SubjectType_ORGANIZATION {
					exist, o := srv.groupExist(subject)
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
				} else if subject_type == rbacpb.SubjectType_PEER {
					if permissions[i].Owners != nil {
						if permissions[i].Owners.Peers != nil {
							if Utility.Contains(permissions[i].Owners.Peers, subject) {
								if !Utility.Contains(owned_files, path) {
									owned_files = append(owned_files, path)
								}
							}
						}
					}
				}
			} else if fi.IsDir() {
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
			fmt.Println("fail to retreive file at path ", path)
		}
	}

	// Calculate used space.
	used_space := uint64(0)
	for i := 0; i < len(owned_files); i++ {
		path := srv.formatPath(owned_files[i])
		fi, err := os.Stat(srv.formatPath(path))
		if err == nil {
			if !fi.IsDir() {
				used_space += uint64(fi.Size())
			}
		} else {
			fmt.Println("fail to get stat for file  ", path, "with error", err)
		}
	}

	// save the value, so no recalculation will be required.
	return used_space, srv.setSubjectUsedSpace(subject, subject_type, used_space)
}

// * Return the subject available disk space *
func (srv *server) GetSubjectAvailableSpace(ctx context.Context, rqst *rbacpb.GetSubjectAvailableSpaceRqst) (*rbacpb.GetSubjectAvailableSpaceRsp, error) {
	available_space, err := srv.getSubjectAvailableSpace(rqst.Subject, rqst.Type)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	return &rbacpb.GetSubjectAvailableSpaceRsp{AvailableSpace: available_space}, nil
}

// * Return the subject allocated disk space *
func (srv *server) GetSubjectAllocatedSpace(ctx context.Context, rqst *rbacpb.GetSubjectAllocatedSpaceRqst) (*rbacpb.GetSubjectAllocatedSpaceRsp, error) {

	allocated_space, err := srv.getSubjectAllocatedSpace(rqst.Subject, rqst.Type)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	return &rbacpb.GetSubjectAllocatedSpaceRsp{AllocatedSpace: allocated_space}, nil
}

// * Set the user allocated space *
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

		if !Utility.Contains(admin.Members, account.Id+"@"+account.Domain) {
			return nil, errors.New(account.Id + "@" + account.Domain + " must be admin to set Allocated space.")
		}
	}

	subject_type := rqst.Type
	subject := rqst.Subject
	id := "ALLOCATED_SPACE/"
	if subject_type == rbacpb.SubjectType_ACCOUNT {
		exist, a := srv.accountExist(subject)
		if !exist {
			return nil, errors.New("no account exist with id " + subject)
		}
		id += "ACCOUNT/" + a

		// Here I will create the account directory...
		path := config.GetDataDir() + "/files/users/" + a
		if !Utility.Exists(path) {
			Utility.CreateDirIfNotExist(path)
			err := srv.addResourceOwner("/users/"+a, "file", a, rbacpb.SubjectType_ACCOUNT)
			if err != nil {
				fmt.Println("fail to set resource owner: ", err)
			}
		}

	} else if subject_type == rbacpb.SubjectType_APPLICATION {
		exist, a := srv.applicationExist(subject)
		if !exist {
			return nil, errors.New("no application exist with id " + subject)
		}
		id += "APPLICATION/" + a
	} else if subject_type == rbacpb.SubjectType_GROUP {
		exist, g := srv.groupExist(subject)
		if !exist {
			return nil, errors.New("no group exist with id " + subject)
		}
		id += "GROUP/" + g
	} else if subject_type == rbacpb.SubjectType_ORGANIZATION {
		exist, o := srv.organizationExist(subject)
		if !exist {
			return nil, errors.New("no organization exist with id " + subject)
		}

		id += "ORGANIZATION/" + o
	} else if subject_type == rbacpb.SubjectType_PEER {
		if !srv.peerExist(subject) {
			return nil, errors.New("no peer exist with id " + subject)
		}
		id += "PEER/" + subject
	}

	b := make([]byte, 8)
	binary.LittleEndian.PutUint64(b, rqst.AllocatedSpace)
	err = srv.setItem(id, b)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &rbacpb.SetSubjectAllocatedSpaceRsp{}, nil
}

// Save the resource permission
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
	if allowed != nil {
		for i := 0; i < len(allowed); i++ {
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
					}
				}
			}

			// Groups
			if allowed[i].Groups != nil {
				for j := 0; j < len(allowed[i].Groups); j++ {
					exist, g := srv.groupExist(allowed[i].Groups[j])
					if exist {

						err := srv.setSubjectResourcePermissions("PERMISSIONS/GROUPS/"+g, path)
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
					exist, o := srv.organizationExist(allowed[i].Organizations[j])
					if exist {
						err := srv.setSubjectResourcePermissions("PERMISSIONS/ORGANIZATIONS/"+o, path)
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
					exist, a := srv.applicationExist(allowed[i].Applications[j])
					if exist {
						err := srv.setSubjectResourcePermissions("PERMISSIONS/APPLICATIONS/"+a, path)
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
					if srv.peerExist(allowed[i].Peers[j]) {
						err := srv.setSubjectResourcePermissions("PERMISSIONS/PEERS/"+allowed[i].Peers[j], path)
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
					exist, a := srv.accountExist(denied[i].Accounts[j])
					if exist {
						err := srv.setSubjectResourcePermissions("PERMISSIONS/ACCOUNTS/"+a, path)
						if err != nil {
							return err
						}
					}
				}
			}

			// Applications
			if denied[i].Applications != nil {
				for j := 0; j < len(denied[i].Applications); j++ {
					exist, a := srv.applicationExist(denied[i].Applications[j])
					if exist {
						err := srv.setSubjectResourcePermissions("PERMISSIONS/APPLICATIONS/"+a, path)
						if err != nil {
							return err
						}
					}
				}
			}

			// Peers
			if denied[i].Peers != nil {
				for j := 0; j < len(denied[i].Peers); j++ {
					if srv.peerExist(denied[i].Peers[j]) {
						err := srv.setSubjectResourcePermissions("PERMISSIONS/PEERS/"+denied[i].Peers[j], path)
						if err != nil {
							return err
						}
					}
				}
			}

			// Groups
			if denied[i].Groups != nil {
				for j := 0; j < len(denied[i].Groups); j++ {
					exist, g := srv.groupExist(denied[i].Groups[j])
					if exist {
						err := srv.setSubjectResourcePermissions("PERMISSIONS/GROUPS/"+g, path)
						if err != nil {
							return err
						}
					}
				}
			}

			// Organizations
			if denied[i].Organizations != nil {
				for j := 0; j < len(denied[i].Organizations); j++ {
					exist, o := srv.organizationExist(denied[i].Organizations[j])
					if exist {
						err := srv.setSubjectResourcePermissions("PERMISSIONS/ORGANIZATIONS/"+o, path)
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
				exist, a := srv.accountExist(owners.Accounts[j])
				if exist {
					err := srv.setSubjectResourcePermissions("PERMISSIONS/ACCOUNTS/"+a, path)
					if err != nil {
						return err
					}
					share.Accounts = append(share.Accounts, a)

					// Here I will set the used space.
					if permissions.ResourceType == "file" {
						used_space, err := srv.getSubjectUsedSpace(owners.Accounts[j], rbacpb.SubjectType_ACCOUNT)
						if err != nil {
							used_space, err = srv.initSubjectUsedSpace(owners.Accounts[j], rbacpb.SubjectType_ACCOUNT)
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
					fmt.Println("no account found with id ", owners.Accounts[j])
				}
			}
		}

		// Applications
		if owners.Applications != nil {
			for j := 0; j < len(owners.Applications); j++ {
				exist, a := srv.applicationExist(owners.Applications[j])
				if exist {
					err := srv.setSubjectResourcePermissions("PERMISSIONS/APPLICAITONS/"+a, path)
					if err != nil {
						return err
					}
					share.Applications = append(share.Applications, a)

					if permissions.ResourceType == "file" {

						used_space, err := srv.getSubjectUsedSpace(owners.Applications[j], rbacpb.SubjectType_APPLICATION)
						if err != nil {
							used_space, err = srv.initSubjectUsedSpace(owners.Applications[j], rbacpb.SubjectType_APPLICATION)
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
			for j := 0; j < len(owners.Peers); j++ {
				if srv.peerExist(owners.Peers[j]) {
					err := srv.setSubjectResourcePermissions("PERMISSIONS/PEERS/"+owners.Peers[j], path)
					if err != nil {
						return err
					}
					share.Peers = append(share.Peers, owners.Peers[j])

					if permissions.ResourceType == "file" {
						used_space, err := srv.getSubjectUsedSpace(owners.Peers[j], rbacpb.SubjectType_PEER)
						if err != nil {
							used_space, err = srv.initSubjectUsedSpace(owners.Peers[j], rbacpb.SubjectType_PEER)
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
			for j := 0; j < len(owners.Groups); j++ {
				exist, g := srv.groupExist(owners.Groups[j])
				if exist {
					err := srv.setSubjectResourcePermissions("PERMISSIONS/GROUPS/"+g, path)
					if err != nil {
						return err
					}
					share.Groups = append(share.Groups, g)

					if permissions.ResourceType == "file" {
						used_space, err := srv.getSubjectUsedSpace(owners.Groups[j], rbacpb.SubjectType_GROUP)
						if err != nil {
							used_space, err = srv.initSubjectUsedSpace(owners.Groups[j], rbacpb.SubjectType_GROUP)
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
					fmt.Println("no group found with id ", owners.Groups[j])
				}
			}
		}

		// Organizations
		if owners.Organizations != nil {
			for j := 0; j < len(owners.Organizations); j++ {
				exist, o := srv.organizationExist(owners.Organizations[j])
				if exist {
					err := srv.setSubjectResourcePermissions("PERMISSIONS/ORGANIZATIONS/"+o, path)
					if err != nil {
						return err
					}
					share.Organizations = append(share.Organizations, o)
					if permissions.ResourceType == "file" {
						used_space, err := srv.getSubjectUsedSpace(owners.Organizations[j], rbacpb.SubjectType_ORGANIZATION)
						if err != nil {
							used_space, err = srv.initSubjectUsedSpace(owners.Organizations[j], rbacpb.SubjectType_ORGANIZATION)
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

	if err != nil {
		return err
	}

	return nil
}

// * Set resource permissions this method will replace existing permission at once *
func (srv *server) SetResourcePermissions(ctx context.Context, rqst *rbacpb.SetResourcePermissionsRqst) (*rbacpb.SetResourcePermissionsRqst, error) {

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
func (srv *server) deleteResourceTypePathIndexation(resource_type string, path string) error {

	data, err := srv.getItem(resource_type)
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

	return srv.setItem(resource_type, data)
}

/**
 * Remove a resource path for an entity.
 */
func (srv *server) deleteSubjectResourcePermissions(subject string, path string) error {
	srv.cache.RemoveItem(path)
	data, err := srv.getItem(subject)
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

	return srv.setItem(subject, data)

}

// Remouve a resource permission
func (srv *server) deleteResourcePermissions(path string, permissions *rbacpb.Permissions) error {

	// simply remove it from the cache...
	defer srv.cache.RemoveItem(path)

	// Allowed resources
	allowed := permissions.Allowed
	if allowed != nil {
		for i := 0; i < len(allowed); i++ {

			// Accounts
			for j := 0; j < len(allowed[i].Accounts); j++ {
				exist, a := srv.accountExist(allowed[i].Accounts[j])
				if exist {
					err := srv.deleteSubjectResourcePermissions("PERMISSIONS/ACCOUNTS/"+a, path)
					if err != nil {
						fmt.Println(err)
					}
				}
			}

			// Groups
			for j := 0; j < len(allowed[i].Groups); j++ {
				exist, g := srv.groupExist(allowed[i].Groups[j])
				if exist {
					err := srv.deleteSubjectResourcePermissions("PERMISSIONS/GROUPS/"+g, path)
					if err != nil {
						fmt.Println(err)
					}
				}
			}

			// Organizations
			for j := 0; j < len(allowed[i].Organizations); j++ {
				exist, o := srv.organizationExist(allowed[i].Organizations[j])
				if exist {
					err := srv.deleteSubjectResourcePermissions("PERMISSIONS/ORGANIZATIONS/"+o, path)
					if err != nil {
						fmt.Println(err)
					}
				}
			}

			// Applications
			for j := 0; j < len(allowed[i].Applications); j++ {
				exist, a := srv.applicationExist(allowed[i].Applications[j])
				if exist {
					err := srv.deleteSubjectResourcePermissions("PERMISSIONS/APPLICATIONS/"+a, path)
					if err != nil {
						fmt.Println(err)
					}
				}
			}

			// Peers
			for j := 0; j < len(allowed[i].Peers); j++ {
				err := srv.deleteSubjectResourcePermissions("PERMISSIONS/PEERS/"+allowed[i].Peers[j], path)
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
				exist, a := srv.accountExist(denied[i].Accounts[j])
				if exist {
					err := srv.deleteSubjectResourcePermissions("PERMISSIONS/ACCOUNTS/"+a, path)
					if err != nil {
						fmt.Println(err)
					}
				}
			}
			// Applications
			for j := 0; j < len(denied[i].Applications); j++ {
				exist, a := srv.applicationExist(denied[i].Applications[j])
				if exist {
					err := srv.deleteSubjectResourcePermissions("PERMISSIONS/APPLICATIONS/"+a, path)
					if err != nil {
						fmt.Println(err)
					}
				}
			}

			// Peers
			for j := 0; j < len(denied[i].Peers); j++ {
				err := srv.deleteSubjectResourcePermissions("PERMISSIONS/PEERS/"+denied[i].Peers[j], path)
				if err != nil {
					fmt.Println(err)
				}
			}

			// Groups
			for j := 0; j < len(denied[i].Groups); j++ {
				exist, g := srv.groupExist(denied[i].Groups[j])
				if exist {
					err := srv.deleteSubjectResourcePermissions("PERMISSIONS/GROUPS/"+g, path)
					if err != nil {
						fmt.Println(err)
					}
				}
			}

			// Organizations
			for j := 0; j < len(denied[i].Organizations); j++ {
				exist, o := srv.organizationExist(denied[i].Organizations[j])
				if exist {
					err := srv.deleteSubjectResourcePermissions("PERMISSIONS/ORGANIZATIONS/"+o, path)
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
				exist, a := srv.accountExist(owners.Accounts[j])
				if exist {
					err := srv.deleteSubjectResourcePermissions("PERMISSIONS/ACCOUNTS/"+a, path)
					if err != nil {
						fmt.Println(err)
					}

					if permissions.ResourceType == "file" {
						used_space, err := srv.getSubjectUsedSpace(owners.Accounts[j], rbacpb.SubjectType_ACCOUNT)
						if err != nil {
							used_space, err = srv.initSubjectUsedSpace(owners.Accounts[j], rbacpb.SubjectType_ACCOUNT)
						}

						fi, err := os.Stat(srv.formatPath(path))
						if err == nil {
							if !fi.IsDir() {
								used_space -= uint64(fi.Size())
								srv.setSubjectUsedSpace(owners.Accounts[j], rbacpb.SubjectType_ACCOUNT, used_space)
							}
						} else {
							fmt.Println("no path found ", path, err)
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
						fmt.Println(err)
					}
					if permissions.ResourceType == "file" {
						used_space, err := srv.getSubjectUsedSpace(owners.Applications[j], rbacpb.SubjectType_APPLICATION)
						if err != nil {
							used_space, err = srv.initSubjectUsedSpace(owners.Applications[j], rbacpb.SubjectType_APPLICATION)
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
					fmt.Println(err)
				}
				if permissions.ResourceType == "file" {
					used_space, err := srv.getSubjectUsedSpace(owners.Peers[j], rbacpb.SubjectType_PEER)
					if err != nil {
						used_space, err = srv.initSubjectUsedSpace(owners.Peers[j], rbacpb.SubjectType_PEER)
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
						fmt.Println(err)
					}

					if permissions.ResourceType == "file" {
						used_space, err := srv.getSubjectUsedSpace(owners.Groups[j], rbacpb.SubjectType_GROUP)
						if err != nil {
							used_space, err = srv.initSubjectUsedSpace(owners.Groups[j], rbacpb.SubjectType_GROUP)
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
						fmt.Println(err)
					}
					if permissions.ResourceType == "file" {
						used_space, err := srv.getSubjectUsedSpace(owners.Organizations[j], rbacpb.SubjectType_ORGANIZATION)
						if err != nil {
							used_space, err = srv.initSubjectUsedSpace(owners.Organizations[j], rbacpb.SubjectType_ORGANIZATION)
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
		fmt.Println("fail to remove key ", path)
	}

	data, err := proto.Marshal(permissions)
	if err != nil {
		return err
	}

	encoded := []byte(base64.StdEncoding.EncodeToString(data))
	srv.publish("delete_resources_permissions_event", encoded)
	if err != nil {
		return err
	}

	return srv.removeItem(path)
}

// test if all subject exist...
func (srv *server) cleanupPermission(permission *rbacpb.Permission) (bool, *rbacpb.Permission) {
	hasChange := false
	if permission == nil {
		return false, nil
	}

	// fmt.Println("cleanupPermission")
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

// test if the permission has change...
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
	for i := 0; i < len(permissions.Allowed); i++ {
		permissionHasChange, permission := srv.cleanupPermission(permissions.Allowed[i])
		if permissionHasChange {
			permissions.Allowed[i] = permission
			hasChange = true
		}
	}

	for i := 0; i < len(permissions.Denied); i++ {
		permissionHasChange, permission := srv.cleanupPermission(permissions.Denied[i])
		if permissionHasChange {
			permissions.Allowed[i] = permission
			hasChange = true
		}
	}

	return hasChange, permissions, nil
}

// Remove all deleted subject from permission.
func (srv *server) cleanupSubjectPermissions(subjectType rbacpb.SubjectType, subjects []string) (bool, []string) {
	// So here I will remove subject that no more exist in the permissions and keep up to date...
	subjects_ := make([]string, 0)
	needSave := false

	if subjectType == rbacpb.SubjectType_ACCOUNT {
		for i := 0; i < len(subjects); i++ {
			exist, a := srv.accountExist(subjects[i])
			if exist {
				subjects_ = append(subjects_, a)
			} else {
				needSave = true
			}
		}
	} else if subjectType == rbacpb.SubjectType_APPLICATION {
		for i := 0; i < len(subjects); i++ {
			exist, a := srv.applicationExist(subjects[i])
			if exist {
				subjects_ = append(subjects_, a)
			} else {
				needSave = true
			}
		}

	} else if subjectType == rbacpb.SubjectType_GROUP {
		for i := 0; i < len(subjects); i++ {
			exist, g := srv.groupExist(subjects[i])
			if exist {
				subjects_ = append(subjects_, g)
			} else {
				needSave = true
			}
		}
	} else if subjectType == rbacpb.SubjectType_ORGANIZATION {
		for i := 0; i < len(subjects); i++ {
			exist, o := srv.organizationExist(subjects[i])
			if exist {
				subjects_ = append(subjects_, o)
			} else {
				needSave = true
			}
		}
	} else if subjectType == rbacpb.SubjectType_PEER {
		for i := 0; i < len(subjects); i++ {
			if srv.peerExist(subjects[i]) {
				subjects_ = append(subjects_, subjects[i])
			} else {
				needSave = true
			}
		}
	}

	return needSave, subjects_
}

// Return a resource permission.
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

// * Delete a resource permissions (when a resource is deleted) *
func (srv *server) DeleteResourcePermissions(ctx context.Context, rqst *rbacpb.DeleteResourcePermissionsRqst) (*rbacpb.DeleteResourcePermissionsRqst, error) {

	permissions, err := srv.getResourcePermissions(rqst.Path)
	if err != nil {
		if strings.Contains(err.Error(), "item not found") {
			return &rbacpb.DeleteResourcePermissionsRqst{}, nil
		}
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	err = srv.deleteResourcePermissions(rqst.Path, permissions)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &rbacpb.DeleteResourcePermissionsRqst{}, nil
}

// * Delete a specific resource permission *
func (srv *server) DeleteResourcePermission(ctx context.Context, rqst *rbacpb.DeleteResourcePermissionRqst) (*rbacpb.DeleteResourcePermissionRqst, error) {

	permissions, err := srv.getResourcePermissions(rqst.Path)
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
	err = srv.setResourcePermissions(rqst.Path, permissions.ResourceType, permissions)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &rbacpb.DeleteResourcePermissionRqst{}, nil
}

// * Get the resource Permission.
func (srv *server) GetResourcePermission(ctx context.Context, rqst *rbacpb.GetResourcePermissionRqst) (*rbacpb.GetResourcePermissionRsp, error) {

	permissions, err := srv.getResourcePermissions(rqst.Path)
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

// * Set specific resource permission  ex. read permission... *
func (srv *server) SetResourcePermission(ctx context.Context, rqst *rbacpb.SetResourcePermissionRqst) (*rbacpb.SetResourcePermissionRsp, error) {

	permissions, err := srv.getResourcePermissions(rqst.Path)
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
	err = srv.setResourcePermissions(rqst.Path, permissions.ResourceType, permissions)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &rbacpb.SetResourcePermissionRsp{}, nil
}

// * Get resource permissions *
func (srv *server) GetResourcePermissions(ctx context.Context, rqst *rbacpb.GetResourcePermissionsRqst) (*rbacpb.GetResourcePermissionsRsp, error) {

	permissions, err := srv.getResourcePermissions(rqst.Path)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &rbacpb.GetResourcePermissionsRsp{Permissions: permissions}, nil
}

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
	if subjectType == rbacpb.SubjectType_ACCOUNT {
		exist, a := srv.accountExist(subject)
		if exist {
			if !Utility.Contains(owners.Accounts, a) {
				owners.Accounts = append(owners.Accounts, a)
				needSave = true
			}
		} else {
			return errors.New("account with id " + subject + " donsent exit")
		}
	} else if subjectType == rbacpb.SubjectType_APPLICATION {
		exist, a := srv.applicationExist(subject)
		if exist {
			if !Utility.Contains(owners.Applications, a) {
				owners.Applications = append(owners.Applications, a)
				needSave = true
			}
		} else {
			return errors.New("application with id " + subject + " donsent exit")
		}
	} else if subjectType == rbacpb.SubjectType_GROUP {
		exist, g := srv.groupExist(subject)
		if exist {
			if !Utility.Contains(owners.Groups, g) {
				owners.Groups = append(owners.Groups, g)
				needSave = true
			}
		} else {
			return errors.New("group with id " + subject + " donsent exit")
		}
	} else if subjectType == rbacpb.SubjectType_ORGANIZATION {
		exist, o := srv.organizationExist(subject)
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

	// Save permission if it's owner has changed.
	if needSave {
		permissions.Owners = owners
		err = srv.setResourcePermissions(path, permissions.ResourceType, permissions)
		if err != nil {
			return err
		}
		fmt.Println("ressource owner was set for ", subject, path)
	}

	return nil
}

// * Add resource owner do nothing if it already exist
func (srv *server) AddResourceOwner(ctx context.Context, rqst *rbacpb.AddResourceOwnerRqst) (*rbacpb.AddResourceOwnerRsp, error) {

	err := srv.addResourceOwner(rqst.Path, rqst.ResourceType, rqst.Subject, rqst.Type)

	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
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
	err = srv.setResourcePermissions(path, permissions.ResourceType, permissions)
	if err != nil {
		return err
	}

	return nil
}

// Remove a Subject from denied list and allowed list.
func (srv *server) removeResourceSubject(subject string, subjectType rbacpb.SubjectType, path string) error {

	permissions, err := srv.getResourcePermissions(path)
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

	err = srv.setResourcePermissions(path, permissions.ResourceType, permissions)
	if err != nil {
		return err
	}

	return nil
}

// * Remove resource owner
func (srv *server) RemoveResourceOwner(ctx context.Context, rqst *rbacpb.RemoveResourceOwnerRqst) (*rbacpb.RemoveResourceOwnerRsp, error) {
	err := srv.removeResourceOwner(rqst.Subject, rqst.Type, rqst.Path)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &rbacpb.RemoveResourceOwnerRsp{}, nil
}

// * That function must be call when a subject is removed to clean up permissions.
func (srv *server) DeleteAllAccess(ctx context.Context, rqst *rbacpb.DeleteAllAccessRqst) (*rbacpb.DeleteAllAccessRsp, error) {
	subjectId := ""
	if rqst.Type == rbacpb.SubjectType_ACCOUNT {
		exist, a := srv.accountExist(rqst.Subject)
		if exist {
			subjectId = "PERMISSIONS/ACCOUNTS/" + a
		} else {
			return nil, errors.New("no account found with id " + rqst.Subject)
		}
	} else if rqst.Type == rbacpb.SubjectType_APPLICATION {
		exist, a := srv.applicationExist(rqst.Subject)
		if exist {
			subjectId = "PERMISSIONS/APPLICATIONS/" + a
		} else {
			return nil, errors.New("2167 no application found with id " + rqst.Subject)
		}
	} else if rqst.Type == rbacpb.SubjectType_GROUP {
		exist, g := srv.groupExist(rqst.Subject)
		if exist {
			subjectId = "PERMISSIONS/GROUPS/" + g
		} else {
			return nil, errors.New("no group found with id " + rqst.Subject)
		}
	} else if rqst.Type == rbacpb.SubjectType_ORGANIZATION {
		exist, o := srv.organizationExist(rqst.Subject)
		if exist {
			subjectId = "PERMISSIONS/ORGANIZATIONS/" + o
		} else {
			return nil, errors.New("no organization found with id " + rqst.Subject)
		}
	} else if rqst.Type == rbacpb.SubjectType_PEER {
		subjectId = "PERMISSIONS/PEERS/" + subjectId
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
	for i := 0; i < len(paths); i++ {

		// Remove from owner
		srv.removeResourceOwner(rqst.Subject, rqst.Type, paths[i])

		// Remove from subject.
		srv.removeResourceSubject(rqst.Subject, rqst.Type, paths[i])

		// Now I will send an update event.
		permissions, err := srv.getResourcePermissions(paths[i])
		if err == nil {
			// That's the way to marshal object as evt data
			data_, _ := proto.Marshal(permissions)
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
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &rbacpb.DeleteAllAccessRsp{}, nil
}

// Return true if the file is found in the public path...
func isPublic(path string, exact_match bool) bool {
	public := config.GetPublicDirs()
	path = strings.ReplaceAll(path, "\\", "/")
	if Utility.Exists(path) {
		for i := 0; i < len(public); i++ {
			if !exact_match {
				if strings.HasPrefix(path, public[i]) {
					return true
				}
			} else {
				if path == public[i] {
					return true
				}
			}
		}
	}
	return false
}

func (srv *server) validateSubject(subject string, subjectType rbacpb.SubjectType) (string, error) {

	// first of all I will validate if the subject exsit.
	if subjectType == rbacpb.SubjectType_ACCOUNT {

		exist, a := srv.accountExist(subject)
		if !exist {
			return "", errors.New("no account exist with id " + a)
		}

		return a, nil
	} else if subjectType == rbacpb.SubjectType_APPLICATION {
		exist, a := srv.applicationExist(subject)
		if !exist {
			return "", errors.New("no application exist with id " + a)
		}

		return a, nil

	} else if subjectType == rbacpb.SubjectType_GROUP {
		exist, g := srv.groupExist(subject)
		if !exist {
			return "", errors.New("no group exist with id " + g)
		}

		return g, nil
	} else if subjectType == rbacpb.SubjectType_ORGANIZATION {
		exist, o := srv.organizationExist(subject)
		if !exist {
			return "", errors.New("no organization exist with id " + o)
		}
		return o, nil
	} else if subjectType == rbacpb.SubjectType_ROLE {
		exist, r := srv.roleExist(subject)
		if !exist {
			return "", errors.New("no role exist with id " + r)
		}
		return r, nil
	} else if subjectType == rbacpb.SubjectType_PEER {
		if !srv.peerExist(subject) {
			return "", errors.New("no peer exist with id " + subject)
		}

		return subject, nil
	}

	return "", errors.New("no subject found with id " + subject)
}

// Return true if the subject own the resourse.
func (srv *server) isOwner(subject string, subjectType rbacpb.SubjectType, path string) bool {
	subject, err := srv.validateSubject(subject, subjectType)
	if err != nil {
		return false
	}

	// test if the subject is the direct owner of the resource.
	permissions, err := srv.getResourcePermissions(path)
	if err == nil {
		if permissions.Owners != nil {
			owners := permissions.Owners
			if owners != nil {
				hasOwner := false
				if owners.Accounts != nil {
					if len(owners.Accounts) > 0 {
						hasOwner = true
					}
				}

				if owners.Applications != nil {
					if len(owners.Applications) > 0 {
						hasOwner = true
					}
				}

				if owners.Groups != nil {
					if len(owners.Groups) > 0 {
						hasOwner = true
					}
				}

				if owners.Organizations != nil {
					if len(owners.Organizations) > 0 {
						hasOwner = true
					}
				}

				if owners.Peers != nil {
					if len(owners.Peers) > 0 {
						hasOwner = true
					}
				}

				if hasOwner {
					if subjectType == rbacpb.SubjectType_ACCOUNT {

						if owners.Accounts != nil {
							if Utility.Contains(owners.Accounts, subject) {
								return true
							}
						} else {
							account, err := srv.getAccount(subject)

							if account.Groups != nil && err == nil {
								for i := 0; i < len(account.Groups); i++ {
									groupId := account.Groups[i]
									isOwner := srv.isOwner(groupId, rbacpb.SubjectType_GROUP, path)
									if isOwner {
										return true
									}
								}
							}

							// from the account I will get the list of group.
							if account.Organizations != nil && err == nil {
								for i := 0; i < len(account.Organizations); i++ {
									organizationId := account.Organizations[i]
									isOwner := srv.isOwner(organizationId, rbacpb.SubjectType_ORGANIZATION, path)
									if isOwner {
										return true
									}
								}
							}
						}

					} else if subjectType == rbacpb.SubjectType_APPLICATION {

						exist, a := srv.applicationExist(subject)
						if owners.Applications != nil && exist {
							if Utility.Contains(owners.Applications, subject) || Utility.Contains(owners.Applications, a) {
								return true
							}
						}

					} else if subjectType == rbacpb.SubjectType_GROUP {

						exist, g := srv.groupExist(subject)
						if owners.Groups != nil && exist {
							if Utility.Contains(owners.Groups, subject) || Utility.Contains(owners.Groups, g) {
								return true
							}
						}

					} else if subjectType == rbacpb.SubjectType_ORGANIZATION {
						exist, o := srv.organizationExist(subject)
						if owners.Organizations != nil && exist {
							if Utility.Contains(owners.Organizations, subject) || Utility.Contains(owners.Organizations, o) {
								return true
							}
						}
					} else if subjectType == rbacpb.SubjectType_PEER {
						if owners.Peers != nil {
							if Utility.Contains(owners.Peers, subject) {
								return true
							}
						}
					}
				} else if strings.LastIndex(path, "/") > 0 {
					return srv.isOwner(subject, subjectType, path[0:strings.LastIndex(path, "/")])
				}

			} else if strings.LastIndex(path, "/") > 0 {
				return srv.isOwner(subject, subjectType, path[0:strings.LastIndex(path, "/")])
			}
		} else if strings.LastIndex(path, "/") > 0 {
			return srv.isOwner(subject, subjectType, path[0:strings.LastIndex(path, "/")])
		}
	} else if strings.LastIndex(path, "/") > 0 {
		return srv.isOwner(subject, subjectType, path[0:strings.LastIndex(path, "/")])
	}

	return false
}

/**
 * Validate if access to a resource at given path is denied for subject
 */
func (srv *server) validateAccessDenied(subject string, subjectType rbacpb.SubjectType, name string, path string) bool {

	subject, err := srv.validateSubject(subject, subjectType)
	if err != nil {
		return false
	}

	// test if the subject is the direct owner of the resource.
	permissions, err := srv.getResourcePermissions(path)
	if err == nil {
		if permissions.Denied != nil {
			var denied *rbacpb.Permission
			for i := 0; i < len(permissions.Denied); i++ {
				if permissions.Denied[i].Name == name {
					denied = permissions.Denied[i]
					break
				}
			}

			if denied != nil {
				if subjectType == rbacpb.SubjectType_ACCOUNT {
					if denied.Accounts != nil {
						if Utility.Contains(denied.Accounts, subject) {
							return true
						}
					} else {
						account, err := srv.getAccount(subject)

						if account.Groups != nil && err == nil {
							for i := 0; i < len(account.Groups); i++ {
								groupId := account.Groups[i]
								isDenied := srv.validateAccessDenied(groupId, rbacpb.SubjectType_GROUP, name, path)
								if isDenied {
									return true
								}
							}
						}

						// from the account I will get the list of group.
						if account.Organizations != nil && err == nil {
							for i := 0; i < len(account.Organizations); i++ {
								organizationId := account.Organizations[i]
								isDenied := srv.validateAccessDenied(organizationId, rbacpb.SubjectType_ORGANIZATION, name, path)
								if isDenied {
									return true
								}
							}
						}
					}

				} else if subjectType == rbacpb.SubjectType_APPLICATION {

					exist, a := srv.applicationExist(subject)
					if denied.Applications != nil && exist {
						if Utility.Contains(denied.Applications, subject) || Utility.Contains(denied.Applications, a) {
							return true
						}
					}

				} else if subjectType == rbacpb.SubjectType_GROUP {

					exist, g := srv.groupExist(subject)
					if denied.Groups != nil && exist {
						if Utility.Contains(denied.Groups, subject) || Utility.Contains(denied.Groups, g) {
							return true
						}
					}

				} else if subjectType == rbacpb.SubjectType_ORGANIZATION {
					exist, o := srv.organizationExist(subject)
					if denied.Organizations != nil && exist {
						if Utility.Contains(denied.Organizations, subject) || Utility.Contains(denied.Organizations, o) {
							return true
						}
					}
				} else if subjectType == rbacpb.SubjectType_PEER {
					if denied.Peers != nil {
						if Utility.Contains(denied.Peers, subject) {
							return true
						}
					}
				}
			} else if strings.LastIndex(path, "/") > 0 {
				return srv.validateAccessDenied(subject, subjectType, name, path[0:strings.LastIndex(path, "/")])
			}
		} else if strings.LastIndex(path, "/") > 0 {
			return srv.validateAccessDenied(subject, subjectType, name, path[0:strings.LastIndex(path, "/")])
		}
	} else if strings.LastIndex(path, "/") > 0 {
		return srv.validateAccessDenied(subject, subjectType, name, path[0:strings.LastIndex(path, "/")])
	}

	return false
}

func (srv *server) validateAccessAllowed(subject string, subjectType rbacpb.SubjectType, name string, path string) bool {
	subject, err := srv.validateSubject(subject, subjectType)
	if err != nil {
		return false
	}

	// test if the subject is the direct owner of the resource.
	permissions, err := srv.getResourcePermissions(path)

	if err == nil {
		if permissions.Allowed != nil {
			var allowed *rbacpb.Permission
			for i := 0; i < len(permissions.Allowed); i++ {
				if permissions.Allowed[i].Name == name {
					allowed = permissions.Allowed[i]
					break
				}
			}

			if allowed != nil {
				if subjectType == rbacpb.SubjectType_ACCOUNT {
					if allowed.Accounts != nil {
						if len(allowed.Accounts) > 0 {
							if Utility.Contains(allowed.Accounts, subject) {
								return true
							}
						}
					} else {
						// So here I will validate over it groups.
						account, err := srv.getAccount(subject)
						if err == nil {
							if account.Groups != nil && err == nil {
								for i := 0; i < len(account.Groups); i++ {
									groupId := account.Groups[i]
									if !strings.Contains(groupId, "@") {
										groupId = groupId + "@" + srv.Domain
									}
									isAllowed := srv.validateAccessAllowed(groupId, rbacpb.SubjectType_GROUP, name, path)
									if isAllowed {
										return true
									}
								}
							}

							// from the account I will get the list of group.
							if account.Organizations != nil && err == nil {
								for i := 0; i < len(account.Organizations); i++ {
									organizationId := account.Organizations[i]
									if !strings.Contains(organizationId, "@") {
										organizationId = organizationId + "@" + srv.Domain
									}

									isAllowed := srv.validateAccessAllowed(organizationId, rbacpb.SubjectType_ORGANIZATION, name, path)
									if isAllowed {
										return true
									}
								}
							}

							// Validate external account with local groups.
							groups, err := srv.getGroups()
							if err == nil {
								for i := 0; i < len(groups); i++ {
									if Utility.Contains(groups[i].Members, subject) {
										id := groups[i].Id + "@" + groups[i].Domain
										// if the role id is local admin
										isAllowed := srv.validateAccessAllowed(id, rbacpb.SubjectType_GROUP, name, path)
										if isAllowed {
											return true
										}
									}
								}
							}

							organizations, err := srv.getOrganizations()
							if err == nil {
								for i := 0; i < len(organizations); i++ {
									if Utility.Contains(organizations[i].Accounts, subject) {
										id := organizations[i].Id + "@" + organizations[i].Domain
										// if the role id is local admin
										isAllowed := srv.validateAccessAllowed(id, rbacpb.SubjectType_ORGANIZATION, name, path)
										if isAllowed {
											return true
										}
									}
								}
							}

						}
					}
				} else if subjectType == rbacpb.SubjectType_APPLICATION {
					if allowed.Applications != nil {
						if Utility.Contains(allowed.Applications, subject) {
							return true
						}
					}

				} else if subjectType == rbacpb.SubjectType_GROUP {
					if Utility.Contains(allowed.Groups, subject) {
						return true
					}

				} else if subjectType == rbacpb.SubjectType_ORGANIZATION {
					if allowed.Organizations != nil {
						if Utility.Contains(allowed.Organizations, subject) {
							return true
						}
					}
				} else if subjectType == rbacpb.SubjectType_PEER {
					if allowed.Peers != nil {
						if Utility.Contains(allowed.Peers, subject) {
							return true
						}
					}
				}
			}
		}
	}

	// Now I will test parent directories permission and inherit the permission.
	if permissions == nil {

		// validate parent permission.
		if strings.LastIndex(path, "/") > 0 {
			dir := filepath.Dir(path)
			return srv.validateAccessAllowed(subject, subjectType, name, dir)
		}

		return true // no permissions exist so I will set it to true by default...
	}

	// read only access by default if no permission are set...
	if isPublic(path, false) {
		if name == "read" {
			return true
		}
		// protected public path by default.
		return false
	}

	// Permissions exist and nothing was found for so not the subject is not allowed
	return false
}

// Return  accessAllowed, accessDenied, error
func (srv *server) validateAccess(subject string, subjectType rbacpb.SubjectType, name string, path string) (bool, bool, error) {

	// validate if the subject exist
	subject, err := srv.validateSubject(subject, subjectType)
	if err != nil {
		return false, false, err
	}

	if len(path) == 0 {
		return false, false, errors.New("no path was given to validate access for suject " + subject)
	}

	// .hidden files can be read by all... also file in public directory can be read by all...
	if strings.Contains(path, "/.hidden/") {
		return true, false, nil
	}

	path_ := srv.formatPath(path)
	if strings.HasSuffix(path_, ".ts") == true {
		if Utility.Exists(filepath.Dir(path_) + "/playlist.m3u8") {
			return true, false, nil
		}
	}

	// validate ownership...
	if srv.isOwner(subject, subjectType, path) {
		return true, false, nil
	} else if name == "owner" {
		permissions, err := srv.getResourcePermissions(path)
		if err != nil {
			if strings.HasPrefix(err.Error(), "item not found") {
				return true, false, nil
			}
		} else if permissions.Owners == nil {
			// no owner so permissions are open...
			return true, false, nil
		} else if permissions.Owners != nil {
			hasOwner := false
			if len(permissions.Owners.Accounts) > 0 {
				hasOwner = true
			} else if len(permissions.Owners.Applications) > 0 {
				hasOwner = true
			} else if len(permissions.Owners.Organizations) > 0 {
				hasOwner = true
			} else if len(permissions.Owners.Groups) > 0 {
				hasOwner = true
			} else if len(permissions.Owners.Peers) > 0 {
				hasOwner = true
			}
			if !hasOwner {
				return true, false, nil
			}

		}
		// must be owner...
		return false, false, nil
	}

	// validate if subect has it access denied...
	isDenied := srv.validateAccessDenied(subject, subjectType, name, path)
	if isDenied {
		return false, isDenied, nil
	}

	// first I will test if permissions is define
	isAllowed := srv.validateAccessAllowed(subject, subjectType, name, path)
	if !isAllowed {
		fmt.Println(subject, "has not ", name, " acces to", path)
		return false, false, nil
	}

	// The user has access.
	fmt.Println(subject, "has", name, "acces to", path)
	return true, false, nil
}

// * Validate if a account can get access to a given resource for a given operation (read, write...) That function is recursive. *
func (srv *server) ValidateAccess(ctx context.Context, rqst *rbacpb.ValidateAccessRqst) (*rbacpb.ValidateAccessRsp, error) {
	// Here I will get information from context.
	hasAccess, accessDenied, err := srv.validateAccess(rqst.Subject, rqst.Type, rqst.Permission, rqst.Path)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// The permission is set.
	return &rbacpb.ValidateAccessRsp{HasAccess: hasAccess, AccessDenied: accessDenied}, nil
}

/*
* Set action permissions.
When gRPC service methode are called they must validate the resource pass in parameters.
So each service is reponsible to give access permissions requirement.
*/
func (srv *server) setActionResourcesPermissions(permissions map[string]interface{}) error {

	// So here I will keep values in local storage.cap()
	data, err := json.Marshal(permissions["resources"])
	if err != nil {
		return err
	}

	return srv.setItem(permissions["action"].(string), data)
}

/**
 * Set Action Resource
 */
func (srv *server) SetActionResourcesPermissions(ctx context.Context, rqst *rbacpb.SetActionResourcesPermissionsRqst) (*rbacpb.SetActionResourcesPermissionsRsp, error) {

	err := srv.setActionResourcesPermissions(rqst.Permissions.AsMap())
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &rbacpb.SetActionResourcesPermissionsRsp{}, nil
}

// Retreive the resource infos from the database.
func (srv *server) getActionResourcesPermissions(action string) ([]*rbacpb.ResourceInfos, error) {

	if len(action) == 0 {
		return nil, errors.New("no action given")
	}
	data, err := srv.getItem(action)
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
		field := ""
		if info["field"] != nil {
			field = info["field"].(string)
		}
		infos_ = append(infos_, &rbacpb.ResourceInfos{Index: int32(Utility.ToInt(info["index"])), Permission: info["permission"].(string), Field: field})
	}

	return infos_, err
}

// * Return the action resource informations. That function must be called
// before calling ValidateAction. In that way the list of resource affected
// by the rpc method will be given and resource access validated.
// ex. CopyFile(src, dest) -> src and dest are resource path and must be validated
// for read and write access respectivly.
func (srv *server) GetActionResourceInfos(ctx context.Context, rqst *rbacpb.GetActionResourceInfosRqst) (*rbacpb.GetActionResourceInfosRsp, error) {
	infos, err := srv.getActionResourcesPermissions(rqst.Action)
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
func (srv *server) validateAction(action string, subject string, subjectType rbacpb.SubjectType, resources []*rbacpb.ResourceInfos) (bool, bool, error) {

	fmt.Println("validate action ", action, " for subject ", subject, " of type ", subjectType.String(), " with resources ", resources)
	// Exception
	if len(resources) == 0 {
		if strings.HasPrefix(action, "/echo.EchoService") ||
			strings.HasPrefix(action, "/resource.ResourceService") ||
			strings.HasPrefix(action, "/event.EventService") ||
			action == "/file.FileService/GetFileInfo" {
			return true, false, nil
		}
	}

	// Guest role.
	guest, err := srv.getRole("guest")
	if err != nil {
		fmt.Println("fail to retreive guest role with error ", err)
		return false, false, err
	}

	// Test if the guest role contain the action...
	if Utility.Contains(guest.Actions, action) && len(resources) == 0 {
		return true, false, nil
	}

	// test if the subject exist.
	subject, err = srv.validateSubject(subject, subjectType)
	if err != nil {
		return false, false, err
	}

	var actions []string

	// Validate the access for a given suject...
	hasAccess := false

	// So first of all I will validate the actions itself...
	if subjectType == rbacpb.SubjectType_APPLICATION {
		//srv.logServiceInfo("", Utility.FileLine(), Utility.FunctionName(), "validate action "+action+" for application "+subject)

		application, err := srv.getApplication(subject)
		if err != nil {

			srv.logServiceInfo("", Utility.FileLine(), Utility.FunctionName(), "fail to retreive application "+subject+" from the resource...")
			return false, false, err
		}

		actions = application.Actions

	} else if subjectType == rbacpb.SubjectType_PEER {
		//srv.logServiceInfo("", Utility.FileLine(), Utility.FunctionName(), "validate action "+action+" for peer "+subject)
		peer, err := srv.getPeer(subject)
		if err != nil {
			srv.logServiceError("", Utility.FileLine(), Utility.FunctionName(), err.Error())
			return false, false, err
		}
		actions = peer.Actions

	} else if subjectType == rbacpb.SubjectType_ROLE {
		//srv.logServiceInfo("", Utility.FileLine(), Utility.FunctionName(), "validate action "+action+" for role "+subject)
		role, err := srv.getRole(subject)
		if err != nil {
			srv.logServiceError("", Utility.FileLine(), Utility.FunctionName(), err.Error())
			return false, false, err
		}

		// If the role is sa then I will it has all permission...
		domain, _ := config.GetDomain()
		if role.Domain == domain && role.Name == "admin" {
			return true, false, nil
		}

		if err != nil {
			srv.logServiceError("", Utility.FileLine(), Utility.FunctionName(), err.Error())
			return false, false, err
		}

		actions = role.Actions

	} else if subjectType == rbacpb.SubjectType_ACCOUNT {
		//srv.logServiceInfo("", Utility.FileLine(), Utility.FunctionName(), "validate action "+action+" for account "+subject)
		// If the user is the super admin i will return true.
		if subject == "sa@"+srv.Domain {
			return true, false, nil
		}

		account, err := srv.getAccount(subject)
		if err != nil {
			srv.logServiceError("", Utility.FileLine(), Utility.FunctionName(), err.Error())
			return false, false, err
		}

		// call the rpc method.
		if account.Roles != nil {

			for i := 0; i < len(account.Roles); i++ {
				roleId := account.Roles[i]

				// Here I will add the domain to the role id if it's not already set.
				if !strings.Contains(roleId, "@") {
					roleId = roleId + "@" + srv.Domain
				}

				// if the role id is local admin
				if roleId == "admin@"+srv.Domain {
					return true, false, nil
				} else if strings.HasSuffix(roleId, "@"+srv.Domain) {
					hasAccess, _, _ = srv.validateAction(action, roleId, rbacpb.SubjectType_ROLE, resources)
					if hasAccess {
						break
					}
				}

			}
		}

		// Validate external account with local roles....
		if !hasAccess {
			roles, err := srv.getRoles()
			if err == nil {
				for i := 0; i < len(roles); i++ {
					roleId := roles[i].Id + "@" + roles[i].Domain

					if Utility.Contains(roles[i].Members, subject) {

						// if the role id is local admin
						hasAccess, _, _ = srv.validateAction(action, roleId, rbacpb.SubjectType_ROLE, resources)
						if hasAccess {
							break
						}
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
		return false, true, nil
	} else if subjectType == rbacpb.SubjectType_ROLE {
		// I will not validate the resource access for the role only the method.
		return true, false, nil
	}

	// Now I will validate the resource access infos
	permissions_, _ := srv.getActionResourcesPermissions(action)
	if len(resources) > 0 {
		if permissions_ == nil {
			err := errors.New("no resources path are given for validations")
			return false, false, err
		}
		for i := 0; i < len(resources); i++ {
			if len(resources[i].Path) > 0 { // Here if the path is empty i will simply not validate it.
				hasAccess, accessDenied, err := srv.validateAccess(subject, subjectType, resources[i].Permission, resources[i].Path)
				if err != nil {
					return false, false, err
				}

				return hasAccess, accessDenied, nil
			}
		}
	}

	//srv.logServiceInfo("", Utility.FileLine(), Utility.FunctionName(), "subject "+subject+" can call the method '"+action)
	return true, false, nil
}

// * Validate the actions...
func (srv *server) ValidateAction(ctx context.Context, rqst *rbacpb.ValidateActionRqst) (*rbacpb.ValidateActionRsp, error) {

	// So here From the context I will validate if the application can execute the action...
	var err error
	if len(rqst.Action) == 0 {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("no action was given to validate")))
	}

	// If the address is local I will give the permission.
	hasAccess, accessDenied, err := srv.validateAction(rqst.Action, rqst.Subject, rqst.Type, rqst.Infos)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &rbacpb.ValidateActionRsp{
		HasAccess:    hasAccess,
		AccessDenied: accessDenied,
	}, nil
}

// Set the subject share resource.
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

// Save / Create a Share.
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
	for i := 0; i < len(share.Accounts); i++ {
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

	for i := 0; i < len(share.Applications); i++ {
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

	for i := 0; i < len(share.Organizations); i++ {
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

	for i := 0; i < len(share.Groups); i++ {
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

	for i := 0; i < len(share.Peers); i++ {
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

	// fmt.Println("unshareResource")
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
	for i := 0; i < len(share.Accounts); i++ {
		a := "SHARED/ACCOUNTS/" + share.Accounts[i]
		err := srv.unsetSubjectSharedResource(a, uuid)
		if err != nil {
			return err
		}
	}

	for i := 0; i < len(share.Applications); i++ {
		a := "SHARED/APPLICATIONS/" + share.Applications[i]
		err := srv.unsetSubjectSharedResource(a, uuid)
		if err != nil {
			return err
		}
	}

	for i := 0; i < len(share.Organizations); i++ {
		o := "SHARED/ORGANIZATIONS/" + share.Organizations[i]
		err := srv.unsetSubjectSharedResource(o, uuid)
		if err != nil {
			return err
		}
	}

	for i := 0; i < len(share.Groups); i++ {
		g := "SHARED/GROUPS/" + share.Groups[i]
		err := srv.unsetSubjectSharedResource(g, uuid)
		if err != nil {
			return err
		}
	}

	for i := 0; i < len(share.Peers); i++ {
		p := "SHARED/PEERS/" + share.Peers[i]
		err := srv.unsetSubjectSharedResource(p, uuid)
		if err != nil {
			return err
		}
	}

	return srv.removeItem(uuid)
}

// Get the list of accessible shared resource.
// TODO if account also get share for groups and organization that the acount is part of...
func (srv *server) getSharedResource(subject string, subjectType rbacpb.SubjectType) ([]*rbacpb.Share, error) {

	// So here I will get the share resource for a given subject.
	id := "SHARED/"
	if subjectType == rbacpb.SubjectType_ACCOUNT {
		id += "ACCOUNTS"
		exist, a := srv.accountExist(subject)
		if !exist {
			return nil, errors.New("no account exist with id " + subject)
		}
		id += "/" + a
	} else if subjectType == rbacpb.SubjectType_APPLICATION {
		id += "APPLICATIONS"
		exist, a := srv.applicationExist(subject)
		if !exist {
			return nil, errors.New("no application exist with id " + subject)
		}
		id += "/" + a
	} else if subjectType == rbacpb.SubjectType_GROUP {
		id += "GROUPS"
		exist, g := srv.groupExist(subject)
		if !exist {
			return nil, errors.New("no group exist with id " + subject)
		}
		id += "/" + g
	} else if subjectType == rbacpb.SubjectType_PEER {
		id += "PEERS/" + subject
	} else if subjectType == rbacpb.SubjectType_ORGANIZATION {
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
	for i := 0; i < len(shared); i++ {
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

		for i := 0; i < len(account.Groups); i++ {
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

		for i := 0; i < len(account.Organizations); i++ {
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

		for i := 0; i < len(group.Organizations); i++ {
			share__, err := srv.getSharedResource(group.Organizations[i], rbacpb.SubjectType_ORGANIZATION)
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
func (srv *server) GetSharedResource(ctx context.Context, rqst *rbacpb.GetSharedResourceRqst) (*rbacpb.GetSharedResourceRsp, error) {

	// retreive all shared resource for a given subject.
	share, err := srv.getSharedResource(rqst.Subject, rqst.Type)

	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	if len(rqst.Owner) > 0 {
		// fmt.Println("get resource share with: ", rqst.Owner)
		share_ := make([]*rbacpb.Share, 0)
		for i := 0; i < len(share); i++ {
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
	if subjectType == rbacpb.SubjectType_ACCOUNT {
		share.Accounts = Utility.RemoveString(share.Accounts, subject)
		exist, a := srv.accountExist(subject)
		if exist {
			share.Accounts = Utility.RemoveString(share.Accounts, a)
		}

		// remove the permission.
		for i := 0; i < len(permissions.Allowed); i++ {
			permissions.Allowed[i].Accounts = Utility.RemoveString(permissions.Allowed[i].Accounts, subject)
		}

		for i := 0; i < len(permissions.Denied); i++ {
			permissions.Denied[i].Accounts = Utility.RemoveString(permissions.Denied[i].Accounts, subject)
		}

	} else if subjectType == rbacpb.SubjectType_APPLICATION {
		share.Applications = Utility.RemoveString(share.Applications, subject)
		exist, a := srv.applicationExist(subject)
		if !exist {
			share.Applications = Utility.RemoveString(share.Applications, a)
		}

		for i := 0; i < len(permissions.Allowed); i++ {
			permissions.Allowed[i].Applications = Utility.RemoveString(permissions.Allowed[i].Applications, subject)
		}

		for i := 0; i < len(permissions.Denied); i++ {
			permissions.Denied[i].Applications = Utility.RemoveString(permissions.Denied[i].Applications, subject)
		}

	} else if subjectType == rbacpb.SubjectType_GROUP {

		share.Groups = Utility.RemoveString(share.Groups, subject)
		exist, g := srv.groupExist(subject)
		if exist {
			share.Groups = Utility.RemoveString(share.Groups, g)
		}

		for i := 0; i < len(permissions.Allowed); i++ {
			permissions.Allowed[i].Groups = Utility.RemoveString(permissions.Allowed[i].Groups, subject)
		}

		for i := 0; i < len(permissions.Denied); i++ {
			permissions.Denied[i].Groups = Utility.RemoveString(permissions.Denied[i].Groups, subject)
		}

	} else if subjectType == rbacpb.SubjectType_PEER {
		share.Peers = Utility.RemoveString(share.Peers, subject)
		for i := 0; i < len(permissions.Allowed); i++ {
			permissions.Allowed[i].Peers = Utility.RemoveString(permissions.Allowed[i].Peers, subject)
		}

		for i := 0; i < len(permissions.Denied); i++ {
			permissions.Denied[i].Peers = Utility.RemoveString(permissions.Denied[i].Peers, subject)
		}

	} else if subjectType == rbacpb.SubjectType_ORGANIZATION {
		share.Organizations = Utility.RemoveString(share.Organizations, subject)
		exist, o := srv.organizationExist(subject)
		if exist {
			share.Organizations = Utility.RemoveString(share.Organizations, o)
		}

		for i := 0; i < len(permissions.Allowed); i++ {
			permissions.Allowed[i].Organizations = Utility.RemoveString(permissions.Allowed[i].Organizations, subject)
		}

		for i := 0; i < len(permissions.Denied); i++ {
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

// Remove a subject from a share.
func (srv *server) RemoveSubjectFromShare(ctx context.Context, rqst *rbacpb.RemoveSubjectFromShareRqst) (*rbacpb.RemoveSubjectFromShareRsp, error) {

	// Here I will get the share and remove the subject from it.
	// the id will be compose of the domain @ path ex. domain@/usr/toto/titi
	uuid := Utility.GenerateUUID(rqst.Domain + rqst.Path)

	err := srv.removeSubjectFromShare(rqst.Subject, rqst.Type, uuid)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &rbacpb.RemoveSubjectFromShareRsp{}, nil
}

func (srv *server) deleteSubjectShare(subject string, subjectType rbacpb.SubjectType) error {
	// fmt.Println("deleteSubjectShare")
	// First of all I will get the list of share the subject is part of.
	id := "SHARED/"

	if subjectType == rbacpb.SubjectType_ACCOUNT {
		id += "ACCOUNTS"
		exist, a := srv.accountExist(subject)
		if !exist {
			return errors.New("no account exist with id " + a)
		}
		id += "/" + a
	} else if subjectType == rbacpb.SubjectType_APPLICATION {
		id += "APPLICATIONS"
		exist, a := srv.applicationExist(subject)
		if !exist {
			return errors.New("no application exist with id " + a)
		}
		id += "/" + a
	} else if subjectType == rbacpb.SubjectType_GROUP {
		id += "GROUPS"
		exist, g := srv.groupExist(subject)
		if !exist {
			return errors.New("no group exist with id " + g)
		}
		id += "/" + g
	} else if subjectType == rbacpb.SubjectType_PEER {
		id += "PEERS/" + subject
	} else if subjectType == rbacpb.SubjectType_ORGANIZATION {
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
	for i := 0; i < len(shared); i++ {
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

// Delete the subject
func (srv *server) DeleteSubjectShare(ctx context.Context, rqst *rbacpb.DeleteSubjectShareRqst) (*rbacpb.DeleteSubjectShareRsp, error) {

	err := srv.deleteSubjectShare(rqst.Subject, rqst.Type)

	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &rbacpb.DeleteSubjectShareRsp{}, nil
}

// * Get the list of all resource permission for a given resource type ex. blog or file...
func (srv *server) GetResourcePermissionsByResourceType(rqst *rbacpb.GetResourcePermissionsByResourceTypeRqst, stream rbacpb.RbacService_GetResourcePermissionsByResourceTypeServer) error {
	permissions, err := srv.getResourceTypePathIndexation(rqst.ResourceType)

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

// * Return the list of permissions for a given subject. If no resource type was given all resource will be return. *
func (srv *server) GetResourcePermissionsBySubject(rqst *rbacpb.GetResourcePermissionsBySubjectRqst, stream rbacpb.RbacService_GetResourcePermissionsBySubjectServer) error {

	permissions, err := srv.getSubjectResourcePermissions(rqst.Subject, rqst.ResourceType, rqst.SubjectType)

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
