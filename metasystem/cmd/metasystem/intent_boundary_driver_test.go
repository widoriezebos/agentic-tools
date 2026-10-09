package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal/branch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/loopstop"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
)

// This is the preparation part of the two-unit contract. Automatic admission
// and the final hand-in need their own completion proof before activation.
func TestDriverPublicBoundaryNextUnit(t *testing.T) {
	t.Parallel()
	if os.Getenv("BOUNDARY_DRIVER_ISOLATED") == "" {
		bin := t.TempDir()
		if err := testexec.WriteFile(filepath.Join(bin, "git"), []byte("#!/bin/sh\nexit 1\n"), 0700); err != nil {
			t.Fatal(err)
		}
		cmd := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^TestDriverPublicBoundaryNextUnit$", "-test.timeout=30m", "-test.v")
		cmd.Env = append(os.Environ(), "BOUNDARY_DRIVER_ISOLATED=1", "PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"))
		if data, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("isolated public boundary: %v\n%s", err, data)
		}
		return
	}
	for _, mode := range []string{"headless", "unbound-handoff", "unknown-predecessor", "person", "transfer", "changed-tip", "changed-design", "changed-goal", "unreadable", "cancelled"} {
		t.Run(mode, func(t *testing.T) {
			t.Parallel()
			bed := newFleetBoundaryBed(t, "auto")
			// The fleet fixture's branch adapter reports publication. Keep the
			// same completed subject and close decision in its unit record so
			// the real tree owner can relinquish custody before the next build.
			bed.run.Subjects[0].Published = strings.Repeat("b", 40)
			bed.run.Rounds[0].Stop = &loopstop.Stop{Decision: "close"}
			bed.saveRun(t, bed.run)
			page, data := designGatePage(t, bed.workBed, "- Critique: closed at round 1 on 0 material findings (reader)")
			data = append(data, []byte("\n## Units\n\n| Unit | Lines |\n| --- | ---: |\n| finished | 5 |\n| next | 5 |\n\n## Acceptance\n\nBoth units work.\n")...)
			if err := os.WriteFile(page, data, 0600); err != nil {
				t.Fatal(err)
			}
			policy := "auto"
			if mode == "person" {
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
			bed.owners.delivery = &intentDeliveryOwners{branchState: func(string, string) (intentBranchState, error) { return state, nil }}
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
			cycle := func() int {
				before := len(bed.starter.launched())
				var output bytes.Buffer
				code := runStewardRunWithDependencies([]string{"--repo", bed.root()}, &output, &output,
					func(root string, _ steward.WorkerCensus, _ func() error, _ time.Duration, cfg steward.TickConfig) error {
						if cfg.AdvanceBoundary == nil {
							t.Fatal("production boundary adapter missing")
						}
						return cfg.AdvanceBoundary(root)
					}, nil, nil, func(string) int { return 1 }, func() (bool, error) { return false, nil }, bed.owners)
				if code != 0 {
					t.Log(output.String())
				}
				if len(bed.starter.launched()) != before {
					t.Fatal("preparation started a child")
				}
				return code
			}
			if headless {
				if cycle() != 0 || bed.boundaries(t)[0].Next != nil {
					t.Fatal("unbound handoff prepared next work")
				}
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
				if mode != "unknown-predecessor" {
					var event steward.UnitBoundary
					if err := json.NewDecoder(observed).Decode(&event); err != nil || event.Handoff == "" {
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
			if act == nil || act.Unit != "next" || act.Tip != state.BranchTip || strings.Contains(shellCommand(act.Command), "FILE") || strings.Contains(shellCommand(act.Command), "COMMAND") {
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
			if err := os.WriteFile(brief, []byte("Build only next.\n"), 0600); err != nil {
				t.Fatal(err)
			}
			bed.head = state.BranchTip
			code, built := bed.runJSON(bed.owners, act.Command[1:]...)
			if code != 0 {
				t.Fatalf("follow prepared command: %d %+v", code, built)
			}
			run := resultData(t, built)["run"].(string)
			record, err := (&launch.UnitRunner{Root: bed.unitRoot}).Status(run)
			if err != nil || record.Base != state.BranchTip {
				t.Fatalf("current-tip build: %+v %v", record, err)
			}
			if cycle() != 0 || bed.boundaries(t)[0].Next.Effect != run {
				t.Fatal("submitted build was not observed")
			}
			if mode == "cancelled" {
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
			state.Status.Units = append(state.Status.Units, branch.UnitStatus{Unit: "next", ReadState: "read clean"})
			record.Rounds[0].Stop = &loopstop.Stop{Decision: "close"}
			runner := &launch.UnitRunner{Root: bed.unitRoot, Manager: bed.manager, Git: workGit{bed.workBed}}
			bed.saveRun(t, record)
			if err := runner.ReviewSubject(run, func(review launch.UnitReview, retain func(launch.UnitSubject) error) error {
				return retain(launch.UnitSubject{Round: 1, Commit: state.BranchTip, Published: strings.Repeat("b", 40), ResultDigest: launch.UnitResultDigest(review.Result), DiffDigest: review.DiffDigest})
			}); err != nil {
				t.Fatal(err)
			}
			// This event belongs to the still-present person's session. The
			// headless session's second handoff is separate unfinished work.
			if !headless {
				if code, problem := bed.observe(t); code != 0 || len(bed.boundaries(t)) != 2 {
					t.Fatalf("second public boundary: %d %s %+v", code, problem, bed.boundaries(t))
				}
				if code, status := bed.runJSON(bed.owners, "work", "status", bed.id); code != 0 || strings.Contains(fmt.Sprint(status.Data), "boundaryAct") {
					t.Fatalf("old preparation hid the new boundary: %d %+v", code, status)
				}
			}
			if cycle() != 0 {
				t.Fatal("completed progress failed")
			}
			events := bed.boundaries(t)
			act = events[len(events)-1].Next
			if mode == "transfer" {
				if act.Unit != "transfer-destination" {
					t.Fatalf("transfer destination disappeared: %+v", act)
				}
			} else if shellCommand(act.Command) != "metasystem work land "+bed.id {
				t.Fatalf("final hand-in preparation: %+v", act)
			}
			if code, status := bed.runJSON(bed.owners, "work", "status", bed.id); code != 0 || !strings.Contains(fmt.Sprint(status.Data), "boundaryAct") {
				t.Fatalf("public status hid preparation: %d %+v", code, status)
			}
			if mode == "person" {
				if _, live, err := (identity.KernelProber{}).Probe(int64(os.Getpid())); err != nil || live != identity.Alive {
					t.Fatalf("person session ended: %s %v", live, err)
				}
			}
		})
	}
}
