package main

import (
	"fmt"
	"os"
	"time"

	"github.com/globulario/services/golang/event/eventpb"
	"github.com/globulario/services/golang/event/event_client"
)

func main() {
	addr := "localhost:10010"
	if len(os.Args) > 1 {
		addr = os.Args[1]
	}

	// Subscriber client
	sub, err := event_client.NewEventService_Client(addr, "event.EventService")
	if err != nil {
		fmt.Println("subscriber connect failed:", err)
		os.Exit(1)
	}
	fmt.Println("subscriber connected")

	received := make(chan string, 10)

	err = sub.Subscribe("test.*", "test-sub-1", func(evt *eventpb.Event) {
		fmt.Println("RECEIVED:", evt.GetName(), string(evt.GetData()))
		received <- evt.GetName()
	})
	if err != nil {
		fmt.Println("subscribe failed:", err)
		os.Exit(1)
	}
	fmt.Println("subscribed to test.*")

	time.Sleep(2 * time.Second)

	// Publisher client
	pub, err := event_client.NewEventService_Client(addr, "event.EventService")
	if err != nil {
		fmt.Println("publisher connect failed:", err)
		os.Exit(1)
	}
	fmt.Println("publisher connected")

	err = pub.Publish("test.something", []byte(`{"msg":"hello from wildcard test"}`))
	if err != nil {
		fmt.Println("publish failed:", err)
		os.Exit(1)
	}
	fmt.Println("published test.something")

	// Wait for delivery
	select {
	case name := <-received:
		fmt.Println("SUCCESS: wildcard subscription received event", name)
	case <-time.After(10 * time.Second):
		fmt.Println("FAIL: no event received after 10s")
	}
}
