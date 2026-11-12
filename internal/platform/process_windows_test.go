package platform

import "os/exec"

// ownGroup does nothing on Windows: the starter has no process group to
// check. Check 1 still proves that the tree lives after its starter ends,
// and that the name of the job lives after the starter closed its handle.
func ownGroup(*exec.Cmd) {}

func groupOfStarterHolds(int) bool { return false }
