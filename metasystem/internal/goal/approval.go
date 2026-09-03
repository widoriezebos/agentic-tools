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

func budgetHasNormCoverage(repoRoot string, f *GoalFile, budget Budget) (norm uint64, covered bool, err error) {
	norm, err = config.GoalNormJobMinutes(filepath.Join(repoRoot, "metasystem.conf"))
	if err != nil {
		return 0, false, err
	}
	if budget.ReservedJobMinutesLimit <= norm {
		return norm, true, nil
	}
	return norm, f.NormApproval != nil && f.NormApproval.Minutes >= budget.ReservedJobMinutesLimit, nil
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
	norm, covered, err := budgetHasNormCoverage(repoRoot, f, *f.Budget)
	if err != nil {
		return Budget{}, err
	}
	if !covered {
		return Budget{}, refuseGoalNorm(f.Id, f.Budget.ReservedJobMinutesLimit, norm)
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
	f.Approved = &ApprovalRecord{
		By: r.Actor.historyActor(), At: r.stamp(), Revision: f.Revision, Opid: r.opid(),
		Authority: authority, Digest: ApprovalDigest(f.Intent, *f.Budget), ReviewBy: reviewBy,
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
		if err := budget.Validate(); err != nil {
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
					if f.State == StateQueued || f.Budget == nil {
						return nil, fmt.Errorf("goal %s is queued; approval requires one complete budget tuple", id)
					}
					existing := *f.Budget
					nextBudget = &existing
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
			norm = fmt.Sprintf("%s/%d/%d", f.NormApproval.ApprovedRef, f.NormApproval.Minutes, f.NormApproval.GoalRevision)
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
