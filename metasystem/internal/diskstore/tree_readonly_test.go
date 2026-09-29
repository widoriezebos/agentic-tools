package diskstore

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

// readOnlyModuleTree builds what Go leaves in a module cache: directories
// 0555 and files 0444, nested, with a symlink inside pointing at a
// read-only directory outside the item. It returns the outside directory.
func readOnlyModuleTree(t *testing.T, item, outside string) {
	t.Helper()
	module := filepath.Join(item, "go", "pkg", "mod", "github.com", "pelletier", "go-toml", "v2@v2.4.3")
	template := filepath.Join(module, ".github", "ISSUE_TEMPLATE")
	for _, dir := range []string{template, outside} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for _, file := range []string{filepath.Join(template, "bug.md"), filepath.Join(module, "go.mod"), filepath.Join(outside, "keep")} {
		if err := os.WriteFile(file, []byte("module data"), 0o444); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink(outside, filepath.Join(module, "escape")); err != nil {
		t.Fatal(err)
	}
	var dirs []string
	if err := filepath.WalkDir(filepath.Join(item, "go"), func(path string, entry fs.DirEntry, err error) error {
		if err == nil && entry.IsDir() {
			dirs = append(dirs, path)
		}
		return err
	}); err != nil {
		t.Fatal(err)
	}
	for index := len(dirs) - 1; index >= 0; index-- {
		if err := os.Chmod(dirs[index], 0o555); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Chmod(outside, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = os.Chmod(outside, 0o755)
		_ = filepath.WalkDir(item, func(path string, entry fs.DirEntry, err error) error {
			if err == nil && entry.IsDir() {
				_ = os.Chmod(path, 0o755)
			}
			return nil
		})
	})
}

// A tree Go made read-only is removed: RemoveTree adds owner write to each
// directory inside the item before removing its entries. A symlink inside
// is removed as a link: the read-only directory it points at, outside the
// item, keeps its mode and its content (mutation: follow symlinks).
func TestRemoveTreeRemovesAReadOnlyModuleCache(t *testing.T) {
	t.Parallel()
	dir := realDir(t)
	item := filepath.Join(dir, "steward-test-home-1")
	outside := filepath.Join(dir, "outside")
	readOnlyModuleTree(t, item, outside)
	if err := RemoveTree(context.Background(), item); err != nil {
		t.Fatalf("remove a read-only module tree = %v", err)
	}
	if _, err := os.Lstat(item); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("the item survived: %v", err)
	}
	info, err := os.Lstat(outside)
	if err != nil || info.Mode().Perm() != 0o555 {
		t.Fatalf("the directory outside the item was changed: %v %v", info, err)
	}
	if data, err := os.ReadFile(filepath.Join(outside, "keep")); err != nil || string(data) != "module data" {
		t.Fatalf("the content outside the item was touched: %q %v", data, err)
	}
}

// --strays removes a stray holding a read-only module cache, and a removal
// that still stops names a remedy that is true: a permission the engine
// cannot lift is not settled by running the command again.
func TestStraysRemoveAReadOnlyTreeAndNameATruthfulRemedy(t *testing.T) {
	t.Parallel()
	root := realDir(t)
	temp := filepath.Join(root, "tmp")
	item := filepath.Join(temp, "steward-test-home-528179668")
	readOnlyModuleTree(t, item, filepath.Join(root, "outside"))
	// The symlink inside keeps its own time, so the act runs three days on.
	later := time.Now().Add(72 * time.Hour)
	options := passOptions(Registry{Dir: filepath.Join(root, "stores")}, root, TempStrays{Roots: []string{temp}})
	options.Mode, options.Now = ModePreview, later
	report, err := RunPass(context.Background(), options)
	if err != nil {
		t.Fatal(err)
	}
	plan, err := ReadPlan(options.PlanDir, report.Plan)
	if err != nil {
		t.Fatal(err)
	}
	outcomes := ExecuteStrays(context.Background(), plan, later, &UseCensus{Taken: true}, []string{temp})
	if len(outcomes) != 1 || !outcomes[0].Done || outcomes[0].Reason != "removed" || outcomes[0].Bytes == 0 {
		t.Fatalf("the read-only stray = %+v", outcomes)
	}
	denied := &fs.PathError{Op: "remove", Path: item + "/x", Err: syscall.EACCES}
	reason, command := removalStopped(item, denied)
	if strings.Contains(command, "again") || !strings.Contains(reason, "permission") || !strings.Contains(command, item) {
		t.Fatalf("a permission the engine cannot lift = %q; run %q", reason, command)
	}
	reason, command = removalStopped(item, context.DeadlineExceeded)
	if !strings.Contains(command, "--strays again") || !strings.Contains(reason, "removal stopped") {
		t.Fatalf("a cut-short removal = %q; run %q", reason, command)
	}
}
