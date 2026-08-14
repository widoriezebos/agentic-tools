package validate

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// One committed repository per test; the gate's whole decision surface runs
// against real git, exactly as the shim invokes it.
type baselineFixture struct {
	t    *testing.T
	repo string
}

func newBaselineFixture(t *testing.T) *baselineFixture {
	t.Helper()
	f := &baselineFixture{t: t, repo: t.TempDir()}
	f.git("init", "-q", "-b", "main")
	os.WriteFile(filepath.Join(f.repo, "README.md"), []byte("fixture\n"), 0o644)
	f.git("add", ".")
	f.commit("first")
	return f
}

func (f *baselineFixture) git(args ...string) string {
	f.t.Helper()
	cmd := exec.Command("git", append([]string{"-C", f.repo}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		f.t.Fatalf("git %v: %v %s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func (f *baselineFixture) commit(message string) {
	f.t.Helper()
	f.git("-c", "user.name=fixture", "-c", "user.email=fixture.invalid",
		"commit", "-q", "--allow-empty", "-m", message)
}

func (f *baselineFixture) run(command string, mutate ...func(*RefactorBaselineParams)) (int, string, string) {
	f.t.Helper()
	p := RefactorBaselineParams{
		Command: command, Cwd: f.repo, File: "plans/refactor-baseline",
		Gate: "fixture gate", MaxAgeMinutes: 1440, MaxCommits: 40,
	}
	for _, m := range mutate {
		m(&p)
	}
	var out, errOut strings.Builder
	code := RefactorBaseline(p, &out, &errOut)
	return code, out.String(), errOut.String()
}

func TestRefactorBaselineRecordAndCheck(t *testing.T) {
	f := newBaselineFixture(t)
	code, out, _ := f.run("record")
	if code != 0 || !strings.Contains(out, "trusted baseline recorded: ") {
		t.Fatalf("record: code=%d out=%q", code, out)
	}
	data, err := os.ReadFile(filepath.Join(f.repo, "plans", "refactor-baseline"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if !strings.HasPrefix(text, "sha=") || !strings.Contains(text, "\nrecorded_epoch=") ||
		!strings.HasSuffix(text, "\ngate=fixture gate\n") {
		t.Fatalf("on-disk format drifted: %q", text)
	}
	// Check right after record: the baseline file's own dirt is tolerated.
	code, out, errText := f.run("check")
	if code != 0 || !strings.Contains(out, "refactor baseline safe: ") {
		t.Fatalf("check after record: code=%d out=%q err=%q", code, out, errText)
	}
	// Foreign dirt blocks.
	os.WriteFile(filepath.Join(f.repo, "stray.txt"), []byte("x\n"), 0o644)
	code, _, errText = f.run("check")
	if code != 1 || !strings.Contains(errText, "dirty beyond the baseline file") {
		t.Fatalf("foreign dirt: code=%d err=%q", code, errText)
	}
	os.Remove(filepath.Join(f.repo, "stray.txt"))
	// A path with a space stays literal in the porcelain comparison.
	spaced := filepath.Join(f.repo, "with space.txt")
	os.WriteFile(spaced, []byte("x\n"), 0o644)
	code, _, _ = f.run("check")
	if code != 1 {
		t.Fatal("spaced foreign dirt did not block")
	}
	os.Remove(spaced)
}

func TestRefactorBaselineRecordRefusals(t *testing.T) {
	f := newBaselineFixture(t)
	code, _, errText := f.run("record", func(p *RefactorBaselineParams) { p.Gate = "" })
	if code != 2 || !strings.Contains(errText, "record requires --gate") {
		t.Fatalf("missing gate: code=%d err=%q", code, errText)
	}
	os.WriteFile(filepath.Join(f.repo, "dirt.txt"), []byte("x\n"), 0o644)
	code, _, errText = f.run("record")
	if code != 1 || !strings.Contains(errText, "worktree is dirty; a baseline must be an exact committed state") {
		t.Fatalf("dirty record: code=%d err=%q", code, errText)
	}
}

func TestRefactorBaselineCheckRefusals(t *testing.T) {
	f := newBaselineFixture(t)
	baseline := filepath.Join(f.repo, "plans", "refactor-baseline")

	code, _, errText := f.run("check")
	if code != 1 || !strings.Contains(errText, "no trusted baseline at plans/refactor-baseline") {
		t.Fatalf("absent baseline: code=%d err=%q", code, errText)
	}

	os.MkdirAll(filepath.Dir(baseline), 0o755)
	os.WriteFile(baseline, []byte("sha=\nrecorded_epoch=+5\n"), 0o644)
	code, _, errText = f.run("check")
	if code != 2 || !strings.Contains(errText, "baseline file is malformed") {
		t.Fatalf("malformed: code=%d err=%q", code, errText)
	}

	os.WriteFile(baseline, []byte(fmt.Sprintf("sha=%040d\nrecorded_epoch=1\ngate=g\n", 0)), 0o644)
	code, _, errText = f.run("check")
	if code != 1 || !strings.Contains(errText, "unknown to this repository") {
		t.Fatalf("unknown sha: code=%d err=%q", code, errText)
	}

	// A known commit that is not an ancestor: a side branch ahead of HEAD.
	head := f.git("rev-parse", "HEAD")
	f.git("checkout", "-q", "-b", "side")
	f.commit("side work")
	side := f.git("rev-parse", "HEAD")
	f.git("checkout", "-q", "main")
	os.WriteFile(baseline, []byte("sha="+side+"\nrecorded_epoch="+
		fmt.Sprint(time.Now().Unix())+"\ngate=g\n"), 0o644)
	code, _, errText = f.run("check")
	if code != 1 || !strings.Contains(errText, "not an ancestor of HEAD") {
		t.Fatalf("diverged: code=%d err=%q", code, errText)
	}

	// Cadence backstops: commits, then age.
	os.WriteFile(baseline, []byte("sha="+head+"\nrecorded_epoch="+
		fmt.Sprint(time.Now().Unix())+"\ngate=g\n"), 0o644)
	f.commit("one more")
	code, _, errText = f.run("check", func(p *RefactorBaselineParams) { p.MaxCommits = 0 })
	if code != 1 || !strings.Contains(errText, "commits since the trusted baseline (max 0)") {
		t.Fatalf("commit backstop: code=%d err=%q", code, errText)
	}
	os.WriteFile(baseline, []byte("sha="+f.git("rev-parse", "HEAD")+
		"\nrecorded_epoch=1\ngate=g\n"), 0o644)
	code, _, errText = f.run("check")
	if code != 1 || !strings.Contains(errText, "minutes old (max 1440)") {
		t.Fatalf("age backstop: code=%d err=%q", code, errText)
	}
}

func TestRefactorBaselineEnvironmentRefusals(t *testing.T) {
	f := newBaselineFixture(t)
	outside := t.TempDir()
	code, _, errText := f.run("check", func(p *RefactorBaselineParams) {
		p.File = filepath.Join(outside, "baseline")
	})
	if code != 2 || !strings.Contains(errText, "must live inside the repository") {
		t.Fatalf("outside file: code=%d err=%q", code, errText)
	}
	notRepo := t.TempDir()
	code, _, errText = f.run("check", func(p *RefactorBaselineParams) { p.Cwd = notRepo })
	if code != 2 || !strings.Contains(errText, "not inside a git repository") {
		t.Fatalf("non-repo: code=%d err=%q", code, errText)
	}
	code, _, _ = f.run("neither")
	if code != 2 {
		t.Fatal("unknown command accepted")
	}
}
