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

	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel/phase"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/hostload"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lock"
)

func TestFleetBuildPublicAdmission(t *testing.T) {
	t.Parallel()
	t.Run("kinds and shared lock", func(t *testing.T) {
		t.Parallel()
		bed := newWorkBed(t)
		home, owner := t.TempDir(), t.TempDir()
		registerLane(t, home, owner, "Wido", bed.manager.Now())
		bed.manager.CapacityHome = home
		conf := filepath.Join(t.TempDir(), "settings.conf")
		if err := os.WriteFile(conf, []byte("host.builds=1\nhost.load-max=8\n"), 0600); err != nil {
			t.Fatal(err)
		}
		bed.manager.BuildPolicy = config.GetParams{ConfPath: conf, LookupEnv: func(string) (string, bool) { return "", false }}
		brief := filepath.Join(bed.root(), "sized.md")
		if err := os.WriteFile(brief, []byte("| Unit | Lines |\n| --- | --- |\n| build | 5 |\n"), 0600); err != nil {
			t.Fatal(err)
		}
		bed.starter.hold = "build"
		observing, release := make(chan struct{}), make(chan struct{})
		bed.manager.CapacitySources.Load = func(at time.Time) hostload.Sample {
			close(observing)
			<-release
			return hostload.Sample{Available: true, Load1m: 0}
		}
		first := make(chan error, 1)
		go func() {
			_, err := bed.manager.Start(launch.StartSpec{ID: "first", Kind: "build", Goal: "first-goal", WorkingDirectory: bed.worktree, Brief: brief})
			first <- err
		}()
		<-observing
		held, err := lock.File(filepath.Join(bed.manager.Store.Root, "build-admission.lock"), 0600, lock.TryExclusive)
		if err == nil {
			_ = held.Release()
			close(release)
			<-first
			t.Fatal("host observation was outside the admission lock")
		}
		if !lock.Busy(err) {
			close(release)
			<-first
			t.Fatal(err)
		}
		_, err = bed.manager.Start(launch.StartSpec{ID: "second", Kind: "build", Goal: "second-goal", WorkingDirectory: bed.worktree, Brief: brief})
		close(release)
		if firstErr := <-first; firstErr != nil {
			t.Fatal(firstErr)
		}
		if !launch.IsCode(err, "LAUNCH_BUILD_CAPACITY") {
			t.Fatalf("contending build: %v", err)
		}
		bed.manager.CapacitySources.Load = func(time.Time) hostload.Sample { return hostload.Sample{Available: true} }
		_, err = bed.manager.Start(launch.StartSpec{ID: "second", Kind: "build", Goal: "second-goal", WorkingDirectory: bed.worktree, Brief: brief})
		if !launch.IsCode(err, "LAUNCH_BUILD_CAPACITY") {
			t.Fatalf("running build did not fill host.builds=1: %v", err)
		}
		if _, err := bed.manager.Store.Read("second"); !os.IsNotExist(err) {
			t.Fatalf("second refusal wrote a record: %v", err)
		}
		bed.manager.CapacitySources.Load = func(time.Time) hostload.Sample {
			t.Error("non-build sampled host admission")
			return hostload.Sample{Detail: "unreadable"}
		}
		diff := filepath.Join(t.TempDir(), "empty.diff")
		if err := os.WriteFile(diff, nil, 0600); err != nil {
			t.Fatal(err)
		}
		bed.manager.Settings.SeatRuntime = "codex"
		bed.manager.Settings.LandingModel = "fixture-model"
		bed.manager.Lane = func() (launch.LaneCheckout, error) {
			return launch.LaneCheckout{Registered: true, Checkout: bed.worktree, Module: bed.root()}, nil
		}
		for _, kind := range []string{"read", "design", "landing", "seat"} {
			bed.starter.hold = kind
			dir, fence := bed.worktree, bed.root()
			if kind == "seat" {
				dir = t.TempDir()
				fence = dir
			}
			_, err := bed.manager.Start(launch.StartSpec{ID: kind, Kind: kind, Goal: "non-build", Tag: "test", WorkingDirectory: dir, FenceRoot: fence, Brief: brief, DiffFile: diff})
			if err != nil {
				t.Fatalf("%s was held by build admission: %v", kind, err)
			}
		}
	})

	for _, row := range []struct {
		name, policy, limit, activeGoal                            string
		load                                                       float64
		person, unknown, absentOwner, changedOwner, corrupt, allow bool
	}{
		{name: "above", policy: "auto", limit: "8", load: 9},
		{name: "at", policy: "auto", limit: "8", load: 8},
		{name: "below", policy: "auto", limit: "8", load: 7, allow: true},
		{name: "defaults", load: 0, allow: true},
		{name: "person bypass", policy: "1", limit: "8", load: 9, person: true, activeGoal: "other", allow: true},
		{name: "person unknown", policy: "person", limit: "bad", person: true, unknown: true, allow: true},
		{name: "cap", policy: "1", limit: "8", activeGoal: "other"},
		{name: "unassigned", policy: "1", limit: "8", activeGoal: "unassigned"},
		{name: "same goal", policy: "1", limit: "8", activeGoal: "same", allow: true},
		{name: "large cap", policy: "99999999999999999999999", limit: "8", activeGoal: "other", allow: true},
		{name: "person asks", policy: "person", limit: "8"},
		{name: "unknown load", policy: "auto", limit: "8", unknown: true},
		{name: "absent owner", policy: "auto", limit: "8", absentOwner: true},
		{name: "changed owner", policy: "auto", limit: "8", changedOwner: true},
		{name: "corrupt builds", policy: "auto", limit: "8", corrupt: true},
		{name: "invalid limit", policy: "auto", limit: "NaN"},
		{name: "zero limit", policy: "auto", limit: "0"},
		{name: "invalid policy", policy: "0", limit: "8"},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			bed := newWorkBed(t)
			if err := os.MkdirAll(filepath.Join(bed.root(), ".git"), 0700); err != nil {
				t.Fatal(err)
			}
			home, owner := t.TempDir(), t.TempDir()
			registerLane(t, home, owner, "Wido", bed.manager.Now())
			bed.manager.CapacityHome = home
			load := row.load
			bed.manager.CapacitySources.Load = func(at time.Time) hostload.Sample {
				if row.changedOwner {
					_ = os.Remove(lane.RecordPath(home))
				}
				return hostload.Sample{At: at.Format(time.RFC3339Nano), Load1m: load, Available: !row.unknown, Detail: "synthetic kernel read", Cores: 12}
			}
			if row.absentOwner {
				if err := os.Remove(lane.RecordPath(home)); err != nil {
					t.Fatal(err)
				}
			}
			if row.activeGoal != "" || row.corrupt {
				goal := row.activeGoal
				if goal == "same" {
					goal = bed.id
				}
				if goal == "unassigned" {
					goal = ""
				}
				if err := bed.manager.Store.Create(launch.Record{ID: "reserved", Kind: "build", Goal: goal, State: launch.Starting}); err != nil {
					t.Fatal(err)
				}
				if row.corrupt {
					if err := os.WriteFile(filepath.Join(bed.manager.Store.Root, "reserved", "record.json"), []byte("{"), 0600); err != nil {
						t.Fatal(err)
					}
				}
			}
			conf := filepath.Join(bed.root(), "metasystem.conf")
			original, err := os.ReadFile(conf)
			if err != nil {
				t.Fatal(err)
			}
			settings := string(original)
			if row.policy != "" {
				settings += "\nhost.builds=" + row.policy + "\n"
			}
			if row.limit != "" {
				settings += "host.load-max=" + row.limit + "\n"
			}
			if err := os.WriteFile(conf, []byte(settings), 0600); err != nil {
				t.Fatal(err)
			}
			owners := bed.workOwners()
			owners.lookupEnv = func(string) (string, bool) { return "", false }
			owners.policies.Registry = func(string) (config.PolicyRegistry, error) { return config.PolicyRegistry{}, nil }
			// No lineage and no human proof is an agent, including copied argv.
			owners.prove = func(string, int64, humanauthority.Reader, string, string, time.Time) (humanauthority.Proof, error) {
				return humanauthority.Proof{}, errors.New("agent process")
			}
			if row.person {
				owners.prove = enrolledPersonProver(t, bed.root(), bed.manager.Now())
			}
			owners.processes.ask = func(root string, in channelAskInput) (channel.Question, []string, int, error) {
				return askChannelQuestionVia(root, in, channelAskSurface{
					identity: func(string) (string, string, error) { return "fixture-machine", "fixture-lineage", nil },
					load:     func(string) (phase.Loaded, error) { return phase.Loaded{}, nil },
					cursor:   func(string) (string, bool, error) { return "", false, nil },
				})
			}
			brief := bed.brief("build.md", "Build this unit.\n")
			run := func(name string) (int, intentResult) {
				t.Helper()
				command, args, _ := resolveIntentArgv([]string{"work", "build", bed.id, "--work", name, "--brief", brief, "--lines", "5", "--json", "--check", "fixture-check"})
				var out, stderr bytes.Buffer
				code := runIntentIn(command, args, &out, &stderr, bed.root(), owners)
				var result intentResult
				if err := json.Unmarshal(out.Bytes(), &result); err != nil {
					t.Fatalf("exit %d: %s %s: %v", code, out.String(), stderr.String(), err)
				}
				bed.recordReadDirs(result)
				return code, result
			}
			if row.name == "person bypass" {
				root, personProof := bed.manager.Store.Root, owners.prove
				blocked := filepath.Join(t.TempDir(), "not-a-directory")
				if err := os.WriteFile(blocked, nil, 0600); err != nil {
					t.Fatal(err)
				}
				bed.manager.Store.Root = blocked
				code, interrupted := run("first")
				if code != 1 || len(bed.starter.launched()) != 0 {
					t.Fatalf("interrupted reservation: %d %+v", code, interrupted)
				}
				bed.manager.Store.Root = root
				owners.prove = func(string, int64, humanauthority.Reader, string, string, time.Time) (humanauthority.Proof, error) {
					return humanauthority.Proof{}, errors.New("automatic resume")
				}
				id := resultData(t, interrupted)["run"].(string)
				command, args, _ := resolveIntentArgv([]string{"work", "build", "run:" + id, "--json"})
				var out, stderr bytes.Buffer
				code = runIntentIn(command, args, &out, &stderr, bed.root(), owners)
				var resumed intentResult
				if err := json.Unmarshal(out.Bytes(), &resumed); err != nil {
					t.Fatalf("resume exit %d: %s %s: %v", code, out.String(), stderr.String(), err)
				}
				if code != 1 || resumed.Outcome != intentRefused || !strings.Contains(resumed.Summary, "host.load-max") || len(bed.starter.launched()) != 0 {
					t.Fatalf("automatic resume inherited the person's authority: %d %+v", code, resumed)
				}
				owners.prove = personProof
			}
			code, result := run("first")
			launched := bed.starter.launched()
			if row.allow {
				if code != 0 || len(launched) == 0 || launched[0] != "build" {
					t.Fatalf("admitted build: %d %+v launches=%v", code, result, launched)
				}
				if row.name == "defaults" {
					if config.MustDefault("host.load-max") != "8" || config.MustDefault("host.builds") != "auto" {
						t.Fatal("missing compiled declaration")
					}
					load = 9
					if code, held := run("fresh"); code != 1 || held.Outcome != intentRefused {
						t.Fatalf("fresh load ignored: %d %+v", code, held)
					}
				}
				if row.person {
					owners.prove = func(string, int64, humanauthority.Reader, string, string, time.Time) (humanauthority.Proof, error) {
						return humanauthority.Proof{}, errors.New("automatic resume")
					}
					if code, held := run("automatic"); code != 1 || held.Outcome != intentRefused {
						t.Fatalf("inherited person actor: %d %+v", code, held)
					}
				}
				return
			}
			if code != 1 || result.Outcome != intentRefused || len(launched) != 0 {
				t.Fatalf("capacity is an executed failure: %d %+v launches=%v", code, result, launched)
			}
			records, err := bed.manager.Store.List()
			if !row.corrupt && err != nil {
				t.Fatal(err)
			}
			for _, record := range records {
				if record.ID != "reserved" {
					t.Fatalf("refusal wrote a launch: %+v", record)
				}
			}
			if row.name == "above" || row.name == "at" {
				for _, fact := range []string{"host.load-max=8", "limit=8", "load="} {
					if !strings.Contains(result.Summary, fact) {
						t.Fatalf("refusal hides %s: %+v", fact, result)
					}
				}
				if result.Next == nil || !strings.Contains(result.Next.Reason, "person") {
					t.Fatalf("missing person remedy: %+v", result)
				}
			}
			if row.policy == "person" {
				id := targetID(result.Targets, "question", "")
				q, err := channel.ReadQuestion(bed.root(), id)
				if err != nil || q.State != "open" || q.Kind != "other" || !strings.Contains(strings.Join(q.Facts, " "), "host.builds=person") {
					t.Fatalf("person policy did not ask: %+v %v result=%+v", q, err, result)
				}
			}
			// The starting step remains waiting in its original round, without a launch or correction.
			entries, err := os.ReadDir(bed.unitRoot)
			if err != nil {
				t.Fatal(err)
			}
			retained := 0
			for _, entry := range entries {
				if !entry.IsDir() || entry.Name() == "named" {
					continue
				}
				runner := &launch.UnitRunner{Root: bed.unitRoot}
				record, err := runner.Status(entry.Name())
				if err != nil {
					continue
				}
				retained++
				if len(record.Rounds) != 1 || record.Rounds[0].Outcome != "" || record.Rounds[0].Steps[0].State != launch.StepStarting {
					t.Fatalf("hold consumed an execution or correction: %+v", record)
				}
			}
			if retained != 1 {
				t.Fatalf("refusal retained %d runs, want one waiting run", retained)
			}
			if row.name == "above" {
				owners.prove = enrolledPersonProver(t, bed.root(), bed.manager.Now())
				if code, resumed := run("first"); code != 0 {
					t.Fatalf("person cannot follow the exact refusal remedy: %d %+v", code, resumed)
				}
			}

		})
	}
}

func TestFleetBuildPolicyFallback(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name, settings              string
		load                        float64
		allow, registry, unreadable bool
	}{
		{name: "conf without repository", settings: "host.builds=auto\nhost.load-max=20\n", load: 9, allow: true},
		{name: "default without repository", allow: true},
		{name: "default still holds", load: 9},
		{name: "conf person still holds", settings: "host.builds=person\n"},
		{name: "unreadable registry", registry: true, settings: "host.builds=auto\nhost.load-max=20\n", load: 9, allow: true},
		{name: "malformed conf", unreadable: true},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			bed := newWorkBed(t)
			bed.manager.CapacitySources.Load = func(time.Time) hostload.Sample {
				return hostload.Sample{Available: true, Load1m: row.load}
			}
			conf := filepath.Join(bed.root(), "metasystem.conf")
			data, err := os.ReadFile(conf)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(conf, append(data, []byte("\n"+row.settings)...), 0600); err != nil {
				t.Fatal(err)
			}
			if row.unreadable {
				if err := os.WriteFile(conf, []byte("host.builds=auto\nhost.builds=person\n"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			owners := bed.workOwners()
			owners.lookupEnv = func(string) (string, bool) { return "", false }
			owners.prove = func(string, int64, humanauthority.Reader, string, string, time.Time) (humanauthority.Proof, error) {
				return humanauthority.Proof{}, errors.New("agent process")
			}
			if row.registry {
				if err := os.MkdirAll(filepath.Join(bed.root(), ".git"), 0700); err != nil {
					t.Fatal(err)
				}
				home := t.TempDir()
				if err := os.MkdirAll(filepath.Dir(lane.RecordPath(home)), 0700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(lane.RecordPath(home), []byte("{"), 0600); err != nil {
					t.Fatal(err)
				}
				owners.policies.Registry = func(string) (config.PolicyRegistry, error) {
					_, _, _, err := lane.ReadGuarded(home)
					return config.PolicyRegistry{}, &config.PolicyReadError{Source: "lane", Err: err}
				}
			}
			owners.processes.ask = func(root string, in channelAskInput) (channel.Question, []string, int, error) {
				return askChannelQuestionVia(root, in, channelAskSurface{
					identity: func(string) (string, string, error) { return "fixture-machine", "fixture-lineage", nil },
					load:     func(string) (phase.Loaded, error) { return phase.Loaded{}, nil },
					cursor:   func(string) (string, bool, error) { return "", false, nil },
				})
			}
			brief := bed.brief("fallback.md", "Build this unit.\n")
			code, result := fleetWorkJSON(t, bed, owners, "work", "build", bed.id, "fallback", "--brief", brief, "--lines", "5", "--check", "fixture-check")
			if row.allow {
				if code != 0 || len(bed.starter.launched()) == 0 {
					t.Fatalf("fallback did not launch: %d %+v", code, result)
				}
			} else if code != 1 || result.Outcome != intentRefused || len(bed.starter.launched()) != 0 {
				t.Fatalf("fallback bypassed admission: %d %+v", code, result)
			}
		})
	}
}

func TestFleetRevisePublicAdmission(t *testing.T) {
	t.Parallel()
	for _, target := range []string{"goal", "run"} {
		for _, policy := range []string{"load", "cap", "person"} {
			for _, person := range []bool{false, true} {
				name := "agent"
				if person {
					name = "person"
				}
				t.Run(target+"/"+policy+"/"+name, func(t *testing.T) {
					t.Parallel()
					bed := newWorkBed(t)
					owners := bed.workOwners()
					owners.lookupEnv = func(string) (string, bool) { return "", false }
					personProof := enrolledPersonProver(t, bed.root(), bed.manager.Now())
					owners.prove = personProof
					brief := bed.brief("first.md", "Build this unit.\n")
					code, built := fleetWorkJSON(t, bed, owners, "work", "build", bed.id, "correction", "--brief", brief, "--lines", "5", "--check", "fixture-check")
					if code != 0 {
						t.Fatalf("seed build: %d %+v", code, built)
					}
					conf := filepath.Join(bed.root(), "metasystem.conf")
					data, err := os.ReadFile(conf)
					if err != nil {
						t.Fatal(err)
					}
					settings := "host.builds=auto\nhost.load-max=8\n"
					if policy == "load" {
						bed.manager.CapacitySources.Load = func(time.Time) hostload.Sample {
							return hostload.Sample{Available: true, Load1m: 9}
						}
					} else if policy == "cap" {
						settings = "host.builds=1\nhost.load-max=8\n"
						if err := bed.manager.Store.Create(launch.Record{ID: "other", Kind: "build", Goal: "other", State: launch.Starting}); err != nil {
							t.Fatal(err)
						}
					} else {
						settings = "host.builds=person\nhost.load-max=8\n"
					}
					if err := os.WriteFile(conf, append(data, []byte("\n"+settings)...), 0600); err != nil {
						t.Fatal(err)
					}
					if !person {
						owners.prove = func(string, int64, humanauthority.Reader, string, string, time.Time) (humanauthority.Proof, error) {
							return humanauthority.Proof{}, errors.New("agent process")
						}
					}
					owners.processes.ask = func(root string, in channelAskInput) (channel.Question, []string, int, error) {
						return askChannelQuestionVia(root, in, channelAskSurface{
							identity: func(string) (string, string, error) { return "fixture-machine", "fixture-lineage", nil },
							load:     func(string) (phase.Loaded, error) { return phase.Loaded{}, nil },
							cursor:   func(string) (string, bool, error) { return "", false, nil },
						})
					}
					correction := bed.brief("correction.md", "Correct this unit.\n")
					argv := []string{"work", "revise", bed.id, "--work", "correction", "--brief", correction}
					if target == "run" {
						argv = []string{"work", "revise", "run:" + resultData(t, built)["run"].(string), "--brief", correction}
					}
					before := len(bed.starter.launched())
					code, result := fleetWorkJSON(t, bed, owners, argv...)
					if person {
						if code != 0 || len(bed.starter.launched()) <= before || bed.starter.launched()[before] != "build" {
							t.Fatalf("person's correction did not build: %d %+v launches=%v", code, result, bed.starter.launched())
						}
					} else {
						if code != 1 || result.Outcome != intentRefused || len(bed.starter.launched()) != before || !strings.Contains(result.Summary, "host.") {
							t.Fatalf("agent's correction was not held: %d %+v launches=%v", code, result, bed.starter.launched())
						}
						owners.prove = personProof
						if target == "run" {
							argv = []string{"work", "build", "run:" + resultData(t, built)["run"].(string)}
							if policy != "person" && (result.Next == nil || strings.Join(result.Next.Argv, " ") != "metasystem "+strings.Join(argv, " ")) {
								t.Fatalf("held correction has no working resume remedy: %+v", result)
							}
							if policy == "person" {
								q, err := channel.ReadQuestion(bed.root(), targetID(result.Targets, "question", ""))
								if err != nil || !strings.Contains(strings.Join(q.Facts, " "), strings.Join(argv, " ")) {
									t.Fatalf("person question hides the resume command: %+v %v", q, err)
								}
							}
						}
						code, resumed := fleetWorkJSON(t, bed, owners, argv...)
						if code != 0 || len(bed.starter.launched()) <= before {
							t.Fatalf("person could not repeat held correction: %d %+v", code, resumed)
						}
						runner := &launch.UnitRunner{Root: bed.unitRoot}
						record, err := runner.Status(resultData(t, built)["run"].(string))
						if err != nil || len(record.Rounds) != 2 {
							t.Fatalf("resume spent another correction round: %+v %v", record, err)
						}
					}
				})
			}
		}
	}
}

func TestFleetBuildGrantProbeDoesNotRefuse(t *testing.T) {
	t.Parallel()
	bed := newWorkBed(t)
	owners := bed.workOwners()
	owners.lookupEnv = func(string) (string, bool) { return "", false }
	owners.prove = func(root string, _ int64, _ humanauthority.Reader, _, _ string, at time.Time) (humanauthority.Proof, error) {
		return humanauthority.HelmProof(root, humanauthority.HelmGrant{By: "Wido", Grant: "fixture-grant"}, at)
	}
	brief := bed.brief("grant.md", "Build this unit.\n")
	code, result := fleetWorkJSON(t, bed, owners, "work", "build", bed.id, "grant", "--brief", brief, "--lines", "5", "--check", "fixture-check")
	if code != 0 || len(bed.starter.launched()) == 0 {
		t.Fatalf("grant build: %d %+v", code, result)
	}
	before := len(bed.starter.launched())
	bed.manager.CapacitySources.Load = func(time.Time) hostload.Sample {
		return hostload.Sample{Available: true, Load1m: 9}
	}
	code, held := fleetWorkJSON(t, bed, owners, "work", "build", bed.id, "grant-held", "--brief", brief, "--lines", "5", "--check", "fixture-check")
	if code != 1 || held.Outcome != intentRefused || len(bed.starter.launched()) != before {
		t.Fatalf("grant inherited the person's bypass: %d %+v", code, held)
	}
	data, err := os.ReadFile(humanauthority.AttorneyLogPath(bed.root()))
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "refused") {
		t.Fatalf("admitted build logged a refusal: %s", data)
	}
}

func fleetWorkJSON(t *testing.T, bed *workBed, owners intentOwners, argv ...string) (int, intentResult) {
	t.Helper()
	command, args, ok := resolveIntentArgv(argv)
	if !ok {
		t.Fatalf("no public command %q", argv)
	}
	var out, stderr bytes.Buffer
	code := runIntentIn(command, append([]string{"--json"}, args...), &out, &stderr, bed.root(), owners)
	var result intentResult
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("exit %d: %s %s: %v", code, out.String(), stderr.String(), err)
	}
	bed.recordReadDirs(result)
	return code, result
}
