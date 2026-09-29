package evidence

// The bound in the machine pass (design engine-owns-disk-lifetimes Part B
// 3.12 "Roots", "Where it runs"; DL4D-12): the steward that wins the
// machine flock runs, over every evidence root of the host, the citation
// generation's next step, the host registry of roots and the segment
// indexes, each segment's compaction loop with its checkout as context,
// the blob reference check and sweep, and the machine cap. Each root is
// named with its owner; a segment no armed checkout resolves is an orphan,
// reported and Unknown to the bound.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/diskstore"
	"golang.org/x/sys/unix"
)

// HostCheckout is one armed checkout as the machine pass sees it.
type HostCheckout struct {
	Installation string
	Facts        diskstore.CheckoutFacts
	FactsErr     error
	Settings     diskstore.Settings
	SettingsErr  error
}

// RootEntry is one line of the host registry of evidence roots,
// ~/.metasystem/stores/evidence-roots.json: machinery never removes one.
type RootEntry struct {
	Root         string          `json:"root"`
	Checkout     string          `json:"checkout"`
	GitRoot      string          `json:"gitRoot"`
	Installation string          `json:"installation"`
	RootCommit   string          `json:"rootCommit,omitempty"`
	FirstSeen    time.Time       `json:"firstSeen"`
	LastSeen     time.Time       `json:"lastSeen"`
	Retired      json.RawMessage `json:"retired,omitempty"`
	Disposed     json.RawMessage `json:"disposed,omitempty"`
}

// RootsRegistryPath is the host registry of evidence roots.
func RootsRegistryPath(homeStateRoot string) string {
	return filepath.Join(homeStateRoot, "stores", "evidence-roots.json")
}

// ReadRootsRegistry reads the registry; absent is empty.
func ReadRootsRegistry(path string) ([]RootEntry, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var entries []RootEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, fmt.Errorf("%s is unreadable: %w", path, err)
	}
	return entries, nil
}

// RecordRoots records each armed checkout's resolved root in the registry
// under evidence-roots.lock: a new entry gets firstSeen, an existing one a
// fresh lastSeen; nothing is removed.
func RecordRoots(path string, checkouts []HostCheckout, now time.Time, sync diskstore.Syncer) error {
	lockFile, err := lockRegistry(path)
	if err != nil {
		return err
	}
	defer func() { _ = unix.Flock(int(lockFile.Fd()), unix.LOCK_UN); _ = lockFile.Close() }()
	entries, err := ReadRootsRegistry(path)
	if err != nil {
		return err
	}
	changed := false
	for _, checkout := range checkouts {
		if checkout.SettingsErr != nil || checkout.Settings.EvidenceRoot.Path == "" {
			continue
		}
		root := checkout.Settings.EvidenceRoot.Path
		found := false
		for index := range entries {
			if entries[index].Root == root && entries[index].Installation == checkout.Installation {
				entries[index].LastSeen, found, changed = now.UTC(), true, true
			}
		}
		if !found {
			entries = append(entries, RootEntry{Root: root, Checkout: checkout.Facts.GitRoot, GitRoot: checkout.Facts.GitRoot, Installation: checkout.Installation,
				RootCommit: checkout.Facts.RootCommit, FirstSeen: now.UTC(), LastSeen: now.UTC()})
			changed = true
		}
	}
	if !changed {
		return nil
	}
	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return err
	}
	return sync.WriteDurable(path, append(data, '\n'), fmt.Sprintf("roots-%d", now.UnixNano()))
}

func lockRegistry(path string) (*os.File, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	file, err := os.OpenFile(filepath.Join(filepath.Dir(path), "evidence-roots.lock"), os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		return nil, err
	}
	if err := unix.Flock(int(file.Fd()), unix.LOCK_EX); err != nil {
		_ = file.Close()
		return nil, err
	}
	return file, nil
}

// Root is one evidence root of the host.
type Root struct {
	Path     string
	Owner    string
	Segments []Segment
	// Unsegmented are the root's top-level entries that belong to no
	// segment: reported and measured, disposed of only by a person.
	Unsegmented []string
}

// segmentDirs are the directories under a root that hold segments; the
// rest of a root's top level is its own bookkeeping or unsegmented.
var segmentDirs = []string{"agents", "suite-failures", "events"}

var rootBookkeeping = map[string]bool{"agents": true, "suite-failures": true, "events": true, "segments": true, "disposals": true,
	"RETIRED.json": true, ".DS_Store": true}

// DiscoverRoots lists the host's evidence roots: every armed checkout's
// resolved root, every registry entry's root, and every directory under
// $HOME/metasystem-evidence but the blob store; each with its segments,
// its owner and its unsegmented entries. It reads only.
func DiscoverRoots(userHome string, checkouts []HostCheckout, registry []RootEntry) []Root {
	paths := map[string]bool{}
	for _, checkout := range checkouts {
		if checkout.SettingsErr == nil && checkout.Settings.EvidenceRoot.Path != "" {
			paths[checkout.Settings.EvidenceRoot.Path] = true
		}
	}
	for _, entry := range registry {
		paths[entry.Root] = true
	}
	if entries, err := os.ReadDir(diskstore.EvidenceParent(userHome)); err == nil {
		for _, entry := range entries {
			if entry.IsDir() && entry.Name() != diskstore.BlobStoreName {
				paths[filepath.Join(diskstore.EvidenceParent(userHome), entry.Name())] = true
			}
		}
	}
	var roots []Root
	for path := range paths {
		if info, err := os.Stat(path); err != nil || !info.IsDir() {
			continue
		}
		roots = append(roots, discoverRoot(path, checkouts))
	}
	sort.Slice(roots, func(i, j int) bool { return roots[i].Path < roots[j].Path })
	return roots
}

func discoverRoot(path string, checkouts []HostCheckout) Root {
	root := Root{Path: path}
	var owners []string
	claimedGit, claimedInstallation := map[string]bool{}, map[string]bool{}
	for index := range checkouts {
		checkout := checkouts[index]
		if checkout.SettingsErr != nil || checkout.Settings.EvidenceRoot.Path != path {
			continue
		}
		owners = append(owners, checkout.Facts.GitRoot)
		segment := Segment{Root: path, Git: diskstore.Segment(checkout.Facts.GitRoot), Installation: diskstore.Segment(checkout.Installation),
			Context: &Context{Installation: checkout.Installation, Facts: checkout.Facts, Settings: checkout.Settings}}
		claimedGit[segment.Git], claimedInstallation[segment.Installation] = true, true
		switch recorded, err := ReadSegmentIndex(path, segment.Git); {
		case checkout.FactsErr != nil:
			segment.Unknown = "the checkout's facts cannot be read: " + checkout.FactsErr.Error()
		case errors.Is(err, os.ErrNotExist):
			// The index is recorded by the pass's registry step.
			segment.LedgerIdentity = checkout.Facts.LedgerIdentity
		case err != nil:
			segment.Unknown = err.Error()
		default:
			segment.LedgerIdentity = recorded.LedgerIdentity
			if reason := recorded.Revalidate(checkout.Facts); reason != "" {
				segment.Unknown = reason + "; a person decides: metasystem evidence show --all, then metasystem evidence dispose PATH --export DIR --preview"
			}
		}
		root.Segments = append(root.Segments, segment)
	}
	for _, directory := range segmentDirs {
		entries, _ := os.ReadDir(filepath.Join(path, directory))
		for _, entry := range entries {
			name := entry.Name()
			if !entry.IsDir() || claimedGit[name] || claimedInstallation[name] {
				continue
			}
			if directory == "events" {
				claimedInstallation[name] = true
				root.Segments = append(root.Segments, Segment{Root: path, Installation: name,
					Unknown: "orphan segment events/" + name + ": no armed checkout's installation hashes to it; arm its checkout (metasystem system setup there), or a person disposes of it: metasystem evidence dispose PATH --export DIR --preview"})
				continue
			}
			claimedGit[name] = true
			root.Segments = append(root.Segments, Segment{Root: path, Git: name,
				Unknown: "orphan segment " + name + ": no armed checkout's git root hashes to it; arm its checkout (metasystem system setup there), or a person disposes of it: metasystem evidence dispose PATH --export DIR --preview"})
		}
	}
	if entries, err := os.ReadDir(path); err == nil {
		for _, entry := range entries {
			if !rootBookkeeping[entry.Name()] {
				root.Unsegmented = append(root.Unsegmented, filepath.Join(path, entry.Name()))
			}
		}
	}
	switch {
	case len(owners) > 0:
		root.Owner = "root of " + strings.Join(owners, ", ")
	default:
		if retired, err := os.ReadFile(filepath.Join(path, "RETIRED.json")); err == nil {
			var pointer struct {
				Checkouts []string `json:"checkouts"`
				Successor string   `json:"successor"`
			}
			_ = json.Unmarshal(retired, &pointer)
			root.Owner = "retired root of " + strings.Join(pointer.Checkouts, ", ") + ", successor " + pointer.Successor
		} else {
			root.Owner = "unclaimed root: no armed checkout resolves to it; claim it by arming its checkout (metasystem system setup there), or a person disposes of its entries: metasystem evidence dispose PATH --export DIR --preview"
		}
		for index := range root.Segments {
			if root.Segments[index].Unknown == "" || strings.HasPrefix(root.Segments[index].Unknown, "orphan segment") {
				root.Segments[index].Unknown = root.Owner + "; its segments are Unknown to the bound until a context checkout is armed"
			}
		}
	}
	return root
}

// HostCap is the machine cap's position after the pass.
type HostCap struct {
	TotalBytes int64
	CapBytes   int64
	Compacted  int
	Pending    []string
}

// CompactHost is the machine cap (3.12): while the physical bytes of every
// root plus the blob store exceed evidence.machine-cap-gib, the oldest
// eligible item on the host (across every segment whose context is armed
// and observed) is compacted, one item per bound-lock acquisition, with a
// fresh host walk before each cap test. Nothing is removed.
func (b Bound) CompactHost(ctx context.Context, roots []Root, settings map[string]PassSettings, capBytes int64,
	positions map[string]*diskstore.EvidenceSegment) HostCap {
	host := HostCap{CapBytes: capBytes}
	measure := func(ctx context.Context) (int64, int64, bool) {
		var total int64
		complete := true
		for _, root := range roots {
			bytes, _, done := diskstore.Measure(ctx, root.Path)
			total += bytes
			complete = complete && done
		}
		if b.Blobs.Dir != "" {
			bytes, _, done := diskstore.Measure(ctx, b.Blobs.Dir)
			total += bytes
			complete = complete && done
		}
		return total, 0, complete && ctx.Err() == nil
	}
	type candidate struct {
		segment Segment
		item    Item
	}
	var candidates []candidate
	for _, root := range roots {
		for _, segment := range root.Segments {
			if segment.Unknown != "" || segment.Context == nil {
				continue
			}
			items, err := segment.Items(ctx)
			if err != nil {
				continue
			}
			for _, item := range items {
				if ok, _ := b.Candidate(segment, item, settings[segment.Git].AgeFloor); ok {
					candidates = append(candidates, candidate{segment: segment, item: item})
				}
			}
		}
	}
	sort.SliceStable(candidates, func(i, j int) bool { return candidates[i].item.EndedAt.Before(candidates[j].item.EndedAt) })
	test := capTest{measure: measure, capBytes: capBytes, rule: diskstore.RuleMachineCap, where: "the host's evidence roots"}
	for _, next := range candidates {
		if ctx.Err() != nil {
			host.Pending = append(host.Pending, "the pass budget ran out in the machine cap")
			break
		}
		position := positions[next.segment.Git]
		if position == nil {
			position = &diskstore.EvidenceSegment{Segment: next.segment.Git, Root: next.segment.Root, Held: map[string]int64{}}
			positions[next.segment.Git] = position
		}
		before := position.Compacted
		outcome := b.compactOne(ctx, next.segment, settings[next.segment.Git], next.item, position, test)
		host.Compacted += position.Compacted - before
		if outcome.stop {
			break
		}
	}
	host.TotalBytes, _, _ = measure(ctx)
	return host
}

// Lines renders the machine cap's position.
func (h HostCap) Lines() []string {
	var lines []string
	if h.Compacted > 0 {
		lines = append(lines, fmt.Sprintf("machine cap: compacted %d item(s) across the host", h.Compacted))
	}
	if h.TotalBytes > h.CapBytes {
		lines = append(lines, fmt.Sprintf("machine cap: the host's evidence holds %s, over %s = %s after compaction; %s and %s name what a person may remove",
			formatGiB(h.TotalBytes), config.DiskEvidenceMachineCapKey, formatGiB(h.CapBytes), "metasystem disk show", "metasystem evidence show --all"))
	}
	return append(lines, h.Pending...)
}
