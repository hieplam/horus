//go:build goal

package goal

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"unicode"
)

// The oracle of the row speech-real. The test starts the built `horus mcp`
// through tests/speech/real_run.py (an MCP client), which calls
// generate_speech for the 5 sentences of tests/speech/sentences.json. The
// script only gives the raw answers of the tools. This file measures the
// loudness of each file with ffmpeg and computes the share of wrong words from
// the transcript in the answer, so a stub that prints a summary line passes
// nothing.

const (
	targetLUFS    = -18.0
	lufsTolerance = 0.5
)

var limitRe = regexp.MustCompile(`(?m)^\| ` + "`speech-real`" + ` \|.*share of wrong words of each sentence is at most ([0-9.]+)`)

// wrongWordLimit reads the limit from the row speech-real of the spec, so the
// test and the spec cannot differ.
func wrongWordLimit(t *testing.T) float64 {
	t.Helper()
	spec, err := os.ReadFile(filepath.Join(root, "docs", "specs", "2026-11-20-text-to-speech.md"))
	if err != nil {
		t.Fatal(err)
	}
	m := limitRe.FindSubmatch(spec)
	if m == nil {
		t.Fatal("the row speech-real of the spec has no \"share of wrong words of each sentence is at most <limit>\"")
	}
	v, err := strconv.ParseFloat(string(m[1]), 64)
	if err != nil || v <= 0 || v >= 1 {
		t.Fatalf("the limit %q of the row speech-real is not a number between 0 and 1", m[1])
	}
	return v
}

// realCall is one entry of the output of tests/speech/real_run.py speak.
type realCall struct {
	ID     string `json:"id"`
	Text   string `json:"text"`
	Answer struct {
		OK     bool `json:"ok"`
		Result struct {
			Workflow      string  `json:"workflow"`
			Voice         string  `json:"voice"`
			Path          string  `json:"path"`
			Bytes         int64   `json:"bytes"`
			Format        string  `json:"format"`
			AudioSeconds  float64 `json:"audio_seconds"`
			Transcript    string  `json:"transcript"`
			Seconds       float64 `json:"seconds"`
			StartedWorker bool    `json:"started_worker"`
		} `json:"result"`
		Error *errorInfo `json:"error"`
	} `json:"answer"`
}

func speechReal(t *testing.T) {
	checkWrongShare(t) // the function that this oracle uses is the function of baseline.py
	limit := wrongWordLimit(t)
	if os.Getenv("HORUS_SPEECH_REAL") != "1" {
		t.Skip("needs HORUS_SPEECH_REAL=1 on a PC where the speech models are installed (install_workflow for vi-speech with both consents), with ffmpeg on the PATH and a Python that has the package mcp (docs/mcp.md, \"Test the server\")")
	}
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Fatalf("ffmpeg is not on the PATH: this oracle measures the loudness with it")
	}
	py, why := python()
	if py == "" {
		t.Fatal(why)
	}
	repo, _ := filepath.Abs(root)
	goexe, err := command(root, nil, "go", "env", "GOEXE").Output()
	if err != nil {
		t.Fatalf("go env GOEXE: %v", err)
	}
	exe := filepath.Join(t.TempDir(), "horus"+strings.TrimSpace(string(goexe)))
	if out, err := command(root, nil, "go", "build", "-o", exe, "./cmd/horus").CombinedOutput(); err != nil {
		t.Fatalf("go build ./cmd/horus: %v\n%s", err, out)
	}
	keep := os.Getenv("HORUS_SPEECH_REAL_OUT") // an optional folder that keeps the files and the answers
	outDir := keep
	if outDir == "" {
		outDir = t.TempDir()
	}
	outDir, _ = filepath.Abs(outDir)
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// The 5 sentences come from the committed file. The suite reads it itself.
	var fixed struct {
		Sentences []struct{ ID, Text string } `json:"sentences"`
	}
	data, err := os.ReadFile(filepath.Join(root, "tests", "speech", "sentences.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &fixed); err != nil || len(fixed.Sentences) != 5 {
		t.Fatalf("tests/speech/sentences.json: want 5 sentences, got %d (%v)", len(fixed.Sentences), err)
	}

	cmd := command(root, nil, py, "-I", filepath.Join(repo, "tests", "speech", "real_run.py"), "speak",
		"--horus", exe, "--out", outDir)
	var stderr strings.Builder
	cmd.Stderr = &stderr
	raw, err := cmd.Output()
	if err != nil {
		t.Fatalf("tests/speech/real_run.py speak: %v\n%s", err, tail(stderr.String()))
	}
	if keep != "" {
		_ = os.WriteFile(filepath.Join(outDir, "answers.json"), raw, 0o644)
	}
	var got struct {
		Calls []realCall `json:"calls"`
	}
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("the output of tests/speech/real_run.py: %v\n%s", err, raw)
	}
	if len(got.Calls) != len(fixed.Sentences) {
		t.Fatalf("the client made %d calls, want %d", len(got.Calls), len(fixed.Sentences))
	}

	made, worst := 0, 0.0
	for i, s := range fixed.Sentences {
		c := got.Calls[i]
		r := c.Answer.Result
		bad := []string{}
		if c.ID != s.ID || c.Text != s.Text {
			bad = append(bad, "the client did not send the sentence of the file")
		}
		if !c.Answer.OK {
			bad = append(bad, fmt.Sprintf("generate_speech failed: %+v", c.Answer.Error))
		}
		want := filepath.Join(outDir, s.ID+".wav")
		if filepath.Clean(r.Path) != filepath.Clean(want) {
			bad = append(bad, fmt.Sprintf("the file is %q, want %q", r.Path, want))
		}
		info, statErr := os.Stat(want)
		if statErr != nil || info.Size() == 0 {
			bad = append(bad, "the file does not exist or is empty")
		}
		loud, lerr := loudness(want)
		if lerr != nil {
			bad = append(bad, "loudness: "+lerr.Error())
		} else if d := loud - targetLUFS; d > lufsTolerance || d < -lufsTolerance {
			bad = append(bad, fmt.Sprintf("loudness %.1f LUFS is not %.0f +/- %.1f", loud, targetLUFS, lufsTolerance))
		}
		share := wrongShare(s.Text, r.Transcript)
		if share > limit {
			bad = append(bad, fmt.Sprintf("share of wrong words %.3f is above %.3f", share, limit))
		}
		worst = max(worst, share)
		status := "ok"
		if len(bad) > 0 {
			status = "FAIL"
			t.Errorf("%s: %s", s.ID, strings.Join(bad, "; "))
		} else {
			made++
		}
		t.Logf("%s: %s, loudness %.1f LUFS, share of wrong words %.3f (limit %.3f), audio %.1f s, call %.1f s, started worker %v\n  text:       %s\n  transcript: %s",
			s.ID, status, loud, share, limit, r.AudioSeconds, r.Seconds, r.StartedWorker, s.Text, r.Transcript)
	}
	t.Logf("speech files that an agent made through horus: %d of %d, worst share of wrong words %.3f, limit %.3f", made, len(fixed.Sentences), worst, limit)
}

var lufsLine = regexp.MustCompile(`I:\s+(-?[\d.]+) LUFS`)

// loudness gives the integrated loudness of an audio file, measured by ffmpeg.
func loudness(path string) (float64, error) {
	out, err := exec.Command("ffmpeg", "-nostats", "-i", path, "-af", "ebur128=framelog=quiet", "-f", "null", "-").CombinedOutput()
	if err != nil {
		return 0, fmt.Errorf("ffmpeg: %v", err)
	}
	hits := lufsLine.FindAllStringSubmatch(string(out), -1)
	if len(hits) == 0 {
		return 0, fmt.Errorf("ffmpeg gave no loudness for %s", path)
	}
	return strconv.ParseFloat(hits[len(hits)-1][1], 64)
}

// compose gives the text in the form NFC for the Latin letters: a base letter
// and a mark become one letter. Python (baseline.py) does the same with
// unicodedata.normalize.
func compose(s string) string {
	table := map[[2]rune]rune{}
	for _, p := range composePairs {
		table[[2]rune{p[0], p[1]}] = p[2]
	}
	var out []rune
	for _, r := range s {
		if n := len(out); n > 0 && r >= 0x300 && r <= 0x36F {
			if c, ok := table[[2]rune{out[n-1], r}]; ok {
				out[n-1] = c
				continue
			}
		}
		out = append(out, r)
	}
	return string(out)
}

var tagRe = regexp.MustCompile(`\[[^\]]*\]`)

// words gives the words of a text: no tags, NFC, lower case, no punctuation.
// It is the function `words` of tests/speech/baseline.py.
func words(text string) []string {
	text = strings.ToLower(compose(tagRe.ReplaceAllString(text, " ")))
	return strings.FieldsFunc(text, func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsNumber(r) })
}

// wrongShare is the words of the text that the transcript does not have,
// divided by the words of the text. It is `wrong_share` of
// tests/speech/baseline.py, rounded to 3 digits.
func wrongShare(text, transcript string) float64 {
	want := words(text)
	if len(want) == 0 {
		return 1
	}
	have := map[string]int{}
	for _, w := range words(transcript) {
		have[w]++
	}
	wrong := 0
	for _, w := range want {
		if have[w] > 0 {
			have[w]--
		} else {
			wrong++
		}
	}
	v, _ := strconv.ParseFloat(fmt.Sprintf("%.3f", float64(wrong)/float64(len(want))), 64)
	return v
}

// checkWrongShare fixes wrongShare against the values that
// tests/speech/baseline.py gives for the same pairs. It runs first in the row.
func checkWrongShare(t *testing.T) {
	t.Helper()
	cases := []struct {
		text, transcript string
		want             float64
	}{
		{"Máy chủ nhận yêu cầu, xử lý.", "máy chủ nhận yêu cầu xử lý", 0},
		{"Máy chủ nhận yêu cầu, xử lý.", "Mấy chủ nhận yêu cầu xử lý", 0.143},
		{"[cười] Thật ra lỗi này", "Thật ra lối này", 0.25},
		{"Máy chủ", "Ma\u0301y chu\u0309", 0}, // NFD input
		{"Máy chủ", "", 1},
		{"Xin chào", "chào chào chào", 0.5},
	}
	for _, c := range cases {
		if got := wrongShare(c.text, c.transcript); got != c.want {
			t.Errorf("wrongShare(%q, %q) = %v, want %v", c.text, c.transcript, got, c.want)
		}
	}
}
