package report

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func frontierRepo(t *testing.T) string {
	t.Helper()
	repo := t.TempDir()
	for _, args := range [][]string{
		{"init", "-q"}, {"config", "user.name", "m"}, {"config", "user.email", "m@x"},
		{"commit", "-q", "--allow-empty", "-m", "init"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = repo
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
	}
	return repo
}

// commitAll commits the frontier checkpoint so the next record sees a clean
// worktree, exactly as the workflow prescribes.
func commitAll(t *testing.T, repo string) {
	t.Helper()
	for _, args := range [][]string{{"add", "-A"}, {"commit", "-qm", "checkpoint"}} {
		cmd := exec.Command("git", args...)
		cmd.Dir = repo
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
	}
}

func noEnv(string) string { return "" }

func TestFrontierRecordAndChallenge(t *testing.T) {
	repo := frontierRepo(t)
	file := filepath.Join(repo, "plans", "frontier")
	opts := FrontierOptions{File: file, Repo: repo, Env: noEnv,
		Score: "80", MinDelta: "1", Eval: "declared eval"}
	lines, ferr := FrontierRecord(opts)
	if ferr != nil || len(lines) != 2 || !strings.HasPrefix(lines[0], "frontier recorded: score 80 at ") {
		t.Fatalf("record failed: %v %v", ferr, lines)
	}
	cases := []struct {
		score string
		code  int
		want  string
	}{
		{"79", 1, "does not beat frontier"},
		{"80.5", 1, "within noise floor"},
		{"82", 0, "new frontier"},
	}
	for _, tc := range cases {
		lines, ferr := FrontierChallenge(FrontierOptions{File: file, Env: noEnv, Score: tc.score})
		if tc.code == 0 {
			if ferr != nil || !strings.Contains(lines[0], tc.want) {
				t.Fatalf("challenge %s: %v %v", tc.score, ferr, lines)
			}
			continue
		}
		if ferr == nil || ferr.Code != tc.code || !strings.Contains(ferr.Message, tc.want) {
			t.Fatalf("challenge %s: %v", tc.score, ferr)
		}
	}
	// The stored floor holds when the flag is forgotten; an explicit zero
	// overrides it.
	if _, ferr := FrontierChallenge(FrontierOptions{File: file, Env: noEnv, Score: "80.5", MinDelta: "0"}); ferr != nil {
		t.Fatalf("explicit zero floor refused: %v", ferr)
	}
	// A regression without --force refuses; with --force it re-baselines.
	commitAll(t, repo)
	if _, ferr := FrontierRecord(FrontierOptions{File: file, Repo: repo, Env: noEnv, Score: "75", Eval: "e"}); ferr == nil || ferr.Code != 1 {
		t.Fatalf("regression accepted: %v", ferr)
	}
	if _, ferr := FrontierRecord(FrontierOptions{File: file, Repo: repo, Env: noEnv, Score: "75", Eval: "e", Force: true}); ferr != nil {
		t.Fatalf("force re-baseline refused: %v", ferr)
	}
	// A direction change without --force refuses.
	commitAll(t, repo)
	if _, ferr := FrontierRecord(FrontierOptions{File: file, Repo: repo, Env: noEnv, Score: "70", Eval: "e", Direction: "min"}); ferr == nil ||
		!strings.Contains(ferr.Message, "direction min differs") {
		t.Fatalf("direction change accepted: %v", ferr)
	}
}

func TestFrontierWindowAndMalformed(t *testing.T) {
	dir := t.TempDir()
	write := func(name, content string) string {
		path := filepath.Join(dir, name)
		os.WriteFile(path, []byte(content), 0o644)
		return path
	}
	now := func() time.Time { return time.Unix(1_000_000, 0) }
	expired := write("old", "sha=x\nrecorded_epoch=1\nscore=80\nmin_delta=1\nmax_age_minutes=60\neval=e\nartifact=\n")
	if _, ferr := FrontierChallenge(FrontierOptions{File: expired, Env: noEnv, Now: now, Score: "99"}); ferr == nil ||
		ferr.Code != 1 || !strings.Contains(ferr.Message, "frontier expired") {
		t.Fatalf("expired frontier compared: %v", ferr)
	}
	nowindow := write("nowindow", "sha=x\nrecorded_epoch=1\nscore=80\nmin_delta=1\nmax_age_minutes=\neval=e\nartifact=\n")
	if _, ferr := FrontierChallenge(FrontierOptions{File: nowindow, Env: noEnv, Now: now, Score: "99"}); ferr != nil {
		t.Fatalf("windowless frontier expired: %v", ferr)
	}
	lines, _ := FrontierStatus(FrontierOptions{File: nowindow})
	if !strings.HasSuffix(strings.Join(lines, "\n"), "direction=max") {
		t.Fatalf("legacy status hid direction: %v", lines)
	}
	for name, content := range map[string]string{
		"sideways": "sha=x\nrecorded_epoch=1\nscore=80\nmin_delta=1\ndirection=sideways\nmax_age_minutes=\neval=e\nartifact=\n",
		"empty":    "sha=x\nrecorded_epoch=1\nscore=80\nmin_delta=1\ndirection=\nmax_age_minutes=\neval=e\nartifact=\n",
	} {
		path := write(name, content)
		if _, ferr := FrontierChallenge(FrontierOptions{File: path, Env: noEnv, Now: now, Score: "99"}); ferr == nil || ferr.Code != 2 {
			t.Fatalf("%s direction accepted: %v", name, ferr)
		}
	}
	badScore := write("badscore", "sha=x\nrecorded_epoch=1\nscore=NaNish\nmin_delta=1\nmax_age_minutes=\neval=e\nartifact=\n")
	if _, ferr := FrontierChallenge(FrontierOptions{File: badScore, Env: noEnv, Score: "99"}); ferr == nil || ferr.Code != 2 {
		t.Fatalf("malformed stored score accepted: %v", ferr)
	}
}

func TestFrontierDirectionMin(t *testing.T) {
	repo := frontierRepo(t)
	file := filepath.Join(repo, "plans", "frontier-min")
	if _, ferr := FrontierRecord(FrontierOptions{File: file, Repo: repo, Env: noEnv,
		Score: "80", MinDelta: "1", Direction: "min", Eval: "e"}); ferr != nil {
		t.Fatalf("min record refused: %v", ferr)
	}
	if _, ferr := FrontierChallenge(FrontierOptions{File: file, Env: noEnv, Score: "79.5"}); ferr == nil {
		t.Fatal("within-noise min improvement accepted")
	}
	if _, ferr := FrontierChallenge(FrontierOptions{File: file, Env: noEnv, Score: "78"}); ferr != nil {
		t.Fatalf("real min improvement refused: %v", ferr)
	}
	// challenge refuses any direction input.
	if _, ferr := FrontierChallenge(FrontierOptions{File: file, Env: noEnv, Score: "78", Direction: "min"}); ferr == nil || ferr.Code != 2 {
		t.Fatalf("challenge accepted a direction flag: %v", ferr)
	}
}

func TestFrontierUsageErrors(t *testing.T) {
	repo := frontierRepo(t)
	file := filepath.Join(repo, "plans", "frontier")
	if _, ferr := FrontierRecord(FrontierOptions{File: file, Repo: repo, Env: noEnv, Score: "abc", Eval: "e"}); ferr == nil || ferr.Code != 2 {
		t.Fatalf("non-numeric record score accepted: %v", ferr)
	}
	if _, ferr := FrontierRecord(FrontierOptions{File: file, Repo: repo, Env: noEnv, Score: "80"}); ferr == nil || ferr.Code != 2 {
		t.Fatalf("record without eval accepted: %v", ferr)
	}
	if _, ferr := FrontierRecord(FrontierOptions{File: file, Repo: t.TempDir(), Env: noEnv, Score: "80", Eval: "e"}); ferr == nil || ferr.Code != 2 {
		t.Fatalf("record outside git accepted: %v", ferr)
	}
	dirty := frontierRepo(t)
	os.WriteFile(filepath.Join(dirty, "scratch"), []byte("x"), 0o644)
	if _, ferr := FrontierRecord(FrontierOptions{File: filepath.Join(dirty, "f"), Repo: dirty, Env: noEnv, Score: "80", Eval: "e"}); ferr == nil || ferr.Code != 1 {
		t.Fatalf("dirty worktree accepted: %v", ferr)
	}
	if _, ferr := FrontierChallenge(FrontierOptions{File: filepath.Join(repo, "absent"), Env: noEnv, Score: "80"}); ferr == nil || ferr.Code != 1 {
		t.Fatalf("absent frontier challenged: %v", ferr)
	}
	lines, _ := FrontierStatus(FrontierOptions{File: filepath.Join(repo, "absent")})
	if !strings.Contains(lines[0], "no frontier recorded") {
		t.Fatalf("absent status wrong: %v", lines)
	}
	// Env fallbacks resolve when flags are absent.
	env := func(key string) string {
		return map[string]string{"METASYSTEM_FRONTIER_MIN_DELTA": "2", "METASYSTEM_FRONTIER_DIRECTION": "min"}[key]
	}
	if _, ferr := FrontierRecord(FrontierOptions{File: file, Repo: repo, Env: env, Score: "80", Eval: "e"}); ferr != nil {
		t.Fatalf("env-resolved record refused: %v", ferr)
	}
	data, _ := os.ReadFile(file)
	if !strings.Contains(string(data), "min_delta=2\ndirection=min") {
		t.Fatalf("env values not persisted:\n%s", data)
	}
}

// validate-report-3: an unreadable frontier is not an ABSENT one — treating
// it as absent skips every guard and overwrites the record without --force.
func TestFrontierUnreadableFileRefuses(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("permission bits cannot bite as root")
	}
	repo := frontierRepo(t)
	// The frontier lives OUTSIDE the worktree here, so the unreadable file
	// hits the read path rather than the dirty-worktree guard (git cannot
	// hash a chmod-000 file and reports the tree dirty first).
	file := filepath.Join(t.TempDir(), "frontier")
	opts := FrontierOptions{File: file, Repo: repo, Env: noEnv,
		Score: "80", MinDelta: "1", Eval: "declared eval"}
	if _, ferr := FrontierRecord(opts); ferr != nil {
		t.Fatalf("baseline record failed: %v", ferr)
	}
	if err := os.Chmod(file, 0o000); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(file, 0o644)

	worse := FrontierOptions{File: file, Repo: repo, Env: noEnv,
		Score: "10", MinDelta: "1", Eval: "declared eval"}
	ferr := func() *FrontierError { _, e := FrontierRecord(worse); return e }()
	if ferr == nil || ferr.Code != 2 || !strings.Contains(ferr.Message, "unreadable") {
		t.Fatalf("record over an unreadable frontier must refuse with the cause: %v", ferr)
	}
	if _, cerr := FrontierChallenge(FrontierOptions{File: file, Env: noEnv, Score: "99"}); cerr == nil ||
		cerr.Code != 2 || !strings.Contains(cerr.Message, "unreadable") {
		t.Fatalf("challenge must name unreadable, not claim no frontier: %v", cerr)
	}
	os.Chmod(file, 0o644)
	// The guarded state survived: the recorded score is still the baseline.
	fields, err := frontierReadFields(file)
	if err != nil || fields["score"] != "80" {
		t.Fatalf("the frontier was clobbered: %v %v", fields, err)
	}
}
