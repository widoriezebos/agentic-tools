package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
)

// The holder's step over a sent-back goal (g1-s69 D2, §6).
//
// A human's send-back is a line on the goal's history with the correction
// brief published beside it; every seat reads it after its sync, and the seat
// that holds the goal is the one that revises. Its step is one work revise on
// the published brief — with --work where the line names the work item — and
// then one line of its own on the history: the attempt that correction
// started, or that the goal has several work items and the human must name
// one. The answer is what makes the step happen once: a send-back already
// answered is not revised again, and a pass that died between the revise and
// its answer rejoins the same attempt, because work revise keeps an identical
// request and never spends another.
//
// It runs as work revise G with no --brief, on the holder's seat. The loop
// that takes the step by itself on the holder's next activity over its claims
// is slice D's; until then the holder's session runs it.

const sentBackReviseDetail = "Without --brief, a goal sent back from review is corrected from the brief the review published, once, and the attempt is recorded on the goal."

func runIntentReviseSentBack(inv *intentInvocation) int {
	if inv.input.has("brief") || len(inv.input.args) != 1 || strings.Contains(inv.input.args[0], ":") {
		return runIntentRevise(inv)
	}
	if problem := inv.selectRoot(); problem != nil {
		return runIntentRevise(inv)
	}
	result, handled := inv.reviseSentBack(inv.input.args[0])
	if !handled {
		return runIntentRevise(inv)
	}
	return inv.render(result)
}

// reviseSentBack takes the holder's step on one goal, and says whether the
// goal carries a send-back to take it on.
func (inv *intentInvocation) reviseSentBack(id string) (intentResult, bool) {
	projection, _, problem := inv.projection()
	if problem != nil {
		return intentResult{}, false
	}
	file := projection.Tree.Live[id]
	line, standing := goal.SentBackOf(file)
	if !standing {
		return intentResult{}, false
	}
	targets := inv.targets(id)
	request, err := syncReqWithProofAtWithDependencies("send-back", inv.stateRoot, "", "", nil, inv.owners.commandNow, inv.owners.dependencies)
	if err != nil {
		return intentResult{Outcome: intentFailed, code: 1, Targets: targets, Summary: "cannot tell which seat acts: " + err.Error() + "; nothing was done"}, true
	}
	if file.Claimed == nil || file.Claimed.Machine != request.Actor.Machine || file.Claimed.Lineage != request.Actor.Lineage {
		return intentResult{Outcome: intentRefused, code: 1, Targets: targets,
			Summary: fmt.Sprintf("goal %s was sent back from review by %s, and the seat that holds it revises; this session does not hold it; nothing was done", id, line.By)}, true
	}
	brief, err := goal.ReadPublished(request.Endpoint, line.Brief)
	if err != nil {
		return intentResult{Outcome: intentFailed, code: 1, Targets: targets, Summary: "cannot read the brief the review published: " + err.Error() + "; nothing was done"}, true
	}
	path := filepath.Join(inv.stateRoot, "artifacts", "agents", "sent-back", id+"-"+line.Opid+".md")
	err = os.MkdirAll(filepath.Dir(path), 0o755)
	if err == nil {
		err = os.WriteFile(path, brief, 0o644)
	}
	if err != nil {
		return intentResult{Outcome: intentFailed, code: 1, Targets: targets, Summary: "cannot keep the published brief for the revision: " + err.Error() + "; nothing was done"}, true
	}
	raw := []string{id, "--brief", path, "--json", "--repo", inv.stateRoot}
	if line.Work != "" {
		raw = append(raw, "--work", line.Work)
	}
	revise := inv.owners.sentBackRevise
	if revise == nil {
		revise = reviseInProcess
	}
	revised := revise(inv, raw)
	said := fmt.Sprintf("goal %s was sent back by %s at %s; ", id, line.By, shortCommit(line.Tip))
	answer := goal.SendBackAnswer{Review: line.Opid}
	switch {
	case len(candidatesOf(revised)) > 1:
		answer.Candidates = candidatesOf(revised)
	case attemptOf(revised) > 0 && (revised.Outcome == intentConfirmed || revised.Outcome == intentInProgress || revised.Outcome == intentUnchanged):
		answer.Attempt, answer.Work = attemptOf(revised), line.Work
	default:
		revised.Summary = said + revised.Summary
		return revised, true
	}
	answered, err := goal.AnswerSendBack(request, id, answer)
	if err == nil && answered.Outcome != goal.OutcomeConfirmed && !answered.Unchanged {
		err = fmt.Errorf("%s", answered.Detail)
	}
	if err != nil {
		revised.Outcome, revised.code = intentPartial, 1
		revised.Summary = said + revised.Summary + "; the answer was not recorded on the goal: " + err.Error()
		return revised, true
	}
	if len(answer.Candidates) > 0 {
		return intentResult{Outcome: intentConfirmed, Targets: targets, Data: revised.Data,
			Summary: said + fmt.Sprintf("it has %d work items (%s), so the reviewer names one; recorded on the goal", len(answer.Candidates), strings.Join(answer.Candidates, ", "))}, true
	}
	revised.Summary = said + revised.Summary + fmt.Sprintf("; attempt %d is recorded on the goal", answer.Attempt)
	return revised, true
}

// reviseInProcess runs the public work revise with the published brief, in
// this process, and reads its one JSON result back.
func reviseInProcess(inv *intentInvocation, raw []string) intentResult {
	var stdout, stderr bytes.Buffer
	runIntentIn(inv.command, raw, &stdout, &stderr, inv.cwd, inv.owners)
	var result intentResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		return intentResult{Outcome: intentFailed, code: 1, Summary: "work revise answered nothing this step can read: " + strings.TrimSpace(stderr.String())}
	}
	if result.Outcome != intentConfirmed && result.Outcome != intentUnchanged {
		result.code = 1
	}
	return result
}

func candidatesOf(result intentResult) []string {
	data, _ := result.Data.(map[string]any)
	var names []string
	switch listed := data["candidates"].(type) {
	case []string:
		names = listed
	case []any:
		for _, one := range listed {
			if name, ok := one.(string); ok {
				names = append(names, name)
			}
		}
	}
	return names
}

func attemptOf(result intentResult) int {
	data, _ := result.Data.(map[string]any)
	revision, _ := data["revision"].(map[string]any)
	switch attempt := revision["attempt"].(type) {
	case int:
		return attempt
	case float64:
		return int(attempt)
	}
	return 0
}
