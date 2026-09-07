// Package brain owns the checkout-local brain designation and its host-local
// compare-and-swap pointer. It deliberately knows nothing about commands,
// runtimes, goals, or network transports.
package brain

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lock"
)

const (
	Schema          = 1
	LedgerBytes     = 26
	MachineBytes    = 32
	DeclaredByBytes = 64
	DeclaredAtBytes = 20
	HeaderBytes     = 320

	PacketRelativePath = "records/misc/fleet-coordinator-brain-role-packet.md"
)

type State string

const (
	Declared   State = "declared"
	Undeclared State = "undeclared"
	Corrupt    State = "corrupt"
)

type Record struct {
	Schema     int    `json:"schema"`
	Ledger     string `json:"ledger"`
	Machine    string `json:"machine"`
	DeclaredBy string `json:"declaredBy"`
	DeclaredAt string `json:"declaredAt"`
}

type ReadResult struct {
	State  State   `json:"state"`
	Record *Record `json:"record,omitempty"`
	Reason string  `json:"reason,omitempty"`
}

type StatusRecord struct {
	Line         string `json:"line"`
	ProducedAt   string `json:"producedAt"`
	LastPostedAt string `json:"lastPostedAt"`
}

func Path(stateRoot string) string {
	return filepath.Join(stateRoot, "artifacts", "agents", "brain.json")
}

func StatusPath(stateRoot string) string {
	return filepath.Join(stateRoot, "artifacts", "agents", "brain-status.json")
}

func WriteStatus(stateRoot string, record Record, now time.Time) error {
	status := StatusRecord{
		Line:       StatusLine(record),
		ProducedAt: now.UTC().Format(time.RFC3339),
	}
	if data, err := os.ReadFile(StatusPath(stateRoot)); err == nil {
		var prior StatusRecord
		if json.Unmarshal(data, &prior) == nil {
			status.LastPostedAt = prior.LastPostedAt
		}
	}
	encoded, err := json.Marshal(status)
	if err != nil {
		return err
	}
	_, err = atomicfile.WriteText(StatusPath(stateRoot), string(encoded)+"\n", stateRoot)
	return err
}

func StatusLine(record Record) string {
	return fmt.Sprintf("BRAIN: %s for ledger %s since %s", record.Machine, record.Ledger, record.DeclaredAt)
}

func ReadStatus(stateRoot string) (StatusRecord, error) {
	data, err := os.ReadFile(StatusPath(stateRoot))
	if err != nil {
		return StatusRecord{}, err
	}
	var status StatusRecord
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&status); err != nil {
		return StatusRecord{}, err
	}
	if err := requireJSONEnd(decoder); err != nil {
		return StatusRecord{}, err
	}
	return status, nil
}

func StatusDue(stateRoot string, now time.Time) bool {
	status, err := ReadStatus(stateRoot)
	if err != nil || status.LastPostedAt == "" {
		return true
	}
	posted, err := time.Parse(time.RFC3339, status.LastPostedAt)
	return err != nil || posted.Before(now.UTC().Add(-4*time.Hour))
}

func MarkStatusPosted(stateRoot string, now time.Time) error {
	status, err := ReadStatus(stateRoot)
	if err != nil {
		return err
	}
	status.LastPostedAt = now.UTC().Format(time.RFC3339)
	encoded, err := json.Marshal(status)
	if err != nil {
		return err
	}
	_, err = atomicfile.WriteText(StatusPath(stateRoot), string(encoded)+"\n", stateRoot)
	return err
}

func Read(stateRoot, ledgerIdentity string) ReadResult {
	data, err := os.ReadFile(Path(stateRoot))
	if os.IsNotExist(err) {
		return ReadResult{State: Undeclared}
	}
	if err != nil {
		return corrupt("unreadable: " + err.Error())
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var record Record
	if err := decoder.Decode(&record); err != nil {
		return corrupt("not JSON: " + err.Error())
	}
	if err := requireJSONEnd(decoder); err != nil {
		return corrupt("not JSON: " + err.Error())
	}
	if reason := validate(record, ledgerIdentity); reason != "" {
		return corrupt(reason)
	}
	return ReadResult{State: Declared, Record: &record}
}

func requireJSONEnd(decoder *json.Decoder) error {
	var extra any
	if err := decoder.Decode(&extra); errors.Is(err, io.EOF) {
		return nil
	} else if err != nil {
		return err
	}
	return errors.New("more than one JSON value")
}

func corrupt(reason string) ReadResult { return ReadResult{State: Corrupt, Reason: reason} }

func validate(record Record, ledgerIdentity string) string {
	if record.Schema != Schema {
		return fmt.Sprintf("wrong schema %d (want 1)", record.Schema)
	}
	if len(record.Ledger) != LedgerBytes {
		return fmt.Sprintf("ledger must be exactly %d bytes", LedgerBytes)
	}
	if ledgerIdentity == "" || record.Ledger != ledgerIdentity {
		return fmt.Sprintf("ledger %q is not this checkout's ledger %q", record.Ledger, ledgerIdentity)
	}
	if reason := validateMachine(record.Machine); reason != "" {
		return reason
	}
	if reason := validateDeclaredBy(record.DeclaredBy); reason != "" {
		return reason
	}
	if len(record.DeclaredAt) != DeclaredAtBytes {
		return fmt.Sprintf("declaredAt must be the fixed %d-byte RFC3339 UTC form", DeclaredAtBytes)
	}
	parsed, err := time.Parse(time.RFC3339, record.DeclaredAt)
	if err != nil || parsed.UTC().Format(time.RFC3339) != record.DeclaredAt {
		return "declaredAt is not the fixed 20-byte RFC3339 UTC form"
	}
	return ""
}

func validateMachine(machine string) string {
	if machine == "" {
		return "machine is missing"
	}
	if len(machine) > MachineBytes {
		return fmt.Sprintf("machine exceeds its %d-byte cap", MachineBytes)
	}
	if !utf8.ValidString(machine) || strings.IndexFunc(machine, func(r rune) bool { return r <= ' ' || r == 0x7f }) >= 0 {
		return "machine must be one word with no whitespace or control characters"
	}
	return ""
}

func validateDeclaredBy(name string) string {
	if name == "" {
		return "declaredBy is missing"
	}
	if len(name) > DeclaredByBytes {
		return fmt.Sprintf("declaredBy exceeds its %d-byte cap", DeclaredByBytes)
	}
	if !utf8.ValidString(name) || strings.IndexFunc(name, func(r rune) bool { return r < ' ' || r == 0x7f }) >= 0 {
		return "declaredBy must be one line with no control characters"
	}
	return ""
}

func ValidateDeclaration(ledger, machine, declaredBy, declaredAt string) error {
	record := Record{Schema: Schema, Ledger: ledger, Machine: machine, DeclaredBy: declaredBy, DeclaredAt: declaredAt}
	if reason := validate(record, ledger); reason != "" {
		return errors.New(reason)
	}
	return nil
}

func RegistryHome() (string, error) {
	if override := os.Getenv("METASYSTEM_SUPERVISION_REGISTRY_HOME"); override != "" {
		if !filepath.IsAbs(override) {
			return "", fmt.Errorf("METASYSTEM_SUPERVISION_REGISTRY_HOME must name an absolute run-scoped home")
		}
		return filepath.Clean(override), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return home, nil
}

func PointerPath(registryHome, ledgerIdentity string) string {
	return filepath.Join(registryHome, ".metasystem", "brain", ledgerIdentity)
}

type DeclareOptions struct {
	StateRoot      string
	RegistryHome   string
	LedgerIdentity string
	Machine        string
	DeclaredBy     string
	Now            time.Time
}

func Declare(options DeclareOptions) (Record, error) {
	checkout, err := canonicalPath(options.StateRoot)
	if err != nil {
		return Record{}, err
	}
	stamp := options.Now.UTC().Format(time.RFC3339)
	record := Record{Schema: Schema, Ledger: options.LedgerIdentity, Machine: options.Machine, DeclaredBy: options.DeclaredBy, DeclaredAt: stamp}
	if reason := validate(record, options.LedgerIdentity); reason != "" {
		return Record{}, errors.New(reason)
	}
	current := Read(checkout, options.LedgerIdentity)
	if current.State == Declared {
		return Record{}, fmt.Errorf("this checkout is already the brain of ledger %s, declared by %s at %s; withdraw it first: metasystem brain withdraw --root %s --by <name>", current.Record.Ledger, current.Record.DeclaredBy, current.Record.DeclaredAt, checkout)
	}
	if current.State == Corrupt {
		return Record{}, errors.New(RemedialRefusal(current.Reason, checkout))
	}
	pointer := PointerPath(options.RegistryHome, options.LedgerIdentity)
	held, err := acquire(pointer+".lock.d", 5*time.Second)
	if err != nil {
		return Record{}, fmt.Errorf("another brain declaration is in progress on this host (%v); retry", err)
	}
	defer held.Release()
	if data, readErr := os.ReadFile(pointer); readErr == nil {
		other := strings.TrimSpace(string(data))
		if other != "" && other != checkout {
			otherState := Read(other, options.LedgerIdentity)
			if otherState.State == Declared {
				return Record{}, fmt.Errorf("this host already has a brain for ledger %s at %s; one brain per host per fleet; withdraw it there first", options.LedgerIdentity, other)
			}
		}
	} else if !os.IsNotExist(readErr) {
		return Record{}, fmt.Errorf("read brain host pointer: %w", readErr)
	}
	if _, err := atomicfile.WriteText(pointer, checkout+"\n", options.RegistryHome); err != nil {
		return Record{}, fmt.Errorf("write brain host pointer: %w", err)
	}
	encoded, err := json.Marshal(record)
	if err != nil {
		return Record{}, err
	}
	if _, err := atomicfile.WriteText(Path(checkout), string(encoded)+"\n", checkout); err != nil {
		return Record{}, fmt.Errorf("write brain declaration: %w", err)
	}
	return record, nil
}

func Withdraw(stateRoot, registryHome, ledgerIdentity string) (bool, error) {
	checkout, err := canonicalPath(stateRoot)
	if err != nil {
		return false, err
	}
	if ledgerIdentity == "" {
		if err := os.Remove(Path(checkout)); err == nil {
			return true, nil
		} else if os.IsNotExist(err) {
			return false, nil
		} else {
			return false, err
		}
	}
	pointer := PointerPath(registryHome, ledgerIdentity)
	held, err := acquire(pointer+".lock.d", 5*time.Second)
	if err != nil {
		return false, fmt.Errorf("another brain declaration is in progress on this host (%v); retry", err)
	}
	defer held.Release()
	existed := false
	if err := os.Remove(Path(checkout)); err == nil {
		existed = true
	} else if !os.IsNotExist(err) {
		return false, err
	}
	if data, readErr := os.ReadFile(pointer); readErr == nil && strings.TrimSpace(string(data)) == checkout {
		if err := os.Remove(pointer); err != nil && !os.IsNotExist(err) {
			return false, err
		}
	} else if readErr != nil && !os.IsNotExist(readErr) {
		return false, readErr
	}
	return existed, nil
}

func acquire(path string, wait time.Duration) (*lock.Lock, error) {
	exact, state, err := (identity.KernelProber{}).Probe(int64(os.Getpid()))
	if err != nil || state != identity.Alive {
		return nil, fmt.Errorf("read declaring process identity: %v", err)
	}
	self := lock.Identity{Pid: int64(os.Getpid()), PidStartedAt: exact.StartedAt.Unix(), Tag: "brain-declare"}
	probe := func(holder lock.Identity) lock.Liveness {
		switch identity.AliveRef(identity.KernelProber{}, identity.Ref{Pid: holder.Pid, StartedAtSec: holder.PidStartedAt}) {
		case identity.Alive:
			return lock.Alive
		case identity.Dead:
			return lock.Dead
		default:
			return lock.Unknown
		}
	}
	return lock.Acquire(path, self, lock.Options{Wait: wait, Poll: 25 * time.Millisecond, Probe: probe})
}

func canonicalPath(path string) (string, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	if resolved, resolveErr := filepath.EvalSymlinks(absolute); resolveErr == nil {
		absolute = resolved
	}
	return filepath.Clean(absolute), nil
}

// PhaseOne reads only the declaration, ledger identity supplied by the
// caller, and role packet. Its returned text is immutable input to the
// bounded second boot phase.
func PhaseOne(stateRoot, repo, ledgerIdentity string, bound int) (ReadResult, string) {
	state := Read(stateRoot, ledgerIdentity)
	if state.State == Undeclared {
		return state, ""
	}
	header := fmt.Sprintf("BRAIN SEAT declaration unreadable for ledger %s. The standing instruction is the record at %s.", ledgerIdentity, PacketRelativePath)
	if state.State == Declared {
		header = fmt.Sprintf("BRAIN SEAT %s for ledger %s, declared by %s %s. The standing instruction is the record at %s.", state.Record.Machine, state.Record.Ledger, state.Record.DeclaredBy, state.Record.DeclaredAt, PacketRelativePath)
	}
	parts := []string{header}
	if state.State == Corrupt {
		parts = append(parts, RemedialRefusal(state.Reason, stateRoot))
	}
	packetPath := filepath.Join(repo, filepath.FromSlash(PacketRelativePath))
	packet, err := os.ReadFile(packetPath)
	if err != nil {
		parts = append(parts, fmt.Sprintf("THE ROLE PACKET IS MISSING at %s (%v); this seat is declared brain and has no instruction: take no work, tell Wido", packetPath, err))
		return state, strings.Join(parts, "\n")
	}
	if len(header)+1+len(packet) <= bound*60/100 {
		parts = append(parts, string(packet))
		return state, strings.Join(parts, "\n")
	}
	parts = append(parts, fmt.Sprintf("PACKET TOO LARGE FOR THIS CHANNEL (%d of %d); read the whole record before anything else", len(packet), bound))
	if standing := standingInstruction(packet); standing != "" {
		parts = append(parts, standing)
	} else {
		parts = append(parts, fmt.Sprintf("THE ROLE PACKET IS MISSING at %s (section \"The standing instruction\" was not found); this seat is declared brain and has no instruction: take no work, tell Wido", packetPath))
	}
	return state, strings.Join(parts, "\n")
}

func standingInstruction(packet []byte) string {
	lines := strings.Split(string(packet), "\n")
	start := -1
	for index, line := range lines {
		if strings.TrimSpace(line) == "## The standing instruction" {
			start = index
			break
		}
	}
	if start < 0 {
		return ""
	}
	end := len(lines)
	for index := start + 1; index < len(lines); index++ {
		if strings.HasPrefix(lines[index], "## ") {
			end = index
			break
		}
	}
	return strings.TrimRight(strings.Join(lines[start:end], "\n"), "\n")
}

func RemedialRefusal(reason, checkout string) string {
	return fmt.Sprintf("this checkout's brain declaration is unreadable (%s); until a human repairs it nothing here dispatches, lands, claims, cancels, closes, reaps, or carries a human's word: metasystem brain withdraw --root %s --by <name>, then metasystem brain declare --root %s --by <name> if this seat is the brain", reason, checkout, checkout)
}

func Fence(stateRoot, act, ledgerIdentity string) string {
	result := Read(stateRoot, ledgerIdentity)
	if result.State == Undeclared {
		return ""
	}
	if result.State == Corrupt {
		return RemedialRefusal(result.Reason, stateRoot)
	}
	switch act {
	case "dispatch", "follow-up":
		return "this checkout is declared the brain; the brain never dispatches. A node runs, from its own checkout: metasystem delegate --role <role> --brief <file> --goal <id> --destructive-reach <class>"
	case "cancel":
		return "this checkout is declared the brain; the brain never cancels a node's job. The node that owns it runs: metasystem delegate --cancel <job id>"
	case "close":
		return "this checkout is declared the brain; the brain never closes or reaps dispatcher records. The node that owns the chain runs: scripts/agents/dispatch.sh close --job <root job>"
	case "reap":
		return "this checkout is declared the brain; the brain never closes or reaps dispatcher records. The node that owns the chain runs: scripts/agents/dispatch.sh reap"
	case "land":
		return "land refused: this checkout is declared the brain; the brain never lands. A node lands from its own checkout: scripts/agents/land.sh -m <message> --goal <id> --chain <root-job> <pathspec>"
	case "claim":
		return "this checkout is declared the brain; the brain never claims. A node claims: metasystem goal claim --root <checkout> --id <id>"
	default:
		return fmt.Sprintf("this checkout is declared the brain; act %s is fenced", act)
	}
}
