// rbac_access.go: access checks (allowed/denied/owner).

package main

import (
	"context"
	"errors"
	"path/filepath"
	"strings"

	"github.com/globulario/services/golang/rbac/rbacpb"
	Utility "github.com/globulario/utility"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (srv *server) validateSubject(subject string, subjectType rbacpb.SubjectType) (string, error) {

	// first of all I will validate if the subject exsit.
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
	case rbacpb.SubjectType_PEER:
		if !srv.peerExist(subject) {
			return "", errors.New("no peer exist with id " + subject)
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
					switch subjectType {
					case rbacpb.SubjectType_ACCOUNT:

						if owners.Accounts != nil {
							if Utility.Contains(owners.Accounts, subject) {
								return true
							}
						} else {
							account, err := srv.getAccount(subject)

							if account.Groups != nil && err == nil {
								for i := range account.Groups {
									groupId := account.Groups[i]
									isOwner := srv.isOwner(groupId, rbacpb.SubjectType_GROUP, path)
									if isOwner {
										return true
									}
								}
							}

							// from the account I will get the list of group.
							if account.Organizations != nil && err == nil {
								for i := range account.Organizations {
									organizationId := account.Organizations[i]
									isOwner := srv.isOwner(organizationId, rbacpb.SubjectType_ORGANIZATION, path)
									if isOwner {
										return true
									}
								}
							}
						}

					case rbacpb.SubjectType_APPLICATION:

						exist, a := srv.applicationExist(subject)
						if owners.Applications != nil && exist {
							if Utility.Contains(owners.Applications, subject) || Utility.Contains(owners.Applications, a) {
								return true
							}
						}

					case rbacpb.SubjectType_GROUP:

						exist, g := srv.groupExist(subject)
						if owners.Groups != nil && exist {
							if Utility.Contains(owners.Groups, subject) || Utility.Contains(owners.Groups, g) {
								return true
							}
						}

					case rbacpb.SubjectType_ORGANIZATION:
						exist, o := srv.organizationExist(subject)
						if owners.Organizations != nil && exist {
							if Utility.Contains(owners.Organizations, subject) || Utility.Contains(owners.Organizations, o) {
								return true
							}
						}
					case rbacpb.SubjectType_PEER:
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
						if Utility.Contains(denied.Accounts, subject) {
							return true
						}
					} else {
						account, err := srv.getAccount(subject)

						if account.Groups != nil && err == nil {
							for i := range account.Groups {
								groupId := account.Groups[i]
								isDenied := srv.validateAccessDenied(groupId, rbacpb.SubjectType_GROUP, name, path)
								if isDenied {
									return true
								}
							}
						}

						// from the account I will get the list of group.
						if account.Organizations != nil && err == nil {
							for i := range account.Organizations {
								organizationId := account.Organizations[i]
								isDenied := srv.validateAccessDenied(organizationId, rbacpb.SubjectType_ORGANIZATION, name, path)
								if isDenied {
									return true
								}
							}
						}
					}

				case rbacpb.SubjectType_APPLICATION:

					exist, a := srv.applicationExist(subject)
					if denied.Applications != nil && exist {
						if Utility.Contains(denied.Applications, subject) || Utility.Contains(denied.Applications, a) {
							return true
						}
					}

				case rbacpb.SubjectType_GROUP:

					exist, g := srv.groupExist(subject)
					if denied.Groups != nil && exist {
						if Utility.Contains(denied.Groups, subject) || Utility.Contains(denied.Groups, g) {
							return true
						}
					}

				case rbacpb.SubjectType_ORGANIZATION:
					exist, o := srv.organizationExist(subject)
					if denied.Organizations != nil && exist {
						if Utility.Contains(denied.Organizations, subject) || Utility.Contains(denied.Organizations, o) {
							return true
						}
					}
				case rbacpb.SubjectType_PEER:
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
			for i := range permissions.Allowed {
				if permissions.Allowed[i].Name == name {
					allowed = permissions.Allowed[i]
					break
				}
			}

			if allowed != nil {
				switch subjectType {
				case rbacpb.SubjectType_ACCOUNT:
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
							if account.Groups != nil {
								for i := range account.Groups {
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
							if account.Organizations != nil {
								for i := range account.Organizations {
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
								for i := range groups {
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
								for i := range organizations {
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
				case rbacpb.SubjectType_APPLICATION:
					if allowed.Applications != nil {
						if Utility.Contains(allowed.Applications, subject) {
							return true
						}
					}

				case rbacpb.SubjectType_GROUP:
					if Utility.Contains(allowed.Groups, subject) {
						return true
					}

				case rbacpb.SubjectType_ORGANIZATION:
					if allowed.Organizations != nil {
						if Utility.Contains(allowed.Organizations, subject) {
							return true
						}
					}
				case rbacpb.SubjectType_PEER:
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
	if permissions == nil || permissions.Allowed == nil {
		// validate parent permission.
		if strings.LastIndex(path, "/") > 0 {
			dir := filepath.Dir(path)
			return srv.validateAccessAllowed(subject, subjectType, name, dir)
		}

		return true // no permissions exist so I will set it to true by default...
	}

	// read only access by default if no permission are set...
	if isPublic(path, false) {
		return name == "read"
	}

	// Permissions exist and nothing was found for so not the subject is not allowed
	return false
}

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
	if strings.HasSuffix(path_, ".ts") {
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
			if strings.HasPrefix(err.Error(), "item not found") || strings.Contains(err.Error(), "Key not found") {
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
		return false, false, nil
	}

	// The user has access.
	return true, false, nil
}

// ValidateAccess checks whether the specified subject has the required permission for a given resource path and type.
// It retrieves access information from the context and validates the request using internal access control logic.
// Returns a response indicating whether access is granted and if access is denied, or an error if validation fails.
//
// Parameters:
//   ctx  - The context for the request, which may contain authentication and authorization metadata.
//   rqst - The access validation request, including subject, type, permission, and resource path.
//
// Returns:
//   *rbacpb.ValidateAccessRsp - The response containing access validation results.
//   error                     - An error if access validation fails.
func (srv *server) ValidateAccess(ctx context.Context, rqst *rbacpb.ValidateAccessRqst) (*rbacpb.ValidateAccessRsp, error) {
	// Here I will get information from context.
	hasAccess, accessDenied, err := srv.validateAccess(rqst.Subject, rqst.Type, rqst.Permission, rqst.Path)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// The permission is set.
	return &rbacpb.ValidateAccessRsp{HasAccess: hasAccess, AccessDenied: accessDenied}, nil
}
