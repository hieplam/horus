// Progress notifications. Part of the port of horus/server.py (_reporter).
package server

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Reporter sends one progress notification. total 0 means unknown.
type Reporter func(ctx context.Context, progress, total float64, message string)

// reporter returns the Reporter of one call. It logs each report. It sends a
// notification only when the caller sent a progressToken, and it ignores a
// failed send, as _reporter does.
func reporter(req *mcp.CallToolRequest) Reporter {
	var token any
	var session *mcp.ServerSession
	if req != nil {
		session = req.Session
		if req.Params != nil {
			token = req.Params.GetProgressToken()
		}
	}
	return func(ctx context.Context, progress, total float64, message string) {
		logf("INFO", "progress %v %s", progress, message)
		if token == nil || session == nil {
			return
		}
		_ = session.NotifyProgress(ctx, &mcp.ProgressNotificationParams{
			ProgressToken: token, Progress: progress, Total: total, Message: message,
		})
	}
}

func progressToken(req *mcp.CallToolRequest) any {
	if req == nil || req.Params == nil {
		return nil
	}
	return req.Params.GetProgressToken()
}
