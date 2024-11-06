package notification

import (
	"fmt"
	"log"
	"net"
	"net/smtp"
)

type SMTPEmailSender struct {
	host   string
	port   string
	from   string
	to     string
	useTLS bool
}

func NewSMTPEmailSender(host, port, from, to string) *SMTPEmailSender {
	useTLS := port != "25"

	return &SMTPEmailSender{
		host:   host,
		port:   port,
		from:   from,
		to:     to,
		useTLS: useTLS,
	}
}

func (s *SMTPEmailSender) SendEmail(subject, body string) error {
	log.Printf("Sending email to %s", s.to)

	// Create message
	msg := []byte(fmt.Sprintf("To: %s\r\n"+
		"From: %s\r\n"+
		"Subject: %s\r\n"+
		"\r\n"+
		"%s\r\n", s.to, s.from, subject, body))

	// Simple SMTP connection without TLS or auth
	c, err := smtp.Dial(net.JoinHostPort(s.host, s.port))
	if err != nil {
		return fmt.Errorf("failed to connect to SMTP server: %w", err)
	}
	defer c.Close()

	if err := c.Mail(s.from); err != nil {
		return fmt.Errorf("failed to set sender: %w", err)
	}
	if err := c.Rcpt(s.to); err != nil {
		return fmt.Errorf("failed to set recipient: %w", err)
	}

	w, err := c.Data()
	if err != nil {
		return fmt.Errorf("failed to create data writer: %w", err)
	}
	defer w.Close()

	_, err = w.Write(msg)
	if err != nil {
		return fmt.Errorf("failed to write message: %w", err)
	}

	return nil
}
