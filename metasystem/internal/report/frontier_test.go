package report

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

type frontierGitStep struct {
	repo string
	args []string
	out  string
	err  error
}

func frontierGitScript(t *testing.T, steps []frontierGitStep) func(string, ...string) (string, error) {
	t.Helper()
	next := 0
	t.Cleanup(func() {
		if next != len(steps) {
			t.Errorf("frontier Git transcript consumed %d of %d calls", next, len(steps))
		}
	})
	return func(repo string, args ...string) (string, error) {
		t.Helper()
		if next >= len(steps) {
			t.Fatalf("unexpected frontier Git call: repo=%q args=%q", repo, args)
		}
		step := steps[next]
		if repo != step.repo || !slices.Equal(args, step.args) {
			t.Fatalf("frontier Git call %d: got repo=%q args=%q, want repo=%q args=%q",
				next, repo, args, step.repo, step.args)
		}
		next++
		return step.out, step.err
	}
}

func frontierCleanReads(repo, sha string) []frontierGitStep {
	return []frontierGitStep{
		{repo, []string{"rev-parse", "--is-inside-work-tree"}, "true", nil},
		{repo, []string{"status", "--porcelain"}, "", nil},
		{repo, []string{"rev-parse", "HEAD"}, sha, nil},
		{repo, []string{"rev-parse", "--short", "HEAD"}, sha[:7], nil},
	}
}

func frontierCheckReads(repo string) []frontierGitStep {
	return []frontierGitStep{
		{repo, []string{"rev-parse", "--is-inside-work-tree"}, "true", nil},
		{repo, []string{"status", "--porcelain"}, "", nil},
	}
}

func noEnv(string) string { return "" }

func TestFrontierRecordAndChallenge(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	file := filepath.Join(repo, "plans", "frontier")
	sha := strings.Repeat("a", 40)
	steps := frontierCleanReads(repo, sha)
	steps = append(steps, frontierCheckReads(repo)...)
	steps = append(steps, frontierCleanReads(repo, sha)...)
	steps = append(steps, frontierCheckReads(repo)...)
	gitRead := frontierGitScript(t, steps)
	opts := FrontierOptions{File: file, Repo: repo, Env: noEnv,
		Score: "80", MinDelta: "1", Eval: "declared eval"}
	lines, ferr := frontierRecordWithGit(opts, gitRead)
	if ferr != nil || len(lines) != 2 || lines[0] != "frontier recorded: score 80 at "+sha[:7] ||
		lines[1] != "commit "+file+" with the frontier checkpoint" {
		t.Fatalf("record failed: %v %v", ferr, lines)
	}
	if fields, err := frontierReadFields(file); err != nil || fields["sha"] != sha {
		t.Fatalf("recorded SHA: %v %v", fields, err)
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
	// The next invocation declares a clean checkpoint; a regression without
	// --force refuses, while --force re-baselines.
	if _, ferr := frontierRecordWithGit(FrontierOptions{File: file, Repo: repo, Env: noEnv, Score: "75", Eval: "e"}, gitRead); ferr == nil || ferr.Code != 1 {
		t.Fatalf("regression accepted: %v", ferr)
	}
	lines, ferr = frontierRecordWithGit(FrontierOptions{File: file, Repo: repo, Env: noEnv, Score: "75", Eval: "e", Force: true}, gitRead)
	if ferr != nil || len(lines) != 2 || lines[0] != "frontier recorded: score 75 at "+sha[:7] {
		t.Fatalf("force re-baseline refused: %v", ferr)
	}
	if fields, err := frontierReadFields(file); err != nil || fields["sha"] != sha || fields["score"] != "75" {
		t.Fatalf("forced frontier contents: %v %v", fields, err)
	}
	// A direction change without --force refuses.
	if _, ferr := frontierRecordWithGit(FrontierOptions{File: file, Repo: repo, Env: noEnv, Score: "70", Eval: "e", Direction: "min"}, gitRead); ferr == nil ||
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
	t.Parallel()
	repo := t.TempDir()
	file := filepath.Join(repo, "plans", "frontier-min")
	sha := strings.Repeat("b", 40)
	lines, ferr := frontierRecordWithGit(FrontierOptions{File: file, Repo: repo, Env: noEnv,
		Score: "80", MinDelta: "1", Direction: "min", Eval: "e"}, frontierGitScript(t, frontierCleanReads(repo, sha)))
	if ferr != nil || len(lines) != 2 || lines[0] != "frontier recorded: score 80 at "+sha[:7] {
		t.Fatalf("min record refused: %v", ferr)
	}
	if fields, err := frontierReadFields(file); err != nil || fields["sha"] != sha || fields["direction"] != "min" {
		t.Fatalf("min frontier contents: %v %v", fields, err)
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
	t.Parallel()
	repo := t.TempDir()
	file := filepath.Join(repo, "plans", "frontier")
	noGit := frontierGitScript(t, nil)
	if _, ferr := frontierRecordWithGit(FrontierOptions{File: file, Repo: repo, Env: noEnv, Score: "abc", Eval: "e"}, noGit); ferr == nil || ferr.Code != 2 {
		t.Fatalf("non-numeric record score accepted: %v", ferr)
	}
	if _, ferr := frontierRecordWithGit(FrontierOptions{File: file, Repo: repo, Env: noEnv, Score: "80"}, noGit); ferr == nil || ferr.Code != 2 {
		t.Fatalf("record without eval accepted: %v", ferr)
	}
	outside := t.TempDir()
	outsideGit := frontierGitScript(t, []frontierGitStep{{outside, []string{"rev-parse", "--is-inside-work-tree"}, "", errors.New("outside repository")}})
	if _, ferr := frontierRecordWithGit(FrontierOptions{File: file, Repo: outside, Env: noEnv, Score: "80", Eval: "e"}, outsideGit); ferr == nil || ferr.Code != 2 {
		t.Fatalf("record outside git accepted: %v", ferr)
	}
	dirty := t.TempDir()
	dirtyGit := frontierGitScript(t, []frontierGitStep{
		{dirty, []string{"rev-parse", "--is-inside-work-tree"}, "true", nil},
		{dirty, []string{"status", "--porcelain"}, "?? scratch", nil},
	})
	if _, ferr := frontierRecordWithGit(FrontierOptions{File: filepath.Join(dirty, "f"), Repo: dirty, Env: noEnv, Score: "80", Eval: "e"}, dirtyGit); ferr == nil || ferr.Code != 1 {
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
	sha := strings.Repeat("c", 40)
	lines, ferr := frontierRecordWithGit(FrontierOptions{File: file, Repo: repo, Env: env, Score: "80", Eval: "e"}, frontierGitScript(t, frontierCleanReads(repo, sha)))
	if ferr != nil || len(lines) != 2 || lines[0] != "frontier recorded: score 80 at "+sha[:7] {
		t.Fatalf("env-resolved record refused: %v", ferr)
	}
	data, _ := os.ReadFile(file)
	if !strings.Contains(string(data), "sha="+sha+"\n") || !strings.Contains(string(data), "min_delta=2\ndirection=min") {
		t.Fatalf("env values not persisted:\n%s", data)
	}
}

// validate-report-3: an unreadable frontier is not an ABSENT one — treating
// it as absent skips every guard and overwrites the record without --force.
func TestFrontierUnreadableFileRefuses(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("permission bits cannot bite as root")
	}
	t.Parallel()
	repo := t.TempDir()
	// The frontier lives outside the declared worktree, so the unreadable
	// file reaches the read path after the clean-status fact.
	file := filepath.Join(t.TempDir(), "frontier")
	sha := strings.Repeat("d", 40)
	steps := append(frontierCleanReads(repo, sha), frontierCheckReads(repo)...)
	gitRead := frontierGitScript(t, steps)
	opts := FrontierOptions{File: file, Repo: repo, Env: noEnv,
		Score: "80", MinDelta: "1", Eval: "declared eval"}
	if _, ferr := frontierRecordWithGit(opts, gitRead); ferr != nil {
		t.Fatalf("baseline record failed: %v", ferr)
	}
	if fields, err := frontierReadFields(file); err != nil || fields["sha"] != sha {
		t.Fatalf("baseline SHA: %v %v", fields, err)
	}
	if err := os.Chmod(file, 0o000); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(file, 0o644)

	worse := FrontierOptions{File: file, Repo: repo, Env: noEnv,
		Score: "10", MinDelta: "1", Eval: "declared eval"}
	ferr := func() *FrontierError { _, e := frontierRecordWithGit(worse, gitRead); return e }()
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

func TestGitAdapterFrontierReadsRepoState(t *testing.T) {
	repo := t.TempDir()
	runGit := func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = repo
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %q: %v %s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	runGit("init", "-q")
	runGit("-c", "user.name=frontier", "-c", "user.email=frontier@example.test",
		"commit", "-q", "--allow-empty", "-m", "baseline")
	root, err := frontierGit(repo, "rev-parse", "--show-toplevel")
	wantRoot, pathErr := filepath.EvalSymlinks(repo)
	if err != nil || pathErr != nil || root != wantRoot {
		t.Fatalf("Git cwd: got %q, want %q: %v %v", root, wantRoot, err, pathErr)
	}
	clean, err := frontierGit(repo, "status", "--porcelain")
	if err != nil || clean != "" {
		t.Fatalf("clean status: %q %v", clean, err)
	}
	full, err := frontierGit(repo, "rev-parse", "HEAD")
	if err != nil || full != runGit("rev-parse", "HEAD") {
		t.Fatalf("full HEAD: %q %v", full, err)
	}
	short, err := frontierGit(repo, "rev-parse", "--short", "HEAD")
	if err != nil || short != runGit("rev-parse", "--short", "HEAD") || !strings.HasPrefix(full, short) {
		t.Fatalf("short HEAD: %q %v", short, err)
	}
	if err := os.WriteFile(filepath.Join(repo, "scratch"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	dirty, err := frontierGit(repo, "status", "--porcelain")
	if err != nil || dirty != "?? scratch" {
		t.Fatalf("dirty status: %q %v", dirty, err)
	}
}

// Record enforces the stored frontier's guards before reading HEAD, and a
// refused record leaves the recorded frontier byte-for-byte intact.
func TestFrontierRecordRefusesAgainstStoredFrontier(t *testing.T) {
	t.Parallel()
	now := func() time.Time { return time.Unix(1_000_000, 0) }
	fresh := "sha=x\nrecorded_epoch=999940\nscore=80\nmin_delta=1\nmax_age_minutes=60\neval=e\nartifact=\n"
	for _, test := range []struct {
		name, stored string
		opts         FrontierOptions
		code         int
		want         string
	}{
		{"noise floor flag", fresh, FrontierOptions{MinDelta: "tiny"}, 2, "invalid noise floor: tiny"},
		{"window flag", fresh, FrontierOptions{MaxAge: "soon"}, 2, "invalid measurement window: soon"},
		{"direction flag", fresh, FrontierOptions{Direction: "sideways"}, 2, "direction"},
		{"legacy direction is max", fresh, FrontierOptions{Direction: "min"}, 1, "direction min differs from the recorded frontier's max"},
		{"unparseable epoch", strings.Replace(fresh, "recorded_epoch=999940", "recorded_epoch=yesterday", 1), FrontierOptions{}, 2, "frontier file is malformed"},
		{"expired window", strings.Replace(fresh, "recorded_epoch=999940", "recorded_epoch=1", 1), FrontierOptions{}, 1, "older than its measurement window"},
		{"malformed score", strings.Replace(fresh, "score=80", "score=high", 1), FrontierOptions{}, 2, "frontier file is malformed"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			repo := t.TempDir()
			file := filepath.Join(repo, "frontier")
			if err := os.WriteFile(file, []byte(test.stored), 0o644); err != nil {
				t.Fatal(err)
			}
			opts := test.opts
			opts.File, opts.Repo, opts.Env, opts.Now, opts.Score, opts.Eval = file, repo, noEnv, now, "99", "e"
			_, ferr := frontierRecordWithGit(opts, frontierGitScript(t, frontierCheckReads(repo)))
			if ferr == nil || ferr.Code != test.code || !strings.Contains(ferr.Message, test.want) {
				t.Fatalf("record = %v, want code %d containing %q", ferr, test.code, test.want)
			}
			if data, err := os.ReadFile(file); err != nil || string(data) != test.stored {
				t.Fatalf("refused record changed the frontier: %q, %v", data, err)
			}
		})
	}
}

// A clean worktree whose HEAD cannot be read records nothing.
func TestFrontierRecordRefusesWithoutHead(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	headless := append(frontierCheckReads(repo), frontierGitStep{repo, []string{"rev-parse", "HEAD"}, "", errors.New("unborn branch")})
	file := filepath.Join(repo, "plans", "frontier")
	if _, ferr := frontierRecordWithGit(FrontierOptions{File: file, Repo: repo, Env: noEnv, Score: "80", Eval: "e"}, frontierGitScript(t, headless)); ferr == nil ||
		ferr.Code != 2 || ferr.Message != "not inside a git repository" {
		t.Fatalf("record without HEAD = %v", ferr)
	}
	if _, err := os.Stat(file); !os.IsNotExist(err) {
		t.Fatalf("record without HEAD wrote a frontier: %v", err)
	}
}
