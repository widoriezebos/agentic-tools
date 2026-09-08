// Package stopfence owns the per-checkout process-creation fence.
package stopfence

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lock"
)

const SchemaVersion = 1

const (
	StateOpen   = "open"
	StateClosed = "closed"

	PhaseArmed          = "armed"
	PhaseStopping       = "stopping"
	PhaseStopped        = "stopped"
	PhaseStopIncomplete = "stop-incomplete"
)

// Process identifies one process without relying on a reusable pid alone.
type Process struct {
	Pid               int64  `json:"pid"`
	PidStartedAt      int64  `json:"pidStartedAt"`
	PidStartedAtMicro int64  `json:"pidStartedAtMicro,omitempty"`
	PidStartTicks     int64  `json:"pidStartTicks,omitempty"`
	BootID            string `json:"bootId,omitempty"`
}

func ProcessFromRef(ref identity.Ref) Process {
	return Process{Pid: ref.Pid, PidStartedAt: ref.StartedAtSec, PidStartedAtMicro: ref.StartedAtUnixMicro, PidStartTicks: ref.StartTicks, BootID: ref.BootID}
}

func (p Process) Ref() identity.Ref {
	return identity.Ref{Pid: p.Pid, StartedAtSec: p.PidStartedAt, StartedAtUnixMicro: p.PidStartedAtMicro, StartTicks: p.PidStartTicks, BootID: p.BootID}
}

// Actor records which transition last changed the fence.
type Actor struct {
	Verb string `json:"verb"`
	Process
}

// Survivor is one durable reason a stop could not claim completion. Process
// survivors carry their identity; inventory and record failures name the
// affected family or file without inventing a process.
type Survivor struct {
	Component    string `json:"component"`
	ID           string `json:"id,omitempty"`
	Path         string `json:"path,omitempty"`
	MachineID    string `json:"machineId,omitempty"`
	Pid          int64  `json:"pid,omitempty"`
	PidStartedAt int64  `json:"pidStartedAt,omitempty"`
	Tag          string `json:"tag,omitempty"`
	Reason       string `json:"reason"`
}

// Record is the durable state of one checkout's process-creation fence.
type Record struct {
	SchemaVersion int        `json:"schemaVersion"`
	State         string     `json:"state"`
	Phase         string     `json:"phase"`
	Generation    int64      `json:"generation"`
	ChangedAt     string     `json:"changedAt"`
	By            Actor      `json:"by"`
	Checkout      string     `json:"checkout"`
	NotStopped    []Survivor `json:"notStopped"`
}

// RecordUnreadableError keeps the generation that can still be established
// from an otherwise unreadable fence so the human arm path can replace it
// without moving the generation backwards.
type RecordUnreadableError struct {
	Reason            string
	HighestGeneration int64
}

func (e *RecordUnreadableError) Error() string { return e.Reason }

// CreationClaim joins an in-flight creator to a stop that closes its generation.
type CreationClaim struct {
	SchemaVersion int     `json:"schemaVersion"`
	Verb          string  `json:"verb"`
	Generation    int64   `json:"generation"`
	Creator       Process `json:"creator"`
	OpenedAt      string  `json:"openedAt"`
	Path          string  `json:"-"`
}

// ClaimProblem names one file that could not be reduced to a creation claim.
type ClaimProblem struct {
	Path string
	Err  error
}

type Claim struct{ path string }

func TransitionPath(root string) string {
	return filepath.Join(root, "artifacts", "agents", "supervision", "transition.json")
}

func LockPath(root string) string {
	return filepath.Join(root, "artifacts", "agents", "supervision", "transition.lock.d")
}

func CreatingDir(root string) string {
	return filepath.Join(root, "artifacts", "agents", "supervision", "creating")
}

// Read returns an implicit open generation-zero record when no fence was written.
func Read(root string) (Record, error) {
	data, err := os.ReadFile(TransitionPath(root))
	if errors.Is(err, os.ErrNotExist) {
		return Record{SchemaVersion: SchemaVersion, State: StateOpen, Phase: PhaseArmed, NotStopped: []Survivor{}}, nil
	}
	if err != nil {
		return Record{}, unreadableRecordError(err.Error(), nil)
	}
	var record Record
	if err := json.Unmarshal(data, &record); err != nil {
		return Record{}, unreadableRecordError(fmt.Sprintf("stop fence is unreadable: %v", err), data)
	}
	if err := validateRecord(record); err != nil {
		return Record{}, unreadableRecordError(err.Error(), data)
	}
	return record, nil
}

func unreadableRecordError(reason string, data []byte) error {
	highest := int64(0)
	var envelope struct {
		Generation *int64 `json:"generation"`
	}
	if len(data) > 0 && json.Unmarshal(data, &envelope) == nil && envelope.Generation != nil && *envelope.Generation >= 0 {
		highest = *envelope.Generation
	}
	return &RecordUnreadableError{Reason: reason, HighestGeneration: highest}
}

func validateRecord(record Record) error {
	if record.SchemaVersion != SchemaVersion {
		return fmt.Errorf("stop fence schema version %d is unsupported", record.SchemaVersion)
	}
	if record.State != StateOpen && record.State != StateClosed {
		return fmt.Errorf("stop fence state %q is invalid", record.State)
	}
	validPhase := record.State == StateOpen && record.Phase == PhaseArmed
	validPhase = validPhase || record.State == StateClosed && (record.Phase == PhaseStopping || record.Phase == PhaseStopped || record.Phase == PhaseStopIncomplete)
	if !validPhase || record.Generation < 0 {
		return fmt.Errorf("stop fence phase %q is invalid for state %q", record.Phase, record.State)
	}
	return nil
}

// Closed reads the fence once and returns its complete record.
func Closed(root string) (bool, Record, error) {
	record, err := Read(root)
	return record.State == StateClosed, record, err
}

// Publish atomically publishes a validated transition record and preserves the
// durable writer's two outcomes: durable is false with a nil error only when
// the replacement committed but its post-rename directory sync failed.
func Publish(root string, record Record) (durable bool, err error) {
	record.SchemaVersion = SchemaVersion
	if record.NotStopped == nil {
		record.NotStopped = []Survivor{}
	}
	if err := validateRecord(record); err != nil {
		return false, err
	}
	data, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return false, err
	}
	return atomicfile.WriteText(TransitionPath(root), string(append(data, '\n')), root)
}

// Write retains the error-only interface used by readers and fixtures that do
// not make a crash-durability claim. A committed replacement is success even
// when Publish reports that its durability could not be confirmed.
func Write(root string, record Record) error {
	_, err := Publish(root, record)
	return err
}

// Completed reports the only durable shape that proves a completed stop.
func Completed(record Record) bool {
	return record.State == StateClosed && record.Phase == PhaseStopped && len(record.NotStopped) == 0
}

// ClosedDescription renders the phase-sensitive first line shared by every
// process-creation refusal. Callers retain ownership of any terminal qualifier.
func ClosedDescription(record Record, checkout string) (string, error) {
	if record.State != StateClosed {
		return "", fmt.Errorf("closed-fence description requires a closed record, got state %q", record.State)
	}
	if record.Checkout != "" {
		checkout = record.Checkout
	}
	switch {
	case Completed(record):
		return fmt.Sprintf("the metasystem is stopped for %s since %s, by %s pid %d", checkout, record.ChangedAt, record.By.Verb, record.By.Pid), nil
	case record.Phase == PhaseStopping:
		return fmt.Sprintf("stop unfinished for %s since %s by %s pid %d", checkout, record.ChangedAt, record.By.Verb, record.By.Pid), nil
	default:
		return fmt.Sprintf("stop incomplete for %s since %s by %s pid %d; %d unresolved entries from the last stop", checkout, record.ChangedAt, record.By.Verb, record.By.Pid, len(record.NotStopped)), nil
	}
}

// ClosedCommand names the transition that can actually change the described
// state. Only a completed stop points at arm; unfinished and incomplete stops
// point at a fresh stop.
func ClosedCommand(record Record, checkout string) (string, error) {
	if record.State != StateClosed {
		return "", fmt.Errorf("closed-fence remedy requires a closed record, got state %q", record.State)
	}
	if record.Checkout != "" {
		checkout = record.Checkout
	}
	if Completed(record) {
		return "metasystem arm --repo " + checkout, nil
	}
	return "metasystem stop --repo " + checkout, nil
}

// HealthPrefix keeps health's closed-fence vocabulary aligned with the
// durable phase without changing its machine-readable stopped outcome.
func HealthPrefix(record Record) (string, error) {
	if record.State != StateClosed {
		return "", fmt.Errorf("closed-fence health prefix requires a closed record, got state %q", record.State)
	}
	if Completed(record) {
		return "HEALTH STOPPED ", nil
	}
	if record.Phase == PhaseStopping {
		return "HEALTH STOP UNFINISHED ", nil
	}
	return "HEALTH STOP INCOMPLETE ", nil
}

// Acquire serializes stop and arm transitions for one checkout.
func Acquire(root, verb string, ref identity.Ref, scaleMilli int) (*lock.Lock, error) {
	if scaleMilli < 1 {
		scaleMilli = 1000
	}
	wait := time.Duration((int64(10*time.Second)*int64(scaleMilli) + 999) / 1000)
	if wait < 10*time.Millisecond {
		wait = 10 * time.Millisecond
	}
	self := lock.Identity{Pid: ref.Pid, PidStartedAt: ref.StartedAtSec, Label: verb}
	return lock.Acquire(LockPath(root), self, lock.Options{
		Wait: wait, Poll: 25 * time.Millisecond,
		Probe: func(holder lock.Identity) lock.Liveness {
			state := identity.AliveRef(identity.KernelProber{}, identity.Ref{Pid: holder.Pid, StartedAtSec: holder.PidStartedAt})
			switch state {
			case identity.Alive:
				return lock.Alive
			case identity.Dead:
				return lock.Dead
			default:
				return lock.Unknown
			}
		},
	})
}

// Creating publishes an in-flight creation claim before any record or child exists.
func Creating(root, verb string, generation int64, ref identity.Ref) (*Claim, error) {
	if verb == "" || strings.ContainsAny(verb, `/\\`) || ref.Pid <= 0 || generation < 0 {
		return nil, errors.New("invalid creation claim")
	}
	nonceBytes := make([]byte, 8)
	if _, err := rand.Read(nonceBytes); err != nil {
		return nil, err
	}
	name := fmt.Sprintf("%s-%d-%s.json", verb, ref.Pid, hex.EncodeToString(nonceBytes))
	path := filepath.Join(CreatingDir(root), name)
	record := CreationClaim{SchemaVersion: SchemaVersion, Verb: verb, Generation: generation, Creator: ProcessFromRef(ref), OpenedAt: time.Now().UTC().Format(time.RFC3339)}
	data, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return nil, err
	}
	if _, err := atomicfile.WriteText(path, string(append(data, '\n')), root); err != nil {
		return nil, err
	}
	return &Claim{path: path}, nil
}

func (c *Claim) Path() string {
	if c == nil {
		return ""
	}
	return c.path
}

func (c *Claim) Close() error {
	if c == nil || c.path == "" {
		return nil
	}
	err := os.Remove(c.path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

// CloseClaim closes only a structurally valid claim file, never an arbitrary path.
func CloseClaim(path string) error {
	claim, err := readClaim(path)
	if err != nil {
		return err
	}
	if claim.Path != filepath.Clean(path) {
		return errors.New("creation claim path is invalid")
	}
	return (&Claim{path: claim.Path}).Close()
}

// Claims lists claims at or below the generation a stop just closed.
func Claims(root string, maximumGeneration int64) ([]CreationClaim, error) {
	claims, problems, err := InspectClaims(root, maximumGeneration)
	if err != nil {
		return nil, err
	}
	if len(problems) > 0 {
		return nil, fmt.Errorf("%s: %w", problems[0].Path, problems[0].Err)
	}
	return claims, nil
}

// InspectClaims reduces every claim independently so stop can report and
// remove malformed files without losing the valid claims beside them.
func InspectClaims(root string, maximumGeneration int64) ([]CreationClaim, []ClaimProblem, error) {
	paths, err := filepath.Glob(filepath.Join(CreatingDir(root), "*.json"))
	if err != nil {
		return nil, nil, err
	}
	claims := make([]CreationClaim, 0, len(paths))
	problems := make([]ClaimProblem, 0)
	for _, path := range paths {
		claim, err := readClaim(path)
		if err != nil {
			problems = append(problems, ClaimProblem{Path: filepath.Clean(path), Err: err})
			continue
		}
		if claim.Generation <= maximumGeneration {
			claims = append(claims, claim)
		}
	}
	return claims, problems, nil
}

func readClaim(path string) (CreationClaim, error) {
	clean := filepath.Clean(path)
	if filepath.Ext(clean) != ".json" || filepath.Base(filepath.Dir(clean)) != "creating" || filepath.Base(filepath.Dir(filepath.Dir(clean))) != "supervision" {
		return CreationClaim{}, errors.New("creation claim path is outside the creating directory")
	}
	data, err := os.ReadFile(clean)
	if err != nil {
		return CreationClaim{}, err
	}
	var claim CreationClaim
	if err := json.Unmarshal(data, &claim); err != nil {
		return CreationClaim{}, fmt.Errorf("creation claim is unreadable: %w", err)
	}
	if claim.SchemaVersion != SchemaVersion || claim.Verb == "" || claim.Generation < 0 || claim.Creator.Pid <= 0 || claim.Creator.Ref().Mode() == identity.CompareInvalid {
		return CreationClaim{}, errors.New("creation claim is invalid")
	}
	if _, err := time.Parse(time.RFC3339, claim.OpenedAt); err != nil {
		return CreationClaim{}, errors.New("creation claim openedAt is invalid")
	}
	claim.Path = clean
	return claim, nil
}

// RemoveClaimFile removes one path only after proving it is a JSON file in
// this root's engine-owned creating directory. It deliberately does not parse
// the file: malformed claims are precisely the files this operation repairs.
func RemoveClaimFile(root, path string) error {
	clean := filepath.Clean(path)
	if filepath.Ext(clean) != ".json" || filepath.Dir(clean) != filepath.Clean(CreatingDir(root)) {
		return errors.New("creation claim path is outside the creating directory")
	}
	if err := os.Remove(clean); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

// CreatorLiveness preserves UNKNOWN as distinct from a live creator.
func CreatorLiveness(claim CreationClaim, prober identity.Prober) identity.Liveness {
	return identity.AliveRef(prober, claim.Creator.Ref())
}

// RemoveStale removes a claim only after its exact creator identity is dead.
func RemoveStale(claim CreationClaim, prober identity.Prober) (bool, error) {
	if CreatorLiveness(claim, prober) != identity.Dead {
		return false, nil
	}
	if err := os.Remove(claim.Path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return false, err
	}
	return true, nil
}
