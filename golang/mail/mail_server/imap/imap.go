package imap

import (
	"crypto/tls"
	"fmt"
	"log"
	"time"

	imap_server "github.com/emersion/go-imap/server"
	"github.com/globulario/services/golang/persistence/persistence_client"
	Utility "github.com/globulario/utility"
)

// Global variables for backend configuration
var (
	Store            *persistence_client.Persistence_Client
	Backend_address  string
	Backend_port     int
	Backend_password string
)

// saveMessage stores an IMAP message in the backend database.
func saveMessage(user string, mailBox string, body []byte, flags []string, date time.Time) error {
	fmt.Println("---- imap ----> Saving message in the backend.")

	// Prepare the message data
	data := make(map[string]interface{})
	data["Date"] = date
	data["Flags"] = flags
	data["Size"] = uint32(len(body))
	data["Body"] = body
	data["Uid"] = date.Unix() // Using Unix time as the UID

	// Convert the data to JSON format
	jsonStr, err := Utility.ToJson(data)
	if err != nil {
		return err
	}

	// Insert the message into the user's mail box in the database
	_, err = Store.InsertOne("local_resource", user+"_db", mailBox, jsonStr, "")
	if err != nil {
		return fmt.Errorf("error inserting message into database: %v", err)
	}

	return nil
}

// renameCollection renames a collection in the backend database.
func renameCollection(database string, name string, rename string) error {
	// Construct MongoDB script for renaming the collection
	script := `db=db.getSiblingDB('admin');db.adminCommand({renameCollection:'` + database + `.` + name + `', to:'` + database + `.` + rename + `'})`
	err := Store.RunAdminCmd("local_resource", "sa", Backend_password, script)
	return err
}

// startImap initializes and starts the IMAP server with optional TLS configuration.
func startImap(port int, keyFile string, certFile string) {
	// Create a new backend instance
	be := new(Backend_impl)

	// Create a new IMAP server
	s := imap_server.New(be)
	s.Addr = "0.0.0.0:" + Utility.ToString(port)

	go func() {
		// If certificate files are provided, enable TLS
		if len(certFile) > 0 {
			cer, err := tls.LoadX509KeyPair(certFile, keyFile)
			if err != nil {
				log.Printf("Error loading certificates: %v", err)
				return
			}

			s.TLSConfig = &tls.Config{Certificates: []tls.Certificate{cer}}
			if err := s.ListenAndServeTLS(); err != nil {
				log.Fatal("Error starting IMAP server with TLS: ", err)
			}
		} else {
			// For testing, allow plain text authentication over unencrypted connections
			s.AllowInsecureAuth = true
			if err := s.ListenAndServe(); err != nil {
				log.Fatal("Error starting IMAP server without TLS: ", err)
			}
		}
	}()
}

// StartImap initializes the IMAP server with multiple ports and TLS support.
func StartImap(store *persistence_client.Persistence_Client, backendAddress string, backendPort int, backendPassword string, keyFile string, certFile string, port int, tlsPort int, altPort int) {
	// Set global backend configuration
	Store = store
	Backend_address = backendAddress
	Backend_port = backendPort
	Backend_password = backendPassword

	// Start the IMAP servers with different port configurations
	startImap(port, "", "")               // Non-TLS server
	startImap(tlsPort, keyFile, certFile) // TLS-enabled server
	startImap(altPort, keyFile, certFile) // Alternate TLS-enabled server
}
