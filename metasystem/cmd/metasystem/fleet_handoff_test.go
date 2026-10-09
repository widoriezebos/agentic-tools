package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testprovider"
)

const fleetHandoffChild = "FLEET_HANDOFF_CHILD_ROOT"

func TestFleetBoundaryPublicHandoff(t *testing.T) {
	t.Parallel()
	if root := os.Getenv(fleetHandoffChild); root != "" {
		terms := make(chan os.Signal, 8)
		signal.Notify(terms, syscall.SIGTERM)
		defer signal.Stop(terms)
		ready, observed := os.NewFile(3, "ready"), os.NewFile(4, "observed")
		defer observed.Close()
		if _, err := ready.Write([]byte("x")); err != nil {
			t.Fatal(err)
		}
		ready.Close()
		released := make(chan struct{})
		go func() { _, _ = io.Copy(io.Discard, os.Stdin); close(released) }()
		for {
			select {
			case <-terms:
				events, err := steward.ReadUnitBoundaries(root)
				if err != nil || len(events) == 0 || events[len(events)-1].Handoff == "" {
					t.Fatalf("signal preceded durable binding: %+v %v", events, err)
				}
				if err := json.NewEncoder(observed).Encode(events[len(events)-1]); err != nil {
					t.Fatal(err)
				}
			case <-released:
				return
			}
		}
	}
	if os.Getenv("FLEET_HANDOFF_ISOLATED") == "" {
		tools := t.TempDir()
		if err := testexec.WriteFile(filepath.Join(tools, "git"), []byte(`#!/bin/sh
root=$(pwd -P)
if [ "$1" = -C ]; then root=$2; shift 2; fi
if [ "$1" = -c ] && [ "$2" = core.logAllRefUpdates=false ]; then shift 2; fi
[ -f "$root/fleet-seat-brief" ] || exit 1
tip=aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa
case "$*" in
  'config --get metasystem.goal.machine') printf 'm1\n' ;;
  'config --get goal.sync-remote') printf 'local\n' ;;
  'rev-parse --verify --quiet refs/metasystem/goals/accepted'|'rev-parse --verify --quiet refs/metasystem/goals/accepted^{commit}'|'rev-parse --verify refs/heads/metasystem/goals') printf '%s\n' "$tip" ;;
  "cat-file -e $tip:./plans/goals/backlog.md") exit 0 ;;
  "ls-tree -r --name-only $tip -- plans/goals/ records/goals/") printf 'plans/goals/backlog.md\nplans/goals/brief-goal.md\n' ;;
  "ls-tree -r --name-only $tip -- plans/goals/backlog.md") printf 'plans/goals/backlog.md\n' ;;
  "ls-tree --name-only $tip -- plans/goals/backlog.md") printf 'plans/goals/backlog.md\n' ;;
  'rev-parse --verify --quiet refs/heads/goal/brief-goal') cat "$root/tip" ;;
  'cat-file --batch')
    while IFS= read -r object; do
      case "$object" in
        "$tip:./plans/goals/backlog.md") file="$root/plans/goals/backlog.md" ;;
        "$tip:./plans/goals/brief-goal.md") file="$root/plans/goals/brief-goal.md" ;;
        *) exit 1 ;;
      esac
      printf '%s blob %s\n' "$tip" "$(wc -c < "$file" | tr -d ' ')"
      cat "$file"
      printf '\n'
    done ;;
  update-ref\ -d\ refs/metasystem/goals/fetch/*|update-ref\ -d\ refs/metasystem/goals/txn/*) exit 0 ;;
  *) exit 1 ;;
esac
`), 0700); err != nil {
			t.Fatal(err)
		}
		command := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^TestFleetBoundaryPublicHandoff$", "-test.timeout=30m", "-test.v")
		for _, entry := range os.Environ() {
			name, _, _ := strings.Cut(entry, "=")
			if name != "PATH" && name != "METASYSTEM_SUPERVISION_REGISTRY_HOME" && name != "METASYSTEM_TESTENV_REGISTRY_NONCE" {
				command.Env = append(command.Env, entry)
			}
		}
		command.Env = append(command.Env, "FLEET_HANDOFF_ISOLATED=1", "PATH="+tools+string(os.PathListSeparator)+os.Getenv("PATH"))
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("isolated public handoff: %v\n%s", err, output)
		}
		return
	}
	t.Run("driver-read-failure", func(t *testing.T) {
		t.Parallel()
		brief := fleetSeatBrief(t, "invalid", time.Date(2026, 10, 8, 8, 0, 0, 0, time.UTC))
		for _, text := range []string{"stop advancing at that boundary", "metasystem session handoff", `Seat driver unavailable: seat.driver must be auto or person, not "invalid"`} {
			if !strings.Contains(brief, text) {
				t.Fatalf("unreadable driver brief missing %q: %s", text, brief)
			}
		}
	})
	t.Run("abnormal-seat-restarts", testFleetAbnormalSeatRestarts)
	t.Run("completed-seat-progress", testFleetCompletedSeatProgress)
	t.Run("unreserved-start-failure", testFleetUnreservedStartFailure)
	for _, scenario := range []string{"headless", "capture-before-binding", "person-driver", "person-session", "unknown-identity", "reused-pid", "capture-failure", "binding-failure"} {
		t.Run(scenario, func(t *testing.T) {
			t.Parallel()
			driver, lineage := "auto", steward.SeatLineage
			if scenario == "person-driver" {
				driver = "person"
			}
			if scenario == "person-session" {
				lineage = "coordinator"
			}
			bed := newFleetBoundaryBed(t, driver)
			if scenario == "headless" {
				brief := fleetSeatBrief(t, driver, bed.now)
				for _, instruction := range []string{"stop advancing at that boundary", "metasystem session handoff", "binding is durable"} {
					if !strings.Contains(brief, instruction) {
						t.Fatalf("headless brief missing %q: %s", instruction, brief)
					}
				}
			}
			if scenario == "person-driver" {
				brief := fleetSeatBrief(t, driver, bed.now)
				for _, instruction := range []string{"stop advancing at that boundary", "At the completed unit boundary", "metasystem session handoff", "binding is durable"} {
					if strings.Contains(brief, instruction) {
						t.Fatalf("person-driver brief includes automatic handoff instruction %q: %s", instruction, brief)
					}
				}
			}
			child, release, observed := startFleetHandoffChild(t, bed.root())
			noSignal := func() {
				release.Close()
				if err := child.Wait(); err != nil {
					t.Fatalf("session exit: %v", err)
				}
				var event steward.UnitBoundary
				if err := json.NewDecoder(observed).Decode(&event); err != io.EOF {
					t.Fatalf("session was signalled: %+v %v", event, err)
				}
			}
			exact, state, err := (identity.KernelProber{}).Probe(int64(child.Process.Pid))
			if err != nil || state != identity.Alive {
				t.Fatalf("predecessor identity: %s %v", state, err)
			}
			ref, tag := exact.Ref(), "-test.run=^TestFleetBoundaryPublicHandoff$"
			if scenario == "unknown-identity" {
				tag = "missing-predecessor-tag"
			}
			if err := os.Remove(filepath.Join(bed.root(), "artifacts", "agents", "mains", "worktree-lease.json")); err != nil {
				t.Fatal(err)
			}
			announcementPath, err := lease.AnnounceWithPair(bed.root(), bed.session, ref.Pid, ref.StartedAtSec, ref.StartTicks, ref.BootID, tag, "fake", lineage)
			if err != nil {
				t.Fatal(err)
			}
			announcementBytes, err := os.ReadFile(announcementPath)
			var announcement lease.Announcement
			if err != nil || json.Unmarshal(announcementBytes, &announcement) != nil {
				t.Fatalf("predecessor announcement: %v", err)
			}
			mainID := announcement.MainId
			ref = identity.Ref{Pid: announcement.Pid, StartedAtSec: announcement.PidStartedAt,
				StartTicks: announcement.PidStartTicks, BootID: announcement.BootID}
			if code, problem := bed.observe(t); code != 0 {
				t.Fatalf("public completed boundary: %d %s", code, problem)
			}
			if events := bed.boundaries(t); len(events) != 1 || events[0].Handoff != "" || events[0].Signalled {
				t.Fatalf("boundary without capture: %+v", events)
			}
			caller := steward.HandoffCaller{Class: lease.ClassMain, MainId: mainID, HolderMainId: mainID,
				Runtime: "fake", Session: bed.session, Machine: "m1", Ref: ref, Tag: tag}
			work := fleetHandoffWork(t, bed)
			file, _ := work.OwnedClaim(bed.id)
			caller.Machine = file.Claimed.Machine
			noteDir := filepath.Join(bed.root(), ".handoff-notes")
			note := filepath.Join(noteDir, "lessons.md")
			if err := os.MkdirAll(noteDir, 0700); err != nil {
				t.Fatal(err)
			}
			writeContextHandoffNote(t, note, "Continue from the completed unit.\n", bed.now.Add(-time.Minute))
			conf, err := os.OpenFile(filepath.Join(bed.root(), "metasystem.conf"), os.O_APPEND|os.O_WRONLY, 0600)
			if err != nil {
				t.Fatal(err)
			}
			_, err = fmt.Fprintf(conf, "context.handoff.note-directory.fake=%s\nrole.steward-continuation.runtime=fake\nrole.steward-continuation.model.fake=fixture\n", noteDir)
			conf.Close()
			if err != nil {
				t.Fatal(err)
			}
			if err := steward.MintIdentity(steward.RepoIdentityPath(bed.root()), steward.InstallIdentity{
				RepoIdentity: bed.root(), Generation: 1, InstallPath: "/bin/true", MintedAt: bed.now.Format(time.RFC3339),
			}); err != nil {
				t.Fatal(err)
			}
			if err := os.MkdirAll(filepath.Join(bed.root(), "memory"), 0700); err != nil {
				t.Fatal(err)
			}
			readWork := func(string, time.Time) (goal.ClaimableBudgetedWork, error) { return fleetHandoffWork(t, bed), nil }
			var persisted steward.HandoffResult
			var atSignal steward.UnitBoundary
			if scenario == "capture-before-binding" || scenario == "binding-failure" {
				persisted, err = steward.HandoffWithWorkReader(bed.root(), caller, steward.HandoffRecord{NotePath: note, NoteDirectory: noteDir}, bed.now,
					filepath.Join(bed.root(), "memory", "receipts.log"), readWork)
				if err != nil {
					t.Fatal(err)
				}
				if bed.boundaries(t)[0].Handoff != "" {
					t.Fatal("capture alone bound the boundary")
				}
				if scenario == "capture-before-binding" {
					if code, problem := bed.observe(t); code != 0 {
						t.Fatalf("capture recovery: %d %s", code, problem)
					}
					if err := json.NewDecoder(observed).Decode(&atSignal); err != nil || atSignal.Handoff != persisted.Nonce {
						t.Fatalf("recovered capture was not bound before SIGTERM: %+v %v", atSignal, err)
					}
				}
			}
			if scenario == "binding-failure" {
				if err := os.Chmod(filepath.Dir(fleetBoundaryPath(bed.root())), 0500); err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { _ = os.Chmod(filepath.Dir(fleetBoundaryPath(bed.root())), 0700) })
			}
			if scenario == "capture-failure" {
				if err := os.Remove(note); err != nil {
					t.Fatal(err)
				}
			}
			if scenario == "reused-pid" {
				caller.Ref.StartedAtSec -= 100
				caller.Ref.StartedAtUnixMicro, caller.Ref.StartTicks, caller.Ref.BootID = 0, 0, ""
			}
			var captured contextHandoffOutput
			code, output, problem := fleetHandoffCommand(t, bed, caller, readWork, note)
			if scenario == "binding-failure" {
				if code == 0 || !strings.Contains(problem, "permission denied") || bed.boundaries(t)[0].Handoff != "" {
					t.Fatalf("public binding failure: %d %s %s", code, output, problem)
				}
				if code, problem := bed.observe(t); code == 0 || !strings.Contains(problem, "permission denied") || bed.boundaries(t)[0].Signalled {
					t.Fatalf("failed binding signal: %d %s", code, problem)
				}
				if err := os.Chmod(filepath.Dir(fleetBoundaryPath(bed.root())), 0700); err != nil {
					t.Fatal(err)
				}
				code, output, problem = fleetHandoffCommand(t, bed, caller, readWork, note)
			}
			if scenario == "capture-failure" {
				if code == 0 || !strings.Contains(problem, "HANDOFF_NOTE_OUTSIDE_MEMORY") {
					t.Fatalf("exact capture failure missing: %d %s", code, problem)
				}
				if code, problem := bed.observe(t); code != 0 {
					t.Fatalf("failed capture observation: %d %s", code, problem)
				}
				if events := bed.boundaries(t); events[0].Handoff != "" || events[0].Signalled {
					t.Fatalf("failed capture ended session: %+v", events)
				}
				noSignal()
				return
			}
			if code != 0 || json.Unmarshal([]byte(output), &captured) != nil {
				t.Fatalf("public capture: %d %s %s", code, output, problem)
			}
			stateBytes, err := os.ReadFile(captured.StatePath)
			if err != nil {
				t.Fatal(err)
			}
			if persisted.Nonce != "" && persisted.Nonce != captured.Nonce {
				t.Fatalf("re-entry replaced capture %s with %s", persisted.Nonce, captured.Nonce)
			}
			code, problem = bed.observe(t)
			if scenario == "unknown-identity" {
				if code == 0 || !strings.Contains(problem, "identity is unknown") {
					t.Fatalf("handoff must hold with the exact error: %d %s", code, problem)
				}
				if bed.boundaries(t)[0].Signalled {
					t.Fatal("failed binding or unknown identity sent a signal")
				}
				noSignal()
				return
			}
			if code != 0 {
				t.Fatalf("public steward handoff: %d %s", code, problem)
			}
			if scenario == "person-driver" || scenario == "person-session" {
				if event := bed.boundaries(t)[0]; event.Handoff != "" || event.Signalled {
					t.Fatalf("person was bound or signalled: %+v", event)
				}
				noSignal()
				return
			}
			if scenario == "reused-pid" {
				if _, alive, err := (identity.KernelProber{}).Probe(ref.Pid); err != nil || alive != identity.Alive {
					t.Fatalf("reused process was signalled: %s %v", alive, err)
				}
				noSignal()
				return
			}
			if event := bed.boundaries(t)[0]; !event.Signalled || event.Handoff != captured.Nonce {
				t.Fatalf("durable signal result missing: %+v", event)
			}
			if atSignal.Handoff == "" {
				if err := json.NewDecoder(observed).Decode(&atSignal); err != nil || atSignal.Handoff != captured.Nonce {
					t.Fatalf("predecessor saw no binding at SIGTERM: %+v %v", atSignal, err)
				}
			}
			if event := bed.boundaries(t)[0]; !event.Signalled || event.Handoff != captured.Nonce {
				t.Fatalf("durable signal result missing: %+v", event)
			}
			// A stale note cannot force another capture after publication.
			writeContextHandoffNote(t, note, "changed after capture\n", time.Unix(1, 0))
			code, output, problem = fleetHandoffCommand(t, bed, caller, readWork, note)
			var replay contextHandoffOutput
			if code != 0 || json.Unmarshal([]byte(output), &replay) != nil || replay.Nonce != captured.Nonce {
				t.Fatalf("capture replay: %d %s %s", code, output, problem)
			}
			if after, err := os.ReadFile(captured.StatePath); err != nil || !bytes.Equal(after, stateBytes) {
				t.Fatalf("immutable capture changed: %v", err)
			}
			if code, problem := bed.observe(t); code != 0 {
				t.Fatalf("bound replay: %d %s", code, problem)
			}
			// The legacy ledger keeps admission on the same owned goal while
			// Git is unavailable. Boundary and capture used the public accepted
			// projection above; exact predecessor admission is shared by both.
			if err := os.Remove(filepath.Join(bed.root(), "plans", "goals", "backlog.md")); err != nil {
				t.Fatal(err)
			}
			legacy := fmt.Sprintf("# Goals\n\n## Current goal: %s — Continue the completed unit.\n- Origin: main\n- Next step: Continue.\n", bed.id)
			if err := os.WriteFile(goal.LedgerPath(bed.root()), []byte(legacy), 0600); err != nil {
				t.Fatal(err)
			}
			launches := 0
			admit := func() steward.ReviveOutcome {
				outcome, err := steward.CompleteRevival(bed.root(), steward.TickConfig{Now: bed.now, ProviderHome: testprovider.Home(bed.root())}, fleetHandoffCensus{}, captured.Nonce,
					func(steward.Intent) error { launches++; return nil }, nil)
				if err != nil {
					t.Fatal(err)
				}
				return outcome
			}
			if out := admit(); !out.Held || launches != 0 {
				t.Fatalf("successor admitted before observed death: %+v launches=%d", out, launches)
			}
			release.Close()
			if err := child.Wait(); err != nil {
				t.Fatalf("predecessor exit: %v", err)
			}
			before, err := steward.LoadEvidence(steward.EvidencePath(bed.root()))
			if err != nil {
				t.Fatal(err)
			}
			// A planned handoff remains admissible even with a spent dry cap.
			before.DryRevivals = 3
			if err := steward.SaveEvidence(bed.root(), steward.EvidencePath(bed.root()), before); err != nil {
				t.Fatal(err)
			}
			if out := admit(); !out.Launched || launches != 1 {
				t.Fatalf("dead predecessor did not admit successor: %+v launches=%d", out, launches)
			}
			after, err := steward.LoadEvidence(steward.EvidencePath(bed.root()))
			if err != nil || after.DryRevivals != before.DryRevivals || after.AbnormalCount != before.AbnormalCount {
				t.Fatalf("planned handoff spent an abnormal attempt: %+v %v", after, err)
			}
			if out := admit(); out.Launched || launches != 1 {
				t.Fatalf("successor was repeated: %+v launches=%d", out, launches)
			}
			var repeated steward.UnitBoundary
			if err := json.NewDecoder(observed).Decode(&repeated); err != io.EOF {
				t.Fatalf("predecessor signalled again: %+v %v", repeated, err)
			}
		})
	}
}

type fleetHandoffCensus struct{}

type fleetBriefLauncher struct{ spec steward.SeatLaunchSpec }

func (l *fleetBriefLauncher) StartSeat(spec steward.SeatLaunchSpec) error {
	l.spec = spec
	return nil
}
func (*fleetBriefLauncher) SeatLaunch(string) (steward.SeatLaunchState, error) {
	return steward.SeatLaunchState{}, nil
}
func (*fleetBriefLauncher) SeatAllowed(string) (bool, string, error) { return true, "", nil }

func fleetSeatBrief(t *testing.T, driver string, now time.Time) string {
	t.Helper()
	root := fleetSeatFixture(t, driver, now)
	launcher := &fleetBriefLauncher{}
	record, err := steward.StartSeat(root, steward.TickConfig{Now: now, ProviderHome: testprovider.Home(root), Seat: launcher, WorkStateRoot: t.TempDir()}, fleetHandoffCensus{}, steward.SeatSelection{Goal: "brief-goal"})
	if err != nil || record.LaunchID == "" || launcher.spec.Brief == "" {
		t.Fatalf("public seat start: %+v %v", record, err)
	}
	brief, err := os.ReadFile(launcher.spec.Brief)
	if err != nil {
		t.Fatal(err)
	}
	return string(brief)
}

func fleetSeatFixture(t *testing.T, driver string, now time.Time) string {
	t.Helper()
	root := t.TempDir()
	testprovider.Register(t, root)
	file := &goal.GoalFile{Id: "brief-goal", State: goal.StateApproved, Tier: 1, Intent: "Continue the unit.", Origin: goal.OriginMain,
		NextStep: "Build it.", OpenedAt: now.Add(-time.Hour).Format(time.RFC3339), Revision: 1,
		Budget: &goal.Budget{ElapsedLimit: "4h", AttemptLimit: 4, ReservedJobMinutesLimit: 240, ActiveJobLimit: 2}}
	opid := goal.Opid("01ARZ3NDEKTSV4RRFFQ69G5FAV", "m1", "approval")
	at := now.Add(-time.Hour).Format(time.RFC3339)
	file.History = []goal.HistoryLine{{At: at, Opid: opid, Verb: "approve", Actor: "human:Wido", Targets: []string{file.Id}, Keep: -1}}
	file.Approved = &goal.ApprovalRecord{By: "human:Wido", At: at, Revision: 1, Opid: opid,
		Authority: goal.ApprovalAuthorityProven, Digest: goal.ApprovalDigest(file.Intent, file.Tier, *file.Budget)}
	if _, problems := goal.ParseTreeFiles(map[string][]byte{
		"plans/goals/backlog.md":    goal.RenderRoot(&goal.RootRecord{Identity: "01ARZ3NDEKTSV4RRFFQ69G5FAV", FormatVersion: "1", SyncMode: goal.SyncLocal, Revision: 1}),
		"plans/goals/brief-goal.md": goal.RenderFile(file),
	}); len(problems) != 0 {
		t.Fatalf("brief fixture: %v", problems)
	}
	for path, data := range map[string][]byte{
		"fleet-seat-brief":          nil,
		"metasystem.conf":           []byte("seat.driver=" + driver + "\n"),
		"plans/goals/backlog.md":    goal.RenderRoot(&goal.RootRecord{Identity: "01ARZ3NDEKTSV4RRFFQ69G5FAV", FormatVersion: "1", SyncMode: goal.SyncLocal, Revision: 1}),
		"plans/goals/brief-goal.md": goal.RenderFile(file),
	} {
		name := filepath.Join(root, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(name), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(name, data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func (fleetHandoffCensus) Workers(string) (steward.Workers, error) {
	return steward.Workers{CensusComplete: true}, nil
}

func fleetBoundaryPath(root string) string {
	return filepath.Join(root, "artifacts", "agents", "steward", "unit-boundaries.json")
}

func fleetHandoffWork(t *testing.T, bed *fleetBoundaryBed) goal.ClaimableBudgetedWork {
	t.Helper()
	inv := &intentInvocation{cwd: bed.root(), owners: bed.owners}
	if problem := inv.selectRoot(); problem != nil {
		t.Fatal(problem.Summary)
	}
	projection, _, problem := inv.projection()
	if problem != nil {
		t.Fatal(problem.Summary)
	}
	machine, err := bed.owners.dependencies.machine(inv.stateRoot)
	if err != nil {
		t.Fatal(err)
	}
	work, err := goal.ClaimableWorkFromProjection(projection, machine, identity.KernelProber{})
	if err != nil {
		t.Fatal(err)
	}
	return work
}

func fleetHandoffCommand(t *testing.T, bed *fleetBoundaryBed, caller steward.HandoffCaller, readWork func(string, time.Time) (goal.ClaimableBudgetedWork, error), note string) (int, string, string) {
	t.Helper()
	return captureContextVerb(t, func(args []string, stdout, stderr io.Writer) int {
		return runContextHandoffWithInputs(args, contextHandoffInputs{
			resolveMachine: func(string) (string, error) { return caller.Machine, nil }, readWork: readWork,
			caller: func(stateroot.Installation, string, func(string) (string, error)) (steward.HandoffCaller, error) {
				return caller, nil
			},
			now: func() time.Time { return bed.now },
		}, stdout, stderr)
	}, "--root", bed.root(), "--note", note, "--no-delegates", "--json", "--verbose")
}

func startFleetHandoffChild(t *testing.T, root string) (*exec.Cmd, *os.File, *os.File) {
	t.Helper()
	readyRead, readyWrite, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	releaseRead, releaseWrite, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	observedRead, observedWrite, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	child := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^TestFleetBoundaryPublicHandoff$", "-test.timeout=30m")
	child.Env = append(os.Environ(), fleetHandoffChild+"="+root)
	child.Stdin, child.ExtraFiles = releaseRead, []*os.File{readyWrite, observedWrite}
	var problem bytes.Buffer
	child.Stderr = &problem
	if err := child.Start(); err != nil {
		t.Fatal(err)
	}
	readyWrite.Close()
	releaseRead.Close()
	observedWrite.Close()
	t.Cleanup(func() {
		releaseWrite.Close()
		if child.ProcessState == nil {
			if err := child.Wait(); err != nil {
				t.Errorf("predecessor: %v %s", err, &problem)
			}
		}
		observedRead.Close()
	})
	var ready [1]byte
	if _, err := io.ReadFull(readyRead, ready[:]); err != nil || ready[0] != 'x' {
		t.Fatalf("predecessor readiness: %q %v", ready, err)
	}
	readyRead.Close()
	return child, releaseWrite, observedRead
}

// The launcher substitutes only process outcomes; the tick reads retained
// branch tips, reaps seats and rechecks admission through StartSeat.
type fleetRestartLauncher struct {
	started func(steward.SeatLaunchSpec) error
	state   steward.SeatLaunchState
}

func (l *fleetRestartLauncher) StartSeat(spec steward.SeatLaunchSpec) error { return l.started(spec) }
func (l *fleetRestartLauncher) SeatLaunch(string) (steward.SeatLaunchState, error) {
	return l.state, nil
}
func (*fleetRestartLauncher) SeatAllowed(string) (bool, string, error) { return true, "", nil }

func testFleetAbnormalSeatRestarts(t *testing.T) {
	t.Parallel()
	for _, scenario := range []string{"hour-cap", "same-class", "no-progress", "unknown-start"} {
		t.Run(scenario, func(t *testing.T) {
			t.Parallel()
			now := time.Date(2026, 10, 8, 8, 0, 0, 0, time.UTC)
			root := fleetSeatFixture(t, "auto", now)
			launched, restarts := 0, 0
			launcher := &fleetRestartLauncher{}
			launcher.started = func(steward.SeatLaunchSpec) error {
				ev, err := steward.LoadEvidence(steward.EvidencePath(root))
				if err != nil {
					return err
				}
				if launched > 0 && (ev.AbnormalCount != restarts+1 || !ev.Abnormal[ev.AbnormalCount-1].Pending) {
					t.Fatalf("seat launched before counting: %+v", ev)
				}
				launched++
				if launched > 1 {
					restarts++
				}
				if scenario == "unknown-start" && launched == 2 {
					return errors.New("process creation outcome unavailable")
				}
				launcher.state = steward.SeatLaunchState{Found: true, Terminal: true, State: "failed", FinishedAt: now.Format(time.RFC3339)}
				return nil
			}
			cfg := steward.TickConfig{Now: now, ProviderHome: testprovider.Home(root), Seat: launcher, WorkStateRoot: t.TempDir()}
			first, err := steward.StartSeat(root, cfg, fleetHandoffCensus{}, steward.SeatSelection{Goal: "brief-goal"})
			if err != nil || first.LaunchID == "" {
				t.Fatalf("initial seat: %+v %v", first, err)
			}
			observe := func(tip string) steward.TickResult {
				t.Helper()
				if err := os.WriteFile(filepath.Join(root, "tip"), []byte(tip), 0600); err != nil {
					t.Fatal(err)
				}
				result, err := steward.RunTick(root, cfg, fleetHandoffCensus{})
				if err != nil {
					t.Fatal(err)
				}
				return result
			}
			// Initial start consumes no abnormal allowance, even if it ends dry.
			tick := observe(strings.Repeat("b", 40))
			if tick.Seat == nil {
				t.Fatalf("first abnormal retry held: %+v", tick.Decision)
			}
			_, err = steward.StartSeat(root, cfg, fleetHandoffCensus{}, *tick.Seat)
			if scenario == "unknown-start" {
				if err == nil {
					t.Fatal("unknown start was reported successful")
				}
			} else if err != nil {
				t.Fatal(err)
			}
			if scenario == "hour-cap" {
				launcher.state.State = "cancelled"
			}
			tip := strings.Repeat("c", 40)
			if scenario == "no-progress" {
				tip = strings.Repeat("b", 40)
			}
			tick = observe(tip)
			if scenario == "hour-cap" {
				if tick.Seat == nil {
					t.Fatalf("second abnormal retry held: %+v", tick.Decision)
				}
				if _, err := steward.StartSeat(root, cfg, fleetHandoffCensus{}, *tick.Seat); err != nil {
					t.Fatal(err)
				}
				launcher.state.State = "failed"
				tick = observe(strings.Repeat("d", 40))
			}
			if tick.Seat != nil || tick.Decision.Action != steward.ActNotify || !strings.Contains(tick.Decision.Reason, "machine revive "+root+" (unavailable until fleet-provider-and-session-recovery R2 lands)") {
				t.Fatalf("public seat tick did not stop: %+v", tick)
			}
			if restarts > 2 {
				t.Fatalf("seat exceeded two restarts: %d", restarts)
			}
			var output, problem bytes.Buffer
			if code := runStewardStatus([]string{"--repo", root}, &output, &problem); code != 0 || !strings.Contains(output.String(), "machine revive") {
				t.Fatalf("seat stop missing from status: %d %s %s", code, &output, &problem)
			}
		})
	}
}

func testFleetCompletedSeatProgress(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 10, 8, 8, 0, 0, 0, time.UTC)
	root := fleetSeatFixture(t, "auto", now)
	launched := 0
	launcher := &fleetRestartLauncher{
		state: steward.SeatLaunchState{Found: true, Terminal: true, State: "completed", FinishedAt: now.Format(time.RFC3339)},
	}
	launcher.started = func(steward.SeatLaunchSpec) error {
		ev, err := steward.LoadEvidence(steward.EvidencePath(root))
		if err != nil {
			return err
		}
		if ev.AbnormalCount != 0 || ev.Abnormal != [2]steward.AbnormalRestart{} {
			t.Fatalf("completed seat with progress reserved an abnormal restart: %+v", ev)
		}
		launched++
		return nil
	}
	cfg := steward.TickConfig{Now: now, ProviderHome: testprovider.Home(root), Seat: launcher, WorkStateRoot: t.TempDir()}
	selection := steward.SeatSelection{Goal: "brief-goal"}
	for i := 0; i < 4; i++ {
		if record, err := steward.StartSeat(root, cfg, fleetHandoffCensus{}, selection); err != nil || record.LaunchID == "" {
			t.Fatalf("normal seat start %d: %+v %v", i, record, err)
		}
		if err := os.WriteFile(filepath.Join(root, "tip"), []byte(strings.Repeat(string(rune('b'+i)), 40)), 0600); err != nil {
			t.Fatal(err)
		}
		cfg.Now = cfg.Now.Add(time.Minute)
		tick, err := steward.RunTick(root, cfg, fleetHandoffCensus{})
		if err != nil || tick.Seat == nil {
			t.Fatalf("completed seat with progress stopped the next start: %+v %v", tick.Decision, err)
		}
		selection = *tick.Seat
	}
	if launched != 4 {
		t.Fatalf("normal completions stopped automatic starts: %d", launched)
	}
}

func testFleetUnreservedStartFailure(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 10, 8, 8, 0, 0, 0, time.UTC)
	root := fleetSeatFixture(t, "auto", now)
	launched := 0
	launcher := &fleetRestartLauncher{}
	launcher.started = func(steward.SeatLaunchSpec) error {
		ev, err := steward.LoadEvidence(steward.EvidencePath(root))
		if err != nil {
			return err
		}
		if launched == 0 && ev.AbnormalCount != 0 {
			t.Fatalf("initial start was reserved: %+v", ev)
		}
		launched++
		// A supervisor can start and die before Manager.Start returns its error.
		child := exec.CommandContext(t.Context(), "/bin/sh", "-c", testexec.ReadyPrologue+"exit 1")
		if err := testexec.StartReady(child); err != nil {
			t.Fatal(err)
		}
		if err := child.Wait(); err == nil || child.ProcessState.ExitCode() != 1 {
			t.Fatalf("supervisor did not fail after starting: %v", err)
		}
		launcher.state = steward.SeatLaunchState{Found: true, Terminal: true, State: "failed", FinishedAt: now.Format(time.RFC3339)}
		return errors.New("child-start: executable unavailable")
	}
	cfg := steward.TickConfig{Now: now, ProviderHome: testprovider.Home(root), Seat: launcher, WorkStateRoot: t.TempDir()}
	selection := steward.SeatSelection{Goal: "brief-goal"}
	record, err := steward.StartSeat(root, cfg, fleetHandoffCensus{}, selection)
	if err == nil || record.Outcome != steward.SeatStartFailed || record.LaunchState != "" {
		t.Fatalf("unreserved failed start: %+v %v", record, err)
	}
	if err := os.WriteFile(filepath.Join(root, "tip"), []byte(strings.Repeat("b", 40)), 0600); err != nil {
		t.Fatal(err)
	}
	for _, elapsed := range []time.Duration{time.Minute, 2 * time.Hour} {
		cfg.Now = now.Add(elapsed)
		tick, err := steward.RunTick(root, cfg, fleetHandoffCensus{})
		if err != nil || tick.Seat != nil || tick.Decision.Action != steward.ActNotify || !strings.Contains(tick.Decision.Reason, "unknown launch outcome") {
			t.Fatalf("unknown unreserved start admitted a restart: %+v %v", tick.Decision, err)
		}
		if _, err := steward.StartSeat(root, cfg, fleetHandoffCensus{}, selection); err == nil || !strings.Contains(err.Error(), "unknown launch outcome") {
			t.Fatalf("locked start bypassed the unknown outcome hold: %v", err)
		}
	}
	if launched != 1 {
		t.Fatalf("unknown start launched %d automatic restarts; expected none", launched-1)
	}
	var output, problem bytes.Buffer
	if code := runStewardStatus([]string{"--repo", root}, &output, &problem); code != 0 || !strings.Contains(output.String(), "unknown launch outcome") || !strings.Contains(output.String(), "machine revive "+root+" (unavailable until fleet-provider-and-session-recovery R2 lands)") {
		t.Fatalf("unknown outcome hold missing from public status: %d %s %s", code, &output, &problem)
	}
}
