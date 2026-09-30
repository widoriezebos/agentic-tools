package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batchowner"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/kernel"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
)

const kernelVerbBatch = "01j5x00000000000000000vb01"

func (bed *kernelBed) runWith(t *testing.T, owners intentOwners, words ...string) (int, string) {
	t.Helper()
	command, ok := findIntentAction(words[0], words[1])
	if !ok {
		t.Fatalf("no public command %s %s", words[0], words[1])
	}
	var stdout, stderr bytes.Buffer
	code := runIntentIn(command, words[2:], &stdout, &stderr, bed.cwd, owners)
	return code, stdout.String() + stderr.String()
}

// landing begin and landing prove (K4, K6) act on the admitted lane: the
// registered nested layout, the host home and the lane's claim identity
// reach the kernel, and each outcome reads as two plain lines.
func TestLandingBeginAndProveVerbs(t *testing.T) {
	t.Parallel()
	bed := newKernelBed(t)
	bed.enroll(t, runningTestBinary(t))
	owners := bed.owners()
	var begun kernel.BeginRequest
	owners.landing.begin = func(request kernel.BeginRequest) (kernel.BeginOutcome, error) {
		begun = request
		return kernel.BeginOutcome{Changed: true, Opening: batch.Opening{Base: "1111111111111111111111111111111111111111",
			Candidate: "2222222222222222222222222222222222222222", Series: make([]batch.SeriesCommit, 2), Deviation: 3}}, nil
	}
	code, text := bed.runWith(t, owners, "landing", "begin", "--batch", kernelVerbBatch, "--members", "goal-a, change:0123456789ab", "--base", "origin/main", "--head", "HEAD")
	if code != 0 || !strings.Contains(text, "recorded batch "+kernelVerbBatch+"'s series of 2 commits") {
		t.Fatalf("landing begin = %d\n%s", code, text)
	}
	if begun.Home != bed.home || string(begun.Layout.Checkout) != bed.checkout || string(begun.Layout.Install) != bed.installation ||
		!strings.HasSuffix(begun.Actor, "+"+batchowner.LandingOwnerLineage) || strings.Join(begun.Members, ",") != "goal-a,change:0123456789ab" ||
		begun.Base != "origin/main" || begun.Head != "HEAD" {
		t.Fatalf("landing begin asked the kernel %+v; want the admitted nested lane, its home and claim identity", begun)
	}
	if code, text := bed.runWith(t, owners, "landing", "begin", "--batch", kernelVerbBatch, "--base", "origin/main"); code != 2 || !strings.Contains(text, "nothing was recorded") {
		t.Fatalf("landing begin without members = %d\n%s", code, text)
	}

	var proved kernel.ProveRequest
	owners.landing.prove = func(request kernel.ProveRequest) (batch.ProofAttempt, error) {
		proved = request
		return batch.ProofAttempt{ID: "a1", Subject: batch.SubjectMember, Member: "goal-a", Status: batch.AttemptRed, RedGroups: []string{"app-standard"}}, nil
	}
	code, text = bed.runWith(t, owners, "landing", "prove", "--batch", kernelVerbBatch, "--subject", "member:goal-a")
	if code != 1 || !strings.Contains(text, "tests of goal-a on the base of batch "+kernelVerbBatch+" failed: app-standard") ||
		!strings.Contains(text, "landing status --verbose") || proved.Subject != "member:goal-a" || string(proved.Layout.Checkout) != bed.checkout {
		t.Fatalf("a red member = %d (asked %+v)\n%s", code, proved, text)
	}
	owners.landing.prove = func(request kernel.ProveRequest) (batch.ProofAttempt, error) {
		return batch.ProofAttempt{ID: "a2", Subject: batch.SubjectBatch, Status: batch.AttemptUnavailable, Reason: "the test run was refused: the lane cannot be named"}, nil
	}
	code, text = bed.runWith(t, owners, "landing", "prove", "--batch", kernelVerbBatch, "--subject", "batch", "--json")
	var result intentResult
	if err := json.Unmarshal([]byte(text), &result); err != nil || code != 1 || result.Outcome != intentFailed ||
		!strings.Contains(result.Summary, "could not run, which says nothing about the work") || result.Next == nil {
		t.Fatalf("an unavailable run = %d %+v %v\n%s", code, result, err, text)
	}
	owners.landing.prove = func(request kernel.ProveRequest) (batch.ProofAttempt, error) {
		return batch.ProofAttempt{}, &lane.Refusal{Code: lane.CodePaused, Message: "the landing lane is stopped by Wido, so prove was not started",
			Fix: "a person resumes it: metasystem landing start", Argv: []string{"metasystem", "landing", "start"}}
	}
	code, text = bed.runWith(t, owners, "landing", "prove", "--batch", kernelVerbBatch, "--subject", "batch")
	if code != 1 || !strings.Contains(text, "stopped by Wido") || !strings.Contains(text, "landing start") || strings.Contains(text, lane.CodePaused) {
		t.Fatalf("a paused prove = %d\n%s", code, text)
	}

	// The production kernel behind the verb: a batch the lane does not hold
	// is refused in two lines, and nothing is recorded.
	code, text = bed.runWith(t, bed.owners(), "landing", "begin", "--batch", kernelVerbBatch, "--members", "goal-a", "--base", "origin/main", "--head", "HEAD")
	if code != 1 || !strings.Contains(text, "batch "+kernelVerbBatch+" can't be read") || strings.Contains(text, batch.CodeBeginRefused) {
		t.Fatalf("begin of an unknown batch = %d\n%s", code, text)
	}
	code, text = bed.runWith(t, bed.owners(), "landing", "prove", "--batch", kernelVerbBatch, "--subject", "batch")
	if code != 1 || !strings.Contains(text, "can't be read") || strings.Contains(text, kernel.CodeProveRefused) {
		t.Fatalf("prove of an unknown batch = %d\n%s", code, text)
	}
}
