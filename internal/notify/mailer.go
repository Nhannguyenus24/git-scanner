package notify

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net/mail"
	"net/smtp"
	"net/textproto"
	"strings"
	"time"

	"github.com/mservice/gitscanner/libs/id"
)

// Message is one outbound mail. Body is the plain-text part every client can
// read; HTML, when set, is sent alongside it as the preferred alternative.
type Message struct {
	To      string
	Subject string
	Body    string
	HTML    string
}

// DisclosureInfo is the operator's identity and the project's description,
// configured from the environment. It personalises the disclosure mail so the
// recipient knows who is contacting them and can look the project up.
//
// It carries no payment link, bounty or other ask. plan.md §0.3 keeps that out
// of disclosure mail on purpose: a security notice that also solicits reads as
// leverage and undermines the one thing the mail is for — getting the
// credential rotated. The InfoURL is informational (the project's page), never
// a donation link.
type DisclosureInfo struct {
	Project   string // e.g. "GitScanner, a community secret-scanning project"
	Signature string // e.g. "— Nhan, a software engineer in Vietnam"
	InfoURL   string // a page describing the project; optional
}

// Sender delivers a Message. Implementations must be safe for concurrent use.
type Sender interface {
	Send(ctx context.Context, m Message) error
}

// LogSender writes the mail to the log instead of delivering it.
//
// It is the default whenever SMTP_ADDR is unset, which means dev and CI run the
// whole disclosure path — suppression, rate cap, the one-per-repository
// constraint and rendering — without ever sending real mail to a real
// maintainer. An accidentally unconfigured production deployment therefore
// fails visibly in the logs rather than silently mailing strangers.
type LogSender struct {
	log *slog.Logger
}

// NewLogSender returns a sender that only logs.
func NewLogSender(log *slog.Logger) *LogSender { return &LogSender{log: log} }

// Send logs the message and reports success.
func (s *LogSender) Send(_ context.Context, m Message) error {
	s.log.Info("disclosure mail (not sent: no SMTP configured)",
		"to", m.To, "subject", m.Subject, "body_bytes", len(m.Body))
	return nil
}

// SMTPSender delivers over SMTP. net/smtp upgrades to STARTTLS whenever the
// server offers it, which Gmail's submission port (587) requires.
type SMTPSender struct {
	addr     string
	from     string // header form, may carry a display name
	envelope string // bare address for MAIL FROM
	auth     smtp.Auth
}

// NewSMTPSender builds a sender for addr ("host:port"). auth is used only when
// user is non-empty, so an unauthenticated relay stays supported.
func NewSMTPSender(addr, from, user, password string) *SMTPSender {
	s := &SMTPSender{addr: addr, from: from, envelope: from}
	if a, err := mail.ParseAddress(from); err == nil {
		s.envelope = a.Address
	}
	if user != "" {
		host, _, found := strings.Cut(addr, ":")
		if !found {
			host = addr
		}
		s.auth = smtp.PlainAuth("", user, password, host)
	}
	return s
}

// Send delivers one message.
//
// Date and Message-ID are set here because receiving filters score a mail
// missing them as spam, and a disclosure in a spam folder is no disclosure.
func (s *SMTPSender) Send(_ context.Context, m Message) error {
	domain := "localhost"
	if _, d, ok := strings.Cut(s.envelope, "@"); ok {
		domain = d
	}

	var buf bytes.Buffer
	buf.WriteString("From: " + s.from + "\r\n" +
		"To: " + m.To + "\r\n" +
		"Subject: " + mime.QEncoding.Encode("utf-8", m.Subject) + "\r\n" +
		"Date: " + time.Now().Format(time.RFC1123Z) + "\r\n" +
		"Message-ID: <" + id.New().String() + "@" + domain + ">\r\n" +
		"MIME-Version: 1.0\r\n")

	if m.HTML == "" {
		buf.WriteString("Content-Type: text/plain; charset=utf-8\r\n" +
			"Content-Transfer-Encoding: quoted-printable\r\n\r\n")
		if err := writeQP(&buf, m.Body); err != nil {
			return err
		}
	} else {
		// multipart/alternative lists the plainest part first; clients render
		// the last one they understand, so HTML wins and text is the fallback.
		mw := multipart.NewWriter(&buf)
		buf.WriteString("Content-Type: multipart/alternative; boundary=" + mw.Boundary() + "\r\n\r\n")
		for _, part := range []struct{ ctype, content string }{
			{"text/plain; charset=utf-8", m.Body},
			{"text/html; charset=utf-8", m.HTML},
		} {
			w, err := mw.CreatePart(textproto.MIMEHeader{
				"Content-Type":              {part.ctype},
				"Content-Transfer-Encoding": {"quoted-printable"},
			})
			if err != nil {
				return fmt.Errorf("notify: mime part: %w", err)
			}
			if err := writeQP(w, part.content); err != nil {
				return err
			}
		}
		if err := mw.Close(); err != nil {
			return fmt.Errorf("notify: mime close: %w", err)
		}
	}

	if err := smtp.SendMail(s.addr, s.auth, s.envelope, []string{m.To}, buf.Bytes()); err != nil {
		return fmt.Errorf("notify: send mail: %w", err)
	}
	return nil
}

// writeQP encodes as quoted-printable, which keeps every line under SMTP's
// 998-byte limit; long HTML lines sent as 8bit get truncated or rejected.
func writeQP(w io.Writer, s string) error {
	qp := quotedprintable.NewWriter(w)
	if _, err := io.WriteString(qp, strings.ReplaceAll(s, "\n", "\r\n")); err != nil {
		return fmt.Errorf("notify: encode body: %w", err)
	}
	if err := qp.Close(); err != nil {
		return fmt.Errorf("notify: encode body: %w", err)
	}
	return nil
}
