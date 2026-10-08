package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/processchange"
)

func driftStatus(t *testing.T, bed *workBed) ([]processchange.DriftStop, map[string]string) {
	t.Helper()
	code, result, output := bed.work("goal", "status", bed.id)
	if code != 0 {
		t.Fatalf("goal status: %d %+v %s", code, result, output)
	}
	data := resultData(t, result)
	var stops []processchange.DriftStop
	body, _ := json.Marshal(data["processStops"])
	if err := json.Unmarshal(body, &stops); err != nil {
		t.Fatal(err)
	}
	var bands map[string]string
	body, _ = json.Marshal(data["work"].([]any)[0].(map[string]any)["processBands"])
	if err := json.Unmarshal(body, &bands); err != nil {
		t.Fatal(err)
	}
	return stops, bands
}

func driftSettings(t *testing.T, bed *workBed, person bool) (int, intentResult, string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(bed.root(), ".git"), 0700); err != nil {
		t.Fatal(err)
	}
	owners := bed.workOwners()
	owners.dependencies.ownerLineage = func() string { return "builder" }
	owners.prove = fixedFixtureGoalAuthority
	if person {
		owners.prove = enrolledPersonProver(t, bed.root(), bed.manager.Now())
	}
	owners.policies.ConfPath = func(checkout string) (string, error) { return filepath.Join(checkout, "settings.conf"), nil }
	owners.policies.Registry = func(string) (config.PolicyRegistry, error) { return config.PolicyRegistry{}, nil }
	var stdout, stderr bytes.Buffer
	command, args, ok := resolveIntentArgv([]string{"settings", "set", "launch.codex.sandbox", "workspace-write", "--repo", bed.root(), "--goal", bed.id, "--json"})
	if !ok {
		t.Fatal("settings set did not resolve")
	}
	code := runIntentIn(command, args, &stdout, &stderr, bed.root(), owners)
	var result intentResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("settings: %v %s %s", err, &stdout, &stderr)
	}
	return code, result, stdout.String()
}

func TestProcessDriftPublicStop(t *testing.T) {
	t.Parallel()
	bed := newWorkBed(t)
	clock := &workClock{now: time.Date(2026, 10, 8, 10, 0, 0, 0, time.UTC)}
	bed.manager.Now, bed.manager.Supervisor = clock.Now, processCostStarter{bed.starter, clock}
	processEstimatePage(t, bed, "1", "100")
	code, built, output := processEvidenceBuild(bed)
	if code != 0 {
		t.Fatalf("ordinary build held: %d %+v %s", code, built, output)
	}
	stops, bands := driftStatus(t, bed)
	if len(stops) != 1 || stops[0].Stop.Loop != "process" || stops[0].Stop.Attempt != 1 || stops[0].Stop.Budget != 1 || bands["unit elapsed minutes"] != "above band" || stops[0].Question == "" {
		t.Fatalf("drift: %+v %v", stops, bands)
	}
	textCode, text, stderr := bed.run(bed.workOwners(), "goal", "status", bed.id)
	if textCode != 0 || !strings.Contains(text, "Automatic process changes are held") {
		t.Fatalf("human status: %d %s %s", textCode, text, stderr)
	}
	q, err := channel.ReadQuestion(bed.root(), stops[0].Question)
	if err != nil || q.State != "open" || !strings.Contains(q.Facts[0], "unit elapsed") {
		t.Fatalf("question: %+v %v", q, err)
	}
	paths, err := filepath.Glob(filepath.Join(bed.root(), "process", "episodes", "*", "stops", "*.json"))
	if err != nil || len(paths) != 1 {
		t.Fatalf("stop storage: %v %v", paths, err)
	}
	before, _ := os.ReadFile(paths[0])
	clock.Sleep(time.Minute)
	if code, result, _ := driftSettings(t, bed, false); code != 1 || !strings.Contains(result.Summary, "held") {
		t.Fatalf("agent intervention: %d %+v", code, result)
	}
	again, _ := driftStatus(t, bed)
	after, _ := os.ReadFile(paths[0])
	if len(again) != 1 || again[0].ID != stops[0].ID || !bytes.Equal(before, after) {
		t.Fatalf("replay changed stop: %+v", again)
	}
	paths, _ = filepath.Glob(filepath.Join(bed.root(), "process", "episodes", "*", "stops", "*.json"))
	questions, unknown := channel.WalkQuestions(bed.root())
	if len(paths) != 1 || len(questions) != 1 || len(unknown) != 0 {
		t.Fatalf("replay duplicated stop or ask: %v %+v %v", paths, questions, unknown)
	}
	if code, result, _ := driftSettings(t, bed, true); code != 0 {
		t.Fatalf("person blocked: %d %+v", code, result)
	}
	if err := os.WriteFile(paths[0], []byte("damaged stop"), 0600); err != nil {
		t.Fatal(err)
	}
	if code, result, _ := bed.work("goal", "status", bed.id); code != 0 || !strings.Contains(jsonText(result.Data), "process stop unavailable") {
		t.Fatalf("damage hidden: %d %+v", code, result)
	}
	if code, result, _ := driftSettings(t, bed, true); code != 0 {
		t.Fatalf("damaged advisory stop vetoed person: %d %+v", code, result)
	}
	t.Log("collection opened one stop; repeated admission preserved it; a person applied the explicit setting")
}

func TestProcessDriftEqualityAndUnknownPublicStatus(t *testing.T) {
	t.Parallel()
	bed := newWorkBed(t)
	clock := &workClock{now: time.Date(2026, 10, 8, 10, 0, 0, 0, time.UTC)}
	bed.manager.Now = clock.Now
	processEstimatePage(t, bed, "1", "100")
	code, built, output := processEvidenceBuild(bed)
	if code != 0 {
		t.Fatalf("build: %d %+v %s", code, built, output)
	}
	clock.Sleep(2 * time.Minute)
	stops, bands := driftStatus(t, bed)
	if len(stops) != 0 || bands["unit elapsed minutes"] != "in band" {
		t.Fatalf("equality: %+v %v", stops, bands)
	}
	if code, result, _ := driftSettings(t, bed, false); code != 1 || strings.Contains(result.Summary, "automatic process changes are held") {
		t.Fatalf("equality became drift: %d %+v", code, result)
	}
	stops, _ = driftStatus(t, bed)
	if len(stops) != 0 {
		t.Fatalf("equal band opened stop: %+v", stops)
	}
	clock.Sleep(time.Nanosecond)
	stops, bands = driftStatus(t, bed)
	if len(stops) != 0 || bands["unit elapsed minutes"] != "above band" {
		t.Fatalf("status must be read-only: %+v %v", stops, bands)
	}
	if code, result, _ := driftSettings(t, bed, false); code != 1 || !strings.Contains(result.Summary, "automatic process changes are held") {
		t.Fatalf("fresh admission missed overrun: %d %+v", code, result)
	}
	stops, _ = driftStatus(t, bed)
	if len(stops) != 1 {
		t.Fatalf("missed collection not recovered: %+v", stops)
	}
	t.Log("exactly twice the estimate stayed in band; fresh admission detected the overrun")
}

func TestProcessDriftCheckBandsPublicStatus(t *testing.T) {
	t.Parallel()
	for _, full := range []bool{false, true} {
		t.Run(map[bool]string{false: "declared-check", true: "full-suite"}[full], func(t *testing.T) {
			t.Parallel()
			bed := newWorkBed(t)
			clock := &workClock{now: time.Date(2026, 10, 8, 10, 0, 0, 0, time.UTC)}
			bed.manager.Now, bed.manager.Supervisor = clock.Now, processCostStarter{bed.starter, clock}
			check := "0"
			if full {
				check = "100"
			}
			processEstimatePage(t, bed, "500", check)
			if full {
				hook := bed.workOwnersHook
				bed.workOwnersHook = func(owners *intentWorkOwners) {
					hook(owners)
					git := owners.git
					owners.git = func(root string, args ...string) ([]byte, error) {
						if len(args) == 2 && args[0] == "show" && strings.HasSuffix(args[1], ":metasystem.conf") {
							return []byte("proof.full=" + strings.Join(workArgv, " ") + "\nproof.cheap=" + shellCommand(workArgv) + "\nproof.audits=true\nproof.deadline=15\n"), nil
						}
						return git(root, args...)
					}
				}
			}
			code, built, output := processEvidenceBuild(bed)
			if code != 0 {
				t.Fatalf("build: %d %+v %s", code, built, output)
			}
			if full {
				// Retained direct full-suite executions have their own argv; nested checks remain unavailable.
				run := resultData(t, built)["run"].(string)
				record, err := (&launch.UnitRunner{Root: bed.unitRoot}).Status(run)
				if err != nil {
					t.Fatal(err)
				}
				record.Rounds[0].Steps[1].Command.Argv = workArgv
				body, err := json.Marshal(record)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(bed.unitRoot, run, "run.json"), body, 0600); err != nil {
					t.Fatal(err)
				}
				if code, result, _ := driftSettings(t, bed, false); code != 1 {
					t.Fatalf("full-suite history did not hold the process change: %d %+v", code, result)
				}
			}

			stops, bands := driftStatus(t, bed)
			name := "unit check minutes"
			if full {
				name = "unit full-suite minutes"
			}
			if len(stops) != 1 || bands[name] != "above band" || stops[0].Stop.Measure.Name != name {
				t.Fatalf("check band: %+v %v", stops, bands)
			}
		})
	}
}

func TestProcessDriftFreshEpisodePublicCollection(t *testing.T) {
	t.Parallel()
	for _, standing := range []bool{false, true} {
		t.Run(map[bool]string{false: "old-cost-only", true: "standing-stop"}[standing], func(t *testing.T) {
			t.Parallel()
			bed := newWorkBed(t)
			clock := &workClock{now: time.Date(2026, 10, 8, 10, 0, 0, 0, time.UTC)}
			bed.manager.Now = clock.Now
			if standing {
				bed.manager.Supervisor = processCostStarter{bed.starter, clock}
			}
			processEstimatePage(t, bed, "1", "100")
			if code, result, output := processEvidenceBuild(bed); code != 0 {
				t.Fatalf("first build: %d %+v %s", code, result, output)
			}
			before, _ := driftStatus(t, bed)
			if (len(before) == 1) != standing {
				t.Fatalf("initial stop: %+v", before)
			}
			clock.Sleep(time.Hour)
			announceProofFixtureHolder(t, bed.root())
			owners := bed.workOwners()
			owners.commandNow = func(string) (time.Time, error) { return clock.Now(), nil }
			for _, args := range [][]string{
				{"goal", "release", bed.id, "--lineage", "m1", "--reason", "Start work under a fresh claim"},
				{"goal", "claim", bed.id, "--lineage", "m2"},
			} {
				if args[1] == "claim" {
					owners.dependencies.machine = func(string) (string, error) { return "mac-other", nil }
				}
				if code, result := bed.runJSON(owners, args...); code != 0 {
					t.Fatalf("%v: %d %+v", args, code, result)
				}
			}
			file := bed.project()
			if file.Claimed == nil || file.Claimed.EpisodeAt != clock.Now().Format(time.RFC3339) {
				t.Fatalf("claim did not start a fresh episode: %+v", file.Claimed)
			}
			_, bands := driftStatus(t, bed)
			if bands["unit elapsed minutes"] != "unknown" {
				t.Fatalf("old elapsed blocks a new episode: %v", bands)
			}
			bed.worktree = filepath.Join(filepath.Dir(bed.worktree), "fresh-work")
			if err := os.MkdirAll(bed.worktree, 0700); err != nil {
				t.Fatal(err)
			}
			args := append([]string{"work", "build", "--json", bed.id, "evidence", "--brief", "estimate.md"}, workCheck...)
			code, stdout, stderr := bed.run(owners, args...)
			var result intentResult
			if err := json.Unmarshal([]byte(stdout), &result); err != nil {
				t.Fatalf("fresh collection: %v %s %s", err, stdout, stderr)
			}
			bed.recordReadDirs(result)
			if code != 0 {
				t.Fatalf("fresh build and collection: %d %+v", code, result)
			}
			after, bands := driftStatus(t, bed)
			if len(after) != len(before) || (standing && (after[0].ID != before[0].ID || after[0].Question != before[0].Question)) {
				t.Fatalf("fresh collection duplicated the standing stop: before=%+v after=%+v", before, after)
			}
			want := "in band"
			if standing {
				want = "above band"
			}
			if bands["unit elapsed minutes"] != want {
				t.Fatalf("current episode band: %v, want %s", bands, want)
			}
			paths, err := filepath.Glob(filepath.Join(bed.root(), "process", "episodes", "*", "stops", "*.json"))
			if err != nil || len(paths) != len(before) {
				t.Fatalf("stop storage duplicated: %v %v", paths, err)
			}
		})
	}
}

func TestProcessDriftUnreadableRunsPublicStatus(t *testing.T) {
	t.Parallel()
	bed := newWorkBed(t)
	processEstimatePage(t, bed, "500", "100")
	if code, result, output := processEvidenceBuild(bed); code != 0 {
		t.Fatalf("build: %d %+v %s", code, result, output)
	}
	if err := os.Chmod(bed.unitRoot, 0300); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(bed.unitRoot, 0700) })
	if _, err := os.ReadDir(bed.unitRoot); err == nil {
		t.Skip("this user can read directories without read permission")
	}
	_, bands := driftStatus(t, bed)
	if len(bands) != 3 {
		t.Fatalf("missing unknown bands: %v", bands)
	}
	for name, band := range bands {
		if band != "unknown" {
			t.Fatalf("unreadable runs yielded a known %s band: %v", name, bands)
		}
	}
	code, result, output := bed.work("goal", "status", bed.id)
	if code != 0 || !strings.Contains(jsonText(resultData(t, result)["work"].([]any)[0].(map[string]any)["processUnknown"]), "process measurement unavailable") {
		t.Fatalf("run read error hidden: %d %+v %s", code, result, output)
	}
	textCode, text, stderr := bed.run(bed.workOwners(), "goal", "status", bed.id)
	if textCode != 0 || !strings.Contains(text, "process measurement unavailable") {
		t.Fatalf("human status hid read error: %d %s %s", textCode, text, stderr)
	}
}
