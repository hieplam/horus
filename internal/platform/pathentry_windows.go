package platform

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
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

// removeSelf starts a hidden PowerShell that waits until this process ends
// (up to 60 s), then deletes the file self (up to 10 s of tries).
func removeSelf(self string) error {
	ps := filepath.Join(os.Getenv("SystemRoot"), "System32", "WindowsPowerShell", "v1.0", "powershell.exe")
	if os.Getenv("SystemRoot") == "" || !IsFile(ps) {
		p, err := exec.LookPath("powershell.exe")
		if err != nil {
			return fmt.Errorf("powershell.exe was not found, so horus cannot delete %s after its end. Delete it by hand.", self)
		}
		ps = p
	}
	script := fmt.Sprintf("$p=%d; $f='%s'; "+
		"for ($i=0; $i -lt 600; $i++) { if (-not (Get-Process -Id $p -ErrorAction SilentlyContinue)) { break }; Start-Sleep -Milliseconds 100 }; "+
		"for ($i=0; $i -lt 50; $i++) { Remove-Item -LiteralPath $f -Force -ErrorAction SilentlyContinue; if (-not (Test-Path -LiteralPath $f)) { break }; Start-Sleep -Milliseconds 200 }",
		os.Getpid(), strings.ReplaceAll(self, "'", "''"))
	start := func(flags uint32) error {
		cmd := exec.Command(ps, "-NoProfile", "-NonInteractive", "-Command", script)
		cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: flags, HideWindow: true}
		if err := cmd.Start(); err != nil {
			return err
		}
		return cmd.Process.Release()
	}
	// Leave the Job Object of a terminal or a CI runner when it allows it, so
	// the end of the job does not end the PowerShell too early.
	err := start(createNoWindow | createNewProcessGroup | breakawayFromJob)
	if errors.Is(err, errorAccessDenied) {
		err = start(createNoWindow | createNewProcessGroup)
	}
	if err != nil {
		return fmt.Errorf("cannot start powershell.exe to delete %s: %w", self, err)
	}
	return nil
}
