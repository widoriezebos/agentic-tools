package diskstore

// A landing's release set (design engine-owns-disk-lifetimes Part B, 3.6
// "Release at landing is a recorded set"; R22): when a landing selects what
// lands, it records the goal's accepted workspaces whose work the selected
// tip contains; the landing's durable completion owner releases exactly
// those ids after the push, and its recovery and the sweeper retry an
// unfinished set. A workspace made after the set was recorded is never
// discovered by a replay of the same landing.

import (
	"context"
	"path/filepath"
	"sort"
)

// The states of one store in a release set.
const (
	ReleasePending  = "pending"
	ReleaseReleased = "released"
	ReleaseKept     = "kept"
	ReleaseAbsent   = "absent"
)

// ReleaseEntry is one store of a set and what its release did.
type ReleaseEntry struct {
	ID      string `json:"id"`
	Path    string `json:"path,omitempty"`
	State   string `json:"state"`
	Reason  string `json:"reason,omitempty"`
	Command string `json:"command,omitempty"`
	Archive string `json:"archive,omitempty"`
}

// ReleaseSet is the stores a landing ends, recorded in the landing's own
// record when the landing is selected.
type ReleaseSet struct {
	Tip    string         `json:"tip"`
	Stores []ReleaseEntry `json:"stores"`
}

// Finished reports a set with nothing left to retry: every store released,
// absent, or kept for a reason only a person or the goal's end settles.
func (s ReleaseSet) Finished() bool {
	for _, entry := range s.Stores {
		if entry.State == ReleasePending {
			return false
		}
	}
	return true
}

// SelectReleaseSet names the goal's accepted copies whose recorded copyOf
// or branch tip is an ancestor of tip (the commit the landing selects, a
// --through prefix included). It reads only.
func SelectReleaseSet(ctx context.Context, registry Registry, gitRoot, goalID, tip string, git WorkspaceGit) (ReleaseSet, error) {
	set := ReleaseSet{Tip: tip, Stores: []ReleaseEntry{}}
	records, _ := registry.Inventory()
	request := WorkspaceReleaseRequest{GitRoot: gitRoot, Git: git}
	for _, record := range records {
		if record.Class != WorkspaceClass || record.Layout != LayoutCopy || record.State != StateAccepted ||
			record.Owner != (Owner{Kind: OwnerGoal, Ref: goalID}) {
			continue
		}
		candidates := []string{record.CopyOf}
		if branchTip, found := revParse(ctx, request, "refs/heads/"+WorkspaceBranch(record.Owner, filepath.Base(record.Path))); found {
			candidates = append(candidates, branchTip)
		}
		for _, commit := range candidates {
			if commit == "" {
				continue
			}
			if _, err := git(ctx, gitRoot, "merge-base", "--is-ancestor", commit, tip); err == nil {
				set.Stores = append(set.Stores, ReleaseEntry{ID: record.ID, Path: record.Path, State: ReleasePending})
				break
			}
		}
	}
	sort.Slice(set.Stores, func(i, j int) bool { return set.Stores[i].ID < set.Stores[j].ID })
	return set, nil
}

// RunReleaseSet releases every pending store of set through
// ReleaseWorkspace and records each outcome; request supplies everything
// but the id. It reports whether any entry changed, so a repeat of a
// finished set writes nothing.
func RunReleaseSet(ctx context.Context, request WorkspaceReleaseRequest, set *ReleaseSet) bool {
	changed := false
	for index := range set.Stores {
		entry := &set.Stores[index]
		if entry.State != ReleasePending {
			continue
		}
		before := *entry
		request.ID = entry.ID
		outcome, err := ReleaseWorkspace(ctx, request)
		switch {
		case err != nil:
			entry.Reason, entry.Command = err.Error(), "metasystem disk show"
		case outcome.Done && outcome.Already:
			entry.State, entry.Reason = ReleaseAbsent, outcome.Reason
		case outcome.Done:
			entry.State, entry.Reason, entry.Archive = ReleaseReleased, "", outcome.Archive
		case outcome.Kept:
			entry.State, entry.Reason, entry.Command = ReleaseKept, outcome.Reason, outcome.Command
		default:
			entry.Reason, entry.Command = outcome.Reason, outcome.Command
		}
		changed = changed || *entry != before
	}
	return changed
}
