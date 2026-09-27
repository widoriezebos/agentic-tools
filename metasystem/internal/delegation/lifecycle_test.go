package delegation_test

import (
	"encoding/json"
	"errors"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/delegation"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/delegation/fake"
)

func TestNewRefusesAPartialPortSet(t *testing.T) {
	t.Parallel()
	config := delegation.Config{Root: t.TempDir(), RepoScope: "/repo"}
	complete := fake.NewSet().Ports()
	if _, err := delegation.New(config, complete); err != nil {
		t.Fatalf("a complete port set was refused: %v", err)
	}
	for _, missing := range []struct {
		name  string
		clear func(*delegation.Ports)
	}{
		{"lease", func(p *delegation.Ports) { p.Lease = nil }},
		{"steward", func(p *delegation.Ports) { p.Steward = nil }},
		{"adapter", func(p *delegation.Ports) { p.Adapter = nil }},
		{"goal", func(p *delegation.Ports) { p.Goal = nil }},
		{"record", func(p *delegation.Ports) { p.Records = nil }},
		{"event", func(p *delegation.Ports) { p.Events = nil }},
		{"process", func(p *delegation.Ports) { p.Process = nil }},
		{"git", func(p *delegation.Ports) { p.Git = nil }},
		{"clock", func(p *delegation.Ports) { p.Clock = nil }},
		{"host", func(p *delegation.Ports) { p.Host = nil }},
		{"guard", func(p *delegation.Ports) { p.Guard = nil }},
	} {
		ports := complete
		missing.clear(&ports)
		lifecycle, err := delegation.New(config, ports)
		if err == nil || lifecycle != nil {
			t.Fatalf("a port set without %s operations was accepted", missing.name)
		}
		if !strings.Contains(err.Error(), missing.name+" operations are not wired") {
			t.Fatalf("refusal does not name the %s port: %v", missing.name, err)
		}
	}
	if _, err := delegation.New(config, delegation.Ports{}); err == nil || strings.Count(err.Error(), "not wired") != 11 {
		t.Fatalf("an empty port set must name all eleven missing ports: %v", err)
	}
	if _, err := delegation.New(delegation.Config{}, complete); err == nil {
		t.Fatal("a lifecycle without an installation root was accepted")
	}
}

func TestPhasesAreTheRetiredScriptRouter(t *testing.T) {
	t.Parallel()
	want := []delegation.Phase{"dispatch", "watch", "follow-up", "status", "cancel", "close", "reap"}
	if got := delegation.Phases(); !reflect.DeepEqual(got, want) {
		t.Fatalf("phases %v, want %v", got, want)
	}
}

func TestNewResolvesTheRepositoryScopeThroughGit(t *testing.T) {
	t.Parallel()
	doubles := fake.NewSet()
	root := t.TempDir()
	if _, err := delegation.New(delegation.Config{Root: root}, doubles.Ports()); err == nil ||
		!strings.Contains(err.Error(), "is not inside a git repository") {
		t.Fatalf("a root Git cannot place was accepted: %v", err)
	}
	resolved, _ := filepath.EvalSymlinks(root)
	doubles.Git.Responses["*|rev-parse --show-toplevel"] = fake.GitResponse{Stdout: resolved + "\n"}
	if _, err := delegation.New(delegation.Config{Root: root}, doubles.Ports()); err != nil {
		t.Fatal(err)
	}
}

func TestUnknownCommandsAndMalformedCallbacksAreUsage(t *testing.T) {
	t.Parallel()
	b := newBed(t)
	for _, argv := range [][]string{{"frobnicate"}, {"__record-cas"}, {"__handshake-timeout", "--job"}, {"__cancel-owned", "--job", "a", "extra"}, {"__no-such-callback"}} {
		if result := b.run(argv...); result.ExitCode != 2 {
			t.Fatalf("%v exited %d, want usage 2", argv, result.ExitCode)
		}
	}
}

func TestCallbackAuthorityRefusalLeavesTheRecordUntouched(t *testing.T) {
	t.Parallel()
	b := newBed(t)
	b.writeRecord("job-a", map[string]any{"status": "running"})
	b.doubles.Lease.AuthorizeFunc = func(_ delegation.Invocation, mode delegation.AuthorityMode, job string) error {
		if mode != delegation.AuthorityRecordWriter || job != "job-a" {
			t.Errorf("authorized %s for %q", mode, job)
		}
		return errors.New("control-plane write refused for caller class DELEGATE")
	}
	patch := b.writeFile("patch.json", `{"phase":"late"}`)
	result := b.run("__record-cas", "--job", "job-a", "--expect", "running", "--status", "running", "--patch", patch)
	requireExit(t, result, 1, b.stderr.String())
	if !strings.Contains(b.stderr.String(), "refused for caller class DELEGATE") {
		t.Fatalf("stderr %q", b.stderr.String())
	}
	if _, has := b.record("job-a")["phase"]; has {
		t.Fatal("a refused callback wrote the record")
	}
}

func TestRecordCASCallbackPrintsTheLostCompare(t *testing.T) {
	t.Parallel()
	b := newBed(t)
	b.writeRecord("job-a", map[string]any{"status": "running"})
	patch := b.writeFile("patch.json", `{"phase":"x"}`)
	result := b.run("__record-cas", "--job", "job-a", "--expect", "pending", "--status", "running", "--patch", patch)
	requireExit(t, result, 3, b.stderr.String())
	if string(result.Stdout) != "observed=running\n" {
		t.Fatalf("stdout %q", result.Stdout)
	}
	result = b.run("__record-cas", "--job", "job-a", "--expect", "running", "--status", "running", "--patch", patch)
	requireExit(t, result, 0, b.stderr.String())
	if b.record("job-a")["phase"] != "x" {
		t.Fatal("the metadata update did not land")
	}
}

func jsonInt(value int64) string {
	encoded, _ := json.Marshal(value)
	return string(encoded)
}

// record-protocol-fixtures' surviving leg: the __record-create callback
// forwards to the record owner under holder-only authority for its job.
func TestRecordCreateCallbackPersistsTheReservation(t *testing.T) {
	t.Parallel()
	b := newBed(t)
	source := b.writeFile("smoke.json", `{"jobId":"smoke-job","status":"pending-setup","mainId":"main-smoke","claimEpoch":1}`)
	requireExit(t, b.run("__record-create", "--job", "smoke-job", "--source", source), 0, b.stderr.String())
	if record := b.record("smoke-job"); record["status"] != "pending-setup" || record["mainId"] != "main-smoke" {
		t.Fatalf("record %v", record)
	}
	if calls := b.calls("lease.Authorize"); len(calls) != 1 || !strings.Contains(calls[0], "mode=holder-only job=smoke-job") {
		t.Fatalf("authority %v", calls)
	}
}
