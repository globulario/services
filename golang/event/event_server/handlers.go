package main

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"time"

	"github.com/globulario/services/golang/event/eventpb"
	Utility "github.com/globulario/utility"
)

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

// run drives the in-memory pub/sub loop.
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
	quits := make(map[string]chan bool)                            // uuid -> quit
	ka := make(chan *eventpb.KeepAlive)

	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
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

			case "publish":
				name, _ := a["name"].(string)
				data, _ := a["data"].([]byte)
				if name == "" {
					srv.logger.Error("invalid publish request: missing channel name")
					continue
				}
				uuids := channels[name]
				if uuids == nil {
					continue
				}
				var toDelete []string
				for _, uuid := range uuids {
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

func (srv *server) Publish(ctx context.Context, rqst *eventpb.PublishRequest) (*eventpb.PublishResponse, error) {
	if rqst == nil || rqst.Evt == nil || rqst.Evt.Name == "" {
		srv.logger.Error("Publish: invalid request", "err", errMissingChanName)
		return &eventpb.PublishResponse{Result: false}, errMissingChanName
	}
	publish := map[string]interface{}{"action": "publish", "name": rqst.Evt.Name, "data": rqst.Evt.Data}
	srv.actions <- publish
	return &eventpb.PublishResponse{Result: true}, nil
}
