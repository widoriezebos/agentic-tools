package lease

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/census"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
)

// Announcement is a main's identity record under artifacts/agents/mains. The
// base fields identify a live process; mainId and commandHash are what let it
// authenticate — a record missing them names a process but authenticates no
// main. ownerLineage, when present, is the logical writer the process belongs
// to, so a mission's successive processes are one owner rather than rivals.
type Announcement struct {
	SessionId    string `json:"sessionId"`
	MainId       string `json:"mainId,omitempty"`
	Pid          int64  `json:"pid"`
	PidStartedAt int64  `json:"pidStartedAt"`
	// The clock-step-immune identity pair (issue #1); zero values on
	// announcements that predate it fall back to the seconds comparison.
	PidStartTicks int64  `json:"pidStartTicks,omitempty"`
	BootID        string `json:"bootId,omitempty"`
	Pgid          int64  `json:"pgid"`
	Runtime       string `json:"runtime"`
	InstanceTag   string `json:"instanceTag"`
	CommandHash   string `json:"commandHash,omitempty"`
	AnnouncedAt   string `json:"announcedAt"`
	OwnerLineage  string `json:"ownerLineage,omitempty"`
}

type announcementFile struct {
	Path string
	Ann  Announcement
}

var (
	// The identity grammars and the announcement/protocol-file split have
	// ONE home in census (review lease-census-4).
	mainIDPattern      = census.MainIDRe
	commandHashPattern = census.CommandHashRe
)

// Class values a caller can resolve to.
const (
	ClassMain              = "MAIN"
	ClassDelegate          = "DELEGATE"
	ClassSupervision       = "SUPERVISION"
	ClassAdapterSupervisor = "ADAPTER-SUPERVISOR"
	ClassSteward           = "STEWARD"
	ClassHuman             = "HUMAN"
)

// Classification is who a caller is relative to this checkout.
type Classification struct {
	Class        string
	MainId       string
	Pid          int64
	JobId        string
	Announcement *Announcement
}

// readAnnouncements lists the valid main announcements. In strict mode a
// malformed or tampering-shaped record refuses the whole read (a classifier
// must not silently ignore a bad identity file); in lax mode — used by
// announce and retire, which only need to find or replace one record — a bad
// file is skipped. A record without the one-writer identity fields is always
// skipped: it is an older announcement that authenticates nobody, not an
// error, so one such file cannot refuse every write in a checkout.
func readAnnouncements(root string, strict bool) ([]announcementFile, error) {
	dir := filepath.Join(root, "artifacts/agents/mains")
	entries, err := filepath.Glob(filepath.Join(dir, "*.json"))
	if err != nil {
		return nil, err
	}
	sort.Strings(entries)
	var out []announcementFile
	for _, path := range entries {
		name := filepath.Base(path)
		if !census.IsAnnouncementFile(name) {
			continue
		}
		var raw map[string]json.RawMessage
		if data, err := os.ReadFile(path); err != nil || json.Unmarshal(data, &raw) != nil {
			if strict {
				return nil, fmt.Errorf("caller classification refused: unreadable announcement %s", name)
			}
			continue
		}
		if err := census.ValidateAnnouncementKeys(func(visit func(string) bool) {
			for key := range raw {
				visit(key)
			}
		}); err != nil {
			if strict {
				return nil, fmt.Errorf("caller classification refused: invalid announcement schema %s: %v", name, err)
			}
			continue
		}
		_, hasMainID := raw["mainId"]
		_, hasHash := raw["commandHash"]
		if !hasMainID && !hasHash {
			continue // Names a process, authenticates nobody.
		}
		var ann Announcement
		if data, _ := json.Marshal(raw); json.Unmarshal(data, &ann) != nil {
			if strict {
				return nil, fmt.Errorf("caller classification refused: invalid announcement schema %s", name)
			}
			continue
		}
		if !mainIDPattern.MatchString(ann.MainId) {
			if strict {
				return nil, fmt.Errorf("caller classification refused: invalid main identity %s", name)
			}
			continue
		}
		if !commandHashPattern.MatchString(ann.CommandHash) {
			if strict {
				return nil, fmt.Errorf("caller classification refused: invalid command hash %s", name)
			}
			continue
		}
		out = append(out, announcementFile{Path: path, Ann: ann})
	}
	return out, nil
}

// authenticatedAnnouncement returns the announcement a live pid authenticates
// as its own: same pid, same start time, and a command whose hash matches
// what the record recorded. Nil when the pid is not live or matches nothing.
func authenticatedAnnouncement(pid int64, records []announcementFile, probe identity.FixtureProbe) *Announcement {
	id, ok := ProcessIdentity(pid, probe)
	if !ok {
		return nil
	}
	digest := CommandHash(id.Command)
	for i := range records {
		ann := records[i].Ann
		if ann.Pid != pid || ann.CommandHash != digest {
			continue
		}
		// The pair decides when both sides carry it (issue #1: the
		// btime-derived second drifts on time-synced guests); legacy
		// announcements keep the seconds rule.
		if ann.PidStartTicks > 0 && ann.BootID != "" && id.StartTicks > 0 && id.BootID != "" {
			if ann.PidStartTicks == id.StartTicks && ann.BootID == id.BootID {
				return &records[i].Ann
			}
			continue
		}
		if ann.PidStartedAt == id.StartedAt {
			return &records[i].Ann
		}
	}
	return nil
}

// allAdapterSignatures compiles the delegate signatures from every runtime
// adapter (all of them, not only the configured runtimes: a delegate of any
// installed runtime must be recognised as a delegate). runtime-common.sh is
// shared code, not an adapter.
func allAdapterSignatures(root string) ([]census.Signature, error) {
	dir := filepath.Join(root, "scripts/agents/adapters")
	entries, err := filepath.Glob(filepath.Join(dir, "*.sh"))
	if err != nil {
		return nil, err
	}
	sort.Strings(entries)
	var sigs []census.Signature
	for _, path := range entries {
		name := filepath.Base(path)
		if name == "runtime-common.sh" {
			continue
		}
		text, err := census.SignatureText(path)
		if err != nil {
			return nil, fmt.Errorf("caller classification refused: signature registry failed for %s", name)
		}
		matches, excludes := census.ParseSignatureText(text)
		sig, err := census.CompileSignature(strings.TrimSuffix(name, ".sh"), matches, excludes)
		if err != nil {
			return nil, fmt.Errorf("caller classification refused: invalid signature registry for %s", name)
		}
		sigs = append(sigs, sig)
	}
	return sigs, nil
}

type procKey struct {
	pid     int64
	started int64
}

// custodyIdentities returns the (pid, start) pairs that the supervision state
// and the job records own. A caller whose ancestry passes through one of
// these is an internal helper of an already-authorised operation, not a
// foreign writer, so it is classified rather than refused.
func custodyIdentities(root string) (supervision map[procKey]bool, adapters map[procKey]string, err error) {
	supervision = map[procKey]bool{}
	adapters = map[procKey]string{}

	statePath := filepath.Join(root, "artifacts/agents/supervision/state.json")
	data, readErr := os.ReadFile(statePath)
	if readErr != nil && !os.IsNotExist(readErr) {
		// FAIL CLOSED (review lease-census-1): a state file that EXISTS but
		// cannot be read is not the same as supervision being unarmed. The
		// old code silently skipped here, custody came back empty, Classify
		// fell through to HUMAN — and RequireHolder passes HUMAN through
		// the write gate ungated. A permissions mishap must refuse, exactly
		// as the neighbouring parse failure always has.
		return nil, nil, fmt.Errorf("caller classification refused: supervision state is unreadable: %v", readErr)
	}
	if readErr == nil {
		var state struct {
			Owner      *supervisedProc            `json:"owner"`
			Components map[string]*supervisedProc `json:"components"`
		}
		if json.Unmarshal(data, &state) != nil {
			return nil, nil, fmt.Errorf("caller classification refused: supervision state is unreadable")
		}
		procs := []*supervisedProc{state.Owner}
		for _, c := range state.Components {
			procs = append(procs, c)
		}
		for _, p := range procs {
			if p != nil && p.Pid > 0 {
				supervision[procKey{p.Pid, p.PidStartedAt}] = true
			}
		}
	}

	jobs, _ := filepath.Glob(filepath.Join(root, "artifacts/agents/jobs", "*.json"))
	for _, path := range jobs {
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			// A record that vanished between glob and read is a race, not a
			// failure; anything else refuses — a record this pass cannot
			// read may carry the very custody that gates this caller.
			if os.IsNotExist(readErr) {
				continue
			}
			return nil, nil, fmt.Errorf("caller classification refused: job record unreadable: %s: %v", filepath.Base(path), readErr)
		}
		var job struct {
			JobId            string           `json:"jobId"`
			Pid              *int64           `json:"pid"`
			PidStartedAt     *int64           `json:"pidStartedAt"`
			CustodyProcesses []supervisedProc `json:"custodyProcesses"`
		}
		if json.Unmarshal(data, &job) != nil || job.JobId == "" {
			// Corrupt records refuse like corrupt state: silently dropping
			// custody is the same fail-open with a different spelling.
			return nil, nil, fmt.Errorf("caller classification refused: job record corrupt or unidentified: %s", filepath.Base(path))
		}
		if job.Pid != nil && job.PidStartedAt != nil {
			adapters[procKey{*job.Pid, *job.PidStartedAt}] = job.JobId
		}
		for _, p := range job.CustodyProcesses {
			if p.Pid > 0 {
				adapters[procKey{p.Pid, p.PidStartedAt}] = job.JobId
			}
		}
	}
	return supervision, adapters, nil
}

type supervisedProc struct {
	Pid          int64 `json:"pid"`
	PidStartedAt int64 `json:"pidStartedAt"`
}

// Classify resolves who the caller is. It first checks whether the caller
// itself authenticates a main, then walks its ancestry: a MAIN ancestor makes
// the caller that main's work, a delegate-signed ancestor makes it a
// DELEGATE, a supervision or adapter-supervisor ancestor names it as such,
// and a process running this repository's installed steward binary — proven
// by the installation identity record — is the STEWARD. A caller with no
// recognised ancestor is a HUMAN.
func Classify(root string, caller int64) (Classification, error) {
	probe, err := fixtureProbe(root)
	if err != nil {
		return Classification{}, err
	}
	records, err := readAnnouncements(root, true)
	if err != nil {
		return Classification{}, err
	}
	if own := authenticatedAnnouncement(caller, records, probe); own != nil {
		return Classification{Class: ClassMain, MainId: own.MainId, Announcement: own}, nil
	}
	signatures, err := allAdapterSignatures(root)
	if err != nil {
		return Classification{}, err
	}
	supervision, adapters, err := custodyIdentities(root)
	if err != nil {
		return Classification{}, err
	}
	stewardBinary := verifiedStewardBinary(root)
	if stewardBinary != "" {
		if command, cok := ProcessCommand(caller, probe); cok && sameExecutable(commandExecutable(command), stewardBinary) {
			return Classification{Class: ClassSteward, Pid: caller}, nil
		}
	}
	seen := map[int64]bool{caller: true}
	current, ok := ParentPid(caller)
	for ok && !seen[current] {
		seen[current] = true
		if ann := authenticatedAnnouncement(current, records, probe); ann != nil {
			return Classification{Class: ClassMain, MainId: ann.MainId, Announcement: ann}, nil
		}
		if command, cok := ProcessCommand(current, probe); cok {
			if census.Runtime(command, signatures) != "" {
				return Classification{Class: ClassDelegate, Pid: current}, nil
			}
			if stewardBinary != "" && sameExecutable(commandExecutable(command), stewardBinary) {
				return Classification{Class: ClassSteward, Pid: current}, nil
			}
		}
		if start, sok := StartedAt(current, probe); sok {
			key := procKey{current, start}
			if supervision[key] {
				return Classification{Class: ClassSupervision, Pid: current}, nil
			}
			if jobID, isAdapter := adapters[key]; isAdapter {
				return Classification{Class: ClassAdapterSupervisor, Pid: current, JobId: jobID}, nil
			}
		}
		current, ok = ParentPid(current)
	}
	return Classification{Class: ClassHuman}, nil
}

// verifiedStewardBinary authenticates this repository's steward
// installation and returns its binary path, or empty when no valid
// installation exists. Verification failures mean "not the steward" —
// the caller keeps walking toward its other classifications.
func verifiedStewardBinary(root string) string {
	top, err := filepath.Abs(root)
	if err != nil {
		return ""
	}
	idPath, err := steward.IdentityPath(top)
	if err != nil {
		return ""
	}
	id, err := steward.VerifyIdentity(idPath, top)
	if err != nil {
		return ""
	}
	return id.InstallPath
}

// sameExecutable compares two executable paths with symlinks
// resolved on both sides — the process table reports resolved paths
// (on darwin, /var is /private/var) while the installation records
// the path as given.
func sameExecutable(a, b string) bool {
	if a == b {
		return true
	}
	ra, errA := filepath.EvalSymlinks(a)
	rb, errB := filepath.EvalSymlinks(b)
	return errA == nil && errB == nil && ra == rb
}

// commandExecutable is the command line's first token — the
// executable a process was launched as.
func commandExecutable(command string) string {
	if i := strings.IndexAny(command, " \t"); i >= 0 {
		return command[:i]
	}
	return command
}
