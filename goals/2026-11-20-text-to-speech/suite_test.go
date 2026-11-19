//go:build goal

// Package goal holds the goal rows of docs/specs/2026-11-20-text-to-speech.md:
// one test function for each row. Run it with
//
//	go test -tags goal -v ./goals/2026-11-20-text-to-speech
package goal

import "testing"

// The task that builds the outcome of a row gives the row its oracle.

func TestSpeechReal(t *testing.T) {
	t.Fatal("not built: generate_speech with the real models, 5 fixed sentences, the loudness and the transcript of each file")
}

func TestSpeechHidden(t *testing.T) {
	t.Fatal("not built: the tool generate_speech and the contract check of its schema")
}

func TestSpeechInstall(t *testing.T) {
	t.Fatal("not built: install_workflow of a speech workflow, and the uninstall, in a sandbox home folder")
}

func TestSpeechErrors(t *testing.T) {
	t.Fatal("not built: the contract checks of the 8 speech failures with a fake speech worker")
}

func TestSpeechContract(t *testing.T) {
	t.Fatal("not built: the speech checks of tests/mcp_contract.py")
}

func TestSpeechMacos(t *testing.T) {
	t.Fatal("not built: a speech check in the macOS job of CI")
}
