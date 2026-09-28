package delegation_test

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/delegation"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/delegation/fake"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
)

// bed is one lifecycle over a private installation root: the real job-record
// owner on disk, and scripted doubles for the lease, the process table,
// Git, the clock and the host.
type bed struct {
	t       *testing.T
	root    string
	doubles *fake.Set
	life    *delegation.Lifecycle
	stderr  bytes.Buffer
	// launches are the adapter starts a dispatch bed saw; noHandshake names
	// jobs whose fake runtime never signals a session.
	launches    []delegation.AdapterLaunch
	noHandshake map[string]bool
}

func newBed(t *testing.T) *bed {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for _, dir := range []string{"artifacts/agents/jobs", "artifacts/agents/record-locks", "artifacts/agents/locks", "artifacts/agents/hb"} {
		if err := os.MkdirAll(filepath.Join(root, dir), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte("evidence.root="+filepath.Join(root, "evidence")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	doubles := fake.NewSet()
	doubles.Process.Root = root
	doubles.Records.CreateFunc = func(job, source string) error { return dispatch.RecordCreate(root, job, source) }
	doubles.Records.SetupFunc = func(job, source string) error { return dispatch.RecordSetup(root, job, source) }
	doubles.Records.CASFunc = func(job, expect, target, patch string) (string, error) {
		return dispatch.RecordCAS(root, job, expect, target, patch)
	}
	holder := int64(4)
	doubles.Lease.RequireFunc = func(delegation.Invocation, *int64) (lease.HolderView, error) {
		return lease.HolderView{Class: lease.ClassHuman, Holder: true, ClaimEpoch: &holder}, nil
	}
	life, err := delegation.New(delegation.Config{Root: root, RepoScope: root, Engine: "/engine/metasystem"}, doubles.Ports())
	if err != nil {
		t.Fatal(err)
	}
	return &bed{t: t, root: root, doubles: doubles, life: life, noHandshake: map[string]bool{}}
}

// run runs one command inside the delegate boundary.
func (b *bed) run(args ...string) delegation.Result {
	b.t.Helper()
	return b.runEnv(delegation.Env{DelegateInternal: true, RecordOutcome: true}, args...)
}

func (b *bed) runEnv(env delegation.Env, args ...string) delegation.Result {
	b.t.Helper()
	b.stderr.Reset()
	return b.life.Run(context.Background(), delegation.Request{
		Invocation: delegation.Invocation{CallerPid: int64(os.Getpid())},
		Env:        env, LockTag: "delegation-test-lock-tag", Stderr: &b.stderr,
	}, args)
}

func (b *bed) recordPath(job string) string {
	return filepath.Join(b.root, "artifacts", "agents", "jobs", job+".json")
}

func (b *bed) writeRecord(job string, fields map[string]any) {
	b.t.Helper()
	record := map[string]any{"jobId": job, "role": "implementer", "runtime": "fake", "instanceTag": "tag-" + job}
	for key, value := range fields {
		record[key] = value
	}
	encoded, err := json.Marshal(record)
	if err != nil {
		b.t.Fatal(err)
	}
	if err := os.WriteFile(b.recordPath(job), append(encoded, '\n'), 0o644); err != nil {
		b.t.Fatal(err)
	}
}

func (b *bed) record(job string) map[string]any {
	b.t.Helper()
	record, err := dispatch.ReadRecordObject(b.recordPath(job))
	if err != nil {
		b.t.Fatal(err)
	}
	return record
}

func (b *bed) writeFile(relative, content string) string {
	b.t.Helper()
	path := filepath.Join(b.root, relative)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		b.t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		b.t.Fatal(err)
	}
	return path
}

func (b *bed) eventNames() []string {
	var names []string
	for _, event := range b.doubles.Events.Emitted {
		names = append(names, event.Name)
	}
	return names
}

func (b *bed) calls(prefix string) []string {
	var matched []string
	for _, call := range b.doubles.Log.Calls() {
		if strings.HasPrefix(call, prefix) {
			matched = append(matched, call)
		}
	}
	return matched
}

func requireExit(t *testing.T, result delegation.Result, code int, stderr string) {
	t.Helper()
	if result.ExitCode != code {
		t.Fatalf("exit %d, want %d; stdout %q stderr %q", result.ExitCode, code, result.Stdout, stderr)
	}
}

func outcomeOf(t *testing.T, result delegation.Result) map[string]any {
	t.Helper()
	var outcome map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(result.Outcome), &outcome); err != nil {
		t.Fatalf("no typed outcome recorded (%q): %v", result.Outcome, err)
	}
	return outcome
}
