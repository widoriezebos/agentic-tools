package evidence

// The bound in the machine pass (design engine-owns-disk-lifetimes Part B
// 3.12 "Roots", "Where it runs"; DL4D-12): the steward that wins the
// machine flock runs, over every evidence root of the host, the citation
// generation's next step, the host registry of roots and the segment
// indexes, the blob reference check and sweep, and reports each segment
// and the machine cap against their bounds (Round B2-3: it never compacts
// or removes). Each root is named with its owner; a segment no armed
// checkout resolves is an orphan, reported and Unknown to the bound.

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

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
	// NotManaged are the entries that belong to no segment: the root's
	// unknown top-level entries and anything under agents, suite-failures
	// or events whose name is not an engine segment name exactly (Round
	// B2-3, rule 3). They are never items: evidence show lists them as
	// "not managed: remove by hand if unneeded" and dispose refuses them.
	// Another evidence root nested here is never listed.
	NotManaged []string
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
		roots = append(roots, discoverRoot(path, checkouts, paths))
	}
	sort.Slice(roots, func(i, j int) bool { return roots[i].Path < roots[j].Path })
	return roots
}

func discoverRoot(path string, checkouts []HostCheckout, roots map[string]bool) Root {
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
	claimedDirs := map[string][]string{}
	for _, segment := range root.Segments {
		claimedDirs["agents"] = append(claimedDirs["agents"], filepath.Join(path, "agents", segment.Git))
		claimedDirs["suite-failures"] = append(claimedDirs["suite-failures"], filepath.Join(path, "suite-failures", segment.Git))
		claimedDirs["events"] = append(claimedDirs["events"], filepath.Join(path, "events", segment.Installation))
	}
	for _, directory := range segmentDirs {
		entries, _ := os.ReadDir(filepath.Join(path, directory))
		for _, entry := range entries {
			name := entry.Name()
			full := filepath.Join(path, directory, name)
			if notAnItem(name) || claimedGit[name] || claimedInstallation[name] || isEvidenceRoot(full, roots) {
				continue
			}
			if !entry.IsDir() || !segmentName12(name) {
				// Not an engine segment name exactly (a legacy basename, an
				// upper-case spelling): not managed, never an item. A
				// spelling that is a claimed segment's own directory on a
				// case-insensitive volume is that segment, never listed.
				if !sameAsAny(full, claimedDirs[directory]) {
					root.NotManaged = append(root.NotManaged, full)
				}
				continue
			}
			if directory == "events" {
				claimedInstallation[name] = true
				root.Segments = append(root.Segments, Segment{Root: path, Installation: name,
					Unknown: "orphan segment events/" + name + ": no armed checkout's installation hashes to it; arm its checkout (metasystem system setup there), or a person disposes of each of its items by path (metasystem evidence show --all --verbose lists them; each is held until --override, having no checkout to judge it against)"})
				continue
			}
			claimedGit[name] = true
			root.Segments = append(root.Segments, Segment{Root: path, Git: name,
				Unknown: "orphan segment " + name + ": no armed checkout's git root hashes to it; arm its checkout (metasystem system setup there), or a person disposes of each of its items by path (metasystem evidence show --all --verbose lists them; each is held until --override, having no checkout to judge it against)"})
		}
	}
	if entries, err := os.ReadDir(path); err == nil {
		for _, entry := range entries {
			full := filepath.Join(path, entry.Name())
			if !rootBookkeeping[entry.Name()] && !notAnItem(entry.Name()) && !isEvidenceRoot(full, roots) {
				root.NotManaged = append(root.NotManaged, full)
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

// isEvidenceRoot reports a directory that is itself an evidence root:
// the resolved root of an armed checkout or a registry entry (the same
// file, never a string comparison), or one carrying a root's own
// structure (Round B2-3, rule 3). Such a directory is never listed.
func isEvidenceRoot(path string, roots map[string]bool) bool {
	info, err := os.Lstat(path)
	if err != nil || !info.IsDir() {
		return false
	}
	for root := range roots {
		if other, err := os.Stat(root); err == nil && os.SameFile(info, other) {
			return true
		}
	}
	for _, marker := range []string{"agents", "suite-failures", "events", "segments", "disposals", "RETIRED.json"} {
		if pathPresent(filepath.Join(path, marker)) {
			return true
		}
	}
	return false
}

// sameAsAny reports a path that is the same file as one of paths.
func sameAsAny(path string, paths []string) bool {
	info, err := os.Lstat(path)
	if err != nil {
		return false
	}
	for _, other := range paths {
		if candidate, err := os.Lstat(other); err == nil && os.SameFile(info, candidate) {
			return true
		}
	}
	return false
}

// notAnItem is a name enumeration never yields: a disposal record, a stage,
// a dot-file (Round B2-2, R1).
func notAnItem(name string) bool {
	return diskstore.IsDisposalRecord(name) || diskstore.IsPartial(name) || strings.HasPrefix(name, ".")
}
