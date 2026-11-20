package archtest

import "strings"

// The layers of the spec docs/specs/2026-11-11-restructure.md.
const (
	wiring  = "wiring"
	handler = "handler"
	service = "service"
	adapter = "adapter"
	base    = "base"
	test    = "test" // test support: only a test file can import it
)

// layers gives the layer of each package, by its path in the module: the
// layer table of the spec. A package that is not in this table breaks the
// rule direction.
var layers = map[string]string{
	"cmd/horus":                        wiring,
	"internal/cli":                     handler,
	"internal/mcpserver":               handler,
	"internal/webui":                   handler, // the routes of horus api that are not MCP: /readyz, /livez, the Horus UI, /api/summary, /api/events
	"internal/worker":                  handler, // the loop of horus worker: it joins jobs and generate
	"internal/generate":                service,
	"internal/provision":               service,
	"internal/doctor":                  service,
	"internal/devtools":                service,
	"internal/jobs":                    service,
	"internal/speak":                   service,
	"internal/comfy":                   adapter,
	"internal/download":                adapter,
	"internal/modelfiles":              adapter,
	"internal/backend":                 adapter,
	"internal/speechworker":            adapter,
	"internal/postgres":                adapter, // the job store and the Postgres broker (pgx)
	"internal/catalog":                 base,
	"internal/graph":                   base,
	"internal/herr":                    base,
	"internal/jobdata":                 base,
	"internal/settings":                base,
	"internal/platform":                base,
	"internal/speechsetup":             base,
	"internal/version":                 base,
	"workflows":                        base, // the built-in workflows (go:embed)
	"internal/archtest":                test,
	"internal/comfy/comfytest":         test,
	"internal/backend/comfyclitest":    test,
	"internal/brokertest":              test, // the contract suites of jobs.Broker and jobs.Store, the in-memory broker and job store
	"internal/postgres/pgtest":         test, // a new test database for each test
	"internal/webui/uifixture":         test, // the two fixtures of the Horus UI: the example data of the reference, and other numbers
	"internal/speechworker/fakeworker": test, // the fake speech worker: a program that the tests build
	"goals":                            test, // the link check of the goal suites
}

// layerOf gives the layer of a package. Each goal suite (goals/<spec stem>)
// is a test package, so a new suite needs no line in the table.
func layerOf(pkg string) (string, bool) {
	if strings.HasPrefix(pkg, "goals/") {
		return test, true
	}
	layer, ok := layers[pkg]
	return layer, ok
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
