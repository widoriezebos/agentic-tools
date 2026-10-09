package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/board"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/hostcapacity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/hostload"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/outage"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/httpd"
)

func TestFleetRecoveryDuePublicMachineList(t *testing.T) {
	t.Parallel()
	b := newMachineBed(t)
	owners := b.owners()
	now := machineBedNow
	owners.landing.now = func() time.Time { return now }
	write := func(interval string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(b.landing, "metasystem.conf"), []byte("provider.recovery-alert-after="+interval+"\n"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	write("9m")
	if _, err := outage.Observe(b.home, "claude-headless", "fixture", outage.ProviderLimit, "HTTP 429", "failed", now); err != nil {
		t.Fatal(err)
	}
	read := func(want string) {
		t.Helper()
		for _, format := range []string{"--json", "--verbose"} {
			command, args, _ := resolveIntentArgv([]string{"machine", "list", format})
			var out, stderr bytes.Buffer
			if code := runIntentIn(command, args, &out, &stderr, b.this, owners); code != 0 {
				t.Fatalf("machine list exit %d: %s", code, stderr.String())
			}
			if format == "--json" {
				var envelope struct {
					Data struct {
						ThisComputer struct{ Capacity hostcapacity.Snapshot }
					}
				}
				if err := json.Unmarshal(out.Bytes(), &envelope); err != nil {
					t.Fatal(err)
				}
				recovery := envelope.Data.ThisComputer.Capacity.Providers.Recovery
				if len(recovery) != 1 || recovery[0].Provider != "anthropic" || recovery[0].Interval != "9m" || recovery[0].Due != want {
					t.Fatalf("recovery observation: %+v, want due %s", recovery, want)
				}
			} else if !strings.Contains(out.String(), "interval 9m") || !strings.Contains(out.String(), "due "+laneDueText(want)) {
				t.Fatalf("recovery interval/due hidden: %s", out.String())
			}
		}
	}
	read("pending success")
	write("1m")
	now = now.Add(time.Minute)
	if err := outage.Clear(b.home, "anthropic", now); err != nil {
		t.Fatal(err)
	}
	read("pending success")
	now = now.Add(time.Minute)
	if _, err := outage.Observe(b.home, "claude-headless", "fixture", "", "", "successful-probe", now); err != nil {
		t.Fatal(err)
	}
	read(now.Add(9 * time.Minute).UTC().Format(time.RFC3339Nano))
}

func laneDueText(value string) string {
	if at, err := time.Parse(time.RFC3339Nano, value); err == nil {
		return at.Local().Format("2006-01-02 15:04 MST")
	}
	return value
}

func TestFleetUsagePublicViews(t *testing.T) {
	t.Parallel()
	b := newMachineBed(t)
	b.launchDir = t.TempDir()
	now := machineBedNow
	projects, rollouts := t.TempDir(), t.TempDir()
	store := launch.Store{Root: b.launchDir}
	m := &launch.Manager{Store: store, Now: func() time.Time { return now }, Prober: machineProber{dead: b.dead},
		Adapters: map[string]launch.Adapter{"claude-headless": launch.ClaudeHeadless{ProjectsRoot: projects}, "codex-exec": launch.CodexExec{SessionsRoot: rollouts}}}
	owners := b.owners()
	owners.processes.launches = func() *launch.Manager { return m }
	owners.landing.now = func() time.Time { return now }
	owners.machines.capacitySources.Load = func(time.Time) hostload.Sample { return hostload.Sample{Available: true, Cores: 4} }
	sources := owners.machines.capacitySources
	sources.RegistryPath = b.registry
	sources.Usage = m.CapacityUsage
	api := httpd.New(httpd.Info{Now: func() time.Time { return now }, Board: &httpd.BoardSource{
		Home: b.home, Seats: func() ([]board.Seat, error) { return []board.Seat{}, nil },
		Capacity: func(at time.Time) hostcapacity.Snapshot { return hostcapacity.Read(b.home, m, at, sources) },
	}}, &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 7878}, fstest.MapFS{})
	write := func(path, content string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
	}
	call := func(id, timestamp string, n int) string {
		return fmt.Sprintf(`{"type":"assistant","timestamp":%q,"message":{"id":%q,"usage":{"input_tokens":%d,"cache_read_input_tokens":%d,"cache_creation_input_tokens":%d,"output_tokens":%d}}}`+"\n", timestamp, id, n, n*2, n*3, n*4)
	}
	stamp := func(at time.Time) string { return at.Format(time.RFC3339Nano) }
	long := call("old", stamp(now.Add(-2*time.Hour)), 1) + call("start", stamp(now.Add(-time.Hour)), 2) + call("recent", stamp(now.Add(-time.Minute)), 3) + call("end", stamp(now), 4) + call("future", stamp(now.Add(time.Minute)), 5)
	write(filepath.Join(projects, "seat-one", "active.jsonl"), long)
	write(filepath.Join(projects, "resumed-copy", "active.jsonl"), long)
	write(filepath.Join(projects, "seat-two", "ended.jsonl"), long)
	write(filepath.Join(projects, "failed.jsonl"), call("failed", stamp(now.Add(-time.Minute)), 10))
	write(filepath.Join(projects, "cancelled.jsonl"), call("cancelled", stamp(now.Add(-time.Minute)), 20))
	write(filepath.Join(projects, "bad-time.jsonl"), call("missing", "", 30)+call("invalid", "yesterday", 40))
	seed := func(id, session, runtime, checkout string, state launch.State) {
		t.Helper()
		if err := store.Create(launch.Record{ID: id, Adapter: runtime, State: state, Supervisor: &identity.Ref{Pid: 556, StartedAtSec: 556}, WorkingDirectory: checkout, StartedAt: stamp(now.Add(-24 * time.Hour)), AdapterData: map[string]json.RawMessage{"sessionID": json.RawMessage(fmt.Sprintf("%q", session))}}); err != nil {
			t.Fatal(err)
		}
	}
	seed("active", "active", "claude-headless", b.this, launch.Running)
	seed("resume", "active", "claude-headless", b.this, launch.Completed)
	seed("ended", "ended", "claude-headless", b.other, launch.Completed)
	seed("failed", "failed", "claude-headless", b.other, launch.Failed)
	seed("cancelled", "cancelled", "claude-headless", b.this, launch.Cancelled)
	seed("bad-time", "bad-time", "claude-headless", b.this, launch.Completed)
	seed("missing", "missing", "claude-headless", b.this, launch.Failed)
	seed("no-id", "", "claude-headless", b.other, launch.Cancelled)
	seed("codex", "codex", "codex-exec", b.this, launch.Running)
	seed("codex-resume", "codex", "codex-exec", b.this, launch.Completed)
	var codex string
	for i, at := range []time.Time{now.Add(-2 * time.Hour), now.Add(-time.Hour), now.Add(-time.Minute), now} {
		n := (i + 1) * 10
		row := fmt.Sprintf(`{"type":"event_msg","timestamp":%q,"payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":%d},"total_token_usage":{"input_tokens":%d,"cached_input_tokens":%d,"cache_write_input_tokens":%d,"output_tokens":%d}}}}`+"\n", stamp(at), n, n, n*2, n*3, n*4)
		codex += row + row
	}
	write(filepath.Join(rollouts, "rollout-fixture-codex.jsonl"), codex)
	if _, err := outage.Observe(b.home, "claude-headless", "fixture", outage.ProviderLimit, "You've hit your session limit, resets 12pm (Europe/Amsterdam)", "provider-failure", now.Add(-time.Hour)); err != nil {
		t.Fatal(err)
	}
	var beforeRead func()
	read := func() hostcapacity.Snapshot {
		t.Helper()
		if beforeRead != nil {
			beforeRead()
		}
		command, args, _ := resolveIntentArgv([]string{"machine", "list", "--json"})
		var out, stderr bytes.Buffer
		code := runIntentIn(command, args, &out, &stderr, b.this, owners)
		var cli struct {
			Data struct {
				ThisComputer struct{ Capacity hostcapacity.Snapshot }
			}
		}
		if err := json.Unmarshal(out.Bytes(), &cli); err != nil || code != 0 {
			t.Fatalf("machine list exit %d: %s %s (%v)", code, out.String(), stderr.String(), err)
		}
		response := httptest.NewRecorder()
		if beforeRead != nil {
			beforeRead()
		}
		api.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "http://127.0.0.1:7878/api/board", nil))
		var board struct{ Capacity *hostcapacity.Snapshot }
		if err := json.Unmarshal(response.Body.Bytes(), &board); err != nil || response.Code != 200 || board.Capacity == nil {
			t.Fatalf("GET /api/board: %d %s (%v)", response.Code, response.Body.String(), err)
		}
		if !reflect.DeepEqual(cli.Data.ThisComputer.Capacity.Usage, board.Capacity.Usage) || cli.Data.ThisComputer.Capacity.At != board.Capacity.At {
			t.Fatalf("public views disagree: %+v / %+v", cli.Data.ThisComputer.Capacity, board.Capacity)
		}
		return cli.Data.ThisComputer.Capacity
	}
	view := read()
	if !view.OwnerKnown || len(view.Usage.Sessions) != 8 || view.Usage.From != stamp(now.Add(-time.Hour)) || view.Usage.Until != stamp(now) || view.Usage.AccountWindow != nil || view.Usage.AccountLimit != nil || view.Usage.AccountNumerator != nil || view.Usage.AccountPercent != nil {
		t.Fatalf("usage bounds, deduplication or invented account limits: %+v", view)
	}
	find := func(view hostcapacity.Snapshot, id string) hostcapacity.SessionUsage {
		t.Helper()
		for _, session := range view.Usage.Sessions {
			if session.Session == id {
				return session
			}
		}
		t.Fatalf("session %q hidden: %+v", id, view.Usage)
		return hostcapacity.SessionUsage{}
	}
	wantHour := &hostcapacity.Tokens{Calls: 2, Input: 5, CacheRead: 10, CacheCreation: 15, Output: 20, PeakContext: 18}
	wantTotal := &hostcapacity.Tokens{Calls: 3, Input: 6, CacheRead: 12, CacheCreation: 18, Output: 24, PeakContext: 18}
	for _, id := range []string{"active", "ended"} {
		session := find(view, id)
		if !reflect.DeepEqual(session.Totals, wantTotal) || !reflect.DeepEqual(session.TrailingHour, wantHour) || len(session.Problems) != 0 || session.Provisional != (id == "active") || session.Provider != "anthropic" {
			t.Fatalf("24-hour %s session: %+v", id, session)
		}
	}
	for id, n := range map[string]int64{"failed": 10, "cancelled": 20} {
		session := find(view, id)
		if session.Totals == nil || session.Totals.Input != n || session.Totals.Calls != 1 || !reflect.DeepEqual(session.Totals, session.TrailingHour) {
			t.Fatalf("unsuccessful session hidden: %+v", session)
		}
	}
	c := find(view, "codex")
	if c.Provider != "openai" || !c.Provisional || c.WorkingDirectory != b.this || !reflect.DeepEqual(c.Totals, &hostcapacity.Tokens{Calls: 3, Input: 30, CacheRead: 60, CacheCreation: 90, Output: 120, PeakContext: 30}) || !reflect.DeepEqual(c.TrailingHour, &hostcapacity.Tokens{Calls: 2, Input: 20, CacheRead: 40, CacheCreation: 60, Output: 80, PeakContext: 30}) || len(c.Problems) != 0 {
		t.Fatalf("actual provider/cumulative codex calls: %+v", c)
	}
	if missing := find(view, "missing"); missing.Totals != nil || missing.TrailingHour != nil || len(missing.Problems) == 0 {
		t.Fatalf("missing transcript became zero: %+v", missing)
	}
	if bad := find(view, "bad-time"); bad.Totals == nil || bad.Totals.Input != 70 || !strings.Contains(strings.Join(bad.Problems, " "), "2 calls") {
		t.Fatalf("invalid timestamps hidden: %+v", bad)
	}
	if absent := find(view, ""); absent.Totals != nil || !strings.Contains(strings.Join(absent.Problems, " "), "session identity") {
		t.Fatalf("missing identity hidden: %+v", absent)
	}
	if _, err := m.Usage("active"); err == nil {
		t.Fatal("active observation changed ended-only final usage contract")
	}
	command, args, _ := resolveIntentArgv([]string{"machine", "list", "--verbose"})
	var text, stderr bytes.Buffer
	if code := runIntentIn(command, args, &text, &stderr, b.this, owners); code != 0 {
		t.Fatalf("text machine list exit %d: %s", code, stderr.String())
	}
	for _, required := range []string{"session totals", "trailing hour", "provisional", "account", "percentage unknown", "cache read", "cache creation", "peak context", "missing or invalid timestamps"} {
		if !strings.Contains(strings.Join(strings.Fields(text.String()), " "), required) {
			t.Fatalf("text view hides %q: %s", required, text.String())
		}
	}
	seed("conflict", "active", "claude-headless", b.other, launch.Completed)
	if conflicting := find(read(), "active"); !strings.Contains(strings.Join(conflicting.Problems, " "), "conflicting working directory") {
		t.Fatalf("conflicting attribution hidden: %+v", conflicting)
	}
	seed("unregistered", "ended", "claude-headless", "/unregistered", launch.Completed)
	seed("unknown-checkout", "failed", "claude-headless", "/unregistered", launch.Failed)
	write(filepath.Join(projects, "unregistered.jsonl"), call("unregistered", stamp(now.Add(-time.Minute)), 7))
	seed("unknown-registration", "unregistered", "claude-headless", "/unregistered", launch.Completed)
	if unknown := find(read(), "unregistered"); !strings.Contains(strings.Join(unknown.Problems, " "), "registration is unknown") {
		t.Fatalf("unregistered usage appears attributed: %+v", unknown)
	}
	write(filepath.Join(rollouts, "rollout-fixture-codex.jsonl"), "{\n")
	if corrupt := find(read(), "codex"); corrupt.Totals != nil || len(corrupt.Problems) == 0 {
		t.Fatalf("corrupt rollout became zero: %+v", corrupt)
	}
	owners.machines.capacitySources.Load = func(at time.Time) hostload.Sample {
		registerLane(t, b.home, b.other, "Wido", at)
		return hostload.Sample{Available: true}
	}
	sources.Load = owners.machines.capacitySources.Load
	beforeRead = func() { registerLane(t, b.home, b.landing, "Wido", now) }
	changed := read()
	if changed.OwnerKnown || !strings.Contains(strings.Join(changed.Usage.Problems, " "), "ownership changed") {
		t.Fatalf("registration replacement hid coverage: %+v", changed)
	}
}

func TestFleetUsageDefaultSummary(t *testing.T) {
	t.Parallel()
	b := newMachineBed(t)
	b.launchDir = t.TempDir()
	projects := t.TempDir()
	store := launch.Store{Root: b.launchDir}
	for i := range 100 {
		session := fmt.Sprintf("session-%03d", i)
		if err := os.WriteFile(filepath.Join(projects, session+".jsonl"), []byte(fmt.Sprintf(`{"type":"assistant","timestamp":%q,"message":{"id":%q,"usage":{"input_tokens":1}}}`+"\n", machineBedNow.Add(-time.Minute).Format(time.RFC3339Nano), session)), 0600); err != nil {
			t.Fatal(err)
		}
		for _, runtime := range []string{"claude-headless", "plain-exec"} {
			if err := store.Create(launch.Record{ID: runtime + session, Adapter: runtime, State: launch.Completed, WorkingDirectory: b.this, FinishedAt: machineBedNow.Format(time.RFC3339Nano), AdapterData: map[string]json.RawMessage{"sessionID": json.RawMessage(fmt.Sprintf("%q", session))}}); err != nil {
				t.Fatal(err)
			}
		}
	}
	// Old retained evidence must not be scanned: a corrupt transcript would otherwise be reported.
	for _, session := range []string{"old", "empty"} {
		finish := machineBedNow
		content := "{}\n"
		if session == "old" {
			finish = finish.Add(-2 * time.Hour)
			content = "{\n"
		}
		if err := os.WriteFile(filepath.Join(projects, session+".jsonl"), []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
		if err := store.Create(launch.Record{ID: session, Adapter: "claude-headless", State: launch.Completed, WorkingDirectory: b.this, FinishedAt: finish.Format(time.RFC3339Nano), AdapterData: map[string]json.RawMessage{"sessionID": json.RawMessage(fmt.Sprintf("%q", session))}}); err != nil {
			t.Fatal(err)
		}
	}
	owners := b.owners()
	owners.processes.launches = func() *launch.Manager {
		return &launch.Manager{Store: store, Now: func() time.Time { return machineBedNow }, Adapters: map[string]launch.Adapter{"claude-headless": launch.ClaudeHeadless{ProjectsRoot: projects}}}
	}
	outputs := map[string]string{}
	for _, format := range []string{"", "--verbose", "--json"} {
		argv := []string{"machine", "list"}
		if format != "" {
			argv = append(argv, format)
		}
		command, args, _ := resolveIntentArgv(argv)
		var out, stderr bytes.Buffer
		if code := runIntentIn(command, args, &out, &stderr, b.this, owners); code != 0 {
			t.Fatalf("machine list %s exit %d: %s", format, code, stderr.String())
		}
		outputs[format] = out.String()
	}
	if lines := strings.Count(outputs[""], "\n"); lines > 40 || strings.Contains(outputs[""], "session-000") || strings.Count(outputs[""], "100 sessions") != 1 {
		t.Fatalf("default output is not one seat/provider summary (%d lines): %s", lines, outputs[""])
	}
	if !strings.Contains(outputs["--verbose"], "anthropic session session-000") || strings.Contains(outputs["--verbose"], "exec session") {
		t.Fatalf("verbose session detail: %s", outputs["--verbose"])
	}
	var envelope struct {
		Data struct {
			ThisComputer struct{ Capacity hostcapacity.Snapshot }
		}
	}
	if err := json.Unmarshal([]byte(outputs["--json"]), &envelope); err != nil {
		t.Fatal(err)
	}
	if sessions := envelope.Data.ThisComputer.Capacity.Usage.Sessions; len(sessions) != 100 {
		t.Fatalf("plain executions, old or empty sessions included: %d sessions", len(sessions))
	}
}
