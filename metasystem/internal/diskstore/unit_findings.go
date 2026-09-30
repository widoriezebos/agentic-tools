package diskstore

// A unit read's findings directory (Round D3 N4): a registered temporary
// store owned by the unit's named inputs (owner ref: the named key), which
// outlives the command that made it because a later round's read writes
// into it. It ends with its unit: once the unit record is released, its
// named entry .named/<key>.json is gone, and with the named lock free (no
// build or review is preparing the name again) nothing can write into it.

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/unix"
)

// UnitReadFindingsClass is the class of a unit read's findings store.
const UnitReadFindingsClass = "unit-read-findings"

// UnitFindingsProof is the unit owner kind's proof for a findings store.
type UnitFindingsProof struct {
	// UnitRoot is ~/.metasystem/unit, which holds .named.
	UnitRoot string
}

func (UnitFindingsProof) Kind() OwnerKind { return OwnerUnit }

// Observe reads only: the unit's named entry and the named lock, probed
// without creating either.
func (p UnitFindingsProof) Observe(_ context.Context, record Record) Verdict {
	key := record.Owner.Ref
	if record.Class != UnitReadFindingsClass || key == "" || strings.ContainsAny(key, "/\\x00") || strings.HasPrefix(key, ".") || p.UnitRoot == "" {
		return Verdict{Decision: Pending, Reason: "a " + record.Class + " store of unit " + key + " has no proof in this engine", Command: "metasystem disk show"}
	}
	named := filepath.Join(p.UnitRoot, ".named", key+".json")
	if _, err := os.Lstat(named); err == nil {
		return Verdict{Decision: Keep, Reason: "its unit is still recorded (" + named + ")", Command: "metasystem work status"}
	} else if !errors.Is(err, os.ErrNotExist) {
		return Verdict{Decision: Pending, Reason: "the unit's named entry cannot be read: " + err.Error(), Command: "metasystem disk show"}
	}
	free, err := probeFree(filepath.Join(p.UnitRoot, ".named", key+".lock"))
	switch {
	case err != nil:
		return Verdict{Decision: Pending, Reason: "the unit's named lock cannot be read: " + err.Error(), Command: "metasystem disk show"}
	case !free:
		return Verdict{Decision: Pending, Reason: "a build or review is preparing the unit's name", Command: "metasystem disk clean, once it has ended"}
	}
	return Verdict{Decision: Release, Reason: "its unit record is released"}
}

// Apply removes the store's tree inside its critical section, the marker
// last.
func (UnitFindingsProof) Apply(ctx context.Context, critical *Critical) error {
	return RemoveStore(ctx, critical.Record())
}

// probeFree reports whether a lock file is free to LOCK_EX|LOCK_NB, without
// creating it; an absent file is free.
func probeFree(path string) (bool, error) {
	file, err := os.OpenFile(path, os.O_RDONLY, 0)
	if errors.Is(err, os.ErrNotExist) {
		return true, nil
	}
	if err != nil {
		return false, err
	}
	defer file.Close()
	if err := unix.Flock(int(file.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
		if errors.Is(err, unix.EWOULDBLOCK) || errors.Is(err, unix.EAGAIN) {
			return false, nil
		}
		return false, fmt.Errorf("probe %s: %w", path, err)
	}
	_ = unix.Flock(int(file.Fd()), unix.LOCK_UN)
	return true, nil
}
