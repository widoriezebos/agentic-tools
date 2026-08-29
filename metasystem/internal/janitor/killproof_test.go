package janitor

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/registry"
)

const tag = "metasystem-supervision-owner-repo-123-watcher-2-456"

func observed(pid int64, startSec int64, argv ...string) identity.Exact {
	return identity.Exact{Pid: pid, StartedAt: time.Unix(startSec, 0), Argv: argv, ArgvKnown: true}
}

// Exercise the kill-authority and three-part process proof row by row.
func TestKillable(t *testing.T) {
	recorded := &registry.ProcessRef{Pid: 41, PidStartedAt: 100}
	watcherArgv := []string{"bash", "/repo/scripts/watch-background-jobs.sh", "--census", "--instance-tag", tag}
	cases := []struct {
		name     string
		observed identity.Exact
		recorded *registry.ProcessRef
		want     bool
	}{
		{
			name:     "real leaked component IS killable",
			observed: observed(41, 100, watcherArgv...),
			recorded: recorded,
			want:     true,
		},
		{
			name: "same-second pid reuse with WRONG argv is not",
			observed: observed(41, 100,
				"vim", "notes-about-"+tag+".md"),
			recorded: recorded,
			want:     false,
		},
		{
			name:     "recycled pid, different start second",
			observed: observed(41, 101, watcherArgv...),
			recorded: recorded,
			want:     false,
		},
		{
			// A shell QUOTING the tag matches no shape
			// because the tag is not the tag-flag's value.
			name: "tag mention in a grep is never killable",
			observed: observed(41, 100,
				"grep", "-rn", tag, "/repo"),
			recorded: recorded,
			want:     false,
		},
		{
			name:     "unreadable argv means report, not kill",
			observed: identity.Exact{Pid: 41, StartedAt: time.Unix(100, 0)},
			recorded: recorded,
			want:     false,
		},
		{
			// An establishment orphan has no recorded identity, so its argv
			// shape plus the claim tag is the proof.
			name: "signature-only owner proof",
			observed: observed(77, 999,
				"/repo/bin/metasystem", "supervise", "owner", "--repo", "/repo", "--tag", tag),
			recorded: nil,
			want:     true,
		},
		{
			name: "flag=value spelling matches",
			observed: observed(41, 100,
				"/repo/scripts/watch-background-jobs.sh", "--instance-tag="+tag),
			recorded: recorded,
			want:     true,
		},
		{
			name: "the go owner verb is a known shape",
			observed: observed(41, 100,
				"/usr/local/bin/metasystem", "supervise", "owner", "--tag", tag),
			recorded: recorded,
			want:     true,
		},
		{
			name: "wrong tag in the tag position",
			observed: observed(41, 100,
				"/repo/scripts/watch-background-jobs.sh", "--instance-tag", "some-other-tag"),
			recorded: recorded,
			want:     false,
		},
	}
	for _, row := range cases {
		t.Run(row.name, func(t *testing.T) {
			_, got := Killable(row.observed, row.recorded, DefaultShapes(), []string{tag})
			if got != row.want {
				t.Fatalf("killable=%v, want %v", got, row.want)
			}
		})
	}
}

func TestMatchShapeRequiresAllIncludes(t *testing.T) {
	// "dispatch.sh" alone must not match the reaper shape without its
	// "reap" subcommand — dispatch runs many verbs that are not
	// supervision components.
	argv := []string{"bash", "/repo/scripts/agents/dispatch.sh", "dispatch", "--instance-tag", tag}
	if _, ok := MatchShape(DefaultShapes(), argv, tag); ok {
		t.Fatal("a non-reap dispatch verb matched the reaper shape")
	}
}

func TestGroupOwnershipShapesRequireTheTagPosition(t *testing.T) {
	shapes := DefaultShapes()
	real := []struct {
		name string
		argv []string
	}{
		{
			name: "adapter supervisor dispatch",
			argv: []string{"bash", "/repo/scripts/agents/adapters/codex.sh", "dispatch", "--job", "job-a", "--start-gate", "/tmp/gate", "--instance-tag", tag},
		},
		{
			name: "adapter supervisor follow-up behind the execution guard",
			argv: []string{"bash", "/repo/scripts/agents/checkout-execution-guard.sh", "run-member", "--", "/repo/scripts/agents/adapters/codex.sh", "follow-up", "--job", "job-b", "--instance-tag=" + tag},
		},
		{
			name: "claude adapter supervisor",
			argv: []string{"bash", "/repo/scripts/agents/adapters/claude.sh", "dispatch", "--job", "job-c", "--instance-tag", tag},
		},
		{
			name: "devin adapter supervisor",
			argv: []string{"bash", "/repo/scripts/agents/adapters/devin.sh", "follow-up", "--job", "job-d", "--instance-tag", tag},
		},
		{
			name: "fake adapter supervisor dispatch",
			argv: []string{"bash", "/repo/scripts/agents/adapters/fake.sh", "dispatch", "--job", "job-e", "--start-gate", "/tmp/gate", "--instance-tag", tag},
		},
		{
			name: "codex cli launch",
			argv: []string{"codex", "exec", "--json", "-c", `metasystem_instance_tag="` + tag + `"`, "-"},
		},
		{
			name: "claude cli launch",
			argv: []string{"claude", "-p", "--name", tag, "--output-format", "stream-json"},
		},
		{
			name: "devin cli launch",
			argv: []string{"devin", "-p", "--config", "/round/" + tag, "--prompt-file", "/round/prompt.md"},
		},
		{
			name: "tagged hold child",
			argv: []string{"/repo/bin/metasystem", "util", "hold", "--tag", tag, "--stopped-file", "/tmp/stopped"},
		},
	}
	for _, row := range real {
		t.Run(row.name, func(t *testing.T) {
			if shape, ok := MatchShape(shapes, row.argv, tag); !ok {
				t.Fatalf("real adapter argv did not match: %v", row.argv)
			} else if shape.Name == "" {
				t.Fatal("a matching adapter shape must carry a report label")
			}
		})
	}

	rgLeader := []string{"rg", tag, "/repo"}
	if _, ok := MatchShape(shapes, rgLeader, tag); ok {
		t.Fatal("a group leader that merely searches for the tag must not prove ownership")
	}
	recorded := &registry.ProcessRef{Pid: 41, PidStartedAt: 100}
	if _, ok := Killable(observed(41, 100, rgLeader...), recorded, shapes, []string{tag}); ok {
		t.Fatal("a group leader that merely searches for the tag must not satisfy the kill predicate")
	}
	stable := identity.Exact{Pid: 41, StartedAt: time.UnixMicro(100_000_001)}
	leaderReader := &tagVerificationReader{starts: []identity.Exact{stable, stable}, argv: rgLeader}
	leaderVerification := identity.VerifyProcess(leaderReader, 41, func(argv []string) bool {
		_, ok := MatchShape(shapes, argv, tag)
		return ok
	})
	if got := groupOwnershipFromVerifications([]identity.Verification{leaderVerification}, false); got != GroupNotOwned {
		t.Fatalf("rg group leader ownership = %s, want NOT-OWNED", got)
	}

	for _, purpose := range []string{"custody", "adoption"} {
		t.Run(purpose, func(t *testing.T) {
			reader := &tagVerificationReader{
				starts: []identity.Exact{stable, stable},
				argv:   rgLeader,
			}
			result := identity.VerifyProcess(reader, 41, func(argv []string) bool {
				_, ok := MatchShape(shapes, argv, tag)
				return ok
			})
			if result.Outcome != identity.VerificationNotOurs {
				t.Fatalf("%s proof outcome = %s, want NOT-OURS", purpose, result.Outcome)
			}
		})
	}
}

type tagVerificationReader struct {
	starts []identity.Exact
	argv   []string
	read   int
}

func (r *tagVerificationReader) Probe(int64) (identity.Exact, identity.Liveness, error) {
	exact := r.starts[r.read]
	r.read++
	exact.Argv = append([]string(nil), r.argv...)
	exact.ArgvKnown = true
	return exact, identity.Alive, nil
}

func TestGroupOwnershipGuardsRefuseWithoutScanning(t *testing.T) {
	if got := GroupOwnership(1, "metasystem-job-x-1"); got != GroupNotOwned {
		t.Fatalf("pgid 1 ownership = %s, want NOT-OWNED", got)
	}
	if got := GroupOwnership(4242, ""); got != GroupNotOwned {
		t.Fatalf("empty-tag ownership = %s, want NOT-OWNED", got)
	}
}

func TestGroupOwnershipOnLiveGroups(t *testing.T) {
	tag := fmt.Sprintf("metasystem-job-live-%d", os.Getpid())
	// The trailing words ride bash's positional slots: real kernel argv
	// carrying the tagged-hold shape tokens and the tag as --tag's value.
	owned := exec.Command("bash", "-c", "sleep 30", "metasystem", "util", "hold", "--tag", tag)
	owned.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := owned.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = owned.Process.Kill(); _, _ = owned.Process.Wait() }()
	if got := GroupOwnership(int64(owned.Process.Pid), tag); got != GroupOwned {
		t.Fatalf("shaped live group ownership = %s, want OWNED", got)
	}

	unshaped := exec.Command("sleep", "30")
	unshaped.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := unshaped.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = unshaped.Process.Kill(); _, _ = unshaped.Process.Wait() }()
	if got := GroupOwnership(int64(unshaped.Process.Pid), tag); got == GroupOwned {
		t.Fatalf("tagless live group ownership = %s; a group without the positioned tag must never be OWNED", got)
	}
}

func TestGroupOwnershipVerificationFold(t *testing.T) {
	rows := []struct {
		name      string
		outcomes  []identity.VerificationOutcome
		uncertain bool
		want      GroupOwnershipOutcome
	}{
		{"empty scan", nil, false, GroupNotOwned},
		{"verified wins immediately", []identity.VerificationOutcome{identity.VerificationNotOurs, identity.VerificationVerified}, false, GroupOwned},
		{"indeterminate member defers", []identity.VerificationOutcome{identity.VerificationIndeterminate}, false, GroupIndeterminate},
		{"uncertain membership defers", []identity.VerificationOutcome{identity.VerificationNotOurs}, true, GroupIndeterminate},
		{"dead members prove nothing", []identity.VerificationOutcome{identity.VerificationDead}, false, GroupNotOwned},
	}
	for _, row := range rows {
		t.Run(row.name, func(t *testing.T) {
			var verifications []identity.Verification
			for _, outcome := range row.outcomes {
				verifications = append(verifications, identity.Verification{Outcome: outcome})
			}
			if got := groupOwnershipFromVerifications(verifications, row.uncertain); got != row.want {
				t.Fatalf("fold = %s, want %s", got, row.want)
			}
		})
	}
}
