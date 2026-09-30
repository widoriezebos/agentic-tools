// Package lane is the host's one landing lane (batch-lane design U12): the
// record that names the landing checkout every seat of this host lands
// through, the flock that lets one batch prove at a time on the host, and the
// keeper that restarts the lane's owner when it dies. Everything lives in the
// host directory beside the board and the bridge (~/.metasystem/host); the
// caller passes the home, so tests never touch the real one.
package lane

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lock"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/realpath"
)

// The refusal codes of the lane (refusal register rows name Resolve and
// Register).
const (
	CodeMismatch        = "LANDING_LANE_MISMATCH"
	CodeGone            = "LANDING_LANE_GONE"
	CodeRegisterInvalid = "LANDING_LANE_REGISTER_INVALID"
	CodeUnarmed         = "LANDING_LANE_UNARMED"
	CodeNoMachine       = "LANDING_LANE_NO_MACHINE"
	// CodeAccountUnresolved leads ResolveAccount's refusals.
	CodeAccountUnresolved = "LANE_ACCOUNT_UNRESOLVED"
)

// Record is the host's registration of its landing lane.
type Record struct {
	Root         string `json:"root"`
	RegisteredBy string `json:"registeredBy"`
	At           string `json:"at"`
}

// Refusal is a lane refusal: what happened, and the fix a person runs.
// Argv is that fix as one command, when it is one.
type Refusal struct {
	Code, Message, Fix string
	Argv               []string
}

func (r *Refusal) Error() string { return r.Code + ": " + r.Message }

// Resolution is the lane a caller lands through: empty Root when neither the
// seat nor the host names one. Registered says this call wrote the record;
// FromHost that the seat named none and the host's record decided.
type Resolution struct {
	Root       string
	Record     Record
	Registered bool
	FromHost   bool
}

// HostDir is the host directory under home, shared with the board.
func HostDir(home string) string { return filepath.Join(home, "host") }

// RecordPath is the lane record.
func RecordPath(home string) string { return filepath.Join(HostDir(home), "landing-lane.json") }

// LockPath is the flock every lane record, keeper state and pause is written under.
func LockPath(home string) string { return filepath.Join(HostDir(home), "landing-lane.lock") }

// ProvingPath is the flock the batch that proves holds.
func ProvingPath(home string) string { return filepath.Join(HostDir(home), "landing-proving.lock") }

// gone reports whether path no longer exists.
func gone(path string) bool {
	_, err := os.Stat(path)
	return errors.Is(err, fs.ErrNotExist)
}

func resolved(path string) string { return realpath.Resolve(filepath.Clean(path)) }

// withLock runs fn under the lane flock, creating the host directory.
func withLock(home string, fn func() error) error {
	if home == "" || !filepath.IsAbs(home) {
		return fmt.Errorf("the landing lane needs an absolute home, got %q", home)
	}
	if err := os.MkdirAll(HostDir(home), 0o700); err != nil {
		return err
	}
	held, err := lock.File(LockPath(home), 0o600, lock.Exclusive)
	if err != nil {
		return err
	}
	defer held.Release()
	return fn()
}

// Read returns the host's lane record; false when none is registered.
func Read(home string) (Record, bool, error) {
	var record Record
	ok, err := readJSON(RecordPath(home), &record)
	if ok && record.Root == "" {
		return Record{}, false, fmt.Errorf("the landing lane record %s names no checkout; register it with metasystem landing set", RecordPath(home))
	}
	return record, ok, err
}

func readJSON(path string, into any) (bool, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if err := json.Unmarshal(data, into); err != nil {
		return false, fmt.Errorf("%s is unreadable: %w", path, err)
	}
	return true, nil
}

func writeJSON(home, path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	_, err = atomicfile.WriteFile(path, append(data, '\n'), 0o600, HostDir(home))
	return err
}

// Resolve decides the lane a seat lands through from its own
// landing.batch-root (seatRoot, empty when unset) and the host's record:
//
//   - seat set, record empty: the seat's root is registered (when register)
//     and used;
//   - both set and the same checkout: used;
//   - both set and different: refused, LANDING_LANE_MISMATCH;
//   - seat unset: the record's root;
//   - neither: no lane.
//
// A registered root that no longer exists is refused, LANDING_LANE_GONE, and
// never replaced by the seat's setting.
func Resolve(home, seatRoot, by string, now time.Time, register bool) (Resolution, error) {
	seatRoot = strings.TrimSpace(seatRoot)
	var result Resolution
	err := withLock(home, func() error {
		record, ok, err := Read(home)
		if err != nil {
			return err
		}
		if ok {
			result.Record = record
			if gone(record.Root) {
				return goneRefusal(record)
			}
			if seatRoot != "" && resolved(seatRoot) != resolved(record.Root) {
				return mismatchRefusal(record, resolved(seatRoot))
			}
			result.Root, result.FromHost = record.Root, seatRoot == ""
			return nil
		}
		if seatRoot == "" {
			return nil
		}
		result.Root = resolved(seatRoot)
		if !register {
			return nil
		}
		result.Record = Record{Root: result.Root, RegisteredBy: by, At: now.UTC().Format(time.RFC3339)}
		result.Registered = true
		return writeJSON(home, RecordPath(home), result.Record)
	})
	return result, err
}

func mismatchRefusal(record Record, seat string) *Refusal {
	return &Refusal{Code: CodeMismatch,
		Message: fmt.Sprintf("this seat's landing.batch-root is %s, but this host's landing lane is %s (%s); a host lands through one lane, so nothing was done", seat, record.Root, registeredText(record)),
		Fix:     fmt.Sprintf("make this seat land through the host's lane: metasystem settings set landing.batch-root %s — or, if the lane itself should move, a person runs: metasystem landing set %s", record.Root, seat)}
}

func goneRefusal(record Record) *Refusal {
	return &Refusal{Code: CodeGone,
		Message: fmt.Sprintf("this host's landing lane %s (%s) no longer exists; nothing was done and no other lane was chosen", record.Root, registeredText(record)),
		Fix:     "restore that checkout, or a person registers the lane that replaces it: metasystem landing set PATH"}
}

func registeredText(record Record) string {
	text := "registered by " + record.RegisteredBy
	if at, err := time.Parse(time.RFC3339, record.At); err == nil {
		text += " at " + at.Local().Format("2006-01-02 15:04 MST")
	}
	return text
}

// Register makes root the host's lane at a person's word. The same checkout
// again changes nothing (changed false); a move returns the previous record.
// The keeper's restart count starts over with a new lane.
func Register(home, root, by string, now time.Time) (previous Record, changed bool, err error) {
	root = strings.TrimSpace(root)
	if !filepath.IsAbs(root) {
		return Record{}, false, &Refusal{Code: CodeRegisterInvalid, Message: fmt.Sprintf("the landing lane must be an absolute path, got %q; nothing was registered", root),
			Fix: "name the landing checkout by its full path: metasystem landing set /path/to/landing-checkout"}
	}
	if info, statErr := os.Stat(root); statErr != nil || !info.IsDir() {
		return Record{}, false, &Refusal{Code: CodeRegisterInvalid, Message: fmt.Sprintf("%s is not an existing directory; nothing was registered", root),
			Fix: "create or restore the landing checkout first, then run metasystem landing set again"}
	}
	root = resolved(root)
	err = withLock(home, func() error {
		// An unreadable record is replaced at a person's word.
		current, ok, _ := Read(home)
		if ok && resolved(current.Root) == root {
			return nil
		}
		previous, changed = current, true
		if err := writeJSON(home, RecordPath(home), Record{Root: root, RegisteredBy: by, At: now.UTC().Format(time.RFC3339)}); err != nil {
			return err
		}
		return removeIfPresent(keeperPath(home))
	})
	return previous, changed, err
}

func removeIfPresent(path string) error {
	if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return nil
}

// HoldProving takes the host's proving flock for the life of the calling
// process, waiting while another proof holds it: the process that runs a
// batch's proof or diagnostic holds it, never the owner that launched it, so
// an owner's pause, restart or lane move cannot strand it. The kernel
// releases it when the process ends; release gives it back sooner.
func HoldProving(home string) (release func() error, err error) {
	if home == "" || !filepath.IsAbs(home) {
		return nil, fmt.Errorf("the proving lock needs an absolute home, got %q", home)
	}
	if err := os.MkdirAll(HostDir(home), 0o700); err != nil {
		return nil, err
	}
	held, err := lock.File(ProvingPath(home), 0o600, lock.Exclusive)
	if err != nil {
		return nil, err
	}
	// The holder's pid, written into the lock file it keeps open: the file
	// is never removed, so its inode stays the same across holders.
	if file := held.File(); file.Truncate(0) == nil {
		_, _ = file.WriteAt([]byte(strconv.Itoa(os.Getpid())+"\n"), 0)
	}
	return held.Release, nil
}

// ProbeProving tests the host's proving flock without keeping it: busy names
// the proof that holds it ("pid N"). The owner probes before it starts a
// batch, so a batch keeps collecting while another proves; a probe that
// races a start only makes the second proof wait for the first.
func ProbeProving(home string) (holder string, busy bool, err error) {
	if home == "" || !filepath.IsAbs(home) {
		return "", false, fmt.Errorf("the proving lock needs an absolute home, got %q", home)
	}
	if err := os.MkdirAll(HostDir(home), 0o700); err != nil {
		return "", false, err
	}
	held, err := lock.File(ProvingPath(home), 0o600, lock.TryExclusive)
	if err != nil {
		if lock.Busy(err) {
			return ProvingHolder(home), true, nil
		}
		return "", false, err
	}
	return "", false, held.Release()
}

// ProvingHolder names the proving flock's last holder from the pid it wrote.
func ProvingHolder(home string) string {
	data, err := os.ReadFile(ProvingPath(home))
	if pid := strings.TrimSpace(string(data)); err == nil && pid != "" {
		return "pid " + pid
	}
	return "another landing owner"
}
