package landing

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
)

func TestParkLockUsesInjectedClockAndPause(t *testing.T) {
	t.Parallel()

	f := newAdoptedObserveFixture(t)
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
		Root: f.root, Chain: chain, TargetCommit: f.git("rev-parse", "HEAD"),
		Reason: "chain-recertification-target-moved", Detail: "fixture lock contention",
		CallerPID: int64(os.Getpid()), Clock: func() time.Time { return now },
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
	if os.Getenv("LANDING_PARK_CLOCK_HELPER") == "1" {
		result, err := Park(ParkParams{
			Root: os.Getenv("LANDING_PARK_CLOCK_ROOT"), Chain: os.Getenv("LANDING_PARK_CLOCK_CHAIN"),
			TargetCommit: os.Getenv("LANDING_PARK_CLOCK_TARGET"),
			Reason:       "chain-recertification-target-moved", Detail: "fixture publication",
			CallerPID: int64(os.Getpid()), Clock: func() time.Time { return now },
		})
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

	f := newAdoptedObserveFixture(t)
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
		"LANDING_PARK_CLOCK_TARGET="+f.git("rev-parse", "HEAD"),
		"METASYSTEM_FAKE_PROCESS_IDENTITY_FILE="+identityTable,
	)
	if output, err := helper.CombinedOutput(); err != nil {
		t.Fatalf("park clock helper failed: %v\n%s", err, output)
	}
}
