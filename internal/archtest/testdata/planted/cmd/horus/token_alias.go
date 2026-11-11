// Planted: the same as token_name.go, through an import alias.
package main

import (
	"os"

	dl "github.com/hieplam/horus/internal/download"
)

var aliased = os.Getenv(dl.TokenVariable)
