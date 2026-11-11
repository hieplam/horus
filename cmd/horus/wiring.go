// The wiring: cmd/horus creates the adapters and gives them to the services.
package main

import (
	"github.com/hieplam/horus/internal/comfy"
	"github.com/hieplam/horus/internal/generate"
)

// newImages makes the image service on the ComfyUI adapter. An empty url
// means the HORUS_COMFYUI_URL setting, read at each request.
func newImages(url string) *generate.Service {
	return generate.New(comfy.New(url))
}
