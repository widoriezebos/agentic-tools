package diskstore

import (
	"context"
	"crypto/rand"
	"errors"
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
	_, created := ownScratch(t)
	child := exec.Command("sh", "-c", `exec 3>&-; exec cat`)
	if err := created.prepare(child); err != nil {
		t.Fatal(err)
	}
	stdin, err := child.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := child.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = stdin.Close(); _ = child.Wait() })
	if released, err := created.releaseIfIdle(context.Background()); released || err != nil {
		t.Fatalf("a release with a live prepared child = %v, %v; want kept", released, err)
	}
	if _, err := os.Lstat(created.record.Path); err != nil {
		t.Fatalf("the root went while its child lives: %v", err)
	}
	_ = stdin.Close()
	_ = child.Wait()
	if released, err := created.releaseIfIdle(context.Background()); !released || err != nil {
		t.Fatalf("the release after the child ended = %v, %v", released, err)
	}
}

// Round D1 F-1(b): the seam appends the writer lock after the launcher's
// own descriptors and names its number in the child's environment.
func TestPrepareChildAppendsTheLockAndNamesItsDescriptor(t *testing.T) {
	t.Parallel()
	_, created := ownScratch(t)
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
	bed, created := ownScratch(t)
	created.closeWriter()
	options := passOptions(bed.registry, realDir(t),
		RegisteredStores{Registry: bed.registry, Proofs: map[OwnerKind]OwnerProof{OwnerProcess: ProcessProof{Prober: fixedProber{state: identity.Dead}}}})
	options.CensusReader = fakeCensus(map[int64]identity.ProcessUse{4242: {Cwd: created.record.Path}}, nil)
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
	_, created := ownScratch(t)
	outside := filepath.Join(realDir(t), "outside")
	precious := filepath.Join(outside, "sub", "precious")
	if err := os.MkdirAll(filepath.Dir(precious), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(precious, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	for name, target := range map[string]string{"link-dir": outside, "link-file": precious} {
		if err := os.Symlink(target, filepath.Join(created.record.Path, name)); err != nil {
			t.Fatal(err)
		}
	}
	if released, err := created.releaseIfIdle(context.Background()); !released || err != nil {
		t.Fatalf("a root holding symlinks was not released: %v, %v", released, err)
	}
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
