package batch

import (
	"fmt"
	"slices"
	"strings"
	"time"
)

// TrunkRedHold records a durable trunk-red observation for a batch.
type TrunkRedHold struct {
	Opid        string     `json:"opid"`
	Opids       []string   `json:"opids"`
	Red         TrunkRed   `json:"red"`
	Entries     []EntryRef `json:"entries"`
	RecordedAt  string     `json:"recordedAt,omitempty"`
	CheckedTree string     `json:"checkedTree,omitempty"`
}

// HoldTrunkRed durably holds a trunk-red observation with the batch that saw it.
func (store Store) HoldTrunkRed(id string, red TrunkRed, opid string, at time.Time, actor string) error {
	return store.holdTrunkRed(id, "", red, opid, at, actor)
}

// HoldRegisteredTrunkRed holds a landing batch on entries already published
// in the shared register. It records references instead of publishing a
// second sighting with reconstructed evidence.
func (store Store) HoldRegisteredTrunkRed(id string, entries []OpenEntry, baseCommit, baseTree, opid string, at time.Time, actor string) error {
	return store.Update(id, func(record *Record) error {
		if record.State != StateLanding {
			return fmt.Errorf("batch %s cannot hold registered trunk red from state %s", id, record.State)
		}
		if len(entries) == 0 || opid == "" || len(record.Units) == 0 {
			return fmt.Errorf("batch %s cannot hold registered trunk red without entries, opid, and units", id)
		}
		groups := make([]RedGroup, 0, len(entries))
		refs := make([]EntryRef, 0, len(entries))
		seen := map[string]bool{}
		for _, entry := range entries {
			if entry.ID == "" || entry.Group == "" || seen[entry.ID] {
				continue
			}
			seen[entry.ID] = true
			groups = append(groups, RedGroup{ID: entry.Group, Status: "failed"})
			refs = append(refs, EntryRef{ID: entry.ID, Group: entry.Group})
		}
		if len(refs) == 0 {
			return fmt.Errorf("batch %s cannot hold registered trunk red without valid entries", id)
		}
		red := TrunkRed{BatchID: id, AttemptID: "shared-trunk-red", BaseCommit: baseCommit, BaseTree: baseTree, Groups: groups,
			Joiners: make([]Claim, len(record.Units)), SeenAt: at}
		for index, unit := range record.Units {
			red.Joiners[index] = unit.Claim
		}
		record.TrunkRed = &TrunkRedHold{Opid: opid, Opids: []string{opid}, Red: red, Entries: refs,
			RecordedAt: at.UTC().Format(time.RFC3339Nano)}
		record.Transition(StateHeldTrunkRed, at, "trunk-red-hold", actor, "entries="+strings.Join(entryIDs(refs), ",")+" opid="+opid)
		return nil
	})
}

func entryIDs(entries []EntryRef) []string {
	ids := make([]string, len(entries))
	for index, entry := range entries {
		ids[index] = entry.ID
	}
	return ids
}

// reholdTrunkRed replaces an existing hold only while it still carries the
// operation that the diagnostic observed.
func (store Store) reholdTrunkRed(id, expectedOpid string, red TrunkRed, opid string, at time.Time, actor string) error {
	return store.holdTrunkRed(id, expectedOpid, red, opid, at, actor)
}

func (store Store) holdTrunkRed(id, expectedOpid string, red TrunkRed, opid string, at time.Time, actor string) error {
	return store.Update(id, func(record *Record) error {
		if record.State == StateHeldTrunkRed && record.TrunkRed != nil && record.TrunkRed.Opid == opid {
			return nil
		}
		if expectedOpid != "" {
			if record.State != StateHeldTrunkRed || record.TrunkRed == nil || record.TrunkRed.Opid != expectedOpid {
				return fmt.Errorf("batch %s moved before the new base red was recorded", id)
			}
		} else if record.State != StateProving && record.State != StateDiagnosing {
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
		if record.TrunkRed != nil && slices.Contains(record.TrunkRed.Opids, opid) {
			return fmt.Errorf("batch %s cannot reuse trunk-red opid %s", id, opid)
		}
		red.BatchID = id
		red.Joiners = make([]Claim, len(record.Units))
		for index, unit := range record.Units {
			red.Joiners[index] = unit.Claim
		}
		red.SeenAt = at
		if record.TrunkRed == nil {
			record.TrunkRed = &TrunkRedHold{Opid: opid, Opids: []string{opid}, Red: red}
		} else {
			record.TrunkRed.Opid = opid
			record.TrunkRed.Opids = append(record.TrunkRed.Opids, opid)
			record.TrunkRed.Red = red
			record.TrunkRed.Entries = nil
			record.TrunkRed.RecordedAt = ""
			record.TrunkRed.CheckedTree = ""
		}
		groupIDs := make([]string, len(red.Groups))
		for index, group := range red.Groups {
			groupIDs[index] = group.ID
		}
		detail := fmt.Sprintf("attempt=%s groups=%s opid=%s", red.AttemptID, strings.Join(groupIDs, ","), opid)
		record.Transition(StateHeldTrunkRed, at, "trunk-red-hold", actor, detail)
		return nil
	})
}
