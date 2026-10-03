package main

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/brain"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal/ownercall"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
)

type partnerBed struct {
	*grantEverythingBed
	deps   brainActDependencies
	starts []launch.Command
	active bool
}

func newPartnerBed(t *testing.T) *partnerBed {
	b := &partnerBed{grantEverythingBed: newGrantEverythingBed(t)}
	registry := t.TempDir()
	b.deps = brainActDependencies{
		ledgerIdentity: func(string) string { return b.rootRecord().Identity },
		machine:        b.dependencies().machine,
		project: func(string) (goal.Projection, error) {
			return goal.Projection{Tree: &goal.TreeGoals{Live: map[string]*goal.GoalFile{}}}, nil
		},
		scan:         func(string) goal.ScanResult { return goal.ScanResult{} },
		registryHome: func() (string, error) { return registry, nil },
		now:          func() time.Time { at, _ := b.commandNow(b.root()); return at },
	}
	return b
}

func (b *partnerBed) owners() intentOwners {
	o := b.grantEverythingBed.owners()
	o.attorney.entries = func(string) ([]goal.PowerOfAttorneyEntry, error) { return b.rootRecord().PowerOfAttorney, nil }
	o.partner = partnerOwners{brain: &b.deps, running: func(string) (bool, error) { return b.active, nil },
		command: func(_, runtime, model, dir string) (launch.Command, error) {
			return launch.Command{Program: runtime, Args: []string{model}, Directory: dir, Environment: []string{"METASYSTEM_OWNER_LINEAGE=" + launch.PartnerOwnerLineage}}, nil
		},
		replace: func(command launch.Command) error { b.starts = append(b.starts, command); return nil },
	}
	return o
}

func TestPartnerStartDeclaresGrantsAndLaunchesUnderThePartnerLineage(t *testing.T) {
	t.Parallel()
	b := newPartnerBed(t)
	code, result := b.runJSON(b.owners(), "partner", "start", "--for", "1w", "--runtime", "codex", "--model", "chosen")
	state := brain.Read(b.root(), b.rootRecord().Identity)
	entries := b.rootRecord().PowerOfAttorney
	if code != 0 || result.Outcome != intentConfirmed || state.Record == nil || state.Record.Role != brain.Partner || len(entries) != 1 || len(b.starts) != 1 {
		t.Fatalf("start: %d %+v state=%+v grants=%+v starts=%+v", code, result, state, entries, b.starts)
	}
	checkout, _ := canonicalCheckout(b.root())
	now, _ := b.commandNow(b.root())
	entry := entries[0]
	if !entry.General() || entry.For != "mac-cli" || entry.Checkout != checkout || entry.Lineage != launch.PartnerOwnerLineage || entry.By != "human:Wido" || entry.Until != now.Add(7*24*time.Hour).Format(time.RFC3339) {
		t.Fatalf("grant binding: %+v", entry)
	}
	if b.starts[0].Program != "codex" || b.starts[0].Directory != checkout || b.starts[0].Args[0] != "chosen" || b.starts[0].Environment[0] != "METASYSTEM_OWNER_LINEAGE=project-partner" {
		t.Fatalf("launch: %+v", b.starts[0])
	}
	before, _ := os.ReadFile(brain.Path(b.root()))
	code, _ = b.runJSON(b.owners(), "partner", "start")
	after, _ := os.ReadFile(brain.Path(b.root()))
	if code != 0 || len(b.starts) != 2 || len(b.rootRecord().PowerOfAttorney) != 1 || !bytes.Equal(before, after) {
		t.Fatal("repeat changed declaration or grant instead of just opening")
	}
	b.active = true
	code, result = b.runJSON(b.owners(), "partner", "start")
	if code != 0 || result.Outcome != intentUnchanged || len(b.starts) != 2 {
		t.Fatalf("already running: %d %+v", code, result)
	}
	inv := &intentInvocation{owners: b.owners(), stateRoot: b.root()}
	if status := inv.withHelm(intentResult{Summary: "status"}, b.root()); !strings.HasPrefix(status.Summary, "PROJECT PARTNER:") {
		t.Fatalf("status: %+v", status)
	}
	code, result = b.runJSON(b.owners(), "partner", "end")
	if code != 0 || result.Outcome != intentConfirmed || brain.Read(b.root(), b.rootRecord().Identity).State != brain.Undeclared || b.rootRecord().PowerOfAttorney[0].Revoked == "" {
		t.Fatalf("end: %d %+v", code, result)
	}
	code, _ = b.runJSON(b.owners(), "partner", "end")
	if code != 0 {
		t.Fatal("repeat end refused")
	}
}

func TestPartnerStartRefusesAnAgentCaller(t *testing.T) {
	t.Parallel()
	b := newPartnerBed(t)
	b.prove = func(root string, _ int64, _ humanauthority.Reader, _, _ string, now time.Time) (humanauthority.Proof, error) {
		return humanauthority.HelmProof(root, humanauthority.HelmGrant{By: "Wido", Grant: "grant-for-agent"}, now)
	}
	for _, verb := range []string{"start", "end"} {
		args := []string{"partner", verb}
		if verb == "start" {
			args = append(args, "--for", "1w")
		}
		code, result := b.runJSON(b.owners(), args...)
		if code == 0 || !strings.Contains(result.Summary, "own enrolled terminal") {
			t.Fatalf("agent %s: %d %+v", verb, code, result)
		}
	}
	if len(b.starts) != 0 || len(b.rootRecord().PowerOfAttorney) != 0 || brain.Read(b.root(), b.rootRecord().Identity).State != brain.Undeclared {
		t.Fatal("agent start mutated or launched")
	}
}

func TestPartnerGrantNeverWithdrawsTheDeclaration(t *testing.T) {
	t.Parallel()
	b := newPartnerBed(t)
	if code, _ := b.runJSON(b.owners(), "partner", "start", "--for", "1w"); code != 0 {
		t.Fatal("start failed")
	}
	deps := b.deps
	grantProof := func(root string, _ int64, _ humanauthority.Reader, now time.Time) (humanauthority.Proof, error) {
		return humanauthority.HelmProof(root, humanauthority.HelmGrant{By: "Wido", Grant: b.rootRecord().PowerOfAttorney[0].ID}, now)
	}
	deps.classify = func(root string, pid int64) (lease.ClassifyResult, error) {
		return coordinatorDirectCaller(root, pid, deps.now(), grantProof)
	}
	owners := b.owners()
	calls := defaultIntentOwnerCalls()
	calls.brain = func(_ string, caller ownercall.Process, stdout, stderr io.Writer, root, by string) int {
		return brainWithdrawWith(caller, stdout, stderr, root, by, false, deps)
	}
	owners.delivery = &intentDeliveryOwners{calls: calls}
	before, _ := os.ReadFile(brain.Path(b.root()))
	code, result := b.runJSON(owners, "settings", "coordinator", "--withdraw", "--by", "Wido")
	after, _ := os.ReadFile(brain.Path(b.root()))
	if code == 0 || result.Summary != coordinatorOwnActRefusal("withdraw") || !bytes.Equal(before, after) {
		t.Fatalf("grantee withdrawal: %d %+v changed=%v", code, result, !bytes.Equal(before, after))
	}
	entry := b.rootRecord().PowerOfAttorney[0]
	if code, _ := b.runJSON(b.owners(), "grant", "revoke", entry.ID); code != 0 {
		t.Fatal("revoke failed")
	}
	if fence := brain.Fence(b.root(), "claim", b.rootRecord().Identity); !strings.Contains(fence, "partner never claims") {
		t.Fatalf("revoke lost fences: %s", fence)
	}
	deps.classify = func(root string, pid int64) (lease.ClassifyResult, error) {
		return coordinatorDirectCaller(root, pid, deps.now(), func(r string, p int64, reader humanauthority.Reader, now time.Time) (humanauthority.Proof, error) {
			return b.prove(r, p, reader, "", "", now)
		})
	}
	code, result = b.runJSON(owners, "settings", "coordinator", "--withdraw", "--by", "Wido")
	if code != 0 || brain.Read(b.root(), b.rootRecord().Identity).State != brain.Undeclared {
		t.Fatalf("person withdrawal: %d %+v", code, result)
	}
}

func TestPartnerRoleFencesClaimDispatchLandAndCarriesThePersonsWord(t *testing.T) {
	t.Parallel()
	b := gcliBrainNewBed(t, true)
	record, err := brain.Declare(brain.DeclareOptions{StateRoot: b.root(), RegistryHome: t.TempDir(), LedgerIdentity: gcliBrainLedger, Machine: "mac-cli", DeclaredBy: "Wido", Role: brain.Partner, Now: time.Date(2026, 9, 1, 9, 0, 0, 0, time.UTC)})
	if err != nil {
		t.Fatal(err)
	}
	for _, act := range []string{"claim", "dispatch", "land"} {
		if fence := brain.Fence(b.root(), act, record.Ledger); !strings.Contains(fence, "this is the project partner's checkout") || !strings.Contains(fence, "a seat does") {
			t.Fatalf("%s fence: %s", act, fence)
		}
	}
	facts := b.dependencies().authorityFacts
	classification, err := brainHumanWordClassificationWithFacts("approve", b.root(), "Wido", nil, facts)
	if err != nil || classification.Class == lease.ClassHuman {
		t.Fatalf("partner must allow the person's word from the agent class: %+v %v", classification, err)
	}
	_, packet := brain.PhaseOne(b.root(), record.Ledger, 10000)
	for _, text := range []string{"PROJECT PARTNER", "plain-English impact", "claim sentence of AGENTS.md", "Never let a subagent"} {
		if !strings.Contains(packet, text) {
			t.Fatalf("role text omits %q", text)
		}
	}
}

func TestPartnerStartWithUnsetRuntime(t *testing.T) {
	// PATH is isolated: only the synthetic codex executable is available.
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "codex"), []byte("#!/bin/sh\nexit 1\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	b := newPartnerBed(t)
	conf := filepath.Join(b.root(), "metasystem.conf")
	if err := os.WriteFile(conf, []byte("metasystem.runtimes=claude,codex,devin\nui.partner.model=chosen\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	o := b.owners()
	o.partner.command = partnerCommand
	code, result := b.runJSON(o, "partner", "start", "--for", "1w")
	if code != 0 || len(b.starts) != 1 || b.starts[0].Program != filepath.Join(dir, "codex") || strings.Join(b.starts[0].Args, " ") != "--model chosen" {
		t.Fatalf("unset runtime: %d %+v %+v", code, result, b.starts)
	}
	t.Setenv("PATH", t.TempDir())
	code, result = b.runJSON(o, "partner", "start")
	if code == 0 || !strings.Contains(result.Summary, "install claude, codex or devin") || len(b.starts) != 1 {
		t.Fatalf("no runtime: %d %+v", code, result)
	}
}

func TestPartnerBootReadsTheHandoffNoteWithinItsShare(t *testing.T) {
	t.Parallel()
	b := newBrainBed(t)
	var stdout, stderr bytes.Buffer
	if code := brainDeclareRoleWith(ownercall.Process{Pid: 1}, &stdout, &stderr, b.root, "Wido", brain.Partner, false, b.deps()); code != 0 {
		t.Fatalf("declare: %s", stderr.String())
	}
	missing := b.boot(10000, 5000)
	if missing.Sections["handoff"] != "skipped" {
		t.Fatalf("missing note: %+v", missing.Sections)
	}
	b.writeFile("plans/handoff-project-partner.md", "A decision from the previous session.")
	short := b.boot(10000, 5000)
	if short.Sections["handoff"] != "complete" || !strings.Contains(short.Payload, "A decision from the previous session.") {
		t.Fatalf("short note: %+v", short)
	}
	b.writeFile("plans/handoff-project-partner.md", "Keep the beginning.\n"+strings.Repeat("a long note é\n", 1000)+"DO NOT FIT THIS END")
	long := b.boot(10000, 5000)
	_, phase := brain.PhaseOne(b.root, brainBedLedger, 10000)
	_, note, _ := strings.Cut(long.Payload, "PROJECT PARTNER HANDOFF")
	if long.Sections["handoff"] != "cut" || !strings.Contains(note, "Keep the beginning.") || strings.Contains(note, "DO NOT FIT THIS END") || len("PROJECT PARTNER HANDOFF"+note)+1 > (10000-len(phase))*20/100 {
		t.Fatalf("long note exceeded its share: %d %+v", len(note), long.Sections)
	}
	requireBootBound(t, long, 10000)
}

func TestPartnerStartFailureLeavesNoGrantAndNoLaunch(t *testing.T) {
	t.Parallel()
	b := newPartnerBed(t)
	code, result := b.runJSON(b.owners(), "partner", "start")
	if code == 0 || !strings.Contains(result.Summary, "needs its end") {
		t.Fatalf("missing duration: %d %+v", code, result)
	}
	b.deps.scan = func(string) goal.ScanResult { return goal.ScanResult{Busy: []goal.Item{{Kind: "job", Id: "builder"}}} }
	code, result = b.runJSON(b.owners(), "partner", "start", "--for", "1w")
	if code == 0 || !strings.Contains(result.Summary, "in flight") || len(b.rootRecord().PowerOfAttorney) != 0 || len(b.starts) != 0 {
		t.Fatalf("busy: %d %+v", code, result)
	}
	o := b.owners()
	o.partner.command = func(string, string, string, string) (launch.Command, error) {
		return launch.Command{}, errors.New("runtime unavailable")
	}
	code, _ = b.runJSON(o, "partner", "start", "--for", "1w")
	if code == 0 || brain.Read(b.root(), b.rootRecord().Identity).State != brain.Undeclared {
		t.Fatal("runtime failure changed declaration")
	}
}

func init() {
	registerIdempotency("partner start", idemStateful, "a live declaration and grant are reused, and an already running partner opens nothing", TestPartnerStartDeclaresGrantsAndLaunchesUnderThePartnerLineage)
	registerIdempotency("partner end", idemStateful, "an ended partner has no grant or declaration left to change", TestPartnerStartDeclaresGrantsAndLaunchesUnderThePartnerLineage)
}
