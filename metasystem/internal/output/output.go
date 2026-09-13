// Package output owns bounded command output retained under the control root.
package output

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

const MaxInlineBytes = 32 * 1024
const Dir = "artifacts/agents/output"

const spillTimeLayout = "2006-01-02T15-04-05.000000000Z"

var validVerb = regexp.MustCompile(`^[a-z][a-z0-9-]{0,31}$`)

type Reference struct {
	OutputMode    string `json:"outputMode"`
	SchemaVersion int    `json:"schemaVersion"`
	Verb          string `json:"verb"`
	Path          string `json:"path"`
	Bytes         int    `json:"bytes"`
	Digest        string `json:"digest"`
	Format        string `json:"format"`
}

var randomNonce = func() (string, error) {
	bytes := make([]byte, 4)
	if _, err := io.ReadFull(rand.Reader, bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

func Spill(root, verb, ext string, data []byte, now time.Time) (Reference, error) {
	if !filepath.IsAbs(root) {
		return Reference{}, fmt.Errorf("output root must be absolute: %s", root)
	}
	if !validVerb.MatchString(verb) {
		return Reference{}, fmt.Errorf("invalid output verb %q", verb)
	}
	format := "text"
	switch ext {
	case "json":
		format = "json"
	case "log", "txt":
	default:
		return Reference{}, fmt.Errorf("invalid output extension %q", ext)
	}

	dir := filepath.Join(root, filepath.FromSlash(Dir))
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return Reference{}, fmt.Errorf("create output directory: %w", err)
	}
	digest := sha256.Sum256(data)
	reference := Reference{
		OutputMode:    "file",
		SchemaVersion: 1,
		Verb:          verb,
		Bytes:         len(data),
		Digest:        "sha256:" + hex.EncodeToString(digest[:]),
		Format:        format,
	}
	stamp := now.UTC().Format(spillTimeLayout)
	for attempt := 0; attempt < 8; attempt++ {
		nonce, err := randomNonce()
		if err != nil {
			return Reference{}, fmt.Errorf("create output nonce: %w", err)
		}
		path := filepath.Join(dir, fmt.Sprintf("%s-%s-%d-%s.%s", verb, stamp, os.Getpid(), nonce, ext))
		file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
		if errors.Is(err, os.ErrExist) {
			continue
		}
		if err != nil {
			return Reference{}, fmt.Errorf("create output file: %w", err)
		}
		if _, err := file.Write(data); err != nil {
			_ = file.Close()
			_ = os.Remove(path)
			return Reference{}, fmt.Errorf("write output file: %w", err)
		}
		if err := file.Close(); err != nil {
			_ = os.Remove(path)
			return Reference{}, fmt.Errorf("close output file: %w", err)
		}
		reference.Path = path
		return reference, nil
	}
	return Reference{}, fmt.Errorf("create output file: exhausted collision retries")
}

func Detect(data []byte) (Reference, bool) {
	var reference Reference
	if err := json.Unmarshal(data, &reference); err != nil {
		return Reference{}, false
	}
	return reference, reference.OutputMode == "file" && reference.SchemaVersion == 1
}

func (r Reference) Line() string {
	digest := strings.TrimPrefix(r.Digest, "sha256:")
	return fmt.Sprintf("output-reference verb=%s path=%s bytes=%d sha256=%s format=%s", r.Verb, r.Path, r.Bytes, digest, r.Format)
}

func Prune(root string, olderThan time.Duration, now time.Time) ([]string, error) {
	if olderThan < 0 {
		return nil, fmt.Errorf("output retention window must not be negative")
	}
	dir, err := filepath.Abs(filepath.Join(root, filepath.FromSlash(Dir)))
	if err != nil {
		return nil, fmt.Errorf("resolve output directory: %w", err)
	}
	dirInfo, err := os.Lstat(dir)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("inspect output directory: %w", err)
	}
	if dirInfo.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("output directory must not be a symlink: %s", dir)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("list output directory: %w", err)
	}
	cutoff := now.Add(-olderThan)
	removed := make([]string, 0)
	for _, entry := range entries {
		path := filepath.Join(dir, entry.Name())
		info, err := os.Lstat(path)
		if err != nil {
			return nil, fmt.Errorf("inspect output path %s: %w", path, err)
		}
		if !info.Mode().IsRegular() || !info.ModTime().Before(cutoff) {
			continue
		}
		if err := os.Remove(path); err != nil {
			return nil, fmt.Errorf("remove output file %s: %w", path, err)
		}
		removed = append(removed, path)
	}
	sort.Strings(removed)
	return removed, nil
}
