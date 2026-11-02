// Package mailer delivers transactional emails.
package mailer

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/smtp"
	"strconv"
	"strings"
	"sync"
	"time"
)

// LogMailer writes emails to the log instead of sending them. Default in development,
// so verification / reset links can be copied from `docker compose logs api`.
type LogMailer struct{ Logger *slog.Logger }

func (m LogMailer) Send(ctx context.Context, to, subject, body string) error {
	m.Logger.InfoContext(ctx, "email (not sent: SMTP not configured)", "to", to, "subject", subject, "body", body)
	return nil
}

// SMTPMailer sends plain-text mail through an SMTP relay (STARTTLS when offered).
type SMTPMailer struct {
	Host     string
	Port     int
	Username string
	Password string
	From     string
}

func (m SMTPMailer) Send(_ context.Context, to, subject, body string) error {
	if strings.ContainsAny(to+subject, "\r\n") {
		return fmt.Errorf("mailer: header injection attempt")
	}
	addr := net.JoinHostPort(m.Host, strconv.Itoa(m.Port))
	var auth smtp.Auth
	if m.Username != "" {
		auth = smtp.PlainAuth("", m.Username, m.Password, m.Host)
	}
	msg := strings.Join([]string{
		"From: " + m.From,
		"To: " + to,
		"Subject: " + subject,
		"Date: " + time.Now().UTC().Format(time.RFC1123Z),
		"MIME-Version: 1.0",
		"Content-Type: text/plain; charset=UTF-8",
		"",
		body,
	}, "\r\n")
	return smtp.SendMail(addr, auth, m.From, []string{to}, []byte(msg))
}

// Recorder keeps sent messages in memory (tests).
type Recorder struct {
	mu   sync.Mutex
	Sent []Message
}

type Message struct{ To, Subject, Body string }

func (r *Recorder) Send(_ context.Context, to, subject, body string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.Sent = append(r.Sent, Message{to, subject, body})
	return nil
}

func (r *Recorder) Last() Message {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.Sent) == 0 {
		return Message{}
	}
	return r.Sent[len(r.Sent)-1]
}
