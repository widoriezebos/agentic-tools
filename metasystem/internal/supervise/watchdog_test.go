package supervise

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// watchdogRepo lays out a checkout with a supervision directory and installs
// a fake process table pinning THIS test process's start to 100. The census
// liveness check believes the kernel first, so only a genuinely live pid can
// read alive — the fixture then supplies its recorded start second.
func watchdogRepo(t *testing.T) string {
	t.Helper()
	repo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repo, "artifacts", "agents", "supervision"), 0o755); err != nil {
		t.Fatal(err)
	}
	table := filepath.Join(repo, "proc-table.json")
	entry := `{"` + itoaTest(int64(os.Getpid())) + `":{"started":100,"command":"owner"}}`
	if err := os.WriteFile(table, []byte(entry), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("METASYSTEM_FAKE_PROCESS_IDENTITY_FILE", table)
	// The fixture authority requires a fake-mode conf at the root
	// (agnosticism B1): a fixture without it is now REFUSED.
	if err := os.WriteFile(filepath.Join(repo, "metasystem.conf"),
		[]byte("metasystem.runtimes=fake\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return repo
}

func writeSupervisionFile(t *testing.T, repo, name, content string) {
	t.Helper()
	path := filepath.Join(repo, "artifacts", "agents", "supervision", name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

const watchdogNow = int64(1786000000)

func healthyCensus(completed int64) string {
	return `{"verdict":"SUCCESS","completedAtEpoch":` + itoaTest(completed) + `,"intervalSec":60,"fingerprint":"fp-1","inventory":[]}`
}

func healthyState() string {
	self := itoaTest(int64(os.Getpid()))
	return `{"fingerprint":"fp-1","owner":{"pid":` + self + `,"pidStartedAt":100},"components":{"watcher":{"pid":` + self + `,"pidStartedAt":100}}}`
}

func itoaTest(v int64) string { return strconv.FormatInt(v, 10) }

func TestWatchdogReportJudgments(t *testing.T) {
	cases := []struct {
		name    string
		census  string
		state   string
		want    []string // substrings, one per expected line
		healthy bool
	}{
		{
			name:   "healthy is silent",
			census: healthyCensus(watchdogNow - 10), state: healthyState(),
			healthy: true,
		},
		{
			name:  "absent census and state name both symptoms",
			want:  []string{"never reported; state unreadable"},
			state: "", census: "",
		},
		{
			name:   "failed verdict",
			census: `{"verdict":"DEGRADED","completedAtEpoch":` + itoaTest(watchdogNow-10) + `,"intervalSec":60,"fingerprint":"fp-1"}`,
			state:  healthyState(),
			want:   []string{"last census failed"},
		},
		{
			name:   "stale census ages in human terms",
			census: healthyCensus(watchdogNow - 200), state: healthyState(),
			want: []string{"last census 3m old"},
		},
		{
			name:   "fingerprint drift",
			census: healthyCensus(watchdogNow - 10),
			state:  strings.Replace(healthyState(), "fp-1", "fp-OLD", 1),
			want:   []string{"code changed since arming"},
		},
		{
			name:   "dead identities named sorted",
			census: healthyCensus(watchdogNow - 10),
			state: strings.Replace(healthyState(),
				`"components":{"watcher":{"pid":`+itoaTest(int64(os.Getpid()))+`,"pidStartedAt":100}}`,
				`"components":{"watcher":{"pid":999999,"pidStartedAt":1},"reaper":{"pid":999998,"pidStartedAt":1}}`, 1),
			want: []string{"reaper+watcher not running"},
		},
		{
			name: "untracked grouped by runtime",
			census: `{"verdict":"SUCCESS","completedAtEpoch":` + itoaTest(watchdogNow-10) + `,"intervalSec":60,"fingerprint":"fp-1",` +
				`"inventory":[{"class":"UNTRACKED","pid":7,"runtime":"claude"},{"class":"UNTRACKED","pid":9,"runtime":"claude"},{"class":"UNTRACKED","pid":8},{"class":"CUSTODY","pid":42,"runtime":"claude"}]}`,
			state: healthyState(),
			want:  []string{"UNTRACKED agents (not this checkout's work; detail: bin/metasystem proc census): claude 7,9; unknown 8"},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			repo := watchdogRepo(t)
			if c.census != "" {
				writeSupervisionFile(t, repo, "last-census.json", c.census)
			}
			if c.state != "" {
				writeSupervisionFile(t, repo, "state.json", c.state)
			}
			lines := WatchdogReport(repo, time.Unix(watchdogNow, 0))
			if c.healthy {
				if len(lines) != 0 {
					t.Fatalf("healthy supervision must report nothing, got %q", lines)
				}
				return
			}
			if len(lines) != len(c.want) {
				t.Fatalf("want %d line(s) %q, got %q", len(c.want), c.want, lines)
			}
			for i, want := range c.want {
				if !strings.Contains(lines[i], want) {
					t.Fatalf("line %d %q does not contain %q", i, lines[i], want)
				}
			}
		})
	}
}

func TestHumanAgeSpellings(t *testing.T) {
	cases := map[int64]string{
		90:        "90s",
		200:       "3m",
		7200:      "2h",
		7320:      "2h2m",
		49 * 3600: "2d",
	}
	for seconds, want := range cases {
		if got := humanAge(seconds); got != want {
			t.Fatalf("humanAge(%d) = %q, want %q", seconds, got, want)
		}
	}
}
