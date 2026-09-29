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
	"os"
	"path/filepath"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/diskstore"
)

// recordReleaseSet returns the landing's record with its release set,
// selecting and writing the set when the record has none yet.
func (inv *intentInvocation) recordReleaseSet(dir, landedPath, goalID, subject string) (intentLanded, error) {
	var landed intentLanded
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

// runReleaseSet releases the pending stores of a swept landing's set and
// rewrites the record when an entry changed.
func (inv *intentInvocation) runReleaseSet(landedPath string, landed *intentLanded) error {
	if landed.ReleaseSet == nil || landed.ReleaseSet.Finished() || !landed.Swept {
		return nil
	}
	owners := inv.owners.disk.withDefaults()
	request := diskstore.WorkspaceReleaseRequest{Registry: diskstore.CheckoutRegistry(inv.stateRoot), GitRoot: inv.layout.GitRoot,
		Git: owners.git, Census: owners.census(), By: "the landing of " + landed.Landing, Now: owners.now().UTC()}
	if !diskstore.RunReleaseSet(context.Background(), request, landed.ReleaseSet) {
		return nil
	}
	return writeIntentInputs(filepath.Dir(landedPath), map[string]string{landedPath: mustJSON(*landed)})
}

// finishReleaseSets retries every swept landing of the goal whose release
// set is unfinished; what stays pending waits for the next retry.
func (inv *intentInvocation) finishReleaseSets(base string) {
	entries, _ := filepath.Glob(filepath.Join(base, "*", "landed.json"))
	for _, path := range entries {
		var landed intentLanded
		if encoded, err := os.ReadFile(path); err != nil || json.Unmarshal(encoded, &landed) != nil {
			continue
		}
		_ = inv.runReleaseSet(path, &landed)
	}
}
