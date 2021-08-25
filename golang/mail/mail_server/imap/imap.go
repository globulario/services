package imap

import (
	"crypto/tls"
	"fmt"
	"log"
	"time"

	"github.com/davecourtois/Utility"

	// "github.com/emersion/go-imap/backend/memory"
	imap_server "github.com/emersion/go-imap/server"
	"github.com/globulario/services/golang/persistence/persistence_client"
)

////////////////////////////////////////////////////////////////////////////////
// The Backend implementation
////////////////////////////////////////////////////////////////////////////////
var (
	Store            *persistence_client.Persistence_Client
	Backend_address  string
	Backend_port     int
	Backend_password string
)

/**
 * Save a message in the backend.
 */
func saveMessage(user string, mailBox string, body []byte, flags []string, date time.Time) error {

	data := make(map[string]interface{})

	data["Date"] = date
	data["Flags"] = flags
	data["Size"] = uint32(len(body))
	data["Body"] = body
	data["Uid"] = date.Unix() // I will use the unix time as Uid
	jsonStr, err := Utility.ToJson(data)
	if err != nil {
		return err
	}
	// Now I will insert the message into the inbox of the user.
	_, err = Store.InsertOne("local_ressource", user+"_db", mailBox, jsonStr, "")
	if err != nil {
		fmt.Println(err)
	}

	return err
}

/**
 * Rename the connection.
 */
func renameCollection(database string, name string, rename string) error {

	script := `db=db.getSiblingDB('admin');db.adminCommand({renameCollection:'` + database + `.` + name + `', to:'` + database + `.` + rename + `'})`
	err := Store.RunAdminCmd("local_ressource", "sa", Backend_password, script)
	return err
}

func startImap(port int, keyFile string, certFile string) {

	// Create backend instance.
	be := new(Backend_impl)

	// Create a new server
	s := imap_server.New(be)
	s.Addr = "0.0.0.0:" + Utility.ToString(port)

	go func() {
		if len(certFile) > 0 {
			cer, err := tls.LoadX509KeyPair(certFile, keyFile)
			if err != nil {
				log.Println(err)
				return
			}

			s.TLSConfig = &tls.Config{Certificates: []tls.Certificate{cer}}
			if err := s.ListenAndServeTLS(); err != nil {
				log.Fatal(err)
			}
		} else {
			// Since we will use this server for testing only, we can allow plain text
			// authentication over unencrypted connections
			s.AllowInsecureAuth = true

			if err := s.ListenAndServe(); err != nil {
				log.Fatal(err)
			}
		}
	}()
}

func StartImap(store *persistence_client.Persistence_Client, backend_address string, backend_port int, backend_password string, keyFile string, certFile string, port int, tls_port int, alt_port int) {

	// keep backend info
	store = store
	backend_password = backend_password
	backend_address = backend_address
	backend_port = backend_port

	// Create a memory backend
	startImap(port, "", "")
	startImap(tls_port, keyFile, certFile)
	startImap(alt_port, keyFile, certFile)
}
