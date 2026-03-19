package event_client

import (
	"context"
	"errors"
	"sync/atomic"
	"time"

	"github.com/globulario/services/golang/event/eventpb"
	globular "github.com/globulario/services/golang/globular_client"
	"github.com/globulario/services/golang/security"
	Utility "github.com/globulario/utility"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

////////////////////////////////////////////////////////////////////////////////
// echo Client Service
////////////////////////////////////////////////////////////////////////////////

type Event_Client struct {
	cc *grpc.ClientConn
	c  eventpb.EventServiceClient

	// The id of the service
	id string

	// The name of the service
	name string

	// The client domain
	domain string

	// The address where connection with client can be done. ex: globule0.globular.cloud:10101
	address string

	//  keep the last connection state of the client.
	state string

	// The mac address of the server
	mac string

	// The port
	port int

	// is the connection is secure?
	hasTLS bool

	// Link to client key file
	keyFile string

	// Link to client certificate file.
	certFile string

	// certificate authority file
	caFile string

	// The client context
	ctx context.Context

	// the client uuid.
	uuid string

	// The event channel.
	actions chan map[string]interface{}

	// Return true if the client is connected.
	isConnected atomic.Bool

	// it will be started at first subjecribe...
	isRunning atomic.Bool
}

// Create a connection to the service.
func NewEventService_Client(address string, id string) (*Event_Client, error) {
	client := new(Event_Client)

	err := globular.InitClient(client, address, id)
	if err != nil {
		return nil, err
	}

	client.uuid = Utility.RandomUUID()

	// The channel where data will be exchange.
	client.actions = make(chan map[string]interface{}, 64)

	err = client.Reconnect()
	if err != nil {
		return nil, err
	}

	return client, nil
}

// Try to connect...
func (client *Event_Client) Reconnect() error {
	var err error
	nb_try_connect := 10

	for i := 0; i < nb_try_connect; i++ {
		client.cc, err = globular.GetClientConnection(client)
		if err == nil {
			client.c = eventpb.NewEventServiceClient(client.cc)
			break
		}

		// wait 500 millisecond before next try
		time.Sleep(500 * time.Millisecond)
	}

	return err
}

/**
 * Process event from the server. Only one stream is needed between the server
 * and the client. Local handler are kept in a map with a unique uuid, so many
 * handler can exist for a single event.
 */
func (client *Event_Client) run() error {

	// Create the channel.
	data_channel := make(chan *eventpb.Event, 100)
	keep_alive := make(chan *eventpb.KeepAlive, 1)
	exit := make(chan bool, 1)

	// start listenting to events from the server...
	err := client.onEvent(client.uuid, data_channel, keep_alive, exit)
	if err != nil {
		return err
	}

	// the map that will contain the event handler.
	handlers := make(map[string]map[string]func(*eventpb.Event))
	client.isRunning.Store(true)

	//

	for {
		select {
		case <-exit:

			// Close old connection before reconnecting.
			if client.cc != nil {
				client.cc.Close()
			}

			// Give the server a moment to come back.
			time.Sleep(2 * time.Second)

			/** So here I will try to reconnect **/
			err := client.Reconnect()
			if err != nil {
				return err
			}

			// Drain any stale exit signals from the old stream goroutine.
			select {
			case <-exit:
			default:
			}

			nb_try_connect := 10

			for i := 0; i < nb_try_connect; i++ {
				// Now I will reconnect the stream...
				err = client.onEvent(client.uuid, data_channel, keep_alive, exit)
				if err == nil {
					// Re-subscribe all handlers on the server.
					for name, listeners := range handlers {
						for uuid := range listeners {
							rqst := &eventpb.SubscribeRequest{
								Name: name,
								Uuid: uuid,
							}
							_, err := client.c.Subscribe(client.GetCtx(), rqst)
							if err != nil {
								return err
							}
						}
					}

					break
				}

				time.Sleep(500 * time.Millisecond)
			}

			if err != nil {
				return err
			}
			
		case <-keep_alive:
			/** Nothing to do here...**/

		case evt := <-data_channel:
			// So here I received an event, I will dispatch it to it function.
			handlers_ := handlers[evt.Name]
			for _, fct := range handlers_ {
				// Call the handler.
				fct(evt)
			}
		case action := <-client.actions:
			if action["action"].(string) == "subscribe" {
				if handlers[action["name"].(string)] == nil {
					handlers[action["name"].(string)] = make(map[string]func(*eventpb.Event))
				}
				// Set it handler.
				handlers[action["name"].(string)][action["uuid"].(string)] = action["fct"].(func(*eventpb.Event))
			} else if action["action"].(string) == "unsubscribe" {
				// Now I will remove the handler...
				for _, handler := range handlers {
					if handler[action["uuid"].(string)] != nil {
						delete(handler, action["uuid"].(string))
					}
				}
			} else if action["action"].(string) == "stop" {
				client.isConnected.Store(false)
				client.isRunning.Store(false)
				break
			}
		}
	}

}

// The address where the client can connect.
func (client *Event_Client) SetAddress(address string) {
	client.address = address
}

func (client *Event_Client) Invoke(method string, rqst interface{}, ctx context.Context) (interface{}, error) {
	if ctx == nil {
		ctx = client.GetCtx()
	}
	return globular.InvokeClientRequest(client.c, ctx, method, rqst)
}

func (client *Event_Client) GetCtx() context.Context {
	if client.ctx == nil {
		client.ctx = globular.GetClientContext(client)
	}
	token, err := security.GetLocalToken(client.GetMac())
	if err == nil {
		md := metadata.New(map[string]string{"token": string(token), "domain": client.domain, "mac": client.mac})
		client.ctx = metadata.NewOutgoingContext(context.Background(), md)
	}
	return client.ctx
}

// Return the domain
func (client *Event_Client) GetDomain() string {
	return client.domain
}

// Return the address
func (client *Event_Client) GetAddress() string {
	return client.address
}

// Return the last know connection state
func (client *Event_Client) GetState() string {
	return client.state
}

// Return the id of the service instance
func (client *Event_Client) GetId() string {
	return client.id
}

// Return the name of the service
func (client *Event_Client) GetName() string {
	return client.name
}

func (client *Event_Client) GetMac() string {
	return client.mac
}

// must be close when no more needed.
func (client *Event_Client) Close() {

	// nothing to do if the client is not connected.
	if !client.isConnected.Load() && !client.isRunning.Load() {
		return
	}

	client.isConnected.Store(false)

	// Signal the run loop to stop (non-blocking in case it already exited).
	select {
	case client.actions <- map[string]interface{}{"action": "stop"}:
	default:
	}

	if client.cc != nil {
		client.cc.Close()
	}
}

// Set grpc_service port.
func (client *Event_Client) SetPort(port int) {
	client.port = port
}

// Return the grpc port number
func (client *Event_Client) GetPort() int {
	return client.port
}

// Set the client instance id.
func (client *Event_Client) SetId(id string) {
	client.id = id
}

// Set the client name.
func (client *Event_Client) SetName(name string) {
	client.name = name
}

func (client *Event_Client) SetMac(mac string) {
	client.mac = mac
}

// Set the domain.
func (client *Event_Client) SetDomain(domain string) {
	client.domain = domain
}

func (client *Event_Client) SetState(state string) {
	client.state = state
}

////////////////// TLS ///////////////////

// Get if the client is secure.
func (client *Event_Client) HasTLS() bool {
	return client.hasTLS
}

// Get the TLS certificate file path
func (client *Event_Client) GetCertFile() string {
	return client.certFile
}

// Get the TLS key file path
func (client *Event_Client) GetKeyFile() string {
	return client.keyFile
}

// Get the TLS key file path
func (client *Event_Client) GetCaFile() string {
	return client.caFile
}

// Set the client is a secure client.
func (client *Event_Client) SetTLS(hasTls bool) {
	client.hasTLS = hasTls
}

// Set TLS certificate file path
func (client *Event_Client) SetCertFile(certFile string) {
	client.certFile = certFile
}

// Set TLS key file path
func (client *Event_Client) SetKeyFile(keyFile string) {
	client.keyFile = keyFile
}

// Set TLS authority trust certificate file path
func (client *Event_Client) SetCaFile(caFile string) {
	client.caFile = caFile
}

// /////////////////// API ///////////////////////
// Stop the service.
func (client *Event_Client) StopService() {
	client.c.Stop(client.GetCtx(), &eventpb.StopRequest{})
}

// Publish and event over the network
func (client *Event_Client) Publish(name string, data []byte) error {

	rqst := &eventpb.PublishRequest{
		Evt: &eventpb.Event{
			Name: name,
			Data: data,
		},
	}

	_, err := client.c.Publish(client.GetCtx(), rqst)
	if err != nil {
		return err
	}
	return nil
}

func (client *Event_Client) onEvent(uuid string, data_channel chan *eventpb.Event, keep_alive chan *eventpb.KeepAlive, exit chan bool) error {

	rqst := &eventpb.OnEventRequest{
		Uuid: uuid,
	}

	stream, err := client.c.OnEvent(client.GetCtx(), rqst)
	if err != nil {
		return err
	}

	client.isConnected.Store(true)

	// Run in it own goroutine.
	go func() {
		for {
			msg, err := stream.Recv()
			if err != nil || !client.isConnected.Load() || msg == nil {
				// Stream broke — signal reconnect (don't call Close which stops the run loop).
				client.isConnected.Store(false)
				select {
				case exit <- true:
				default:
				}
				return
			}

			// Get the result...
			switch op := msg.Data.(type) {
			case *eventpb.OnEventResponse_Evt:
				select {
				case data_channel <- op.Evt:
				default:
					// drop event if handler is too slow
				}
			case *eventpb.OnEventResponse_Ka:
				select {
				case keep_alive <- op.Ka:
				default:
				}
			}
		}

	}()

	// Wait for subscriber uuid and return it to the function caller.
	return nil
}

/**
 * Maximize chance to connect with the event server...
 **/
func (client *Event_Client) Subscribe(name string, uuid string, fct func(evt *eventpb.Event)) error {
	if !client.isRunning.Load() {
		// Open a connection with the server. In case the server is not readyz
		// It will wait 5 second and try it again.
		nb_try_connect := 10

		go func() {
			for nb_try_connect > 0 {
				err := client.run()
				if err != nil && nb_try_connect == 0 {
					return
				}
				time.Sleep(500 * time.Millisecond) // wait five seconds.
				nb_try_connect--
			}
		}()
	}

	var lastErr error
	for nbTry := 30; nbTry > 0; nbTry-- {
		lastErr = client.subscribe(name, uuid, fct)
		if lastErr == nil {
			return nil
		}
		time.Sleep(1 * time.Second)
	}

	return errors.New("fail to subscribe to " + name + ": " + lastErr.Error())
}

// Subscribe to an event it return it subscriber uuid. The uuid must be use
// to unsubscribe from the channel. data_channel is use to get event data.
func (client *Event_Client) subscribe(name string, uuid string, fct func(evt *eventpb.Event)) error {
	rqst := &eventpb.SubscribeRequest{
		Name: name,
		Uuid: client.uuid,
	}

	_, err := client.c.Subscribe(client.GetCtx(), rqst)
	if err != nil {
		return err
	}

	action := make(map[string]interface{})
	action["action"] = "subscribe"
	action["uuid"] = uuid
	action["name"] = name
	action["fct"] = fct

	// set the action.
	client.actions <- action

	return nil
}

// Exit event channel.
func (client *Event_Client) UnSubscribe(name string, uuid string) error {

	// Unsubscribe from the event channel.
	rqst := &eventpb.UnSubscribeRequest{
		Name: name,
		Uuid: client.uuid,
	}

	_, err := client.c.UnSubscribe(client.GetCtx(), rqst)
	if err != nil {
		return err
	}

	action := make(map[string]interface{})
	action["action"] = "unsubscribe"
	action["uuid"] = uuid
	action["name"] = name

	// set the action.
	client.actions <- action

	return nil
}

// In event_client.Event_Client


// SubscribeCtx subscribes to events with the specified name and uuid, using the provided context and handler function.
// If the context is nil, it uses the client's default context. The subscription is registered both locally and on the server.
// The handler function fct will be called for each received event. The subscription is automatically unsubscribed when the context is done.
// Returns an error if the server-side registration fails.
func (client *Event_Client) SubscribeCtx(ctx context.Context, name, uuid string, fct func(*eventpb.Event)) error {
    if ctx == nil {
        ctx = client.GetCtx()
    }
    // ensure the background loop is running
    if !client.isRunning.Load() {
        go client.run()
    }

    // server-side registration
    rqst := &eventpb.SubscribeRequest{Name: name, Uuid: client.uuid}
    if _, err := client.c.Subscribe(ctx, rqst); err != nil {
        return err
    }

    // register local handler
    action := map[string]interface{}{
        "action": "subscribe",
        "uuid":   uuid,
        "name":   name,
        "fct":    fct,
    }
    client.actions <- action

    // auto-unsubscribe when ctx is done
    go func() {
        <-ctx.Done()
        _ = client.UnSubscribe(name, uuid) // best-effort
    }()
    return nil
}


// UnSubscribeCtx unsubscribes the client from an event stream identified by name and uuid.
// If the provided context is nil, it uses the client's default context.
// It sends an UnSubscribeRequest to the event service and, upon success,
// notifies the client's actions channel about the unsubscription.
// Returns an error if the unsubscribe operation fails.
func (client *Event_Client) UnSubscribeCtx(ctx context.Context, name, uuid string) error {
    if ctx == nil {
        ctx = client.GetCtx()
    }
    rqst := &eventpb.UnSubscribeRequest{Name: name, Uuid: client.uuid}
    if _, err := client.c.UnSubscribe(ctx, rqst); err != nil {
        return err
    }
    action := map[string]interface{}{"action": "unsubscribe", "uuid": uuid, "name": name}
    client.actions <- action
    return nil
}