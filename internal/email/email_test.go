package email

import (
	"context"
	"net/mail"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sesv2"
)

func lookupFrom(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

func sampleNotification() QuoteNotification {
	return QuoteNotification{
		Reference:    "GFS-260914-ABCDEF",
		FirstName:    "Jane",
		LastName:     "Doe",
		Company:      "Acme Corp",
		Email:        "jane@acme.test",
		Phone:        "+1 713 555 0100",
		Origin:       "Houston, TX",
		Destination:  "Dallas, TX",
		ServiceType:  "Full Truckload (FTL)",
		WeightKg:     1200,
		CargoDetails: "2 pallets",
		SubmittedAt:  time.Date(2026, 9, 14, 12, 0, 0, 0, time.UTC),
	}
}

var testEnvelope = Envelope{From: mail.Address{Name: "Website", Address: "notify@gfs.test"}, To: "sales@gfs.test"}

func TestBuildMessageUsesProspectOnlyAsReplyTo(t *testing.T) {
	msg, err := BuildMessage(testEnvelope, sampleNotification())
	if err != nil {
		t.Fatal(err)
	}
	if msg.From.Address != "notify@gfs.test" {
		t.Errorf("From = %q, want the verified sender", msg.From.Address)
	}
	if msg.ReplyTo == nil || msg.ReplyTo.Address != "jane@acme.test" {
		t.Fatalf("ReplyTo = %v, want jane@acme.test", msg.ReplyTo)
	}

	parsed, err := mail.ReadMessage(strings.NewReader(string(msg.Raw)))
	if err != nil {
		t.Fatalf("raw message is not valid RFC 5322: %v", err)
	}
	if got := parsed.Header.Get("Reply-To"); !strings.Contains(got, "jane@acme.test") {
		t.Errorf("Reply-To header = %q", got)
	}
	if got := parsed.Header.Get("From"); !strings.Contains(got, "notify@gfs.test") {
		t.Errorf("From header = %q", got)
	}
	if got := parsed.Header.Get("X-Quote-Reference"); got != "GFS-260914-ABCDEF" {
		t.Errorf("X-Quote-Reference = %q", got)
	}
}

func TestBuildMessagePreventsHeaderInjection(t *testing.T) {
	n := sampleNotification()
	n.FirstName = "Eve\r\nBcc: attacker@evil.test"
	n.Company = "Acme\nX-Injected: yes"
	n.Origin = "Houston\r\n\r\nbody"

	msg, err := BuildMessage(testEnvelope, n)
	if err != nil {
		t.Fatal(err)
	}
	header, _, _ := strings.Cut(string(msg.Raw), "\r\n\r\n")
	for _, line := range strings.Split(header, "\r\n") {
		lower := strings.ToLower(line)
		if strings.HasPrefix(lower, "bcc:") || strings.HasPrefix(lower, "x-injected:") {
			t.Fatalf("injected header line found: %q", line)
		}
	}
	if strings.ContainsAny(msg.Subject, "\r\n") {
		t.Errorf("subject contains line breaks: %q", msg.Subject)
	}
	if msg.ReplyTo != nil && strings.ContainsAny(msg.ReplyTo.Name, "\r\n") {
		t.Errorf("reply-to name contains line breaks: %q", msg.ReplyTo.Name)
	}
}

func TestBuildMessageEscapesHTML(t *testing.T) {
	n := sampleNotification()
	n.CargoDetails = `<script>alert("x")</script>`
	msg, err := BuildMessage(testEnvelope, n)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(msg.HTML, "<script>") {
		t.Error("HTML body contains unescaped user input")
	}
}

func TestBuildMessageOmitsInvalidReplyTo(t *testing.T) {
	n := sampleNotification()
	n.Email = "not-an-email"
	msg, err := BuildMessage(testEnvelope, n)
	if err != nil {
		t.Fatal(err)
	}
	if msg.ReplyTo != nil {
		t.Errorf("ReplyTo = %v, want nil", msg.ReplyTo)
	}
	if msg.From.Name != "Website" {
		t.Errorf("From name = %q, want fallback EMAIL_FROM_NAME", msg.From.Name)
	}
}

func TestBuildMessageShowsProspectAsSender(t *testing.T) {
	msg, err := BuildMessage(testEnvelope, sampleNotification())
	if err != nil {
		t.Fatal(err)
	}
	if msg.From.Name != "Jane Doe (jane@acme.test)" || msg.From.Address != "notify@gfs.test" {
		t.Errorf("From = %q <%s>", msg.From.Name, msg.From.Address)
	}
	parsed, err := mail.ReadMessage(strings.NewReader(string(msg.Raw)))
	if err != nil {
		t.Fatal(err)
	}
	from, err := mail.ParseAddress(parsed.Header.Get("From"))
	if err != nil || from.Name != "Jane Doe (jane@acme.test)" || from.Address != "notify@gfs.test" {
		t.Errorf("From header = %q (%v)", parsed.Header.Get("From"), err)
	}

	n := sampleNotification()
	n.FirstName, n.LastName = "", ""
	msg, err = BuildMessage(testEnvelope, n)
	if err != nil {
		t.Fatal(err)
	}
	if msg.From.Name != "jane@acme.test" {
		t.Errorf("From name without a name = %q", msg.From.Name)
	}
}

func TestProvidersAreRegistered(t *testing.T) {
	got := strings.Join(Providers(), ",")
	for _, name := range []string{"brevo", "log", "ses"} {
		if !strings.Contains(got, name) {
			t.Errorf("provider %q not registered (have %s)", name, got)
		}
	}
}

func TestNewEmailProviderUnknown(t *testing.T) {
	_, err := NewEmailProvider(context.Background(), "carrier-pigeon", Options{To: "sales@gfs.test", Lookup: lookupFrom(nil)})
	if err == nil || !strings.Contains(err.Error(), "unknown provider") || !strings.Contains(err.Error(), "brevo") {
		t.Fatalf("err = %v, want unknown provider error listing registered providers", err)
	}
}

// Switching providers must be a configuration change only: identical code
// path, different EMAIL_PROVIDER value.
func TestSwitchingProvidersIsConfigurationOnly(t *testing.T) {
	env := lookupFrom(map[string]string{
		"BREVO_API_KEY":             "xkeysib-test",
		"BREVO_FROM_EMAIL":          "notify@gfs.test",
		"SES_FROM_EMAIL":            "notify@gfs.test",
		"SES_AWS_REGION":            "us-east-1",
		"SES_AWS_ACCESS_KEY_ID":     "AKIDEXAMPLE",
		"SES_AWS_SECRET_ACCESS_KEY": "secret",
	})
	for _, name := range []string{"brevo", "ses", "log"} {
		p, err := NewEmailProvider(context.Background(), name, Options{To: "sales@gfs.test", Lookup: env})
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if p.Name() != name {
			t.Errorf("Name() = %q, want %q", p.Name(), name)
		}
	}
}

func TestSESRequiresRegion(t *testing.T) {
	_, err := NewEmailProvider(context.Background(), "ses", Options{To: "sales@gfs.test", Lookup: lookupFrom(map[string]string{"SES_FROM_EMAIL": "notify@gfs.test"})})
	if err == nil || !strings.Contains(err.Error(), "SES_AWS_REGION") {
		t.Fatalf("err = %v, want missing region", err)
	}
}

type fakeSES struct {
	in *sesv2.SendEmailInput
}

func (f *fakeSES) SendEmail(_ context.Context, in *sesv2.SendEmailInput, _ ...func(*sesv2.Options)) (*sesv2.SendEmailOutput, error) {
	f.in = in
	return &sesv2.SendEmailOutput{MessageId: aws.String("ses-message-1")}, nil
}

func TestSESSendBuildsRequest(t *testing.T) {
	client := &fakeSES{}
	p := &SESProvider{Client: client, FromEmail: "notify@gfs.test", FromName: "Website", To: "sales@gfs.test", ConfigurationSet: "web"}
	res := p.SendQuoteNotification(context.Background(), sampleNotification())
	if res.Err != nil {
		t.Fatal(res.Err)
	}
	if res.MessageID != "ses-message-1" || res.Provider != "ses" {
		t.Errorf("result = %+v", res)
	}
	in := client.in
	if !strings.Contains(aws.ToString(in.FromEmailAddress), "notify@gfs.test") {
		t.Errorf("From = %q", aws.ToString(in.FromEmailAddress))
	}
	if len(in.ReplyToAddresses) != 1 || !strings.Contains(in.ReplyToAddresses[0], "jane@acme.test") {
		t.Errorf("ReplyTo = %v", in.ReplyToAddresses)
	}
	if got := in.Destination.ToAddresses; len(got) != 1 || got[0] != "sales@gfs.test" {
		t.Errorf("To = %v", got)
	}
	if aws.ToString(in.ConfigurationSetName) != "web" {
		t.Errorf("ConfigurationSetName = %q", aws.ToString(in.ConfigurationSetName))
	}
}
