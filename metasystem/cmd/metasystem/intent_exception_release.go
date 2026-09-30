package main

// The exception route's release set (design engine-owns-disk-lifetimes Part
// B, 3.6 "Release at landing is a recorded set", R22, U6d): work land G
// --exception composes the goal's whole landing candidate from the goal
// branch tip; at that selection the route records, once per exception, the
// goal's clean workspaces whose work that tip holds, in its own landing
// record artifacts/agents/landing-intent/<goal>/exception-<opid>/landed.json
// of the main checkout it lands into. The carried transaction names the
// commit it is about to push in the record before its single push and
// releases the set after it succeeded; a crash in between is finished by
// the next work land of the goal and by the sweeper's retry of landing
// release sets, which find the named commit on the remote-tracking ref (the
// staged record's shape, so every engine reads it the same way). A replay
// never selects a second set.

import (
	"os"
	"path/filepath"
)

// exceptionLandingPath is the exception's landing record in the main
// checkout primary, and its directory.
func exceptionLandingPath(primary, goalID, opid string) (string, string) {
	dir := filepath.Join(primary, "artifacts", "agents", "landing-intent", goalID, "exception-"+opid)
	return dir, filepath.Join(dir, "landed.json")
}

// recordExceptionRelease records the exception's release set, selected at
// the goal branch tip its composition was made from; an empty tip (one
// that moved while the candidate was composed, or could not be read)
// records nothing, so nothing is released. A repeat keeps the first set.
func (inv *intentInvocation) recordExceptionRelease(primary, goalID, opid, tip string) error {
	if tip == "" || opid == "" {
		return nil
	}
	dir, landedPath := exceptionLandingPath(primary, goalID, opid)
	_, err := inv.recordReleaseSetAt(dir, landedPath, goalID, tip, func(landed *intentLanded) { landed.Exception = opid })
	return err
}

// exceptionRelease is the carried transaction's release owners for one
// exception: record names the commit about to be pushed and the
// remote-tracking ref it goes to; landed marks the record landed and
// releases its set. An exception with no recorded set records and releases
// nothing.
func (inv *intentInvocation) exceptionRelease(primary, goalID, opid string) carriedRelease {
	_, landedPath := exceptionLandingPath(primary, goalID, opid)
	recorded := func() bool {
		_, err := os.Stat(landedPath)
		return err == nil
	}
	return carriedRelease{
		record: func(commit, branch string) error {
			if !recorded() {
				return nil
			}
			return inv.updateLanding(landedPath, func(landed *intentLanded) bool {
				endpoint := "refs/remotes/origin/" + branch
				if landed.Swept || landed.ReleaseSet == nil || landed.Staged && landed.Subject == commit && landed.Endpoint == endpoint {
					return false
				}
				landed.Staged, landed.Subject, landed.Endpoint, landed.Branch, landed.Exception = true, commit, endpoint, branch, opid
				return true
			})
		},
		landed: func(commit string) {
			if !recorded() {
				return
			}
			if err := inv.updateLanding(landedPath, func(landed *intentLanded) bool {
				if landed.Swept || landed.Subject != commit {
					return false
				}
				landed.Landing, landed.Swept = commit, true
				return true
			}); err != nil {
				return
			}
			_, _ = inv.runReleaseSet(landedPath)
		},
	}
}

// finishExceptionRelease finishes the exception's set after its landing
// completed (a landing recovered from its recorded consumption pushes
// nothing in this process): a record whose named commit the
// remote-tracking ref holds is marked landed and released. It returns the
// record, or false when the exception has none.
func (inv *intentInvocation) finishExceptionRelease(primary, goalID, opid string) (intentLanded, bool) {
	_, landedPath := exceptionLandingPath(primary, goalID, opid)
	if _, err := os.Stat(landedPath); err != nil {
		return intentLanded{}, false
	}
	landed, err := inv.runReleaseSet(landedPath)
	return landed, err == nil
}
