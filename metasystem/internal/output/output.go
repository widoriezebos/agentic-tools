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

// NewestSince returns the newest regular spill written strictly after since.
// Spill discovery is an optional status hint, so an unavailable directory
// produces no result instead of making the caller's primary check fail.
func NewestSince(root string, since time.Time) (path string, bytes int64, ok bool) {
	directory := filepath.Join(root, filepath.FromSlash(Dir))
	entries, err := os.ReadDir(directory)
	if err != nil {
		return "", 0, false
	}
	var newest time.Time
	var newestName string
	for _, entry := range entries {
		candidate := filepath.Join(directory, entry.Name())
		info, err := os.Lstat(candidate)
		if err != nil || !info.Mode().IsRegular() || !info.ModTime().After(since) {
			continue
		}
		if newestName == "" || info.ModTime().After(newest) ||
			(info.ModTime().Equal(newest) && entry.Name() < newestName) {
			newest = info.ModTime()
			newestName = entry.Name()
			path = candidate
			bytes = info.Size()
		}
	}
	return path, bytes, newestName != ""
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
