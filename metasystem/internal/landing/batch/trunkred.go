package batch

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
)

// Failure identifies one failed test reported by a proof group.
type Failure struct {
	Report    string `json:"report"`
	Classname string `json:"classname"`
	Name      string `json:"name"`
	Status    string `json:"status"`
	Reason    string `json:"reason"`
}

// RedGroup records one proof group that did not pass.
type RedGroup struct {
	ID            string    `json:"id"`
	InputManifest []string  `json:"inputManifest,omitempty"`
	Status        string    `json:"status"`
	NotRunReason  string    `json:"notRunReason"`
	LogPath       string    `json:"logPath"`
	LogDigest     string    `json:"logDigest"`
	Failures      []Failure `json:"failures"`
	// The collection evidence the known-flake predicate judges (D1). Adapter
	// is the testing-contract adapter that produced it; a command group's
	// evidence format follows a colon (command:junit-xml).
	CollectionComplete    bool      `json:"collectionComplete,omitempty"`
	Missing               []Failure `json:"missing,omitempty"`
	Unexpected            []Failure `json:"unexpected,omitempty"`
	NativeExitStatus      *int      `json:"nativeExitStatus,omitempty"`
	Adapter               string    `json:"adapter,omitempty"`
	LongestSilentSeconds  int64     `json:"longestSilentSeconds,omitempty"`
	LongestZeroCPUSeconds int64     `json:"longestZeroCpuSeconds,omitempty"`
}

// RedGroupFromResult is the red evidence of one group that did not pass.
func RedGroupFromResult(group proofrun.GroupResult) RedGroup {
	failures := func(items []proofrun.NativeTestIdentity, failedOnly bool) []Failure {
		var out []Failure
		for _, item := range items {
			if !failedOnly || item.Status == "failed" {
				out = append(out, Failure{Report: item.Report, Classname: item.Classname, Name: item.Name, Status: item.Status, Reason: item.Reason})
			}
		}
		return out
	}
	return RedGroup{ID: group.ID, Status: group.Status, NotRunReason: group.NotRunReason, LogPath: group.LogPath, LogDigest: group.LogDigest,
		InputManifest: slices.Clone(group.InputManifest), Failures: failures(group.Observed, true), CollectionComplete: group.CollectionComplete,
		Missing: failures(group.Missing, false), Unexpected: failures(group.Unexpected, false), NativeExitStatus: group.NativeExitStatus,
		LongestSilentSeconds: group.LongestSilentSeconds, LongestZeroCPUSeconds: group.LongestZeroCPUSeconds}
}

// TrunkRed describes proof failures observed on a batch base tree.
type TrunkRed struct {
	BatchID    string     `json:"batchId"`
	AttemptID  string     `json:"attemptId"`
	BaseCommit string     `json:"baseCommit"`
	BaseTree   string     `json:"baseTree"`
	Groups     []RedGroup `json:"groups"`
	Joiners    []Claim    `json:"joiners"`
	SeenAt     time.Time  `json:"seenAt"`
}

// EntryRef identifies one trunk-red ledger entry and its proof group.
type EntryRef struct {
	ID    string `json:"id"`
	Group string `json:"group"`
}

// Green identifies the proof result that clears a trunk-red entry.
type Green struct{ AttemptID, BaseCommit, BaseTree, Group string }

// The register's classes (D1); an empty class reads as trunk-red.
const (
	ClassTrunkRed     = "trunk-red"
	ClassPendingFlake = "pending-flake"
	ClassKnownFlake   = "known-flake"
	ClassHang         = "hang"
	ClassQuality      = "quality"
)

// OpenEntry describes an unresolved register entry.
type OpenEntry struct {
	ID, Group, OwnerMachine, FixGoal string
	Holds                            []string
	LastBaseCommit                   string
	Class, Identity, Owner           string
	AllowanceUntil                   time.Time
}

// HoldsLanding reports whether the entry holds a landing: only a trunk red does.
func (entry OpenEntry) HoldsLanding() bool {
	return entry.Class == "" || entry.Class == ClassTrunkRed
}

// CarriesLanding reports whether a known flake's allowance is still running at now.
func (entry OpenEntry) CarriesLanding(now time.Time) bool {
	return entry.Class == ClassKnownFlake && now.Before(entry.AllowanceUntil)
}

// Expired reports whether a known flake's allowance has ended at now: the
// identity blocks again until the entry closes by a proven fix or a person.
func (entry OpenEntry) Expired(now time.Time) bool {
	return entry.Class == ClassKnownFlake && !now.Before(entry.AllowanceUntil)
}

// FlakeSighting is a red attempt and its executed rerun on the same tree that
// passed. TipTree is empty when both ran on the base tree (main).
type FlakeSighting struct {
	BatchID, BaseCommit, BaseTree, TipTree string
	RedAttempt                             string
	Groups                                 []RedGroup
	RedSample                              proofrun.LoadSample
	GreenAttempt, GreenLogPath             string
	GreenLogDigest                         string
	GreenSample                            proofrun.LoadSample
	SeenAt                                 time.Time
}

// Promotion is main's red-then-green for identities that become known flakes,
// with the owner the lane chose and the location the allowance is counted in
// (nil reads as time.Local).
type Promotion struct {
	FlakeSighting
	Owner, OwnerMachine string
	Location            *time.Location
}

// HangEvidence is what the watchdog preserved for a stalled group.
type HangEvidence struct {
	EvidenceDir, Dump, Section, LastStartedTest string
	LongestSilentSeconds, LongestZeroCPUSeconds int64
}

// HangSighting is one stalled group, on a batch tip or on the base (TipTree empty).
type HangSighting struct {
	BatchID, AttemptID, BaseCommit, BaseTree, TipTree string
	Group                                             RedGroup
	Evidence                                          HangEvidence
	Sample                                            proofrun.LoadSample
	SeenAt                                            time.Time
}

// FlakeLedgerOwner is the register's intake beside the trunk red: pending and
// known flakes and hangs, and the open entries of chosen classes.
type FlakeLedgerOwner interface {
	LedgerOwner
	OpenByClass(classes ...string) ([]OpenEntry, error)
	RecordPending(opid string, sighting FlakeSighting) ([]EntryRef, error)
	RecordHang(opid string, hang HangSighting) ([]EntryRef, error)
	Promote(opid string, promotion Promotion) ([]EntryRef, error)
}

// FlakeID is the register identity of one failing test in a group.
func FlakeID(group string, failure Failure) string {
	return TrunkRedID(RedGroup{ID: group, Failures: []Failure{failure}})
}

// LedgerOwner records and clears trunk-red entries outside the batch lock.
type LedgerOwner interface {
	Record(opid string, red TrunkRed) ([]EntryRef, error)
	Clear(opid string, ref EntryRef, green Green) error
	Open() ([]OpenEntry, error)
}

// UnboundLedgerOwner refuses operations until a ledger owner is supplied.
type UnboundLedgerOwner struct{}

var errLedgerOwnerUnbound = fmt.Errorf("%s: no ledger owner is bound", codeTrunkRedOwnerUnbound)

// Record refuses because no ledger owner is bound.
func (UnboundLedgerOwner) Record(string, TrunkRed) ([]EntryRef, error) {
	return nil, errLedgerOwnerUnbound
}

// Clear refuses because no ledger owner is bound.
func (UnboundLedgerOwner) Clear(string, EntryRef, Green) error {
	return errLedgerOwnerUnbound
}

// Open refuses because no ledger owner is bound.
func (UnboundLedgerOwner) Open() ([]OpenEntry, error) {
	return nil, errLedgerOwnerUnbound
}

// OpenByClass refuses because no ledger owner is bound.
func (UnboundLedgerOwner) OpenByClass(...string) ([]OpenEntry, error) {
	return nil, errLedgerOwnerUnbound
}

// RecordPending refuses because no ledger owner is bound.
func (UnboundLedgerOwner) RecordPending(string, FlakeSighting) ([]EntryRef, error) {
	return nil, errLedgerOwnerUnbound
}

// RecordHang refuses because no ledger owner is bound.
func (UnboundLedgerOwner) RecordHang(string, HangSighting) ([]EntryRef, error) {
	return nil, errLedgerOwnerUnbound
}

// Promote refuses because no ledger owner is bound.
func (UnboundLedgerOwner) Promote(string, Promotion) ([]EntryRef, error) {
	return nil, errLedgerOwnerUnbound
}

// WithLedgerOwner returns a store copy bound to o.
func (s Store) WithLedgerOwner(o LedgerOwner) Store {
	s.seams.ledgerOwner = o
	return s
}

// LedgerOwner returns the bound owner or a refusing owner when none is bound.
func (s Store) LedgerOwner() LedgerOwner {
	if s.seams.ledgerOwner == nil {
		return UnboundLedgerOwner{}
	}
	return s.seams.ledgerOwner
}

// FlakeLedger returns the bound owner's flake intake, or a refusing owner.
func (s Store) FlakeLedger() FlakeLedgerOwner {
	if owner, ok := s.seams.ledgerOwner.(FlakeLedgerOwner); ok {
		return owner
	}
	return UnboundLedgerOwner{}
}

// OwnerMachine returns the first joiner's machine.
func (red TrunkRed) OwnerMachine() string {
	if len(red.Joiners) == 0 {
		return ""
	}
	return red.Joiners[0].Machine
}

// TrunkRedID returns the stable identity of a red proof group.
func TrunkRedID(group RedGroup) string {
	identityLines := make([]string, 0, len(group.Failures))
	for _, failure := range group.Failures {
		if failure.Status == "failed" {
			line, _ := json.Marshal([]string{failure.Report, failure.Classname, failure.Name})
			identityLines = append(identityLines, string(line)+"\n")
		}
	}
	identityText := group.ID + "\n"
	if len(identityLines) > 0 {
		slices.Sort(identityLines)
		identityLines = slices.Compact(identityLines)
		identityText += strings.Join(identityLines, "")
	} else {
		identityText += group.Status
	}
	digest := sha256.Sum256([]byte(identityText))
	return "tr-" + strings.ReplaceAll(group.ID, "/", "-") + "-" + fmt.Sprintf("%x", digest[:6])
}
