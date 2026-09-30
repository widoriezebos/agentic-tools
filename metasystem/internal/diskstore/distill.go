package diskstore

import (
	"archive/tar"
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// The distiller (3.5; U5c): it never deletes unique bytes. Per file it runs
// one replacement transaction (DL3B-10): (1) stage the replacement as
// <name>.partial-<stage> (a gzip stream, a tar.gz of a nested .git
// directory, or a copy into the host blob store), synced; (2) verify the
// stage reproduces the original's digest; (3) for a blob, publish the
// reference, then publish the recipe line in DISTILLED.txt, both durable;
// (4) rename the stage to its final name; (5) unlink the original; (6) sync
// the directory. The path-to-bytes mapping is on disk before any original
// goes. A restart reads DISTILLED.txt first and finishes or discards each
// interrupted transaction by the rules of 3.5.

// DistilledName is a bundle's recipe manifest.
const DistilledName = "DISTILLED.txt"

// DistilledSchema names the manifest's format.
const DistilledSchema = "metasystem.distilled/1"

// Recipe kinds.
const (
	RecipeGzip      = "gzip"
	RecipeGit       = "git"
	RecipeBlob      = "blob"
	RecipeCollision = "collision"
	// RecipeStage is an intent line: a stage the distiller is about to
	// write, listed before it exists, so a restart removes only stages the
	// manifest lists as its own (Round B2, F-5).
	RecipeStage = "stage"
)

// Restores reports a line that maps an original to a replacement.
func (l RecipeLine) Restores() bool { return l.Kind != RecipeCollision && l.Kind != RecipeStage }

// DistilledHeader is the manifest's first line.
type DistilledHeader struct {
	Schema string `json:"schema"`
	// Created is the bundle's original creation stamp, so ageing never
	// reads the mtimes compression wrote (DL-21, DL-22).
	Created time.Time `json:"created"`
}

// RecipeLine is one manifest line: how the original at Path is restored.
type RecipeLine struct {
	Kind string `json:"kind"`
	// Path is the original's slash-separated path inside the bundle.
	Path   string `json:"path"`
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256"`
	// Replacement is the replacement's path inside the bundle (gzip, git)
	// or the blob's digest (blob).
	Replacement string `json:"replacement,omitempty"`
	// Stage is the in-flight name while the transaction is open.
	Stage string    `json:"stage,omitempty"`
	Mtime time.Time `json:"mtime"`
	Note  string    `json:"note,omitempty"`
}

// DistillRules are one distillation's inputs.
type DistillRules struct {
	// CompressAbove is disk.compress-above-mib in bytes: files at or above
	// it are gzipped (or go to the blob store when they are executables).
	CompressAbove int64
	Blobs         BlobStore
	// Referrer names this bundle in a blob reference:
	// <segment of the git root>-<bundle name>.
	Referrer     string
	Installation string
	Segment      string
	// Stage is the pass's id, in every .partial name this call writes.
	Stage string
	// Known reports which of the given files are byte-identical to an
	// object of the checkout's repository (the 09-24 distiller's rule, now
	// only a hint: such a file's bytes are copied into the blob store,
	// which the recipe depends on, never the repository). nil knows none;
	// an error knows none.
	Known func(ctx context.Context, paths []string) (map[string]bool, error)
	// Owner supplies OWNER.json for a legacy bundle without one: from the
	// attempt record while it exists, else goal unknown. nil writes none.
	Owner func(bundle string) (BundleOwner, error)
	Sync  Syncer
	// interrupt, in tests, stops the transaction after the named step.
	interrupt func(step, path string) bool
}

// DistillResult says what one call did.
type DistillResult struct {
	Replaced  []RecipeLine `json:"replaced,omitempty"`
	Finished  []RecipeLine `json:"finished,omitempty"`
	Kept      []string     `json:"kept,omitempty"`
	Discarded []string     `json:"discarded,omitempty"`
	Owner     bool         `json:"ownerWritten,omitempty"`
}

// Changed reports whether the call changed anything.
func (r DistillResult) Changed() bool {
	return len(r.Replaced)+len(r.Finished)+len(r.Discarded) > 0 || r.Owner
}

// errInterrupted is a test's simulated crash.
var errInterrupted = errors.New("interrupted")

// Distill distils one bundle directory. A second run is a no-op. now stamps
// a legacy owner file and the manifest's creation stamp when the bundle's
// name carries none.
func Distill(ctx context.Context, bundle string, rules DistillRules, now time.Time) (DistillResult, error) {
	var result DistillResult
	if err := checkBundleDir(bundle); err != nil {
		return result, err
	}
	release, err := rules.Blobs.TryShared()
	if err != nil {
		return result, err
	}
	defer release()
	if rules.Owner != nil {
		if _, err := os.Lstat(filepath.Join(bundle, OwnerFileName)); errors.Is(err, os.ErrNotExist) {
			owner, ownerErr := rules.Owner(bundle)
			if ownerErr != nil {
				return result, ownerErr
			}
			owner.WrittenBy, owner.WrittenAt = "distiller", now.UTC()
			if result.Owner, err = WriteBundleOwner(bundle, owner, rules.Sync); err != nil {
				return result, err
			}
		}
	}
	header, lines, _, err := ReadDistilled(bundle)
	if err != nil {
		return result, err
	}
	if err := rules.recover(ctx, bundle, header, lines, &result); err != nil {
		return result, err
	}
	header, lines, present, err := ReadDistilled(bundle)
	if err != nil {
		return result, err
	}
	candidates, err := distillCandidates(ctx, bundle, lines)
	if err != nil {
		return result, err
	}
	if len(candidates) == 0 {
		return result, nil
	}
	plans, err := rules.plan(ctx, bundle, candidates, lines, &result)
	if err != nil {
		return result, err
	}
	if len(plans) == 0 && len(result.Kept) == 0 {
		return result, nil
	}
	if !present {
		header = DistilledHeader{Schema: DistilledSchema, Created: bundleCreated(bundle, now)}
	}
	for _, line := range plans {
		if line.Kind == RecipeCollision {
			lines = append(lines, line)
			if err := writeDistilled(bundle, header, lines, rules.Sync, rules.Stage); err != nil {
				return result, err
			}
			present = true
			continue
		}
		if !present {
			// DISTILLED.txt is written before the bundle's first replacement.
			if err := writeDistilled(bundle, header, lines, rules.Sync, rules.Stage); err != nil {
				return result, err
			}
			present = true
		}
		var updated []RecipeLine
		updated, err = rules.transact(ctx, bundle, header, lines, line)
		if errors.Is(err, errInterrupted) {
			return result, err
		}
		if errors.Is(err, errOriginalChanged) {
			// The line is published and its replacement stands: the
			// manifest keeps it.
			lines = updated
			result.Kept = append(result.Kept, line.Path+": "+err.Error())
			continue
		}
		if err != nil {
			result.Kept = append(result.Kept, line.Path+": "+err.Error())
			continue
		}
		lines = updated
		result.Replaced = append(result.Replaced, line)
	}
	removeEmptyDirs(bundle, result.Replaced)
	return result, nil
}

func checkBundleDir(bundle string) error {
	if !filepath.IsAbs(bundle) || filepath.Clean(bundle) != bundle {
		return fmt.Errorf("a bundle path must be absolute and clean, got %q", bundle)
	}
	info, err := os.Lstat(bundle)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return fmt.Errorf("%s is not a bundle directory", bundle)
	}
	return nil
}

// bundleCreated is the stamp a bundle's name starts with
// (20060102T150405Z-…), else now.
func bundleCreated(bundle string, now time.Time) time.Time {
	name := filepath.Base(bundle)
	if len(name) >= 16 {
		if stamp, err := time.Parse("20060102T150405Z", name[:16]); err == nil {
			return stamp.UTC()
		}
	}
	return now.UTC()
}

// ReadDistilled reads a bundle's manifest; present is false when there is
// none.
func ReadDistilled(bundle string) (DistilledHeader, []RecipeLine, bool, error) {
	data, err := os.ReadFile(filepath.Join(bundle, DistilledName))
	if errors.Is(err, os.ErrNotExist) {
		return DistilledHeader{}, nil, false, nil
	}
	if err != nil {
		return DistilledHeader{}, nil, false, err
	}
	scanner := bufio.NewScanner(bytes.NewReader(data))
	scanner.Buffer(make([]byte, 64*1024), 16<<20)
	var header DistilledHeader
	var lines []RecipeLine
	first := true
	for scanner.Scan() {
		text := strings.TrimSpace(scanner.Text())
		if text == "" {
			continue
		}
		if first {
			first = false
			if err := json.Unmarshal([]byte(text), &header); err != nil || header.Schema != DistilledSchema {
				return header, nil, true, fmt.Errorf("%s has no %s header", filepath.Join(bundle, DistilledName), DistilledSchema)
			}
			continue
		}
		var line RecipeLine
		if err := json.Unmarshal([]byte(text), &line); err != nil {
			return header, nil, true, fmt.Errorf("%s has an unreadable line: %w", filepath.Join(bundle, DistilledName), err)
		}
		lines = append(lines, line)
	}
	if first {
		return header, nil, true, fmt.Errorf("%s is empty", filepath.Join(bundle, DistilledName))
	}
	return header, lines, true, scanner.Err()
}

func writeDistilled(bundle string, header DistilledHeader, lines []RecipeLine, sync Syncer, stage string) error {
	var buffer bytes.Buffer
	encoded, err := json.Marshal(header)
	if err != nil {
		return err
	}
	buffer.Write(append(encoded, '\n'))
	for _, line := range lines {
		encoded, err := json.Marshal(line)
		if err != nil {
			return err
		}
		buffer.Write(append(encoded, '\n'))
	}
	return sync.WriteDurable(filepath.Join(bundle, DistilledName), buffer.Bytes(), stage)
}

// candidate is one member the distiller may replace.
type candidate struct {
	rel  string // slash path inside the bundle
	git  bool   // a nested .git directory
	info os.FileInfo
}

// distillCandidates lists the members not yet distilled: regular files other
// than the owner file, the manifest, replacements and stages; and nested
// .git directories. Symlinks stay as they are.
func distillCandidates(ctx context.Context, bundle string, lines []RecipeLine) ([]candidate, error) {
	named, stages := map[string]bool{}, map[string]bool{}
	for _, line := range lines {
		if line.Stage != "" {
			stages[line.Stage] = true
		}
		if line.Kind == RecipeStage {
			continue
		}
		named[line.Path] = true
		if line.Kind != RecipeBlob && line.Replacement != "" {
			named[line.Replacement] = true
		}
	}
	var candidates []candidate
	err := filepath.WalkDir(bundle, func(full string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}
		if full == bundle {
			return nil
		}
		rel := filepath.ToSlash(strings.TrimPrefix(full, bundle+string(filepath.Separator)))
		name := entry.Name()
		if stages[rel] || ownManifestStage(rel) {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.IsDir() {
			if name == ".git" {
				if !named[rel] {
					info, err := entry.Info()
					if err != nil {
						return err
					}
					candidates = append(candidates, candidate{rel: rel, git: true, info: info})
				}
				return filepath.SkipDir
			}
			return nil
		}
		if !entry.Type().IsRegular() || named[rel] || rel == OwnerFileName || rel == DistilledName {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		candidates = append(candidates, candidate{rel: rel, info: info})
		return nil
	})
	sort.Slice(candidates, func(i, j int) bool { return candidates[i].rel < candidates[j].rel })
	return candidates, err
}

// plan decides each candidate's recipe (3.5): a nested .git is packed; a
// file whose bytes the blob store already holds, that the repository knows,
// or that is an executable at or above the threshold goes to the blob
// store (identical engines across bundles cost one blob); any other file at
// or above the threshold is gzipped; smaller files stay. A replacement name
// already present as a foreign member is a collision: the file is kept.
func (rules DistillRules) plan(ctx context.Context, bundle string, candidates []candidate, lines []RecipeLine, result *DistillResult) ([]RecipeLine, error) {
	var regular []string
	for _, candidate := range candidates {
		if !candidate.git && candidate.info.Size() > 0 {
			regular = append(regular, filepath.Join(bundle, filepath.FromSlash(candidate.rel)))
		}
	}
	known := map[string]bool{}
	if rules.Known != nil && len(regular) > 0 {
		if answer, err := rules.Known(ctx, regular); err == nil {
			known = answer
		}
	}
	var plans []RecipeLine
	for _, candidate := range candidates {
		if err := ctx.Err(); err != nil {
			return plans, err
		}
		full := filepath.Join(bundle, filepath.FromSlash(candidate.rel))
		line := RecipeLine{Path: candidate.rel, Mtime: candidate.info.ModTime().UTC()}
		if candidate.git {
			digest, size, err := treeDigest(ctx, full)
			if err != nil {
				return plans, err
			}
			line.Kind, line.SHA256, line.Size, line.Replacement = RecipeGit, digest, size, candidate.rel+".tar.gz"
		} else {
			size := candidate.info.Size()
			if size == 0 {
				continue
			}
			digest, _, err := FileDigest(ctx, full)
			if err != nil {
				return plans, err
			}
			line.SHA256, line.Size = digest, size
			held, verifyErr := rules.Blobs.Verify(ctx, digest)
			switch {
			case verifyErr != nil:
				line.Kind, line.Note = RecipeCollision, "the blob store's "+digest+" differs from its name; the file is kept: "+verifyErr.Error()
			case held || known[full] || size >= rules.CompressAbove && executable(full, candidate.info):
				line.Kind, line.Replacement = RecipeBlob, digest
			case size >= rules.CompressAbove:
				line.Kind, line.Replacement = RecipeGzip, candidate.rel+".gz"
			default:
				continue
			}
		}
		if line.Kind == RecipeGzip || line.Kind == RecipeGit {
			if _, err := os.Lstat(filepath.Join(bundle, filepath.FromSlash(line.Replacement))); err == nil {
				line.Note = line.Replacement + " already exists in the bundle and is not a replacement; both are kept"
				line.Kind, line.Replacement = RecipeCollision, ""
			}
		}
		if line.Kind == RecipeCollision {
			if collisionRecorded(lines, line) {
				continue
			}
			result.Kept = append(result.Kept, line.Path+": "+line.Note)
		}
		plans = append(plans, line)
	}
	return plans, nil
}

func collisionRecorded(lines []RecipeLine, line RecipeLine) bool {
	for _, existing := range lines {
		if existing.Kind == RecipeCollision && existing.Path == line.Path && existing.SHA256 == line.SHA256 {
			return true
		}
	}
	return false
}

// executable reports a file with an executable bit or a Mach-O or ELF
// header: a routing choice (executables are the bundles' repeated bytes),
// never a judgement that the bytes can be rebuilt (DL4C-07).
func executable(path string, info os.FileInfo) bool {
	if info.Mode().Perm()&0o111 != 0 {
		return true
	}
	file, err := os.Open(path)
	if err != nil {
		return false
	}
	defer file.Close()
	var magic [4]byte
	if _, err := io.ReadFull(file, magic[:]); err != nil {
		return false
	}
	switch string(magic[:]) {
	case "\x7fELF", "\xfe\xed\xfa\xce", "\xfe\xed\xfa\xcf", "\xce\xfa\xed\xfe", "\xcf\xfa\xed\xfe", "\xca\xfe\xba\xbe":
		return true
	}
	return false
}

// transact runs one replacement transaction and returns the manifest's lines
// with the new line published.
func (rules DistillRules) transact(ctx context.Context, bundle string, header DistilledHeader, lines []RecipeLine, line RecipeLine) ([]RecipeLine, error) {
	original := filepath.Join(bundle, filepath.FromSlash(line.Path))
	var stage string
	var err error
	switch line.Kind {
	case RecipeBlob:
		if held, _ := rules.Blobs.Verify(ctx, line.SHA256); !held {
			stage, err = rules.Blobs.stage(ctx, original, line.SHA256, rules.Stage)
			if err != nil {
				return nil, err
			}
		}
	case RecipeGzip, RecipeGit:
		stage = filepath.Join(bundle, filepath.FromSlash(line.Replacement)) + PartialSuffix + rules.Stage
		// The stage is listed before it is written: only a listed stage is
		// ever removed by a restart.
		intent := RecipeLine{Kind: RecipeStage, Path: line.Path, Stage: relOrBlob(bundle, stage, rules.Blobs), Mtime: line.Mtime}
		if err := writeDistilled(bundle, header, append(append([]RecipeLine(nil), lines...), intent), rules.Sync, rules.Stage); err != nil {
			return nil, err
		}
		if line.Kind == RecipeGzip {
			err = gzipStage(ctx, original, stage, line.SHA256)
		} else {
			err = tarStage(ctx, original, stage, line.SHA256)
		}
	default:
		return nil, fmt.Errorf("no transaction for recipe kind %s", line.Kind)
	}
	if err != nil {
		if stage != "" {
			_ = os.Remove(stage)
		}
		return nil, err
	}
	if rules.stop("staged", line.Path) {
		return nil, errInterrupted
	}
	if line.Kind == RecipeBlob {
		ref := BlobRef{Referrer: rules.Referrer, Recipe: filepath.Join(bundle, DistilledName), Installation: rules.Installation, Segment: rules.Segment}
		if err := rules.Blobs.WriteRef(line.SHA256, ref, rules.Stage); err != nil {
			return nil, err
		}
		if rules.stop("referenced", line.Path) {
			return nil, errInterrupted
		}
	}
	if stage != "" {
		line.Stage = relOrBlob(bundle, stage, rules.Blobs)
	}
	updated := append(append([]RecipeLine(nil), lines...), line)
	if err := writeDistilled(bundle, header, updated, rules.Sync, rules.Stage); err != nil {
		return nil, err
	}
	if rules.stop("published", line.Path) {
		return nil, errInterrupted
	}
	if err := rules.finish(ctx, bundle, line, stage); err != nil {
		if errors.Is(err, errOriginalChanged) {
			return updated, err
		}
		return nil, err
	}
	return updated, nil
}

// relOrBlob names a stage in a manifest line: a path inside the bundle, or
// the blob stage's file name in the store.
func relOrBlob(bundle, stage string, blobs BlobStore) string {
	if filepath.Dir(stage) == blobs.Dir {
		return filepath.Base(stage)
	}
	return filepath.ToSlash(strings.TrimPrefix(stage, bundle+string(filepath.Separator)))
}

// finish is steps (4) to (6): the stage renamed to its final name (the
// original's mtime carried), the original unlinked, the directory synced.
func (rules DistillRules) finish(ctx context.Context, bundle string, line RecipeLine, stage string) error {
	original := filepath.Join(bundle, filepath.FromSlash(line.Path))
	if stage != "" {
		switch line.Kind {
		case RecipeBlob:
			if err := rules.Blobs.publish(ctx, line.SHA256, stage); err != nil {
				return err
			}
		default:
			final := filepath.Join(bundle, filepath.FromSlash(line.Replacement))
			_ = os.Chtimes(stage, line.Mtime, line.Mtime)
			if err := os.Rename(stage, final); err != nil {
				return err
			}
		}
	}
	if rules.stop("renamed", line.Path) {
		return errInterrupted
	}
	// The original is re-checked just before its unlink (Round B2-4, F-3):
	// one written to after its stage verified is kept beside its
	// replacement and reported.
	if matches, err := originalMatches(ctx, original, line); err != nil || !matches {
		return errOriginalChanged
	}
	if line.Kind == RecipeGit {
		if err := RemoveTree(ctx, original); err != nil {
			return err
		}
	} else if err := os.Remove(original); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return rules.Sync.SyncDir(filepath.Dir(original))
}

// errOriginalChanged is an original that no longer matches its published
// line when it is about to be unlinked: both are kept and reported.
var errOriginalChanged = errors.New("it changed after its replacement was verified; both are kept for a person to decide")

func (rules DistillRules) stop(step, path string) bool {
	return rules.interrupt != nil && rules.interrupt(step, path)
}

// recover applies the restart rules of 3.5 to every line whose transaction
// may be open, and removes the bundle's stages no line names.
func (rules DistillRules) recover(ctx context.Context, bundle string, header DistilledHeader, lines []RecipeLine, result *DistillResult) error {
	restoring := map[string]bool{}
	for _, line := range lines {
		if line.Restores() {
			restoring[line.Path] = true
		}
	}
	// A listed stage whose transaction never published its line is this
	// distiller's incomplete work: removed, and its intent line dropped.
	var kept []RecipeLine
	dropped := false
	for _, line := range lines {
		if line.Kind != RecipeStage {
			kept = append(kept, line)
			continue
		}
		if !restoring[line.Path] && line.Stage != "" {
			full := filepath.Join(bundle, filepath.FromSlash(line.Stage))
			if fileExists(full) {
				if err := RemoveTree(ctx, full); err != nil {
					return err
				}
				result.Discarded = append(result.Discarded, line.Stage)
			}
		}
		dropped = true
	}
	if dropped {
		if err := writeDistilled(bundle, header, kept, rules.Sync, rules.Stage); err != nil {
			return err
		}
		lines = kept
	}
	for _, line := range lines {
		if !line.Restores() {
			continue
		}
		original := filepath.Join(bundle, filepath.FromSlash(line.Path))
		if _, err := os.Lstat(original); errors.Is(err, os.ErrNotExist) {
			continue // the original is gone: complete
		}
		// The original is unlinked only while it still is what the line
		// recorded (Round B2-3, N3-6): one rewritten after the line was
		// published is kept beside its replacement and reported.
		if matches, err := originalMatches(ctx, original, line); err != nil || !matches {
			reason := "it no longer matches its published line"
			if err != nil {
				reason = "it cannot be read against its published line: " + err.Error()
			}
			result.Kept = append(result.Kept, line.Path+": "+reason+"; the original and its replacement are both kept for a person")
			continue
		}
		stage, final := rules.linePaths(bundle, line)
		finalOK, err := rules.verifyReplacement(ctx, line, final)
		if err != nil {
			result.Kept = append(result.Kept, line.Path+": its replacement does not reproduce it ("+err.Error()+"); both are kept")
			continue
		}
		if !finalOK {
			switch {
			case stage != "" && fileExists(stage):
				if err := rules.verifyStage(ctx, line, stage); err != nil {
					_ = os.Remove(stage)
					stage = ""
				}
			default:
				stage = ""
			}
			if stage == "" {
				// Nothing verified to publish: stage the original afresh.
				var redoErr error
				stage, redoErr = rules.restage(ctx, bundle, line)
				if redoErr != nil {
					result.Kept = append(result.Kept, line.Path+": "+redoErr.Error())
					continue
				}
			}
		} else {
			if stage != "" {
				_ = os.Remove(stage)
			}
			stage = ""
		}
		if line.Kind == RecipeBlob {
			ref := BlobRef{Referrer: rules.Referrer, Recipe: filepath.Join(bundle, DistilledName), Installation: rules.Installation, Segment: rules.Segment}
			if err := rules.Blobs.WriteRef(line.SHA256, ref, rules.Stage); err != nil {
				return err
			}
		}
		if err := rules.finish(ctx, bundle, line, stage); err != nil {
			return err
		}
		result.Finished = append(result.Finished, line)
	}
	// The manifest's and owner file's own stages at the bundle's top
	// level are this engine's; captured content lives in source-* members
	// and is never one of them.
	top, _ := os.ReadDir(bundle)
	for _, entry := range top {
		if ownManifestStage(entry.Name()) {
			if err := os.Remove(filepath.Join(bundle, entry.Name())); err == nil {
				result.Discarded = append(result.Discarded, entry.Name())
			}
		}
	}
	return nil
}

// originalMatches reports whether an original still is what its line
// recorded: a file by its sha256, a git directory by its tree digest.
func originalMatches(ctx context.Context, original string, line RecipeLine) (bool, error) {
	var digest string
	var err error
	if line.Kind == RecipeGit {
		digest, _, err = treeDigest(ctx, original)
	} else {
		digest, _, err = FileDigest(ctx, original)
	}
	if err != nil {
		return false, err
	}
	return digest == line.SHA256, nil
}

// ownManifestStage is a top-level stage of DISTILLED.txt or OWNER.json.
func ownManifestStage(rel string) bool {
	return strings.HasPrefix(rel, DistilledName+PartialSuffix) || strings.HasPrefix(rel, OwnerFileName+PartialSuffix)
}

// linePaths are a line's stage and final replacement paths.
func (rules DistillRules) linePaths(bundle string, line RecipeLine) (stage, final string) {
	if line.Kind == RecipeBlob {
		final = rules.Blobs.Path(line.SHA256)
		if line.Stage != "" {
			stage = filepath.Join(rules.Blobs.Dir, line.Stage)
		}
		return stage, final
	}
	final = filepath.Join(bundle, filepath.FromSlash(line.Replacement))
	if line.Stage != "" {
		stage = filepath.Join(bundle, filepath.FromSlash(line.Stage))
	}
	return stage, final
}

func (rules DistillRules) verifyReplacement(ctx context.Context, line RecipeLine, final string) (bool, error) {
	if line.Kind == RecipeBlob {
		return rules.Blobs.Verify(ctx, line.SHA256)
	}
	if !fileExists(final) {
		return false, nil
	}
	return true, rules.verifyStage(ctx, line, final)
}

func (rules DistillRules) verifyStage(ctx context.Context, line RecipeLine, path string) error {
	var digest string
	var err error
	switch line.Kind {
	case RecipeBlob:
		digest, _, err = FileDigest(ctx, path)
	case RecipeGzip:
		digest, err = gzipDigest(ctx, path)
	case RecipeGit:
		digest, err = tarDigest(ctx, path)
	}
	if err != nil {
		return err
	}
	if digest != line.SHA256 {
		return fmt.Errorf("reproduces %s, not %s", digest, line.SHA256)
	}
	return nil
}

func (rules DistillRules) restage(ctx context.Context, bundle string, line RecipeLine) (string, error) {
	original := filepath.Join(bundle, filepath.FromSlash(line.Path))
	switch line.Kind {
	case RecipeBlob:
		return rules.Blobs.stage(ctx, original, line.SHA256, rules.Stage)
	case RecipeGzip:
		stage := filepath.Join(bundle, filepath.FromSlash(line.Replacement)) + PartialSuffix + rules.Stage
		return stage, gzipStage(ctx, original, stage, line.SHA256)
	case RecipeGit:
		stage := filepath.Join(bundle, filepath.FromSlash(line.Replacement)) + PartialSuffix + rules.Stage
		return stage, tarStage(ctx, original, stage, line.SHA256)
	}
	return "", fmt.Errorf("no transaction for recipe kind %s", line.Kind)
}

func fileExists(path string) bool {
	_, err := os.Lstat(path)
	return err == nil
}

// gzipStage writes the gzip of source to stage, synced, and verifies it
// inflates to digest.
func gzipStage(ctx context.Context, source, stage, digest string) error {
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(stage, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	writer := gzip.NewWriter(out)
	if _, err := copyChunks(ctx, writer, in); err != nil {
		return errors.Join(err, writer.Close(), out.Close())
	}
	if err := errors.Join(writer.Close(), out.Sync(), out.Close()); err != nil {
		return err
	}
	got, err := gzipDigest(ctx, stage)
	if err != nil {
		return err
	}
	if got != digest {
		return fmt.Errorf("the staged gzip inflates to %s, not %s", got, digest)
	}
	return nil
}

func gzipDigest(ctx context.Context, path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	reader, err := gzip.NewReader(file)
	if err != nil {
		return "", err
	}
	digest, _, err := readerDigest(ctx, reader)
	return digest, errors.Join(err, reader.Close())
}

// treeDigest is a directory's content digest: the sha256 of its sorted
// listing, one line per entry (a directory, a regular file's size and
// sha256, a symlink's target), and the bytes of its regular files.
func treeDigest(ctx context.Context, root string) (string, int64, error) {
	var listing []string
	var total int64
	err := filepath.WalkDir(root, func(full string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}
		rel := "."
		if full != root {
			rel = filepath.ToSlash(strings.TrimPrefix(full, root+string(filepath.Separator)))
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		switch {
		case info.Mode()&os.ModeSymlink != 0:
			target, err := os.Readlink(full)
			if err != nil {
				return err
			}
			listing = append(listing, "l "+rel+" "+target)
		case info.IsDir():
			listing = append(listing, "d "+rel)
		case info.Mode().IsRegular():
			digest, size, err := FileDigest(ctx, full)
			if err != nil {
				return err
			}
			total += size
			listing = append(listing, fmt.Sprintf("f %s %d %s", rel, size, digest))
		default:
			return fmt.Errorf("%s is neither a file, a directory nor a symlink", full)
		}
		return nil
	})
	if err != nil {
		return "", 0, err
	}
	return listingDigest(listing), total, nil
}

func listingDigest(listing []string) string {
	sort.Strings(listing)
	hash := sha256.New()
	for _, line := range listing {
		hash.Write([]byte(line + "\n"))
	}
	return hex.EncodeToString(hash.Sum(nil))
}

// tarStage packs the directory source into a tar.gz at stage, synced, and
// verifies the archive's listing reproduces digest.
func tarStage(ctx context.Context, source, stage, digest string) error {
	out, err := os.OpenFile(stage, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	compressed := gzip.NewWriter(out)
	archive := tar.NewWriter(compressed)
	walkErr := filepath.WalkDir(source, func(full string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		rel := "."
		if full != source {
			rel = filepath.ToSlash(strings.TrimPrefix(full, source+string(filepath.Separator)))
		}
		link := ""
		if info.Mode()&os.ModeSymlink != 0 {
			if link, err = os.Readlink(full); err != nil {
				return err
			}
		}
		header, err := tar.FileInfoHeader(info, link)
		if err != nil {
			return err
		}
		header.Name = rel
		if info.IsDir() {
			header.Name = rel + "/"
		}
		header.Uname, header.Gname = "", ""
		if err := archive.WriteHeader(header); err != nil {
			return err
		}
		if info.Mode().IsRegular() {
			file, err := os.Open(full)
			if err != nil {
				return err
			}
			_, copyErr := copyChunks(ctx, archive, file)
			if err := errors.Join(copyErr, file.Close()); err != nil {
				return err
			}
		}
		return nil
	})
	if err := errors.Join(walkErr, archive.Close(), compressed.Close(), out.Sync(), out.Close()); err != nil {
		return err
	}
	got, err := tarDigest(ctx, stage)
	if err != nil {
		return err
	}
	if got != digest {
		return fmt.Errorf("the staged archive reproduces %s, not %s", got, digest)
	}
	return nil
}

// tarDigest is the listing digest of a tar.gz as treeDigest computes it over
// the unpacked directory.
func tarDigest(ctx context.Context, archivePath string) (string, error) {
	file, err := os.Open(archivePath)
	if err != nil {
		return "", err
	}
	defer file.Close()
	compressed, err := gzip.NewReader(file)
	if err != nil {
		return "", err
	}
	defer compressed.Close()
	archive := tar.NewReader(compressed)
	var listing []string
	for {
		header, err := archive.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return "", err
		}
		name := strings.TrimSuffix(header.Name, "/")
		if name != "." && (path.IsAbs(name) || strings.HasPrefix(path.Clean(name), "..")) {
			return "", fmt.Errorf("archive entry %q leaves its root", header.Name)
		}
		switch header.Typeflag {
		case tar.TypeDir:
			listing = append(listing, "d "+name)
		case tar.TypeSymlink:
			listing = append(listing, "l "+name+" "+header.Linkname)
		case tar.TypeReg:
			digest, size, err := readerDigest(ctx, archive)
			if err != nil {
				return "", err
			}
			listing = append(listing, fmt.Sprintf("f %s %d %s", name, size, digest))
		default:
			return "", fmt.Errorf("archive entry %q has an unsupported type", header.Name)
		}
	}
	return listingDigest(listing), nil
}

// removeEmptyDirs removes the directories distillation emptied: each
// replaced original's parents, deepest first, while empty. The bundle
// itself and any directory that held nothing distilled stay.
func removeEmptyDirs(bundle string, replaced []RecipeLine) {
	seen := map[string]bool{}
	var directories []string
	for _, line := range replaced {
		for directory := filepath.Dir(filepath.Join(bundle, filepath.FromSlash(line.Path))); directory != bundle && strings.HasPrefix(directory, bundle+string(filepath.Separator)); directory = filepath.Dir(directory) {
			if !seen[directory] {
				seen[directory] = true
				directories = append(directories, directory)
			}
		}
	}
	sort.Slice(directories, func(i, j int) bool { return len(directories[i]) > len(directories[j]) })
	for _, directory := range directories {
		_ = os.Remove(directory) // only an empty directory goes
	}
}
