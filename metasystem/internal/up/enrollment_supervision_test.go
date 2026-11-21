package up

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/enginepin"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/enginepin/testdata/pinfixture"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/state"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/supervise"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/supervise/testdata/ownerfixture"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/up/testdata/startfixture"
)

func TestStartChecksEnrollmentBeforeReplacingSupervision(t *testing.T) {
	t.Parallel()
	if generationOwnerChild(t) {
		return
	}
	registryHome := canonicalRuntimePath(t.TempDir())
	if err := os.Setenv("METASYSTEM_SUPERVISION_REGISTRY_HOME", registryHome); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(registryHome, ".metasystem"), 0700); err != nil {
		t.Fatal(err)
	}
	for _, fault := range []string{"newer", "ordinary-newer", "before-stop", "after-stop", "preparing-command", "after-launch-check", "path-only", "digest-only", "generation-only", "republished", "current"} {
		t.Run(fault, func(t *testing.T) {
			options := seatUpOptions(t)
			root := options.Root
			options.Binary, _ = startfixture.Stage(t, root)
			options.MetasystemRoot = root
			options.RecoverOnly, options.IfDown = fault != "ordinary-newer", true
			options.RepairStewardRunner = currentRunnerRepair
			home := canonicalRuntimePath(t.TempDir())
			startfixture.Register(t, home, root)
			options.LaneHome = func() (string, error) { return home, nil }
			arming, err := supervisionOptions(options)
			if err != nil {
				t.Fatal(err)
			}
			arming.Command = func(args ...string) (*exec.Cmd, error) { return ownerfixture.Command("--up-owner-helper", args...) }
			started, err := supervise.EnsureArmed(arming)
			if err != nil || !started.Inspection.Armed() {
				t.Fatalf("fixture owner: %+v %v", started, err)
			}
			armedAt := time.Now()
			t.Cleanup(func() { _, _ = supervise.ShutdownAt(root, root, root, arming.OwnerTagPrefix, 1) })

			path := steward.RepoIdentityPath(root)
			pin, err := steward.VerifyIdentity(path, root)
			if err != nil {
				t.Fatal(err)
			}
			next := pin
			next.Generation++
			next.InstallPath, next.InstallDigest = pinfixture.Pin(t, root, next.Generation, "newer")
			oldPath, _ := pinfixture.Pin(t, root, pin.Generation-1, "older")
			livePath := oldPath
			switch fault {
			case "newer", "ordinary-newer":
				livePath = next.InstallPath
			case "current":
				livePath = pin.InstallPath
			case "republished":
				next = pin
				next.MintedAt = "2026-11-21T00:00:00Z"
			case "path-only":
				next.Generation, next.InstallDigest = pin.Generation, pin.InstallDigest
			case "digest-only":
				next.Generation, next.InstallPath = pin.Generation, pin.InstallPath
			case "generation-only":
				next.InstallPath, next.InstallDigest = pin.InstallPath, pin.InstallDigest
			}
			mint := func() {
				if err := steward.MintIdentity(path, next); err != nil {
					t.Fatal(err)
				}
			}
			options.Executions = func(string) map[string]enginepin.Execution {
				exact := pinfixture.Process(41, livePath)
				e := enginepin.Read(root, exact.Ref(), pinfixture.Prober{41: exact})
				return map[string]enginepin.Execution{"owner": e, "watcher": e, "reaper": e, "runner": e}
			}
			commands := 0
			options.EnsureArmed = func(o supervise.EnsureOptions) (supervise.EnsureResult, error) {
				switch fault {
				case "before-stop", "after-stop":
					gate := o.ProofGate
					o.ProofGate = func(stop func() error) (string, error) {
						return gate(func() error {
							if fault == "before-stop" {
								mint()
							}
							err := stop()
							if err == nil && fault == "after-stop" {
								mint()
							}
							return err
						})
					}
				case "current", "preparing-command", "after-launch-check":
				default:
					mint()
				}
				if fault == "after-launch-check" {
					read := o.ChangedEnrollment
					o.ChangedEnrollment = func() (int, error) {
						gen, err := read()
						if o.TargetDigest == pin.InstallDigest && commands == 1 && gen == 0 && err == nil {
							mint()
						}
						return gen, err
					}
				}
				o.Command = func(args ...string) (*exec.Cmd, error) {
					commands++
					if fault == "preparing-command" {
						mint()
					}
					if o.TargetDigest == next.InstallDigest {
						livePath = next.InstallPath
					} else {
						livePath = pin.InstallPath
					}
					return arming.Command(args...)
				}
				return supervise.EnsureArmed(o)
			}
			result := generationStart(options)
			after, ownerErr := supervise.ReadArmingOwner(root)
			switch fault {
			case "after-launch-check":
				if result.ExitCode() != 0 || commands != 1 {
					t.Fatalf("first launch: %+v commands=%d", result, commands)
				}
				options.Binary = next.InstallPath
				result = generationStart(options)
				if result.ExitCode() != 0 || commands != 2 || livePath != next.InstallPath {
					t.Fatalf("new start: %+v commands=%d", result, commands)
				}
			case "current", "republished":
				want, count := "replaced", 1
				if fault == "current" {
					want, count = "not-needed", 0
				}
				if result.ExitCode() != 0 || component(result, "supervision-owner").Outcome != want || commands != count || ownerErr != nil || (after.Pid == started.Owner.Pid) != (count == 0) {
					t.Fatalf("stable: %+v commands=%d owner=%+v %v", result, commands, after, ownerErr)
				}
			default:
				lines := result.Lines()
				wantCommands := 0
				if fault == "preparing-command" {
					wantCommands = 1
				}
				if commands != wantCommands || result.ExitCode() != 1 || options.RecoverOnly && result.Outcome != "recovery-partial" || !strings.Contains(lines[len(lines)-1], fmt.Sprintf("Enrollment changed to install %d", next.Generation)) {
					t.Fatalf("stale start: %s commands=%d", strings.Join(lines, "\n"), commands)
				}
				if fault == "after-stop" || fault == "preparing-command" {
					if !os.IsNotExist(ownerErr) {
						t.Fatalf("expected stopped supervision: %+v %v", after, ownerErr)
					}
					if _, err := os.Stat(filepath.Join(supervise.SupervisionDir(root), "lock.d")); !os.IsNotExist(err) {
						t.Fatalf("left owner election: %v", err)
					}
				} else {
					if ownerErr != nil || after != started.Owner {
						t.Fatalf("new supervision changed: %+v %v", after, ownerErr)
					}
					if _, err := os.Stat(filepath.Join(supervise.SupervisionDir(root), "lock.d", "shutdown-intent.json")); !os.IsNotExist(err) {
						t.Fatalf("shutdown intent: %v", err)
					}
					inspection := supervise.InspectArmed(filepath.Join(root, "artifacts", "agents"), after.Pid, after.PidStartedAt, after.InstanceTag, int64(arming.IntervalSec), armedAt)
					if !inspection.Armed() {
						t.Fatalf("supervision no longer live: %+v", inspection)
					}
				}
				record, found, err := state.Read(plain.Dir(root))
				if err != nil || !found || record.Generation == nil || record.Generation.Verdict != "deferred" || record.Generation.Reason != result.Deferred {
					t.Fatalf("deferral missing: %+v %v", record.Generation, err)
				}
			}
		})
	}
}

func TestFollowRestartRecordsLostSupervisionWait(t *testing.T) {
	t.Parallel()
	options := seatUpOptions(t)
	options.Binary, _ = startfixture.Stage(t, options.Root)
	options.FollowRestart = true
	options.EnsureArmed = supervise.EnsureArmed
	dir := filepath.Join(supervise.SupervisionDir(options.Root), "cap-authority.lock.d")
	if err := os.MkdirAll(filepath.Dir(dir), 0700); err != nil {
		t.Fatal(err)
	}
	pid, tag := int64(os.Getpid()), os.Args[0]
	if err := dispatch.OwnerLockClaim(dir, pid, tag); err != nil {
		t.Fatal(err)
	}
	defer dispatch.OwnerLockRelease(dir, pid, tag)
	result := ordinary(options)
	if result.ExitCode() != 1 || !strings.Contains(component(result, "supervision-owner").Detail, "remained busy") {
		t.Fatalf("cap wait: %+v", result)
	}
	data, err := os.ReadFile(filepath.Join(options.Root, "artifacts", "agents", "steward", "runner.log"))
	if err != nil || !strings.Contains(string(data), "engine follow: the restart did not take supervision") || !strings.Contains(string(data), "remained busy") {
		t.Fatalf("follow record missing: %v", err)
	}
}
