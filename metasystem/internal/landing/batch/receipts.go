package batch

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"time"
)

// PrefixReceipt records how one exact prefix obtained every selected group.
type PrefixReceipt struct {
	GoalID, Tree, AttemptID, ResultPath string
	DecisionID                          string   `json:"decisionId,omitempty"`
	FreshEpisode                        string   `json:"freshEpisode,omitempty"`
	FreshExpiresAt                      string   `json:"freshExpiresAt,omitempty"`
	CommitIDs                           []string `json:"CommitIDs,omitempty"`
	Units                               []string `json:"Units,omitempty"`
	LastUnit                            string   `json:"LastUnit,omitempty"`
	Reused                              map[string]string
	Executed                            []string
	// CachedPass are groups the test tool served wholly from its result cache.
	CachedPass []string `json:"CachedPass,omitempty"`
}

// PrefixDecision contains the coverage and policy context for one cumulative
// tree. Groups are additional admitted obligations; the testing owner still
// computes the protected delivery floor on the exact tree.
type PrefixDecision struct {
	Groups         []string
	PolicyContext  string
	IdentityTree   string
	FreshRequired  bool
	FreshMaxAgeMS  int64
	FreshEpisode   string
	FreshExpiresAt string
}

type PrefixEpisode struct {
	DecisionID string `json:"decisionId"`
	Token      string `json:"token"`
	ExpiresAt  string `json:"expiresAt,omitempty"`
}

func PrefixDecisionID(base, tree string, units []Unit, sealed map[string]Claim, decision PrefixDecision) (string, error) {
	type member struct {
		GoalID, Chain string
		Claim         Claim
		Groups        []string
	}
	bound := struct {
		Version, Base, Tree, Policy string
		Groups                      []string
		Members                     []member
		FreshRequired               bool
		FreshMaxAgeMS               int64
	}{Version: "prefix-decision-v1", Base: base, Tree: tree, Policy: decision.PolicyContext, Groups: slices.Clone(decision.Groups), FreshRequired: decision.FreshRequired, FreshMaxAgeMS: decision.FreshMaxAgeMS}
	if decision.IdentityTree != "" {
		bound.Tree = decision.IdentityTree
	}
	for _, unit := range units {
		claim, ok := sealed[unit.GoalID]
		if !ok {
			return "", fmt.Errorf("%s: %s has no sealed claim", codePrefixAuthorityRefused, unit.GoalID)
		}
		bound.Members = append(bound.Members, member{GoalID: unit.GoalID, Chain: unit.Chain, Claim: claim, Groups: slices.Clone(unit.SelectedGroups)})
	}
	data, err := json.Marshal(bound)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

type PrefixBudgetRefusal struct{ Reason string }

func (refusal *PrefixBudgetRefusal) Error() string { return refusal.Reason }

type PrefixRevisionRefusal struct{ Reason string }

func (refusal *PrefixRevisionRefusal) Error() string { return refusal.Reason }

type PrefixFencedRefusal struct{ Reason string }

func (refusal *PrefixFencedRefusal) Error() string { return refusal.Reason }

// PrefixAdmissionRefusal preserves a non-budget admission decision so receipt
// composition can retry it without changing the member's lifecycle.
type PrefixAdmissionRefusal struct {
	Code, Reason string
}

func (refusal *PrefixAdmissionRefusal) Error() string { return refusal.Reason }

// HandlePrefixMemberRefusal returns a fenced, revision-moved or budget-closed
// member through the existing claim lifecycle and recomposes its survivors.
func HandlePrefixMemberRefusal(store Store, id, goalID, actor string, at time.Time, cause error) error {
	state := ""
	var fenced *PrefixFencedRefusal
	var revision *PrefixRevisionRefusal
	var budget *PrefixBudgetRefusal
	var joinRed *JoinAdmissionRed
	switch {
	case errors.As(cause, &fenced), errors.As(cause, &revision), errors.As(cause, &joinRed):
		state = UnitEjected
	case errors.As(cause, &budget):
		state = UnitWithdrawnBudget
	default:
		return cause
	}
	if err := ReassembleSurvivorsWithReturns(store, id, actor, at, []ReturnDecision{{GoalID: goalID, Outcome: state, Reason: cause.Error()}}); err != nil {
		return errors.Join(cause, err)
	}
	return nil
}
