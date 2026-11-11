package provision

import (
	"os"
	"testing"
)

// The test binary is also two fake programs, so the tests of
// InstallComfyMCP and Register run the real platform.Programs on every
// system:
//
//   - HORUS_FAKE_PY set: the Python launcher and the python of the venv
//     (fakePython, comfymcp_test.go).
//   - HORUS_FAKE_CLAUDE set: the claude program (fakeClaudeMain,
//     register_test.go).
func TestMain(m *testing.M) {
	if os.Getenv("HORUS_FAKE_PY") != "" {
		os.Exit(fakePython(os.Args[1:]))
	}
	if rec := os.Getenv("HORUS_FAKE_CLAUDE"); rec != "" {
		os.Exit(fakeClaudeMain(rec))
	}
	os.Exit(m.Run())
}
