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

	"crypto/rand"
	"golang.org/x/sys/unix"
	"io"
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
	if record.Class != UnitReadFindingsClass || key == "" || strings.ContainsAny(key, "/\x00") || strings.HasPrefix(key, ".") || p.UnitRoot == "" {
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

// unitReadFindingsPrefix names a findings store: <prefix><named key>.
const unitReadFindingsPrefix = "metasystem-unit-read."

// UnitReadFindingsName is the findings store's name for a unit's named key.
func UnitReadFindingsName(key string) string { return unitReadFindingsPrefix + key }

// UnitReadFindingsOwner is the owner a findings store at path is registered
// to, when path is one (TMPDIR/metasystem/<prefix><key>).
func UnitReadFindingsOwner(path string) (Owner, bool) {
	key, ok := strings.CutPrefix(filepath.Base(path), unitReadFindingsPrefix)
	if !ok || key == "" || filepath.Base(filepath.Dir(path)) != processScratchParent {
		return Owner{}, false
	}
	return Owner{Kind: OwnerUnit, Ref: key}, true
}

// PrepareTempStore readies an owner's registered temporary store at path for
// its next use (Round D3, the findings store between a unit's rounds): a
// store that is there, and is the recorded directory, is emptied in place
// (every entry but its marker removed; the directory and its inode kept)
// under the record lock held shared; one that is gone is made again at the
// same path through the registry, its new identity recorded under the
// record lock; one whose record is released is registered anew. A path that
// is not the recorded store is refused and left as it is.
func PrepareTempStore(ctx context.Context, path, class string, owner Owner) error {
	registry, err := machineRegistry()
	if err != nil {
		return err
	}
	return prepareTempStore(ctx, registry, path, class, owner, rand.Reader)
}

func prepareTempStore(ctx context.Context, registry Registry, path, class string, owner Owner, entropy io.Reader) error {
	if filepath.Base(filepath.Dir(path)) != processScratchParent {
		return fmt.Errorf("temporary store %s is not in a TMPDIR/%s directory", path, processScratchParent)
	}
	records, unreadable := registry.Inventory()
	for _, bad := range unreadable {
		return fmt.Errorf("store registry holds an unreadable record %s: %s", bad.Path, bad.Reason)
	}
	id := ""
	for _, record := range records {
		if record.Path == path && record.State != StateReleased {
			if record.Owner != owner || record.Class != class {
				return fmt.Errorf("%s is %s %s's %s store, not this owner's", path, record.Owner.Kind, record.Owner.Ref, record.Class)
			}
			id = record.ID
		}
	}
	if id == "" {
		_, err := createTempStore(filepath.Dir(filepath.Dir(path)), registry, filepath.Base(path), class, owner, entropy)
		return err
	}
	if _, err := os.Lstat(path); errors.Is(err, os.ErrNotExist) {
		return restoreTempStore(registry, id, path, class, owner, entropy)
	}
	entrant, err := registry.Enter(id)
	if errors.Is(err, ErrStoreGone) {
		_, err := createTempStore(filepath.Dir(filepath.Dir(path)), registry, filepath.Base(path), class, owner, entropy)
		return err
	}
	if err != nil {
		return err
	}
	defer entrant.Leave()
	if err := Revalidate(entrant.Record); err != nil {
		return fmt.Errorf("%s is not the recorded store, so it is left as it is: %w", path, err)
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.Name() == MarkerName {
			continue
		}
		child := filepath.Join(path, entry.Name())
		if entry.Type()&os.ModeSymlink != 0 {
			err = removeEntry(ctx, child, nil)
		} else {
			err = removeTree(ctx, child, nil)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

// restoreTempStore makes an accepted store whose directory is gone again at
// its recorded path, and records the new directory's identity and marker,
// under the record lock (the owner's own act, which waits for a sweeper's
// critical section). A store released meanwhile is registered anew.
func restoreTempStore(registry Registry, id, path, class string, owner Owner, entropy io.Reader) error {
	critical, err := registry.lockExclusive(id)
	if err != nil {
		return err
	}
	record := critical.Record()
	if record.State == StateReleased || record.State == StateReleasing {
		_ = critical.Release()
		_, err := createTempStore(filepath.Dir(filepath.Dir(path)), registry, filepath.Base(path), class, owner, entropy)
		return err
	}
	defer critical.Release()
	if record.Path != path || record.Owner != owner || record.Class != class || !record.Identity.Marker {
		return fmt.Errorf("store record %s changed under its lock", id)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	if err := os.Mkdir(path, 0o700); err != nil {
		return fmt.Errorf("temporary store %s: %w", path, err)
	}
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	device, inode, _ := fileID(info)
	record.Identity.RootDevice, record.Identity.RootInode, record.Identity.RootGeneration = device, inode, pathGeneration(path)
	record.Notes = appendNote(record.Notes, "the directory was gone and was made again by its owner")
	if err := critical.Write(record); err != nil {
		return errors.Join(err, os.Remove(path))
	}
	return WriteMarker(record)
}
