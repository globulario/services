// rbac_streams.go: streaming RPCs for permission listings.

package main

import (
	"github.com/globulario/services/golang/rbac/rbacpb"
	Utility "github.com/globulario/utility"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// GetResourcePermissionsByResourceType streams the list of permissions associated with a given resource type.
// It retrieves permissions using the specified resource type from the request and sends them in batches
// of up to 25 permissions per response message via the provided gRPC stream.
// If an error occurs during permission retrieval, it returns an internal error status.
// Parameters:
//   - rqst: The request containing the resource type for which permissions are to be fetched.
//   - stream: The gRPC server stream used to send batches of permissions to the client.
// Returns:
//   - error: An error if permission retrieval fails; otherwise, nil.
func (srv *server) GetResourcePermissionsByResourceType(rqst *rbacpb.GetResourcePermissionsByResourceTypeRqst, stream rbacpb.RbacService_GetResourcePermissionsByResourceTypeServer) error {
	permissions, err := srv.getResourceTypePathIndexation(rqst.ResourceType)

	if err != nil {
		status.Errorf(
			codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
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

// GetResourcePermissionsBySubject streams the permissions associated with a specific subject and resource type.
// It retrieves the permissions for the given subject and resource type, then sends them in batches over the provided gRPC stream.
// The function supports pagination by sending up to 25 permissions per message.
// Parameters:
//   - rqst: The request containing the subject, resource type, and subject type.
//   - stream: The gRPC server stream to send the permissions responses.
// Returns:
//   - error: An error if the permissions could not be retrieved or sent; otherwise, nil.
func (srv *server) GetResourcePermissionsBySubject(rqst *rbacpb.GetResourcePermissionsBySubjectRqst, stream rbacpb.RbacService_GetResourcePermissionsBySubjectServer) error {

	permissions, err := srv.getSubjectResourcePermissions(rqst.Subject, rqst.ResourceType, rqst.SubjectType)

	if err != nil {
		status.Errorf(
			codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
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
