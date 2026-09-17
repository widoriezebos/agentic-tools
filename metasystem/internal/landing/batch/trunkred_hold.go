package batch

import (
	"fmt"
	"strings"
	"time"
)

// TrunkRedHold records a durable trunk-red observation for a batch.
type TrunkRedHold struct {
	Opid       string     `json:"opid"`
	Opids      []string   `json:"opids"`
	Red        TrunkRed   `json:"red"`
	Entries    []EntryRef `json:"entries"`
	RecordedAt string     `json:"recordedAt,omitempty"`
}

// HoldTrunkRed durably holds a trunk-red observation with the batch that saw it.
func (store Store) HoldTrunkRed(id string, red TrunkRed, opid string, at time.Time, actor string) error {
	return store.Update(id, func(record *Record) error {
		if record.State != StateProving && record.State != StateDiagnosing {
			return fmt.Errorf("batch %s cannot hold trunk red from state %s", id, record.State)
		}
		if opid == "" {
			return fmt.Errorf("batch %s cannot hold trunk red with an empty opid", id)
		}
		if len(red.Groups) == 0 {
			return fmt.Errorf("batch %s cannot hold trunk red without red groups", id)
		}
		if len(record.Units) == 0 {
			return fmt.Errorf("batch %s cannot hold trunk red without units", id)
		}
		red.BatchID = id
		red.Joiners = make([]Claim, len(record.Units))
		for index, unit := range record.Units {
			red.Joiners[index] = unit.Claim
		}
		red.SeenAt = at
		record.TrunkRed = &TrunkRedHold{Opid: opid, Opids: []string{opid}, Red: red}
		groupIDs := make([]string, len(red.Groups))
		for index, group := range red.Groups {
			groupIDs[index] = group.ID
		}
		detail := fmt.Sprintf("attempt=%s groups=%s opid=%s", red.AttemptID, strings.Join(groupIDs, ","), opid)
		record.Transition(StateHeldTrunkRed, at, "trunk-red-hold", actor, detail)
		return nil
	})
}
