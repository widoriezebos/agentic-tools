package proofrun

import (
	"bufio"
	"context"
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

	"github.com/widoriezebos/agentic-tools/metasystem/internal/diskstore"
)

// proofAttemptEnvironment names the proof attempt a launched suite runs
// under (launcher.go exports it).
const proofAttemptEnvironment = "METASYSTEM_PROOF_ATTEMPT"

// WriteBundleOwner writes a suite-failure bundle's OWNER.json before the
// bundle receives a byte (design engine-owns-disk-lifetimes 3.5, 3.12
// clause 1): the attempt the bundle was written under and the goal that
// attempt is accounted to, read from its record now; "standalone" and goal
// "none" outside an attempt; goal "unknown" when the record cannot be read.
// The checkout facts that need git (root commit, ledger identity) are
// completed by the checkout pass that distils the bundle: this path runs
// when a suite has failed and runs no git.
// installation is where the bundle lives; attemptRoot holds the attempt's
// record.
func WriteBundleOwner(bundle, installation, attemptRoot, attempt string, now time.Time) error {
	owner := diskstore.BundleOwner{Attempt: diskstore.AttemptStandalone, Goal: diskstore.GoalNone,
		Installation: installation, GitRoot: gitRootAbove(installation), WrittenBy: "bundle-writer", WrittenAt: now.UTC()}
	if attempt != "" {
		owner.Attempt, owner.Goal = attempt, diskstore.GoalUnknown
		if record, err := ReadAttempt(attemptRoot, attempt); err == nil {
			owner.Goal = diskstore.GoalNone
			if goal := record.AccountedGoal(); goal != "" {
				owner.Goal = goal
			}
		}
	}
	_, err := diskstore.WriteBundleOwner(bundle, owner, diskstore.Syncer{})
	return err
}

// gitRootAbove is the nearest directory from dir upward holding a .git
// entry, else dir.
func gitRootAbove(dir string) string {
	for current := dir; ; {
		if _, err := os.Lstat(filepath.Join(current, ".git")); err == nil {
			return current
		}
		parent := filepath.Dir(current)
		if parent == current {
			return dir
		}
		current = parent
	}
}

type EvidenceResult struct {
	CopiedBytes int64
	Dropped     []string
	Errors      []string
	// Truncated is true when the byte cap cut the bundle short.
	Truncated bool
}

// PreserveEvidence copies only regular files and symlinks from the header's
// declared evidence inventory, never an engine artifact store or git object
// store nested inside a copied tree (nestedStoreReason); a bundle cut short
// by the byte cap carries TruncationMarker in its copy note. The caller supplies the hard byte ceiling; a
// separate bounded process supplies the hard wall-clock ceiling.
func PreserveEvidence(destination string, sources []string, maxBytes int64) (EvidenceResult, error) {
	return preserveEvidence(context.Background(), destination, sources, maxBytes, evidenceCopyRules{})
}

// DetachedEvidenceGroupMaxBytes bounds one group's detached suite-failure
// bundle. It is diagnostic evidence, not a copy of the candidate.
const DetachedEvidenceGroupMaxBytes int64 = 16 * 1024 * 1024

// detachedEvidenceFileMaxBytes omits any single regular file larger than
// this from a detached bundle; a log survives while a stray blob does not.
const detachedEvidenceFileMaxBytes int64 = 1024 * 1024

// evidenceCopyRules narrows what a copy takes. The zero value is the generic
// PreserveEvidence behavior.
type evidenceCopyRules struct {
	skipGitDirs  bool
	skipSymlinks bool
	maxFileBytes int64
}

func preserveEvidence(ctx context.Context, destination string, sources []string, maxBytes int64, rules evidenceCopyRules) (EvidenceResult, error) {
	if destination == "" || maxBytes < 1 {
		return EvidenceResult{}, errors.New("evidence destination and positive byte cap are required")
	}
	if err := os.MkdirAll(destination, 0o700); err != nil {
		return EvidenceResult{}, fmt.Errorf("create watchdog evidence directory: %w", err)
	}
	result := EvidenceResult{}
	for index, source := range sources {
		if err := ctx.Err(); err != nil {
			result.Errors = append(result.Errors, err.Error())
			break
		}
		if source == "" {
			continue
		}
		label := fmt.Sprintf("source-%03d-%s", index+1, safeEvidenceName(filepath.Base(source)))
		target := filepath.Join(destination, label)
		info, err := os.Lstat(source)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("%s: %v", source, err))
			continue
		}
		if info.IsDir() {
			err = filepath.WalkDir(source, func(path string, entry fs.DirEntry, walkErr error) error {
				if err := ctx.Err(); err != nil {
					return err
				}
				if walkErr != nil {
					result.Errors = append(result.Errors, fmt.Sprintf("%s: %v", path, walkErr))
					return nil
				}
				rel, relErr := filepath.Rel(source, path)
				if relErr != nil {
					result.Errors = append(result.Errors, fmt.Sprintf("%s: %v", path, relErr))
					return nil
				}
				if rel == "." {
					return os.MkdirAll(target, 0o700)
				}
				if entry.IsDir() {
					if reason := nestedStoreReason(rel, rules); reason != "" {
						result.Dropped = append(result.Dropped, path+" ("+reason+")")
						return filepath.SkipDir
					}
				}
				return copyEvidenceEntry(ctx, path, filepath.Join(target, rel), entry, maxBytes, rules, &result)
			})
		} else {
			err = copyEvidenceEntry(ctx, source, target, dirEntryFromInfo{info}, maxBytes, rules, &result)
		}
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("%s: %v", source, err))
		}
	}
	if err := writeEvidenceNote(destination, result, maxBytes); err != nil {
		return result, err
	}
	return result, nil
}

// PreserveDetachedSuiteFailures moves the ignored fixture failure subtree out
// of a disposable candidate before its owner removes that candidate. It is
// deliberately specific to this cleanup boundary; ordinary evidence capture
// continues to use PreserveEvidence directly. The copy is diagnostic and
// bounded: it skips .git directories, symlinks, and regular files over 1 MiB,
// and stops at maxBytes. Those omissions are recorded as DROPPED lines in the
// bundle's copy note and are not errors.
func PreserveDetachedSuiteFailures(controlRoot, candidateRoot, owner string, timeout time.Duration, maxBytes int64) (EvidenceResult, string, error) {
	// A zero timeout is no clock (the copy is bounded by bytes only); the
	// byte bound stays positive.
	if controlRoot == "" || candidateRoot == "" || owner == "" || maxBytes < 1 {
		return EvidenceResult{}, "", errors.New("keeping evidence needs a control root, a candidate root, an owner and a byte limit")
	}
	controlRoot, err := filepath.Abs(controlRoot)
	if err != nil {
		return EvidenceResult{}, "", err
	}
	candidateRoot, err = filepath.Abs(candidateRoot)
	if err != nil {
		return EvidenceResult{}, "", err
	}
	if relative, relErr := filepath.Rel(candidateRoot, controlRoot); relErr != nil {
		return EvidenceResult{}, "", relErr
	} else if relative == "." || relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return EvidenceResult{}, "", errors.New("detached evidence control root is inside the disposable candidate")
	}
	ctx, cancel := context.WithCancel(context.Background())
	if timeout > 0 {
		ctx, cancel = context.WithTimeout(context.Background(), timeout)
	}
	defer cancel()
	var sources []string
	err = filepath.WalkDir(candidateRoot, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if entry.IsDir() && entry.Name() == ".git" && path != candidateRoot {
			return filepath.SkipDir
		}
		if entry.IsDir() && strings.HasSuffix(filepath.ToSlash(path), "/artifacts/agents/suite-failures") {
			sources = append(sources, path)
			return filepath.SkipDir
		}
		return nil
	})
	if err != nil {
		return EvidenceResult{}, "", fmt.Errorf("find detached suite-failure evidence: %w", err)
	}
	if len(sources) == 0 {
		return EvidenceResult{}, "", nil
	}
	sort.Strings(sources)
	attempt := os.Getenv(proofAttemptEnvironment)
	destination := filepath.Join(controlRoot, "artifacts", "agents", "suite-failures",
		time.Now().UTC().Format("20060102T150405Z")+"-detached-"+safeEvidenceName(owner)+"-"+diskstore.BundleRunName(attempt, os.Getpid())+fmt.Sprintf("-%d", time.Now().UnixNano()))
	if err := WriteBundleOwner(destination, controlRoot, controlRoot, attempt, time.Now()); err != nil {
		return EvidenceResult{}, destination, fmt.Errorf("preserve detached suite-failure evidence at %s: %w", destination, err)
	}
	result, copyErr := preserveEvidence(ctx, destination, sources, maxBytes,
		evidenceCopyRules{skipGitDirs: true, skipSymlinks: true, maxFileBytes: detachedEvidenceFileMaxBytes})
	if ctx.Err() != nil {
		note := fmt.Sprintf("DROPPED detached evidence copy exceeded its %s timeout; partial evidence retained", timeout)
		_ = AppendEvidenceNote(destination, note)
		return result, destination, fmt.Errorf("%s at %s", note, destination)
	}
	if copyErr != nil {
		return result, destination, fmt.Errorf("preserve detached suite-failure evidence at %s: %w", destination, copyErr)
	}
	// Dropped entries are intentional bounds recorded in the copy note;
	// only real read or write failures make the preservation incomplete.
	if len(result.Errors) > 0 {
		return result, destination, fmt.Errorf("detached suite-failure evidence was only partially preserved at %s (dropped=%d errors=%d)", destination, len(result.Dropped), len(result.Errors))
	}
	return result, destination, nil
}

// nestedStoreReason names why a directory nested below a copied source is
// not copied, or returns "" to copy it. Every capture skips the engine's own
// artifact store (artifacts/agents: suite-failures, proof-runs, candidate
// engines, steward pins) and git object stores wherever they occur inside
// the copied tree: an agent fixture repository carries the prior captures in
// its store, and copying them made each capture nest every earlier one
// (2.1 GB to 16.8 GB in 15 minutes, 2026-09-28). The source root itself is
// never skipped; rel is the slash path below it.
func nestedStoreReason(rel string, rules evidenceCopyRules) string {
	rel = filepath.ToSlash(rel)
	base := path.Base(rel)
	switch {
	case rules.skipGitDirs && base == ".git":
		return "git directory skipped"
	case base == "objects" && path.Base(path.Dir(rel)) == ".git":
		return "git object store skipped"
	case base == "agents" && path.Base(path.Dir(rel)) == "artifacts":
		return "metasystem artifact store skipped"
	}
	return ""
}

type dirEntryFromInfo struct{ os.FileInfo }

func (d dirEntryFromInfo) Type() fs.FileMode          { return d.Mode().Type() }
func (d dirEntryFromInfo) Info() (os.FileInfo, error) { return d.FileInfo, nil }

func copyEvidenceEntry(ctx context.Context, source, target string, entry fs.DirEntry, maxBytes int64, rules evidenceCopyRules, result *EvidenceResult) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	info, err := entry.Info()
	if err != nil {
		return err
	}
	if info.IsDir() {
		return os.MkdirAll(target, 0o700)
	}
	if info.Mode()&os.ModeSymlink != 0 && rules.skipSymlinks {
		result.Dropped = append(result.Dropped, source+" (symlink not followed)")
		return nil
	}
	if info.Mode()&os.ModeSymlink != 0 {
		link, err := os.Readlink(source)
		if err != nil {
			return err
		}
		if result.CopiedBytes+int64(len(link)) > maxBytes {
			result.Dropped = append(result.Dropped, source+" (size cap)")
			result.Truncated = true
			return nil
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
			return err
		}
		if err := os.Symlink(link, target); err != nil {
			return err
		}
		result.CopiedBytes += int64(len(link))
		return nil
	}
	if !info.Mode().IsRegular() {
		result.Dropped = append(result.Dropped, source+" (not a regular file or symlink)")
		return nil
	}
	if rules.maxFileBytes > 0 && info.Size() > rules.maxFileBytes {
		result.Dropped = append(result.Dropped, fmt.Sprintf("%s (%d bytes over the %d-byte per-file cap)", source, info.Size(), rules.maxFileBytes))
		return nil
	}
	remaining := maxBytes - result.CopiedBytes
	if remaining <= 0 {
		result.Dropped = append(result.Dropped, source+" (size cap)")
		result.Truncated = true
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
		return err
	}
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	written, copyErr := copyEvidenceBytes(ctx, out, in, remaining)
	closeErr := out.Close()
	result.CopiedBytes += written
	if copyErr != nil {
		return copyErr
	}
	if closeErr != nil {
		return closeErr
	}
	if info.Size() > written {
		result.Dropped = append(result.Dropped, fmt.Sprintf("%s (%d bytes beyond size cap)", source, info.Size()-written))
		result.Truncated = true
	}
	return nil
}

func copyEvidenceBytes(ctx context.Context, out *os.File, in *os.File, limit int64) (int64, error) {
	buffer := make([]byte, 32*1024)
	var written int64
	for written < limit {
		if err := ctx.Err(); err != nil {
			return written, err
		}
		want := int64(len(buffer))
		if remaining := limit - written; remaining < want {
			want = remaining
		}
		read, readErr := in.Read(buffer[:want])
		if read > 0 {
			stored, writeErr := out.Write(buffer[:read])
			written += int64(stored)
			if writeErr != nil {
				return written, writeErr
			}
			if stored != read {
				return written, errors.New("short evidence write")
			}
		}
		if readErr != nil {
			if errors.Is(readErr, os.ErrClosed) {
				return written, readErr
			}
			if errors.Is(readErr, io.EOF) {
				return written, nil
			}
			return written, readErr
		}
	}
	return written, nil
}

// TruncationMarker is the one line a capture's copy note carries when the
// bundle reached its byte cap.
func TruncationMarker(maxBytes int64) string {
	return fmt.Sprintf("TRUNCATED bundle reached its %d-byte cap; the DROPPED lines name what was not copied", maxBytes)
}

func writeEvidenceNote(destination string, result EvidenceResult, maxBytes int64) error {
	file, err := os.OpenFile(filepath.Join(destination, "copy-note.txt"), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return fmt.Errorf("write watchdog evidence note: %w", err)
	}
	writer := bufio.NewWriter(file)
	fmt.Fprintf(writer, "copied-bytes=%d\n", result.CopiedBytes)
	if result.Truncated {
		fmt.Fprintln(writer, TruncationMarker(maxBytes))
	}
	for _, dropped := range result.Dropped {
		fmt.Fprintf(writer, "DROPPED %s\n", dropped)
	}
	for _, item := range result.Errors {
		fmt.Fprintf(writer, "ERROR %s\n", item)
	}
	flushErr := writer.Flush()
	closeErr := file.Close()
	if flushErr != nil {
		return flushErr
	}
	return closeErr
}

func AppendEvidenceNote(destination, note string) error {
	if err := os.MkdirAll(destination, 0o700); err != nil {
		return err
	}
	file, err := os.OpenFile(filepath.Join(destination, "copy-note.txt"), os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o600)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = fmt.Fprintln(file, strings.TrimSpace(note))
	return err
}

func safeEvidenceName(name string) string {
	if name == "" || name == "." || name == string(filepath.Separator) {
		return "root"
	}
	return strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_' || r == '.' {
			return r
		}
		return '_'
	}, name)
}
