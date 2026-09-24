// Package proofrun owns the immutable filesystem projection used to transfer
// one gate proof to byte-identical descendants.
package proofrun

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/behaviorsurface"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
)

const recordLengthBytes = 8

var ErrDigestMismatch = errors.New("proof-run manifest digest mismatch")

type entry struct {
	path       string
	kind       byte
	executable bool
	target     []byte
	fileDigest [sha256.Size]byte
	digested   bool
}

type manifest struct {
	entries []entry
	records []byte
	digest  string
	layout  []byte
}

type completeManifestRoot struct {
	projectRoot        string
	installationPrefix string
	hasGit             bool
}

func (m manifest) fullDigest() string {
	var framed bytes.Buffer
	framed.Write(m.layout)
	for _, item := range m.entries {
		body := recordBody(item)
		var length [recordLengthBytes]byte
		binary.BigEndian.PutUint64(length[:], uint64(len(body)))
		framed.Write(length[:])
		framed.Write(body)
	}
	digest := sha256.Sum256(framed.Bytes())
	return hex.EncodeToString(digest[:])
}

// The manifest record format is normative. Each entry below the root, except
// the root-level artifacts/, bin/, and .git closures, contributes one record.
// A record body is the raw relative-path bytes followed by NUL, one kind byte
// (f, l, or d), and one executable byte (0 or 1). Only mode&0100, the owner
// execute bit, decides that byte; group and other execute bits do not. A
// symlink body then carries its raw target bytes; a regular-file body instead
// carries the 32 raw bytes of the SHA-256 digest of its contents; a directory
// body ends there. Symlinks are never followed. Records sort by raw path bytes.
// Each body is prefixed by its unsigned 64-bit big-endian byte length, and the
// manifest digest is the SHA-256 digest of the concatenated length-framed
// records.
func readManifest(root string) (manifest, error) {
	return readManifestAt(root, "", false)
}

func readCompleteManifest(root string) (manifest, completeManifestRoot, error) {
	location, err := resolveCompleteManifestRoot(root)
	if err != nil {
		return manifest{}, completeManifestRoot{}, err
	}
	result, err := readManifestAt(location.projectRoot, location.installationPrefix, true)
	if err == nil {
		result.layout = completeLayoutRecord(location)
	}
	return result, location, err
}

func readManifestAt(root, installationPrefix string, complete bool) (manifest, error) {
	return readManifestAtWithHook(root, installationPrefix, complete, nil)
}

func readManifestAtWithHook(root, installationPrefix string, complete bool, beforeEntry func(string, fs.DirEntry) error) (manifest, error) {
	canonical, err := filepath.Abs(root)
	if err != nil {
		return manifest{}, fmt.Errorf("resolve manifest root: %w", err)
	}
	info, err := os.Stat(canonical)
	if err != nil {
		return manifest{}, fmt.Errorf("read manifest root: %w", err)
	}
	if !info.IsDir() {
		return manifest{}, fmt.Errorf("manifest root is not a directory: %s", root)
	}

	entries := make([]entry, 0)
	err = filepath.WalkDir(canonical, func(path string, dirEntry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == canonical {
			return nil
		}
		rel, err := filepath.Rel(canonical, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if isLocalConfiguration(rel) {
			if dirEntry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if beforeEntry != nil {
			if err := beforeEntry(rel, dirEntry); err != nil {
				return err
			}
		}
		if dirEntry.IsDir() {
			if complete && rel == ".git" {
				return filepath.SkipDir
			}
			if complete {
				excluded, err := completeDirectoryExcluded(rel, installationPrefix)
				if err != nil {
					return err
				}
				if excluded {
					return filepath.SkipDir
				}
			}
			// Only the hard runtime roots prune whole subtrees; a
			// directory name alone cannot answer projection membership
			// (bare "scripts" matches no pattern while
			// scripts/agents/** lives beneath it — first drawn as a
			// dropped go-gate.sh in the frozen export, 2026-08-29).
			if !complete && hardExcluded(rel) {
				return filepath.SkipDir
			}
			return nil
		}
		if !complete && hardExcluded(rel) {
			return nil
		}
		if complete {
			included, err := completeInputIncluded(rel, installationPrefix)
			if err != nil {
				return err
			}
			if !included {
				return nil
			}
		}
		item, err := inspectEntry(path, rel, dirEntry)
		if err != nil {
			return err
		}
		// Export scope and digest scope deliberately differ: every
		// non-runtime byte is EXPORTED so tests find the content they
		// read (a projection-only export skewed coverage by mass
		// skips, measured 2026-08-29), while only ENGINE-projection
		// members are DIGESTED so deliberately mutated fixture copies
		// still byte-match and reuse the proof.
		item.digested = complete || engineProjectionMember(rel)
		entries = append(entries, item)
		return nil
	})
	if err != nil {
		return manifest{}, fmt.Errorf("walk proof-run manifest: %w", err)
	}
	sort.Slice(entries, func(i, j int) bool {
		return bytes.Compare([]byte(entries[i].path), []byte(entries[j].path)) < 0
	})

	var framed bytes.Buffer
	for _, item := range entries {
		if !item.digested {
			continue
		}
		body := recordBody(item)
		var length [recordLengthBytes]byte
		binary.BigEndian.PutUint64(length[:], uint64(len(body)))
		framed.Write(length[:])
		framed.Write(body)
	}
	digest := sha256.Sum256(framed.Bytes())
	return manifest{entries: entries, records: framed.Bytes(), digest: hex.EncodeToString(digest[:])}, nil
}

func completeLayoutRecord(location completeManifestRoot) []byte {
	kind := "no-git"
	if location.hasGit {
		kind = "git"
	}
	body := []byte("complete-project-layout\x00" + kind + "\x00" + location.installationPrefix)
	framed := make([]byte, recordLengthBytes, recordLengthBytes+len(body))
	binary.BigEndian.PutUint64(framed, uint64(len(body)))
	return append(framed, body...)
}

func completeDirectoryExcluded(rel, installationPrefix string) (bool, error) {
	prefix := strings.Trim(filepath.ToSlash(installationPrefix), "/")
	if prefix != "" && (rel == prefix || strings.HasPrefix(prefix, rel+"/")) {
		return false, nil
	}
	included, err := completeInputIncluded(rel, installationPrefix)
	return !included, err
}

func resolveCompleteManifestRoot(root string) (completeManifestRoot, error) {
	canonical, err := filepath.Abs(root)
	if err == nil {
		canonical, err = filepath.EvalSymlinks(canonical)
	}
	if err != nil {
		return completeManifestRoot{}, fmt.Errorf("resolve complete manifest root: %w", err)
	}
	info, err := os.Stat(canonical)
	if err != nil || !info.IsDir() {
		if err == nil {
			err = fmt.Errorf("not a directory")
		}
		return completeManifestRoot{}, fmt.Errorf("read complete manifest root %s: %w", root, err)
	}
	if !hasGitMarker(canonical) {
		return completeManifestRoot{projectRoot: canonical}, nil
	}
	workspace := gittree.Workspace{Dir: canonical}
	projectRoot, err := workspace.TopLevel()
	if err != nil {
		return completeManifestRoot{}, fmt.Errorf("resolve frozen project root: %w", err)
	}
	projectRoot, err = filepath.EvalSymlinks(projectRoot)
	if err != nil {
		return completeManifestRoot{}, fmt.Errorf("resolve frozen project root: %w", err)
	}
	prefix, err := filepath.Rel(projectRoot, canonical)
	if err != nil || prefix == ".." || strings.HasPrefix(prefix, ".."+string(filepath.Separator)) {
		return completeManifestRoot{}, fmt.Errorf("resolve frozen installation prefix: %w", err)
	}
	if prefix == "." {
		prefix = ""
	} else {
		prefix = filepath.ToSlash(prefix)
	}
	return completeManifestRoot{projectRoot: projectRoot, installationPrefix: prefix, hasGit: true}, nil
}

func hasGitMarker(root string) bool {
	for current := root; ; current = filepath.Dir(current) {
		if _, err := os.Lstat(filepath.Join(current, ".git")); err == nil {
			return true
		}
		parent := filepath.Dir(current)
		if parent == current {
			return false
		}
	}
}

func completeInputIncluded(rel, installationPrefix string) (bool, error) {
	if isLocalConfiguration(rel) || rel == ".git" || strings.HasPrefix(rel, ".git/") {
		return false, nil
	}
	if enginePolicyErr != nil {
		return false, fmt.Errorf("load behavior-surface policy for frozen input: %w", enginePolicyErr)
	}
	return enginePolicy.Includes(behaviorsurface.Landing, rel, installationPrefix)
}

func isLocalConfiguration(rel string) bool {
	for _, component := range strings.Split(filepath.ToSlash(rel), "/") {
		if component == "metasystem.conf.local" {
			return true
		}
	}
	return false
}

// excluded inverts the ENGINE projection: the frozen closure is the
// gate's DECLARED input set (behavior-surface enginePaths — the data
// m1's witness always scoped to, plus named test-read files), so
// deliberately mutated fixture copies (pruned skills, tailored conf)
// still byte-match and reuse the proof. The original whole-tree
// closure made every nested mutated copy re-pay the full gate, which
// defeated the goal (measured 2026-08-29: 1h adopt with whole-tree
// vs the armed class with projection scope).
func hardExcluded(rel string) bool {
	components := strings.Split(rel, "/")
	if first := components[0]; first == "artifacts" || first == "bin" || first == ".git" {
		return true
	}
	// An installed dependency tree is content on disk, never repository
	// content: it is excluded by name at any depth, as vendor is skipped
	// elsewhere, so a frozen candidate receives no copy of it (g1-s8
	// revision 5, the exclusion slice).
	for _, component := range components {
		if component == "node_modules" {
			return true
		}
	}
	return false
}

var enginePolicy, enginePolicyErr = behaviorsurface.Load()

func engineProjectionMember(rel string) bool {
	if enginePolicyErr != nil {
		// A missing policy fails closed to the whole tree (sound,
		// merely slow), never silently narrowing the proof.
		return true
	}
	include, err := enginePolicy.Includes(behaviorsurface.Engine, rel, "")
	if err != nil {
		return true
	}
	return include
}

func inspectEntry(path, rel string, dirEntry fs.DirEntry) (entry, error) {
	info, err := dirEntry.Info()
	if err != nil {
		return entry{}, fmt.Errorf("lstat %q: %w", rel, err)
	}
	item := entry{path: rel, executable: info.Mode().Perm()&0100 != 0}
	switch {
	case info.Mode().IsRegular():
		item.kind = 'f'
		file, err := os.Open(path)
		if err != nil {
			return entry{}, fmt.Errorf("open %q: %w", rel, err)
		}
		hash := sha256.New()
		_, copyErr := io.Copy(hash, file)
		closeErr := file.Close()
		if copyErr != nil {
			return entry{}, fmt.Errorf("hash %q: %w", rel, copyErr)
		}
		if closeErr != nil {
			return entry{}, fmt.Errorf("close %q: %w", rel, closeErr)
		}
		copy(item.fileDigest[:], hash.Sum(nil))
	case info.Mode()&os.ModeSymlink != 0:
		item.kind = 'l'
		target, err := os.Readlink(path)
		if err != nil {
			return entry{}, fmt.Errorf("read symlink %q: %w", rel, err)
		}
		item.target = []byte(target)
	case info.IsDir():
		item.kind = 'd'
	default:
		return entry{}, fmt.Errorf("unsupported entry kind at %q", rel)
	}
	return item, nil
}

func recordBody(item entry) []byte {
	body := make([]byte, 0, len(item.path)+1+2+sha256.Size)
	body = append(body, []byte(item.path)...)
	body = append(body, 0, item.kind)
	if item.executable {
		body = append(body, 1)
	} else {
		body = append(body, 0)
	}
	switch item.kind {
	case 'f':
		body = append(body, item.fileDigest[:]...)
	case 'l':
		body = append(body, item.target...)
	}
	return body
}

func Digest(root string) (string, error) {
	m, err := readManifest(root)
	if err != nil {
		return "", err
	}
	return m.digest, nil
}

// FullDigest uses the same framing and exclusions as the witness manifest but
// includes every non-runtime source/input entry, not only ENGINE projection.
func FullDigest(root string) (string, error) {
	m, _, err := readCompleteManifest(root)
	if err != nil {
		return "", err
	}
	return m.fullDigest(), nil
}

func Verify(root, expected string) (string, error) {
	if len(expected) != sha256.Size*2 {
		return "", fmt.Errorf("expected manifest digest must be 64 lowercase hexadecimal characters")
	}
	if _, err := hex.DecodeString(expected); err != nil || strings.ToLower(expected) != expected {
		return "", fmt.Errorf("expected manifest digest must be 64 lowercase hexadecimal characters")
	}
	actual, err := FullDigest(root)
	if err != nil {
		return "", err
	}
	if actual != expected {
		return actual, fmt.Errorf("%w: expected %s, found %s", ErrDigestMismatch, expected, actual)
	}
	return actual, nil
}
