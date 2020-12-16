package postprocessor_client

import (
	//"encoding/json"
	"log"
	"testing"

	"github.com/globulario/Globular/postprocessor/postprocessor_client"
)

var (
	client, err := postprocessor_client.NewPostprocessor_Client("localhost", "postprocessor.postprocessorService")
)

// Test various function here.
func TestPostprocessor(t *testing.T) {

	// Connect to the plc client.
	log.Println("--------> no implemented!")
}
