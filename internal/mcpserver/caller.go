package mcpserver

import (
	"context"
	"net/url"
	"path"
	"strings"
	"time"
)

// rootsWithin is how long callerOf waits for the roots of a client. A client
// that does not answer gives a caller without the folder.
const rootsWithin = 2 * time.Second

// callerOf names the client of the session, for the column caller of the
// Horus UI: the title of the client (else its name), then " · folder " and
// the last element of the first root of the client, when the client gives a
// root. A session without a client name gives "".
func callerOf(ctx context.Context, c *call) string {
	if c.req == nil || c.req.Session == nil {
		return ""
	}
	p := c.req.Session.InitializeParams()
	if p == nil || p.ClientInfo == nil {
		return ""
	}
	name := strings.TrimSpace(p.ClientInfo.Title)
	if name == "" {
		name = strings.TrimSpace(p.ClientInfo.Name)
	}
	if name == "" {
		return ""
	}
	// A client on the protocol version 2026-07-28 cannot be asked for roots
	// while a request runs (SEP-2322): ListRoots gives an error, and the
	// caller is the name only. The server cannot tell from the capabilities of the client whether it
	// has roots, so it asks. A client without roots answers with an error,
	// and the caller is the name only.
	rctx, cancel := context.WithTimeout(ctx, rootsWithin)
	defer cancel()
	res, err := c.req.Session.ListRoots(rctx, nil)
	if err != nil || res == nil {
		return name
	}
	for _, r := range res.Roots {
		if r == nil {
			continue
		}
		if f := folderOfRoot(r.URI); f != "" {
			return name + " · folder " + f
		}
		break // only the first root counts
	}
	return name
}

// folderOfRoot is the last element of the path of a root URI
// (file:///C:/work/blog gives "blog"), or "" when the URI has none.
func folderOfRoot(uri string) string {
	u, err := url.Parse(uri)
	if err != nil || u.Scheme != "file" {
		return ""
	}
	p := strings.TrimRight(strings.ReplaceAll(u.Path, `\`, "/"), "/")
	if p == "" {
		return ""
	}
	b := path.Base(p)
	if b == "." || b == "/" || strings.HasSuffix(b, ":") {
		return ""
	}
	return b
}
