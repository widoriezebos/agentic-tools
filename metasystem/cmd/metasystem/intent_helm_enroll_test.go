package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
)

// The helm and a person's acts share one notion of a person: helm take at a
// terminal that is not enrolled enrolls it in the same act, with the same
// proof, so what the helm holder does there counts as a person's act.

func (b *helmBed) enrollment() (humanauthority.Enrollment, bool) {
	enrollment, err := humanauthority.ReadEnrollment(b.inst)
	return enrollment, err == nil
}

// Wido's morning (2026-09-29): Wido enrolled at tty-1, then took the helm at
// tty-2 (sshd, pid 60). The take enrolls tty-2 as Wido and says so.
func TestHelmTakeEnrollsTheTerminalItIsTakenAt(t *testing.T) {
	t.Parallel()
	b := newHelmBed(t, 60, true)
	code, out := b.run("helm", "take", "--reason", "by hand")
	record, _ := b.signature()
	enrollment, ok := b.enrollment()
	if code != 0 || !ok || enrollment.TerminalID != "tty-2" || enrollment.Human != "Wido" || enrollment.Generation != 2 {
		t.Fatalf("take at an unenrolled terminal = %d, enrollment %+v:\n%s", code, enrollment, out)
	}
	if !strings.Contains(out, "enrolled now, as Wido") || record.Enrollment != "proven" || record.By != "Wido" {
		t.Fatalf("the take must say the terminal is now enrolled and record it (%+v):\n%s", record, out)
	}
	// A repeat is applied: the enrollment and signature are left as they are.
	before, _ := os.ReadFile(filepath.Join(b.inst, "artifacts", "agents", "authority", "human-terminal.json"))
	code, out = b.run("helm", "take", "--reason", "by hand")
	after, _ := os.ReadFile(filepath.Join(b.inst, "artifacts", "agents", "authority", "human-terminal.json"))
	if code != 0 || !strings.Contains(out, "Wido already has the helm since") || strings.Contains(out, "enrolled now") || string(before) != string(after) {
		t.Fatalf("a repeat take = %d, enrollment changed %v:\n%s", code, string(before) != string(after), out)
	}
}

// With no enrollment on the machine at all, --name names the person enrolled
// (and at the helm); without it the account name is used.
func TestHelmTakeEnrollsWithTheNameGiven(t *testing.T) {
	t.Parallel()
	b := newHelmBed(t, 20, false)
	code, out := b.run("helm", "take", "--reason", "by hand", "--name", "Ann")
	enrollment, ok := b.enrollment()
	if record, _ := b.signature(); code != 0 || !ok || enrollment.Human != "Ann" || record.By != "Ann" || !strings.Contains(out, "enrolled now, as Ann") {
		t.Fatalf("take --name Ann = %d, enrollment %+v, record %+v:\n%s", code, enrollment, record, out)
	}
	b = newHelmBed(t, 20, false)
	if code, out := b.run("helm", "take", "--reason", "by hand"); code != 0 || !strings.Contains(out, "enrolled now, as wido") {
		t.Fatalf("take without a name = %d:\n%s", code, out)
	}
}

// An already enrolled terminal is left as it is, even when the helm records
// another name.
func TestHelmTakeLeavesAnEnrolledTerminalAsItIs(t *testing.T) {
	t.Parallel()
	b := newHelmBed(t, 20, true)
	before, _ := b.enrollment()
	code, out := b.run("helm", "take", "--reason", "by hand", "--by", "Ann")
	after, _ := b.enrollment()
	if code != 0 || after != before || strings.Contains(out, "enrolled now") {
		t.Fatalf("take at the enrolled terminal = %d, enrollment %+v -> %+v:\n%s", code, before, after, out)
	}
}

// An agent is refused, and nothing is enrolled. Mutation witness: dropping
// the agent-ancestry check in the terminal proof admits pid 80 and fails this.
func TestHelmTakeByAnAgentEnrollsNothing(t *testing.T) {
	t.Parallel()
	b := newHelmBed(t, 80, false)
	code, out := b.run("helm", "take", "--reason", "by hand", "--name", "Ann")
	if _, enrolled := b.enrollment(); code != 3 || enrolled || !strings.Contains(out, "an agent started this shell") {
		t.Fatalf("take by an agent = %d, enrolled %v:\n%s", code, enrolled, out)
	}
}

// The helm holder's status says whether their terminal is enrolled, and if
// not, the one command that enrolls it.
func TestHelmStatusSaysWhetherTheHoldersTerminalIsEnrolled(t *testing.T) {
	t.Parallel()
	b := newHelmBed(t, 60, true)
	b.wantTake(nil, 0, "proven", "Wido")
	inv := &intentInvocation{owners: b.owners}
	lines := strings.Join(inv.helmStatusLines(b.root), "\n")
	if !strings.Contains(lines, "the helm holder's terminal is enrolled as Wido") {
		t.Fatalf("status after an enrolling take:\n%s", lines)
	}
	// Someone enrolls another terminal afterwards: status names the command.
	_, err := humanauthority.Enroll(b.inst, 20, person(), "Wido", helmNow.Add(time.Minute))
	helmMust(t, err)
	lines = strings.Join(inv.helmStatusLines(b.root), "\n")
	if !strings.Contains(lines, "the helm holder's terminal is not enrolled") || !strings.Contains(lines, "metasystem system enroll --name Wido") {
		t.Fatalf("status with the holder's terminal no longer enrolled:\n%s", lines)
	}
}

// The witness of the morning's defect: at a terminal that is not enrolled,
// disk clean --strays is refused naming system enroll; after helm take there,
// the same person's --strays is admitted.
func TestHelmTakeAdmitsThePersonsActsAtThatTerminal(t *testing.T) {
	t.Parallel()
	bed := newDiskBed(t)
	_, err := humanauthority.Enroll(bed.inst, 20, person(), "Wido", helmNow)
	helmMust(t, err)
	shell := func() int64 { return 60 }
	bed.owners.helm = helmOwners{reader: person(), pid: shell, now: func() time.Time { return helmNow },
		machine: func(string) (string, error) { return "m1e", nil }, account: func() string { return "wido" }, zone: time.UTC}
	bed.owners.disk.person = provenPerson(person(), shell, func(string) (time.Time, error) { return helmNow, nil })
	idle := bed.stray("metasystem-audit.idle", 72*time.Hour)
	if code, out := bed.run("disk", "clean", "--preview"); code != 0 {
		t.Fatalf("preview = %d:\n%s", code, out)
	}
	if code, out := bed.run("disk", "clean", "--strays"); code != 3 || !strings.Contains(out, "→ metasystem system enroll --name Wido") ||
		strings.Contains(out, humanauthority.OutcomeTerminalMissing) {
		t.Fatalf("strays at an unenrolled terminal = %d:\n%s", code, out)
	}
	if code, out := bed.run("helm", "take", "--reason", "cleaning by hand"); code != 0 || !strings.Contains(out, "enrolled now, as Wido") {
		t.Fatalf("helm take = %d:\n%s", code, out)
	}
	if code, out := bed.run("disk", "clean", "--strays"); code != 0 || !strings.Contains(out, "removed: ") || !strings.Contains(out, filepath.Base(idle)) {
		t.Fatalf("strays after helm take = %d:\n%s", code, out)
	}
	if _, err := os.Stat(idle); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("the idle stray survived")
	}
}
