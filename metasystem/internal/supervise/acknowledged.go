package supervise

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
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
