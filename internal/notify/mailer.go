package notify

import (
	"context"
	"fmt"
	"log/slog"
	"net/smtp"
	"sort"
	"strings"

	"github.com/mservice/gitscanner/libs/entity"
)

// Message is one outbound mail.
type Message struct {
	To      string
	Subject string
	Body    string
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

// SMTPSender delivers over SMTP.
type SMTPSender struct {
	addr string
	from string
	auth smtp.Auth
}

// NewSMTPSender builds a sender for addr ("host:port"). auth is used only when
// user is non-empty, so an unauthenticated relay stays supported.
func NewSMTPSender(addr, from, user, password string) *SMTPSender {
	s := &SMTPSender{addr: addr, from: from}
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
func (s *SMTPSender) Send(_ context.Context, m Message) error {
	payload := []byte("From: " + s.from + "\r\n" +
		"To: " + m.To + "\r\n" +
		"Subject: " + m.Subject + "\r\n" +
		"Content-Type: text/plain; charset=utf-8\r\n" +
		"\r\n" + m.Body + "\r\n")

	if err := smtp.SendMail(s.addr, s.auth, s.from, []string{m.To}, payload); err != nil {
		return fmt.Errorf("notify: send mail: %w", err)
	}
	return nil
}

// RenderDisclosure builds the disclosure mail for a repository.
//
// The body carries only what the maintainer needs to find and rotate the
// credential: the masked preview, the file and the line. It never contains the
// secret itself, and per plan.md §0.3 it never contains a payment link, a
// bounty request or any other ask — the mail exists to get the credential
// rotated, and anything that reads as leverage would undermine that.
func RenderDisclosure(repo entity.Repository, findings []entity.Finding, info DisclosureInfo) Message {
	var b strings.Builder

	fmt.Fprintf(&b, "Hello,\n\n")
	project := info.Project
	if project == "" {
		project = "an automated secret-scanning project"
	}
	fmt.Fprintf(&b, "I run %s. A scan of the public repository %s found %s that look like "+
		"live credentials committed to its history. I am writing so you can rotate them "+
		"before someone else finds and abuses them.\n\n",
		project, repo.FullName(), plural(len(findings), "value", "values"))

	fmt.Fprintf(&b, "What was found:\n\n")
	for _, f := range sortedBySeverity(findings) {
		fmt.Fprintf(&b, "  - %s", f.FilePath)
		if f.LineNumber > 0 {
			fmt.Fprintf(&b, ":%d", f.LineNumber)
		}
		fmt.Fprintf(&b, "\n    value:    %s\n", f.MaskedPreview)
		fmt.Fprintf(&b, "    severity: %s\n", f.Severity)
		if name := f.Metadata["provider_name"]; name != "" {
			fmt.Fprintf(&b, "    provider: %s\n", name)
		} else if f.ProviderSlug != "" {
			fmt.Fprintf(&b, "    provider: %s\n", f.ProviderSlug)
		}
		for _, line := range enrichmentLines(f) {
			fmt.Fprintf(&b, "    %s\n", line)
		}
		if f.ReachableFromHEAD {
			fmt.Fprintf(&b, "    note:     still present on the default branch\n")
		} else {
			fmt.Fprintf(&b, "    note:     not on the default branch, but still present in history\n")
		}
		b.WriteString("\n")
	}

	b.WriteString("Only a masked form of each value is shown above; the full values are not " +
		"stored or included here.\n\n")
	b.WriteString("What I suggest:\n\n")
	b.WriteString("  1. Rotate the affected credentials at the provider. This is the step that " +
		"matters -- a credential in public git history should be treated as compromised " +
		"even after the file is changed.\n")
	b.WriteString("  2. Remove the values from the repository, including history if you can.\n")
	b.WriteString("  3. Consider a pre-commit secret scan to catch the next one earlier.\n\n")
	b.WriteString("If these are test or placeholder values, or you would rather not receive " +
		"this kind of notice, reply and I will suppress future mail for your address or " +
		"domain.\n")

	if info.InfoURL != "" {
		fmt.Fprintf(&b, "\nMore about the project: %s\n", info.InfoURL)
	}
	if info.Signature != "" {
		fmt.Fprintf(&b, "\n%s\n", info.Signature)
	}

	return Message{
		To:      "",
		Subject: fmt.Sprintf("Credentials found in public repository %s", repo.FullName()),
		Body:    b.String(),
	}
}

// sortedBySeverity orders findings most severe first so the mail leads with
// what matters, without mutating the caller's slice.
func sortedBySeverity(findings []entity.Finding) []entity.Finding {
	out := make([]entity.Finding, len(findings))
	copy(out, findings)
	sort.SliceStable(out, func(i, j int) bool { return out[i].Severity > out[j].Severity })
	return out
}

func plural(n int, one, many string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, one)
	}
	return fmt.Sprintf("%d %s", n, many)
}

// enrichmentLines turns the non-secret facts the enricher gathered into lines
// for the mail: the provider category, whether a liveness probe found the key
// still active, and any identity it read back (account, owner). It never emits a
// secret — everything here is metadata the engine derived without the value.
func enrichmentLines(f entity.Finding) []string {
	var out []string
	if c := f.Metadata["category"]; c != "" {
		out = append(out, "category: "+c)
	}
	switch f.Metadata["probe_status"] {
	case "live":
		out = append(out, "status:   CONFIRMED LIVE by a check against the provider -- rotate this first")
	case "dead":
		out = append(out, "status:   the provider rejected it (already rotated or revoked)")
	}
	if id := f.Metadata["probe_owner"]; id != "" {
		out = append(out, "account:  authenticates as "+id)
	} else if id := f.Metadata["probe_account"]; id != "" {
		out = append(out, "account:  "+id)
	}
	if a := f.Metadata["aws_account_id"]; a != "" {
		out = append(out, "aws acct: "+a)
	}
	return out
}
