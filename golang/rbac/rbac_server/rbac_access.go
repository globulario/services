// rbac_access.go: access checks (allowed/denied/owner).

package main

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/globulario/services/golang/rbac/rbacpb"
	Utility "github.com/globulario/utility"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// matchID returns true if list contains id either exactly or by bare-id (before "@").
// It also handles the case where list items are FQ and id is bare (or vice-versa).
func matchID(list []string, id string) bool {
	if len(list) == 0 || id == "" {
		return false
	}
	// fast path: exact match
	if Utility.Contains(list, id) {
		return true
	}
	// compare on bare forms both ways
	bare := id
	if i := strings.Index(id, "@"); i >= 0 {
		bare = id[:i]
	}
	for _, v := range list {
		if v == bare {
			return true
		}
		if j := strings.Index(v, "@"); j >= 0 && v[:j] == bare {
			return true
		}
	}
	return false
}

func (srv *server) validateSubject(subject string, subjectType rbacpb.SubjectType) (string, error) {
	switch subjectType {
	case rbacpb.SubjectType_ACCOUNT:
		exist, a := srv.accountExist(subject)
		if !exist {
			return "", errors.New("no account exist with id " + a)
		}
		return a, nil
	case rbacpb.SubjectType_APPLICATION:
		exist, a := srv.applicationExist(subject)
		if !exist {
			return "", errors.New("no application exist with id " + a)
		}
		return a, nil
	case rbacpb.SubjectType_GROUP:
		exist, g := srv.groupExist(subject)
		if !exist {
			return "", errors.New("no group exist with id " + g)
		}
		return g, nil
	case rbacpb.SubjectType_ORGANIZATION:
		exist, o := srv.organizationExist(subject)
		if !exist {
			return "", errors.New("no organization exist with id " + o)
		}
		return o, nil
	case rbacpb.SubjectType_ROLE:
		exist, r := srv.roleExist(subject)
		if !exist {
			return "", errors.New("no role exist with id " + r)
		}
		return r, nil
	case rbacpb.SubjectType_NODE_IDENTITY:
		if !srv.nodeIdentityExists(subject) {
			return "", errors.New("no node identity exists with id " + subject)
		}
		return subject, nil
	}
	return "", errors.New("no subject found with id " + subject)
}

func (srv *server) isOwner(subject string, subjectType rbacpb.SubjectType, path string) bool {
	subject, err := srv.validateSubject(subject, subjectType)
	if err != nil {
		return false
	}

	permissions, err := srv.getResourcePermissions(path)
	if err == nil {
		if permissions.Owners != nil {
			owners := permissions.Owners
			if owners != nil {
				hasOwner := false
				if len(owners.Accounts) > 0 {
					hasOwner = true
				}
				if len(owners.Applications) > 0 {
					hasOwner = true
				}
				if len(owners.Groups) > 0 {
					hasOwner = true
				}
				if len(owners.Organizations) > 0 {
					hasOwner = true
				}
				if len(owners.NodeIdentities) > 0 {
					hasOwner = true
				}

				if hasOwner {
					switch subjectType {
					case rbacpb.SubjectType_ACCOUNT:
						if owners.Accounts != nil {
							if matchID(owners.Accounts, subject) {
								return true
							}
						} else {
							account, err := srv.getAccount(subject)
							if err == nil && account.Groups != nil {
								for i := range account.Groups {
									groupId := account.Groups[i]
									if srv.isOwner(groupId, rbacpb.SubjectType_GROUP, path) {
										return true
									}
								}
							}
							if err == nil && account.Organizations != nil {
								for i := range account.Organizations {
									organizationId := account.Organizations[i]
									if srv.isOwner(organizationId, rbacpb.SubjectType_ORGANIZATION, path) {
										return true
									}
								}
							}
						}
					case rbacpb.SubjectType_APPLICATION:
						exist, a := srv.applicationExist(subject)
						if owners.Applications != nil && exist {
							if matchID(owners.Applications, subject) || matchID(owners.Applications, a) {
								return true
							}
						}
					case rbacpb.SubjectType_GROUP:
						exist, g := srv.groupExist(subject)
						if owners.Groups != nil && exist {
							if matchID(owners.Groups, subject) || matchID(owners.Groups, g) {
								return true
							}
						}
					case rbacpb.SubjectType_ORGANIZATION:
						exist, o := srv.organizationExist(subject)
						if owners.Organizations != nil && exist {
							if matchID(owners.Organizations, subject) || matchID(owners.Organizations, o) {
								return true
							}
						}
					case rbacpb.SubjectType_NODE_IDENTITY:
						if owners.NodeIdentities != nil && matchID(owners.NodeIdentities, subject) {
							return true
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

func (srv *server) validateAccessDenied(subject string, subjectType rbacpb.SubjectType, name string, path string) bool {
	subject, err := srv.validateSubject(subject, subjectType)
	if err != nil {
		return false
	}

	permissions, err := srv.getResourcePermissions(path)
	if err == nil {
		if permissions.Denied != nil {
			var denied *rbacpb.Permission
			for i := range permissions.Denied {
				if permissions.Denied[i].Name == name {
					denied = permissions.Denied[i]
					break
				}
			}

			if denied != nil {
				switch subjectType {
				case rbacpb.SubjectType_ACCOUNT:
					if denied.Accounts != nil {
						if matchID(denied.Accounts, subject) {
							return true
						}
					} else {
						account, err := srv.getAccount(subject)
						if err == nil && account.Groups != nil {
							for i := range account.Groups {
								groupId := account.Groups[i]
								if !strings.Contains(groupId, "@") {
									groupId = groupId + "@" + srv.Domain
								}
								if srv.validateAccessDenied(groupId, rbacpb.SubjectType_GROUP, name, path) {
									return true
								}
							}
						}
						if err == nil && account.Organizations != nil {
							for i := range account.Organizations {
								organizationId := account.Organizations[i]
								if !strings.Contains(organizationId, "@") {
									organizationId = organizationId + "@" + srv.Domain
								}
								if srv.validateAccessDenied(organizationId, rbacpb.SubjectType_ORGANIZATION, name, path) {
									return true
								}
							}
						}
						// Also handle memberships only present on the group/org entities (mirrors allow path)
						if groups, err := srv.getGroups(); err == nil {
							for i := range groups {
								if matchID(groups[i].Accounts, subject) { // FIX: was Utility.Contains
									id := groups[i].Id + "@" + groups[i].Domain
									if srv.validateAccessDenied(id, rbacpb.SubjectType_GROUP, name, path) {
										return true
									}
								}
							}
						}
						if organizations, err := srv.getOrganizations(); err == nil {
							for i := range organizations {
								if matchID(organizations[i].Accounts, subject) { // FIX: was Utility.Contains
									id := organizations[i].Id + "@" + organizations[i].Domain
									if srv.validateAccessDenied(id, rbacpb.SubjectType_ORGANIZATION, name, path) {
										return true
									}
								}
							}
						}
					}

				case rbacpb.SubjectType_APPLICATION:
					exist, a := srv.applicationExist(subject)
					if denied.Applications != nil && exist {
						if matchID(denied.Applications, subject) || matchID(denied.Applications, a) {
							return true
						}
					}

				case rbacpb.SubjectType_GROUP:
					exist, g := srv.groupExist(subject)
					if denied.Groups != nil && exist {
						if matchID(denied.Groups, subject) || matchID(denied.Groups, g) {
							return true
						}
					}

				case rbacpb.SubjectType_ORGANIZATION:
					exist, o := srv.organizationExist(subject)
					if denied.Organizations != nil && exist {
						if matchID(denied.Organizations, subject) || matchID(denied.Organizations, o) {
							return true
						}
					}

				case rbacpb.SubjectType_NODE_IDENTITY:
					if denied.NodeIdentities != nil {
						if matchID(denied.NodeIdentities, subject) {
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
		fmt.Println("validateAccessAllowed: subject validation error:", err)
		return false
	}

	// Test if the user is the sa
	if subjectType == rbacpb.SubjectType_ACCOUNT && strings.HasPrefix(subject, "sa@") {
		fmt.Println("Subject is service account, allowing access")
		return true
	}

	permissions, err := srv.getResourcePermissions(path)
	if err == nil {
		if permissions.Allowed != nil {
			var allowed *rbacpb.Permission
			for i := range permissions.Allowed {
				if permissions.Allowed[i].Name == name {
					allowed = permissions.Allowed[i]
					break
				}
			}

			if allowed != nil {
				switch subjectType {
				case rbacpb.SubjectType_ACCOUNT:
					if allowed.Accounts != nil && len(allowed.Accounts) > 0 {
						if matchID(allowed.Accounts, subject) {
							return true
						}
					} else {
						account, err := srv.getAccount(subject)
						if err == nil && account.Groups != nil {
							for i := range account.Groups {
								groupId := account.Groups[i]
								if !strings.Contains(groupId, "@") {
									groupId = groupId + "@" + srv.Domain
								}
								if srv.validateAccessAllowed(groupId, rbacpb.SubjectType_GROUP, name, path) {
									return true
								}
							}
						}
						if err == nil && account.Organizations != nil {
							for i := range account.Organizations {
								organizationId := account.Organizations[i]
								if !strings.Contains(organizationId, "@") {
									organizationId = organizationId + "@" + srv.Domain
								}
								if srv.validateAccessAllowed(organizationId, rbacpb.SubjectType_ORGANIZATION, name, path) {
									return true
								}
							}
						}
						// external memberships
						if groups, err := srv.getGroups(); err == nil {
							for i := range groups {
								if matchID(groups[i].Accounts, subject) { // FIX: was Utility.Contains
									id := groups[i].Id + "@" + groups[i].Domain
									if srv.validateAccessAllowed(id, rbacpb.SubjectType_GROUP, name, path) {
										return true
									}
								}
							}
						}
						if organizations, err := srv.getOrganizations(); err == nil {
							for i := range organizations {
								if matchID(organizations[i].Accounts, subject) { // FIX: was Utility.Contains
									id := organizations[i].Id + "@" + organizations[i].Domain
									if srv.validateAccessAllowed(id, rbacpb.SubjectType_ORGANIZATION, name, path) {
										return true
									}
								}
							}
						}
					}

				case rbacpb.SubjectType_APPLICATION:
					if allowed.Applications != nil {
						if matchID(allowed.Applications, subject) {
							return true
						}
					}

				case rbacpb.SubjectType_GROUP:
					if matchID(allowed.Groups, subject) {
						return true
					}

				case rbacpb.SubjectType_ORGANIZATION:
					if allowed.Organizations != nil {
						if matchID(allowed.Organizations, subject) {
							return true
						}
					}

				case rbacpb.SubjectType_NODE_IDENTITY:
					if allowed.NodeIdentities != nil {
						if matchID(allowed.NodeIdentities, subject) {
							return true
						}
					}
				}
			}
		}
	}

	// inherit from parent
	if permissions == nil || permissions.Allowed == nil {
		if strings.LastIndex(path, "/") > 0 {
			dir := filepath.Dir(path)
			return srv.validateAccessAllowed(subject, subjectType, name, dir)
		}
	}

	// public: read-only
	if srv.isPublic(path) {
		return name == "read"
	}

	return false
}

func (srv *server) validateAccess(subject string, subjectType rbacpb.SubjectType, name string, path string) (bool, bool, error) {
	subject, err := srv.validateSubject(subject, subjectType)
	if err != nil {
		return false, false, err
	}

	if len(path) == 0 {
		return false, false, errors.New("no path was given to validate access for suject " + subject)
	}

	// .hidden and HLS chunk policy
	if strings.Contains(path, "/.hidden/") {
		return true, false, nil
	}
	path_ := srv.formatPath(path)
	if strings.HasSuffix(path_, ".ts") {
		if srv.storageExists(filepath.Dir(path_) + "/playlist.m3u8") {
			return true, false, nil
		}
	}

	// owners have full access
	if srv.isOwner(subject, subjectType, path) {
		return true, false, nil
	} else if name == "owner" {
		permissions, err := srv.getResourcePermissions(path)
		if err != nil {
			if strings.HasPrefix(err.Error(), "item not found") || strings.Contains(err.Error(), "Key not found") {
				return true, false, nil
			}
		} else if permissions.Owners == nil {
			return true, false, nil
		} else if permissions.Owners != nil {
			hasOwner := false
			if len(permissions.Owners.Accounts) > 0 ||
				len(permissions.Owners.Applications) > 0 ||
				len(permissions.Owners.Organizations) > 0 ||
				len(permissions.Owners.Groups) > 0 ||
				len(permissions.Owners.NodeIdentities) > 0 {
				hasOwner = true
			}
			if !hasOwner {
				return true, false, nil
			}
		}
		return false, false, nil
	}

	// explicit denials override
	if isDenied := srv.validateAccessDenied(subject, subjectType, name, path); isDenied {
		return false, true, nil
	}

	// then allows
	if isAllowed := srv.validateAccessAllowed(subject, subjectType, name, path); !isAllowed {
		return false, false, nil
	}

	return true, false, nil
}

// ValidateAccess RPC
func (srv *server) ValidateAccess(ctx context.Context, rqst *rbacpb.ValidateAccessRqst) (*rbacpb.ValidateAccessRsp, error) {
	hasAccess, accessDenied, err := srv.validateAccess(rqst.Subject, rqst.Type, rqst.Permission, rqst.Path)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	return &rbacpb.ValidateAccessRsp{HasAccess: hasAccess, AccessDenied: accessDenied}, nil
}
