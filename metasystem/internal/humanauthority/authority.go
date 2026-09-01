// Package humanauthority proves that a human-reserved command descends from
// the enrolled interactive terminal without crossing an agent process.
package humanauthority

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"golang.org/x/sys/unix"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/census"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/governance"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

const (
	OutcomeProven          = "HUMAN_AUTHORITY_PROVEN"
	OutcomeAgent           = "AGENT_IN_AUTHORITY_CHAIN"
	OutcomeTerminalMissing = "TERMINAL_NOT_REACHED"
	OutcomeUnreadable      = "ANCESTRY_UNREADABLE"
	OutcomeChanged         = "ANCESTRY_CHANGED"
	OutcomeArgvUnreadable  = "ARGV_UNREADABLE"
	OutcomeReused          = "PROCESS_REUSED"
	OutcomeCycle           = "ANCESTRY_CYCLE"
	OutcomeTemporary       = "TEMPORARY_HUMAN_WORD"
	TemporaryWordRuling    = governance.TemporaryGoalAuthorityRuling
	reviewByDateLayout     = "2006-01-02"
)

// ProcessRef is the stable birth identity recorded in enrollments and proofs.
type ProcessRef struct {
	PID          int64  `json:"pid"`
	PIDStartedAt int64  `json:"pidStartedAt"`
	StartTicks   int64  `json:"startTicks,omitempty"`
	BootID       string `json:"bootId,omitempty"`
}

// Enrollment is the local terminal root every later proof must reach.
type Enrollment struct {
	Schema        int        `json:"schema"`
	EnrolledAt    time.Time  `json:"enrolledAt"`
	Generation    uint64     `json:"generation"`
	TerminalID    string     `json:"terminalId"`
	TerminalRef   ProcessRef `json:"terminalRef"`
	SessionLeader ProcessRef `json:"sessionLeaderRef"`
}

// Node is one stable process observation. Argument bytes are represented only
// by digests so authority records do not retain command-line contents.
type Node struct {
	Ref              ProcessRef `json:"ref"`
	ParentRef        ProcessRef `json:"parentRef"`
	ExecutableDigest string     `json:"executableDigest"`
	ArgumentDigest   string     `json:"argumentDigest"`
	AgentRuntime     *string    `json:"agentRuntime,omitempty"`
	TerminalMatch    bool       `json:"terminalMatch"`
}

// Proof is the complete Ruling-C decision record for one invocation.
type Proof struct {
	Schema             int        `json:"schema"`
	CheckedAt          time.Time  `json:"checkedAt"`
	InvokerRef         ProcessRef `json:"invokerRef"`
	TerminalRef        ProcessRef `json:"terminalRef"`
	TerminalGeneration uint64     `json:"terminalGeneration"`
	SignatureSetDigest string     `json:"signatureSetDigest"`
	Outcome            string     `json:"outcome"`
	Nodes              []Node     `json:"nodes"`
	TemporaryHumanWord string     `json:"temporaryHumanWord,omitempty"`
	ReviewBy           string     `json:"reviewBy,omitempty"`
	Departure          string     `json:"departure,omitempty"`
	observedRoot       string
	observed           bool
}

// Valid reports whether the proof carries every fact required to authorize a
// human-reserved mutation. It does not turn a parsed JSON document into a new
// observation; production obtains proofs only from Prove.
func (p Proof) Valid() bool {
	if !p.observed || p.Schema != 1 || p.Outcome != OutcomeProven || p.CheckedAt.IsZero() ||
		p.InvokerRef.PID < 1 || p.TerminalRef.PID < 1 || p.TerminalGeneration == 0 ||
		len(p.SignatureSetDigest) != 64 || len(p.Nodes) == 0 || p.TemporaryHumanWord != "" ||
		p.ReviewBy != "" || p.Departure != "" {
		return false
	}
	for _, node := range p.Nodes {
		if node.AgentRuntime != nil || node.Ref.PID < 1 || node.ParentRef.PID < 1 ||
			node.ExecutableDigest == "" || node.ArgumentDigest == "" {
			return false
		}
	}
	last := p.Nodes[len(p.Nodes)-1]
	return last.TerminalMatch && sameRef(last.Ref, p.TerminalRef)
}

// ValidFor binds this in-process observation to the root whose ancestry and
// installed signature set were checked. Parsed proof JSON has no authority.
func (p Proof) ValidFor(root string) bool {
	abs, err := filepath.Abs(root)
	return err == nil && p.Valid() && p.observedRoot == filepath.Clean(abs)
}

// AuthorizesSetObligation accepts either enrolled-terminal ancestry or the
// temporary recorded-relay form scoped to the two goal mutations that can
// consume it. The relay records the supplied words but cannot verify who
// supplied them. Other human-only mutations continue to depend on ValidFor.
func (p Proof) AuthorizesSetObligation(root string) bool {
	return p.ValidFor(root) || p.temporaryValidFor(root)
}

// TemporarySetObligationFor reports whether the proof is the temporary
// recorded-relay form scoped to set-obligation.
func (p Proof) TemporarySetObligationFor(root string) bool {
	return p.temporaryValidFor(root)
}

// AuthorizesResume accepts the same two proof classes at the breach-stop
// boundary without making temporary authority valid for any other verb.
func (p Proof) AuthorizesResume(root string) bool {
	return p.ValidFor(root) || p.temporaryValidFor(root)
}

// TemporaryResumeFor reports whether a resume is using the temporary proof
// form rather than enrolled-terminal ancestry.
func (p Proof) TemporaryResumeFor(root string) bool {
	return p.temporaryValidFor(root)
}

// ValidateTemporaryWordPair checks only the optional relay pair's transport
// shape; recording the supplied words does not verify human provenance. The
// goal relay applies stricter grant-time substance, past-date, and horizon
// rules than the steward relay.
func ValidateTemporaryWordPair(humanWord, reviewBy string) error {
	if (humanWord == "") != (reviewBy == "") {
		return fmt.Errorf("--temporary-human-word and --review-by travel together")
	}
	if humanWord == "" {
		return nil
	}
	if strings.TrimSpace(humanWord) == "" {
		return fmt.Errorf("--temporary-human-word must contain non-whitespace human words")
	}
	if _, err := time.Parse(reviewByDateLayout, reviewBy); err != nil {
		return fmt.Errorf("--review-by must be a real date in YYYY-MM-DD form")
	}
	return nil
}

func validateTemporaryGoalAuthority(humanWord, reviewBy string, checkedAt time.Time) error {
	if err := ValidateTemporaryWordPair(humanWord, reviewBy); err != nil {
		return err
	}
	if len(strings.Fields(humanWord)) < 3 {
		return fmt.Errorf("--temporary-human-word must contain at least three words; this is a minimum-of-substance guard, not proof of human provenance")
	}
	if checkedAt.IsZero() {
		return fmt.Errorf("temporary human authority requires a non-zero observation time")
	}
	reviewDate, _ := time.Parse(reviewByDateLayout, reviewBy)
	checkedUTC := checkedAt.UTC()
	checkedDate := time.Date(checkedUTC.Year(), checkedUTC.Month(), checkedUTC.Day(), 0, 0, 0, 0, time.UTC)
	if reviewDate.Before(checkedDate) {
		return fmt.Errorf("--review-by %s is in the past", reviewBy)
	}
	horizon, _ := time.Parse(reviewByDateLayout, governance.TemporaryGoalAuthorityHorizon)
	if reviewDate.After(horizon) {
		return fmt.Errorf("--review-by %s exceeds temporary goal authority horizon %s", reviewBy, governance.TemporaryGoalAuthorityHorizon)
	}
	return nil
}

func (p Proof) temporaryValidFor(root string) bool {
	abs, err := filepath.Abs(root)
	if err != nil || !p.observed || p.observedRoot != filepath.Clean(abs) || p.Schema != 1 ||
		p.Outcome != OutcomeTemporary || p.CheckedAt.IsZero() || p.Departure != TemporaryWordRuling {
		return false
	}
	if err := validateTemporaryGoalAuthority(p.TemporaryHumanWord, p.ReviewBy, p.CheckedAt); err != nil || p.TemporaryHumanWord == "" {
		return false
	}
	return p.InvokerRef == (ProcessRef{}) && p.TerminalRef == (ProcessRef{}) &&
		p.TerminalGeneration == 0 && p.SignatureSetDigest == "" && len(p.Nodes) == 0
}

// TemporaryGoalProof records relayed words presented as the human's and a
// re-approval date against the real wall clock. It cannot verify who supplied
// the words, carries no ancestry facts, and is recognized only by resume and
// set-obligation.
func TemporaryGoalProof(root, humanWord, reviewBy string) (Proof, error) {
	return temporaryGoalProofAt(root, humanWord, reviewBy, time.Now().UTC())
}

// temporaryGoalProofAt keeps evaluation at a supplied instant inside the
// authority owner. Callers outside this package enter through
// TemporaryGoalProof, which always reads the real wall clock.
func temporaryGoalProofAt(root, humanWord, reviewBy string, now time.Time) (Proof, error) {
	if humanWord == "" && reviewBy == "" {
		return Proof{}, fmt.Errorf("temporary recorded-relay authority requires the supplied word and review-by date")
	}
	if err := validateTemporaryGoalAuthority(humanWord, reviewBy, now); err != nil {
		return Proof{}, err
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return Proof{}, err
	}
	return Proof{
		Schema: 1, CheckedAt: now.UTC(), Outcome: OutcomeTemporary,
		TemporaryHumanWord: humanWord, ReviewBy: reviewBy, Departure: TemporaryWordRuling,
		observedRoot: filepath.Clean(abs), observed: true,
	}, nil
}

// ProveOrTemporaryGoalAuthority preserves the authority preference order for
// the two goal relay verbs. A proved enrolled-terminal path wins even when
// temporary flags are present or malformed; relay validation starts only
// after enrolled ancestry fails and reads its own real wall clock.
func ProveOrTemporaryGoalAuthority(root string, invokerPID int64, reader Reader, humanWord, reviewBy string, ancestryNow time.Time) (Proof, error) {
	proof, ancestryErr := Prove(root, invokerPID, reader, ancestryNow)
	if ancestryErr == nil {
		return proof, nil
	}
	if humanWord == "" && reviewBy == "" {
		return Proof{}, ancestryErr
	}
	return TemporaryGoalProof(root, humanWord, reviewBy)
}

// Snapshot is one process read used by the stable ancestry walk.
type Snapshot struct {
	Exact           identity.Exact
	Executable      string
	ExecutableKnown bool
	ParentPID       int64
	ParentKnown     bool
	TerminalID      string
	TerminalKnown   bool
}

// Reader supplies process facts. Tests inject a fully deterministic tree.
type Reader interface {
	Read(pid int64) (Snapshot, error)
	SessionLeader(pid int64) (int64, error)
}

// KernelReader reads all production facts from the operating system.
type KernelReader struct{}

func (KernelReader) Read(pid int64) (Snapshot, error) {
	exact, state, err := (identity.KernelProber{}).Probe(pid)
	if err != nil || state != identity.Alive {
		return Snapshot{}, fmt.Errorf("process %d is not a readable live process", pid)
	}
	parent, parentOK := identity.ParentPid(pid)
	terminal, terminalOK := identity.ControllingTerminalIdentity(pid)
	executable, executableOK := identity.ExecutablePath(pid)
	return Snapshot{Exact: exact, Executable: executable, ExecutableKnown: executableOK,
		ParentPID: parent, ParentKnown: parentOK,
		TerminalID: terminal, TerminalKnown: terminalOK}, nil
}

func (KernelReader) SessionLeader(pid int64) (int64, error) {
	sid, err := unix.Getsid(int(pid))
	return int64(sid), err
}

func refOf(exact identity.Exact) ProcessRef {
	return ProcessRef{PID: exact.Pid, PIDStartedAt: exact.StartedAt.Unix(), StartTicks: exact.StartTicks, BootID: exact.BootID}
}

func sameRef(left, right ProcessRef) bool {
	if left.PID != right.PID {
		return false
	}
	if left.StartTicks > 0 && left.BootID != "" && right.StartTicks > 0 && right.BootID != "" {
		return left.StartTicks == right.StartTicks && left.BootID == right.BootID
	}
	return left.PIDStartedAt == right.PIDStartedAt
}

func stableRead(reader Reader, pid int64) (Snapshot, string) {
	first, err := reader.Read(pid)
	if err != nil {
		return Snapshot{}, OutcomeUnreadable
	}
	if !first.Exact.ArgvKnown {
		return Snapshot{}, OutcomeArgvUnreadable
	}
	if !first.ExecutableKnown {
		return Snapshot{}, OutcomeUnreadable
	}
	second, err := reader.Read(pid)
	if err != nil {
		return Snapshot{}, OutcomeUnreadable
	}
	if !sameRef(refOf(first.Exact), refOf(second.Exact)) {
		return Snapshot{}, OutcomeReused
	}
	if !first.ParentKnown || !second.ParentKnown {
		return Snapshot{}, OutcomeUnreadable
	}
	if first.ParentPID != second.ParentPID {
		return Snapshot{}, OutcomeChanged
	}
	if !second.Exact.ArgvKnown {
		return Snapshot{}, OutcomeArgvUnreadable
	}
	if !second.ExecutableKnown {
		return Snapshot{}, OutcomeUnreadable
	}
	if first.Executable != second.Executable || !sameArguments(first.Exact.Argv, second.Exact.Argv) {
		return Snapshot{}, OutcomeChanged
	}
	if !first.TerminalKnown || !second.TerminalKnown || first.TerminalID != second.TerminalID {
		return Snapshot{}, OutcomeUnreadable
	}
	return second, ""
}

func sameArguments(first, second []string) bool {
	if len(first) != len(second) {
		return false
	}
	for index := range first {
		if first[index] != second[index] {
			return false
		}
	}
	return true
}

func signatureSet(root string) ([]census.Signature, string, error) {
	paths, err := filepath.Glob(filepath.Join(root, "scripts", "agents", "adapters", "*.sh"))
	if err != nil {
		return nil, "", err
	}
	sort.Strings(paths)
	hash := sha256.New()
	var signatures []census.Signature
	for _, path := range paths {
		runtime := strings.TrimSuffix(filepath.Base(path), ".sh")
		if runtime == "runtime-common" {
			continue
		}
		text, err := census.SignatureText(path)
		if err != nil {
			return nil, "", err
		}
		matches, excludes := census.ParseSignatureText(text)
		signature, err := census.CompileSignature(runtime, matches, excludes)
		if err != nil {
			return nil, "", err
		}
		signatures = append(signatures, signature)
		hash.Write([]byte(runtime))
		hash.Write([]byte{0})
		hash.Write([]byte(text))
	}
	if len(signatures) == 0 {
		return nil, "", fmt.Errorf("no adapter signatures are installed under %s", root)
	}
	return signatures, hex.EncodeToString(hash.Sum(nil)), nil
}

func enrollmentPath(root string) string {
	return filepath.Join(root, "artifacts", "agents", "authority", "human-terminal.json")
}

// ReadEnrollment reads the exact enrolled terminal record without accepting
// unknown fields or trailing JSON.
func ReadEnrollment(root string) (Enrollment, error) {
	data, err := os.ReadFile(enrollmentPath(root))
	if err != nil {
		return Enrollment{}, err
	}
	var enrollment Enrollment
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&enrollment); err != nil {
		return Enrollment{}, fmt.Errorf("human terminal enrollment is unreadable: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return Enrollment{}, fmt.Errorf("human terminal enrollment has trailing JSON")
	}
	if enrollment.Schema != 1 || enrollment.Generation == 0 || enrollment.TerminalID == "" ||
		enrollment.TerminalRef.PID < 1 || enrollment.SessionLeader.PID < 1 || enrollment.EnrolledAt.IsZero() {
		return Enrollment{}, fmt.Errorf("human terminal enrollment is incomplete")
	}
	return enrollment, nil
}

func walkToPID(root string, start, terminalPID int64, terminalID string, reader Reader) (ProcessRef, error) {
	signatures, _, err := signatureSet(root)
	if err != nil {
		return ProcessRef{}, err
	}
	seen := map[int64]bool{}
	current := start
	for current > 0 {
		if seen[current] {
			return ProcessRef{}, fmt.Errorf("%s", OutcomeCycle)
		}
		seen[current] = true
		snapshot, outcome := stableRead(reader, current)
		if outcome != "" {
			return ProcessRef{}, fmt.Errorf("%s", outcome)
		}
		if runtime := census.Runtime(strings.Join(snapshot.Exact.Argv, " "), signatures); runtime != "" {
			return ProcessRef{}, fmt.Errorf("%s: %s", OutcomeAgent, runtime)
		}
		if snapshot.TerminalID != terminalID {
			return ProcessRef{}, fmt.Errorf("%s", OutcomeTerminalMissing)
		}
		if current == terminalPID {
			return refOf(snapshot.Exact), nil
		}
		current = snapshot.ParentPID
	}
	return ProcessRef{}, fmt.Errorf("%s", OutcomeTerminalMissing)
}

// Enroll records the direct invoker as this terminal's root only after an
// agent-free stable walk reaches the operating system's session leader.
func Enroll(root string, invokerPID int64, reader Reader, now time.Time) (Enrollment, error) {
	if reader == nil {
		reader = KernelReader{}
	}
	invoker, outcome := stableRead(reader, invokerPID)
	if outcome != "" {
		return Enrollment{}, fmt.Errorf("terminal enrollment refused: %s", outcome)
	}
	if invoker.TerminalID == "" {
		return Enrollment{}, fmt.Errorf("terminal enrollment refused: %s", OutcomeTerminalMissing)
	}
	sessionPID, err := reader.SessionLeader(invokerPID)
	if err != nil || sessionPID < 1 {
		return Enrollment{}, fmt.Errorf("terminal enrollment refused: %s", OutcomeUnreadable)
	}
	sessionRef, err := walkToPID(root, invokerPID, sessionPID, invoker.TerminalID, reader)
	if err != nil {
		return Enrollment{}, fmt.Errorf("terminal enrollment refused: %w", err)
	}
	generation := uint64(1)
	if prior, readErr := ReadEnrollment(root); readErr == nil {
		generation = prior.Generation + 1
	} else if !os.IsNotExist(readErr) {
		return Enrollment{}, readErr
	}
	enrollment := Enrollment{Schema: 1, EnrolledAt: now.UTC(), Generation: generation,
		TerminalID: invoker.TerminalID, TerminalRef: refOf(invoker.Exact), SessionLeader: sessionRef}
	encoded, err := json.MarshalIndent(enrollment, "", "  ")
	if err != nil {
		return Enrollment{}, err
	}
	durable, err := atomicfile.WriteText(enrollmentPath(root), string(encoded)+"\n", root)
	if err != nil {
		return Enrollment{}, err
	}
	if !durable {
		return Enrollment{}, fmt.Errorf("human terminal enrollment was written but its durability is unknown")
	}
	return enrollment, nil
}

// Prove walks from the command's real parent to the exact enrolled terminal.
// Every node is read twice, checked against every installed agent signature,
// and retained only as identity and argument digests.
func Prove(root string, invokerPID int64, reader Reader, now time.Time) (Proof, error) {
	if reader == nil {
		reader = KernelReader{}
	}
	enrollment, err := ReadEnrollment(root)
	if err != nil {
		return Proof{}, fmt.Errorf("human authority has no readable terminal enrollment: %w", err)
	}
	signatures, signatureDigest, err := signatureSet(root)
	if err != nil {
		return Proof{}, err
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return Proof{}, err
	}
	proof := Proof{Schema: 1, CheckedAt: now.UTC(), TerminalRef: enrollment.TerminalRef,
		TerminalGeneration: enrollment.Generation, SignatureSetDigest: signatureDigest, Outcome: OutcomeTerminalMissing}
	proof.observedRoot = filepath.Clean(absRoot)
	proof.observed = true
	seen := map[int64]bool{}
	current := invokerPID
	var expectedParent *ProcessRef
	for current > 0 {
		if seen[current] {
			proof.Outcome = OutcomeCycle
			return proof, fmt.Errorf("%s", proof.Outcome)
		}
		seen[current] = true
		snapshot, outcome := stableRead(reader, current)
		if outcome != "" {
			proof.Outcome = outcome
			return proof, fmt.Errorf("%s", proof.Outcome)
		}
		if snapshot.TerminalID != enrollment.TerminalID {
			proof.Outcome = OutcomeTerminalMissing
			return proof, fmt.Errorf("%s", proof.Outcome)
		}
		if expectedParent != nil && !sameRef(refOf(snapshot.Exact), *expectedParent) {
			proof.Outcome = OutcomeReused
			return proof, fmt.Errorf("%s", proof.Outcome)
		}
		if proof.InvokerRef.PID == 0 {
			proof.InvokerRef = refOf(snapshot.Exact)
		}
		parentSnapshot, err := reader.Read(snapshot.ParentPID)
		if err != nil {
			proof.Outcome = OutcomeUnreadable
			return proof, fmt.Errorf("%s", proof.Outcome)
		}
		executable := sha256.Sum256([]byte(snapshot.Executable))
		arguments := sha256.Sum256([]byte(strings.Join(snapshot.Exact.Argv, "\x00")))
		parentRef := refOf(parentSnapshot.Exact)
		node := Node{Ref: refOf(snapshot.Exact), ParentRef: parentRef,
			ExecutableDigest: hex.EncodeToString(executable[:]), ArgumentDigest: hex.EncodeToString(arguments[:])}
		if runtime := census.Runtime(strings.Join(snapshot.Exact.Argv, " "), signatures); runtime != "" {
			node.AgentRuntime = &runtime
			proof.Nodes = append(proof.Nodes, node)
			proof.Outcome = OutcomeAgent
			return proof, fmt.Errorf("%s: %s", proof.Outcome, runtime)
		}
		node.TerminalMatch = sameRef(node.Ref, enrollment.TerminalRef) && snapshot.TerminalID == enrollment.TerminalID
		proof.Nodes = append(proof.Nodes, node)
		if node.TerminalMatch {
			sessionPID, sessionErr := reader.SessionLeader(current)
			if sessionErr != nil || sessionPID != enrollment.SessionLeader.PID {
				proof.Outcome = OutcomeTerminalMissing
				return proof, fmt.Errorf("%s", proof.Outcome)
			}
			sessionSnapshot, sessionOutcome := stableRead(reader, sessionPID)
			if sessionOutcome != "" || !sameRef(refOf(sessionSnapshot.Exact), enrollment.SessionLeader) {
				proof.Outcome = OutcomeReused
				return proof, fmt.Errorf("%s", proof.Outcome)
			}
			proof.Outcome = OutcomeProven
			return proof, nil
		}
		expectedParent = &parentRef
		current = snapshot.ParentPID
	}
	return proof, fmt.Errorf("%s", proof.Outcome)
}

const (
	setObligationAction = "goal set-obligation"
	resumeAction        = "goal resume"
)

// RecordProof stores the observed proof beside the local authority records,
// bound to the exact operation. The record is audit evidence, never a token a
// later process may present as authority.
func RecordProof(root, operationID, action string, proof Proof) error {
	return recordProof(root, operationID, action, proof, proof.ValidFor(root))
}

// RecordSetObligationProof stores the proof for the one verb allowed to
// consume the temporary remote-word authority form.
func RecordSetObligationProof(root, operationID string, proof Proof) error {
	return recordProof(root, operationID, setObligationAction, proof, proof.AuthorizesSetObligation(root))
}

// RecordResumeProof stores either accepted proof form for one exact resume.
func RecordResumeProof(root, operationID string, proof Proof) error {
	return recordProof(root, operationID, resumeAction, proof, proof.AuthorizesResume(root))
}

func recordProof(root, operationID, action string, proof Proof, recordable bool) error {
	if operationID == "" || filepath.Base(operationID) != operationID || action == "" || !recordable {
		return fmt.Errorf("cannot record an incomplete human authority proof")
	}
	record := struct {
		Schema      int    `json:"schema"`
		OperationID string `json:"operationId"`
		Action      string `json:"action"`
		Proof       Proof  `json:"proof"`
	}{Schema: 1, OperationID: operationID, Action: action, Proof: proof}
	encoded, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return err
	}
	path := filepath.Join(root, "artifacts", "agents", "authority", "proofs", operationID+".json")
	durable, err := atomicfile.WriteText(path, string(encoded)+"\n", root)
	if err != nil {
		return err
	}
	if !durable {
		return fmt.Errorf("human authority proof was written but its durability is unknown")
	}
	return nil
}
