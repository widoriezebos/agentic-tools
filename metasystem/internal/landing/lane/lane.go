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
	// CodeRecordIncomplete is a lane record an older engine wrote: it names
	// no installation or custody epoch, so a person registers it again.
	CodeRecordIncomplete = "LANDING_LANE_RECORD_INCOMPLETE"
	// CodeNotRegistered is a gated operation on a computer with no lane.
	CodeNotRegistered = "LANDING_LANE_NOT_REGISTERED"
	// CodePaused is a gated operation refused because a person paused the
	// lane (or its pause cannot be read).
	CodePaused = "LANDING_LANE_PAUSED"
	// CodeUnreachable is a lane checkout that can't be read, which is not
	// taken for gone: a volume that is not mounted, a folder that can't be
	// read.
	CodeUnreachable = "LANDING_LANE_UNREACHABLE"
	// CodeUnsetting is a join or an agent operation refused while a person
	// unsets the lane, and a registration refused until that unset ends.
	CodeUnsetting = "LANDING_LANE_UNSETTING"
	// CodeAccountUnresolved leads ResolveAccount's refusals.
	CodeAccountUnresolved = "LANE_ACCOUNT_UNRESOLVED"
)

// Record is the host's registration of its landing lane: its layout as
// landing set resolved it, and the custody epoch that registration took.
type Record struct {
	Root string `json:"root"`
	// Install is the lane's installation (Layout.Install).
	Install string `json:"install,omitempty"`
	// CustodyEpoch is the lane's claim epoch: every registration takes a
	// greater one than any before it on this computer.
	CustodyEpoch uint64 `json:"custodyEpoch,omitempty"`
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

// Resolution is the lane a caller lands through: empty Root when the host
// has none registered. FromHost says the seat named none and the host's
// record decided.
type Resolution struct {
	Root     string
	Record   Record
	FromHost bool
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

// checkoutGone reports whether path was removed from a folder that is still
// there: only then is the checkout gone. Any other failure to read it (a
// volume that is not mounted, a folder that cannot be read) is returned as
// an error, never taken for gone.
func checkoutGone(path string) (bool, error) {
	_, err := os.Stat(path)
	switch {
	case err == nil:
		return false, nil
	case !errors.Is(err, fs.ErrNotExist):
		return false, err
	}
	if info, parentErr := os.Stat(filepath.Dir(path)); parentErr != nil || !info.IsDir() {
		if parentErr == nil {
			parentErr = fmt.Errorf("%s is not a folder", filepath.Dir(path))
		}
		return false, fmt.Errorf("the folder that held it can't be reached: %w", parentErr)
	}
	return true, nil
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

// Read returns the host's lane record; false when none is registered. A
// record without its layout or custody epoch (an older engine's) is
// returned with a CodeRecordIncomplete refusal naming landing set.
func Read(home string) (Record, bool, error) {
	var record Record
	ok, err := readJSON(RecordPath(home), &record)
	if ok && record.Root == "" {
		return Record{}, false, fmt.Errorf("the landing lane record %s names no checkout; register it with metasystem landing set", RecordPath(home))
	}
	if ok && err == nil {
		if _, layoutErr := record.Layout(); layoutErr != nil || record.CustodyEpoch == 0 {
			return record, true, incompleteRefusal(record)
		}
	}
	return record, ok, err
}

func incompleteRefusal(record Record) *Refusal {
	return &Refusal{Code: CodeRecordIncomplete,
		Message: fmt.Sprintf("this computer's landing lane record for %s names no installation or custody epoch (an older engine wrote it), so nothing was done", record.Root),
		Fix:     "a person registers the lane again: metasystem landing set " + record.Root,
		Argv:    []string{"metasystem", "landing", "set", record.Root}}
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
// landing.batch-root (seatRoot, empty when unset) and the host's record.
// Only a person's landing set registers a lane (design r10 §1); a seat's
// setting never does:
//
//   - record set, seat unset or naming the same checkout: the record's root;
//   - record set, seat naming another checkout: refused, LANDING_LANE_MISMATCH;
//   - no record: no lane, whatever the seat names; the seat lands itself.
//
// A registered root that no longer exists is refused, LANDING_LANE_GONE, and
// never replaced by the seat's setting.
func Resolve(home, seatRoot string) (Resolution, error) {
	seatRoot = strings.TrimSpace(seatRoot)
	var result Resolution
	err := withLock(home, func() error {
		record, ok, err := Read(home)
		if err != nil || !ok {
			return err
		}
		result.Record = record
		if gone(record.Root) {
			return goneRefusal(record)
		}
		if seatRoot != "" && resolved(seatRoot) != resolved(record.Root) {
			return mismatchRefusal(record, resolved(seatRoot))
		}
		result.Root, result.FromHost = record.Root, seatRoot == ""
		return nil
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

// Register makes layout the host's lane at a person's word (landing set).
// The same layout again changes nothing (changed false); anything else
// writes a record with a custody epoch greater than any this computer has
// used, and returns the previous record. It is refused while an unset is
// under way. The keeper's restart count starts over with a new lane.
func Register(home string, layout Layout, by string, now time.Time) (previous Record, changed bool, err error) {
	if layout.Checkout == "" || layout.Install == "" {
		return Record{}, false, &Refusal{Code: CodeRegisterInvalid, Message: "no landing checkout was resolved; nothing was registered",
			Fix: "name the landing checkout by its full path: metasystem landing set /path/to/landing-checkout"}
	}
	err = withLock(home, func() error {
		if journal, fenced, _ := ReadUnset(home); fenced {
			return unsettingRefusal(journal)
		}
		// An unreadable or older record is replaced at a person's word.
		current, ok, readErr := Read(home)
		if ok && readErr == nil && current.Root == string(layout.Checkout) && current.Install == string(layout.Install) {
			return nil
		}
		used, err := readEpoch(home)
		if err != nil {
			return err
		}
		if ok && current.CustodyEpoch > used {
			used = current.CustodyEpoch
		}
		previous, changed = current, true
		record := Record{Root: string(layout.Checkout), Install: string(layout.Install), CustodyEpoch: used + 1, RegisteredBy: by, At: now.UTC().Format(time.RFC3339)}
		// The epoch is spent before the record names it, so a crash between
		// the two never hands the same epoch to two registrations.
		if err := writeJSON(home, epochPath(home), epochRecord{CustodyEpoch: record.CustodyEpoch}); err != nil {
			return err
		}
		if err := writeJSON(home, RecordPath(home), record); err != nil {
			return err
		}
		return removeIfPresent(keeperPath(home))
	})
	return previous, changed, err
}

// epochRecord is the greatest custody epoch this computer's lane has used;
// it outlives every unset, so no epoch is used twice.
type epochRecord struct {
	CustodyEpoch uint64 `json:"custodyEpoch"`
}

func epochPath(home string) string { return filepath.Join(HostDir(home), "landing-lane-epoch.json") }

func readEpoch(home string) (uint64, error) {
	var epoch epochRecord
	if _, err := readJSON(epochPath(home), &epoch); err != nil {
		return 0, fmt.Errorf("the landing lane's registration count can't be read, so nothing was registered: %w", err)
	}
	return epoch.CustodyEpoch, nil
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
