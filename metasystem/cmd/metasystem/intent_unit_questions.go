package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel"
	dispatchcore "github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
)

// recordUnitStopOverride publishes the person's impact statement before the
// command admits its effect. Failed commands retain that admission evidence
// but cannot close any finding's question.
func (inv *intentInvocation) recordUnitStopOverride(id, kind, reason, impact, who string, worktrees ...string) error {
	at := inv.unitStopNow()
	operation, err := goal.NewOperationULID()
	if err != nil {
		return err
	}
	record := struct {
		Goal      string    `json:"goal"`
		Kind      string    `json:"kind"`
		Reason    string    `json:"reason"`
		Impact    string    `json:"impact"`
		Who       string    `json:"who"`
		At        time.Time `json:"at"`
		Worktrees []string  `json:"worktrees,omitempty"`
	}{id, kind, reason, impact, who, at.UTC(), worktrees}
	data, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return err
	}
	path := filepath.Join(inv.layout.InstallationRoot.Path(), "artifacts", "agents", "channel", "unit-stop-overrides", operation+".json")
	if _, err := atomicfile.WriteFile(path, append(data, '\n'), 0o600, ""); err != nil {
		return err
	}
	_, err = fmt.Fprintln(inv.stderr, impact+" Reason: "+reason)
	return err
}

// A conclusion retains its repository's worktree paths before branch cleanup
// can remove them. Repeats include those paths to finish the shared unit store.
func (inv *intentInvocation) goalReviewWorktrees(id string) ([]string, bool, error) {
	recorded := false
	paths := []string{inv.layout.GitRoot}
	registered, err := inv.registeredWorktrees()
	for path := range registered {
		if !slices.Contains(paths, path) {
			paths = append(paths, path)
		}
	}
	records, scanErr := filepath.Glob(filepath.Join(inv.layout.InstallationRoot.Path(), "artifacts", "agents", "channel", "unit-stop-overrides", "*.json"))
	if scanErr != nil {
		return paths, recorded, scanErr
	}
	for _, path := range records {
		data, readErr := os.ReadFile(path)
		var record struct {
			Goal, Kind string
			Worktrees  []string
		}
		if readErr == nil {
			readErr = json.Unmarshal(data, &record)
		}
		if readErr != nil {
			err = errors.Join(err, readErr)
			continue
		}
		if record.Goal == id && record.Kind == "goal-done" {
			recorded = true
			for _, tree := range record.Worktrees {
				if !slices.Contains(paths, tree) {
					paths = append(paths, tree)
				}
			}
		}
	}
	return paths, recorded, err
}

// Cleanup stays discoverable while its records are open or cannot be read.
func (inv *intentInvocation) goalReviewCleanupPending(id string) bool {
	worktrees, _, treeErr := inv.goalReviewWorktrees(id)
	root := inv.layout.InstallationRoot.Path()
	chains, chainErr := dispatchcore.GoalReviewCleanupPending(root, id)
	units, unitErr := inv.work().units(inv.layout).GoalReviewCleanupPending(id, worktrees)
	questions, unreadable := channel.WalkOpenQuestions(root)
	if treeErr != nil || chainErr != nil || unitErr != nil || len(unreadable) > 0 || chains || units {
		return true
	}
	for _, question := range questions {
		if question.Goal == id && question.UnitStop != nil {
			return true
		}
	}
	return false
}

func (inv *intentInvocation) unitStopNow() time.Time {
	if inv.owners.commandNow != nil {
		if at, err := inv.owners.commandNow(inv.layout.InstallationRoot.Path()); err == nil {
			return at
		}
	}
	return time.Now()
}

// recordUnitStopActForReview joins a successful risk act to its finding's
// exact review subject. Other findings and other reviews remain open.
func recordUnitStopActForReview(root, id, review, finding, reason string, at time.Time) error {
	questions, unreadable := channel.WalkOpenQuestions(root)
	if len(unreadable) > 0 {
		return fmt.Errorf("risk was recorded, but its questions could not be read: %v", unreadable)
	}
	for _, q := range questions {
		stop := q.UnitStop
		if q.Goal != id || stop == nil || stop.Review != review || stop.Finding != finding {
			continue
		}
		act := channel.UnitStopAct{ID: "accept-risk:" + q.ID, Goal: id, Loop: stop.Loop, Subject: stop.Subject, Attempt: stop.Attempt, Findings: []string{finding}, Kind: "goal-accept-risk", Reason: reason, At: at}
		if err := channel.RecordUnitStopAct(root, act); err != nil {
			return err
		}
	}
	return nil
}

func unitStopActor(actor []string) string {
	for i, v := range actor {
		if v == "--by" && i+1 < len(actor) {
			return actor[i+1]
		}
	}
	return ""
}

// Build holds ask for the retained round's remedy. Successful continuation
// closes only that hold, including when collection resumes after a restart.
func (inv *intentInvocation) syncBuildHolds(record launch.UnitRunRecord) error {
	root := inv.layout.InstallationRoot.Path()
	for index, round := range record.Rounds {
		for _, step := range round.Steps {
			if step.RetryLaunch != "" {
				if err := channel.RecordUnitStopAct(root, channel.UnitStopAct{ID: step.RetryLaunch + ":retry", Goal: record.Goal, Loop: "unit-build", Subject: record.Goal + "/" + record.Unit + "/" + record.ID, Attempt: round.Number, Findings: []string{step.RetryLaunch}, Kind: "work-review-retry", Reason: step.RetryReason, At: inv.unitStopNow()}); err != nil {
					return err
				}
			}
		}
		kind := round.Outcome
		if round.SizeAcceptedBy != "" {
			kind = "build-size"
		}
		failure := round.Stop != nil && round.Stop.Loop == "unit-build" && kind != "build-gap" && kind != "build-size"
		if kind != "build-gap" && kind != "build-size" && !failure {
			continue
		}
		subject := record.Goal + "/" + record.Unit + "/" + record.ID
		act := "work-review-size"
		next := inv.workArgv(record, "review", "--reason", fmt.Sprintf("accepted at %d lines", round.BuildLines), "--by", "NAME")
		if kind == "build-gap" {
			act = "work-revise"
			next, _ = inv.workContinuation(record.Goal, launch.NamedWork{Unit: record.Unit, Record: &record}, true)
		}
		if failure {
			act = "work-revise"
			next, _ = inv.workContinuation(record.Goal, launch.NamedWork{Unit: record.Unit, Record: &record}, true)
			if round.Cause == "environment" || round.Cause == "deadline" {
				act = "work-review-retry"
			}
			if act == "work-review-retry" {
				for _, step := range round.Steps {
					if step.State == launch.StepFailed {
						kind = step.LaunchID
						break
					}
				}
			}
		}
		supersededSize := kind == "build-size" && round.SizeAcceptedBy == "" && index+1 < len(record.Rounds)
		if supersededSize {
			act = "work-revise"
		}
		if index+1 < len(record.Rounds) || round.SizeAcceptedBy != "" {
			if err := channel.RecordUnitStopAct(root, channel.UnitStopAct{ID: record.ID + ":" + kind + ":" + fmt.Sprint(round.Number), Goal: record.Goal, Loop: "unit-build", Subject: subject, Attempt: round.Number, Findings: []string{kind}, Kind: act, UnitClosed: supersededSize, Reason: "the retained build hold was continued", At: inv.unitStopNow()}); err != nil {
				return err
			}
			continue
		}
		decision := fmt.Sprintf("Unit %s of goal %s stopped after build round %d: a build step failed", record.Unit, record.Goal, round.Number)
		switch kind {
		case "build-size":
			decision = fmt.Sprintf("Unit %s of goal %s changed %d lines against the %d it declared", record.Unit, record.Goal, round.BuildLines, round.DeclaredLines)
		case "build-gap":
			decision = fmt.Sprintf("Unit %s of goal %s stopped after build round %d: the builder reported a gap", record.Unit, record.Goal, round.Number)
		}
		facts := []string{round.Stop.Class, fmt.Sprintf("%d changed lines against %d declared", round.BuildLines, round.DeclaredLines), "Retained builder message: " + round.GapMessage}
		if failure {
			decision, facts, next = inv.failureAsk(record, round, act, next)
		}
		request := channel.AskRequest{Plain: &channel.Plain{Decision: decision, IfNothing: "the unit stays held; this question closes when the act is recorded"}, RepoRoot: root, Goal: record.Goal, Kind: "other", Facts: facts, Recommendation: "Run the requested act to continue this retained round.", UnitStop: &channel.UnitStopQuestion{Loop: "unit-build", Subject: subject, Attempt: round.Number, Finding: kind, Needs: shellCommand(next), AcceptableActs: []string{act}}, Now: inv.unitStopNow()}
		asked := correctionChoice(request, record, round, next)
		_, err := channel.Ask(asked)
		if err != nil && asked.Contract {
			// Words a person cannot read ask as before.
			_, err = channel.Ask(request)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

// correctionChoice asks a held build's remedy as solutions, when its command
// is ready to run: Continue, Retry or Accept the size runs the pasteable act,
// Stop here runs nothing.
func correctionChoice(request channel.AskRequest, record launch.UnitRunRecord, round launch.UnitRound, next []string) channel.AskRequest {
	act, words, at := request.UnitStop.AcceptableActs[0], strings.ReplaceAll(record.Goal, "-", " "), request.Now.Local().Format("15:04")
	ready := next
	if act == "work-review-size" {
		ready = withoutOption(next, "by")
	}
	if slices.ContainsFunc(ready, func(word string) bool { return word == "FILE" || word == "TEXT" || word == "NAME" }) {
		return request
	}
	label, decision := "Continue", fmt.Sprintf("%s: the build of unit %s stopped at %s (%s).", words, record.Unit, at, round.Stop.Class)
	consequence := "the builder corrects and finishes the unit from the prepared brief; one more round of the goal's budget."
	switch {
	case act == "work-review-size":
		label, decision = "Accept the size", fmt.Sprintf("%s: the build of unit %s grew to %d changed lines against the %d planned, at %s.", words, record.Unit, round.BuildLines, round.DeclaredLines, at)
		consequence = "the checks and the review go on with this size; nothing else changes."
	case act == "work-review-retry" && retriesBuilder(round):
		label, decision = "Retry", fmt.Sprintf("%s: the build of unit %s could not finish at %s (%s).", words, record.Unit, at, round.Stop.Class)
		consequence = "the builder takes another turn with the same brief, keeping its changes; no new round."
	case act == "work-review-retry":
		label, decision = "Retry", fmt.Sprintf("%s: the check of unit %s could not finish at %s (%s).", words, record.Unit, at, round.Stop.Class)
		consequence = "I run the failed check again with the same inputs; no new build."
	case act != "work-revise":
		return request
	}
	request.Contract, request.Recommendation = true, label
	request.Plain = &channel.Plain{Decision: decision, IfNothing: "it waits; nothing builds or lands until you choose."}
	request.Options = []channel.Option{
		{Label: label, Consequence: consequence + " Undo: metasystem work stop " + record.Goal + ".", Act: withoutOption(next, "by")},
		{Label: "Stop here", Consequence: "the work stays in its worktree; nothing builds, lands or concludes. This question stays open."},
	}
	return request
}

// failureAsk says what stopped a failed round and gives the person a command
// to paste: the machinery's reason, the known person and, for a correction,
// a prepared brief the person may edit first.
func (inv *intentInvocation) failureAsk(record launch.UnitRunRecord, round launch.UnitRound, act string, next []string) (string, []string, []string) {
	decision := fmt.Sprintf("Unit %s of goal %s stopped after build round %d: %s", record.Unit, record.Goal, round.Number, round.Stop.Class)
	var facts []string
	if round.DeclaredLines > 0 {
		facts = append(facts, fmt.Sprintf("%d changed lines against %d declared", round.BuildLines, round.DeclaredLines))
	}
	if round.GapMessage != "" {
		facts = append(facts, "The builder's last words: "+lastParagraph(round.GapMessage))
	}
	if act == "work-revise" {
		path := filepath.Join(round.Directory, "correction-brief.md")
		if err := writeCorrectionBrief(path, record, round); err != nil {
			facts = append(facts, "A correction brief could not be prepared ("+err.Error()+"); write one and give its path for FILE.")
		} else {
			next = replaceWord(next, "FILE", path)
			facts = append(facts, "A correction brief is prepared at "+path+"; edit it before you run the command if you want a different correction.")
		}
	}
	if inv.stateRoot == "" {
		_ = inv.selectRoot()
	}
	next = replaceWord(replaceWord(next, "TEXT", "the machinery stopped: "+round.Stop.Class), "NAME", inv.knownPerson())
	return decision, facts, next
}

// writeCorrectionBrief prepares the correction brief once; a person's edits
// survive every later question sync.
func writeCorrectionBrief(path string, record launch.UnitRunRecord, round launch.UnitRound) error {
	if regularFile(path) {
		return nil
	}
	text := fmt.Sprintf("Correct unit %s of goal %s after build round %d. The round stopped: %s.\n", record.Unit, record.Goal, round.Number, round.Stop.Class)
	if round.GapMessage != "" {
		text += "\nThe builder's last words: " + lastParagraph(round.GapMessage) + "\n"
	}
	if open := filepath.Join(round.Directory, "open.md"); regularFile(open) {
		text += "\nThe builder's open items are in " + open + ".\n"
	}
	_, err := atomicfile.WriteText(path, text+"\nThe change so far is in the worktree; keep what is right and finish the unit.\n", round.Directory)
	return err
}

// lastParagraph is the builder message's last paragraph, at most its last
// 600 bytes; a Claude result file gives its result text.
func lastParagraph(path string) string {
	data, err := os.ReadFile(path)
	text := string(data)
	if err == nil && filepath.Base(path) == "result.json" {
		var result struct {
			Result string `json:"result"`
		}
		err = json.Unmarshal(data, &result)
		text = result.Result
	}
	paragraphs := regexp.MustCompile(`\n[ \t\r]*\n`).Split(strings.TrimSpace(text), -1)
	last := strings.TrimSpace(paragraphs[len(paragraphs)-1])
	if err != nil || last == "" {
		return "The builder left no readable last message."
	}
	for cut := len(last) - 600; cut > 0 && cut < len(last); cut++ {
		if utf8.RuneStart(last[cut]) {
			return last[cut:]
		}
	}
	return last[max(len(last)-600, 0):]
}
