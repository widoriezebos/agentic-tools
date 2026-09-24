package gittree

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestHistorySteeringFilesFindsPhysicalGraftFile(t *testing.T) {
	root := t.TempDir()
	run := func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	run("init", "-q")
	w := Workspace{Dir: root}
	before, err := w.HistorySteeringFiles()
	if err != nil || len(before) != 0 {
		t.Fatalf("before graft: files %v, error %v", before, err)
	}
	common := run("rev-parse", "--git-common-dir")
	if !filepath.IsAbs(common) {
		common = filepath.Join(root, common)
	}
	if err := os.MkdirAll(filepath.Join(common, "info"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(common, "info", "grafts"), []byte("present\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	after, err := w.HistorySteeringFiles()
	if err != nil || len(after) != 1 || after[0] != filepath.Join("info", "grafts") {
		t.Fatalf("after graft: files %v, error %v", after, err)
	}
}
