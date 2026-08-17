package janitor

import (
	"fmt"
	"math"
	"os"
	"sort"
	"strings"
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

// Headroom checks free space on the filesystems the given paths
// touch, each against floorBytes. Paths sharing a device id collapse
// to one result, but that dedup exists ONLY to suppress duplicate
// warnings — entries are PER-PATH advisory checks and must never be
// summed or treated as independent pools: APFS volumes in one
// container report DISTINCT device ids while sharing one free-space
// pool (verified on this host: Data/VM/Preboot each report the
// container's free bytes), so any aggregate or pressure decision
// needs a real pool identity this measurement does not provide
// (plans/opus-window-review-dh.md finding 3). A path whose
// filesystem cannot be established is an error, never a silent
// skip: an unmeasurable filesystem is a refusal, not a pass.
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
	// The measurement is FD-PINNED: the deepest existing ancestor is
	// opened once, and both the device id (Fstat) and the free bytes
	// (Fstatfs) come from that one open file — a rename, mount, or
	// symlink swap between two path-based calls can no longer pair one
	// filesystem's identity with another's capacity (review finding 2).
	// Only ENOENT ascends to the parent (a not-yet-created target
	// still has a filesystem to measure); EACCES, ELOOP, ENOTDIR, and
	// every other failure REFUSES — the requested path could not be
	// established, and converting that into an answer about some
	// ancestor reported success for a measurement that never happened
	// (review finding 1).
	// The walk uses the RAW path: filepath.Clean before opening erased
	// components that should refuse — 'file/../.' cleans to '.' and
	// measured the working directory while the real path is ENOTDIR
	// (verification round S3). The open is O_NONBLOCK so a FIFO without
	// a writer cannot hang the guard (M5); directories and regular
	// files are unaffected.
	current := path
	for {
		fd, openErr := syscall.Open(current, syscall.O_RDONLY|syscall.O_NONBLOCK|syscall.O_CLOEXEC, 0)
		if openErr == nil {
			f := os.NewFile(uintptr(fd), current)
			var fs syscall.Statfs_t
			if err := syscall.Fstatfs(int(f.Fd()), &fs); err != nil {
				f.Close()
				return 0, 0, fmt.Errorf("fstatfs %s: %w", current, err)
			}
			info, err := f.Stat()
			f.Close()
			if err != nil {
				return 0, 0, fmt.Errorf("fstat %s: %w", current, err)
			}
			sys, ok := info.Sys().(*syscall.Stat_t)
			if !ok {
				return 0, 0, fmt.Errorf("stat sys is not a Stat_t for %s", current)
			}
			// Checked unsigned multiply: a pathological Bavail*Bsize
			// saturates at MaxInt64 instead of wrapping negative
			// (review finding 5).
			bavail := uint64(fs.Bavail)
			bsize := uint64(fs.Bsize)
			var free int64
			if bsize != 0 && bavail > uint64(math.MaxInt64)/bsize {
				free = math.MaxInt64
			} else {
				free = int64(bavail * bsize)
			}
			return free, uint64(sys.Dev), nil
		}
		if openErr != syscall.ENOENT {
			return 0, 0, fmt.Errorf("cannot establish %s: %w", current, openErr)
		}
		parent, ok := rawParent(current)
		if !ok {
			return 0, 0, fmt.Errorf("no existing ancestor filesystem for the path")
		}
		current = parent
	}
}

// rawParent strips the last path component WITHOUT cleaning: filepath.Dir
// internally Cleans, which let 'absent/../x' ascend into '.' after the
// ENOENT — measuring a directory the requested path never named
// (verification round 2, finding 3). A remaining '.' or '..' component
// at the tail is refused outright: ascending across dot components from
// a missing entry has no sound lexical meaning for a guard.
func rawParent(path string) (string, bool) {
	trimmed := strings.TrimRight(path, "/")
	if trimmed == "" || trimmed == "/" {
		return "", false
	}
	idx := strings.LastIndexByte(trimmed, '/')
	if idx < 0 {
		return "", false
	}
	parent := trimmed[:idx]
	if parent == "" {
		parent = "/"
	}
	tail := trimmed[idx+1:]
	if tail == "." || tail == ".." {
		return "", false
	}
	base := parent[strings.LastIndexByte(strings.TrimRight(parent, "/"), '/')+1:]
	if base == "." || base == ".." {
		return "", false
	}
	return parent, true
}
