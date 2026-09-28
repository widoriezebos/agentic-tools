package landpath

// The carried bed: one test's carried landing over the harness bed, with the
// goal ledger owners of the carried transaction scripted per instance. The
// carry status the next run reads is a field the test sets, so a crash is
// modeled as land.sh's fixture seams were: the transaction stops at a named
// seam, and a second Land run reads the carry status that crash left.

import (
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing"
)

const (
	carriedFetchedTip  = "1111111111111111111111111111111111111111"
	carriedReservedTip = "2222222222222222222222222222222222222222"
)

// carriedKilled is the panic a killed landing stops with: Land re-panics it
// without running its cleanup, as SIGKILL skipped land.sh's EXIT trap.
type carriedKilled struct{ point string }

// carriedBed is one test's carried landing.
type carriedBed struct {
	*bed
	status       landing.CarryStatus
	statusOutput string
	statusCode   int

	fetchOutput string
	fetchCode   int

	// reserveOutput and reserveCode replace the reservation's answer.
	reserveOutput string
	reserveCode   int
	intentCode    int
	intentOutput  string
	intents       int
	carriedCode   int

	// crashAt stops the transaction at a seam with an exit (the EXIT trap
	// runs); killAt stops it without cleanup.
	crashAt, killAt string
	seams           []string
	rebaseConflict  bool
}

func newCarriedBed(t *testing.T) *carriedBed {
	t.Helper()
	c := &carriedBed{bed: newBed(t)}
	c.status = landing.CarryStatus{Word: "ok", Consumption: "none", Reservation: "reservation: none", Intent: "none",
		Past: "missing-declaration", By: "human:wido", Workspace: "w-t1", Source: "carry"}
	c.fetchOutput = "fetched tip=" + carriedFetchedTip + " behind=0\n"
	c.observed = carriedObservationFor("op1")
	owners := &c.owners
	owners.GoalFetch = func(string) (string, int) {
		c.log.add("goal-fetch")
		return c.fetchOutput, c.fetchCode
	}
	owners.CarryStatus = func(_, carried, goal, ledgerTip string) (landing.CarryStatus, string, int) {
		c.log.add("carry-status word=%s goal=%s ledger=%s", carried, goal, ledgerTip)
		return c.status, c.statusOutput, c.statusCode
	}
	owners.GoalCarrying = func(r CarryingRequest) (string, int) {
		switch {
		case r.Abandon != "":
			c.log.add("abandon %s why=%s", r.Abandon, r.Why)
			return "", 0
		case r.Commit != "":
			c.intents++
			c.log.add("intent carrying=%s commit=%s tree=%s workspace=%s past=%s battery=%s missing=%s failing=%s judge=%s ledger=%s by=%s",
				r.Carrying, r.Commit, r.Tree, r.Workspace, r.Past, r.Battery, r.Missing, r.Failing, r.Judge, r.Ledger, r.By)
			if c.intentCode != 0 || c.intentOutput != "" {
				return c.intentOutput, c.intentCode
			}
			return "carrying=entry-" + string(rune('0'+c.intents)) + " ledger=" + carriedReservedTip + "\n", 0
		default:
			c.log.add("reserve ref=%s tree=%s by=%s", r.Ref, r.Tree, r.By)
			if c.reserveCode != 0 || c.reserveOutput != "" {
				return c.reserveOutput, c.reserveCode
			}
			return "carrying=row-1 ledger=" + carriedReservedTip + "\n", 0
		}
	}
	owners.GoalCarried = func(r CarriedRequest) (string, int) {
		c.log.add("carried entry=%s rebuild=%s ref=%s repair=%t", r.Entry, r.RebuildFromCommit, r.Ref, r.RepairCounselor)
		return "confirmed\n", c.carriedCode
	}
	owners.ChannelAskCarry = func(_, goal, wants, fact string) {
		c.log.add("channel-ask goal=%s wants=%s fact=%s", goal, wants, fact)
	}
	owners.Seam = func(point string) {
		c.seams = append(c.seams, point)
		if point == c.killAt {
			panic(carriedKilled{point})
		}
		if point == c.crashAt {
			exitLanding(143)
		}
	}
	// Any commit's tree is the staged tree; the rebases the carried
	// transaction runs move HEAD to a rebased commit.
	c.git.on("rev-parse", func(call GitCall) GitResult {
		if len(call.Args) == 2 && strings.HasSuffix(call.Args[1], "^{tree}") && call.Args[1] != "HEAD^{tree}" {
			return ok(c.git.tree + "\n")
		}
		return c.git.defaults(call)
	})
	c.git.on("commit-tree", func(GitCall) GitResult { return ok("wip\n") })
	c.git.on("rebase", func(call GitCall) GitResult {
		if len(call.Args) == 2 && call.Args[1] == "--abort" {
			return ok("")
		}
		if c.rebaseConflict {
			return failed(1, "CONFLICT (content): Merge conflict in payload.txt\n")
		}
		c.git.head += "'"
		return ok("")
	})
	c.git.on("push", func(call GitCall) GitResult {
		c.git.originHead = c.git.head
		return ok("")
	})
	return c
}

// carriedObservationFor is the evaluator's decision that binds word to the
// bed's refusal and reserved ledger.
func carriedObservationFor(word string) landing.Observation {
	return landing.Observation{Mode: "observe", Code: "human-carried",
		Provenance:     "carried opid=" + word + " past=missing-declaration ledger=" + carriedReservedTip + " by=human:wido",
		VerdictTrailer: "pass carried code=missing-declaration", GoalRevision: 3}
}

func carriedRequest() LandRequest {
	return LandRequest{Goal: "g1", GoalSet: true, Carried: "op1", StagedOnly: true, SkipTransport: true}
}

// landCarried runs one carried landing with fresh output buffers and reports
// the seam a kill stopped it at.
func (c *carriedBed) landCarried(request LandRequest) (status int, killed string) {
	c.t.Helper()
	c.stdout.Reset()
	c.stderr.Reset()
	c.seams = nil
	defer func() {
		if recovered := recover(); recovered != nil {
			kill, isKill := recovered.(carriedKilled)
			if !isKill {
				panic(recovered)
			}
			status, killed = 137, kill.point
		}
	}()
	return c.land(request), ""
}

// calls are the owner calls with prefix, in order.
func (c *carriedBed) calls(prefix string) []string {
	var matched []string
	for _, call := range c.log.calls {
		if strings.HasPrefix(call, prefix) {
			matched = append(matched, call)
		}
	}
	return matched
}

// inOrder fails unless the owner log reaches every prefix in order.
func (c *carriedBed) inOrder(prefixes ...string) {
	c.t.Helper()
	position := 0
	for _, call := range c.log.calls {
		if position < len(prefixes) && strings.HasPrefix(call, prefixes[position]) {
			position++
		}
	}
	if position != len(prefixes) {
		c.t.Fatalf("owner order: reached %d of %q in\n%s", position, prefixes, strings.Join(c.log.calls, "\n"))
	}
}

// linesInOrder fails unless stdout holds every text in order.
func (c *carriedBed) linesInOrder(texts ...string) {
	c.t.Helper()
	output := c.stdout.String()
	at := 0
	for _, text := range texts {
		index := strings.Index(output[at:], text)
		if index < 0 {
			c.t.Fatalf("stdout lacks %q after offset %d:\n%s", text, at, output)
		}
		at += index + len(text)
	}
}

// pushes counts the pushes to origin.
func (c *carriedBed) pushes() int { return len(c.git.called("push --porcelain origin")) }

// commits counts git commit calls.
func (c *carriedBed) commits() int { return len(c.git.called("commit ")) }
