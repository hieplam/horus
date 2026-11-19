package jobs

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"path"
	"strconv"
	"strings"
)

// RecipeVersion is the v of the fingerprint text. A new recipe gets a new v
// and new key prefixes, so that an old key never matches (ADR 0027).
const RecipeVersion = 1

// PathStyle is the operating system of the host path in out_path. The recipe
// cleans the path by the rules of that system: an API process in a Linux
// container gets the host path of a Windows PC.
type PathStyle string

// The path styles.
const (
	PathWindows PathStyle = "windows"
	PathMacOS   PathStyle = "macos"
	PathLinux   PathStyle = "linux"
)

// Request is the input of the recipe: the arguments of one call after the
// argument check.
type Request struct {
	Tool     string // generate_image
	Workflow string
	// WorkflowRev is the revision of the workflow: see WorkflowRev.
	WorkflowRev string
	// Subject is the subject as the caller wrote it. The recipe joins its
	// words with one space.
	Subject string
	// Seed is the seed of the caller. nil: the caller gave no seed.
	Seed *uint64
	// Width and Height are the size of the caller. nil: the default of the
	// manifest, DefaultWidth and DefaultHeight.
	Width, Height               *int64
	DefaultWidth, DefaultHeight int64
	// OutPath is the host path of the caller. "": none.
	OutPath string
	// PathStyle is the system of OutPath. It must be set when OutPath is.
	PathStyle PathStyle
	Overwrite bool
	// CallerKey is the idempotency_key of the caller. "": none.
	CallerKey string
}

// Fingerprint is the result of the recipe.
type Fingerprint struct {
	// Text is the JSON text: no spaces, the keys in the order of ADR 0027.
	Text string
	// Hash is the request fingerprint: the hex SHA-256 of Text. The column
	// request_hash.
	Hash string
	// Key is the idempotency key: "k1:" + the hex SHA-256 of the tool, a line
	// end and the key of the caller; else "f1:" + Hash when the caller gave a
	// seed; else "" (NULL), because the caller asked for a new random image.
	// The column idempotency_key.
	Key string
}

// recipeText is the fingerprint text. The fields are in the order of ADR 0027.
type recipeText struct {
	V           int     `json:"v"`
	Tool        string  `json:"tool"`
	Workflow    string  `json:"workflow"`
	WorkflowRev string  `json:"workflow_rev"`
	Subject     string  `json:"subject"`
	Seed        *string `json:"seed"`
	Width       int64   `json:"width"`
	Height      int64   `json:"height"`
	OutPath     *string `json:"out_path"`
	Overwrite   bool    `json:"overwrite"`
}

// Recipe computes the fingerprint and the key of a request, with recipe
// version 1. The seed is decimal text in a JSON string, so that 2^64-1 keeps
// each digit. The JSON text keeps <, > and & as they are.
func Recipe(r Request) (Fingerprint, error) {
	t := recipeText{
		V:           RecipeVersion,
		Tool:        r.Tool,
		Workflow:    r.Workflow,
		WorkflowRev: r.WorkflowRev,
		Subject:     strings.Join(strings.Fields(r.Subject), " "),
		Width:       r.DefaultWidth,
		Height:      r.DefaultHeight,
		Overwrite:   r.Overwrite,
	}
	if r.Seed != nil {
		s := strconv.FormatUint(*r.Seed, 10)
		t.Seed = &s
	}
	if r.Width != nil {
		t.Width = *r.Width
	}
	if r.Height != nil {
		t.Height = *r.Height
	}
	if r.OutPath != "" {
		p, err := CleanPath(r.OutPath, r.PathStyle)
		if err != nil {
			return Fingerprint{}, err
		}
		t.OutPath = &p
	}
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(t); err != nil {
		return Fingerprint{}, err
	}
	text := strings.TrimSuffix(b.String(), "\n")
	f := Fingerprint{Text: text, Hash: sha256Hex([]byte(text))}
	switch {
	case r.CallerKey != "":
		f.Key = "k1:" + sha256Hex([]byte(r.Tool+"\n"+r.CallerKey))
	case r.Seed != nil:
		f.Key = "f1:" + f.Hash
	}
	return f, nil
}

// WorkflowRev is the revision of a workflow: the hex SHA-256 of manifest.json,
// one 0 byte and workflow.json.
func WorkflowRev(manifest, workflow []byte) string {
	h := sha256.New()
	h.Write(manifest)
	h.Write([]byte{0})
	h.Write(workflow)
	return hex.EncodeToString(h.Sum(nil))
}

func sha256Hex(b []byte) string {
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:])
}

// CleanPath cleans a host path by the rules of its system, on any system:
// one separator between the parts, no "." part, and each ".." part removes
// the part before it. On Windows the separator is \ and / counts as \. On
// Windows and macOS the path is in lower case, because their file systems
// do not tell the case apart.
func CleanPath(p string, style PathStyle) (string, error) {
	switch style {
	case PathLinux:
		return path.Clean(p), nil
	case PathMacOS:
		return strings.ToLower(path.Clean(p)), nil
	case PathWindows:
		s := strings.ReplaceAll(p, "/", `\`)
		vol := ""
		switch {
		case len(s) >= 2 && s[1] == ':' && isLetter(s[0]):
			vol, s = s[:2], s[2:]
		case strings.HasPrefix(s, `\\`):
			// A UNC path: \\server\share\rest.
			parts := strings.SplitN(s[2:], `\`, 3)
			if len(parts) >= 2 {
				vol, s = `\\`+parts[0]+`\`+parts[1], `\`
				if len(parts) == 3 {
					s += parts[2]
				}
			}
		}
		clean := strings.ReplaceAll(path.Clean(strings.ReplaceAll(s, `\`, "/")), "/", `\`)
		return strings.ToLower(vol + clean), nil
	}
	return "", fmt.Errorf("jobs: unknown path style %q", style)
}

func isLetter(c byte) bool { return 'a' <= c && c <= 'z' || 'A' <= c && c <= 'Z' }
