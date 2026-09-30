package landpath

// The carried scenarios of the former land-fixtures.sh bed, ported to the Go
// landing path. Each test names the scenario it ports; the ledger owners'
// own behavior (the carrying row, the carried row, the counselor record, the
// carry debt) is covered in internal/goal and noted where a scenario reached
// it.

import (
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing"
)

// TestCarriedFreshLandsOneCommitPastOneRefusal ports carried-fresh: one word
// lands one commit. The landing fetches origin and the ledger, reads the
// word, stages, reserves, re-fetches, commits under the live judge with every
// carried trailer exactly once, creates its local intent, prints the carried
// advisory before its single push attempt, then completes the ledger record;
// nothing is abandoned.
func TestCarriedFreshLandsOneCommitPastOneRefusal(t *testing.T) {
	t.Parallel()
	c := newCarriedBed(t)
	status, killed := c.landCarried(carriedRequest())
	if killed != "" {
		t.Fatalf("killed at %s", killed)
	}
	c.expect(status, 0)
	c.inOrder("require-holder", "with-held", "brain-fence", "goal-fetch", "carry-status word=op1 goal=g1 ledger="+carriedFetchedTip,
		"drift empty=false", "receipt-line goal=g1", "reserve ref=op1 tree=t1 by=human:wido", "observe judge=live goal=g1",
		"held base=refs/remotes/origin/main",
		"intent carrying=row-1 commit=c1 tree=t1 workspace=w-t1 past=missing-declaration battery=green missing=- failing=- judge=live ledger="+carriedReservedTip+" by=human:wido",
		"notify g1 c1", "carried entry=entry-1 rebuild= ref= repair=false")
	if got := strings.Join(c.seams, ","); got != "carry-forward,reservation,second-carry-forward,before-push,after-push,after-record" {
		t.Fatalf("seams %s", got)
	}
	for _, line := range []string{"Carry: op1", "Carried-By: human:wido", "Carried-Tree: workspace=w-t1 project=t1",
		"Carried-Past: missing-declaration", "Carried-Battery: green", "Carried-Ledger: " + carriedReservedTip, "Goal-Item: g1"} {
		if countExact(c.git.message, line) != 1 {
			t.Fatalf("carried commit lacks %q exactly once:\n%s", line, c.git.message)
		}
	}
	for _, key := range append([]string{"Landing-Provenance"}, carriedKeys...) {
		if countLines(c.git.message, `(?m)^`+key+`:`) != 1 {
			t.Fatalf("%s is not singular:\n%s", key, c.git.message)
		}
	}
	if !strings.Contains(c.git.message, "Carried-Judge: live sha256=") {
		t.Fatalf("judge trailer:\n%s", c.git.message)
	}
	c.linesInOrder("step: fetch origin for carried landing", "step: stage caller paths", "step: receipt line for the landing",
		"step: fetch origin after carry reservation", "step: commit", "step: goal held at the rebased base",
		"carried reservation: row-1\n", "carried ledger: "+carriedReservedTip+"\n", "carried judge: live sha256=",
		"carried live failure: -\n", "carried ordinary verdict: pass carried code=missing-declaration\n",
		"carried testing result: sufficient=true missing=- failing=- uncovered=- discrepancies=-\n",
		"carried obligation finding: carried:c1\n", "carried exception count after this one: 1\n",
		"step: push carried commit to origin (single attempt)", "step: complete carried goal record")
	if c.pushes() != 1 || c.commits() != 1 || len(c.calls("abandon")) != 0 || c.log.has("transport") {
		t.Fatalf("pushes=%d commits=%d log=%v", c.pushes(), c.commits(), c.log.calls)
	}
	if shows := c.git.called("show " + carriedReservedTip + ":./plans/goals/g1.md"); len(shows) != 1 || shows[0].Dir != c.root {
		t.Fatalf("advisory read the goal at the reserved ledger: %v", c.git.calls)
	}
	// Without --skip-transport the carried landing mirrors transport after
	// its record.
	c = newCarriedBed(t)
	request := carriedRequest()
	request.SkipTransport = false
	status, _ = c.landCarried(request)
	c.expect(status, 0)
	c.inOrder("carried entry=entry-1", "transport main")
}

// TestCarriedPrefixedJudgesTheMetasystemSubtree ports carried-prefixed: the
// metasystem directory is a subdirectory of the repository. The evaluator
// judges the metasystem subtree while the carried trailers and the intent
// name the whole-project tree, and the advisory reads the goal file relative
// to the metasystem root.
func TestCarriedPrefixedJudgesTheMetasystemSubtree(t *testing.T) {
	t.Parallel()
	c := newCarriedBed(t)
	c.git.prefix = "metasystem/"
	c.git.on("rev-parse t1:metasystem", func(GitCall) GitResult { return ok("sub1\n") })
	var observed []ObserveRequest
	live := c.owners.Live
	c.owners.Live = func() Judge {
		judge := live()
		judge.Observe = func(request ObserveRequest) (landing.Observation, int) {
			observed = append(observed, request)
			return c.observed, 0
		}
		return judge
	}
	status, _ := c.landCarried(carriedRequest())
	c.expect(status, 0)
	if len(observed) != 1 || observed[0].Tree != "sub1" || observed[0].ProjectTree != "t1" || observed[0].Judge != "live" ||
		observed[0].Carried != "op1" || observed[0].LedgerTip != carriedReservedTip || observed[0].CarriedBy != "human:wido" {
		t.Fatalf("observation %+v", observed)
	}
	if countExact(c.git.message, "Carried-Tree: workspace=w-t1 project=t1") != 1 || len(c.calls("intent carrying=row-1 commit=c1 tree=t1 workspace=w-t1")) != 1 {
		t.Fatalf("carried trailers or intent do not name the project tree:\n%s\n%v", c.git.message, c.log.calls)
	}
	if shows := c.git.called("show " + carriedReservedTip + ":./plans/goals/g1.md"); len(shows) != 1 || shows[0].Dir != c.root {
		t.Fatalf("goal read: %v", c.git.calls)
	}
}

// TestCarriedSecondWordLandsAfterTheFirst ports carried-second: after one
// carried landing, a second word on the same goal lands a second commit in
// its own transaction, and the advisory counts the exception the first
// landing recorded. The counselor lines staged with the second commit are
// ordinary staged bytes; that the carried record appends its counselor line
// once and replays idempotently is covered by
// internal/goal TestCarryLifecycleLandsThroughTheJournal.
func TestCarriedSecondWordLandsAfterTheFirst(t *testing.T) {
	t.Parallel()
	c := newCarriedBed(t)
	status, _ := c.landCarried(carriedRequest())
	c.expect(status, 0)
	first := c.git.head
	c.status.Consumption = "ledger:row-first"
	c.git.on("show "+carriedReservedTip+":./plans/goals/g1.md", func(GitCall) GitResult {
		return ok("# g1\n- BudgetExceptions: 1\n")
	})
	// The second word: a fresh status for op2 and an observation binding it.
	c.status = landing.CarryStatus{Word: "ok", Consumption: "none", Reservation: "reservation: none", Intent: "none",
		Past: "missing-declaration", By: "human:wido", Workspace: "w-t1", Source: "carry"}
	c.observed = carriedObservationFor("op2")
	c.git.originHead = c.git.head
	request := carriedRequest()
	request.Carried = "op2"
	status, _ = c.landCarried(request)
	c.expect(status, 0, "carried exception count after this one: 2", "carried obligation finding: carried:c2")
	if c.git.head == first || countExact(c.git.message, "Carry: op2") != 1 || countExact(c.git.message, "Carry: op1") != 0 {
		t.Fatalf("second commit:\n%s", c.git.message)
	}
	if c.pushes() != 2 || c.commits() != 2 || len(c.calls("carried entry=")) != 2 || len(c.calls("abandon")) != 0 {
		t.Fatalf("pushes=%d commits=%d log=%v", c.pushes(), c.commits(), c.log.calls)
	}
}

// TestCarriedRedBatteryCarriesTheRedGroups ports carried-red-battery: a word
// past a named testing group lands with a red battery. The commit records
// the missing and failing groups, the intent carries them to the ledger, and
// the advisory names the red obligation. That the recorded landing opens the
// carried:<commit>:battery-red obligation is covered by internal/goal
// TestCarriedRedBatteryOpensTheRedObligation.
func TestCarriedRedBatteryCarriesTheRedGroups(t *testing.T) {
	t.Parallel()
	c := newCarriedBed(t)
	c.status.Past = "group:fixture-carry"
	c.observed.Provenance = "carried opid=op1 past=group:fixture-carry ledger=" + carriedReservedTip + " by=human:wido"
	var directFix string
	live := c.owners.Live
	c.owners.Live = func() Judge {
		judge := live()
		observe := judge.Observe
		judge.Observe = func(request ObserveRequest) (landing.Observation, int) {
			directFix = request.DirectFix
			return observe(request)
		}
		judge.VerifyCarried = func(string, string, string) ([]byte, int) {
			return []byte(`{"delivery":{"sufficient":false,"missingGroups":["fixture-carry"],"failingGroups":[]}}`), 1
		}
		return judge
	}
	request := carriedRequest()
	request.DirectFix = "register-carriage"
	status, _ := c.landCarried(request)
	c.expect(status, 0, "carried testing result: sufficient=false missing=fixture-carry failing=- uncovered=- discrepancies=-",
		"carried obligation finding: carried:c1:battery-red")
	if directFix != "register-carriage" {
		t.Fatalf("the carried landing dropped --direct-fix: %q", directFix)
	}
	if countExact(c.git.message, "Carried-Battery: red missing=fixture-carry failing=-") != 1 ||
		countExact(c.git.message, "Carried-Past: group:fixture-carry") != 1 {
		t.Fatalf("red battery trailers:\n%s", c.git.message)
	}
	if len(c.calls("intent carrying=row-1 commit=c1 tree=t1 workspace=w-t1 past=group:fixture-carry battery=red missing=fixture-carry failing=-")) != 1 {
		t.Fatalf("intent: %v", c.log.calls)
	}
}

// TestCarriedCrashBeforePushClosesTheIntentAndReleases ports
// carried-intent-failure: the landing exits at the before-push seam after
// its local intent was created. Its cleanup first closes the intent through
// the carried record owner, then abandons the reservation; nothing reached
// origin. A rejected push that is a moving origin asks and cleans up the
// same way. That the carried record closes an unpushed intent terminal
// without a row, so the abandonment is not refused as in flight, is covered
// by internal/goal TestCarriedIntentBeforePushClosesWithoutARow.
func TestCarriedCrashBeforePushClosesTheIntentAndReleases(t *testing.T) {
	t.Parallel()
	c := newCarriedBed(t)
	c.crashAt = "before-push"
	status, _ := c.landCarried(carriedRequest())
	c.expect(status, 143, "carried exception count after this one: 1")
	c.inOrder("intent carrying=row-1 commit=c1", "carried entry=entry-1 rebuild= ref= repair=false",
		"abandon row-1 why=carried landing exited before its push (status 143)")
	if c.pushes() != 0 || len(c.calls("abandon")) != 1 || c.log.has("notify") {
		t.Fatalf("pushes=%d log=%v", c.pushes(), c.log.calls)
	}

	c = newCarriedBed(t)
	c.git.on("push", func(GitCall) GitResult { return failed(1, " ! [rejected] main -> main (fetch first)\n") })
	status, _ = c.landCarried(carriedRequest())
	c.expect(status, 3, "main moved while this landed, so nothing was pushed\nrun: metasystem work land g1 --using-exception op1  (lands it on the new main)\n")
	c.inOrder("intent carrying=row-1", "carried entry=entry-1", "abandon row-1 why=main moved while this landed, so nothing was pushed")
	if c.pushes() != 1 || c.log.has("notify") {
		t.Fatalf("pushes=%d log=%v", c.pushes(), c.log.calls)
	}

	// Any other push failure fails the step and still releases.
	c = newCarriedBed(t)
	c.git.on("push", func(GitCall) GitResult { return failed(1, "remote: permission denied\n") })
	status, _ = c.landCarried(carriedRequest())
	c.expect(status, 1, "step failed: push carried commit to origin (single attempt) (exit 1)")
	c.inOrder("carried entry=entry-1", "abandon row-1 why=step push carried commit to origin (single attempt) failed with exit 1: remote: permission denied")

	// An intent the ledger owner refuses stops before the push and releases
	// the reservation; there is no entry to close.
	c = newCarriedBed(t)
	c.intentCode, c.intentOutput = 1, "reservation workspace differs: row=a intent=b\n"
	status, _ = c.landCarried(carriedRequest())
	c.expect(status, 1, "reservation workspace differs")
	if c.pushes() != 0 || len(c.calls("carried entry=")) != 0 || len(c.calls("abandon row-1 why=carried landing exited before its push (status 1)")) != 1 {
		t.Fatalf("pushes=%d log=%v", c.pushes(), c.log.calls)
	}
}

// TestCarriedKillBeforePushRecoversTheLocalCommit ports carried-crash-local:
// the landing is killed at the before-push seam, so no cleanup runs: the
// intent stays created and the reservation open. Origin then moves; the
// rerun (path mode, empty index) finds the local carried commit, rebases it
// onto the moved origin without re-committing, keeps the open reservation,
// verifies the commit's trailers, writes a new intent for the rebased
// commit, pushes once and completes the record.
func TestCarriedKillBeforePushRecoversTheLocalCommit(t *testing.T) {
	t.Parallel()
	c := newCarriedBed(t)
	c.killAt = "before-push"
	status, killed := c.landCarried(carriedRequest())
	if killed != "before-push" || status != 137 {
		t.Fatalf("status=%d killed=%q", status, killed)
	}
	if c.pushes() != 0 || len(c.calls("abandon")) != 0 || len(c.calls("carried entry=")) != 0 {
		t.Fatalf("a killed landing cleaned up: %v", c.log.calls)
	}
	crashed, message := c.git.head, c.git.message
	c.killAt = ""
	c.git.originHead = "h9"
	c.git.stagedEmpty = true
	c.status.Consumption = "local:" + crashed
	c.status.Reservation = "reservation: open:row-1"
	c.status.Intent = "carrying:entry-1"
	request := carriedRequest()
	request.StagedOnly = false
	status, _ = c.landCarried(request)
	c.expect(status, 0)
	rebased := crashed + "'"
	if c.git.head != rebased || c.git.message != message || c.commits() != 1 || len(c.git.called("rebase refs/remotes/origin/main")) != 1 {
		t.Fatalf("recovery re-committed or did not rebase: head=%s commits=%d calls=%v", c.git.head, c.commits(), c.git.calls)
	}
	if len(c.calls("reserve")) != 1 || c.pushes() != 1 || c.git.originHead != rebased {
		t.Fatalf("reserve=%d pushes=%d origin=%s", len(c.calls("reserve")), c.pushes(), c.git.originHead)
	}
	c.inOrder("carry-status", "held base=refs/remotes/origin/main", "intent carrying=row-1 commit="+rebased+" tree=t1 workspace=w-t1",
		"notify g1 "+rebased, "carried entry=entry-2")
	if len(c.calls("abandon")) != 0 || len(c.calls("carried entry=")) != 1 {
		t.Fatalf("log %v", c.log.calls)
	}
	c.linesInOrder("carried reservation: row-1\n", "carried obligation finding: carried:"+rebased+"\n",
		"step: push carried commit to origin (single attempt)")
	if got := strings.Join(c.seams, ","); got != "before-push,after-push,after-record" {
		t.Fatalf("recovery seams %s", got)
	}
}

// TestCarriedLocalRecoveryReservation covers the recovery reservation
// branches of carried-crash-local: a closed or absent reservation is
// reserved again and origin re-fetched and rebased before the publication;
// an expired one asks for a new exception and leaves the commit at HEAD; a
// rebase conflict asks; a recovered commit whose trailers do not bind the
// word refuses.
func TestCarriedLocalRecoveryReservation(t *testing.T) {
	t.Parallel()
	recovering := func(t *testing.T) *carriedBed {
		c := newCarriedBed(t)
		c.git.head = "c1"
		c.git.message = "land the change\n\nCarried-By: human:wido\nCarry: op1\nCarried-Tree: workspace=w-t1 project=t1\n" +
			"Carried-Past: missing-declaration\nCarried-Battery: green\nCarried-Judge: live sha256=abc\nCarried-Ledger: " + carriedReservedTip +
			"\nLanding-Provenance-Verdict: pass carried\nGoal-Item: g1"
		c.status.Consumption = "local:c1"
		c.status.Intent = "carrying:entry-0"
		return c
	}
	t.Run("abandoned reservation is reserved again", func(t *testing.T) {
		c := recovering(t)
		c.status.Reservation = "reservation: abandoned:row-0"
		fetches := 0
		c.git.on("fetch", func(GitCall) GitResult {
			fetches++
			if fetches == 2 {
				c.git.originHead = "h9"
			}
			return ok("")
		})
		status, _ := c.landCarried(carriedRequest())
		c.expect(status, 0)
		c.inOrder("reserve ref=op1 tree=t1 by=human:wido", "intent carrying=row-1 commit=c1'' tree=t1", "carried entry=entry-1")
		if fetches != 2 || len(c.git.called("rebase refs/remotes/origin/main")) != 2 || c.commits() != 0 || c.pushes() != 1 {
			t.Fatalf("fetches=%d calls=%v", fetches, c.git.calls)
		}
	})
	t.Run("expired reservation asks", func(t *testing.T) {
		c := recovering(t)
		c.status.Reservation = "reservation: expired:row-0"
		status, _ := c.landCarried(carriedRequest())
		c.expect(status, 3, "exception op1 has expired; its commit stays at HEAD\nneeded first: record a new one with metasystem work land g1 --exception CODE --reason R\n")
		if c.pushes() != 0 || len(c.calls("abandon")) != 0 || len(c.calls("intent")) != 0 || len(c.calls("reserve")) != 0 {
			t.Fatalf("log %v", c.log.calls)
		}
	})
	t.Run("rebase conflict asks", func(t *testing.T) {
		c := recovering(t)
		c.status.Reservation = "reservation: open:row-0"
		c.git.originHead = "h9"
		c.rebaseConflict = true
		status, _ := c.landCarried(carriedRequest())
		c.expect(status, 3, "the unfinished exception commit conflicts with main")
		if len(c.git.called("rebase --abort")) != 1 || c.pushes() != 0 || len(c.calls("abandon")) != 0 {
			t.Fatalf("calls %v log %v", c.git.calls, c.log.calls)
		}
	})
	t.Run("foreign carry trailer refuses", func(t *testing.T) {
		c := recovering(t)
		c.status.Reservation = "reservation: open:row-0"
		c.git.message = strings.Replace(c.git.message, "Carry: op1", "Carry: op9", 1)
		status, _ := c.landCarried(carriedRequest())
		c.expect(status, 1, "local carried commit names Carry: op9, not op1")
		c.inOrder("abandon row-0 why=carried landing exited before its push (status 1)")
		if c.pushes() != 0 {
			t.Fatal("pushed a foreign commit")
		}
	})
	t.Run("doubled trailer refuses", func(t *testing.T) {
		c := recovering(t)
		c.status.Reservation = "reservation: open:row-0"
		c.git.message += "\nCarried-Battery: red"
		status, _ := c.landCarried(carriedRequest())
		c.expect(status, 1, "local carried commit c1' has 2 Carried-Battery trailers; expected exactly one")
	})
	t.Run("missing goal item refuses", func(t *testing.T) {
		c := recovering(t)
		c.status.Reservation = "reservation: open:row-0"
		c.git.message = strings.Replace(c.git.message, "Goal-Item: g1", "Goal-Item: g2", 1)
		status, _ := c.landCarried(carriedRequest())
		c.expect(status, 1, "local carried commit must have exactly one Goal-Item: g1")
	})
	t.Run("moved workspace asks for a replacement", func(t *testing.T) {
		c := recovering(t)
		c.status.Reservation = "reservation: open:row-0"
		c.status.Source = "answer"
		c.status.Workspace = "w-old"
		status, _ := c.landCarried(carriedRequest())
		c.expect(status, 3, "word workspace=w-old candidate workspace=w-t1 (tree t1)", "run: metasystem work land g1 --exception missing-declaration --replace-exception op1 --reason 'the recovered carried workspace changed' --by wido")
		c.inOrder("channel-ask goal=g1 wants=carry workspace=w-t1 goal=g1 past=missing-declaration fact=the recovered commit workspace differs from the channel carry word",
			"abandon row-0 why=the files changed since exception op1 was recorded")
	})
}

// TestCarriedAsksBeforeAnyCommit ports carried-asks: every stop a person
// must answer exits 3 with the exact ask. The evaluator's refusal of a word
// whose named refusal is not the one the landing saw is asked at the commit
// step, after the reservation, and the reservation is abandoned; asks before
// the reservation abandon nothing. No ask commits or pushes.
func TestCarriedAsksBeforeAnyCommit(t *testing.T) {
	t.Parallel()
	t.Run("the refusal the word names is not the one the landing saw", func(t *testing.T) {
		c := newCarriedBed(t)
		c.status.Past = "conflicting-declarations"
		c.observed = landing.Observation{Mode: "refuse", RefusesAgent: true, Code: "carried-refusal-mismatch",
			Refusal: "word past=conflicting-declarations but the landing saw ordinary=missing-declaration", Provenance: "none change=x",
			VerdictTrailer: "would-refuse code=carried-refusal-mismatch"}
		status, _ := c.landCarried(carriedRequest())
		c.expect(status, 3, "the landing saw ordinary=missing-declaration", "step failed: commit (exit 3)")
		c.inOrder("reserve ref=op1", "observe judge=live", "abandon row-1 why=step commit failed with exit 3: carried-refusal-mismatch: word past=conflicting-declarations but the landing saw ordinary=missing-declaration")
		if c.commits() != 0 || c.pushes() != 0 || len(c.calls("intent")) != 0 || len(c.calls("abandon")) != 1 {
			t.Fatalf("commits=%d pushes=%d log=%v", c.commits(), c.pushes(), c.log.calls)
		}
	})
	cases := []struct {
		name  string
		setup func(c *carriedBed)
		text  string
	}{
		{"origin unreachable", func(c *carriedBed) {
			c.git.on("fetch", func(GitCall) GitResult { return failed(128, "fatal: unable to access origin\n") })
		}, "origin couldn't be fetched, so nothing was landed\nrun: metasystem work land g1 --using-exception op1  (once origin answers)\n"},
		{"goal fetch failed", func(c *carriedBed) { c.fetchOutput, c.fetchCode = "goal fetch: remote refused\n", 1 }, "goal fetch: remote refused"},
		{"goal fetch without a tip", func(c *carriedBed) { c.fetchOutput = "fetched tip=abc\n" }, "goal fetch returned no accepted ledger tip: fetched tip=abc"},
		{"carry status failed", func(c *carriedBed) {
			c.statusOutput, c.statusCode = "carry-ledger-moved: accepted ledger is x, not y", 1
		}, "carry-ledger-moved: accepted ledger is x, not y"},
		{"carry status incomplete", func(c *carriedBed) { c.status.Reservation = "" }, "exception op1 reads incomplete in the goal records"},
		{"superseded word", func(c *carriedBed) { c.status.Consumption = "superseded:op2" },
			"exception op1 was replaced by op2\nrun: metasystem work land g1 --using-exception op2\n"},
		{"expired word", func(c *carriedBed) { c.status.Word = "expired" }, "exception op1 has expired, so nothing was landed"},
		{"missing word", func(c *carriedBed) { c.status.Word = "missing" }, "goal g1 has no exception op1, so nothing was landed"},
		{"unproven word", func(c *carriedBed) { c.status.Word = "unproven" }, "exception op1 wasn't recorded by a person at an enrolled terminal"},
		{"unknown word state", func(c *carriedBed) { c.status.Word = "odd" }, "carry word state: odd"},
		{"unsupported consumption", func(c *carriedBed) { c.status.Consumption = "elsewhere:x" }, "consumption state: elsewhere:x"},
		{"not on main", func(c *carriedBed) { c.git.branch = "topic" }, "an exception lands main, and this checkout is on topic\nrun: git switch main  (then repeat this command)\n"},
		{"reservation refused", func(c *carriedBed) {
			c.reserveOutput, c.reserveCode = "carry-debt-unpaid: carry debt is unpaid: reservation r-a on goal g0 is in flight (seat=m1 expires=x)\n", 3
		}, "carry-debt-unpaid: carry debt is unpaid: reservation r-a on goal g0 is in flight (seat=m1 expires=x)"},
		{"reservation incomplete", func(c *carriedBed) { c.reserveOutput = "carrying=row-1 ledger=short\n" },
			"reservation returned an incomplete row: carrying=row-1 ledger=short"},
		{"workspace moved before the reservation", func(c *carriedBed) { c.status.Workspace = "w-old" },
			"word workspace=w-old candidate workspace=w-t1 (tree t1)"},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			c := newCarriedBed(t)
			test.setup(c)
			status, _ := c.landCarried(carriedRequest())
			c.expect(status, 3, test.text)
			if c.commits() != 0 || c.pushes() != 0 || len(c.calls("abandon")) != 0 || len(c.calls("intent")) != 0 {
				t.Fatalf("commits=%d pushes=%d log=%v", c.commits(), c.pushes(), c.log.calls)
			}
		})
	}
}

// TestCarriedOriginMoveAfterTheReservationReleasesIt covers the second carry
// forward of carried-asks: when origin moved between the reservation and the
// commit, the staged candidate is carried onto origin through a WIP commit;
// a conflict or a changed workspace asks and abandons the reservation with
// its reason, and a channel-sourced word asks the channel again. A clean
// move lands the candidate on the new origin.
func TestCarriedOriginMoveAfterTheReservationReleasesIt(t *testing.T) {
	t.Parallel()
	moving := func(t *testing.T) *carriedBed {
		c := newCarriedBed(t)
		fetches := 0
		c.git.on("fetch", func(GitCall) GitResult {
			fetches++
			if fetches == 2 {
				c.git.originHead = "h9"
			}
			return ok("")
		})
		return c
	}
	t.Run("conflict", func(t *testing.T) {
		c := moving(t)
		c.rebaseConflict = true
		c.git.on("diff --name-only --diff-filter=U", func(GitCall) GitResult { return ok("payload.txt\nother.txt\n") })
		status, _ := c.landCarried(carriedRequest())
		c.expect(status, 3, "the change conflicts with main in payload.txt,other.txt, so nothing was landed")
		c.inOrder("reserve ref=op1", "abandon row-1 why=carried landing stopped at rebase conflict")
		if len(c.git.called("rebase --abort")) != 1 || len(c.git.called("reset --soft h0")) != 1 || len(c.git.called("update-ref HEAD wip h0")) != 1 {
			t.Fatalf("the WIP was not unwound: %v", c.git.calls)
		}
		if c.commits() != 0 || c.pushes() != 0 {
			t.Fatal("a conflicted carried landing committed")
		}
	})
	t.Run("workspace moved", func(t *testing.T) {
		c := moving(t)
		c.status.Source = "answer"
		trees := 0
		c.git.on("write-tree", func(GitCall) GitResult {
			trees++
			if c.git.originHead == "h9" && len(c.git.called("reset --soft refs/remotes/origin/main")) > 0 {
				return ok("t2\n")
			}
			return ok("t1\n")
		})
		status, _ := c.landCarried(carriedRequest())
		c.expect(status, 3, "word workspace=w-t1 candidate workspace=w-t2 (tree t2)", "run: metasystem work land g1 --exception missing-declaration --replace-exception op1 --reason 'origin moved the carried workspace' --by wido  (a person replaces the exception)")
		c.inOrder("reserve ref=op1", "abandon row-1 why=origin moved the carried workspace",
			"channel-ask goal=g1 wants=carry workspace=w-t2 goal=g1 past=missing-declaration fact=origin moved the candidate workspace after the channel carry word")
		if c.commits() != 0 || c.pushes() != 0 {
			t.Fatal("a moved workspace committed")
		}
	})
	t.Run("clean move lands", func(t *testing.T) {
		c := moving(t)
		status, _ := c.landCarried(carriedRequest())
		c.expect(status, 0)
		if len(c.git.called("commit-tree t1 -p HEAD -m carried wip")) != 1 || len(c.git.called("reset --soft refs/remotes/origin/main")) != 1 ||
			c.commits() != 1 || c.pushes() != 1 || len(c.calls("abandon")) != 0 {
			t.Fatalf("calls %v log %v", c.git.calls, c.log.calls)
		}
	})
}

// TestCarriedLedgerPathIsRefusedByName ports carried-ledger-path: a carried
// landing that stages a goal-ledger path is refused by the evaluator by
// name, exits 3 at the commit, and abandons its reservation.
func TestCarriedLedgerPathIsRefusedByName(t *testing.T) {
	t.Parallel()
	c := newCarriedBed(t)
	c.observed = landing.Observation{Mode: "refuse", RefusesAgent: true, Code: "ledger-path-not-goal-verb",
		Refusal: "plans/goals/illicit.md changes only through goal verbs", Provenance: "none change=x", VerdictTrailer: "would-refuse code=ledger-path-not-goal-verb"}
	status, _ := c.landCarried(carriedRequest())
	c.expect(status, 3, "ledger-path-not-goal-verb: plans/goals/illicit.md changes only through goal verbs")
	c.inOrder("reserve ref=op1", "abandon row-1 why=step commit failed with exit 3: ledger-path-not-goal-verb")
	if c.commits() != 0 || c.pushes() != 0 || len(c.calls("intent")) != 0 {
		t.Fatalf("log %v", c.log.calls)
	}
}

// TestCarriedCrashAfterPushCompletesTheRecord ports carried-crash: the
// landing exits at the after-push seam, with its commit on origin and no
// ledger row. The push disarmed the release, so nothing is abandoned. The
// rerun reads origin:<commit> and completes the record from the open intent
// without another commit or push; with no intent it rebuilds the record from
// the commit; once the ledger holds the row it only repairs a missing
// counselor line.
func TestCarriedCrashAfterPushCompletesTheRecord(t *testing.T) {
	t.Parallel()
	c := newCarriedBed(t)
	c.crashAt = "after-push"
	status, _ := c.landCarried(carriedRequest())
	c.expect(status, 143)
	if c.pushes() != 1 || len(c.calls("abandon")) != 0 || len(c.calls("carried entry=")) != 0 {
		t.Fatalf("pushes=%d log=%v", c.pushes(), c.log.calls)
	}
	pushed := c.git.head
	c.git.stagedEmpty = true
	c.status.Consumption = "origin:" + pushed
	c.status.Reservation = "reservation: open:row-1"
	c.status.Intent = "carrying:entry-1"
	request := carriedRequest()
	request.StagedOnly = false
	status, _ = c.landCarried(request)
	c.expect(status, 0, "already landed as "+pushed+"; completing the record", "step: complete carried goal record")
	if c.pushes() != 1 || c.commits() != 1 || len(c.calls("carried entry=entry-1 rebuild= ref= repair=false")) != 1 || len(c.calls("abandon")) != 0 {
		t.Fatalf("pushes=%d commits=%d log=%v", c.pushes(), c.commits(), c.log.calls)
	}
	if len(c.seams) != 0 || c.log.has("transport") {
		t.Fatalf("seams %v log %v", c.seams, c.log.calls)
	}

	c.status.Intent = "none"
	request.SkipTransport = false
	status, _ = c.landCarried(request)
	c.expect(status, 0, "step: rebuild carried goal record")
	c.inOrder("carried entry= rebuild="+pushed+" ref=op1 repair=false", "transport main")

	c.status.Consumption = "ledger:row-9"
	c.status.Counselor = "counselor: missing"
	status, _ = c.landCarried(request)
	c.expect(status, 0, "step: repair carried counselor record", "already recorded in the goal ledger as row-9")
	if len(c.calls("carried entry= rebuild= ref=op1 repair=true")) != 1 {
		t.Fatalf("log %v", c.log.calls)
	}
	c.status.Counselor = "counselor: written"
	status, _ = c.landCarried(request)
	c.expect(status, 0, "already recorded in the goal ledger as row-9")
	if len(c.calls("carried entry= rebuild= ref=op1 repair=true")) != 1 || c.pushes() != 1 {
		t.Fatalf("a recorded word repaired again or pushed: %v", c.log.calls)
	}

	// A record the owner refuses fails the step; the pushed commit stands
	// and nothing is abandoned.
	c.status.Consumption = "origin:" + pushed
	c.status.Intent = "carrying:entry-1"
	c.carriedCode = 1
	status, _ = c.landCarried(request)
	c.expect(status, 1, "step failed: complete carried goal record (exit 1)")
	if len(c.calls("abandon")) != 0 {
		t.Fatalf("log %v", c.log.calls)
	}
}

// TestCarriedSecondSeatStopsOnTheFirstSeatsDebt ports carried-two-seat,
// carried-debt-abandoned and carried-debt-expired at the landing path: seat
// A pushed and crashed before its record; seat B's reservation is refused by
// the ledger owner with carry-debt-unpaid, which the landing relays as an
// ask (exit 3) before it commits, pushes or holds a row to abandon; seat A's
// rerun completes its record (TestCarriedCrashAfterPushCompletesTheRecord);
// seat B's rerun is refused again while the review obligation stands. Which
// debt the owner names (the in-flight row and seat, the unrecorded word after
// its row was abandoned or the word expired, the carried:<commit> review
// obligation) is covered in internal/goal: TestHCL60CarryDebtAsksAtTheWord,
// TestCarryLifecycleLandsThroughTheJournal and
// TestCarryDebtUnrecordedAfterTheReservationEnds.
func TestCarriedSecondSeatStopsOnTheFirstSeatsDebt(t *testing.T) {
	t.Parallel()
	for _, debt := range []string{
		"carry-debt-unpaid: carry debt is unpaid: reservation row-a on goal fx is in flight (seat=fixture-machine expires=x)",
		"carry-debt-unpaid: carry debt is unpaid: commit ca carries word op-a without its ledger row",
		"carry-debt-unpaid: carry debt is unpaid: goal fx has open obligation carried:ca at commit:ca",
	} {
		c := newCarriedBed(t)
		c.reserveOutput, c.reserveCode = debt+"\n", 3
		c.git.originHead = "ca"
		status, _ := c.landCarried(carriedRequest())
		c.expect(status, 3, debt)
		if c.commits() != 0 || c.pushes() != 0 || len(c.calls("abandon")) != 0 || c.log.has("observe") {
			t.Fatalf("seat B went on past the debt: commits=%d pushes=%d log=%v", c.commits(), c.pushes(), c.log.calls)
		}
	}
}
