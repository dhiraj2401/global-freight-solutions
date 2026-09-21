# Global Freight Solutions: Website

A single-page, conversion-focused logistics website built with **Go + htmx 4 + Tailwind CSS v4**, backed by **Neon Postgres**, with a **pluggable email-provider layer** (Brevo REST API, AWS SES, easy to extend).

> **Content status:** every metric, contact detail, client name, testimonial and port claim is a **placeholder** adapted from the design reference. Replace them in [`internal/content/site.go`](internal/content/site.go) and set `HasPlaceholders: false` before launch. Photos in `static/img/` come from the design reference PDF. Confirm you have the rights to use them, or replace them (see [Content & imagery](#9-content--imagery)).

---

## Contents

1. [Architecture overview](#1-architecture-overview)
2. [Local setup](#2-local-setup)
3. [Email provider configuration](#3-email-provider-configuration)
4. [Cost analysis](#4-cost-analysis)
5. [Security](#5-security)
6. [Deployment](#6-deployment)
7. [Operations](#7-operations)
8. [Future roadmap](#8-future-roadmap)
9. [Content & imagery](#9-content--imagery)

---

## 1. Architecture overview

| Layer | Choice | Notes |
|---|---|---|
| Rendering | Go `html/template`, server-side | Contextual auto-escaping; no client framework |
| Interactivity | htmx **4.0.0** (vendored in `static/js/htmx.min.js`) | fetch-based, explicit `:inherited` inheritance, `htmx:phase:action` events |
| Styling | Tailwind CSS **v4** standalone CLI | CSS-first config in `static/css/tailwind.css`; compiled `app.css` is committed |
| Persistence | Neon Postgres via `pgx/v5` `pgxpool` | Tuned for serverless + PgBouncer |
| Email | `internal/email` provider registry | `EMAIL_PROVIDER=brevo \| ses \| log` |
| Hosting | Vercel Go runtime (`api/index.go`) or container | Templates, assets and migrations are embedded in the binary |

### How a quote request flows

```
Browser (htmx 4)                    Go handler (POST /api/quote)                  External
────────────────                    ───────────────────────────                   ────────
<form hx-post="/api/quote"   ──►  1. CSRF: Origin must match host/ALLOWED_ORIGIN
      hx-target="#quote-…"         2. Parse (32 KB cap) → honeypot check
      hx-swap="outerHTML">         3. Validate  ──► 422 + form with inline errors
                                   4. Rate limit (HMAC-hashed IP, per hour) ──► 429
                                   5. INSERT quote_requests  ─────────────────────► Neon
                                   6. EmailProvider.SendQuoteNotification ────────► Brevo / SES
                                      (From: verified sender, Reply-To: prospect)      │
                                   7. INSERT quote_email_events (sent | failed) ──► Neon
◄── 200 success fragment ───────   8. Render partials/form-success                    ▼
    (reference number)                                              IONOS mailbox → Outlook
```

* **The lead is the source of truth.** It is saved before any email is attempted, and an email failure never fails the request. Failures are logged and recorded in `quote_email_events`.
* **Email is sent synchronously.** Serverless platforms freeze the instance once the response is written, so fire-and-forget goroutines are unreliable. The send uses a context detached from client disconnects, with `EMAIL_SEND_TIMEOUT_SECONDS` (default 8 s).
* **Progressive enhancement.** Without JavaScript the same forms submit normally and receive a full page (`templates/result.html`).
* **htmx 4 behaviour relied on:** all status codes swap except 204/304, so a 422/429/403/503 response returns the form re-rendered with messages. Response headers such as `HX-Redirect` are honoured. No localStorage history cache.

### Email provider plug-in architecture

```
internal/email/
├── provider.go   EmailProvider interface, QuoteNotification, SendResult, Options, Factory
├── factory.go    Register(name, factory) · NewEmailProvider(ctx, name, opts) · Providers()
├── render.go     Provider-agnostic message rendering (subject, text, HTML, RFC 5322 MIME)
├── brevo.go      init(): Register("brevo", …): Brevo transactional email REST API (HTTPS)
├── ses.go        init(): Register("ses", …): SES v2 SendEmail API
└── log.go        init(): Register("log", …): renders and logs, never sends (dev default)
```

Each provider registers itself in `init()` and reads **its own** environment variables through `Options.Lookup`. The application only knows the interface, so switching providers is a one-variable change and adding one touches no existing code.

### Repository layout

```
api/index.go                  Vercel serverless entrypoint (imports only the root package)
cmd/server/main.go            Local / container HTTP server with graceful shutdown
cmd/migrate/main.go           Applies db/migrations (tracked in schema_migrations)
gfs.go                        go:embed assets; NewHandler; ServeServerless
internal/config/              Env loading + validation; .env loader for local dev
internal/content/site.go      ALL site copy, contact details, metrics, images (placeholders marked)
internal/db/                  Store interface, Postgres (pgxpool) + in-memory implementations, migrator
internal/email/               Provider interface, registry, Brevo, SES, log
internal/handlers/            home.go, quote.go, track.go, shared rendering/CSRF/IP helpers
internal/server/              Router, security headers (CSP), logging, recovery, static caching
internal/templates/           Template renderer, helper funcs, inline SVG icon set
templates/                    base.html, index.html, result.html, components/, partials/
static/                       css/tailwind.css (input) → css/app.css (built), js/, img/
db/migrations/                001_initial_schema.sql
```

---

## 2. Local setup

### Prerequisites

* **Go 1.25+** (`pgx/v5` v5.11 requires it): <https://go.dev/dl/>
* **Tailwind CSS v4 standalone CLI**: only needed when you change classes. `make tailwind` downloads it to `./bin` (no Node.js).
* **Bun** *(optional)*: not required by the project. Use it only if you prefer running scripts with it, e.g. `bunx concurrently "make css-watch" "make dev"`.
* **Neon** account *(optional locally)*: without `DATABASE_URL` the app uses an in-memory store.

### Run it

```bash
cp .env.example .env        # optional; defaults work out of the box
make dev                    # or: go run ./cmd/server
# → http://localhost:8080
```

With no configuration, development mode uses the **in-memory store** and the **`log` email provider**. Quote notifications are rendered and printed to the terminal, not sent.

### Styles

```bash
make tailwind               # one-time: download ./bin/tailwindcss (v4.3.3)
make css-watch              # rebuild static/css/app.css while editing templates
make css                    # minified production build: commit static/css/app.css
```

Tailwind v4 is configured **in CSS** (`@theme`, `@source`, `@utility` in `static/css/tailwind.css`), so there is no `tailwind.config.js`. Theme colours are CSS variables, so light and dark mode need no `dark:` duplication.

### Database

```bash
# In .env: DATABASE_URL (pooled) and DATABASE_URL_UNPOOLED (direct) from the Neon console
make migrate                # or: go run ./cmd/migrate
```

The migrator takes a Postgres advisory lock, applies each pending file in its own transaction and records it in `schema_migrations`. It prefers `DATABASE_URL_UNPOOLED` because session locks don't survive PgBouncer transaction pooling.

### Tests

```bash
make test                   # go test ./...
```

Covers validation, CSRF rejection, honeypot, rate limiting, email-failure resilience, no-JS fallback, provider registry/switching, Brevo API request construction, error handling, rate-limit retry and deadlines, SES request construction, header-injection and HTML-escaping in emails, config validation, and the fully wired router with embedded assets.

---

## 3. Email provider configuration

All providers share:

| Variable | Purpose |
|---|---|
| `EMAIL_PROVIDER` | `brevo`, `ses` or `log` |
| `SALES_NOTIFICATION_EMAIL` | Recipient inbox (e.g. an IONOS mailbox that forwards to Outlook) |
| `EMAIL_FROM_NAME` | Fallback From display name, used only if the prospect's email can't be parsed |
| `EMAIL_SEND_TIMEOUT_SECONDS` | Upper bound on a send (default 8) |

**Addressing rules (all providers):**

```
From:     Jane Doe (jane@acme.com) <website-notifications@yourdomain.com>   ← BREVO_FROM_EMAIL / SES_FROM_EMAIL
Reply-To: Jane Doe <jane@acme.com>                                          ← email entered in the form
To:       quote@yourdomain.com                                               ← SALES_NOTIFICATION_EMAIL
```

* The inbox shows the **prospect as the sender** (name and email in the From display name), and *Reply* in Outlook goes straight to them.
* The From **address** stays your verified sender. Brevo and SES only send from verified senders, and using the prospect's own domain would fail DMARC at the receiving server (spam or rejection).
* Header values are stripped of CR/LF to prevent header injection.

### Brevo (REST API)

The provider calls Brevo's transactional email endpoint, `POST https://api.brevo.com/v3/smtp/email`, instead of speaking SMTP. On Vercel this is the better fit:

* **One HTTPS request** instead of a multi-step SMTP conversation (connect, EHLO, STARTTLS, AUTH, DATA), so less latency inside a short function invocation.
* **No outbound port 587**: some serverless and corporate networks restrict SMTP egress; port 443 is always open.
* **Connection reuse**: a shared HTTP client keeps TLS connections alive across warm invocations.
* **Structured results**: Brevo returns a `messageId` (stored in `quote_email_events.provider_message_id`) and JSON errors with a code and message.

Setup:

1. Create an account at <https://www.brevo.com>.
2. **Senders, Domains & Dedicated IPs → Domains:** add your domain and publish the DNS records Brevo gives you (DKIM, the Brevo verification code and DMARC). Wait until it shows *Authenticated*.
3. **Settings → SMTP & API → API Keys:** generate an API key (starts with `xkeysib-`). This is different from the SMTP key.
4. **Check authorised IPs:** if IP blocking is enabled under **Security → Authorised IPs**, requests from Vercel's changing IP addresses are rejected with 401. Disable IP blocking for this key's use, or deploy with fixed egress IPs.
5. Configure:

```bash
EMAIL_PROVIDER=brevo
BREVO_API_KEY=xkeysib-…
BREVO_FROM_EMAIL=website-notifications@yourdomain.com
```

Request details:

* **Auth:** `api-key` header over HTTPS. The key never appears in logs or error messages.
* **Payload:** `sender`, `to`, `replyTo` (the prospect), `subject`, `htmlContent`, `textContent`, an `X-Quote-Reference` header and a `quote-notification` tag for filtering in Brevo's logs.
* **Retries:** only on `429` and `503`, once, honouring `Retry-After`. Those responses mean the email was not accepted, so a retry cannot create a duplicate. Brevo has no idempotency keys, so other failures are not retried. A retry is skipped if it can't finish within `EMAIL_SEND_TIMEOUT_SECONDS`.
* **Errors** are recorded as, for example, `brevo api: status 401 unauthorized: Key not found`.

Migrating from the SMTP version: remove `BREVO_SMTP_HOST`, `BREVO_SMTP_PORT`, `BREVO_SMTP_USER` and `BREVO_SMTP_KEY`, and add `BREVO_API_KEY`. If only the old SMTP key is set, startup fails with a message saying so.

### AWS SES (v2 API)

1. **SES console → Identities → Create identity → Domain.** Enable Easy DKIM and publish the CNAMEs. Optionally configure a custom MAIL FROM domain for SPF alignment.
2. **Request production access** (Account dashboard). In the sandbox SES only delivers to verified addresses.
3. **IAM:** create a user or role limited to sending from your identity:

   ```json
   {
     "Version": "2012-10-17",
     "Statement": [{
       "Effect": "Allow",
       "Action": "ses:SendEmail",
       "Resource": [
         "arn:aws:ses:us-east-1:<ACCOUNT_ID>:identity/yourdomain.com",
         "arn:aws:ses:us-east-1:<ACCOUNT_ID>:configuration-set/*"
       ]
     }]
   }
   ```

4. Create access keys for that user and configure:

```bash
EMAIL_PROVIDER=ses
SES_AWS_REGION=us-east-1
SES_AWS_ACCESS_KEY_ID=AKIA…
SES_AWS_SECRET_ACCESS_KEY=…
SES_FROM_EMAIL=website-notifications@yourdomain.com
# SES_CONFIGURATION_SET=website-notifications   # optional, for delivery/bounce events
```

> **Why `SES_AWS_*`?** Vercel reserves `AWS_REGION`, `AWS_ACCESS_KEY_ID` and `AWS_SECRET_ACCESS_KEY`. The provider reads the `SES_`-prefixed names first and falls back to the standard AWS credential chain (env vars, shared config, IAM role) everywhere else.

### Switching providers

```bash
EMAIL_PROVIDER=ses     # was: brevo
```

Redeploy (Vercel applies env changes on the next deployment) or restart the server. **No code changes.** Startup fails fast with a clear message if the selected provider's variables are missing.

### Adding a provider (e.g. Resend)

Create one file; nothing else changes:

```go
// internal/email/resend.go
package email

func init() { Register("resend", newResendFromEnv) }

type ResendProvider struct{ APIKey, FromEmail, FromName, To string }

func newResendFromEnv(_ context.Context, opts Options) (EmailProvider, error) {
	if err := requireEnv(opts.Lookup, "RESEND_API_KEY", "RESEND_FROM_EMAIL"); err != nil {
		return nil, fmt.Errorf("resend: %w", err)
	}
	return &ResendProvider{APIKey: opts.Lookup("RESEND_API_KEY"), FromEmail: opts.Lookup("RESEND_FROM_EMAIL"),
		FromName: opts.FromName, To: opts.To}, nil
}

func (p *ResendProvider) Name() string { return "resend" }

func (p *ResendProvider) SendQuoteNotification(ctx context.Context, n QuoteNotification) SendResult {
	msg, err := BuildMessage(Envelope{From: mail.Address{Name: p.FromName, Address: p.FromEmail}, To: p.To}, n)
	if err != nil {
		return SendResult{Provider: p.Name(), Err: err}
	}
	// POST msg.Subject / msg.HTML / msg.Text / msg.ReplyTo to the provider's API with ctx …
	return SendResult{Provider: p.Name(), MessageID: "…"}
}
```

Then set `EMAIL_PROVIDER=resend`.

---

## 4. Cost analysis

Assumed volume: **500 notifications/day ≈ 15,000/month**.

| | Brevo | AWS SES |
|---|---|---|
| Free tier | 300 emails/day (~9,000/month): **insufficient** at 500/day | Limited-time free tier for new accounts (check current terms) |
| Paid plan needed | **Starter: 20,000 emails/month ≈ $18/month** | Pay-as-you-go **$0.10 per 1,000** emails |
| Monthly cost at 15K | **~$18** | **~$1.50** (+ negligible data transfer) |
| Setup effort | Low: API key + domain auth | Medium: domain verification, production-access request, IAM |

**SES is roughly 12× cheaper** at this volume.

> Pricing is as supplied with the project brief. Both vendors change plans and regional pricing, so confirm on brevo.com/pricing and aws.amazon.com/ses/pricing before budgeting.

**Recommendation**

* **Development & initial launch: Brevo.** Fastest path to authenticated delivery, with no sandbox or production-access wait.
* **Production at scale: SES.** Once domain verification and production access are approved, set `EMAIL_PROVIDER=ses`. No code changes.
* **Sanity-check the volume assumption.** 15,000 notifications/month means 500 new quote requests per day. If real lead volume is well below 300/day, Brevo's free tier covers it and the cost difference is moot.

---

## 5. Security

* **Secrets:** environment variables only. `.env` is git-ignored; `.env.example` holds no secrets. Database URLs are never logged or echoed in errors.
* **SQL:** every query is parameterised (`$1…$n`) through pgx.
* **HTML escaping:** `html/template` contextual escaping for pages and the HTML email; user input is never marked safe. URLs from content pass an allow-list (`href`, `tel` template funcs).
* **Email header injection:** CR/LF and control characters stripped from every header value; addresses parsed with `net/mail`.
* **CSRF:** form POSTs require an `Origin` matching the request host or `ALLOWED_ORIGIN`. Requests without `Origin` are rejected when `Sec-Fetch-Site` reports cross-site. No cookies or sessions are used.
* **CORS:** none. The endpoints are same-origin HTML form handlers, not a cross-origin API, so no `Access-Control-Allow-*` headers are sent.
* **Privacy:** client IPs are stored only as `HMAC-SHA256(IP_HASH_SALT, ip)`. Rotating the salt unlinks old hashes. Request logs omit form values, query strings and IPs.
* **Abuse controls:** honeypot field, per-IP-hash hourly limit (`QUOTE_RATE_LIMIT_PER_HOUR`), 32 KB body cap, strict server-side validation.
* **Phone numbers** (`internal/phone`): both forms use a browser `pattern` pre-check. The server is authoritative: US/Canada numbers must have valid area codes and exchanges; other numbers need `+` and a country code (8–15 digits); extensions are allowed. Numbers are stored normalized, e.g. `+17135550198 ext. 42`.
* **Headers:** strict CSP (`script-src 'self'`, no inline script or style; htmx's injected indicator CSS is disabled and shipped in `app.css`), `X-Frame-Options: DENY`, `nosniff`, `Referrer-Policy`, `Permissions-Policy`, HSTS in production.
* **Proxy headers:** `X-Real-IP` / `X-Forwarded-For` are trusted only on Vercel (which overwrites them) or when `TRUST_PROXY_HEADERS=true`.

---

## 6. Deployment

### Vercel (Go runtime)

Vercel's Go runtime runs `api/*.go` files that export `func Handler(http.ResponseWriter, *http.Request)`. It does not run `cmd/server/main.go`. This repo therefore ships:

* `api/index.go`: a thin `Handler` that lazily builds the app once per instance (pool reused across warm invocations).
* `vercel.json`: rewrites every path to `/api/index`; routing happens in Go's `ServeMux`.
* `go:embed`: templates, CSS, JS and images are compiled into the function, so there is no file-system dependency.

Steps:

1. Import the repository in Vercel (framework preset: **Other**).
2. **Storage → Neon** integration (or paste connection strings). It provides `DATABASE_URL` (pooled) and `DATABASE_URL_UNPOOLED`.
3. **Settings → Environment Variables** (Production and Preview): `EMAIL_PROVIDER`, provider variables, `SALES_NOTIFICATION_EMAIL`, `IP_HASH_SALT`, `ALLOWED_ORIGIN`. `APP_ENV` defaults to `production` on Vercel.
4. Run migrations from your machine or CI: `DATABASE_URL_UNPOOLED=… go run ./cmd/migrate`.
5. Deploy. Check `https://<domain>/healthz?db=1`.

`vercel.json` sets `maxDuration: 15` to leave headroom for DB + email within one invocation.

### Container (Fly.io, Render, Cloud Run, ECS)

```bash
docker build -t gfs-website .
docker run --env-file .env -p 8080:8080 gfs-website
docker run --env-file .env --entrypoint /migrate gfs-website   # migrations
```

Distroless, non-root, static binary; graceful shutdown on SIGTERM.

### Database connection pooling

`internal/db/postgres.go` configures `pgxpool` for serverless:

* `MaxConns` small (`DB_MAX_CONNS`, default 4), `MinConns=0`, 30 s idle timeout, so instances don't hoard connections.
* `QueryExecModeExec`: no named server-side prepared statements, which is compatible with Neon's PgBouncer (`-pooler`) transaction pooling.
* Lazy connect: cold starts don't pay for a DB round trip until a query runs.

---

## 7. Operations

```sql
-- Newest leads
SELECT public_reference, first_name, company_name, email, phone, origin, destination, service_type, created_at
FROM quote_requests ORDER BY created_at DESC LIMIT 50;

-- Leads whose notification email failed (follow up manually)
SELECT q.public_reference, q.email, e.provider, e.error_message, e.created_at
FROM quote_email_events e JOIN quote_requests q ON q.id = e.quote_request_id
WHERE e.event_type = 'failed' ORDER BY e.created_at DESC;

-- Pipeline status update
UPDATE quote_requests SET status = 'contacted', contacted_at = now() WHERE public_reference = 'GFS-260914-XXXXXX';
```

Logs are structured JSON in production. Search for `quote: notification email failed` to alert on delivery problems.

---

## 8. Future roadmap

* **More providers:** Resend, SendGrid, Postmark (one file each, see above).
* **Automatic failover:** a composite provider that tries a secondary when the primary errors.
* **Delivery webhooks:** SES event destinations and Brevo webhooks → `quote_email_events` (`delivered`, `bounced`, `complaint`).
* **Retry queue:** re-send `failed` events via a cron job (Vercel Cron → `/api/cron/retry-emails`) with backoff.
* **Sales dashboard:** authenticated htmx views over `quote_requests` with status transitions and assignment.
* **Integrations:** TMS/visibility API for real shipment tracking (`TRACKING_URL` is the interim hook), CRM sync (HubSpot/Salesforce), carrier onboarding portal.
* **Spam hardening:** Cloudflare Turnstile if the honeypot and rate limit prove insufficient.

---

## 9. Content & imagery

* **All copy lives in [`internal/content/site.go`](internal/content/site.go).** Blocks marked `PLACEHOLDER` contain unverified claims (metrics, fleet size, carrier counts, ports, testimonials, client names, 555 phone number, addresses, legal links). A small banner appears in development while `HasPlaceholders` is `true`, and production logs a warning.
* **Client logos:** the "trusted by" strip uses fictional wordmarks. Show real client logos only with written permission.
* **Testimonials:** illustrative only. Replace them with real, attributable quotes collected with consent.
* **Images:** `static/img/*.jpg` were extracted from the supplied design reference as placeholders. Confirm licensing or replace them with owned or licensed photography at the same paths. Any image path that doesn't exist falls back to a branded gradient panel automatically. The account-manager slot is intentionally empty because the reference photo carries third-party branding.
* **Tracking:** "Track Shipment" opens a dialog. Without `TRACKING_URL` it explains that status updates come from the account manager; with it, it redirects to your tracking portal.
