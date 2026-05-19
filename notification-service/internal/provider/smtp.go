package provider

import (
	"context"
	"fmt"
	"net/smtp"
	"os"
	"strconv"
)

type SMTPSender struct {
	host string
	port int
	user string
	pass string
	from string
}

func NewSMTPSender() (*SMTPSender, error) {
	port := 587
	if raw := os.Getenv("SMTP_PORT"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil {
			return nil, err
		}
		port = parsed
	}

	host := os.Getenv("SMTP_HOST")
	if host == "" {
		return nil, fmt.Errorf("SMTP_HOST is required for REAL provider")
	}

	return &SMTPSender{
		host: host,
		port: port,
		user: os.Getenv("SMTP_USER"),
		pass: os.Getenv("SMTP_PASS"),
		from: os.Getenv("SMTP_FROM"),
	}, nil
}

func (s *SMTPSender) Send(ctx context.Context, to, subject, body string) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	from := s.from
	if from == "" {
		from = s.user
	}

	addr := fmt.Sprintf("%s:%d", s.host, s.port)
	msg := []byte(fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\n\r\n%s", from, to, subject, body))

	var auth smtp.Auth
	if s.user != "" {
		auth = smtp.PlainAuth("", s.user, s.pass, s.host)
	}

	return smtp.SendMail(addr, auth, from, []string{to}, msg)
}
