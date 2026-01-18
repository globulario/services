package mail_client

import (
	"log"
	"testing"

	"github.com/globulario/services/golang/mail/mailpb"
	"github.com/globulario/services/golang/testutil"
)

// newTestClient creates a client for testing, skipping if external services are not available.
func newTestClient(t *testing.T) *Mail_Client {
	t.Helper()
	testutil.SkipIfNoExternalServices(t)

	addr := testutil.GetAddress()
	client, err := NewMailService_Client(addr, "mail.MailService")
	if err != nil {
		t.Fatalf("NewMailService_Client: %v", err)
	}
	return client
}

// First test create a fresh new connection...
func TestCreateConnection(t *testing.T) {
	client := newTestClient(t)
	saUser, saPwd := testutil.GetSACredentials()
	addr := testutil.GetAddress()

	err := client.CreateConnection("test_smtp", saUser, saPwd, 587, addr)
	if err != nil {
		log.Panicln(err)
	}
}

/**
 * Test send email whitout attachements.
 */
func TestSendEmail(t *testing.T) {
	client := newTestClient(t)

	from := "sa@globular.io"
	to := []string{"test-1norsw1sk@srv1.mail-tester.com"}
	cc := []*mailpb.CarbonCopy{/*&mailpb.CarbonCopy{Name: "Dave Courtois", Address: "sa@globular.io"}*/}
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
