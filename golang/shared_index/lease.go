package shared_index

import (
	"context"
	"fmt"
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/globulario/services/golang/config"
	"go.etcd.io/etcd/client/v3/concurrency"
)

const (
	defaultLeaseTTL = 30 // seconds
)

// writerLease manages an etcd-based lease for writer election.
// Only one instance at a time holds the lease and processes the index queue.
type writerLease struct {
	group    string // "search", "title", "blog"
	isWriter atomic.Bool
	cancel   context.CancelFunc
	logger   *slog.Logger
}

func newWriterLease(group string, logger *slog.Logger) *writerLease {
	return &writerLease{group: group, logger: logger}
}

// Campaign starts the election loop. It blocks until the context is cancelled.
// When this instance wins, isWriter becomes true and onElected is called.
// When the lease is lost, isWriter becomes false and onLost is called.
func (wl *writerLease) Campaign(ctx context.Context, onElected, onLost func()) {
	ctx, wl.cancel = context.WithCancel(ctx)

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			if err := wl.runElection(ctx, onElected, onLost); err != nil {
				wl.logger.Warn("election round failed, retrying", "group", wl.group, "err", err)
				wl.isWriter.Store(false)
				onLost()
				select {
				case <-ctx.Done():
					return
				case <-time.After(5 * time.Second):
				}
			}
		}
	}()
}

func (wl *writerLease) runElection(ctx context.Context, onElected, onLost func()) error {
	client, err := config.NewEtcdClient()
	if err != nil {
		return fmt.Errorf("etcd client: %w", err)
	}
	defer client.Close()

	sess, err := concurrency.NewSession(client, concurrency.WithTTL(defaultLeaseTTL))
	if err != nil {
		return fmt.Errorf("etcd session: %w", err)
	}
	defer sess.Close()

	electionKey := fmt.Sprintf("/globular/sharedindex/%s/writer", wl.group)
	election := concurrency.NewElection(sess, electionKey)

	hostname, _ := config.GetHostname()
	wl.logger.Info("campaigning for writer", "group", wl.group, "key", electionKey)

	if err := election.Campaign(ctx, hostname); err != nil {
		return fmt.Errorf("campaign: %w", err)
	}

	wl.logger.Info("elected as writer", "group", wl.group)
	wl.isWriter.Store(true)
	onElected()

	// Wait until session expires or context cancelled.
	select {
	case <-sess.Done():
		wl.logger.Warn("writer session lost", "group", wl.group)
		wl.isWriter.Store(false)
		onLost()
	case <-ctx.Done():
		wl.isWriter.Store(false)
		_ = election.Resign(context.Background())
		onLost()
	}
	return nil
}

// IsWriter returns true if this instance currently holds the writer lease.
func (wl *writerLease) IsWriter() bool {
	return wl.isWriter.Load()
}

// Stop cancels the election loop and resigns the lease.
func (wl *writerLease) Stop() {
	if wl.cancel != nil {
		wl.cancel()
	}
	wl.isWriter.Store(false)
}

