package goal

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
)

// ApprovalSweepListing is the exact, digest-bound world presented for the
// one-time grandfathering decision.
type ApprovalSweepListing struct {
	Lines   []string
	Digest  string
	Skipped []string
}

type ClassificationProposal struct {
	ID     string
	Tier   uint8
	Reason string
}

type ClassificationSweepListing struct {
	Proposals        []ClassificationProposal
	Lines            []string
	Digest           string
	TierLawInstalled bool
}

func classificationListing(t *TreeGoals, draft []byte) (ClassificationSweepListing, error) {
	if !utf8.Valid(draft) {
		return ClassificationSweepListing{}, fmt.Errorf("SWEEP_MALFORMED_ROW: line 1 is not UTF-8")
	}
	rows := map[string]ClassificationProposal{}
	for index, raw := range strings.Split(strings.ReplaceAll(string(draft), "\r\n", "\n"), "\n") {
		lineNo := index + 1
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		malformed := false
		for _, r := range raw {
			if r == '\t' || r < 0x20 || r == 0x7f {
				malformed = true
				break
			}
		}
		fields := strings.Fields(raw)
		if malformed || len(fields) < 3 || !validId(fields[0]) || len(strings.Join(fields[2:], " ")) > 200 {
			id := "unknown"
			if len(fields) > 0 {
				id = fields[0]
			}
			return ClassificationSweepListing{}, fmt.Errorf("SWEEP_MALFORMED_ROW: line %d goal %s must be <goal-id> <tier> <reason>", lineNo, id)
		}
		tierValue, err := strconv.ParseUint(fields[1], 10, 8)
		if err != nil || tierValue < 1 || tierValue > 3 {
			return ClassificationSweepListing{}, fmt.Errorf("SWEEP_MALFORMED_ROW: line %d goal %s has tier %s, want 1, 2, or 3", lineNo, fields[0], fields[1])
		}
		goalFile := t.Live[fields[0]]
		if goalFile == nil {
			return ClassificationSweepListing{}, fmt.Errorf("SWEEP_UNKNOWN_GOAL: goal %s is not an open tierless goal", fields[0])
		}
		if goalFile.Tier != 0 {
			continue
		}
		if _, duplicate := rows[fields[0]]; duplicate {
			return ClassificationSweepListing{}, fmt.Errorf("SWEEP_DUPLICATE_GOAL: goal %s appears more than once", fields[0])
		}
		rows[fields[0]] = ClassificationProposal{ID: fields[0], Tier: uint8(tierValue), Reason: strings.Join(fields[2:], " ")}
	}
	var missing []string
	for _, id := range sortedGoalIds(t.Live) {
		if t.Live[id].Tier == 0 {
			if _, ok := rows[id]; !ok {
				missing = append(missing, id)
			}
		}
	}
	if len(missing) > 0 {
		return ClassificationSweepListing{}, fmt.Errorf("SWEEP_INCOMPLETE: goal %s is absent from the draft", missing[0])
	}
	listing := ClassificationSweepListing{TierLawInstalled: t.Root != nil && t.Root.TierLaw != ""}
	ids := make([]string, 0, len(rows))
	for id := range rows {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		proposal := rows[id]
		listing.Proposals = append(listing.Proposals, proposal)
		listing.Lines = append(listing.Lines, fmt.Sprintf("%s %d %s", proposal.ID, proposal.Tier, proposal.Reason))
	}
	sum := sha256.Sum256([]byte(strings.Join(listing.Lines, "\n") + "\n"))
	listing.Digest = hex.EncodeToString(sum[:])
	return listing, nil
}

func PreviewClassificationSweep(e Endpoint, draft []byte, now time.Time) (ClassificationSweepListing, error) {
	p, err := Project(e, false, now)
	if err != nil {
		return ClassificationSweepListing{}, err
	}
	return classificationListing(p.Tree, draft)
}

// InstallTierLaw records the classify-sweep law when there were no tierless
// goals to edit. The marker is still its own durable confirmation act.
func InstallTierLaw(r VerbRequest) (PublishResult, error) {
	if r.Actor.Human == "" {
		return PublishResult{}, fmt.Errorf("classify-sweep confirmation requires --by")
	}
	return Publish(r.Endpoint, installTierLawRequest(r))
}

func installTierLawRequest(r VerbRequest) PublishRequest {
	return PublishRequest{
		Opid: r.opid(), Machine: r.Actor.Machine, Lineage: r.Actor.Lineage,
		Intent:  Intent{Verb: "classify-sweep", Args: intentArgs(r, nil)},
		Message: "goal classify-sweep",
		Mutate: func(tip string) ([]Change, error) {
			t, err := loadTree(r.Endpoint.Root, tip)
			if err != nil {
				return nil, err
			}
			if t.Root.TierLaw != "" {
				if t.Root.TierLaw == r.opid() {
					return nil, AlreadyApplied{}
				}
				return nil, LostToCompetitor{Winner: t.Root.TierLaw}
			}
			for _, id := range sortedGoalIds(t.Live) {
				if t.Live[id].Tier == 0 {
					return nil, fmt.Errorf("SWEEP_LISTING_CHANGED: goal %s is still an open tierless goal; preview again", id)
				}
			}
			t.Root.TierLaw = r.opid()
			t.Root.Revision++
			t.Root.History = append(t.Root.History, HistoryLine{
				At: r.stamp(), Opid: r.opid(), Verb: "edit", Actor: r.Actor.historyActor(), Keep: -1, Reason: "TierLaw",
			})
			return []Change{{Path: goalsPrefix + "backlog.md", Content: RenderRoot(t.Root)}}, nil
		},
		Validate: func(commit string) error { return ValidateCommit(r.Endpoint.Root, commit) },
	}
}

// ClassifyTier applies one migration edit. The final edit installs TierLaw in
// the same transaction, so an interruption leaves either remaining tierless
// goals with no marker or a complete classified ledger with its marker.
func ClassifyTier(r VerbRequest, proposal ClassificationProposal, installLaw bool) (PublishResult, error) {
	if r.Actor.Human == "" || proposal.Tier < 1 || proposal.Tier > 3 || strings.TrimSpace(proposal.Reason) == "" {
		return PublishResult{}, fmt.Errorf("classify-sweep confirmation requires --by and a valid proposal")
	}
	return Publish(r.Endpoint, PublishRequest{
		Opid: r.opid(), Machine: r.Actor.Machine, Lineage: r.Actor.Lineage,
		Intent:  Intent{Verb: "edit", Targets: []string{proposal.ID}, Deltas: []FieldDelta{{Target: proposal.ID, Field: "tier", New: strconv.Itoa(int(proposal.Tier))}}, Args: intentArgs(r, map[string]string{"classifyReason": proposal.Reason})},
		Message: "goal classify-sweep " + proposal.ID,
		Mutate: func(tip string) ([]Change, error) {
			t, err := loadTree(r.Endpoint.Root, tip)
			if err != nil {
				return nil, err
			}
			f := t.Live[proposal.ID]
			if f == nil || f.Tier != 0 {
				return nil, fmt.Errorf("SWEEP_LISTING_CHANGED: goal %s is no longer an open tierless goal", proposal.ID)
			}
			box, err := config.TierBox(filepath.Join(r.Endpoint.Root, "metasystem.conf"), proposal.Tier)
			if err != nil {
				return nil, err
			}
			f.Tier = proposal.Tier
			if f.Budget == nil {
				f.Budget = &box
			} else {
				// Every tierless tuple was admitted before the fifth member had
				// tier-specific meaning. This also covers a legacy tuple that an
				// unrelated write already expanded to five stored members.
				f.Budget.ReviewRoundLimit = box.ReviewRoundLimit
			}
			f.legacyFourBudget = false
			touch(f, r, "edit", []string{proposal.ID})
			f.History[len(f.History)-1].Reason = proposal.Reason
			if f.Approved != nil {
				f.Approved.Digest = ApprovalDigest(f.Intent, f.Tier, *f.Budget)
			}
			changes := []Change{{Path: livePath(proposal.ID), Content: RenderFile(f)}}
			if installLaw {
				for _, id := range sortedGoalIds(t.Live) {
					if id != proposal.ID && t.Live[id].Tier == 0 {
						return nil, fmt.Errorf("SWEEP_LISTING_CHANGED: goal %s is still an open tierless goal; preview again", id)
					}
				}
				t.Root.TierLaw = r.opid()
				t.Root.Revision++
				t.Root.History = append(t.Root.History, HistoryLine{At: r.stamp(), Opid: r.opid(), Verb: "edit", Actor: r.Actor.historyActor(), Targets: []string{proposal.ID}, Keep: -1, Reason: "TierLaw"})
				changes = appendRootChange(t, changes)
			}
			return ackDisplacements(t, r, changes), nil
		},
		Validate: func(commit string) error { return ValidateCommit(r.Endpoint.Root, commit) },
	})
}

func approvalHorizon(t *TreeGoals, now time.Time) ApprovalHorizon {
	h := ApprovalHorizon{Now: now}
	if t != nil && t.Root != nil && t.Root.FleetEnrollment != nil {
		h.EnrolledAt, _ = time.Parse(time.RFC3339, t.Root.FleetEnrollment.At)
	}
	return h
}

func approvalRequired(f *GoalFile, verb string) error {
	state := "missing"
	id := "unknown"
	if f != nil {
		state, id = f.State, f.Id
	}
	return fmt.Errorf("APPROVAL_REQUIRED: goal %s is %s and not approved for execution; only the human approves it with goal approve -- this %s is refused", id, state, verb)
}

func budgetHasNormCoverage(repoRoot string, f *GoalFile, budget Budget) (box Budget, covered bool, err error) {
	tier := f.Tier
	if tier == 0 {
		tier = 3
	}
	box, err = config.TierBox(filepath.Join(repoRoot, "metasystem.conf"), tier)
	if err != nil {
		return Budget{}, false, err
	}
	if budget.ReservedJobMinutesLimit <= box.ReservedJobMinutesLimit && budget.ReviewRoundLimit <= box.ReviewRoundLimit {
		return box, true, nil
	}
	return box, f.NormApproval != nil && f.NormApproval.Minutes >= budget.ReservedJobMinutesLimit && f.NormApproval.ReviewRounds >= budget.ReviewRoundLimit, nil
}

// requireApprovedForClaim is the single admission gate for every path that
// creates a claimed revision.
func requireApprovedForClaim(repoRoot string, t *TreeGoals, f *GoalFile, now time.Time, verb string) (Budget, error) {
	if f == nil || f.Approved == nil || f.Budget == nil {
		return Budget{}, approvalRequired(f, verb)
	}
	if err := f.ValidateApprovalRecord(); err != nil {
		return Budget{}, fmt.Errorf("APPROVAL_REQUIRED: goal %s has an invalid approval: %v", f.Id, err)
	}
	box, covered, err := budgetHasNormCoverage(repoRoot, f, *f.Budget)
	if err != nil {
		return Budget{}, err
	}
	if !covered {
		return Budget{}, refuseGoalNorm(f.Id, *f.Budget, box)
	}
	if expired, why := f.ApprovalExpired(approvalHorizon(t, now)); expired {
		return Budget{}, fmt.Errorf("APPROVAL_EXPIRED: goal %s was approved by a relayed word (review by %s, approved %s); that approval no longer admits new work because %s; a fresh approval is required at the enrolled terminal",
			f.Id, f.Approved.ReviewBy, f.Approved.At, why)
	}
	return *f.Budget, nil
}

func restingState(f *GoalFile) string {
	if f != nil && f.Approved != nil {
		return StateApproved
	}
	return StateQueued
}

func approvalProofClass(root string, proof *humanauthority.Proof) (authority, reviewBy string, temporary bool, err error) {
	if proof == nil {
		return "", "", false, fmt.Errorf("human approval requires freshly observed enrolled-human authority or a recorded temporary relay whose human provenance is not verified")
	}
	if proof.ValidFor(root) {
		return ApprovalAuthorityProven, "", false, nil
	}
	if proof.AuthorizesResume(root) && proof.TemporaryResumeFor(root) {
		return ApprovalAuthorityRelayed, proof.ReviewBy, true, nil
	}
	return "", "", false, fmt.Errorf("human approval requires freshly observed enrolled-human authority or a recorded temporary relay whose human provenance is not verified")
}

func refuseRelayedAfterFleetEnrollment(t *TreeGoals, temporary bool) error {
	if temporary && t != nil && t.Root != nil && t.Root.FleetEnrollment != nil {
		return fmt.Errorf("RELAY_AFTER_ENROLLMENT: the fleet enrolled its first agent-free terminal at %s on %s; relayed words end at the first enrolled session",
			t.Root.FleetEnrollment.At, t.Root.FleetEnrollment.Machine)
	}
	return nil
}

func bindApproval(f *GoalFile, r VerbRequest, authority, reviewBy string) {
	digest := ApprovalDigest(f.Intent, f.Tier, *f.Budget)
	if f.Tier == 0 {
		digest = legacyApprovalDigest(f.Intent, *f.Budget)
	}
	f.Approved = &ApprovalRecord{
		By: r.Actor.historyActor(), At: r.stamp(), Revision: f.Revision, Opid: r.opid(),
		Authority: authority, Digest: digest, ReviewBy: reviewBy,
	}
}

func recordApprovalRelay(f *GoalFile, proof *humanauthority.Proof, temporary bool) {
	if temporary {
		f.History[len(f.History)-1].recordTemporaryRelay(proof.ReviewBy, proof.Departure, proof.TemporaryHumanWord)
	}
}

func appendRootChange(t *TreeGoals, changes []Change) []Change {
	rootChange := Change{Path: goalsPrefix + "backlog.md", Content: RenderRoot(t.Root)}
	for i := range changes {
		if changes[i].Path == rootChange.Path {
			changes[i] = rootChange
			return changes
		}
	}
	return append(changes, rootChange)
}

func armApprovalGate(t *TreeGoals, r VerbRequest, changes []Change) []Change {
	if t.Root == nil || t.Root.ApprovalGate != nil {
		return changes
	}
	t.Root.ApprovalGate = &ApprovalGateRecord{Since: r.stamp(), Opid: r.opid()}
	rootAlreadyTouched := false
	for _, change := range changes {
		if change.Path == goalsPrefix+"backlog.md" {
			rootAlreadyTouched = true
			break
		}
	}
	if !rootAlreadyTouched {
		t.Root.Revision++
	}
	return appendRootChange(t, changes)
}

// Approve moves queued work into the human-approved state or re-ratifies a
// standing record. One complete tuple applies to every target.
func Approve(r VerbRequest, ids []string, budget *Budget, proof *humanauthority.Proof) (PublishResult, error) {
	if r.Actor.Human == "" {
		return PublishResult{}, fmt.Errorf("goal approve is human-only and requires --by from an authorized human boundary")
	}
	if len(ids) == 0 {
		return PublishResult{}, fmt.Errorf("goal approve needs at least one goal id")
	}
	authority, reviewBy, temporary, err := approvalProofClass(r.Endpoint.Root, proof)
	if err != nil {
		return PublishResult{}, err
	}
	if budget != nil {
		maximum, maxErr := config.ReviewRoundMax(filepath.Join(r.Endpoint.Root, "metasystem.conf"))
		if maxErr != nil {
			return PublishResult{}, maxErr
		}
		if err := budget.Validate(maximum); err != nil {
			return PublishResult{}, fmt.Errorf("invalid approval budget: %v", err)
		}
	}
	targets := append([]string(nil), ids...)
	sort.Strings(targets)
	return Publish(r.Endpoint, PublishRequest{
		Opid: r.opid(), Machine: r.Actor.Machine, Lineage: r.Actor.Lineage,
		Intent: Intent{Verb: "approve", Targets: targets, Args: intentArgs(r, func() map[string]string {
			args := map[string]string{}
			if budget != nil {
				args = budgetIntentArgs(*budget)
			}
			return args
		}())},
		Message: "goal approve " + strings.Join(targets, ","),
		Mutate: func(tip string) ([]Change, error) {
			t, err := loadTree(r.Endpoint.Root, tip)
			if err != nil {
				return nil, err
			}
			if err := refuseRelayedAfterFleetEnrollment(t, temporary); err != nil {
				return nil, err
			}
			if len(targets) > 1 {
				for _, id := range targets {
					if f := t.Live[id]; f != nil && f.State == StateClaimed {
						return nil, fmt.Errorf("goal %s is claimed; a repeated --id approval batch cannot change or re-ratify claimed work", id)
					}
				}
			}
			var changes []Change
			changed := false
			for _, id := range targets {
				f := t.Live[id]
				if f == nil {
					return nil, fmt.Errorf("goal %s is not live; approval does not rewrite the archive", id)
				}
				if opidLanded(f, r) {
					return nil, AlreadyApplied{}
				}
				if f.State != StateQueued && f.State != StateApproved && f.State != StateClaimed {
					return nil, fmt.Errorf("goal %s is %s; approve admits queued or already-approved work and may re-ratify a claim", id, f.State)
				}
				if f.State == StateClaimed && budget != nil {
					return nil, fmt.Errorf("goal %s is claimed; its tuple changes through goal set-budget", id)
				}
				nextBudget := budget
				if nextBudget == nil {
					if f.Tier == 0 {
						return nil, fmt.Errorf("goal %s has no tier; classify it before approval", id)
					}
					box, boxErr := config.TierBox(filepath.Join(r.Endpoint.Root, "metasystem.conf"), f.Tier)
					if boxErr != nil {
						return nil, boxErr
					}
					nextBudget = &box
				}
				var norm *GoalNormApprovalClaim
				if budget == nil {
					norm = f.NormApproval
				} else {
					var normErr error
					norm, normErr = goalNormApproval(r.Endpoint.Root, t, f, *nextBudget, r.ApprovedRef)
					if normErr != nil {
						return nil, normErr
					}
				}
				if f.Approved != nil && f.Approved.Authority == ApprovalAuthorityProven && authority == ApprovalAuthorityProven &&
					*f.Budget == *nextBudget && sameGoalNormApproval(f.NormApproval, norm) {
					if expired, _ := f.ApprovalExpired(approvalHorizon(t, r.Now)); !expired {
						continue
					}
				}
				if temporary {
					if err := repeatedRelayedActError(t.Root, f, "approve", proof.Departure); err != nil {
						return nil, err
					}
				}
				f.Budget = nextBudget
				f.NormApproval = norm
				if f.State == StateQueued {
					f.State = StateApproved
				}
				touch(f, r, "approve", targets)
				recordApprovalRelay(f, proof, temporary)
				bindApproval(f, r, authority, reviewBy)
				changes = append(changes, Change{Path: livePath(id), Content: RenderFile(f)})
				changed = true
			}
			if !changed {
				return nil, NothingToDo{Reason: "every target already has the same proven approval"}
			}
			changes = armApprovalGate(t, r, changes)
			return ackDisplacements(t, r, changes), nil
		},
		Validate: func(commit string) error { return ValidateCommit(r.Endpoint.Root, commit) },
	})
}

// Unapprove withdraws execution authority. A claimed goal is parked so
// already-running jobs can finish while no new reservation can enter.
func Unapprove(r VerbRequest, id, because string, proof *humanauthority.Proof) (PublishResult, error) {
	if r.Actor.Human == "" || strings.TrimSpace(because) == "" {
		return PublishResult{}, fmt.Errorf("goal unapprove is human-only and requires --by and --because")
	}
	_, _, temporary, err := approvalProofClass(r.Endpoint.Root, proof)
	if err != nil {
		return PublishResult{}, err
	}
	return Publish(r.Endpoint, PublishRequest{
		Opid: r.opid(), Machine: r.Actor.Machine, Lineage: r.Actor.Lineage,
		Intent:  Intent{Verb: "unapprove", Targets: []string{id}, Args: intentArgs(r, map[string]string{"because": because})},
		Message: "goal unapprove " + id,
		Mutate: func(tip string) ([]Change, error) {
			t, err := loadTree(r.Endpoint.Root, tip)
			if err != nil {
				return nil, err
			}
			if err := refuseRelayedAfterFleetEnrollment(t, temporary); err != nil {
				return nil, err
			}
			f := t.Live[id]
			if f == nil || f.Approved == nil {
				return nil, approvalRequired(f, "unapprove")
			}
			if opidLanded(f, r) {
				return nil, AlreadyApplied{}
			}
			if temporary {
				if err := repeatedRelayedActError(t.Root, f, "unapprove", proof.Departure); err != nil {
					return nil, err
				}
			}
			displaced := ""
			switch f.State {
			case StateApproved:
				f.State = StateQueued
			case StateClaimed:
				if f.StopFence != nil {
					return nil, fmt.Errorf("goal %s is breach-stopped; resume it before withdrawing approval", id)
				}
				displaced = pairMarker(f.Claimed)
				f.State = StateParked
				f.Parked = &ParkRecord{By: r.Actor.historyActor(), At: r.stamp(), Because: "approval revoked: " + because, Displaced: displaced}
				if err := clearClaimBinding(f); err != nil {
					return nil, err
				}
			case StateParked:
			default:
				return nil, fmt.Errorf("goal %s is %s; its approval cannot be withdrawn in this state", id, f.State)
			}
			f.Approved, f.Budget, f.NormApproval = nil, nil, nil
			touchDisplaced(f, r, "unapprove", []string{id}, displaced)
			f.History[len(f.History)-1].Reason = because
			recordApprovalRelay(f, proof, temporary)
			return ackDisplacements(t, r, []Change{{Path: livePath(id), Content: RenderFile(f)}}), nil
		},
		Validate: func(commit string) error { return ValidateCommit(r.Endpoint.Root, commit) },
	})
}

func sweepListing(t *TreeGoals) ApprovalSweepListing {
	listing := ApprovalSweepListing{}
	for _, id := range sortedGoalIds(t.Live) {
		f := t.Live[id]
		budget := "-"
		if f.Budget != nil && f.Budget.Validate() == nil {
			budget = renderBudgetRecord(*f.Budget)
		} else {
			listing.Skipped = append(listing.Skipped, id)
		}
		norm := "-"
		if f.NormApproval != nil {
			norm = fmt.Sprintf("%s/%d/%d/%d", f.NormApproval.ApprovedRef, f.NormApproval.Minutes, f.NormApproval.ReviewRounds, f.NormApproval.GoalRevision)
		}
		authority := "-"
		if f.Approved != nil {
			authority = f.Approved.Authority
		}
		// strconv.Quote makes the exact intent visible while keeping one line
		// per goal. The digest covers these exact bytes.
		listing.Lines = append(listing.Lines, fmt.Sprintf("id=%s state=%s origin=%s intent=%s budget=%s normApproval=%s authority=%s",
			f.Id, f.State, f.Origin, strconv.Quote(f.Intent), budget, norm, authority))
	}
	sort.Strings(listing.Lines)
	sum := sha256.Sum256([]byte(strings.Join(listing.Lines, "\n") + "\n"))
	listing.Digest = hex.EncodeToString(sum[:])
	return listing
}

// PreviewApprovalSweep reads the accepted tree without changing it.
func PreviewApprovalSweep(e Endpoint, now time.Time) (ApprovalSweepListing, error) {
	p, err := Project(e, false, now)
	if err != nil {
		return ApprovalSweepListing{}, err
	}
	return sweepListing(p.Tree), nil
}

// ApproveSweep confirms the exact listing the human saw. Its digest includes
// each exact intent, so an edit between preview and confirmation refuses.
func ApproveSweep(r VerbRequest, confirm string, proof *humanauthority.Proof) (PublishResult, error) {
	if r.Actor.Human == "" || !hexDigest(confirm) {
		return PublishResult{}, fmt.Errorf("goal approve --sweep confirmation is human-only and requires --by plus a listing sha256")
	}
	authority, reviewBy, temporary, err := approvalProofClass(r.Endpoint.Root, proof)
	if err != nil {
		return PublishResult{}, err
	}
	return Publish(r.Endpoint, PublishRequest{
		Opid: r.opid(), Machine: r.Actor.Machine, Lineage: r.Actor.Lineage,
		Intent:  Intent{Verb: "approve", Args: intentArgs(r, map[string]string{"sweep": "true", "listing": confirm})},
		Message: "goal approve --sweep",
		Mutate: func(tip string) ([]Change, error) {
			t, err := loadTree(r.Endpoint.Root, tip)
			if err != nil {
				return nil, err
			}
			if err := refuseRelayedAfterFleetEnrollment(t, temporary); err != nil {
				return nil, err
			}
			listing := sweepListing(t)
			if listing.Digest != confirm {
				return nil, fmt.Errorf("SWEEP_LISTING_CHANGED: confirmation %s does not match current listing %s; preview again", confirm, listing.Digest)
			}
			if temporary {
				for _, f := range t.Live {
					if f.Approved != nil {
						return nil, fmt.Errorf("a relayed sweep refuses after any approval exists; use the enrolled terminal to ratify the fleet")
					}
				}
				if first, ok := firstRecordedRelayedActIn(t.Root.History, "", "approve", proof.Departure); ok {
					return nil, fmt.Errorf("the fleet already used its relayed sweep on %s with recorded word %q", first.At, first.TemporaryHumanWord)
				}
			}
			var changes []Change
			approved, ratified := 0, 0
			for _, id := range sortedGoalIds(t.Live) {
				f := t.Live[id]
				if f.Budget == nil || f.Budget.Validate() != nil || (f.Approved != nil && f.Approved.Authority == ApprovalAuthorityProven) {
					continue
				}
				_, covered, coverageErr := budgetHasNormCoverage(r.Endpoint.Root, f, *f.Budget)
				if coverageErr != nil {
					return nil, coverageErr
				}
				if !covered {
					continue
				}
				if f.Approved == nil {
					approved++
				} else {
					ratified++
				}
				if f.State == StateQueued {
					f.State = StateApproved
				}
				touch(f, r, "approve", []string{id})
				f.History[len(f.History)-1].Reason = "sweep"
				recordApprovalRelay(f, proof, temporary)
				bindApproval(f, r, authority, reviewBy)
				changes = append(changes, Change{Path: livePath(id), Content: RenderFile(f)})
			}
			if len(changes) == 0 {
				return nil, NothingToDo{Reason: "the proven sweep found no absent or relayed approval to ratify"}
			}
			t.Root.Revision++
			t.Root.History = append(t.Root.History, HistoryLine{At: r.stamp(), Opid: r.opid(), Verb: "approve",
				Actor: r.Actor.historyActor(), Keep: -1,
				Reason: fmt.Sprintf("sweep listing=%s approved=%d ratified=%d skipped=%s", confirm, approved, ratified, strings.Join(listing.Skipped, ","))})
			if temporary {
				t.Root.History[len(t.Root.History)-1].recordTemporaryRelay(proof.ReviewBy, proof.Departure, proof.TemporaryHumanWord)
			}
			changes = appendRootChange(t, changes)
			changes = armApprovalGate(t, r, changes)
			return ackDisplacements(t, r, changes), nil
		},
		Validate: func(commit string) error { return ValidateCommit(r.Endpoint.Root, commit) },
	})
}

// RecordFleetEnrollment publishes the first terminal enrollment into the
// synced root record. Later machines observe the same cutoff.
func RecordFleetEnrollment(r VerbRequest, generation uint64) (PublishResult, error) {
	if generation == 0 {
		return PublishResult{}, fmt.Errorf("fleet enrollment requires a positive generation")
	}
	return Publish(r.Endpoint, PublishRequest{
		Opid: r.opid(), Machine: r.Actor.Machine, Lineage: r.Actor.Lineage,
		Intent:  Intent{Verb: "enroll-terminal", Args: intentArgs(r, map[string]string{"generation": strconv.FormatUint(generation, 10)})},
		Message: "goal enroll-terminal",
		Mutate: func(tip string) ([]Change, error) {
			t, err := loadTree(r.Endpoint.Root, tip)
			if err != nil {
				return nil, err
			}
			if t.Root.FleetEnrollment != nil {
				return nil, NothingToDo{Reason: "the fleet's first terminal enrollment is already recorded"}
			}
			t.Root.FleetEnrollment = &FleetEnrollmentRecord{At: r.stamp(), Machine: r.Actor.Machine, Generation: generation, Opid: r.opid()}
			t.Root.Revision++
			t.Root.History = append(t.Root.History, HistoryLine{At: r.stamp(), Opid: r.opid(), Verb: "enroll-terminal", Actor: r.Actor.historyActor(), Keep: -1})
			return []Change{{Path: goalsPrefix + "backlog.md", Content: RenderRoot(t.Root)}}, nil
		},
		Validate: func(commit string) error { return ValidateCommit(r.Endpoint.Root, commit) },
	})
}
