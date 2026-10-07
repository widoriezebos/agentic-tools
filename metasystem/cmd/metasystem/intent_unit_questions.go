package main

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
)

// recordUnitStopOverride publishes the person's impact statement before the
// command admits its effect. Failed commands retain that admission evidence
// but cannot close any finding's question.
func (inv *intentInvocation) recordUnitStopOverride(id, kind, reason, impact, who string) error {
	at := inv.unitStopNow()
	operation, err := goal.NewOperationULID()
	if err != nil {
		return err
	}
	record := struct {
		Goal   string    `json:"goal"`
		Kind   string    `json:"kind"`
		Reason string    `json:"reason"`
		Impact string    `json:"impact"`
		Who    string    `json:"who"`
		At     time.Time `json:"at"`
	}{id, kind, reason, impact, who, at.UTC()}
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
