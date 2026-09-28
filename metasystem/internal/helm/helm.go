// Package helm reads and writes a seat's human-at-the-helm signature. A seat
// is one checkout plus its linked worktrees, named by the git common dir; the
// signature is <common-dir>/metasystem/helm.json beside the append-only
// helm.log and helm-yields.log. Standard library only: the common dir is found
// as git finds it, without a subprocess or git's steering variables.
package helm

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

// Record is the signature helm take writes.
type Record struct {
	Schema     int    `json:"schema"`
	By         string `json:"by"`
	At         string `json:"at"`
	Reason     string `json:"reason"`
	Machine    string `json:"machine,omitempty"`
	Checkout   string `json:"checkout,omitempty"`
	Enrollment string `json:"enrollment,omitempty"`
	EnrolledAs string `json:"enrolledAs,omitempty"`
	Leader     string `json:"leader,omitempty"`
	LeaderRef  string `json:"leaderRef,omitempty"`
}

// State is what Active reads: the record (By, Reason, Machine, Checkout,
// Leader) and Since. Malformed says why a present signature could not be
// decoded (Active stays true, By is "unknown"); Diagnostic says why no
// repository was found (Active is false).
type State struct {
	Active bool
	Record
	Since                 time.Time
	Malformed, Diagnostic string
}

// Seat names one seat's files.
type Seat struct{ CommonDir, Checkout, Dir, Signature, Log, Yields string }

// Locate walks up from root to the first directory holding .git: a directory
// is the common dir; a "gitdir: PATH" file names the worktree's git dir, whose
// commondir file (when present) names the common dir.
func Locate(root string) (Seat, error) {
	dir, err := filepath.Abs(root)
	if err != nil {
		return Seat{}, err
	}
	if resolved, err := filepath.EvalSymlinks(dir); err == nil {
		dir = resolved
	}
	for ; ; dir = filepath.Dir(dir) {
		common := filepath.Join(dir, ".git")
		if info, err := os.Stat(common); err == nil {
			if !info.IsDir() {
				if common, err = commonFromFile(dir, common); err != nil {
					return Seat{}, err
				}
			}
			checkout := common
			if filepath.Base(common) == ".git" {
				checkout = filepath.Dir(common)
			}
			meta := filepath.Join(common, "metasystem")
			return Seat{CommonDir: common, Checkout: checkout, Dir: meta, Signature: filepath.Join(meta, "helm.json"),
				Log: filepath.Join(meta, "helm.log"), Yields: filepath.Join(meta, "helm-yields.log")}, nil
		}
		if filepath.Dir(dir) == dir {
			return Seat{}, fmt.Errorf("no git repository contains %s", root)
		}
	}
}

func commonFromFile(dir, dotgit string) (string, error) {
	data, err := os.ReadFile(dotgit)
	line := strings.TrimSpace(string(data))
	if err != nil || !strings.HasPrefix(line, "gitdir:") {
		return "", fmt.Errorf("%s does not name a git dir (%v)", dotgit, err)
	}
	gitdir := strings.TrimSpace(strings.TrimPrefix(line, "gitdir:"))
	if !filepath.IsAbs(gitdir) {
		gitdir = filepath.Join(dir, gitdir)
	}
	common := gitdir
	if pointer, err := os.ReadFile(filepath.Join(gitdir, "commondir")); err == nil {
		if common = strings.TrimSpace(string(pointer)); !filepath.IsAbs(common) {
			common = filepath.Join(gitdir, common)
		}
	}
	if resolved, err := filepath.EvalSymlinks(common); err == nil {
		common = resolved
	}
	return filepath.Clean(common), nil
}

func absent(err error) bool { return errors.Is(err, os.ErrNotExist) || errors.Is(err, syscall.ENOTDIR) }

// Active reports whether root's seat is at the helm. It never errors: no
// repository is inactive with a diagnostic; a present signature that cannot
// be read or decoded is active and says why.
func Active(root string) State {
	seat, err := Locate(root)
	if err != nil {
		return State{Diagnostic: err.Error()}
	}
	data, err := os.ReadFile(seat.Signature)
	if absent(err) {
		return State{}
	}
	record, problem := Decode(data)
	if err != nil {
		problem = fmt.Sprintf("unreadable: %v", err)
	}
	if problem != "" {
		return State{Active: true, Record: Record{By: "unknown"}, Malformed: seat.Signature + ": " + problem}
	}
	since, _ := time.Parse(time.RFC3339, record.At)
	return State{Active: true, Record: record, Since: since}
}

// Decode reads a signature's bytes; the string says why they are not one.
func Decode(data []byte) (Record, string) {
	var record Record
	if err := json.Unmarshal(data, &record); err != nil {
		return Record{}, "malformed: " + err.Error()
	}
	if _, err := time.Parse(time.RFC3339, record.At); record.Schema != 1 || strings.TrimSpace(record.By) == "" || err != nil {
		return record, fmt.Sprintf("malformed: schema %d, by %q, at %q", record.Schema, record.By, record.At)
	}
	return record, ""
}

// Write stores the signature atomically (a temporary file renamed over the
// path), mode 0600.
func Write(root string, record Record) (Seat, error) {
	seat, err := Locate(root)
	if err != nil {
		return seat, err
	}
	record.Schema = 1
	encoded, _ := json.MarshalIndent(record, "", "  ")
	_ = os.MkdirAll(seat.Dir, 0o700)
	temp, err := os.CreateTemp(seat.Dir, ".helm-*.json") // mode 0600
	if err != nil {
		return seat, err
	}
	_, writeErr := temp.Write(append(encoded, '\n'))
	if err = errors.Join(writeErr, temp.Sync(), temp.Close()); err == nil {
		err = os.Rename(temp.Name(), seat.Signature)
	}
	if err != nil {
		_ = os.Remove(temp.Name())
		return seat, err
	}
	return seat, nil
}

// Removal is what Remove found: Raw holds the signature's bytes when they
// could be read, ReadErr why not.
type Removal struct {
	Seat    Seat
	Present bool
	Raw     []byte
	ReadErr error
}

// Remove deletes the signature and nothing else. It reads the bytes first only
// so the caller can log who held the helm; a failed read never stops it.
func Remove(root string) (Removal, error) {
	seat, err := Locate(root)
	if err != nil {
		return Removal{}, err
	}
	removal := Removal{Seat: seat}
	if _, err := os.Lstat(seat.Signature); absent(err) {
		return removal, nil
	}
	removal.Present = true
	removal.Raw, removal.ReadErr = os.ReadFile(seat.Signature)
	if err := os.Remove(seat.Signature); err != nil {
		return removal, fmt.Errorf("%s cannot be removed: %v", seat.Signature, err)
	}
	return removal, nil
}
