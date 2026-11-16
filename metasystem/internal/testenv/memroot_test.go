package testenv

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/diskstore"
	"golang.org/x/sys/unix"
)

func fakeMemoryRoot(t *testing.T) memoryRootProbe {
	t.Helper()
	root := filepath.Join(t.TempDir(), "mount")
	checkTestenv(t, os.Mkdir(root, 0o700))
	checkTestenv(t, os.WriteFile(filepath.Join(root, ".mst-ready-2"), nil, 0o600))
	probe := memoryRootProbe{
		path:  func() string { return root },
		mount: func(string, os.FileInfo) bool { return true },
		statfs: func(_ string, space *unix.Statfs_t) error {
			space.Bsize, space.Bavail = 4096, (512<<20)/4096
			return nil
		},
		lease: &memoryRootLeaseState{},
	}
	t.Cleanup(func() {
		if probe.lease.file != nil {
			checkTestenv(t, probe.lease.file.Close())
		}
	})
	return probe
}

func TestProvenMemoryRoot(t *testing.T) {
	t.Parallel()
	for _, condition := range []string{"ready", "missing", "file", "old marker", "no marker", "directory marker", "symlink marker", "symlink root", "not mounted", "low space", "statfs error", "exclusive lease", "symlink lease", "directory lease"} {
		t.Run(condition, func(t *testing.T) {
			t.Parallel()
			probe := fakeMemoryRoot(t)
			root := probe.path()
			marker := filepath.Join(root, ".mst-ready-2")
			switch condition {
			case "missing", "file":
				checkTestenv(t, os.RemoveAll(root))
				if condition == "file" {
					checkTestenv(t, os.WriteFile(root, nil, 0o600))
				}
			case "old marker", "no marker", "directory marker", "symlink marker":
				checkTestenv(t, os.Remove(marker))
				if condition == "old marker" {
					checkTestenv(t, os.WriteFile(filepath.Join(root, ".mst-ready"), nil, 0o600))
				} else if condition == "directory marker" {
					checkTestenv(t, os.Mkdir(marker, 0o700))
				} else if condition == "symlink marker" {
					target := filepath.Join(t.TempDir(), "ready")
					checkTestenv(t, os.WriteFile(target, nil, 0o600))
					checkTestenv(t, os.Symlink(target, marker))
				}
			case "symlink root":
				checkTestenv(t, os.Rename(root, root+"-target"))
				checkTestenv(t, os.Symlink(root+"-target", root))
			case "not mounted":
				probe.mount = func(string, os.FileInfo) bool { return false }
			case "low space", "statfs error":
				probe.statfs = func(_ string, space *unix.Statfs_t) error {
					space.Bsize, space.Bavail = 1, (512<<20)-1
					if condition == "statfs error" {
						return errors.New("statfs unavailable")
					}
					return nil
				}
			case "exclusive lease":
				file, err := os.OpenFile(root+".lease", os.O_CREATE|os.O_RDWR, 0o600)
				checkTestenv(t, err)
				t.Cleanup(func() { _ = file.Close() })
				checkTestenv(t, unix.Flock(int(file.Fd()), unix.LOCK_EX|unix.LOCK_NB))
				probe.mount = func(string, os.FileInfo) bool {
					t.Fatal("mount checked before obtaining lease")
					return true
				}
			case "symlink lease":
				checkTestenv(t, os.Symlink(marker, root+".lease"))
			case "directory lease":
				checkTestenv(t, os.Mkdir(root+".lease", 0o700))
			}
			got, ok := probe.provenRoot()
			if ok != (condition == "ready") || (ok && got != root) {
				t.Fatalf("provenMemoryRoot = %q, %v", got, ok)
			}
			if ok {
				lease := probe.lease.file
				if !probe.acquireLease() || probe.lease.file != lease {
					t.Fatal("lease was not retained across calls")
				}
				file, err := os.OpenFile(root+".lease", os.O_RDWR, 0)
				checkTestenv(t, err)
				defer file.Close()
				if err := unix.Flock(int(file.Fd()), unix.LOCK_EX|unix.LOCK_NB); !errors.Is(err, unix.EWOULDBLOCK) {
					t.Fatalf("keeper was not excluded: %v", err)
				}
			}
		})
	}
}

func TestMemoryRootMountRejectsOrdinaryDirectory(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	info, err := os.Stat(root)
	checkTestenv(t, err)
	if memoryRootMount(root, info) {
		t.Fatal("ordinary directory accepted as a mount")
	}
}

func TestCreateRegistryHomeMemoryRoot(t *testing.T) {
	t.Parallel()
	for _, condition := range []string{"ready", "not proven", "mkdir fails"} {
		t.Run(condition, func(t *testing.T) {
			t.Parallel()
			probe := fakeMemoryRoot(t)
			root := probe.path()
			want := root
			if condition == "not proven" {
				checkTestenv(t, os.Remove(filepath.Join(root, ".mst-ready-2")))
			}
			if condition != "ready" {
				want = "/tmp"
			}
			registry, err := probe.createRegistryHome(func(dir, pattern string) (string, error) {
				if dir == root && condition == "mkdir fails" {
					return "", errors.New("memory root full")
				}
				return os.MkdirTemp(dir, pattern)
			})
			checkTestenv(t, err)
			t.Cleanup(func() { checkTestenv(t, registry.cleanup()) })
			if filepath.Dir(registry.path) != want {
				t.Fatalf("home = %q, want parent %q", registry.path, want)
			}
		})
	}
}

func TestInheritedMemoryRootTakesLeaseBestEffort(t *testing.T) {
	t.Parallel()
	for _, blocked := range []bool{false, true} {
		t.Run(map[bool]string{false: "available", true: "blocked"}[blocked], func(t *testing.T) {
			t.Parallel()
			probe := fakeMemoryRoot(t)
			root := probe.path()
			owner, err := createRegistryHomeUnder(os.MkdirTemp, root)
			checkTestenv(t, err)
			t.Cleanup(func() { checkTestenv(t, owner.cleanup()) })
			if blocked {
				file, err := os.OpenFile(root+".lease", os.O_CREATE|os.O_RDWR, 0o600)
				checkTestenv(t, err)
				t.Cleanup(func() { _ = file.Close() })
				checkTestenv(t, unix.Flock(int(file.Fd()), unix.LOCK_EX|unix.LOCK_NB))
			}
			inherited := inheritedRegistryHomeAt(owner.path, owner.nonce)
			if inherited == nil {
				t.Fatal("live registry home was not inherited")
			}
			joined, err := probe.registryHomeForProcess(os.MkdirTemp, inherited)
			checkTestenv(t, err)
			t.Cleanup(func() { checkTestenv(t, joined.cleanup()) })
			if joined.path != owner.path || joined.owner || (probe.lease.file != nil) == blocked {
				t.Fatalf("joined = %+v, memory lease = %v", joined, probe.lease.file)
			}
		})
	}
}

// testenv cannot share the diskstore spelling in production: diskstore's
// own package tests use testenv, so that import would create a cycle.
func TestMemoryRootMatchesDiskFloorExclusion(t *testing.T) {
	t.Parallel()
	root := defaultMemoryRoot.path()
	if !diskstore.OnTestMemoryRoot(root) || !diskstore.OnTestMemoryRoot(filepath.Join(root, "home")) ||
		!diskstore.OnTestMemoryRoot("/private"+root) || diskstore.OnTestMemoryRoot(root+"x") ||
		diskstore.OnTestMemoryRoot(filepath.Dir(root)) {
		t.Fatalf("test memory root %q drifted from the disk floor exclusion", root)
	}
}
