package diskstore

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	armed "github.com/widoriezebos/agentic-tools/metasystem/internal/registry"
)

// A temporary store is a registered marker store in TMPDIR/metasystem that
// outlives the process that made it: its owner (a read attempt's context,
// DL-18) releases it at its own end, never a process's release. Its record
// is in the machine registry before it holds a byte, and the record stays as
// history after the release.

// errNoTempStore is a path no unreleased record of the owner names.
var errNoTempStore = errors.New("no store record of this owner names the path")

// CreateTempStore makes the temporary store name for owner in the process's
// TMPDIR. A path it did not make is refused, and kept; an interrupted
// earlier try of the same owner is released through its own record first.
func CreateTempStore(name, class string, owner Owner) (string, error) {
	registry, err := machineRegistry()
	if err != nil {
		return "", err
	}
	return createTempStore(scratchTempRoot(), registry, name, class, owner, rand.Reader)
}

// ReleaseTempStore is the owner's release of its temporary store at path,
// once the owner has proved nothing uses it.
func ReleaseTempStore(ctx context.Context, path, class string, owner Owner) error {
	registry, err := machineRegistry()
	if err != nil {
		return err
	}
	return releaseTempStore(ctx, registry, path, class, owner)
}

func machineRegistry() (Registry, error) {
	path, err := armed.DefaultPath()
	if err != nil {
		return Registry{}, fmt.Errorf("store registry: %w", err)
	}
	if !filepath.IsAbs(path) {
		return Registry{}, fmt.Errorf("store registry: the home state root %q is not absolute", filepath.Dir(path))
	}
	return MachineRegistry(filepath.Dir(path)), nil
}

func createTempStore(tempRoot string, registry Registry, name, class string, owner Owner, entropy io.Reader) (string, error) {
	if name == "" || strings.ContainsAny(name, "/\x00") || strings.HasPrefix(name, ".") {
		return "", fmt.Errorf("temporary store name %q must be one plain path element", name)
	}
	root, err := filepath.EvalSymlinks(tempRoot)
	if err != nil {
		return "", fmt.Errorf("temporary store: %w", err)
	}
	// A TMPDIR inside a registered store (a process scratch root a child
	// was given) is refused: the outer store's release would take this
	// one with it.
	for dir := root; ; dir = filepath.Dir(dir) {
		if _, err := os.Lstat(filepath.Join(dir, MarkerName)); err == nil {
			return "", fmt.Errorf("temporary store: TMPDIR %s lies inside the registered store %s", root, dir)
		}
		if dir == filepath.Dir(dir) {
			break
		}
	}
	parent := filepath.Join(root, processScratchParent)
	if err := os.MkdirAll(parent, 0o700); err != nil {
		return "", fmt.Errorf("temporary store: %w", err)
	}
	path := filepath.Join(parent, name)
	if err := releaseTempStore(context.Background(), registry, path, class, owner); err != nil && !errors.Is(err, errNoTempStore) {
		return "", fmt.Errorf("temporary store %s: an earlier try could not be released: %w", path, err)
	}
	// Made empty first: the kernel stamps the instant the record carries
	// (this package reads no clock), and its device and inode are recorded
	// before it holds a byte. A path that exists is not this request's.
	if err := os.Mkdir(path, 0o700); err != nil {
		if errors.Is(err, os.ErrExist) {
			return "", fmt.Errorf("temporary store %s exists and no record of %s %s names it; it is kept: metasystem disk show", path, owner.Kind, owner.Ref)
		}
		return "", fmt.Errorf("temporary store: %w", err)
	}
	info, err := os.Lstat(path)
	if err != nil {
		return "", errors.Join(fmt.Errorf("temporary store: %w", err), os.Remove(path))
	}
	device, inode, _ := fileID(info)
	record, err := registry.Register(Registration{Path: path, Class: class, Owner: owner, Lifetime: LifetimeOwner, CapKind: CapNone,
		RootDevice: device, RootInode: inode, RootGeneration: pathGeneration(path)}, info.ModTime(), entropy)
	if err != nil {
		// Unregistered and still empty: os.Remove removes only an empty
		// directory.
		return "", errors.Join(fmt.Errorf("temporary store: %w", err), os.Remove(path))
	}
	if err := WriteMarker(record); err != nil {
		return "", err
	}
	if _, err := registry.Accept(record.ID); err != nil {
		return "", err
	}
	return path, nil
}

// releaseTempStore removes the owner's store at path inside its critical
// section. The fail-closed rules for every removal bind it:
//  1. an unreadable record in the registry, an unreadable root or marker
//     holds the release;
//  2. the path only finds the record; the store is the recorded directory
//     by device and inode, never by the path string;
//  3. the record stays as history; only the store's payload goes;
//  4. the owner proves nothing uses the store before it asks, on every
//     attempt, and a cut-short removal stays releasing for the next;
//  5. the store is the owner's, never a person's.
func releaseTempStore(ctx context.Context, registry Registry, path, class string, owner Owner) error {
	records, unreadable := registry.Inventory()
	for _, bad := range unreadable {
		if _, err := os.Lstat(bad.Path); errors.Is(err, os.ErrNotExist) {
			continue // released and removed by its owner since the listing
		}
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
		return errNoTempStore
	}
	critical, err := registry.TryCritical(id)
	if err != nil {
		return err
	}
	defer critical.Release()
	record := critical.Record()
	if record.Owner != owner || record.Class != class || record.Path != path || !record.Identity.Marker {
		return fmt.Errorf("store record %s changed under its lock", id)
	}
	info, err := os.Lstat(path)
	switch {
	case errors.Is(err, os.ErrNotExist):
	case err != nil:
		return err
	default:
		if err := sameRoot(record, info); err != nil {
			return err
		}
		// A reserved store (its creation cut short) is its own only while it
		// carries its marker or is empty (Round D1 F-6).
		if record.State == StateReleasing || record.State == StateReserved {
			err = revalidateReleasing(record)
		} else {
			err = Revalidate(record)
		}
		if err != nil {
			return err
		}
		if record.State != StateReleasing {
			record.State = StateReleasing
			if err := critical.Write(record); err != nil {
				return err
			}
		}
		if err := RemoveStore(ctx, record); err != nil {
			return fmt.Errorf("removal cut short; the store is releasing: %w", err)
		}
	}
	record = critical.Record()
	record.State, record.ReleasedBy = StateReleased, "owner"
	return critical.Write(record)
}
