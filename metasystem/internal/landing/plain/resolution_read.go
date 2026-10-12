package plain

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
)

func resolutionReadBound(install string) (float64, error) {
	raw, _, err := config.Get(config.GetParams{Key: "landing.read-minutes", ConfPath: filepath.Join(install, "metasystem.conf")})
	if errors.Is(err, os.ErrNotExist) {
		raw, err = config.MustDefault("landing.read-minutes"), nil
	}
	bound, parseErr := strconv.ParseFloat(raw, 64)
	if err == nil && (parseErr != nil || bound <= 0) {
		err = fmt.Errorf("landing.read-minutes must be positive, got %q", raw)
	}
	return bound, err
}

// ExpireResolutionRead stops a recorded lane-owned read only when its hand-in may return.
// stop must confirm every read job ended; a finished successful read is left alone.
func ExpireResolutionRead(install string, stop func(string) (bool, error), seams ProveSeams) (*Entry, error) {
	fix, err := ReadFix(install)
	if err != nil || fix == nil || fix.Tip == "" || fix.State != "reviewing" || !fix.ReadLaneOwned || fix.Read == "" {
		return nil, err
	}
	fix.UpdateReadMinutes(seams.now())
	bound, err := resolutionReadBound(install)
	if err != nil || fix.ReadMinutes <= bound {
		return nil, err
	}
	var returned *Entry
	err = withLock(install, func() error {
		current, err := readFix(install, false, fix.Goal)
		if err != nil || current == nil || current.State != "reviewing" || current.Attempt != fix.Attempt || current.Read != fix.Read || !current.ReadLaneOwned {
			return err
		}
		latest, ok, err := Latest(install, fix.Goal)
		if err != nil {
			return err
		}
		if !ok || latest.SHA != fix.Tip || latest.State != StateWaiting {
			return fmt.Errorf("%w: the resolution of %s belongs to an older hand-in", ErrNotWaiting, fix.Goal)
		}
		cause := &Cause{Kind: "own", Goal: fix.Goal, SHA: fix.Tip, Evidence: fix.Commit}
		if _, err := checkReturnLocked(install, latest, cause, nil, seams.now(), seams); err != nil {
			var refused *Refusal
			if !errors.As(err, &refused) {
				return err
			}
			current.ReadMinutes = fix.ReadMinutes
			current.Reason = fmt.Sprintf("merge read ran %g min, bound %g; %s; a person can use landing give-back %s", fix.ReadMinutes, bound, refused.Reason, fix.Goal)
			return WriteFix(install, current)
		}
		checkout := install
		if seams.Lane != nil {
			lane, err := seams.Lane()
			if err != nil {
				return err
			}
			checkout = lane.Root
		}
		stopped, err := stop(fix.Read)
		if err != nil || !stopped {
			return err
		}
		ref := "refs/heads/lane-merges/" + fix.Goal
		location := fmt.Sprintf("the automatic merge and the resolution are published at origin %s (%s)", ref, fix.Commit)
		if _, err := seams.git(checkout, "push", "origin", fix.Commit+":"+ref); err != nil {
			location = fmt.Sprintf("the merge could not be published: %s; the saved resolution patch is fixes/%s.resolution.patch", err, fix.Attempt)
		}
		reason := fmt.Sprintf("the resolution read of %s ran %g min past the bound of %g min; %s; review and hand in again", fix.Goal, fix.ReadMinutes-bound, bound, location)
		current.ReadMinutes, current.Reason = fix.ReadMinutes, reason
		if err := WriteFix(install, current); err != nil {
			return err
		}
		entry, _, err := returnLocked(install, fix.Goal, reason, cause, nil, seams.now(), seams)
		if entry.State == StateReturned {
			returned = &entry
		}
		return err
	})
	return returned, err
}
