package launch

import (
	"encoding/json"
	"fmt"
)

type UnitReviewAct struct {
	Key      string   `json:"key"`
	Round    int      `json:"round"`
	State    string   `json:"state"`
	Command  []string `json:"command,omitempty"`
	Summary  string   `json:"summary,omitempty"`
	Details  []string `json:"details,omitempty"`
	Template string   `json:"template,omitempty"`
	Effect   string   `json:"effect,omitempty"`
}

// ReviewActKey binds an act to the run’s current canonical evidence.
func ReviewActKey(record UnitRunRecord) string {
	data, _ := json.Marshal([]any{record.ID, record.State, record.Rounds, record.Subjects, record.Revisions})
	return digestHex(data)
}
func (runner *UnitRunner) RetainReviewAct(id string, act UnitReviewAct) error {
	held, err := runner.lock(id)
	if err != nil {
		return err
	}
	defer releaseUnitLock(held)
	record, err := runner.read(id)
	if err != nil {
		return err
	}
	if ReviewActKey(record) != act.Key || len(record.Rounds) != act.Round {
		return fmt.Errorf("the review decision changed; observe this run again")
	}
	record.ReviewAct = &act
	return runner.save(record)
}
