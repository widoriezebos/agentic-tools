package proofrun

// Proof attempt retention (design engine-owns-disk-lifetimes Part B, 3.5
// "Proof records and retained proof", U5b; Round B3-2 rulings R1, R2).
// Attempt records, process records and suite records are accounting (the
// goal budget counts them) and are never removed here. What goes is an
// attempt's large payload, the retained proof under proof-runs/<attempt>/:
// over disk.proof-target-gib the checkout pass empties the payload of the
// oldest terminal attempts past disk.proof-keep-days that nothing still
// needs, and leaves a note there naming when. Nothing needs a payload
// that no retention root reaches: a live attempt, a young one, one whose
// freshness has not expired, one any record kind names, and every attempt
// a root names (reuse, previous, retry, waits, sources; transitively). The
// class fails closed: an attempt record that cannot be read or has an
// unknown schema, or a record kind that cannot be read, holds the whole
// class for the pass. The window alone never releases a payload.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/diskstore"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

// AttemptNamer is one kind of record that can still need a proof attempt.
// Named lists the attempt ids the kind's live records name; an error means
// the kind could not be read, and the class releases nothing that pass.
type AttemptNamer interface {
	Kind() string
	Named(ctx context.Context, now time.Time) ([]string, error)
}

// PayloadNote is the note an attempt's payload directory keeps once its
// payload was released.
const PayloadNote = "payload-released.json"

// AttemptReferences are the attempts one attempt names: its reuse sources,
// its previous and retried attempts, and the producers it waited on or
// took groups from. Every non-empty value counts, whatever its shape.
func AttemptReferences(attempt Attempt) []string {
	var named []string
	add := func(values ...string) {
		for _, value := range values {
			if value != "" && value != attempt.AttemptID {
				named = append(named, value)
			}
		}
	}
	add(attempt.PreviousAttempt)
	if attempt.Retry != nil {
		add(attempt.Retry.PriorAttempt)
	}
	for _, values := range []map[string]string{attempt.TestWaits, attempt.TestSources} {
		for _, value := range values {
			add(value)
		}
	}
	if attempt.TestResult != nil {
		add(TestResultReferences(*attempt.TestResult)...)
	}
	if attempt.PendingCoverage != nil && attempt.PendingCoverage.Evidence != nil {
		add(attempt.PendingCoverage.Evidence.AttemptID)
	}
	return named
}

// TestResultReferences are the attempts a test result names: its own and
// every group's reuse source.
func TestResultReferences(result TestResult) []string {
	named := []string{result.AttemptID}
	for _, group := range result.Groups {
		named = append(named, group.ReuseAttempt)
	}
	return named
}

// ScratchNamer is the scratch records' kind: every attempt a present
// scratch record names, so ReconcileScratch is never stranded.
type ScratchNamer struct{ Control string }

func (ScratchNamer) Kind() string { return "proof-run scratch records" }

func (n ScratchNamer) Named(context.Context, time.Time) ([]string, error) {
	paths, err := filepath.Glob(filepath.Join(ScratchStore(n.Control), "*.json"))
	if err != nil {
		return nil, err
	}
	var named []string
	for _, path := range paths {
		record, err := readScratchRecordFile(path)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("scratch record %s: %w", path, err)
		}
		if record.Attempt != "" {
			named = append(named, record.Attempt)
		}
	}
	return named, nil
}

// Retention is the attempt payloads' class in a checkout pass.
type Retention struct {
	Control string
	// Target is disk.proof-target-gib in bytes; Keep disk.proof-keep-days.
	Target int64
	Keep   time.Duration
	Namers []AttemptNamer
	// Alive reads a recorded process's liveness.
	Alive func(identity.Ref) identity.Liveness

	count int
	bytes int64
	// seen is every attempt record's modification time at the plan, so
	// the apply reads only what changed since.
	seen  map[string]time.Time
	units map[string]*attemptUnit
}

type attemptUnit struct {
	attempt Attempt
	records []string
	bytes   int64
	ended   time.Time
}

func (*Retention) Name() string { return "proof attempt payloads" }

// Totals are every attempt and the bytes of the whole proof-run store.
func (r *Retention) Totals() (int, int64) { return r.count, r.bytes }

func (r *Retention) root() string {
	return filepath.Join(r.Control, "artifacts", "agents", "proof-runs")
}

func (r *Retention) payload(id string) string { return filepath.Join(r.root(), id) }

// hold is the one item a class that cannot judge reports: nothing is
// released this pass.
func (r *Retention) hold(reason string) []diskstore.Item {
	return []diskstore.Item{{Class: r.Name(), Key: "~hold", Path: r.root(), Verdict: diskstore.Verdict{Decision: diskstore.Pending,
		Reason: reason + "; no attempt payload is released this pass", Command: "metasystem test status"}}}
}

// Plan reads every attempt record and its process records, measures, and,
// over the target, names the oldest payloads nothing needs until the store
// would be under it. Any record it cannot read holds the class. It writes
// nothing.
func (r *Retention) Plan(ctx context.Context, pass *diskstore.Pass) ([]diskstore.Item, error) {
	r.count, r.bytes, r.units, r.seen = 0, 0, map[string]*attemptUnit{}, map[string]time.Time{}
	if _, err := os.Stat(r.root()); errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	r.bytes, _, _ = diskstore.Measure(ctx, r.root())
	records, err := r.attemptRecords()
	if err != nil {
		return r.hold(err.Error()), nil
	}
	paths, err := filepath.Glob(filepath.Join(attemptsDir(r.Control), "*.json"))
	if err != nil {
		return nil, err
	}
	for _, path := range paths {
		id := strings.TrimSuffix(filepath.Base(path), ".json")
		if info, err := os.Stat(path); err == nil {
			r.seen[path] = info.ModTime()
		}
		r.count++
		attempt, err := readRetainedAttempt(r.Control, id)
		if err != nil {
			return r.hold("attempt record " + id + " cannot be read (" + err.Error() + "), so what it needs is unknown"), nil
		}
		unit := &attemptUnit{attempt: attempt, records: records[id], ended: attemptEnded(attempt)}
		unit.bytes = payloadBytes(ctx, r.payload(id))
		r.units[id] = unit
	}
	if r.bytes <= r.Target {
		return nil, nil
	}
	roots := map[string]bool{}
	for _, namer := range r.Namers {
		named, err := namer.Named(ctx, pass.Now)
		if err != nil {
			return r.hold("which attempts the " + namer.Kind() + " name is unknown (" + err.Error() + ")"), nil
		}
		for _, id := range named {
			roots[id] = true
		}
	}
	for id, unit := range r.units {
		if r.isRoot(unit, pass.Now) {
			roots[id] = true
		}
	}
	closeOver(roots, r.units)
	order := make([]*attemptUnit, 0, len(r.units))
	for _, unit := range r.units {
		order = append(order, unit)
	}
	sort.Slice(order, func(i, j int) bool {
		if !order[i].ended.Equal(order[j].ended) {
			return order[i].ended.Before(order[j].ended)
		}
		return order[i].attempt.AttemptID < order[j].attempt.AttemptID
	})
	var items []diskstore.Item
	over := r.bytes - r.Target
	for _, unit := range order {
		if over <= 0 || ctx.Err() != nil {
			break
		}
		id := unit.attempt.AttemptID
		if roots[id] || unit.bytes == 0 {
			continue
		}
		if reason := r.processesAlive(unit); reason != "" {
			items = append(items, diskstore.Item{Class: r.Name(), Key: id, Path: r.payload(id), Bytes: unit.bytes,
				Verdict: diskstore.Verdict{Decision: diskstore.Keep, Reason: reason, Command: "metasystem work status"}})
			continue
		}
		items = append(items, diskstore.Item{Class: r.Name(), Key: id, Path: r.payload(id), Bytes: unit.bytes,
			Verdict: diskstore.Verdict{Decision: diskstore.Release, Reason: "terminal, past disk.proof-keep-days, needed by nothing; its payload goes, its records stay"}})
		over -= unit.bytes
	}
	if over > 0 {
		items = append(items, diskstore.Item{Class: r.Name(), Key: "~target", Path: r.root(), Verdict: diskstore.Verdict{Decision: diskstore.Keep,
			Reason:  fmt.Sprintf("the proof-run store is over its target of %d GiB after every payload its proofs allow; what remains is records, or payloads live, young, fresh or still needed", r.Target>>30),
			Command: "metasystem disk show"}})
	}
	return items, nil
}

// isRoot reports the retention roots an attempt is by itself: live,
// younger than the window, or fresh.
func (r *Retention) isRoot(unit *attemptUnit, now time.Time) bool {
	if unit.attempt.Terminal == nil || unit.ended.IsZero() || now.Sub(unit.ended) < r.Keep {
		return true
	}
	if unit.attempt.FreshnessExpiresAt != "" {
		expires, err := time.Parse(time.RFC3339Nano, unit.attempt.FreshnessExpiresAt)
		if err != nil || expires.After(now) {
			return true
		}
	}
	return false
}

// closeOver adds every attempt a root names, transitively.
func closeOver(roots map[string]bool, units map[string]*attemptUnit) {
	queue := make([]string, 0, len(roots))
	for id := range roots {
		queue = append(queue, id)
	}
	for len(queue) > 0 {
		id := queue[0]
		queue = queue[1:]
		unit := units[id]
		if unit == nil {
			continue
		}
		for _, named := range AttemptReferences(unit.attempt) {
			if !roots[named] {
				roots[named] = true
				queue = append(queue, named)
			}
		}
	}
}

// processesAlive names a recorded process of the attempt that is not
// proven dead; an unreadable process record counts as alive.
func (r *Retention) processesAlive(unit *attemptUnit) string {
	for _, path := range unit.records {
		data, err := os.ReadFile(path)
		if err != nil {
			return "process record unreadable, so its process counts as alive: " + err.Error()
		}
		var record Record
		if err := json.Unmarshal(data, &record); err != nil {
			return "process record unreadable, so its process counts as alive: " + err.Error()
		}
		for label, process := range map[string]ProcessIdentity{"suite": record.SuiteProcess, "watchdog": record.Watchdog, "launcher": record.Launcher} {
			if process.Pid <= 0 {
				continue
			}
			if state := r.Alive(process.Ref()); state != identity.Dead {
				return fmt.Sprintf("its %s pid %d is %s", label, process.Pid, state)
			}
		}
	}
	return ""
}

// attemptRecords indexes the process and suite records by the attempt they
// name; a record that cannot be read is an error.
func (r *Retention) attemptRecords() (map[string][]string, error) {
	byAttempt := map[string][]string{}
	for _, pattern := range []string{filepath.Join(r.root(), "processes", "*.json"), filepath.Join(r.root(), "*.json")} {
		paths, _ := filepath.Glob(pattern)
		for _, path := range paths {
			data, err := os.ReadFile(path)
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			var record struct {
				AttemptID string `json:"attemptId"`
			}
			if err == nil {
				err = json.Unmarshal(data, &record)
			}
			if err != nil {
				return nil, fmt.Errorf("proof-run record %s cannot be read: %w", path, err)
			}
			if record.AttemptID != "" {
				byAttempt[record.AttemptID] = append(byAttempt[record.AttemptID], path)
			}
		}
	}
	return byAttempt, nil
}

// payloadBytes is the payload's size without its note.
func payloadBytes(ctx context.Context, dir string) int64 {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0
	}
	var bytes int64
	for _, entry := range entries {
		if entry.Name() == PayloadNote {
			continue
		}
		measured, _, _ := diskstore.Measure(ctx, filepath.Join(dir, entry.Name()))
		bytes += measured
	}
	return bytes
}

func attemptEnded(attempt Attempt) time.Time {
	stamps := []string{attempt.EndedAt, attempt.StartedAt}
	if attempt.Terminal != nil {
		stamps = append([]string{attempt.Terminal.At}, stamps...)
	}
	for _, stamp := range stamps {
		if parsed, err := time.Parse(time.RFC3339Nano, stamp); err == nil {
			return parsed
		}
	}
	return time.Time{}
}

// Apply empties one attempt's payload under the proof mutation lock, taken
// without waiting: the attempt is reloaded and judged again, every attempt
// written since the plan is read for a reference to it (one that cannot
// be read keeps it), the note is written, then every payload entry but the
// note is removed. The attempt's records are never touched.
func (r *Retention) Apply(ctx context.Context, pass *diskstore.Pass, item diskstore.Item) diskstore.Verdict {
	held, err := TryAcquireMutation(r.Control)
	if errors.Is(err, ErrMutationHeld) {
		return diskstore.Verdict{Decision: diskstore.Pending, Reason: "a proof run holds the proof mutation lock", Command: "metasystem disk clean, once that run has ended"}
	}
	if err != nil {
		return diskstore.Verdict{Decision: diskstore.Pending, Reason: err.Error(), Command: "metasystem disk show"}
	}
	defer held.Release()
	id := item.Key
	attempt, err := readRetainedAttempt(r.Control, id)
	if err != nil {
		return diskstore.Verdict{Decision: diskstore.Pending, Reason: "attempt record unreadable: " + err.Error(), Command: "metasystem test status"}
	}
	records, err := r.attemptRecords()
	if err != nil {
		return diskstore.Verdict{Decision: diskstore.Pending, Reason: err.Error(), Command: "metasystem test status"}
	}
	unit := &attemptUnit{attempt: attempt, records: records[id], ended: attemptEnded(attempt)}
	if r.isRoot(unit, pass.Now) {
		return diskstore.Verdict{Decision: diskstore.Keep, Reason: "the attempt changed since the plan", Command: "metasystem disk show"}
	}
	if reason := r.processesAlive(unit); reason != "" {
		return diskstore.Verdict{Decision: diskstore.Keep, Reason: reason, Command: "metasystem work status"}
	}
	if referrer := r.newerReferrer(id); referrer != "" {
		return diskstore.Verdict{Decision: diskstore.Keep, Reason: "attempt " + referrer + " written since the plan names it or cannot be read", Command: "metasystem disk show"}
	}
	dir := r.payload(id)
	note, _ := json.Marshal(map[string]any{"attempt": id, "payloadReleasedAt": pass.Now.UTC().Format(time.RFC3339), "bytes": item.Bytes})
	if _, err := atomicfile.WriteFile(filepath.Join(dir, PayloadNote), append(note, '\n'), 0o600, dir); err != nil {
		return diskstore.Verdict{Decision: diskstore.Pending, Reason: "the payload note could not be written: " + err.Error(), Command: "metasystem disk show"}
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return diskstore.Verdict{Decision: diskstore.Pending, Reason: err.Error(), Command: "metasystem disk show"}
	}
	for _, entry := range entries {
		if entry.Name() == PayloadNote {
			continue
		}
		if err := diskstore.RemoveTree(ctx, filepath.Join(dir, entry.Name())); err != nil {
			return diskstore.Verdict{Decision: diskstore.Pending, Reason: "removal cut short (" + err.Error() + "); the next pass finishes it", Command: "metasystem disk clean"}
		}
	}
	return diskstore.Verdict{Decision: diskstore.Release, Reason: "payload of an attempt past its window, needed by nothing, released; its records stay"}
}

// newerReferrer is an attempt record written or changed since the plan
// that names id, or cannot be read.
func (r *Retention) newerReferrer(id string) string {
	paths, _ := filepath.Glob(filepath.Join(attemptsDir(r.Control), "*.json"))
	for _, path := range paths {
		info, err := os.Stat(path)
		if seen, ok := r.seen[path]; err != nil || ok && info.ModTime().Equal(seen) {
			continue
		}
		other := strings.TrimSuffix(filepath.Base(path), ".json")
		attempt, err := readRetainedAttempt(r.Control, other)
		if err != nil {
			return other
		}
		for _, named := range AttemptReferences(attempt) {
			if named == id {
				return other
			}
		}
	}
	return ""
}

// readRetainedAttempt decodes an attempt record for its retention facts
// (terminal, times, freshness, references, process keys). A record that
// does not decode into the attempt type, names another attempt, or carries
// a schema this engine does not know is unreadable: what it needs is then
// unknown.
func readRetainedAttempt(control, id string) (Attempt, error) {
	path, err := AttemptPath(control, id)
	if err != nil {
		return Attempt{}, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return Attempt{}, err
	}
	var attempt Attempt
	if err := json.Unmarshal(data, &attempt); err != nil {
		return Attempt{}, fmt.Errorf("proof attempt %s: %w", id, err)
	}
	if attempt.AttemptID != id {
		return Attempt{}, fmt.Errorf("proof attempt %s names %q", id, attempt.AttemptID)
	}
	switch attempt.SchemaVersion {
	case LegacyAttemptSchemaVersion, AttemptSchemaVersion, CandidateAttemptSchemaVersion, IdentityAttemptSchemaVersion:
	default:
		return Attempt{}, fmt.Errorf("proof attempt %s has schema %d, which this engine does not know", id, attempt.SchemaVersion)
	}
	return attempt, nil
}
