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

	client, err := smtp.Dial(s.host + ":" + s.port)
	if err != nil {
		return fmt.Errorf("failed to connect to SMTP server: %v", err)
	}
	defer client.Close()

	if err := client.Mail(s.from); err != nil {
		return fmt.Errorf("MAIL FROM error: %v", err)
	}
	if err := client.Rcpt(to); err != nil {
		return fmt.Errorf("RCPT TO error: %v", err)
	}

	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("DATA error: %v", err)
	}
	defer w.Close()

	_, err = w.Write([]byte(msg))
	return err
}
