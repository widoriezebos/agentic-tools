package main

import (
	"bytes"
	"cmp"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel"
	dispatchcore "github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goalbudget"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
)

// boxStop is when the bed's goal stopped at its budget.
var boxStop = time.Date(2026, 9, 1, 9, 0, 0, 0, time.UTC)

// boxQuestion is a stopped goal's box question as the breach stop asks it.
func boxQuestion(t *testing.T, root, goalID string) channel.Question {
	t.Helper()
	q, err := channel.Ask(channel.AskRequest{RepoRoot: root, Goal: goalID, Kind: "stop", Contract: true, Now: boxStop, Recommendation: "One more day",
		Decision: &channel.Decision{Kind: "breach-stop", Subject: goalID, State: "2"},
		Facts:    []string{"stopped at revision 2: elapsed-limit"},
		Plain:    &channel.Plain{Decision: strings.ReplaceAll(goalID, "-", " ") + ": it used its 4h box and stopped at 09:00.", IfNothing: "it stays stopped; nothing resumes it."},
		Options: []channel.Option{
			{Label: "One more day", Consequence: "its time box becomes the time used so far plus one working day (8 hours), counted when you choose. Undo: metasystem goal budget " + goalID + " 4h/4/240m/2/3 (it stops again).",
				Act: []string{"metasystem", "goal", "budget", goalID, "one-more-day"}},
			{Label: "Leave it stopped", Consequence: "nothing runs and nothing is spent; this question stays open."},
		}})
	if err != nil {
		t.Fatal(err)
	}
	return q
}

func oneMoreDay(spent time.Duration) string {
	return goalbudget.FormatWorkingDuration((spent + time.Minute - 1).Truncate(time.Minute) + 8*time.Hour)
}

func TestOneMoreDayAdmitsWorkAfterAnElapsedBreach(t *testing.T) {
	t.Parallel()
	bed := newIntentBed(t, true, nil)
	before := bed.goalFile(bedGoal)
	// The person answers ten hours after the stop: the clock kept counting.
	actAt := boxStop.Add(10 * time.Hour)
	owners := bed.owners()
	owners.commandNow = func(string) (time.Time, error) { return actAt, nil }
	human := []string{"--fixture-human-authority", "--lineage", "m1"}
	code, result := bed.runJSON(owners, append([]string{"goal", "budget", bedGoal, "one-more-day"}, human...)...)
	file := bed.goalFile(bedGoal)
	spent := dispatchcore.ProjectBudget(bed.root(), before, actAt).Elapsed
	if code != 0 || result.Outcome != intentConfirmed || file.StopFence != nil || file.Budget == nil || file.Budget.ElapsedLimit != oneMoreDay(spent) {
		t.Fatalf("one more day = %d %+v; fence %+v box %+v, want the elapsed limit %s", code, result, file.StopFence, file.Budget, oneMoreDay(spent))
	}
	kept, raised := *before.Budget, *file.Budget
	kept.ElapsedLimit = raised.ElapsedLimit
	// The episode is kept: it still starts at the claim, not at the act.
	if kept != raised || file.Claimed == nil || file.Claimed.EpisodeAt != cmp.Or(before.Claimed.EpisodeAt, before.Claimed.At) {
		t.Fatalf("one more day changed more than the time box: %+v -> %+v, episode %s -> %s", before.Budget, file.Budget, before.Claimed.EpisodeAt, file.Claimed.EpisodeAt)
	}
	if admission := dispatchcore.ProjectBudget(bed.root(), file, actAt); admission.Elapsed >= admission.Limits.ElapsedDuration() {
		t.Fatalf("admission stays closed on elapsed: used %s of %s", admission.Elapsed, admission.Limits.ElapsedLimit)
	}
	code, again := bed.runJSON(owners, append([]string{"goal", "budget", bedGoal, "one-more-day"}, human...)...)
	if code != 0 || again.Outcome != intentUnchanged || !strings.Contains(again.Summary, "not stopped by its budget") || bed.goalFile(bedGoal).Budget.ElapsedLimit != raised.ElapsedLimit {
		t.Fatalf("a second one-more-day = %d %+v", code, again)
	}
}

// boxAnswer runs the act a recorded answer chose, as the poll runs it: no
// terminal, no inherited session.
func boxAnswer(t *testing.T, bed *intentBed, id string, argv ...string) (int, intentResult) {
	t.Helper()
	bed.lineage = ""
	owners := bed.owners()
	owners.prove = func(string, int64, humanauthority.Reader, string, string, time.Time) (humanauthority.Proof, error) {
		return humanauthority.Proof{}, errors.New("an agent process is an ancestor of this shell")
	}
	command, rest, ok := resolveIntentArgv(append(argv, "--answer", id))
	if !ok {
		t.Fatalf("no public command %q", argv)
	}
	var out, stderr bytes.Buffer
	code := runIntentIn(command, append(rest, "--json"), &out, &stderr, bed.root(), owners)
	var result intentResult
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("%v printed no JSON result: %v; %s %s", argv, err, out.String(), stderr.String())
	}
	return code, result
}

func TestChannelOneMoreDayActsWithoutAnInheritedLineage(t *testing.T) {
	t.Parallel()
	answered := func(t *testing.T, text, lineage string) (*intentBed, channel.Question) {
		bed := newIntentBed(t, true, nil)
		channelFor(t, bed.root())
		q := recordChoice(t, bed.root(), boxQuestion(t, bed.root(), bedGoal), func(q *channel.Question) { q.Answer.Text, q.Answer.Lineage = text, lineage })
		return bed, q
	}
	t.Run("the chosen act", func(t *testing.T) {
		t.Parallel()
		bed, q := answered(t, "one more day", "steward")
		code, result := boxAnswer(t, bed, q.ID, "goal", "budget", bedGoal, "one-more-day")
		file := bed.goalFile(bedGoal)
		last := file.History[len(file.History)-1]
		if state, _ := answerState(t, result); code != 0 || state != "started" || file.StopFence != nil {
			t.Fatalf("the answer did not raise the box: %d %+v fence %+v", code, result, file.StopFence)
		}
		steward := goal.Opid("01ARZ3NDEKTSV4RRFFQ69G5FAV", "mac-cli", "steward")
		if last.Actor != "human:Wido" || !strings.HasSuffix(last.Opid, steward[strings.LastIndex(steward, "-"):]) || last.AuthorityOutcome != goal.AuthorityOutcomeVerifiedChannelAnswer || last.ChannelContext != q.ID {
			t.Fatalf("the act is not the person's from the steward's session: %+v", last)
		}
		if proofs, _ := filepath.Glob(filepath.Join(bed.root(), "artifacts", "agents", "authority", "proofs", "*.json")); len(proofs) == 0 {
			t.Fatal("the channel proof was not recorded")
		}
		closed, err := channel.ReadQuestion(bed.root(), q.ID)
		if err != nil || closed.State != "closed" {
			t.Fatalf("the box question stays open: %+v %v", closed, err)
		}
		if marked := channel.GoalOpenQuestions(bed.root()); len(marked) != 0 {
			t.Fatalf("the goal is still held by its question: %+v", marked)
		}
	})
	for _, row := range []struct {
		name, text, lineage string
		argv                []string
		want                string
	}{
		{name: "an answer with no session", text: "one more day", argv: []string{"goal", "budget", bedGoal, "one-more-day"}, want: "names no session"},
		{name: "another box", text: "one more day", lineage: "steward", argv: []string{"goal", "budget", bedGoal, "7d/4/240m/2/3"}, want: `authorizes "One more day"`},
		{name: "an answer that chose no act", text: "leave it stopped", lineage: "steward", argv: []string{"goal", "budget", bedGoal, "one-more-day"}, want: "nothing was done"},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			bed, q := answered(t, row.text, row.lineage)
			code, result := boxAnswer(t, bed, q.ID, row.argv...)
			if state, _ := answerState(t, result); code == 0 || result.Outcome != intentRefused || state != "refused" || !strings.Contains(result.Summary, row.want) || bed.goalFile(bedGoal).StopFence == nil {
				t.Fatalf("%s was admitted: %d %+v", row.name, code, result)
			}
		})
	}
}

func TestResumedStoppedGoalClosesItsBreachQuestion(t *testing.T) {
	t.Parallel()
	bed := newIntentBed(t, true, nil)
	mine, other := boxQuestion(t, bed.root(), bedGoal), boxQuestion(t, bed.root(), "another-goal")
	code, result := bed.runJSON(bed.owners(), "goal", "resume", bedGoal, "--fixture-human-authority", "--lineage", "m1")
	if code != 0 || result.Outcome != intentConfirmed || bed.goalFile(bedGoal).StopFence != nil {
		t.Fatalf("resume = %d %+v", code, result)
	}
	closed, _ := channel.ReadQuestion(bed.root(), mine.ID)
	open, _ := channel.ReadQuestion(bed.root(), other.ID)
	if closed.State != "closed" || !strings.HasPrefix(closed.ClosedBecause, "resume at ") || closed.Thread != nil || open.State != "open" {
		t.Fatalf("the resume closed %s (%q), and the other goal's question is %s", closed.State, closed.ClosedBecause, open.State)
	}
	if _, err := os.Stat(filepath.Join(bed.root(), "artifacts", "agents", "channel", "fleet", "notices.json")); !os.IsNotExist(err) {
		t.Fatal(fmt.Sprint("the close queued a post: ", err))
	}
}
