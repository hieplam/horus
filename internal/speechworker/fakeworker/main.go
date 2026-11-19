// Command fakeworker is a fake of the speech worker for the tests. It loads
// no model. It speaks the JSON lines of the speech worker
// (internal/speechworker/worker.py) and writes a short WAV file. The Go
// tests and the contract test build it and put it where the adapter
// expects the Python of the speech venv:
//
//	go build -o <venv>/Scripts/python.exe ./internal/speechworker/fakeworker
//
// It ignores its arguments. It answers:
//
//	voices  the voices "Hải Đăng" (the default) and "Mai Anh"
//	speak   writes 0.5 s of a quiet tone to "out" as a WAV file (48 kHz,
//	        16-bit, mono, also for the format mp3), and answers with the
//	        loudness -18.0 and the text as the transcript
//
// The environment sets the behaviour of each start:
//
//	HORUS_FAKE_SPEECH         the mode: empty = every request works
//	                            load   the first line says that the models did not load, then exit 1
//	                            die    a speak request ends the process with exit 3, without an answer
//	                            error  a speak request gets the answer ok=false
//	                            slow   a speak request waits 2 minutes before its answer
//	                            noise  each answer comes after a line that is not JSON and an
//	                                   answer with another id
//	HORUS_FAKE_SPEECH_RECORD  a file: each start appends "start <args>",
//	                          each request appends "<op> <id>"
package main

import (
	"bufio"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	rate    = 48000
	seconds = 0.5
)

func main() { os.Exit(run()) }

func run() int {
	mode := os.Getenv("HORUS_FAKE_SPEECH")
	record("start " + strings.Join(os.Args[1:], " "))
	out := json.NewEncoder(os.Stdout)
	if mode == "load" {
		fmt.Fprintln(os.Stderr, "fake worker: no model file")
		out.Encode(map[string]any{"ready": false, "error": "FileNotFoundError: no model file"})
		return 1
	}
	out.Encode(map[string]any{"ready": true, "voices": 2, "load_seconds": 0})

	in := bufio.NewScanner(os.Stdin)
	in.Buffer(make([]byte, 1<<20), 1<<20)
	for in.Scan() {
		var req struct {
			ID     *int   `json:"id"`
			Op     string `json:"op"`
			Text   string `json:"text"`
			Voice  string `json:"voice"`
			Out    string `json:"out"`
			Format string `json:"format"`
		}
		if err := json.Unmarshal(in.Bytes(), &req); err != nil {
			out.Encode(map[string]any{"id": nil, "ok": false, "error": "not a JSON line"})
			continue
		}
		id := 0
		if req.ID != nil {
			id = *req.ID
		}
		record(fmt.Sprintf("%s %d", req.Op, id))
		if mode == "noise" {
			fmt.Println("a line that is not JSON")
			out.Encode(map[string]any{"id": id + 1000, "ok": true})
		}
		switch {
		case req.Op == "voices":
			out.Encode(map[string]any{"id": id, "ok": true, "default": "Hải Đăng",
				"voices": []map[string]string{{"id": "Hải Đăng", "label": "Hải Đăng"}, {"id": "Mai Anh", "label": "Mai Anh"}}})
		case req.Op != "speak":
			out.Encode(map[string]any{"id": id, "ok": false, "error": "no request " + req.Op})
		case mode == "die":
			fmt.Fprintln(os.Stderr, "fake worker: stops in the request")
			return 3
		case mode == "error":
			out.Encode(map[string]any{"id": id, "ok": false, "error": "fake worker: the request failed"})
		default:
			if mode == "slow" {
				time.Sleep(2 * time.Minute)
			}
			n, err := writeWAV(req.Out)
			if err != nil {
				out.Encode(map[string]any{"id": id, "ok": false, "error": err.Error()})
				continue
			}
			out.Encode(map[string]any{"id": id, "ok": true, "path": req.Out, "bytes": n, "format": req.Format,
				"audio_seconds": seconds, "lufs": -18.0, "transcript": req.Text, "speak_seconds": 0, "read_seconds": 0})
		}
	}
	return 0
}

// record appends one line to the record file, when the test names one.
func record(line string) {
	name := os.Getenv("HORUS_FAKE_SPEECH_RECORD")
	if name == "" {
		return
	}
	if f, err := os.OpenFile(name, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644); err == nil {
		f.WriteString(line + "\n")
		f.Close()
	}
}

// writeWAV writes a quiet 440 Hz tone of `seconds` to path, and gives its
// size in bytes.
func writeWAV(path string) (int64, error) {
	if !filepath.IsAbs(path) {
		return 0, fmt.Errorf("out is not an absolute path: %q", path)
	}
	samples := int(rate * seconds)
	data := make([]byte, 2*samples)
	for i := range samples {
		v := int16(1000 * math.Sin(2*math.Pi*440*float64(i)/rate))
		binary.LittleEndian.PutUint16(data[2*i:], uint16(v))
	}
	head := make([]byte, 44)
	copy(head[0:], "RIFF")
	binary.LittleEndian.PutUint32(head[4:], uint32(36+len(data)))
	copy(head[8:], "WAVEfmt ")
	binary.LittleEndian.PutUint32(head[16:], 16)     // the size of the fmt chunk
	binary.LittleEndian.PutUint16(head[20:], 1)      // PCM
	binary.LittleEndian.PutUint16(head[22:], 1)      // mono
	binary.LittleEndian.PutUint32(head[24:], rate)   // samples per second
	binary.LittleEndian.PutUint32(head[28:], rate*2) // bytes per second
	binary.LittleEndian.PutUint16(head[32:], 2)      // bytes per frame
	binary.LittleEndian.PutUint16(head[34:], 16)     // bits per sample
	copy(head[36:], "data")
	binary.LittleEndian.PutUint32(head[40:], uint32(len(data)))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return 0, err
	}
	if err := os.WriteFile(path, append(head, data...), 0o644); err != nil {
		return 0, err
	}
	return int64(len(head) + len(data)), nil
}
