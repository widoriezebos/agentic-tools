package run

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/sys/unix"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

// The verbs. Every mutation holds the runs lock for the whole operation;
// authority (holder-only with nullable HUMAN coordinates) is enforced at
// the command layer exactly like the goal family, with the distilled
// caller arriving here.

// Caller is the classified invoker, distilled at the command layer.
type Caller struct {
	Class        string
	MainId       string
	OwnerLineage string
	ClaimEpoch   *int64
	SessionId    string
}

func (c Caller) coordinates() (*string, *string, *int64) {
	if c.Class == "HUMAN" {
		return nil, nil, nil
	}
	mainId, lineage := c.MainId, c.OwnerLineage
	return &mainId, &lineage, c.ClaimEpoch
}

// LaunchParams describes a pending registration.
type LaunchParams struct {
	Id, Kind, Display, Log string
	StaleAfterMin          int
	WindDownMin            int
	Expect                 Expect
	GoalId                 string
}

// Launch writes the PENDING record — before any process exists — and
// returns the minted nonce for the wrapper's argv (MON-01). It never
// deletes on failure; the fence concludes stale pendings.
func (s *Store) Launch(caller Caller, p LaunchParams) (nonce string, err error) {
	err = s.withLock(func() error {
		if err := s.checkEpoch(caller); err != nil {
			return err
		}
		existing, err := s.Read(p.Id)
		if err != nil {
			return err
		}
		if existing != nil && !Terminal(existing.Status) {
			return fmt.Errorf("run id %s is live (%s); ids are unique among live records", p.Id, existing.Status)
		}
		minted, err := mintNonce()
		if err != nil {
			return err
		}
		nonce = minted
		logPath, err := s.resolveLog(p.Log)
		if err != nil {
			return err
		}
		mainId, lineage, epoch := caller.coordinates()
		record := &Record{
			SchemaVersion: 1, RunId: p.Id, Kind: p.Kind, Display: p.Display,
			Custody: CustodyWrapped, Generation: 1, LaunchNonce: minted,
			Log: logPath, StartedAt: s.nowISO(),
			MainId: mainId, OwnerLineage: lineage, ClaimEpoch: epoch,
			SessionId: caller.SessionId, GoalId: p.GoalId,
			StaleAfterMin: defaulted(p.StaleAfterMin, 30),
			WindDownMin:   defaulted(p.WindDownMin, DefaultWindDown),
			Evidence:      Evidence{Mode: EvidenceSidecar},
			Expect:        p.Expect, Status: StatusLaunching,
		}
		if err := s.write(record); err != nil {
			return err
		}
		s.emit("run-launched", map[string]string{"runId": p.Id, "kind": p.Kind, "custody": CustodyWrapped})
		return nil
	})
	return nonce, err
}

// Bind is the wrapper's first act: launching→running with its kernel
// identity, authenticated by the nonce in its own argv.
func (s *Store) Bind(id, nonce string, pid, pgid int64) error {
	return s.withLock(func() error {
		record, err := s.Read(id)
		if err != nil {
			return err
		}
		if record == nil {
			return fmt.Errorf("no run record %s to bind", id)
		}
		if record.LaunchNonce != nonce {
			return fmt.Errorf("bind nonce does not match run %s", id)
		}
		exact, state, _ := s.prober().Probe(pid)
		if state != identity.Alive {
			return fmt.Errorf("bind identity for pid %d is not provably alive", pid)
		}
		if kernelPgid, err := s.getpgid(pid); err != nil || kernelPgid != pgid {
			return fmt.Errorf("bind pgid %d does not match the kernel for pid %d", pgid, pid)
		}
		started := exact.StartedAt.Unix()
		_, err = s.cas(id, StatusLaunching, record.Generation, func(r *Record) error {
			r.Pid = &pid
			r.PidStartedAt = &started
			r.Pgid = &pgid
			r.Status = StatusRunning
			return nil
		})
		if err == nil {
			s.emit("run-transition", map[string]string{
				"runId": id, "from": StatusLaunching, "to": StatusRunning,
				"generation": fmt.Sprint(record.Generation),
			})
		}
		return err
	})
}

// Register binds an ALREADY-RUNNING process as an adopted record. The
// kinship predicate (pgid terms only, both-ways ancestry) upgrades to
// adopted-verified; anything less is adopted-unverified, honestly
// labeled and never signaled.
func (s *Store) Register(caller Caller, p LaunchParams, pid int64, verdictPattern string) error {
	return s.withLock(func() error {
		if err := s.checkEpoch(caller); err != nil {
			return err
		}
		existing, err := s.Read(p.Id)
		if err != nil {
			return err
		}
		if existing != nil && !Terminal(existing.Status) {
			return fmt.Errorf("run id %s is live (%s)", p.Id, existing.Status)
		}
		exact, state, _ := s.prober().Probe(pid)
		if state != identity.Alive {
			return fmt.Errorf("register: pid %d is not provably alive", pid)
		}
		pgid, err := s.getpgid(pid)
		if err != nil {
			return fmt.Errorf("register: pgid for %d unreadable: %v", pid, err)
		}
		custody := CustodyAdoptedUnverified
		if s.kinship(pgid) {
			custody = CustodyAdoptedVerified
		}
		nonce, err := mintNonce()
		if err != nil {
			return err
		}
		logPath, err := s.resolveLog(p.Log)
		if err != nil {
			return err
		}
		started := exact.StartedAt.Unix()
		pgid64 := pgid
		mainId, lineage, epoch := caller.coordinates()
		mode := EvidenceNone
		if verdictPattern != "" {
			mode = EvidencePattern
		}
		record := &Record{
			SchemaVersion: 1, RunId: p.Id, Kind: p.Kind, Display: p.Display,
			Custody: custody, Generation: 1, LaunchNonce: nonce,
			Pid: &pid, PidStartedAt: &started, Pgid: &pgid64,
			Log: logPath, StartedAt: s.nowISO(),
			MainId: mainId, OwnerLineage: lineage, ClaimEpoch: epoch,
			SessionId: caller.SessionId, GoalId: p.GoalId,
			StaleAfterMin: defaulted(p.StaleAfterMin, 30),
			WindDownMin:   defaulted(p.WindDownMin, DefaultWindDown),
			Evidence:      Evidence{Mode: mode, VerdictPattern: verdictPattern},
			Expect:        p.Expect, Status: StatusRunning,
		}
		if err := s.write(record); err != nil {
			return err
		}
		s.emit("run-launched", map[string]string{"runId": p.Id, "kind": p.Kind, "custody": custody})
		return nil
	})
}

// kinship: there exists a live process P with P.pgid equal to the
// target's pgid such that P is an ancestor of the caller or the caller
// is an ancestor of P (pgid terms only — the r7 predicate).
func (s *Store) kinship(targetPgid int64) bool {
	self := int64(os.Getpid())
	ancestors := map[int64]bool{}
	for pid := self; pid > 1; {
		ancestors[pid] = true
		ppid, err := parentOf(pid)
		if err != nil || ppid <= 0 || ancestors[ppid] {
			break
		}
		pid = ppid
	}
	// Ancestor of the caller inside the group?
	for pid := range ancestors {
		if pg, err := unix.Getpgid(int(pid)); err == nil && int64(pg) == targetPgid {
			return true
		}
	}
	// Caller an ancestor of a group member? Walk up from the group
	// leader (the strongest member we can name without enumeration).
	for pid := targetPgid; pid > 1; {
		if ancestors[pid] {
			return true
		}
		ppid, err := parentOf(pid)
		if err != nil || ppid <= 0 {
			break
		}
		pid = ppid
	}
	return false
}

// Adopt rebinds identity on a RUNNING record — only when the old
// generation's leader is provably dead AND its recorded group provably
// empty. Generation increments; hungSince clears.
func (s *Store) Adopt(caller Caller, id string, pid int64) error {
	return s.withLock(func() error {
		if err := s.checkEpoch(caller); err != nil {
			return err
		}
		record, err := s.Read(id)
		if err != nil {
			return err
		}
		if record == nil {
			return fmt.Errorf("no run record %s", id)
		}
		if record.Status != StatusRunning {
			return fmt.Errorf("adopt requires a running record; %s is %s", id, record.Status)
		}
		if record.Pid == nil {
			return fmt.Errorf("adopt requires a bound old generation")
		}
		if identity.AliveRef(s.prober(), identity.Ref{Pid: *record.Pid, StartedAtSec: *record.PidStartedAt}) != identity.Dead {
			return fmt.Errorf("adopt refused: the old generation's leader is not provably dead")
		}
		if record.Pgid != nil && !s.groupEmpty(*record.Pgid) {
			return fmt.Errorf("adopt refused: the old generation's group is not provably empty")
		}
		exact, state, _ := s.prober().Probe(pid)
		if state != identity.Alive {
			return fmt.Errorf("adopt: pid %d is not provably alive", pid)
		}
		pgid, err := s.getpgid(pid)
		if err != nil {
			return fmt.Errorf("adopt: pgid for %d unreadable: %v", pid, err)
		}
		nonce, err := mintNonce()
		if err != nil {
			return err
		}
		started := exact.StartedAt.Unix()
		pgid64 := pgid
		from := record.Generation
		_, err = s.cas(id, StatusRunning, from, func(r *Record) error {
			r.Pid = &pid
			r.PidStartedAt = &started
			r.Pgid = &pgid64
			r.Generation = from + 1
			r.LaunchNonce = nonce
			r.HungSince = nil
			// Custody is RECOMPUTED from fresh kinship every adoption —
			// an old adopted-verified label never survives a generation
			// whose kinship fails (critique finding 8).
			if r.Custody == CustodyWrapped {
				r.Evidence = Evidence{Mode: EvidenceNone}
			}
			if s.kinship(pgid64) {
				r.Custody = CustodyAdoptedVerified
			} else {
				r.Custody = CustodyAdoptedUnverified
			}
			return nil
		})
		if err == nil {
			s.emit("run-transition", map[string]string{
				"runId": id, "from": StatusRunning, "to": StatusRunning,
				"generation": fmt.Sprint(from + 1),
			})
		}
		return err
	})
}

// Ack acknowledges a terminal record.
func (s *Store) Ack(caller Caller, id string) error {
	return s.withLock(func() error {
		if err := s.checkEpoch(caller); err != nil {
			return err
		}
		record, err := s.Read(id)
		if err != nil {
			return err
		}
		if record == nil {
			return fmt.Errorf("no run record %s", id)
		}
		if !Terminal(record.Status) {
			return fmt.Errorf("ack requires a terminal record; %s is %s", id, record.Status)
		}
		if record.Acked {
			return fmt.Errorf("run %s is already acknowledged", id)
		}
		_, err = s.cas(id, record.Status, record.Generation, func(r *Record) error {
			r.Acked = true
			return nil
		})
		return err
	})
}

// Prune drops ACKED terminal records older than 14 days, with their
// sidecars, reporting every drop.
func (s *Store) Prune(caller Caller) (dropped []string, err error) {
	err = s.withLock(func() error {
		if err := s.checkEpoch(caller); err != nil {
			return err
		}
		cutoff := s.now().AddDate(0, 0, -PruneAgeDays)
		for _, path := range RecordFiles(s.Root) {
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			var record Record
			if json.Unmarshal(data, &record) != nil {
				continue
			}
			if !Terminal(record.Status) || !record.Acked || record.EndedAt == nil {
				continue
			}
			ended, err := time.Parse("2006-01-02T15:04:05Z", *record.EndedAt)
			if err != nil || ended.After(cutoff) {
				continue
			}
			for gen := 1; gen <= record.Generation; gen++ {
				os.Remove(SidecarPath(s.Root, record.RunId, gen))
			}
			if err := os.Remove(path); err != nil {
				return err
			}
			dropped = append(dropped, fmt.Sprintf("%s — %s (%s, ended %s)", record.RunId, record.Display, record.Status, *record.EndedAt))
		}
		return nil
	})
	return dropped, err
}

// resolveLog applies the bind-time path rule: absolute or repo-relative,
// resolved, contained to the repo or /tmp, symlinks resolved.
func (s *Store) resolveLog(log string) (string, error) {
	if log == "" {
		return "", fmt.Errorf("a run requires a log path")
	}
	path := log
	if !filepath.IsAbs(path) {
		path = filepath.Join(s.Root, path)
	}
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		path = resolved // the file exists: pin its REAL location
	} else {
		resolvedDir, dirErr := filepath.EvalSymlinks(filepath.Dir(path))
		if dirErr != nil {
			return "", fmt.Errorf("log directory unresolvable: %v", dirErr)
		}
		path = filepath.Join(resolvedDir, filepath.Base(path))
	}
	root, err := filepath.EvalSymlinks(s.Root)
	if err != nil {
		root = s.Root
	}
	inRepo := strings.HasPrefix(path, root+string(filepath.Separator))
	tmpRoot := filepath.Clean(os.TempDir()) + string(filepath.Separator)
	inTmp := strings.HasPrefix(path, "/tmp/") || strings.HasPrefix(path, tmpRoot) || strings.HasPrefix(path, "/private/tmp/")
	if !inRepo && !inTmp {
		return "", fmt.Errorf("log path escapes the repo and /tmp: %s", path)
	}
	if len(path) > MaxLogPathBytes {
		return "", fmt.Errorf("log path exceeds %d bytes", MaxLogPathBytes)
	}
	return path, nil
}

func defaulted(value, fallback int) int {
	if value == 0 {
		return fallback
	}
	return value
}

// parentOf answers a live pid's parent through the identity package's
// per-platform reader; Unknown or dead answers refuse the walk.
func parentOf(pid int64) (int64, error) {
	ppid, ok := identity.ParentPid(pid)
	if !ok {
		return 0, fmt.Errorf("parent of %d unreadable", pid)
	}
	return ppid, nil
}

// groupEmpty reports whether no live process remains in the group. A
// group whose membership cannot be read is NOT provably empty.
func (s *Store) groupEmpty(pgid int64) bool {
	pids, err := identity.AllPids()
	if err != nil {
		return false
	}
	for _, pid := range pids {
		pg, err := unix.Getpgid(int(pid))
		if err != nil {
			if err == unix.ESRCH {
				continue // raced an exit: provably absent
			}
			return false // unreadable membership is NOT provably empty
		}
		if int64(pg) == pgid {
			return false
		}
	}
	return true
}

// FailLaunch concludes a pending record the launcher could not spawn —
// the reservation is never deleted, it fails loudly (critique finding 10).
func (s *Store) FailLaunch(id, note string) error {
	return s.withLock(func() error {
		record, err := s.Read(id)
		if err != nil || record == nil {
			return fmt.Errorf("no run record %s to fail", id)
		}
		if record.Status != StatusLaunching {
			return fmt.Errorf("run %s is %s, not launching", id, record.Status)
		}
		var result AssessResult
		return s.terminalize(record, StatusLaunchFailed, nil, &note, &result)
	})
}
