package up

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/enginepin"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/enginepin/testdata/pinfixture"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/registry"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/supervise"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/supervise/testdata/ownerfixture"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/up/testdata/startfixture"
)

func upOwnerHelper(args []string) error {
	exact, err := ownerfixture.AwaitPublication(args)
	if err != nil {
		return err
	}
	root := ownerfixture.Argument(args, "--repo")
	tag := ownerfixture.Argument(args, "--tag")
	interval, _ := strconv.Atoi(ownerfixture.Argument(args, "--interval"))
	cap, _ := strconv.Atoi(ownerfixture.Argument(args, "--watcher-cap"))
	fingerprint := ownerfixture.Argument(args, "--fingerprint")
	checkout := supervise.DiskCheckout{Root: root, Self: exact.Ref(), SelfTag: tag, IntervalSec: interval, WatcherCap: cap, Fingerprint: fingerprint}
	generation := checkout.PriorGeneration() + 1
	held := []supervise.Held{}
	for _, role := range []supervise.Component{supervise.Watcher, supervise.Reaper} {
		ref, err := ownerfixture.Component("--up-component-helper", tag+"-"+string(role)+"-1")
		if err != nil {
			return err
		}
		held = append(held, supervise.Held{Component: role, Identity: ref, Tag: tag + "-" + string(role) + "-1", Generation: generation})
	}
	if err := checkout.PublishState(held); err != nil {
		return err
	}
	ledger := supervise.RegistryLedger{CheckoutPath: root, OwnerTag: tag, Append: func(row map[string]any) error {
		data, err := supervise.EncodeRecord(row)
		if err != nil {
			return err
		}
		return registry.AppendFrame(filepath.Join(os.Getenv("METASYSTEM_SUPERVISION_REGISTRY_HOME"), ".metasystem", "armed-checkouts.jsonl"), data)
	}}
	if err := ledger.AppendRelaunched(generation, held[0].Tag, held[1].Tag, 0); err != nil {
		return err
	}
	for _, member := range held {
		if err := ledger.AppendLaunched(member); err != nil {
			return err
		}
	}
	now := time.Now().Unix()
	records := map[string]any{"watcher.heartbeat.json": map[string]any{"observedAtEpoch": now, "loadedCapMin": cap}, "reaper.heartbeat.json": map[string]any{"observedAtEpoch": now}, "last-census.json": map[string]any{"verdict": "SUCCESS", "fingerprint": fingerprint, "generation": generation, "completedAtEpoch": now}}
	for name, value := range records {
		data, err := json.Marshal(value)
		if err != nil {
			return err
		}
		if err = os.WriteFile(filepath.Join(supervise.SupervisionDir(root), name), data, 0600); err != nil {
			return err
		}
	}
	return ownerfixture.Hold()
}

func dispatchUpHelper() bool {
	for i, arg := range os.Args {
		if arg == "--up-owner-helper" {
			if err := upOwnerHelper(os.Args[i+1:]); err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
			return true
		}
		if arg == "--up-component-helper" {
			_ = testexec.ReportReady()
			if err := ownerfixture.Hold(); err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
			return true
		}
	}
	return false
}

func generationOwnerChild(t *testing.T) bool {
	t.Helper()
	if os.Getenv("U12A_UP_CHILD") == t.Name() {
		return false
	}
	command := exec.Command(os.Args[0], "-test.run=^"+regexp.QuoteMeta(t.Name())+"$", "-test.v", "-test.timeout=30m")
	command.Env = append(os.Environ(), "U12A_UP_CHILD="+t.Name())
	data, err := command.CombinedOutput()
	t.Logf("%s", data)
	if err != nil {
		t.Fatal(err)
	}
	return true
}

func TestStartDefersSupervisorReplacementDuringALiveProof(t *testing.T) {
	t.Parallel()
	if generationOwnerChild(t) {
		return
	}
	prior := enrolledCommand
	enrolledCommand = func(*steward.EnrolledBinary) func(...string) (*exec.Cmd, error) {
		return func(args ...string) (*exec.Cmd, error) { return ownerfixture.Command("--up-owner-helper", args...) }
	}
	t.Cleanup(func() { enrolledCommand = prior })
	if err := os.Setenv("METASYSTEM_SUPERVISION_REGISTRY_HOME", t.TempDir()); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(os.Getenv("METASYSTEM_SUPERVISION_REGISTRY_HOME"), ".metasystem"), 0700); err != nil {
		t.Fatal(err)
	}
	for _, fault := range []string{"live", "changed-live", "ended", "changed-unknown", "bad-registration", "bad-running"} {
		t.Run(fault, func(t *testing.T) {
			t.Parallel()
			options := Options{Root: canonicalRuntimePath(t.TempDir()), WaitScaleMilli: 1000, RecoverOnly: true, IfDown: true, RepairStewardRunner: currentRunnerRepair}
			options.Scope = options.Root
			options.MetasystemRoot = options.Root
			binary, _ := startfixture.Stage(t, options.Root)
			options.Binary = binary
			home := t.TempDir()
			options.LaneHome = func() (string, error) { return home, nil }
			startfixture.Register(t, home, options.Root)
			arming, err := supervisionOptions(options)
			if err != nil {
				t.Fatal(err)
			}
			arming.Command = func(args ...string) (*exec.Cmd, error) { return ownerfixture.Command("--up-owner-helper", args...) }
			started, err := supervise.EnsureArmed(arming)
			if err != nil || !started.Inspection.Armed() {
				t.Fatalf("initial owner: %+v %v", started, err)
			}
			t.Cleanup(func() {
				_, _ = supervise.ShutdownAt(options.Root, options.Root, options.Root, arming.OwnerTagPrefix, 1)
			})
			old, _ := pinfixture.Pin(t, options.Root, 147, "obsolete")
			options.Executions = func(root string) map[string]enginepin.Execution {
				owner, err := supervise.ReadArmingOwner(root)
				if err != nil {
					t.Fatal(err)
				}
				prober := pinfixture.Prober{}
				add := func(ref identity.Ref, path string, unknown bool) {
					prober[ref.Pid] = identity.Exact{Pid: ref.Pid, StartedAt: time.Unix(ref.StartedAtSec, 0), StartTicks: ref.StartTicks, BootID: ref.BootID, Exe: path, ExeKnown: !unknown}
				}
				path := binary
				if owner.Pid == started.Owner.Pid {
					path = old
				}
				add(identity.Ref{Pid: owner.Pid, StartedAtSec: owner.PidStartedAt, StartTicks: owner.PidStartTicks, BootID: owner.BootID}, path, fault == "changed-unknown")
				var document struct {
					Components map[string]struct {
						Pid           int64
						PidStartedAt  int64
						PidStartTicks int64
						BootID        string
					}
				}
				data, err := os.ReadFile(filepath.Join(supervise.SupervisionDir(root), "state.json"))
				if err != nil {
					t.Fatal(err)
				}
				if err = json.Unmarshal(data, &document); err != nil {
					t.Fatal(err)
				}
				for _, component := range document.Components {
					add(identity.Ref{Pid: component.Pid, StartedAtSec: component.PidStartedAt, StartTicks: component.PidStartTicks, BootID: component.BootID}, binary, false)
				}
				runner := pinfixture.Process(41, binary)
				prober[41] = runner
				startfixture.JSON(t, filepath.Join(root, "artifacts/agents/steward/runner.json"), steward.RunnerRecord{Pid: 41, PidStartedAt: runner.StartedAt.Unix()})
				return steward.ReadExecutions(root, prober)
			}
			if strings.HasPrefix(fault, "changed-") {
				if err := os.WriteFile(filepath.Join(options.Root, "metasystem.conf"), []byte("metasystem.runtimes=\nwatch.interval-sec=2\n"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			startfixture.JSON(t, filepath.Join(plain.Dir(options.Root), "running.json"), plain.Running{Attempt: "proof-148"})
			options.ProofAlive = func(plain.Running) bool { return fault != "ended" }
			if fault == "bad-registration" {
				if err := os.WriteFile(lane.RecordPath(home), []byte("{"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			if fault == "bad-running" {
				if err := os.WriteFile(filepath.Join(plain.Dir(options.Root), "running.json"), []byte("{"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			result := recovery(options)
			after, err := supervise.ReadArmingOwner(options.Root)
			if err != nil {
				t.Fatal(err)
			}
			lines := strings.Join(result.Lines(), "\n")
			if fault == "ended" {
				if result.ExitCode() != 0 || after.Pid == started.Owner.Pid || component(result, "engine-generation").Outcome != "verified" {
					t.Fatalf("ended proof: %+v owner=%+v", result, after)
				}
			} else {
				if result.ExitCode() != 1 || result.Outcome != "recovery-partial" || after.Pid != started.Owner.Pid {
					t.Fatalf("guard failed: %+v owner=%+v", result, after)
				}
				want := []string{"deferred", "proof-148"}
				if fault == "changed-unknown" {
					want = []string{"unverifiable", "executable identity is unknown"}
				}
				if fault == "bad-registration" {
					want = []string{"unreadable", "unexpected end of JSON input"}
				}
				if fault == "bad-running" {
					want = []string{"cannot be decoded", "unexpected end of JSON input"}
				}
				for _, word := range want {
					if !strings.Contains(lines, word) {
						t.Fatalf("missing %q: %s", word, lines)
					}
				}
				if _, err := os.Stat(filepath.Join(supervise.SupervisionDir(options.Root), "lock.d", "shutdown-intent.json")); !os.IsNotExist(err) {
					t.Fatalf("guard wrote shutdown intent: %v", err)
				}
			}
		})
	}
}
