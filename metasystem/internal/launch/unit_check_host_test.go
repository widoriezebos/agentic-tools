package launch

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/hostload"
)

type checkHostLoad struct {
	mu   sync.Mutex
	load float64
}

func (h *checkHostLoad) set(load float64) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.load = load
}

func (h *checkHostLoad) sample(at time.Time) hostload.Sample {
	h.mu.Lock()
	defer h.mu.Unlock()
	return hostload.Sample{At: at.Format(time.RFC3339Nano), Available: true, Load1m: h.load}
}

func checkHostPolicy(t *testing.T, fixture unitFixture, settings string) string {
	t.Helper()
	conf := filepath.Join(t.TempDir(), "settings.conf")
	if err := os.WriteFile(conf, []byte(settings), 0600); err != nil {
		t.Fatal(err)
	}
	fixture.manager.BuildPolicy = config.GetParams{ConfPath: conf, LookupEnv: func(string) (string, bool) { return "", false }}
	return conf
}

func TestUnitCheckWaitsForHostLoad(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name    string
		actor   string
		start   float64
		proof   float64
		policy  string
		atProof func(t *testing.T, fixture unitFixture, conf string)
		refused bool
	}{
		{name: "loaded", proof: 9, refused: true},
		{name: "builds-cap-full", policy: "host.builds=1\nhost.load-max=8\n", atProof: func(t *testing.T, fixture unitFixture, _ string) {
			other := Record{ID: "other-build", Kind: "build", Goal: "other-goal", Adapter: "codex-exec", WorkingDirectory: t.TempDir(), State: Running, StartedAt: fixture.manager.Now().Format(time.RFC3339Nano), AdapterData: map[string]json.RawMessage{}}
			supervisor, child := ref(10), ref(20)
			other.Supervisor, other.Child, other.ProcessGroup = &supervisor, &child, &child
			if err := fixture.manager.Store.Create(other); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "builds-person", policy: "host.builds=auto\nhost.load-max=8\n", atProof: func(t *testing.T, _ unitFixture, conf string) {
			if err := os.WriteFile(conf, []byte("host.builds=person\nhost.load-max=8\n"), 0600); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "person-at-load", actor: "person", start: 9, proof: 9},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			events := []string{"branch", "branch", "round"}
			if row.refused {
				events = []string{"branch", "branch", "round", "branch"}
			}
			fixture := newUnitFixture(t, "", events...)
			host := &checkHostLoad{load: row.start}
			fixture.manager.CapacitySources.Load = host.sample
			conf := ""
			if row.policy != "" {
				conf = checkHostPolicy(t, fixture, row.policy)
			}
			fixture.runner.Actor = row.actor
			fixture.starter.onBuild = func(Record) error {
				host.set(row.proof)
				if row.atProof != nil {
					row.atProof(t, fixture, conf)
				}
				return nil
			}
			result, err := fixture.runner.Advance(UnitRequest{Plan: fixture.plan})
			if !row.refused {
				if err != nil || len(fixture.starter.order) < 2 || fixture.starter.order[1] != "proof" {
					t.Fatalf("the check did not start: order=%v result=%+v err=%v", fixture.starter.order, result, err)
				}
				return
			}
			if !IsCode(err, "LAUNCH_BUILD_CAPACITY") {
				t.Fatalf("the loaded host admitted the check: order=%v err=%v", fixture.starter.order, err)
			}
			record, statusErr := fixture.runner.Status(result.Record.ID)
			if statusErr != nil || len(record.Rounds) != 1 {
				t.Fatalf("status: %+v %v", record, statusErr)
			}
			step := stepNamed(t, record.Rounds[0], "proof:check")
			if step.State != StepStarting || len(step.LaunchIDs) != 1 || step.LaunchIDs[0] != step.LaunchID {
				t.Fatalf("the waiting check is not a starting step with one launch: %+v", step)
			}
			if _, err := fixture.manager.Store.Read(step.LaunchID); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("the waiting check left a launch record: %v", err)
			}
			host.set(0)
			if _, err := fixture.runner.Advance(UnitRequest{Resume: result.Record.ID}); err != nil {
				t.Fatalf("the check did not start once the host was quiet: %v", err)
			}
			if len(fixture.starter.ids) < 2 || fixture.starter.ids[1] != step.LaunchID {
				t.Fatalf("the check started under another launch id: %v, want %s", fixture.starter.ids, step.LaunchID)
			}
		})
	}
}

func TestUnitComparisonProofIgnoresHostLoad(t *testing.T) {
	t.Parallel()
	f, _ := attributionFixture(t, false, "")
	host := &checkHostLoad{}
	f.manager.CapacitySources.Load = host.sample
	start := f.starter.onStart
	f.starter.onStart = func(record Record) error {
		if record.Kind == "proof" && !strings.HasSuffix(record.ID, "-base") {
			host.set(9)
		}
		return start(record)
	}
	result, err := f.runner.Advance(UnitRequest{Plan: f.plan})
	if err != nil || len(f.starter.ids) != 3 || !strings.HasSuffix(f.starter.ids[2], "-base") || result.Record.Rounds[0].Cause != "own" {
		t.Fatalf("the comparison waited for the host: launches=%v result=%+v err=%v", f.starter.ids, result, err)
	}
}

func TestUnitCheckOutputWriterLogsEachWrite(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "output.log")
	log, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		t.Fatal(err)
	}
	defer log.Close()
	output := newOutputProgress(log)
	if n, err := output.Write([]byte("shard one\n")); n != 10 || err != nil {
		t.Fatalf("write: %d %v", n, err)
	}
	if data, err := os.ReadFile(path); err != nil || string(data) != "shard one\n" {
		t.Fatalf("the log does not hold the write while the command runs: %q %v", data, err)
	}
	select {
	case <-output.wrote:
	default:
		t.Fatal("the write did not signal progress")
	}
	if output.buffer.String() != "shard one\n" {
		t.Fatalf("the buffer lost the write: %q", output.buffer.String())
	}
}

func TestUnitCheckStreamsOutputLog(t *testing.T) {
	t.Parallel()
	cheap, audits := "printf 'landing package internal/a 1 fail 9\\n'; exit 1", "printf 'audit\\n'"
	check := UnitCheck{Cheap: cheap, Audits: audits, Minutes: 15, Directory: t.TempDir(), Environment: []string{}}
	context := CheckContext{CheckExecution: CheckExecution{Goal: "fixture", Run: "run", Round: "round", Role: "attestation"}, Parent: "proof-launch",
		Now: func() time.Time { return time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC) }, After: func(time.Duration) <-chan time.Time { return nil }}
	directory, exits, err := check.Run(check.Directory, t.TempDir(), context)
	if err == nil || len(exits) != 2 {
		t.Fatalf("exits=%+v err=%v", exits, err)
	}
	data, err := os.ReadFile(filepath.Join(directory, "output.log"))
	want := "== cheap: " + cheap + "\nlanding package internal/a 1 fail 9\n== audits: " + audits + "\naudit\n"
	if err != nil || string(data) != want {
		t.Fatalf("output.log:\n%s\nwant:\n%s\nerr=%v", data, want, err)
	}
	var retained struct{ Exits []CheckExit }
	raw, err := os.ReadFile(filepath.Join(directory, "result.json"))
	if err != nil || json.Unmarshal(raw, &retained) != nil || len(retained.Exits) != 2 || retained.Exits[0].Output != "landing package internal/a 1 fail 9\n" || retained.Exits[1].Output != "audit\n" {
		t.Fatalf("the log changed the retained output: %s %v", raw, err)
	}
}
