package steward

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/census"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

type censusSeamProbe func(int64) (identity.Exact, identity.Liveness, error)

func (p censusSeamProbe) Probe(pid int64) (identity.Exact, identity.Liveness, error) { return p(pid) }

func censusSeamWrite(t *testing.T, root, rel, body string) {
	t.Helper()
	path := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func censusSeam(t *testing.T, root string) RuntimeWorkerCensus {
	t.Helper()
	censusSeamWrite(t, root, "bin/metasystem", "fixture engine\n")
	if _, err := census.Fingerprint(root, root); err != nil {
		t.Fatal(err)
	}
	censusSeamWrite(t, root, "artifacts/agents/supervision/state.json", `{"generation":1,"owner":{"pid":4400,"pidStartedAt":1800,"instanceTag":"owner-t"},"components":{"watcher":{"pid":4401,"pidStartedAt":1800,"instanceTag":"watcher-t"},"reaper":{"pid":4402,"pidStartedAt":1800,"instanceTag":"reaper-t"}}}`)
	censusSeamWrite(t, root, "census-processes.json", `[]`)
	return RuntimeWorkerCensus{MetasystemRoot: root, ProcessFile: filepath.Join(root, "census-processes.json"), Now: func() time.Time { return time.Date(2030, 1, 2, 3, 4, 5, 0, time.UTC) }, Prober: censusSeamProbe(func(pid int64) (identity.Exact, identity.Liveness, error) {
		return identity.Exact{Pid: pid, StartedAt: time.Unix(1800, 0), Argv: []string{"owner-t watcher-t reaper-t"}}, identity.Alive, nil
	})}
}

func TestRuntimeWorkerCensusCompletesOnInjectedObservations(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"all-live", "owner-dead", "tag-missing", "owner-start-mismatch"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			censusSeamWrite(t, root, "metasystem.conf", "metasystem.runtimes=fake\n")
			c := censusSeam(t, root)
			base := c.Prober
			c.Prober = censusSeamProbe(func(pid int64) (identity.Exact, identity.Liveness, error) {
				exact, state, err := base.Probe(pid)
				if name == "owner-dead" && pid == 4400 {
					state = identity.Dead
				}
				if name == "tag-missing" && pid == 4401 {
					exact.Argv = []string{"no tag"}
				}
				if name == "owner-start-mismatch" && pid == 4400 {
					exact.StartedAt = time.Unix(1801, 0)
				}
				return exact, state, err
			})
			w, err := c.Workers(root)
			if err != nil || w.CensusComplete != (name == "all-live") || w.Live != 0 || w.LiveSeatMains != 0 || w.Untracked != 0 || w.Unprovable != 0 {
				t.Fatalf("observations %s: %+v %v", name, w, err)
			}
		})
	}
}

func TestRuntimeWorkerCensusReadsTheInjectedClock(t *testing.T) {
	t.Parallel()
	for _, minutes := range []int{5, 11} {
		t.Run(fmt.Sprint(minutes), func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			censusSeamWrite(t, root, "metasystem.conf", "metasystem.runtimes=fake\n")
			c := censusSeam(t, root)
			ended := c.Now()
			c.Now = func() time.Time { return ended.Add(time.Duration(minutes) * time.Minute) }
			censusSeamWrite(t, root, "artifacts/agents/runs/drain-run.json", `{"schemaVersion":1,"runId":"drain-run","kind":"suite","display":"x","custody":"wrapped","generation":1,"pid":901,"pidStartedAt":5000,"pgid":901,"launchNonce":"abababababababababababababababab","log":"/tmp/x.log","startedAt":"2030-01-02T03:00:00Z","sessionId":"s","goalId":"","staleAfterMin":30,"windDownMin":10,"endedAt":"2030-01-02T03:04:05Z","evidence":{"mode":"exit-sidecar"},"expect":{"green":"","red":"","hung":"","unknown":""},"status":"draining","provisionalVerdict":"unknown","acked":false}`)
			censusSeamWrite(t, root, "census-processes.json", fmt.Sprintf(`[{"pid":920,"pgid":901,"pidStartedAt":7000,"argv":"metasystem-fake-agent","cwd":%q,"alive":true}]`, root))
			base := c.Prober
			c.Prober = censusSeamProbe(func(pid int64) (identity.Exact, identity.Liveness, error) {
				if pid == 901 {
					return identity.Exact{}, identity.Dead, nil
				}
				return base.Probe(pid)
			})
			w, err := c.Workers(root)
			live, untracked := 1, 0
			if minutes == 11 {
				live, untracked = 0, 1
			}
			if err != nil || !w.CensusComplete || w.Live != live || w.LiveSeatMains != 0 || w.Untracked != untracked || w.Unprovable != 1 {
				t.Fatalf("at +%dm: %+v %v", minutes, w, err)
			}
		})
	}
}

func TestTheLadderCountsAnAnnouncedShellThroughTheInjectedScan(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"live", "dead", "unknown", "probe-error"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			bed := newSeatBed(t, seatReadyGoal("alpha", "Build it."))
			bed.now = time.Date(2030, 1, 2, 3, 4, 5, 0, time.UTC)
			censusSeamWrite(t, bed.root, "metasystem.conf", "metasystem.runtimes=fake\nlaunch.seat.runtime=fake\nlaunch.build.model.fake=fake-model\nlaunch.critique.model.fake=fake-model\nlaunch.design.model.fake=fake-model\nlaunch.read.model.fake=fake-model\n")
			c := censusSeam(t, bed.root)
			censusSeamWrite(t, bed.root, "census-processes.json", `[{"pid":4300,"pidStartedAt":1700,"argv":"/bin/zsh","alive":true}]`)
			censusSeamWrite(t, bed.root, "artifacts/agents/mains/shell.json", `{"sessionId":"shell","pid":4300,"pidStartedAt":1700,"pgid":4300,"runtime":"fake","instanceTag":"shell","announcedAt":"2030-01-02T03:00:00Z"}`)
			censusSeamWrite(t, bed.root, "artifacts/agents/mains/worktree-lease.json", `{"pid":4300,"pidStartedAt":1700}`)
			base := c.Prober
			c.Prober = censusSeamProbe(func(pid int64) (identity.Exact, identity.Liveness, error) {
				if pid != 4300 {
					return base.Probe(pid)
				}
				state := identity.Alive
				if name == "dead" {
					state = identity.Dead
				}
				if name == "unknown" {
					state = identity.Unknown
				}
				if name == "probe-error" {
					return identity.Exact{}, identity.Unknown, errors.New("probe failed")
				}
				return identity.Exact{Pid: pid, StartedAt: time.Unix(1700, 0)}, state, nil
			})
			live, doubt := 0, 0
			if name == "live" {
				live = 2
			}
			if name == "unknown" || name == "probe-error" {
				doubt = 2
			}
			w, err := c.Workers(bed.root)
			if err != nil || !w.CensusComplete || w.Live != live || w.LiveSeatMains != live || w.Untracked != 0 || w.Unprovable != doubt {
				t.Fatalf("%s shell: %+v %v", name, w, err)
			}
			result := bed.tickWith(c)
			if name == "dead" {
				if result.Decision.Action != ActRevive || result.Seat == nil || result.Seat.Goal != "alpha" {
					t.Fatalf("dead shell: %+v %+v", result.Decision, result.Seat)
				}
			} else if result.Decision.Action != ActNotify || result.Seat != nil || doubt > 0 && !strings.Contains(result.Decision.Reason, "death not provable") {
				t.Fatalf("retained shell: %+v %+v", result.Decision, result.Seat)
			}
		})
	}
}

func TestTheRepairNoticeIsNeitherLiveNorDoubt(t *testing.T) {
	t.Parallel()
	notice := census.HumanTerminalNotice + " withdrawn pid=4300 start=1700"
	v := census.Verdict{Verdict: "SUCCESS", Diagnostics: []string{notice}, Inventory: []census.InventoryItem{{Class: "ANNOUNCED"}}}
	w := workersFromVerdict(v)
	if !w.CensusComplete || w.Live != 1 || w.LiveSeatMains != 1 || w.Untracked != 0 || w.Unprovable != 0 {
		t.Fatalf("notice beside main: %+v", w)
	}
	v.Diagnostics = append(v.Diagnostics, "RACED-EXIT pid=9", "RUN-RECORD-UNREADABLE HUMAN-TERMINAL.json")
	if w := workersFromVerdict(v); w.Unprovable != 1 {
		t.Fatalf("unreadable record lost: %+v", w)
	}
	bed := newSeatBed(t, seatReadyGoal("alpha", "Build it."))
	bed.now = time.Date(2030, 1, 2, 3, 4, 5, 0, time.UTC)
	result := bed.tick(workersFromVerdict(census.Verdict{Verdict: "SUCCESS", Diagnostics: []string{notice}}))
	if result.Decision.Action != ActRevive || result.Seat == nil || result.Seat.Goal != "alpha" {
		t.Fatalf("notice blocked ready work: %+v %+v", result.Decision, result.Seat)
	}
}
