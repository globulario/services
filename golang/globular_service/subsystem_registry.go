package globular_service

import (
	"time"

	"github.com/globulario/services/golang/subsystem"
)

// Re-export subsystem types so existing callers (controller, node_agent, etc.)
// continue to work via globular_service without import changes.
// The canonical implementation is in the subsystem package, which both
// globular_service and interceptors can import without a cycle.

type SubsystemState = subsystem.SubsystemState

const (
	SubsystemHealthy  = subsystem.SubsystemHealthy
	SubsystemDegraded = subsystem.SubsystemDegraded
	SubsystemFailed   = subsystem.SubsystemFailed
	SubsystemStarting = subsystem.SubsystemStarting
	SubsystemStopped  = subsystem.SubsystemStopped
)

type SubsystemEntry = subsystem.SubsystemEntry
type SubsystemHandle = subsystem.SubsystemHandle

func RegisterSubsystem(name string, expectedInterval time.Duration) *SubsystemHandle {
	return subsystem.RegisterSubsystem(name, expectedInterval)
}

func DeregisterSubsystem(name string) {
	subsystem.DeregisterSubsystem(name)
}

func SubsystemSnapshot() []SubsystemEntry {
	return subsystem.SubsystemSnapshot()
}

func SubsystemOverallState() SubsystemState {
	return subsystem.SubsystemOverallState()
}
