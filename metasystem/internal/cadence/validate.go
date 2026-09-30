package cadence

// landing validate in production (lane runtime design r10 §4): the durable
// reservation in host lane state, the run launched under the kernel's
// custody store, and gaterun.Validate's seams bound to the ledger, the run
// store and the weight.

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/gaterun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/custody"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lock"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
	runpkg "github.com/widoriezebos/agentic-tools/metasystem/internal/run"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/strictjson"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

// ReservationPath is landing validate's reservation in host lane state.
func ReservationPath(home string) string {
	return filepath.Join(lane.HostDir(home), "landing-validation.json")
}

func reservationLock(home string) string {
	return filepath.Join(lane.HostDir(home), "landing-validation.lock")
}

// ReadReservation reads the reservation strictly; nil when there is none.
func ReadReservation(home string) (*gaterun.Validation, error) {
	var reservation gaterun.Validation
	if err := strictjson.Read(ReservationPath(home), &reservation); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	if reservation.RunID == "" || reservation.Key.TrunkTree == "" {
		return nil, fmt.Errorf("the validation reservation %s is incomplete", ReservationPath(home))
	}
	return &reservation, nil
}

func withReservation(home string, fn func(*gaterun.Validation) error) error {
	if err := os.MkdirAll(lane.HostDir(home), 0o700); err != nil {
		return err
	}
	held, err := lock.File(reservationLock(home), 0o600, lock.Exclusive)
	if err != nil {
		return err
	}
	defer func() { _ = held.Release() }()
	current, err := ReadReservation(home)
	if err != nil {
		return err
	}
	return fn(current)
}

func writeReservation(home string, reservation gaterun.Validation) error {
	data, err := json.MarshalIndent(reservation, "", "  ")
	if err != nil {
		return err
	}
	_, err = atomicfile.WriteFile(ReservationPath(home), append(data, '\n'), 0o600, lane.HostDir(home))
	return err
}

// Reserve records a run's key and id durably before it launches; another
// run's reservation refuses it.
func Reserve(home string, reservation gaterun.Validation) error {
	return withReservation(home, func(current *gaterun.Validation) error {
		if current != nil {
			return fmt.Errorf("validation run %s is already reserved", current.RunID)
		}
		return writeReservation(home, reservation)
	})
}

// RecordReservation updates the reservation of the same run id.
func RecordReservation(home string, reservation gaterun.Validation) error {
	return withReservation(home, func(current *gaterun.Validation) error {
		if current == nil || current.RunID != reservation.RunID {
			return fmt.Errorf("validation run %s is not the reserved run", reservation.RunID)
		}
		return writeReservation(home, reservation)
	})
}

// ClearReservation removes the reservation of runID only.
func ClearReservation(home, runID string) error {
	return withReservation(home, func(current *gaterun.Validation) error {
		if current == nil {
			return nil
		}
		if current.RunID != runID {
			return fmt.Errorf("validation run %s is not the reserved run (%s is)", runID, current.RunID)
		}
		err := os.Remove(ReservationPath(home))
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return err
	})
}

// custodySubject is the subject a validation run's custody is opened for.
func custodySubject(runID string) string { return "validation " + runID }

// ReservationCustody reads a reserved run's custody: by its record, or, when
// the launch did not get to record the id, by the run id it was opened
// for. No record opened for the run means nothing was started.
func ReservationCustody(home string, reservation gaterun.Validation, probes custody.Probes) (string, string) {
	id := reservation.Custody
	if id == "" {
		record, ok, err := custody.FindSubject(home, custody.KindValidate, custodySubject(reservation.RunID))
		if err != nil {
			return gaterun.CustodyUnknown, "the custody store can't be read: " + err.Error()
		}
		if !ok {
			return gaterun.CustodyDead, "nothing was started for run " + reservation.RunID
		}
		id = record.ID
	}
	state, err := custody.Probe(home, id, probes)
	if err != nil {
		return gaterun.CustodyUnknown, err.Error()
	}
	return state.State, state.Why
}

// FinalizationPending is the landing agent's wake read (A-a's
// finalization-pending reason): a validation run is reserved and its
// custody is no longer live, so a landing validate would finalize it or run
// it again. Unknown custody is an error: the wake reads it as unread.
func FinalizationPending(home string) (bool, error) {
	reservation, err := ReadReservation(home)
	if err != nil || reservation == nil {
		return false, err
	}
	state, why := ReservationCustody(home, *reservation, custody.Probes{})
	switch state {
	case gaterun.CustodyLive:
		return false, nil
	case gaterun.CustodyDead:
		return true, nil
	}
	return false, fmt.Errorf("validation run %s: %s", reservation.RunID, why)
}

// cadenceResultPath is the result file a validation run writes.
func cadenceResultPath(root, runID string) string {
	return filepath.Join(root, "artifacts", "agents", "proof-runs", "cadence", runID+".json")
}

// RunOutcome reads how a settled validation run ended: terminal green or
// red in the run store with a readable result is usable; anything else
// (launch-failed, ended-unknown, an unreadable record or result) is not.
func RunOutcome(store *runpkg.Store, root, runID string) gaterun.RunOutcome {
	if _, err := store.Assess(runID); err != nil {
		return gaterun.RunOutcome{Why: "the run record can't be assessed: " + err.Error()}
	}
	record, err := store.Read(runID)
	if err != nil || record == nil {
		return gaterun.RunOutcome{Why: fmt.Sprintf("the run record can't be read: %v", err)}
	}
	if record.Status != runpkg.StatusGreen && record.Status != runpkg.StatusRed {
		return gaterun.RunOutcome{Why: "the run ended " + record.Status}
	}
	var result proofrun.TestResult
	if err := strictjson.Read(cadenceResultPath(root, runID), &result); err != nil {
		return gaterun.RunOutcome{Why: "the run's result can't be read: " + err.Error()}
	}
	if result.AttemptID == "" {
		return gaterun.RunOutcome{Why: "the run's result names no attempt"}
	}
	return gaterun.RunOutcome{Usable: true, Result: result}
}

// ValidateLane is one landing validate on the lane whose host lane state is
// home and whose installation is root.
type ValidateLane struct {
	Home, Root string
	// Owner is the lane's cadence identity: its lineage and custody epoch,
	// and the trunk fetch, weight threshold, testing preparation and worker
	// policy.
	Owner Owner
	Clock func() time.Time
	// Probes are custody's reads; Poll is how long an attached call sleeps
	// between reads.
	Probes custody.Probes
	Poll   time.Duration
	Sleep  func(time.Duration)
	// Publish publishes a finalized status (K-c routes it through the lane
	// publication boundary); nil uses the ledger's claim and publish.
	Publish func(goal.CadenceClaimKey, goal.CadenceStatus, []goal.TrunkRedRecordGroup) error
	// Gate admits a launch and a publication under the lane's pause (K2):
	// it runs start under the host flock when the lane admits validate.
	// nil admits.
	Gate func(start func() error) error
}

// Seams binds gaterun.Validate to production.
func (v ValidateLane) Seams() (gaterun.ValidateSeams, error) {
	root, home, clock := v.Root, v.Home, v.Clock
	endpoint, err := goal.ResolveEndpoint(root)
	if err != nil {
		return gaterun.ValidateSeams{}, err
	}
	machine, err := goal.ResolveMachine(root)
	if err != nil {
		return gaterun.ValidateSeams{}, err
	}
	actor := goal.Actor{Machine: machine, Lineage: v.Owner.Lineage}
	store := cadenceRunStore(root, v.Owner, clock)
	poll, sleep := v.Poll, v.Sleep
	if poll <= 0 {
		poll = 5 * time.Second
	}
	if sleep == nil {
		sleep = time.Sleep
	}
	gate := v.Gate
	if gate == nil {
		gate = func(start func() error) error { return start() }
	}
	publish := v.Publish
	if publish == nil {
		ledger := gaterun.GoalCadenceLedger{Endpoint: endpoint, Actor: actor}
		publish = func(key goal.CadenceClaimKey, status goal.CadenceStatus, red []goal.TrunkRedRecordGroup) error {
			now := clock().UTC()
			claim, err := ledger.Claim(now, key, gaterun.CadenceForcedInterval)
			if err != nil {
				return err
			}
			switch claim.Outcome {
			case goal.CadenceClaimComplete:
				return nil
			case goal.CadenceClaimAcquired:
				if claim.Claim == nil {
					return fmt.Errorf("the acquired validation claim has no record")
				}
				return ledger.Publish(now, claim.Claim.Opid, status, red)
			}
			return fmt.Errorf("another publication of this validation key is under way (%s)", claim.Outcome)
		}
	}
	latest := func() (*goal.CadenceStatus, error) {
		projection, err := goal.Project(endpoint, false, clock().UTC())
		if err != nil {
			return nil, err
		}
		return projection.Tree.Cadence, nil
	}
	return gaterun.ValidateSeams{
		Clock:       clock,
		Reservation: func() (*gaterun.Validation, error) { return ReadReservation(home) },
		Reserve:     func(reservation gaterun.Validation) error { return Reserve(home, reservation) },
		Record:      func(reservation gaterun.Validation) error { return RecordReservation(home, reservation) },
		Clear:       func(runID string) error { return ClearReservation(home, runID) },
		Custody: func(reservation gaterun.Validation) (string, string) {
			return ReservationCustody(home, reservation, v.Probes)
		},
		Barrier: func() ([]string, []string, error) {
			settlement, err := custody.Settle(home, v.Probes)
			return settlement.Live, settlement.Unknown, err
		},
		Wait: func(reservation gaterun.Validation) error {
			for {
				if state, _ := ReservationCustody(home, reservation, v.Probes); state != gaterun.CustodyLive {
					return nil
				}
				sleep(poll)
			}
		},
		Outcome: func(reservation gaterun.Validation) gaterun.RunOutcome {
			return RunOutcome(store, root, reservation.RunID)
		},
		Gap: func() error {
			projection, err := goal.Project(endpoint, false, clock().UTC())
			if err != nil {
				return fmt.Errorf("the goal ledger can't be read, so the standing validation authority is unknown: %w", err)
			}
			if gap := goal.StandingAuthorityGap(projection.Tree, AuthorityGoal, actor); gap != nil {
				return &gaterun.ValidateGap{Reason: gap.Reason + ", so no landing validation runs", Command: gap.Command}
			}
			return nil
		},
		Claim: func(at time.Time) (gaterun.CadenceAuthority, error) {
			return claimCadenceAuthority(endpoint, actor, v.Owner.Epoch, at)
		},
		Plan:   func() (gaterun.ValidationPlan, error) { return planValidation(root, v.Owner, clock) },
		Latest: latest,
		NewRunID: func() (string, error) {
			ulid, err := goal.NewOperationULID()
			return "cadence-" + strings.ToLower(ulid), err
		},
		Launch: func(reservation gaterun.Validation) (string, error) {
			id := ""
			err := gate(func() error {
				var err error
				id, err = launchValidation(home, root, v.Owner, clock, store, reservation)
				return err
			})
			return id, err
		},
		Weight: func() (gaterun.WeightState, error) {
			state, _, err := gaterun.WeightCheckAt(root, v.Owner.WeightThreshold(root), clock().UTC())
			return state, err
		},
		Discharge: func(authority gaterun.CadenceAuthority, runID string, at time.Time) error {
			_, err := gaterun.WeightDischargeAt(root, authority.GoalID, authority.ObligationRevision, runID, at)
			return err
		},
		Publish: func(key goal.CadenceClaimKey, status goal.CadenceStatus, red []goal.TrunkRedRecordGroup) error {
			return gate(func() error { return publish(key, status, red) })
		},
	}, nil
}

// planValidation fetches the trunk and decides the key and whether a run is
// due, from retained evidence without building.
func planValidation(root string, owner Owner, clock func() time.Time) (gaterun.ValidationPlan, error) {
	commit, tree, err := owner.FetchOrigin(root)
	if err != nil {
		return gaterun.ValidationPlan{}, fmt.Errorf("the trunk can't be fetched: %w", err)
	}
	trunk := gaterun.CadenceTrunk{Commit: commit, Tree: tree}
	now := clock().UTC()
	endpoint, err := goal.ResolveEndpoint(root)
	if err != nil {
		return gaterun.ValidationPlan{}, err
	}
	projection, err := goal.Project(endpoint, false, now)
	if err != nil {
		return gaterun.ValidationPlan{}, err
	}
	prepared, err := owner.Prepare(cadencePreparationRequest(root, tree))
	if err != nil {
		return gaterun.ValidationPlan{}, err
	}
	deepOnly, err := testpolicy.DeepOnlySectionGroupIDs(prepared.EffectiveContract)
	if err != nil {
		return gaterun.ValidationPlan{}, err
	}
	weight, due, err := gaterun.WeightCheckAt(root, owner.WeightThreshold(root), now)
	if err != nil {
		return gaterun.ValidationPlan{}, err
	}
	revalidation, err := revalidateCadenceWith(root, prepared, trunk, deepOnly, productionCadenceRevalidationDependencies(owner))
	if err != nil {
		return gaterun.ValidationPlan{}, err
	}
	return gaterun.PlanValidation(now, trunk, projection.Tree.Cadence, weight, due, deepOnly, revalidation)
}

// launchValidation starts the reserved run as a governed run of this
// engine's own test run, under custody: the wrapper is the custody child
// and leads its own session. It returns the custody record id and does not
// wait; the wrapper is reaped in the background.
func launchValidation(home, root string, owner Owner, clock func() time.Time, store *runpkg.Store, reservation gaterun.Validation) (string, error) {
	creation, err := store.BeginCreation("cadence-run")
	if err != nil {
		return "", err
	}
	defer creation.Close()
	epoch := owner.Epoch
	runID := reservation.RunID
	nonce, err := store.Launch(runpkg.Caller{Class: "MAIN", MainId: owner.Lineage, OwnerLineage: owner.Lineage, ClaimEpoch: &epoch}, runpkg.LaunchParams{
		Id: runID, Kind: "suite", Display: "deep cadence validation", Log: filepath.Join("artifacts", "agents", "runs", runID+".log"),
		GoalId: reservation.Authority.GoalID, ObligationRevision: reservation.Authority.ObligationRevision, StandingShared: true,
		FenceGeneration: &creation.Generation,
	})
	if err != nil {
		return "", err
	}
	record, err := store.Read(runID)
	if err != nil || record == nil {
		return "", fmt.Errorf("the validation run record is unreadable: %v", err)
	}
	self, err := os.Executable()
	if err != nil {
		_ = store.FailLaunch(runID, "the engine executable can't be found: "+err.Error())
		return "", err
	}
	testArgs := []string{"internal", "test", "run", "--root", root, "--goal", reservation.Authority.GoalID, "--tree", reservation.Trunk.Tree,
		"--mode", "deep", "--purpose", "cadence", "--result", cadenceResultPath(root, runID)}
	if reservation.ForceGroups {
		testArgs = append(testArgs, "--force-groups")
	}
	wrapArgs := append([]string{"run", "wrap", "--root", root, "--id", runID, "--nonce", nonce, "--log", record.Log, "--", self}, testArgs...)
	command := exec.Command(self, wrapArgs...)
	command.Dir, command.Env = root, os.Environ()
	command.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	opened, err := custody.Start(home, custody.KindValidate, custodySubject(runID), clock(), command)
	if err != nil {
		if command.Process == nil {
			_ = store.FailLaunch(runID, "wrapper spawn failed: "+err.Error())
		}
		return opened.ID, err
	}
	go func() { _ = command.Wait() }()
	if err := store.CompleteLaunch(runID, creation.Generation); err != nil {
		return opened.ID, err
	}
	return opened.ID, nil
}
