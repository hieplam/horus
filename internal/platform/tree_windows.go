// The process tree on Windows: a named Job Object. A child of a process in
// the job is in the job too, so the job holds the whole tree.
//
// The job has no JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE flag, so the tree lives
// after its starter ends. Windows removes the name of a job when its last
// handle closes, also while processes of the job still run. So StartTree
// copies a handle of the job into the root process. The name lives as long
// as the root lives, and EndTree can open the job by its name.
//
// The root starts suspended. StartTree puts it in the job, then resumes it.
// So the root cannot start a child before it is in the job.
package platform

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"syscall"
	"unsafe"
)

const (
	createSuspended       = 0x00000004
	createNewProcessGroup = 0x00000200
	createNoWindow        = 0x08000000

	processTerminate  = 0x0001
	processDupHandle  = 0x0040
	processSetQuota   = 0x0100
	jobTerminate      = 0x0008
	threadSuspendRes  = 0x0002
	th32csSnapThread  = 0x00000004
	errorAlreadyExist = syscall.Errno(183)
	errorFileNotFound = syscall.Errno(2)

	// treePrefix starts the name of each job: the session namespace, so
	// another horus process of the same user session can open it.
	treePrefix = `Local\horus-tree-`
)

var (
	kernel32                 = syscall.NewLazyDLL("kernel32.dll")
	procCreateJobObject      = kernel32.NewProc("CreateJobObjectW")
	procOpenJobObject        = kernel32.NewProc("OpenJobObjectW")
	procAssignProcessToJob   = kernel32.NewProc("AssignProcessToJobObject")
	procTerminateJobObject   = kernel32.NewProc("TerminateJobObject")
	procCreateToolhelp32Snap = kernel32.NewProc("CreateToolhelp32Snapshot")
	procThread32First        = kernel32.NewProc("Thread32First")
	procThread32Next         = kernel32.NewProc("Thread32Next")
	procOpenThread           = kernel32.NewProc("OpenThread")
	procResumeThread         = kernel32.NewProc("ResumeThread")
)

// threadEntry32 is THREADENTRY32 of the Windows API.
type threadEntry32 struct {
	size     uint32
	usage    uint32
	threadID uint32
	ownerPID uint32
	basePri  int32
	deltaPri int32
	flags    uint32
}

func startTree(cmd *exec.Cmd) (string, error) {
	suffix := make([]byte, 8)
	if _, err := rand.Read(suffix); err != nil {
		return "", err
	}
	name := treePrefix + hex.EncodeToString(suffix)
	job, err := createJob(name)
	if err != nil {
		return "", fmt.Errorf("cannot make the Job Object: %w", err)
	}
	defer syscall.CloseHandle(job)

	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.CreationFlags |= createSuspended | createNewProcessGroup | createNoWindow
	cmd.SysProcAttr.HideWindow = true
	if cmd.Cancel != nil { // only exec.CommandContext sets Cancel
		cmd.Cancel = func() error {
			err := endTree(name)
			_ = cmd.Process.Kill() // the root, in case it is not in the job yet
			return err
		}
	}
	if err := cmd.Start(); err != nil {
		return "", err
	}
	if err := adopt(job, uint32(cmd.Process.Pid)); err != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		return "", fmt.Errorf("cannot put process %d in the Job Object: %w", cmd.Process.Pid, err)
	}
	return name, nil
}

// adopt puts the suspended process pid in the job, gives it a handle of the
// job, then resumes its threads.
func adopt(job syscall.Handle, pid uint32) error {
	p, err := syscall.OpenProcess(processSetQuota|processTerminate|processDupHandle, false, pid)
	if err != nil {
		return err
	}
	defer syscall.CloseHandle(p)
	if r, _, e := procAssignProcessToJob.Call(uintptr(job), uintptr(p)); r == 0 {
		return e
	}
	self, err := syscall.GetCurrentProcess()
	if err != nil {
		return err
	}
	var copied syscall.Handle // a handle in the root; the root never uses it
	if err := syscall.DuplicateHandle(self, job, p, &copied, 0, false, syscall.DUPLICATE_SAME_ACCESS); err != nil {
		return err
	}
	return resume(pid)
}

// resume resumes every thread of the process pid. A process that starts
// suspended has one thread.
func resume(pid uint32) error {
	snap, _, e := procCreateToolhelp32Snap.Call(th32csSnapThread, 0)
	if syscall.Handle(snap) == syscall.InvalidHandle {
		return e
	}
	defer syscall.CloseHandle(syscall.Handle(snap))
	te := threadEntry32{size: uint32(unsafe.Sizeof(threadEntry32{}))}
	resumed := 0
	r, _, e := procThread32First.Call(snap, uintptr(unsafe.Pointer(&te)))
	for r != 0 {
		if te.ownerPID == pid {
			t, _, e := procOpenThread.Call(threadSuspendRes, 0, uintptr(te.threadID))
			if t == 0 {
				return e
			}
			n, _, e := procResumeThread.Call(t)
			syscall.CloseHandle(syscall.Handle(t))
			if int32(n) == -1 {
				return e
			}
			resumed++
		}
		r, _, e = procThread32Next.Call(snap, uintptr(unsafe.Pointer(&te)))
	}
	if resumed == 0 {
		return fmt.Errorf("no thread of process %d was found: %v", pid, e)
	}
	return nil
}

func createJob(name string) (syscall.Handle, error) {
	n, err := syscall.UTF16PtrFromString(name)
	if err != nil {
		return 0, err
	}
	h, _, e := procCreateJobObject.Call(0, uintptr(unsafe.Pointer(n)))
	if h == 0 {
		return 0, e
	}
	if errors.Is(e, errorAlreadyExist) {
		syscall.CloseHandle(syscall.Handle(h))
		return 0, fmt.Errorf("a Job Object with the name %s exists", name)
	}
	return syscall.Handle(h), nil
}

func endTree(id string) error {
	if !strings.HasPrefix(id, treePrefix) {
		return ErrBadTree
	}
	n, err := syscall.UTF16PtrFromString(id)
	if err != nil {
		return ErrBadTree
	}
	h, _, e := procOpenJobObject.Call(jobTerminate, 0, uintptr(unsafe.Pointer(n)))
	if h == 0 {
		if errors.Is(e, errorFileNotFound) {
			return nil // no process of the tree is left
		}
		return fmt.Errorf("cannot open the Job Object %s: %w", id, e)
	}
	defer syscall.CloseHandle(syscall.Handle(h))
	if r, _, e := procTerminateJobObject.Call(h, 1); r == 0 {
		return fmt.Errorf("cannot end the Job Object %s: %w", id, e)
	}
	return nil
}

func noConsole(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.CreationFlags |= createNoWindow
}
