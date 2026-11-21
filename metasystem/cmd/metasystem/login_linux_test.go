package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/board"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/helm"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
)

type loginProcesses map[int64]humanauthority.Snapshot

func (table loginProcesses) Read(pid int64) (humanauthority.Snapshot, error) {
	if s, ok := table[pid]; ok {
		return s, nil
	}
	return humanauthority.Snapshot{}, os.ErrNotExist
}
func (table loginProcesses) SessionLeader(int64) (int64, error) { return 40, nil }

func loginProcessesFor(ssh bool) loginProcesses {
	table := loginProcesses{}
	add := func(pid, parent int64, name, tty string, uid uint32, hidden bool) {
		s := humanauthority.Snapshot{Exact: identity.Exact{Pid: pid, StartedAt: time.Unix(pid*10, 0), BootID: "boot-A", StartTicks: pid * 100, Argv: []string{name}, ArgvKnown: true}, ParentPID: parent, ParentKnown: true, OwnerUID: uid, OwnerKnown: true, TerminalID: tty, TerminalKnown: true, KernelName: name, ExecutableDenied: hidden}
		if !hidden {
			s.Executable, s.ExecutableKnown = "/usr/bin/"+name, true
		}
		table[pid] = s
	}
	add(40, 30, "bash", "pts/4", 501, false)
	add(1, 0, "systemd", "", 0, true)
	if ssh {
		add(30, 20, "sshd-session", "", 501, true)
		add(20, 1, "sshd-session", "", 0, true)
	} else {
		add(30, 1, "tmux", "", 501, false)
	}
	add(51, 40, "codex", "pts/4", 501, false)
	add(52, 51, "sh", "pts/4", 501, false)
	return table
}

const loginGuardrail = "This check catches known agents in the terminal's process chain, but it cannot tell who sent the command."

func TestLoginHelmHandover(t *testing.T) {
	t.Parallel()
	for _, ssh := range []bool{true, false} {
		b := newHelmFleetBed(t)
		table := loginProcessesFor(ssh)
		pid := int64(52)
		b.owners.helm.pid = func() int64 { return pid }
		b.owners.helm.reader = table
		act := func(verb string, code int) {
			t.Helper()
			args := []string{"--repo", b.lane}
			if verb == "take" {
				args = append(args, "--reason", "by hand")
			}
			got, _, raw := b.helm(t, b.seat, verb, args...)
			if got != code || code != 0 && (strings.Count(raw, loginGuardrail) != 1 || !strings.Contains(raw, humanauthority.OutcomeAgent)) {
				t.Fatalf("helm %s = %d: %s", verb, got, raw)
			}
		}
		layout, err := b.owners.resolver.ResolveLayout(b.seat)
		helmMust(t, err)
		root := checkoutAuthorityRoot(layout)
		before, readErr := humanauthority.ReadEnrollment(root)
		act("take", 3)
		after, err := humanauthority.ReadEnrollment(root)
		if (err == nil) != (readErr == nil) || !reflect.DeepEqual(before, after) || helm.Active(b.lane).Active {
			t.Fatal("refusal wrote enrollment or helm")
		}
		pid = 40
		act("take", 0)
		holder := helm.Active(b.lane)
		enrollment, err := humanauthority.ReadEnrollment(root)
		if !holder.Active || holder.By != "Wido" || err != nil || enrollment.TerminalRef.PID != 40 || enrollment.Login == nil {
			t.Fatalf("take: %+v, %+v, %v", holder, enrollment, err)
		}
		source := helmDrainSource(holder.Record)
		requireHelmDrain(t, b.lane, source)
		pid = 52
		act("take", 3)
		act("return", 3)
		if !reflect.DeepEqual(helm.Active(b.lane).Record, holder.Record) {
			t.Fatal("refusal replaced holder")
		}
		requireHelmDrain(t, b.lane, source)
		// A different qualifying shell may return the helm without replacing enrollment.
		s := table[40]
		s.Exact.StartTicks++
		table[40] = s
		pid = 40
		act("return", 0)
		after, err = humanauthority.ReadEnrollment(root)
		drain, drainErr := plain.ReadDrain(b.lane)
		if err != nil || !reflect.DeepEqual(after, enrollment) || helm.Active(b.lane).Active || drain != nil || drainErr != nil {
			t.Fatal("return did not preserve enrollment and remove holder and drain", err, drainErr)
		}
	}
}

func TestLoginPublicEnrollmentRetry(t *testing.T) {
	t.Parallel()
	b := newProcessBed(t)
	root, err := filepath.EvalSymlinks(b.root())
	helmMust(t, err)
	table := loginProcessesFor(true)
	runtimeNode := table[51]
	runtimeNode.Exact.Argv = []string{"metasystem-fake-agent"}
	table[51] = runtimeNode
	pid := int64(40)
	o := b.owners()
	o.delivery.boardView = func(string, time.Time) board.View { return board.View{} }
	home := t.TempDir()
	o.landing.home = func() (string, error) { return home, nil }
	o.processes.enroll = func(path string, _ int64, _ humanauthority.Reader, by string, now time.Time) (humanauthority.Enrollment, error) {
		if path != root {
			t.Fatal("noncanonical authority root", path)
		}
		return humanauthority.Enroll(path, pid, table, by, now)
	}
	o.dependencies.proveHuman = func(path string, _ int64, _ humanauthority.Reader, now time.Time) (humanauthority.Proof, error) {
		if path != root {
			t.Fatalf("proof checkout %q, want %q", path, root)
		}
		return humanauthority.Prove(root, pid, table, now)
	}
	clock := o.commandNow
	o.commandNow = func(string) (time.Time, error) { return time.Time{}, errors.New("fixture clock unavailable") }
	run := func(want string, published bool) {
		t.Helper()
		code, result := b.runJSON(o, "system", "enroll", "--name", "Wido")
		data, _ := result.Data.(map[string]any)
		if want == intentPartial && (result.Next == nil || !strings.Contains(result.Next.Reason, "same login shell")) {
			t.Fatal("partial did not give the same-shell retry", result.Next)
		}
		if result.Outcome != want || (code == 0) != published || data["fleetPublished"] != published || data["enrollment"] == nil {
			t.Fatalf("enrollment: %d %+v", code, result)
		}
	}
	run(intentPartial, false)
	first, err := humanauthority.ReadEnrollment(root)
	helmMust(t, err)
	o.commandNow = clock
	run(intentConfirmed, true)
	count := b.repo.publications
	endpoint, err := o.dependencies.endpoint(root)
	helmMust(t, err)
	cutoff := func() goal.FleetEnrollmentRecord {
		t.Helper()
		projected, err := goal.Project(endpoint, false, first.EnrolledAt)
		helmMust(t, err)
		if projected.Tree.Root.FleetEnrollment == nil {
			t.Fatal("repair left no fleet cutoff")
		}
		return *projected.Tree.Root.FleetEnrollment
	}
	published := cutoff()
	if published.Generation != first.Generation || published.At != first.EnrolledAt.UTC().Format(time.RFC3339) {
		t.Fatalf("wrong cutoff after repair: %+v", published)
	}
	run(intentUnchanged, true)
	if cutoff() != published {
		t.Fatal("repeat replaced the first fleet cutoff")
	}
	last, err := humanauthority.ReadEnrollment(root)
	if err != nil || first.Login == nil || !reflect.DeepEqual(first, last) || b.repo.publications != count {
		t.Fatal("retry changed enrollment or republished the cutoff", err)
	}
	for _, args := range [][]string{{"system", "enroll", "--name", "Wido"}, {"system", "status"}} {
		code, out, stderr := b.run(o, args...)
		if code != 0 || strings.Count(strings.Join(strings.Fields(out+stderr), " "), loginGuardrail) != 1 || !strings.Contains(out, "ssh login") {
			t.Fatalf("%v: %d %s%s", args, code, out, stderr)
		}
	}
	pid = 52
	code, out, stderr := b.run(o, "system", "enroll", "--name", "Wido")
	if code == 0 || strings.Count(strings.Join(strings.Fields(out+stderr), " "), loginGuardrail) != 1 || !strings.Contains(out+stderr, "own login shell") {
		t.Fatalf("runtime refusal: %d %s%s", code, out, stderr)
	}
	pid = 40
	for _, tc := range []struct {
		name, reason string
		alter        func()
	}{
		{"missing terminal", "interactive login with a terminal", func() { s := table[40]; s.TerminalID = ""; table[40] = s }},
		{"unrecognised SSH parent", "immediate root-owned sshd-session parent", func() { table = loginProcessesFor(true); s := table[20]; s.KernelName = "service"; table[20] = s }},
	} {
		tc.alter()
		code, out, stderr := b.run(o, "system", "enroll", "--name", "Wido")
		words := strings.Join(strings.Fields(out+stderr), " ")
		if code == 0 || !strings.Contains(words, tc.reason) || strings.Count(words, loginGuardrail) != 1 {
			t.Fatalf("%s: %d %s", tc.name, code, words)
		}
	}
	after, err := humanauthority.ReadEnrollment(root)
	if err != nil || !reflect.DeepEqual(after, last) || b.repo.publications != count {
		t.Fatal("refusal changed enrollment or cutoff")
	}
}

type loginChangesDuringTake struct {
	loginProcesses
	walks int
}

func (r *loginChangesDuringTake) SessionLeader(pid int64) (int64, error) {
	r.walks++
	if r.walks == 2 {
		return 0, os.ErrNotExist
	}
	return r.loginProcesses.SessionLeader(pid)
}

func TestLoginHelmTakeSurvivesEnrollmentFailure(t *testing.T) {
	t.Parallel()
	b := newHelmFleetBed(t)
	r := &loginChangesDuringTake{loginProcesses: loginProcessesFor(true)}
	b.owners.helm.reader = r
	b.owners.helm.pid = func() int64 { return 40 }
	code, _, raw := b.helm(t, b.seat, "take", "--repo", b.lane, "--reason", "by hand")
	holder := helm.Active(b.lane)
	if code != 0 || !holder.Active || holder.Enrollment != "unreadable" {
		t.Fatalf("take after second walk failed: %d %s", code, raw)
	}
	layout, err := b.owners.resolver.ResolveLayout(b.seat)
	helmMust(t, err)
	root := checkoutAuthorityRoot(layout)
	if _, err := humanauthority.ReadEnrollment(root); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("failed enrollment wrote a record", err)
	}
	code, _, raw = b.helm(t, b.seat, "return", "--repo", b.lane)
	if code != 0 || helm.Active(b.lane).Active {
		t.Fatalf("return without prior enrollment: %d %s", code, raw)
	}
	if _, err := humanauthority.ReadEnrollment(root); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("return created an enrollment", err)
	}
}

func TestLoginHeadlessLandingNamesEnrollment(t *testing.T) {
	t.Parallel()
	b := newPersonRefusalBed(t)
	helmMust(t, os.Remove(filepath.Join(b.install, "artifacts", "agents", "authority", "human-terminal.json")))
	b.owners.prove = func(root string, _ int64, _ humanauthority.Reader, _, _ string, at time.Time) (humanauthority.Proof, error) {
		return humanauthority.Prove(root, 99, person(), at)
	}
	code, out := b.run(t, b.root, "resume", "--json")
	var result intentResult
	helmMust(t, json.Unmarshal([]byte(out), &result))
	if code != 1 || result.Outcome != intentRefused || result.TerminalCheck != loginGuardrail ||
		!strings.Contains(result.Summary, "the terminal's session could not be reached") || result.Next == nil ||
		len(result.Next.Argv) != 5 || !reflect.DeepEqual(result.Next.Argv[:4], []string{"metasystem", "system", "enroll", "--name"}) ||
		result.Next.Reason != "use an interactive login with a terminal, then repeat this command" {
		t.Fatalf("headless landing: %d %s", code, out)
	}
}
