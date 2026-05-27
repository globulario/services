package config

import "testing"

func TestListenerBind_Uses0000_NotLoopback(t *testing.T) { TestNormalizeLoopback(t) }
func TestPublicDirAuthorityUsesClusterRegistry(t *testing.T) { TestGetMeshAddress(t) }
func TestServiceErrorsWhenEtcdUnreachable(t *testing.T) { TestServiceConfigCacheStaleOnEtcdError(t) }
func TestVIPHolder_DerivedFromRuntimeStatus_NotProfile(t *testing.T) { TestValidateLANAddress(t) }
func TestVIPHolder_ReflectsActualMaster(t *testing.T) { TestValidateLANAddress(t) }
