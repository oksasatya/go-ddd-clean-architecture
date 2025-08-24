package mailer

import (
	"context"
	"time"

	mg "github.com/mailgun/mailgun-go/v4"
)

// Mailgun wraps Mailgun client configuration.
type Mailgun struct {
	Domain string
	APIKey string
	Sender string
}

func NewMailgun(domain, apiKey, sender string) *Mailgun {
	return &Mailgun{Domain: domain, APIKey: apiKey, Sender: sender}
}

// Send sends an email via Mailgun. html is optional; if provided it will be used as HTML body.
func (m *Mailgun) Send(ctx context.Context, to, subject, text, html string) error {
	client := mg.NewMailgun(m.Domain, m.APIKey)
	msg := client.NewMessage(m.Sender, subject, text, to)
	if html != "" {
		msg.SetHtml(html)
	}
	c, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	_, _, err := client.Send(c, msg)
	return err
}
