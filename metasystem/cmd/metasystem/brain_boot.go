package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"
	"unicode/utf8"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/brain"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/diskstore"
	dispatchcore "github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/narratordigest"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
)

const minimumBrainContextBytes = 2048

type brainBootLine struct {
	Text  string `json:"text"`
	ID    string `json:"id,omitempty"`
	Group string `json:"group,omitempty"`
}

type brainBootSection struct {
	Status       string          `json:"status"`
	Lines        []brainBootLine `json:"lines,omitempty"`
	Cursor       int64           `json:"cursor,omitempty"`
	PrefixSHA256 string          `json:"prefixSha256,omitempty"`
}

type brainBootOutput struct {
	Declared           bool              `json:"declared"`
	State              brain.State       `json:"state"`
	Payload            string            `json:"payload"`
	Bytes              int               `json:"bytes"`
	Sections           map[string]string `json:"sections"`
	DigestEmitted      bool              `json:"digestEmitted"`
	DigestCursor       int64             `json:"digestCursor"`
	DigestPrefixSHA256 string            `json:"digestPrefixSha256"`
	DeclarationSHA256  string            `json:"declarationSha256,omitempty"`
}

var newBrainBootInputsCommand = func(executable string, args ...string) *exec.Cmd {
	return exec.Command(executable, args...)
}

type brainBootTimer struct {
	C    <-chan time.Time
	Stop func() bool
}

var brainBootNow = time.Now

var newBrainBootTimer = func(duration time.Duration) brainBootTimer {
	timer := time.NewTimer(duration)
	return brainBootTimer{C: timer.C, Stop: timer.Stop}
}

// brainBootDependencies are the boot composer's seams: the ledger identity,
// the optional-input child, and the deadline clock and timer.
type brainBootDependencies struct {
	ledgerIdentity func(string) string
	inputsCommand  func(executable string, args ...string) *exec.Cmd
	now            func() time.Time
	timer          func(time.Duration) brainBootTimer
}

func composeBrainBootMode(root, repo string, bound, deadlineMS int, readOnly bool) (brainBootOutput, error) {
	return composeBrainBootModeWithIdentity(root, repo, bound, deadlineMS, readOnly, goal.ExistingLedgerIdentity)
}

func composeBrainBootModeWithIdentity(root, repo string, bound, deadlineMS int, readOnly bool, ledgerIdentity func(string) string) (brainBootOutput, error) {
	return composeBrainBootWith(root, repo, bound, deadlineMS, readOnly, brainBootDependencies{
		ledgerIdentity: ledgerIdentity, inputsCommand: newBrainBootInputsCommand,
		now: brainBootNow, timer: newBrainBootTimer,
	})
}

func composeBrainBootWith(root, repo string, bound, deadlineMS int, readOnly bool, deps brainBootDependencies) (brainBootOutput, error) {
	started := deps.now()
	state, phaseOne := brain.PhaseOne(root, deps.ledgerIdentity(root), bound)
	if state.State == brain.Undeclared {
		return brainBootOutput{Declared: false}, nil
	}
	if len(phaseOne) > bound {
		return brainBootOutput{}, fmt.Errorf("standing instruction exceeds the declared context bound")
	}

	dir, done, err := diskstore.ScratchDir("metasystem-brain-boot-*")
	if err != nil {
		return brainBootOutput{}, err
	}
	defer done()
	executable, err := os.Executable()
	if err != nil {
		return brainBootOutput{}, err
	}
	cmd := deps.inputsCommand(executable, "brain", "boot-inputs", "--root", root, "--repo", repo, "--dir", dir)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	// The reader is an engine child writing into this process's scratch: it
	// inherits the writer lock and takes its own scratch nested in ours.
	if err := diskstore.PrepareChild(cmd); err != nil {
		return brainBootOutput{}, err
	}
	// The reader answers through its output files; its words are not read.
	if err := cmd.Start(); err != nil {
		return brainBootOutput{}, fmt.Errorf("start optional-input reader: %w", err)
	}
	waited := make(chan error, 1)
	go func() { waited <- cmd.Wait() }()
	remaining := time.Duration(deadlineMS)*time.Millisecond - deps.now().Sub(started)
	timedOut := false
	if remaining <= 0 {
		timedOut = true
	} else {
		timer := deps.timer(remaining)
		select {
		case <-waited:
			timer.Stop()
		case <-timer.C:
			timedOut = true
		}
	}
	if timedOut {
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM)
		killTimer := deps.timer(200 * time.Millisecond)
		select {
		case <-waited:
			killTimer.Stop()
		case <-killTimer.C:
			_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
			<-waited
		}
	}
	names := []string{"asks", "held", "fleet", "digest"}
	sections := make(map[string]brainBootSection, len(names))
	states := make(map[string]string, len(names))
	var missing []string
	for _, name := range names {
		section, readErr := readBrainBootSection(dir, name)
		if readErr != nil {
			states[name] = "skipped"
			missing = append(missing, name)
			continue
		}
		sections[name] = section
		states[name] = section.Status
	}

	deadlineLine := ""
	if len(missing) > 0 {
		deadlineLine = fmt.Sprintf("BOOT DEADLINE: %s not read within %d ms; they are read again at the next session start", strings.Join(missing, ", "), deadlineMS)
	}
	payload := phaseOne
	available := bound - len(payload)
	if deadlineLine != "" {
		available -= appendedBytes(payload, deadlineLine)
	}
	if available < 0 {
		available = 0
	}
	initial := available
	carry := 0
	allocations := []struct {
		name    string
		percent int
	}{{"asks", 30}, {"held", 10}, {"fleet", 25}, {"digest", 35}}
	for index, allocation := range allocations {
		section, ok := sections[allocation.name]
		if !ok {
			continue
		}
		share := initial*allocation.percent/100 + carry
		if index == len(allocations)-1 {
			share = available
		}
		text, cut := fitBrainSection(allocation.name, section, payload, share)
		used := 0
		if text != "" {
			used = appendedBytes(payload, text)
			payload = appendPayload(payload, text)
		}
		if cut && states[allocation.name] == "complete" {
			states[allocation.name] = "cut"
		}
		available -= used
		if available < 0 {
			available = 0
		}
		carry = share - used
		if carry < 0 {
			carry = 0
		}
	}
	if deadlineLine != "" {
		payload = appendPayload(payload, deadlineLine)
	}
	if len(payload) > bound {
		return brainBootOutput{}, fmt.Errorf("composed payload is %d bytes over a %d-byte bound", len(payload), bound)
	}
	declarationSHA := ""
	if state.State == brain.Declared && readOnly {
		encoded, err := json.Marshal(*state.Record)
		if err != nil {
			return brainBootOutput{}, fmt.Errorf("encode brain declaration: %w", err)
		}
		declarationSHA = fmt.Sprintf("%x", sha256.Sum256(encoded))
	}
	if state.State == brain.Declared && !readOnly {
		if err := brain.WriteStatus(root, *state.Record, time.Now().UTC()); err != nil {
			return brainBootOutput{}, fmt.Errorf("write brain status: %w", err)
		}
	}

	digest := sections["digest"]
	digestEmitted := (states["digest"] == "complete" || states["digest"] == "cut") && digest.Cursor > 0
	return brainBootOutput{
		Declared: true, State: state.State, Payload: payload, Bytes: len(payload), Sections: states,
		DigestEmitted: digestEmitted, DigestCursor: digest.Cursor, DigestPrefixSHA256: digest.PrefixSHA256,
		DeclarationSHA256: declarationSHA,
	}, nil
}

func appendPayload(existing, section string) string {
	if section == "" {
		return existing
	}
	if existing == "" || strings.HasSuffix(existing, "\n") {
		return existing + section
	}
	return existing + "\n" + section
}

func appendedBytes(existing, section string) int {
	return len(appendPayload(existing, section)) - len(existing)
}

func readBrainBootSection(dir, name string) (brainBootSection, error) {
	data, err := os.ReadFile(filepath.Join(dir, name+".json"))
	if err != nil {
		return brainBootSection{}, err
	}
	var section brainBootSection
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&section); err != nil {
		return brainBootSection{}, err
	}
	if section.Status != "complete" && section.Status != "error" {
		return brainBootSection{}, fmt.Errorf("invalid section status %q", section.Status)
	}
	return section, nil
}

func fitBrainSection(name string, section brainBootSection, existing string, share int) (string, bool) {
	if len(section.Lines) == 0 {
		return "", false
	}
	header := map[string]string{
		"asks": "ASKS AWAITING WIDO:", "held": "HELD HERE:", "fleet": "FLEET:",
		"digest": "NARRATOR DIGEST since the brain last booted:",
	}[name]
	lines := append([]brainBootLine(nil), section.Lines...)
	removed := 0
	for {
		texts := make([]string, 0, len(lines)+2)
		texts = append(texts, header)
		for _, line := range lines {
			texts = append(texts, line.Text)
		}
		if removed > 0 {
			switch name {
			case "asks":
				id := "<id>"
				if len(section.Lines) > len(lines) && section.Lines[len(lines)].ID != "" {
					id = section.Lines[len(lines)].ID
				}
				texts = append(texts, fmt.Sprintf("%d more; run metasystem question show %s", removed, id))
			case "digest":
				texts = append([]string{header, fmt.Sprintf("%d older lines cut; read records/narrator-digest.log", removed)}, texts[1:]...)
			default:
				texts = append(texts, fmt.Sprintf("%d more", removed))
			}
		}
		candidate := strings.Join(texts, "\n")
		if appendedBytes(existing, candidate) <= share {
			return candidate, removed > 0
		}
		if len(lines) == 0 {
			return "", true
		}
		removeAt := len(lines) - 1
		if name == "digest" {
			removeAt = 0
		} else if name == "fleet" {
			removeAt = firstGroup(lines, "job")
			if removeAt < 0 {
				removeAt = firstGroup(lines, "claim")
			}
			if removeAt < 0 {
				removeAt = len(lines) - 1
			}
		}
		lines = append(lines[:removeAt], lines[removeAt+1:]...)
		removed++
	}
}

func firstGroup(lines []brainBootLine, group string) int {
	for index, line := range lines {
		if line.Group == group {
			return index
		}
	}
	return -1
}

func runBrainBootInputs(args []string, stdout, stderr io.Writer) int {
	flags := newFlagSet("brain boot-inputs", stdout, stderr)
	root := pathFlag(flags, "root", "", "checkout state root")
	repo := pathFlag(flags, "repo", "", "checkout containing records")
	dir := flags.String("dir", "", "parent-owned section directory")
	if flags.Parse(args) != nil || !requireFlags(flags, stderr, "root", "repo", "dir") || *root == "" || *repo == "" || *dir == "" {
		fmt.Fprintln(stderr, "brain boot-inputs needs --root, --repo, and --dir")
		return 2
	}
	if err := writeBrainBootInputs(*root, *repo, *dir, defaultBrainBootInputReaders()); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return 0
}

// brainBootInputReaders are the optional-input child's reads of the ledger
// and the digest layout.
type brainBootInputReaders struct {
	machine         func(string) (string, error)
	resolveEndpoint func(string) (goal.Endpoint, error)
	project         func(goal.Endpoint) (goal.Projection, error)
	resolveLayout   func(string) (stateroot.Layout, error)
}

func defaultBrainBootInputReaders() brainBootInputReaders {
	return brainBootInputReaders{
		machine: goal.ResolveMachine, resolveEndpoint: goal.ResolveEndpoint,
		project: func(endpoint goal.Endpoint) (goal.Projection, error) {
			return goal.Project(endpoint, false, time.Now().UTC())
		},
		resolveLayout: stateroot.ResolveLayout,
	}
}

// writeBrainBootInputs publishes the four optional sections in order, so a
// deadline keeps every section completed before it.
func writeBrainBootInputs(root, repo, dir string, readers brainBootInputReaders) error {
	if err := writeBrainBootSection(dir, "asks", readBrainAsks(root)); err != nil {
		return err
	}
	held, fleet := readBrainFleetWith(root, readers)
	if err := writeBrainBootSection(dir, "held", held); err != nil {
		return err
	}
	if err := writeBrainBootSection(dir, "fleet", fleet); err != nil {
		return err
	}
	return writeBrainBootSection(dir, "digest", readBrainDigestWithLayoutReader(repo, readers.resolveLayout))
}

func writeBrainBootSection(dir, name string, section brainBootSection) error {
	encoded, err := json.Marshal(section)
	if err != nil {
		return err
	}
	_, err = atomicfile.WriteText(filepath.Join(dir, name+".json"), string(encoded)+"\n", dir)
	return err
}

func readBrainAsks(root string) brainBootSection {
	section := brainBootSection{Status: "complete"}
	questions, unreadable := channel.WalkOpenQuestions(root)
	for _, q := range questions {
		line := fmt.Sprintf("%s goal %s %s from %s since %s: %s", q.ID, q.Goal, q.Kind, q.Machine, q.OpenedAt.UTC().Format(time.RFC3339), q.Wants)
		section.Lines = append(section.Lines, brainBootLine{Text: clipBrainLine(line), ID: q.ID})
	}
	if len(unreadable) > 0 {
		section.Status = "error"
		section.Lines = append(section.Lines, brainBootLine{Text: fmt.Sprintf("%d unreadable question files; run metasystem question list", len(unreadable))})
	}
	return section
}

func readBrainFleetWith(root string, readers brainBootInputReaders) (brainBootSection, brainBootSection) {
	held := brainBootSection{Status: "complete"}
	fleet := brainBootSection{Status: "complete"}
	machine, machineErr := readers.machine(root)
	endpoint, endpointErr := readers.resolveEndpoint(root)
	if machineErr != nil || endpointErr != nil {
		reason := errors.Join(machineErr, endpointErr)
		line := fmt.Sprintf("LEDGER unreadable (%v); run metasystem goal list --root %s", reason, root)
		held.Status, fleet.Status = "error", "error"
		held.Lines = append(held.Lines, brainBootLine{Text: line})
		fleet.Lines = append(fleet.Lines, brainBootLine{Text: line})
	} else if projection, projectErr := readers.project(endpoint); projectErr != nil {
		line := fmt.Sprintf("LEDGER unreadable (%v); run metasystem goal list --root %s", projectErr, root)
		held.Status, fleet.Status = "error", "error"
		held.Lines = append(held.Lines, brainBootLine{Text: line})
		fleet.Lines = append(fleet.Lines, brainBootLine{Text: line})
	} else {
		ids := make([]string, 0, len(projection.Tree.Live))
		for id := range projection.Tree.Live {
			ids = append(ids, id)
		}
		sort.Strings(ids)
		queued := 0
		for _, id := range ids {
			item := projection.Tree.Live[id]
			switch {
			case item.Claimed != nil:
				line := clipBrainLine(fmt.Sprintf("%s claimed by %s since %s: %s", id, item.Claimed.Machine, item.Claimed.At, item.NextStep))
				fleet.Lines = append(fleet.Lines, brainBootLine{Text: line, ID: id, Group: "claim"})
				if item.Claimed.Machine == machine {
					held.Lines = append(held.Lines, brainBootLine{Text: line, ID: id})
				}
			case item.Approved != nil:
				fleet.Lines = append(fleet.Lines, brainBootLine{Text: clipBrainLine(fmt.Sprintf("%s approved and awaiting a node: %s", id, item.NextStep)), ID: id})
			default:
				queued++
			}
		}
		if len(held.Lines) > 0 {
			held.Lines = append(held.Lines, brainBootLine{Text: fmt.Sprintf("release them to a node: metasystem goal release --root %s --id <id>", root)})
		}
		if queued > 0 {
			fleet.Lines = append(fleet.Lines, brainBootLine{Text: fmt.Sprintf("%d queued unapproved goals", queued)})
		}
	}

	jobPaths, _ := filepath.Glob(filepath.Join(root, "artifacts", "agents", "jobs", "*.json"))
	unreadableJobs := 0
	for _, path := range jobPaths {
		data, err := os.ReadFile(path)
		var record struct {
			JobID  string `json:"jobId"`
			Status string `json:"status"`
			Role   string `json:"role"`
			GoalID any    `json:"goalId"`
		}
		if err != nil || json.Unmarshal(data, &record) != nil || record.Status == "" {
			unreadableJobs++
			continue
		}
		if dispatchcore.TerminalStatus(record.Status) {
			continue
		}
		goalID := "none"
		switch value := record.GoalID.(type) {
		case string:
			if value != "" {
				goalID = value
			}
		}
		if record.JobID == "" {
			record.JobID = strings.TrimSuffix(filepath.Base(path), ".json")
		}
		fleet.Lines = append(fleet.Lines, brainBootLine{Text: clipBrainLine(fmt.Sprintf("job %s %s %s goal %s", record.JobID, record.Status, record.Role, goalID)), ID: record.JobID, Group: "job"})
	}
	if unreadableJobs > 0 {
		fleet.Status = "error"
		fleet.Lines = append(fleet.Lines, brainBootLine{Text: fmt.Sprintf("%d unreadable job records under artifacts/agents/jobs", unreadableJobs)})
	}

	censusPath := filepath.Join(root, "artifacts", "agents", "supervision", "last-census.json")
	censusData, censusErr := os.ReadFile(censusPath)
	var census struct {
		Verdict          string         `json:"verdict"`
		CompletedAtEpoch int64          `json:"completedAtEpoch"`
		Counts           map[string]int `json:"counts"`
	}
	if censusErr != nil || json.Unmarshal(censusData, &census) != nil || census.Verdict == "" || census.Counts == nil {
		if censusErr == nil {
			censusErr = fmt.Errorf("malformed census verdict")
		}
		fleet.Status = "error"
		fleet.Lines = append(fleet.Lines, brainBootLine{Text: fmt.Sprintf("CENSUS absent or unreadable (%v); run metasystem system status", censusErr)})
	} else {
		age := "age unknown"
		if census.CompletedAtEpoch > 0 {
			age = time.Since(time.Unix(census.CompletedAtEpoch, 0)).Round(time.Second).String() + " old"
		}
		fleet.Lines = append(fleet.Lines, brainBootLine{Text: fmt.Sprintf("census %s custody %d announced %d untracked %d, %s", census.Verdict, census.Counts["CUSTODY"], census.Counts["ANNOUNCED"], census.Counts["UNTRACKED"], age)})
	}
	return held, fleet
}

func readBrainDigestWithLayoutReader(repo string, resolveLayout func(string) (stateroot.Layout, error)) brainBootSection {
	section := brainBootSection{Status: "complete"}
	pending, err := narratordigest.PendingWithLayoutReader(repo, resolveLayout, "brain")
	if err != nil {
		section.Status = "error"
		section.Lines = append(section.Lines, brainBootLine{Text: fmt.Sprintf("DIGEST unreadable (%v); read records/narrator-digest.log by hand; the brain cursor was not advanced", err)})
		return section
	}
	section.Cursor, section.PrefixSHA256 = pending.Cursor, pending.PrefixSHA256
	message := pending.Message
	if first, rest, ok := strings.Cut(message, "\n"); ok && strings.HasPrefix(first, "NARRATOR DIGEST") {
		message = rest
	}
	for _, line := range strings.Split(strings.TrimSuffix(message, "\n"), "\n") {
		if line != "" {
			section.Lines = append(section.Lines, brainBootLine{Text: line})
		}
	}
	return section
}

func clipBrainLine(line string) string {
	line = strings.ReplaceAll(strings.ReplaceAll(line, "\r", " "), "\n", " ")
	if len(line) <= 160 {
		return line
	}
	cut := 157
	for cut > 0 && !utf8.ValidString(line[:cut]) {
		cut--
	}
	return line[:cut] + "…"
}

// brainStartDelivered records that a published SessionStart delivered the
// read-only brain packet: the declaration it observed must still be current,
// the brain's status is written, and the brain digest advances to the cursor
// the packet emitted (cursor < 0 when it emitted none).
func brainStartDelivered(root, repo, declarationSHA string, digestCursor int64, digestPrefix string, ledgerIdentity func(string) string, resolveLayout func(string) (stateroot.Layout, error)) error {
	if root == "" || repo == "" || declarationSHA == "" {
		return fmt.Errorf("the delivery needs a root, a repository and a declaration")
	}
	if (digestCursor >= 0) != (digestPrefix != "") {
		return errors.New("the narrator summary delivery needs both its cursor and its prefix, or neither")
	}
	state := brain.Read(root, ledgerIdentity(root))
	if state.State != brain.Declared || state.Record == nil {
		return fmt.Errorf("the delivered declaration is no longer current")
	}
	encoded, err := json.Marshal(*state.Record)
	if err != nil {
		return err
	}
	if fmt.Sprintf("%x", sha256.Sum256(encoded)) != declarationSHA {
		return fmt.Errorf("the declaration changed before delivery acknowledgment")
	}
	if err := brain.WriteStatus(root, *state.Record, time.Now().UTC()); err != nil {
		return fmt.Errorf("write brain status: %w", err)
	}
	if digestCursor >= 0 {
		if err := narratordigest.AdvanceWithLayoutReader(repo, digestCursor, digestPrefix, resolveLayout, "brain"); err != nil {
			return fmt.Errorf("the coordinator's narrator summary could not be advanced: %w", err)
		}
	}
	return nil
}
