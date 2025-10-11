// rbac_ownership.go: ownership management helpers.

package main

import (
	"context"
	"errors"
	"log/slog"
	"strings"

	"github.com/globulario/services/golang/rbac/rbacpb"
	Utility "github.com/globulario/utility"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func isNotFoundErr(err error) bool {
    if err == nil { return false }
    s := strings.ToLower(err.Error())
    // accept all common variants across stores (etcd, mongo, scylla, in-mem)
    return strings.Contains(s, "not found") ||       // "not found", "no entity found"
           strings.Contains(s, "key not found") ||   // etcd errors
           strings.Contains(s, "item not found")
}

func (srv *server) addResourceOwner(path, subject, resourceType_ string, subjectType rbacpb.SubjectType) error {
	if len(path) == 0 {
		return errors.New("no resource path was given")
	}

	if len(subject) == 0 {
		return errors.New("no subject was given")
	}

	if len(subject) == 0 {
		return errors.New("no resource type was given")
	}

	slog.Info("addResourceOwner call",
    "path", path,
    "resourceType", resourceType_,
    "subject", subject,
    "subjectType", subjectType.String())

	permissions, err := srv.getResourcePermissions(path)

	needSave := false
    if err != nil {
        if isNotFoundErr(err) {
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

	err := srv.addResourceOwner(rqst.Path, rqst.Subject, rqst.ResourceType, rqst.Type)

	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	return &rbacpb.AddResourceOwnerRsp{}, nil
}

// --- rbac_ownership.go ---
// Safe, idempotent: removing an owner when none is set just no-ops.
// NOTE: keep the signature you actually expose; this one matches the call sites in your logs.
func (srv *server) removeResourceOwner(path string, subject string, subjectType rbacpb.SubjectType) error {
	perms, err := srv.getResourcePermissions(path)
	if err != nil {
		// If the resource has no permissions yet, there's nothing to remove.
		// Do NOT turn this into an error — treat as no-op.
		return nil
	}
	if perms == nil || perms.Owners == nil {
		// Nothing to do
		return nil
	}

	owners := perms.Owners
	switch subjectType {
	case rbacpb.SubjectType_ACCOUNT:
		owners.Accounts = removeString(owners.Accounts, subject)
	case rbacpb.SubjectType_APPLICATION:
		owners.Applications = removeString(owners.Applications, subject)
	case rbacpb.SubjectType_GROUP:
		owners.Groups = removeString(owners.Groups, subject)
	case rbacpb.SubjectType_ORGANIZATION:
		owners.Organizations = removeString(owners.Organizations, subject)
	case rbacpb.SubjectType_PEER:
		owners.Peers = removeString(owners.Peers, subject)
	}

	// Persist back (resource type comes from the stored record)
	if err := srv.setResourcePermissions(path, perms.ResourceType, perms); err != nil {
		return err
	}
	return nil
}

// Remove all explicit access (Allowed/Denied) for a subject across resources.
// IMPORTANT: This intentionally does NOT touch Owners — owners keep implicit rights.
func (srv *server) DeleteAllAccess(ctx context.Context, rqst *rbacpb.DeleteAllAccessRqst) (*rbacpb.DeleteAllAccessRsp, error) {
	subject := rqst.Subject
	stype := rqst.Type

	// Iterate every permissions record and strip the subject from Allowed/Denied only.
	paths, err := srv.listAllPermissionPaths() // implement to return []string of permission-bearing paths
	if err != nil {
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	for _, p := range paths {
		perms, getErr := srv.getResourcePermissions(p)
		if getErr != nil || perms == nil {
			continue // skip unreadable/missing entries
		}

		// Scrub from Allowed
		for i := range perms.Allowed {
			scrubPermissionSubjects(perms.Allowed[i], subject, stype)
		}
		// Scrub from Denied
		for i := range perms.Denied {
			scrubPermissionSubjects(perms.Denied[i], subject, stype)
		}

		// DO NOT modify perms.Owners here — owner rights are implicit and must remain.

		// Persist only if something actually changed (optional micro-opt).
		if err := srv.setResourcePermissions(p, perms.ResourceType, perms); err != nil {
			return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
	}

	return &rbacpb.DeleteAllAccessRsp{}, nil
}

// --- rbac_permissions.go (or a shared helpers file) ---

// removeString returns a fresh slice with all occurrences of s removed (nil-safe).
func removeString(in []string, s string) []string {
	if len(in) == 0 {
		return in
	}
	out := in[:0]
	for _, v := range in {
		if v != s {
			out = append(out, v)
		}
	}
	// If everything was removed, make it an empty (non-nil) slice to keep JSON/stores consistent.
	if len(out) == 0 {
		return []string{}
	}
	return out
}

// scrubPermissionSubjects removes the subject from the correct field in a Permission.
func scrubPermissionSubjects(p *rbacpb.Permission, subject string, stype rbacpb.SubjectType) {
	if p == nil {
		return
	}
	switch stype {
	case rbacpb.SubjectType_ACCOUNT:
		p.Accounts = removeString(p.Accounts, subject)
	case rbacpb.SubjectType_APPLICATION:
		p.Applications = removeString(p.Applications, subject)
	case rbacpb.SubjectType_GROUP:
		p.Groups = removeString(p.Groups, subject)
	case rbacpb.SubjectType_ORGANIZATION:
		p.Organizations = removeString(p.Organizations, subject)
	case rbacpb.SubjectType_PEER:
		p.Peers = removeString(p.Peers, subject)
	}
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
	err := srv.removeResourceOwner(rqst.Path, rqst.Subject, rqst.Type)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &rbacpb.RemoveResourceOwnerRsp{}, nil
}
