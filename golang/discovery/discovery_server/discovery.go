package main

import (
	"context"
	"errors"

	"github.com/davecourtois/Utility"
	"github.com/globulario/services/golang/discovery/discoverypb"
	"github.com/globulario/services/golang/resource/resource_client"
	"github.com/globulario/services/golang/resource/resourcepb"
	"github.com/globulario/services/golang/security"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var (
	resource_client_ *resource_client.Resource_Client
)

///////////////////// resource service functions ////////////////////////////////////

// Publish a service. The service must be install localy on the server.
func (srv *server) PublishService(ctx context.Context, rqst *discoverypb.PublishServiceRequest) (*discoverypb.PublishServiceResponse, error) {

	clientId, _, err := security.GetClientId(ctx)
	if err != nil {
		return nil, err
	}

	if clientId != rqst.User {
		return nil, errors.New("the user id dosent match the token id")
	}

	// Make sure the user is part of the organization if one is given.
	publisherId := rqst.User
	if len(rqst.Organization) > 0 {
		isMember, err := srv.isOrganizationMember(rqst.User, rqst.Organization)
		if err != nil {
			return nil, err
		}
		publisherId = rqst.Organization
		if !isMember {
			return nil, err
		}
	}

	// Now I will upload the service to the repository...
	descriptor := &resourcepb.PackageDescriptor{
		Id:           rqst.ServiceId,
		Name:         rqst.ServiceName,
		PublisherId:  publisherId,
		Version:      rqst.Version,
		Description:  rqst.Description,
		Keywords:     rqst.Keywords,
		Repositories: []string{rqst.RepositoryId},
		Discoveries:  []string{rqst.DicorveryId},
		Type:         resourcepb.PackageType_SERVICE_TYPE,
	}

	// Publish the service package.
	err = srv.publishPackageDescriptor(descriptor)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &discoverypb.PublishServiceResponse{
		Result: true,
	}, nil
}

// Publish a web application to a globular node. That must be use at development mostly...
func (srv *server) PublishApplication(ctx context.Context, rqst *discoverypb.PublishApplicationRequest) (*discoverypb.PublishApplicationResponse, error) {

	clientId, _, err := security.GetClientId(ctx)
	if err != nil {
		return nil, err
	}

	if clientId != rqst.User {
		return nil, errors.New("the user id dosent match the token id")
	}

	publisherId := rqst.User
	if len(rqst.Organization) > 0 {
		publisherId = rqst.Organization
	}

	// Now I will upload the service to the repository...
	descriptor := &resourcepb.PackageDescriptor{
		Id:           rqst.Name,
		Name:         rqst.Name,
		PublisherId:  publisherId,
		Version:      rqst.Version,
		Description:  rqst.Description,
		Keywords:     rqst.Keywords,
		Repositories: []string{rqst.Repository},
		Discoveries:  []string{rqst.Discovery},
		Type:         resourcepb.PackageType_APPLICATION_TYPE,
		Icon:         rqst.Icon,
		Alias:        rqst.Alias,
		Actions:      rqst.Actions,
		Roles:        rqst.Roles,
		Groups:       rqst.Groups,
	}

	// Fist of all publish the package descriptor.
	err = srv.publishPackageDescriptor(descriptor)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &discoverypb.PublishApplicationResponse{}, nil
}
