package main

import (
	"context"

	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/resource/resourcepb"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// GetServicesCorsPolicies returns the CORS configuration for every registered service.
func (srv *server) GetServicesCorsPolicies(
	ctx context.Context,
	rq *resourcepb.GetServicesCorsPoliciesRqst,
) (*resourcepb.GetServicesCorsPoliciesRsp, error) {

	cfgs, err := config.GetServicesConfigurations()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get service configurations: %v", err)
	}

	var policies []*resourcepb.ServiceCorsPolicy
	for _, c := range cfgs {
		id, _ := c["Id"].(string)
		if id == "" {
			continue
		}
		name, _ := c["Name"].(string)
		allowAll, _ := c["AllowAllOrigins"].(bool)
		origins, _ := c["AllowedOrigins"].(string)

		policies = append(policies, &resourcepb.ServiceCorsPolicy{
			Id:              id,
			Name:            name,
			AllowAllOrigins: allowAll,
			AllowedOrigins:  origins,
		})
	}

	return &resourcepb.GetServicesCorsPoliciesRsp{Policies: policies}, nil
}

// SetServiceCorsPolicy updates the CORS settings for a single service and persists them.
func (srv *server) SetServiceCorsPolicy(
	ctx context.Context,
	rq *resourcepb.SetServiceCorsPolicyRqst,
) (*resourcepb.SetServiceCorsPolicyRsp, error) {

	if rq.Id == "" {
		return nil, status.Error(codes.InvalidArgument, "service id is required")
	}

	cfg, err := config.GetServiceConfigurationById(rq.Id)
	if err != nil || cfg == nil {
		return nil, status.Errorf(codes.NotFound, "service %q not found", rq.Id)
	}

	cfg["AllowAllOrigins"] = rq.AllowAllOrigins
	cfg["AllowedOrigins"] = rq.AllowedOrigins

	if err := config.SaveServiceConfiguration(cfg); err != nil {
		return nil, status.Errorf(codes.Internal, "save service configuration: %v", err)
	}

	return &resourcepb.SetServiceCorsPolicyRsp{}, nil
}
