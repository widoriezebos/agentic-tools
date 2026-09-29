package identity

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// ReadProcessUse sees this process's open file, its cwd and its executable.
func TestReadProcessUseSeesAnOpenFile(t *testing.T) {
	t.Parallel()
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "held.txt")
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	use, err := ReadProcessUse(int64(os.Getpid()))
	if err != nil {
		t.Fatal(err)
	}
	var resolved []string
	for _, open := range use.Files {
		if real, err := filepath.EvalSymlinks(open); err == nil {
			resolved = append(resolved, real)
		}
	}
	if !slices.Contains(resolved, path) {
		t.Fatalf("open files %v do not include %s", use.Files, path)
	}
	if use.Cwd == "" || use.Executable == "" {
		t.Fatalf("use = %+v", use)
	}
	if uid, ok := ProcessUID(int64(os.Getpid())); !ok || uid != uint32(os.Getuid()) {
		t.Fatalf("uid = %d, %v", uid, ok)
	}
}
