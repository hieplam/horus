package speechworker

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestRealWorker runs the adapter with the real speech worker and the real
// models. It runs only with HORUS_SPEECH_REAL=1, HORUS_SPEECH_PYTHON (the
// Python of a speech venv) and HORUS_SPEECH_MODELS (the folder with
// vieneu/, codec/ and whisper-small/). ffmpeg must be on the PATH. It uses
// about 2 GB of memory.
func TestRealWorker(t *testing.T) {
	if os.Getenv("HORUS_SPEECH_REAL") != "1" {
		t.Skip("set HORUS_SPEECH_REAL=1, HORUS_SPEECH_PYTHON and HORUS_SPEECH_MODELS to run the real speech worker")
	}
	python, models := os.Getenv("HORUS_SPEECH_PYTHON"), os.Getenv("HORUS_SPEECH_MODELS")
	if python == "" || models == "" {
		t.Fatal("HORUS_SPEECH_PYTHON and HORUS_SPEECH_MODELS must name the Python of a speech venv and the model folder")
	}
	dir := t.TempDir()
	w := New(context.Background(), Config{Python: python, Script: filepath.Join(dir, "speech", "worker.py"), Models: models})
	defer w.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	v, err := w.Voices(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("voices: %d, default %q", len(v.Voices), v.Default)
	if len(v.Voices) != 25 || v.Default == "" || !v.Started {
		t.Errorf("Voices = %d voices, default %q, started %v; want 25 voices and a default", len(v.Voices), v.Default, v.Started)
	}
	for i, text := range []string{"Xin chào các bạn, hôm nay chúng ta cùng tìm hiểu về hàng đợi tin nhắn.", "Máy chủ nhận yêu cầu, xử lý dữ liệu, rồi trả kết quả về cho người dùng."} {
		started := time.Now()
		out := filepath.Join(dir, "out", []string{"one.wav", "two.mp3"}[i])
		s, err := w.Speak(ctx, Text{Text: text, Voice: v.Default, Out: out, Format: strings.TrimPrefix(filepath.Ext(out), ".")})
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("%s: %.1f s, audio %.2f s, lufs %.1f, started %v, transcript %q", filepath.Base(out), time.Since(started).Seconds(), s.AudioSeconds, s.LUFS, s.Started, s.Transcript)
		fi, err := os.Stat(out)
		if err != nil || fi.Size() != s.Bytes || s.Started || s.LUFS < -18.5 || s.LUFS > -17.5 || strings.TrimSpace(s.Transcript) == "" {
			t.Errorf("Speak %s = %+v (file: %v); want the file, no new worker, -18 LUFS and a transcript", out, s, err)
		}
	}
}
