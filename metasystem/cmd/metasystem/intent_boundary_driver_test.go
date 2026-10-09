package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal/branch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/helm"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/hostload"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/loopstop"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testprovider"
)

// boundaryAuthorityStarter checks the detached child's current authority before
// the fixture records a started build.
type boundaryAuthorityStarter struct {
	bed *fleetBoundaryBed
}

func (s boundaryAuthorityStarter) StartSupervisor(id, state string) (identity.Ref, error) {
	record, err := s.bed.manager.Store.Read(id)
	if err != nil {
		return identity.Ref{}, err
	}
	if record.Kind == "build" {
		if err := checkUnitChildAuthority(s.bed.manager, record, s.bed.owners); err != nil {
			return identity.Ref{}, err
		}
	}
	return s.bed.starter.StartSupervisor(id, state)
}

// The public commands and resident loop share the same boundary admission.
func TestDriverPublicBoundaryNextUnit(t *testing.T) {
	t.Parallel()
	if os.Getenv("BOUNDARY_DRIVER_ISOLATED") == "" {
		python, err := exec.LookPath("python3")
		if err != nil {
			t.Fatal(err)
		}
		bin := t.TempDir()
		if err := testexec.WriteFile(filepath.Join(bin, "git"), []byte("#!"+python+"\n"+driverGitScript), 0700); err != nil {
			t.Fatal(err)
		}
		selection := "^TestDriverPublicBoundaryNextUnit$"
		if parts := strings.SplitN(flag.Lookup("test.run").Value.String(), "/", 2); len(parts) == 2 {
			selection += "/" + parts[1]
		}
		cmd := exec.CommandContext(t.Context(), os.Args[0], "-test.run="+selection, "-test.timeout=30m", "-test.v")
		for _, entry := range os.Environ() {
			name, _, _ := strings.Cut(entry, "=")
			if name != "PATH" && name != "METASYSTEM_SUPERVISION_REGISTRY_HOME" && name != "METASYSTEM_TESTENV_REGISTRY_NONCE" {
				cmd.Env = append(cmd.Env, entry)
			}
		}
		cmd.Env = append(cmd.Env, "BOUNDARY_DRIVER_ISOLATED=1", "BOUNDARY_DRIVER_PARENT_REGISTRY="+os.Getenv("METASYSTEM_SUPERVISION_REGISTRY_HOME"), "PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"))
		if data, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("isolated public boundary: %v\n%s", err, data)
		}
		return
	}
	if parent := os.Getenv("BOUNDARY_DRIVER_PARENT_REGISTRY"); parent != "" && parent == os.Getenv("METASYSTEM_SUPERVISION_REGISTRY_HOME") {
		t.Fatal("the resident loop inherited another fixture's machine registry")
	}
	for _, mode := range []string{"headless", "unbound-handoff", "unknown-predecessor", "person", "person-agent", "person-agent-unknown-lineage", "hand-in-prepared", "transfer", "changed-tip", "changed-design", "changed-goal", "changed-boundary", "unreadable", "cancelled", "prepare-error", "closed-goal", "rearm-failed", "rearm-unknown", "replacement", "pending-tip", "pending-goal", "pending-foreign", "brief-equals", "brief-relative", "pending-ready", "pending-rearm", "pending-rearm-unknown", "pending-handoff", "last-admission", "child-admission", "hand-in-helm", "hand-in-no-end", "no-lane", "publication"} {
		t.Run(mode, func(t *testing.T) {
			t.Parallel()
			nextUnit := "next"
			if mode == "publication" {
				nextUnit = "u1"
			}
			var publisher *driverReviewFixture
			var bed *fleetBoundaryBed
			if mode == "publication" {
				publisher = newDriverReviewFixture(t, "clean")
				bed = newFleetBoundaryBed(t, "auto", publisher.bed)
				bed.owners = publisher.owners
				inspect := bed.owners.work.inspectRead
				bed.owners.work.inspectRead = func(root, id, commit string) (branch.BranchReadResult, error) {
					if commit == strings.Repeat("a", 40) {
						return branch.BranchReadResult{State: "collected", Published: true, AttestationCommit: strings.Repeat("b", 40)}, nil
					}
					return inspect(root, id, commit)
				}
			} else {
				bed = newFleetBoundaryBed(t, "auto")
			}
			// The fleet fixture's branch adapter reports publication. Keep the
			// same completed subject and close decision in its unit record so
			// the real tree owner can relinquish custody before the next build.
			bed.run.Subjects[0].Published = strings.Repeat("b", 40)
			bed.run.Rounds[0].Stop = &loopstop.Stop{Decision: "close"}
			bed.saveRun(t, bed.run)
			page, data := designGatePage(t, bed.workBed, "- Critique: closed at round 1 on 0 material findings (reader)")
			data = append(data, []byte("\n## Units\n\n| Unit | Lines |\n| --- | ---: |\n| finished | 5 |\n| next | 5 |\n\n## Acceptance\n\nBoth units work.\n")...)
			if publisher != nil {
				data = bytes.ReplaceAll(data, []byte("| next |"), []byte("| u1 |"))
			}
			if err := os.WriteFile(page, data, 0600); err != nil {
				t.Fatal(err)
			}
			policy := "auto"
			if strings.HasPrefix(mode, "person") {
				policy = "person"
				path := filepath.Join(bed.root(), "metasystem.conf")
				conf, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, bytes.ReplaceAll(conf, []byte("seat.driver=auto"), []byte("seat.driver=person")), 0600); err != nil {
					t.Fatal(err)
				}
			}
			bed.owners.work.config = func(key, _ string) (string, string, int, error) {
				if key == "seat.driver" {
					return policy, "fixture", 0, nil
				}
				return "auto", "fixture", 0, nil
			}
			state := intentBranchState{BranchTip: strings.Repeat("a", 40), Status: branch.Status{Units: []branch.UnitStatus{{Unit: "finished", ReadState: "read clean"}}}}
			if mode == "transfer" {
				state.Status.Units[0].ReadState = "read transferred"
				state.Status.ReviewObligations = []goal.ReviewObligation{{TargetUnit: "transfer-destination"}}
			}
			delivery := defaultIntentDeliveryOwners()
			delivery.branchState = func(string, string) (intentBranchState, error) {
				if publisher != nil && publisher.run != "" {
					record, err := (&launch.UnitRunner{Root: bed.unitRoot}).Status(publisher.run)
					if err != nil {
						return state, err
					}
					if len(record.Subjects) > 0 && record.Subjects[0].Published != "" {
						state.BranchTip = publisher.raw.read
						state.Status.Units = []branch.UnitStatus{{Unit: "finished", ReadState: "read clean"}, {Unit: nextUnit, ReadState: "read clean"}}
					}
				}
				return state, nil
			}
			if publisher != nil {
				delivery.branchRead = publisher.owners.delivery.branchRead
				delivery.publishRead = publisher.owners.delivery.publishRead
				delivery.closeOwner = publisher.owners.delivery.closeOwner
				delivery.recordWriter = publisher.owners.delivery.recordWriter
			}
			laneRoot := t.TempDir()
			if err := os.WriteFile(filepath.Join(laneRoot, "metasystem.conf"), []byte("metasystem.template=true\n"), 0600); err != nil {
				t.Fatal(err)
			}
			delivery.laneInstall = func(string) (string, error) { return laneRoot, nil }
			laneHome := testprovider.Register(t, laneRoot)
			bed.owners.landing.home = func() (string, error) { return laneHome, nil }
			bed.owners.landing.plainProve = plain.ProveSeams{Git: func(_ string, args ...string) (string, error) {
				if args[0] == "fetch" {
					return "", nil
				}
				return strings.Repeat("9", 40), nil
			}, Incidents: func(string, string, string) ([]goal.TrunkRedEntry, error) { return nil, nil }}
			bed.owners.policies = config.PolicyReaders{Registry: func(string) (config.PolicyRegistry, error) { return config.PolicyRegistry{Lane: laneRoot}, nil }, ConfPath: func(root string) (string, error) { return filepath.Join(root, "metasystem.conf"), nil }, Helm: helm.Active}
			delivery.laneRoot = func(string, time.Time) (string, bool, error) { return laneRoot, true, nil }
			delivery.laneContains = func(string, string) (bool, error) { return false, nil }
			delivery.landingGate = func(*intentInvocation, string, string) (string, error) { return state.BranchTip, nil }
			delivery.now = func() time.Time { return bed.now }
			handIns := 0
			delivery.laneRegistrant = func(string) string { handIns++; return "fixture-seat" }
			bed.owners.delivery = delivery
			var release *os.File
			var child *exec.Cmd
			var caller steward.HandoffCaller
			var observed *os.File
			headless := mode == "headless" || mode == "unbound-handoff" || mode == "unknown-predecessor"
			if headless {
				child, release, observed = startFleetHandoffChild(t, bed.root())
				defer release.Close()
				exact, live, err := (identity.KernelProber{}).Probe(int64(child.Process.Pid))
				if err != nil || live != identity.Alive {
					t.Fatalf("predecessor: %s %v", live, err)
				}
				if err := os.Remove(filepath.Join(bed.root(), "artifacts", "agents", "mains", "worktree-lease.json")); err != nil {
					t.Fatal(err)
				}
				ref := exact.Ref()
				announcement, err := lease.AnnounceWithPair(bed.root(), bed.session, ref.Pid, ref.StartedAtSec, ref.StartTicks, ref.BootID, "-test.run=^TestFleetBoundaryPublicHandoff$", "fake", steward.SeatLineage)
				if err != nil {
					t.Fatal(err)
				}
				body, _ := os.ReadFile(announcement)
				var main lease.Announcement
				if err := json.Unmarshal(body, &main); err != nil {
					t.Fatal(err)
				}
				caller = steward.HandoffCaller{Class: lease.ClassMain, MainId: main.MainId, HolderMainId: main.MainId, Runtime: "fake", Session: bed.session, Machine: "m1", Ref: ref, Tag: "-test.run=^TestFleetBoundaryPublicHandoff$"}
				if mode == "unknown-predecessor" {
					caller.Tag = "unobservable-predecessor"
				}
			}
			if code, problem := bed.observe(t); code != 0 {
				t.Fatalf("boundary: %d %s", code, problem)
			}
			if len(bed.boundaries(t)) != 1 {
				_, status := bed.runJSON(bed.owners, "work", "status", bed.id)
				t.Fatalf("initial boundary absent: outcome=%s steps=%+v subjects=%+v status=%s", bed.run.Rounds[0].Outcome, bed.run.Rounds[0].Steps[len(bed.run.Rounds[0].Steps)-1].FinishedAt, bed.run.Subjects, resultWords(status))
			}
			if strings.HasPrefix(mode, "person-agent") {
				events := bed.boundaries(t)
				events[0].Lineage = steward.SeatLineage
				if mode == "person-agent-unknown-lineage" {
					events[0].Lineage = ""
				}
				events[0].Handoff = ""
				body, _ := json.Marshal(events)
				if err := os.WriteFile(filepath.Join(bed.root(), "artifacts", "agents", "steward", "unit-boundaries.json"), body, 0600); err != nil {
					t.Fatal(err)
				}
			}
			engine := "ready"
			builds := func() int {
				count := 0
				for _, kind := range bed.starter.launched() {
					if kind == "build" {
						count++
					}
				}
				return count
			}
			driveBuilds := 0
			cycle := func() int {
				before := len(bed.starter.launched())
				var output bytes.Buffer
				code := runStewardRunWithDependencies([]string{"--repo", bed.root()}, &output, &output,
					func(root string, census steward.WorkerCensus, revive func() error, interval time.Duration, cfg steward.TickConfig) error {
						if cfg.AdvanceBoundary == nil {
							t.Fatal("production boundary adapter missing")
						}
						cfg.WorkStateRoot = filepath.Dir(bed.unitRoot)
						cfg.ProviderHome = testprovider.Home(root)
						cfg.Now = bed.now
						drive := cfg.DriveWork
						cfg.DriveWork = func(root string) error {
							before := builds()
							err := drive(root)
							driveBuilds += builds() - before
							return err
						}
						cfg.Seat, cfg.KeepLandingLane, cfg.ProbeProvider = nil, nil, nil
						if engine == "unknown" {
							cfg.RearmAtBoundary = nil
						}
						clockNow := bed.now
						cfg.RunnerClock = &steward.HandoffClock{Now: func() time.Time { return clockNow }, Sleep: func(d time.Duration) {
							clockNow = clockNow.Add(d)
							if err := os.WriteFile(filepath.Join(root, "artifacts", "agents", "steward", "stop"), []byte("stop"), 0600); err != nil {
								t.Fatal(err)
							}
						}}
						return steward.RunLoop(root, census, func() error { return nil }, interval, cfg)
					}, nil, nil, func(string) int { return 1 }, func() (bool, error) {
						if engine == "failed" {
							return false, errors.New("engine unavailable")
						}
						return engine == "replacement", nil
					}, bed.owners)
				if code != 0 {
					t.Log(output.String())
				}
				if len(bed.starter.launched()) != before && !strings.HasPrefix(mode, "pending-") {
					t.Fatal("preparation started a child")
				}
				return code
			}
			refusesBuild := func(brief string) {
				t.Helper()
				owners := bed.owners
				owners.prove = nil
				before := len(bed.starter.launched())
				code, result := bed.runJSON(owners, "work", "build", bed.id, "--work", nextUnit, "--brief", brief)
				if code == 0 || len(bed.starter.launched()) != before {
					t.Fatalf("boundary admitted work prematurely: %d %s", code, result.Summary)
				}
			}

			if mode == "prepare-error" || mode == "closed-goal" {
				healthy := bed.boundaries(t)[0]
				broken := healthy
				broken.Goal = "old-goal"
				file := bed.goalFile(bed.id)
				file.Id = broken.Goal
				bed.addGoal(file)
				if mode == "closed-goal" {
					broken.Goal = "gone-goal"
				}
				delivery.branchState = func(_, id string) (intentBranchState, error) {
					if id == "old-goal" {
						return intentBranchState{}, errors.New("branch state unavailable")
					}
					return state, nil
				}
				body, _ := json.Marshal([]steward.UnitBoundary{healthy, broken})
				if err := os.WriteFile(filepath.Join(bed.root(), "artifacts", "agents", "steward", "unit-boundaries.json"), body, 0600); err != nil {
					t.Fatal(err)
				}
				if cycle() != 0 {
					t.Fatal("one goal stopped the loop")
				}
				events := bed.boundaries(t)
				if events[0].Next == nil || events[0].Next.Unit != nextUnit {
					t.Fatalf("healthy goal skipped: %+v", events)
				}
				if mode == "closed-goal" && events[1].Next != nil {
					t.Fatalf("closed goal was not quietly skipped: %+v", events)
				}
				if mode == "prepare-error" && (events[1].Next == nil || !strings.Contains(events[1].Next.Summary, "branch state unavailable")) {
					t.Fatalf("missing goal diagnostic: %+v", events)
				}
				return
			}
			if strings.HasPrefix(mode, "rearm-") || mode == "replacement" {
				engine = strings.TrimPrefix(mode, "rearm-")
				if cycle() != 0 || bed.boundaries(t)[0].Next != nil {
					t.Fatal("unready engine prepared work")
				}
				refusesBuild(bed.brief("unsafe.md", "Build next.\n"))
				engine = "ready"
			}

			if headless {
				if cycle() != 0 || bed.boundaries(t)[0].Next != nil {
					t.Fatal("unbound handoff prepared next work")
				}
				refusesBuild(bed.brief("unbound.md", "Build next.\n"))
				noteDir := filepath.Join(bed.root(), ".handoff-notes")
				if err := os.MkdirAll(noteDir, 0700); err != nil {
					t.Fatal(err)
				}
				note := filepath.Join(noteDir, "lessons.md")
				writeContextHandoffNote(t, note, "Continue the next required unit.\n", bed.now.Add(-time.Minute))
				conf, err := os.OpenFile(filepath.Join(bed.root(), "metasystem.conf"), os.O_APPEND|os.O_WRONLY, 0600)
				if err != nil {
					t.Fatal(err)
				}
				_, err = fmt.Fprintf(conf, "context.handoff.note-directory.fake=%s\nrole.steward-continuation.runtime=fake\nrole.steward-continuation.model.fake=fixture\n", noteDir)
				conf.Close()
				if err != nil {
					t.Fatal(err)
				}
				if err := steward.MintIdentity(steward.RepoIdentityPath(bed.root()), steward.InstallIdentity{RepoIdentity: bed.root(), Generation: 1, InstallPath: "/bin/true", MintedAt: bed.now.Format(time.RFC3339)}); err != nil {
					t.Fatal(err)
				}
				if err := os.MkdirAll(filepath.Join(bed.root(), "memory"), 0700); err != nil {
					t.Fatal(err)
				}
				reader := func(string, time.Time) (goal.ClaimableBudgetedWork, error) { return fleetHandoffWork(t, bed), nil }
				work, _ := reader("", bed.now)
				file, _ := work.OwnedClaim(bed.id)
				caller.Machine = file.Claimed.Machine
				code, output, problem := fleetHandoffCommand(t, bed, caller, reader, note)
				if code != 0 {
					t.Fatalf("public handoff: %d %s %s", code, output, problem)
				}
				if code, problem := bed.observe(t); (mode != "unknown-predecessor" && code != 0) || (mode == "unknown-predecessor" && (code == 0 || !strings.Contains(problem, "identity is unknown"))) {
					t.Fatalf("bound capture: %d %s", code, problem)
				}
				if cycle() != 0 || bed.boundaries(t)[0].Next != nil {
					t.Fatal("live predecessor prepared next work")
				}
				refusesBuild(bed.brief("live.md", "Build next.\n"))
				if mode != "unknown-predecessor" {
					var event steward.UnitBoundary
					if err := json.NewDecoder(observed).Decode(&event); err != nil || event.Handoff == "" || event.Session != caller.Session || event.Goal != bed.id {
						t.Fatalf("durable handoff signal: %+v %v", event, err)
					}
				}
				release.Close()
				if err := child.Wait(); err != nil {
					t.Fatal(err)
				}
			}
			if mode == "unbound-handoff" {
				other := bed.goalFile(bed.id)
				other.Id = "other-goal"
				bed.addGoal(other)
				bed.owners.delivery.branchState = func(_, id string) (intentBranchState, error) {
					if id == other.Id {
						complete := state
						complete.Status.Units = []branch.UnitStatus{{Unit: "whole", Whole: true, ReadState: "read clean"}}
						return complete, nil
					}
					return state, nil
				}
				broken := bed.boundaries(t)[0]
				broken.Session = "different-session"
				broken.Next = &steward.BoundaryAct{Summary: "stale preparation", Command: []string{"metasystem", "work", "land", bed.id}}
				healthy := broken
				healthy.Goal, healthy.Session, healthy.Lineage, healthy.Handoff, healthy.Next = other.Id, bed.session, "coordinator", "", nil
				data, err := json.Marshal([]steward.UnitBoundary{healthy, broken})
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(bed.root(), "artifacts", "agents", "steward", "unit-boundaries.json"), data, 0600); err != nil {
					t.Fatal(err)
				}
				if cycle() != 0 {
					t.Fatal("one unbound handoff blocked the whole preparation pass")
				}
				events := bed.boundaries(t)
				if events[0].Next == nil || len(events[0].Next.Command) == 0 {
					t.Fatalf("other goal was not prepared: %+v", events)
				}
				if events[1].Next == nil || len(events[1].Next.Command) != 0 || !strings.Contains(events[1].Next.Summary, "handoff does not bind") {
					t.Fatalf("unbound handoff kept stale preparation or lost its diagnostic: %+v", events)
				}
				if code, status := bed.runJSON(bed.owners, "work", "status", bed.id); code != 0 || !strings.Contains(fmt.Sprint(status.Data), "handoff does not bind") {
					t.Fatalf("public status hid the goal's handoff failure: %d %+v", code, status)
				}
				return
			}
			if cycle() != 0 {
				t.Fatal("next preparation failed")
			}
			act := bed.boundaries(t)[0].Next
			if act == nil || act.Unit != nextUnit || act.Tip != state.BranchTip || strings.Contains(shellCommand(act.Command), "FILE") || strings.Contains(shellCommand(act.Command), "COMMAND") {
				t.Fatalf("prepared act: %+v", act)
			}
			brief := flagValue(act.Command, "--brief")
			if body, err := os.ReadFile(brief); err != nil || !bytes.Contains(body, []byte("Both units work")) {
				t.Fatalf("public brief scaffold: %s %v", body, err)
			}
			if cycle() != 0 || bed.boundaries(t)[0].Next.Command[len(act.Command)-1] != brief {
				t.Fatal("replay replaced preparation")
			}
			if strings.HasPrefix(mode, "changed-") {
				switch mode {
				case "changed-boundary":
					events := bed.boundaries(t)
					events[0].Session = "new-boundary"
					body, _ := json.Marshal(events)
					if err := os.WriteFile(filepath.Join(bed.root(), "artifacts", "agents", "steward", "unit-boundaries.json"), body, 0600); err != nil {
						t.Fatal(err)
					}
				case "changed-tip":
					state.BranchTip = strings.Repeat("c", 40)
				case "changed-design":
					if err := os.WriteFile(page, append(data, []byte("\nAdditional accepted criteria.\n")...), 0600); err != nil {
						t.Fatal(err)
					}
				case "changed-goal":
					file := bed.goalFile(bed.id)
					file.NextStep = "Prepare the revised scope."
					bed.addGoal(file)
				}
				if cycle() != 0 || flagValue(bed.boundaries(t)[0].Next.Command, "--brief") == brief {
					t.Fatal("changed subject reused an old brief")
				}
				bed.head = state.BranchTip
				refusesBuild(brief)
				return
			}
			if mode == "unreadable" {
				if err := os.WriteFile(page, []byte("broken"), 0600); err != nil {
					t.Fatal(err)
				}
				if cycle() == 0 && len(bed.boundaries(t)[0].Next.Command) != 0 {
					t.Fatal("unreadable design offered another effect")
				}
				return
			}
			body := []byte("Build only next.\n")
			if publisher != nil {
				body = []byte("Read each round: yes\nBuild only next.\n")
				act.Command = append(act.Command, "--read-tool-calls", "12")
			}
			if err := os.WriteFile(brief, body, 0600); err != nil {
				t.Fatal(err)
			}
			if mode != "person" {
				bed.owners.prove = nil
			}

			var secondChild *exec.Cmd
			var secondRelease, secondObserved *os.File
			var secondCaller steward.HandoffCaller
			if headless {
				first := bed.boundaries(t)[0]
				if _, err := steward.ConsumeIntent(bed.root(), first.Handoff); err != nil {
					t.Fatal(err)
				}
				if err := steward.StampLaunch(bed.root(), first.Handoff); err != nil {
					t.Fatal(err)
				}
				secondChild, secondRelease, secondObserved = startFleetHandoffChild(t, bed.root())
				defer secondRelease.Close()
				exact, live, err := (identity.KernelProber{}).Probe(int64(secondChild.Process.Pid))
				if err != nil || live != identity.Alive {
					t.Fatalf("second predecessor: %s %v", live, err)
				}
				bed.session = "fleet-second"
				bed.now = exact.StartedAt.Add(time.Second)
				bed.manager.Now = func() time.Time { return bed.now }
				last := len(bed.run.Rounds[0].Steps) - 1
				bed.run.Rounds[0].Steps[last].FinishedAt = exact.StartedAt.Add(-time.Second).Format(time.RFC3339Nano)
				bed.saveRun(t, bed.run)
				announcement, err := lease.AnnounceWithPair(bed.root(), bed.session, exact.Pid, exact.StartedAt.Unix(), exact.StartTicks, exact.BootID, "-test.run=^TestFleetBoundaryPublicHandoff$", "fake", steward.SeatLineage)
				if err != nil {
					t.Fatal(err)
				}
				var main lease.Announcement
				if err := json.Unmarshal(mustRead(t, announcement), &main); err != nil {
					t.Fatal(err)
				}
				work := fleetHandoffWork(t, bed)
				file, _ := work.OwnedClaim(bed.id)
				secondCaller = steward.HandoffCaller{Class: lease.ClassMain, MainId: main.MainId, HolderMainId: main.MainId, Runtime: "fake", Session: bed.session, Machine: file.Claimed.Machine, Ref: exact.Ref(), Tag: "-test.run=^TestFleetBoundaryPublicHandoff$"}
			}
			waiting := strings.HasPrefix(mode, "pending-")
			if waiting {
				bed.manager.Supervisor = boundaryAuthorityStarter{bed: bed}
			}
			bed.manager.CapacitySources.Load = func(at time.Time) hostload.Sample {
				load := 0.0
				if waiting {
					load = 1000000
				}
				return hostload.Sample{At: at.Format(time.RFC3339Nano), Available: true, Load1m: load}
			}
			if mode == "last-admission" {
				claim := bed.owners.connection.claimCheck
				calls := 0
				bed.owners.connection.claimCheck = func(root, id string, e goal.Endpoint) func() error {
					check := claim(root, id, e)
					return func() error {
						calls++
						if calls == 2 {
							state.BranchTip = strings.Repeat("d", 40)
						}
						return check()
					}
				}
			}

			bed.head = state.BranchTip
			if mode == "brief-equals" || mode == "brief-relative" {
				for i, arg := range act.Command {
					if arg == "--brief" {
						if mode == "brief-relative" {
							relative, err := filepath.Rel(bed.root(), brief)
							if err != nil {
								t.Fatal(err)
							}
							act.Command[i+1] = relative
						} else {
							act.Command[i] += "=" + act.Command[i+1]
							act.Command = append(act.Command[:i+1], act.Command[i+2:]...)
						}
						break
					}
				}
			}
			if mode == "child-admission" {
				bed.manager.Supervisor = &recoveryStarter{bed: bed.workBed, crash: true}
				func() {
					defer func() {
						if recover() != "reservation crash" {
							t.Fatal("no reservation crash")
						}
					}()
					bed.runJSON(bed.owners, act.Command[1:]...)
				}()
				runs, _, err := (&launch.UnitRunner{Root: bed.unitRoot}).GoalRuns(bed.id)
				if err != nil {
					t.Fatal(err)
				}
				operation := ""
				for _, one := range runs {
					if one.Unit == nextUnit {
						operation = one.Record.Rounds[0].Steps[0].LaunchID
					}
				}
				if operation == "" {
					t.Fatal("the public build did not retain its reservation")
				}
				ended := make(chan struct{})
				close(ended)
				processes := &recoveryProcesses{s: &recoveryStarter{ended: ended}}
				bed.manager.Processes = processes
				state.BranchTip = strings.Repeat("d", 40)
				var output bytes.Buffer
				code := runLaunchSuperviseIn([]string{"--id", operation}, &output, &output, func() *launch.Manager { return bed.manager }, bed.owners)
				execution, err := bed.manager.Store.Read(operation)
				children, _ := processes.counts()
				if code != 1 || err != nil || children != 0 || execution.Child != nil || !strings.HasPrefix(execution.Reason, "authority-held: ") {
					t.Fatalf("final child admission: code=%d children=%d reason=%s err=%v %s", code, children, execution.Reason, err, output.String())
				}
				return
			}
			beforeBuild := len(bed.starter.launched())
			code, built := bed.runJSON(bed.owners, act.Command[1:]...)
			if mode == "last-admission" {
				if code == 0 || len(bed.starter.launched()) != beforeBuild {
					t.Fatalf("last admission ignored tip change: %d %+v launches=%d", code, built.Summary, len(bed.starter.launched()))
				}
				return
			}
			if strings.HasPrefix(mode, "person-agent") {
				if code != 3 {
					t.Fatalf("agent must admit the build and await the next explicit step: %d %s", code, built.Summary)
				}
			} else if code != 0 && !waiting {
				t.Fatalf("follow prepared command: %d %+v", code, built.Summary)
			}
			run := resultData(t, built)["run"].(string)
			if publisher != nil {
				publisher.run = run
			}
			record, err := (&launch.UnitRunner{Root: bed.unitRoot}).Status(run)
			if err != nil || record.Base != state.BranchTip {
				t.Fatalf("current-tip build: %+v %v", record.State, err)
			}
			if strings.HasPrefix(mode, "person-agent") && (record.Operation.Actor != "agent" || len(bed.starter.launched()) != beforeBuild+1) {
				t.Fatalf("agent build under person policy: actor=%q launches=%v", record.Operation.Actor, bed.starter.launched())
			}
			if strings.HasPrefix(mode, "person-agent") {
				bed.owners.prove = bed.personProof
				if code, result := bed.runJSON(bed.owners, "work", "build", "run:"+run); code != 0 {
					t.Fatalf("person continues the agent's admitted build: %d %s", code, result.Summary)
				}
				bed.owners.prove = nil
				record, err = (&launch.UnitRunner{Root: bed.unitRoot}).Status(run)
				if err != nil || record.State != "awaiting-judgement" {
					t.Fatalf("continued agent build: state=%s err=%v", record.State, err)
				}
			}
			if waiting {
				before := len(bed.starter.launched())
				beforeBuilds := len(slices.Collect(func(yield func(string) bool) {
					for _, kind := range bed.starter.launched() {
						if kind == "build" && !yield(kind) {
							return
						}
					}
				}))
				waiting = mode == "pending-rearm" || mode == "pending-rearm-unknown" || mode == "pending-foreign"
				if mode == "pending-ready" {
					if cycle() != 0 || driveBuilds != 1 || builds() != beforeBuilds+1 {
						t.Fatalf("ready pending build did not start during DriveWork: drive=%d launches=%v", driveBuilds, bed.starter.launched())
					}
					return
				}
				switch mode {
				case "pending-tip":
					state.BranchTip = strings.Repeat("c", 40)
					bed.head = state.BranchTip
				case "pending-goal":
					file := bed.goalFile(bed.id)
					file.NextStep = "Continue the changed accepted scope."
					bed.addGoal(file)
				case "pending-foreign":
					engine = "failed"
					events := bed.boundaries(t)
					foreign := events[0]
					foreign.Seat, foreign.Next = filepath.Join(bed.root(), "elsewhere"), &steward.BoundaryAct{Summary: "foreign newer snapshot"}
					body, _ := json.Marshal(append(events, foreign))
					if err := os.WriteFile(filepath.Join(bed.root(), "artifacts", "agents", "steward", "unit-boundaries.json"), body, 0600); err != nil {
						t.Fatal(err)
					}
				case "pending-rearm":
					engine = "failed"
				case "pending-rearm-unknown":
					engine = "unknown"
				case "pending-handoff":
					events := bed.boundaries(t)
					events[0].Lineage = steward.SeatLineage
					events[0].Handoff = ""
					body, _ := json.Marshal(events)
					if err := os.WriteFile(filepath.Join(bed.root(), "artifacts", "agents", "steward", "unit-boundaries.json"), body, 0600); err != nil {
						t.Fatal(err)
					}
				}
				if cycle() != 0 || len(bed.starter.launched()) != before {
					t.Fatal("pending run crossed changed boundary")
				}
				if mode == "pending-tip" || mode == "pending-goal" {
					code, result := bed.runJSON(bed.owners, "work", "build", "run:"+run)
					if code == 0 || (mode == "pending-goal" && !strings.Contains(resultWords(result), "a person must choose its next work")) {
						t.Fatalf("changed pending work lost its actual remedy: %d %s", code, result.Summary)
					}
					act := bed.boundaries(t)[0].Next
					if len(act.Command) != 0 || !strings.Contains(act.Summary, "a person must choose its next work") {
						t.Fatalf("changed queued preparation offered an unusable command: %+v", act)
					}
					return
				}
				waiting = false
				if mode == "pending-rearm" || mode == "pending-rearm-unknown" || mode == "pending-foreign" {
					if bed.boundaries(t)[0].Engine != "" {
						t.Fatal("failed or unknown re-arm retained an admission stamp")
					}
					if cycle() != 0 || len(bed.starter.launched()) != before {
						t.Fatal("pending run started after failed or unknown re-arm")
					}
				}
				engine = "ready"
				if mode == "pending-handoff" {
					events := bed.boundaries(t)
					events[0].Lineage = "coordinator"
					body, _ := json.Marshal(events)
					if err := os.WriteFile(filepath.Join(bed.root(), "artifacts", "agents", "steward", "unit-boundaries.json"), body, 0600); err != nil {
						t.Fatal(err)
					}
				}
				if cycle() != 0 || len(slices.Collect(func(yield func(string) bool) {
					for _, kind := range bed.starter.launched() {
						if kind == "build" && !yield(kind) {
							return
						}
					}
				})) != beforeBuilds+1 {
					t.Fatalf("ready pending build was not resumed by resident: before=%d launches=%v", beforeBuilds, bed.starter.launched())
				}
				for range 3 {
					cycle()
				}
				record, err = (&launch.UnitRunner{Root: bed.unitRoot}).Status(run)
				if err != nil || record.State != "awaiting-judgement" {
					t.Fatalf("resumed run did not finish: %+v %v", record.State, err)
				}
				if mode == "pending-foreign" {
					return
				}
			}

			if publisher != nil {
				// Public review and publication retain the exact collected subject.
				for range 2 {
					if code, result := bed.runJSON(bed.owners, "work", "review", bed.id, "--work", nextUnit); code != 0 && !strings.Contains(result.Summary, "publication") {
						t.Fatalf("public publication: %d %s", code, result.Summary)
					}
				}
				record, err = (&launch.UnitRunner{Root: bed.unitRoot}).Status(run)
				if err != nil || len(record.Subjects) != 1 || record.Subjects[0].Published == "" || publisher.commits != 1 || publisher.readCommits != 1 || publisher.pushes != 2 {
					t.Fatalf("publication did not hold: subjects=%+v commits=%d reads=%d pushes=%d err=%v", record.Subjects, publisher.commits, publisher.readCommits, publisher.pushes, err)
				}
			}
			if publisher == nil && (cycle() != 0 || bed.boundaries(t)[0].Next.Effect != run) {
				t.Fatal("submitted build was not observed")
			}
			if mode == "cancelled" {
				bed.owners.prove = bed.personProof
				if code, result := bed.runJSON(bed.owners, "work", "stop", "run:"+run); code != 0 {
					t.Fatalf("public stop: %d %+v", code, result)
				}
				if cycle() != 0 {
					t.Fatal("cancelled preparation read failed")
				}
				act = bed.boundaries(t)[0].Next
				if act.Effect != "" || len(act.Command) != 0 || !strings.Contains(act.Summary, "cancelled") {
					t.Fatalf("cancelled work offered continuation: %+v", act)
				}
				return
			}
			if publisher == nil {

				state.Status.Units = append(state.Status.Units, branch.UnitStatus{Unit: nextUnit, ReadState: "read clean"})
				record.Rounds[0].Stop = &loopstop.Stop{Decision: "close"}
				runner := &launch.UnitRunner{Root: bed.unitRoot, Manager: bed.manager, Git: workGit{bed.workBed}}
				bed.saveRun(t, record)
				if err := runner.ReviewSubject(run, func(review launch.UnitReview, retain func(launch.UnitSubject) error) error {
					return retain(launch.UnitSubject{Round: 1, Commit: state.BranchTip, Published: strings.Repeat("b", 40), ResultDigest: launch.UnitResultDigest(review.Result), DiffDigest: review.DiffDigest})
				}); err != nil {
					t.Fatal(err)
				}
			}

			if headless {
				first := bed.boundaries(t)[0]
				consumed, err := steward.ConsumedIntent(bed.root(), first.Handoff)
				if err != nil {
					t.Fatal(err)
				}
				job := map[string]any{"jobId": consumed.JobId, "status": "completed", "endedAt": bed.now.Format(time.RFC3339Nano), "chainClosed": true}
				body, _ := json.Marshal(job)
				path := filepath.Join(bed.root(), "artifacts", "agents", "jobs", consumed.JobId+".json")
				if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, body, 0600); err != nil {
					t.Fatal(err)
				}
				if _, err := steward.ReapContinuations(bed.root()); err != nil {
					t.Fatal(err)
				}
				if code, problem := bed.observe(t); code != 0 || len(bed.boundaries(t)) != 2 {
					t.Fatalf("second headless boundary: %d %s %+v", code, problem, bed.boundaries(t))
				}
				if cycle() != 0 || bed.boundaries(t)[1].Next != nil {
					t.Fatal("second boundary crossed withheld handoff")
				}
				note := filepath.Join(bed.root(), ".handoff-notes", "lessons.md")
				writeContextHandoffNote(t, note, "Hand the whole reviewed goal to the landing lane.\n", bed.now.Add(-time.Millisecond))
				reader := func(string, time.Time) (goal.ClaimableBudgetedWork, error) { return fleetHandoffWork(t, bed), nil }
				if code, out, problem := fleetHandoffCommand(t, bed, secondCaller, reader, note); code != 0 {
					t.Fatalf("second handoff: %d %s %s", code, out, problem)
				}
				if code, problem := bed.observe(t); code != 0 {
					t.Fatalf("second binding: %d %s", code, problem)
				}
				if cycle() != 0 || handIns != 0 {
					t.Fatal("hand-in crossed live second predecessor")
				}
				var event steward.UnitBoundary
				if err := json.NewDecoder(secondObserved).Decode(&event); err != nil || event.Handoff == "" || event.Session != secondCaller.Session || event.Goal != bed.id {
					t.Fatalf("second bound signal: %+v %v", event, err)
				}
				secondRelease.Close()
				if err := secondChild.Wait(); err != nil {
					t.Fatal(err)
				}
			} else {
				if code, problem := bed.observe(t); code != 0 || len(bed.boundaries(t)) != 2 {
					t.Fatalf("second public boundary: %d %s %+v", code, problem, bed.boundaries(t))
				}
				if code, status := bed.runJSON(bed.owners, "work", "status", bed.id); code != 0 || strings.Contains(fmt.Sprint(status.Data), "boundaryAct") {
					t.Fatalf("old preparation hid the new boundary: %d %+v", code, status)
				}
			}
			if mode == "no-lane" {
				delivery.laneRoot = func(string, time.Time) (string, bool, error) { return "", false, nil }
			}
			if mode == "hand-in-no-end" {
				start, end := bytes.Index(data, []byte("\n## Units\n")), bytes.Index(data, []byte("\n## Acceptance\n"))
				if start < 0 || end < start {
					t.Fatal("missing unit declaration fixture")
				}
				if err := os.WriteFile(page, append(append([]byte{}, data[:start]...), data[end:]...), 0600); err != nil {
					t.Fatal(err)
				}
			}
			if mode == "hand-in-helm" {
				if cycle() != 0 || bed.boundaries(t)[len(bed.boundaries(t))-1].Next == nil {
					t.Fatal("hand-in was not prepared before the helm became active")
				}
				body, _ := json.Marshal(helm.Record{Schema: 1, By: "Wido", At: bed.now.Format(time.RFC3339Nano)})
				if err := os.MkdirAll(filepath.Join(bed.root(), ".git", "metasystem"), 0700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(bed.root(), ".git", "metasystem", "helm.json"), body, 0600); err != nil {
					t.Fatal(err)
				}
			}

			if cycle() != 0 {
				t.Fatal("completed progress failed")
			}
			if handIns != 0 {
				t.Fatal("boundary driver executed the prepared hand-in")
			}
			events := bed.boundaries(t)
			act = events[len(events)-1].Next
			if mode == "transfer" {
				if act.Unit != "transfer-destination" {
					t.Fatalf("transfer destination disappeared: %+v", act)
				}
			} else if mode == "hand-in-no-end" {
				if len(act.Command) != 0 || !strings.Contains(act.Summary, "no declared end") {
					t.Fatalf("undeclared completion offered hand-in: %+v", act)
				}
			} else if shellCommand(act.Command) != "metasystem work land "+bed.id {
				t.Fatalf("final hand-in preparation: %+v", act)
			}
			if mode == "no-lane" || mode == "hand-in-helm" || mode == "hand-in-no-end" {
				if entry, found, err := plain.Latest(laneRoot, bed.id); err != nil || found {
					t.Fatalf("hand-in crossed lane/helm authority: %+v %v", entry, err)
				}
				return
			}

			if code, status := bed.runJSON(bed.owners, "work", "status", bed.id); code != 0 || !strings.Contains(fmt.Sprint(status.Data), "boundaryAct") {
				t.Fatalf("public status hid preparation: %d %+v", code, status)
			}
			if mode == "hand-in-prepared" {
				for range 2 {
					if cycle() != 0 {
						t.Fatal("prepared hand-in replay failed")
					}
				}
				if entry, found, err := plain.Latest(laneRoot, bed.id); err != nil || found || handIns != 0 {
					t.Fatalf("prepared hand-in changed the lane: %+v found=%v count=%d err=%v", entry, found, handIns, err)
				}
				return
			}
			if mode != "transfer" {
				bed.owners.prove = bed.personProof
				if code, result := bed.runJSON(bed.owners, act.Command[1:]...); code != 0 {
					t.Fatalf("follow prepared hand-in: %d %+v", code, result)
				}
				if cycle() != 0 {
					t.Fatal("hand-in reconciliation failed")
				}
				entry, found, err := plain.Latest(laneRoot, bed.id)
				if err != nil || !found || handIns != 1 || entry.SHA != state.BranchTip {
					t.Fatalf("one real hand-in: %+v %v count=%d", entry, err, handIns)
				}
				events = bed.boundaries(t)
				events[len(events)-1].Next.Effect = ""
				body, _ := json.Marshal(events)
				if err := os.WriteFile(filepath.Join(bed.root(), "artifacts", "agents", "steward", "unit-boundaries.json"), body, 0600); err != nil {
					t.Fatal(err)
				}
				for range 2 {
					if cycle() != 0 {
						t.Fatal("crash replay failed")
					}
				}
				events = bed.boundaries(t)
				if handIns != 1 || events[len(events)-1].Next.Effect != entry.SHA {
					t.Fatalf("duplicate hand-in after response loss: %d %+v", handIns, events)
				}
				if bed.goalFile(bed.id) == nil {
					t.Fatal("hand-in concluded the goal")
				}
			}

			if strings.HasPrefix(mode, "person") {
				if _, live, err := (identity.KernelProber{}).Probe(int64(os.Getpid())); err != nil || live != identity.Alive {
					t.Fatalf("person session ended: %s %v", live, err)
				}
			}
		})
	}
}
