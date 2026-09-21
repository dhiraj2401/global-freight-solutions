// Package content is the single source of truth for all copy, contact
// details, metrics and imagery rendered on the site.
//
// ─────────────────────────────────────────────────────────────────────────────
// PLACEHOLDER NOTICE
// Every business claim in this file (metrics, fleet size, carrier counts,
// phone numbers, addresses, client names, testimonials, port coverage) was
// adapted from a design reference and is NOT verified. Replace each block
// marked PLACEHOLDER with approved, substantiated content before launch, then
// set HasPlaceholders to false to remove the development banner.
// ─────────────────────────────────────────────────────────────────────────────
package content

import "time"

// Site aggregates every section of the landing page.
type Site struct {
	HasPlaceholders bool

	Brand          Brand
	SEO            SEO
	Nav            []Link
	Hero           Hero
	ServiceTicker  []IconText
	Advantage      Advantage
	Metrics        Metrics
	AccountManager AccountManager
	WhyUs          WhyUs
	Process        Process
	Services       Services
	Warehousing    Warehousing
	Industries     Industries
	Ports          Ports
	TrustedBy      TrustedBy
	CarrierPartner CarrierPartner
	Testimonials   Testimonials
	Contact        Contact
	Track          Track
	Footer         Footer

	// ServiceOptions populates the quote form select. Values are stored in
	// quote_requests.service_type and validated server-side.
	ServiceOptions []Option
}

type Link struct {
	Label string
	Href  string
}

type Image struct {
	Src string // "/static/img/..." — cleared at startup if the file is missing
	Alt string
}

type IconText struct {
	Icon  string
	Title string
	Text  string
}

type Stat struct {
	Icon  string
	Value string
	Label string
}

type Option struct {
	Value string
	Label string
}

type Brand struct {
	Name            string
	ShortName       string
	Descriptor      string
	ReferencePrefix string
	ResponseTime    string
	Phone           string
	PhoneHref       string
	Email           string
	HeadOffice      string
	Address         []string
	Hours           string
}

type SEO struct {
	Title       string
	Description string
}

type Hero struct {
	Badges       []string
	Eyebrow      string
	Title        string
	Subtitle     string
	PrimaryCTA   Link
	SecondaryCTA Link
	Stats        []Stat
	Image        Image
	FormTitle    string
	FormSubtitle string
	FormFootnote string
}

type NumberedPoint struct {
	Number string
	Title  string
	Text   string
}

type Advantage struct {
	Eyebrow     string
	Title       string
	TitleAccent string
	Body        string
	CTA         Link
	Points      []NumberedPoint
}

type Metrics struct {
	Eyebrow    string
	Title      string
	Stats      []Stat
	Highlights []IconText
}

type AccountManager struct {
	Eyebrow        string
	TitleLines     []string
	Body           string
	Features       []IconText
	ChecklistTitle string
	Checklist      []string
	CalloutTitle   string
	CalloutText    string
	Image          Image
}

type WhyUs struct {
	Eyebrow string
	Title   string
	Items   []IconText
}

type Process struct {
	Eyebrow string
	Title   string
	Steps   []IconText
}

type ServiceCard struct {
	Icon  string
	Title string
	Text  string
	Image Image
	Href  string
}

type Services struct {
	Eyebrow string
	Title   string
	CTA     Link
	Items   []ServiceCard
}

type Checklist struct {
	Title string
	Items []string
}

type Warehousing struct {
	Eyebrow string
	Title   string
	Body    string
	CTA     Link
	Image   Image
	Cards   []Checklist
}

type Industries struct {
	Eyebrow string
	Title   string
	Items   []IconText
}

type Ports struct {
	Eyebrow   string
	Title     string
	Body      string
	CTA       Link
	Image     Image
	Ports     []IconText
	MoreLabel string
}

type Logo struct {
	Name  string
	Style string // "serif" | "wide" | "mono" | "rounded" — typographic treatment
}

type TrustedBy struct {
	Eyebrow string
	Logos   []Logo
}

type CarrierPartner struct {
	Eyebrow   string
	Title     string
	Body      string
	Benefits  []string
	Image     Image
	CardTitle string
	Steps     []IconText
	CTA       Link
}

type Testimonial struct {
	Quote    string
	Name     string
	Role     string
	Company  string
	Initials string
	Rating   int
}

type Testimonials struct {
	Eyebrow string
	Title   string
	Items   []Testimonial
}

type ContactDetail struct {
	Icon  string
	Label string
	Value string
	Href  string
}

type Contact struct {
	Badge        string
	Title        string
	Body         string
	Details      []ContactDetail
	Image        Image
	FormTitle    string
	FormFootnote string
}

type Track struct {
	Title        string
	Body         string
	NotConnected string
	InputLabel   string
	InputHint    string
	SubmitLabel  string
}

type SocialLink struct {
	Icon  string
	Label string
	Href  string
}

type FooterColumn struct {
	Title string
	Links []Link
}

type Footer struct {
	Blurb   string
	Columns []FooterColumn
	Social  []SocialLink
	Legal   []Link
}

// Stars returns a slice of length Rating for template iteration.
func (t Testimonial) Stars() []struct{} { return make([]struct{}, t.Rating) }

// CopyrightYear is the current year for the footer.
func (s *Site) CopyrightYear() int { return time.Now().Year() }

// Default returns the site content.
func Default() *Site {
	brand := Brand{
		// PLACEHOLDER: confirm legal entity name and all contact details.
		Name:            "Global Freight Solutions",
		ShortName:       "GFS",
		Descriptor:      "Logistics",
		ReferencePrefix: "GFS",
		ResponseTime:    "2 business hours",
		Phone:           "+1 (713) 555-0198", // PLACEHOLDER: 555 fictional number
		PhoneHref:       "tel:+17135550198",
		Email:           "quote@example.com", // PLACEHOLDER
		HeadOffice:      "Houston, Texas, USA",
		Address:         []string{"1200 Logistics Blvd, Suite 400", "Houston, TX 77002"}, // PLACEHOLDER
		Hours:           "Mon – Sat: 8:00 AM – 8:00 PM (CT)",                             // PLACEHOLDER
	}

	return &Site{
		HasPlaceholders: true,
		Brand:           brand,

		SEO: SEO{
			Title:       brand.Name + " | Asset-Backed Freight, Warehousing & Drayage",
			Description: "Road, air and sea freight, warehousing, transloading and port drayage with a dedicated account manager. Request a free quote.",
		},

		Nav: []Link{
			{"About", "#about"},
			{"Services", "#services"},
			{"Industries", "#industries"},
			{"Warehousing", "#warehousing"},
			{"Fleet", "#fleet"},
			{"Testimonials", "#testimonials"},
			{"Contact", "#contact"},
		},

		Hero: Hero{
			Badges:       []string{"Reliable Logistics Partner", "End-to-End Solutions"},
			Eyebrow:      "USA · Delivering Globally",
			Title:        "Moving Businesses Further.",
			Subtitle:     "Integrated logistics across road, rail, air and sea, plus warehousing, built for speed, reliability and growth.",
			PrimaryCTA:   Link{"Get A Free Quote", "#contact"},
			SecondaryCTA: Link{"Track Shipment", "#track"},
			// PLACEHOLDER: unverified figures from the design reference.
			Stats: []Stat{
				{"package", "13,000+", "Shipments monthly"},
				{"truck", "150+", "Fleet vehicles"},
				{"globe", "500+", "Business clients"},
			},
			Image:        Image{"/static/img/hero-port.jpg", "Container terminal with cranes and a truck at sunset"},
			FormTitle:    "Get Your Free Quote",
			FormSubtitle: "Quick, simple, reliable.",
			FormFootnote: "No spam. We reply within " + brand.ResponseTime + ".",
		},

		ServiceTicker: []IconText{
			{Icon: "truck", Title: "Road Freight"},
			{Icon: "plane", Title: "Air Freight"},
			{Icon: "ship", Title: "Sea Freight"},
			{Icon: "package", Title: "FTL"},
			{Icon: "container", Title: "LTL"},
			{Icon: "warehouse", Title: "Warehousing"},
			{Icon: "snowflake", Title: "Cold Chain"},
			{Icon: "anchor", Title: "Port Logistics"},
			{Icon: "route", Title: "Supply Chain"},
			{Icon: "zap", Title: "Express Delivery"},
		},

		Advantage: Advantage{
			Eyebrow:     "Our Advantage",
			Title:       "Not Just Logistics —",
			TitleAccent: "An Asset-Backed Partner.",
			Body:        "We own and operate assets, giving you greater control, reliability and better rates, so your freight keeps moving even when the market tightens.",
			CTA:         Link{"Explore Our Fleet & Assets", "#fleet"},
			Points: []NumberedPoint{
				{"01", "Control When It Matters Most", "With our own fleet and infrastructure, we keep your freight moving without intermediary delays."},
				{"02", "Better Pricing Through Assets", "More capacity, lower costs and consistent rates that give you an edge in any market condition."},
				{"03", "End-to-End Accountability", "One partner, complete visibility, and a dedicated team that owns the outcome from dock to door."},
			},
		},

		Metrics: Metrics{
			Eyebrow: "Our Performance",
			Title:   "Numbers That Speak for Themselves",
			// PLACEHOLDER: unverified performance claims. Substantiate before launch.
			Stats: []Stat{
				{"truck", "98%", "On-time deliveries"},
				{"clock", "<2hr", "Quote response"},
				{"network", "300+", "Carrier network"},
				{"headset", "24/7", "Live monitoring"},
			},
			Highlights: []IconText{
				{"star", "High Repeat Clients", "Our clients trust us, and they keep coming back."},
				{"map-pin", "Transparent Tracking", "Real-time updates at every step."},
				{"shield", "Rapid Issue Resolution", "Your dedicated manager solves problems fast."},
			},
		},

		AccountManager: AccountManager{
			Eyebrow:    "Dedicated Account Managers",
			TitleLines: []string{"One Manager.", "Every Shipment.", "Zero Confusion."},
			Body:       "A single point of contact who knows your business, your lanes and your priorities. No call centers, no runarounds.",
			Features: []IconText{
				{"phone", "Direct Phone Access", "Talk to a real person, anytime."},
				{"briefcase", "Know Your Business", "Your goals and preferences, always in mind."},
				{"activity", "Proactive Problem Solving", "We fix issues before they become problems."},
			},
			ChecklistTitle: "What Your Account Manager Does",
			Checklist: []string{
				"Quote coordination", "Carrier dispatch", "Live load tracking",
				"Rate negotiation", "Documentation support", "Delivery confirmation",
			},
			CalloutTitle: "We Answer Every Time.",
			CalloutText:  "Real people. Real support. Real results.",
			// Intentionally empty: the reference photo carries third-party
			// branding. Add licensed photography at this path to enable it.
			Image: Image{"/static/img/account-manager.jpg", "Account manager monitoring shipments across multiple screens"},
		},

		WhyUs: WhyUs{
			Eyebrow: "Why Businesses Choose " + brand.ShortName,
			Title:   "We Solve the Problems That Cost You Money.",
			Items: []IconText{
				{"truck", "Asset-Backed Reliability", "Our own fleet means freight keeps moving."},
				{"user-check", "Dedicated Account Manager", "One expert, every time."},
				{"clock", "Quotes in 2 Hours", "Fast, competitive pricing."},
				{"shield-check", "300+ Vetted Carriers", "A trusted network nationwide."}, // PLACEHOLDER figure
				{"layers", "Complete Supply Chain", "From pickup to delivery."},
				{"globe", "Domestic & International", "Coverage across the United States and beyond."},
			},
		},

		Process: Process{
			Eyebrow: "How It Works",
			Title:   "Your Shipment in 5 Simple Steps",
			Steps: []IconText{
				{"file-text", "Request Quote", "Share your shipment details."},
				{"user-check", "Manager Assigns Carrier", "We find the best match."},
				{"truck", "Pickup", "Your freight is collected."},
				{"map-pin", "Live Tracking", "Track in real time."},
				{"check", "Delivered", "On time. Every time."},
			},
		},

		Services: Services{
			Eyebrow: "Our Services",
			Title:   "Every Mode. Every Lane.",
			CTA:     Link{"Discuss Your Freight", "#contact"},
			Items: []ServiceCard{
				{"truck", "Road Freight", "FTL, LTL and cross-country transport.", Image{"/static/img/service-road.jpg", "Semi truck on a highway at sunset"}, "#contact"},
				{"plane", "Air Freight", "Fast and reliable global air cargo.", Image{"/static/img/service-air.jpg", "Cargo aircraft above the clouds"}, "#contact"},
				{"ship", "Sea Freight", "FCL, LCL and project shipping.", Image{"/static/img/service-sea.jpg", "Loaded container ship at sea"}, "#contact"},
				{"snowflake", "Refrigerated (Reefer)", "Temperature-controlled transport.", Image{"/static/img/service-reefer.jpg", "Refrigerated trailer at a loading dock"}, "#contact"},
				{"hard-hat", "Project & Heavy Haul", "For oversized and specialized cargo.", Image{"/static/img/service-heavy.jpg", "Heavy mining equipment ready for transport"}, "#contact"},
				{"warehouse", "Warehousing", "Short- and long-term storage solutions.", Image{"/static/img/service-warehouse.jpg", "Warehouse aisle with stocked racking"}, "#warehousing"},
			},
		},

		Warehousing: Warehousing{
			Eyebrow: "Port Drayage · Warehousing · Transloading",
			Title:   "Your Freight. Stored. Moved. Delivered.",
			Body:    "Modern facilities and last-mile transloading to keep your supply chain running smoothly.",
			CTA:     Link{"Explore Warehousing", "#contact"},
			Image:   Image{"/static/img/warehouse-interior.jpg", ""},
			Cards: []Checklist{
				{"Warehousing", []string{"Short- & long-term storage", "24/7 security and monitoring", "Inventory management", "Cross-dock support", "Flexible space options"}},
				{"Transloading", []string{"Direct port-to-rail transfers", "Reduced transit delays", "Lower drayage costs", "Secured storage yard", "Export blocking & bracing"}},
			},
		},

		Industries: Industries{
			Eyebrow: "Industries We Serve",
			Title:   "Built for Businesses That Can't Afford Delays.",
			Items: []IconText{
				{Icon: "factory", Title: "Manufacturing"},
				{Icon: "cart", Title: "Retail & E-Commerce"},
				{Icon: "utensils", Title: "Food & Beverage"},
				{Icon: "hard-hat", Title: "Construction"},
				{Icon: "pill", Title: "Pharmaceuticals"},
				{Icon: "droplet", Title: "Industrial & Oil/Gas"},
				{Icon: "ship", Title: "Import/Export"},
				{Icon: "leaf", Title: "Agriculture"},
			},
		},

		Ports: Ports{
			Eyebrow: "Port Services",
			Title:   "Port Drayage Across Major US Ports.",
			Body:    "Containers moving faster, with fewer delays. We handle drayage at major US ports.",
			CTA:     Link{"Get a Drayage Quote", "#contact"},
			Image:   Image{"/static/img/port-sunset.jpg", "Port cranes at sunrise over the harbor"},
			// PLACEHOLDER: confirm actual port coverage.
			Ports: []IconText{
				{Icon: "anchor", Title: "Los Angeles"},
				{Icon: "ship", Title: "Long Beach"},
				{Icon: "container", Title: "Houston"},
				{Icon: "building", Title: "New York / New Jersey"},
				{Icon: "anchor", Title: "Savannah"},
				{Icon: "ship", Title: "Seattle–Tacoma"},
				{Icon: "building", Title: "Miami"},
				{Icon: "container", Title: "Oakland"},
			},
			MoreLabel: "Ask about other ports",
		},

		TrustedBy: TrustedBy{
			Eyebrow: "Trusted by businesses across America",
			// PLACEHOLDER: fictional names. Only display real client logos
			// with written permission.
			Logos: []Logo{
				{"Northwind", "wide"},
				{"Harbor & Pine", "serif"},
				{"KESTREL", "mono"},
				{"brightline", "rounded"},
				{"Summit Foods", "serif"},
				{"AXIOM", "wide"},
				{"Tidewater", "rounded"},
			},
		},

		CarrierPartner: CarrierPartner{
			Eyebrow:   "Partner With Us",
			Title:     "Become a Carrier Partner.",
			Body:      "Join our network of vetted carriers. Get consistent loads, fair payments and a team that supports you.",
			Benefits:  []string{"Consistent load opportunities", "Fast & reliable payments", "Long-term partnerships", "Nationwide coverage"},
			Image:     Image{"/static/img/container-yard.jpg", ""},
			CardTitle: "Carrier Onboarding",
			Steps: []IconText{
				{Title: "Register", Text: "Create your profile."},
				{Title: "Upload Documents", Text: "Submit COI, W-9 and authority details."},
				{Title: "Get Approved", Text: "Quick verification."},
				{Title: "Start Hauling", Text: "Book active loads."},
			},
			// PLACEHOLDER: point at the carrier onboarding portal or form.
			CTA: Link{"Join Our Carrier Network", "mailto:carriers@example.com?subject=Carrier%20partnership"},
		},

		Testimonials: Testimonials{
			Eyebrow: "Trusted Nationwide",
			Title:   "What Shippers Say About Us.",
			// PLACEHOLDER: illustrative testimonials. Replace with real,
			// attributable quotes (with consent) before launch.
			Items: []Testimonial{
				{"Their team is responsive, helpful, and their rates are consistently competitive. Switching simplified our entire inbound program.", "Alex Morgan", "Operations Head", "Sample Manufacturing Co.", "AM", 5},
				{"On-time delivery, great communication and competitive pricing. A dependable partner for high-volume lanes.", "Priya Shah", "Supply Chain Manager", "Sample Retail Group", "PS", 5},
				{"Our dedicated account manager understands our business and always goes the extra mile to prevent terminal delays.", "Jordan Lee", "Logistics Director", "Sample Infrastructure LLC", "JL", 5},
			},
		},

		Contact: Contact{
			Badge: "Get In Touch",
			Title: "Ready to Move Your Freight?",
			Body:  "Tell us about your shipment and our logistics team will get back to you with a competitive quote, fast and tailored to your needs.",
			Details: []ContactDetail{
				{"home", "Head office", brand.HeadOffice, ""},
				{"phone", "Call us directly", brand.Phone, brand.PhoneHref},
				{"mail", "Email our team", brand.Email, "mailto:" + brand.Email},
				{"clock", "Operating hours", brand.Hours, ""},
			},
			Image:        Image{"/static/img/contact-terminal.jpg", ""},
			FormTitle:    "Request a Shipment Quote",
			FormFootnote: "Your data is safe. We never share your information.",
		},

		Track: Track{
			Title:        "Track a Shipment",
			Body:         "Enter your PRO, BOL or reference number.",
			InputLabel:   "Tracking number",
			InputHint:    "Letters, numbers and dashes only.",
			SubmitLabel:  "Track",
			NotConnected: "Online tracking is being connected. Your account manager can give you a live status update right now.",
		},

		Footer: Footer{
			Blurb: "Integrated logistics solutions across the USA. Asset-backed control, dedicated managers, and reliable execution.",
			Columns: []FooterColumn{
				{"Quick Links", []Link{{"About", "#about"}, {"Our Fleet", "#fleet"}, {"Industries", "#industries"}, {"Carrier Partners", "#carriers"}, {"Contact", "#contact"}}},
				{"Services", []Link{{"Road Freight", "#services"}, {"Air Freight", "#services"}, {"Sea Freight", "#services"}, {"Warehousing", "#warehousing"}, {"Port Drayage", "#ports"}}},
			},
			// PLACEHOLDER: set real profile URLs or remove.
			Social: []SocialLink{
				{"linkedin", "LinkedIn", "#"},
				{"facebook", "Facebook", "#"},
				{"x-social", "X", "#"},
				{"instagram", "Instagram", "#"},
			},
			// PLACEHOLDER: legal pages are not included in this build.
			Legal: []Link{{"Privacy Policy", "#"}, {"Terms of Service", "#"}},
		},

		ServiceOptions: []Option{
			{"ftl", "Full Truckload (FTL)"},
			{"ltl", "Less Than Truckload (LTL)"},
			{"reefer", "Refrigerated (Reefer)"},
			{"heavy_haul", "Project & Heavy Haul"},
			{"air", "Air Freight"},
			{"sea", "Sea Freight (FCL/LCL)"},
			{"drayage", "Port Drayage"},
			{"warehousing", "Warehousing"},
			{"transloading", "Transloading"},
			{"other", "Other / Not sure"},
		},
	}
}

// Images returns pointers to every image slot so callers can validate or
// rewrite paths in one place.
func (s *Site) Images() []*Image {
	refs := []*Image{
		&s.Hero.Image,
		&s.AccountManager.Image,
		&s.Warehousing.Image,
		&s.Ports.Image,
		&s.CarrierPartner.Image,
		&s.Contact.Image,
	}
	for i := range s.Services.Items {
		refs = append(refs, &s.Services.Items[i].Image)
	}
	return refs
}

// ServiceLabel returns the display label for a service option value.
func (s *Site) ServiceLabel(value string) string {
	for _, o := range s.ServiceOptions {
		if o.Value == value {
			return o.Label
		}
	}
	return value
}
