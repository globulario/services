package event_client

import (
	"errors"
	"fmt"
	"strconv"

	"log"
	"time"

	"context"

	"github.com/davecourtois/Utility"
	"github.com/globulario/services/golang/event/eventpb"
	globular "github.com/globulario/services/golang/globular_client"
	"google.golang.org/grpc"
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

	// the client uuid.
	uuid string

	// The event channel.
	actions chan map[string]interface{}

	// Return true if the client is connected.
	isConnected bool
}

// Create a connection to the service.
func NewEventService_Client(address string, id string) (*Event_Client, error) {
	client := new(Event_Client)

	err := globular.InitClient(client, address, id)
	if err != nil {
		return nil, err
	}

	client.cc, err = globular.GetClientConnection(client)
	if err != nil {
		return nil, err
	}

	client.c = eventpb.NewEventServiceClient(client.cc)
	client.uuid = Utility.RandomUUID()

	// The channel where data will be exchange.
	client.actions = make(chan map[string]interface{})

	// Open a connection with the server. In case the server is not readyz
	// It will wait 5 second and try it again.
	nb_try_connect := 10
	go func() {
		for nb_try_connect > 0 {
			err := client.run()
			if err != nil && nb_try_connect == 0 {
				fmt.Println("Fail to create event client: ", address, id, err)
				return
			}
			time.Sleep(500 * time.Millisecond) // wait five seconds.
			nb_try_connect--
		}
	}()

	return client, nil
}

/**
 * Process event from the server. Only one stream is needed between the server
 * and the client. Local handler are kept in a map with a unique uuid, so many
 * handler can exist for a single event.
 */
func (event_client *Event_Client) run() error {

	// Create the channel.
	data_channel := make(chan *eventpb.Event, 0)

	// start listenting to events from the server...
	err := event_client.onEvent(event_client.uuid, data_channel)
	if err != nil {
		return err
	}

	// the map that will contain the event handler.
	handlers := make(map[string]map[string]func(*eventpb.Event))

	for {
		select {
		case evt := <-data_channel:
			// So here I received an event, I will dispatch it to it function.
			handlers_ := handlers[evt.Name]
			for _, fct := range handlers_ {
				// Call the handler.
				fct(evt)
			}
		case action := <-event_client.actions:
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
				event_client.isConnected = false
				break
			}
		}
	}

}

func (event_client *Event_Client) Invoke(method string, rqst interface{}, ctx context.Context) (interface{}, error) {
	if ctx == nil {
		ctx = globular.GetClientContext(event_client)
	}
	return globular.InvokeClientRequest(event_client.c, ctx, method, rqst)
}

// Return the domain
func (event_client *Event_Client) GetDomain() string {
	return event_client.domain
}

// Return the address
func (event_client *Event_Client) GetAddress() string {
	return event_client.domain + ":" + strconv.Itoa(event_client.port)
}

// Return the id of the service instance
func (event_client *Event_Client) GetId() string {
	return event_client.id
}

// Return the name of the service
func (event_client *Event_Client) GetName() string {
	return event_client.name
}

func (event_client *Event_Client) GetMac() string {
	return event_client.mac
}

// must be close when no more needed.
func (event_client *Event_Client) Close() {

	// nothing to do if the client is not connected.
	if !event_client.isConnected {
		return
	}

	event_client.cc.Close()

	action := make(map[string]interface{})
	action["action"] = "stop"
	// set the action.
	event_client.actions <- action
}

// Set grpc_service port.
func (event_client *Event_Client) SetPort(port int) {
	event_client.port = port
}

// Set the client instance id.
func (event_client *Event_Client) SetId(id string) {
	event_client.id = id
}

// Set the client name.
func (event_client *Event_Client) SetName(name string) {
	event_client.name = name
}

func (event_client *Event_Client) SetMac(mac string) {
	event_client.mac = mac
}

// Set the domain.
func (event_client *Event_Client) SetDomain(domain string) {
	event_client.domain = domain
}

////////////////// TLS ///////////////////

// Get if the client is secure.
func (event_client *Event_Client) HasTLS() bool {
	return event_client.hasTLS
}

// Get the TLS certificate file path
func (event_client *Event_Client) GetCertFile() string {
	return event_client.certFile
}

// Get the TLS key file path
func (event_client *Event_Client) GetKeyFile() string {
	return event_client.keyFile
}

// Get the TLS key file path
func (event_client *Event_Client) GetCaFile() string {
	return event_client.caFile
}

// Set the client is a secure client.
func (event_client *Event_Client) SetTLS(hasTls bool) {
	event_client.hasTLS = hasTls
}

// Set TLS certificate file path
func (event_client *Event_Client) SetCertFile(certFile string) {
	event_client.certFile = certFile
}

// Set TLS key file path
func (event_client *Event_Client) SetKeyFile(keyFile string) {
	event_client.keyFile = keyFile
}

// Set TLS authority trust certificate file path
func (event_client *Event_Client) SetCaFile(caFile string) {
	event_client.caFile = caFile
}

///////////////////// API ///////////////////////
// Stop the service.
func (event_client *Event_Client) StopService() {
	event_client.c.Stop(globular.GetClientContext(event_client), &eventpb.StopRequest{})
}

// Publish and event over the network
func (event_client *Event_Client) Publish(name string, data interface{}) error {
	rqst := &eventpb.PublishRequest{
		Evt: &eventpb.Event{
			Name: name,
			Data: data.([]byte),
		},
	}

	_, err := event_client.c.Publish(globular.GetClientContext(event_client), rqst)
	if err != nil {
		return err
	}

	return nil
}

func (event_client *Event_Client) onEvent(uuid string, data_channel chan *eventpb.Event) error {

	rqst := &eventpb.OnEventRequest{
		Uuid: uuid,
	}

	stream, err := event_client.c.OnEvent(globular.GetClientContext(event_client), rqst)
	if err != nil {
		return err
	}

	event_client.isConnected = true

	// Run in it own goroutine.
	go func() {
		for {
			msg, err := stream.Recv()
			if err != nil || !event_client.isConnected || msg == nil {
				// end of stream...
				event_client.Close()
				stream.CloseSend()
				return
			}

			// Get the result...
			data_channel <- msg.Evt
		}

	}()

	// Wait for subscriber uuid and return it to the function caller.
	return nil
}

/**
 * Maximize chance to connect with the event server...
 **/
func (event_client *Event_Client) Subscribe(name string, uuid string, fct func(evt *eventpb.Event)) error {
	registered := false

	for nbTry := 30; !registered && nbTry > 0; nbTry-- {
		err := event_client.subscribe(name, uuid, fct)
		if err == nil {
			log.Println("subscription to ", name, " succeed!")
			registered = true
		} else {
			nbTry--
			time.Sleep(2 * time.Second)
		}
	}

	if !registered {
		return errors.New("fail to subscribe to " + name)
	}

	return nil
}

// Subscribe to an event it return it subscriber uuid. The uuid must be use
// to unsubscribe from the channel. data_channel is use to get event data.
func (event_client *Event_Client) subscribe(name string, uuid string, fct func(evt *eventpb.Event)) error {
	rqst := &eventpb.SubscribeRequest{
		Name: name,
		Uuid: event_client.uuid,
	}

	_, err := event_client.c.Subscribe(globular.GetClientContext(event_client), rqst)
	if err != nil {
		return err
	}

	action := make(map[string]interface{})
	action["action"] = "subscribe"
	action["uuid"] = uuid
	action["name"] = name
	action["fct"] = fct

	// set the action.
	event_client.actions <- action

	return nil
}

// Exit event channel.
func (event_client *Event_Client) UnSubscribe(name string, uuid string) error {

	// Unsubscribe from the event channel.
	rqst := &eventpb.UnSubscribeRequest{
		Name: name,
		Uuid: event_client.uuid,
	}

	_, err := event_client.c.UnSubscribe(globular.GetClientContext(event_client), rqst)
	if err != nil {
		return err
	}

	action := make(map[string]interface{})
	action["action"] = "unsubscribe"
	action["uuid"] = uuid
	action["name"] = name

	// set the action.
	event_client.actions <- action

	return nil
}
