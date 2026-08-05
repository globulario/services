package oci

import (
	"context"
	"fmt"
	"io"
	"sync"
	"time"
)

// Run maintains one admitted OCI service until its context is cancelled or the
// owned container exits. systemd supervises this process; Docker restart policy
// remains disabled so there is exactly one repair authority.
func (r *Reconciler) Run(ctx context.Context, spec ServiceSpec, stdout, stderr io.Writer) error {
	state, err := r.Apply(ctx, spec)
	if err != nil {
		_ = NotifySystemd("STATUS=OCI reconcile failed: " + err.Error())
		return err
	}
	_ = NotifySystemd("READY=1\nSTATUS=OCI service ready: " + state.ContainerName)

	containerID := state.ContainerID
	waitCh := make(chan error, 1)
	go func() {
		exitCode, waitErr := r.Runtime.WaitContainer(ctx, containerID)
		if waitErr != nil {
			waitCh <- Wrap(FailureStateInspection, "wait container", waitErr)
			return
		}
		if exitCode != 0 {
			waitCh <- fmt.Errorf("container %s exited with status %d", state.ContainerName, exitCode)
			return
		}
		waitCh <- nil
	}()

	var logsWG sync.WaitGroup
	logsWG.Add(1)
	go func() {
		defer logsWG.Done()
		_ = r.Runtime.StreamLogs(ctx, containerID, stdout, stderr)
	}()

	livenessCh := make(chan error, 1)
	if stringsProbeEnabled(spec.Spec.Health.Liveness) {
		go func() { livenessCh <- r.monitorLiveness(ctx, spec.Spec.Health.Liveness) }()
	}

	select {
	case <-ctx.Done():
		_ = NotifySystemd("STOPPING=1\nSTATUS=Stopping OCI service")
		stopCtx, cancel := context.WithTimeout(context.Background(), stopTimeout(spec)+5*time.Second)
		defer cancel()
		_, removeErr := r.Remove(stopCtx, spec)
		logsWG.Wait()
		if removeErr != nil {
			return removeErr
		}
		return nil
	case waitErr := <-waitCh:
		logsWG.Wait()
		latest, _ := r.Runtime.InspectContainer(context.WithoutCancel(ctx), state.ContainerName)
		state.Phase = PhaseStopped
		state.Ready = false
		state.Running = false
		state.ExitCode = latest.ExitCode
		if waitErr != nil {
			state.Phase = PhaseFailed
			state.FailureClass = FailureStateInspection
			state.Error = waitErr.Error()
		}
		_ = r.writeState(state)
		return waitErr
	case liveErr := <-livenessCh:
		_ = NotifySystemd("STATUS=OCI liveness failed: " + liveErr.Error())
		stopCtx, cancel := context.WithTimeout(context.Background(), stopTimeout(spec)+5*time.Second)
		defer cancel()
		_, _ = r.Remove(stopCtx, spec)
		logsWG.Wait()
		return Wrap(FailureLiveness, "liveness monitor", liveErr)
	}
}

func (r *Reconciler) monitorLiveness(ctx context.Context, probe Probe) error {
	interval := durationMillis(probe.IntervalMillis, 10*time.Second)
	threshold := probe.FailureThreshold
	if threshold <= 0 {
		threshold = 3
	}
	failures := 0
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if err := CheckProbe(ctx, probe); err != nil {
				failures++
				if failures >= threshold {
					return fmt.Errorf("failed %d consecutive checks: %w", failures, err)
				}
				continue
			}
			failures = 0
			_ = NotifySystemd("WATCHDOG=1\nSTATUS=OCI service healthy")
		}
	}
}

func stringsProbeEnabled(probe Probe) bool {
	return probe.Type != "" && probe.Type != "none" && probe.Type != "NONE"
}
