package janitor

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"syscall"
)

// The disk-hygiene headroom guard (backlog item 19, implementation-
// first per D81): the ENOSPC incident that motivated the goal was a
// full disk masquerading as a code failure. This guard makes a full
// disk NAME ITSELF, per physical filesystem, before the suite or a
// provision path assumes space. It is the first shipped slice; the
// class registry, sweep, and journal follow behind their fixtures
// (plans/disk-hygiene-design.md, plans/dh-critique-r1..r3.md).

// FilesystemHeadroom is the free-space fact for one physical
// filesystem, named by the device id its representative path sits
// on.
type FilesystemHeadroom struct {
	// Path is a representative path that resolves onto this device.
	Path string
	// Device is the filesystem's device id — distinct paths on the
	// same device dedup to one entry.
	Device uint64
	// FreeBytes is the space available to a non-privileged writer.
	FreeBytes int64
	// FloorBytes is the required minimum for this check.
	FloorBytes int64
}

// BelowFloor reports whether this filesystem is under its floor.
func (f FilesystemHeadroom) BelowFloor() bool { return f.FreeBytes < f.FloorBytes }

// Deficit is how many bytes short of the floor this filesystem is,
// or zero when it meets the floor.
func (f FilesystemHeadroom) Deficit() int64 {
	if f.FreeBytes >= f.FloorBytes {
		return 0
	}
	return f.FloorBytes - f.FreeBytes
}

// Headroom checks free space on the DISTINCT physical filesystems
// the given paths touch, each against floorBytes. Paths on the same
// device collapse to one result (keyed by device id), so a floor is
// evaluated once per filesystem, not once per path — the design's
// per-filesystem rule. A path whose filesystem cannot be stat'd is
// returned as an error, never silently skipped: an unmeasurable
// filesystem is a refusal, not a pass.
func Headroom(paths []string, floorBytes int64) ([]FilesystemHeadroom, error) {
	byDevice := map[uint64]FilesystemHeadroom{}
	for _, path := range paths {
		free, device, err := statFilesystem(path)
		if err != nil {
			return nil, fmt.Errorf("headroom: cannot measure %s: %w", path, err)
		}
		if _, seen := byDevice[device]; seen {
			continue
		}
		byDevice[device] = FilesystemHeadroom{
			Path:       path,
			Device:     device,
			FreeBytes:  free,
			FloorBytes: floorBytes,
		}
	}
	results := make([]FilesystemHeadroom, 0, len(byDevice))
	for _, r := range byDevice {
		results = append(results, r)
	}
	sort.Slice(results, func(i, j int) bool { return results[i].Path < results[j].Path })
	return results, nil
}

// statFilesystem returns the free bytes available to an ordinary
// writer and the device id for the filesystem containing path.
func statFilesystem(path string) (int64, uint64, error) {
	// Device id from Stat (Stat_t.Dev, cross-platform) and free
	// bytes from Statfs (Bavail*Bsize, cross-platform) — avoiding
	// the platform-specific Fsid representation entirely. Walk up to
	// the deepest existing ancestor: a not-yet-created target still
	// has a filesystem to measure.
	current := filepath.Clean(path)
	for {
		info, statErr := os.Stat(current)
		if statErr == nil {
			var fs syscall.Statfs_t
			if err := syscall.Statfs(current, &fs); err != nil {
				return 0, 0, err
			}
			sys, ok := info.Sys().(*syscall.Stat_t)
			if !ok {
				return 0, 0, fmt.Errorf("stat sys is not a Stat_t for %s", current)
			}
			return int64(fs.Bavail) * int64(fs.Bsize), uint64(sys.Dev), nil
		}
		parent := filepath.Dir(current)
		if parent == current {
			return 0, 0, fmt.Errorf("no existing ancestor filesystem for the path")
		}
		current = parent
	}
}
