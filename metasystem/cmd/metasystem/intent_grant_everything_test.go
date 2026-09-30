package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
)

// enrolledPersonProver enrolls the person() tree's zsh (20) at root and
// proves every later call from it: the person at the enrolled terminal,
// walked, not a fixture proof.
func enrolledPersonProver(t *testing.T, root string, now time.Time) goalAuthorityProver {
	t.Helper()
	if _, err := humanauthority.Enroll(root, 20, person(), "Wido", now); err != nil {
		t.Fatal(err)
	}
	return func(r string, _ int64, _ humanauthority.Reader, _, _ string, at time.Time) (humanauthority.Proof, error) {
		return humanauthority.Prove(r, 20, person(), at)
	}
}

type grantEverythingBed struct {
	*intentBed
	prove    goalAuthorityProver
	holder   lease.CurrentHolderView
	locks    []string
	admitted *humanauthority.HelmGrant
}

func newGrantEverythingBed(t *testing.T) *grantEverythingBed {
	bed := newIntentBed(t, false, nil)
	now, err := bed.commandNow(bed.root())
	if err != nil {
		t.Fatal(err)
	}
	return &grantEverythingBed{intentBed: bed, prove: enrolledPersonProver(t, bed.root(), now),
		holder: lease.CurrentHolderView{MainId: "main-1", OwnerLineage: "lin-main"}}
}

func (b *grantEverythingBed) owners() intentOwners {
	owners := b.intentBed.owners()
	owners.prove = b.prove
	owners.helm.zone = time.FixedZone("CEST", 2*3600)
	owners.attorney = attorneyIntentOwners{
		holder: func(string) (lease.CurrentHolderView, error) { return b.holder, nil },
		exclusive: func(root string, _ time.Duration) (bool, error) {
			b.locks = append(b.locks, root)
			return true, nil
		},
		admit: func(string, int64, time.Time) (humanauthority.HelmGrant, bool) {
			if b.admitted == nil {
				return humanauthority.HelmGrant{}, false
			}
			return *b.admitted, true
		},
	}
	return owners
}

func (b *grantEverythingBed) rootRecord() *goal.RootRecord {
	b.t.Helper()
	record, problems := goal.ParseRoot(b.repo.commit(b.repo.accepted).files["plans/goals/backlog.md"])
	if len(problems) != 0 {
		b.t.Fatalf("root record: %v", problems)
	}
	return record
}

// W8: the person grants everything at the enrolled terminal in one easy
// line; the grant is bound to this checkout and the lease holder's lineage,
// a repeat holds, list shows the time left in local time, status leads with
// it, and revoke ends it under the grant lock.
func TestGrantEverythingAtTheEnrolledTerminal(t *testing.T) {
	t.Parallel()
	b := newGrantEverythingBed(t)
	now, _ := b.commandNow(b.root())
	code, result := b.runJSON(b.owners(), "grant", "add", "--acts", "everything", "--for", "24h")
	if code != 0 || result.Outcome != intentConfirmed || len(result.Targets) != 1 {
		t.Fatalf("grant everything = %d %+v", code, result)
	}
	id := result.Targets[0].ID
	checkout, _ := filepath.EvalSymlinks(b.root())
	wantEnd := now.Add(24 * time.Hour).Truncate(time.Minute)
	if !strings.Contains(result.Summary, "granted "+id+": the main session of "+checkout) ||
		!strings.Contains(result.Summary, "until "+wantEnd.In(time.FixedZone("CEST", 2*3600)).Format("15:04 MST (2006-01-02)")) {
		t.Fatalf("summary = %q", result.Summary)
	}
	var entry goal.PowerOfAttorneyEntry
	for _, candidate := range b.rootRecord().PowerOfAttorney {
		if candidate.ID == id {
			entry = candidate
		}
	}
	if !entry.General() || entry.Checkout != checkout || entry.Lineage != "lin-main" || entry.By != "human:Wido" || entry.Until != wantEnd.UTC().Format(time.RFC3339) {
		t.Fatalf("the ledger entry = %+v", entry)
	}

	code, result = b.runJSON(b.owners(), "grant", "add", "--acts", "everything", "--for", "24h")
	if code != 0 || result.Outcome != intentUnchanged {
		t.Fatalf("the same grant again = %d %+v", code, result)
	}

	code, stdout, _ := b.run(b.owners(), "grant", "list")
	if code != 0 || !strings.Contains(stdout, id+"  everything  by Wido  for the main session of "+checkout) || !strings.Contains(stdout, "24h00m left") {
		t.Fatalf("grant list = %d %q", code, stdout)
	}

	inv := &intentInvocation{owners: b.owners(), stateRoot: b.root()}
	inv.owners.attorney.entries = func(string) ([]goal.PowerOfAttorneyEntry, error) { return b.rootRecord().PowerOfAttorney, nil }
	if line := inv.attorneyStatusLine(b.root()); !strings.HasPrefix(line, "POWER OF ATTORNEY: ") || !strings.Contains(line, "acts for Wido until") ||
		!strings.HasSuffix(line, "(grant "+id+") — metasystem grant revoke "+id+" ends it") {
		t.Fatalf("status line = %q", line)
	}

	code, result = b.runJSON(b.owners(), "grant", "revoke", id)
	if code != 0 || result.Outcome != intentConfirmed || len(b.locks) != 1 || b.locks[0] != checkout {
		t.Fatalf("revoke = %d %+v, locks %v", code, result, b.locks)
	}
	if line := inv.attorneyStatusLine(b.root()); line != "" {
		t.Fatalf("status after revoke = %q", line)
	}
	code, result = b.runJSON(b.owners(), "grant", "revoke", id)
	if code != 0 || result.Outcome != intentUnchanged {
		t.Fatalf("a second revoke = %d %+v", code, result)
	}
}

// W5 at the command edge: a general grant is refused, with the examples,
// when its end is missing or malformed, beside --tiers or another act, from
// a proof that is not the walk's own (the fixture proof here), and when no
// session holds the lease.
func TestGrantEverythingRefusals(t *testing.T) {
	t.Parallel()
	b := newGrantEverythingBed(t)
	before := len(b.rootRecord().PowerOfAttorney)
	for _, args := range [][]string{
		{"--acts", "everything"},
		{"--acts", "everything", "--for", "8d"},
		{"--acts", "everything", "--for", "24h", "--until", "18:00"},
		{"--acts", "everything", "--for", "24h", "--tiers", "1"},
		{"--acts", "everything,approve", "--for", "24h"},
		{"--acts", "approve", "--tiers", "1", "--for", "24h"},
	} {
		code, result := b.runJSON(b.owners(), append([]string{"grant", "add"}, args...)...)
		if code == 0 || result.Outcome != intentRefused || !strings.Contains(result.Summary+" "+result.Decision, "--for 24h") {
			t.Errorf("%v = %d %+v", args, code, result)
		}
	}
	fixture := b.owners()
	fixture.prove = fixedFixtureGoalAuthority
	if code, result := b.runJSON(fixture, "grant", "add", "--acts", "everything", "--for", "8h"); code == 0 || !strings.Contains(result.Summary, "enrolled terminal") {
		t.Errorf("a fixture proof granted: %d %+v", code, result)
	}
	b.holder = lease.CurrentHolderView{}
	if code, result := b.runJSON(b.owners(), "grant", "add", "--acts", "everything", "--for", "8h"); code == 0 || !strings.Contains(result.Summary, "no session holds this checkout's lease") {
		t.Errorf("no holder granted: %d %+v", code, result)
	}
	if after := len(b.rootRecord().PowerOfAttorney); after != before {
		t.Fatalf("a refusal wrote %d entries", after-before)
	}
}

// W7 (M6): an agent session's pause or done is the granting person's act
// when a live grant admits it; without one the session's shortcut stands.
func TestActorSelectionAsksTheGrantBeforeTheSessionShortcut(t *testing.T) {
	t.Parallel()
	b := newGrantEverythingBed(t)
	b.lineage = "lin-main"
	command := mustIntentCommand(t, "goal done")
	inv := &intentInvocation{command: command, owners: b.owners(), stateRoot: b.root(), input: intentInput{values: map[string][]string{}}}
	args, problem := inv.actorArgs("g1")
	if problem != nil || slices.Contains(args, "--by") {
		t.Fatalf("without a grant the session acts as itself: %v %+v", args, problem)
	}
	b.admitted = &humanauthority.HelmGrant{By: "Wido", Class: lease.ClassMain, Grant: "01M-grant"}
	inv.owners = b.owners()
	args, problem = inv.actorArgs("g1")
	if problem != nil || !slices.Equal(args, []string{"--by", "Wido"}) {
		t.Fatalf("under a grant the act is the person's: %v %+v", args, problem)
	}
	b.admitted = &humanauthority.HelmGrant{By: "wido", Class: "DELEGATE"}
	inv.owners = b.owners()
	if args, _ = inv.actorArgs("g1"); slices.Contains(args, "--by") {
		t.Fatalf("a helm grant is not a power of attorney here: %v", args)
	}
}

// W5 (M3): metasystem.runtimes, which turns fixture mode on, is set only by
// the person's own proof at the enrolled terminal; other keys need none.
func TestSettingsSetAnAuthorityKeyNeedsTheDirectPerson(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "scripts", "agents"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte("metasystem.template=true\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 9, 30, 7, 0, 0, 0, time.UTC)
	walk := enrolledPersonProver(t, root, now)
	owners := intentOwners{resolver: stateroot.NewResolver(fakeTop(root), noExecutable),
		commandNow: func(string) (time.Time, error) { return now, nil }}
	set := func(owners intentOwners, key, value string) (int, intentResult) {
		t.Helper()
		var stdout, stderr bytes.Buffer
		code := runIntentIn(mustIntentCommand(t, "settings set"), []string{key, value, "--json"}, &stdout, &stderr, root, owners)
		var result intentResult
		if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
			t.Fatalf("settings set %s: %v %q %q", key, err, stdout.String(), stderr.String())
		}
		return code, result
	}
	owners.prove = fixedFixtureGoalAuthority
	if code, result := set(owners, "metasystem.runtimes", "fake"); code == 0 || !strings.Contains(result.Summary, "enrolled terminal") {
		t.Fatalf("a fixture proof set the authority key: %d %+v", code, result)
	}
	helmProof := func(r string, pid int64, reader humanauthority.Reader, word, review string, at time.Time) (humanauthority.Proof, error) {
		return humanauthority.HelmProof(r, humanauthority.HelmGrant{By: "Wido", Grant: "01M-grant"}, at)
	}
	owners.prove = helmProof
	if code, result := set(owners, "metasystem.runtimes", "fake"); code == 0 || !strings.Contains(result.Summary, "enrolled terminal") {
		t.Fatalf("a grant proof set the authority key: %d %+v", code, result)
	}
	data, _ := os.ReadFile(filepath.Join(root, "metasystem.conf.local"))
	if strings.Contains(string(data), "metasystem.runtimes") {
		t.Fatalf("a refusal wrote the key: %q", data)
	}
	owners.prove = walk
	if code, result := set(owners, "metasystem.runtimes", "fake"); code != 0 || result.Outcome != intentConfirmed {
		t.Fatalf("the person at the enrolled terminal = %d %+v", code, result)
	}
	owners.prove = nil
	if code, result := set(owners, "role.default.model.claude", "claude-opus-5-5"); code != 0 || result.Outcome != intentConfirmed {
		t.Fatalf("an ordinary key = %d %+v", code, result)
	}
}
