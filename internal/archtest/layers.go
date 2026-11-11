package archtest

// The layers of the spec docs/specs/2026-11-11-restructure.md.
const (
	wiring  = "wiring"
	handler = "handler"
	service = "service"
	adapter = "adapter"
	base    = "base"
	test    = "test" // test support: only a test file can import it
)

// layers gives the layer of each package, by its path in the module. A
// package that is not in this table breaks the rule direction.
var layers = map[string]string{
	// The layer table of the spec.
	"cmd/horus":                wiring,
	"internal/cli":             handler,
	"internal/mcpserver":       handler,
	"internal/generate":        service,
	"internal/provision":       service,
	"internal/doctor":          service,
	"internal/devtools":        service,
	"internal/comfy":           adapter,
	"internal/download":        adapter,
	"internal/modelfiles":      adapter,
	"internal/backend":         adapter,
	"internal/catalog":         base,
	"internal/graph":           base,
	"internal/herr":            base,
	"internal/settings":        base,
	"internal/platform":        base,
	"internal/archtest":        test,
	"internal/comfy/comfytest": test,

	// Phase 1 packages. Each one has the layer of the package that its code
	// moves to. Remove a line when its package is gone.
	"internal/scripts/checksetup":      service, // internal/doctor
	"internal/scripts/installcomfymcp": service, // internal/provision
	"internal/scripts/newworkflow":     service, // internal/devtools
	"internal/scripts/register":        service, // internal/provision
	"internal/scripts/syncworkflows":   service, // internal/devtools
}

// mayImport gives, for each layer, the layers of the module that its
// packages can import. A layer that is not a key can import every layer.
var mayImport = map[string][]string{
	handler: {service, base},
	service: {base},
	adapter: {base},
	base:    {base},
}

// serviceBans are the standard packages that a service must not import.
var serviceBans = []string{"net/http", "os/exec"}

// osExempt are the packages (and their subfolders) that can hold code for
// one operating system.
var osExempt = []string{"internal/platform", "internal/backend"}

// tokenExempt are the places where the text HF_TOKEN can be: the download
// package, and the refusal of internal/settings with its test.
var tokenExempt = []string{"internal/download", "internal/settings/settings.go", "internal/settings/settings_test.go"}

// osNames are the operating system names of Go (GOOS), and "unix".
var osNames = []string{"aix", "android", "darwin", "dragonfly", "freebsd", "hurd", "illumos", "ios", "js",
	"linux", "nacl", "netbsd", "openbsd", "plan9", "solaris", "unix", "wasip1", "windows", "zos"}
