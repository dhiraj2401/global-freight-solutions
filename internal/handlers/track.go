package handlers

import (
	"net/http"
	"net/url"
	"regexp"
	"strings"
)

var trackingNumberPattern = regexp.MustCompile(`^[A-Z0-9-]{4,40}$`)

// trackView is the data for partials/track-result.
type trackView struct {
	Number  string
	Message string
	Error   string
	Phone   string
}

// TrackHandler handles the "Track Shipment" dialog. When TRACKING_URL is set
// it redirects to that tracking portal; otherwise it explains how to get a
// status update until a TMS integration exists.
func (app *App) TrackHandler(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 4<<10)
	view := trackView{Phone: app.site.Brand.Phone}

	if !app.sameOrigin(r) {
		view.Error = "We couldn't verify this request. Please reload the page and try again."
		app.renderFragment(w, r, http.StatusForbidden, "partials/track-result", "track", view)
		return
	}
	if err := r.ParseForm(); err != nil {
		view.Error = "We couldn't read that tracking number. Please try again."
		app.renderFragment(w, r, http.StatusBadRequest, "partials/track-result", "track", view)
		return
	}

	number := strings.ToUpper(strings.TrimSpace(r.PostFormValue("tracking_number")))
	if !trackingNumberPattern.MatchString(number) {
		view.Error = "Enter a valid tracking number: 4–40 letters, numbers or dashes."
		app.renderFragment(w, r, http.StatusUnprocessableEntity, "partials/track-result", "track", view)
		return
	}

	if app.cfg.TrackingURL != "" {
		target := strings.ReplaceAll(app.cfg.TrackingURL, "{ref}", url.QueryEscape(number))
		if isHTMX(r) {
			w.Header().Set("HX-Redirect", target)
			w.WriteHeader(http.StatusOK)
			return
		}
		http.Redirect(w, r, target, http.StatusSeeOther)
		return
	}

	view.Number = number
	view.Message = app.site.Track.NotConnected
	app.renderFragment(w, r, http.StatusOK, "partials/track-result", "track", view)
}
