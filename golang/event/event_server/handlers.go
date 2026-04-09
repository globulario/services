package main

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"github.com/globulario/services/golang/event/eventpb"
	"github.com/gocql/gocql"
	Utility "github.com/globulario/utility"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// busDisconnectLogLimiter rate-limits the "ScyllaDB bus not connected" error
// log to once per 30 seconds. Without this, a flood of Publish calls when the
// bus is down burns CPU on logging alone.
var busDisconnectLogLimiter atomic.Int64 // UnixNano of next allowed log

var (
	errMissingStream   = errors.New("event service: missing stream")
	errMissingUUID     = errors.New("event service: missing uuid")
	errMissingChanName = errors.New("event service: missing channel name")
)

func (srv *server) Stop(ctx context.Context, _ *eventpb.StopRequest) (*eventpb.StopResponse, error) {
	if srv.exit != nil {
		select {
		case srv.exit <- true:
		default:
		}
	}
	return &eventpb.StopResponse{}, srv.StopService()
}

// run drives the pub/sub loop. Subscriptions are local (tied to gRPC streams),
// but events are published and polled from ScyllaDB so all cluster instances
// see every event.
func (srv *server) run() {
	if srv.logger == nil {
		srv.logger = slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	}

	if srv.actions == nil {
		srv.actions = make(chan map[string]interface{}, 1024)
	}
	if srv.exit == nil {
		srv.exit = make(chan bool)
	}

	channels := make(map[string][]string)                          // channel -> uuids
	streams := make(map[string]eventpb.EventService_OnEventServer) // uuid -> stream
	quits := make(map[string]chan bool)                             // uuid -> quit
	ka := make(chan *eventpb.KeepAlive)

	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	// ScyllaDB poller — reads new events and dispatches to local subscribers.
	pollTicker := time.NewTicker(pollInterval)
	defer pollTicker.Stop()

	// Reconnect ticker — try to connect to ScyllaDB every 10s if bus is nil.
	reconnectTicker := time.NewTicker(10 * time.Second)
	defer reconnectTicker.Stop()

	done := make(chan bool)

	go func() {
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				ka <- &eventpb.KeepAlive{}
			}
		}
	}()

	srv.logger.Info("event loop started", "service", srv.Name, "id", srv.Id)

	for {
		select {
		case <-srv.exit:
			close(done)
			for uuid, q := range quits {
				select {
				case q <- true:
				default:
				}
				delete(quits, uuid)
				delete(streams, uuid)
			}
			srv.logger.Info("event loop stopped",
				"service", srv.Name,
				"id", srv.Id,
				"channels", len(channels),
				"streams", len(streams))
			return

		case ka_ := <-ka:
			var toDelete []string
			for uuid, stream := range streams {
				if stream == nil {
					toDelete = append(toDelete, uuid)
					continue
				}
				if err := stream.Send(&eventpb.OnEventResponse{Data: &eventpb.OnEventResponse_Ka{Ka: ka_}}); err != nil {
					srv.logger.Warn("keepalive send failed; will drop stream", "uuid", uuid, "err", err)
					toDelete = append(toDelete, uuid)
				}
			}
			if len(toDelete) > 0 {
				srv.cleanupSubscribers(toDelete, channels, quits, streams)
			}

		case <-reconnectTicker.C:
			// Periodically try to (re)connect the ScyllaDB bus.
			if srv.bus == nil {
				b := newScyllaBus(srv.logger)
				if err := b.connect(); err == nil {
					srv.logger.Info("ScyllaDB event bus reconnected")
					srv.bus = b
				} else {
					srv.logger.Warn("ScyllaDB event bus reconnect failed", "err", err)
				}
			}

		case <-pollTicker.C:
			// Poll ScyllaDB for new events from any instance.
			if srv.bus == nil {
				continue
			}
			events := srv.bus.pollOnce()
			for _, ev := range events {
				srv.dispatchToLocal(ev.name, ev.data, channels, streams, quits)
			}

		case a := <-srv.actions:
			action, _ := a["action"].(string)
			switch action {
			case "onevent":
				stream, _ := a["stream"].(eventpb.EventService_OnEventServer)
				uuid, _ := a["uuid"].(string)
				qc, ok := a["quit"].(chan bool)
				if stream == nil || uuid == "" || !ok {
					srv.logger.Error("invalid onevent request", "uuid", uuid, "has_stream", stream != nil, "has_quit", ok)
					continue
				}
				streams[uuid] = stream
				quits[uuid] = qc
				srv.logger.Info("stream registered", "uuid", uuid)

			case "subscribe":
				name, _ := a["name"].(string)
				uuid, _ := a["uuid"].(string)
				if name == "" || uuid == "" {
					srv.logger.Error("invalid subscribe request", "name", name, "uuid", uuid)
					continue
				}
				if channels[name] == nil {
					channels[name] = make([]string, 0)
				}
				if !Utility.Contains(channels[name], uuid) {
					channels[name] = append(channels[name], uuid)
					srv.logger.Info("subscribed", "channel", name, "uuid", uuid, "subscribers", len(channels[name]))
				}

			case "unsubscribe":
				name, _ := a["name"].(string)
				uuid, _ := a["uuid"].(string)
				if name == "" || uuid == "" {
					srv.logger.Error("invalid unsubscribe request", "name", name, "uuid", uuid)
					continue
				}
				uuids := make([]string, 0, len(channels[name]))
				for _, id := range channels[name] {
					if id != uuid {
						uuids = append(uuids, id)
					}
				}
				if len(uuids) == 0 {
					delete(channels, name)
				} else {
					channels[name] = uuids
				}
				srv.logger.Info("unsubscribed", "channel", name, "uuid", uuid, "remaining", len(channels[name]))

			case "quit":
				uuid, _ := a["uuid"].(string)
				if uuid == "" {
					srv.logger.Error("invalid quit request: missing uuid")
					continue
				}
				srv.cleanupSubscribers([]string{uuid}, channels, quits, streams)
				srv.logger.Info("stream quit", "uuid", uuid)

			default:
				srv.logger.Warn("unknown action", "action", action)
			}
		}
	}
}

// dispatchToLocal sends an event to all local subscribers whose channel
// pattern matches the event name.
func (srv *server) dispatchToLocal(
	name string, data []byte,
	channels map[string][]string,
	streams map[string]eventpb.EventService_OnEventServer,
	quits map[string]chan bool,
) {
	seen := make(map[string]bool)
	var matchedUUIDs []string
	for pattern, puuids := range channels {
		if matchesChannel(pattern, name) {
			for _, u := range puuids {
				if !seen[u] {
					seen[u] = true
					matchedUUIDs = append(matchedUUIDs, u)
				}
			}
		}
	}
	if len(matchedUUIDs) == 0 {
		return
	}
	var toDelete []string
	for _, uuid := range matchedUUIDs {
		stream := streams[uuid]
		if stream == nil {
			toDelete = append(toDelete, uuid)
			continue
		}
		err := stream.Send(&eventpb.OnEventResponse{
			Data: &eventpb.OnEventResponse_Evt{
				Evt: &eventpb.Event{Name: name, Data: data},
			},
		})
		if err != nil {
			srv.logger.Warn("event send failed; will drop subscriber", "channel", name, "uuid", uuid, "err", err)
			toDelete = append(toDelete, uuid)
		}
	}
	if len(toDelete) > 0 {
		srv.cleanupSubscribers(toDelete, channels, quits, streams)
	}
}

func (srv *server) cleanupSubscribers(
	toDelete []string,
	channels map[string][]string,
	quits map[string]chan bool,
	streams map[string]eventpb.EventService_OnEventServer,
) {
	for _, uuid := range toDelete {
		for name, ch := range channels {
			uuids := make([]string, 0, len(ch))
			for _, id := range ch {
				if id != uuid {
					uuids = append(uuids, id)
				}
			}
			if len(uuids) == 0 {
				delete(channels, name)
			} else {
				channels[name] = uuids
			}
		}

		if q, ok := quits[uuid]; ok {
			select {
			case q <- true:
			default:
			}
			delete(quits, uuid)
		}

		if _, ok := streams[uuid]; ok {
			delete(streams, uuid)
		}

		srv.logger.Info("subscriber cleanup", "uuid", uuid)
	}
}

// matchesChannel returns true if the subscription pattern matches the event name.
func matchesChannel(pattern, eventName string) bool {
	if pattern == eventName {
		return true
	}
	if strings.HasSuffix(pattern, ".*") {
		prefix := strings.TrimSuffix(pattern, "*")
		return strings.HasPrefix(eventName, prefix)
	}
	if pattern == "*" {
		return true
	}
	return false
}

func (srv *server) Quit(ctx context.Context, rqst *eventpb.QuitRequest) (*eventpb.QuitResponse, error) {
	if rqst == nil || rqst.Uuid == "" {
		srv.logger.Error("Quit: invalid request", "err", errMissingUUID)
		return &eventpb.QuitResponse{Result: false}, errMissingUUID
	}
	msg := map[string]interface{}{"action": "quit", "uuid": rqst.Uuid}
	srv.actions <- msg
	srv.logger.Info("Quit: ok", "uuid", rqst.Uuid)
	return &eventpb.QuitResponse{Result: true}, nil
}

func (srv *server) OnEvent(rqst *eventpb.OnEventRequest, stream eventpb.EventService_OnEventServer) error {
	if stream == nil {
		srv.logger.Error("OnEvent: missing stream", "err", errMissingStream)
		return errMissingStream
	}
	if rqst == nil || rqst.Uuid == "" {
		srv.logger.Error("OnEvent: invalid request", "err", errMissingUUID)
		return errMissingUUID
	}

	onevent := map[string]interface{}{
		"action": "onevent",
		"stream": stream,
		"uuid":   rqst.Uuid,
		"quit":   make(chan bool),
	}
	srv.actions <- onevent
	srv.logger.Info("OnEvent: registered", "uuid", rqst.Uuid)

	<-onevent["quit"].(chan bool)
	srv.logger.Info("OnEvent: stream ended", "uuid", rqst.Uuid)
	return nil
}

func (srv *server) Subscribe(ctx context.Context, rqst *eventpb.SubscribeRequest) (*eventpb.SubscribeResponse, error) {
	if rqst == nil || rqst.Uuid == "" {
		srv.logger.Error("Subscribe: invalid request", "err", errMissingUUID)
		return &eventpb.SubscribeResponse{Result: false}, errMissingUUID
	}
	if rqst.Name == "" {
		srv.logger.Error("Subscribe: invalid request", "err", errMissingChanName)
		return &eventpb.SubscribeResponse{Result: false}, errMissingChanName
	}
	subscribe := map[string]interface{}{"action": "subscribe", "name": rqst.Name, "uuid": rqst.Uuid}
	srv.actions <- subscribe
	srv.logger.Info("Subscribe: ok", "channel", rqst.Name, "uuid", rqst.Uuid)
	return &eventpb.SubscribeResponse{Result: true}, nil
}

func (srv *server) UnSubscribe(ctx context.Context, rqst *eventpb.UnSubscribeRequest) (*eventpb.UnSubscribeResponse, error) {
	if rqst == nil || rqst.Uuid == "" {
		srv.logger.Error("UnSubscribe: invalid request", "err", errMissingUUID)
		return &eventpb.UnSubscribeResponse{Result: false}, errMissingUUID
	}
	if rqst.Name == "" {
		srv.logger.Error("UnSubscribe: invalid request", "err", errMissingChanName)
		return &eventpb.UnSubscribeResponse{Result: false}, errMissingChanName
	}
	unsubscribe := map[string]interface{}{"action": "unsubscribe", "name": rqst.Name, "uuid": rqst.Uuid}
	srv.actions <- unsubscribe
	srv.logger.Info("UnSubscribe: ok", "channel", rqst.Name, "uuid", rqst.Uuid)
	return &eventpb.UnSubscribeResponse{Result: true}, nil
}

// Publish writes the event to ScyllaDB. All instances will pick it up via
// their poll loop and dispatch to local subscribers.
func (srv *server) Publish(ctx context.Context, rqst *eventpb.PublishRequest) (*eventpb.PublishResponse, error) {
	if rqst == nil || rqst.Evt == nil || rqst.Evt.Name == "" {
		srv.logger.Error("Publish: invalid request", "err", errMissingChanName)
		return &eventpb.PublishResponse{Result: false}, errMissingChanName
	}
	if srv.bus == nil {
		// Rate-limit this log to avoid CPU burn from flood of publish calls.
		now := time.Now().UnixNano()
		if next := busDisconnectLogLimiter.Load(); now >= next {
			busDisconnectLogLimiter.Store(time.Now().Add(30 * time.Second).UnixNano())
			srv.logger.Error("Publish: ScyllaDB bus not connected (suppressing further logs for 30s)")
		}
		return &eventpb.PublishResponse{Result: false}, errors.New("event bus not connected")
	}
	if err := srv.bus.publish(rqst.Evt.Name, rqst.Evt.Data); err != nil {
		srv.logger.Error("Publish: ScyllaDB write failed", "event", rqst.Evt.Name, "err", err)
		return &eventpb.PublishResponse{Result: false}, err
	}
	return &eventpb.PublishResponse{Result: true}, nil
}

// QueryEvents reads recent events from ScyllaDB.
func (srv *server) QueryEvents(_ context.Context, rqst *eventpb.QueryEventsRequest) (*eventpb.QueryEventsResponse, error) {
	if srv.bus == nil {
		return &eventpb.QueryEventsResponse{}, nil
	}

	nameFilter := ""
	if rqst != nil {
		nameFilter = rqst.GetNameFilter()
	}
	limit := 100
	if rqst != nil && rqst.GetLimit() > 0 {
		limit = int(rqst.GetLimit())
	}

	// Convert afterSequence to a TimeUUID for the ScyllaDB query.
	var afterSeq gocql.UUID
	if rqst != nil && rqst.GetAfterSequence() > 0 {
		// Map the uint64 sequence to a time-based UUID.
		// afterSequence was from the ring buffer era; for ScyllaDB we use
		// the zero UUID to start from the beginning.
		afterSeq = gocql.MinTimeUUID(time.Now().Add(-10 * time.Minute))
	}

	events, _ := srv.bus.queryEvents(nameFilter, afterSeq, limit)

	var out []*eventpb.PersistedEvent
	for _, ev := range events {
		out = append(out, &eventpb.PersistedEvent{
			Name:     ev.name,
			Data:     ev.data,
			Ts:       timestamppb.New(ev.seq.Time()),
			Sequence: uint64(ev.seq.Time().UnixNano()),
		})
	}

	var latestSeq uint64
	if len(out) > 0 {
		latestSeq = out[len(out)-1].Sequence
	}

	return &eventpb.QueryEventsResponse{
		Events:         out,
		LatestSequence: latestSeq,
	}, nil
}
