package platform

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
	"unsafe"
)

// envKey is the registry key (under HKEY_CURRENT_USER) of the PATH of the
// user, and pathValue its value. Only pathEntry and removePathEntry name
// them. A test uses its own scratch key with the functions *At.
const (
	envKey    = `Environment`
	pathValue = "Path"
)

const (
	regSZ        = 1 // REG_SZ
	regExpandSZ  = 2 // REG_EXPAND_SZ
	keySetValue  = 0x0002
	hwndBroad    = 0xffff
	wmSetting    = 0x001A
	smtoAbortHng = 0x0002

	breakawayFromJob  = 0x01000000
	errorAccessDenied = syscall.Errno(5)
)

var (
	advapi32                = syscall.NewLazyDLL("advapi32.dll")
	procRegSetValueExW      = advapi32.NewProc("RegSetValueExW")
	user32                  = syscall.NewLazyDLL("user32.dll")
	procSendMessageTimeoutW = user32.NewProc("SendMessageTimeoutW")
)

// regRead reads the string value name of the key subkey under
// HKEY_CURRENT_USER, as it is (REG_EXPAND_SZ is not expanded). found is false
// when the key or the value does not exist.
func regRead(subkey, name string) (value string, typ uint32, found bool, err error) {
	var h syscall.Handle
	k, err := syscall.UTF16PtrFromString(subkey)
	if err != nil {
		return "", 0, false, err
	}
	if err := syscall.RegOpenKeyEx(syscall.HKEY_CURRENT_USER, k, 0, syscall.KEY_READ, &h); err != nil {
		if errors.Is(err, syscall.ERROR_FILE_NOT_FOUND) {
			return "", 0, false, nil
		}
		return "", 0, false, err
	}
	defer syscall.RegCloseKey(h)
	n, err := syscall.UTF16PtrFromString(name)
	if err != nil {
		return "", 0, false, err
	}
	var size uint32
	if err := syscall.RegQueryValueEx(h, n, nil, &typ, nil, &size); err != nil {
		if errors.Is(err, syscall.ERROR_FILE_NOT_FOUND) {
			return "", 0, false, nil
		}
		return "", 0, false, err
	}
	if typ != regSZ && typ != regExpandSZ {
		return "", typ, true, fmt.Errorf("HKCU\\%s\\%s is not a string value (type %d)", subkey, name, typ)
	}
	if size == 0 {
		return "", typ, true, nil
	}
	buf := make([]uint16, size/2+1)
	if err := syscall.RegQueryValueEx(h, n, nil, &typ, (*byte)(unsafe.Pointer(&buf[0])), &size); err != nil {
		return "", 0, false, err
	}
	return syscall.UTF16ToString(buf[:size/2]), typ, true, nil
}

// regWrite writes the string value name of the key subkey under
// HKEY_CURRENT_USER, with the type typ (REG_SZ or REG_EXPAND_SZ). The key
// must exist.
func regWrite(subkey, name, value string, typ uint32) error {
	var h syscall.Handle
	k, err := syscall.UTF16PtrFromString(subkey)
	if err != nil {
		return err
	}
	if err := syscall.RegOpenKeyEx(syscall.HKEY_CURRENT_USER, k, 0, keySetValue, &h); err != nil {
		return err
	}
	defer syscall.RegCloseKey(h)
	n, err := syscall.UTF16PtrFromString(name)
	if err != nil {
		return err
	}
	data, err := syscall.UTF16FromString(value)
	if err != nil {
		return err
	}
	r, _, _ := procRegSetValueExW.Call(uintptr(h), uintptr(unsafe.Pointer(n)), 0, uintptr(typ),
		uintptr(unsafe.Pointer(&data[0])), uintptr(len(data)*2))
	if r != 0 {
		return syscall.Errno(r)
	}
	return nil
}

// hasEntryAt reports if the value Path of the key subkey has the folder dir
// as an entry.
func hasEntryAt(subkey, dir string) (bool, error) {
	v, _, found, err := regRead(subkey, pathValue)
	if err != nil || !found {
		return false, err
	}
	return HasListEntry(v, dir), nil
}

// removeEntryAt removes the entry dir from the value Path of the key subkey,
// and keeps the type of the value. It writes nothing when the value has no
// such entry. changed reports a write.
func removeEntryAt(subkey, dir string) (changed bool, err error) {
	v, typ, found, err := regRead(subkey, pathValue)
	if err != nil || !found {
		return false, err
	}
	out, changed := RemoveListEntry(v, dir)
	if !changed {
		return false, nil
	}
	return true, regWrite(subkey, pathValue, out, typ)
}

func pathEntry(program, _ string) ([]string, error) {
	dir := filepath.Dir(program)
	if !onlyHorusFiles(dir) {
		return nil, nil
	}
	ok, err := hasEntryAt(envKey, dir)
	if err != nil || !ok {
		return nil, err
	}
	return []string{`HKCU\Environment\Path`}, nil
}

func removePathEntry(program, home string) error {
	where, err := pathEntry(program, home)
	if err != nil || len(where) == 0 {
		return err
	}
	changed, err := removeEntryAt(envKey, filepath.Dir(program))
	if err != nil || !changed {
		return err
	}
	// Tell the open programs (Explorer) that the environment changed, as
	// [Environment]::SetEnvironmentVariable does. A new terminal then gets
	// the new PATH.
	env, _ := syscall.UTF16PtrFromString("Environment")
	var result uintptr
	procSendMessageTimeoutW.Call(hwndBroad, wmSetting, 0, uintptr(unsafe.Pointer(env)), smtoAbortHng, 5000,
		uintptr(unsafe.Pointer(&result)))
	return nil
}

// removeSelfReady is how long removeSelf waits for the line "ready" of the
// PowerShell. On a busy machine, powershell.exe can take 10 s and more to
// start.
const removeSelfReady = 60 * time.Second

// removeSelf starts a hidden PowerShell and returns when the PowerShell is
// ready: it holds a handle of this process and wrote the line "ready". So
// the start of PowerShell is over before this process ends. When this
// process ends (up to 60 s), the PowerShell deletes the file self (up to
// 10 s of tries). After "ready", the script uses only .NET calls. A cmdlet
// such as Get-Process loads its module on the first call, and that load can
// take a second on a busy machine.
func removeSelf(self string) error {
	ps := filepath.Join(os.Getenv("SystemRoot"), "System32", "WindowsPowerShell", "v1.0", "powershell.exe")
	if os.Getenv("SystemRoot") == "" || !IsFile(ps) {
		p, err := exec.LookPath("powershell.exe")
		if err != nil {
			return fmt.Errorf("powershell.exe was not found, so horus cannot delete %s after its end. Delete it by hand.", self)
		}
		ps = p
	}
	script := fmt.Sprintf("$p=[Diagnostics.Process]::GetProcessById(%d); $f='%s'; $null=[IO.File]::Exists($f); "+
		"[Console]::Out.WriteLine('ready'); [Console]::Out.Flush(); "+
		"$null=$p.WaitForExit(60000); "+
		"for ($i=0; $i -lt 50; $i++) { try { [IO.File]::Delete($f) } catch {}; if (-not [IO.File]::Exists($f)) { break }; [Threading.Thread]::Sleep(200) }",
		os.Getpid(), strings.ReplaceAll(self, "'", "''"))
	start := func(flags uint32) (*exec.Cmd, io.ReadCloser, error) {
		cmd := exec.Command(ps, "-NoProfile", "-NonInteractive", "-Command", script)
		cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: flags, HideWindow: true}
		out, err := cmd.StdoutPipe()
		if err != nil {
			return nil, nil, err
		}
		if err := cmd.Start(); err != nil {
			return nil, nil, err
		}
		return cmd, out, nil
	}
	// Leave the Job Object of a terminal or a CI runner when it allows it, so
	// the end of the job does not end the PowerShell too early.
	cmd, out, err := start(createNoWindow | createNewProcessGroup | breakawayFromJob)
	if errors.Is(err, errorAccessDenied) {
		cmd, out, err = start(createNoWindow | createNewProcessGroup)
	}
	if err != nil {
		return fmt.Errorf("cannot start powershell.exe to delete %s: %w", self, err)
	}
	defer out.Close()
	line := make(chan string, 1)
	go func() {
		s, _ := bufio.NewReader(out).ReadString('\n')
		line <- strings.TrimSpace(s)
	}()
	select {
	case s := <-line:
		if s != "ready" {
			_ = cmd.Process.Kill()
			return fmt.Errorf("powershell.exe ended before it was ready to delete %s (it wrote %q). Delete it by hand.", self, s)
		}
	case <-time.After(removeSelfReady):
		_ = cmd.Process.Kill()
		return fmt.Errorf("powershell.exe was not ready in %s, so horus cannot delete %s after its end. Delete it by hand.", removeSelfReady, self)
	}
	return cmd.Process.Release()
}
