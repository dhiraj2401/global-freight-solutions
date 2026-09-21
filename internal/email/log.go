package email

import (
	"context"
	"log/slog"
	"net/mail"
)

func init() {
	Register("log", func(_ context.Context, opts Options) (EmailProvider, error) {
		return &LogProvider{To: opts.To, FromName: opts.FromName, Logger: slog.Default()}, nil
	})
}

// LogProvider renders the notification and writes a summary to the logger
// instead of sending it. It is the default in development so the app runs
// without email credentials. Never use it in production.
type LogProvider struct {
	To       string
	FromName string
	Logger   *slog.Logger
}

// Name implements EmailProvider.
func (l *LogProvider) Name() string { return "log" }

// SendQuoteNotification implements EmailProvider.
func (l *LogProvider) SendQuoteNotification(_ context.Context, notif QuoteNotification) SendResult {
	res := SendResult{Provider: l.Name()}
	msg, err := BuildMessage(Envelope{From: mail.Address{Name: l.FromName, Address: "notifications@localhost.test"}, To: l.To}, notif)
	if err != nil {
		res.Err = err
		return res
	}
	res.MessageID = msg.ID
	replyTo := ""
	if msg.ReplyTo != nil {
		replyTo = msg.ReplyTo.String()
	}
	l.Logger.Info("email (log provider): quote notification rendered, not sent",
		"message_id", msg.ID, "to", msg.To.String(), "reply_to", replyTo, "subject", msg.Subject)
	l.Logger.Debug("email (log provider): body", "text", msg.Text)
	return res
}
