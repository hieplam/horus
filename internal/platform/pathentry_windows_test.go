package platform

import (
	"crypto/rand"
	"encoding/hex"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"unsafe"
)

var (
	procRegCreateKeyExW = advapi32.NewProc("RegCreateKeyExW")
	procRegDeleteKeyW   = advapi32.NewProc("RegDeleteKeyW")
)

// scratchKey makes the key HKCU\Software\horus-test-<random> and deletes it
// at the end of the test. The test never opens HKCU\Environment.
func scratchKey(t *testing.T) string {
	t.Helper()
	b := make([]byte, 8)
	rand.Read(b)
	sub := `Software\horus-test-` + hex.EncodeToString(b)
	if sub == envKey {
		t.Fatal("the scratch key is the real key")
	}
	p, _ := syscall.UTF16PtrFromString(sub)
	var h syscall.Handle
	var disp uint32
	r, _, _ := procRegCreateKeyExW.Call(uintptr(syscall.HKEY_CURRENT_USER), uintptr(unsafe.Pointer(p)), 0, 0, 0,
		uintptr(syscall.KEY_READ|keySetValue), 0, uintptr(unsafe.Pointer(&h)), uintptr(unsafe.Pointer(&disp)))
	if r != 0 {
		t.Fatalf("RegCreateKeyEx: %v", syscall.Errno(r))
	}
	syscall.RegCloseKey(h)
	t.Cleanup(func() { procRegDeleteKeyW.Call(uintptr(syscall.HKEY_CURRENT_USER), uintptr(unsafe.Pointer(p))) })
	return sub
}

// osPathEntryCase: removeEntryAt on a scratch key removes the one entry, keeps
// the type REG_EXPAND_SZ and every other entry byte for byte, and writes
// nothing the second time.
func osPathEntryCase(t *testing.T) {
	key := scratchKey(t)
	dir := filepath.Join(t.TempDir(), "Programs", "horus")
	value := `%USERPROFILE%\go\bin;C:\Tools;` + dir + `;D:\x y`
	if err := regWrite(key, pathValue, value, regExpandSZ); err != nil {
		t.Fatal(err)
	}
	if ok, err := hasEntryAt(key, dir); err != nil || !ok {
		t.Fatalf("hasEntryAt = %v, %v", ok, err)
	}
	changed, err := removeEntryAt(key, dir)
	if err != nil || !changed {
		t.Fatalf("removeEntryAt = %v, %v", changed, err)
	}
	got, typ, found, err := regRead(key, pathValue)
	if err != nil || !found {
		t.Fatalf("regRead: %v %v", found, err)
	}
	if want := `%USERPROFILE%\go\bin;C:\Tools;D:\x y`; got != want || typ != regExpandSZ {
		t.Errorf("value = %q type %d, want %q type %d", got, typ, want, regExpandSZ)
	}
	if changed, err := removeEntryAt(key, dir); err != nil || changed {
		t.Errorf("a second removal wrote: %v %v", changed, err)
	}
}

// makeLink makes the NTFS junction link to the folder target, as the test
// installs of the owner do. A junction needs no administrator.
func makeLink(t *testing.T, link, target string) {
	t.Helper()
	cmdExe := os.Getenv("ComSpec")
	if cmdExe == "" {
		cmdExe = "cmd.exe"
	}
	if out, err := exec.Command(cmdExe, "/c", "mklink", "/J", link, target).CombinedOutput(); err != nil {
		t.Fatalf("mklink /J: %v %s", err, out)
	}
}
