package event_client

import (
	"log"
	"strconv"
	"testing"
	"time"

	"github.com/davecourtois/Utility"
	"github.com/globulario/services/golang/event/eventpb"
)

func subscribeTo(client *Event_Client, subject string) string {
	fct := func(evt *eventpb.Event) {
		log.Println("---> event received: ", string(evt.Data))
	}

	uuid := Utility.RandomUUID()
	err := client.Subscribe(subject, uuid, fct)
	if err != nil {
		log.Println("---> err", err)
	}
	return uuid
}

/**
 * Test event
 */

func _TestEventService(t *testing.T) {
	log.Println("Test event service")
	domain := "globular.live"

	// The topic.
	subject := "my topic"
	size := 10 // test with 500 client...
	clients := make([]*Event_Client, size)
	uuids := make([]string, size)
	for i := 0; i < size; i++ {
		c, err := NewEventService_Client(domain, "event.EventService")
		if err != nil {
			log.Panicln("---> err", err)
		}
		uuids[i] = subscribeTo(c, subject)
		log.Println("client ", i)
		clients[i] = c
	}

	for i := 0; i < size; i++ {
		clients[0].Publish(subject, []byte("--->"+strconv.Itoa(i)+" this is a message! "+Utility.ToString(i)))
	}

	// Here I will simply suspend this thread to give time to publish message
	time.Sleep(time.Second * 1)

	for i := 0; i < size; i++ {
		log.Println("---> close the client")
		clients[i].UnSubscribe(subject, uuids[i])
	}

}

func TestPublishEvent(t *testing.T) {
	log.Println("test event service.")
	c, err := NewEventService_Client("localhost", "event.EventService")
	if err != nil {
		log.Println("fail to connect to event service ", err)
	}

	// Here I will publish a simple string...

	c.Publish("on_echo_event", []byte("This is a simple test!"))
}
