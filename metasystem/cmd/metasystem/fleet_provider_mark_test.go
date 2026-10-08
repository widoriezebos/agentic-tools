package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/adapter"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/hostcapacity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/outage"
)

func TestFleetProviderMarkPublicLifecycle(t *testing.T) {
	t.Parallel()
	b := newMachineBed(t)
	now := machineBedNow
	owners := b.owners()
	owners.landing.now = func() time.Time { return now }
	read := func(checkout string) hostcapacity.Snapshot {
		t.Helper()
		command, args, ok := resolveIntentArgv([]string{"machine", "list", "--json"})
		if !ok {
			t.Fatal("machine list is not declared")
		}
		var out, stderr bytes.Buffer
		code := runIntentIn(command, args, &out, &stderr, checkout, owners)
		var result struct {
			Data struct {
				ThisComputer struct{ Capacity hostcapacity.Snapshot }
			}
		}
		if err := json.Unmarshal(out.Bytes(), &result); err != nil || code != 0 {
			t.Fatalf("machine list exit %d: %s %s (%v)", code, out.String(), stderr.String(), err)
		}
		return result.Data.ThisComputer.Capacity
	}
	assertMarks := func(providers ...string) {
		t.Helper()
		first, second := read(b.this), read(b.other)
		if !reflect.DeepEqual(first.Providers, second.Providers) || !first.Providers.Available {
			t.Fatalf("seats disagree about provider coverage: %+v / %+v", first.Providers, second.Providers)
		}
		var actual []string
		for _, mark := range first.Providers.Marks {
			actual = append(actual, mark.Provider)
		}
		if !reflect.DeepEqual(actual, providers) {
			t.Fatalf("providers = %v, want %v", actual, providers)
		}
	}
	write := func(path, content string) {
		t.Helper()
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	// The ended launch is retained by the real landing launch store. Reaping
	// uses its recorded end, even when another seat reads the result later.
	store := launch.Store{Root: b.launchDir}
	record := launch.Record{ID: "landing-limited", Kind: launch.LandingKind, Adapter: "claude-headless", State: launch.Failed,
		WorkingDirectory: b.landing, FinishedAt: now.Format(time.RFC3339Nano), AdapterData: map[string]json.RawMessage{"model": json.RawMessage(`"landing-model"`)}}
	if err := store.Create(record); err != nil {
		t.Fatal(err)
	}
	dir, err := store.StateDir(record.ID)
	if err != nil {
		t.Fatal(err)
	}
	write(filepath.Join(dir, "result.json"), `{"is_error":true,"result":"You've hit your session limit, resets 12pm (Europe/Amsterdam)"}`)
	settings := launch.DefaultSettings()
	settings.LandingRuntime, settings.LandingModel = "claude", "landing-model"
	manager := &launch.Manager{Store: store, Now: func() time.Time { return now }, Prober: machineProber{dead: b.dead}, Processes: machineProcesses{dead: b.dead}}
	agent := landingAgent{manager: func() *launch.Manager { return manager }, now: func() time.Time { return now },
		settings: func(string) (launch.Settings, error) { return settings, nil }, machine: func(string) (string, error) { return "m1e", nil }}
	keeper := newLandingAgentKeeper(b.landing, b.home, agent)
	if err := keeper.Reap[0](record.ID); err != nil {
		t.Fatal(err)
	}
	assertMarks("anthropic")
	mark := read(b.other).Providers.Marks[0]
	if mark.Source != record.ID || mark.Runtime != record.Adapter || mark.Model != "landing-model" || mark.ResetAt != now.Add(24*time.Hour).Format(time.RFC3339Nano) {
		t.Fatalf("landing evidence lost its execution, adapter, model or observed reset: %+v", mark)
	}
	if line, err := keeper.ProviderHold(b.landing); err != nil || !strings.Contains(line, "provider") {
		t.Fatalf("automatic landing start ignores the shared hold: %q %v", line, err)
	}
	if _, err := os.Stat(filepath.Join(b.landing, "artifacts", "agents", "outage.json")); !os.IsNotExist(err) {
		t.Fatal("landing reaping wrote a checkout-local mark")
	}
	// Repairing an incomplete lane registration must retain this installation's limit.
	owner, _, err := lane.Read(b.home)
	if err != nil {
		t.Fatal(err)
	}
	oldEpoch := owner.CustodyEpoch
	owner.Install = ""
	data, err := json.Marshal(owner)
	if err != nil {
		t.Fatal(err)
	}
	write(lane.RecordPath(b.home), string(data))
	owners.landing.validate = func(string, string, time.Time) (string, error) {
		return b.landing, nil
	}
	owners.landing.ready = func(string) error { return nil }
	owners.landing.machine = func(string) (string, error) { return "landing", nil }
	owners.prove = enrolledPersonProver(t, b.this, now)
	owners.landing.person = provenPerson(person(), func() int64 { return 20 }, func(string) (time.Time, error) { return now, nil })
	command, args, ok := resolveIntentArgv([]string{"landing", "set", b.landing, "--by", "Wido", "--json"})
	if !ok {
		t.Fatal("landing set is not declared")
	}
	var out, stderr bytes.Buffer
	if code := runIntentIn(command, args, &out, &stderr, b.this, owners); code != 0 {
		t.Fatalf("landing set exit %d: %s %s", code, out.String(), stderr.String())
	}
	carried, err := outage.ReadProviders(b.home)
	if err != nil || carried.Owner.CustodyEpoch <= oldEpoch {
		t.Fatalf("registration repair: %+v %v", carried.Owner, err)
	}
	assertMarks("anthropic")
	if _, standing := carried.Standing("claude", now); !standing {
		t.Fatal("registration repair lost the standing limit")
	}
	// The second provider arrives through the production delegate adjudicator.
	job := filepath.Join(b.other, "delegate.json")
	log := filepath.Join(b.other, "delegate.log")
	write(job, `{"runtime":"codex","requestedModel":"codex-model"}`)
	write(log, "HTTP 429 Too Many Requests\n")
	p := adapter.AdjudicateParams{Runtime: "codex", Model: "codex-model", Home: b.home, ObservedAt: now.Add(time.Second), Root: b.other, Job: "delegate-limited", RecordPath: job,
		Stage: "initial", CLIStatus: 1, LogPath: log}
	if verdict, err := adapter.AdjudicateTurn(p); err != nil || verdict != "fail-pending runtime_error handshake" {
		t.Fatalf("delegate result: %q %v", verdict, err)
	}
	assertMarks("anthropic", "openai")
	if err := keeper.Reap[0](record.ID); err != nil {
		t.Fatal(err)
	}
	if got := read(b.this).Providers.Marks[0]; got.ConsecutiveFailures != 1 || got.LastAt != mark.LastAt {
		t.Fatalf("replayed landing result became fresh evidence: %+v", got)
	}
	// A genuine successful delegate response closes only its own provider.
	p.Stage, p.SettleOK, p.ObservedAt = "settle-result", true, now.Add(time.Second)
	if verdict, err := adapter.AdjudicateTurn(p); err != nil || verdict != "finish completed null completed" {
		t.Fatalf("delegate success: %q %v", verdict, err)
	}
	assertMarks("anthropic")
	p.Stage, p.CLIStatus, p.ObservedAt = "initial", 1, now.Add(time.Second)
	if _, err := adapter.AdjudicateTurn(p); err != nil {
		t.Fatal(err)
	}
	assertMarks("anthropic")
	state, err := outage.ReadProviders(b.home)
	if err != nil {
		t.Fatal(err)
	}
	closed := state.Current["openai"]
	if len(closed.Intervals) != 1 || closed.Intervals[0].Until != now.Add(time.Second).Format(time.RFC3339Nano) || closed.ClearedAt != closed.Intervals[0].Until {
		t.Fatalf("success did not retain its closed interval and watermark: %+v", closed)
	}
	// A successful landing response shares the same close and replay rules.
	now = now.Add(3 * time.Second)
	success := record
	success.ID, success.State, success.FinishedAt = "landing-recovered", launch.Completed, now.Format(time.RFC3339Nano)
	if err := store.Create(success); err != nil {
		t.Fatal(err)
	}
	successDir, err := store.StateDir(success.ID)
	if err != nil {
		t.Fatal(err)
	}
	write(filepath.Join(successDir, "result.json"), `{"is_error":false,"result":"ok"}`)
	if err := keeper.Reap[0](success.ID); err != nil {
		t.Fatal(err)
	}
	assertMarks()
	now = now.Add(time.Second)
	if err := keeper.Reap[0](record.ID); err != nil {
		t.Fatal(err)
	}
	assertMarks()
	if line, err := keeper.ProviderHold(b.landing); err != nil || line != "" {
		t.Fatalf("recovery left an automatic landing hold: %q %v", line, err)
	}
	// Landing adapters and settings runtimes identify the same provider.
	for _, c := range []struct{ adapter, runtime, provider string }{
		{"codex-exec", "openai", "openai"}, {"devin-print", "devin", "devin"},
	} {
		now = now.Add(time.Second)
		limited := record
		limited.ID, limited.Adapter, limited.FinishedAt = "limited-"+c.adapter, c.adapter, now.Format(time.RFC3339Nano)
		if err := store.Create(limited); err != nil {
			t.Fatal(err)
		}
		dir, err := store.StateDir(limited.ID)
		if err != nil {
			t.Fatal(err)
		}
		write(filepath.Join(dir, "result.json"), `{"is_error":true,"result":"HTTP 429 Too Many Requests"}`)
		if err := keeper.Reap[0](limited.ID); err != nil {
			t.Fatal(err)
		}
		assertMarks(c.provider)
		settings.LandingRuntime = c.runtime
		if line, err := keeper.ProviderHold(b.landing); err != nil || !strings.Contains(line, "provider") {
			t.Fatalf("%s hold: %q %v", c.adapter, line, err)
		}
		now = now.Add(time.Second)
		limited.ID, limited.State, limited.FinishedAt = "recovered-"+c.adapter, launch.Completed, now.Format(time.RFC3339Nano)
		if err := store.Create(limited); err != nil {
			t.Fatal(err)
		}
		if err := keeper.Reap[0](limited.ID); err != nil {
			t.Fatal(err)
		}
		assertMarks()
	}
	state, err = outage.ReadProviders(b.home)
	if err != nil {
		t.Fatal(err)
	}
	// Corrupt state and lost registration remain visible unknowns.
	statePath := filepath.Join(b.landing, "artifacts", "agents", fmt.Sprintf("providers-%d.json", state.Owner.CustodyEpoch))
	for _, corrupt := range []string{"{", "{}", "null"} {
		write(statePath, corrupt)
		if got := read(b.this); got.Providers.Available || !strings.Contains(got.Providers.Detail, "unknown") {
			t.Fatalf("corrupt provider state became healthy: %+v", got.Providers)
		}
	}
	if err := keeper.Reap[0](record.ID); err != nil {
		t.Fatalf("provider hint failure held reaping: %v", err)
	}
	if err := os.Remove(lane.RecordPath(b.home)); err != nil {
		t.Fatal(err)
	}
	if got := read(b.other); got.OwnerKnown || got.Providers.Available || !strings.Contains(got.Providers.Detail, "unknown") {
		t.Fatalf("registration loss became healthy: %+v", got)
	}
}
