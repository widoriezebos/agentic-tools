package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/helm"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/readsubject"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
)

func driverBed(t *testing.T) *workBed {
	t.Helper()
	bed := declaredCheckBed(t, "proof.cheap=true\nproof.audits=true\nproof.deadline=15\n")
	if err := os.MkdirAll(filepath.Join(bed.root(), ".git"), 0700); err != nil {
		t.Fatal(err)
	}
	return bed
}

func driverOwners(t *testing.T, bed *workBed, policy string, atHelm *bool) intentOwners {
	t.Helper()
	owners := bed.workOwners()
	conf := filepath.Join(t.TempDir(), "driver.conf")
	if err := os.WriteFile(conf, []byte("seat.driver="+policy+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	owners.lookupEnv = func(string) (string, bool) { return "", false }
	owners.policies = config.PolicyReaders{
		Registry: func(string) (config.PolicyRegistry, error) { return config.PolicyRegistry{}, nil },
		ConfPath: func(string) (string, error) { return conf, nil },
		Helm:     func(string) helm.State { return helm.State{Active: *atHelm, By: "Wido"} },
	}
	owners.prove = func(root string, _ int64, _ humanauthority.Reader, _, _ string, now time.Time) (humanauthority.Proof, error) {
		return humanauthority.HelmProof(root, humanauthority.HelmGrant{By: "Wido", Grant: "driver-test"}, now)
	}
	return owners
}

func driverVerb(t *testing.T, bed *workBed, owners intentOwners, argv ...string) (int, intentResult) {
	t.Helper()
	code, out, stderr := bed.run(owners, append(argv, "--json")...)
	var result intentResult
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("%v: %d %s %s: %v", argv, code, out, stderr, err)
	}
	bed.recordReadDirs(result)
	return code, result
}

func driverBuild(t *testing.T, bed *workBed, unit string, withRead ...bool) string {
	t.Helper()
	body := "Build the unit.\n"
	if len(withRead) > 0 && withRead[0] {
		body = "Read each round: yes\n" + body
	}
	brief := bed.brief(unit+".md", body)
	code, result, _ := bed.work("work", "build", bed.id, unit, "--brief", brief, "--lines", "20")
	if code != 0 && code != 3 {
		t.Fatalf("build: %d %+v", code, result)
	}
	return resultData(t, result)["run"].(string)
}

func driverFinish(t *testing.T, bed *workBed, kind string) {
	t.Helper()
	records, err := bed.manager.Store.List()
	if err != nil {
		t.Fatal(err)
	}
	for _, record := range records {
		if record.Kind != kind || record.State.Terminal() {
			continue
		}
		if _, err := bed.manager.Store.Update(record.ID, func(current *launch.Record) error {
			exit := 0
			current.State, current.ExitCode = launch.Completed, &exit
			current.FinishedAt = bed.manager.Now().UTC().Format(time.RFC3339Nano)
			return nil
		}); err != nil {
			t.Fatal(err)
		}
	}
}

func TestDriverPublicContinuationAuthority(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"person", "helm"} {
		for _, verb := range []string{"wait", "build", "build-reentry", "revise", "revise-by"} {
			t.Run(mode+"/"+verb, func(t *testing.T) {
				t.Parallel()
				bed := driverBed(t)
				revision := strings.HasPrefix(verb, "revise")
				if !revision {
					bed.starter.hold = "build"
				} else {
					bed.manager.Supervisor = &stopReadStarter{bed: bed, reads: [][]readsubject.Finding{{stopFinding("regression", "a.go")}}}
				}
				run := driverBuild(t, bed, "authority", revision)
				driverFinish(t, bed, "build")
				bed.starter.hold = "proof"
				atHelm := mode == "helm"
				policy := "auto"
				if mode == "person" {
					policy = "person"
				}
				owners := driverOwners(t, bed, policy, &atHelm)
				action, _, _ := strings.Cut(verb, "-")
				argv := []string{"work", action, "run:" + run}
				if verb == "build-reentry" {
					argv = []string{"work", "build", bed.id, "authority", "--brief", filepath.Join(bed.root(), "authority.md"), "--lines", "20"}
				}
				if revision {
					bed.starter.hold = "build"
					record, err := (&launch.UnitRunner{Root: bed.unitRoot}).Status(run)
					if err != nil || len(record.Rounds[0].Reads) != 1 {
						t.Fatalf("read: %+v %v", record, err)
					}
					finding := record.Rounds[0].Reads[0].Findings[0].ID
					argv = append(argv, "--brief", bed.brief("correction.md", fmt.Sprintf("Correct the unit.\n\n## Decisions on round 1\n\n| Finding | Decision | Evidence |\n| --- | --- | --- |\n| %s | fixed | a.go:12 |\n", finding)))
				}
				if verb == "revise-by" {
					argv = append(argv, "--reason", "correct the unit", "--by", "Wido")
				}
				before := len(bed.starter.launched())
				code, result := driverVerb(t, bed, owners, argv...)
				if code != 3 || result.Next == nil || !strings.Contains(result.Summary, "awaits your command") || len(bed.starter.launched()) != before {
					t.Fatalf("agent continued: %d %+v launches=%v", code, result, bed.starter.launched())
				}
				record, err := (&launch.UnitRunner{Root: bed.unitRoot}).Status(run)
				if err != nil || record.Rounds[0].Steps[0].State != launch.StepPassed || len(record.Rounds) != 1 {
					t.Fatalf("ended build wasn't collected or correction started: %+v %v", record, err)
				}
				if _, err := os.Stat(humanauthority.AttorneyLogPath(bed.root())); !errors.Is(err, os.ErrNotExist) {
					t.Fatalf("person probe wrote refusal log: %v", err)
				}
				now, err := owners.commandNow(bed.root())
				if err != nil {
					t.Fatal(err)
				}
				owners.prove = enrolledPersonProver(t, bed.root(), now)
				// Follow exactly the proposed command, with this invocation's own person proof.
				personArgv := result.Next.Argv[1:]
				if verb == "wait" {
					personArgv = argv
				}
				code, personResult := driverVerb(t, bed, owners, personArgv...)
				if code != 3 || len(bed.starter.launched()) != before+1 {
					t.Fatalf("person couldn't follow remedy: %d %+v launches=%v", code, personResult, bed.starter.launched())
				}
				owners = driverOwners(t, bed, policy, &atHelm)
				driverFinish(t, bed, "proof")
				if revision {
					driverFinish(t, bed, "build")
					bed.starter.hold = "proof"
				}
				_, _ = driverVerb(t, bed, owners, "work", "build", "run:"+run)
				if revision && len(bed.starter.launched()) != before+1 {
					t.Fatal("an earlier person invocation gave the agent continuation power")
				}
			})
		}
	}
}

func TestDriverPublicUnknownReadAuthority(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"person", "helm"} {
		t.Run(mode, func(t *testing.T) {
			t.Parallel()
			bed := driverBed(t)
			reader := &stopReadStarter{bed: bed, reads: [][]readsubject.Finding{{stopFinding("regression", "a.go")}}, corrupt: "missing-class"}
			bed.manager.Supervisor = reader
			run := driverBuild(t, bed, "unknown-read", true)
			beforeRecord, err := (&launch.UnitRunner{Root: bed.unitRoot}).Status(run)
			if err != nil || beforeRecord.Rounds[0].Stop == nil || !strings.HasPrefix(beforeRecord.Rounds[0].Stop.Handoff, "stopped ") {
				t.Fatalf("unknown read wasn't stopped: %+v %v", beforeRecord, err)
			}
			atHelm := mode == "helm"
			policy := "auto"
			if mode == "person" {
				policy = "person"
			}
			owners := driverOwners(t, bed, policy, &atHelm)
			before := len(bed.starter.launched())
			code, result := driverVerb(t, bed, owners, "work", "review", "run:"+run)
			if code != 3 || !strings.Contains(result.Summary, "awaits your command") || result.Next == nil || len(bed.starter.launched()) != before {
				t.Fatalf("agent retried the unknown read: %d %+v launches=%v", code, result, bed.starter.launched())
			}
			record, err := (&launch.UnitRunner{Root: bed.unitRoot}).Status(run)
			if err != nil || record.Rounds[0].UnknownRetries != 0 || len(record.Rounds[0].Steps) != len(beforeRecord.Rounds[0].Steps) {
				t.Fatalf("observation spent the read allowance: %+v %v", record, err)
			}
			now, err := owners.commandNow(bed.root())
			if err != nil {
				t.Fatal(err)
			}
			owners.prove = enrolledPersonProver(t, bed.root(), now)
			reader.corrupt = ""
			reader.reads = [][]readsubject.Finding{{}}
			bed.starter.hold = "read"
			code, result = driverVerb(t, bed, owners, result.Next.Argv[1:]...)
			if code != 3 || len(bed.starter.launched()) != before+1 {
				t.Fatalf("person couldn't follow unknown-read remedy: %d %+v launches=%v", code, result, bed.starter.launched())
			}
		})
	}
}

func driverStewardCycle(t *testing.T, bed *workBed, owners intentOwners, resident ...bool) {
	t.Helper()
	if err := driverStewardCycleResult(t, bed, owners, resident...); err != nil {
		t.Fatal(err)
	}
}

func driverStewardCycleResult(t *testing.T, bed *workBed, owners intentOwners, resident ...bool) error {
	var driveErr error
	t.Helper()
	var clock *steward.HandoffClock
	if len(resident) > 0 && resident[0] {
		start := bed.manager.Now()
		clock = &steward.HandoffClock{Now: bed.manager.Now, Sleep: func(d time.Duration) {
			bed.manager.Sleep(d)
			if !bed.manager.Now().Before(start.Add(time.Second)) {
				if err := os.WriteFile(filepath.Join(bed.root(), "artifacts", "agents", "steward", "stop"), nil, 0600); err != nil {
					t.Fatal(err)
				}
			}
		}}
	}
	registered := families()
	for i := range registered {
		if registered[i].name != "steward" {
			continue
		}
		for j := range registered[i].verbs {
			if registered[i].verbs[j].name != "run" {
				continue
			}
			registered[i].verbs[j].run = func(args []string, stdout, stderr io.Writer) int {
				return runStewardRunWithDependencies(args, stdout, stderr,
					func(root string, census steward.WorkerCensus, revive func() error, interval time.Duration, cfg steward.TickConfig) error {
						if clock != nil {
							return steward.RunLoop(root, census, revive, interval, cfg)
						}
						if cfg.DriveWork == nil {
							return errors.New("steward has no work continuation")
						}
						driveErr = cfg.DriveWork(root)
						if driveErr != nil {
							fmt.Fprintln(stderr, driveErr)
						}
						return nil
					}, nil, clock, func(string) int { return 1 }, nil, owners)
			}
		}
	}
	var stdout, stderr bytes.Buffer
	code := dispatchWithFamiliesAndRepositoryTop([]string{"steward", "run", "--repo", bed.root()}, &stdout, &stderr, registered, fakeTop(bed.root()))
	if code != 0 {
		t.Fatalf("steward cycle: %d %s %s", code, &stdout, &stderr)
	}
	return driveErr
}

type driverGit struct{ first, second *workBed }

func (g driverGit) Run(directory string, environment []string, args ...string) ([]byte, error) {
	actual, _ := filepath.EvalSymlinks(directory)
	second, _ := filepath.EvalSymlinks(g.second.worktree)
	if actual == second {
		return (workGit{g.second}).Run(directory, environment, args...)
	}
	return (workGit{g.first}).Run(directory, environment, args...)
}

func TestDriverPublicStewardCollection(t *testing.T) {
	t.Parallel()
	bed := driverBed(t)
	bed.starter.hold = "build"
	first := driverBuild(t, bed, "z-oldest")
	bed.manager.Sleep(time.Second)
	other := &workBed{intentBed: bed.intentBed, id: bed.id, worktree: filepath.Join(filepath.Dir(bed.worktree), "other-work"), manager: bed.manager, starter: bed.starter,
		unitRoot: bed.unitRoot, branchListed: true, head: bed.head, designGate: bed.designGate, workOwnersHook: bed.workOwnersHook, readDirs: map[string]bool{}}
	if err := os.MkdirAll(other.worktree, 0700); err != nil {
		t.Fatal(err)
	}
	second := driverBuild(t, other, "a-newest")
	driverFinish(t, bed, "build")
	bed.starter.hold = "proof"
	atHelm := true
	owners := driverOwners(t, bed, "auto", &atHelm)
	units := owners.work.units
	owners.work.units = func(layout stateroot.Layout) *launch.UnitRunner {
		runner := units(layout)
		runner.Git = driverGit{bed, other}
		return runner
	}
	git := owners.work.git
	owners.work.git = func(root string, args ...string) ([]byte, error) {
		data, err := git(root, args...)
		if slices.Equal(args, []string{"worktree", "list", "--porcelain"}) && err == nil {
			data = append(data, []byte("\nworktree "+other.worktree+"\nHEAD "+other.head+"\nbranch refs/heads/goal/"+other.id+"\n")...)
		}
		return data, err
	}
	before := len(bed.starter.launched())
	helmDir := filepath.Join(bed.root(), ".git", "metasystem")
	if err := os.MkdirAll(helmDir, 0700); err != nil {
		t.Fatal(err)
	}
	helmPath := filepath.Join(helmDir, "helm.json")
	if err := os.WriteFile(helmPath, []byte(`{"schema":1,"by":"Wido","at":"2026-09-01T10:00:00Z"}`), 0600); err != nil {
		t.Fatal(err)
	}
	driverStewardCycle(t, bed, owners, true)
	if err := os.Remove(helmPath); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(bed.root(), "artifacts", "agents", "steward", "stop")); err != nil {
		t.Fatal(err)
	}
	for _, run := range []string{first, second} {
		record, err := (&launch.UnitRunner{Root: bed.unitRoot}).Status(run)
		if err != nil || record.Rounds[0].Steps[0].State != launch.StepPassed {
			t.Fatalf("helm suppressed collection: %+v %v", record, err)
		}
	}
	if len(bed.starter.launched()) != before {
		t.Fatal("the automatic steward inherited the caller's power under the helm")
	}
	atHelm = false
	personOwners := driverOwners(t, bed, "person", &atHelm)
	personOwners.work, personOwners.connection = owners.work, owners.connection
	driverStewardCycle(t, bed, personOwners)
	if len(bed.starter.launched()) != before {
		t.Fatal("the steward started work under person policy")
	}
	driverStewardCycle(t, bed, owners)
	if len(bed.starter.launched()) != before+1 {
		t.Fatalf("automatic steward did not advance exactly one ready run: %v", bed.starter.launched())
	}
	for _, run := range []string{first, second} {
		record, err := (&launch.UnitRunner{Root: bed.unitRoot}).Status(run)
		if err != nil || len(record.Rounds) != 1 || record.State != "running" || record.Rounds[0].Steps[0].State != launch.StepPassed {
			t.Fatalf("automatic steward lost collected state: %+v %v", record, err)
		}
		steps := record.Rounds[0].Steps
		if run == first && (len(steps) < 2 || steps[1].State != launch.StepRunning) || run == second && len(steps) != 1 {
			t.Fatalf("automatic steward did not start the oldest ready run: %+v %v", record, err)
		}
	}

}

func TestDriverPublicWaitDeadline(t *testing.T) {
	t.Parallel()
	bed := driverBed(t)
	bed.starter.hold = "build"
	run := driverBuild(t, bed, "deadline")
	atHelm := false
	owners := driverOwners(t, bed, "auto", &atHelm)
	start := bed.manager.Now()
	sleep := bed.manager.Sleep
	bed.manager.Sleep = func(d time.Duration) {
		sleep(d)
		if bed.manager.Now().Sub(start) > 3*time.Second {
			t.Fatal("wait exceeded its absolute caller deadline")
		}
		if bed.manager.Now().Sub(start) == time.Second {
			driverFinish(t, bed, "build")
			bed.starter.hold = "proof"
		}
	}
	code, result := driverVerb(t, bed, owners, "work", "wait", "run:"+run, "--timeout", "3s")
	if code != 3 || bed.manager.Now().Sub(start) != 3*time.Second || !strings.Contains(result.Summary, "caller's wait timed out") {
		t.Fatalf("caller deadline reset or hidden: elapsed=%s %d %+v", bed.manager.Now().Sub(start), code, result)
	}
	records, err := bed.manager.Store.List()
	if err != nil {
		t.Fatal(err)
	}
	running := 0
	for _, record := range records {
		if record.Kind == "proof" && record.State == launch.Running {
			running++
		}
	}
	if running != 1 {
		t.Fatalf("wait timeout stopped work: %+v", records)
	}
	driverFinish(t, bed, "proof")
	code, result = driverVerb(t, bed, owners, "work", "wait", "run:"+run, "--timeout", "1s")
	if code != 0 || resultData(t, result)["state"] != "awaiting-judgement" {
		t.Fatalf("work didn't survive timeout: %d %+v", code, result)
	}
}

func TestDriverPublicStewardSkipsBlockedWork(t *testing.T) {
	t.Parallel()
	for _, damage := range []string{"refusal", "entry", "worktree"} {
		t.Run(damage, func(t *testing.T) {
			t.Parallel()
			bed := driverBed(t)
			file := bed.goalFile(bed.id)
			workApprovedBox(file)
			file.Budget.ActiveJobLimit, file.Budget.ReservedJobMinutesLimit = 4, 3000
			file.Approved.Digest = goal.ApprovalDigest(file.Intent, file.Tier, *file.Budget, file.Risk)
			bed.addGoal(file)
			bed.starter.hold = "build"
			first := driverBuild(t, bed, "z-oldest")
			bed.manager.Sleep(time.Second)
			other := &workBed{intentBed: bed.intentBed, id: bed.id, worktree: filepath.Join(filepath.Dir(bed.worktree), "other-work"), manager: bed.manager, starter: bed.starter,
				unitRoot: bed.unitRoot, branchListed: true, head: bed.head, designGate: bed.designGate, workOwnersHook: bed.workOwnersHook, readDirs: map[string]bool{}}
			if err := os.MkdirAll(other.worktree, 0700); err != nil {
				t.Fatal(err)
			}
			second := driverBuild(t, other, "a-newest")
			driverFinish(t, bed, "build")
			bed.starter.hold = "proof"
			atHelm := false
			owners := driverOwners(t, bed, "auto", &atHelm)
			units := owners.work.units
			owners.work.units = func(layout stateroot.Layout) *launch.UnitRunner {
				runner := units(layout)
				runner.Git = driverGit{bed, other}
				return runner
			}
			git := owners.work.git
			owners.work.git = func(root string, args ...string) ([]byte, error) {
				data, err := git(root, args...)
				if slices.Equal(args, []string{"worktree", "list", "--porcelain"}) && err == nil {
					data = append(data, []byte("\nworktree "+other.worktree+"\nHEAD "+other.head+"\nbranch refs/heads/goal/"+other.id+"\n")...)
				}
				return data, err
			}
			expectedReason := ""
			switch damage {
			case "refusal":
				record, err := (&launch.UnitRunner{Root: bed.unitRoot}).Status(first)
				if err != nil {
					t.Fatal(err)
				}
				var plan struct{ Build struct{ Brief string } }
				if err := json.Unmarshal(mustRead(t, record.Plan), &plan); err != nil {
					t.Fatal(err)
				}
				brief := plan.Build.Brief
				if !filepath.IsAbs(brief) {
					brief = filepath.Join(record.PlanDirectory, brief)
				}
				if err := os.WriteFile(brief, append(mustRead(t, brief), []byte("\nedited after start\n")...), 0600); err != nil {
					t.Fatal(err)
				}
				expectedReason = "inputs or settings"
			case "entry":
				entries, err := filepath.Glob(filepath.Join(bed.unitRoot, ".named", "*.json"))
				if err != nil || len(entries) != 2 {
					t.Fatalf("entries: %v %v", entries, err)
				}
				var entry map[string]any
				if err := json.Unmarshal(mustRead(t, entries[0]), &entry); err != nil {
					t.Fatal(err)
				}
				if entry["run"] != first {
					entries[0] = entries[1]
				}
				if err := os.WriteFile(entries[0], []byte("damaged"), 0600); err != nil {
					t.Fatal(err)
				}
				expectedReason = "saved unit record is damaged"
			case "worktree":
				if err := os.RemoveAll(bed.worktree); err != nil {
					t.Fatal(err)
				}
				expectedReason = bed.worktree
			}
			before := len(bed.starter.launched())
			driveErr := driverStewardCycleResult(t, bed, owners)
			if driveErr == nil || !strings.Contains(driveErr.Error(), expectedReason) {
				t.Fatalf("damage unreported: %v", driveErr)
			}
			current, err := (&launch.UnitRunner{Root: bed.unitRoot}).Status(second)
			if err != nil || current.Rounds[0].Steps[0].State != launch.StepPassed || len(current.Rounds[0].Steps) < 2 || current.Rounds[0].Steps[1].State != launch.StepRunning || len(bed.starter.launched()) != before+1 {
				t.Fatalf("younger run not collected and started in one tick: %v steps=%+v launches=%v diagnostic=%v", err, current.Rounds[0].Steps, bed.starter.launched(), driveErr)
			}
			if damage == "refusal" {
				code, status := driverVerb(t, bed, owners, "work", "status", bed.id, "--work", "z-oldest")
				if code != 0 || !strings.Contains(resultWords(status), expectedReason) {
					t.Fatalf("refusal invisible in status: %d %+v", code, status)
				}
			}
		})
	}
}
