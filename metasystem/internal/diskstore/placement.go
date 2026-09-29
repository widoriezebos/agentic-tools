package diskstore

// Placement (engine-owns-disk-lifetimes Part B, 3.12 "Placement rules",
// R21, R22): caches and source copies never live under an evidence root,
// and a rebuildable store never enters one. The shapes below classify a
// tree for the report and the refusal text only; nothing is removed or
// skipped by a name match.

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/placement"
)

// The two misplacement kinds a report names.
const (
	PlacementCache      = placement.Cache
	PlacementSourceCopy = placement.SourceCopy
)

// Placement is a tree's misplacement (placement.Placement).
type Placement = placement.Placement

// PlacementOf classifies one directory (placement.Of).
func PlacementOf(path string) Placement { return placement.Of(path) }

// MisplacedLine is the report line of a misplaced tree of bytes: its kind,
// its shape and size, and the removal a person runs once nothing needs it
// (such a tree is never an evidence item, so a person removes it by hand;
// Round B2-3 rule 3).
func MisplacedLine(path string, misplaced Placement, bytes int64) Line {
	return Line{Class: misplaced.Kind, Path: path,
		Reason:  fmt.Sprintf("%s under an evidence root, %s: it is not evidence and is never counted or managed there", misplaced.Shape, formatBytes(bytes)),
		Command: "a person removes it once nothing needs it: rm -rf -- " + shellQuote(path)}
}

// RebuildableHolding reports the registered, unreleased rebuildable store
// that holds path: the store's directory is path itself or one of its
// ancestors, compared by device and inode, never by spelling. A registry
// that cannot be read whole, or a store path that cannot be examined, is an
// error: nothing can say the path is free of one (fail-closed rule 1).
func (r Registry) RebuildableHolding(path string) (Record, bool, error) {
	records, unreadable := r.Inventory()
	if len(unreadable) > 0 {
		return Record{}, false, fmt.Errorf("store record %s is unreadable: %s", unreadable[0].Path, unreadable[0].Reason)
	}
	type key struct{ device, inode uint64 }
	stores := map[key]Record{}
	for _, record := range records {
		if record.Lifetime != LifetimeRebuildable || record.State == StateReleased {
			continue
		}
		info, err := os.Stat(record.Path)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return Record{}, false, fmt.Errorf("rebuildable store %s: %w", record.ID, err)
		}
		device, inode, ok := fileID(info)
		if !ok {
			return Record{}, false, fmt.Errorf("rebuildable store %s has no file identity", record.ID)
		}
		stores[key{device, inode}] = record
	}
	if len(stores) == 0 {
		return Record{}, false, nil
	}
	current, err := filepath.Abs(path)
	if err != nil {
		return Record{}, false, err
	}
	// Walk the physical ancestors: a spelling through a link into the
	// store still lies in it.
	if resolved, err := filepath.EvalSymlinks(current); err == nil {
		current = resolved
	}
	for {
		info, err := os.Stat(current)
		if err == nil {
			if device, inode, ok := fileID(info); ok {
				if record, held := stores[key{device, inode}]; held {
					return record, true, nil
				}
			}
		} else if !errors.Is(err, os.ErrNotExist) {
			return Record{}, false, err
		}
		parent := filepath.Dir(current)
		if parent == current {
			return Record{}, false, nil
		}
		current = parent
	}
}
