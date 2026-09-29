package diskstore

// A delegate's quarantine object store and the common store's alternates
// (engine-owns-disk-lifetimes Part B, 3.7, R12, R17, DL2-18, DL3B-09). A
// delegate writes its objects into <worktree gitdir>/objects-quarantine,
// which the common store borrows through objects/info/alternates. Before
// the worktree goes, every object the quarantine holds is published into
// the common store and verified by content in files that never consult
// alternates; only then is the quarantine's alternates line removed, under
// the alternates lock, and only then may the worktree be removed.

import (
	"bytes"
	"compress/zlib"
	"context"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"hash"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/lock"
)

// QuarantineName is the quarantine directory inside a worktree's gitdir.
const QuarantineName = "objects-quarantine"

// alternatesLock serializes every rewrite of one store's alternates file.
func alternatesLock(objects string) (*lock.FileLock, error) {
	if err := os.MkdirAll(filepath.Join(objects, "info"), 0o755); err != nil {
		return nil, err
	}
	return lock.File(filepath.Join(objects, "info", "alternates.lock"), 0o644, lock.Exclusive)
}

func readAlternates(objects string) ([]string, error) {
	data, err := os.ReadFile(filepath.Join(objects, "info", "alternates"))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var lines []string
	for _, line := range strings.Split(string(data), "\n") {
		if line != "" {
			lines = append(lines, line)
		}
	}
	return lines, nil
}

// writeAlternates replaces the alternates file atomically: a staged copy,
// synced, renamed over it, the directory synced.
func writeAlternates(objects string, lines []string) error {
	info := filepath.Join(objects, "info")
	stage, err := os.CreateTemp(info, ".alternates-*")
	if err != nil {
		return err
	}
	content := ""
	for _, line := range lines {
		content += line + "\n"
	}
	_, writeErr := stage.WriteString(content)
	if err := errors.Join(writeErr, stage.Sync(), stage.Close()); err != nil {
		_ = os.Remove(stage.Name())
		return err
	}
	if err := os.Rename(stage.Name(), filepath.Join(info, "alternates")); err != nil {
		_ = os.Remove(stage.Name())
		return err
	}
	return syncDirectory(info)
}

// AddAlternate appends path to objects' alternates under the alternates
// lock; a line already present is success and writes nothing.
func AddAlternate(objects, path string) error {
	held, err := alternatesLock(objects)
	if err != nil {
		return err
	}
	defer held.Release()
	lines, err := readAlternates(objects)
	if err != nil {
		return err
	}
	for _, line := range lines {
		if line == path {
			return nil
		}
	}
	return writeAlternates(objects, append(lines, path))
}

// RemoveAlternate drops path from objects' alternates under the lock; an
// absent line is success.
func RemoveAlternate(objects, path string) error {
	held, err := alternatesLock(objects)
	if err != nil {
		return err
	}
	defer held.Release()
	lines, err := readAlternates(objects)
	if err != nil {
		return err
	}
	var kept []string
	for _, line := range lines {
		if line != path {
			kept = append(kept, line)
		}
	}
	if len(kept) == len(lines) {
		return nil
	}
	return writeAlternates(objects, kept)
}

// AbsorbResult is what one absorb verified.
type AbsorbResult struct {
	Loose, Packed int
}

// objectHash is the repository's object format's hash.
func objectHash(format string) (func() hash.Hash, int, error) {
	switch format {
	case "sha1":
		return sha1.New, sha1.Size, nil
	case "sha256":
		return sha256.New, sha256.Size, nil
	}
	return nil, 0, fmt.Errorf("unknown object format %q", format)
}

// AbsorbRequest is one quarantine to absorb into the common store.
type AbsorbRequest struct {
	Git WorkspaceGit
	// GitRoot is a directory of the common repository; CommonObjects its
	// objects directory; Quarantine the delegate's quarantine store.
	GitRoot, CommonObjects, Quarantine string
	// Stage names this absorb's partial files.
	Stage string
	// skip, in tests, leaves one object unpublished, so the final
	// verification must catch it.
	skip func(id string) bool
}

// AbsorbQuarantine publishes every object of the quarantine into the common
// store and verifies it by content, never through alternates (3.7, R12,
// R17). It inventories the quarantine (every loose object file, every id of
// every pack index); publishes each loose object under a .partial name,
// synced, inflated and hashed to its name, then renamed into place (an
// existing destination verified the same way and kept when equal, a
// differing one an error and never overwritten); imports each pack by
// `git index-pack --strict` under a partial name and renames the pair into
// place; then verifies the whole inventory against the common store's own
// files: every loose object inflated and hashed again, every pack index
// read by `git verify-pack` (which reads the pack beside it) listing every
// id. Only then is the quarantine's alternates line removed, under the
// alternates lock. Any read error stops it with nothing removed.
func AbsorbQuarantine(ctx context.Context, request AbsorbRequest) (AbsorbResult, error) {
	var result AbsorbResult
	git, quarantine, common := request.Git, request.Quarantine, request.CommonObjects
	formatOut, err := git(ctx, request.GitRoot, "rev-parse", "--show-object-format")
	if err != nil {
		return result, fmt.Errorf("object format: %w", err)
	}
	newHash, size, err := objectHash(strings.TrimSpace(string(formatOut)))
	if err != nil {
		return result, err
	}
	if info, err := os.Lstat(quarantine); errors.Is(err, os.ErrNotExist) {
		return result, RemoveAlternate(common, quarantine)
	} else if err != nil {
		return result, err
	} else if !info.IsDir() {
		return result, fmt.Errorf("%s is not a directory", quarantine)
	}
	// Inventory.
	var loose []string
	entries, err := os.ReadDir(quarantine)
	if err != nil {
		return result, err
	}
	for _, entry := range entries {
		name := entry.Name()
		if len(name) != 2 || !isHex(name) || !entry.IsDir() {
			continue
		}
		objects, err := os.ReadDir(filepath.Join(quarantine, name))
		if err != nil {
			return result, err
		}
		for _, object := range objects {
			if rest := object.Name(); len(rest) == 2*size-2 && isHex(rest) {
				loose = append(loose, name+rest)
			}
		}
	}
	indexes, err := filepath.Glob(filepath.Join(quarantine, "pack", "*.idx"))
	if err != nil {
		return result, err
	}
	sort.Strings(indexes)
	packs := map[string][]string{}
	for _, index := range indexes {
		ids, err := indexIDs(index, size)
		if err != nil {
			return result, err
		}
		packs[index] = ids
	}
	skip := request.skip
	if skip == nil {
		skip = func(string) bool { return false }
	}
	// Publish.
	for _, id := range loose {
		if err := ctx.Err(); err != nil {
			return result, err
		}
		if skip(id) {
			continue
		}
		if err := absorbLoose(filepath.Join(quarantine, id[:2], id[2:]), filepath.Join(common, id[:2], id[2:]), id, request.Stage, newHash); err != nil {
			return result, err
		}
	}
	for _, index := range indexes {
		if err := ctx.Err(); err != nil {
			return result, err
		}
		if skip(filepath.Base(index)) {
			continue
		}
		if err := importPack(ctx, git, request.GitRoot, common, index, packs[index], request.Stage); err != nil {
			return result, err
		}
	}
	// Verify the whole inventory in the common store's own files.
	for _, id := range loose {
		if err := verifyLoose(filepath.Join(common, id[:2], id[2:]), id, newHash); err != nil {
			return result, fmt.Errorf("the common store does not hold %s: %w", id, err)
		}
		result.Loose++
	}
	for _, index := range indexes {
		final := filepath.Join(common, "pack", filepath.Base(index))
		if err := covered(ctx, git, request.GitRoot, final, packs[index]); err != nil {
			return result, fmt.Errorf("the common store does not hold the pack of %s: %w", filepath.Base(index), err)
		}
		result.Packed += len(packs[index])
	}
	return result, RemoveAlternate(common, quarantine)
}

func isHex(text string) bool {
	_, err := hex.DecodeString(text)
	return err == nil && strings.ToLower(text) == text
}

// verifyLoose inflates a loose object file and hashes it; it must hash to
// id.
func verifyLoose(path, id string, newHash func() hash.Hash) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	reader, err := zlib.NewReader(file)
	if err != nil {
		return fmt.Errorf("loose object %s does not inflate: %w", id, err)
	}
	digest := newHash()
	if _, err := io.Copy(digest, reader); err != nil {
		return fmt.Errorf("loose object %s does not inflate: %w", id, err)
	}
	if err := reader.Close(); err != nil {
		return fmt.Errorf("loose object %s does not inflate: %w", id, err)
	}
	if got := hex.EncodeToString(digest.Sum(nil)); got != id {
		return fmt.Errorf("loose object %s hashes to %s", id, got)
	}
	return nil
}

func absorbLoose(source, destination, id, stage string, newHash func() hash.Hash) error {
	if err := verifyLoose(source, id, newHash); err != nil {
		return fmt.Errorf("the quarantine's own copy: %w", err)
	}
	if _, err := os.Lstat(destination); err == nil {
		if err := verifyLoose(destination, id, newHash); err != nil {
			return fmt.Errorf("the common store already holds a differing %s; both are kept: %w", id, err)
		}
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		return err
	}
	partial := destination + ".partial-" + stage
	if err := copyFileSynced(source, partial); err != nil {
		_ = os.Remove(partial)
		return err
	}
	if err := verifyLoose(partial, id, newHash); err != nil {
		_ = os.Remove(partial)
		return err
	}
	if err := os.Rename(partial, destination); err != nil {
		_ = os.Remove(partial)
		return err
	}
	return syncDirectory(filepath.Dir(destination))
}

func copyFileSynced(source, destination string) error {
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o444)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(out, in)
	return errors.Join(copyErr, out.Sync(), out.Close())
}

// indexIDs reads the object ids of a version-2 pack index.
func indexIDs(path string, size int) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	header := 8 + 256*4
	if len(data) < header || !bytes.Equal(data[:4], []byte{0xff, 't', 'O', 'c'}) || binary.BigEndian.Uint32(data[4:8]) != 2 {
		return nil, fmt.Errorf("%s is not a version-2 pack index", filepath.Base(path))
	}
	count := int(binary.BigEndian.Uint32(data[8+255*4 : header]))
	if len(data) < header+count*size {
		return nil, fmt.Errorf("%s is truncated", filepath.Base(path))
	}
	ids := make([]string, count)
	for index := range count {
		start := header + index*size
		ids[index] = hex.EncodeToString(data[start : start+size])
	}
	return ids, nil
}

// importPack imports one quarantine pack into the common store under
// partial names and renames the pair into place; a published pair that
// already verifies is kept. An index without its pack, a truncated pack or
// a leftover partial is never evidence: the pair is imported afresh.
func importPack(ctx context.Context, git WorkspaceGit, gitRoot, commonObjects, index string, ids []string, stage string) error {
	pack := strings.TrimSuffix(index, ".idx") + ".pack"
	if _, err := os.Lstat(pack); err != nil {
		return fmt.Errorf("the quarantine's index %s has no pack: %w", filepath.Base(index), err)
	}
	base := filepath.Base(strings.TrimSuffix(index, ".idx"))
	packDir := filepath.Join(commonObjects, "pack")
	if err := os.MkdirAll(packDir, 0o755); err != nil {
		return err
	}
	finalPack, finalIndex := filepath.Join(packDir, base+".pack"), filepath.Join(packDir, base+".idx")
	if covered(ctx, git, gitRoot, finalIndex, ids) == nil {
		return nil
	}
	stagedPack := filepath.Join(packDir, "tmp_"+base+".partial-"+stage+".pack")
	stagedIndex := strings.TrimSuffix(stagedPack, ".pack") + ".idx"
	cleanup := func() { _ = os.Remove(stagedPack); _ = os.Remove(stagedIndex) }
	if err := copyFileSynced(pack, stagedPack); err != nil {
		cleanup()
		return err
	}
	if _, err := git(ctx, gitRoot, "index-pack", "--strict", "-o", stagedIndex, stagedPack); err != nil {
		cleanup()
		return fmt.Errorf("index-pack --strict refused %s: %w", base, err)
	}
	if err := os.Rename(stagedPack, finalPack); err != nil {
		cleanup()
		return err
	}
	if err := os.Rename(stagedIndex, finalIndex); err != nil {
		cleanup()
		return err
	}
	return syncDirectory(packDir)
}

// covered proves a published index usable (verify-pack reads the pack
// beside it) and listing every id.
func covered(ctx context.Context, git WorkspaceGit, gitRoot, index string, ids []string) error {
	if _, err := os.Lstat(index); err != nil {
		return err
	}
	out, err := git(ctx, gitRoot, "verify-pack", "-v", index)
	if err != nil {
		return fmt.Errorf("verify-pack %s: %w", filepath.Base(index), err)
	}
	listed := map[string]bool{}
	for _, line := range strings.Split(string(out), "\n") {
		if fields := strings.Fields(line); len(fields) > 0 && isHex(fields[0]) {
			listed[fields[0]] = true
		}
	}
	for _, id := range ids {
		if !listed[id] {
			return fmt.Errorf("verify-pack %s does not list %s", filepath.Base(index), id)
		}
	}
	return nil
}
