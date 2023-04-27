package mail_client

import (
	"fmt"
	"log"
	"testing"

	"github.com/globulario/services/golang/mail/mailpb"
)

var (
	client *Mail_Client
)

// smtpServer data to smtp server
type smtpServer struct {
	host string
	port string
}

// Address URI to smtp server
func (s *smtpServer) Address() string {
	return s.host + ":" + s.port
}


// First test create a fresh new connection...
func TestCreateConnection(t *testing.T) {

	var err error
	client, err = NewMailService_Client("globule-aws.globular.io:443", "mail.MailService")
	if err != nil {
		log.Panicln(err)
	}

	fmt.Println("Connection creation test.")
	err = client.CreateConnection("test_smtp", "dave", "1234", 587, "globule-aws.globular.io")

	if err != nil {
		log.Panicln(err)
	}

	fmt.Println("connection was createad!")
}

/**
 * Test send email whitout attachements.
 */
func TestSendEmail(t *testing.T) {

	from := "dave@globular.io"
	to := []string{"dave@globular.io"}
	cc := []*mailpb.CarbonCopy{&mailpb.CarbonCopy{Name: "Dave Courtois", Address: "dave@globular.io"}}
	subject := "Smtp Test"
	body := `<meta http-equiv="Content-Type" content="text/html; charset=utf-8"><div dir="ltr">Message test.</div>`
	bodyType := int32(mailpb.BodyType_HTML)

	err := client.SendEmail("test_smtp", from, to, cc, subject, body, bodyType)

	if err != nil {
		log.Panicln(err)
	}
}

/**
 * Test send email with attachements.
 */
/*
func TestSendEmailWithAttachements(t *testing.T) {

	from := "dave.courtois@safrangroup.com"
	to := []string{"dave.courtois@safrangroup.com"}
	cc := []*mailpb.CarbonCopy{&mailpb.CarbonCopy{Name: "Dave Courtois", Address: "dave.courtois60@gmail.com"}}
	subject := "Smtp Test"
	body := `<meta http-equiv="Content-Type" content="text/html; charset=utf-8"><div dir="ltr">Message test.</div>`
	bodyType := int32(mailpb.BodyType_HTML)
	attachments := []string{"attachements/Daft Punk - Get Lucky (Official Audio) ft. Pharrell Williams, Nile Rodgers.mp3", "attachements/NGEN3549.JPG", "attachements/NGEN3550.JPG"}

	err := client.SendEmailWithAttachements("test_smtp", from, to, cc, subject, body, bodyType, attachments)

	if err != nil {
		log.Panicln(err)
	}

}*/
