// Package httpguard holds the checks of a request to horus api, which serves
// only on 127.0.0.1 (ADR 0003, ADR 0030, ADR 0040): the Host header, the
// Origin header and the bearer token. They are pure functions. The handlers
// internal/webui (the Horus UI) and internal/mcpserver (/mcp) call them in
// front of their routes, so that both routes use one check.
//
// A web page of another domain can send a request to 127.0.0.1 through DNS
// rebinding. The browser then sends the name of that domain in the Host
// header, and the origin of the page in the Origin header. So a request must
// name 127.0.0.1:<port> or localhost:<port> in the Host header, and send no
// Origin header or the origin of horus api itself.
package httpguard

import (
	"crypto/sha256"
	"crypto/subtle"
	"strconv"
	"strings"
)

// HostAllowed tells if the Host header host is 127.0.0.1:<port> or
// localhost:<port> for one of the ports.
func HostAllowed(host string, ports []int) bool {
	for _, p := range ports {
		ps := strconv.Itoa(p)
		if host == "127.0.0.1:"+ps || host == "localhost:"+ps {
			return true
		}
	}
	return false
}

// OriginAllowed tells if the Origin header origin is absent ("") or is the
// origin of horus api itself: http://127.0.0.1:<port> or
// http://localhost:<port>, for one of the ports. An agent harness sends no
// Origin header. A web page in a browser sends its own origin, and "null"
// from a file or a sandbox.
func OriginAllowed(origin string, ports []int) bool {
	if origin == "" {
		return true
	}
	host, ok := strings.CutPrefix(origin, "http://")
	return ok && HostAllowed(host, ports)
}

// BearerMatches tells if the Authorization header authorization is
// "Bearer <token>" and the token equals want. The compare takes the same time
// for each token: it compares the SHA-256 of the two values, so neither the
// length nor the first wrong character changes the time. An empty want never
// matches: horus api without HORUS_API_TOKEN refuses each request.
func BearerMatches(authorization, want string) bool {
	if want == "" {
		return false
	}
	scheme, token, ok := strings.Cut(strings.TrimSpace(authorization), " ")
	if !ok || !strings.EqualFold(scheme, "Bearer") {
		return false
	}
	got := sha256.Sum256([]byte(strings.TrimSpace(token)))
	exp := sha256.Sum256([]byte(want))
	return subtle.ConstantTimeCompare(got[:], exp[:]) == 1
}
