package proofrun

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type EvidenceResult struct {
	CopiedBytes int64
	Dropped     []string
	Errors      []string
}

// PreserveEvidence copies only regular files and symlinks from the header's
// declared evidence inventory. The caller supplies the hard byte ceiling; a
// separate bounded process supplies the hard wall-clock ceiling.
func PreserveEvidence(destination string, sources []string, maxBytes int64) (EvidenceResult, error) {
	return preserveEvidence(context.Background(), destination, sources, maxBytes)
}

func preserveEvidence(ctx context.Context, destination string, sources []string, maxBytes int64) (EvidenceResult, error) {
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
				return copyEvidenceEntry(ctx, path, filepath.Join(target, rel), entry, maxBytes, &result)
			})
		} else {
			err = copyEvidenceEntry(ctx, source, target, dirEntryFromInfo{info}, maxBytes, &result)
		}
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("%s: %v", source, err))
		}
	}
	if err := writeEvidenceNote(destination, result); err != nil {
		return result, err
	}
	return result, nil
}

// PreserveDetachedSuiteFailures moves the ignored fixture failure subtree out
// of a disposable candidate before its owner removes that candidate. It is
// deliberately specific to this cleanup boundary; ordinary evidence capture
// continues to use PreserveEvidence directly.
func PreserveDetachedSuiteFailures(controlRoot, candidateRoot, owner string, timeout time.Duration, maxBytes int64) (EvidenceResult, string, error) {
	if controlRoot == "" || candidateRoot == "" || owner == "" || timeout <= 0 || maxBytes < 1 {
		return EvidenceResult{}, "", errors.New("detached evidence preservation requires control root, candidate root, owner, and positive bounds")
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
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	var sources []string
	err = filepath.WalkDir(candidateRoot, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if err := ctx.Err(); err != nil {
			return err
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
	destination := filepath.Join(controlRoot, "artifacts", "agents", "suite-failures",
		time.Now().UTC().Format("20060102T150405Z")+"-detached-"+safeEvidenceName(owner)+fmt.Sprintf("-%d-%d", os.Getpid(), time.Now().UnixNano()))
	result, copyErr := preserveEvidence(ctx, destination, sources, maxBytes)
	if ctx.Err() != nil {
		note := fmt.Sprintf("DROPPED detached evidence copy exceeded its %s timeout; partial evidence retained", timeout)
		_ = AppendEvidenceNote(destination, note)
		return result, destination, fmt.Errorf("%s at %s", note, destination)
	}
	if copyErr != nil {
		return result, destination, fmt.Errorf("preserve detached suite-failure evidence at %s: %w", destination, copyErr)
	}
	if len(result.Dropped) > 0 || len(result.Errors) > 0 {
		return result, destination, fmt.Errorf("detached suite-failure evidence was only partially preserved at %s (dropped=%d errors=%d)", destination, len(result.Dropped), len(result.Errors))
	}
	return result, destination, nil
}

type dirEntryFromInfo struct{ os.FileInfo }

func (d dirEntryFromInfo) Type() fs.FileMode          { return d.Mode().Type() }
func (d dirEntryFromInfo) Info() (os.FileInfo, error) { return d.FileInfo, nil }

func copyEvidenceEntry(ctx context.Context, source, target string, entry fs.DirEntry, maxBytes int64, result *EvidenceResult) error {
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
	if info.Mode()&os.ModeSymlink != 0 {
		link, err := os.Readlink(source)
		if err != nil {
			return err
		}
		if result.CopiedBytes+int64(len(link)) > maxBytes {
			result.Dropped = append(result.Dropped, source+" (size cap)")
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
	remaining := maxBytes - result.CopiedBytes
	if remaining <= 0 {
		result.Dropped = append(result.Dropped, source+" (size cap)")
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

func writeEvidenceNote(destination string, result EvidenceResult) error {
	file, err := os.OpenFile(filepath.Join(destination, "copy-note.txt"), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return fmt.Errorf("write watchdog evidence note: %w", err)
	}
	writer := bufio.NewWriter(file)
	fmt.Fprintf(writer, "copied-bytes=%d\n", result.CopiedBytes)
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
