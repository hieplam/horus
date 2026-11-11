// Package herr is the error model of horus.
//
// Every failure is an *Error with a stable Code. A caller (a person or an AI
// agent) decides what to do from three fields, without reading the message:
//
//	retryable   true: the same call can pass when it is sent again.
//	action_by   who must act before the next call can pass:
//	              "caller"  change the arguments (for example a smaller size),
//	              "retry"   send the same call again,
//	              "human"   a person must do something on this machine
//	                        (accept a license, set a token, free disk space).
//	next_step   one sentence: the exact thing to do next.
//
// Details holds the facts (file names, allowed values, the backend message).
// Codes is the single list of codes. docs/mcp.md shows the same table.
//
// The package is named herr so that it does not clash with the standard
// package errors.
package herr

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// Spec is one row of the code table.
type Spec struct {
	Code      string
	Retryable bool
	ActionBy  string
	NextStep  string
}

// Codes is the code table.
//
// A next_step names the horus subcommand of a setup step, not a script.
var Codes = []Spec{
	{"INVALID_ARGUMENT", false, "caller", "Fix the argument named in details.field, then call again."},
	{"WORKFLOW_NOT_FOUND", false, "caller", "Call list_workflows and use one of the names in details.known."},
	{"WORKFLOW_NOT_INSTALLED", false, "caller", "Call install_workflow for this workflow, then call generate_image again."},
	{"LICENSE_CONFIRMATION_REQUIRED", false, "human",
		"Show the license and the download size to the user. When the user agrees, call install_workflow again with accept_license=true."},
	{"CONFIRMATION_REQUIRED", false, "human",
		"Show the plan in details.files to the user. When the user agrees, call again with confirm=true."},
	{"OUTPUT_EXISTS", false, "caller", "Pass another out_path, or pass overwrite=true."},
	{"BACKEND_UNAVAILABLE", true, "human",
		"The local image backend (ComfyUI) does not answer and horus could not start it. Ask the user to run horus start-comfyui in the horus repo and read docs/setup.md."},
	{"BACKEND_NOT_INSTALLED", false, "human", "Ask the user to set up horus on this machine: docs/setup.md in the horus repo."},
	{"BACKEND_RESTART_REQUIRED", true, "human",
		"The image backend must restart to see new model files. Ask the user to restart ComfyUI, then call again."},
	{"NODE_PACK_MISSING", false, "human", "The backend lacks a node pack that this workflow needs. Ask the user to install it (see details.missing)."},
	{"HF_TOKEN_INVALID", false, "human",
		"The Hugging Face token is missing or wrong. Ask the user to put a valid Read token in HF_TOKEN in the .env file of the horus repo."},
	{"LICENSE_NOT_ACCEPTED", false, "human",
		"The Hugging Face account has not accepted the model license. Ask the user to open details.license_url, accept the license, then call again."},
	{"DISK_FULL", false, "human", "Not enough free disk space. Ask the user to free the space in details, then call again."},
	{"FILE_CONFLICT", false, "human",
		"A different file with the same name is on disk. horus never overwrites it. Ask the user to move or delete details.path."},
	{"FILE_IN_USE", true, "human",
		"A model file is open in the image backend and Windows does not let horus delete it. Ask the user to stop ComfyUI, then call again."},
	{"DOWNLOAD_FAILED", true, "retry", "Call install_workflow again. It resumes the partial download."},
	{"OUT_OF_MEMORY", true, "caller", "Call again. If it fails again, use a smaller width and height."},
	{"GENERATION_FAILED", true, "retry", "Call generate_image again with the same arguments. If it fails twice, report details.backend_message to the user."},
	{"GENERATION_TIMEOUT", true, "retry", "The job took longer than the time limit and horus cancelled it. Call again when the machine is less busy."},
	{"CANCELLED", true, "retry", "The job was cancelled on the backend. Call again if you still need the image."},
	{"INTERNAL", false, "human", "This is a bug in horus. Report details to the user."},
}

// lookup returns the row of a code.
func lookup(code string) (Spec, bool) {
	for _, s := range Codes {
		if s.Code == code {
			return s, true
		}
	}
	return Spec{}, false
}

// Error is a horus failure with a stable code.
type Error struct {
	Code    string
	Message string
	Details map[string]any
	// NextStep replaces the default next_step of the code when it is not empty.
	NextStep string
	// Retryable replaces the default retryable of the code when it is not nil.
	Retryable *bool
}

// New makes an *Error. It panics on an unknown code, as HorusError raises ValueError.
func New(code, message string, details map[string]any) *Error {
	if _, ok := lookup(code); !ok {
		panic(fmt.Sprintf("unknown error code %s", code))
	}
	return &Error{Code: code, Message: message, Details: details}
}

func (e *Error) Error() string { return e.Code + ": " + e.Message }

// Info is the error part of the envelope, with the keys in the order of
// HorusError.to_dict.
type Info struct {
	Code      string         `json:"code"`
	Message   string         `json:"message"`
	Retryable bool           `json:"retryable"`
	ActionBy  string         `json:"action_by"`
	NextStep  string         `json:"next_step"`
	Details   map[string]any `json:"details"`
}

// Info returns the error part of the envelope.
func (e *Error) Info() Info {
	s, ok := lookup(e.Code)
	if !ok {
		panic(fmt.Sprintf("unknown error code %s", e.Code))
	}
	info := Info{Code: e.Code, Message: e.Message, Retryable: s.Retryable, ActionBy: s.ActionBy, NextStep: s.NextStep, Details: e.Details}
	if e.Retryable != nil {
		info.Retryable = *e.Retryable
	}
	if e.NextStep != "" {
		info.NextStep = e.NextStep
	}
	if info.Details == nil {
		info.Details = map[string]any{}
	}
	return info
}

// TableJSON is the output of `horus errors -json`: one object whose keys are
// the codes in table order, each with retryable, action_by and next_step.
func TableJSON() []byte {
	var b bytes.Buffer
	b.WriteString("{\n")
	for i, s := range Codes {
		row, _ := json.Marshal(struct {
			Retryable bool   `json:"retryable"`
			ActionBy  string `json:"action_by"`
			NextStep  string `json:"next_step"`
		}{s.Retryable, s.ActionBy, s.NextStep})
		key, _ := json.Marshal(s.Code)
		fmt.Fprintf(&b, " %s: %s", key, row)
		if i < len(Codes)-1 {
			b.WriteString(",")
		}
		b.WriteString("\n")
	}
	b.WriteString("}\n")
	return b.Bytes()
}
