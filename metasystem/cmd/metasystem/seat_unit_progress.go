package main

import (
	"fmt"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
)

// seatUnitProgress projects retained work milestones beside the public next
// step. Merely moving a branch tip is not a work milestone.
func seatUnitProgress(unit *steward.UnitStage, work launch.NamedWork, next []string) {
	r := work.Record
	if r == nil {
		return
	}
	unit.Continuable = r.State == "awaiting-judgement" && len(next) > 0
	unit.Advances = append(unit.Advances, "admitted:"+r.ID)
	for _, round := range r.Rounds {
		for _, step := range round.Steps {
			if step.State == launch.StepRunning || step.State == launch.StepStarting {
				unit.Continuable = false
			}
			if step.State == launch.StepPassed {
				unit.Advances = append(unit.Advances, fmt.Sprintf("run:%s/round:%d/%s", r.ID, round.Number, step.Name))
			}
		}
	}
	if strings.HasPrefix(unit.Stage, "reviewed; its read is collected") {
		if subject := currentSubject(work); subject != nil {
			unit.Advances = append(unit.Advances, "read:"+subject.Commit)
			if strings.HasPrefix(unit.Stage, "reviewed; its read is collected and published") {
				unit.Advances = append(unit.Advances, "published:"+subject.Commit)
			}
		}
	}
}
