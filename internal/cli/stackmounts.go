// horus stack-mounts: the shared folders of the stack (ADR 0038). Docker
// Compose cannot make one mount for each pair of a list in a variable, so
// this command writes them into compose.override.yaml, which docker compose
// reads next to compose.yaml.
package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/hieplam/horus/internal/settings"
)

// OverrideFile is the file of the mounts next to compose.yaml. docker compose
// reads it when no -f flag names the files.
const OverrideFile = "compose.override.yaml"

// DefaultSharedContainer is the container folder of the default shared
// folder: the output folder of horus on the host.
const DefaultSharedContainer = settings.SharedPrefix + "outputs"

// StackMounts is horus stack-mounts. It reads HORUS_SHARED_DIRS of the host
// (default: one pair, the output folder of horus and /shared/outputs), makes
// each host folder that is missing, and writes the mounts and the settings of
// the shared folders into the override file.
func StackMounts(args []string, out, errOut io.Writer) int {
	fs := newFlags("horus stack-mounts", errOut)
	file := fs.String("o", "", "the file to write (default: "+OverrideFile+" in the repo root, next to compose.yaml)")
	fs.Usage = func() {
		w := fs.Output()
		fmt.Fprintln(w, "Usage: horus stack-mounts [-o <file>]")
		fmt.Fprintln(w, "")
		fmt.Fprintln(w, "Writes the shared folders of the stack into "+OverrideFile+": one mount of each host")
		fmt.Fprintln(w, "folder into horus-api and horus-worker, and the settings HORUS_SHARED_DIRS, HORUS_OUTPUT_DIR")
		fmt.Fprintln(w, "and HORUS_HOST_OS of the two containers. docker compose up -d reads the file.")
		fmt.Fprintln(w, "")
		fmt.Fprintln(w, "HORUS_SHARED_DIRS holds pairs <host folder>=<container folder>, with ; between two pairs, for")
		fmt.Fprintln(w, `example C:\Users\me\Pictures\horus=/shared/pictures. Each container folder is one folder under`)
		fmt.Fprintln(w, "/shared/. Without the setting, the one pair is the output folder of horus and "+DefaultSharedContainer+".")
		fmt.Fprintln(w, "A call without out_path writes its PNG into the first shared folder.")
		fmt.Fprintln(w, "")
		fs.PrintDefaults()
	}
	if code, done := parse(fs, args); done {
		return code
	}
	if fs.NArg() > 0 {
		fmt.Fprintf(errOut, "horus stack-mounts: no arguments allowed, got %q\n", fs.Args())
		return 2
	}
	dirs, err := settings.SharedDirs()
	if err != nil {
		fmt.Fprintln(errOut, "horus stack-mounts:", err)
		return 1
	}
	if len(dirs) == 0 {
		dirs = []settings.SharedDir{{Host: settings.OutputDir(), Container: DefaultSharedContainer}}
	}
	for _, d := range dirs {
		if !filepath.IsAbs(d.Host) {
			fmt.Fprintf(errOut, "horus stack-mounts: the host folder %q of HORUS_SHARED_DIRS is not an absolute path\n", d.Host)
			return 1
		}
		if err := os.MkdirAll(d.Host, 0o755); err != nil {
			fmt.Fprintln(errOut, "horus stack-mounts:", err)
			return 1
		}
	}
	path := *file
	if path == "" {
		root := settings.Root()
		if root == "" {
			fmt.Fprintln(errOut, "horus stack-mounts: no horus clone here: run it in the clone of the stack, or give -o <file>")
			return 1
		}
		path = filepath.Join(root, OverrideFile)
	}
	if err := os.WriteFile(path, []byte(StackOverride(dirs, runtime.GOOS)), 0o644); err != nil {
		fmt.Fprintln(errOut, "horus stack-mounts:", err)
		return 1
	}
	fmt.Fprintf(out, "horus stack-mounts: wrote %s with %d shared folders:\n", path, len(dirs))
	for _, d := range dirs {
		fmt.Fprintf(out, "  %s  ->  %s\n", d.Host, d.Container)
	}
	fmt.Fprintln(out, "Run docker compose up -d to apply the mounts.")
	return 0
}

// StackOverride is the text of the override file for the shared folders dirs
// of a host with the system hostOS: for horus-api and horus-worker, one bind
// mount of each pair, and the settings HORUS_SHARED_DIRS, HORUS_OUTPUT_DIR
// (the container folder of the first pair) and HORUS_HOST_OS.
func StackOverride(dirs []settings.SharedDir, hostOS string) string {
	var b strings.Builder
	b.WriteString("# The shared folders of the horus stack. horus stack-mounts writes this file\n")
	b.WriteString("# from HORUS_SHARED_DIRS, and docker compose reads it next to compose.yaml.\n")
	b.WriteString("# Do not edit it: change HORUS_SHARED_DIRS, run horus stack-mounts again,\n")
	b.WriteString("# then docker compose up -d.\n")
	b.WriteString("services:\n")
	for _, svc := range []string{"horus-api", "horus-worker"} {
		fmt.Fprintf(&b, "  %s:\n", svc)
		b.WriteString("    environment:\n")
		fmt.Fprintf(&b, "      HORUS_SHARED_DIRS: %s\n", yamlString(settings.FormatSharedDirs(dirs)))
		if len(dirs) > 0 {
			fmt.Fprintf(&b, "      HORUS_OUTPUT_DIR: %s\n", yamlString(dirs[0].Container))
		}
		fmt.Fprintf(&b, "      HORUS_HOST_OS: %s\n", yamlString(hostOS))
		b.WriteString("    volumes:\n")
		for _, d := range dirs {
			b.WriteString("      - type: bind\n")
			fmt.Fprintf(&b, "        source: %s\n", yamlString(d.Host))
			fmt.Fprintf(&b, "        target: %s\n", yamlString(d.Container))
		}
	}
	return b.String()
}

// yamlString is s as a YAML string in double quotes. A JSON string is a YAML
// string in double quotes. docker compose reads $ as the start of a
// variable, so each $ is $$.
func yamlString(s string) string {
	var b strings.Builder
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(strings.ReplaceAll(s, "$", "$$"))
	return strings.TrimSuffix(b.String(), "\n")
}
