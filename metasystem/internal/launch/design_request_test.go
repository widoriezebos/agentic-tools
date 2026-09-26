package launch

import (
	"errors"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func designFixture(t *testing.T) (*Manager, *completingStarter, DesignRequest) {
	t.Helper()
	m, _, probe, now := manager(t)
	// The fixture starter completes each launch at once; its supervisor has
	// then exited.
	probe.states[10] = identity.Dead
	m.Adapters["claude-headless"], m.Adapters["plain-exec"], m.Adapters["codex-exec"] = fakeAdapter{}, fakeAdapter{}, fakeAdapter{}
	m.Settings = DefaultSettings()
	m.Settings.WaitCapSeconds = 2
	m.StartCap = time.Minute
	m.Now = func() time.Time { return *now }
	starter := &completingStarter{m: m}
	m.Supervisor = starter
	checkout := t.TempDir()
	destination := filepath.Join(checkout, "plans", "designs", "g.md")
	return m, starter, DesignRequest{Goal: "g", RecordID: "01M3EFDSFTKWEMSDCP1BB7TDGA", Destination: destination,
		WorkingDirectory: checkout, Brief: []byte("Design the reader.\n"), Contract: []byte("Write a draft design record.\n"),
		Header: []byte("# g design\n\n- Kind: design\n- Id: 01M3EFDSFTKWEMSDCP1BB7TDGA\n- Status: draft\n- Goals: g\n")}
}

// TestDesignRequestReplay: an identical request rejoins its attempt without
// another launch, a new brief makes one new attempt against the document's
// current bytes, and a request after an older attempt is refused as stale.
func TestDesignRequestReplay(t *testing.T) {
	t.Parallel()
	m, starter, request := designFixture(t)
	first, err := m.RequestDesign(request)
	if err != nil || first.Rejoined || first.Attempt.Attempt != 1 || len(starter.ids) != 1 || first.Record.Kind != "design" {
		t.Fatalf("first request: err=%v rejoined=%v attempt=%d launches=%d kind=%q", err, first.Rejoined, first.Attempt.Attempt, len(starter.ids), first.Record.Kind)
	}
	again, err := m.RequestDesign(request)
	if err != nil || !again.Rejoined || again.Attempt.LaunchID != first.Attempt.LaunchID || len(starter.ids) != 1 {
		t.Fatalf("replay: %+v %v launches=%d", again, err, len(starter.ids))
	}
	os.MkdirAll(filepath.Dir(request.Destination), 0o755)
	os.WriteFile(request.Destination, []byte("edited by the person\n"), 0o644)
	changed := request
	changed.Brief = []byte("Design the reader and the writer.\n")
	second, err := m.RequestDesign(changed)
	if err != nil || second.Attempt.Attempt != 2 || !second.Attempt.ExpectedPresent || len(starter.ids) != 2 {
		t.Fatalf("new request: %+v %v", second, err)
	}
	if prior, _ := os.ReadFile(second.Attempt.Prior); string(prior) != "edited by the person\n" {
		t.Fatalf("the frozen prior is not the current document: %q", prior)
	}
	// The author's prompt names its one output and the frozen prior; the
	// first attempt's prompt carries the reserved head instead.
	for _, attempt := range []DesignAttempt{first.Attempt, second.Attempt} {
		prompt, _ := os.ReadFile(attempt.Brief)
		if !strings.Contains(string(prompt), DesignOutputHeading+"\n\nWrite the complete design record to this one file") ||
			!strings.Contains(string(prompt), "    "+attempt.Draft+"\n") || !strings.Contains(string(prompt), "\n## The request\n\n") {
			t.Fatalf("attempt %d's prompt does not name its output %s:\n%s", attempt.Attempt, attempt.Draft, prompt)
		}
		if names := strings.Contains(string(prompt), "    "+attempt.Prior+"\n"); attempt.Prior != "" && !names ||
			attempt.Prior == "" && !strings.Contains(string(prompt), "exactly this head:\n\n"+string(request.Header)) {
			t.Fatalf("attempt %d's prompt does not name its starting point:\n%s", attempt.Attempt, prompt)
		}
	}
	// The first request again still rejoins attempt 1, launching nothing.
	if old, err := m.RequestDesign(request); err != nil || !old.Rejoined || old.Attempt.Attempt != 1 || old.Current != 2 || len(starter.ids) != 2 {
		t.Fatalf("old request after a newer attempt: %+v %v", old, err)
	}
	stale := changed
	stale.After, stale.Brief = 1, []byte("another\n")
	if _, err := m.RequestDesign(stale); !errors.Is(err, ErrDesignStale) || len(starter.ids) != 2 {
		t.Fatalf("a request after attempt 1: %v", err)
	}
}

// TestDesignRequestCrashBeforeSupervisor: a launch whose supervisor never
// claimed it is failed only atomically under the record lock; a supervisor
// that claims first makes recovery refuse, a supervisor that arrives after
// recovery refuses the terminal record before any child, and an uncertain
// output owner or a young record is never recovered.
func TestDesignRequestCrashBeforeSupervisor(t *testing.T) {
	t.Parallel()
	m, _, _, now := manager(t)
	m.StartCap = time.Minute
	m.Now = func() time.Time { return *now }
	create := func(id string, change func(*Record)) {
		t.Helper()
		record := Record{ID: id, Kind: "design", State: Starting, StartedAt: now.UTC().Format(time.RFC3339Nano), WorkingDirectory: t.TempDir()}
		if change != nil {
			change(&record)
		}
		if err := m.Store.Create(record); err != nil {
			t.Fatal(err)
		}
	}
	create("20260917t120000-0000000001", nil)
	if _, recovered, _ := m.RecoverStrandedStart("20260917t120000-0000000001"); recovered {
		t.Fatal("a launch younger than the start cap was recovered")
	}
	*now = now.Add(2 * time.Minute)
	record, recovered, err := m.RecoverStrandedStart("20260917t120000-0000000001")
	if err != nil || !recovered || record.State != Failed || record.Reason != "supervisor-start-unrecorded" {
		t.Fatalf("recovery winner: %+v %v %v", record, recovered, err)
	}
	if _, err := m.Supervise("20260917t120000-0000000001"); err == nil || !strings.Contains(err.Error(), "LAUNCH_ALREADY_SUPERVISED") {
		t.Fatalf("a late supervisor after recovery must refuse before any child: %v", err)
	}
	// The supervisor wins: its claim is recorded first, recovery refuses.
	create("20260917t120000-0000000002", nil)
	if _, err := m.Store.Update("20260917t120000-0000000002", func(r *Record) error { self := ref(10); r.Supervisor = &self; return nil }); err != nil {
		t.Fatal(err)
	}
	if record, recovered, _ := m.RecoverStrandedStart("20260917t120000-0000000002"); recovered || record.State != Starting {
		t.Fatalf("recovery after a supervisor claim: %+v", record)
	}
	// Uncertain output custody is never erased by recovery.
	create("20260917t120000-0000000003", func(r *Record) {
		r.OutputOwnerUnproven = true
		r.StartedAt = now.Add(-time.Hour).UTC().Format(time.RFC3339Nano)
	})
	if record, recovered, _ := m.RecoverStrandedStart("20260917t120000-0000000003"); recovered || record.State != Starting {
		t.Fatalf("recovery with an uncertain output owner: %+v", record)
	}
}

// TestDesignRequestEntryUsesTheResolvedStore: production builds its store
// with an empty Root that resolves to the default launch root; the retained
// entry, frozen prior and prompt must live there, never relative to the
// directory the caller runs in, so the author's prompt names absolute paths.
func TestDesignRequestEntryUsesTheResolvedStore(t *testing.T) {
	t.Parallel()
	m := &Manager{}
	root, err := DefaultRoot()
	if err != nil {
		t.Skipf("no default launch root here: %v", err)
	}
	directory := m.designDir("/checkout/plans/designs/g.md")
	if !filepath.IsAbs(directory) || !strings.HasPrefix(directory, filepath.Clean(root)+string(filepath.Separator)) {
		t.Fatalf("the design entry directory %q is not under the resolved launch root %q", directory, root)
	}
}
