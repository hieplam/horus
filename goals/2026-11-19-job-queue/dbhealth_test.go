//go:build goal

// The measurement of the health check of horus-db, a part of the row
// stack-starts-together. Docker Compose starts horus-worker when horus-db is
// healthy, and horus worker connects over TCP. So horus-db must never be
// healthy while the database refuses a TCP connection. On a new volume, the
// image of Postgres first runs a temporary server that listens only on the
// Unix socket: a health check that asks the Unix socket passes too early.
package goal

import (
	"encoding/json"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"
)

// dbService is the service horus-db in docker compose config --format json.
type dbService struct {
	Image       string             `json:"image"`
	Environment map[string]*string `json:"environment"`
	Healthcheck struct {
		Test []string `json:"test"`
	} `json:"healthcheck"`
	Volumes []struct {
		Type   string `json:"type"`
		Target string `json:"target"`
	} `json:"volumes"`
}

// shQuote quotes one word for sh.
func shQuote(s string) string { return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'" }

// healthCommand gives the health check of a compose file as one sh command.
func healthCommand(t *testing.T, test []string) string {
	t.Helper()
	if len(test) >= 2 && test[0] == "CMD" {
		words := []string{}
		for _, w := range test[1:] {
			words = append(words, shQuote(w))
		}
		return strings.Join(words, " ")
	}
	if len(test) == 2 && test[0] == "CMD-SHELL" {
		return "sh -c " + shQuote(test[1])
	}
	t.Fatalf("compose.yaml: the health check of horus-db is %q, want a CMD or a CMD-SHELL command", test)
	return ""
}

// healthSample is one change in the measurement: the exit code of the health
// check of compose.yaml, and the exit code of pg_isready over TCP right after.
type healthSample struct {
	at          time.Time
	health, tcp int
}

// checkDBHealthNeedsTCP starts a container of the image of horus-db on a new
// volume, with the environment of horus-db from compose.yaml. A loop in the
// container runs the health check of horus-db from compose.yaml every 50 ms,
// and pg_isready -h 127.0.0.1 right after it, until the TCP check passed for
// 40 samples. The row fails when the health check gives exit 0 and the TCP
// check does not. It also fails when the measurement saw no time in which TCP
// was refused, or when the health check does not pass at the end. env is the
// environment of docker compose for the test stack.
func checkDBHealthNeedsTCP(t *testing.T, env []string) {
	t.Helper()
	// The output of config holds the password of the run: never print it.
	code, out := compose(t, env, "config", "--format", "json")
	if code != 0 {
		t.Fatalf("docker compose -p %s config: exit code %d", testStack, code)
	}
	var cfg struct {
		Services map[string]dbService `json:"services"`
	}
	if err := json.Unmarshal([]byte(out), &cfg); err != nil {
		t.Fatalf("docker compose config: the output is not JSON: %v", err)
	}
	db, ok := cfg.Services["horus-db"]
	if !ok || db.Image == "" {
		t.Fatal("compose.yaml: no service horus-db with an image")
	}
	health := healthCommand(t, db.Healthcheck.Test)

	// A container and a volume of this run only. The values of the
	// environment go into the environment of docker run, never onto its
	// command line.
	name := testStack + "-db-health-" + randomHex(t)[:8]
	args := []string{"run", "-d", "--name", name}
	runVars := env
	for k, v := range db.Environment {
		if v != nil {
			args = append(args, "-e", k)
			runVars = append(runVars, k+"="+*v)
		}
	}
	volumes := []string{}
	for i, v := range db.Volumes {
		if v.Type == "volume" {
			volumes = append(volumes, name+"-"+strconv.Itoa(i))
			args = append(args, "-v", volumes[len(volumes)-1]+":"+v.Target)
		}
	}
	t.Cleanup(func() {
		if code, out := run(t, root, "docker", "rm", "-f", "-v", name); code != 0 {
			t.Errorf("docker rm -f -v %s: exit code %d\n%s", name, code, out)
		}
		for _, v := range volumes {
			if code, out := run(t, root, "docker", "volume", "rm", "-f", v); code != 0 {
				t.Errorf("docker volume rm %s: exit code %d\n%s", v, code, out)
			}
		}
	})
	start := exec.Command("docker", append(args, db.Image)...)
	start.Dir, start.Env = root, runVars
	if b, err := start.CombinedOutput(); err != nil {
		t.Fatalf("docker run %s (%s): %v\n%s", name, db.Image, err, b)
	}

	// The loop prints one line for each change: the time in ms, the exit code
	// of the health check, the exit code of the TCP check. It ends when the
	// TCP check passed 40 times in a row, or after 2400 samples.
	script := `quiet=0; last=; n=0
while [ $n -lt 2400 ] && [ $quiet -lt 40 ]; do
  ` + health + ` >/dev/null 2>&1; h=$?
  pg_isready -h 127.0.0.1 -U "$POSTGRES_USER" -d "$POSTGRES_DB" >/dev/null 2>&1; c=$?
  if [ "$h $c" != "$last" ]; then last="$h $c"; echo "$(date +%s%3N) $h $c"; fi
  if [ $c -eq 0 ]; then quiet=$((quiet+1)); else quiet=0; fi
  n=$((n+1)); sleep 0.05
done
echo "$(date +%s%3N) end"
`
	loop := exec.Command("docker", "exec", "-i", name, "sh")
	loop.Dir, loop.Stdin = root, strings.NewReader(script)
	b, err := loop.CombinedOutput()
	if err != nil {
		_, logs := run(t, root, "docker", "logs", "--tail", "20", name)
		t.Fatalf("the measurement in %s: %v\n%s\n%s", name, err, b, logs)
	}

	var samples []healthSample
	var end time.Time
	for _, line := range strings.Split(strings.TrimSpace(string(b)), "\n") {
		f := strings.Fields(line)
		if len(f) < 2 {
			t.Fatalf("the measurement: a line that does not parse: %q", line)
		}
		ms, err := strconv.ParseInt(f[0], 10, 64)
		if err != nil {
			t.Fatalf("the measurement: a line that does not parse: %q", line)
		}
		at := time.UnixMilli(ms).UTC()
		if f[1] == "end" {
			end = at
			break
		}
		if len(f) != 3 {
			t.Fatalf("the measurement: a line that does not parse: %q", line)
		}
		samples = append(samples, healthSample{at: at, health: atoi(f[1]), tcp: atoi(f[2])})
	}
	if len(samples) == 0 || end.IsZero() {
		t.Fatalf("the measurement gave no sample:\n%s", b)
	}

	var early time.Duration // the time in which horus-db is healthy and refuses TCP
	sawRefused := false
	for i, s := range samples {
		next := end
		if i+1 < len(samples) {
			next = samples[i+1].at
		}
		note := ""
		if s.tcp != 0 {
			sawRefused = true
			if s.health == 0 {
				early += next.Sub(s.at)
				note = "   <- healthy, and TCP is refused"
			}
		}
		t.Logf("%s the health check of horus-db: exit %d; pg_isready -h 127.0.0.1: exit %d%s",
			s.at.Format("15:04:05.000"), s.health, s.tcp, note)
	}
	last := samples[len(samples)-1]
	switch {
	case early > 0:
		t.Fatalf("the health check of horus-db (%s) gives exit 0 for %d ms while the database refuses TCP: "+
			"Docker Compose starts horus-worker too early", health, early.Milliseconds())
	case !sawRefused:
		t.Fatal("the measurement saw no time in which the database refused TCP, so it proves nothing")
	case last.health != 0 || last.tcp != 0:
		t.Fatalf("at the end, the health check of horus-db (%s) gives exit %d and the TCP check gives exit %d, want 0 and 0",
			health, last.health, last.tcp)
	}
	t.Logf("the health check of horus-db: %s; the time in which horus-db is healthy and refuses TCP: %d ms",
		health, early.Milliseconds())
}
