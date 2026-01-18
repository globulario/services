# Mail Service

<p align="center">
  <img src="../logo.png" alt="Globular Logo" width="100"/>
</p>

The Mail Service provides email delivery capabilities via SMTP integration.

## Overview

This service enables Globular applications to send emails including plain text, HTML, and messages with file attachments.

## Features

- **SMTP Integration** - Connect to any SMTP server
- **HTML Emails** - Rich formatted content
- **Attachments** - File attachments via streaming
- **CC/BCC Support** - Multiple recipient types
- **Connection Pooling** - Efficient mail server connections

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                        Mail Service                              │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  ┌────────────────────────────────────────────────────────────┐ │
│  │                 Connection Manager                         │ │
│  │                                                            │ │
│  │  SMTP Server ◄──► Connection Pool ◄──► Auth               │ │
│  │                                                            │ │
│  └────────────────────────────────────────────────────────────┘ │
│                                                                  │
│  ┌────────────────────────────────────────────────────────────┐ │
│  │                   Message Builder                          │ │
│  │                                                            │ │
│  │  To/CC/BCC │ Subject │ Body (Text/HTML) │ Attachments     │ │
│  │                                                            │ │
│  └────────────────────────────────────────────────────────────┘ │
│                                                                  │
│  ┌────────────────────────────────────────────────────────────┐ │
│  │                   Delivery Engine                          │ │
│  │                                                            │ │
│  │  Queue ──▶ Send ──▶ Retry (on failure) ──▶ Log            │ │
│  │                                                            │ │
│  └────────────────────────────────────────────────────────────┘ │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

## API Reference

### Connection Management

| Method | Description | Parameters |
|--------|-------------|------------|
| `CreateConnection` | Configure SMTP server | `id`, `host`, `port`, `user`, `password`, `tls` |
| `DeleteConnection` | Remove connection | `id` |

### Email Operations

| Method | Description | Parameters |
|--------|-------------|------------|
| `SendEmail` | Send email (text/HTML) | `connectionId`, `to[]`, `cc[]`, `subject`, `body`, `isHtml` |
| `SendEmailWithAttachments` | Send with files (streaming) | `connectionId`, `email`, `attachments[]` |

## Usage Examples

### Go Client

```go
import (
    mail "github.com/globulario/services/golang/mail/mail_client"
)

client, _ := mail.NewMailService_Client("localhost:10118", "mail.MailService")
defer client.Close()

// Create SMTP connection
err := client.CreateConnection(
    "gmail",                    // connection ID
    "smtp.gmail.com",           // host
    587,                        // port
    "sender@gmail.com",         // user
    "app-password",             // password
    true,                       // use TLS
)

// Send simple email
err = client.SendEmail(
    "gmail",
    []string{"recipient@example.com"},
    []string{"cc@example.com"},
    "Hello from Globular",
    "This is the email body.",
    false, // plain text
)

// Send HTML email
htmlBody := `
<html>
<body>
  <h1>Welcome!</h1>
  <p>Thank you for signing up.</p>
  <a href="https://example.com/verify">Verify Email</a>
</body>
</html>
`
err = client.SendEmail(
    "gmail",
    []string{"user@example.com"},
    nil,
    "Welcome to Our Service",
    htmlBody,
    true, // HTML
)

// Send with attachments
attachments := []*mailpb.Attachment{
    {
        Filename: "report.pdf",
        Data:     pdfData,
        MimeType: "application/pdf",
    },
    {
        Filename: "image.png",
        Data:     imageData,
        MimeType: "image/png",
    },
}
err = client.SendEmailWithAttachments(
    "gmail",
    &mailpb.Email{
        To:      []string{"user@example.com"},
        Subject: "Monthly Report",
        Body:    "Please find attached the monthly report.",
        IsHtml:  false,
    },
    attachments,
)
```

## Configuration

```json
{
  "port": 10118,
  "connections": [
    {
      "id": "default",
      "host": "smtp.example.com",
      "port": 587,
      "user": "noreply@example.com",
      "useTLS": true
    }
  ],
  "defaultFrom": "noreply@example.com",
  "maxRetries": 3,
  "retryDelay": "5s"
}
```

## Common SMTP Configurations

| Provider | Host | Port | TLS |
|----------|------|------|-----|
| Gmail | smtp.gmail.com | 587 | Yes |
| Outlook | smtp.office365.com | 587 | Yes |
| SendGrid | smtp.sendgrid.net | 587 | Yes |
| AWS SES | email-smtp.region.amazonaws.com | 587 | Yes |

## Dependencies

None - Standalone service.

---

[Back to Services Overview](../README.md)
