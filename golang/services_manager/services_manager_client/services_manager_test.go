package service_manager_client

import (
	"log"
	"testing"
	"time"

	"github.com/globulario/services/golang/authentication/authentication_client"
)

// Test various function here.
func Test(t *testing.T) {
	t.Log("This is a test!")
	
}

var(
	token = ""
	domain="globular.cloud"
	discovery = "globular.cloud"
	organization = "globulario"
	client, _ = NewServicesManagerService_Client(domain, "services_manager.ServicesManagerService")
	authentication_client_, _= authentication_client.NewAuthenticationService_Client(domain, "authentication.AuthenticationService")
)

func TestInstallService(t *testing.T) {
	
	err := client.InstallService(token, domain, "sa", discovery, "globulario", "b94d0011-39a0-4bdb-9a5c-7e9abc23b26b")
	if err != nil {
		log.Panicln(err)
	}
	time.Sleep(time.Second * 5)
}


func TestStartService(t *testing.T) {
	log.Println("---> test get config.")
	token, err := authentication_client_.Authenticate("sa", "adminadmin")
	service_pid, proxy_pid, err := client.StartService("file.FileService")
	if err != nil {

		log.Println("---> ", err)
	}
	log.Println("service pid:", service_pid, " proxy pid:", proxy_pid)

}

func TestStopService(t *testing.T) {
	token, err := authentication_client_.Authenticate("sa", "adminadmin")
	if err != nil {
		log.Println("Fail to authenticate to globular.cloud")
		log.Println(err.Error())
		return
	}

	err = client.StopService("file.FileService")
	if err != nil {
		log.Println(err)
		return
	}
	log.Println("---> stop service succeeded")
}

func TestUninstallService(t *testing.T) {
	err := client.UninstallService("globulario", "b94d0011-39a0-4bdb-9a5c-7e9abc23b26b", "0.0.1")
	if err != nil {
		log.Panicln(err)
	}
}

func TestRestartServices(t *testing.T) {
	err := client.RestartServices()
	if err != nil {
		log.Println(err)
		t.FailNow()
	}
	log.Println("RestartServices succeed!")
}

func TestPublishService(t *testing.T) {
	err := client.PublishService("echo_server", "localhost:8080", "localhost:8080", "Echo is the simplest serive of all.", []string{"test", "echo"})
	if err != nil {
		log.Panicln(err)
	}
}
