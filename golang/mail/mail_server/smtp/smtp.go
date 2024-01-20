package smtp

import (
	"bytes"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"strings"
	"time"

	"github.com/emersion/go-smtp"
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

type Sender struct {
	Hostname string
}

func (s *Sender) Send(from string, to []string, r io.Reader) error {
	// TODO: buffer r if sending to multiple recipients
	// TODO: group recipients with same domain
	fmt.Println(from, "try to send message to ", to)
	for _, addr := range to {
		_, domain, err := splitAddress(addr)
		if err != nil {
			return err
		}

		fmt.Println("lookup for address ", domain)
		mxs, err := net.LookupMX(domain)
		if err != nil {
			return err
		}

		for _, mx := range mxs {
			fmt.Println("-------------> 61 ", mx.Host)
		}

		if len(mxs) == 0 {
			mxs = []*net.MX{{Host: domain}}
		}

		for _, mx := range mxs {
			c, err := smtp.Dial(mx.Host + ":25")
			if err != nil {
				if err != nil {
					fmt.Println("66 ----------> ", err)
				}
				return err
			}

			if err := c.Hello(s.Hostname); err != nil {
				if err != nil {
					fmt.Println("73 ----------> ", err)
				}
				return err
			}

			if ok, _ := c.Extension("STARTTLS"); ok {
				tlsConfig := &tls.Config{ServerName: mx.Host}
				if err := c.StartTLS(tlsConfig); err != nil {
					if err != nil {
						fmt.Println("82 ----------> ", err)
					}
					return err
				}
			}

			if err := c.Mail(from, &smtp.MailOptions{}); err != nil {
				if err != nil {
					fmt.Println("90 ----------> ", err)
				}
				return err
			}
			if err := c.Rcpt(addr); err != nil {
				if err != nil {
					fmt.Println("96 ----------> ", err)
				}
				return err
			}

			wc, err := c.Data()
			if err != nil {
				if err != nil {
					fmt.Println("104 ----------> ", err)
				}
				return err
			}
			if _, err := io.Copy(wc, r); err != nil {
				if err != nil {
					fmt.Println("110 ----------> ", err)
				}
				return err
			}
			if err := wc.Close(); err != nil {
				if err != nil {
					fmt.Println("116 ----------> ", err)
				}
				return err
			}

			if err := c.Quit(); err != nil {
				if err != nil {
					fmt.Println("124 ----------> ", err)
				}
				return err
			}
		}
	}

	return nil
}

func splitAddress(addr string) (local, domain string, err error) {
	parts := strings.SplitN(addr, "@", 2)
	if len(parts) != 2 {
		return "", "", errors.New("mta: invalid mail address")
	}
	return parts[0], parts[1], nil
}

func hasAccount(email string) bool {
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

		srv := &smtpd.Server{
			Addr:    "0.0.0.0:" + Utility.ToString(port),
			Appname: "MyServerApp",
			AuthHandler: func(remoteAddr net.Addr, mechanism string, username []byte, password []byte, shared []byte) (bool, error) {
				fmt.Println("--------> authenticate user ", string(username), string(password))

				answer_ := make(chan map[string]interface{})
				authenticate <- map[string]interface{}{"user": string(username), "pwd": string(password), "answer": answer_}

				// wait for answer...
				answer := <-answer_
				if answer["err"] != nil {
					return false, answer["err"].(error)
				}
				return answer["valid"].(bool), nil
			},
			AuthMechs:    map[string]bool{},
			AuthRequired: false,
			Handler: func(remoteAddr net.Addr, from string, to []string, data []byte) error {

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
				return
			}
			srv.TLSConfig = &tls.Config{Certificates: []tls.Certificate{cer}}
		}

		srv.ListenAndServe()

	}()
}

func saveMessage(email string, mailBox string, body []byte, flags []string) error {

	fmt.Println("try to save message from ", email, mailBox)

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

func StartSmtp(store *persistence_client.Persistence_Client, backend_address string, backend_port int, domain string, keyFile string, certFile string, port int, tls_port int, alt_port int) {

	// create channel's
	incomming = make(chan map[string]interface{})
	outgoing = make(chan map[string]interface{})

	// authenticate to send (or optinaly receive) user email
	authenticate = make(chan map[string]interface{})

	// Validate that the email is manage by the srv.
	validateRcpt = make(chan map[string]interface{})

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
				err := Store.CreateConnection(connection_id, connection_id, backend_address, float64(backend_port), 0, user, pwd, 500, "", false)

				if err != nil {
					answer_ <- map[string]interface{}{"valid": false, "err": err}
				} else {
					answer_ <- map[string]interface{}{"valid": true, "err": nil}
				}
			}
		}
	}()

	// non tls at port 25
	startSmtp(domain, port, "", "")

	// tls at port 465
	startSmtp(domain, tls_port, keyFile, certFile)

	// Alt at port 587
	startSmtp(domain, alt_port, keyFile, certFile)
}
