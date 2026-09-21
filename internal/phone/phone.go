// Package phone validates and normalizes phone numbers submitted through the
// site's forms. Normalize is authoritative; InputPattern is only a coarse
// in-browser pre-check so obvious typos are caught before submitting.
package phone

import (
	"errors"
	"regexp"
	"strings"
)

// InputPattern is used as the HTML pattern attribute on phone inputs. It is
// valid for browsers' "v"-flag pattern compilation and for Go's regexp.
const InputPattern = `\+?[0-9\s\(\)\.\-]{7,20}(\s*[A-Za-z\.]{1,10}\s*[0-9]{1,6})?`

// InputHint is shown by browsers when InputPattern does not match.
const InputHint = "Enter a phone number with area code, e.g. +1 713 555 0198"

// User-facing validation messages.
var (
	ErrEmpty       = errors.New("Please enter a phone number.")
	ErrCharacters  = errors.New("Phone numbers can only contain digits, spaces and + ( ) - . characters.")
	ErrLength      = errors.New("Please enter a complete phone number, including area code.")
	ErrCountryCode = errors.New("For numbers outside the US and Canada, start with + and the country code.")
	ErrNANP        = errors.New("That isn't a valid US or Canada number. Please check the area code.")
)

var extensionPattern = regexp.MustCompile(`(?i)^(.*?)\s*(?:extension|ext\.?|x)\s*([0-9]{1,6})$`)

// Normalize validates raw and returns it in E.164 form (e.g. "+17135550198"),
// with " ext. 123" appended when an extension was given.
//
// Rules:
//   - 10 digits, or 11 starting with 1, are treated as US/Canada (NANP) and
//     must have an area code and exchange starting with 2–9 (no N11 codes).
//   - Anything else must start with "+" and a country code: 8–15 digits.
func Normalize(raw string) (string, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return "", ErrEmpty
	}

	ext := ""
	if m := extensionPattern.FindStringSubmatch(s); m != nil {
		s, ext = strings.TrimSpace(m[1]), m[2]
	}

	international := strings.HasPrefix(s, "+")
	var b strings.Builder
	for i, r := range s {
		switch {
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '+' && i == 0:
		case r == ' ' || r == '(' || r == ')' || r == '-' || r == '.' || r == ' ':
		default:
			return "", ErrCharacters
		}
	}
	digits := b.String()

	switch {
	case international && strings.HasPrefix(digits, "1"), !international && len(digits) == 11 && digits[0] == '1':
		if len(digits) != 11 {
			return "", ErrLength
		}
		if !validNANP(digits[1:]) {
			return "", ErrNANP
		}
	case international:
		if len(digits) < 8 || len(digits) > 15 {
			return "", ErrLength
		}
		if digits[0] == '0' {
			return "", ErrCountryCode
		}
	case len(digits) == 10:
		if !validNANP(digits) {
			return "", ErrNANP
		}
		digits = "1" + digits
	case len(digits) < 10:
		return "", ErrLength
	default:
		return "", ErrCountryCode
	}

	out := "+" + digits
	if ext != "" {
		out += " ext. " + ext
	}
	return out, nil
}

// validNANP checks a 10-digit North American number.
func validNANP(ten string) bool {
	area, exchange := ten[:3], ten[3:6]
	return area[0] >= '2' && exchange[0] >= '2' && area[1:] != "11" && exchange[1:] != "11"
}
