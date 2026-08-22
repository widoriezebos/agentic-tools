package run

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

// The conclusion engine: evidence rules only, never opinion. Unknown
// identity concludes NOTHING (three-way discipline); dead-plus-no-evidence
// is ended-unknown, the one verdict for that fact; adopted-pattern
// evidence is evaluated ONCE at draining entry and frozen.

// Sidecar is the wrapper's atomic exit evidence, believed only when
// nonce AND generation match the record.
type Sidecar struct {
	RunId      string `json:"runId"`
	Generation int    `json:"generation"`
	Nonce      string `json:"nonce"`
	ExitCode   int64  `json:"exitCode"`
	EndedAt    string `json:"endedAt"`
}

// WriteSidecar is the wrapper's last act.
func (s *Store) WriteSidecar(id string, generation int, nonce string, exitCode int64) error {
	data, err := json.MarshalIndent(Sidecar{
		RunId: id, Generation: generation, Nonce: nonce,
		ExitCode: exitCode, EndedAt: s.nowISO(),
	}, "", " ")
	if err != nil {
		return err
	}
	return atomicWrite(SidecarPath(s.Root, id, generation), append(data, '\n'))
}

// readSidecar returns the matching sidecar or nil (forged, stale, or
// absent sidecars are nil; unreadable ones surface as errors).
func (s *Store) readSidecar(record *Record) (*Sidecar, error) {
	data, err := os.ReadFile(SidecarPath(s.Root, record.RunId, record.Generation))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var sidecar Sidecar
	if err := json.Unmarshal(data, &sidecar); err != nil {
		return nil, fmt.Errorf("sidecar for %s.g%d unparsable: %v", record.RunId, record.Generation, err)
	}
	if sidecar.Nonce != record.LaunchNonce || sidecar.Generation != record.Generation {
		return nil, nil
	}
	return &sidecar, nil
}

// AssessResult reports what one pass over one record observed.
type AssessResult struct {
	Transitioned bool
	From, To     string
	Unreadable   []string
}

// Assess advances one record per the rules — the watcher's verb and the
// manual conclude alike. It holds the runs lock.
func (s *Store) Assess(id string) (AssessResult, error) {
	var result AssessResult
	err := s.withLock(func() error {
		return s.assessHeld(id, &result)
	})
	return result, err
}

// assessLaunching applies the launch fence.
func (s *Store) assessLaunching(record *Record, result *AssessResult) error {
	started, err := time.Parse("2006-01-02T15:04:05Z", record.StartedAt)
	if err != nil {
		result.Unreadable = append(result.Unreadable, record.RunId+": startedAt unparsable")
		return nil
	}
	if s.now().Sub(started) < LaunchFenceMin*time.Minute {
		return nil
	}
	return s.terminalize(record, StatusLaunchFailed, nil, strPtr("launch fence: no bind within the fence"), result)
}

// assessRunning probes the leader three-way and applies the evidence
// table on death.
func (s *Store) assessRunning(record *Record, result *AssessResult) error {
	if record.Pid == nil || record.PidStartedAt == nil {
		result.Unreadable = append(result.Unreadable, record.RunId+": running record with null identity")
		return nil
	}
	switch identity.AliveRef(s.prober(), identity.Ref{Pid: *record.Pid, StartedAtSec: *record.PidStartedAt,
		StartTicks: record.PidStartTicks, BootID: record.BootID}) {
	case identity.Alive:
		return s.assessHung(record)
	case identity.Unknown:
		result.Unreadable = append(result.Unreadable, record.RunId+": leader liveness unknown")
		return nil
	}
	// Dead leader: freeze the provisional verdict NOW (pattern evidence
	// evaluated once here — descendants writing the log later cannot
	// change it), then drain or terminalize by group emptiness.
	verdict, exitCode, note, unreadable := s.provisional(record)
	result.Unreadable = append(result.Unreadable, unreadable...)
	if record.Pgid != nil && s.groupEmpty(*record.Pgid) {
		return s.terminalizeWithVerdict(record, verdict, exitCode, note, result)
	}
	endedAt := s.nowISO()
	_, err := s.cas(record.RunId, StatusRunning, record.Generation, func(r *Record) error {
		r.Status = StatusDraining
		r.ProvisionalVerdict = &verdict
		r.ExitCode = exitCode
		r.EndedAt = &endedAt
		r.Error = note
		return nil
	})
	if err == nil {
		result.Transitioned, result.From, result.To = true, StatusRunning, StatusDraining
		s.emit("run-transition", map[string]string{
			"runId": record.RunId, "from": StatusRunning, "to": StatusDraining,
			"generation": fmt.Sprint(record.Generation),
		})
	}
	return err
}

// assessDraining finalizes when the group empties or the wind-down
// (running from endedAt — draining entry) expires.
func (s *Store) assessDraining(record *Record, result *AssessResult) error {
	expired := false
	if record.EndedAt != nil {
		if ended, err := time.Parse("2006-01-02T15:04:05Z", *record.EndedAt); err == nil {
			expired = s.now().Sub(ended) >= time.Duration(record.WindDownMin)*time.Minute
		}
	}
	if record.Pgid != nil && !s.groupEmpty(*record.Pgid) && !expired {
		return nil
	}
	verdict := StatusEndedUnknown
	if record.ProvisionalVerdict != nil {
		verdict = *record.ProvisionalVerdict
	}
	return s.terminalizeWithVerdict(record, verdict, record.ExitCode, record.Error, result)
}

// assessHung sets or clears the hung flag by log mtime.
func (s *Store) assessHung(record *Record) error {
	info, err := os.Stat(record.Log)
	quiet := false
	if err == nil {
		quiet = s.now().Sub(info.ModTime()) >= time.Duration(record.StaleAfterMin)*time.Minute
	}
	switch {
	case quiet && record.HungSince == nil:
		since := s.nowISO()
		_, err := s.cas(record.RunId, StatusRunning, record.Generation, func(r *Record) error {
			r.HungSince = &since
			return nil
		})
		return err
	case !quiet && record.HungSince != nil:
		_, err := s.cas(record.RunId, StatusRunning, record.Generation, func(r *Record) error {
			r.HungSince = nil
			return nil
		})
		return err
	}
	return nil
}

// provisional applies the evidence table for a dead leader.
func (s *Store) provisional(record *Record) (verdict string, exitCode *int64, note *string, unreadable []string) {
	switch record.Evidence.Mode {
	case EvidenceSidecar:
		sidecar, err := s.readSidecar(record)
		if err != nil {
			return StatusEndedUnknown, nil, strPtr(err.Error()), []string{err.Error()}
		}
		if sidecar == nil {
			return StatusEndedUnknown, nil, strPtr("dead leader with no matching sidecar"), nil
		}
		code := sidecar.ExitCode
		if code == 0 {
			return StatusGreen, &code, nil, nil
		}
		return StatusRed, &code, nil, nil
	case EvidencePattern:
		if resolved, err := filepath.EvalSymlinks(record.Log); err != nil || resolved != record.Log {
			message := record.RunId + ": log path moved since bind; refusing pattern evidence"
			return StatusEndedUnknown, nil, strPtr(message), []string{message}
		}
		tail, err := readTail(record.Log, PatternTailBytes)
		if err != nil {
			message := record.RunId + ": log unreadable at conclusion: " + err.Error()
			return StatusEndedUnknown, nil, strPtr(message), []string{message}
		}
		matched, err := regexp.Match(record.Evidence.VerdictPattern, tail)
		if err == nil && matched {
			return StatusGreen, nil, nil, nil
		}
		return StatusEndedUnknown, nil, strPtr("verdict pattern did not match; never a red guess"), nil
	default:
		return StatusEndedUnknown, nil, strPtr("dead leader with no evidence mode"), nil
	}
}

// terminalizeWithVerdict finalizes from running/draining.
func (s *Store) terminalizeWithVerdict(record *Record, verdict string, exitCode *int64, note *string, result *AssessResult) error {
	seq, err := s.nextTerminalSeq()
	if err != nil {
		return err
	}
	from := record.Status
	endedAt := record.EndedAt
	if endedAt == nil {
		now := s.nowISO()
		endedAt = &now
	}
	_, err = s.cas(record.RunId, from, record.Generation, func(r *Record) error {
		r.Status = verdict
		r.ProvisionalVerdict = nil
		r.TerminalSeq = &seq
		r.ExitCode = exitCode
		r.Error = note
		r.EndedAt = endedAt
		return nil
	})
	if err == nil {
		result.Transitioned, result.From, result.To = true, from, verdict
		s.emit("run-transition", map[string]string{
			"runId": record.RunId, "from": from, "to": verdict,
			"generation": fmt.Sprint(record.Generation),
		})
	}
	return err
}

// terminalize is the launching→launch-failed ramp.
func (s *Store) terminalize(record *Record, status string, exitCode *int64, note *string, result *AssessResult) error {
	seq, err := s.nextTerminalSeq()
	if err != nil {
		return err
	}
	from := record.Status
	_, err = s.cas(record.RunId, from, record.Generation, func(r *Record) error {
		r.Status = status
		r.TerminalSeq = &seq
		r.ExitCode = exitCode
		r.Error = note
		if r.EndedAt == nil {
			stamped := s.nowISO()
			r.EndedAt = &stamped
		}
		return nil
	})
	if err == nil {
		result.Transitioned, result.From, result.To = true, from, status
		s.emit("run-transition", map[string]string{
			"runId": record.RunId, "from": from, "to": status,
			"generation": fmt.Sprint(record.Generation),
		})
	}
	return err
}

func readTail(path string, limit int64) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return nil, err
	}
	offset := int64(0)
	if info.Size() > limit {
		offset = info.Size() - limit
	}
	buf := make([]byte, info.Size()-offset)
	_, err = f.ReadAt(buf, offset)
	return buf, err
}

func strPtr(s string) *string { return &s }

// SweepStale is the takeover sweep's run half: it
// holds the runs lock for the whole pass, signals ONLY wrapped runs whose
// group ownership the proof function establishes (adopted custody never
// carries the argv nonce and is never signaled — it surfaces through the
// returned refusal), waits a bounded drain after TERM, and FORCES the
// conclusion to ended-unknown when the assessment cannot move the record
// — a takeover never completes over an unswept run.
func (s *Store) SweepStale(epoch int64,
	proof func(pgid int64, nonce string) (owned, provable bool),
	kill func(pgid int64) error) error {
	return s.withLock(func() error {
		records, unreadable := s.List()
		if len(unreadable) > 0 {
			return fmt.Errorf("run sweep refused: %s", unreadable[0])
		}
		for i := range records {
			record := &records[i]
			if record.ClaimEpoch == nil || *record.ClaimEpoch >= epoch {
				continue
			}
			if record.Status != StatusRunning && record.Status != StatusDraining {
				continue
			}
			if record.Custody != CustodyWrapped {
				return fmt.Errorf("run sweep refused: stale run %s has custody %s — never signaled, only surfaced", record.RunId, record.Custody)
			}
			if record.Pgid == nil {
				return fmt.Errorf("run sweep refused: stale run %s has no bound group", record.RunId)
			}
			owned, provable := proof(*record.Pgid, record.LaunchNonce)
			if !provable {
				return fmt.Errorf("run sweep cannot prove ownership of stale run %s", record.RunId)
			}
			if !owned {
				return fmt.Errorf("run sweep refused: stale run %s group ownership disproven; surfacing", record.RunId)
			}
			if err := kill(*record.Pgid); err != nil {
				return fmt.Errorf("run sweep cannot stop stale run %s: %v", record.RunId, err)
			}
			// Bounded drain: give the TERM five seconds to land.
			deadline := time.Now().Add(5 * time.Second)
			for time.Now().Before(deadline) && !s.groupEmpty(*record.Pgid) {
				time.Sleep(100 * time.Millisecond)
			}
			var result AssessResult
			if err := s.assessHeld(record.RunId, &result); err != nil {
				return err
			}
			fresh, err := s.Read(record.RunId)
			if err != nil {
				return err
			}
			if !Terminal(fresh.Status) {
				note := "swept at takeover: forced conclusion"
				if err := s.terminalizeWithVerdict(fresh, StatusEndedUnknown, nil, &note, &result); err != nil {
					return fmt.Errorf("run sweep could not conclude %s: %v", record.RunId, err)
				}
			}
			s.emit("run-swept", map[string]string{"runId": record.RunId, "reason": "stale-claim-epoch"})
		}
		return nil
	})
}

// assessHeld is Assess's body for callers already holding the runs lock.
func (s *Store) assessHeld(id string, result *AssessResult) error {
	record, err := s.Read(id)
	if err != nil {
		return err
	}
	if record == nil {
		return fmt.Errorf("no run record %s", id)
	}
	switch record.Status {
	case StatusLaunching:
		return s.assessLaunching(record, result)
	case StatusRunning:
		return s.assessRunning(record, result)
	case StatusDraining:
		return s.assessDraining(record, result)
	default:
		return nil
	}
}
