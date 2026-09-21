package phone

import (
	"errors"
	"regexp"
	"testing"
)

func TestNormalize(t *testing.T) {
	tests := []struct {
		in   string
		want string
		err  error
	}{
		{"+1 (713) 555-0198", "+17135550198", nil},
		{"713.555.0198", "+17135550198", nil},
		{"(713) 555 0198", "+17135550198", nil},
		{"1-713-555-0198", "+17135550198", nil},
		{"713-555-0198 ext. 42", "+17135550198 ext. 42", nil},
		{"+1 713 555 0198 x7", "+17135550198 ext. 7", nil},
		{"713 555 0198 extension 123", "+17135550198 ext. 123", nil},
		{"+44 20 7946 0958", "+442079460958", nil},
		{"+91 98765 43210", "+919876543210", nil},
		{"", "", ErrEmpty},
		{"   ", "", ErrEmpty},
		{"call me", "", ErrCharacters},
		{"713-555-01a8", "", ErrCharacters},
		{"713+555+0198", "", ErrCharacters},
		{"12", "", ErrLength},
		{"555-0198", "", ErrLength},
		{"+1 713 555 01", "", ErrLength},
		{"+44 20", "", ErrLength},
		{"+1234567890123456", "", ErrLength},
		{"+0 20 7946 0958", "", ErrCountryCode},
		{"442079460958", "", ErrCountryCode},
		{"(013) 555-0198", "", ErrNANP},
		{"(713) 155-0198", "", ErrNANP},
		{"(911) 555-0198", "", ErrNANP},
		{"+1 (713) 411-0198", "", ErrNANP},
	}
	for _, tt := range tests {
		got, err := Normalize(tt.in)
		if !errors.Is(err, tt.err) || got != tt.want {
			t.Errorf("Normalize(%q) = %q, %v; want %q, %v", tt.in, got, err, tt.want, tt.err)
		}
	}
}

// Every number the server accepts must also pass the browser pre-check,
// otherwise valid users would be blocked client-side.
func TestInputPatternAcceptsValidNumbers(t *testing.T) {
	re := regexp.MustCompile(`^(?:` + InputPattern + `)$`)
	for _, in := range []string{"+1 (713) 555-0198", "713.555.0198", "713-555-0198 ext. 42", "+1 713 555 0198 x7", "713 555 0198 extension 123", "+44 20 7946 0958"} {
		if _, err := Normalize(in); err != nil {
			t.Fatalf("fixture %q is not valid: %v", in, err)
		}
		if !re.MatchString(in) {
			t.Errorf("InputPattern rejects valid number %q", in)
		}
	}
	for _, in := range []string{"call me", "12"} {
		if re.MatchString(in) {
			t.Errorf("InputPattern accepts %q", in)
		}
	}
}
