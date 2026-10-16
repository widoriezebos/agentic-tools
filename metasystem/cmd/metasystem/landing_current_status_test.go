package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
)

func runCurrentStatus(t *testing.T, bed layoutBed, flags ...string) (int, string) {
	t.Helper()
	command, _ := findIntentAction("landing", "status")
	var out bytes.Buffer
	code := runIntentIn(command, flags, &out, &out, bed.cwd, bed.owners)
	return code, out.String()
}

func TestLandingStatusQueueHistoryAndWording(t *testing.T) {
	t.Parallel()
	for _, verbose := range []bool{false, true} {
		t.Run(map[bool]string{false: "default", true: "verbose"}[verbose], func(t *testing.T) {
			t.Parallel()
			bed := landingLayoutBed(landingLayoutRunning)(t)
			home, _ := bed.owners.landing.home()
			record, _, err := lane.Read(home)
			if err != nil {
				t.Fatal(err)
			}
			dir := plain.Dir(record.Install)
			if err := os.MkdirAll(dir, 0755); err != nil {
				t.Fatal(err)
			}
			old := bed.now.Add(-72 * time.Hour).Format(time.RFC3339)
			returned := bed.now.Add(-48 * time.Hour).Format(time.RFC3339)
			recent := bed.now.Add(-time.Hour).Format(time.RFC3339)
			lines := []plain.Line{
				{Goal: "unresolved", SHA: "u", Seat: "secret-seat", At: old}, {Goal: "unresolved", SHA: "u", Outcome: plain.StateReturned, At: returned, Reason: "fix failing check"},
				{Goal: "same", SHA: "s", Seat: "secret-seat", At: old}, {Goal: "same", SHA: "s", Outcome: plain.StateReturned, At: returned}, {Goal: "same", SHA: "s", Seat: "secret-seat", At: recent},
				{Goal: "changed", SHA: "c1", Seat: "secret-seat", At: old}, {Goal: "changed", SHA: "c1", Outcome: plain.StateReturned, At: returned}, {Goal: "changed", SHA: "c2", Seat: "secret-seat", At: recent},
				{Goal: "old-landing", SHA: "l1", Seat: "secret-seat", At: old}, {Goal: "recent-landing", SHA: "l2", Seat: "secret-seat", At: old}, {Goal: "external", SHA: "e", Seat: "secret-seat", At: old},
				{Goal: "replaced", SHA: "r1", Seat: "secret-seat", At: old}, {Goal: "replaced", SHA: "r2", Seat: "secret-seat", At: recent},
			}
			var data bytes.Buffer
			for _, line := range lines {
				if err := json.NewEncoder(&data).Encode(line); err != nil {
					t.Fatal(err)
				}
			}
			helmMust(t, os.WriteFile(filepath.Join(dir, "queue.jsonl"), data.Bytes(), 0600))
			selectionLines(t, filepath.Join(dir, "pushes.jsonl"), plain.Pushed{Old: "m0", Commit: "m1", At: returned}, plain.Pushed{Old: "m1", Commit: "m2", At: recent})
			selectionLines(t, filepath.Join(dir, "batch.json"), plain.Batch{ID: "selection", Lane: record, State: plain.BatchPrepared, Base: "main", Members: []plain.GoalSHA{{Goal: "same", SHA: "s"}}})
			helmMust(t, os.WriteFile(filepath.Join(dir, "resolve-begun.json"), []byte("old repair"), 0600))
			git := bed.owners.landing.plainProve.Git
			seams := bed.owners.landing.plainProve
			seams.Git = func(dir string, args ...string) (string, error) {
				if args[0] == "cat-file" {
					if strings.HasPrefix(args[2], "l1^") || strings.HasPrefix(args[2], "l2^") || strings.HasPrefix(args[2], "e^") || strings.HasPrefix(args[2], "main^") {
						return "", nil
					}
					return "", errors.New("not on main")
				}
				if args[0] == "merge-base" {
					return "", nil
				}
				if args[0] == "rev-list" && len(args) > 2 && args[1] == "--parents" {
					return "main m2 e\nm2 l2\nl2 m1\nm1 l1\nl1 m0\nm0\ne", nil
				}
				if args[0] == "rev-list" {
					return "", nil
				}
				return git(dir, args...)
			}
			bed.owners.landing.plainProve = seams
			bed.owners.landing.status = func(home string, record lane.Record, view lane.View) plain.Status {
				return plain.ReadStatus(home, record, view, seams)
			}
			var flags []string
			if verbose {
				flags = []string{"--verbose"}
			}
			code, text := runCurrentStatus(t, bed, flags...)
			compact := oneSpaced(text)
			if code != 0 || !strings.Contains(compact, "3 waiting, 1 returned") {
				t.Fatalf("current counts: %d %s", code, text)
			}
			jsonCode, jsonText := runCurrentStatus(t, bed, "--json")
			if jsonCode != 0 || !strings.Contains(jsonText, "3 waiting, 1 returned") {
				t.Fatalf("JSON current counts: %d %s", jsonCode, jsonText)
			}
			queue := text
			history := ""
			if at := strings.Index(text, "\nHistory\n"); at >= 0 {
				queue, history = text[:at], text[at:]
			}
			for _, want := range []string{"Queue", "unresolved", "fix failing check", "s · waiting", "c2 · waiting", "r2 · waiting", "based on main"} {
				if !strings.Contains(oneSpaced(queue), want) {
					t.Errorf("missing %q: %s", want, text)
				}
			}
			for _, forbidden := range []string{"secret-seat", "prepared", "superseded", "resolve-begun.json", "proofs/"} {
				if strings.Contains(text, forbidden) {
					t.Errorf("internal wording %q: %s", forbidden, text)
				}
			}
			for _, forbidden := range []string{"old-landing", "recent-landing", "external", "c1 ·", "r1 ·"} {
				if strings.Contains(queue, forbidden) {
					t.Errorf("history in queue %q: %s", forbidden, text)
				}
			}
			if verbose {
				for _, want := range []string{"History", "old-landing", "recent-landing", "external", "time not recorded", "c1", "r1", "replaced by a newer hand-in", lane.LocalText(returned), lane.LocalText(recent)} {
					if !strings.Contains(oneSpaced(history), oneSpaced(want)) {
						t.Errorf("history missing %q: %s", want, text)
					}
				}
				if strings.Count(history, "returned, "+lane.LocalText(returned)) != 2 {
					t.Errorf("replaced returns lost their event times: %s", history)
				}
			} else if history != "" {
				t.Fatal("default view shows history: " + text)
			}
			if strings.Contains(text, "to do:") {
				t.Fatal("active lane asks for an unnecessary act: " + text)
			}
		})
	}
}

func TestLandingStatusClosedBatchHasNoSelectedBlock(t *testing.T) {
	t.Parallel()
	for _, verbose := range []bool{false, true} {
		t.Run(map[bool]string{false: "default", true: "verbose"}[verbose], func(t *testing.T) {
			t.Parallel()
			bed := landingLayoutBed(landingLayoutRunning)(t)
			read := bed.owners.landing.status
			bed.owners.landing.status = func(home string, r lane.Record, v lane.View) plain.Status {
				status := read(home, r, v)
				status.Batch = &plain.Batch{ID: "closed-selection", State: plain.BatchClosed, SealedAt: bed.now.Format(time.RFC3339), Members: []plain.GoalSHA{{Goal: "landed-goal", SHA: "tip"}}}
				return status
			}
			var flags []string
			if verbose {
				flags = []string{"--verbose"}
			}
			code, text := runCurrentStatus(t, bed, flags...)
			if code != 0 || strings.Contains(text, "Selected batch") || strings.Contains(text, "closed-selection") {
				t.Fatalf("closed batch displayed: %d %s", code, text)
			}
		})
	}
}

func TestLandingProofStatusControlledStartup(t *testing.T) {
	t.Parallel()
	for _, outcome := range []string{"success", "launch error", "dead owner"} {
		t.Run(outcome, func(t *testing.T) {
			t.Parallel()
			bed := landingLayoutBed(landingLayoutRunning)(t)
			home, _ := bed.owners.landing.home()
			record, _, _ := lane.Read(home)
			seams := bed.owners.landing.plainProve
			seams.Trunk, seams.Fresh = true, true
			ownerAlive := true
			seams.Alive = func(running plain.Running) bool {
				if running.Admission != nil && running.Admission.State == "pending" {
					if running.Launch == nil || running.Launch.Owner == "" || running.Process != running.Launch.Owner {
						t.Fatalf("pending owner was not substituted: %+v", running)
					}
					ref, err := identity.ParseRef(running.Launch.Owner)
					if err != nil || running.Pid != ref.Pid {
						t.Fatalf("pending owner PID: %+v %v", running, err)
					}
				}
				return ownerAlive
			}
			seams.Executable = func() (string, error) { return "/fixture/engine", nil }
			seams.NewID = func() string { return "startup" }
			git := seams.Git
			seams.Git = func(dir string, args ...string) (string, error) {
				if args[0] == "ls-tree" {
					return "", nil
				}
				switch strings.Join(args, " ") {
				case "rev-parse --verify origin/main^{commit}":
					return "head", nil
				case "rev-parse --verify head^{tree}":
					return "tree", nil
				}
				return git(dir, args...)
			}
			check := func(want string, recovery bool) {
				t.Helper()
				for _, flags := range [][]string{nil, {"--json"}} {
					code, text := runCurrentStatus(t, bed, flags...)
					if code != 0 {
						t.Fatalf("status: %d %s", code, text)
					}
					summary := oneSpaced(text)
					if len(flags) > 0 {
						var response struct {
							Summary string
							Data    plain.Status
						}
						helmMust(t, json.Unmarshal([]byte(text), &response))
						summary = response.Summary
						state := "running"
						if want == "starting the check" {
							state = "pending"
						}
						if recovery {
							state = "failed"
						}
						if response.Data.RunningProof == nil || response.Data.RunningProof.State != state {
							t.Fatalf("JSON ownership: %s", text)
						}
					}
					if !strings.Contains(summary, want) || strings.Contains(summary, "metasystem landing resume") != recovery {
						t.Fatalf("want %q recovery=%v: %s", want, recovery, text)
					}
				}
			}
			bed.owners.landing.status = func(home string, r lane.Record, v lane.View) plain.Status { return plain.ReadStatus(home, r, v, seams) }
			seams.Launch = func(_ []string, _ string, log string) (int64, error) {
				running, recorded, alive, err := plain.ReadRunning(record.Install, seams)
				if err != nil || !recorded || !alive || running.Admission.State != "pending" || running.Pid != 0 || running.Process != "" || running.Launch == nil || running.Launch.Owner == "" {
					t.Fatalf("launch has no durable live ownership: %+v recorded=%v alive=%v err=%v", running, recorded, alive, err)
				}
				check("starting the check", false)
				helmMust(t, os.WriteFile(log, nil, 0600))
				if outcome == "launch error" {
					return 0, errors.New("launcher unavailable")
				}
				if outcome == "dead owner" {
					ownerAlive = false
					check("the lane's check failed", true)
					return 0, errors.New("owner died")
				}
				return int64(os.Getpid()), nil
			}
			_, _, err := plain.Start(record.Install, record.Root, seams)
			if outcome == "success" {
				if err != nil {
					t.Fatal(err)
				}
				check("proving main", false)
			} else {
				if err == nil {
					t.Fatal("launcher failure was accepted")
				}
				check("the lane's check failed", true)
			}
		})
	}
}
