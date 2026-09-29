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
	set, err := diskstore.SelectReleaseSet(context.Background(), diskstore.CheckoutRegistry(inv.stateRoot), inv.layout.GitRoot, goalID, subject, owners.git)
	if err != nil {
		return landed, err
	}
	landed.Subject, landed.ReleaseSet = subject, &set
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
	if landed.ReleaseSet == nil || landed.ReleaseSet.Finished() || !landed.Swept {
		return landed, nil
	}
	owners := inv.owners.disk.withDefaults()
	request := diskstore.WorkspaceReleaseRequest{Registry: diskstore.CheckoutRegistry(inv.stateRoot), GitRoot: inv.layout.GitRoot,
		Git: owners.git, TakeCensus: owners.census, By: "the landing of " + landed.Landing, Now: owners.now().UTC()}
	if !diskstore.RunReleaseSet(context.Background(), request, landed.ReleaseSet) {
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
