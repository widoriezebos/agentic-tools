package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/board"
	dispatchcore "github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/hooks"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/up"
)

func personClaimsBed(t *testing.T) (*intentBed, intentOwners) {
	t.Helper()
	bed := newIntentBed(t, false, func(f *goal.GoalFile) {
		workApprovedBox(f)
		f.State, f.Claimed, f.StopCapability, f.StopFence = goal.StateApproved, nil, nil, nil
	})
	owners := bed.owners()
	owners.dependencies.claimHolder.Holder = func(string) (lease.CurrentHolderView, error) { return lease.CurrentHolderView{}, lease.ErrLeaseAbsent }
	owners.binding = func(root, id string, now time.Time) (dispatchcore.GoalBinding, error) {
		return dispatchcore.ResolveGoalBindingWithReads(root, id, now, dispatchcore.ProofAdmissionReads{
			ResolveEndpoint: owners.dependencies.endpoint, ResolveMachine: owners.dependencies.machine,
			Receipt: dispatchcore.ReceiptAdmissionSource{
				AcceptedLedgerTip: func(string) (string, bool, error) { return bed.repo.accepted, true, nil },
				TopLevel:          fakeTop(bed.root()),
				FileAt: func(_ string, tip, path string) ([]byte, bool, error) {
					data, ok := bed.repo.commit(tip).files[path]
					return data, ok, nil
				},
			},
		})
	}
	return bed, owners
}

func TestPersonClaimBuildSessionStartBuild(t *testing.T) {
	t.Parallel()
	for _, approved := range []bool{true, false} {
		t.Run(map[bool]string{true: "approved", false: "awaiting approval"}[approved], func(t *testing.T) {
			t.Parallel()
			bed := newWorkBedWith(t, func(file *goal.GoalFile) {
				workApprovedBox(file)
				file.State, file.Claimed, file.StopCapability, file.StopFence = goal.StateApproved, nil, nil, nil
				if !approved {
					makeQueued(file)
				}
			})
			owners := bed.workOwners()
			owners.dependencies.claimHolder.Holder = func(string) (lease.CurrentHolderView, error) {
				return lease.CurrentHolderView{MainId: "dead-main", OwnerLineage: "m1"}, nil
			}
			owners.dependencies.claimHolder.Caller = func(string, int64) (lease.ClassifyResult, error) {
				return lease.ClassifyResult{Class: lease.ClassUntrusted}, nil
			}
			ids := []string{bedGoal, "second-build"}
			second := bed.goalFile(bedGoal)
			second.Id = ids[1]
			bed.addGoal(second)
			for _, id := range ids {
				claimAreaDesign(t, bed.intentBed, id, "shared/**")
				reservePersonGoal(t, bed.intentBed, owners, id)
				bed.id = id
				result := claimFixtureBuild(t, bed, owners)
				if result.Outcome != intentRefused || !reflect.DeepEqual(result.Next.Argv, []string{"metasystem", "session", "start"}) || len(bed.starter.launched()) != 0 {
					t.Fatalf("build did not name the session remedy without launching: %+v", result)
				}
			}
			reserved := map[string]*goal.GoalFile{}
			for _, id := range ids {
				reserved[id] = bed.goalFile(id)
			}
			bed.lineage = "m1"
			owners = claimFixtureOwners(t, bed)
			owners.processes = defaultProcessIntentOwners()
			owners.processes.process.repositoryTop = fakeTop(bed.root())
			binary := filepath.Join(bed.root(), "bin", "metasystem")
			if err := os.MkdirAll(filepath.Dir(binary), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := testexec.WriteFile(binary, []byte("synthetic engine"), 0o755); err != nil {
				t.Fatal(err)
			}
			owners.processes.executable = func() (string, error) { return binary, nil }
			owners.processes.adoptionReads = &claimAdoptionReads{dependencies: owners.dependencies, clock: owners.commandNow, project: goal.Project}
			owners.processes.up = func(options up.Options) up.Result {
				if options.RestampStopCapability == nil {
					t.Fatal("session start lost its adoption callback")
				}
				adoption, err := options.RestampStopCapability(bed.root(), "m1", 1)
				if err != nil {
					t.Fatal(err)
				}
				return up.Result{Outcome: "armed", Authority: "writer", Adoption: &adoption, Components: []up.ComponentOutcome{{Component: "stop-capability", Outcome: "restamped", Adoption: &adoption}}}
			}
			code, started := bed.runJSON(owners, "session", "start", "--lineage", "m1")
			if code != 0 || started.Outcome != intentConfirmed {
				t.Fatalf("session remedy: %d %+v", code, started)
			}
			for _, id := range ids {
				bed.worktree = filepath.Join(t.TempDir(), "work")
				if err := os.MkdirAll(bed.worktree, 0700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(filepath.Dir(bed.worktree), "index"), []byte("index"), 0600); err != nil {
					t.Fatal(err)
				}
				file := bed.goalFile(id)
				if file.StopCapability.ClaimEpoch != 1 || !reflect.DeepEqual(file.Claimed, reserved[id].Claimed) || !reflect.DeepEqual(file.Budget, reserved[id].Budget) || !reflect.DeepEqual(file.Approved, reserved[id].Approved) {
					t.Fatalf("session adoption changed ownership/accounting: %+v", file)
				}
				bed.id = id
				if !approved {
					launches := len(bed.starter.launched())
					result := claimFixtureBuild(t, bed, owners)
					if result.Outcome != intentRefused || result.Next == nil || !reflect.DeepEqual(result.Next.Argv, []string{"metasystem", "goal", "approve", id}) || len(bed.starter.launched()) != launches {
						t.Fatalf("adopted unapproved goal launched: %+v", result)
					}
					if code, result := bed.runJSON(owners, "goal", "approve", id, "--budget", "norm", "--by", "Wido", "--fixture-human-authority"); code != 0 {
						t.Fatalf("approval remedy: %d %+v", code, result)
					}
				}
				if result := claimFixtureBuild(t, bed, owners); result.Outcome != intentConfirmed {
					t.Fatalf("build after remedies: %+v", result)
				}
			}
		})
	}
}

func TestPersonClaimBoardAndOfflineProjection(t *testing.T) {
	t.Parallel()
	bed, owners := personClaimsBed(t)
	reservePersonGoal(t, bed, owners, bedGoal)
	owners = adoptingHolder(owners, launch.SeatOwnerLineage, 9)
	owners.delivery = &intentDeliveryOwners{boardView: func(string, time.Time) board.View {
		return board.View{Readable: true, Seats: []board.SeatView{{Machine: "mac-cli", Goals: []board.GoalView{{Goal: bedGoal, Stage: board.StageClaimedIdle}}}}}
	}}
	inv := &intentInvocation{owners: owners, stateRoot: bed.root(), cwd: bed.root()}
	view := inv.hostBoardView(syncRequestTestNow)
	if text, ok := view.GoalLine(bedGoal, syncRequestTestNow, time.Local); !ok || !strings.Contains(text, "reserved, awaiting session start") {
		t.Fatalf("board implied a running agent: %+v %q", view, text)
	}
	calls := 0
	project := func(endpoint goal.Endpoint, fetch bool, now time.Time) (goal.Projection, error) {
		calls++
		if fetch {
			t.Fatal("offline hook adoption fetched")
		}
		return goal.Project(endpoint, fetch, now)
	}
	processes := defaultProcessIntentOwners()
	processes.process.repositoryTop = fakeTop(bed.root())
	processes.executable = func() (string, error) { return os.Executable() }
	processes.adoptionReads = &claimAdoptionReads{dependencies: owners.dependencies, clock: owners.commandNow, project: project}
	var result up.StopCapabilityRestampResult
	processes.up = func(options up.Options) up.Result {
		var err error
		result, err = options.RestampStopCapability(bed.root(), launch.SeatOwnerLineage, 9)
		if err != nil {
			t.Fatal(err)
		}
		return up.Result{Outcome: "armed", Adoption: &result, Components: []up.ComponentOutcome{{Component: "stop-capability", Outcome: "restamped", Adoption: &result}}}
	}
	var hookOut, hookErr bytes.Buffer
	hook := hookOwners{processes: &processes}
	state, err := stateroot.ParseInstallation(bed.root())
	if err != nil {
		t.Fatal(err)
	}
	if code := hook.Up(hooks.UpRequest{MetasystemRoot: state, Repo: bed.root()}, &hookOut, &hookErr); code != 0 || result.Pending || calls != 1 || bed.goalFile(bedGoal).StopCapability.ClaimEpoch != 9 || !strings.Contains(hookOut.String(), "adoption=restamped") {
		t.Fatalf("offline hook adoption: code=%d %+v calls=%d stdout=%s stderr=%s", code, result, calls, hookOut.String(), hookErr.String())
	}

	hookResult := result
	var explicitOut, explicitErr bytes.Buffer
	if code := runUpWithProcessOwners([]string{"--repo", bed.root(), "--metasystem-root", bed.root(), "--json"}, processes, &explicitOut, &explicitErr); code != 0 || calls != 1 || result.Observation == nil || result.Observation.Outcome != "fresh" || result.Goals[0].Outcome != "current" || !strings.Contains(explicitOut.String(), `"adoption"`) {
		t.Fatalf("explicit up lost the shared idempotent callback: code=%d calls=%d %+v stdout=%s stderr=%s", code, calls, result, explicitOut.String(), explicitErr.String())
	}
	result = hookResult
	var stdout, stderr bytes.Buffer
	if code := answerUp(up.Result{Outcome: "armed", Adoption: &result, Components: []up.ComponentOutcome{{Component: "stop-capability", Outcome: "restamped", Adoption: &result}}}, true, &stdout, &stderr); code != 0 || !strings.Contains(stdout.String(), `"adoption"`) || !strings.Contains(stderr.String(), "adoption=restamped") {
		t.Fatalf("up lost typed/text results: %d %s %s", code, stdout.String(), stderr.String())
	}
}

type personClaimTransport struct {
	goal.Repository
	publish  func(string, string) (goal.CASOutcome, error)
	accepted func() (string, bool, error)
}

func (r personClaimTransport) Publish(tip, commit string) (goal.CASOutcome, error) {
	if r.publish == nil {
		return r.Repository.Publish(tip, commit)
	}
	return r.publish(tip, commit)
}

func (r personClaimTransport) Accepted() (string, bool, error) {
	if r.accepted == nil {
		return r.Repository.Accepted()
	}
	return r.accepted()
}

func TestPersonClaimUnknownAdviceDoesNotVetoPublication(t *testing.T) {
	t.Parallel()
	bed, owners := personClaimsBed(t)
	endpoint := owners.dependencies.endpoint
	reads := 0
	owners.dependencies.endpoint = func(root string) (goal.Endpoint, error) {
		e, err := endpoint(root)
		e.Repository = personClaimTransport{Repository: e.Repository, accepted: func() (string, bool, error) {
			reads++
			if reads == 1 {
				return "", false, errors.New("local observation unavailable")
			}
			return bed.repo.Accepted()
		}}
		return e, err
	}
	reservePersonGoal(t, bed, owners, bedGoal)
	file := bed.goalFile(bedGoal)
	if reads < 2 || file.StopCapability.ClaimEpoch != 0 || file.Claimed.Known || !strings.Contains(strings.Join(file.Claimed.Warnings, ";"), "areas unknown") {
		t.Fatalf("unknown advice vetoed or guessed a reservation: reads=%d %+v", reads, file)
	}
}

func TestPersonClaimLostConfirmationAndPublicationRace(t *testing.T) {
	t.Parallel()
	for _, loseConfirmation := range []bool{true, false} {
		t.Run(map[bool]string{true: "lost confirmation", false: "release won publication"}[loseConfirmation], func(t *testing.T) {
			t.Parallel()
			bed, owners := personClaimsBed(t)
			for _, id := range []string{"a-one", "b-two", "c-three"} {
				f := bed.goalFile(bedGoal)
				f.Id = id
				bed.addGoal(f)
				reservePersonGoal(t, bed, owners, id)
			}
			owners = adoptingHolder(owners, launch.SeatOwnerLineage, 6)
			endpoint := owners.dependencies.endpoint
			publishes := 0
			owners.dependencies.endpoint = func(root string) (goal.Endpoint, error) {
				e, err := endpoint(root)
				e.Repository = personClaimTransport{Repository: e.Repository, publish: func(tip, commit string) (goal.CASOutcome, error) {
					publishes++
					if !loseConfirmation && publishes == 1 {
						f := bed.goalFile("a-one")
						f.State, f.Claimed, f.StopCapability = goal.StateApproved, nil, nil
						bed.addGoal(f)
						return goal.CASRefused, errors.New("another writer released the goal")
					}
					outcome, err := bed.repo.Publish(tip, commit)
					if loseConfirmation && publishes == 2 {
						bed.repo.captureErr = errors.New("confirming transport unavailable")
					}
					return outcome, err
				}}
				return e, err
			}
			result := adoptPersonGoals(t, bed, owners, launch.SeatOwnerLineage, 6)
			if !result.Pending || len(result.Goals) != 3 {
				t.Fatalf("partial adoption lost ids: %+v", result)
			}
			if loseConfirmation {
				if result.Goals[0].Outcome != "restamped" || result.Goals[1].Outcome != "pending" || result.Goals[1].Operation == "" || bed.goalFile("c-three").StopCapability.ClaimEpoch != 0 {
					t.Fatalf("lost confirmation was presented as rollback or erased success: %+v", result)
				}
				bed.repo.captureErr = nil
				e, err := owners.dependencies.endpoint(bed.root())
				if err != nil {
					t.Fatal(err)
				}
				e.ClaimHolder = owners.dependencies.claimHolder.Facts
				reports, err := goal.Recover(e)
				if err != nil {
					t.Fatalf("recover lost confirmation: %+v %v", reports, err)
				}
				entry, err := goal.ReadEntry(bed.root(), result.Goals[1].Operation)
				if err != nil || entry.Outcome != goal.OutcomeConfirmed {
					t.Fatalf("recovery did not confirm the published operation: %+v %v", entry, err)
				}
				retry := adoptPersonGoals(t, bed, owners, launch.SeatOwnerLineage, 6)
				if retry.Pending || retry.Goals[0].Outcome != "current" || retry.Goals[1].Outcome != "current" {
					t.Fatalf("recovery/retry replaced confirmed adoption: %+v", retry)
				}
			} else if bed.goalFile("a-one").Claimed != nil || result.Goals[0].Outcome != "pending" || result.Goals[1].Outcome != "restamped" || bed.goalFile("c-three").StopCapability.ClaimEpoch != 6 {
				t.Fatalf("publication race overwrote the winner or hid later goals: %+v", result)
			}
		})
	}
}

func reservePersonGoal(t *testing.T, bed *intentBed, owners intentOwners, id string, extra ...string) {
	t.Helper()
	args := append([]string{"goal", "claim", id, "--by", "Wido", "--fixture-human-authority"}, extra...)
	if code, result := bed.runJSON(owners, args...); code != 0 {
		t.Fatalf("reserve %s: exit=%d %+v", id, code, result)
	}
	f := bed.goalFile(id)
	if f.Claimed == nil || f.Claimed.By != "human:Wido" || !f.PersonalReservation() {
		t.Fatalf("reservation has no person provenance: %+v", f)
	}
}

func adoptingHolder(owners intentOwners, lineage string, epoch int64) intentOwners {
	owners.dependencies.claimHolder = claimHolderReaders{
		Holder: func(string) (lease.CurrentHolderView, error) {
			return lease.CurrentHolderView{MainId: "main-live", OwnerLineage: lineage, ClaimEpoch: epoch, Pid: 77}, nil
		},
		Caller: func(string, int64) (lease.ClassifyResult, error) {
			return lease.ClassifyResult{Class: lease.ClassMain, Holder: true, MainId: "main-live", ClaimEpoch: &epoch}, nil
		},
	}
	return owners
}

func adoptPersonGoals(t *testing.T, bed *intentBed, owners intentOwners, lineage string, epoch int64) up.StopCapabilityRestampResult {
	t.Helper()
	result, err := restampStopCapabilityWithReads(bed.root(), lineage, epoch, owners.dependencies, owners.commandNow, goal.Project)
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func TestPersonClaimPublicReservationAndAllAdoption(t *testing.T) {
	t.Parallel()
	bed, owners := personClaimsBed(t)
	second := bed.goalFile(bedGoal)
	second.Id = "second-reservation"
	second.History = append([]goal.HistoryLine(nil), second.History...)
	for i := range second.History {
		second.History[i].Targets = []string{second.Id}
	}
	bed.addGoal(second)
	for _, id := range []string{bedGoal, second.Id} {
		claimAreaDesign(t, bed, id, "metasystem/shared/**")
	}
	before := map[string]*goal.GoalFile{bedGoal: bed.goalFile(bedGoal), second.Id: bed.goalFile(second.Id)}
	for _, id := range []string{bedGoal, second.Id} {
		reservePersonGoal(t, bed, owners, id)
	}
	for _, id := range []string{bedGoal, second.Id} {
		f := bed.goalFile(id)
		if f.Claimed.Lineage != launch.SeatOwnerLineage || f.StopCapability.ClaimEpoch != 0 || !reflect.DeepEqual(f.Approved, before[id].Approved) || !reflect.DeepEqual(f.Budget, before[id].Budget) {
			t.Fatalf("reservation manufactured session authority or changed approval: %+v", f)
		}
		if _, err := owners.binding(bed.root(), id, time.Date(2026, 9, 1, 10, 0, 0, 0, time.UTC)); err == nil || !strings.Contains(err.Error(), "metasystem session start") {
			t.Fatalf("epoch zero entered shared work binding: %v", err)
		}
	}
	if warnings := strings.Join(bed.goalFile(second.Id).Claimed.Warnings, ";"); !strings.Contains(warnings, "overlaps goal "+bedGoal) || !strings.Contains(warnings, "already claims") {
		t.Fatalf("person's area/quota advice was not persisted: %q", warnings)
	}
	reserved := map[string]*goal.GoalFile{bedGoal: bed.goalFile(bedGoal), second.Id: bed.goalFile(second.Id)}
	owners = adoptingHolder(owners, launch.SeatOwnerLineage, 8)
	result := adoptPersonGoals(t, bed, owners, launch.SeatOwnerLineage, 8)
	if result.Pending || len(result.Goals) != 2 {
		t.Fatalf("not every reservation was adopted: %+v", result)
	}
	for _, id := range []string{bedGoal, second.Id} {
		f := bed.goalFile(id)
		if f.StopCapability.ClaimEpoch != 8 || !reflect.DeepEqual(f.Claimed, reserved[id].Claimed) || !reflect.DeepEqual(f.Budget, reserved[id].Budget) || !reflect.DeepEqual(f.Approved, reserved[id].Approved) {
			t.Fatalf("adoption changed ownership/accounting or missed %s: %+v", id, f)
		}
		if _, err := owners.binding(bed.root(), id, time.Date(2026, 9, 1, 10, 0, 0, 0, time.UTC)); err != nil {
			t.Fatalf("session remedy did not clear binding refusal: %v", err)
		}
	}
	publications := bed.publications()
	result = adoptPersonGoals(t, bed, owners, launch.SeatOwnerLineage, 8)
	if result.Pending || len(result.Goals) != 2 || result.Goals[0].Outcome != "current" || result.Goals[1].Outcome != "current" || bed.publications() != publications {
		t.Fatalf("repeat adoption wrote another episode: %+v", result)
	}
}

func TestPersonClaimLineageUsesLiveOrStableHolder(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, lineage string
		live          bool
		want          string
		epoch         int64
	}{
		{"live fallback", "main-live", true, "main-live", 9},
		{"dead stable", "stable-owner", false, "stable-owner", 0},
		{"dead process", "main-live", false, launch.SeatOwnerLineage, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			bed, owners := personClaimsBed(t)
			owners = adoptingHolder(owners, tc.lineage, 9)
			if !tc.live {
				owners.dependencies.claimHolder.Caller = func(string, int64) (lease.ClassifyResult, error) {
					return lease.ClassifyResult{Class: lease.ClassUntrusted}, nil
				}
			}
			reservePersonGoal(t, bed, owners, bedGoal)
			f := bed.goalFile(bedGoal)
			if f.Claimed.Lineage != tc.want || f.StopCapability.ClaimEpoch != tc.epoch {
				t.Fatalf("holder lineage/epoch = %+v %+v; want %s/%d", f.Claimed, f.StopCapability, tc.want, tc.epoch)
			}
		})
	}
}

func TestPersonClaimAllAdoptionKeepsPendingAndRemedies(t *testing.T) {
	t.Parallel()
	bed, owners := personClaimsBed(t)
	for _, id := range []string{"a-current", "b-broken", "c-mismatch", "d-adopt", "e-later"} {
		f := bed.goalFile(bedGoal)
		f.Id = id
		f.History = append([]goal.HistoryLine(nil), f.History...)
		bed.addGoal(f)
		reservePersonGoal(t, bed, owners, id)
	}
	foreign := bed.goalFile(bedGoal)
	foreign.Id = "foreign-machine"
	bed.addGoal(foreign)
	peer := owners
	peer.dependencies.machine = func(string) (string, error) { return "mac-peer", nil }
	reservePersonGoal(t, bed, peer, foreign.Id)
	// The projection includes an already current and a damaged local claim
	// before later reservations; neither can hide them.
	f := bed.goalFile("a-current")
	f.StopCapability.ClaimEpoch = 6
	bed.addGoal(f)
	f = bed.goalFile("b-broken")
	f.StopCapability = nil
	bed.addGoal(f)
	// A different explicitly stable lineage remains reserved for that session.
	mismatchOwners := adoptingHolder(owners, "other-lineage", 6)
	f = bed.goalFile("c-mismatch")
	f.State, f.Claimed, f.StopCapability = goal.StateApproved, nil, nil
	bed.addGoal(f)
	reservePersonGoal(t, bed, mismatchOwners, "c-mismatch")
	owners = adoptingHolder(owners, launch.SeatOwnerLineage, 6)
	result := adoptPersonGoals(t, bed, owners, launch.SeatOwnerLineage, 6)
	byID := map[string]up.GoalAdoptionOutcome{}
	for _, g := range result.Goals {
		byID[g.GoalID] = g
	}
	if !result.Pending || byID["a-current"].Outcome != "current" || byID["b-broken"].Outcome != "pending" || byID["d-adopt"].Outcome != "restamped" || byID["e-later"].Outcome != "restamped" || !strings.Contains(byID["c-mismatch"].Remedy, "fresh matching session") {
		t.Fatalf("adoption hid a later match or lost a remedy: %+v", result)
	}
	processes := defaultProcessIntentOwners()
	processes.process.repositoryTop = fakeTop(bed.root())
	processes.adoptionReads = &claimAdoptionReads{dependencies: owners.dependencies, clock: owners.commandNow, project: goal.Project}
	processes.up = func(options up.Options) up.Result {
		adoption, err := options.RestampStopCapability(bed.root(), launch.SeatOwnerLineage, 6)
		if err != nil {
			t.Fatal(err)
		}
		return up.Result{Outcome: "armed", Adoption: &adoption, Components: []up.ComponentOutcome{{Component: "stop-capability", Outcome: "pending", Adoption: &adoption}}}
	}
	owners.processes = processes
	code, started := bed.runJSON(owners, "session", "start", "--lineage", launch.SeatOwnerLineage)
	encoded, _ := json.Marshal(started.Data)
	if code != 1 || started.Outcome != intentPartial || !strings.Contains(string(encoded), `"pending":true`) || !strings.Contains(string(encoded), "e-later") {
		t.Fatalf("armed session hid partial adoption: code=%d %+v data=%s", code, started, encoded)
	}
	if bed.goalFile(foreign.Id).StopCapability.ClaimEpoch != 0 {
		t.Fatal("session adoption moved another machine's reservation")
	}
	// A hand-started session can fill its absent lineage, then the exact remedy
	// adopts the reservation without releasing it.
	owners = adoptingHolder(owners, "main-live", 6)
	result = adoptPersonGoals(t, bed, owners, "main-live", 6)
	for _, g := range result.Goals {
		if g.GoalID == "c-mismatch" && g.Remedy != "metasystem session start --lineage other-lineage" {
			t.Fatalf("hand-started lineage remedy cannot be followed: %+v", g)
		}
	}
	owners = adoptingHolder(owners, "other-lineage", 7)
	result = adoptPersonGoals(t, bed, owners, "other-lineage", 7)
	if bed.goalFile("c-mismatch").StopCapability.ClaimEpoch != 7 {
		t.Fatalf("matching session did not adopt: %+v", result)
	}
}

func TestPersonClaimAdoptionAuthorityLossRetainsConfirmed(t *testing.T) {
	t.Parallel()
	bed, owners := personClaimsBed(t)
	for _, id := range []string{"a-one", "b-two", "c-three"} {
		f := bed.goalFile(bedGoal)
		f.Id = id
		bed.addGoal(f)
		reservePersonGoal(t, bed, owners, id)
	}
	owners = adoptingHolder(owners, launch.SeatOwnerLineage, 6)
	reader := owners.dependencies.claimHolder.Caller
	owners.dependencies.claimHolder.Caller = func(root string, pid int64) (lease.ClassifyResult, error) {
		if bed.goalFile("a-one").StopCapability.ClaimEpoch == 6 {
			return lease.ClassifyResult{}, errors.New("holder observation lost")
		}
		return reader(root, pid)
	}
	result := adoptPersonGoals(t, bed, owners, launch.SeatOwnerLineage, 6)
	if !result.Pending || result.Goals[0].GoalID != "a-one" || result.Goals[0].Outcome != "restamped" {
		t.Fatalf("authority loss erased confirmed adoption: %+v", result)
	}
	for _, g := range result.Goals[1:] {
		if g.Outcome != "pending" {
			t.Fatalf("authority loss allowed later effects: %+v", g)
		}
	}
	owners = adoptingHolder(owners, launch.SeatOwnerLineage, 6)
	retry := adoptPersonGoals(t, bed, owners, launch.SeatOwnerLineage, 6)
	if retry.Pending || retry.Goals[0].Outcome != "current" {
		t.Fatalf("retry lost confirmed ownership: %+v", retry)
	}
}

func TestPersonClaimUnnamedDoesNotSelectAndAgentCannotUseBy(t *testing.T) {
	t.Parallel()
	bed, owners := personClaimsBed(t)
	before := bed.publications()
	code, result := bed.runJSON(owners, "goal", "claim", "--by", "Wido", "--fixture-human-authority")
	if code == 0 || !strings.Contains(result.Summary, "name") || bed.publications() != before {
		t.Fatalf("unnamed person claim guessed a choice: %d %+v", code, result)
	}
	reservePersonGoal(t, bed, owners, bedGoal)
	file := bed.goalFile(bedGoal)
	encoded, _ := json.Marshal(file)
	if !strings.Contains(string(encoded), "human:Wido") {
		t.Fatal("claim by missing from structured state")
	}
	// A stored person name cannot waive an agent's ordinary quota.
	other := bed.goalFile(bedGoal)
	other.Id = "agent-next"
	other.State, other.Claimed, other.StopCapability = goal.StateApproved, nil, nil
	bed.addGoal(other)
	bed.lineage = "m1"
	announceProofFixtureHolder(t, bed.root())
	code, result = bed.runJSON(owners, "goal", "claim", "agent-next")
	if code == 0 || bed.goalFile("agent-next").Claimed != nil || !strings.Contains(result.Summary, "already claims") {
		t.Fatalf("stored By granted the next agent personal authority: %d %+v", code, result)
	}
}

func TestPersonClaimPublicArcAndTakeover(t *testing.T) {
	t.Parallel()
	bed, owners := personClaimsBed(t)
	for _, id := range []string{bedGoal, "arc-next", "arc-paused", "outside-arc"} {
		file := bed.goalFile(bedGoal)
		file.Id = id
		makeQueued(file)
		file.Arc = ""
		if id != "outside-arc" {
			file.Arc = "chosen"
		}
		if id == "arc-paused" {
			file.State = goal.StateParked
			file.Parked = &goal.ParkRecord{By: "human:Wido", At: file.History[0].At, Because: "Explicit pause."}
		}
		bed.addGoal(file)
	}
	reservePersonGoal(t, bed, owners, bedGoal, "--arc")
	for _, id := range []string{bedGoal, "arc-next"} {
		file := bed.goalFile(id)
		if !file.PersonalReservation() || file.StopCapability.ClaimEpoch != 0 || file.Approved != nil || !strings.Contains(strings.Join(file.Claimed.Warnings, ";"), "areas unknown") {
			t.Fatalf("public arc lost personal warning/binding: %+v", file)
		}
	}
	owners.dependencies.machine = func(string) (string, error) { return "mac-peer", nil }
	reservePersonGoal(t, bed, owners, bedGoal, "--take-over", "--reason", "Move the chosen pair.")
	for _, id := range []string{bedGoal, "arc-next"} {
		if file := bed.goalFile(id); file.Claimed.Machine != "mac-peer" || file.StopCapability.ClaimEpoch != 0 || !file.PersonalReservation() {
			t.Fatalf("public takeover lost scope/proof: %+v", file)
		}
	}
	if bed.goalFile("arc-paused").State != goal.StateParked || bed.goalFile("outside-arc").Claimed != nil {
		t.Fatal("public arc/takeover changed explicit selection")
	}
}

func TestPersonClaimHolderLostDuringReplayStopsPass(t *testing.T) {
	t.Parallel()
	bed, owners := personClaimsBed(t)
	for _, id := range []string{"a-one", "b-two"} {
		file := bed.goalFile(bedGoal)
		file.Id = id
		bed.addGoal(file)
		reservePersonGoal(t, bed, owners, id)
	}
	owners = adoptingHolder(owners, launch.SeatOwnerLineage, 6)
	classify := owners.dependencies.claimHolder.Caller
	reads := 0
	owners.dependencies.claimHolder.Caller = func(root string, pid int64) (lease.ClassifyResult, error) {
		reads++
		if reads == 3 {
			return lease.ClassifyResult{Class: lease.ClassUntrusted}, nil
		}
		return classify(root, pid)
	}
	endpoint := owners.dependencies.endpoint
	publishes := 0
	owners.dependencies.endpoint = func(root string) (goal.Endpoint, error) {
		e, err := endpoint(root)
		e.Repository = personClaimTransport{Repository: e.Repository, publish: func(tip, commit string) (goal.CASOutcome, error) {
			publishes++
			if publishes == 1 {
				bed.addGoal(bed.goalFile(bedGoal))
				return goal.CASRefused, errors.New("another writer advanced the ledger")
			}
			return bed.repo.Publish(tip, commit)
		}}
		return e, err
	}
	result := adoptPersonGoals(t, bed, owners, launch.SeatOwnerLineage, 6)
	if !result.Pending || len(result.Goals) != 2 || reads != 3 || publishes != 1 {
		t.Fatalf("replay failed to stop on shared authority loss: %+v reads=%d publishes=%d", result, reads, publishes)
	}
	for _, outcome := range result.Goals {
		if outcome.Outcome != "pending" || bed.goalFile(outcome.GoalID).StopCapability.ClaimEpoch != 0 {
			t.Fatalf("replay or a later claim used stale holder authority: %+v", outcome)
		}
	}
}

func TestPersonClaimReservationBudgetAndApprovalKeepEpoch(t *testing.T) {
	t.Parallel()
	bed, owners := personClaimsBed(t)
	f := bed.goalFile(bedGoal)
	makeQueued(f)
	bed.addGoal(f)
	reservePersonGoal(t, bed, owners, bedGoal)
	for _, epoch := range []int64{0, 7} {
		if epoch > 0 {
			owners = adoptingHolder(owners, launch.SeatOwnerLineage, epoch)
			adoptPersonGoals(t, bed, owners, launch.SeatOwnerLineage, epoch)
		}
		for _, verb := range []string{"approve", "budget"} {
			args := []string{"goal", verb, bedGoal, "--budget", "norm", "--by", "Wido", "--fixture-human-authority"}
			before := bed.goalFile(bedGoal)
			if verb == "budget" {
				args[4] = "30m/2/10m/1/2"
			}
			if code, result := bed.runJSON(owners, args...); code != 0 {
				t.Fatalf("%s at epoch %d: %d %+v", verb, epoch, code, result)
			}
			f = bed.goalFile(bedGoal)
			if f.StopCapability.ClaimEpoch != epoch || f.Claimed.By != "human:Wido" || !f.PersonalReservation() {
				t.Fatalf("%s substituted the person's epoch or lost provenance: %+v", verb, f)
			}
			if verb == "budget" && (reflect.DeepEqual(f.Budget, before.Budget) || f.Claimed.EpisodeRevision != before.Claimed.EpisodeRevision || f.Claimed.Revision == before.Claimed.Revision) {
				t.Fatalf("budget did not own its accounting transition: before=%+v after=%+v", before, f)
			}
		}
	}
}

func TestPersonClaimPublicAdoptionPreservesStewardEligibility(t *testing.T) {
	t.Parallel()
	for _, state := range []string{"approved", "unapproved without budget", "unapproved with budget"} {
		t.Run(state, func(t *testing.T) {
			t.Parallel()
			bed, owners := personClaimsBed(t)
			approved := state == "approved"
			file := bed.goalFile(bedGoal)
			if !approved {
				file.Approved = nil
				file.State = goal.StateQueued
				if state == "unapproved without budget" {
					file.Budget = nil
				}
			}
			bed.addGoal(file)
			reservePersonGoal(t, bed, owners, bedGoal)
			before := bed.goalFile(bedGoal)
			checkSeat := func(ended bool) {
				endpoint, err := owners.dependencies.endpoint(bed.root())
				if err != nil {
					t.Fatal(err)
				}
				projection, err := goal.Project(endpoint, false, syncRequestTestNow)
				if err != nil {
					t.Fatal(err)
				}
				work, err := goal.ClaimableWorkFromProjection(projection, "mac-cli", seatTickProber{})
				if err != nil {
					t.Fatal(err)
				}
				world := steward.SeatWorldFrom(work, projection.Tree.Live, goal.GateSettings{}, nil, syncRequestTestNow)
				if len(world.Held) != 1 || world.Held[0].StepDue != approved {
					t.Fatalf("session changed seat eligibility: %+v", world)
				}
				var records []steward.SeatRecord
				if ended {
					records = []steward.SeatRecord{{Goal: bedGoal, LaunchState: "completed", Outcome: "progress"}}
				}
				decision, selection := steward.PlanSeat(world, records, 3, true)
				if approved {
					if decision.Action != steward.ActRevive || selection == nil || selection.Goal != bedGoal {
						t.Fatalf("approved reservation cannot start its adopter: %+v %+v", decision, selection)
					}
				} else if decision.Action != steward.ActNotify || selection != nil || !strings.Contains(decision.Reason, "this seat holds "+bedGoal) || !strings.Contains(decision.Reason, "awaits a person's approval") {
					t.Fatalf("unapproved reservation regained launch authority after seat end: %+v %+v", decision, selection)
				}
			}
			checkSeat(false)
			owners = adoptingHolder(owners, launch.SeatOwnerLineage, 7)
			processes := defaultProcessIntentOwners()
			processes.process.repositoryTop = fakeTop(bed.root())
			processes.adoptionReads = &claimAdoptionReads{dependencies: owners.dependencies, clock: owners.commandNow, project: goal.Project}
			processes.up = func(options up.Options) up.Result {
				adoption, err := options.RestampStopCapability(bed.root(), launch.SeatOwnerLineage, 7)
				if err != nil {
					t.Fatal(err)
				}
				return up.Result{Outcome: "armed", Adoption: &adoption, Components: []up.ComponentOutcome{{Component: "stop-capability", Outcome: "restamped", Adoption: &adoption}}}
			}
			owners.processes = processes
			if code, result := bed.runJSON(owners, "session", "start", "--lineage", launch.SeatOwnerLineage); code != 0 || result.Outcome != intentConfirmed {
				t.Fatalf("adopter's session start: %d %+v", code, result)
			}
			after := bed.goalFile(bedGoal)
			if after.StopCapability.ClaimEpoch != 7 || !reflect.DeepEqual(after.Claimed, before.Claimed) || !reflect.DeepEqual(after.Budget, before.Budget) || !reflect.DeepEqual(after.Approved, before.Approved) {
				t.Fatalf("adoption reset ownership, accounting or approval: %+v", after)
			}
			checkSeat(true)
		})
	}
}
