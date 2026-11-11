package platform

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// The test binary is also the helper program of the tree test. The variable
// HORUS_PLATFORM_HELPER selects its role:
//
//	starter     start the child with StartTree, print the id of the tree, end
//	child       start the grandchild, write "<child pid> <grandchild pid>"
//	            to the file HORUS_TREE_PIDS, then sleep
//	grandchild  sleep
//	end         call EndTree(HORUS_TREE_ID)
const helperEnv = "HORUS_PLATFORM_HELPER"

// helperLife is how long the child and the grandchild live when nothing ends
// them. The test ends them long before.
const helperLife = 2 * time.Minute

func TestMain(m *testing.M) {
	switch os.Getenv(helperEnv) {
	case "":
		os.Exit(m.Run())
	case "starter":
		cmd := exec.Command(os.Args[0])
		cmd.Env = append(os.Environ(), helperEnv+"=child")
		id, err := StartTree(cmd)
		if err != nil {
			fmt.Fprintln(os.Stderr, "StartTree:", err)
			os.Exit(1)
		}
		fmt.Println(id)
		os.Exit(0) // no Wait: the starter ends while the tree runs
	case "child":
		gc := exec.Command(os.Args[0])
		gc.Env = append(os.Environ(), helperEnv+"=grandchild")
		if err := gc.Start(); err != nil {
			os.Exit(1)
		}
		f := os.Getenv("HORUS_TREE_PIDS")
		pids := fmt.Sprintf("%d %d", os.Getpid(), gc.Process.Pid)
		if os.WriteFile(f+".tmp", []byte(pids), 0o644) != nil || os.Rename(f+".tmp", f) != nil {
			os.Exit(1)
		}
		time.Sleep(helperLife)
		os.Exit(0)
	case "grandchild":
		time.Sleep(helperLife)
		os.Exit(0)
	case "end":
		if err := EndTree(os.Getenv("HORUS_TREE_ID")); err != nil {
			fmt.Fprintln(os.Stderr, "EndTree:", err)
			os.Exit(1)
		}
		os.Exit(0)
	case "run":
		// The program of TestRunGivesTheExitCodeAndTheOutput.
		wd, _ := os.Getwd()
		fmt.Println("out " + strings.Join(os.Args[1:], " ") + " " + os.Getenv("HORUS_RUN_EXTRA"))
		fmt.Fprintln(os.Stderr, "err "+filepath.Base(wd))
		os.Exit(5)
	}
	os.Exit(3)
}

func helper(role string, env ...string) *exec.Cmd {
	cmd := exec.Command(os.Args[0])
	cmd.Env = append(append(os.Environ(), helperEnv+"="+role), env...)
	return cmd
}

func countAlive(pids []int) int {
	n := 0
	for _, p := range pids {
		if alive(p) {
			n++
		}
	}
	return n
}

// TestTreeOutlivesItsStarterAndEndsFromAnotherProcess is the oracle of the
// goal row process-tree. A starter process starts a child with StartTree, and
// the child starts a grandchild. Check 1: both live after the starter ends.
// Check 2: a second, separate process calls EndTree, and 0 of them live
// after 5 s.
func TestTreeOutlivesItsStarterAndEndsFromAnotherProcess(t *testing.T) {
	pidsFile := filepath.Join(t.TempDir(), "pids.txt")
	starter := helper("starter", "HORUS_TREE_PIDS="+pidsFile)
	ownGroup(starter)
	var stderr strings.Builder
	starter.Stderr = &stderr
	out, err := starter.Output()
	if err != nil {
		t.Fatalf("the starter failed: %v %s", err, stderr.String())
	}
	id := strings.TrimSpace(string(out))
	t.Cleanup(func() { _ = EndTree(id) })

	var pids []int
	deadline := time.Now().Add(30 * time.Second)
	for len(pids) == 0 {
		if b, err := os.ReadFile(pidsFile); err == nil {
			for _, f := range strings.Fields(string(b)) {
				p, _ := strconv.Atoi(f)
				pids = append(pids, p)
			}
		} else if time.Now().After(deadline) {
			t.Fatalf("the child wrote no pids in 30 s: %v", err)
		} else {
			time.Sleep(20 * time.Millisecond)
		}
	}
	t.Cleanup(func() { // leave no process behind when a check fails
		for _, p := range pids {
			if pr, err := os.FindProcess(p); err == nil && alive(p) {
				_ = pr.Kill()
			}
		}
	})
	if len(pids) != 2 {
		t.Fatalf("pids %v, want 2", pids)
	}

	// Check 1. The starter has ended (Output waited for it).
	if groupOfStarterHolds(starter.Process.Pid) {
		t.Fatal("check 1: a process of the tree was in the process group of its starter: the start is not detached")
	}
	if n := countAlive(pids); n != 2 {
		t.Fatalf("check 1: %d of 2 processes alive after the starter ended", n)
	}

	// Check 2.
	end := helper("end", "HORUS_TREE_ID="+id)
	if b, err := end.CombinedOutput(); err != nil {
		t.Fatalf("the end process failed: %v %s", err, b)
	}
	deadline = time.Now().Add(5 * time.Second)
	for countAlive(pids) > 0 && time.Now().Before(deadline) {
		time.Sleep(50 * time.Millisecond)
	}
	if n := countAlive(pids); n != 0 {
		t.Fatalf("check 2: %d of 2 processes alive 5 s after EndTree", n)
	}
}

// A start through a context: the end of the context ends the whole tree,
// not only the root.
func TestTreeEndsWithItsContext(t *testing.T) {
	pidsFile := filepath.Join(t.TempDir(), "pids.txt")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0])
	cmd.Env = append(os.Environ(), helperEnv+"=child", "HORUS_TREE_PIDS="+pidsFile)
	id, err := StartTree(cmd)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = EndTree(id) })
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	var pids []int
	deadline := time.Now().Add(30 * time.Second)
	for len(pids) < 2 && time.Now().Before(deadline) {
		if b, err := os.ReadFile(pidsFile); err == nil {
			for _, f := range strings.Fields(string(b)) {
				p, _ := strconv.Atoi(f)
				pids = append(pids, p)
			}
		} else {
			time.Sleep(20 * time.Millisecond)
		}
	}
	if len(pids) != 2 {
		t.Fatalf("pids %v, want 2", pids)
	}
	cancel()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("Wait did not return 10 s after the cancel")
	}
	deadline = time.Now().Add(5 * time.Second)
	for countAlive(pids) > 0 && time.Now().Before(deadline) {
		time.Sleep(50 * time.Millisecond)
	}
	if n := countAlive(pids); n != 0 {
		t.Fatalf("%d of 2 processes alive 5 s after the cancel", n)
	}
}

func TestEndTreeRefusesAnIDThatIsNotATree(t *testing.T) {
	for _, id := range []string{"", "1", "0", "-5", "abc"} {
		if err := EndTree(id); !errors.Is(err, ErrBadTree) {
			t.Errorf("EndTree(%q) = %v, want ErrBadTree", id, err)
		}
	}
}

// The real free space of a temporary folder can be read on this system.
func TestFreeSpaceOfThisSystem(t *testing.T) {
	n, err := FreeSpace(t.TempDir())
	if err != nil || n == 0 {
		t.Fatalf("FreeSpace: %d, %v", n, err)
	}
}
