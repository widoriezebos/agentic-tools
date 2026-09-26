package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
)

func designRun(t *testing.T, bed *workBed, args ...string) (int, intentResult) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := runIntentIn(mustIntentCommand(t, args[0]), append(args[1:], "--json"), &stdout, &stderr, bed.root(), bed.workOwners())
	var result intentResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("%v: %v %q %q", args, err, stdout.String(), stderr.String())
	}
	return code, result
}

// fakeAuthor is the design author as a model sees its task: it knows only
// the prompt the real Claude adapter would send on stdin. It finds its one
// output file and its starting point in that prompt, writes the record head
// it was given (from the frozen prior version or the prompt) and a body.
func fakeAuthor(t *testing.T, body string) func(launch.Record) {
	return func(record launch.Record) {
		command, err := launch.ClaudeHeadless{}.Command(record, t.TempDir())
		if err != nil {
			t.Errorf("the author's command: %v", err)
			return
		}
		output, head, err := authorPromptTask(command.Stdin)
		if err != nil {
			t.Errorf("the author prompt: %v\n%s", err, command.Stdin)
			return
		}
		os.WriteFile(output, []byte(strings.TrimRight(head, "\n")+"\n\n"+body), 0o644)
	}
}

// authorPromptTask reads, from an author prompt alone, the output file and
// the record head the draft must start with.
func authorPromptTask(prompt string) (output, head string, err error) {
	_, section, found := strings.Cut(prompt, launch.DesignOutputHeading+"\n")
	if !found {
		return "", "", fmt.Errorf("the prompt has no %q section", launch.DesignOutputHeading)
	}
	section, _, _ = strings.Cut(section, "\n## The request")
	var indented []string
	for _, line := range strings.Split(section, "\n") {
		if strings.HasPrefix(line, "    ") {
			indented = append(indented, strings.TrimSpace(line))
		}
	}
	if len(indented) == 0 || !filepath.IsAbs(indented[0]) {
		return "", "", fmt.Errorf("the prompt names no absolute output file")
	}
	start := section
	if len(indented) > 1 {
		prior, readErr := os.ReadFile(indented[1])
		if readErr != nil {
			return "", "", readErr
		}
		start = string(prior)
	} else if _, after, ok := strings.Cut(section, "exactly this head:\n\n"); ok {
		start = after
	}
	for _, line := range strings.Split(start, "\n") {
		if strings.HasPrefix(line, "# ") || strings.HasPrefix(line, "- ") || line == "" && head != "" && !strings.HasSuffix(head, "\n\n") {
			head += line + "\n"
		}
		if strings.HasPrefix(line, "- Goals:") {
			return indented[0], head, nil
		}
	}
	return "", "", fmt.Errorf("no record head to start from")
}

// TestIntentDesignAuthorJourney: design G has the design lane write a staged
// draft that the project owner publishes into the goal's design document; the
// same request replays without a launch; a later request is made against the
// person's edit; an edit made while an author writes is never overwritten and
// the proposal is kept and shown; a stale --after is refused.
func TestIntentDesignAuthorJourney(t *testing.T) {
	t.Parallel()
	bed := newWorkBed(t)
	bed.starter.author = fakeAuthor(t, "The reader design.\n")
	brief := bed.brief("design-request.md", "Design the reader.\n")
	code, result := designRun(t, bed, "design", bed.id, "--brief", brief)
	data, _ := result.Data.(map[string]any)
	document, _ := data["document"].(string)
	if code != 0 || result.Outcome != intentConfirmed || data["outcome"] != "published" || result.Next == nil || !slices.Contains(result.Next.Argv, "design") {
		t.Fatalf("design: code=%d %+v", code, result)
	}
	path := filepath.Join(bed.root(), document)
	written, _ := os.ReadFile(path)
	if !strings.Contains(string(written), "- Kind: design") || !strings.Contains(string(written), "- Status: draft") || !strings.Contains(string(written), "The reader design.") {
		t.Fatalf("published draft: %q", written)
	}
	launches := len(bed.starter.launched())
	if _, again := designRun(t, bed, "design", bed.id, "--brief", brief); again.Outcome != intentConfirmed || len(bed.starter.launched()) != launches ||
		again.Data.(map[string]any)["rejoined"] != true {
		t.Fatalf("replay: %+v launches=%d->%d", again.Data, launches, len(bed.starter.launched()))
	}
	// The person edits; the next request is made against the edit.
	os.WriteFile(path, append(written, []byte("A person's note.\n")...), 0o644)
	bed.starter.author = fakeAuthor(t, "The reader design, revised.\nA person's note.\n")
	code, result = designRun(t, bed, "design", bed.id, "--brief", bed.brief("more.md", "Revise the reader.\n"))
	if code != 0 || result.Data.(map[string]any)["attempt"] != float64(2) || result.Outcome != intentConfirmed {
		t.Fatalf("second attempt: %+v", result)
	}
	// An edit while the author writes: the proposal is kept, the edit stays.
	bed.starter.author = func(record launch.Record) {
		fakeAuthor(t, "A third version.\n")(record)
		os.WriteFile(path, []byte("edited during the attempt\n"), 0o644)
	}
	_, result = designRun(t, bed, "design", bed.id, "--brief", bed.brief("third.md", "Third.\n"))
	if result.Outcome != intentRefused || result.Data.(map[string]any)["outcome"] != "conflict" {
		t.Fatalf("conflicting edit: %+v", result)
	}
	if current, _ := os.ReadFile(path); string(current) != "edited during the attempt\n" {
		t.Fatalf("the person's edit was overwritten: %q", current)
	}
	if _, shown := designRun(t, bed, "show", "design", "--goal", bed.id, "--out", document, "--attempt", "3"); shown.Outcome != intentConfirmed ||
		!strings.Contains(shown.Summary, "conflict") {
		t.Fatalf("show the kept proposal: %+v", shown)
	}
	if _, stale := designRun(t, bed, "design", bed.id, "--brief", bed.brief("stale.md", "Stale.\n"), "--after", "1", "--out", document); stale.Outcome != intentRefused ||
		!strings.Contains(stale.Summary, "DESIGN_ATTEMPT_STALE") {
		t.Fatalf("stale after: %+v", stale)
	}
}

// TestIntentDesignAdmissionNoClaim: an approved goal nobody holds gets its
// design without a build claim; an unapproved goal is refused before any
// launch.
func TestIntentDesignAdmissionNoClaim(t *testing.T) {
	t.Parallel()
	bed := newWorkBedWith(t, func(file *goal.GoalFile) {
		workApprovedBox(file)
		file.State, file.Claimed, file.StopCapability, file.StopFence = goal.StateApproved, nil, nil, nil
	})
	bed.starter.author = fakeAuthor(t, "Unclaimed design.\n")
	before := bed.goalFile(bedGoal)
	code, result := designRun(t, bed, "design", bed.id, "--brief", bed.brief("d.md", "Design it.\n"))
	after := bed.goalFile(bedGoal)
	if code != 0 || result.Outcome != intentConfirmed || after.State != goal.StateApproved || after.Claimed != nil || len(after.History) != len(before.History) {
		t.Fatalf("design without a claim: %+v state=%s claimed=%+v", result, after.State, after.Claimed)
	}
	queued := newWorkBedWith(t, func(file *goal.GoalFile) {
		file.State, file.Claimed, file.StopCapability, file.StopFence, file.Budget, file.Approved = goal.StateQueued, nil, nil, nil, nil, nil
	})
	if _, refused := designRun(t, queued, "design", queued.id, "--brief", queued.brief("d.md", "Design it.\n")); refused.Outcome != intentRefused || len(queued.starter.launched()) != 0 {
		t.Fatalf("design of an unapproved goal: %+v", refused)
	}
}

// TestIntentDesignLifecycle: a running author shows in status G with the
// wait continuation and is stopped by stop design G through the launch
// owner; a failed author offers exactly one new attempt; several draft
// designs need an explicit document choice.
func TestIntentDesignLifecycle(t *testing.T) {
	t.Parallel()
	bed := newWorkBed(t)
	bed.starter.hold = "design"
	code, result := designRun(t, bed, "design", bed.id, "--brief", bed.brief("d.md", "Design it.\n"))
	if result.Outcome != intentInProgress {
		t.Fatalf("running author: code=%d %+v", code, result)
	}
	_, status := designRun(t, bed, "status", bed.id)
	designs, _ := status.Data.(map[string]any)["designs"].([]any)
	if len(designs) != 1 || status.Next == nil || !slices.Equal(status.Next.Argv, []string{"metasystem", "wait", "goal", bed.id}) {
		t.Fatalf("status of a running author: %+v", status)
	}
	// The launch owner's cancellation is asked for this document's attempt;
	// in this bed its recorded processes cannot be proven dead, and the owner
	// says so rather than reporting a stop.
	_, shown := designRun(t, bed, "show", "design", "--goal", bed.id, "--attempt", "1")
	if _, stopped := designRun(t, bed, "stop", "design", bed.id); stopped.Outcome != intentRefused ||
		!strings.Contains(stopped.Summary, "could not prove every recorded process dead") || shown.Outcome != intentConfirmed {
		t.Fatalf("stop design: %+v", stopped)
	}
	failing := newWorkBed(t)
	failing.starter.fail["design"] = true
	_, failed := designRun(t, failing, "design", failing.id, "--brief", failing.brief("d.md", "Design it.\n"))
	if failed.Outcome != intentFailed || failed.Next == nil || !slices.Equal(failed.Next.Argv[len(failed.Next.Argv)-2:], []string{"--after", "1"}) {
		t.Fatalf("failed author: %+v", failed)
	}
	// A supervisor that claimed its launch and died before any child, while
	// the requesting command is stalled: status stays read-only and offers
	// stop design through the launch owner, whose cancellation proves the
	// recorded supervisor dead by the probe (no child, no group) and stops
	// the attempt; the document is untouched and a new attempt proceeds.
	lost := newWorkBed(t)
	document := filepath.Join(lost.root(), "plans", "designs", lost.id+".md")
	os.MkdirAll(filepath.Dir(document), 0o755)
	original := []byte("# Design\n\n- Kind: design\n- Id: 01M3EFDSFTKWEMSDCP1BB7TLST\n- Status: draft\n- Goals: " + lost.id + "\n\nThe person's draft.\n")
	os.WriteFile(document, original, 0o644)
	stalled := &lostSupervisorStarter{m: lost.manager, claimed: make(chan struct{}), release: make(chan struct{})}
	lost.manager.Supervisor = stalled
	first := make(chan intentResult, 1)
	go func() {
		_, result := designRun(t, lost, "design", lost.id, "--brief", lost.brief("d.md", "Design it.\n"))
		first <- result
	}()
	<-stalled.claimed
	before := len(lost.starter.launched())
	_, status = designRun(t, lost, "status", lost.id)
	if status.Next == nil || !slices.Equal(status.Next.Argv[:4], []string{"metasystem", "stop", "design", lost.id}) {
		t.Fatalf("status of a lost claimed supervisor: %+v", status)
	}
	if again, _ := os.ReadFile(document); !bytes.Equal(again, original) {
		t.Fatalf("status changed the document")
	}
	code, stopped := designRun(t, lost, append([]string{"stop"}, status.Next.Argv[2:]...)...)
	if code != 0 || stopped.Outcome != intentConfirmed || stopped.Data.(map[string]any)["state"] != string(launch.Cancelled) {
		t.Fatalf("stop design of a lost supervisor: code=%d %+v", code, stopped)
	}
	close(stalled.release)
	<-first
	if again, _ := os.ReadFile(document); !bytes.Equal(again, original) {
		t.Fatalf("the stopped attempt changed the document: %q", again)
	}
	lost.manager.Supervisor = lost.starter
	lost.starter.author = fakeAuthor(t, "The next attempt.\n")
	code, next := designRun(t, lost, "design", lost.id, "--brief", lost.brief("d2.md", "Design it again.\n"), "--after", "1")
	if code != 0 || next.Outcome != intentConfirmed || next.Data.(map[string]any)["attempt"] != float64(2) || len(lost.starter.launched()) != before+1 {
		t.Fatalf("a new attempt after the stopped one: code=%d %+v", code, next)
	}

	// Two drafts of one goal need an explicit document choice.
	choice := newWorkBed(t)
	homes := filepath.Join(choice.root(), "plans", "designs")
	os.MkdirAll(homes, 0o755)
	for index, id := range []string{"01M3EFDSFTKWEMSDCP1BB7TDG1", "01M3EFDSFTKWEMSDCP1BB7TDG2"} {
		os.WriteFile(filepath.Join(homes, "part-"+string(rune('a'+index))+".md"),
			[]byte("# Part\n\n- Kind: design\n- Id: "+id+"\n- Status: draft\n- Goals: "+choice.id+"\n\nBody.\n"), 0o644)
	}
	_, ambiguous := designRun(t, choice, "design", choice.id, "--brief", choice.brief("d.md", "Design it.\n"))
	if ambiguous.Outcome != intentRefused || !strings.Contains(ambiguous.Summary, "2 draft designs") || len(choice.starter.launched()) != 0 {
		t.Fatalf("two drafts: %+v", ambiguous)
	}
}

// lostSupervisorStarter is a supervisor that claims its launch record and
// is then gone (a pid the work prober reports dead), while the requesting
// command stalls until released.
type lostSupervisorStarter struct {
	m                *launch.Manager
	claimed, release chan struct{}
}

func (s *lostSupervisorStarter) StartSupervisor(id, _ string) (identity.Ref, error) {
	ref := workRef(30)
	s.m.Store.Update(id, func(record *launch.Record) error {
		record.Supervisor = &ref
		return nil
	})
	close(s.claimed)
	<-s.release
	return ref, nil
}
