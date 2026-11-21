package humanauthority

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoginExecutableObservation(t *testing.T) {
	t.Parallel()
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	comm := " sshd-session "
	if err := os.WriteFile(filepath.Join(dir, "comm"), []byte(comm+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if path, known, denied, name := observeExecutableAt(dir); known || denied || path != "" || name != comm {
		t.Fatalf("absent link or inexact name: %q %v %v %q", path, known, denied, name)
	}
	target := filepath.Join(dir, "image")
	if err := os.Symlink(target, filepath.Join(dir, "exe")); err != nil {
		t.Fatal(err)
	}
	if path, known, denied, name := observeExecutableAt(dir); path != target || !known || denied || name != comm {
		t.Fatalf("readable link: %q %v %v %q", path, known, denied, name)
	}
	if os.Geteuid() == 0 {
		return
	}
	if err := os.Chmod(dir, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chmod(dir, 0700); err != nil {
			t.Error(err)
		}
	})
	if path, known, denied, name := observeExecutableAt(dir); path != "" || known || !denied || name != "" {
		t.Fatalf("denied link: %q %v %v %q", path, known, denied, name)
	}
}
