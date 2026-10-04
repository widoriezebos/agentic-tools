package main

// Release at landing (design engine-owns-disk-lifetimes Part B, 3.6
// "Release at landing is a recorded set", R22): the hand route records
// the goal's workspaces whose work the landing selects in its own
// landed.json before the push, releases exactly those once the merged
// branch is swept, and every later work land of the goal finishes a set
// left unfinished. A replay never selects a second set.

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/diskstore"
)

// recordReleaseSet returns the landing's record with its release set,
// selecting and writing the set when the record has none yet, under the
// record's lock.
func (inv *intentInvocation) recordReleaseSet(dir, landedPath, goalID, subject string) (intentLanded, error) {
	return inv.recordReleaseSetAt(dir, landedPath, goalID, subject, func(landed *intentLanded) { landed.Subject = subject })
}

// recordReleaseSetAt is recordReleaseSet for a landing whose selected tip
// is not the commit it pushes (the exception route's carried commit is
// composed onto main from the goal branch tip): the set is selected at tip,
// and fill completes the new record.
func (inv *intentInvocation) recordReleaseSetAt(dir, landedPath, goalID, tip string, fill func(*intentLanded)) (intentLanded, error) {
	var landed intentLanded
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return landed, err
	}
	release, err := diskstore.LockLandingRecord(landedPath, true)
	if err != nil {
		return landed, err
	}
	defer release()
	if encoded, err := os.ReadFile(landedPath); err == nil && json.Unmarshal(encoded, &landed) == nil && landed.ReleaseSet != nil {
		return landed, nil
	}
	owners := inv.owners.disk.withDefaults()
	set, err := diskstore.SelectReleaseSet(context.Background(), diskstore.CheckoutRegistry(inv.stateRoot), inv.layout.GitRoot, goalID, tip, owners.git)
	if err != nil {
		return landed, err
	}
	fill(&landed)
	landed.ReleaseSet = &set
	return landed, writeIntentInputs(dir, map[string]string{landedPath: mustJSON(landed)})
}

// runReleaseSet releases the pending stores of a swept landing's set under
// the record's lock, reading the record afresh, and returns it as written.
func (inv *intentInvocation) runReleaseSet(landedPath string) (intentLanded, error) {
	release, err := diskstore.LockLandingRecord(landedPath, true)
	if err != nil {
		return intentLanded{}, err
	}
	defer release()
	var landed intentLanded
	encoded, err := os.ReadFile(landedPath)
	if err != nil {
		return landed, err
	}
	if err := json.Unmarshal(encoded, &landed); err != nil {
		return landed, err
	}
	stagedPushed := false
	if landed.Staged && !landed.Swept && landed.Subject != "" && strings.HasPrefix(landed.Endpoint, "refs/remotes/") {
		// A staged landing that crashed after its push: the remote-tracking
		// ref it was pushed to holds its commit.
		if _, err := inv.owners.disk.withDefaults().git(context.Background(), inv.layout.GitRoot, "merge-base", "--is-ancestor", landed.Subject, landed.Endpoint); err == nil {
			landed.Landing, landed.Swept, stagedPushed = landed.Subject, true, true
		}
	}
	if landed.ReleaseSet == nil || landed.ReleaseSet.Finished() || !landed.Swept {
		return landed, nil
	}
	owners := inv.owners.disk.withDefaults()
	request := diskstore.WorkspaceReleaseRequest{Registry: diskstore.CheckoutRegistry(inv.stateRoot), GitRoot: inv.layout.GitRoot,
		Git: owners.git, TakeCensus: owners.census, By: "the landing of " + landed.Landing, Now: owners.now().UTC()}
	if !diskstore.RunReleaseSet(context.Background(), request, landed.ReleaseSet) && !stagedPushed {
		return landed, nil
	}
	return landed, writeIntentInputs(filepath.Dir(landedPath), map[string]string{landedPath: mustJSON(landed)})
}

// releaseSummary says what a landing's release set did, for the landing's
// own summary; empty when the set is empty.
func releaseSummary(set *diskstore.ReleaseSet) string {
	if set == nil || len(set.Stores) == 0 {
		return ""
	}
	counts := map[string]int{}
	for _, entry := range set.Stores {
		counts[entry.State]++
	}
	parts := []string{}
	for _, state := range []string{diskstore.ReleaseReleased, diskstore.ReleaseKept, diskstore.ReleasePending, diskstore.ReleaseAbsent} {
		if counts[state] > 0 {
			parts = append(parts, fmt.Sprintf("%d %s", counts[state], state))
		}
	}
	return "; its workspaces: " + strings.Join(parts, ", ")
}

// finishReleaseSets retries every swept landing of the goal whose release
// set is unfinished; what stays pending waits for the next retry.
func (inv *intentInvocation) finishReleaseSets(base string) {
	entries, _ := filepath.Glob(filepath.Join(base, "*", "landed.json"))
	for _, path := range entries {
		_, _ = inv.runReleaseSet(path)
	}
}

// stagedLandingPath is a staged landing's record of the goal's release set:
// artifacts/agents/landing-intent/<goal>/staged-<commit>/landed.json.
func (inv *intentInvocation) stagedLandingPath(goalID, commit string) (string, string) {
	dir := inv.layout.InstallationRoot.Path("artifacts", "agents", "landing-intent", goalID, "staged-"+commit)
	return dir, filepath.Join(dir, "landed.json")
}

// recordStagedRelease records, before the push, the goal's workspaces the
// commit contains, with the remote-tracking ref it is pushed to; a repeat
// for the same commit writes nothing.
func (inv *intentInvocation) recordStagedRelease(goalID, commit, branch string) error {
	dir, landedPath := inv.stagedLandingPath(goalID, commit)
	if _, err := inv.recordReleaseSet(dir, landedPath, goalID, commit); err != nil {
		return err
	}
	return inv.updateLanding(landedPath, func(landed *intentLanded) bool {
		if landed.Staged {
			return false
		}
		landed.Staged, landed.Endpoint, landed.Branch = true, "refs/remotes/origin/"+branch, branch
		return true
	})
}

// releaseStagedLanding marks the staged landing pushed and releases its
// set; what stays pending is finished by the next work land of the goal or
// the steward's disk pass.
func (inv *intentInvocation) releaseStagedLanding(goalID, commit string) {
	_, landedPath := inv.stagedLandingPath(goalID, commit)
	if err := inv.updateLanding(landedPath, func(landed *intentLanded) bool {
		if landed.Swept {
			return false
		}
		landed.Landing, landed.Swept = commit, true
		return true
	}); err != nil {
		return
	}
	_, _ = inv.runReleaseSet(landedPath)
}

// updateLanding changes one landing record under its lock.
func (inv *intentInvocation) updateLanding(landedPath string, change func(*intentLanded) bool) error {
	release, err := diskstore.LockLandingRecord(landedPath, true)
	if err != nil {
		return err
	}
	defer release()
	var landed intentLanded
	encoded, err := os.ReadFile(landedPath)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(encoded, &landed); err != nil {
		return err
	}
	if !change(&landed) {
		return nil
	}
	return writeIntentInputs(filepath.Dir(landedPath), map[string]string{landedPath: mustJSON(landed)})
}
