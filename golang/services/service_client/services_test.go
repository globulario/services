package service_client

//"encoding/json"
import (
	//	"io/ioutil"
	"log"
	"testing"

	"github.com/globulario/services/golang/services/servicespb"
)

var (
	// Connect to the services client.
	discovery_client, _  = NewPackagesDiscoveryService_Client("localhost", "services.PackageDiscovery")
	repository_client, _ = NewServicesRepositoryService_Client("localhost", "services.PackageRepository")
)

// Test publish a service.
func TestPublishPackageDescriptor(t *testing.T) {
	s := &servicespb.PackageDescriptor{
		Id:          "echo_server",
		PublisherId: "globulario",
		Version:     "1.0.0",
		Description: "Simple service with one function named Echo. It's mostly a test service.",
		Type:        servicespb.PackageType_APPLICATION,
		Keywords:    []string{"Test", "Echo"},
	}

	err := discovery_client.PublishPackageDescriptor(s)
	if err != nil {
		log.Println(err)
		return
	}
	log.Print("Service was publish with success!!!")
}

func TestGetPackageDescriptor(t *testing.T) {

	values, err := discovery_client.GetPackageDescriptor("echo_server", "globulario")
	if err != nil {
		log.Println(err)
		return
	}

	log.Print("Service was retreived with success!!!", values)
}

func TestFindPackagesDescriptor(t *testing.T) {
	values, err := discovery_client.FindServices([]string{"echo_server"})
	if err != nil {
		log.Panic(err)
	}
	log.Print("Services was retreived with success!!!", values)
}

/**
func TestUploadServiceBundle(t *testing.T) {

	// The service bundle...
	err := repository_client.UploadBundle("localhost", "echo_server", "localhost", "linux_amd64", "C:\\temp\\globular\\echo_server.7z")
	if err != nil {
		log.Panicln(err)
	}
}
*/

/*
func TestDownloadServiceBundle(t *testing.T) {
	bundle, err := repository_client.DownloadBundle("localhost", "echo_server", "localhost", "linux_amd64")

	if err != nil {
		log.Panicln(err)
	}

	ioutil.WriteFile("C:\\temp\\echo_server.7z", bundle.Binairies, 777)
}
*/
