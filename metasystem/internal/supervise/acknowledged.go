package supervise

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lock"
)

// The acknowledged-process mechanism: an UNTRACKED report that
// nags forever for a process the human has already judged harmless
// teaches the reader to skim, which is how a REAL untracked agent
// slips past. The human acknowledges one exact process; the watchdog
// then stays silent about THAT process and no other. The safety
// property is one-directional: acknowledgement must never silence a
// new or different process, so every ambiguity here fails toward
// shouting. This is a COOPERATIVE control (unforgeable local
// records are not a product contract): a same-user process that
// forges the record file itself is outside what filesystem state can
// refuse; the verb's authority check and the exact-birth token close
// the accidental and the ordinary-hostile cases.
//
// The load-bearing details: the identity is a
// kernel-resolution birth token, not a whole second (pid reuse within
// a second); the verb authorizes its caller (an untracked agent must
// not acknowledge itself); the census consulted must be CURRENT
// (fresh, successful) and the entry UNTRACKED; the record is strictly
// validated on load (a partial record shouts rather than silences);
// the read-modify-write holds a lock (no lost updates); pruning drops
// an entry only on PROVEN death (unknown keeps).

// AcknowledgedProcess is one standing "this exact process is fine"
// record.
type AcknowledgedProcess struct {
	Pid int64 `json:"pid"`
	// PidStartedAt is the whole-second start the census inventory
	// carries — the coarse match key.
	PidStartedAt int64 `json:"pidStartedAt"`
	// PidStartedAtExactMicro is the kernel-resolution birth token (or
	// the fixture's declared start in fake mode). The watchdog
	// re-probes at report time and silences only on an exact match, so
	// a pid recycled within the same second still shouts.
	PidStartedAtExactMicro int64  `json:"pidStartedAtExactMicro"`
	Reason                 string `json:"reason"`
	AcknowledgedAt         string `json:"acknowledgedAt"`
}

func acknowledgedPath(repo string) string {
	return filepath.Join(repo, "artifacts", "agents", "supervision", "acknowledged-processes.json")
}

func acknowledgedLockDir(repo string) string {
	return filepath.Join(repo, "artifacts", "agents", "supervision", "acknowledged-processes.lock.d")
}

// LoadAcknowledged reads and STRICTLY validates the acknowledgement
// record. An absent record is an empty set. Any other failure —
// unreadable, malformed, or an entry missing a required field — is an
// error, and the watchdog's error path treats an error as an empty
// set, so bad records make the nag REAPPEAR rather than silence
// anything — the record fails toward shouting.
func LoadAcknowledged(repo string) ([]AcknowledgedProcess, error) {
	raw, err := os.ReadFile(acknowledgedPath(repo))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var acks []AcknowledgedProcess
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&acks); err != nil {
		return nil, fmt.Errorf("acknowledged-processes.json is unreadable: %w", err)
	}
	for i, a := range acks {
		switch {
		case a.Pid < 1:
			return nil, fmt.Errorf("acknowledged-processes.json entry %d: pid is missing or invalid", i)
		case a.PidStartedAt < 1:
			return nil, fmt.Errorf("acknowledged-processes.json entry %d: pidStartedAt is missing or invalid", i)
		case a.PidStartedAtExactMicro < 1:
			return nil, fmt.Errorf("acknowledged-processes.json entry %d: pidStartedAtExactMicro is missing or invalid", i)
		case a.Reason == "":
			return nil, fmt.Errorf("acknowledged-processes.json entry %d: reason is missing", i)
		case a.AcknowledgedAt == "":
			return nil, fmt.Errorf("acknowledged-processes.json entry %d: acknowledgedAt is missing", i)
		}
		if _, err := time.Parse(time.RFC3339, a.AcknowledgedAt); err != nil {
			return nil, fmt.Errorf("acknowledged-processes.json entry %d: acknowledgedAt is not RFC3339", i)
		}
	}
	return acks, nil
}

// acknowledgedIndex keys the record by the coarse (pid, second) pair;
// the exact token is verified separately at silence time.
func acknowledgedIndex(acks []AcknowledgedProcess) map[[2]int64]AcknowledgedProcess {
	index := make(map[[2]int64]AcknowledgedProcess, len(acks))
	for _, a := range acks {
		index[[2]int64{a.Pid, a.PidStartedAt}] = a
	}
	return index
}

// exactStartMicro resolves a live process's birth token: kernel death
// refuses outright; a fixture entry (fake mode) supplies its declared
// start at second resolution; otherwise the kernel's exact start at
// its native resolution. ok=false means the token cannot be proven —
// which never authorizes silence.
func exactStartMicro(pid int64, probe identity.FixtureProbe) (int64, bool) {
	exact, state, err := (identity.KernelProber{}).Probe(pid)
	if err == nil && state == identity.Dead {
		return 0, false
	}
	if probe != nil {
		if entry, ok := probe.FixtureEntry(pid); ok && entry.HasStartedAt {
			return entry.StartedAt * 1_000_000, true
		}
	}
	if state == identity.Alive {
		return exact.StartedAt.UnixMicro(), true
	}
	return 0, false
}

// silencedByAcknowledgement reports whether one census UNTRACKED item
// is covered by a live acknowledgement: coarse pair match AND a fresh
// probe proving the exact birth token. Any failure to prove shouts.
func silencedByAcknowledgement(index map[[2]int64]AcknowledgedProcess, pid, startSec int64, probe identity.FixtureProbe) bool {
	entry, ok := index[[2]int64{pid, startSec}]
	if !ok {
		return false
	}
	exact, ok := exactStartMicro(pid, probe)
	return ok && exact == entry.PidStartedAtExactMicro
}

// Acknowledge records that one exact untracked process is
// known-harmless. The caller names only the pid it saw in the nag;
// everything else is derived and verified:
//
//   - the census must be CURRENT (verdict SUCCESS, completed within
//     the same freshness window dispatch trusts) — a stale snapshot
//     proves nothing about the pid it names;
//   - the census entry must be class UNTRACKED — acknowledging a
//     tracked process would pre-silence its later untracked fate;
//   - the exact birth token must be provable NOW and agree with the
//     census's whole-second start — disagreement means the pid was
//     recycled since the census wrote;
//   - the whole read-prune-append-write runs under a lock, so two
//     acknowledgements cannot lose each other;
//   - pruning drops an entry only when its process is PROVENLY dead
//     (kernel veto or identity mismatch); unprovable liveness keeps.
func Acknowledge(repo string, pid int64, reason string, now time.Time, probe identity.FixtureProbe) (AcknowledgedProcess, error) {
	if reason == "" {
		return AcknowledgedProcess{}, fmt.Errorf("acknowledge refused: a human reason is required")
	}
	startSec, censusExact, err := currentCensusUntrackedStart(repo, pid, now)
	if err != nil {
		return AcknowledgedProcess{}, err
	}
	exact, ok := exactStartMicro(pid, probe)
	if !ok {
		return AcknowledgedProcess{}, fmt.Errorf("acknowledge refused: pid %d's exact birth cannot be proven right now", pid)
	}
	// The binding is to THE process the census observed: the live
	// probe's kernel-resolution token must equal the token the census
	// recorded. A pid recycled after the scan — even within the same
	// second — carries a different token and refuses; second-resolution
	// binding alone would leave a same-second window open.
	if exact != censusExact {
		return AcknowledgedProcess{}, fmt.Errorf("acknowledge refused: pid %d is not the process the census observed (recycled since the last scan); re-run after the next census", pid)
	}

	self, _, selfErr := (identity.KernelProber{}).Probe(int64(os.Getpid()))
	if selfErr != nil {
		return AcknowledgedProcess{}, fmt.Errorf("acknowledge refused: own identity unreadable: %v", selfErr)
	}
	held, err := lock.Acquire(acknowledgedLockDir(repo), lock.Identity{
		Pid: int64(os.Getpid()), PidStartedAt: self.StartedAt.Unix(),
	}, lock.Options{
		// Death-only takeover: a crashed acknowledger's husk yields; a
		// live or unprovable holder refuses (the standard lock rule).
		Probe: func(holder lock.Identity) lock.Liveness {
			switch identity.AliveRef(identity.KernelProber{}, identity.Ref{Pid: holder.Pid, StartedAtSec: holder.PidStartedAt}) {
			case identity.Alive:
				return lock.Alive
			case identity.Dead:
				return lock.Dead
			default:
				return lock.Unknown
			}
		},
	})
	if err != nil {
		return AcknowledgedProcess{}, fmt.Errorf("acknowledge refused: %v", err)
	}
	defer held.Release()

	existing, err := LoadAcknowledged(repo)
	if err != nil {
		return AcknowledgedProcess{}, err
	}
	entry := AcknowledgedProcess{
		Pid:                    pid,
		PidStartedAt:           startSec,
		PidStartedAtExactMicro: censusExact,
		Reason:                 reason,
		AcknowledgedAt:         now.UTC().Format(time.RFC3339),
	}
	kept := []AcknowledgedProcess{}
	for _, a := range existing {
		if a.Pid == pid && a.PidStartedAt == startSec {
			continue // replaced by the fresh entry
		}
		// Only PROVEN death expires an entry: the kernel's veto, or a
		// live pid whose identity no longer matches (recycled). An
		// unprovable probe keeps the entry — losing a live
		// acknowledgement to a transient read failure would re-nag a
		// process the human already judged.
		if identity.Custodian(a.Pid, a.PidStartedAt, "", probe) == identity.Dead {
			continue
		}
		kept = append(kept, a)
	}
	kept = append(kept, entry)
	sort.Slice(kept, func(i, j int) bool {
		if kept[i].Pid != kept[j].Pid {
			return kept[i].Pid < kept[j].Pid
		}
		return kept[i].PidStartedAt < kept[j].PidStartedAt
	})

	rendered, err := json.MarshalIndent(kept, "", "  ")
	if err != nil {
		return AcknowledgedProcess{}, err
	}
	if _, err := atomicfile.WriteText(acknowledgedPath(repo), string(rendered)+"\n", ""); err != nil {
		return AcknowledgedProcess{}, err
	}
	return entry, nil
}

// currentCensusUntrackedStart reads the CURRENT census and returns the
// recorded whole-second start for pid, requiring the census to be
// fresh and successful and the entry to be UNTRACKED. The freshness
// window mirrors what dispatch trusts: completed within
// min(2×interval, 180s), never in the future.
func currentCensusUntrackedStart(repo string, pid int64, now time.Time) (startSec, exactMicro int64, err error) {
	last, err := readObjectFile(filepath.Join(repo, "artifacts", "agents", "supervision", "last-census.json"))
	if err != nil {
		if os.IsNotExist(err) {
			return 0, 0, fmt.Errorf("acknowledge refused: no census has run; nothing names pid %d", pid)
		}
		return 0, 0, err
	}
	if verdict, _ := last["verdict"].(string); verdict != "SUCCESS" {
		return 0, 0, fmt.Errorf("acknowledge refused: the last census did not succeed; a failed snapshot proves nothing")
	}
	completed, ok := intField(last["completedAtEpoch"])
	if !ok {
		return 0, 0, fmt.Errorf("acknowledge refused: the last census carries no completion time")
	}
	interval, ok := intField(last["intervalSec"])
	if !ok || interval < 1 {
		return 0, 0, fmt.Errorf("acknowledge refused: the last census carries no interval")
	}
	window := 2 * interval
	if window > 180 {
		window = 180
	}
	age := now.Unix() - completed
	if age < 0 || age >= window {
		return 0, 0, fmt.Errorf("acknowledge refused: the last census is not current (age %ds, window %ds); re-run after the next scan", age, window)
	}
	inventory, _ := last["inventory"].([]any)
	for _, raw := range inventory {
		item, _ := raw.(map[string]any)
		if item == nil {
			continue
		}
		itemPid, pidOK := intField(item["pid"])
		if !pidOK || itemPid != pid {
			continue
		}
		if class, _ := item["class"].(string); class != "UNTRACKED" {
			return 0, 0, fmt.Errorf("acknowledge refused: pid %d is %s, not UNTRACKED; only untracked processes are acknowledged", pid, class)
		}
		itemStart, startOK := intField(item["pidStartedAt"])
		if !startOK {
			return 0, 0, fmt.Errorf("acknowledge refused: the census lists pid %d with no start time", pid)
		}
		itemExact, exactOK := intField(item["pidStartedAtExactMicro"])
		if !exactOK || itemExact < 1 {
			return 0, 0, fmt.Errorf("acknowledge refused: the census predates exact birth tokens; re-run after the next scan")
		}
		return itemStart, itemExact, nil
	}
	return 0, 0, fmt.Errorf("acknowledge refused: pid %d is not in the current census inventory", pid)
}
