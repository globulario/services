package main

import (
	"context"

	"github.com/globulario/services/golang/resource/resourcepb"
	Utility "github.com/globulario/utility"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// CreateReference creates a reference between a source and target resource.
// It uses the persistence service to store the reference information.
// Returns a CreateReferenceRsp on success, or an error if the operation fails.
//
// Parameters:
//   ctx - the context for the request.
//   rqst - the request containing source and target details.
//
// Returns:
//   *resourcepb.CreateReferenceRsp - response indicating success.
//   error - error if the reference could not be created.
func (srv *server) CreateReference(ctx context.Context, rqst *resourcepb.CreateReferenceRqst) (*resourcepb.CreateReferenceRsp, error) {
	// That service made user of persistence service.
	p, err := srv.getPersistenceStore()
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	err = srv.createReference(p, rqst.SourceId, rqst.SourceCollection, rqst.FieldName, rqst.TargetId, rqst.TargetCollection)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// reference was created...
	return &resourcepb.CreateReferenceRsp{}, nil
}

// DeleteReference deletes a reference from a resource.
// It retrieves the persistence store and attempts to remove the specified reference
// identified by RefId, TargetId, TargetField, and TargetCollection from the store.
// Returns a DeleteReferenceRsp on success, or an error with appropriate status code
// if the operation fails.
func (srv *server) DeleteReference(ctx context.Context, rqst *resourcepb.DeleteReferenceRqst) (*resourcepb.DeleteReferenceRsp, error) {
	p, err := srv.getPersistenceStore()
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	err = srv.deleteReference(p, rqst.RefId, rqst.TargetId, rqst.TargetField, rqst.TargetCollection)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &resourcepb.DeleteReferenceRsp{}, nil
}
