package deploy

import (
	"strings"
	"syscall"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/lock"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testenv"
)

// Every way a run or a person's act ends is told by one record line:
// active, the failure of the operation that failed, or stopped when a
// pause ended the call. One that found nothing to do, or was refused,
// writes none. Each row starts with c1 active and main's tip at c2.
func TestEveryEndOfARunOrAnActIsOneLine(t *testing.T) {
	t.Parallel()
	run := func(_ *bed, r *Runner) { _, _ = r.Run() }
	pause := func(_ *bed, r *Runner) { _, _ = r.Pause("") }
	rollback := func(_ *bed, r *Runner) { _, _ = r.Rollback() }
	// resume does what deploy resume does: it runs the runner once it
	// lifted a pause.
	resume := func(_ *bed, r *Runner) {
		if _, paused, err := r.Resume(); err == nil && paused {
			_, _ = r.Run()
		}
	}
	held := func(b *bed) step { return step{HoldBefore: b.fifo("held")} }
	paused := func(b *bed) {
		if _, err := b.runner("Ann", nil).Pause(""); err != nil {
			b.t.Fatal(err)
		}
	}
	onC2 := func(b *bed) { b.deployed("c2") }
	locked := func(b *bed) {
		other, err := lock.File(lockPath(b.dir), 0o600, lock.TryExclusive)
		if err != nil {
			b.t.Fatal(err)
		}
		b.t.Cleanup(func() { _ = other.Release() })
	}
	rows := []struct {
		name  string
		setup func(b *bed)
		act   func(b *bed, r *Runner)
		// stall is the call, "operation commit", a second person's pause
		// ends once its process started.
		stall   string
		outcome string // "" when nothing is written
	}{
		{name: "now: version fails", setup: func(b *bed) { b.script("version", step{Exit: 1, Reason: "no state"}) }, act: run, outcome: OutcomeVersionFailed},
		{name: "now: version answers unreadably", setup: func(b *bed) { b.script("version", step{Raw: "not json"}) }, act: run, outcome: OutcomeVersionFailed},
		{name: "now: version stalls and a pause ends it", setup: func(b *bed) { b.script("version", held(b), step{}) }, act: run, stall: "version c2", outcome: OutcomeStopped},
		{name: "now: a pause comes as version answers", act: func(b *bed, r *Runner) {
			r.Started = func(version Active) {
				testenv.Await(b.t, "version to answer", func() bool { return !version.AdapterAlive() })
				if err := writePause(b.dir, Pause{By: "Ann"}); err != nil {
					b.t.Error(err)
				}
			}
			run(b, r)
		}, outcome: OutcomeStopped},
		{name: "now: build fails", setup: func(b *bed) { b.script("build", step{Exit: 1, Reason: "compile error"}) }, act: run, outcome: OutcomeBuildFailed},
		{name: "now: build stalls and a pause ends it", setup: func(b *bed) { b.script("build", held(b)) }, act: run, stall: "build c2", outcome: OutcomeStopped},
		{name: "now: activate fails", setup: func(b *bed) { b.script("activate", step{Exit: 1, Reason: "port in use"}) }, act: run, outcome: OutcomeActivateFailed},
		{name: "now: activate stalls and a pause ends it", setup: func(b *bed) { b.script("activate", held(b)) }, act: run, stall: "activate c2", outcome: OutcomeStopped},
		{name: "now: version after activate fails", setup: func(b *bed) { b.script("version", step{}, step{Exit: 3}) }, act: run, outcome: OutcomeVerifyFailed},
		{name: "now: deploys main's tip", act: run, outcome: OutcomeActive},
		{name: "now: nothing to do, paused before any call", setup: paused, act: run},
		{name: "now: nothing to do, main's tip already active", setup: func(b *bed) { b.main("c1") }, act: run},
		{name: "now: nothing to do, the lock held by another run", setup: locked, act: run},
		{name: "rollback: version fails", setup: func(b *bed) { onC2(b); b.script("version", step{Exit: 1, Reason: "no state"}) }, act: rollback, outcome: OutcomeVersionFailed},
		{name: "rollback: its adapter call fails", setup: func(b *bed) { onC2(b); b.script("rollback", step{Exit: 1, Reason: "c1 is gone"}) }, act: rollback, outcome: OutcomeActivateFailed},
		{name: "rollback: its adapter call stalls and a pause ends it", setup: func(b *bed) { onC2(b); b.script("rollback", held(b)) }, act: rollback, stall: "rollback c1", outcome: OutcomeStopped},
		{name: "rollback: version after it stalls and a pause ends it", setup: func(b *bed) {
			onC2(b)
			b.script("rollback", step{Exit: 3})
			b.script("version", step{}, held(b), step{})
		}, act: rollback, stall: "version c1", outcome: OutcomeStopped},
		{name: "rollback: returns to the deploy before", setup: onC2, act: rollback, outcome: OutcomeActive},
		{name: "rollback: refused with no deploy before, and nothing starts", act: rollback},
		{name: "rollback: refused while a run holds the lock", setup: func(b *bed) { onC2(b); locked(b) }, act: rollback},
		{name: "rollback: nothing to do, it already holds", setup: func(b *bed) { onC2(b); rollback(b, b.runner("Ann", nil)) }, act: rollback},
		{name: "pause: nothing to do, it calls no adapter", act: pause},
		{name: "resume: its run deploys main's tip", setup: paused, act: resume, outcome: OutcomeActive},
		{name: "resume: its run's version fails", setup: func(b *bed) { paused(b); b.script("version", step{Exit: 1, Reason: "no state"}) }, act: resume, outcome: OutcomeVersionFailed},
		{name: "resume: nothing to do, deploys are not paused", act: resume},
		{name: "resume: refused while a run holds the lock", setup: func(b *bed) { paused(b); locked(b) }, act: resume},
	}
	for _, row := range rows {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			b := newBed(t)
			b.deployed("c1")
			b.main("c2")
			if row.setup != nil {
				row.setup(b)
			}
			before := len(b.lines())
			if row.stall == "" {
				row.act(b, b.runner("Wido", nil))
			} else {
				stalled(t, b, row.stall, row.act)
			}
			appended, want := b.lines()[before:], 1
			if row.outcome == "" {
				want = 0
			}
			if len(appended) != want || want == 1 && appended[0].Outcome != row.outcome {
				t.Fatalf("appended %q; want %d line(s) %s", outcomes(appended), want, row.outcome)
			}
			status, err := ReadStatus(b.dir, 0)
			if err == nil && want == 1 && row.outcome != OutcomeActive && (status.LastFailure == nil || *status.LastFailure != appended[0]) {
				t.Fatalf("status's last failure = %+v; want %+v", status.LastFailure, appended[0])
			}
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}

// stalled runs act until its call of operation on commit started, ends
// that call with a second person's pause, and waits for act to end.
func stalled(t *testing.T, b *bed, call string, act func(*bed, *Runner)) {
	t.Helper()
	operation, commit, _ := strings.Cut(call, " ")
	started, done := make(chan Active, 32), make(chan struct{})
	go func() {
		defer close(done)
		defer close(started)
		act(b, b.runner("Wido", started))
	}()
	stalling := awaitStart(t, started, operation, commit)
	t.Cleanup(func() { _ = syscall.Kill(-stalling.PID, syscall.SIGKILL) })
	if _, err := b.runner("Ann", nil).Pause("it hangs"); err != nil {
		t.Fatal(err)
	}
	<-done
}
