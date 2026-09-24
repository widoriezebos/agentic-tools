package landing

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
)

func TestParkLockUsesInjectedClockAndPause(t *testing.T) {
	t.Parallel()

	f := &observeFixture{t: t, root: t.TempDir()}
	f.write("metasystem.conf", "metasystem.runtimes=fake\n")
	const target = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	wantArgs := []string{"-C", f.root,
		"-c", "core.fileMode=true", "-c", "diff.noprefix=false", "-c", "diff.mnemonicPrefix=false",
		"-c", "apply.ignoreWhitespace=no", "-c", "core.logAllRefUpdates=false",
		"-c", "core.useReplaceRefs=false", "-c", "gc.auto=0", "-c", "maintenance.auto=false",
		"rev-parse", "--verify", target + "^{commit}"}
	rawCalls := 0
	chain := "fake-clock-lock"
	lockPath := filepath.Join(f.root, "artifacts", "agents", "landing", "locks", chain+".d")
	blockerTag := filepath.Base(os.Args[0])
	if err := dispatch.OwnerLockClaim(lockPath, int64(os.Getpid()), blockerTag); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = dispatch.OwnerLockRelease(lockPath, int64(os.Getpid()), blockerTag) })

	now := time.Date(2000, time.January, 1, 0, 0, 0, 0, time.UTC)
	pauses := 0
	_, err := Park(ParkParams{
		Root: f.root, Chain: chain, TargetCommit: target,
		Reason: "chain-recertification-target-moved", Detail: "fixture lock contention",
		CallerPID: int64(os.Getpid()), Clock: func() time.Time { return now },
		RawSource: func(request gittree.RawRequest) gittree.RawResult {
			t.Helper()
			if rawCalls != 0 || request.Dir != f.root || !reflect.DeepEqual(request.Args, wantArgs) ||
				request.Stdin != nil || request.Operation != "git rev-parse --verify "+target+"^{commit}" ||
				!reflect.DeepEqual(request.Env, gittree.ScrubbedEnviron()) {
				t.Fatalf("unexpected or repeated park raw Git request: %+v", request)
			}
			rawCalls++
			return gittree.RawResult{Stdout: []byte(target + "\n")}
		},
		Pause: func(duration time.Duration) {
			if duration != 20*time.Millisecond {
				t.Fatalf("pause = %s, want 20ms", duration)
			}
			pauses++
			if pauses > 1 {
				t.Fatal("park did not apply its injected-clock deadline")
			}
			now = now.Add(landingParkLimit + time.Second)
		},
	})
	if rawCalls != 1 {
		t.Fatalf("raw Git calls = %d, want 1", rawCalls)
	}
	if err == nil || !strings.Contains(err.Error(), "park lock timed out") {
		t.Fatalf("Park error = %v, want injected-clock lock timeout", err)
	}
	if pauses != 1 {
		t.Fatalf("pause count = %d, want 1", pauses)
	}
}

func TestParkPublicationUsesInjectedClock(t *testing.T) {
	t.Parallel()

	now := time.Date(2000, time.January, 1, 0, 0, 0, 0, time.UTC)
	const target = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	if os.Getenv("LANDING_PARK_CLOCK_HELPER") == "1" {
		root := os.Getenv("LANDING_PARK_CLOCK_ROOT")
		if os.Getenv("LANDING_PARK_CLOCK_TARGET") != target {
			t.Fatal("park helper target differs from the fixed fixture target")
		}
		pins := []string{"-C", root,
			"-c", "core.fileMode=true", "-c", "diff.noprefix=false", "-c", "diff.mnemonicPrefix=false",
			"-c", "apply.ignoreWhitespace=no", "-c", "core.logAllRefUpdates=false",
			"-c", "core.useReplaceRefs=false", "-c", "gc.auto=0", "-c", "maintenance.auto=false"}
		commands := [][]string{{"rev-parse", "--verify", target + "^{commit}"}, {"rev-parse", "--show-toplevel"}}
		operations := []string{"git rev-parse --verify " + target + "^{commit}", "git rev-parse --show-toplevel"}
		outputs := []string{target + "\n", root + "\n"}
		rawCalls := 0
		result, err := Park(ParkParams{
			Root: root, Chain: os.Getenv("LANDING_PARK_CLOCK_CHAIN"), TargetCommit: target,
			Reason: "chain-recertification-target-moved", Detail: "fixture publication",
			CallerPID: int64(os.Getpid()), Clock: func() time.Time { return now },
			RawSource: func(request gittree.RawRequest) gittree.RawResult {
				t.Helper()
				if rawCalls >= len(commands) || request.Dir != root || request.Stdin != nil ||
					!reflect.DeepEqual(request.Env, gittree.ScrubbedEnviron()) ||
					!reflect.DeepEqual(request.Args, append(append([]string{}, pins...), commands[rawCalls]...)) ||
					request.Operation != operations[rawCalls] {
					t.Fatalf("unexpected or repeated park raw Git request: %+v", request)
				}
				output := outputs[rawCalls]
				rawCalls++
				return gittree.RawResult{Stdout: []byte(output)}
			},
		})
		if rawCalls != len(commands) {
			t.Fatalf("park raw Git calls = %d, want %d", rawCalls, len(commands))
		}
		if err != nil {
			t.Fatal(err)
		}
		if result.State != "parked" {
			t.Fatalf("Park state = %q, want parked", result.State)
		}
		data, err := os.ReadFile(filepath.Join(os.Getenv("LANDING_PARK_CLOCK_ROOT"), filepath.FromSlash(result.ParkRecord)))
		if err != nil {
			t.Fatal(err)
		}
		var record ParkRecord
		if err := json.Unmarshal(data, &record); err != nil {
			t.Fatal(err)
		}
		if record.ParkedAt != now.Format(time.RFC3339Nano) {
			t.Fatalf("parkedAt = %q, want injected time %q", record.ParkedAt, now.Format(time.RFC3339Nano))
		}
		return
	}

	f := &observeFixture{t: t, root: t.TempDir()}
	chain := "fake-clock-publication"
	f.writeChainRecord(chain, map[string]any{
		"jobId": chain, "parentJob": nil, "role": "implementer",
	})
	f.write("metasystem.conf", "metasystem.runtimes=fake\n")
	physicalRoot, err := filepath.EvalSymlinks(f.root)
	if err != nil {
		t.Fatal(err)
	}
	identityTable := filepath.Join(t.TempDir(), "process-identities.json")
	if err := os.WriteFile(identityTable, []byte(fmt.Sprintf(`{"%d":{"terminal":true}}`, os.Getpid())), 0o600); err != nil {
		t.Fatal(err)
	}
	helper := exec.Command(os.Args[0], "-test.run=^TestParkPublicationUsesInjectedClock$")
	helper.Env = gittree.ScrubbedEnviron(
		"LANDING_PARK_CLOCK_HELPER=1",
		"LANDING_PARK_CLOCK_ROOT="+physicalRoot,
		"LANDING_PARK_CLOCK_CHAIN="+chain,
		"LANDING_PARK_CLOCK_TARGET="+target,
		"METASYSTEM_FAKE_PROCESS_IDENTITY_FILE="+identityTable,
	)
	if output, err := helper.CombinedOutput(); err != nil {
		t.Fatalf("park clock helper failed: %v\n%s", err, output)
	}
}
