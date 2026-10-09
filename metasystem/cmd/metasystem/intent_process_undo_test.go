package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/helm"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/processchange"
)

func TestProcessDriftMissingCheckAllowancePublicStatus(t *testing.T) {
	t.Parallel()
	bed := newWorkBed(t)
	clock := &workClock{now: time.Date(2026, 10, 8, 10, 0, 0, 0, time.UTC)}
	bed.manager.Now, bed.manager.Supervisor = clock.Now, processCostStarter{bed.starter, clock}
	processEstimatePage(t, bed, "500", "-")
	if code, result, output := processEvidenceBuild(bed); code != 0 {
		t.Fatalf("build: %d %+v %s", code, result, output)
	}
	stops, bands := driftStatus(t, bed)
	if len(stops) != 0 || bands["unit check minutes"] != "unknown" {
		t.Fatalf("missing allowance became zero: %+v %v", stops, bands)
	}
}

func undoSettings(t *testing.T, bed *workBed, person bool, args ...string) (int, intentResult, string) {
	t.Helper()
	owners := bed.workOwners()
	owners.commandNow = func(string) (time.Time, error) { return bed.manager.Now(), nil }
	owners.dependencies.ownerLineage = func() string { return "builder" }
	owners.prove = fixedFixtureGoalAuthority
	if person {
		owners.prove = enrolledPersonProver(t, bed.root(), bed.manager.Now())
	}
	owners.policies.ConfPath = func(checkout string) (string, error) { return filepath.Join(checkout, "settings.conf"), nil }
	owners.policies.Registry = func(string) (config.PolicyRegistry, error) { return config.PolicyRegistry{}, nil }
	owners.policies.Helm = func(string) helm.State {
		return helm.State{Active: true, Record: helm.Record{By: "Wido"}, Since: bed.manager.Now()}
	}
	owners.work.settings = func(string) (launch.Settings, error) {
		return launch.ResolveSettings(filepath.Join(bed.root(), "settings.conf"), func(string) (string, bool) { return "", false })
	}
	var stdout, stderr bytes.Buffer
	command, parsed, ok := resolveIntentArgv(append(append([]string{"settings"}, args...), "--json"))
	if !ok {
		t.Fatalf("settings command did not resolve: %v", args)
	}
	code := runIntentIn(command, parsed, &stdout, &stderr, bed.root(), owners)
	var result intentResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("settings: %v %s %s", err, &stdout, &stderr)
	}
	return code, result, stdout.String()
}

func TestProcessSettingUndoPublicReverse(t *testing.T) {
	t.Parallel()
	for _, absent := range []bool{false, true} {
		t.Run(map[bool]string{false: "present-layer", true: "absent-layer"}[absent], func(t *testing.T) {
			t.Parallel()
			bed := newWorkBed(t)
			clock := &workClock{now: time.Date(2026, 10, 8, 10, 0, 0, 0, time.UTC)}
			bed.manager.Now, bed.manager.Supervisor = clock.Now, processCostStarter{bed.starter, clock}
			if err := os.MkdirAll(filepath.Join(bed.root(), ".git"), 0700); err != nil {
				t.Fatal(err)
			}
			conf := filepath.Join(bed.root(), "settings.conf")
			if err := os.WriteFile(conf, []byte("launch.codex.sandbox=workspace-write\n"), 0600); err != nil {
				t.Fatal(err)
			}
			original := "# preserve comment\nsecret=synthetic\n"
			if !absent {
				original += "launch.codex.sandbox=workspace-write\n"
			}
			if err := os.WriteFile(conf+".local", []byte(original), 0600); err != nil {
				t.Fatal(err)
			}
			processEstimatePage(t, bed, "1", "100")
			code, result, _ := undoSettings(t, bed, false, "set", "launch.codex.sandbox", "danger-full-access", "--goal", bed.id)
			proposed := processAct(t, result)
			if code != 1 || proposed.ID == "" || proposed.Actor != "agent" {
				t.Fatalf("agent proposal: %d %+v", code, result)
			}
			code, result, _ = undoSettings(t, bed, true, "set", proposed.Key, proposed.After, "--act", proposed.ID)
			applied := processAct(t, result)
			if code != 0 || applied.Status != "applied" {
				t.Fatalf("apply: %d %+v", code, result)
			}
			historyPath := filepath.Join(bed.root(), "process", "acts", applied.ID+".json")
			history, err := os.ReadFile(historyPath)
			if err != nil {
				t.Fatal(err)
			}
			clock.Sleep(time.Second)
			bed.manager.Settings, err = launch.ResolveSettings(conf, func(string) (string, bool) { return "", false })
			if err != nil {
				t.Fatal(err)
			}
			bed.manager.Settings.BuildRuntime = "codex"
			if code, result, output := processEvidenceBuild(bed); code != 0 {
				t.Fatalf("build: %d %+v %s", code, result, output)
			}
			stops, bands := driftStatus(t, bed)
			if len(stops) != 1 || stops[0].Stop.Cause == nil || stops[0].Stop.Cause.Kind != "process-change" || stops[0].Stop.Cause.Name != applied.ID {
				t.Fatalf("consumed act attribution: %+v", stops)
			}
			reverse := []string{"set", applied.Key, "workspace-write", "--repo", bed.root(), "--undo", applied.ID}
			if absent {
				reverse = []string{"unset", applied.Key, "--repo", bed.root(), "--undo", applied.ID}
			}
			want := shellCommand(append([]string{"metasystem", "settings"}, reverse...))
			// Follow the command displayed by the stop.
			printed := append([]string{"metasystem", "settings"}, reverse...)
			if stops[0].Stop.Handoff != want {
				t.Fatalf("reverse: %s, want %s (%v)", stops[0].Stop.Handoff, want, err)
			}
			q, err := channel.ReadQuestion(bed.root(), stops[0].Question)
			if err != nil || q.Wants != stops[0].Stop.Handoff || !strings.Contains(q.Facts[2], "danger-full-access") {
				t.Fatalf("impact and reverse: %+v %v", q, err)
			}
			if code, result, _ := undoSettings(t, bed, false, printed[2:]...); code != 1 {
				t.Fatalf("agent obtained person undo authority: %+v", result)
			}
			clock.Sleep(time.Second)
			code, result, _ = undoSettings(t, bed, true, printed[2:]...)
			inverse := processAct(t, result)
			if code != 0 || inverse.ID == applied.ID || inverse.Undo != applied.ID || inverse.Status != "applied" || inverse.Unset != absent {
				t.Fatalf("inverse act: %d %+v", code, result)
			}
			afterHistory, _ := os.ReadFile(historyPath)
			if !bytes.Equal(history, afterHistory) {
				t.Fatal("undo rewrote original act")
			}
			local, err := os.ReadFile(conf + ".local")
			if err != nil || !bytes.Contains(local, []byte("# preserve comment\nsecret=synthetic\n")) {
				t.Fatalf("unrelated settings lost: %s %v", local, err)
			}
			value, present, err := config.ConfLookup(conf+".local", applied.Key)
			if err != nil || present == absent || (!absent && value != "workspace-write") {
				t.Fatalf("before-layer not restored: %q %v %v", value, present, err)
			}
			code, shown, _ := undoSettings(t, bed, true, "show", applied.Key)
			source := "conf-local"
			if absent {
				source = "conf"
			}
			if code != 0 || !strings.Contains(jsonText(shown.Data), `"Source":"`+source+`"`) || !strings.Contains(jsonText(shown.Data), "workspace-write") {
				t.Fatalf("inherited source: %d %+v", code, shown)
			}
			state, err := processchange.ReadState(bed.root(), bed.id)
			if err != nil || len(state.Stops) != 0 || len(state.Resolved) != 1 || state.Resolved[0].Resolution != "resolved by removal" {
				t.Fatalf("hold not resolved: %+v %v", state, err)
			}
			q, err = channel.ReadQuestion(bed.root(), stops[0].Question)
			if err != nil || q.State != "closed" {
				t.Fatalf("drift question not closed: %+v %v", q, err)
			}
			if bands["unit elapsed minutes"] != "above band" {
				t.Fatalf("cost disappeared: %v", bands)
			}
			// Refreshing the same sunk cost must not recreate the hold.
			_, _, _ = undoSettings(t, bed, false, "set", applied.Key, "danger-full-access", "--goal", bed.id)
			current, bands := driftStatus(t, bed)
			if len(current) != 0 || bands["unit elapsed minutes"] != "above band" {
				t.Fatalf("sunk cost reopened hold: %+v %v", current, bands)
			}
			// A new physical execution has its own stop, even within the same claim.
			clock.Sleep(time.Second)
			bed.worktree = filepath.Join(filepath.Dir(bed.worktree), "next-execution")
			if err := os.MkdirAll(bed.worktree, 0700); err != nil {
				t.Fatal(err)
			}
			bed.manager.Settings, err = launch.ResolveSettings(conf, func(string) (string, bool) { return "", false })
			if err != nil {
				t.Fatal(err)
			}
			bed.manager.Settings.BuildRuntime = "codex"
			if code, result, output := processEvidenceBuild(bed); code != 0 {
				t.Fatalf("new execution: %d %+v %s", code, result, output)
			}
			fresh, _ := driftStatus(t, bed)
			if len(fresh) != 1 || fresh[0].ID == stops[0].ID {
				t.Fatalf("new execution did not open new episode: %+v", fresh)
			}
			t.Logf("%s; inverse %s preserved original %s and resolved its hold; settings show source %s", want, inverse.ID, applied.ID, source)
		})
	}
}

func init() {
	registerIdempotency("settings unset", idemCreation, "each explicit undo records a new inverse act; an inherited setting's bytes stay unchanged", nil)
}

func TestProcessSettingUndoIdentityPublicVerb(t *testing.T) {
	t.Parallel()
	b, local := processSettingBed(t)
	b.owners.prove = enrolledPersonProver(t, b.seat, b.now)
	code, result, _ := b.run(t, b.seat, "set", "launch.codex.sandbox", "danger-full-access")
	original := processAct(t, result)
	if code != 0 || original.ID == "" {
		t.Fatalf("original: %d %+v", code, result)
	}
	before, err := os.ReadFile(local)
	if err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"unset", original.Key, "--undo", original.ID},
		{"set", original.Key, original.After, "--undo", original.ID},
		{"set", "launch.read.model", "workspace-write", "--undo", original.ID},
		{"set", original.Key, "workspace-write", "--undo", "../outside"},
		{"set", original.Key, "workspace-write", "--undo", original.ID, "--act", original.ID},
		{"set", original.Key, "workspace-write", "--undo", original.ID, "--repo", b.lane},
	} {
		if code, result, _ := b.run(t, b.seat, args[0], args[1:]...); code == 0 {
			t.Fatalf("invalid reverse succeeded: %v %+v", args, result)
		}
		after, err := os.ReadFile(local)
		if err != nil || !bytes.Equal(before, after) {
			t.Fatalf("invalid reverse damaged settings: %v %s %v", args, after, err)
		}
	}
	// A later person's value does not take away their power to undo explicitly.
	b.now = b.now.Add(time.Second)
	if code, result, _ := b.run(t, b.seat, "set", original.Key, "workspace-write"); code != 0 {
		t.Fatalf("later act: %+v", result)
	}
	if code, result, _ := b.run(t, b.seat, "set", original.Key, original.After, "--act", original.ID); code != 1 || processAct(t, result).Status != "superseded" {
		t.Fatalf("older applied act was not superseded: %d %+v", code, result)
	}
	// Unreadable advisory stops must not veto a person's explicit reverse.
	broken := filepath.Join(b.seat, "process", "episodes", "damaged", "stops", "damaged.json")
	if err := os.MkdirAll(filepath.Dir(broken), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(broken, []byte("unreadable history"), 0600); err != nil {
		t.Fatal(err)
	}
	b.now = b.now.Add(time.Second)
	if code, result, _ := b.run(t, b.seat, "set", original.Key, "workspace-write", "--undo", original.ID); code != 0 || processAct(t, result).Undo != original.ID {
		t.Fatalf("person's reverse vetoed by later state or advisory damage: %d %+v", code, result)
	}
}

func TestProcessSettingConsumptionPublicStatus(t *testing.T) {
	t.Parallel()
	bed := newWorkBed(t)
	clock := &workClock{now: time.Date(2026, 10, 8, 10, 0, 0, 0, time.UTC)}
	bed.manager.Now, bed.manager.Supervisor = clock.Now, processCostStarter{bed.starter, clock}
	if err := os.MkdirAll(filepath.Join(bed.root(), ".git"), 0700); err != nil {
		t.Fatal(err)
	}
	conf := filepath.Join(bed.root(), "settings.conf")
	if err := os.WriteFile(conf, []byte("launch.codex.sandbox=workspace-write\n"), 0600); err != nil {
		t.Fatal(err)
	}
	processEstimatePage(t, bed, "1", "100")
	code, result, _ := undoSettings(t, bed, true, "set", "launch.codex.sandbox", "danger-full-access", "--goal", bed.id)
	if code != 0 {
		t.Fatalf("setting: %+v", result)
	}
	// Equal values alone do not establish consumption of a recorded revision.
	file, err := os.OpenFile(conf+".local", os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.WriteString("# unrecorded settings revision\n"); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	bed.manager.Settings, err = launch.ResolveSettings(conf, func(string) (string, bool) { return "", false })
	if err != nil {
		t.Fatal(err)
	}
	bed.manager.Settings.BuildRuntime = "codex"
	if code, result, output := processEvidenceBuild(bed); code != 0 {
		t.Fatalf("build: %d %+v %s", code, result, output)
	}
	stops, _ := driftStatus(t, bed)
	if len(stops) != 1 || stops[0].Stop.Cause == nil || stops[0].Stop.Cause.Kind != "unclassified" {
		t.Fatalf("unrecorded revision stole attribution: %+v", stops)
	}
}
