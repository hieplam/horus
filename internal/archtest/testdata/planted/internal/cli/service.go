// Not a violation: a handler imports a service and a base package.
package cli

import (
	"net/http"

	"github.com/hieplam/horus/internal/generate"
	"github.com/hieplam/horus/internal/herr"
)
