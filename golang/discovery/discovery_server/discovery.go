package main

import (
	"context"
	"log/slog"
	"strings"

	"github.com/globulario/services/golang/discovery/discoverypb"
	"github.com/globulario/services/golang/resource/resourcepb"
	"github.com/globulario/services/golang/security"
	Utility "github.com/globulario/utility"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

///////////////////// resource service functions ////////////////////////////////////

// PublishService registers (or updates) a service package descriptor into the
// resource repository and emits the corresponding discovery event.
// It validates the caller identity (token vs rqst.User) and, if an
// organization is specified, ensures membership before publishing.
//
// Required fields: ServiceId, ServiceName, Version, RepositoryId, DiscoveryId, User.
func (srv *server) PublishService(ctx context.Context, rqst *discoverypb.PublishServiceRequest) (*discoverypb.PublishServiceResponse, error) {
	// Basic normalization
	trim := func(s string) string { return strings.TrimSpace(s) }

	// Fast input validation for clearer error messages
	if trim(rqst.GetServiceId()) == "" ||
		trim(rqst.GetServiceName()) == "" ||
		trim(rqst.GetVersion()) == "" ||
		trim(rqst.GetRepositoryId()) == "" ||
		trim(rqst.GetDiscoveryId()) == "" ||
		trim(rqst.GetUser()) == "" {
		err := status.Errorf(codes.InvalidArgument, "missing required fields: serviceId, serviceName, version, repositoryId, discoveryId, user")
		slog.Error("PublishService invalid argument", "err", err, "rqst", map[string]any{
			"serviceId":    rqst.GetServiceId(),
			"serviceName":  rqst.GetServiceName(),
			"version":      rqst.GetVersion(),
			"repositoryId": rqst.GetRepositoryId(),
			"discoveryId":  rqst.GetDiscoveryId(),
			"user":         rqst.GetUser(),
		})
		return nil, err
	}

	clientID, _, err := security.GetClientId(ctx)
	if err != nil {
		e := status.Errorf(codes.Unauthenticated, "failed to read caller identity")
		slog.Error("PublishService unauthenticated", "err", err)
		return nil, e
	}

	if clientID != rqst.User {
		e := status.Errorf(codes.PermissionDenied, "token subject (%s) does not match request user (%s)", clientID, rqst.User)
		slog.Error("PublishService permission denied: token/user mismatch", "tokenSubject", clientID, "user", rqst.User)
		return nil, e
	}

	// Organization check (if provided)
	publisherID := rqst.User
	if org := trim(rqst.GetOrganization()); org != "" {
		isMember, mErr := srv.isOrganizationMember(rqst.User, org)
		if mErr != nil {
			slog.Error("PublishService organization membership check failed", "user", rqst.User, "org", org, "err", mErr)
			return nil, status.Errorf(codes.Internal, Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), mErr))
		}
		if !isMember {
			e := status.Errorf(codes.PermissionDenied, "user %s is not a member of organization %s", rqst.User, org)
			slog.Error("PublishService permission denied: not an organization member", "user", rqst.User, "org", org)
			return nil, e
		}
		publisherID = org
	}

	slog.Info("PublishService begin",
		"serviceId", rqst.ServiceId,
		"serviceName", rqst.ServiceName,
		"version", rqst.Version,
		"repositoryId", rqst.RepositoryId,
		"discoveryId", rqst.DiscoveryId,
		"publisherId", publisherID,
	)

	descriptor := &resourcepb.PackageDescriptor{
		Id:           rqst.ServiceId,
		Name:         rqst.ServiceName,
		PublisherID:  publisherID,
		Version:      rqst.Version,
		Description:  rqst.Description,
		Keywords:     rqst.Keywords,
		Repositories: []string{rqst.RepositoryId},
		Discoveries:  []string{rqst.DiscoveryId},
		Type:         resourcepb.PackageType_SERVICE_TYPE,
	}

	// Publish the service package.
	if err := srv.publishPackageDescriptor(descriptor); err != nil {
		e := status.Errorf(codes.Internal, Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		slog.Error("PublishService failed to publish descriptor", "serviceId", rqst.ServiceId, "err", err)
		return nil, e
	}

	slog.Info("PublishService success",
		"serviceId", rqst.ServiceId,
		"version", rqst.Version,
	)

	return &discoverypb.PublishServiceResponse{Result: true}, nil
}

// PublishApplication registers (or updates) an application package descriptor
// into the resource repository and emits the corresponding discovery event.
// It validates the caller identity (token vs rqst.User). If Organization is
// set, its value becomes the PublisherID (membership enforcement can be added
// similarly to PublishService if needed).
//
// Required fields: Name, Version, Repository, Discovery, User.
func (srv *server) PublishApplication(ctx context.Context, rqst *discoverypb.PublishApplicationRequest) (*discoverypb.PublishApplicationResponse, error) {
	trim := func(s string) string { return strings.TrimSpace(s) }

	if trim(rqst.GetName()) == "" ||
		trim(rqst.GetVersion()) == "" ||
		trim(rqst.GetRepository()) == "" ||
		trim(rqst.GetDiscovery()) == "" ||
		trim(rqst.GetUser()) == "" {
		err := status.Errorf(codes.InvalidArgument, "missing required fields: name, version, repository, discovery, user")
		slog.Error("PublishApplication invalid argument", "err", err, "rqst", map[string]any{
			"name":       rqst.GetName(),
			"version":    rqst.GetVersion(),
			"repository": rqst.GetRepository(),
			"discovery":  rqst.GetDiscovery(),
			"user":       rqst.GetUser(),
		})
		return nil, err
	}

	clientID, _, err := security.GetClientId(ctx)
	if err != nil {
		e := status.Errorf(codes.Unauthenticated, "failed to read caller identity")
		slog.Error("PublishApplication unauthenticated", "err", err)
		return nil, e
	}

	if clientID != rqst.User {
		e := status.Errorf(codes.PermissionDenied, "token subject (%s) does not match request user (%s)", clientID, rqst.User)
		slog.Error("PublishApplication permission denied: token/user mismatch", "tokenSubject", clientID, "user", rqst.User)
		return nil, e
	}

	publisherID := rqst.User
	if org := trim(rqst.GetOrganization()); org != "" {
		// Note: if you also want to enforce membership here, mirror the check
		// used in PublishService.
		publisherID = org
	}

	slog.Info("PublishApplication begin",
		"name", rqst.Name,
		"version", rqst.Version,
		"repository", rqst.Repository,
		"discovery", rqst.Discovery,
		"publisherId", publisherID,
		"alias", rqst.Alias,
	)

	descriptor := &resourcepb.PackageDescriptor{
		Id:           rqst.Name,
		Name:         rqst.Name,
		PublisherID:  publisherID,
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

	if err := srv.publishPackageDescriptor(descriptor); err != nil {
		e := status.Errorf(codes.Internal, Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		slog.Error("PublishApplication failed to publish descriptor", "name", rqst.Name, "err", err)
		return nil, e
	}

	slog.Info("PublishApplication success",
		"name", rqst.Name,
		"version", rqst.Version,
	)

	return &discoverypb.PublishApplicationResponse{}, nil
}
