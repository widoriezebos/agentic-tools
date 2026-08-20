package lease

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
)

// childOf spawns a child whose parent is this test process, so classifying the
// child walks up through us — letting us stage what the ancestor resolves to.
func childOf(t *testing.T) int64 {
	t.Helper()
	cmd := exec.Command("/bin/sleep", "120")
	if err := cmd.Start(); err != nil {
		t.Fatalf("spawn child: %v", err)
	}
	t.Cleanup(func() { _ = cmd.Process.Kill(); _ = cmd.Wait() })
	return int64(cmd.Process.Pid)
}

func selfStart(t *testing.T) int64 {
	t.Helper()
	s, ok := StartedAt(int64(os.Getpid()), nil)
	if !ok {
		t.Fatal("could not read our own start")
	}
	return s
}

func writeJSON(t *testing.T, path string, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// stageTerminalFact pins the caller's controlling-terminal fact so
// these tests decide identically at a desk and on a headless runner.
func stageTerminalFact(t *testing.T, root string, pid int64, terminal bool) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte("metasystem.runtimes=fake\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	table := filepath.Join(t.TempDir(), "table.json")
	body := fmt.Sprintf(`{"%d": {"terminal": %v}}`, pid, terminal)
	if err := os.WriteFile(table, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("METASYSTEM_FAKE_PROCESS_IDENTITY_FILE", table)
}

func TestClassifyHumanWhenNoAncestorRecognisedAtATerminal(t *testing.T) {
	root := t.TempDir()
	caller := childOf(t)
	stageTerminalFact(t, root, caller, true)
	got, err := Classify(root, caller)
	if err != nil {
		t.Fatal(err)
	}
	if got.Class != ClassHuman {
		t.Fatalf("want HUMAN, got %+v", got)
	}
}

func TestClassifyHeadlessUnrecognisedIsUntrusted(t *testing.T) {
	root := t.TempDir()
	caller := childOf(t)
	stageTerminalFact(t, root, caller, false)
	got, err := Classify(root, caller)
	if err != nil {
		t.Fatal(err)
	}
	if got.Class != ClassUntrusted {
		t.Fatalf("a headless unrecognised caller must be UNTRUSTED, got %+v", got)
	}
}

func TestClassifyMainThroughAnnouncedAncestor(t *testing.T) {
	root := t.TempDir()
	self := int64(os.Getpid())
	if _, err := Announce(root, "sess", self, selfStart(t), "tag", "fake", ""); err != nil {
		t.Fatalf("announce self: %v", err)
	}
	got, err := Classify(root, childOf(t))
	if err != nil {
		t.Fatal(err)
	}
	if got.Class != ClassMain {
		t.Fatalf("a child of an announced main is that main's work; got %+v", got)
	}
}

func TestClassifySupervisionThroughAncestor(t *testing.T) {
	root := t.TempDir()
	self := int64(os.Getpid())
	writeJSON(t, filepath.Join(root, "artifacts/agents/supervision/state.json"),
		fmt.Sprintf(`{"owner":{"pid":%d,"pidStartedAt":%d,"instanceTag":"o"},"components":{}}`, self, selfStart(t)))
	got, err := Classify(root, childOf(t))
	if err != nil {
		t.Fatal(err)
	}
	if got.Class != ClassSupervision || got.Pid != self {
		t.Fatalf("want SUPERVISION at our pid, got %+v", got)
	}
}

func TestClassifyAdapterSupervisorThroughAncestor(t *testing.T) {
	root := t.TempDir()
	self := int64(os.Getpid())
	writeJSON(t, filepath.Join(root, "artifacts/agents/jobs/job-1.json"),
		fmt.Sprintf(`{"jobId":"job-1","pid":%d,"pidStartedAt":%d}`, self, selfStart(t)))
	got, err := Classify(root, childOf(t))
	if err != nil {
		t.Fatal(err)
	}
	if got.Class != ClassAdapterSupervisor || got.JobId != "job-1" {
		t.Fatalf("want ADAPTER-SUPERVISOR of job-1, got %+v", got)
	}
}

func TestClassifyDelegateThroughSignedAncestor(t *testing.T) {
	root := t.TempDir()
	self := int64(os.Getpid())
	command, ok := ProcessCommand(self, nil)
	if !ok {
		t.Skip("cannot read our own command to build a matching signature")
	}
	// An adapter whose signature matches our exact command, so the child's
	// ancestor (us) classifies as a delegate of that runtime.
	adapterDir := filepath.Join(root, "scripts/agents/adapters")
	if err := os.MkdirAll(adapterDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// printf the pattern as an argument, not a format string, so QuoteMeta's
	// backslashes reach the signature registry intact.
	line := "match " + regexp.QuoteMeta(command)
	script := "#!/bin/sh\n[ \"$1\" = signature ] && printf '%s\\n' '" + line + "'\n"
	if err := os.WriteFile(filepath.Join(adapterDir, "fake.sh"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	got, err := Classify(root, childOf(t))
	if err != nil {
		t.Fatal(err)
	}
	if got.Class != ClassDelegate || got.Pid != self {
		t.Fatalf("want DELEGATE at our pid, got %+v", got)
	}
}

func TestReadAnnouncementsSkipsIncompleteButRefusesTampered(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "artifacts/agents/mains")
	// An older record without identity fields: skipped, not an error.
	writeJSON(t, filepath.Join(dir, "old.json"),
		`{"sessionId":"s","pid":1,"pidStartedAt":1,"pgid":1,"runtime":"fake","instanceTag":"t","announcedAt":"a"}`)
	if recs, err := readAnnouncements(root, true); err != nil || len(recs) != 0 {
		t.Fatalf("an identity-less announcement should be skipped, not refuse: %v %d", err, len(recs))
	}
	// A tampering-shaped record (bad mainId) refuses a strict read.
	writeJSON(t, filepath.Join(dir, "bad.json"),
		`{"sessionId":"s","pid":1,"pidStartedAt":1,"pgid":1,"runtime":"fake","instanceTag":"t","announcedAt":"a","mainId":"NOPE","commandHash":"`+CommandHash("x")+`"}`)
	if _, err := readAnnouncements(root, true); err == nil {
		t.Fatal("a malformed main identity must refuse a strict read")
	}
	// The same malformed record is skipped by a lax read.
	if _, err := readAnnouncements(root, false); err != nil {
		t.Fatalf("a lax read must skip a malformed record, not error: %v", err)
	}
}

// B1 critique finding 15: a leaked fixture in a non-fake checkout
// refuses CLASSIFICATION itself — the lease's every decision, takeover
// included, sits behind this gate.
func TestClassifyRefusesLeakedFixture(t *testing.T) {
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte("metasystem.runtimes=claude\n"), 0o644)
	table := filepath.Join(t.TempDir(), "table.json")
	os.WriteFile(table, []byte(`{}`), 0o644)
	t.Setenv("METASYSTEM_FAKE_PROCESS_IDENTITY_FILE", table)
	if _, err := Classify(root, int64(os.Getpid())); err == nil ||
		!strings.Contains(err.Error(), "metasystem.runtimes is not fake") {
		t.Fatalf("leaked fixture did not refuse classification: %v", err)
	}
}

// Issue #12: the Devin HOST CLI's internal raw `devin acp` helper sits
// between the announced main and every orchestrator tool shell. The
// signature exclusion lets the ancestry walk continue THROUGH the helper
// to the announced main — while the delegate-side server (argv0
// devin-delegate-acp) still stops the walk as a delegate. The ancestry is
// a real process chain; the intermediate's argv is supplied through the
// authorized identity fixture (a shebang exec rewrites argv0, so a real
// process cannot carry the helper's exact command).
func TestClassifyDevinAcpHelperWalksToAnnouncedMain(t *testing.T) {
	got := classifyThroughDevinShape(t, "devin acp")
	if got.Class != ClassMain {
		t.Fatalf("a tool shell under the host's acp helper must classify MAIN through the exclusion; got %+v", got)
	}
}

func TestClassifyDevinDelegateServerStaysDelegate(t *testing.T) {
	got := classifyThroughDevinShape(t, "devin-delegate-acp acp")
	if got.Class != ClassDelegate {
		t.Fatalf("a tool shell under the delegate acp server must stay DELEGATE even with a main announced above; got %+v", got)
	}
}

// classifyThroughDevinShape stages: announced main (this test) <-
// intermediate whose fixture command is the given argv <- tool child, and
// classifies the child.
func classifyThroughDevinShape(t *testing.T, intermediateCommand string) Classification {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "metasystem.conf"),
		[]byte("metasystem.runtimes=fake\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	self := int64(os.Getpid())
	if _, err := Announce(root, "sess", self, selfStart(t), "tag", "fake", ""); err != nil {
		t.Fatalf("announce self: %v", err)
	}
	writeDevinAdapter(t, root)
	intermediate, child := grandchild(t, root)
	table := fmt.Sprintf(`{"%d":{"pidStartedAt":1,"command":%q}}`, intermediate, intermediateCommand)
	tablePath := filepath.Join(root, "identity.json")
	if err := os.WriteFile(tablePath, []byte(table), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("METASYSTEM_FAKE_PROCESS_IDENTITY_FILE", tablePath)
	got, err := Classify(root, child)
	if err != nil {
		t.Fatal(err)
	}
	return got
}

// writeDevinAdapter ships the real devin signature lines into a bed.
func writeDevinAdapter(t *testing.T, root string) {
	t.Helper()
	adapterDir := filepath.Join(root, "scripts/agents/adapters")
	if err := os.MkdirAll(adapterDir, 0o755); err != nil {
		t.Fatal(err)
	}
	script := `#!/bin/sh
[ "$1" = signature ] && printf '%s\n' \
  'match ^([^[:space:]]*/)?devin([[:space:]]|$)' \
  'match ^([^[:space:]]*/)?devin-delegate-acp([[:space:]]|$)' \
  'exclude ^([^[:space:]]*/)?devin[[:space:]]+acp([[:space:]]|$)'
`
	if err := os.WriteFile(filepath.Join(adapterDir, "devin.sh"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
}

// grandchild spawns a real intermediate (a plain shell) and returns both
// its pid and its child's pid — the ancestry shape the classify walk
// reads; the intermediate's COMMAND is overridden through the fixture.
func grandchild(t *testing.T, root string) (int64, int64) {
	t.Helper()
	pidFile := filepath.Join(root, "grandchild.pid")
	cmd := exec.Command("/bin/sh", "-c", "sleep 120 & echo $! > \""+pidFile+"\"; wait")
	if err := cmd.Start(); err != nil {
		t.Fatalf("spawn intermediate: %v", err)
	}
	t.Cleanup(func() { _ = cmd.Process.Kill(); _ = cmd.Wait() })
	deadline := time.Now().Add(10 * time.Second)
	for {
		data, err := os.ReadFile(pidFile)
		if err == nil && len(data) > 0 {
			pid, perr := strconv.ParseInt(strings.TrimSpace(string(data)), 10, 64)
			if perr != nil {
				t.Fatalf("grandchild pid unreadable: %v", perr)
			}
			t.Cleanup(func() { _ = syscall.Kill(int(pid), syscall.SIGKILL) })
			return int64(cmd.Process.Pid), pid
		}
		if time.Now().After(deadline) {
			t.Fatal("intermediate never published its child pid")
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// stageStewardInstall mints a valid installation whose binary is a real
// executable we can spawn, so a live process runs "the installed steward".
func stageStewardInstall(t *testing.T, root string) string {
	t.Helper()
	top, err := filepath.Abs(root)
	if err != nil {
		t.Fatal(err)
	}
	bin := filepath.Join(top, "bin", "metasystem-steward")
	if err := os.MkdirAll(filepath.Dir(bin), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("/usr/bin/yes", bin); err != nil {
		t.Fatal(err)
	}
	idPath := steward.RepoIdentityPath(top)
	if err := os.MkdirAll(filepath.Dir(idPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := steward.MintIdentity(idPath, steward.InstallIdentity{
		RepoIdentity: top, Generation: 1, InstallPath: bin, MintedAt: "2026-08-20T10:00:00Z",
	}); err != nil {
		t.Fatal(err)
	}
	return bin
}

// spawnAndSettle starts the binary and waits until the kernel reports
// its argv — probing mid-exec reads an empty command.
func spawnAndSettle(t *testing.T, bin string) int64 {
	t.Helper()
	// The argv carries the steward family token: classification is
	// scoped to "<installed binary> steward …", never the bare
	// executable.
	cmd := exec.Command(bin, "steward")
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cmd.Process.Kill(); _ = cmd.Wait() })
	pid := int64(cmd.Process.Pid)
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if command, ok := ProcessCommand(pid, nil); ok && command != "" {
			return pid
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("spawned steward never became readable")
	return 0
}

func TestClassifyStewardByInstalledBinaryAndIdentity(t *testing.T) {
	root := t.TempDir()
	bin := stageStewardInstall(t, root)
	pid := spawnAndSettle(t, bin)
	stageTerminalFact(t, root, pid, false)
	got, err := Classify(root, pid)
	if err != nil {
		t.Fatal(err)
	}
	if got.Class != ClassSteward {
		t.Fatalf("the installed steward binary with a valid identity must classify STEWARD, got %+v", got)
	}
}

func TestForgedStewardIdentityDoesNotClassifySteward(t *testing.T) {
	root := t.TempDir()
	bin := stageStewardInstall(t, root)
	top, _ := filepath.Abs(root)
	idPath := steward.RepoIdentityPath(top)
	if err := os.Chmod(idPath, 0o644); err != nil { // group/world readable = forged shape
		t.Fatal(err)
	}
	pid := spawnAndSettle(t, bin)
	stageTerminalFact(t, root, pid, false)
	got, err := Classify(root, pid)
	if err != nil {
		t.Fatal(err)
	}
	if got.Class == ClassSteward {
		t.Fatalf("a loose-permission identity must not authenticate the steward: %+v", got)
	}
}

func TestStewardOfAnotherRepositoryIsNotThisSteward(t *testing.T) {
	root := t.TempDir()
	other := t.TempDir()
	bin := stageStewardInstall(t, other) // installed for a different repo
	pid := spawnAndSettle(t, bin)
	stageTerminalFact(t, root, pid, false)
	got, err := Classify(root, pid)
	if err != nil {
		t.Fatal(err)
	}
	if got.Class == ClassSteward {
		t.Fatalf("another repository's steward must not classify here: %+v", got)
	}
}

func TestExecutableComparisonHandlesTokensAndSymlinks(t *testing.T) {
	if commandExecutable("/bin/tool --flag value") != "/bin/tool" {
		t.Fatal("the executable is the command's first token")
	}
	if commandExecutable("/bin/bare") != "/bin/bare" {
		t.Fatal("a bare command is its own token")
	}
	dir := t.TempDir()
	link := filepath.Join(dir, "alias")
	if err := os.Symlink("/bin/sleep", link); err != nil {
		t.Fatal(err)
	}
	if !sameExecutable(link, "/bin/sleep") {
		t.Fatal("symlink-resolved paths compare equal")
	}
	if sameExecutable(filepath.Join(dir, "missing-a"), filepath.Join(dir, "missing-b")) {
		t.Fatal("two unresolvable different paths are not equal")
	}
}

func TestFixtureTerminalReadsFixtureOnly(t *testing.T) {
	if has, present := probeFixtureTerminal(int64(os.Getpid()), nil); present || has {
		t.Fatal("a nil probe stages nothing, and the kernel is never consulted here")
	}
}

func TestNoStewardInstallationMeansNoStewardBinary(t *testing.T) {
	if got := verifiedStewardBinary(t.TempDir()); got != "" {
		t.Fatalf("no identity record, no recognized binary: %q", got)
	}
}

func TestClassifyStewardJobRidesTheActiveIntent(t *testing.T) {
	root := t.TempDir()
	out, err := ClassifyVerb(root, int64(os.Getpid()))
	if err != nil {
		t.Fatal(err)
	}
	if out.StewardJob != "" {
		t.Fatalf("no consumed intent, no steward job: %+v", out)
	}
}

func TestClassifyVerbReportsAnAnnouncedHolder(t *testing.T) {
	root := t.TempDir()
	self := int64(os.Getpid())
	if _, err := Announce(root, "cov sess", self, selfStart(t), "tag", "fake", ""); err != nil {
		t.Fatal(err)
	}
	out, err := ClassifyVerb(root, self)
	if err != nil {
		t.Fatal(err)
	}
	if out.Class != ClassMain || !out.Holder || out.MainId == "" || out.ClaimEpoch == nil {
		t.Fatalf("an announced self is the holding main with lease coordinates: %+v", out)
	}
	view, err := RequireHolder(root, self, nil)
	if err != nil || !view.Holder {
		t.Fatalf("require-holder agrees: %+v %v", view, err)
	}
}

func TestAnnouncementsForFindsExactlyOurRecords(t *testing.T) {
	root := t.TempDir()
	self := int64(os.Getpid())
	if _, err := Announce(root, "af sess", self, selfStart(t), "tag", "fake", ""); err != nil {
		t.Fatal(err)
	}
	if got := AnnouncementsFor(root, self); len(got) != 1 || got[0].Pid != self {
		t.Fatalf("our one announcement: %+v", got)
	}
	if got := AnnouncementsFor(root, self+999999); len(got) != 0 {
		t.Fatalf("a stranger pid has none: %+v", got)
	}
	if got := AnnouncementsFor(t.TempDir(), self); got != nil {
		t.Fatalf("an empty root has none: %+v", got)
	}
}

func TestTerminalBearingCallerOfInstalledBinaryStaysHuman(t *testing.T) {
	root := t.TempDir()
	bin := stageStewardInstall(t, root)
	pid := spawnAndSettle(t, bin)
	stageTerminalFact(t, root, pid, true)
	got, err := Classify(root, pid)
	if err != nil {
		t.Fatal(err)
	}
	// Arming installs the identity for the same binary every verb
	// runs. A person at a terminal invoking it is a person: STEWARD
	// classification here would silently disable every human-reserved
	// verb — taint resolution included — the moment the watchdog arms.
	if got.Class != ClassHuman {
		t.Fatalf("a terminal-bearing caller of the installed binary must stay HUMAN, got %+v", got)
	}
}

func TestBinaryAncestorDoesNotPreemptTheAnnouncedMain(t *testing.T) {
	root := t.TempDir()
	self := int64(os.Getpid())
	if _, err := Announce(root, "sess", self, selfStart(t), "tag", "fake", ""); err != nil {
		t.Fatalf("announce self: %v", err)
	}
	bin := stageStewardInstall(t, root)
	if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte("metasystem.runtimes=fake\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	intermediate, child := grandchild(t, root)
	// The intermediate "is" the installed binary mid-dispatch — the
	// exact shape lease run-held puts between a job child and its
	// MAIN. The walk must reach the MAIN above it: STEWARD here
	// would refuse every holder-only write of ordinary dispatch the
	// moment the watchdog arms.
	table := fmt.Sprintf(`{"%d": {"pidStartedAt": 1, "command": %q}}`, intermediate, bin+" lease run-held --root .")
	tablePath := filepath.Join(root, "identity.json")
	if err := os.WriteFile(tablePath, []byte(table), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("METASYSTEM_FAKE_PROCESS_IDENTITY_FILE", tablePath)
	got, err := Classify(root, child)
	if err != nil {
		t.Fatal(err)
	}
	if got.Class != ClassMain {
		t.Fatalf("a job child under run-held under an announced main is that main's work, got %+v", got)
	}
}

func TestHeadlessChildOfStewardChainClassifiesSteward(t *testing.T) {
	root := t.TempDir()
	bin := stageStewardInstall(t, root)
	if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte("metasystem.runtimes=fake\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	intermediate, child := grandchild(t, root)
	table := fmt.Sprintf(`{"%d": {"pidStartedAt": 1, "command": %q}, "%d": {"terminal": false}}`, intermediate, bin+" steward revive --repo .", child)
	tablePath := filepath.Join(root, "identity.json")
	if err := os.WriteFile(tablePath, []byte(table), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("METASYSTEM_FAKE_PROCESS_IDENTITY_FILE", tablePath)
	got, err := Classify(root, child)
	if err != nil {
		t.Fatal(err)
	}
	if got.Class != ClassSteward {
		t.Fatalf("the runner's own headless chain is the steward, got %+v", got)
	}
}

func TestTerminalCallerUnderPlumbingAncestorStaysHuman(t *testing.T) {
	root := t.TempDir()
	bin := stageStewardInstall(t, root)
	if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte("metasystem.runtimes=fake\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	intermediate, child := grandchild(t, root)
	// A NON-steward invocation of the installed binary (run-held) is
	// transparent, so the terminal decides: pre-arming parity for a
	// person's direct dispatch with no enrolled main.
	table := fmt.Sprintf(`{"%d": {"pidStartedAt": 1, "command": %q}, "%d": {"terminal": true}}`, intermediate, bin+" lease run-held --root .", child)
	tablePath := filepath.Join(root, "identity.json")
	if err := os.WriteFile(tablePath, []byte(table), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("METASYSTEM_FAKE_PROCESS_IDENTITY_FILE", tablePath)
	got, err := Classify(root, child)
	if err != nil {
		t.Fatal(err)
	}
	if got.Class != ClassHuman {
		t.Fatalf("a terminal-bearing caller under transparent plumbing is a person, got %+v", got)
	}
}

func TestDelegateAncestorAboveStewardChainDoesNotPreempt(t *testing.T) {
	root := t.TempDir()
	bin := stageStewardInstall(t, root)
	if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte("metasystem.runtimes=fake\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// A runtime process above the tick (a fixture suite inside an
	// agent session, an agent-invoked manual tick): the NEAREST
	// recognised ancestor — the steward plumbing — owns the chain.
	writeDevinAdapter(t, root)
	intermediate, child := grandchild(t, root)
	// The test process itself wears a delegate's command: the walk
	// WOULD reach it two hops above the child if the steward
	// plumbing did not return first.
	table := fmt.Sprintf(`{"%d": {"pidStartedAt": 1, "command": %q}, "%d": {"pidStartedAt": 1, "command": "devin-delegate-acp session"}, "%d": {"terminal": false}}`,
		intermediate, bin+" steward revive --repo .", int64(os.Getpid()), child)
	tablePath := filepath.Join(root, "identity.json")
	if err := os.WriteFile(tablePath, []byte(table), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("METASYSTEM_FAKE_PROCESS_IDENTITY_FILE", tablePath)
	got, err := Classify(root, child)
	if err != nil {
		t.Fatal(err)
	}
	if got.Class != ClassSteward {
		t.Fatalf("the steward plumbing is the nearest recognised ancestor, got %+v", got)
	}
}
