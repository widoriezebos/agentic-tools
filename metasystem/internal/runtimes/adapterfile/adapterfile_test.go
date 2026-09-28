package adapterfile

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
)

func writeAdapter(t *testing.T, root, name string, mode os.FileMode) string {
	t.Helper()
	dir := filepath.Join(root, "adapters")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, name)
	if err := testexec.WriteFile(path, []byte("#!/bin/sh\nexit 0\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	// Chmod explicitly so the umask cannot mask group/world bits.
	if err := os.Chmod(path, mode); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestUseKeyAndUnnamedNameTheConfigurationFix(t *testing.T) {
	t.Parallel()
	if got := UseKey("acme"); got != "adapters.acme.use" {
		t.Fatalf("UseKey = %q", got)
	}
	msg := Unnamed("acme")
	if !strings.Contains(msg, "adapters.acme.use=external") || !strings.Contains(msg, "metasystem.conf") {
		t.Fatalf("Unnamed does not name the fix: %q", msg)
	}
}

func TestValidName(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"a", "acme", "acme-2", "x" + strings.Repeat("y", 31)} {
		if !ValidName(name) {
			t.Fatalf("ValidName(%q) = false", name)
		}
	}
	for _, name := range []string{"", "2acme", "Acme", "-acme", "acme_x", "acme.sh", "a/b", "x" + strings.Repeat("y", 32)} {
		if ValidName(name) {
			t.Fatalf("ValidName(%q) = true", name)
		}
	}
}

func TestPathIsUnderAdapterDir(t *testing.T) {
	t.Parallel()
	if got := Path("/inst", "acme"); got != filepath.Join("/inst", "adapters", "acme") {
		t.Fatalf("Path = %q", got)
	}
}

// A trusted adapter (regular, executable, owner-only writable, owned by us)
// passes with an empty reason.
func TestCheckTrustedFile(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeAdapter(t, root, "acme", 0o755)
	reason, err := Check(root, "acme")
	if err != nil || reason != "" {
		t.Fatalf("Check trusted = %q, %v", reason, err)
	}
}

// Missing, non-executable and non-regular files are absent, not refused.
func TestCheckAbsent(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if _, err := Check(root, "missing"); !errors.Is(err, ErrAbsent) {
		t.Fatalf("missing file err = %v", err)
	}
	writeAdapter(t, root, "plain", 0o644)
	if _, err := Check(root, "plain"); !errors.Is(err, ErrAbsent) {
		t.Fatalf("non-executable file err = %v", err)
	}
	if err := os.MkdirAll(filepath.Join(root, "adapters", "dir"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := Check(root, "dir"); !errors.Is(err, ErrAbsent) {
		t.Fatalf("directory err = %v", err)
	}
}

// An executable whose name breaks the grammar is refused with a rename fix.
func TestCheckRefusesInvalidName(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeAdapter(t, root, "Bad_Name", 0o755)
	reason, err := Check(root, "Bad_Name")
	if err != nil || !strings.Contains(reason, `"Bad_Name" is not a runtime name`) || !strings.Contains(reason, "rename the file") {
		t.Fatalf("Check invalid name = %q, %v", reason, err)
	}
}

// Group- or world-writable adapters are refused with the chmod fix.
func TestCheckRefusesWritableByOthers(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	for name, mode := range map[string]os.FileMode{"groupw": 0o775, "worldw": 0o757} {
		path := writeAdapter(t, root, name, mode)
		reason, err := Check(root, name)
		if err != nil || !strings.Contains(reason, "group- or world-writable") || !strings.Contains(reason, "chmod go-w "+path) {
			t.Fatalf("Check(%s) = %q, %v", name, reason, err)
		}
	}
}

// fakeInfo is a FileInfo whose Sys is a stat record we control, so the
// ownership rule is proven without chown privileges.
type fakeInfo struct {
	mode os.FileMode
	sys  any
}

func (f fakeInfo) Name() string       { return "acme" }
func (f fakeInfo) Size() int64        { return 0 }
func (f fakeInfo) Mode() os.FileMode  { return f.mode }
func (f fakeInfo) ModTime() time.Time { return time.Time{} }
func (f fakeInfo) IsDir() bool        { return false }
func (f fakeInfo) Sys() any           { return f.sys }

func TestUnsafeRefusesForeignOwner(t *testing.T) {
	t.Parallel()
	info := foreignOwnedInfo(t, 0o755, uint32(os.Getuid()+1))
	reason := Unsafe("/inst/adapters/acme", info)
	if !strings.Contains(reason, "not owned by the installation's user") || !strings.Contains(reason, "/inst/adapters/acme") {
		t.Fatalf("Unsafe foreign owner = %q", reason)
	}
	// Writability is reported before ownership.
	if reason := Unsafe("/p", foreignOwnedInfo(t, 0o777, uint32(os.Getuid()+1))); !strings.Contains(reason, "chmod go-w /p") {
		t.Fatalf("Unsafe writable+foreign = %q", reason)
	}
	// Our own uid is trusted.
	if reason := Unsafe("/p", foreignOwnedInfo(t, 0o755, uint32(os.Getuid()))); reason != "" {
		t.Fatalf("Unsafe own uid = %q", reason)
	}
	// Without a stat record ownership cannot be judged, so only the mode rule applies.
	if reason := Unsafe("/p", fakeInfo{mode: 0o755}); reason != "" {
		t.Fatalf("Unsafe without stat = %q", reason)
	}
}

func foreignOwnedInfo(t *testing.T, mode os.FileMode, uid uint32) os.FileInfo {
	t.Helper()
	return fakeInfo{mode: mode, sys: &syscall.Stat_t{Uid: uid}}
}
