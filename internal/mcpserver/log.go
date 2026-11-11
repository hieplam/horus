// The server log. Part of the port of horus/server.py (main and log).
//
// Each line is "<time> horus <LEVEL> <message>", on stderr. HORUS_LOG_FILE adds
// a copy of the log in that file, for a harness that hides stderr.
package mcpserver

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"
)

var (
	logMu  sync.Mutex
	logOut io.Writer = os.Stderr
)

// logTo sets where the log goes. Tests use it.
func logTo(w io.Writer) { logMu.Lock(); logOut = w; logMu.Unlock() }

func logf(level, format string, a ...any) {
	now := time.Now()
	line := fmt.Sprintf("%s,%03d horus %s %s\n", now.Format("2006-01-02 15:04:05"), now.Nanosecond()/1e6, level,
		strings.TrimRight(fmt.Sprintf(format, a...), "\n"))
	logMu.Lock()
	defer logMu.Unlock()
	_, _ = io.WriteString(logOut, line)
}
