package smtp

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"time"

	"github.com/emersion/go-smtp-mta"

	// I will use persistence store as backend...
	"github.com/davecourtois/Utility"
	"github.com/globulario/services/golang/persistence/persistence_client"
	"github.com/mhale/smtpd"
)

var (
	// The incomming message.
	incomming chan map[string]interface{}

	// The outgoing channel.
	outgoing chan map[string]interface{}

	// This is the authenticated user.
	authenticate chan map[string]interface{}

	// Validate recipient.
	validateRcpt chan map[string]interface{}

	// The backend connection
	Store           *persistence_client.Persistence_Client
	Backend_address string
	Backend_port    int
)

/**
 * Handle incomming message.
 */
func mailHandler(origin net.Addr, from string, to []string, data []byte) {

	// push message in to incomming...
	for i := 0; i < len(to); i++ {
		if hasAccount(to[i]) {
			incomming <- map[string]interface{}{"msg": data, "from": from, "to": to[i]}
		} else if hasAccount(from) {
			outgoing <- map[string]interface{}{"msg": data, "from": from, "to": to[i]}
		}
	}

}

/**
 * Authentication handler.
 */
func authHandler(remoteAddr net.Addr, mechanism string, username []byte, password []byte, shared []byte) (bool, error) {
	log.Println("authication try ", mechanism, string(username), " addresse ", remoteAddr.String())
	answer_ := make(chan map[string]interface{})
	authenticate <- map[string]interface{}{"user": string(username), "pwd": string(password), "answer": answer_}
	// wait for answer...
	answer := <-answer_
	if answer["err"] != nil {
		return false, answer["err"].(error)
	}
	return answer["valid"].(bool), nil
}

func hasAccount(email string) bool {
	query := `{"email":"` + email + `"}`
	count, _ := Store.Count("local_ressource", "local_ressource", "Accounts", query, "")

	if count == 1 {
		return true
	}

	return false
}

/**
 * Recipient validation handler.
 */
func rcptHandler(remoteAddr net.Addr, from string, to string) bool {
	if hasAccount(to) || hasAccount(from) {
		return true
	}

	return false
}

func startSmtp(domain string, port int, keyFile string, certFile string) {
	go func() {
		srv := &smtpd.Server{
			Addr:        "0.0.0.0:" + Utility.ToString(port),
			Handler:     mailHandler,
			HandlerRcpt: rcptHandler,
			AuthHandler: authHandler,
			Appname:     "MyServerApp",
			Hostname:    domain,
		}

		if len(certFile) > 0 {
			srv.TLSRequired = true
			cer, err := tls.LoadX509KeyPair(certFile, keyFile)
			if err != nil {
				log.Println(err)
				return
			}
			srv.TLSConfig = &tls.Config{Certificates: []tls.Certificate{cer}}
		}
		log.Println("start smtp server at address ", srv.Addr)
		srv.ListenAndServe()

	}()
}

func saveMessage(email string, mailBox string, body []byte, flags []string) error {

	query := `{"email":"` + email + `"}`
	jsonStr, err := Store.FindOne("local_ressource", "local_ressource", "Accounts", query, "")
	if err != nil {
		return err
	}

	info := make(map[string]interface{})
	err = json.Unmarshal([]byte(jsonStr), &info)
	if err != nil {
		return err
	}

	data := make(map[string]interface{})
	now := time.Now()
	data["Date"] = now
	data["Flags"] = flags
	data["Size"] = uint32(len(body))
	data["Body"] = body
	data["Uid"] = now.Unix() // I will use the unix time as Uid

	jsonStr, err = Utility.ToJson(data)
	if err != nil {
		return err
	}

	// TODO Insert large one...

	// Now I will insert the message into the inbox of the user.
	_, err = Store.InsertOne("local_ressource", info["name"].(string)+"_db", mailBox, jsonStr, "")
	if err != nil {
		fmt.Println(err)
	}

	return err
}

func StartSmtp(store *persistence_client.Persistence_Client, backend_address string, backend_port int, domain string, keyFile string, certFile string, port int, tls_port int, alt_port int) {
	// keep backend info
	store = store
	backend_address = backend_address
	backend_port = backend_port

	// create channel's
	incomming = make(chan map[string]interface{})
	outgoing = make(chan map[string]interface{})

	// authenticate to send (or optinaly receive) user email
	authenticate = make(chan map[string]interface{})

	// Validate that the email is manage by the server.
	validateRcpt = make(chan map[string]interface{})

	go func() {
		for {
			select {
			case data := <-incomming:
				log.Println("Receive email from ", data["from"], " to ", data["to"])
				saveMessage(data["to"].(string), "INBOX", data["msg"].([]byte), []string{})

			case data := <-outgoing:
				log.Println("Send email to ", data["to"], " by ", data["from"])
				sender := new(mta.Sender)
				sender.Hostname = domain
				err := sender.Send(data["from"].(string), []string{data["to"].(string)}, bytes.NewReader(data["msg"].([]byte)))
				if err != nil {
					log.Println("warning/error when sending email: ", err)
				}
				saveMessage(data["from"].(string), "OUTBOX", data["msg"].([]byte), []string{})

			case data := <-authenticate:

				// Here I will try to connect the user on it db.
				user := data["user"].(string)
				pwd := data["pwd"].(string)
				answer_ := data["answer"].(chan map[string]interface{})
				connection_id := user + "_db"

				// I will use the datastore to authenticate the user.
				err := Store.CreateConnection(connection_id, connection_id, backend_address, float64(backend_port), 0, user, pwd, 5000, "", false)

				if err != nil {
					answer_ <- map[string]interface{}{"valid": false, "err": err}
				} else {
					answer_ <- map[string]interface{}{"valid": true, "err": nil}
				}

			case rcpt := <-validateRcpt:
				log.Println(rcpt)
			}
		}
	}()

	// non tls at port 25
	startSmtp(domain, port, "", "")
	// tls at port 465
	startSmtp(domain, tls_port, keyFile, certFile)
	// Alt at port 587
	startSmtp(domain, alt_port, "", "")
}
