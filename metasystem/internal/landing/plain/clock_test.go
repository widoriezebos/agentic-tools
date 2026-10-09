package plain

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLandingClockKeepsUnknownAndStartsANewEpisode(t *testing.T) {
	t.Parallel()
	b := newBed(t)
	now := bedNow
	run := func() {
		t.Helper()
		var out bytes.Buffer
		result, err := Run(b.install, b.checkout, "fixture", "", &out, ProveSeams{Now: func() time.Time { return now }, Command: func(cmd *exec.Cmd) error {
			now = now.Add(2 * time.Minute)
			_, err := cmd.Stdout.Write([]byte("landing environment fixture\nlanding package fixture/unit 1 ok 120000\nLANDING-CHECKED\t0\n"))
			return err
		}})
		if err != nil || result.Result != Green {
			t.Fatalf("prove: %+v %v %s", result, err, &out)
		}
	}
	sha := b.seat("seat-a", "goal-a")
	b.handIn("seat-a", "goal-a", sha)
	b.merge("goal-a")
	run()
	if _, err := PushChecked(b.install, b.checkout, bedNow.Add(10*time.Minute), nil); err != nil {
		t.Fatal(err)
	}
	// The next hand-in supersedes an entry that the previous push landed.
	now = bedNow.Add(20 * time.Minute)
	b.git(filepath.Join(b.root, "seat-a"), "fetch", "--quiet", "origin")
	sha = b.seat("seat-a", "goal-a")
	if _, _, err := HandIn(b.install, Line{Goal: "goal-a", Branch: "goal/goal-a", SHA: sha, At: now.Format(time.RFC3339)}); err != nil {
		t.Fatal(err)
	}
	b.merge("goal-a")
	run()
	result, ok, err := LastResult(b.install)
	if err != nil || !ok {
		t.Fatal(err)
	}
	// A retained record from an older engine has an end but no measured start.
	result.StartedAt, result.Minutes = "", nil
	if err := appendLine(resultsPath(b.install), result); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(scopePath(b.install, result.Attempt), []byte(`{"scope":"full"}`), 0644); err != nil {
		t.Fatal(err)
	}
	file, err := os.OpenFile(resultsPath(b.install), os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		t.Fatal(err)
	}
	_, err = file.WriteString("{interrupted\n")
	if err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := PushChecked(b.install, b.checkout, bedNow.Add(25*time.Minute), nil); err != nil {
		t.Fatal(err)
	}
	push, ok, err := LastPush(b.install)
	if err != nil || !ok {
		t.Fatal(err)
	}
	c := push.Clock
	if c == nil || c.TotalMinutes == nil || *c.TotalMinutes != 5 || c.HandInAt["goal-a"] != bedNow.Add(20*time.Minute).Format(time.RFC3339) || len(c.Proofs) != 1 || len(c.FixRounds) != 0 {
		t.Fatalf("new episode: %+v", c)
	}
	if c.Proofs[0].Minutes != nil || c.Proofs[0].Shards != nil || !strings.Contains(c.Words(), "proof unknown") || !strings.Contains(c.Words(), "proof history") || !strings.Contains(c.Words(), "scope of ") {
		t.Fatalf("unknown timing was invented: %+v; %s", c, c.Words())
	}
	// A damaged hand-in history cannot establish either duration from hand-in.
	b = newBed(t)
	now = bedNow
	sha = b.seat("seat-a", "goal-a")
	if _, _, err := HandIn(b.install, Line{Goal: "goal-a", Branch: "goal/goal-a", SHA: sha, At: now.Format(time.RFC3339)}); err != nil {
		t.Fatal(err)
	}
	b.merge("goal-a")
	run()
	queue, err := os.ReadFile(queuePath(b.install))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(queuePath(b.install), append(queue, []byte("{interrupted\n")...), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := PushChecked(b.install, b.checkout, bedNow.Add(45*time.Minute), nil); err != nil {
		t.Fatal(err)
	}
	push, ok, err = LastPush(b.install)
	if err != nil || !ok || push.Clock.TotalMinutes != nil || push.Clock.MergeMinutes != nil || !strings.Contains(push.Clock.Words(), "hand-in history") {
		t.Fatalf("damaged hand-in durations: %+v; %v", push.Clock, err)
	}
	// The current records hand-in has a known start; older records cannot choose it.
	b = newBed(t)
	now = bedNow
	oldSHA := b.git(b.checkout, "rev-parse", "HEAD")
	if _, _, err := HandIn(b.install, Line{Goal: "goal-a", Branch: "goal/goal-a", SHA: oldSHA, At: bedNow.Add(-5 * time.Minute).Format(time.RFC3339), Records: true}); err != nil {
		t.Fatal(err)
	}
	sha = b.seat("seat-a", "goal-a")
	if _, _, err := HandIn(b.install, Line{Goal: "goal-a", Branch: "goal/goal-a", SHA: sha, At: now.Format(time.RFC3339), Records: true}); err != nil {
		t.Fatal(err)
	}
	b.merge("goal-a")
	run()
	if _, err := PushChecked(b.install, b.checkout, bedNow.Add(5*time.Minute), nil); err != nil {
		t.Fatal(err)
	}
	push, ok, err = LastPush(b.install)
	if err != nil || !ok || push.Clock.TotalMinutes == nil || *push.Clock.TotalMinutes != 5 || push.Clock.HandInAt["goal-a"] != bedNow.Format(time.RFC3339) {
		t.Fatalf("current records hand-in timing: %+v; %v", push.Clock, err)
	}
}

func TestLandingClockTimesARegisteredFlakeRepeat(t *testing.T) {
	t.Parallel()
	b := newScopeBed(t)
	now := bedNow
	b.seams.Now = func() time.Time { return now }
	b.seams.Judge = func(string, string, []FailedUnit) (map[string]UnitJudgement, error) {
		return map[string]UnitJudgement{"fixture/unit": {Known: true}}, nil
	}
	b.seams.RecordFlake = func(FlakeRecord) (FlakeRecorded, error) { return FlakeRecorded{Goal: "fix-flake", Seen: 1}, nil }
	b.seams.Command = func(cmd *exec.Cmd) error {
		fmt.Fprint(cmd.Stdout, "landing environment image toolchain\n")
		if commandEnv(cmd, "LANDING_ONLY") != "" {
			now = now.Add(3 * time.Minute)
			fmt.Fprint(cmd.Stdout, "LANDING-CHECKED\t0\n")
			return nil
		}
		now = now.Add(5 * time.Minute)
		fmt.Fprint(cmd.Stdout, "LANDING-FAILED\tfixture/unit\tTestFlake\nLANDING-CHECKED\t1\n")
		return exec.Command("/usr/bin/false").Run()
	}
	r := b.run(t)
	if r.Minutes == nil || *r.Minutes != 8 || len(r.FlakeRepeats) != 1 || r.FlakeRepeats[0].Since != bedNow.Add(5*time.Minute).Format(time.RFC3339) || r.FlakeRepeats[0].Minutes == nil || *r.FlakeRepeats[0].Minutes != 3 {
		t.Fatalf("repeat's own timing: %+v, repeats %+v", r, r.FlakeRepeats)
	}
}

func TestProveShellLeadsItsPathWithTheServingEngine(t *testing.T) {
	t.Parallel()
	b := newBed(t)
	var path string
	result, err := Run(b.install, b.checkout, "fixture", "", io.Discard, ProveSeams{Now: func() time.Time { return bedNow }, Command: func(cmd *exec.Cmd) error {
		for _, entry := range cmd.Env {
			if strings.HasPrefix(entry, "PATH=") {
				path = strings.TrimPrefix(entry, "PATH=")
			}
		}
		_, err := cmd.Stdout.Write([]byte("landing environment fixture\nlanding package fixture/unit 1 ok 1\nLANDING-CHECKED\t0\n"))
		return err
	}})
	if err != nil || result.Result != Green {
		t.Fatalf("prove: %+v %v", result, err)
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	if first, _, _ := strings.Cut(path, string(os.PathListSeparator)); first != filepath.Dir(executable) {
		t.Fatalf("the proof shell's PATH must lead with the engine running the lane: %q", path)
	}
}
