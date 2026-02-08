package main

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/discovery/discoverypb"
	"github.com/globulario/services/golang/repository/repositorypb"
	"github.com/globulario/services/golang/resource/resourcepb"
	"github.com/globulario/services/golang/security"
	Utility "github.com/globulario/utility"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// handlers.go - Discovery Service RPC Handlers
//
// Phase 1 Step 2: Renamed from discovery.go to follow Echo refactoring pattern.
//
// This file contains pure business logic handlers for the Discovery service:
// - PublishService: Registers service packages with RBAC validation
// - PublishApplication: Registers application packages with RBAC validation
// - ResolveInstallPlan: Generates installation plans for different profiles
// - GetPackageDescriptor: Retrieves package metadata
//
// All handlers are pure functions with no side effects (no config persistence).
// Authentication, authorization, and resource management are delegated to
// external services (rbac, resource).

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
			return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), mErr))
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
		"PublisherID", publisherID,
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
		e := status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
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
		"PublisherID", publisherID,
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
		e := status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		slog.Error("PublishApplication failed to publish descriptor", "name", rqst.Name, "err", err)
		return nil, e
	}

	slog.Info("PublishApplication success",
		"name", rqst.Name,
		"version", rqst.Version,
	)

	return &discoverypb.PublishApplicationResponse{}, nil
}

func (srv *server) ResolveInstallPlan(ctx context.Context, rqst *discoverypb.ResolveInstallPlanRequest) (*discoverypb.InstallPlan, error) {
	if rqst == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}

	platform := strings.TrimSpace(rqst.GetPlatform())
	if platform == "" {
		platform = "linux/amd64"
	}

	publisher := strings.TrimSpace(rqst.GetChannel())
	if publisher == "" {
		publisher = "globular"
	}

	repoID := ""
	if repos := rqst.GetRepositories(); len(repos) > 0 {
		repoID = repos[0]
	}

	pins := rqst.GetPins()
	steps := make([]*discoverypb.InstallStep, 0, 8)
	addStep := func(id, artifactName string, kind repositorypb.ArtifactKind, action discoverypb.InstallStep_Action, depends []string) {
		artifact := &repositorypb.ArtifactRef{
			PublisherId: publisher,
			Name:        artifactName,
			Version:     resolveVersion(artifactName, pins),
			Platform:    platform,
			Kind:        kind,
		}
		steps = append(steps, &discoverypb.InstallStep{
			Id: id,
		})
		if artifact != nil {
			steps[len(steps)-1].Artifact = artifact
		}
		if repoID != "" {
			steps[len(steps)-1].RepositoryId = repoID
		}
		steps[len(steps)-1].Action = action
		if len(depends) > 0 {
			steps[len(steps)-1].DependsOn = append([]string{}, depends...)
		}
	}

	addStep("nodeagent", "nodeagent", repositorypb.ArtifactKind_AGENT, discoverypb.InstallStep_DOWNLOAD, nil)

	profiles := rqst.GetProfiles()
	if hasProfile(profiles, "control-plane") {
		addStep("etcd", "etcd", repositorypb.ArtifactKind_SUBSYSTEM, discoverypb.InstallStep_INSTALL, []string{"nodeagent"})
		addStep("envoy", "envoy", repositorypb.ArtifactKind_SUBSYSTEM, discoverypb.InstallStep_INSTALL, []string{"nodeagent"})
		addStep("clustercontroller", "clustercontroller", repositorypb.ArtifactKind_SERVICE, discoverypb.InstallStep_INSTALL, []string{"nodeagent"})
		addStep("discovery", "discovery", repositorypb.ArtifactKind_SERVICE, discoverypb.InstallStep_INSTALL, []string{"clustercontroller"})
		addStep("repository", "repository", repositorypb.ArtifactKind_SERVICE, discoverypb.InstallStep_INSTALL, []string{"clustercontroller"})
	}

	if hasProfile(profiles, "storage") {
		addStep("minio", "minio", repositorypb.ArtifactKind_SUBSYSTEM, discoverypb.InstallStep_INSTALL, []string{"nodeagent"})
		addStep("scylla", "scylla", repositorypb.ArtifactKind_SUBSYSTEM, discoverypb.InstallStep_INSTALL, []string{"nodeagent"})
	}

	if hasProfile(profiles, "worker") {
		addStep("globular-core", "globular-core", repositorypb.ArtifactKind_SERVICE, discoverypb.InstallStep_INSTALL, []string{"nodeagent"})
	}

	return &discoverypb.InstallPlan{
		PlanId: Utility.GenerateUUID(platform + ":" + time.Now().String()),
		Steps:  steps,
	}, nil
}

func hasProfile(profiles []string, target string) bool {
	target = strings.ToLower(strings.TrimSpace(target))
	if target == "" {
		return false
	}
	for _, p := range profiles {
		n := strings.ToLower(strings.TrimSpace(p))
		if n == target || strings.HasPrefix(n, target+"-") {
			return true
		}
	}
	return false
}

func resolveVersion(name string, pins map[string]string) string {
	if pins != nil {
		if v := strings.TrimSpace(pins[name]); v != "" {
			return v
		}
	}
	return "latest"
}

func (srv *server) GetPackageDescriptor(ctx context.Context, rqst *resourcepb.GetPackageDescriptorRequest) (*resourcepb.GetPackageDescriptorResponse, error) {
	address, _ := config.GetAddress()
	resourceClient, err := srv.getResourceClient(address)
	if err != nil {
		return nil, status.Errorf(codes.Unavailable, "failed to connect to resource service: %v", err)
	}
	descriptor, err := resourceClient.GetPackageDescriptor(rqst.GetServiceId(), rqst.GetPublisherID(), "")
	if err != nil {
		return nil, status.Errorf(codes.Internal, "resource GetPackageDescriptor failed: %v", err)
	}
	resp := &resourcepb.GetPackageDescriptorResponse{}
	if descriptor != nil {
		resp.Results = []*resourcepb.PackageDescriptor{descriptor}
	}
	return resp, nil
}
