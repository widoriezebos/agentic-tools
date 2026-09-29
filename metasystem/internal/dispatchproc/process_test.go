package dispatchproc

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/census"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

// absentPID is far above any kernel pid limit, so the kernel always
// answers that it is not running.
const absentPID int64 = 1 << 30

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// fakeRoot is a checkout root configured for the fake runtime.
func fakeRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "metasystem.conf"), "metasystem.runtimes=fake\n")
	return root
}

func writeJSON(t *testing.T, path string, value any) {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, path, string(data))
}

func TestPositionedJobTagMatchesOnlyTheShippedTagPosition(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		argv []string
		want bool
	}{
		{"owner flag and value", []string{"/usr/local/bin/metasystem", "supervise", "--tag", "job-1"}, true},
		{"owner equals spelling", []string{"metasystem", "supervise", "--tag=job-1"}, true},
		{"mission run loop", []string{"metasystem", "mission", "run-loop", "--instance-tag", "job-1"}, true},
		{"other tag value", []string{"metasystem", "supervise", "--tag", "job-2"}, false},
		{"tag outside its flag", []string{"metasystem", "supervise", "job-1"}, false},
		{"missing command word", []string{"metasystem", "--tag", "job-1"}, false},
		{"substring of a word is not the command", []string{"notmetasystem", "supervise", "--tag", "job-1"}, false},
		{"empty argv", nil, false},
	}
	at := PositionedJobTagAt("")
	atRoot := PositionedJobTagAt(t.TempDir())
	for _, tc := range cases {
		if got := at(tc.argv, "job-1"); got != tc.want {
			t.Errorf("%s: PositionedJobTagAt(\"\") = %v, want %v", tc.name, got, tc.want)
		}
		if got := atRoot(tc.argv, "job-1"); got != tc.want {
			t.Errorf("%s: PositionedJobTagAt(empty root) = %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestClaimProcessVerifierRefusesAnUntaggedLiveProcessAndReportsAnAbsentOne(t *testing.T) {
	t.Parallel()
	verifier := ClaimProcessVerifier{Root: t.TempDir()}
	own := verifier.Verify(int64(os.Getpid()), "job-never-carried-by-a-test-binary")
	if own.Outcome != identity.VerificationNotOurs || own.Presence != identity.Alive {
		t.Fatalf("own process verification = %+v, want not-ours and alive", own)
	}
	if own.Identity.Pid != int64(os.Getpid()) {
		t.Fatalf("own process identity pid = %d, want %d", own.Identity.Pid, os.Getpid())
	}
	absent := verifier.Verify(absentPID, "job-1")
	if absent.Outcome != identity.VerificationDead || absent.Presence != identity.Dead {
		t.Fatalf("absent process verification = %+v, want dead", absent)
	}
}

func TestStartReaderServesTheFixtureOnlyForAFakeRoot(t *testing.T) {
	pid := int64(os.Getpid())
	started := int64(1_700_000_000)
	exactMicro := started*1_000_000 + 123_456
	table := filepath.Join(t.TempDir(), "identity.json")
	writeJSON(t, table, map[string]any{strconv.FormatInt(pid, 10): map[string]any{
		"pidStartedAt": started, "pidStartedAtExactMicro": exactMicro,
		"pidStartTicks": 99, "bootId": "boot-a",
	}})
	t.Setenv("METASYSTEM_FAKE_PROCESS_IDENTITY_FILE", table)

	production := t.TempDir()
	writeFile(t, filepath.Join(production, "metasystem.conf"), "metasystem.runtimes=claude\n")
	if reader, err := StartReader(production); err == nil {
		t.Fatalf("a non-fake root accepted the identity fixture: %#v", reader)
	}

	reader, err := StartReader(fakeRoot(t))
	if err != nil {
		t.Fatal(err)
	}
	exact, state, err := reader.ReadStart(pid)
	if err != nil || state != identity.Alive {
		t.Fatalf("ReadStart(own pid) = %v, %v; want alive", state, err)
	}
	if exact.StartedAt.Unix() != started {
		t.Fatalf("ReadStart(own pid) start = %v, want the fixture's second %d", exact.StartedAt, started)
	}
	if _, state, err := reader.ReadStart(absentPID); err != nil || state != identity.Dead {
		t.Fatalf("ReadStart(absent pid) = %v, %v; want dead (kernel absence wins)", state, err)
	}
}

func TestStartReaderWithoutFixtureReadsTheKernel(t *testing.T) {
	t.Setenv("METASYSTEM_FAKE_PROCESS_IDENTITY_FILE", "")
	reader, err := StartReader(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	exact, state, err := reader.ReadStart(int64(os.Getpid()))
	if err != nil || state != identity.Alive || exact.Pid != int64(os.Getpid()) {
		t.Fatalf("ReadStart(own pid) = %+v, %v, %v; want the live kernel identity", exact, state, err)
	}
}

func TestTaggedProcessScannerRefusesAProcessFixtureOnANonFakeRoot(t *testing.T) {
	processes := filepath.Join(t.TempDir(), "processes.json")
	writeFile(t, processes, "[]")
	t.Setenv("METASYSTEM_CENSUS_PROCESS_FILE", processes)
	t.Setenv("METASYSTEM_FAKE_PROCESS_IDENTITY_FILE", "")
	result := TaggedProcessScanner{Root: t.TempDir()}.ScanTag("job-1", time.Time{})
	if result.EnumerationError == "" || result.Complete() {
		t.Fatalf("scan on a non-fake root = %+v, want an enumeration error", result)
	}
}

func TestTaggedProcessScannerReportsAnUnreadableProcessFixture(t *testing.T) {
	processes := filepath.Join(t.TempDir(), "processes.json")
	writeFile(t, processes, "not json")
	t.Setenv("METASYSTEM_CENSUS_PROCESS_FILE", processes)
	t.Setenv("METASYSTEM_FAKE_PROCESS_IDENTITY_FILE", "")
	result := TaggedProcessScanner{Root: fakeRoot(t)}.ScanTag("job-1", time.Time{})
	if result.EnumerationError == "" {
		t.Fatalf("scan of an unreadable fixture = %+v, want an enumeration error", result)
	}
}

// TestTaggedProcessScannerClassifiesTheFixtureTable drives the fixture path
// with live pids (this test binary and its parent) so the kernel's
// liveness veto passes: the tagged row is found with the fixture's group,
// a quoted command cannot prove a tag and stays indeterminate, a dead row
// is skipped, and a row the kernel does not know is dropped.
func TestTaggedProcessScannerClassifiesTheFixtureTable(t *testing.T) {
	own := int64(os.Getpid())
	parent := int64(os.Getppid())
	dir := t.TempDir()
	processes := filepath.Join(dir, "processes.json")
	writeJSON(t, processes, []census.Process{
		{Pid: own, PGID: 4242, Started: 1_700_000_000, Argv: "metasystem supervise --tag job-1", Alive: true},
		{Pid: parent, PGID: 5151, Started: 1_700_000_000, Argv: `metasystem supervise --tag "job-1"`, Alive: true},
		{Pid: absentPID, PGID: 1, Started: 1_700_000_000, Argv: "metasystem supervise --tag job-1", Alive: true},
		{Pid: absentPID + 1, PGID: 1, Started: 1_700_000_000, Argv: "metasystem supervise --tag job-1", Alive: false},
	})
	t.Setenv("METASYSTEM_CENSUS_PROCESS_FILE", processes)
	t.Setenv("METASYSTEM_FAKE_PROCESS_IDENTITY_FILE", "")

	scanner := TaggedProcessScanner{Root: fakeRoot(t)}
	result := scanner.ScanTag("job-1", time.Time{})
	if result.EnumerationError != "" {
		t.Fatalf("enumeration error: %s", result.EnumerationError)
	}
	if len(result.Tagged) != 1 || result.Tagged[0].PID != own || result.Tagged[0].PGID != 4242 {
		t.Fatalf("tagged = %+v, want only pid %d in group 4242", result.Tagged, own)
	}
	if result.Tagged[0].Universe != census.ProcessUniverseSignalable {
		t.Fatalf("tagged universe = %q, want signalable", result.Tagged[0].Universe)
	}
	if len(result.Indeterminate) != 1 || result.Indeterminate[0].PID != parent {
		t.Fatalf("indeterminate = %+v, want only the quoted-command pid %d", result.Indeterminate, parent)
	}
	if result.Complete() {
		t.Fatal("a scan with an unreadable command in the universe claimed completeness")
	}

	other := scanner.ScanTag("job-2", time.Time{})
	if len(other.Tagged) != 0 {
		t.Fatalf("scan for another tag found %+v", other.Tagged)
	}
}

func TestTaggedProcessScannerBindsTheFixtureIdentity(t *testing.T) {
	own := int64(os.Getpid())
	started := int64(1_700_000_000)
	exactMicro := started*1_000_000 + 7
	dir := t.TempDir()
	processes := filepath.Join(dir, "processes.json")
	writeJSON(t, processes, []census.Process{
		{Pid: own, PGID: 77, Started: started, Argv: "metasystem supervise --tag job-1", Alive: true},
	})
	table := filepath.Join(dir, "identity.json")
	writeJSON(t, table, map[string]any{strconv.FormatInt(own, 10): map[string]any{
		"pidStartedAt": started, "pidStartedAtExactMicro": exactMicro,
		"pidStartTicks": 11, "bootId": "boot-a",
	}})
	t.Setenv("METASYSTEM_CENSUS_PROCESS_FILE", processes)
	t.Setenv("METASYSTEM_FAKE_PROCESS_IDENTITY_FILE", table)

	result := TaggedProcessScanner{Root: fakeRoot(t)}.ScanTag("job-1", time.Time{})
	if len(result.Tagged) != 1 || !result.Complete() {
		t.Fatalf("scan = %+v, want one tagged process and a complete census", result)
	}
	if got := result.Tagged[0].Identity.StartedAt.Unix(); got != started {
		t.Fatalf("tagged identity start = %d, want the fixture's %d", got, started)
	}
}

func TestTaggedProcessScannerFindsNoCarrierOfAnUnusedTagInTheLiveTable(t *testing.T) {
	t.Setenv("METASYSTEM_CENSUS_PROCESS_FILE", "")
	t.Setenv("METASYSTEM_FAKE_PROCESS_IDENTITY_FILE", "")
	result := TaggedProcessScanner{Root: t.TempDir()}.ScanTag("dispatchproc-test-tag-carried-by-nothing", time.Time{})
	if result.EnumerationError != "" {
		t.Fatalf("live enumeration error: %s", result.EnumerationError)
	}
	if len(result.Tagged) != 0 {
		t.Fatalf("live scan found carriers of an unused tag: %+v", result.Tagged)
	}
}

func TestFixtureAuthorizedNeedsBothFixtureFilesAndAFakeRoot(t *testing.T) {
	dir := t.TempDir()
	processes := filepath.Join(dir, "processes.json")
	identities := filepath.Join(dir, "identity.json")
	writeFile(t, processes, "[]")
	writeFile(t, identities, "{}")
	subdir := filepath.Join(dir, "a-directory")
	if err := os.Mkdir(subdir, 0o755); err != nil {
		t.Fatal(err)
	}
	fake := fakeRoot(t)
	fakeLater := t.TempDir()
	writeFile(t, filepath.Join(fakeLater, "metasystem.conf"), "# comment\nmetasystem.runtimes=fake\n")
	claude := t.TempDir()
	writeFile(t, filepath.Join(claude, "metasystem.conf"), "metasystem.runtimes=claude\n")
	unterminated := t.TempDir()
	writeFile(t, filepath.Join(unterminated, "metasystem.conf"), "metasystem.runtimes=fake")

	cases := []struct {
		name                string
		processes, identity string
		root                string
		want                bool
	}{
		{"both files and a fake root", processes, identities, fake, true},
		{"fake key after other lines", processes, identities, fakeLater, true},
		{"process file unset", "", identities, fake, false},
		{"identity file unset", processes, "", fake, false},
		{"process file missing", filepath.Join(dir, "missing.json"), identities, fake, false},
		{"identity path is a directory", processes, subdir, fake, false},
		{"root without configuration", processes, identities, t.TempDir(), false},
		{"root on another runtime", processes, identities, claude, false},
		{"fake key without its line end", processes, identities, unterminated, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("METASYSTEM_CENSUS_PROCESS_FILE", tc.processes)
			t.Setenv("METASYSTEM_FAKE_PROCESS_IDENTITY_FILE", tc.identity)
			if got := FixtureAuthorized(tc.root); got != tc.want {
				t.Fatalf("FixtureAuthorized = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestClaimAuthorizedLetsTheCompleteFixtureDriveCustodyWithoutACapability
// pins the one test seam: inside the delegate boundary, a complete fake
// fixture authorizes a claim with no bearer word, while outside the boundary
// even the fixture refuses.
func TestClaimAuthorizedLetsTheCompleteFixtureDriveCustodyWithoutACapability(t *testing.T) {
	dir := t.TempDir()
	processes := filepath.Join(dir, "processes.json")
	identities := filepath.Join(dir, "identity.json")
	writeFile(t, processes, "[]")
	writeFile(t, identities, "{}")
	t.Setenv("METASYSTEM_CENSUS_PROCESS_FILE", processes)
	t.Setenv("METASYSTEM_FAKE_PROCESS_IDENTITY_FILE", identities)
	root := fakeRoot(t)
	binding := dispatch.DelegateClaimCapabilityBinding{JobID: "j1", DispatchMode: dispatch.DispatchModeFresh, AdapterVerb: "dispatch"}
	for _, preflight := range []bool{true, false} {
		if !ClaimAuthorized(root, ClaimSurface{DelegateInternal: true}, binding, preflight) {
			t.Fatalf("preflight=%v: the complete fixture did not authorize the delegate boundary", preflight)
		}
		if ClaimAuthorized(root, ClaimSurface{}, binding, preflight) {
			t.Fatalf("preflight=%v: the fixture authorized a caller outside the delegate boundary", preflight)
		}
	}
}

func TestClaimAuthorizedRefusesAnUnmintedCapability(t *testing.T) {
	t.Setenv("METASYSTEM_CENSUS_PROCESS_FILE", "")
	t.Setenv("METASYSTEM_FAKE_PROCESS_IDENTITY_FILE", "")
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "artifacts", "agents"), 0o755); err != nil {
		t.Fatal(err)
	}
	binding := dispatch.DelegateClaimCapabilityBinding{JobID: "j1", DispatchMode: dispatch.DispatchModeFresh, AdapterVerb: "dispatch"}
	surface := ClaimSurface{DelegateInternal: true, Capability: "never-minted"}
	for _, preflight := range []bool{true, false} {
		if ClaimAuthorized(root, surface, binding, preflight) {
			t.Fatalf("preflight=%v: an unminted capability authorized a claim", preflight)
		}
	}
}
