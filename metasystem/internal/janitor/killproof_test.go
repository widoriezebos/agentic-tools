package janitor

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/runtimes"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
)

const tag = "metasystem-supervision-owner-repo-123-watcher-2-456"

func TestMatchShapeRequiresAllIncludes(t *testing.T) {
	// "metasystem mission" alone must not match the run-loop shape without
	// its "run-loop" subcommand: the mission family runs many verbs that
	// are not the detached loop.
	argv := []string{"/repo/bin/metasystem", "mission", "status", "--instance-tag", tag}
	if _, ok := MatchShape(DefaultShapes(), argv, tag); ok {
		t.Fatal("a non-run-loop mission verb matched the run-loop shape")
	}
}

// The retired dispatch.sh's standing shell reaper no longer exists (the
// delegate lifecycle reaps in-process, lease-held, single-shot), so no
// shape recognizes a process by the script's name (design 6.6).
func TestNoShapeRecognizesTheRetiredDispatchScript(t *testing.T) {
	t.Parallel()
	for _, shape := range DefaultShapes() {
		for _, include := range shape.Includes {
			if include == "dispatch.sh" {
				t.Fatalf("shape %s still recognizes the retired dispatch.sh", shape.Name)
			}
		}
	}
	argv := []string{"bash", "/repo/scripts/agents/dispatch.sh", "reap", "--instance-tag", tag}
	if _, ok := MatchShape(DefaultShapes(), argv, tag); ok {
		t.Fatal("a dispatch.sh reap argv still proves ownership")
	}
}

// supervisorArgv is the delegate-supervisor argv the launcher builds, under
// the engine /repo/bin/metasystem.
func supervisorArgv(runtime, verb string, flags ...string) []string {
	return append([]string{"/repo/bin/metasystem"}, runtimes.SupervisorArgs(runtime, verb, flags...)...)
}

func TestGroupOwnershipShapesRequireTheTagPosition(t *testing.T) {
	shapes := DefaultShapes()
	real := []struct {
		name string
		argv []string
	}{
		{
			name: "delegate supervisor dispatch",
			argv: supervisorArgv("codex", runtimes.SupervisorDispatch, "--root", "/repo", "--job", "job-a", "--start-gate", "/tmp/gate", "--instance-tag", tag),
		},
		{
			name: "delegate supervisor behind the delegate guard member wrapper",
			argv: append([]string{"/repo/bin/metasystem", "internal", "delegate", "__run-member", "--root", "/repo", "--"},
				supervisorArgv("codex", runtimes.SupervisorFollowUp, "--root", "/repo", "--job", "job-b", "--instance-tag", tag)...),
		},
		{
			name: "claude delegate supervisor",
			argv: supervisorArgv("claude", runtimes.SupervisorDispatch, "--root", "/repo", "--job", "job-c", "--instance-tag", tag),
		},
		{
			name: "devin delegate supervisor",
			argv: supervisorArgv("devin", runtimes.SupervisorFollowUp, "--root", "/repo", "--job", "job-d", "--instance-tag", tag),
		},
		{
			name: "fake delegate supervisor dispatch",
			argv: supervisorArgv("fake", runtimes.SupervisorDispatch, "--root", "/repo", "--job", "job-e", "--start-gate", "/tmp/gate", "--instance-tag", tag),
		},
		{
			name: "codex host start-turn",
			argv: supervisorArgv("codex", runtimes.SupervisorHostTurn, "--root", "/repo", "--instance-tag", tag),
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

	// The retired shell adapter argv is no longer a supervisor shape.
	retired := []string{"bash", "/repo/scripts/agents/adapters/codex.sh", "dispatch", "--job", "job-a", "--instance-tag", tag}
	if shape, ok := MatchShape(shapes, retired, tag); ok {
		t.Fatalf("a retired adapter-script argv still proves ownership through %s", shape.Name)
	}
	// A supervisor argv carrying another round's tag proves nothing.
	foreign := supervisorArgv("codex", runtimes.SupervisorDispatch, "--job", "job-a", "--instance-tag", "some-other-tag")
	if _, ok := MatchShape(shapes, foreign, tag); ok {
		t.Fatal("a supervisor argv with a foreign tag matched")
	}

	rgLeader := []string{"rg", tag, "/repo"}
	if _, ok := MatchShape(shapes, rgLeader, tag); ok {
		t.Fatal("a group leader that merely searches for the tag must not prove ownership")
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

func (r *tagVerificationReader) ReadStart(int64) (identity.Exact, identity.Liveness, error) {
	exact := r.starts[r.read]
	r.read++
	return exact, identity.Alive, nil
}

func (r *tagVerificationReader) ReadArgv(int64) ([]string, bool) {
	return append([]string(nil), r.argv...), true
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
	if _, err := identity.AllPids(); err != nil {
		t.Skipf("process enumeration is unavailable: %v", err)
	}
	tag := fmt.Sprintf("metasystem-job-live-%d", os.Getpid())
	// The trailing words ride Bash's positional slots while the release pipe
	// keeps the observed argv stable across start/argv/start verification.
	ownedCommand := exec.Command("bash", "-c", "cat <&0 & printf x >&3; wait", "metasystem", "util", "hold", "--tag", tag)
	ownedCommand.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	owned := testutil.StartHeldProcess(t, ownedCommand)
	if got := GroupOwnership(int64(owned.Command.Process.Pid), tag); got != GroupOwned {
		t.Fatalf("shaped live group ownership = %s, want OWNED", got)
	}

	unshapedCommand := exec.Command("/bin/sh", "-c", "printf x >&3; IFS= read -r _ || :")
	unshapedCommand.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	unshaped := testutil.StartHeldProcess(t, unshapedCommand)
	if got := GroupOwnership(int64(unshaped.Command.Process.Pid), tag); got == GroupOwned {
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
		{"empty scan proves nothing", nil, false, GroupIndeterminate},
		{"verified wins immediately", []identity.VerificationOutcome{identity.VerificationNotOurs, identity.VerificationVerified}, false, GroupOwned},
		{"indeterminate member defers", []identity.VerificationOutcome{identity.VerificationIndeterminate}, false, GroupIndeterminate},
		{"uncertain membership defers", []identity.VerificationOutcome{identity.VerificationNotOurs}, true, GroupIndeterminate},
		{"dead members prove nothing", []identity.VerificationOutcome{identity.VerificationDead}, false, GroupIndeterminate},
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

type groupTestReader struct {
	starts map[int64]identity.Exact
	argv   map[int64][]string
}

func (r groupTestReader) ReadStart(pid int64) (identity.Exact, identity.Liveness, error) {
	exact, ok := r.starts[pid]
	if !ok {
		return identity.Exact{}, identity.Dead, nil
	}
	return exact, identity.Alive, nil
}

func (r groupTestReader) ReadArgv(pid int64) ([]string, bool) {
	argv, ok := r.argv[pid]
	return argv, ok
}

func TestGroupOwnershipDependencyTable(t *testing.T) {
	stable := identity.Exact{Pid: 41, StartedAt: time.UnixMicro(100_000_001)}
	reader := groupTestReader{
		starts: map[int64]identity.Exact{41: stable, 42: {Pid: 42, StartedAt: stable.StartedAt}},
		argv: map[int64][]string{
			41: {"metasystem", "util", "hold", "--tag", tag},
			42: {"rg", tag},
		},
	}
	rows := identity.FixedProcessTable{{Pid: 41, Group: 700}, {Pid: 42, Group: 700}, {Pid: 43}, {Pid: 44, Group: 701}}
	base := groupOwnershipDependencies{
		Processes: identity.ScriptedProcessTable{Rows: rows, GroupErr: map[int64]error{43: syscall.ESRCH}},
		Reader:    reader,
	}
	if got := groupOwnership(700, tag, base); got != GroupOwned {
		t.Fatalf("verified member ownership = %s, want OWNED", got)
	}
	base.Processes = identity.ScriptedProcessTable{
		Rows:     identity.FixedProcessTable{{Pid: 41, Group: 701}, {Pid: 42, Group: 701}, {Pid: 43}, {Pid: 44, Group: 701}},
		GroupErr: map[int64]error{43: syscall.EPERM},
	}
	if got := groupOwnership(700, tag, base); got != GroupIndeterminate {
		t.Fatalf("unreadable group membership = %s, want INDETERMINATE", got)
	}
	base.Processes = identity.ScriptedProcessTable{Rows: rows, PidsErr: syscall.EPERM}
	if got := groupOwnership(700, tag, base); got != GroupIndeterminate {
		t.Fatalf("unreadable process table = %s, want INDETERMINATE", got)
	}
}
