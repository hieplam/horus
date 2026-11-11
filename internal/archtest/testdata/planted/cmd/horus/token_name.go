// Planted: the wiring reads the name of the token variable through a
// constant of the download package, so no string names HF_TOKEN here.
package main

import (
	"os"

	"github.com/hieplam/horus/internal/download"
)

var token = os.Getenv(download.DefaultTokenVar)
