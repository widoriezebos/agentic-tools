package up

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/enginepin"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/state"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/supervise"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/up/testdata/startfixture"
)

func generationStartBed(t *testing.T, mode string) (Options, string, string) {
	t.Helper()
	options := seatUpOptions(t)
	binary, digest := startfixture.Stage(t, options.Root)
	options.Binary = binary
	options.Executions = startfixture.Executions(t, options.Root, binary, nil)
	home := t.TempDir()
	options.LaneHome = func() (string, error) { return home, nil }
	options.RepairStewardRunner = currentRunnerRepair
	options.IfDown = true
	options.RecoverOnly = mode == "recovery"
	return options, home, digest
}

func generationStart(options Options) Result {
	if options.RecoverOnly {
		return recovery(options)
	}
	result, _ := ordinaryBody(options)
	return result
}

func assertGenerationObservation(t *testing.T, root, target, verdict, attempt string, observations map[string]enginepin.Execution) {
	t.Helper()
	record, found, err := state.Read(plain.Dir(root))
	if err != nil || !found {
		t.Fatalf("generation record missing: found=%t error=%v", found, err)
	}
	events, skipped, err := state.ReadEvents(plain.Dir(root))
	if err != nil || skipped != 0 {
		t.Fatalf("events: %v skipped=%d", err, skipped)
	}
	var generations []state.Event
	for _, event := range events {
		if event.Kind == "generation" {
			generations = append(generations, event)
		}
	}
	if len(generations) != 1 {
		t.Fatalf("want exactly one generation event, got %d: %+v", len(generations), events)
	}
	want := &state.Generation{Target: target, Verdict: verdict, Attempt: attempt}
	for _, role := range []string{"owner", "watcher", "reaper", "runner"} {
		e := observations[role]
		want.Roles = append(want.Roles, state.GenerationRole{Role: role, Digest: e.Digest, Generation: e.Generation, Source: e.Source, Cause: e.Cause})
	}
	if !reflect.DeepEqual(record.Generation, want) || !reflect.DeepEqual(generations[0].Generation, want) || generations[0].To != verdict {
		t.Fatalf("observation differs: record=%+v event=%+v want=%+v", record.Generation, generations[0], want)
	}
}

func TestStartRecordsTheGenerationEvent(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"ordinary", "recovery"} {
		for _, bed := range []string{"registered-without-state", "registered-with-state", "non-lane", "other-install", "malformed-state", "append-failure", "registration-error", "lock-failure"} {
			t.Run(mode+"/"+bed, func(t *testing.T) {
				t.Parallel()
				options, home, digest := generationStartBed(t, mode)
				dir := plain.Dir(options.Root)
				if bed != "non-lane" {
					registered := options.Root
					if bed == "other-install" {
						registered = t.TempDir()
					}
					startfixture.Register(t, home, registered)
				}
				if bed == "registered-with-state" || bed == "append-failure" || bed == "lock-failure" {
					if _, err := state.Transition(dir, time.Unix(1800000000, 0), "lane", "", "running", "fixture", "", func(r *state.Record) { r.Lane.Word = "running" }); err != nil {
						t.Fatal(err)
					}
				}
				if bed == "malformed-state" {
					startfixture.JSON(t, filepath.Join(dir, "state.json"), "not a record")
				}
				if bed == "append-failure" {
					if err := os.Remove(filepath.Join(dir, "events.jsonl")); err != nil {
						t.Fatal(err)
					}
					if err := os.Mkdir(filepath.Join(dir, "events.jsonl"), 0700); err != nil {
						t.Fatal(err)
					}
				}
				if bed == "lock-failure" {
					if err := os.Remove(filepath.Join(dir, "state.lock")); err != nil {
						t.Fatal(err)
					}
					if err := os.Mkdir(filepath.Join(dir, "state.lock"), 0700); err != nil {
						t.Fatal(err)
					}
				}
				if bed == "registration-error" {
					if err := os.WriteFile(lane.RecordPath(home), []byte("{"), 0600); err != nil {
						t.Fatal(err)
					}
				}
				observed := options.Executions(options.Root)
				result := generationStart(options)
				switch bed {
				case "non-lane", "other-install":
					if result.ExitCode() != 0 {
						t.Fatalf("non-lane start: %+v", result)
					}
					if _, err := os.Stat(dir); !os.IsNotExist(err) {
						t.Fatalf("non-lane created landing directory: %v", err)
					}
				case "malformed-state", "append-failure", "registration-error", "lock-failure":
					lines := strings.Join(result.Lines(), "\n")
					want := map[string]string{"malformed-state": "read " + filepath.Join(dir, "state.json"), "append-failure": "events.jsonl", "registration-error": "unreadable", "lock-failure": "state.lock"}[bed]
					if result.ExitCode() != 1 || !strings.Contains(lines, "could not save which engine each process runs") || !strings.Contains(lines, want) {
						t.Fatalf("recording failure lost (%s): %+v\n%s", want, result, lines)
					}
				default:
					if result.ExitCode() != 0 {
						t.Fatalf("current start: %+v", result)
					}
					assertGenerationObservation(t, options.Root, digest, "one generation", "", observed)
				}
			})
		}
	}
}

func TestPartialStartRecordsTheFinalObservation(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"ordinary", "recovery"} {
		for _, fault := range []string{"runner", "refusal", "readiness", "runner-and-recording"} {
			t.Run(mode+"/"+fault, func(t *testing.T) {
				t.Parallel()
				options, home, digest := generationStartBed(t, mode)
				startfixture.Register(t, home, options.Root)
				options.Executions = startfixture.Executions(t, options.Root, options.Binary, map[string]string{"runner": "unknown"})
				observed := options.Executions(options.Root)
				arms, runners := 0, 0
				options.EnsureArmed = func(supervise.EnsureOptions) (supervise.EnsureResult, error) {
					arms++
					if fault == "refusal" {
						return supervise.EnsureResult{}, errors.New("owner unverifiable")
					}
					r := supervise.EnsureResult{Action: "replaced"}
					if fault == "readiness" {
						r.Inspection = supervise.ArmedInspection{Component: "repo-watcher", Reason: "watcher not ready"}
					}
					return r, nil
				}
				options.EnsureStewardRunner = func(string, *steward.EnrolledBinary) (steward.EnsureRunnerResult, error) {
					runners++
					return steward.EnsureRunnerResult{}, errors.New("steward runner execution is unknown")
				}
				options.RepairStewardRunner = func(string) (steward.RunnerRepairOutcome, error) {
					runners++
					return steward.RunnerRepairOutcome{}, errors.New("steward runner execution is unknown")
				}
				if fault == "runner-and-recording" {
					startfixture.JSON(t, filepath.Join(plain.Dir(options.Root), "state.json"), "broken")
				}
				result := generationStart(options)
				want, launches := "steward-runner", 1
				if fault == "refusal" {
					want, launches = "supervision-owner", 0
				}
				if fault == "readiness" {
					want, launches = "repo-watcher", 0
				}
				if result.ExitCode() != 1 || result.Failed != want || arms != 1 || runners != launches {
					t.Fatalf("partial start: %+v arms=%d runners=%d", result, arms, runners)
				}
				if mode == "recovery" && result.Outcome != "recovery-partial" {
					t.Fatalf("recovery outcome replaced: %+v", result)
				}
				if fault == "runner-and-recording" {
					lines := component(result, "steward-runner").Detail
					if !strings.Contains(lines, "steward runner execution is unknown") || !strings.Contains(lines, "could not save which engine each process runs") {
						t.Fatalf("original or recording error lost: %s", lines)
					}
				} else {
					assertGenerationObservation(t, options.Root, digest, "unverifiable", "", observed)
				}
			})
		}
	}
}

func TestGenerationSampleIsReadOnce(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"ordinary", "recovery", "deferred"} {
		t.Run(mode, func(t *testing.T) {
			t.Parallel()
			options, home, digest := generationStartBed(t, mode)
			startfixture.Register(t, home, options.Root)
			first := options.Executions(options.Root)
			second := make(map[string]enginepin.Execution)
			for role, execution := range first {
				second[role] = execution
			}
			second["runner"] = enginepin.Execution{Cause: "second answer is unknown"}
			reads := 0
			options.Executions = func(string) map[string]enginepin.Execution {
				reads++
				if reads == 1 {
					return first
				}
				return second
			}
			if mode == "deferred" {
				options.EnsureArmed = func(supervise.EnsureOptions) (supervise.EnsureResult, error) {
					return supervise.EnsureResult{Action: "deferred", Attempt: "proof-148"}, nil
				}
			}
			result := generationStart(options)
			verdict, attempt, exit := "one generation", "", 0
			if mode == "deferred" {
				verdict, attempt, exit = "deferred", "proof-148", 1
			}
			if reads != 1 {
				t.Errorf("generation sample: reads=%d, want exactly one", reads)
			}
			if result.ExitCode() != exit {
				t.Fatalf("generation sample verdict: %+v", result)
			}
			if mode != "deferred" && component(result, "engine-generation").Outcome != "verified" {
				t.Fatalf("first sample was not judged: %+v", result)
			}
			if mode == "deferred" && !strings.Contains(strings.Join(result.Lines(), "\n"), "proof-148") {
				t.Fatalf("deferred verdict lost: %+v", result)
			}
			assertGenerationObservation(t, options.Root, digest, verdict, attempt, first)
		})
	}
}
