package diskstore

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"golang.org/x/sys/unix"
)

// awaitGone waits on the kernel's process table, never on a clock, for a
// process this test started (through its helpers) to end.
func awaitGone(t *testing.T, pid int) {
	t.Helper()
	for unix.Kill(pid, 0) == nil {
		if t.Context().Err() != nil {
			t.Fatalf("process %d did not end", pid)
		}
		runtime.Gosched()
	}
}

// Round D1 F-1: a nested engine child C that starts a grandchild G and
// ends while G runs, with G launched through the seam with (extra) or
// without a launcher descriptor of its own. Roots are flat: C's root lies
// beside its parent P's, never inside it, so P's release touches only its
// own; C's release, and the sweeper after it, keep C's root while G lives;
// once G has ended the sweeper releases it.
func TestProcessScratchKeepsANestedRootWhileItsGrandchildLives(t *testing.T) {
	t.Parallel()
	for _, variant := range []string{"prep", "extra"} {
		for _, exit := range []bool{false, true} {
			name := variant
			if exit {
				name += "-parent-exits-unreleased"
			}
			t.Run(name, func(t *testing.T) {
				t.Parallel()
				bed := newScratchBed(t)
				helper := bed.helper("grand-" + variant)
				if exit {
					helper.Env = append(helper.Env, "DISKSTORE_SCRATCH_EXIT=1")
				}
				childIn, stdin, err := os.Pipe()
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { _ = stdin.Close() })
				helper.Stdin = childIn
				output, err := helper.CombinedOutput()
				_ = childIn.Close()
				if err != nil {
					t.Fatalf("helper: %v\n%s", err, output)
				}
				parent := helperValue(t, string(output), "root")
				child := helperValue(t, string(output), "child-root")
				dir := helperValue(t, string(output), "grandchild-dir")
				grandchild, err := strconv.Atoi(helperValue(t, string(output), "grandchild"))
				if err != nil {
					t.Fatal(err)
				}
				if filepath.Dir(child) != filepath.Dir(parent) {
					t.Fatalf("the nested root %s is not flat beside its parent's %s", child, parent)
				}
				if unix.Kill(grandchild, 0) != nil {
					t.Fatal("the grandchild ended before the witness looked")
				}
				if _, err := os.Lstat(dir); err != nil {
					t.Fatalf("the owners' releases removed a root a live grandchild uses:\n%s", output)
				}
				bed.sweep(t, identity.KernelProber{})
				if _, err := os.Lstat(dir); err != nil {
					t.Fatalf("the sweeper removed a root a live grandchild uses: %v", err)
				}
				_ = stdin.Close()
				awaitGone(t, grandchild)
				bed.sweep(t, identity.KernelProber{})
				if _, err := os.Lstat(child); !errors.Is(err, os.ErrNotExist) {
					t.Fatalf("after the grandchild ended the child's root stayed: %v", err)
				}
			})
		}
	}
}

// Round D1 F-1(c): the owner's release keeps its root while a child it
// launched through the seam is alive, even one that closed the inherited
// writer lock; once the child has been waited for, the release goes ahead.
func TestProcessScratchReleaseKeepsTheRootWhileAPreparedChildLives(t *testing.T) {
	t.Parallel()
	runOwnerScenario(t, "prepared")
}

// Round D1 F-1(b): the seam appends the writer lock after the launcher's
// own descriptors and names its number in the child's environment.
func TestPrepareChildAppendsTheLockAndNamesItsDescriptor(t *testing.T) {
	t.Parallel()
	bed := newScratchBed(t)
	created, err := newProcessScratch(bed.temp, bed.registry, rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(created.closeWriter)
	read, write, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = read.Close(); _ = write.Close() })
	child := exec.Command("true")
	child.ExtraFiles = []*os.File{read}
	if err := created.prepare(child); err != nil {
		t.Fatal(err)
	}
	if len(child.ExtraFiles) < 2 || child.ExtraFiles[0] != read || child.ExtraFiles[1] != created.writer {
		t.Fatalf("extra files = %v; want the launcher's first, then the writer lock", child.ExtraFiles)
	}
	if !containsEntry(child.Env, scratchLockFDEnv+"=4") || !containsEntry(child.Env, scratchRootsEnv+"="+filepath.Dir(created.record.Path)) {
		t.Fatalf("child environment lacks the lock descriptor or the roots directory: %v", child.Env)
	}
}

func containsEntry(env []string, want string) bool {
	for _, entry := range env {
		if entry == want {
			return true
		}
	}
	return false
}

// Round D1 F-3: the plan takes the census for a process store too, so a
// preview says keep where the apply keeps.
func TestProcessStorePreviewTakesTheCensus(t *testing.T) {
	t.Parallel()
	bed, record := deadScratch(t)
	options := passOptions(bed.registry, realDir(t),
		RegisteredStores{Registry: bed.registry, Proofs: map[OwnerKind]OwnerProof{OwnerProcess: ProcessProof{Prober: fixedProber{state: identity.Dead}}}})
	options.CensusReader = fakeCensus(map[int64]identity.ProcessUse{4242: {Cwd: record.Path}}, nil)
	options.Mode = ModePreview
	report, err := RunPass(context.Background(), options)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Planned) != 0 || len(report.Kept) != 1 {
		t.Fatalf("preview planned %+v and kept %+v; want the census holder kept", report.Planned, report.Kept)
	}
}

// Round D1 F-5: a symlink entry in a root is unlinked, never followed,
// and never blocks the release.
func TestProcessScratchReleaseUnlinksSymlinks(t *testing.T) {
	t.Parallel()
	outside := filepath.Join(realDir(t), "outside")
	precious := filepath.Join(outside, "sub", "precious")
	if err := os.MkdirAll(filepath.Dir(precious), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(precious, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	runOwnerScenario(t, "symlinks", "DISKSTORE_SCRATCH_OUTSIDE="+outside)
	if _, err := os.Lstat(precious); err != nil {
		t.Fatalf("the release followed a symlink out of the root: %v", err)
	}
}

// Round D1 F-6: a temporary store whose creation was cut short after its
// record, its root still empty, is discarded and made again.
func TestTempStoreAfterAnInterruptedCreationIsMadeAgain(t *testing.T) {
	t.Parallel()
	bed := newTempStoreBed(t)
	path := filepath.Join(mustEval(t, bed.temp), "metasystem", "metasystem-read-context.r1-a1")
	if err := os.MkdirAll(path, 0o700); err != nil {
		t.Fatal(err)
	}
	info, err := os.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}
	device, inode, _ := fileID(info)
	if _, err := bed.registry.Register(Registration{Path: path, Class: "read-context", Owner: readOwner, Lifetime: LifetimeOwner, CapKind: CapNone,
		RootDevice: device, RootInode: inode}, testNow, rand.Reader); err != nil {
		t.Fatal(err)
	}
	made, err := bed.createTemp(t, readOwner)
	if err != nil || made != path {
		t.Fatalf("the interrupted creation was not discarded and made again: %q, %v", made, err)
	}
}

// Round D1 F-10: a relative TMPDIR is made absolute; a registry that cannot
// be written leaves the command working in an unregistered private
// directory, with one warning, that `disk clean --strays` reports.
func TestProcessScratchKeepsTheCommandWorking(t *testing.T) {
	t.Parallel()
	dir := realDir(t)
	relative, err := filepath.Rel(mustCwd(t), filepath.Join(dir, "tmp"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, "tmp"), 0o700); err != nil {
		t.Fatal(err)
	}
	created, err := newProcessScratch(absoluteTemp(relative), MachineRegistry(filepath.Join(dir, "home")), rand.Reader)
	if err != nil || !filepath.IsAbs(created.record.Path) {
		t.Fatalf("a relative TMPDIR: %v, %+v", err, created)
	}
	created.closeWriter()

	blocked := filepath.Join(dir, "blocked")
	if err := os.WriteFile(blocked, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	var warnings strings.Builder
	fallback := openScratch(filepath.Join(dir, "tmp"), Registry{Dir: filepath.Join(blocked, "stores")}, &warnings)
	if fallback == nil || !fallback.fallback || !strings.HasPrefix(filepath.Base(fallback.record.Path), "metasystem-") {
		t.Fatalf("an unwritable registry gave %+v", fallback)
	}
	if !strings.Contains(warnings.String(), "disk clean --strays") || strings.Count(warnings.String(), "\n") != 1 {
		t.Fatalf("warning = %q", warnings.String())
	}
	scratch, done, err := fallback.mkdir("work-")
	if err != nil || filepath.Dir(scratch) != fallback.record.Path {
		t.Fatalf("the fallback does not hand out directories: %q, %v", scratch, err)
	}
	done()
}

func mustCwd(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	return wd
}

// Round D1 (last case): a grandchild started without the seam, in a
// process group of its own, holding no lock, no cwd and no open file in the
// child's root. The dead owner's session is still alive in it (an orphan
// keeps its session after it is reparented to pid 1), so the sweeper keeps
// the root; once the grandchild has ended the root is released.
func TestProcessProofKeepsTheRootWhileTheOwnersSessionLives(t *testing.T) {
	t.Parallel()
	bed := newScratchBed(t)
	helper := bed.helper("grand-bare")
	childIn, stdin, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = stdin.Close() })
	helper.Stdin = childIn
	output, err := helper.CombinedOutput()
	_ = childIn.Close()
	if err != nil {
		t.Fatalf("helper: %v\n%s", err, output)
	}
	child := helperValue(t, string(output), "child-root")
	dir := helperValue(t, string(output), "grandchild-dir")
	grandchild, err := strconv.Atoi(helperValue(t, string(output), "grandchild"))
	if err != nil {
		t.Fatal(err)
	}
	bed.sweep(t, identity.KernelProber{})
	if unix.Kill(grandchild, 0) != nil {
		t.Fatal("the grandchild ended before the witness looked")
	}
	if _, err := os.Lstat(dir); err != nil {
		t.Fatalf("the sweeper removed a root while its owner's session lives: %v", err)
	}
	_ = stdin.Close()
	awaitGone(t, grandchild)
	report := bed.sweep(t, identity.KernelProber{})
	if _, err := os.Lstat(child); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("after the session ended the root stayed: %v\nkept %+v\npending %+v", err, report.Kept, report.Pending)
	}
}

// The owner's group and session are recorded at creation; a live process
// in either keeps a dead owner's root. This test process is in both of its
// own scratch's, so that root is kept.
func TestProcessProofKeepsTheRootWhileTheOwnersGroupLives(t *testing.T) {
	t.Parallel()
	bed := newScratchBed(t)
	created, err := newProcessScratch(bed.temp, bed.registry, rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	created.closeWriter()
	if created.record.OwnerGroup != int64(unix.Getpgrp()) || created.record.OwnerSession == 0 {
		t.Fatalf("record group %d session %d", created.record.OwnerGroup, created.record.OwnerSession)
	}
	bed.sweepWith(t, fixedProber{state: identity.Dead}, fakeCensus(nil, nil))
	if _, err := os.Lstat(created.record.Path); err != nil {
		t.Fatalf("a dead owner's root was removed while its group lives: %v", err)
	}
	for name, change := range map[string]func(*Record){
		"no group recorded": func(r *Record) { r.OwnerGroup, r.OwnerSession = 0, 0 },
	} {
		rewriteRecord(t, bed.registry, created.record.ID, change)
		bed.sweepWith(t, fixedProber{state: identity.Dead}, fakeCensus(nil, nil))
		if _, err := os.Lstat(created.record.Path); err != nil {
			t.Fatalf("%s: the root was removed: %v", name, err)
		}
	}
}

// ownerScenarios are owner releases a test runs in a child of this test
// binary, where no other test forks: a fork in flight holds a copy of the
// owner's writer lock until its exec, which reads as a held lock and leaves
// the root to the sweeper (safe, but not what these witnesses assert).
var ownerScenarios = map[string]func(created *processScratch) error{
	"users": func(created *processScratch) error {
		dir, done, err := created.mkdir("busy-")
		if err != nil {
			return err
		}
		if released, err := created.releaseIfIdle(context.Background()); released || err != nil {
			return fmt.Errorf("a release with a user in flight = %v, %v; want kept", released, err)
		}
		if _, err := os.Lstat(dir); err != nil {
			return fmt.Errorf("the user's directory went: %v", err)
		}
		done()
		done()
		return releasedAndGone(created)
	},
	"prepared": func(created *processScratch) error {
		child := exec.Command("sh", "-c", `exec 3>&-; exec cat`)
		if err := created.prepare(child); err != nil {
			return err
		}
		stdin, err := child.StdinPipe()
		if err != nil {
			return err
		}
		if err := child.Start(); err != nil {
			return err
		}
		if released, err := created.releaseIfIdle(context.Background()); released || err != nil {
			return fmt.Errorf("a release with a live prepared child = %v, %v; want kept", released, err)
		}
		if _, err := os.Lstat(created.record.Path); err != nil {
			return fmt.Errorf("the root went while its child lives: %v", err)
		}
		_ = stdin.Close()
		_ = child.Wait()
		return releasedAndGone(created)
	},
	"symlinks": func(created *processScratch) error {
		outside := os.Getenv("DISKSTORE_SCRATCH_OUTSIDE")
		for name, target := range map[string]string{"link-dir": outside, "link-file": filepath.Join(outside, "sub", "precious")} {
			if err := os.Symlink(target, filepath.Join(created.record.Path, name)); err != nil {
				return err
			}
		}
		return releasedAndGone(created)
	},
}

func releasedAndGone(created *processScratch) error {
	if released, err := created.releaseIfIdle(context.Background()); !released || err != nil {
		return fmt.Errorf("the release = %v, %v; want released", released, err)
	}
	if _, err := os.Lstat(created.record.Path); !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("the root outlived its release: %v", err)
	}
	return nil
}

// runOwnerScenario runs one owner scenario in a child of this test binary.
func runOwnerScenario(t *testing.T, name string, env ...string) {
	t.Helper()
	bed := newScratchBed(t)
	helper := bed.helper("scenario-" + name)
	helper.Env = append(helper.Env, env...)
	if output, err := helper.CombinedOutput(); err != nil {
		t.Fatalf("owner scenario %s: %v\n%s", name, err, output)
	}
}

// ownerScenario runs a scenario in the helper process.
func ownerScenario(name string) error {
	scenario := ownerScenarios[name]
	if scenario == nil {
		return fmt.Errorf("no owner scenario %q", name)
	}
	registry, err := machineRegistry()
	if err != nil {
		return err
	}
	created, err := newProcessScratch(os.TempDir(), registry, rand.Reader)
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(created.record.Path, "payload"), []byte("x"), 0o600); err != nil {
		return err
	}
	return scenario(created)
}
