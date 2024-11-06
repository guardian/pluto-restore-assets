package notification

import (
	"fmt"
	"log"
	"net"
	"net/smtp"
)

type SMTPEmailSender struct {
	host string
	port string
	from string
	to   string
}

func NewSMTPEmailSender(host, port, from, to string) *SMTPEmailSender {
	return &SMTPEmailSender{
		host: host,
		port: port,
		from: from,
		to:   to,
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

	// Use plain SMTP without TLS for internal mail server
	err := smtp.SendMail(
		net.JoinHostPort(s.host, s.port),
		nil, // No authentication needed for internal mail server
		s.from,
		[]string{s.to},
		msg,
	)

	if err != nil {
		return fmt.Errorf("failed to send email: %w", err)
	}

	log.Printf("Email sent successfully to %s", s.to)
	return nil
}
