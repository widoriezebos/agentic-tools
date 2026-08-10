package supervise

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

// RealCheckout implements Checkout against the actual supervision
// surface of one checkout: the lock directory, the state file, and
// the shutdown-intent channel. Every read is three-way (D-1): only a
// definitive negative is Absent; any other failure is Indeterminate,
// and Indeterminate never authorizes anything.
type RealCheckout struct {
	// Root is the checkout root the owner supervises.
	Root string
	// Self is this owner's identity: pid, start second, instance tag.
	Self identity.Ref
	// SelfTag is the owner's instance tag (identity.Ref carries no tag).
	SelfTag string
	// IntervalSec and the component identities are what PublishState
	// writes; the shell watcher and dispatch gate read this schema
	// today, so the fields must stay compatible (a Phase 0 seam by
	// design: the state schema is the shell system's).
	IntervalSec int
	Fingerprint string
	WatcherCap  int
	// generation counts publications the way the shell owner counts
	// launch_set generations; the owner loop sets it per relaunch.
	Generation int64

	// clock is injectable for tests; nil means time.Now.
	clock func() time.Time
}

func (c *RealCheckout) now() time.Time {
	if c.clock != nil {
		return c.clock()
	}
	return time.Now()
}

func (c *RealCheckout) supervisionDir() string {
	return filepath.Join(c.Root, "artifacts", "agents", "supervision")
}

func (c *RealCheckout) ownerFile() string {
	return filepath.Join(c.supervisionDir(), "lock.d", "owner.json")
}

func (c *RealCheckout) statePath() string {
	return filepath.Join(c.supervisionDir(), "state.json")
}

// threeWayStat classifies a path per D-1: Present, definitively
// Absent (ENOENT on the file or any parent), or Indeterminate.
func threeWayStat(path string) FileState {
	_, err := os.Stat(path)
	switch {
	case err == nil:
		return Present
	case errors.Is(err, os.ErrNotExist):
		return Absent
	default:
		return Indeterminate
	}
}

func (c *RealCheckout) RootState() FileState { return threeWayStat(c.Root) }

func (c *RealCheckout) StateFileState() FileState { return threeWayStat(c.statePath()) }

// ownerRecord is the checkout lock's owner file schema — the same
// bytes the shell system writes and reads today.
type ownerRecord struct {
	Pid          int64  `json:"pid"`
	PidStartedAt int64  `json:"pidStartedAt"`
	InstanceTag  string `json:"instanceTag"`
}

func (c *RealCheckout) Currency() CurrencyState {
	content, err := os.ReadFile(c.ownerFile())
	if errors.Is(err, os.ErrNotExist) {
		return NoLock
	}
	if err != nil {
		return Unreadable
	}
	var record ownerRecord
	if json.Unmarshal(content, &record) != nil {
		return Unreadable // uninspectable is alive: never a definitive answer
	}
	if record.Pid == c.Self.Pid && record.PidStartedAt == c.Self.StartedAtSec && record.InstanceTag == c.SelfTag {
		return NamesSelf
	}
	return NamesOther
}

// stateDocument is the published supervision state — schema-compatible
// with the shell watcher, dispatch's census gate, and verify_armed.
type stateDocument struct {
	SchemaVersion        int                       `json:"schemaVersion"`
	Owner                stateIdentity             `json:"owner"`
	Components           map[string]stateComponent `json:"components"`
	IntervalSec          int                       `json:"intervalSec"`
	Generation           int64                     `json:"generation"`
	Fingerprint          string                    `json:"fingerprint"`
	DerivedWatcherCapMin int                       `json:"derivedWatcherCapMin"`
	StartedAt            string                    `json:"startedAt"`
	Engine               string                    `json:"engine"`
	EngineBuild          string                    `json:"engineBuild,omitempty"`
}

type stateIdentity struct {
	Pid          int64  `json:"pid"`
	PidStartedAt int64  `json:"pidStartedAt"`
	InstanceTag  string `json:"instanceTag"`
}

type stateComponent struct {
	Pid          int64  `json:"pid"`
	PidStartedAt int64  `json:"pidStartedAt"`
	InstanceTag  string `json:"instanceTag"`
	Heartbeat    string `json:"heartbeat"`
}

// BuildStamp is set by the linker at build time and stamped into
// every artifact this engine writes (GO-MIG-R4-009: execution is
// attested by the workload's own artifacts, not installation
// paperwork).
var BuildStamp = "dev"

func (c *RealCheckout) StateNamesSelf() (bool, error) {
	content, err := os.ReadFile(c.statePath())
	if err != nil {
		return false, err
	}
	var document stateDocument
	if err := json.Unmarshal(content, &document); err != nil {
		return false, err
	}
	owner := document.Owner
	return owner.Pid == c.Self.Pid && owner.PidStartedAt == c.Self.StartedAtSec && owner.InstanceTag == c.SelfTag, nil
}

// PublishState writes state.json atomically, FENCED (SLC-R4-001): the
// lock's owner file is re-read immediately before the rename, and a
// lock that no longer names this owner aborts the publication — the
// caller's next cycle classifies the supersession.
func (c *RealCheckout) PublishState(held []Held) error {
	components := map[string]stateComponent{}
	for _, member := range held {
		if member.Generation != c.Generation {
			continue
		}
		components[string(member.Component)] = stateComponent{
			Pid:          member.Identity.Pid,
			PidStartedAt: member.Identity.StartedAtSec,
			InstanceTag:  member.Tag,
			Heartbeat:    filepath.Join(c.supervisionDir(), string(member.Component)+".heartbeat.json"),
		}
	}
	document := stateDocument{
		SchemaVersion:        1,
		Owner:                stateIdentity{Pid: c.Self.Pid, PidStartedAt: c.Self.StartedAtSec, InstanceTag: c.SelfTag},
		Components:           components,
		IntervalSec:          c.IntervalSec,
		Generation:           c.Generation,
		Fingerprint:          c.Fingerprint,
		DerivedWatcherCapMin: c.WatcherCap,
		StartedAt:            c.now().UTC().Format("2006-01-02T15:04:05Z"),
		Engine:               "go",
		EngineBuild:          BuildStamp,
	}
	encoded, err := json.MarshalIndent(document, "", "  ")
	if err != nil {
		return fmt.Errorf("state encoding: %w", err)
	}
	temporary, err := os.CreateTemp(c.supervisionDir(), ".state-*")
	if err != nil {
		return fmt.Errorf("state temp file: %w", err)
	}
	defer os.Remove(temporary.Name())
	if _, err := temporary.Write(append(encoded, '\n')); err != nil {
		temporary.Close()
		return fmt.Errorf("state write: %w", err)
	}
	if err := temporary.Sync(); err != nil {
		temporary.Close()
		return fmt.Errorf("state sync: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("state close: %w", err)
	}
	// The fence: currency re-checked as close to the rename as the
	// filesystem allows.
	if c.Currency() != NamesSelf {
		return fmt.Errorf("publication aborted: the lock no longer names this owner (SLC-R4-001)")
	}
	if err := os.Rename(temporary.Name(), c.statePath()); err != nil {
		return fmt.Errorf("state rename: %w", err)
	}
	return nil
}

// RealIntents implements the shutdown-intent channel (D-1,
// SLC-R7-009, SLC-R9-004): latch at exit initiation, consume on read,
// honor only a fresh intent naming this owner.
type RealIntents struct {
	Root    string
	Self    identity.Ref
	SelfTag string
	// LatchWindow is D-6's owner-stop wait: the intent is honored
	// only if younger than this at latch time.
	LatchWindow time.Duration
	clock       func() time.Time
}

type intentRecord struct {
	TargetPid          int64  `json:"targetPid"`
	TargetPidStartedAt int64  `json:"targetPidStartedAt"`
	TargetInstanceTag  string `json:"targetInstanceTag"`
	Requester          string `json:"requester"`
	WrittenAt          string `json:"writtenAt"`
}

func (i *RealIntents) now() time.Time {
	if i.clock != nil {
		return i.clock()
	}
	return time.Now()
}

func (i *RealIntents) intentPath() string {
	return filepath.Join(i.Root, "artifacts", "agents", "supervision", "lock.d", "shutdown-intent.json")
}

// LatchShutdown reads and CONSUMES the intent. True only when the
// intent names exactly this owner and is fresh; stale or foreign
// intents are consumed, reported on stderr, and not honored.
func (i *RealIntents) LatchShutdown() bool {
	path := i.intentPath()
	content, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	defer os.Remove(path) // consumed either way
	var record intentRecord
	if json.Unmarshal(content, &record) != nil {
		fmt.Fprintln(os.Stderr, "shutdown intent unparseable; ignored and consumed")
		return false
	}
	if record.TargetPid != i.Self.Pid || record.TargetPidStartedAt != i.Self.StartedAtSec || record.TargetInstanceTag != i.SelfTag {
		fmt.Fprintf(os.Stderr, "shutdown intent names another identity (pid %d); ignored and consumed\n", record.TargetPid)
		return false
	}
	written, err := time.Parse(time.RFC3339, record.WrittenAt)
	if err != nil || i.now().Sub(written) > i.LatchWindow {
		fmt.Fprintln(os.Stderr, "shutdown intent stale; reported and not honored (SLC-R8-005)")
		return false
	}
	return true
}
