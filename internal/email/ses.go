package email

import (
	"context"
	"errors"
	"fmt"
	"net/mail"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/sesv2"
	"github.com/aws/aws-sdk-go-v2/service/sesv2/types"
)

func init() {
	Register("ses", newSESFromEnv)
}

// SESAPI is the subset of the SES v2 client used by SESProvider.
type SESAPI interface {
	SendEmail(ctx context.Context, in *sesv2.SendEmailInput, optFns ...func(*sesv2.Options)) (*sesv2.SendEmailOutput, error)
}

// SESProvider delivers notifications through the Amazon SES v2 API.
type SESProvider struct {
	Client           SESAPI
	FromEmail        string
	FromName         string
	To               string
	ConfigurationSet string // optional, enables SES event publishing
}

// newSESFromEnv reads SES_AWS_* first and falls back to the standard AWS_*
// variables and credential chain. The SES_ prefix exists because Vercel
// reserves AWS_REGION, AWS_ACCESS_KEY_ID and AWS_SECRET_ACCESS_KEY.
func newSESFromEnv(ctx context.Context, opts Options) (EmailProvider, error) {
	from := strings.TrimSpace(opts.Lookup("SES_FROM_EMAIL"))
	if from == "" {
		return nil, errors.New("ses: missing required environment variables: SES_FROM_EMAIL")
	}
	if _, err := mail.ParseAddress(from); err != nil {
		return nil, fmt.Errorf("ses: invalid SES_FROM_EMAIL: %w", err)
	}
	region := firstNonEmpty(opts.Lookup("SES_AWS_REGION"), opts.Lookup("AWS_REGION"))
	if region == "" {
		return nil, errors.New("ses: missing required environment variables: SES_AWS_REGION (or AWS_REGION)")
	}

	loadOpts := []func(*awsconfig.LoadOptions) error{awsconfig.WithRegion(region)}
	accessKey := strings.TrimSpace(opts.Lookup("SES_AWS_ACCESS_KEY_ID"))
	secretKey := strings.TrimSpace(opts.Lookup("SES_AWS_SECRET_ACCESS_KEY"))
	if accessKey != "" || secretKey != "" {
		if accessKey == "" || secretKey == "" {
			return nil, errors.New("ses: SES_AWS_ACCESS_KEY_ID and SES_AWS_SECRET_ACCESS_KEY must be set together")
		}
		loadOpts = append(loadOpts, awsconfig.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(accessKey, secretKey, opts.Lookup("SES_AWS_SESSION_TOKEN")),
		))
	}

	cfg, err := awsconfig.LoadDefaultConfig(ctx, loadOpts...)
	if err != nil {
		return nil, fmt.Errorf("ses: load aws config: %w", err)
	}
	return &SESProvider{
		Client:           sesv2.NewFromConfig(cfg),
		FromEmail:        from,
		FromName:         opts.FromName,
		To:               opts.To,
		ConfigurationSet: strings.TrimSpace(opts.Lookup("SES_CONFIGURATION_SET")),
	}, nil
}

// Name implements EmailProvider.
func (s *SESProvider) Name() string { return "ses" }

// SendQuoteNotification implements EmailProvider.
func (s *SESProvider) SendQuoteNotification(ctx context.Context, notif QuoteNotification) SendResult {
	res := SendResult{Provider: s.Name()}
	msg, err := BuildMessage(Envelope{From: mail.Address{Name: s.FromName, Address: s.FromEmail}, To: s.To}, notif)
	if err != nil {
		res.Err = err
		return res
	}

	utf8 := aws.String("UTF-8")
	input := &sesv2.SendEmailInput{
		FromEmailAddress: aws.String(msg.From.String()),
		Destination:      &types.Destination{ToAddresses: msg.Recipients},
		Content: &types.EmailContent{
			Simple: &types.Message{
				Subject: &types.Content{Data: aws.String(msg.Subject), Charset: utf8},
				Body: &types.Body{
					Text: &types.Content{Data: aws.String(msg.Text), Charset: utf8},
					Html: &types.Content{Data: aws.String(msg.HTML), Charset: utf8},
				},
			},
		},
	}
	if msg.ReplyTo != nil {
		input.ReplyToAddresses = []string{msg.ReplyTo.String()}
	}
	if s.ConfigurationSet != "" {
		input.ConfigurationSetName = aws.String(s.ConfigurationSet)
	}

	out, err := s.Client.SendEmail(ctx, input)
	if err != nil {
		res.Err = fmt.Errorf("ses: send email: %w", err)
		return res
	}
	res.MessageID = aws.ToString(out.MessageId)
	return res
}
