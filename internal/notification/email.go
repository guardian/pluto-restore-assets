package notification

import (
	"fmt"
	"log"
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
	msg := fmt.Sprintf("Subject: %s\n\n%s", subject, body)

	err := smtp.SendMail(
		s.host+":"+s.port,
		nil, // No auth for now, add if needed
		s.from,
		[]string{s.to},
		[]byte(msg),
	)
	if err != nil {
		return fmt.Errorf("failed to send email: %w", err)
	}

	log.Printf("Email sent successfully to %s", s.to)
	return nil
}
