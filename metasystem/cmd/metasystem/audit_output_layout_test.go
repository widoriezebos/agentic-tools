package main

import (
	"bytes"
	"errors"
	"flag"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/board"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/helm"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/realpath"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stoptransition"
)

// The output-style audit (plans/designs/output-style.md §5): each converted
// verb's default output is a golden under testdata/layout, driven in-process
// with fake owners; its --json is a golden taken from main before the
// conversion, so a conversion cannot move a field.

// updateLayoutJSON rewrites the --json goldens. They are main's output: run
// it only on a tree whose render is main's.
var updateLayoutJSON = flag.Bool("update-layout-json", false, "rewrite testdata/layout/*.json from this tree")

// layoutNow is the goldens' clock: Wednesday 30 September 2026, 10:58 CEST.
var layoutNow = time.Date(2026, 9, 30, 8, 58, 0, 0, time.UTC)

// layoutCase is one golden: a verb, its fixture, and the stable names its
// temporary paths and ids are replaced with.
type layoutCase struct {
	name string
	args []string
	bed  func(t *testing.T) layoutBed
}

type layoutBed struct {
	owners  intentOwners
	cwd     string
	replace []string // old, new pairs applied to every output
}

func layoutCases() []layoutCase {
	return []layoutCase{
		{name: "status", args: []string{"status"}, bed: statusLayoutBed(true)},
		{name: "status-verbose", args: []string{"status", "--verbose"}, bed: statusLayoutBed(true)},
		{name: "status-quiet", args: []string{"status"}, bed: statusLayoutBed(false)},
		{name: "status-refusal", args: []string{"status", "goal-a", "goal-b"}, bed: statusLayoutBed(false)},
		{name: "grant-list", args: []string{"grant", "list"}, bed: grantListLayoutBed(1, false)},
		{name: "grant-list-all", args: []string{"grant", "list", "--all"}, bed: grantListLayoutBed(2, true)},
		{name: "grant-list-empty", args: []string{"grant", "list"}, bed: grantListLayoutBed(0, false)},
		{name: "grant-list-refusal", args: []string{"grant", "list"}, bed: outsideLayoutBed},
	}
}

// statusLayoutBed is a checkout with its five helpers and two processes
// that are not ours, the host board, a landing lane with one batch
// collecting, and (busy) the helm taken and a general grant live.
func statusLayoutBed(busy bool) func(t *testing.T) layoutBed {
	return func(t *testing.T) layoutBed {
		b := newProcessBed(t)
		root := realpath.Resolve(b.root())
		if busy {
			item := func(family, component string, pid int64, line string) *machineItem {
				return machineFixtureItem(family, component, "", pid, line)
			}
			b.families = []stoptransition.Family{
				&machineFamily{name: "steward", items: []*machineItem{item("steward", "steward-runner", 39052, "steward-runner pid 39052 started 1790758391: running")}},
				&machineFamily{name: "supervision", items: []*machineItem{
					item("supervision", "supervision-owner", 39060, "supervision-owner pid 39060 tag metasystem-supervision-owner-m1e-1790758391-38939 generation 205: running"),
					item("supervision", "watcher", 39061, "repo-watcher pid 39061: running"),
					item("supervision", "reaper", 39062, "job-reaper pid 39062: running"),
					item("supervision", "landing-owner", 39063, "landing-batch-owner pid 39063: running")}},
				&machineFamily{name: "untracked", items: []*machineItem{
					item("untracked", "untracked", 2878, "untracked pid 2878 claude claude --allow-dangerously-skip-permissions: running"),
					item("untracked", "untracked", 10947, "untracked pid 10947 codex codex app-server: running")}},
			}
		}
		owners := b.owners()
		owners.commandNow = func(string) (time.Time, error) { return layoutNow, nil }
		owners.helm.zone = layoutZone(t)
		owners.helm.now = func() time.Time { return layoutNow }
		owners.delivery.now = func() time.Time { return layoutNow }
		owners.delivery.boardView = func(string, time.Time) board.View {
			return board.View{Readable: true, Bridge: "live", Seats: []board.SeatView{
				{Machine: "landing", Goals: []board.GoalView{}},
				{Machine: "m1e", Goals: []board.GoalView{{Goal: "switch-on-trial", Unknown: "not claimed", LastProgressAt: layoutNow.Add(-time.Minute)}}},
				{Machine: "ui", Goals: []board.GoalView{}}}}
		}
		base := t.TempDir()
		home, landing := filepath.Join(base, "home"), filepath.Join(base, "agentic-tools-landing")
		for _, dir := range []string{home, landing} {
			if err := os.MkdirAll(dir, 0o755); err != nil {
				t.Fatal(err)
			}
		}
		home, landing = realpath.Resolve(home), realpath.Resolve(landing)
		records := []batch.Record{}
		owners.landing = laneVerbOwners{
			home: func() (string, error) { return home, nil },
			probe: func(string) (lane.OwnerProbe, error) {
				return lane.OwnerProbe{Alive: true, PID: 38928, Since: layoutNow.Add(-5 * time.Minute)}, nil
			},
			records: func(string) ([]batch.Record, error) { return records, nil },
			now:     func() time.Time { return layoutNow },
		}
		owners.delivery.batchRoot = func(string, time.Time) (string, bool, error) { return "", false, nil }
		if busy {
			if _, _, err := lane.Register(home, landing, "Wido", layoutNow.Add(-2*time.Hour)); err != nil {
				t.Fatal(err)
			}
			collecting := batch.Record{Schema: 1, BatchID: "4gr18nm8t3nyev9sssda9jgtsq", State: batch.StateOpen,
				Units:   []batch.Unit{{GoalID: "533209e6d", Chain: "c", SeatRoot: root, State: batch.UnitJoined, Claim: batch.Claim{Machine: "m1e"}}},
				History: []batch.HistoryEntry{{At: layoutNow.Add(-time.Minute).Format(time.RFC3339Nano), Verb: "open", To: batch.StateOpen}}}
			records = append(records, collecting)
			if err := os.MkdirAll(filepath.Join(root, ".git"), 0o755); err != nil {
				t.Fatal(err)
			}
			if _, err := helm.Write(root, helm.Record{By: "wido", At: time.Date(2026, 9, 28, 10, 39, 0, 0, time.UTC).Format(time.RFC3339),
				Reason: "coordinating the verb and machinery batches", Checkout: root, Leader: "login", LeaderRef: "34285@1790591981"}); err != nil {
				t.Fatal(err)
			}
			entry := goal.PowerOfAttorneyEntry{ID: "YY6RQ7765HX224V3NT4TN67JFR-m1e-b6a4eb0a", By: "human:wido", Verbs: []string{goal.GeneralAct},
				Since: layoutNow.Add(-2 * time.Minute).Format(time.RFC3339), For: "m1e", Checkout: root, Lineage: "lin-main",
				Until: time.Date(2026, 10, 1, 8, 56, 0, 0, time.UTC).Format(time.RFC3339)}
			owners.attorney.entries = func(string) ([]goal.PowerOfAttorneyEntry, error) { return []goal.PowerOfAttorneyEntry{entry}, nil }
		} else {
			owners.attorney.entries = func(string) ([]goal.PowerOfAttorneyEntry, error) { return nil, nil }
		}
		return layoutBed{owners: owners, cwd: b.root(), replace: layoutPaths(b.root(), root, "/Users/wido/GitHub/agentic-tools-m1e",
			home, "/Users/wido/.metasystem-home", landing, "/Users/wido/GitHub/agentic-tools-landing")}
	}
}

// grantListLayoutBed is a ledger holding count general grants, the first
// revoked when revoke is set.
func grantListLayoutBed(count int, revoke bool) func(t *testing.T) layoutBed {
	return func(t *testing.T) layoutBed {
		b := newGrantEverythingBed(t)
		owners := b.owners()
		owners.helm.zone = layoutZone(t)
		var replace []string
		for index, spell := range []string{"24h", "8h"}[:count] {
			code, result := b.runJSON(owners, "grant", "add", "--acts", "everything", "--for", spell)
			if code != 0 || len(result.Targets) != 1 {
				t.Fatalf("grant add = %d %+v", code, result)
			}
			id := result.Targets[0].ID
			replace = append(replace, id, []string{"YY6RQ7765HX224V3NT4TN67JFR-m1e-b6a4eb0a", "01K6D2S9W0Q5Y3M7N8P4R2T6V1-m1e-7c1d9e20"}[index])
			if index == 0 && revoke {
				if code, result := b.runJSON(owners, "grant", "revoke", id); code != 0 {
					t.Fatalf("grant revoke = %d %+v", code, result)
				}
			}
		}
		root := realpath.Resolve(b.root())
		return layoutBed{owners: owners, cwd: b.root(), replace: append(replace, layoutPaths(b.root(), root, "/Users/wido/GitHub/agentic-tools-m1e")...)}
	}
}

// outsideLayoutBed runs from a directory that is no repository.
func outsideLayoutBed(t *testing.T) layoutBed {
	dir := realpath.Resolve(t.TempDir())
	notARepository := func(string) (string, error) { return "", errors.New("not a git repository") }
	b := newIntentBed(t, false, nil)
	owners := b.owners()
	owners.resolver = stateroot.NewResolver(notARepository, noExecutable)
	return layoutBed{owners: owners, cwd: dir, replace: []string{dir, "/Users/wido/scratch"}}
}

// layoutPaths are the replacements of a bed's paths: its root as given and
// resolved, then further old/new pairs.
func layoutPaths(given, resolved, stable string, more ...string) []string {
	pairs := []string{resolved, stable}
	if given != resolved {
		pairs = append(pairs, given, stable)
	}
	return append(pairs, more...)
}

func layoutZone(t *testing.T) *time.Location {
	zone, err := time.LoadLocation("Europe/Amsterdam")
	if err != nil {
		t.Fatal(err)
	}
	return zone
}

func runLayoutCase(t *testing.T, c layoutCase, bed layoutBed, args ...string) (int, string, string) {
	t.Helper()
	command, rest, ok := resolveIntentArgv(append(append([]string{}, c.args...), args...))
	if !ok {
		t.Fatalf("no public command %q", c.args)
	}
	var stdout, stderr bytes.Buffer
	code := runIntentIn(command, rest, &stdout, &stderr, bed.cwd, bed.owners)
	replacer := strings.NewReplacer(bed.replace...)
	return code, replacer.Replace(stdout.String()), replacer.Replace(stderr.String())
}

func layoutGolden(name string) string { return filepath.Join("testdata", "layout", name) }

// TestAuditOutputLayoutJSONUnchanged: every golden verb's --json is byte for
// byte what main printed before its text was converted.
func TestAuditOutputLayoutJSONUnchanged(t *testing.T) {
	t.Parallel()
	for _, c := range layoutCases() {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			code, stdout, stderr := runLayoutCase(t, c, c.bed(t), "--json")
			got := strings.Join([]string{"exit " + strconv.Itoa(code), stdout, stderr}, "\n--\n")
			path := layoutGolden(c.name + ".json")
			if *updateLayoutJSON {
				if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
					t.Fatal(err)
				}
				return
			}
			want, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("%v (write main's with -update-layout-json)", err)
			}
			if got != string(want) {
				t.Errorf("%s --json moved:\n%s\nmain printed:\n%s", c.name, got, want)
			}
		})
	}
}
