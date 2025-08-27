package main

import (
	"context"
	"errors"

	"github.com/globulario/services/golang/discovery/discoverypb"
	"github.com/globulario/services/golang/resource/resource_client"
	"github.com/globulario/services/golang/resource/resourcepb"
	"github.com/globulario/services/golang/security"
	Utility "github.com/globulario/utility"
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
		return nil, errors.New("the user id " + rqst.User + "dosent match the token id " + clientId)
	}

	// Make sure the user is part of the organization if one is given.
	PublisherID := rqst.User
	if len(rqst.Organization) > 0 {
		isMember, err := srv.isOrganizationMember(rqst.User, rqst.Organization)
		if err != nil {
			return nil, err
		}
		PublisherID = rqst.Organization
		if !isMember {
			return nil, err
		}
	}

	// Now I will upload the service to the repository...
	descriptor := &resourcepb.PackageDescriptor{
		Id:           rqst.ServiceId,
		Name:         rqst.ServiceName,
		PublisherID:  PublisherID,
		Version:      rqst.Version,
		Description:  rqst.Description,
		Keywords:     rqst.Keywords,
		Repositories: []string{rqst.RepositoryId},
		Discoveries:  []string{rqst.DiscoveryId},
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
		return nil, errors.New("the user id " + rqst.User + " dosent match the token id " + clientId)
	}

	PublisherID := rqst.User
	if len(rqst.Organization) > 0 {
		PublisherID = rqst.Organization
	}

	// Now I will upload the service to the repository...
	descriptor := &resourcepb.PackageDescriptor{
		Id:           rqst.Name,
		Name:         rqst.Name,
		PublisherID:  PublisherID,
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
