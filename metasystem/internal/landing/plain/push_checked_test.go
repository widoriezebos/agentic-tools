package plain

import (
	"errors"
	"testing"
)

// This adapter test uses real Git to observe the fetched main and publication;
// a stub cannot prove that the callback's refusal leaves the remote unchanged.
func TestPushCheckedAdapterPublicationBoundary(t *testing.T) {
	t.Parallel()
	b := newBed(t)
	old := b.originMain()
	sha := b.seat("seat-a", "goal-a")
	head := b.merge("goal-a")
	if _, err := PushChecked(b.install, b.checkout, bedNow, func(string, string) error {
		t.Fatal("checked an unproven push")
		return nil
	}); err == nil {
		t.Fatal("an unproven push was not refused")
	}
	b.prove(b.greenScript)
	// Advance main to a parent of HEAD while the checkout still holds old.
	b.git(b.root, "--git-dir", b.origin, "update-ref", "refs/heads/main", sha)
	if got := b.git(b.checkout, "rev-parse", "refs/remotes/origin/main"); got != old {
		t.Fatalf("fixture main was not stale: %s", got)
	}
	refused := errors.New("design does not stand")
	checks := 0
	before := func(fetched, candidate string) error {
		checks++
		if fetched != sha || candidate != head || b.originMain() != sha {
			t.Fatalf("check before publication: main=%s head=%s remote=%s", fetched, candidate, b.originMain())
		}
		return refused
	}
	outcome, err := PushChecked(b.install, b.checkout, bedNow, before)
	if !errors.Is(err, refused) || outcome.Changed || outcome.Old != sha || outcome.Commit != head || checks != 1 || b.originMain() != sha {
		t.Fatalf("refused publication: outcome=%+v err=%v checks=%d remote=%s", outcome, err, checks, b.originMain())
	}
	if _, ok, err := LastPush(b.install); err != nil || ok {
		t.Fatalf("a refused check recorded a push: present=%v err=%v", ok, err)
	}
	outcome, err = PushChecked(b.install, b.checkout, bedNow, func(fetched, candidate string) error {
		if fetched != sha || candidate != head {
			t.Fatalf("checked the wrong commits: main=%s head=%s", fetched, candidate)
		}
		return nil
	})
	if err != nil || !outcome.Changed || b.originMain() != head {
		t.Fatalf("allowed publication: outcome=%+v err=%v remote=%s", outcome, err, b.originMain())
	}
	if push, ok, err := LastPush(b.install); err != nil || !ok || push.Old != sha || push.Commit != head {
		t.Fatalf("allowed push record: push=%+v present=%v err=%v", push, ok, err)
	}
	outcome, err = PushChecked(b.install, b.checkout, bedNow, func(string, string) error {
		t.Fatal("checked a push already on main")
		return nil
	})
	if err != nil || outcome.Changed {
		t.Fatalf("already on main: outcome=%+v err=%v", outcome, err)
	}
}
