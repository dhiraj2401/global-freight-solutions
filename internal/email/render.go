package email

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	htmltemplate "html/template"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net/mail"
	"net/textproto"
	"strconv"
	"strings"
	texttemplate "text/template"
	"time"
	"unicode"
)

// Envelope identifies sender and recipient for a rendered message.
type Envelope struct {
	From mail.Address
	To   string
}

// Message is a fully rendered notification. API-based providers use the
// structured fields; SMTP providers send Raw.
type Message struct {
	ID         string // Message-ID without angle brackets
	From       mail.Address
	To         mail.Address
	ReplyTo    *mail.Address // prospect's address; nil if it failed to parse
	Subject    string
	Text       string
	HTML       string
	Raw        []byte // RFC 5322 message with CRLF line endings
	Recipients []string
}

// BuildMessage renders the sales notification for n.
//
// The prospect appears as the sender in the inbox: the From display name is
// "Name (prospect@example.com)" and Reply-To is the prospect's address. The
// From address itself stays the verified sender (env.From.Address), because
// sending from the prospect's own domain is rejected by providers and fails
// DMARC at the receiving server. env.From.Name is the fallback display name
// when the prospect's email can't be parsed.
func BuildMessage(env Envelope, n QuoteNotification) (*Message, error) {
	if _, err := mail.ParseAddress(env.From.Address); err != nil {
		return nil, fmt.Errorf("email: invalid from address: %w", err)
	}
	to, err := mail.ParseAddress(env.To)
	if err != nil {
		return nil, fmt.Errorf("email: invalid recipient address: %w", err)
	}

	msg := &Message{
		ID:         newMessageID(env.From.Address),
		From:       mail.Address{Name: headerSafe(env.From.Name, 80), Address: env.From.Address},
		To:         *to,
		Subject:    subjectFor(n),
		Recipients: []string{to.Address},
	}
	if addr, err := mail.ParseAddress(strings.TrimSpace(n.Email)); err == nil {
		addr.Name = headerSafe(strings.TrimSpace(n.FirstName+" "+n.LastName), 80)
		msg.ReplyTo = addr
		msg.From.Name = addr.Address
		if addr.Name != "" {
			msg.From.Name = headerSafe(addr.Name+" ("+addr.Address+")", 200)
		}
	}

	view := newNotificationView(n)
	var text, html bytes.Buffer
	if err := textBody.Execute(&text, view); err != nil {
		return nil, fmt.Errorf("email: render text body: %w", err)
	}
	if err := htmlBody.Execute(&html, view); err != nil {
		return nil, fmt.Errorf("email: render html body: %w", err)
	}
	msg.Text = text.String()
	msg.HTML = html.String()

	raw, err := buildMIME(msg, n.Reference)
	if err != nil {
		return nil, err
	}
	msg.Raw = raw
	return msg, nil
}

func subjectFor(n QuoteNotification) string {
	s := fmt.Sprintf("New quote request %s: %s → %s", n.Reference, n.Origin, n.Destination)
	if c := strings.TrimSpace(n.Company); c != "" {
		s += " (" + c + ")"
	}
	return headerSafe(s, 200)
}

// headerSafe strips CR, LF and other control characters (preventing header
// injection), collapses whitespace and truncates to max runes.
func headerSafe(s string, max int) string {
	var b strings.Builder
	space := false
	count := 0
	for _, r := range s {
		if unicode.IsControl(r) || unicode.IsSpace(r) {
			if !space && b.Len() > 0 {
				b.WriteByte(' ')
				count++
			}
			space = true
			continue
		}
		if count >= max {
			break
		}
		space = false
		b.WriteRune(r)
		count++
	}
	return strings.TrimSpace(b.String())
}

func newMessageID(from string) string {
	domain := "localhost"
	if at := strings.LastIndexByte(from, '@'); at >= 0 && at < len(from)-1 {
		domain = from[at+1:]
	}
	buf := make([]byte, 16)
	_, _ = rand.Read(buf)
	return hex.EncodeToString(buf) + "@" + domain
}

func buildMIME(msg *Message, reference string) ([]byte, error) {
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	for _, part := range []struct{ contentType, content string }{
		{"text/plain; charset=utf-8", msg.Text},
		{"text/html; charset=utf-8", msg.HTML},
	} {
		w, err := mw.CreatePart(textproto.MIMEHeader{
			"Content-Type":              {part.contentType},
			"Content-Transfer-Encoding": {"quoted-printable"},
		})
		if err != nil {
			return nil, fmt.Errorf("email: create mime part: %w", err)
		}
		qp := quotedprintable.NewWriter(w)
		if _, err := qp.Write([]byte(strings.ReplaceAll(part.content, "\n", "\r\n"))); err != nil {
			return nil, fmt.Errorf("email: encode mime part: %w", err)
		}
		if err := qp.Close(); err != nil {
			return nil, fmt.Errorf("email: encode mime part: %w", err)
		}
	}
	if err := mw.Close(); err != nil {
		return nil, fmt.Errorf("email: close mime writer: %w", err)
	}

	var out bytes.Buffer
	header := func(k, v string) { out.WriteString(k + ": " + v + "\r\n") }
	header("From", msg.From.String())
	header("To", msg.To.String())
	if msg.ReplyTo != nil {
		header("Reply-To", msg.ReplyTo.String())
	}
	header("Subject", mime.QEncoding.Encode("utf-8", msg.Subject))
	header("Date", time.Now().UTC().Format(time.RFC1123Z))
	header("Message-ID", "<"+msg.ID+">")
	header("MIME-Version", "1.0")
	header("Auto-Submitted", "auto-generated")
	if ref := headerSafe(reference, 64); ref != "" {
		header("X-Quote-Reference", ref)
	}
	header("Content-Type", "multipart/alternative; boundary="+strconv.Quote(mw.Boundary()))
	out.WriteString("\r\n")
	out.Write(body.Bytes())
	return out.Bytes(), nil
}

type notificationView struct {
	Reference    string
	Name         string
	Company      string
	Email        string
	Phone        string
	Origin       string
	Destination  string
	ServiceType  string
	Weight       string
	CargoDetails string
	SubmittedAt  string
}

func newNotificationView(n QuoteNotification) notificationView {
	orDash := func(s string) string {
		if s = strings.TrimSpace(s); s == "" {
			return "—"
		}
		return s
	}
	weight := "—"
	if n.WeightKg > 0 {
		weight = strconv.FormatFloat(n.WeightKg, 'f', -1, 64) + " kg"
	}
	submitted := n.SubmittedAt
	if submitted.IsZero() {
		submitted = time.Now()
	}
	return notificationView{
		Reference:    orDash(n.Reference),
		Name:         orDash(n.FirstName + " " + n.LastName),
		Company:      orDash(n.Company),
		Email:        orDash(n.Email),
		Phone:        orDash(n.Phone),
		Origin:       orDash(n.Origin),
		Destination:  orDash(n.Destination),
		ServiceType:  orDash(n.ServiceType),
		Weight:       weight,
		CargoDetails: orDash(n.CargoDetails),
		SubmittedAt:  submitted.UTC().Format("Mon 02 Jan 2006, 15:04 MST"),
	}
}

var textBody = texttemplate.Must(texttemplate.New("text").Parse(`New quote request {{.Reference}}
Submitted {{.SubmittedAt}}

CONTACT
  Name:         {{.Name}}
  Company:      {{.Company}}
  Email:        {{.Email}}
  Phone:        {{.Phone}}

SHIPMENT
  Service:      {{.ServiceType}}
  Origin:       {{.Origin}}
  Destination:  {{.Destination}}
  Weight:       {{.Weight}}

CARGO DETAILS
{{.CargoDetails}}

Reply to this email to respond to the prospect directly.
`))

var htmlBody = htmltemplate.Must(htmltemplate.New("html").Parse(`<!doctype html>
<html><body style="margin:0;background:#f6f1ea;font-family:Arial,Helvetica,sans-serif;color:#1a1410">
<table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="background:#f6f1ea;padding:24px 0">
<tr><td align="center">
<table role="presentation" width="600" cellpadding="0" cellspacing="0" style="max-width:600px;background:#ffffff;border-radius:12px;overflow:hidden">
<tr><td style="background:#24170f;padding:20px 28px;color:#ffffff">
<div style="font-size:12px;letter-spacing:.08em;text-transform:uppercase;color:#f0a46f">New quote request</div>
<div style="font-size:22px;font-weight:bold;margin-top:4px">{{.Reference}}</div>
<div style="font-size:13px;color:#d6c8bb;margin-top:4px">Submitted {{.SubmittedAt}}</div>
</td></tr>
<tr><td style="padding:24px 28px">
<h2 style="font-size:14px;text-transform:uppercase;letter-spacing:.06em;color:#d9642e;margin:0 0 8px">Contact</h2>
<table role="presentation" width="100%" cellpadding="6" cellspacing="0" style="font-size:14px">
<tr><td style="color:#6b6259;width:140px">Name</td><td>{{.Name}}</td></tr>
<tr><td style="color:#6b6259">Company</td><td>{{.Company}}</td></tr>
<tr><td style="color:#6b6259">Email</td><td>{{.Email}}</td></tr>
<tr><td style="color:#6b6259">Phone</td><td>{{.Phone}}</td></tr>
</table>
<h2 style="font-size:14px;text-transform:uppercase;letter-spacing:.06em;color:#d9642e;margin:20px 0 8px">Shipment</h2>
<table role="presentation" width="100%" cellpadding="6" cellspacing="0" style="font-size:14px">
<tr><td style="color:#6b6259;width:140px">Service</td><td>{{.ServiceType}}</td></tr>
<tr><td style="color:#6b6259">Origin</td><td>{{.Origin}}</td></tr>
<tr><td style="color:#6b6259">Destination</td><td>{{.Destination}}</td></tr>
<tr><td style="color:#6b6259">Weight</td><td>{{.Weight}}</td></tr>
</table>
<h2 style="font-size:14px;text-transform:uppercase;letter-spacing:.06em;color:#d9642e;margin:20px 0 8px">Cargo details</h2>
<p style="font-size:14px;line-height:1.5;white-space:pre-wrap;margin:0">{{.CargoDetails}}</p>
<p style="font-size:12px;color:#6b6259;margin:24px 0 0">Reply to this email to respond to the prospect directly.</p>
</td></tr>
</table>
</td></tr>
</table>
</body></html>
`))
