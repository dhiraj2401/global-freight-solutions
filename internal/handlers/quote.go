package handlers

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/mail"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gloitel/global-freight-solutions/internal/content"
	"github.com/gloitel/global-freight-solutions/internal/db"
	"github.com/gloitel/global-freight-solutions/internal/email"
	"github.com/gloitel/global-freight-solutions/internal/phone"
)

const (
	variantCompact = "compact"
	variantFull    = "full"

	maxQuoteFormBytes = 32 << 10
	maxUserAgentBytes = 512
)

// quoteInput is the raw submission, echoed back when re-rendering the form.
type quoteInput struct {
	Variant      string
	FullName     string
	FirstName    string
	LastName     string
	Company      string
	Email        string
	Phone        string
	Origin       string
	Destination  string
	ServiceType  string
	WeightKg     string
	CargoDetails string
	Website      string // honeypot
}

// quoteFormView is the data for quote-form-compact / quote-form-full.
type quoteFormView struct {
	Variant        string
	Compact        bool
	Dark           bool
	Values         quoteInput
	Errors         map[string]string
	FormError      string
	ServiceOptions []content.Option
	Site           *content.Site
}

// successView is the data for partials/form-success.
type successView struct {
	Variant      string
	Dark         bool
	FirstName    string
	Reference    string
	ResponseTime string
	Phone        string
}

func (app *App) newQuoteForm(variant string, in quoteInput, errs map[string]string, formError string) quoteFormView {
	return quoteFormView{
		Variant:        variant,
		Compact:        variant == variantCompact,
		Dark:           variant == variantFull,
		Values:         in,
		Errors:         errs,
		FormError:      formError,
		ServiceOptions: app.site.ServiceOptions,
		Site:           app.site,
	}
}

// QuoteHandler accepts quote submissions from both page forms.
//
// Lifecycle: CSRF origin check → parse → honeypot → validate → rate limit →
// persist (the lead is the source of truth) → notify sales by email → respond.
// An email failure never fails the request: the lead is already saved and the
// failure is recorded in quote_email_events for follow-up.
func (app *App) QuoteHandler(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxQuoteFormBytes)
	parseErr := r.ParseForm()
	in := parseQuoteInput(r)
	form := app.newQuoteForm(in.Variant, in, nil, "")
	phone := app.site.Brand.Phone

	if !app.sameOrigin(r) {
		app.logger.Warn("quote: rejected cross-origin submission", "origin", r.Header.Get("Origin"))
		form.FormError = "We couldn't verify this submission. Please reload the page and try again."
		app.renderQuoteForm(w, r, http.StatusForbidden, form)
		return
	}

	if parseErr != nil {
		status := http.StatusBadRequest
		var tooLarge *http.MaxBytesError
		if errors.As(parseErr, &tooLarge) {
			status = http.StatusRequestEntityTooLarge
		}
		form.FormError = "We couldn't read your submission. Please shorten any long fields and try again."
		app.renderQuoteForm(w, r, status, form)
		return
	}

	if in.Website != "" {
		// Honeypot tripped: respond as if successful so bots learn nothing.
		app.logger.Info("quote: honeypot triggered, submission discarded")
		app.renderSuccess(w, r, successView{Variant: in.Variant, Dark: form.Dark, ResponseTime: app.site.Brand.ResponseTime, Phone: phone})
		return
	}

	quote, errs := app.validateQuote(in)
	if len(errs) > 0 {
		form.Errors = errs
		app.renderQuoteForm(w, r, http.StatusUnprocessableEntity, form)
		return
	}

	// Detach from client cancellation: once a valid lead arrives it should be
	// saved and announced even if the visitor navigates away mid-request.
	ctx := context.WithoutCancel(r.Context())
	quote.IPHash = hashIP(app.cfg.IPHashSalt, app.clientIP(r))
	quote.UserAgent = truncateBytes(r.UserAgent(), maxUserAgentBytes)
	quote.Source = "website:" + in.Variant

	if limit := app.cfg.QuoteRateLimitPerHour; limit > 0 {
		countCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
		n, err := app.db.CountQuoteRequestsSince(countCtx, quote.IPHash, app.now().Add(-time.Hour))
		cancel()
		switch {
		case err != nil:
			// Fail open: losing a real lead costs more than an extra spam row.
			app.logger.Warn("quote: rate limit check failed, allowing submission", "error", err)
		case n >= limit:
			app.logger.Warn("quote: rate limit exceeded", "count", n, "limit", limit)
			w.Header().Set("Retry-After", "3600")
			form.FormError = fmt.Sprintf("We've already received several requests from your network. Please call us at %s and we'll help right away.", phone)
			app.renderQuoteForm(w, r, http.StatusTooManyRequests, form)
			return
		}
	}

	id, err := app.saveQuote(ctx, &quote)
	if err != nil {
		app.logger.Error("quote: failed to save quote request", "error", err)
		form.FormError = fmt.Sprintf("Sorry, we couldn't submit your request right now. Please try again in a moment or call us at %s.", phone)
		app.renderQuoteForm(w, r, http.StatusServiceUnavailable, form)
		return
	}

	// Send synchronously: serverless platforms freeze the instance as soon as
	// the response is written, so background goroutines are not reliable.
	app.notifySales(ctx, id, quote)

	app.renderSuccess(w, r, successView{
		Variant:      in.Variant,
		Dark:         form.Dark,
		FirstName:    quote.FirstName,
		Reference:    quote.Reference,
		ResponseTime: app.site.Brand.ResponseTime,
		Phone:        phone,
	})
}

// saveQuote inserts q, regenerating the public reference on the (rare)
// collision. q.Reference is set to the stored value.
func (app *App) saveQuote(ctx context.Context, q *db.QuoteRequest) (string, error) {
	const attempts = 3
	var lastErr error
	for i := 0; i < attempts; i++ {
		q.Reference = db.NewReference(app.site.Brand.ReferencePrefix, app.now())
		insertCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		id, err := app.db.CreateQuoteRequest(insertCtx, *q)
		cancel()
		if err == nil {
			return id, nil
		}
		lastErr = err
		if !errors.Is(err, db.ErrDuplicateReference) {
			break
		}
	}
	return "", lastErr
}

func (app *App) notifySales(ctx context.Context, quoteID string, q db.QuoteRequest) {
	sendCtx, cancel := context.WithTimeout(ctx, app.cfg.EmailSendTimeout)
	defer cancel()

	var weight float64
	if q.WeightKg != nil {
		weight = *q.WeightKg
	}
	result := app.emailProvider.SendQuoteNotification(sendCtx, email.QuoteNotification{
		Reference:    q.Reference,
		FirstName:    q.FirstName,
		LastName:     q.LastName,
		Company:      q.CompanyName,
		Email:        q.Email,
		Phone:        q.Phone,
		Origin:       q.Origin,
		Destination:  q.Destination,
		ServiceType:  app.site.ServiceLabel(q.ServiceType),
		WeightKg:     weight,
		CargoDetails: q.CargoDetails,
		SubmittedAt:  app.now(),
	})

	event := db.EmailEvent{
		QuoteRequestID:    quoteID,
		Provider:          result.Provider,
		ProviderMessageID: result.MessageID,
		EventType:         "sent",
	}
	if result.Err != nil {
		event.EventType = "failed"
		event.ErrorMessage = result.Err.Error()
		app.logger.Error("quote: notification email failed (lead is saved)",
			"provider", result.Provider, "reference", q.Reference, "error", result.Err)
	} else {
		app.logger.Info("quote: notification email sent",
			"provider", result.Provider, "reference", q.Reference, "message_id", result.MessageID)
	}

	eventCtx, cancelEvent := context.WithTimeout(ctx, 3*time.Second)
	defer cancelEvent()
	if err := app.db.RecordEmailEvent(eventCtx, event); err != nil {
		app.logger.Warn("quote: failed to record email event", "reference", q.Reference, "error", err)
	}
}

func (app *App) renderQuoteForm(w http.ResponseWriter, r *http.Request, status int, form quoteFormView) {
	app.renderFragment(w, r, status, "components/quote-form-"+form.Variant, "form-"+form.Variant, form)
}

func (app *App) renderSuccess(w http.ResponseWriter, r *http.Request, view successView) {
	app.renderFragment(w, r, http.StatusOK, "partials/form-success", "success", view)
}

func parseQuoteInput(r *http.Request) quoteInput {
	get := func(key string) string { return strings.TrimSpace(r.PostFormValue(key)) }
	in := quoteInput{
		Variant:      get("variant"),
		FullName:     get("full_name"),
		FirstName:    get("first_name"),
		LastName:     get("last_name"),
		Company:      get("company_name"),
		Email:        get("email"),
		Phone:        get("phone"),
		Origin:       get("origin"),
		Destination:  get("destination"),
		ServiceType:  get("service_type"),
		WeightKg:     get("weight_kg"),
		CargoDetails: get("cargo_details"),
		Website:      get("website"),
	}
	if in.Variant != variantFull {
		in.Variant = variantCompact
	}
	return in
}

// validateQuote checks a submission and returns a normalised record or
// field-keyed error messages.
func (app *App) validateQuote(in quoteInput) (db.QuoteRequest, map[string]string) {
	errs := map[string]string{}
	q := db.QuoteRequest{}

	text := func(field, value, label string, required bool, max int) string {
		value = collapseSpace(value)
		switch {
		case required && value == "":
			errs[field] = "Please enter " + label + "."
		case utf8.RuneCountInString(value) > max:
			errs[field] = fmt.Sprintf("Please keep this under %d characters.", max)
		}
		return value
	}

	if in.Variant == variantCompact {
		name := text("full_name", in.FullName, "your name", true, 120)
		q.FirstName, q.LastName, _ = strings.Cut(name, " ")
	} else {
		q.FirstName = text("first_name", in.FirstName, "your first name", true, 80)
		q.LastName = text("last_name", in.LastName, "your last name", false, 80)
		q.CompanyName = text("company_name", in.Company, "your company name", false, 120)
		if details := strings.TrimSpace(in.CargoDetails); utf8.RuneCountInString(details) > 2000 {
			errs["cargo_details"] = "Please keep cargo details under 2,000 characters."
		} else {
			q.CargoDetails = details
		}
	}

	q.Origin = text("origin", in.Origin, "an origin city", true, 120)
	q.Destination = text("destination", in.Destination, "a destination city", true, 120)

	switch addr, err := mail.ParseAddress(in.Email); {
	case in.Email == "":
		errs["email"] = "Please enter your email address."
	case err != nil || addr.Address != in.Email || len(in.Email) > 254 || !strings.Contains(in.Email[strings.LastIndexByte(in.Email, '@')+1:], "."):
		errs["email"] = "Please enter a valid email address, like name@company.com."
	default:
		q.Email = in.Email
	}

	if normalized, err := phone.Normalize(in.Phone); err != nil {
		errs["phone"] = err.Error()
	} else {
		q.Phone = normalized
	}

	if !validService(app.site.ServiceOptions, in.ServiceType) {
		errs["service_type"] = "Please choose a service type."
	} else {
		q.ServiceType = in.ServiceType
	}

	if in.WeightKg != "" {
		w, err := strconv.ParseFloat(strings.ReplaceAll(in.WeightKg, ",", ""), 64)
		if err != nil || w <= 0 || w > 1_000_000 {
			errs["weight_kg"] = "Please enter a weight in kilograms between 1 and 1,000,000."
		} else {
			q.WeightKg = &w
		}
	}

	return q, errs
}

func validService(options []content.Option, value string) bool {
	for _, o := range options {
		if o.Value == value {
			return true
		}
	}
	return false
}

func collapseSpace(s string) string { return strings.Join(strings.Fields(s), " ") }

func truncateBytes(s string, max int) string {
	if len(s) <= max {
		return s
	}
	s = s[:max]
	for !utf8.ValidString(s) {
		s = s[:len(s)-1]
	}
	return s
}
