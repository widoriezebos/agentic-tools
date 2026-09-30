package landpath

// The landing gate before every push (g1-s70 D2, SOL-S70-01): admission is not
// the last word. A landing in a goal's name reads the goal's gate again
// immediately before each push attempt, the retry after a moved origin and the
// carried form's single push included, so a hold or a changed human word that
// arrived while the landing proved stops the publication.

import (
	"errors"
	"strings"
	"testing"
)

const gateHeld = "LANDING_HELD_BY_SITTING: goal fx is held by Wido's review sitting (plans/reviews/review-of-fx.md); nothing lands while a sitting stands"

// gateAnswers is a landing gate that answers each read in turn and records
// what it was asked.
func gateAnswers(b *bed, answers ...error) *[]string {
	asked := &[]string{}
	b.owners.LandingGate = func(root, goal string) error {
		*asked = append(*asked, goal)
		b.log.add("gate goal=%s", goal)
		answer := answers[0]
		if len(answers) > 1 {
			answers = answers[1:]
		}
		return answer
	}
	return asked
}

func TestTheStagedFormReadsTheGateImmediatelyBeforeItsPush(t *testing.T) {
	t.Parallel()
	b := newBed(t)
	driverPathMode(b)
	asked := gateAnswers(b, errors.New(gateHeld))
	b.expect(b.land(LandRequest{Pathspecs: []string{"payload.txt"}, Goal: "fx", GoalSet: true, SkipTransport: true}), 1,
		"step failed: landing gate before push (exit 1)", gateHeld)
	if len(b.git.called("push")) != 0 || len(*asked) != 1 || (*asked)[0] != "fx" {
		t.Fatalf("a hold recorded after admission did not stop the push: pushes=%d asked=%v", len(b.git.called("push")), *asked)
	}
	driverLogOrder(t, b.log, "held base=refs/remotes/origin/main", "verify tree=t1", "gate goal=fx")

	// A landing in no goal's name has no gate to read.
	plain := newBed(t)
	driverPathMode(plain)
	unasked := gateAnswers(plain, errors.New(gateHeld))
	plain.expect(plain.land(LandRequest{Pathspecs: []string{"payload.txt"}, SkipTransport: true}), 0)
	if len(*unasked) != 0 {
		t.Fatalf("a landing in no goal's name read a gate: %v", *unasked)
	}
}

func TestEveryPushRetryReadsTheGateAgain(t *testing.T) {
	t.Parallel()
	rejectFirst := func(b *bed) *int {
		pushes := 0
		b.git.on("push --porcelain origin refs/heads/main:refs/heads/main", func(GitCall) GitResult {
			pushes++
			if pushes == 1 {
				return GitResult{Code: 1, Stdout: []byte("To origin\n!\trefs/heads/main:refs/heads/main\t[rejected] (fetch first)\nDone\n")}
			}
			return ok("To origin\n \trefs/heads/main:refs/heads/main\th0..c1\nDone\n")
		})
		return &pushes
	}

	// The gate passed at the first attempt; a hold recorded while origin moved
	// stops the retry.
	held := newBed(t)
	driverPathMode(held)
	heldPushes := rejectFirst(held)
	asked := gateAnswers(held, nil, errors.New(gateHeld))
	held.expect(held.land(LandRequest{Pathspecs: []string{"payload.txt"}, Goal: "fx", GoalSet: true, SkipTransport: true}), 1, gateHeld)
	if *heldPushes != 1 || len(*asked) != 2 {
		t.Fatalf("the retry pushed past a hold: pushes=%d gate reads=%d", *heldPushes, len(*asked))
	}
	driverLogOrder(t, held.log, "gate goal=fx", "advance refs/remotes/origin/main", "held base=refs/remotes/origin/main", "gate goal=fx")

	// Released before the retry: the gate passes and the retry lands.
	released := newBed(t)
	driverPathMode(released)
	releasedPushes := rejectFirst(released)
	asked = gateAnswers(released, nil, nil)
	released.expect(released.land(LandRequest{Pathspecs: []string{"payload.txt"}, Goal: "fx", GoalSet: true, SkipTransport: true}), 0)
	if *releasedPushes != 2 || len(*asked) != 2 {
		t.Fatalf("the released retry: pushes=%d gate reads=%d, want two of each", *releasedPushes, len(*asked))
	}
	driverInOrder(t, released.stdout.String(), "step: landing gate before push", "step: push origin (attempt 1 of 3)",
		"step: verify shared testing proof after retry rebase", "step: landing gate before push", "step: push origin (attempt 2 of 3)")
}

func TestTheCarriedFormReadsTheGateBeforeItsSinglePush(t *testing.T) {
	t.Parallel()
	c := newCarriedBed(t)
	asked := gateAnswers(c.bed, errors.New(gateHeld))
	status, killed := c.landCarried(carriedRequest())
	if killed != "" {
		t.Fatalf("killed at %s", killed)
	}
	c.expect(status, 1, "step failed: landing gate before push (exit 1)", gateHeld)
	if c.pushes() != 0 || len(*asked) != 1 || (*asked)[0] != "g1" {
		t.Fatalf("the exceptional form pushed past a hold: pushes=%d asked=%v", c.pushes(), *asked)
	}
	// The stopped carried landing releases its reservation as any failed step does.
	c.inOrder("intent carrying=row-1", "gate goal=g1", "abandon row-1 why=step landing gate before push failed with exit 1: "+gateHeld)
	if strings.Contains(c.stdout.String(), "push carried commit") {
		t.Fatalf("the push step began:\n%s", c.stdout.String())
	}
}
