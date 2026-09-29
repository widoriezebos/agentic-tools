package diskstore

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// The move protocol (3.5): a distilled bundle older than
// disk.suite-failure-move-days moves into its segment of the evidence root,
// <the evidence root>/suite-failures/<segment of the git root>/<name>. The copy
// goes to <destination>.partial-<stage>, every file is verified by digest
// against the source's listing, the copy is renamed into place and its
// directory synced, the bundle's blob references are rewritten to name the
// moved recipe, and only then is the source removed. EXDEV is the normal
// case: the move never renames across. An equal destination removes the
// source; a differing one keeps it for a person; an interrupted move leaves
// a partial the next move of the same bundle discards and redoes.

// MoveResult says what one move did.
type MoveResult struct {
	Destination string   `json:"destination"`
	Moved       bool     `json:"moved,omitempty"`
	Equal       bool     `json:"equal,omitempty"`
	Kept        string   `json:"kept,omitempty"`
	Discarded   []string `json:"discarded,omitempty"`
}

// MoveRules are one move's inputs.
type MoveRules struct {
	// SegmentDir is <the evidence root>/suite-failures/<segment>.
	SegmentDir string
	Blobs      BlobStore
	Referrer   string
	// Installation and Segment are carried into the rewritten references.
	Installation, Segment string
	Stage                 string
	Sync                  Syncer
	// interrupt, in tests, stops the move after the named step.
	interrupt func(step string) bool
}

// MoveBundle moves one distilled bundle into its segment.
func MoveBundle(ctx context.Context, source string, rules MoveRules) (MoveResult, error) {
	var result MoveResult
	if err := checkBundleDir(source); err != nil {
		return result, err
	}
	if !filepath.IsAbs(rules.SegmentDir) {
		return result, fmt.Errorf("the segment directory must be absolute, got %q", rules.SegmentDir)
	}
	name := filepath.Base(source)
	destination := filepath.Join(rules.SegmentDir, name)
	result.Destination = destination
	if err := os.MkdirAll(rules.SegmentDir, 0o755); err != nil {
		return result, err
	}
	// An earlier interrupted move of this bundle left a partial: discard it.
	entries, err := os.ReadDir(rules.SegmentDir)
	if err != nil {
		return result, err
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), name+PartialSuffix) {
			if err := RemoveTree(ctx, filepath.Join(rules.SegmentDir, entry.Name())); err != nil {
				return result, err
			}
			result.Discarded = append(result.Discarded, entry.Name())
		}
	}
	want, _, err := treeDigest(ctx, source)
	if err != nil {
		return result, err
	}
	if _, err := os.Lstat(destination); err == nil {
		got, _, err := treeDigest(ctx, destination)
		if err != nil {
			return result, err
		}
		if got != want {
			result.Kept = fmt.Sprintf("%s already holds different content; both are kept for a person", destination)
			return result, nil
		}
		result.Equal = true
	} else if !errors.Is(err, os.ErrNotExist) {
		return result, err
	} else {
		partial := destination + PartialSuffix + rules.Stage
		if err := CopyTree(ctx, source, partial); err != nil {
			return result, err
		}
		if rules.stop("copied") {
			return result, errInterrupted
		}
		got, _, err := treeDigest(ctx, partial)
		if err != nil {
			return result, err
		}
		if got != want {
			_ = RemoveTree(ctx, partial)
			return result, fmt.Errorf("the copy of %s does not verify (%s, not %s); the source is kept", source, got, want)
		}
		if err := syncTree(partial, rules.Sync); err != nil {
			return result, err
		}
		if err := os.Rename(partial, destination); err != nil {
			return result, err
		}
		if err := rules.Sync.SyncDir(rules.SegmentDir); err != nil {
			return result, err
		}
		result.Moved = true
	}
	if rules.stop("renamed") {
		return result, errInterrupted
	}
	if err := rules.rewriteRefs(destination); err != nil {
		return result, err
	}
	// The source is renamed aside before its removal, so a removal cut
	// short leaves a name that says "moved" rather than a bundle that
	// no longer matches its copy.
	moved := MovedSourceName(source, rules.Stage)
	if err := os.Rename(source, moved); err != nil {
		return result, err
	}
	if err := rules.Sync.SyncDir(filepath.Dir(source)); err != nil {
		return result, err
	}
	if rules.stop("set-aside") {
		return result, errInterrupted
	}
	// The source is re-checked just before its removal (Round B2-4, F-3):
	// one written to after its copy verified is renamed back and kept
	// beside the copy for a person.
	if now, _, err := treeDigest(ctx, moved); err != nil || now != want {
		if renameErr := os.Rename(moved, source); renameErr != nil {
			return result, fmt.Errorf("the source %s changed after its copy verified and could not be renamed back from %s: %w", source, moved, renameErr)
		}
		result.Kept = fmt.Sprintf("%s changed after its copy verified; the source and its copy %s are both kept for a person", source, destination)
		return result, rules.Sync.SyncDir(filepath.Dir(source))
	}
	return result, RemoveTree(ctx, moved)
}

// movedMark is in the name a moved bundle's source carries while it is
// removed.
const movedMark = PartialSuffix + "moved-"

// MovedSourceName is the name a moved source is renamed to before removal.
func MovedSourceName(source, stage string) string { return source + movedMark + stage }

// MovedSource reports whether an entry of a suite-failures directory is a
// moved bundle's source set aside for removal, and the bundle's name.
func MovedSource(name string) (bundle string, ok bool) {
	index := strings.Index(name, movedMark)
	if index <= 0 {
		return "", false
	}
	return name[:index], true
}

func (rules MoveRules) stop(step string) bool { return rules.interrupt != nil && rules.interrupt(step) }

// rewriteRefs points every blob reference of the moved bundle at its moved
// recipe, under the blob store's shared lock.
func (rules MoveRules) rewriteRefs(destination string) error {
	_, lines, present, err := ReadDistilled(destination)
	if err != nil || !present {
		return err
	}
	var digests []string
	for _, line := range lines {
		if line.Kind == RecipeBlob {
			digests = append(digests, line.SHA256)
		}
	}
	if len(digests) == 0 {
		return nil
	}
	release, err := rules.Blobs.TryShared()
	if err != nil {
		return err
	}
	defer release()
	for _, digest := range digests {
		ref := BlobRef{Referrer: rules.Referrer, Recipe: filepath.Join(destination, DistilledName), Installation: rules.Installation, Segment: rules.Segment}
		if err := rules.Blobs.WriteRef(digest, ref, rules.Stage); err != nil {
			return err
		}
	}
	return nil
}

// syncTree syncs every directory of a copied tree (CopyTree synced its
// files), deepest first.
func syncTree(root string, sync Syncer) error {
	var directories []string
	err := filepath.WalkDir(root, func(full string, entry os.DirEntry, err error) error {
		if err == nil && entry.IsDir() {
			directories = append(directories, full)
		}
		return err
	})
	if err != nil {
		return err
	}
	for index := len(directories) - 1; index >= 0; index-- {
		if err := sync.SyncDir(directories[index]); err != nil {
			return err
		}
	}
	return nil
}
