package stopreport

import (
	"os"
	"path/filepath"
	"testing"
)

// EM-44: reading a Stop report where none was ever recorded said "stop
// report directory must not contain symlinks": the directory was missing,
// not redirected. A missing directory says no report is recorded; a real
// redirection names the directory and the link.
func TestStopReportDirectoryRefusalsNameTheCause(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	dir := filepath.Join(root, "artifacts", "agents", "supervision", "stop-verdicts")
	_, err := Resolve(root, "7f3a")
	if want := "no Stop report has been recorded in this installation yet (" + dir + " does not exist); nothing was read"; err == nil || err.Error() != want {
		t.Fatalf("no report directory = %v, want %q", err, want)
	}

	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	_, err = Resolve(root, "7f3a")
	if want := "no Stop report 7f3a is recorded in this installation; nothing was read"; err == nil || err.Error() != want {
		t.Fatalf("no alias directory = %v, want %q", err, want)
	}

	if err := os.MkdirAll(filepath.Join(dir, "aliases"), 0o755); err != nil {
		t.Fatal(err)
	}
	_, err = Resolve(root, "7f3a")
	if want := "no Stop report 7f3a is recorded in this installation; nothing was read"; err == nil || err.Error() != want {
		t.Fatalf("no alias = %v, want %q", err, want)
	}

	redirected := t.TempDir()
	elsewhere := t.TempDir()
	if err := os.MkdirAll(filepath.Join(elsewhere, "agents", "supervision", "stop-verdicts"), 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(redirected, "artifacts")
	if err := os.Symlink(elsewhere, link); err != nil {
		t.Fatal(err)
	}
	_, err = ReportDir(redirected)
	redirectedDir := filepath.Join(redirected, "artifacts", "agents", "supervision", "stop-verdicts")
	if want := "the Stop report directory " + redirectedDir + " passes through a symlink at " + link + ", which is refused"; err == nil || err.Error() != want {
		t.Fatalf("redirected report directory = %v, want %q", err, want)
	}
}
