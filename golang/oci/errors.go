package oci

import (
	"errors"
	"fmt"
)

type FailureClass string

const (
	FailureInvalidSpec        FailureClass = "INVALID_SPEC"
	FailurePolicyBlocked      FailureClass = "POLICY_BLOCKED"
	FailureRuntimeUnavailable FailureClass = "RUNTIME_UNAVAILABLE"
	FailureRegistryAuth       FailureClass = "REGISTRY_AUTH_FAILED"
	FailureImagePull          FailureClass = "IMAGE_PULL_FAILED"
	FailureImageIdentity      FailureClass = "IMAGE_IDENTITY_MISMATCH"
	FailureContainerConflict  FailureClass = "CONTAINER_NAME_CONFLICT"
	FailureNameConflict       FailureClass = FailureContainerConflict
	FailureContainerCreate    FailureClass = "CONTAINER_CREATE_FAILED"
	FailureCreate             FailureClass = FailureContainerCreate
	FailureContainerStart     FailureClass = "CONTAINER_START_FAILED"
	FailureStart              FailureClass = FailureContainerStart
	FailureStateInspection    FailureClass = "RUNTIME_STATE_INSPECTION_FAILED"
	FailureStatePersistence   FailureClass = "OBSERVED_STATE_WRITE_FAILED"
	FailureContainerStop      FailureClass = "CONTAINER_STOP_FAILED"
	FailureStop               FailureClass = FailureContainerStop
	FailureContainerRemove    FailureClass = "CONTAINER_REMOVE_FAILED"
	FailureRemove             FailureClass = FailureContainerRemove
	FailureReadiness          FailureClass = "READINESS_FAILED"
	FailureLiveness           FailureClass = "LIVENESS_FAILED"
	FailureObservedState      FailureClass = FailureStatePersistence
)

type RuntimeError struct {
	Class FailureClass
	Op    string
	Err   error
}

func (e *RuntimeError) Error() string {
	if e == nil {
		return ""
	}
	if e.Op == "" {
		return fmt.Sprintf("%s: %v", e.Class, e.Err)
	}
	return fmt.Sprintf("%s (%s): %v", e.Class, e.Op, e.Err)
}

func (e *RuntimeError) Unwrap() error { return e.Err }

func Wrap(class FailureClass, op string, err error) error {
	if err == nil {
		return nil
	}
	var re *RuntimeError
	if errors.As(err, &re) {
		return err
	}
	return &RuntimeError{Class: class, Op: op, Err: err}
}

func FailureClassOf(err error) FailureClass {
	var re *RuntimeError
	if errors.As(err, &re) {
		return re.Class
	}
	return ""
}

func FailureOf(err error) FailureClass { return FailureClassOf(err) }
