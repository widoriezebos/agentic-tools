package lease

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/census"
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
	Pgid         int64  `json:"pgid"`
	Runtime      string `json:"runtime"`
	InstanceTag  string `json:"instanceTag"`
	CommandHash  string `json:"commandHash,omitempty"`
	AnnouncedAt  string `json:"announcedAt"`
	OwnerLineage string `json:"ownerLineage,omitempty"`
}

type announcementFile struct {
	Path string
	Ann  Announcement
}

var (
	mainIDPattern      = regexp.MustCompile(`^main-[1-9][0-9]*-[1-9][0-9]*-[0-9a-f]{6}$`)
	commandHashPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)
)

// nonAnnouncementFiles are the mains-directory files that are not main
// announcements and must never be classified as one.
var nonAnnouncementFiles = map[string]bool{
	"worktree-lease.json":        true,
	"worktree-commit-token.json": true,
	"reaped-after-claim.json":    true,
}

// Class values a caller can resolve to.
const (
	ClassMain              = "MAIN"
	ClassDelegate          = "DELEGATE"
	ClassSupervision       = "SUPERVISION"
	ClassAdapterSupervisor = "ADAPTER-SUPERVISOR"
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
		if strings.HasSuffix(name, ".protocol-cursor.json") || nonAnnouncementFiles[name] {
			continue
		}
		var raw map[string]json.RawMessage
		if data, err := os.ReadFile(path); err != nil || json.Unmarshal(data, &raw) != nil {
			if strict {
				return nil, fmt.Errorf("caller classification refused: unreadable announcement %s", name)
			}
			continue
		}
		if !hasAll(raw, "sessionId", "pid", "pidStartedAt", "pgid", "runtime", "instanceTag", "announcedAt") {
			if strict {
				return nil, fmt.Errorf("caller classification refused: invalid announcement schema %s", name)
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

func hasAll(raw map[string]json.RawMessage, keys ...string) bool {
	for _, k := range keys {
		if _, ok := raw[k]; !ok {
			return false
		}
	}
	return true
}

// authenticatedAnnouncement returns the announcement a live pid authenticates
// as its own: same pid, same start time, and a command whose hash matches
// what the record recorded. Nil when the pid is not live or matches nothing.
func authenticatedAnnouncement(pid int64, records []announcementFile) *Announcement {
	id, ok := ProcessIdentity(pid)
	if !ok {
		return nil
	}
	digest := CommandHash(id.Command)
	for i := range records {
		ann := records[i].Ann
		if ann.Pid == pid && ann.PidStartedAt == id.StartedAt && ann.CommandHash == digest {
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
	if data, readErr := os.ReadFile(statePath); readErr == nil {
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
			continue
		}
		var job struct {
			JobId            string           `json:"jobId"`
			Pid              *int64           `json:"pid"`
			PidStartedAt     *int64           `json:"pidStartedAt"`
			CustodyProcesses []supervisedProc `json:"custodyProcesses"`
		}
		if json.Unmarshal(data, &job) != nil || job.JobId == "" {
			continue
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
// DELEGATE, and a supervision or adapter-supervisor ancestor names it as
// such. A caller with no recognised ancestor is a HUMAN.
func Classify(root string, caller int64) (Classification, error) {
	records, err := readAnnouncements(root, true)
	if err != nil {
		return Classification{}, err
	}
	if own := authenticatedAnnouncement(caller, records); own != nil {
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
	seen := map[int64]bool{caller: true}
	current, ok := ParentPid(caller)
	for ok && !seen[current] {
		seen[current] = true
		if ann := authenticatedAnnouncement(current, records); ann != nil {
			return Classification{Class: ClassMain, MainId: ann.MainId, Announcement: ann}, nil
		}
		if command, cok := ProcessCommand(current); cok && census.Runtime(command, signatures) != "" {
			return Classification{Class: ClassDelegate, Pid: current}, nil
		}
		if start, sok := StartedAt(current); sok {
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
