// The check of the arguments against the manifest of a speech workflow.
package speak

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/hieplam/horus/internal/catalog"
	"github.com/hieplam/horus/internal/herr"
	"github.com/hieplam/horus/internal/settings"
)

// checked is the output of check.
type checked struct {
	voice  string
	path   string
	format string
}

// textRules is the object "text" of a speech manifest.
type textRules struct {
	MinChars *int64          `json:"min_chars"`
	MaxChars *int64          `json:"max_chars"`
	Rules    json.RawMessage `json:"rules"`
}

// tagRE finds a [tag] marker: a "[", then no "]", then a "]". The speech
// model performs the tags of the manifest and reads any other [tag] aloud.
var tagRE = regexp.MustCompile(`\[[^\]]*\]`)

func invalid(msg string, details map[string]any) *herr.Error {
	return herr.New("INVALID_ARGUMENT", msg, details)
}

// check checks the arguments of r against the manifest of the speech
// workflow w. It changes no file.
func check(w *catalog.Workflow, r Request) (*checked, error) {
	m := w.Manifest
	var rules textRules
	var voices, tags, formats []string
	var def string
	for key, dst := range map[string]any{"text": &rules, "voices": &voices, "tags": &tags, "formats": &formats, "default_voice": &def} {
		if err := m.Decode(key, dst); err != nil {
			return nil, fmt.Errorf("manifest.json of %s: %s: %w", w.Name, key, err)
		}
	}

	lo, hi := int64(1), int64(4000)
	if rules.MinChars != nil {
		lo = *rules.MinChars
	}
	if rules.MaxChars != nil {
		hi = *rules.MaxChars
	}
	// The speech worker joins the white space of the text, so the count does too.
	n := int64(utf8.RuneCountInString(strings.Join(strings.Fields(r.Text), " ")))
	if n < lo || n > hi {
		var list any = []any{}
		if len(rules.Rules) > 0 {
			_ = json.Unmarshal(rules.Rules, &list)
		}
		return nil, invalid(fmt.Sprintf("text has %d characters; allowed: %d to %d", n, lo, hi),
			map[string]any{"field": "text", "min_chars": lo, "max_chars": hi, "got": n, "rules": list})
	}
	var unknown []string
	for _, tag := range tagRE.FindAllString(r.Text, -1) {
		if !slices.Contains(tags, tag) && !slices.Contains(unknown, tag) {
			unknown = append(unknown, tag)
		}
	}
	if len(unknown) > 0 {
		return nil, invalid(fmt.Sprintf("the text has a tag that the speech model does not perform: %s", strings.Join(unknown, ", ")),
			map[string]any{"field": "text", "found": unknown, "allowed_tags": tags})
	}

	c := &checked{voice: def}
	if r.Voice != nil && *r.Voice != "" {
		if !slices.Contains(voices, *r.Voice) {
			return nil, invalid(fmt.Sprintf("voice %s is not a voice of workflow %s", quote(*r.Voice), quote(w.Name)),
				map[string]any{"field": "voice", "got": *r.Voice, "known": voices})
		}
		c.voice = *r.Voice
	}

	if r.OutPath == nil || *r.OutPath == "" {
		c.format = "wav"
		if !slices.Contains(formats, c.format) && len(formats) > 0 {
			c.format = formats[0]
		}
		stamp := time.Now().UTC().Format("20060102T150405Z")
		c.path = filepath.Join(settings.OutputDir(), w.Name, fmt.Sprintf("%s-%s.%s", stamp, randomHex(4), c.format))
	} else {
		p := *r.OutPath
		if !filepath.IsAbs(p) {
			return nil, invalid("out_path must be an absolute path", map[string]any{"field": "out_path", "got": p})
		}
		suffix := strings.TrimPrefix(strings.ToLower(filepath.Ext(p)), ".")
		if !slices.Contains(formats, suffix) || strings.TrimSuffix(filepath.Base(p), filepath.Ext(p)) == "" {
			ends := make([]string, len(formats))
			for i, f := range formats {
				ends[i] = "." + f
			}
			return nil, invalid("out_path must end with "+strings.Join(ends, " or "),
				map[string]any{"field": "out_path", "got": p, "formats": formats})
		}
		c.path, c.format = filepath.Clean(p), suffix
	}
	if _, err := os.Stat(c.path); err == nil && !r.Overwrite {
		return nil, herr.New("OUTPUT_EXISTS", c.path+" exists", map[string]any{"path": c.path})
	}
	return c, nil
}

// randomHex gives n random bytes as hexadecimal text.
func randomHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// quote is repr() of a plain str: 'name'.
func quote(s string) string {
	if strings.Contains(s, "'") && !strings.Contains(s, `"`) {
		return `"` + s + `"`
	}
	return "'" + strings.ReplaceAll(strings.ReplaceAll(s, `\`, `\\`), "'", `\'`) + "'"
}
