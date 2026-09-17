package batch

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"
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

// OpenEntry describes an unresolved trunk-red ledger entry.
type OpenEntry struct {
	ID, Group, OwnerMachine, FixGoal string
	Holds                            []string
	LastBaseCommit                   string
}

// LedgerOwner records and clears trunk-red entries outside the batch lock.
type LedgerOwner interface {
	Record(opid string, red TrunkRed) ([]EntryRef, error)
	Clear(opid string, ref EntryRef, green Green) error
	Open() ([]OpenEntry, error)
}

// UnboundLedgerOwner refuses operations until a ledger owner is supplied.
type UnboundLedgerOwner struct{}

var errLedgerOwnerUnbound = errors.New("TRUNK_RED_OWNER_UNBOUND: no ledger owner is bound")

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
