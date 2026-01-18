package media_client

import (
	"log"
	"testing"

	"github.com/globulario/services/golang/testutil"
)

// newTestClient creates a client for testing, skipping if external services are not available.
func newTestClient(t *testing.T) *Media_Client {
	t.Helper()
	testutil.SkipIfNoExternalServices(t)

	addr := testutil.GetAddress()
	client, err := NewMediaService_Client(addr, "media.MediaService")
	if err != nil {
		t.Fatalf("NewMediaService_Client: %v", err)
	}
	return client
}

// Test connection and basic client operations.
func TestMedia(t *testing.T) {
	client := newTestClient(t)

	// Test basic client operations
	log.Println("Media client connected successfully")
	log.Println("Domain:", client.GetDomain())
	log.Println("Address:", client.GetAddress())
}
