package proofrun

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
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
	if destination == "" || maxBytes < 1 {
		return EvidenceResult{}, errors.New("evidence destination and positive byte cap are required")
	}
	if err := os.MkdirAll(destination, 0o700); err != nil {
		return EvidenceResult{}, fmt.Errorf("create watchdog evidence directory: %w", err)
	}
	result := EvidenceResult{}
	for index, source := range sources {
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
				return copyEvidenceEntry(path, filepath.Join(target, rel), entry, maxBytes, &result)
			})
		} else {
			err = copyEvidenceEntry(source, target, dirEntryFromInfo{info}, maxBytes, &result)
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

type dirEntryFromInfo struct{ os.FileInfo }

func (d dirEntryFromInfo) Type() fs.FileMode          { return d.Mode().Type() }
func (d dirEntryFromInfo) Info() (os.FileInfo, error) { return d.FileInfo, nil }

func copyEvidenceEntry(source, target string, entry fs.DirEntry, maxBytes int64, result *EvidenceResult) error {
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
	written, copyErr := io.CopyN(out, in, remaining)
	if copyErr == io.EOF {
		copyErr = nil
	}
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
