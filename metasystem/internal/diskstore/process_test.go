package diskstore

import (
	"bufio"
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"golang.org/x/sys/unix"
)

// processScratchHelperEnv selects a helper mode a process-scratch test runs
// in a child of this test binary, before testenv (TestMain): the child's
// TMPDIR and machine registry are the ones its parent test chose.
const processScratchHelperEnv = "DISKSTORE_SCRATCH_HELPER"

// processScratchHelper runs one helper mode; handled is false in a normal
// test process.
func processScratchHelper() (code int, handled bool) {
	mode := os.Getenv(processScratchHelperEnv)
	if mode == "" {
		return 0, false
	}
	fail := func(err error) (int, bool) {
		fmt.Fprintln(os.Stderr, "helper:", err)
		return 3, true
	}
	root, err := ProcessScratch()
	if err != nil {
		return fail(err)
	}
	fmt.Printf("root=%s\n", root)
	switch mode {
	case "hold":
		// A child that outlives this process holds the writer lock: cat
		// reads the parent test's pipe, so it ends when the test closes it.
		child := exec.Command("cat")
		child.Stdin, child.Stdout = os.Stdin, os.Stdout
		if err := PrepareChild(child); err != nil {
			return fail(err)
		}
		if err := child.Start(); err != nil {
			return fail(err)
		}
		fmt.Println("ready")
		wait := make(chan os.Signal, 1)
		signal.Notify(wait, syscall.SIGTERM)
		<-wait
		return 0, true
	case "exit":
		if _, _, err := ScratchDir("left-"); err != nil {
			return fail(err)
		}
		os.Exit(0)
	case "release":
		dir, done, err := ScratchDir("used-")
		if err != nil {
			return fail(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "bytes"), []byte("x"), 0o600); err != nil {
			return fail(err)
		}
		done()
		if err := ReleaseProcessScratch(context.Background()); err != nil {
			return fail(err)
		}
		return 0, true
	case "nested":
		child := exec.Command(os.Args[0])
		if err := PrepareChild(child); err != nil {
			return fail(err)
		}
		child.Env = append(child.Env, processScratchHelperEnv+"=nested-child")
		output, err := child.Output()
		if err != nil {
			return fail(fmt.Errorf("nested child: %v: %s", err, output))
		}
		fmt.Printf("child-%s", output)
		if err := ReleaseProcessScratch(context.Background()); err != nil {
			return fail(err)
		}
		return 0, true
	case "nested-child":
		fmt.Printf("tmpdir=%s\n", os.Getenv("TMPDIR"))
		if err := ReleaseProcessScratch(context.Background()); err != nil {
			return fail(err)
		}
		return 0, true
	}
	return fail(fmt.Errorf("unknown helper mode %q", mode))
}

// scratchBed is a helper process's TMPDIR and home state root, both inside
// the test's own directory.
type scratchBed struct {
	temp, home string
	registry   Registry
}

func newScratchBed(t *testing.T) scratchBed {
	t.Helper()
	dir := realDir(t)
	bed := scratchBed{temp: filepath.Join(dir, "tmp"), home: filepath.Join(dir, "home")}
	for _, path := range []string{bed.temp, bed.home} {
		if err := os.Mkdir(path, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	bed.registry = MachineRegistry(filepath.Join(bed.home, ".metasystem"))
	return bed
}

func (b scratchBed) helper(mode string) *exec.Cmd {
	command := exec.Command(os.Args[0], "-test.run=^$")
	command.Env = append(os.Environ(), processScratchHelperEnv+"="+mode, "TMPDIR="+b.temp,
		"METASYSTEM_SUPERVISION_REGISTRY_HOME="+b.home)
	return command
}

func helperValue(t *testing.T, output, key string) string {
	t.Helper()
	for _, line := range strings.Split(output, "\n") {
		if value, ok := strings.CutPrefix(line, key+"="); ok {
			return value
		}
	}
	t.Fatalf("helper printed no %s:\n%s", key, output)
	return ""
}

// onlyRecord is the bed's one process-scratch record.
func (b scratchBed) onlyRecord(t *testing.T) Record {
	t.Helper()
	records, unreadable := b.registry.Inventory()
	if len(records) != 1 || len(unreadable) != 0 {
		t.Fatalf("records = %+v, unreadable = %+v; want one", records, unreadable)
	}
	return records[0]
}

func (b scratchBed) sweep(t *testing.T, prober identity.Prober) Report {
	t.Helper()
	report, err := RunPass(context.Background(), passOptions(b.registry, realDir(t),
		RegisteredStores{Registry: b.registry, Proofs: map[OwnerKind]OwnerProof{OwnerProcess: ProcessProof{Prober: prober}}}))
	if err != nil {
		t.Fatal(err)
	}
	return report
}

// A killed owner whose child still runs leaves its record and root: the
// child inherited the writer lock, so the lock probe fails and the proof
// keeps the root. Once the child exits the probe succeeds, the owner's
// reference reads Dead, and the sweeper releases the root (3.1's process
// row; U1a).
func TestProcessScratchOfAKilledOwnerIsKeptWhileItsChildLives(t *testing.T) {
	t.Parallel()
	bed := newScratchBed(t)
	helper := bed.helper("hold")
	// Plain pipes, not StdinPipe/StdoutPipe: Wait on the killed helper must
	// not close the read end its child still writes to.
	childIn, stdin, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = stdin.Close() })
	stdout, childOut, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = stdout.Close() })
	helper.Stdin, helper.Stdout, helper.Stderr = childIn, childOut, os.Stderr
	if err := helper.Start(); err != nil {
		t.Fatal(err)
	}
	_ = childIn.Close()
	_ = childOut.Close()
	lines := bufio.NewScanner(stdout)
	var printed []string
	for lines.Scan() && lines.Text() != "ready" {
		printed = append(printed, lines.Text())
	}
	root := helperValue(t, strings.Join(printed, "\n"), "root")
	if err := helper.Process.Signal(syscall.SIGKILL); err != nil {
		t.Fatal(err)
	}
	_ = helper.Wait()
	record := bed.onlyRecord(t)
	if record.Path != root || record.State != StateAccepted || record.Owner.Kind != OwnerProcess {
		t.Fatalf("record = %+v, want the accepted process record of %s", record, root)
	}
	if free, err := ProbeWriterLock(record); err != nil || free {
		t.Fatalf("with the child alive the writer lock probe = free %v, %v; want held", free, err)
	}
	ref, err := identity.ParseRef(record.Owner.Ref)
	if err != nil {
		t.Fatal(err)
	}
	if state := identity.AliveRef(identity.KernelProber{}, ref); state != identity.Dead {
		t.Fatalf("the killed owner reads %s", state)
	}
	if verdict := (ProcessProof{Prober: identity.KernelProber{}}).Observe(context.Background(), record); verdict.Decision != Keep {
		t.Fatalf("with the child alive the proof says %+v; want keep", verdict)
	}
	bed.sweep(t, identity.KernelProber{})
	if _, err := os.Lstat(root); err != nil {
		t.Fatalf("a sweep removed a root its child still holds: %v", err)
	}
	// Closing the pipe ends cat. A blocking LOCK_EX through a description
	// of the test's own returns exactly when cat's inherited copy closed.
	_ = stdin.Close()
	waiter, err := os.Open(filepath.Join(root, WriterLockName))
	if err != nil {
		t.Fatal(err)
	}
	if err := unix.Flock(int(waiter.Fd()), unix.LOCK_EX); err != nil {
		t.Fatal(err)
	}
	_ = waiter.Close()
	if free, err := ProbeWriterLock(record); err != nil || !free {
		t.Fatalf("after the child exited the writer lock probe = free %v, %v; want free", free, err)
	}
	if verdict := (ProcessProof{Prober: identity.KernelProber{}}).Observe(context.Background(), record); verdict.Decision != Release {
		t.Fatalf("after the child exited the proof says %+v; want release", verdict)
	}
	bed.sweep(t, identity.KernelProber{})
	if _, err := os.Lstat(root); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("the sweep left the dead process's root: %v", err)
	}
	if released := bed.onlyRecord(t); released.State != StateReleased {
		t.Fatalf("record after the sweep = %s; the record is history and stays", released.State)
	}
}

// A normal end leaves nothing: the root, its record and the record lock go
// at the owner's release, and the scratch parent keeps no entry of it.
func TestProcessScratchNormalReleaseLeavesNothing(t *testing.T) {
	t.Parallel()
	bed := newScratchBed(t)
	output, err := bed.helper("release").CombinedOutput()
	if err != nil {
		t.Fatalf("helper: %v\n%s", err, output)
	}
	root := helperValue(t, string(output), "root")
	if filepath.Dir(root) != filepath.Join(mustEval(t, bed.temp), "metasystem") {
		t.Fatalf("root %s is not under the process's TMPDIR/metasystem", root)
	}
	if _, err := os.Lstat(root); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("the released root is still there: %v", err)
	}
	entries, err := os.ReadDir(bed.registry.Dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if !strings.HasPrefix(entry.Name(), ".") {
			t.Fatalf("the normal release left %s in the registry", entry.Name())
		}
	}
	left, err := os.ReadDir(filepath.Dir(root))
	if err != nil {
		t.Fatal(err)
	}
	if len(left) != 0 {
		t.Fatalf("the scratch parent keeps %v", left)
	}
}

// A nested engine child gets the parent's root as TMPDIR and the writer
// lock through ExtraFiles; its own root lies under the parent's.
func TestProcessScratchOfANestedChildLiesUnderItsParents(t *testing.T) {
	t.Parallel()
	bed := newScratchBed(t)
	output, err := bed.helper("nested").CombinedOutput()
	if err != nil {
		t.Fatalf("helper: %v\n%s", err, output)
	}
	parent := helperValue(t, string(output), "root")
	child := helperValue(t, string(output), "child-root")
	if helperValue(t, string(output), "tmpdir") != parent {
		t.Fatalf("the child's TMPDIR is not its parent's root:\n%s", output)
	}
	if filepath.Dir(child) != filepath.Join(parent, "metasystem") {
		t.Fatalf("the nested root %s does not lie under the parent's %s", child, parent)
	}
	for _, root := range []string{parent, child} {
		if _, err := os.Lstat(root); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("%s outlived its process's normal end: %v", root, err)
		}
	}
}

// An os.Exit that skips the release leaves the record; once the process
// is gone the sweeper releases it by the process proof.
func TestProcessScratchAfterAnExitWithoutReleaseIsReleasedByTheProof(t *testing.T) {
	t.Parallel()
	bed := newScratchBed(t)
	output, err := bed.helper("exit").CombinedOutput()
	if err != nil {
		t.Fatalf("helper: %v\n%s", err, output)
	}
	root := helperValue(t, string(output), "root")
	if record := bed.onlyRecord(t); record.State != StateAccepted {
		t.Fatalf("an exit without release left the record %s", record.State)
	}
	bed.sweep(t, identity.KernelProber{})
	if _, err := os.Lstat(root); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("the sweep left the exited process's root: %v", err)
	}
}

func mustEval(t *testing.T, path string) string {
	t.Helper()
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		t.Fatal(err)
	}
	return resolved
}

// fixedProber answers one liveness for every pid.
type fixedProber struct{ state identity.Liveness }

func (p fixedProber) Probe(int64) (identity.Exact, identity.Liveness, error) {
	if p.state == identity.Unknown {
		return identity.Exact{}, identity.Unknown, errors.New("unreadable")
	}
	return identity.Exact{}, p.state, nil
}

// ownScratch makes a process scratch of this test process in a bed of its
// own; its writer lock is closed, as a dead owner's is.
func ownScratch(t *testing.T) (scratchBed, *processScratch) {
	t.Helper()
	bed := newScratchBed(t)
	created, err := newProcessScratch(bed.temp, bed.registry, rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(created.record.Path, "payload"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	return bed, created
}

// The owner's release waits for its in-process users: a directory handed
// out and not yet done keeps the root, and the release after done removes
// it.
func TestProcessScratchReleaseLeavesARootWithAnInProcessUser(t *testing.T) {
	t.Parallel()
	_, created := ownScratch(t)
	dir, done, err := created.mkdir("busy-")
	if err != nil {
		t.Fatal(err)
	}
	if released, err := created.releaseIfIdle(context.Background()); released || err != nil {
		t.Fatalf("a release with a user in flight = %v, %v; want kept", released, err)
	}
	if _, err := os.Lstat(dir); err != nil {
		t.Fatalf("the user's directory went: %v", err)
	}
	done()
	done()
	if released, err := created.releaseIfIdle(context.Background()); !released || err != nil {
		t.Fatalf("the release after done = %v, %v", released, err)
	}
	if _, err := os.Lstat(created.record.Path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("the root outlived its release: %v", err)
	}
}

// Fail-closed rule 1: an unreadable owner reference, an unknown liveness, a
// foreign marker and a missing writer lock each hold the root; nothing is
// removed.
func TestProcessProofHoldsOnAnyUnreadableInput(t *testing.T) {
	t.Parallel()
	cases := map[string]func(t *testing.T, bed scratchBed, record Record) identity.Prober{
		"owner reference": func(t *testing.T, bed scratchBed, record Record) identity.Prober {
			rewriteRecord(t, bed.registry, record.ID, func(r *Record) { r.Owner.Ref = "pid:1" })
			return fixedProber{state: identity.Dead}
		},
		"liveness": func(*testing.T, scratchBed, Record) identity.Prober { return fixedProber{state: identity.Unknown} },
		"marker": func(t *testing.T, _ scratchBed, record Record) identity.Prober {
			path := filepath.Join(record.Path, MarkerName)
			if err := os.Chmod(path, 0o600); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte("not json"), 0o600); err != nil {
				t.Fatal(err)
			}
			return fixedProber{state: identity.Dead}
		},
		"writer lock": func(t *testing.T, _ scratchBed, record Record) identity.Prober {
			if err := os.Remove(filepath.Join(record.Path, WriterLockName)); err != nil {
				t.Fatal(err)
			}
			return fixedProber{state: identity.Dead}
		},
	}
	for name, spoil := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			bed, created := ownScratch(t)
			_ = created.writer.Close()
			prober := spoil(t, bed, created.record)
			bed.sweep(t, prober)
			if _, err := os.Lstat(filepath.Join(created.record.Path, "payload")); err != nil {
				t.Fatalf("an unreadable %s removed the root's content: %v", name, err)
			}
		})
	}
}

// Fail-closed rule 2: the root is known by its device and inode, never by
// its path string: a directory put in its place with a copy of its marker,
// or a symlink to it, is not the store and is kept.
func TestProcessProofKnowsTheRootByDeviceAndInode(t *testing.T) {
	t.Parallel()
	for _, swap := range []string{"copy", "symlink"} {
		t.Run(swap, func(t *testing.T) {
			t.Parallel()
			bed, created := ownScratch(t)
			_ = created.writer.Close()
			root := created.record.Path
			moved := root + ".moved"
			if err := os.Rename(root, moved); err != nil {
				t.Fatal(err)
			}
			if swap == "symlink" {
				if err := os.Symlink(moved, root); err != nil {
					t.Fatal(err)
				}
			} else if err := CopyTree(context.Background(), moved, root); err != nil {
				t.Fatal(err)
			}
			bed.sweep(t, fixedProber{state: identity.Dead})
			if _, err := os.Lstat(filepath.Join(moved, "payload")); err != nil {
				t.Fatalf("the real root lost its content: %v", err)
			}
			if _, err := os.Stat(filepath.Join(root, "payload")); err != nil {
				t.Fatalf("a %s at the recorded path was removed by its string: %v", swap, err)
			}
		})
	}
}

// Fail-closed rule 3: the sweeper removes only the root's payload; the
// record stays as history and nothing beside the root is touched.
func TestProcessProofKeepsTheRecordAndEverythingBesideTheRoot(t *testing.T) {
	t.Parallel()
	bed, created := ownScratch(t)
	_ = created.writer.Close()
	beside := filepath.Join(filepath.Dir(created.record.Path), "beside")
	if err := os.WriteFile(beside, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	bed.sweep(t, fixedProber{state: identity.Dead})
	if _, err := os.Lstat(created.record.Path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("the dead owner's root stayed: %v", err)
	}
	if record := bed.onlyRecord(t); record.State != StateReleased || record.ReleasedBy != "sweeper" {
		t.Fatalf("record = %+v; want released by the sweeper and kept", record)
	}
	if _, err := os.Lstat(beside); err != nil {
		t.Fatalf("an entry beside the root went: %v", err)
	}
}

// Fail-closed rule 4: a removal cut short stays releasing, and every retry
// re-checks use: a writer lock held again keeps it, an owner alive keeps
// it, and only a free lock and a dead owner finish it.
func TestProcessProofRetryRechecksUse(t *testing.T) {
	t.Parallel()
	bed, created := ownScratch(t)
	_ = created.writer.Close()
	rewriteRecord(t, bed.registry, created.record.ID, func(r *Record) { r.State = StateReleasing })
	holder, err := os.OpenFile(filepath.Join(created.record.Path, WriterLockName), os.O_RDWR, 0)
	if err != nil {
		t.Fatal(err)
	}
	if err := unix.Flock(int(holder.Fd()), unix.LOCK_EX); err != nil {
		t.Fatal(err)
	}
	bed.sweep(t, fixedProber{state: identity.Dead})
	if _, err := os.Lstat(filepath.Join(created.record.Path, "payload")); err != nil {
		t.Fatalf("a releasing root whose writer lock is held again was removed: %v", err)
	}
	_ = holder.Close()
	bed.sweep(t, identity.KernelProber{})
	if _, err := os.Lstat(filepath.Join(created.record.Path, "payload")); err != nil {
		t.Fatalf("a releasing root whose owner runs was removed: %v", err)
	}
	bed.sweep(t, fixedProber{state: identity.Dead})
	if _, err := os.Lstat(created.record.Path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("the retry did not finish the release: %v", err)
	}
}

func rewriteRecord(t *testing.T, registry Registry, id string, change func(*Record)) {
	t.Helper()
	record, err := registry.Load(id)
	if err != nil {
		t.Fatal(err)
	}
	change(&record)
	if err := registry.write(record); err != nil {
		t.Fatal(err)
	}
}
