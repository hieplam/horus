package httpguard

import (
	"crypto/rand"
	"testing"
)

func TestHostAllowed(t *testing.T) {
	ports := []int{8190, 18190}
	for host, want := range map[string]bool{
		"127.0.0.1:8190":    true,
		"localhost:18190":   true,
		"127.0.0.1:8191":    false,
		"127.0.0.1":         false,
		"evil.example:8190": false,
		"[::1]:8190":        false,
		"":                  false,
	} {
		if got := HostAllowed(host, ports); got != want {
			t.Errorf("HostAllowed(%q) = %v, want %v", host, got, want)
		}
	}
}

func TestOriginAllowed(t *testing.T) {
	ports := []int{8190}
	for origin, want := range map[string]bool{
		"":                       true,
		"http://127.0.0.1:8190":  true,
		"http://localhost:8190":  true,
		"https://127.0.0.1:8190": false,
		"http://127.0.0.1:8191":  false,
		"http://evil.example":    false,
		"null":                   false,
	} {
		if got := OriginAllowed(origin, ports); got != want {
			t.Errorf("OriginAllowed(%q) = %v, want %v", origin, got, want)
		}
	}
}

// TestBearerMatches makes its token at run time: no file holds a token, also
// not a made-up one. A failure names the case, not the header.
func TestBearerMatches(t *testing.T) {
	want := rand.Text()
	for _, c := range []struct {
		name, header string
		ok           bool
	}{
		{"the right token", "Bearer " + want, true},
		{"the scheme in lower case", "bearer " + want, true},
		{"spaces around the parts", "  Bearer  " + want + " ", true},
		{"one character more", "Bearer " + want + "x", false},
		{"one character less", "Bearer " + want[:len(want)-1], false},
		{"another scheme", "Basic " + want, false},
		{"no scheme", want, false},
		{"no token", "Bearer", false},
		{"no header", "", false},
	} {
		if got := BearerMatches(c.header, want); got != c.ok {
			t.Errorf("%s: BearerMatches = %v, want %v", c.name, got, c.ok)
		}
	}
	if BearerMatches("Bearer ", "") || BearerMatches("Bearer x", "") {
		t.Error("an empty token must never match")
	}
}
