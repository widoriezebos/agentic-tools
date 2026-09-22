// Package humanauthority proves that a human-reserved command descends from
// either its current interactive terminal or the enrolled terminal without
// crossing an agent process.
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
	"syscall"
	"time"

	"golang.org/x/sys/unix"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/census"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/fixtureauth"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/governance"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

const (
	OutcomeProven          = "HUMAN_AUTHORITY_PROVEN"
	OutcomeAgent           = "AGENT_IN_AUTHORITY_CHAIN"
	OutcomeTerminalMissing = "TERMINAL_NOT_REACHED"
	OutcomeNotEnrolled     = "TERMINAL_NOT_ENROLLED"
	OutcomeUnreadable      = "ANCESTRY_UNREADABLE"
	OutcomeChanged         = "ANCESTRY_CHANGED"
	OutcomeArgvUnreadable  = "ARGV_UNREADABLE"
	OutcomeReused          = "PROCESS_REUSED"
	OutcomeCycle           = "ANCESTRY_CYCLE"
	OutcomeTemporary       = "TEMPORARY_HUMAN_WORD"
	OutcomeChannel         = "AUTHENTICATED_CHANNEL_WORD"
	OutcomeVerifiedChannel = "VERIFIED_CHANNEL_ANSWER"
	// OutcomeSession marks a browser session a human signed into with the
	// channel's one-time code. It is human authority for the goal acts that
	// admit it; it is not enrolled-terminal ancestry and never passes Valid.
	OutcomeSession      = governance.AuthorityOutcomeSignedInSession
	TemporaryWordRuling = governance.TemporaryGoalAuthorityRuling
	reviewByDateLayout  = "2006-01-02"
	GradeTerminal       = "terminal"
	GradeEnrolled       = "enrolled"
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
	Human         string     `json:"human,omitempty"`
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
	OwnerUID         uint32     `json:"ownerUid"`
	OwnerKnown       bool       `json:"ownerKnown"`
	ArgvWithheld     bool       `json:"argvWithheld,omitempty"`
	AgentRuntime     *string    `json:"agentRuntime,omitempty"`
	TerminalMatch    bool       `json:"terminalMatch"`
}

// Proof is the complete Ruling-C decision record for one invocation.
type Proof struct {
	Schema              int        `json:"schema"`
	CheckedAt           time.Time  `json:"checkedAt"`
	InvokerRef          ProcessRef `json:"invokerRef"`
	TerminalRef         ProcessRef `json:"terminalRef"`
	TerminalGeneration  uint64     `json:"terminalGeneration"`
	SignatureSetDigest  string     `json:"signatureSetDigest"`
	Outcome             string     `json:"outcome"`
	ContinuationOutcome string     `json:"continuationOutcome,omitempty"`
	Grade               string     `json:"grade,omitempty"`
	Nodes               []Node     `json:"nodes"`
	TemporaryHumanWord  string     `json:"temporaryHumanWord,omitempty"`
	ReviewBy            string     `json:"reviewBy,omitempty"`
	Departure           string     `json:"departure,omitempty"`
	ChannelProvider     string     `json:"channelProvider,omitempty"`
	ChannelUser         string     `json:"channelUser,omitempty"`
	ChannelRef          string     `json:"channelRef,omitempty"`
	ChannelContext      string     `json:"channelContext,omitempty"`
	ChannelStep         int64      `json:"channelStep,omitempty"`
	FixtureOnly         bool       `json:"fixtureOnly,omitempty"`
	observedRoot        string
	observedTerminalID  string
	observed            bool
}

// Valid reports whether the proof carries every fact required to authorize a
// human-reserved mutation. It does not turn a parsed JSON document into a new
// observation; production obtains proofs only from Prove.
func (p Proof) Valid() bool {
	if p.FixtureOnly {
		return p.observed && p.Schema == 1 && p.Outcome == OutcomeProven && !p.CheckedAt.IsZero() &&
			p.AuthorityGrade() == GradeEnrolled &&
			p.InvokerRef == (ProcessRef{}) && p.TerminalRef == (ProcessRef{}) && p.TerminalGeneration == 0 &&
			p.SignatureSetDigest == "" && len(p.Nodes) == 0 && p.TemporaryHumanWord == "" &&
			p.ReviewBy == "" && p.Departure == "" && p.ContinuationOutcome == "" && p.ChannelProvider == "" && p.ChannelUser == "" &&
			p.ChannelRef == "" && p.ChannelStep == 0
	}
	if !p.observed || p.Schema != 1 || p.Outcome != OutcomeProven || p.CheckedAt.IsZero() ||
		p.InvokerRef.PID < 1 || p.TerminalRef.PID < 1 ||
		len(p.SignatureSetDigest) != 64 || len(p.Nodes) == 0 || p.TemporaryHumanWord != "" ||
		p.ReviewBy != "" || p.Departure != "" || p.ContinuationOutcome != "" {
		return false
	}
	grade := p.AuthorityGrade()
	if (grade == GradeEnrolled && p.TerminalGeneration == 0) ||
		(grade == GradeTerminal && p.TerminalGeneration != 0) ||
		(grade != GradeEnrolled && grade != GradeTerminal) {
		return false
	}
	terminalSeen := false
	for index, node := range p.Nodes {
		last := index == len(p.Nodes)-1
		if node.AgentRuntime != nil || node.Ref.PID < 1 ||
			node.ExecutableDigest == "" || node.ArgumentDigest == "" || !node.OwnerKnown {
			return false
		}
		if last {
			if grade == GradeTerminal && node.ParentRef != (ProcessRef{}) {
				return false
			}
		} else if node.ParentRef.PID < 1 || !sameRef(node.ParentRef, p.Nodes[index+1].Ref) {
			return false
		}
		if node.TerminalMatch {
			if terminalSeen || !sameRef(node.Ref, p.TerminalRef) {
				return false
			}
			terminalSeen = true
		}
	}
	if !sameRef(p.Nodes[0].Ref, p.InvokerRef) {
		return false
	}
	if grade == GradeEnrolled && !p.Nodes[len(p.Nodes)-1].TerminalMatch {
		return false
	}
	return terminalSeen
}

// AuthorityGrade reports the grade carried by a proof. Proof records written
// before grades were introduced are enrolled proofs when their outcome says
// that human authority was proven.
func (p Proof) AuthorityGrade() string {
	if p.Grade == "" && p.Outcome == OutcomeProven {
		return GradeEnrolled
	}
	return p.Grade
}

// ObservedTerminalID returns the controlling-terminal identity read during
// this in-process observation. It is intentionally absent from parsed proof
// records, which cannot be reused as authority.
func (p Proof) ObservedTerminalID() string { return p.observedTerminalID }

// FixtureGoalProof constructs the explicit headless-fixture equivalent of an
// enrolled-terminal proof. The unforgeable-at-the-CLI grant comes from the
// fixtureauth owner and is bound to this exact fake-runtime checkout root.
func FixtureGoalProof(root string, grant fixtureauth.GoalHumanAuthorityProbe, now time.Time) (Proof, error) {
	if !grant.Allows(root) {
		return Proof{}, fmt.Errorf("fixture human authority is not authorized for this root")
	}
	if now.IsZero() {
		return Proof{}, fmt.Errorf("fixture human authority requires a non-zero observation time")
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return Proof{}, err
	}
	return Proof{Schema: 1, CheckedAt: now.UTC(), Outcome: OutcomeProven, Grade: GradeEnrolled, FixtureOnly: true,
		observedRoot: filepath.Clean(absRoot), observed: true}, nil
}

// ValidFor binds this in-process observation to the root whose ancestry and
// installed signature set were checked. Parsed proof JSON has no authority.
func (p Proof) ValidFor(root string) bool {
	abs, err := filepath.Abs(root)
	return err == nil && p.Valid() && p.AuthorityGrade() == GradeEnrolled && p.observedRoot == filepath.Clean(abs)
}

// TerminalValidFor accepts a fresh proof from either an enrolled terminal or
// any agent-free controlling terminal of this host. Parsed proof JSON has no
// authority.
func (p Proof) TerminalValidFor(root string) bool {
	abs, err := filepath.Abs(root)
	grade := p.AuthorityGrade()
	return err == nil && p.Valid() && (grade == GradeTerminal || grade == GradeEnrolled) &&
		p.observedRoot == filepath.Clean(abs)
}

// EnrolledTerminalFor distinguishes a real ancestry observation from the
// explicit fixture equivalent accepted by other human-only mutations.
func (p Proof) EnrolledTerminalFor(root string) bool {
	return !p.FixtureOnly && p.ValidFor(root)
}

// AuthorizesSetObligation accepts either enrolled-terminal ancestry or the
// temporary recorded-relay form scoped to the two goal mutations that can
// consume it. The relay records the supplied words but cannot verify who
// supplied them. Other human-only mutations continue to depend on ValidFor.
func (p Proof) AuthorizesSetObligation(root string) bool {
	return p.ValidFor(root) || p.temporaryValidFor(root) || p.channelValidFor(root)
}

// TemporarySetObligationFor reports whether the proof is the temporary
// recorded-relay form scoped to set-obligation.
func (p Proof) TemporarySetObligationFor(root string) bool {
	return p.temporaryValidFor(root)
}

// AuthorizesResume accepts the same two proof classes at the breach-stop
// boundary without making temporary authority valid for any other verb.
func (p Proof) AuthorizesResume(root string) bool {
	return p.ValidFor(root) || p.temporaryValidFor(root) || p.channelValidFor(root)
}

func (p Proof) ChannelWordFor(root string) bool { return p.channelValidFor(root) }
func (p Proof) channelValidFor(root string) bool {
	abs, err := filepath.Abs(root)
	if err != nil || !p.observed || p.observedRoot != filepath.Clean(abs) || p.Schema != 1 ||
		(p.Outcome != OutcomeChannel && p.Outcome != OutcomeVerifiedChannel) || p.CheckedAt.IsZero() {
		return false
	}
	return (governance.RecordedChannelAuthority{Outcome: p.Outcome, Provider: p.ChannelProvider, UserID: p.ChannelUser, MessageRef: p.ChannelRef, ContextID: p.ChannelContext, Step: p.ChannelStep}).ValidateRecorded() == nil
}

// VerifiedChannelAnswerProof binds a provider-verified reply to the question
// or status thread whose exact token it answered.
func VerifiedChannelAnswerProof(root string, recorded governance.RecordedChannelAuthority, now time.Time) (Proof, error) {
	if recorded.Outcome != governance.AuthorityOutcomeVerifiedChannelAnswer {
		return Proof{}, fmt.Errorf("verified channel answer proof has the wrong outcome")
	}
	if err := recorded.ValidateRecorded(); err != nil {
		return Proof{}, err
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return Proof{}, err
	}
	return Proof{Schema: 1, CheckedAt: now.UTC(), Outcome: OutcomeVerifiedChannel, ChannelProvider: recorded.Provider, ChannelUser: recorded.UserID, ChannelRef: recorded.MessageRef, ChannelContext: recorded.ContextID, ChannelStep: recorded.Step, observedRoot: filepath.Clean(abs), observed: true}, nil
}
func AuthenticatedChannelProof(root string, recorded governance.RecordedChannelAuthority, now time.Time) (Proof, error) {
	if recorded.Outcome != governance.AuthorityOutcomeAuthenticatedChannelWord {
		return Proof{}, fmt.Errorf("authenticated channel proof has the wrong outcome")
	}
	if err := recorded.ValidateRecorded(); err != nil {
		return Proof{}, err
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return Proof{}, err
	}
	return Proof{Schema: 1, CheckedAt: now.UTC(), Outcome: OutcomeChannel, ChannelProvider: recorded.Provider, ChannelUser: recorded.UserID, ChannelRef: recorded.MessageRef, ChannelStep: recorded.Step, observedRoot: filepath.Clean(abs), observed: true}, nil
}

// SignedInSessionProof binds a browser session a human signed into to the
// checkout root the serving process observed. No session secret enters the
// proof: the issuer, the human's handle, and an opaque session reference are
// the whole record, and each must survive one whitespace-separated History
// key unchanged.
func SignedInSessionProof(root string, user, sessionRef, issuer string, now time.Time) (Proof, error) {
	if now.IsZero() {
		return Proof{}, fmt.Errorf("a signed-in session proof requires a non-zero observation time")
	}
	for _, field := range []struct{ name, value string }{
		{"issuer", issuer}, {"user", user}, {"session reference", sessionRef},
	} {
		if field.value == "" || strings.ContainsAny(field.value, " \t\r\n") {
			return Proof{}, fmt.Errorf("a signed-in session proof requires a %s with no whitespace", field.name)
		}
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return Proof{}, err
	}
	return Proof{Schema: 1, CheckedAt: now.UTC(), Outcome: OutcomeSession,
		ChannelProvider: issuer, ChannelUser: user, ChannelRef: sessionRef,
		observedRoot: filepath.Clean(abs), observed: true}, nil
}

// SessionValidFor reports whether this is a freshly minted signed-in session
// proof bound to that root. Parsed proof JSON has no authority, and a session
// proof carries none of the relay or channel-thread facts.
func (p Proof) SessionValidFor(root string) bool {
	abs, err := filepath.Abs(root)
	if err != nil || !p.observed || p.observedRoot != filepath.Clean(abs) || p.Schema != 1 ||
		p.Outcome != OutcomeSession || p.CheckedAt.IsZero() || p.FixtureOnly {
		return false
	}
	return p.ChannelProvider != "" && p.ChannelUser != "" && p.ChannelRef != "" &&
		p.ChannelContext == "" && p.ChannelStep == 0 && p.ReviewBy == "" &&
		p.TemporaryHumanWord == "" && p.Departure == "" && p.Grade == ""
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
	OwnerUID        uint32
	OwnerKnown      bool
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
	owner, ownerOK := identity.ProcessOwner(pid)
	return Snapshot{Exact: exact, Executable: executable, ExecutableKnown: executableOK,
		OwnerUID: owner, OwnerKnown: ownerOK,
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

const enrollmentAncestryWorkaround = "run goal enroll-terminal from a shell whose ancestry up to its session leader is owned by you, for example a shell inside a tmux session you started from Terminal"

type processReadRefusal struct {
	outcome         string
	pid             int64
	executable      string
	executableKnown bool
	ownerUID        uint32
	ownerKnown      bool
	reason          string
}

func newProcessReadRefusal(pid int64, outcome string, snapshot Snapshot, reason string) *processReadRefusal {
	return &processReadRefusal{
		outcome: outcome, pid: pid,
		executable: snapshot.Executable, executableKnown: snapshot.ExecutableKnown,
		ownerUID: snapshot.OwnerUID, ownerKnown: snapshot.OwnerKnown,
		reason: reason,
	}
}

func (refusal *processReadRefusal) Error() string {
	var details strings.Builder
	fmt.Fprintf(&details, "%s: process pid %d", refusal.outcome, refusal.pid)
	if refusal.executableKnown {
		fmt.Fprintf(&details, ", executable %q", refusal.executable)
	}
	if refusal.ownerKnown {
		fmt.Fprintf(&details, ", owner uid %d", refusal.ownerUID)
	}
	fmt.Fprintf(&details, " was not admitted because %s", refusal.reason)
	if refusal.outcome == OutcomeArgvUnreadable || refusal.outcome == OutcomeUnreadable {
		fmt.Fprintf(&details, "; %s", enrollmentAncestryWorkaround)
	}
	return details.String()
}

func stableRead(reader Reader, pid int64) (Snapshot, *processReadRefusal) {
	first, err := reader.Read(pid)
	if err != nil {
		return Snapshot{}, newProcessReadRefusal(pid, OutcomeUnreadable, Snapshot{}, "the process could not be read")
	}
	if !first.ExecutableKnown {
		return Snapshot{}, newProcessReadRefusal(pid, OutcomeUnreadable, first, "the process's executable path is unreadable")
	}
	if !first.OwnerKnown {
		return Snapshot{}, newProcessReadRefusal(pid, OutcomeUnreadable, first, "the process's owner uid is unreadable")
	}
	var firstSystemImage os.FileInfo
	if !first.Exact.ArgvKnown {
		if first.OwnerUID != 0 {
			return Snapshot{}, newProcessReadRefusal(pid, OutcomeArgvUnreadable, first, "the operating system withholds this process's arguments and the process is not owned by root")
		}
		firstSystemImage, err = protectedSystemImage(first.Executable)
		if err != nil {
			return Snapshot{}, newProcessReadRefusal(pid, OutcomeArgvUnreadable, first, err.Error())
		}
	}
	second, err := reader.Read(pid)
	if err != nil {
		return Snapshot{}, newProcessReadRefusal(pid, OutcomeUnreadable, first, "the process could not be read a second time")
	}
	if !sameRef(refOf(first.Exact), refOf(second.Exact)) {
		return Snapshot{}, newProcessReadRefusal(pid, OutcomeReused, first, "the process identity changed between observations")
	}
	if !first.ParentKnown || !second.ParentKnown {
		return Snapshot{}, newProcessReadRefusal(pid, OutcomeUnreadable, first, "the process's parent is unreadable")
	}
	if first.ParentPID != second.ParentPID {
		return Snapshot{}, newProcessReadRefusal(pid, OutcomeChanged, first, "the process's parent changed between observations")
	}
	if first.ParentPID < 0 {
		return Snapshot{}, newProcessReadRefusal(pid, OutcomeUnreadable, first, "the process's parent is invalid")
	}
	if !second.ExecutableKnown {
		return Snapshot{}, newProcessReadRefusal(pid, OutcomeUnreadable, first, "the process's executable path is unreadable on the second observation")
	}
	if !second.OwnerKnown {
		return Snapshot{}, newProcessReadRefusal(pid, OutcomeUnreadable, first, "the process's owner uid is unreadable on the second observation")
	}
	if first.OwnerUID != second.OwnerUID {
		return Snapshot{}, newProcessReadRefusal(pid, OutcomeChanged, first, "the process's owner uid changed between observations")
	}
	if first.Executable != second.Executable {
		return Snapshot{}, newProcessReadRefusal(pid, OutcomeChanged, first, "the process's executable path changed between observations")
	}
	if !first.Exact.ArgvKnown {
		if second.Exact.ArgvKnown {
			return Snapshot{}, newProcessReadRefusal(pid, OutcomeChanged, first, "the process's arguments changed from withheld to readable between observations")
		}
		secondSystemImage, imageErr := protectedSystemImage(second.Executable)
		if imageErr != nil {
			return Snapshot{}, newProcessReadRefusal(pid, OutcomeArgvUnreadable, second, imageErr.Error())
		}
		if !sameSystemImage(firstSystemImage, secondSystemImage) {
			return Snapshot{}, newProcessReadRefusal(pid, OutcomeChanged, second, "the root-owned executable changed between observations")
		}
	} else if !second.Exact.ArgvKnown {
		return Snapshot{}, newProcessReadRefusal(pid, OutcomeArgvUnreadable, second, "the process's arguments changed from readable to withheld between observations")
	} else if !sameArguments(first.Exact.Argv, second.Exact.Argv) {
		return Snapshot{}, newProcessReadRefusal(pid, OutcomeChanged, first, "the process's arguments changed between observations")
	}
	if !first.TerminalKnown || !second.TerminalKnown || first.TerminalID != second.TerminalID {
		return Snapshot{}, newProcessReadRefusal(pid, OutcomeUnreadable, first, "the process's controlling terminal is unreadable or changed")
	}
	return second, nil
}

func protectedSystemImage(path string) (os.FileInfo, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("the operating system withholds this process's arguments and its executable file is unreadable")
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || stat.Uid != 0 || !info.Mode().IsRegular() || info.Mode().Perm()&0o022 != 0 {
		return nil, fmt.Errorf("the operating system withholds this process's arguments and its executable is not a root-owned regular file protected from group and other writes")
	}
	return info, nil
}

func sameSystemImage(first, second os.FileInfo) bool {
	return os.SameFile(first, second) && first.Mode() == second.Mode() &&
		first.Size() == second.Size() && first.ModTime().Equal(second.ModTime())
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

// ProveTerminal performs the enrollment walk without reading or writing an
// enrollment. Every process from the invoker to the process-tree root is read
// stably and checked against every installed adapter signature. The walk must
// remain on the invoker's controlling terminal through its session leader;
// ancestors above the leader have no terminal constraint.
func ProveTerminal(root string, invokerPID int64, reader Reader, now time.Time) (Proof, error) {
	if reader == nil {
		reader = KernelReader{}
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return Proof{}, err
	}
	proof := Proof{Schema: 1, CheckedAt: now.UTC(), Outcome: OutcomeTerminalMissing,
		observedRoot: filepath.Clean(absRoot), observed: true}
	invoker, refusal := stableRead(reader, invokerPID)
	if refusal != nil {
		proof.Outcome = refusal.outcome
		return proof, refusal
	}
	proof.InvokerRef = refOf(invoker.Exact)
	if invoker.TerminalID == "" {
		return proof, fmt.Errorf("%s", proof.Outcome)
	}
	sessionPID, err := reader.SessionLeader(invokerPID)
	if err != nil || sessionPID < 1 {
		proof.Outcome = OutcomeUnreadable
		return proof, fmt.Errorf("%s", proof.Outcome)
	}
	signatures, signatureDigest, err := signatureSet(root)
	if err != nil {
		return Proof{}, err
	}
	proof.SignatureSetDigest = signatureDigest
	proof.observedTerminalID = invoker.TerminalID

	proof.Nodes, proof.TerminalRef, proof.Outcome, proof.ContinuationOutcome, err = walkProcessTree(
		invokerPID, invoker, reader, signatures,
		&terminalWalkConstraint{terminalID: invoker.TerminalID, sessionPID: sessionPID},
	)
	if err == nil {
		proof.Grade = GradeTerminal
	}
	return proof, err
}

type terminalWalkConstraint struct {
	terminalID string
	sessionPID int64
}

// walkProcessTree performs the common stable ancestry walk. A nil terminal
// constraint is used by the live kernel test so a headless test runner can
// exercise the same root and signature checks without claiming authority.
func walkProcessTree(invokerPID int64, invoker Snapshot, reader Reader, signatures []census.Signature, terminal *terminalWalkConstraint) ([]Node, ProcessRef, string, string, error) {
	seen := map[int64]bool{}
	current := invokerPID
	currentSnapshot := invoker
	var nodes []Node
	var expectedParent *ProcessRef
	var terminalRef ProcessRef
	reachedSession := terminal == nil
	agentRuntime := ""

	for current > 0 {
		if seen[current] {
			outcome, continuation, err := terminalWalkFailure(agentRuntime, OutcomeCycle, fmt.Errorf("%s", OutcomeCycle))
			return nodes, terminalRef, outcome, continuation, err
		}
		seen[current] = true
		if current != invokerPID {
			var refusal *processReadRefusal
			currentSnapshot, refusal = stableRead(reader, current)
			if refusal != nil {
				outcome, continuation, err := terminalWalkFailure(agentRuntime, refusal.outcome, refusal)
				return nodes, terminalRef, outcome, continuation, err
			}
		}
		if expectedParent != nil && !sameRef(refOf(currentSnapshot.Exact), *expectedParent) {
			outcome, continuation, err := terminalWalkFailure(agentRuntime, OutcomeReused, fmt.Errorf("%s", OutcomeReused))
			return nodes, terminalRef, outcome, continuation, err
		}

		executable := sha256.Sum256([]byte(currentSnapshot.Executable))
		arguments := sha256.Sum256([]byte(strings.Join(currentSnapshot.Exact.Argv, "\x00")))
		node := Node{
			Ref:              refOf(currentSnapshot.Exact),
			ExecutableDigest: hex.EncodeToString(executable[:]), ArgumentDigest: hex.EncodeToString(arguments[:]),
			OwnerUID: currentSnapshot.OwnerUID, OwnerKnown: currentSnapshot.OwnerKnown,
			ArgvWithheld: !currentSnapshot.Exact.ArgvKnown,
		}
		if runtime := census.Runtime(strings.Join(currentSnapshot.Exact.Argv, " "), signatures); runtime != "" {
			node.AgentRuntime = &runtime
			if agentRuntime == "" {
				agentRuntime = runtime
			}
		}
		if terminal != nil && !reachedSession && currentSnapshot.TerminalID != terminal.terminalID && agentRuntime == "" {
			nodes = append(nodes, node)
			return nodes, terminalRef, OutcomeTerminalMissing, "", fmt.Errorf("%s", OutcomeTerminalMissing)
		}
		if terminal != nil && current == terminal.sessionPID {
			node.TerminalMatch = true
			terminalRef = refOf(currentSnapshot.Exact)
			reachedSession = true
		}

		if currentSnapshot.ParentPID == 0 {
			nodes = append(nodes, node)
			if agentRuntime != "" {
				return nodes, terminalRef, OutcomeAgent, "", fmt.Errorf("%s: %s", OutcomeAgent, agentRuntime)
			}
			if !reachedSession {
				return nodes, terminalRef, OutcomeTerminalMissing, "", fmt.Errorf("%s", OutcomeTerminalMissing)
			}
			return nodes, terminalRef, OutcomeProven, "", nil
		}

		parentSnapshot, readErr := reader.Read(currentSnapshot.ParentPID)
		if readErr != nil {
			node.ParentRef = ProcessRef{PID: currentSnapshot.ParentPID}
			nodes = append(nodes, node)
			outcome, continuation, err := terminalWalkFailure(agentRuntime, OutcomeUnreadable, fmt.Errorf("%s", OutcomeUnreadable))
			return nodes, terminalRef, outcome, continuation, err
		}
		parentRef := refOf(parentSnapshot.Exact)
		node.ParentRef = parentRef
		nodes = append(nodes, node)
		expectedParent = &parentRef
		current = currentSnapshot.ParentPID
	}
	outcome, continuation, err := terminalWalkFailure(agentRuntime, OutcomeUnreadable, fmt.Errorf("%s", OutcomeUnreadable))
	return nodes, terminalRef, outcome, continuation, err
}

func terminalWalkFailure(agentRuntime, laterOutcome string, laterErr error) (string, string, error) {
	if agentRuntime == "" {
		return laterOutcome, "", laterErr
	}
	return OutcomeAgent, laterOutcome, fmt.Errorf("%s: %s; later ancestry outcome %s: %v", OutcomeAgent, agentRuntime, laterOutcome, laterErr)
}

// Enroll records the direct invoker as this terminal's root only after an
// agent-free stable walk reaches the operating system's session leader and
// continues to the process-tree root.
func Enroll(root string, invokerPID int64, reader Reader, human string, now time.Time) (Enrollment, error) {
	if strings.TrimSpace(human) == "" {
		return Enrollment{}, fmt.Errorf("terminal enrollment requires the human's name")
	}
	proof, err := ProveTerminal(root, invokerPID, reader, now)
	if err != nil {
		return Enrollment{}, fmt.Errorf("terminal enrollment refused: %w", err)
	}
	generation := uint64(1)
	if prior, readErr := ReadEnrollment(root); readErr == nil {
		generation = prior.Generation + 1
	} else if !os.IsNotExist(readErr) {
		return Enrollment{}, readErr
	}
	enrollment := Enrollment{Schema: 1, EnrolledAt: now.UTC(), Generation: generation, Human: human,
		TerminalID: proof.observedTerminalID, TerminalRef: proof.InvokerRef, SessionLeader: proof.TerminalRef}
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
		proof, terminalErr := ProveTerminal(root, invokerPID, reader, now)
		if terminalErr != nil {
			return proof, terminalErr
		}
		proof.Outcome = OutcomeNotEnrolled
		proof.Grade = ""
		return proof, fmt.Errorf("%s: human authority has no readable terminal enrollment: %w", proof.Outcome, err)
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
		snapshot, refusal := stableRead(reader, current)
		if refusal != nil {
			proof.Outcome = refusal.outcome
			return proof, refusal
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
			ExecutableDigest: hex.EncodeToString(executable[:]), ArgumentDigest: hex.EncodeToString(arguments[:]),
			OwnerUID: snapshot.OwnerUID, OwnerKnown: snapshot.OwnerKnown,
			ArgvWithheld: !snapshot.Exact.ArgvKnown}
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
			sessionSnapshot, sessionRefusal := stableRead(reader, sessionPID)
			if sessionRefusal != nil || !sameRef(refOf(sessionSnapshot.Exact), enrollment.SessionLeader) {
				proof.Outcome = OutcomeReused
				return proof, fmt.Errorf("%s", proof.Outcome)
			}
			proof.Outcome = OutcomeProven
			proof.Grade = GradeEnrolled
			proof.observedTerminalID = enrollment.TerminalID
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
	carryAction         = "goal carry"
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

// RecordSessionProof stores the proof for one act published under a browser
// session a human signed into. It is the same audit evidence every other act
// leaves; a session proof carries no secret, so what lands here is the issuer,
// the handle and the opaque session reference.
func RecordSessionProof(root, operationID, action string, proof Proof) error {
	return recordProof(root, operationID, action, proof, proof.SessionValidFor(root))
}

// RecordCarryProof stores the proof for a carry word after its ledger
// transaction is confirmed. It never authorizes a later process.
func RecordCarryProof(root, operationID string, proof Proof) error {
	return recordProof(root, operationID, carryAction, proof, proof.ValidFor(root))
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
