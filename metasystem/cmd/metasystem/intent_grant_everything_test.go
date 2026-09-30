package main

import (
	"bytes"
	"encoding/json"
	"errors"
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

// The person's view in text: add, list, the status line and revoke, each a
// short line in local time.
func TestGrantEverythingReadsInText(t *testing.T) {
	t.Parallel()
	b := newGrantEverythingBed(t)
	code, added, _ := b.run(b.owners(), "grant", "add", "--acts", "everything", "--for", "8h")
	if code != 0 || !strings.Contains(added, "granted ") || !strings.Contains(added, "ends it") {
		t.Fatalf("add = %d %q", code, added)
	}
	id := strings.TrimSuffix(strings.Fields(strings.SplitN(added, "granted ", 2)[1])[0], ":")
	code, listed, _ := b.run(b.owners(), "grant", "list")
	if code != 0 || !strings.Contains(listed, "8h00m left") {
		t.Fatalf("list = %d %q", code, listed)
	}
	inv := &intentInvocation{owners: b.owners(), stateRoot: b.root()}
	inv.owners.attorney.entries = func(string) ([]goal.PowerOfAttorneyEntry, error) { return b.rootRecord().PowerOfAttorney, nil }
	status := inv.attorneyStatusLine(b.root())
	code, revoked, _ := b.run(b.owners(), "grant", "revoke", id)
	if code != 0 || !strings.Contains(revoked, "revoked "+id) || status == "" {
		t.Fatalf("revoke = %d %q, status %q", code, revoked, status)
	}
	t.Logf("$ metasystem grant add --acts everything --for 8h\n%s$ metasystem grant list\n%s$ metasystem status (first line)\n%s\n$ metasystem grant revoke %s\n%s", added, listed, status, id, revoked)
}

// A goal act a grant answered carries the grant to its effect: when the
// grant is not on the ledger the act lands on, it is refused there, and a
// person's own act is untouched. A grant add the grant answered is refused
// and logged as refused, never as admitted.
func TestAGrantAnsweredActIsCheckedAtItsEffectAndRefusalsAreLogged(t *testing.T) {
	t.Parallel()
	b := newGrantEverythingBed(t)
	grantProof := func(root string, _ int64, _ humanauthority.Reader, _, _ string, at time.Time) (humanauthority.Proof, error) {
		return humanauthority.HelmProof(root, humanauthority.HelmGrant{By: "Wido", Class: lease.ClassMain, Grant: "01M-missing", Until: "2099-01-01T00:00:00Z"}, at)
	}
	owners := b.owners()
	owners.prove = grantProof
	code, result := b.runJSON(owners, "goal", "prioritize", bedGoal, "1")
	if code == 0 || !strings.Contains(result.Summary+" "+result.Decision, "01M-missing is not recorded") {
		t.Fatalf("an act under an unrecorded grant = %d %+v", code, result)
	}
	if code, result := b.runJSON(b.owners(), "goal", "prioritize", bedGoal, "1"); code != 0 || result.Outcome != intentConfirmed {
		t.Fatalf("the person's own act = %d %+v", code, result)
	}
	code, result = b.runJSON(owners, "grant", "add", "--acts", "everything", "--for", "8h")
	if code == 0 || result.Outcome != intentRefused {
		t.Fatalf("a grant add under a grant = %d %+v", code, result)
	}
	checkout, _ := filepath.EvalSymlinks(b.root())
	data, _ := os.ReadFile(attorneyLogPath(checkout))
	if !strings.Contains(string(data), "refused grant=01M-missing by=Wido act=\"grant add\"") {
		t.Fatalf("the log lacks the refusal: %q", data)
	}
}

// TestGrantStatusLineReadsTheLedgerWhereGrantListDoes (F2): status names
// the repository top as its path while the ledger is read relative to the
// installation (the state root), as grant list reads it. A template
// checkout's top has no plans/goals of its own, so a read there finds no
// root record; the status line must still show the live grant grant list
// shows.
func TestGrantStatusLineReadsTheLedgerWhereGrantListDoes(t *testing.T) {
	t.Parallel()
	b := newGrantEverythingBed(t)
	code, result := b.runJSON(b.owners(), "grant", "add", "--acts", "everything", "--for", "24h")
	if code != 0 || result.Outcome != intentConfirmed || len(result.Targets) != 1 {
		t.Fatalf("grant everything = %d %+v", code, result)
	}
	id := result.Targets[0].ID
	code, listed, _ := b.run(b.owners(), "grant", "list")
	if code != 0 || !strings.Contains(listed, id) {
		t.Fatalf("grant list = %d %q", code, listed)
	}
	top := filepath.Dir(b.root())
	var read []string
	inv := &intentInvocation{owners: b.owners(), stateRoot: b.root()}
	inv.owners.attorney.entries = func(root string) ([]goal.PowerOfAttorneyEntry, error) {
		read = append(read, root)
		if root != b.root() {
			return nil, errors.New("no root record at the repository top")
		}
		return b.rootRecord().PowerOfAttorney, nil
	}
	if line := inv.attorneyStatusLine(top); !strings.Contains(line, "(grant "+id+")") {
		t.Fatalf("status line from the repository top = %q (ledger read at %v); grant list shows %q", line, read, listed)
	}
}

// The trial: with METASYSTEM_OWNER_LINEAGE set, goal open --origin human
// (and every other dual person/seat act chosen by actingAs) took the
// session's shortcut without asking the general grant; only pause and done
// asked it. Every dual act asks the grant first; a session that names
// itself with --lineage, or no grant, keeps the shortcut.
func TestDualActsAskTheGrantBeforeTheSessionShortcut(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		command string
		actor   intentActor
	}{
		{"goal open", actorEither},
		{"goal edit", actorEither},
		{"goal claim", actorEither},
		{"goal release", actorEitherStopping},
		{"goal disallow", actorEither},
	} {
		b := newGrantEverythingBed(t)
		b.lineage = "lin-main"
		inv := &intentInvocation{command: mustIntentCommand(t, row.command), owners: b.owners(), stateRoot: b.root(),
			input: intentInput{values: map[string][]string{"origin": {"human"}}}}
		args, proof, problem := inv.actingAs("open", "g1", row.actor)
		if problem != nil || proof != nil || slices.Contains(args, "--by") {
			t.Fatalf("%s without a grant: the session acts as itself: %v %+v", row.command, args, problem)
		}
		b.admitted = &humanauthority.HelmGrant{By: "Wido", Class: lease.ClassMain, Grant: "01M-grant"}
		inv.owners = b.owners()
		args, _, problem = inv.actingAs("open", "g1", row.actor)
		if problem != nil || !slices.Equal(args, []string{"--by", "Wido"}) {
			t.Fatalf("%s under a grant: the act is the person's: %v %+v", row.command, args, problem)
		}
		named := &intentInvocation{command: inv.command, owners: b.owners(), stateRoot: b.root(),
			input: intentInput{values: map[string][]string{"lineage": {"lin-main"}}}}
		if args, _, _ = named.actingAs("open", "g1", row.actor); slices.Contains(args, "--by") {
			t.Fatalf("%s: a session naming itself with --lineage acts as itself: %v", row.command, args)
		}
	}
}

// goal allow refused every caller carrying a session lineage as an agent
// before asking the grant; under a live grant that admits the session the
// act is the granting person's and goes on to the person's proof.
func TestAllowAsksTheGrantBeforeRefusingTheSession(t *testing.T) {
	t.Parallel()
	b := newGrantEverythingBed(t)
	b.lineage = "lin-main"
	allow := []string{"goal", "allow", bedGoal, "stop-test-changes", "--reason", "the hook entry moved"}
	if _, result := b.runJSON(b.owners(), allow...); !strings.Contains(result.Summary, "runs as an agent session") {
		t.Fatalf("without a grant the session is refused as an agent: %+v", result)
	}
	b.admitted = &humanauthority.HelmGrant{By: "Wido", Class: lease.ClassMain, Grant: "01M-grant"}
	if code, result := b.runJSON(b.owners(), allow...); strings.Contains(result.Summary, "runs as an agent session") {
		t.Fatalf("under a grant the session was refused as an agent: %d %+v", code, result)
	}
}
