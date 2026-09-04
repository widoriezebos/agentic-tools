package fixtureauth

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/unix"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

func fakeCheckout(t *testing.T, runtimes string) string {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "metasystem.conf"),
		[]byte("metasystem.runtimes="+runtimes+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func writeTable(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "table.json")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// The authorization matrix: env unset is benign; a fake checkout
// serves; a non-fake or conf-less checkout REFUSES construction — the
// leaked-fixture fence at every entry point.
func TestAuthorizationMatrix(t *testing.T) {
	table := writeTable(t, `{"42":{"pidStartedAt":100,"command":"runner --tag t","pgid":42}}`)

	os.Unsetenv("METASYSTEM_FAKE_PROCESS_IDENTITY_FILE")
	authorization, err := New(fakeCheckout(t, "claude"))
	if err != nil {
		t.Fatalf("env unset must be benign: %v", err)
	}
	if _, ok := authorization.Identity().FixtureEntry(42); ok {
		t.Fatal("no fixture in play yet an entry was served")
	}

	t.Setenv("METASYSTEM_FAKE_PROCESS_IDENTITY_FILE", table)
	if _, err := New(fakeCheckout(t, "claude")); err == nil {
		t.Fatal("a leaked fixture in a claude checkout was authorized")
	}
	if _, err := New(t.TempDir()); err == nil {
		t.Fatal("an unreadable conf authorized a fixture")
	}
	authorization, err = New(fakeCheckout(t, "fake"))
	if err != nil {
		t.Fatalf("a fake checkout must authorize: %v", err)
	}
	entry, ok := authorization.Identity().FixtureEntry(42)
	if !ok || entry.StartedAt != 100 {
		t.Fatalf("authorized identity read failed: %+v ok=%v", entry, ok)
	}
}

// Each capability value exposes ONLY its authority's facts, and every
// zero/nil value refuses. The group grant additionally demands a
// KERNEL-LIVE leader at its recorded start in its recorded group,
// so the fixture row is built from this test's
// own live process.
func TestCapabilityScoping(t *testing.T) {
	self := int64(os.Getpid())
	exact, _, err := (identity.KernelProber{}).Probe(self)
	if err != nil {
		t.Fatal(err)
	}
	selfPgid, err := unix.Getpgid(int(self))
	if err != nil {
		t.Fatal(err)
	}
	table := writeTable(t, fmt.Sprintf(
		`{"%d":{"pidStartedAt":%d,"command":"runner --tag t","pgid":%d},"7":{"pidStartedAt":5},"42":{"pidStartedAt":100,"command":"stale row","pgid":42}}`,
		self, exact.StartedAt.Unix(), selfPgid))
	t.Setenv("METASYSTEM_FAKE_PROCESS_IDENTITY_FILE", table)
	authorization, err := New(fakeCheckout(t, "fake"))
	if err != nil {
		t.Fatal(err)
	}

	if command, ok := authorization.Command().FixtureCommand(self); !ok || command != "runner --tag t" {
		t.Fatalf("command probe: %q ok=%v", command, ok)
	}
	if _, ok := authorization.Command().FixtureCommand(7); ok {
		t.Fatal("a command-less entry served a command")
	}
	pgid, command, ok := authorization.GroupOwnership().FixtureGroup(self)
	if !ok || pgid != int64(selfPgid) || command != "runner --tag t" {
		t.Fatalf("group grant: %d %q ok=%v", pgid, command, ok)
	}
	if _, _, ok := authorization.GroupOwnership().FixtureGroup(7); ok {
		t.Fatal("a pgid-less entry proved group ownership")
	}
	// A stale row for a dead/recycled pid never authorizes a signal.
	if _, _, ok := authorization.GroupOwnership().FixtureGroup(42); ok {
		t.Fatal("a stale fixture row proved group ownership (finding 2)")
	}
	if path, ok := authorization.Publication().TablePath(); !ok || path != table {
		t.Fatalf("publication grant: %q ok=%v", path, ok)
	}
	if !authorization.Ancestor().Allows() || !authorization.MissionProcess().Allows() || !authorization.ProcessTable().Allows() {
		t.Fatal("authorized probes must allow")
	}
	fixtureRoot := fakeCheckout(t, "fake")
	fixtureAuthorization, err := New(fixtureRoot)
	if err != nil {
		t.Fatal(err)
	}
	if !fixtureAuthorization.GoalHumanAuthority().Allows(fixtureRoot) ||
		fixtureAuthorization.GoalHumanAuthority().Allows(fakeCheckout(t, "fake")) {
		t.Fatal("goal human authority was not bound to its exact fixture root")
	}

	// The zero/nil values refuse everything.
	var nilAuth *Authorization
	if _, ok := nilAuth.Identity().FixtureEntry(42); ok {
		t.Fatal("nil authorization served an entry")
	}
	var zeroGrant GroupOwnershipGrant
	if _, _, ok := zeroGrant.FixtureGroup(42); ok {
		t.Fatal("zero grant proved ownership")
	}
	var zeroPublication PublicationGrant
	if _, ok := zeroPublication.TablePath(); ok {
		t.Fatal("zero publication grant authorized a write")
	}
	var zeroCommand CommandProbe
	if _, ok := zeroCommand.FixtureCommand(42); ok {
		t.Fatal("zero command probe served a command")
	}
	var zeroGoalHumanAuthority GoalHumanAuthorityProbe
	if zeroGoalHumanAuthority.Allows(fixtureRoot) {
		t.Fatal("zero goal human authority authorized a mutation")
	}
}

// FixtureModeRoot is the shared env-independent predicate.
func TestFixtureModeRoot(t *testing.T) {
	if !FixtureModeRoot(fakeCheckout(t, "fake")) {
		t.Fatal("fake checkout not recognized")
	}
	if FixtureModeRoot(fakeCheckout(t, "claude")) || FixtureModeRoot(t.TempDir()) {
		t.Fatal("non-fake or conf-less checkout recognized as fixture mode")
	}
}

func TestGoalClockRequiresFixtureModeAndValidTime(t *testing.T) {
	t.Setenv(fixtureEnv, "")
	t.Setenv(goalNowEnv, "not-a-time")
	production, err := New(fakeCheckout(t, "claude"))
	if err != nil {
		t.Fatal(err)
	}
	if got, ok, err := production.Clock().GoalNow(); err != nil || ok || !got.IsZero() {
		t.Fatalf("production clock honored fixture input: time=%v ok=%v err=%v", got, ok, err)
	}

	fixture, err := New(fakeCheckout(t, "fake"))
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv(goalNowEnv, "")
	if got, ok, err := fixture.Clock().GoalNow(); err != nil || ok || !got.IsZero() {
		t.Fatalf("empty fixture time replaced the wall clock: time=%v ok=%v err=%v", got, ok, err)
	}

	t.Setenv(goalNowEnv, "2026-08-29T14:30:00+02:00")
	got, ok, err := fixture.Clock().GoalNow()
	want := time.Date(2026, 8, 29, 12, 30, 0, 0, time.UTC)
	if err != nil || !ok || !got.Equal(want) || got.Location() != time.UTC {
		t.Fatalf("fixture time = %v ok=%v err=%v, want %v in UTC", got, ok, err, want)
	}

	t.Setenv(goalNowEnv, "not-a-time")
	if _, ok, err := fixture.Clock().GoalNow(); err == nil || ok || !strings.Contains(err.Error(), goalNowEnv) {
		t.Fatalf("malformed fixture time was not refused by name: ok=%v err=%v", ok, err)
	}

	var zero ClockProbe
	if got, ok, err := zero.GoalNow(); err != nil || ok || !got.IsZero() {
		t.Fatalf("zero clock probe served fixture time: time=%v ok=%v err=%v", got, ok, err)
	}
}
