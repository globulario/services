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

	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/persistence/persistence_client"
	Utility "github.com/globulario/utility"
	"github.com/mhale/smtpd"
)

var (
	incoming        chan map[string]interface{}            // Incoming email messages channel
	outgoing        chan map[string]interface{}            // Outgoing email messages channel
	authenticate    chan map[string]interface{}            // Channel for user authentication
	validateRcpt    chan map[string]interface{}            // Channel for recipient validation
	Store           *persistence_client.Persistence_Client // Database connection for persistence
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

		// Lookup MX records for the domain
		fmt.Println("Looking up MX records for domain", domain)
		mxs, err := net.LookupMX(domain)
		if err != nil {
			fmt.Println("Failed to lookup MX records for domain", domain, ":", err)
			return err
		}

		// Default to MX server if no records found
		if len(mxs) == 0 {
			mxs = []*net.MX{{Host: domain}}
		}

		var success bool
		for _, mx := range mxs {
			// Try connecting on port 587 first (preferred port for SMTP with STARTTLS)
			for _, port := range []int{587, 465, 25} {
				fmt.Println("Trying to connect to", mx.Host, "on port", port)
				c, err := smtp.Dial(mx.Host + fmt.Sprintf(":%d", port))
				if err != nil {
					fmt.Println("Failed to connect to", mx.Host, "on port", port, ":", err)
					continue
				}

				// Attempt STARTTLS or SSL based on the port
				if port == 587 {
					// Port 587 uses STARTTLS
					tlsConfig := &tls.Config{
						ServerName:         mx.Host,
						InsecureSkipVerify: false, // Set to true for testing, but should be false in production
					}
					if err := c.StartTLS(tlsConfig); err != nil {
						fmt.Println("Failed to start TLS on", mx.Host, ":", err)
						c.Quit()
						continue
					}
				} else if port == 465 {
					// Port 465 uses SSL/TLS directly, no need for STARTTLS
					tlsConfig := &tls.Config{
						ServerName:         mx.Host,
						InsecureSkipVerify: false,
					}
					if err := c.StartTLS(tlsConfig); err != nil {
						fmt.Println("Failed to start SSL/TLS on", mx.Host, ":", err)
						c.Quit()
						continue
					}
				}

				defer c.Quit()

				// SMTP command sequence
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

				// Send email data
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

			if success {
				break
			}
		}

		if !success {
			return fmt.Errorf("Failed to send to any MX servers for domain %s", domain)
		}
	}

	return nil
}

// hasAccount checks if the user email has an associated account
func hasAccount(email string) bool {
	query := `{"email":"` + email + `"}`
	count, _ := Store.Count("local_resource", "local_resource", "Accounts", query, "")
	return count == 1
}

// rcptHandler handles recipient validation for incoming messages
func rcptHandler(remoteAddr net.Addr, from string, to string) bool {
	return hasAccount(to) || hasAccount(from)
}

// startSmtp initializes the SMTP server on a given domain and port, with optional TLS configuration
func startSmtp(domain string, port int, keyFile string, certFile string) {
	go func() {
		// Setup SMTP server
		srv := &smtpd.Server{
			Addr:    "0.0.0.0:" + Utility.ToString(port),
			Appname: "MyServerApp",
			AuthHandler: func(remoteAddr net.Addr, mechanism string, username []byte, password []byte, shared []byte) (bool, error) {
				answer_ := make(chan map[string]interface{})

				// Send the authentication request to the main thread
				authenticate <- map[string]interface{}{"user": string(username), "pwd": string(password), "answer": answer_}

				// Wait for answer
				answer := <-answer_
				if answer["err"] != nil {
					return false, answer["err"].(error)
				}

				return answer["valid"].(bool), nil
			},
			AuthMechs:    map[string]bool{},
			AuthRequired: false,
			Handler: func(remoteAddr net.Addr, from string, to []string, data []byte) error {
				// Handle incoming messages
				for _, recipient := range to {
					if hasAccount(recipient) {
						incoming <- map[string]interface{}{"msg": data, "from": from, "to": recipient}
					}

					if hasAccount(from) {
						outgoing <- map[string]interface{}{"msg": data, "from": from, "to": recipient}
					}
				}
				return nil
			},
			HandlerRcpt: rcptHandler,
			Hostname:    domain,
			LogRead:     func(remoteIP string, verb string, line string) {},
			LogWrite:    func(remoteIP string, verb string, line string) {},
			MaxSize:     0,
			Timeout:     0,
			TLSConfig:   nil,
			TLSListener: false,
			TLSRequired: false,
		}

		// TLS Setup if certs provided
		if len(certFile) > 0 && len(keyFile) > 0 {
			srv.TLSRequired = true
			cer, err := tls.LoadX509KeyPair(certFile, keyFile)
			if err != nil {
				log.Printf("Failed to load certificates: %v", err)
				return
			}
			srv.TLSConfig = &tls.Config{Certificates: []tls.Certificate{cer}}
		}

		// Start server
		if err := srv.ListenAndServe(); err != nil {
			log.Fatal("Server failed: ", err)
		}
	}()
}

// saveMessage stores an email message in the persistence backend
func saveMessage(email string, mailBox string, body []byte, flags []string) error {
	query := `{"email":"` + email + `"}`
	info, err := Store.FindOne("local_resource", "local_resource", "Accounts", query, "")
	if err != nil {
		return fmt.Errorf("failed to save message: %v", err)
	}

	data := map[string]interface{}{
		"Date":  time.Now(),
		"Flags": flags,
		"Size":  uint32(len(body)),
		"Body":  body,
		"Uid":   time.Now().Unix(),
	}

	_, err = Store.InsertOne("local_resource", info["name"].(string)+"_db", mailBox, data, "")
	return err
}

// StartSmtp initializes the persistence client and starts SMTP servers
func StartSmtp(store *persistence_client.Persistence_Client, backendAddress string, backendPort int, backendPassword string, domain string, keyFile string, certFile string, port int, tlsPort int, altPort int) {
	// Initialize channels
	incoming = make(chan map[string]interface{})
	outgoing = make(chan map[string]interface{})
	authenticate = make(chan map[string]interface{})

	// Use backend address as domain if not set
	if backendAddress == "0.0.0.0" || backendAddress == "localhost" {
		backendAddress, _ = config.GetDomain()
	}

	// Connect to persistence backend
	if err := Store.CreateConnection("local_resource", "local_resource", backendAddress, float64(backendPort), 1, "sa", backendPassword, 500, "", false); err != nil {
		log.Printf("Failed to connect to backend: %v", err)
		return
	}

	// Handle message processing
	go func() {
		for {
			select {
			case data := <-incoming:
				saveMessage(data["to"].(string), "INBOX", data["msg"].([]byte), []string{})

			case data := <-outgoing:
				sender := &Sender{Hostname: domain}
				if err := sender.Send(data["from"].(string), []string{data["to"].(string)}, bytes.NewReader(data["msg"].([]byte))); err != nil {
					log.Printf("Error sending email: %v", err)
				}
				saveMessage(data["from"].(string), "OUTBOX", data["msg"].([]byte), []string{})

			case data := <-authenticate:
				user, pwd, answer := data["user"].(string), data["pwd"].(string), data["answer"].(chan map[string]interface{})
				connectionID := user + "_db"

				// Authenticate user
				err := Store.CreateConnection(connectionID, connectionID, backendAddress, float64(backendPort), 1, user, pwd, 500, "", false)
				if err != nil {
					answer <- map[string]interface{}{"valid": false, "err": err}
				} else {
					answer <- map[string]interface{}{"valid": true, "err": nil}
				}
			}
		}
	}()

	// Start the SMTP servers with specified ports
	startSmtp(domain, port, "", "")
	startSmtp(domain, tlsPort, keyFile, certFile)
	startSmtp(domain, altPort, keyFile, certFile)
}
