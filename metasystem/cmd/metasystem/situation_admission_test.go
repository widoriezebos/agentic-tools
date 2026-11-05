package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
)

type admissionSituationProber struct{ owner identity.Exact }

func (p admissionSituationProber) Probe(pid int64) (identity.Exact, identity.Liveness, error) {
	if pid == p.owner.Pid {
		return p.owner, identity.Alive, nil
	}
	return identity.Exact{Pid: pid, StartedAt: situationNow}, identity.Alive, nil
}

func TestThePageSaysWhoHoldsWhoWaitsAndWhoBypasses(t *testing.T) {
	bed := newWorkBed(t)
	root := bed.root()
	conf := filepath.Join(root, "metasystem.conf")
	writeTestingFixtureFile(t, conf, []byte("metasystem.runtimes=fake\n"+proofrun.AdmissionCapKey+"=1\n"), 0o600)
	directory := filepath.Join(root, "admission")
	t.Setenv("METASYSTEM_PROOF_ADMISSION_TEST_DIR", directory)
	t.Setenv("METASYSTEM_PROOF_ADMISSION_FIXTURE_ROOT", root)
	holder, err := proofrun.AcquireHostResources(proofrun.WithLaneCheckPriority(t.Context()), root, conf, "heavy", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer holder.Close()
	owner, err := proofrun.CurrentProcessIdentity(nil)
	if err != nil {
		t.Fatal(err)
	}
	exact, _, err := (identity.KernelProber{}).Probe(owner.Pid)
	if err != nil {
		t.Fatal(err)
	}
	waiting, done := make(chan struct{}), make(chan struct{})
	ctx, cancel := context.WithCancel(t.Context())
	ctx = proofrun.WithHostResourceWaitObserver(ctx, func() { close(waiting) })
	var waiterErr error
	go func() {
		defer close(done)
		lease, err := proofrun.AcquireHostResources(ctx, root, conf, "heavy", nil)
		waiterErr = err
		if lease != nil {
			_ = lease.Close()
		}
	}()
	defer func() { cancel(); <-done }()
	// The observer fires after the failed scan publishes its locked marker.
	select {
	case <-waiting:
	case <-done:
		t.Fatalf("waiter returned before its failed scan: %v", waiterErr)
	}
	clock := situationNow
	owners := bed.workOwners()
	src := situationEmptyHost(t, steward.SituationSources{Now: func() time.Time { return clock }})
	src.Admission = nil // The production default reads the selected namespace.
	src.Registry = func() (string, error) { return situationArmed(t, root), nil }
	src.Processes = identity.FixedProcessTable{{Pid: 7011, Parent: 7010}, {Pid: 7010, Parent: owner.Pid}, {Pid: 7022, Parent: 1}, {Pid: 7033, Parent: owner.Pid}, {Pid: 7044, Parent: owner.Pid}}
	src.Argv = func(pid int64) ([]string, bool) {
		if pid == owner.Pid {
			return []string{"metasystem", "test", "run", "--goal", "g"}, true
		}
		if pid == 7010 || pid == 7044 {
			return []string{"editor"}, true
		}
		return []string{"go", "test", "./..."}, true
	}
	src.Cwd = func(pid int64) (string, bool) {
		if pid == 7033 {
			return "/elsewhere", true
		}
		return filepath.Join(root, "artifacts", "agents", "workspaces", "g", "build"), true
	}
	src.Prober = admissionSituationProber{exact}
	owners.situation = src
	command, _, _ := resolveIntentArgv([]string{"system", "situation"})
	run := func(args ...string) string {
		t.Helper()
		var out, stderr bytes.Buffer
		if code := runIntentIn(command, args, &out, &stderr, root, owners); code != 0 {
			t.Fatalf("situation = %d: %s%s", code, out.String(), stderr.String())
		}
		return out.String()
	}
	page := run("--verbose")
	for _, want := range []string{"test slots: 1 held, 1 waiting", "1 bypass", "pid " + strconv.FormatInt(owner.Pid, 10) + " holds a test slot: the lane's check", "waits for a test slot: metasystem test run", "pid 7022 go test", ", bypass: no parent holds a test slot"} {
		if !strings.Contains(page, want) {
			t.Fatalf("page lacks %q:\n%s", want, page)
		}
	}
	var result struct {
		Data struct {
			Composition steward.Composition
			Situation   steward.Situation
		}
	}
	raw := run("--json")
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		t.Fatal(err)
	}
	for _, process := range result.Data.Situation.Processes {
		if process.Bypass != (process.Pid == 7022) {
			t.Fatalf("wrong bypass: %+v", process)
		}
	}
	if strings.Contains(raw, `"Hold"`) || strings.Contains(page, "hold:") || slices.ContainsFunc(result.Data.Composition.Wake, func(line string) bool { return strings.HasPrefix(line, "hold|") }) {
		t.Fatalf("retired hold remains: %s", raw)
	}
	cancel()
	<-done
	bad := filepath.Join(root, "unreadable-admission")
	if err := os.WriteFile(bad, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("METASYSTEM_PROOF_ADMISSION_TEST_DIR", bad)
	clock = clock.Add(3 * time.Minute)
	page = run("--verbose")
	if !strings.Contains(page, "admission:") || strings.Contains(page, ", bypass: no parent") {
		t.Fatalf("admission failure was hidden or marked a bypass:\n%s", page)
	}
}
