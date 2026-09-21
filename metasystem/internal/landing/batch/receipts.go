package batch

import (
	"crypto/rand"
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
}

type PrefixRunResult struct {
	AttemptID, ResultPath string
	Reused                map[string]string
	Executed              []string
	Red                   []RedGroup
}

type PrefixReceiptSeams struct {
	Execute         func(goalID, tree string, groups []string) (PrefixRunResult, error)
	ExecuteDecision func(goalID, tree string, decision PrefixDecision) (PrefixRunResult, error)
	Plan            func(units []Unit, tree string) (PrefixDecision, error)
	Verify          func(unit Unit, tree string, decision PrefixDecision) error
	Authorize       func(unit Unit, sealed Claim) error
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
			return "", fmt.Errorf("BATCH_PREFIX_AUTHORITY_REFUSED: %s has no sealed claim", unit.GoalID)
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

type PrefixRedError struct {
	GoalID string
	Groups []RedGroup
}

func (red *PrefixRedError) Error() string { return "prefix proof red for " + red.GoalID }

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

// ComposePrefixReceipts reuses only identity-equal terminal evidence. A
// prefix with no identity differences creates no attempt.
func ComposePrefixReceipts(store Store, id, actor string, at time.Time, seams PrefixReceiptSeams) error {
	record, err := store.Load(id)
	if err != nil {
		return err
	}
	if record.State != StateLanding || record.Proof == nil || record.Proof.Status != "green" {
		return fmt.Errorf("batch %s has no green tip proof", id)
	}
	joined := joinedUnits(record.Units)
	if len(joined) == 0 || len(record.PrefixTrees) < len(joined) {
		return fmt.Errorf("batch %s has no complete prefix tree list", id)
	}
	for index, unit := range joined[:len(joined)-1] {
		tree := record.PrefixTrees[index]
		if seams.Authorize != nil {
			if err := seams.Authorize(unit, record.Seal[unit.GoalID]); err != nil {
				return HandlePrefixMemberRefusal(store, id, unit.GoalID, actor, at, err)
			}
		}
		decision := PrefixDecision{Groups: slices.Clone(record.Proof.SelectedGroups)}
		decisionID := ""
		if seams.Plan != nil {
			decision, err = seams.Plan(joined[:index+1], tree)
			if err != nil {
				return err
			}
			decisionID, err = PrefixDecisionID(record.BaseTree, tree, joined[:index+1], record.Seal, decision)
			if err != nil {
				return err
			}
		}
		if decision.FreshRequired {
			episode, episodeErr := ensurePrefixEpisode(store, id, record, unit.GoalID, index, tree, decisionID, decision.FreshMaxAgeMS, at)
			if episodeErr != nil {
				return episodeErr
			}
			decision.FreshEpisode, decision.FreshExpiresAt = episode.Token, episode.ExpiresAt
		}
		if existing, ok := record.Receipts[unit.GoalID]; ok && existing.Tree == tree && existing.DecisionID == decisionID &&
			existing.FreshEpisode == decision.FreshEpisode && existing.FreshExpiresAt == decision.FreshExpiresAt {
			if seams.Verify == nil || seams.Verify(unit, tree, decision) == nil {
				continue
			}
		}
		receipt := PrefixReceipt{GoalID: unit.GoalID, Tree: tree, CommitIDs: slices.Clone(unit.CommitIDs), LastUnit: unit.LastUnit, Reused: map[string]string{}}
		receipt.DecisionID, receipt.FreshEpisode, receipt.FreshExpiresAt = decisionID, decision.FreshEpisode, decision.FreshExpiresAt
		for _, build := range unit.Builds {
			receipt.Units = append(receipt.Units, build.Units...)
		}
		if seams.Execute == nil && seams.ExecuteDecision == nil {
			return fmt.Errorf("batch %s has no prefix receipt runner", id)
		}
		var result PrefixRunResult
		var runErr error
		if seams.ExecuteDecision != nil {
			result, runErr = seams.ExecuteDecision(unit.GoalID, tree, decision)
		} else {
			result, runErr = seams.Execute(unit.GoalID, tree, slices.Clone(decision.Groups))
		}
		if runErr != nil {
			return HandlePrefixMemberRefusal(store, id, unit.GoalID, actor, at, runErr)
		}
		if len(result.Red) != 0 {
			if err := store.Update(id, func(current *Record) error {
				current.Proof.Status, current.Proof.Failure = "prefix-red", unit.GoalID
				current.Proof.RedGroups = slices.Clone(result.Red)
				current.Proof.PrefixGoal = unit.GoalID
				current.Transition(StateDiagnosing, at, "prefix-proof", actor, unit.GoalID)
				return nil
			}); err != nil {
				return fmt.Errorf("persist prefix red for %s: %w", unit.GoalID, err)
			}
			return &PrefixRedError{GoalID: unit.GoalID, Groups: result.Red}
		}
		if seams.Verify != nil {
			if err := seams.Verify(unit, tree, decision); err != nil {
				return fmt.Errorf("BATCH_PREFIX_PROOF_REFUSED: %s: %w", unit.GoalID, err)
			}
		}
		receipt.AttemptID, receipt.ResultPath, receipt.Executed, receipt.Reused = result.AttemptID, result.ResultPath, slices.Clone(result.Executed), result.Reused
		if receipt.Reused == nil {
			receipt.Reused = map[string]string{}
		}
		if err := store.Update(id, func(current *Record) error {
			if current.Receipts == nil {
				current.Receipts = map[string]PrefixReceipt{}
			}
			current.Receipts[unit.GoalID] = receipt
			return nil
		}); err != nil {
			return err
		}
	}
	return nil
}

func ensurePrefixEpisode(store Store, id string, snapshot Record, goalID string, index int, tree, decisionID string, maxAgeMS int64, at time.Time) (PrefixEpisode, error) {
	var episode PrefixEpisode
	err := store.Update(id, func(current *Record) error {
		if (current.State != StateLanding && current.State != StateSealed && current.State != StateProving) || current.BaseTree != snapshot.BaseTree || index >= len(current.PrefixTrees) || current.PrefixTrees[index] != tree ||
			!slices.EqualFunc(current.Units, snapshot.Units, func(left, right Unit) bool {
				return left.GoalID == right.GoalID && left.State == right.State && left.Claim == right.Claim
			}) {
			return fmt.Errorf("BATCH_PREFIX_DECISION_MOVED: %s changed before freshness episode retention", goalID)
		}
		if existing, ok := current.PrefixEpisodes[goalID]; ok && existing.DecisionID == decisionID && len(existing.Token) == 64 {
			if existing.ExpiresAt == "" {
				episode = existing
				return nil
			}
			if expiry, err := time.Parse(time.RFC3339Nano, existing.ExpiresAt); err == nil && at.Before(expiry) {
				episode = existing
				return nil
			}
		}
		var token [32]byte
		if _, err := rand.Read(token[:]); err != nil {
			return err
		}
		episode = PrefixEpisode{DecisionID: decisionID, Token: hex.EncodeToString(token[:])}
		if maxAgeMS > 0 {
			episode.ExpiresAt = at.Add(time.Duration(maxAgeMS) * time.Millisecond).UTC().Format(time.RFC3339Nano)
		}
		if current.PrefixEpisodes == nil {
			current.PrefixEpisodes = map[string]PrefixEpisode{}
		}
		current.PrefixEpisodes[goalID] = episode
		return nil
	})
	return episode, err
}

// EnsureDecisionEpisode persists a fresh proof decision before native
// admission. Repeating an unchanged decision reuses its token until expiry;
// a changed base, tree, membership or policy identity creates a new one.
func EnsureDecisionEpisode(store Store, id string, snapshot Record, goalID string, index int, tree, decisionID string, maxAgeMS int64, at time.Time) (PrefixEpisode, error) {
	return ensurePrefixEpisode(store, id, snapshot, goalID, index, tree, decisionID, maxAgeMS, at)
}

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
