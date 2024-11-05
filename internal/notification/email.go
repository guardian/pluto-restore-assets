package notification

import (
	"fmt"
	"log"
	"net/smtp"
	"os"
)

type SMTPEmailSender struct {
	from string
	host string
	port string
}

func NewSMTPEmailSender() *SMTPEmailSender {
	return &SMTPEmailSender{
		from: os.Getenv("SMTP_FROM"),
		host: os.Getenv("SMTP_HOST"),
		port: os.Getenv("SMTP_PORT"),
	}
}

func (s *SMTPEmailSender) SendEmail(subject, body string) error {
	log.Printf("Sending email to %s", os.Getenv("NOTIFICATION_EMAIL"))
	to := os.Getenv("NOTIFICATION_EMAIL")
	msg := fmt.Sprintf("Subject: %s\n\n%s", subject, body)

	return smtp.SendMail(s.host+":"+s.port, nil, s.from, []string{to}, []byte(msg))
}
