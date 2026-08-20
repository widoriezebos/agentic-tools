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

func TestClassifyHumanWhenNoAncestorRecognised(t *testing.T) {
	root := t.TempDir()
	got, err := Classify(root, childOf(t))
	if err != nil {
		t.Fatal(err)
	}
	if got.Class != ClassHuman {
		t.Fatalf("want HUMAN, got %+v", got)
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
	home := t.TempDir()
	t.Setenv("METASYSTEM_STEWARD_HOME", home)
	top, err := filepath.Abs(root)
	if err != nil {
		t.Fatal(err)
	}
	dir, err := steward.InstallDir(top)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	bin := filepath.Join(dir, "metasystem-steward")
	if err := os.Symlink("/bin/sleep", bin); err != nil {
		t.Fatal(err)
	}
	idPath, err := steward.IdentityPath(top)
	if err != nil {
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
	cmd := exec.Command(bin, "120")
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
	idPath, _ := steward.IdentityPath(top)
	if err := os.Chmod(idPath, 0o644); err != nil { // group/world readable = forged shape
		t.Fatal(err)
	}
	pid := spawnAndSettle(t, bin)
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
	got, err := Classify(root, pid)
	if err != nil {
		t.Fatal(err)
	}
	if got.Class == ClassSteward {
		t.Fatalf("another repository's steward must not classify here: %+v", got)
	}
}
