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
func TestEventService(t *testing.T) {
	log.Println("Test event service")
	domain := "hub.globular.io"

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