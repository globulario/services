package smtp

import (
	"bytes"
	"crypto/tls"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/smtp"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/persistence/persistence_client"
	Utility "github.com/globulario/utility"
	"github.com/mhale/smtpd"
)

// -----------------------------------------------------------------------------
// Logger
// -----------------------------------------------------------------------------

var logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
	Level: slog.LevelInfo,
}))

// -----------------------------------------------------------------------------
// Globals
// -----------------------------------------------------------------------------

var (
	incoming        chan map[string]interface{}            // Incoming email messages
	outgoing        chan map[string]interface{}            // Outgoing email messages
	authenticate    chan map[string]interface{}            // User authentication requests
	validateRcpt    chan map[string]interface{}            // Recipient validation (kept for compatibility)
	Store           *persistence_client.Persistence_Client // Persistence client
	Backend_address string
	Backend_port    int
)

// Sender represents an email sender (outbound SMTP).
type Sender struct {
	Hostname string
}

// splitAddress splits an email address into local part and domain.
func splitAddress(address string) (localPart, domain string, err error) {
	at := strings.LastIndex(address, "@")
	if at < 0 {
		err = errors.New("invalid email address: " + address)
		return "", "", err
	}
	return address[:at], address[at+1:], nil
}

// Send relays a message to each recipient's MX host using SMTP.
// It prefers port 587 (STARTTLS), then 465 (implicit TLS attempted via STARTTLS),
// then 25 (plain). Public prototype preserved.
func (s *Sender) Send(from string, to []string, r io.Reader) error {
	for _, addr := range to {
		_, domain, err := splitAddress(addr)
		if err != nil {
			logger.Warn("smtp send: invalid recipient", "addr", addr, "err", err)
			return err
		}

		// Lookup MX records for the recipient domain.
		mxs, err := net.LookupMX(domain)
		if err != nil {
			logger.Error("smtp send: MX lookup failed", "domain", domain, "err", err)
			return err
		}
		if len(mxs) == 0 {
			mxs = []*net.MX{{Host: domain}}
		}

		var success bool
		for _, mx := range mxs {
			host := strings.TrimSuffix(mx.Host, ".")
			for _, port := range []int{587, 465, 25} {
				addrPort := host + ":" + strconv.Itoa(port)

				c, err := smtp.Dial(addrPort)
				if err != nil {
					logger.Warn("smtp send: dial failed", "mx", host, "port", port, "err", err)
					continue
				}

				// Ensure we close this client.
				func() {
					defer func() {
						_ = c.Quit()
					}()

					// STARTTLS policy for 587/465 (best effort; 465 typically needs tls.Dial, but we keep behavior).
					if port == 587 || port == 465 {
						tlsConfig := &tls.Config{
							ServerName:         host,
							InsecureSkipVerify: false,
						}
						if err := c.StartTLS(tlsConfig); err != nil {
							logger.Warn("smtp send: STARTTLS failed", "mx", host, "port", port, "err", err)
							return
						}
					}

					if err := c.Hello(s.Hostname); err != nil {
						logger.Warn("smtp send: HELO/EHLO failed", "mx", host, "port", port, "err", err)
						return
					}
					if err := c.Mail(from); err != nil {
						logger.Warn("smtp send: MAIL FROM failed", "from", from, "mx", host, "port", port, "err", err)
						return
					}
					if err := c.Rcpt(addr); err != nil {
						logger.Warn("smtp send: RCPT TO failed", "to", addr, "mx", host, "port", port, "err", err)
						return
					}
					wc, err := c.Data()
					if err != nil {
						logger.Warn("smtp send: DATA failed", "mx", host, "port", port, "err", err)
						return
					}
					if _, err := io.Copy(wc, r); err != nil {
						_ = wc.Close()
						logger.Warn("smtp send: write body failed", "mx", host, "port", port, "err", err)
						return
					}
					if err := wc.Close(); err != nil {
						logger.Warn("smtp send: close data failed", "mx", host, "port", port, "err", err)
						return
					}

					success = true
				}()

				if success {
					break
				}
			}
			if success {
				break
			}
		}

		if !success {
			logger.Error("smtp send: all MX attempts failed", "domain", domain)
			return errors.New("failed to send to any MX servers for domain " + domain)
		}
	}
	return nil
}

// hasAccount checks if a user email has an associated account.
func hasAccount(email string) bool {
	query := `{"email":"` + email + `"}`
	count, err := Store.Count("local_resource", "local_resource", "Accounts", query, "")
	if err != nil {
		logger.Warn("hasAccount: count failed", "email", email, "err", err)
		return false
	}
	return count == 1
}

// rcptHandler validates recipients for incoming messages.
func rcptHandler(remoteAddr net.Addr, from string, to string) bool {
	return hasAccount(to) || hasAccount(from)
}

// startSmtp launches an SMTP server on the given port with optional TLS.
// Unexported; use StartSmtp to configure and run all instances.
func startSmtp(domain string, port int, keyFile string, certFile string) {
	go func() {
		srv := &smtpd.Server{
			Addr:    "0.0.0.0:" + Utility.ToString(port),
			Appname: "MailServer",
			AuthHandler: func(remoteAddr net.Addr, mechanism string, username []byte, password []byte, shared []byte) (bool, error) {
				answerCh := make(chan map[string]interface{})
				authenticate <- map[string]interface{}{
					"user":   string(username),
					"pwd":    string(password),
					"answer": answerCh,
				}
				answer := <-answerCh
				if answer["err"] != nil {
					err := answer["err"].(error)
					logger.Warn("smtp auth: failed", "user", string(username), "remote", remoteAddr.String(), "err", err)
					return false, err
				}
				return answer["valid"].(bool), nil
			},
			AuthMechs:    map[string]bool{},
			AuthRequired: false,
			Handler: func(remoteAddr net.Addr, from string, to []string, data []byte) error {
				for _, rcpt := range to {
					if hasAccount(rcpt) {
						incoming <- map[string]interface{}{"msg": data, "from": from, "to": rcpt}
					}
					if hasAccount(from) {
						outgoing <- map[string]interface{}{"msg": data, "from": from, "to": rcpt}
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

		// TLS (if certs provided)
		if certFile != "" && keyFile != "" {
			cer, err := tls.LoadX509KeyPair(certFile, keyFile)
			if err != nil {
				logger.Error("smtp start: load certs failed", "port", port, "cert", certFile, "key", keyFile, "err", err)
				return
			}
			srv.TLSRequired = true
			srv.TLSConfig = &tls.Config{Certificates: []tls.Certificate{cer}}
		}

		logger.Info("smtp start", "domain", domain, "port", port, "tls", srv.TLSRequired)
		if err := srv.ListenAndServe(); err != nil {
			logger.Error("smtp server exited", "port", port, "err", err)
		}
	}()
}

// saveMessage stores an email message for a user/mailbox in the persistence backend.
func saveMessage(email string, mailBox string, body []byte, flags []string) error {
	query := `{"email":"` + email + `"}`
	info, err := Store.FindOne("local_resource", "local_resource", "Accounts", query, "")
	if err != nil {
		logger.Error("saveMessage: account lookup failed", "email", email, "err", err)
		return errors.New("failed to save message: account lookup error")
	}

	data := map[string]interface{}{
		"Date":  time.Now(),
		"Flags": flags,
		"Size":  uint32(len(body)),
		"Body":  body,
		"Uid":   time.Now().Unix(),
	}

	_, err = Store.InsertOne("local_resource", info["name"].(string)+"_db", mailBox, data, "")
	if err != nil {
		logger.Error("saveMessage: insert failed", "email", email, "mailbox", mailBox, "err", err)
		return err
	}
	return nil
}

// StartSmtp initializes the persistence client and starts SMTP servers on the
// provided ports (plain and TLS variants). Public prototype preserved.
func StartSmtp(
	store *persistence_client.Persistence_Client,
	backendAddress string,
	backendPort int,
	backendPassword string,
	domain string,
	keyFile string,
	certFile string,
	port int,
	tlsPort int,
	altPort int,
) {
	// Initialize channels (singletons for this package)
	incoming = make(chan map[string]interface{})
	outgoing = make(chan map[string]interface{})
	authenticate = make(chan map[string]interface{})
	// keep validateRcpt for compatibility even if not used explicitly
	validateRcpt = make(chan map[string]interface{})

	// Use configured domain if backend address is local
	addr := backendAddress
	if addr == "0.0.0.0" || addr == "localhost" {
		if d, err := config.GetDomain(); err == nil && d != "" {
			addr = d
		}
	}

	// Persist client is expected to be passed in as 'store'
	Store = store
	Backend_address = addr
	Backend_port = backendPort

	// Ensure connection to persistence backend
	if err := Store.CreateConnection("local_resource", "local_resource", addr, float64(backendPort), 1, "sa", backendPassword, 500, "", false); err != nil {
		logger.Error("smtp start: persistence connect failed", "address", addr, "port", backendPort, "err", err)
		return
	}

	// Message processing goroutine
	go func() {
		for {
			select {
			case data := <-incoming:
				if err := saveMessage(data["to"].(string), "INBOX", data["msg"].([]byte), []string{}); err != nil {
					logger.Warn("incoming save failed", "to", data["to"], "err", err)
				}

			case data := <-outgoing:
				sender := &Sender{Hostname: domain}
				if err := sender.Send(
					data["from"].(string),
					[]string{data["to"].(string)},
					bytes.NewReader(data["msg"].([]byte)),
				); err != nil {
					logger.Warn("outgoing send failed", "from", data["from"], "to", data["to"], "err", err)
				}
				if err := saveMessage(data["from"].(string), "OUTBOX", data["msg"].([]byte), []string{}); err != nil {
					logger.Warn("outgoing save failed", "from", data["from"], "err", err)
				}

			case data := <-authenticate:
				user := data["user"].(string)
				pwd := data["pwd"].(string)
				answer := data["answer"].(chan map[string]interface{})
				connectionID := user + "_db"

				if err := Store.CreateConnection(connectionID, connectionID, addr, float64(backendPort), 1, user, pwd, 500, "", false); err != nil {
					logger.Warn("smtp auth: invalid credentials", "user", user, "err", err)
					answer <- map[string]interface{}{"valid": false, "err": err}
				} else {
					answer <- map[string]interface{}{"valid": true, "err": nil}
				}
			}
		}
	}()

	// Start SMTP servers on requested ports
	if port > 0 {
		startSmtp(domain, port, "", "")
	}
	if tlsPort > 0 {
		startSmtp(domain, tlsPort, keyFile, certFile)
	}
	if altPort > 0 {
		startSmtp(domain, altPort, keyFile, certFile)
	}
}
