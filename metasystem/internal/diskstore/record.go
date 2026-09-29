// Package diskstore owns the vocabulary and the records of every store the
// engine writes to disk (design engine-owns-disk-lifetimes, Part B, 3.1).
//
// A store is a directory the engine writes. Its record is
// <registry>/<id>.json, written before the store receives a byte and changed
// only under <registry>/<id>.record-lock with the record reloaded from disk
// first. Ownership lives in the record, never in a git working tree: a
// non-git store carries the marker file .metasystem-store naming its id and
// path; a git worktree store carries nothing inside the tree and is known by
// its recorded path, the gitdir its .git file names and the inode of that
// .git file.
//
// Nothing in this package reads the wall clock: every instant is a
// parameter, so a sweeper and its tests share one artificial clock.
package diskstore

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lock"
)

// Schema is the record schema this package writes and reads.
const Schema = "metasystem.diskstore/1"

// MarkerName is the file inside a non-git store that names its record.
const MarkerName = ".metasystem-store"

// State is a record's place in its lifetime.
type State string

const (
	// StateReserved: the record exists, the store may be partly created.
	StateReserved State = "reserved"
	// StateAccepted: the store is complete and in use by its owner.
	StateAccepted State = "accepted"
	// StateReleasing: written durably before the first unlink of a release
	// that may be interrupted; its content proofs are already established.
	StateReleasing State = "releasing"
	// StateReleased: the store is gone; the record is history.
	StateReleased State = "released"
)

// Lifetime says what ends a store (3.1, 3.12).
type Lifetime string

const (
	LifetimeOwner       Lifetime = "owner"
	LifetimeCache       Lifetime = "cache"
	LifetimeEvidence    Lifetime = "evidence"
	LifetimeRebuildable Lifetime = "rebuildable"
)

// CapKind says what a store's cap means.
type CapKind string

const (
	CapHard   CapKind = "hard"
	CapTarget CapKind = "target"
	CapNone   CapKind = "none"
)

// Owner names who a store belongs to: an owner kind and its reference (an
// encoded process ref, a job id, a goal id, a session name, a launch id, an
// attempt id, the installation, or "machine").
type Owner struct {
	Kind OwnerKind `json:"kind"`
	Ref  string    `json:"ref"`
}

// Identity is how a store is recognised at apply time. A non-git store has
// Marker set; a git worktree store has Gitdir and the device and inode of
// its .git file.
type Identity struct {
	Marker        bool   `json:"marker,omitempty"`
	Gitdir        string `json:"gitdir,omitempty"`
	GitFileDevice uint64 `json:"gitFileDevice,omitempty"`
	GitFileInode  uint64 `json:"gitFileInode,omitempty"`
	// GitFileGeneration is the .git file's inode generation where the
	// file system reports one (Linux: ext4, btrfs, xfs), which a file
	// created at a freed inode number does not share; 0 where it does not.
	GitFileGeneration uint64 `json:"gitFileGeneration,omitempty"`
}

// RebuildFrom is the commit and the command that rebuild a rebuildable
// store; only its producer declares it.
type RebuildFrom struct {
	Commit  string `json:"commit"`
	Command string `json:"command"`
}

// Discard is a person's authorised discard of a store's uncaptured work.
type Discard struct {
	By     string    `json:"by"`
	At     time.Time `json:"at"`
	Reason string    `json:"reason,omitempty"`
}

// Record is one store's registration (3.1's field list).
type Record struct {
	Schema            string       `json:"schema"`
	ID                string       `json:"id"`
	Path              string       `json:"path"`
	Identity          Identity     `json:"identity"`
	Class             string       `json:"class"`
	Owner             Owner        `json:"owner"`
	Checkout          string       `json:"checkout,omitempty"`
	Lifetime          Lifetime     `json:"lifetime"`
	RebuildFrom       *RebuildFrom `json:"rebuildFrom,omitempty"`
	CapBytes          int64        `json:"capBytes,omitempty"`
	CapKind           CapKind      `json:"capKind"`
	State             State        `json:"state"`
	Adopted           bool         `json:"adopted,omitempty"`
	Created           time.Time    `json:"created"`
	Bytes             int64        `json:"bytes,omitempty"`
	MeasuredAt        *time.Time   `json:"measuredAt,omitempty"`
	ReleasedBy        string       `json:"releasedBy,omitempty"`
	Notes             []string     `json:"notes,omitempty"`
	Layout            string       `json:"layout,omitempty"`
	Reservation       string       `json:"reservation,omitempty"`
	CopyOf            string       `json:"copyOf,omitempty"`
	AuthorizedDiscard *Discard     `json:"authorizedDiscard,omitempty"`
}

// Registry is one directory of records: the machine registry
// (~/.metasystem/stores) or a checkout registry
// (<control>/artifacts/agents/stores).
type Registry struct {
	Dir string
	// lockAcquired runs each time a record lock is taken; tests stand a
	// fork's duplicate descriptor in at that moment. Nil in production.
	lockAcquired func(*os.File)
}

// MachineRegistry is the registry of stores that belong to the machine or to
// a process, under the home state root (~/.metasystem).
func MachineRegistry(homeStateRoot string) Registry {
	return Registry{Dir: filepath.Join(homeStateRoot, "stores")}
}

// CheckoutRegistry is the registry of a checkout's runs, jobs, goals and
// seats, under its control root.
func CheckoutRegistry(control string) Registry {
	return Registry{Dir: filepath.Join(control, "artifacts", "agents", "stores")}
}

// RecordPath is where a record lives.
func (r Registry) RecordPath(id string) string { return filepath.Join(r.Dir, id+".json") }

// LockPath is the record's lock file.
func (r Registry) LockPath(id string) string { return filepath.Join(r.Dir, id+".record-lock") }

// ErrNotFound reports a record that does not exist.
var ErrNotFound = errors.New("no such store record")

// Registration is what a producer declares before its store receives bytes.
type Registration struct {
	Path        string
	Git         bool // a linked git worktree: identity from its .git file
	Class       string
	Owner       Owner
	Checkout    string
	Lifetime    Lifetime
	RebuildFrom *RebuildFrom
	CapBytes    int64
	CapKind     CapKind
	Layout      string
	Reservation string
	CopyOf      string
	Adopted     bool
	Notes       []string
}

func validID(id string) bool {
	if len(id) != 26 {
		return false
	}
	return strings.IndexFunc(id, func(r rune) bool { return !strings.ContainsRune(ulidAlphabet, r) }) < 0
}

func validateRegistration(reg Registration) error {
	switch {
	case !filepath.IsAbs(reg.Path) || filepath.Clean(reg.Path) != reg.Path:
		return fmt.Errorf("a store path must be absolute and clean: %q", reg.Path)
	case !KnownOwnerKind(reg.Owner.Kind) || strings.TrimSpace(reg.Owner.Ref) == "":
		return fmt.Errorf("a store needs a known owner kind and a reference: %q %q", reg.Owner.Kind, reg.Owner.Ref)
	case strings.TrimSpace(reg.Class) == "":
		return fmt.Errorf("a store needs a class")
	}
	switch reg.Lifetime {
	case LifetimeOwner, LifetimeCache, LifetimeEvidence:
	case LifetimeRebuildable:
		if reg.RebuildFrom == nil || reg.RebuildFrom.Commit == "" || reg.RebuildFrom.Command == "" {
			return fmt.Errorf("a rebuildable store needs the commit and command that rebuild it")
		}
	default:
		return fmt.Errorf("unknown store lifetime %q", reg.Lifetime)
	}
	switch reg.CapKind {
	case CapHard, CapTarget, CapNone:
	default:
		return fmt.Errorf("unknown cap kind %q", reg.CapKind)
	}
	return nil
}

// Register records a store before it receives bytes. It is idempotent:
// registering a path that already has an unreleased record with the same
// owner returns that record and writes nothing. The same path under another
// owner is refused. A git registration reads the worktree's .git file for
// its identity and writes nothing inside the tree.
func (r Registry) Register(reg Registration, now time.Time, entropy io.Reader) (Record, error) {
	if err := validateRegistration(reg); err != nil {
		return Record{}, err
	}
	var identity Identity
	if reg.Git {
		read, err := ReadGitIdentity(reg.Path)
		if err != nil {
			return Record{}, err
		}
		identity = read
	} else {
		identity.Marker = true
	}
	if err := os.MkdirAll(r.Dir, 0o700); err != nil {
		return Record{}, fmt.Errorf("store registry %s: %w", r.Dir, err)
	}
	held, err := lock.File(filepath.Join(r.Dir, ".register.lock"), 0o600, lock.Exclusive)
	if err != nil {
		return Record{}, fmt.Errorf("store registry lock %s: %w", r.Dir, err)
	}
	defer held.Release()
	records, _ := r.Inventory()
	for _, existing := range records {
		if existing.Path != reg.Path || existing.State == StateReleased {
			continue
		}
		if existing.Owner != reg.Owner {
			return Record{}, fmt.Errorf("store %s is already registered to %s %s (record %s)", reg.Path, existing.Owner.Kind, existing.Owner.Ref, existing.ID)
		}
		return existing, nil
	}
	id, err := NewID(now, entropy)
	if err != nil {
		return Record{}, err
	}
	record := Record{
		Schema: Schema, ID: id, Path: reg.Path, Identity: identity, Class: reg.Class, Owner: reg.Owner,
		Checkout: reg.Checkout, Lifetime: reg.Lifetime, RebuildFrom: reg.RebuildFrom, CapBytes: reg.CapBytes,
		CapKind: reg.CapKind, State: StateReserved, Adopted: reg.Adopted, Created: now.UTC(),
		Notes: reg.Notes, Layout: reg.Layout, Reservation: reg.Reservation, CopyOf: reg.CopyOf,
	}
	// The lock file exists before the record, so no later reader, prober or
	// entrant ever has to create it.
	lockFile, err := os.OpenFile(r.LockPath(id), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return Record{}, fmt.Errorf("store record lock: %w", err)
	}
	_ = lockFile.Close()
	if err := r.write(record); err != nil {
		return Record{}, err
	}
	return record, nil
}

// write publishes a record durably; a doubtful publication is an error, so
// no caller puts bytes into a store whose record may not survive a crash.
func (r Registry) write(record Record) error {
	data, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return err
	}
	durable, err := atomicfile.WriteFile(r.RecordPath(record.ID), append(data, '\n'), 0o600, filepath.Dir(r.Dir))
	if err != nil {
		return fmt.Errorf("write store record %s: %w", record.ID, err)
	}
	if !durable {
		return fmt.Errorf("store record %s is published but its durability is unconfirmed", record.ID)
	}
	return nil
}

// Load reads one record.
func (r Registry) Load(id string) (Record, error) {
	if !validID(id) {
		return Record{}, fmt.Errorf("%q is not a store record id", id)
	}
	data, err := os.ReadFile(r.RecordPath(id))
	if errors.Is(err, os.ErrNotExist) {
		return Record{}, ErrNotFound
	}
	if err != nil {
		return Record{}, err
	}
	var record Record
	if err := json.Unmarshal(data, &record); err != nil {
		return Record{}, fmt.Errorf("store record %s is unreadable: %w", id, err)
	}
	if record.Schema != Schema || record.ID != id {
		return Record{}, fmt.Errorf("store record %s has schema %q and id %q", id, record.Schema, record.ID)
	}
	return record, nil
}

// Unreadable is a record file that could not be read; the sweeper reports it
// as pending, never deletes it.
type Unreadable struct {
	Path   string
	Reason string
}

// Inventory lists every record of the registry. It observes only: an absent
// registry is empty, and nothing is created or adopted.
func (r Registry) Inventory() ([]Record, []Unreadable) {
	entries, err := os.ReadDir(r.Dir)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, []Unreadable{{Path: r.Dir, Reason: err.Error()}}
	}
	var records []Record
	var unreadable []Unreadable
	for _, entry := range entries {
		id, ok := strings.CutSuffix(entry.Name(), ".json")
		if !ok || !validID(id) || !entry.Type().IsRegular() {
			continue
		}
		record, err := r.Load(id)
		if err != nil {
			unreadable = append(unreadable, Unreadable{Path: r.RecordPath(id), Reason: err.Error()})
			continue
		}
		records = append(records, record)
	}
	sort.Slice(records, func(i, j int) bool { return records[i].ID < records[j].ID })
	return records, unreadable
}

// WriteMarker writes the marker of a non-git store that exists as an empty
// or partly created directory. It never replaces a marker of another record.
func WriteMarker(record Record) error {
	if !record.Identity.Marker {
		return fmt.Errorf("store %s is a git worktree; it carries no marker", record.Path)
	}
	path := filepath.Join(record.Path, MarkerName)
	if existing, err := readMarker(record.Path); err == nil {
		if existing.ID == record.ID && existing.Path == record.Path {
			return nil
		}
		return fmt.Errorf("store %s carries the marker of record %s", record.Path, existing.ID)
	}
	data, err := json.Marshal(marker{Schema: Schema, ID: record.ID, Path: record.Path})
	if err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("store marker %s: %w", path, err)
	}
	_, writeErr := file.Write(append(data, '\n'))
	return errors.Join(writeErr, file.Sync(), file.Close())
}

type marker struct {
	Schema string `json:"schema"`
	ID     string `json:"id"`
	Path   string `json:"path"`
}

func readMarker(store string) (marker, error) {
	path := filepath.Join(store, MarkerName)
	info, err := os.Lstat(path)
	if err != nil {
		return marker{}, err
	}
	if !info.Mode().IsRegular() {
		return marker{}, fmt.Errorf("store marker %s is not a regular file", path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return marker{}, err
	}
	var found marker
	if err := json.Unmarshal(data, &found); err != nil || found.Schema != Schema {
		return marker{}, fmt.Errorf("store marker %s is foreign or unreadable", path)
	}
	return found, nil
}
