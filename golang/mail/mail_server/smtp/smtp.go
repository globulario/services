package smtp

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"io"
	"log"
	"net"
	"net/smtp"
	"strings"
	"time"

	// I will use persistence store as backend...
	"github.com/davecourtois/Utility"
	"github.com/globulario/services/golang/config"
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

// Sender represents an email sender
type Sender struct {
	Hostname string
}

// splitAddress splits the email address into the local part and the domain part
func splitAddress(address string) (localPart, domain string, err error) {
	at := strings.LastIndex(address, "@")
	if at < 0 {
		return "", "", fmt.Errorf("Invalid email address: %s", address)
	}
	return address[:at], address[at+1:], nil
}

// Send sends an email using the specified sender, from address, to addresses, and message reader
func (s *Sender) Send(from string, to []string, r io.Reader) error {
	fmt.Println(from, "trying to send message to", to)

	for _, addr := range to {
		_, domain, err := splitAddress(addr)
		if err != nil {
			return err
		}

		fmt.Println("Looking up MX records for domain", domain)
		mxs, err := net.LookupMX(domain)
		if err != nil {
			fmt.Println("Failed to lookup MX records for domain", domain, ":", err)
			return err
		}

		if len(mxs) == 0 {
			mxs = []*net.MX{{Host: domain}}
		}

		var success bool
		for _, mx := range mxs {
			fmt.Println("Trying to connect to", mx.Host)

			// First try with port 587 and STARTTLS
			c, err := smtp.Dial(mx.Host + ":587")
			if err != nil {
				fmt.Println("Failed to connect to", mx.Host, "on port 587:", err)
				continue
			}

			// Start TLS
			tlsConfig := &tls.Config{
				ServerName:         mx.Host,
				InsecureSkipVerify: false,
			}

			if err := c.StartTLS(tlsConfig); err != nil {
				fmt.Println("Failed to start TLS on", mx.Host, ":", err)
				c.Quit()
				continue
			}

			defer c.Quit()

			if err := c.Hello(s.Hostname); err != nil {
				fmt.Println("Hello failed:", err)
				return err
			}

			if err := c.Mail(from); err != nil {
				fmt.Println("Mail command failed:", err)
				return err
			}

			if err := c.Rcpt(addr); err != nil {
				fmt.Println("Rcpt command failed:", err)
				return err
			}

			wc, err := c.Data()
			if err != nil {
				fmt.Println("Data command failed:", err)
				return err
			}

			if _, err := io.Copy(wc, r); err != nil {
				fmt.Println("Copy to data writer failed:", err)
				return err
			}

			if err := wc.Close(); err != nil {
				fmt.Println("Close data writer failed:", err)
				return err
			}

			fmt.Println("Message sent to", addr)
			success = true
			break
		}

		if !success {
			return fmt.Errorf("Failed to send to any MX servers for domain %s", domain)
		}
	}

	return nil
}

func hasAccount(email string) bool {
	fmt.Println("------------> test if account exist 149", email)
	query := `{"email":"` + email + `"}`
	count, _ := Store.Count("local_resource", "local_resource", "Accounts", query, "")

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

		fmt.Println("----------> start smtp server at port ", port)
		srv := &smtpd.Server{
			Addr:    "0.0.0.0:" + Utility.ToString(port),
			Appname: "MyServerApp",
			AuthHandler: func(remoteAddr net.Addr, mechanism string, username []byte, password []byte, shared []byte) (bool, error) {
				answer_ := make(chan map[string]interface{})

				fmt.Println("------------> Try to authenticate 181", string(username), string(password))
				// send the authentication request to the main thread.
				authenticate <- map[string]interface{}{"user": string(username), "pwd": string(password), "answer": answer_}

				// wait for answer...
				answer := <-answer_
				if answer["err"] != nil {
					return false, answer["err"].(error)
				}

				fmt.Println("------------> 188", answer["valid"].(bool))
				return answer["valid"].(bool), nil
			},
			AuthMechs:    map[string]bool{},
			AuthRequired: false,
			Handler: func(remoteAddr net.Addr, from string, to []string, data []byte) error {
				fmt.Println("------------> 189", from, to)
				// push message in to incomming...
				for i := 0; i < len(to); i++ {

					if hasAccount(to[i]) {
						fmt.Println("------------> 190", to[i])
						incomming <- map[string]interface{}{"msg": data, "from": from, "to": to[i]}
					}

					if hasAccount(from) {
						fmt.Println("------------> 195", to[i])
						outgoing <- map[string]interface{}{"msg": data, "from": from, "to": to[i]}
					}

				}
				return nil
			},
			HandlerRcpt: rcptHandler,
			Hostname:    domain,
			LogRead: func(remoteIP string, verb string, line string) {

			},
			LogWrite: func(remoteIP string, verb string, line string) {

			},
			MaxSize:     0,
			Timeout:     0,
			TLSConfig:   nil,
			TLSListener: false,
			TLSRequired: false,
		}

		if len(certFile) > 0 && len(keyFile) > 0 {
			srv.TLSRequired = true
			cer, err := tls.LoadX509KeyPair(certFile, keyFile)
			if err != nil {
				fmt.Println("----------------> 220", err)
				return
			}
			srv.TLSConfig = &tls.Config{Certificates: []tls.Certificate{cer}}
		}

		fmt.Println("----------> smtp server lisen at port ", port)
		err := srv.ListenAndServe()
		if err != nil {
			log.Fatal(err)
		}

	}()
}

func saveMessage(email string, mailBox string, body []byte, flags []string) error {

	fmt.Println("-----------------> try to save message from ", email, mailBox)

	query := `{"email":"` + email + `"}`
	info, err := Store.FindOne("local_resource", "local_resource", "Accounts", query, "")
	if err != nil {
		fmt.Println("fail to save message with error: ", err)
		return err
	}

	data := make(map[string]interface{})
	now := time.Now()
	data["Date"] = now
	data["Flags"] = flags
	data["Size"] = uint32(len(body))
	data["Body"] = body
	data["Uid"] = now.Unix() // I will use the unix time as Uid

	// TODO Insert large one...

	// Now I will insert the message into the inbox of the user.
	_, err = Store.InsertOne("local_resource", info["name"].(string)+"_db", mailBox, data, "")
	if err != nil {
		fmt.Println(err)
	}

	return err
}

func StartSmtp(store *persistence_client.Persistence_Client, backend_address string, backend_port int, backend_password string, domain string, keyFile string, certFile string, port int, tls_port int, alt_port int) {

	// create channel's
	incomming = make(chan map[string]interface{})
	outgoing = make(chan map[string]interface{})

	// authenticate to send (or optinaly receive) user email
	authenticate = make(chan map[string]interface{})

	// Validate that the email is manage by the srv.
	validateRcpt = make(chan map[string]interface{})

	// I will use the backend address as the domain because the user is authenticated.
	if backend_address == "0.0.0.0" || backend_address == "localhost" {
		backend_address, _ = config.GetDomain()
	}

	err := Store.CreateConnection("local_resource", "local_resource", backend_address, float64(backend_port), 1, "sa", backend_password, 500, "", false)
	if err != nil {
		log.Println("fail to create connection to the backend with error: ", err)
		return
	}

	go func() {
		for {
			select {
			case data := <-incomming:
				saveMessage(data["to"].(string), "INBOX", data["msg"].([]byte), []string{})

			case data := <-outgoing:
				sender := new(Sender)
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
				err := Store.CreateConnection(connection_id, connection_id, backend_address, float64(backend_port), 1, user, pwd, 500, "", false)

				if err != nil {
					answer_ <- map[string]interface{}{"valid": false, "err": err}
				} else {
					answer_ <- map[string]interface{}{"valid": true, "err": nil}
				}

			}
		}
	}()

	// non tls at port 25
	//startSmtp(domain, port, "", "")

	// tls at port 465
	startSmtp(domain, tls_port, keyFile, certFile)

	// Alt at port 587
	startSmtp(domain, alt_port, keyFile, certFile)
}
