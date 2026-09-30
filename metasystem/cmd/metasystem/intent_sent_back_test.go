package main

// The holder's step (g1-s69 D2, §8): the seat that holds a sent-back goal runs
// work revise once on the published brief and never twice, selects the one
// eligible work item by leaving --work off, and with several records the
// needs-work line whose repeat carries --work.

import (
	"os"
	"slices"
	"strings"
	"testing"
)

type reviseCall struct {
	raw   []string
	brief string
}

// sentBackBed is a goal waiting to land, held by mac-cli+m1, and sent back by
// Wido with a brief.
func sentBackBed(t *testing.T) *intentBed {
	t.Helper()
	bed := newIntentBed(t, false, waitingToLandBed)
	record := writeReviewRecord(t, bed.root(), "send back")
	brief := bed.root() + "/fix.md"
	if err := os.WriteFile(brief, []byte("# Correction brief\n\n1. Read the reviewed tree (internal/owner.go:60).\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if code, result := bed.runJSON(bed.terminalOwners(), "goal", "review", bedGoal, "--record", record, "--verdict", "send-back", "--brief", brief); code != 0 {
		t.Fatalf("send back = %d %+v", code, result)
	}
	bed.lineage = "m1"
	return bed
}

func holderOwners(bed *intentBed, calls *[]reviseCall, answer func(int) intentResult) intentOwners {
	owners := bed.owners()
	owners.sentBackRevise = func(_ *intentInvocation, raw []string) intentResult {
		brief := ""
		if at := slices.Index(raw, "--brief"); at >= 0 && at+1 < len(raw) {
			read, _ := os.ReadFile(raw[at+1])
			brief = string(read)
		}
		*calls = append(*calls, reviseCall{raw: raw, brief: brief})
		return answer(len(*calls))
	}
	return owners
}

func attemptStarted(attempt int) intentResult {
	return intentResult{Outcome: intentInProgress, Summary: "started the correction",
		Data: map[string]any{"revision": map[string]any{"attempt": float64(attempt)}}}
}

func TestTheHolderRevisesOnceFromThePublishedBrief(t *testing.T) {
	t.Parallel()
	bed := sentBackBed(t)
	var calls []reviseCall
	owners := holderOwners(bed, &calls, func(int) intentResult { return attemptStarted(3) })

	code, result := bed.runJSON(owners, "work", "revise", bedGoal)
	if code == 0 && result.Outcome != intentInProgress {
		t.Fatalf("the holder's step = %d %+v", code, result)
	}
	if len(calls) != 1 || !strings.Contains(calls[0].brief, "Read the reviewed tree") || slices.Contains(calls[0].raw, "--work") {
		t.Fatalf("work revise was not run once on the published brief without --work: %+v", calls)
	}
	if !strings.Contains(result.Summary, "sent back by Wido") || !strings.Contains(result.Summary, "attempt 3 is recorded on the goal") {
		t.Fatalf("the step does not say what it did: %q", result.Summary)
	}
	last := bed.goalFile(bedGoal).History[len(bed.goalFile(bedGoal).History)-1]
	if last.Verb != "send-back" || !strings.HasPrefix(last.Reason, "send-back attempt=3 review=") || last.Actor != "mac-cli+m1" {
		t.Fatalf("the attempt was not recorded by the holder: %+v", last)
	}

	// A later pass finds the attempt line and revises nothing twice: the
	// plain work revise answers, and the brief is never sent again.
	code, result = bed.runJSON(owners, "work", "revise", bedGoal)
	if code == 0 || len(calls) != 1 || result.Next == nil || !slices.Contains(result.Next.Argv, "--brief") {
		t.Fatalf("a second pass revised again or did not fall back: %d %+v calls=%d", code, result, len(calls))
	}
}

func TestTheHolderAsksWhichWorkAndTheRepeatCarriesIt(t *testing.T) {
	t.Parallel()
	bed := sentBackBed(t)
	var calls []reviseCall
	owners := holderOwners(bed, &calls, func(call int) intentResult {
		if call == 1 {
			return intentResult{Outcome: intentRefused, code: 2, Summary: "goal has 2 work items this could mean (discovery, writer); nothing was done",
				Data: map[string]any{"candidates": []any{"discovery", "writer"}}}
		}
		return attemptStarted(2)
	})
	code, result := bed.runJSON(owners, "work", "revise", bedGoal)
	if code != 0 || !strings.Contains(result.Summary, "2 work items (discovery, writer), so the reviewer names one") {
		t.Fatalf("the needs-work step = %d %+v", code, result)
	}
	file := bed.goalFile(bedGoal)
	if last := file.History[len(file.History)-1]; !strings.HasPrefix(last.Reason, "send-back needs-work candidates=discovery,writer review=") {
		t.Fatalf("the needs-work line = %+v", last)
	}

	// The human names one: the act again with --work, a send-back of its own.
	record := bed.root() + "/" + reviewBedRecord
	if code, named := bed.runJSON(bed.terminalOwners(), "goal", "review", bedGoal, "--record", record, "--verdict", "send-back",
		"--brief", bed.root()+"/fix.md", "--work", "writer"); code != 0 || named.Outcome != intentConfirmed {
		t.Fatalf("the send-back naming the work = %d %+v", code, named)
	}
	if code, result = bed.runJSON(owners, "work", "revise", bedGoal); code != 0 && result.Outcome != intentInProgress {
		t.Fatalf("the named step = %d %+v", code, result)
	}
	if len(calls) != 2 || !slices.Contains(calls[1].raw, "--work") || calls[1].raw[slices.Index(calls[1].raw, "--work")+1] != "writer" {
		t.Fatalf("the repeat did not carry --work writer: %+v", calls)
	}
	file = bed.goalFile(bedGoal)
	if last := file.History[len(file.History)-1]; !strings.HasPrefix(last.Reason, "send-back attempt=2 review=") || !strings.HasSuffix(last.Reason, " work=writer") {
		t.Fatalf("the attempt line = %+v", last)
	}
}

func TestOnlyTheHolderTakesTheStep(t *testing.T) {
	t.Parallel()
	bed := sentBackBed(t)
	bed.lineage = "another-session"
	var calls []reviseCall
	code, result := bed.runJSON(holderOwners(bed, &calls, func(int) intentResult { return attemptStarted(3) }), "work", "revise", bedGoal)
	if code == 0 || len(calls) != 0 || !strings.Contains(result.Summary, "another session holds it") || !strings.Contains(result.Decision, "revises it") {
		t.Fatalf("a session that does not hold the goal took the step: %d %+v", code, result)
	}
}
