package durable_test

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/durable"
)

// Reuse the early-sync helper so the shared Main still prepares every test
// process. The parent owns the file; TestMain allocates nothing before Main.
func memoryVolumeFullHelper(t *testing.T) bool {
	t.Helper()
	if os.Getenv(earlySyncFileEnv) != "" {
		if durable.Current() != "full" {
			t.Fatal("memory-volume acceptance requires a full-level helper")
		}
		return true
	}
	fixture := filepath.Join(t.TempDir(), "early-sync")
	if err := os.WriteFile(fixture, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	command := exec.Command(os.Args[0], "-test.run=^"+t.Name()+"$", "-test.count=1", "-test.v")
	command.Env = append(os.Environ(), earlySyncFileEnv+"="+fixture)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("full-level helper failed: %v\n%s", err, output)
	}
	if strings.Contains(string(output), "--- SKIP: "+t.Name()) {
		t.Skip("full-level helper: real memory volume or attestation absent; " + string(output))
	}
	if !strings.Contains(string(output), "--- PASS: "+t.Name()) {
		t.Fatalf("full-level helper did not run acceptance: %s", output)
	}
	t.Log(string(output))
	return false
}

type volumeInfo struct {
	mode   os.FileMode
	uid    uint32
	noStat bool
}

func (i volumeInfo) Name() string       { return "volume-entry" }
func (i volumeInfo) Size() int64        { return 0 }
func (i volumeInfo) Mode() os.FileMode  { return i.mode }
func (i volumeInfo) ModTime() time.Time { return time.Time{} }
func (i volumeInfo) IsDir() bool        { return i.mode.IsDir() }
func (i volumeInfo) Sys() any {
	if i.noStat {
		return nil
	}
	return &syscall.Stat_t{Uid: i.uid}
}

func mountName(fs *syscall.Statfs_t, path string) {
	*fs = syscall.Statfs_t{}
	for i, c := range []byte(path) {
		fs.Mntonname[i] = int8(c)
	}
}

func TestDurabilityMemoryVolumeConditionsAtFull(t *testing.T) {
	t.Parallel()
	if !memoryVolumeFullHelper(t) {
		return
	}
	file, err := os.CreateTemp(t.TempDir(), "sync")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	root := durable.MemoryVolumeRoot()
	uid := uint32(os.Geteuid())
	for _, condition := range []string{
		"attested", "other mount", "mount prefix", "mount suffix", "mount alias",
		"fstatfs error", "root lstat error", "root symlink", "root regular", "root owner", "root metadata",
		"marker absent", "marker lstat error", "marker symlink", "marker directory", "marker fifo", "marker owner", "marker metadata",
	} {
		t.Run(condition, func(t *testing.T) {
			var calls []string
			fstatfs := func(fd int, fs *syscall.Statfs_t) error {
				calls = append(calls, "fstatfs")
				if fd != int(file.Fd()) {
					t.Fatal("classified a different descriptor")
				}
				path := root
				switch condition {
				case "fstatfs error":
					return syscall.EIO
				case "other mount":
					path = "/"
				case "mount prefix":
					path += "/nested"
				case "mount suffix":
					path += "0"
				case "mount alias":
					path = strings.TrimPrefix(root, "/private")
				}
				mountName(fs, path)
				return nil
			}
			lstat := func(path string) (os.FileInfo, error) {
				entry := volumeInfo{uid: uid}
				switch path {
				case root:
					calls = append(calls, "lstat root")
					entry.mode = os.ModeDir | 0o700
					switch condition {
					case "root lstat error":
						return nil, syscall.EACCES
					case "root symlink":
						entry.mode = os.ModeSymlink | 0o700
					case "root regular":
						entry.mode = 0o600
					case "root owner":
						entry.uid++
					case "root metadata":
						entry.noStat = true
					}
				case root + "/.mst-ram-backed":
					calls = append(calls, "lstat marker")
					switch condition {
					case "marker absent":
						return nil, os.ErrNotExist
					case "marker lstat error":
						return nil, syscall.EACCES
					case "marker symlink":
						entry.mode = os.ModeSymlink
					case "marker directory":
						entry.mode = os.ModeDir
					case "marker fifo":
						entry.mode = os.ModeNamedPipe
					case "marker owner":
						entry.uid++
					case "marker metadata":
						entry.noStat = true
					}
				default:
					t.Fatalf("unexpected lstat path: %s", path)
				}
				return entry, nil
			}
			selected := "full"
			if condition == "attested" {
				selected = "kernel"
			}
			sentinel := errors.New("sync error")
			operation := func(name string) func(*os.File) error {
				return func(got *os.File) error {
					if got != file {
						t.Fatal("synced a different file")
					}
					calls = append(calls, name)
					return sentinel
				}
			}
			err := durable.SyncMemoryVolumeForTest(file, fstatfs, lstat, operation("full"), operation("kernel"))
			want := []string{"fstatfs"}
			if condition == "attested" || strings.HasPrefix(condition, "root ") || strings.HasPrefix(condition, "marker ") {
				want = append(want, "lstat root")
			}
			if condition == "attested" || strings.HasPrefix(condition, "marker ") {
				want = append(want, "lstat marker")
			}
			want = append(want, selected)
			if !errors.Is(err, sentinel) || !reflect.DeepEqual(calls, want) || durable.Current() != "full" {
				t.Fatalf("calls=%v want=%v error=%v process=%s", calls, want, err, durable.Current())
			}
		})
	}
}

func TestDurabilityMemoryVolumeRechecksAtFull(t *testing.T) {
	t.Parallel()
	if !memoryVolumeFullHelper(t) {
		return
	}
	file, err := os.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	for _, change := range []string{"marker disappears", "root owner changes", "filesystem replaced"} {
		t.Run(change, func(t *testing.T) {
			var calls []string
			second := false
			fstatfs := func(_ int, fs *syscall.Statfs_t) error {
				calls = append(calls, "fstatfs")
				path := durable.MemoryVolumeRoot()
				if second && change == "filesystem replaced" {
					path = "/"
				}
				mountName(fs, path)
				return nil
			}
			lstat := func(path string) (os.FileInfo, error) {
				info := volumeInfo{uid: uint32(os.Geteuid())}
				if path == durable.MemoryVolumeRoot() {
					calls = append(calls, "root")
					info.mode = os.ModeDir
					if second && change == "root owner changes" {
						info.uid++
					}
				} else {
					calls = append(calls, "marker")
					if second && change == "marker disappears" {
						return nil, os.ErrNotExist
					}
				}
				return info, nil
			}
			full := func(*os.File) error { calls = append(calls, "full"); return nil }
			kernel := func(*os.File) error { calls = append(calls, "kernel"); return nil }
			for i := 0; i < 2; i++ {
				second = i == 1
				if err := durable.SyncMemoryVolumeForTest(file, fstatfs, lstat, full, kernel); err != nil {
					t.Fatal(err)
				}
			}
			want := []string{"fstatfs", "root", "marker", "kernel", "fstatfs"}
			if change != "filesystem replaced" {
				want = append(want, "root")
			}
			if change == "marker disappears" {
				want = append(want, "marker")
			}
			want = append(want, "full")
			if !reflect.DeepEqual(calls, want) {
				t.Fatalf("calls=%v want=%v", calls, want)
			}
		})
	}
}

func TestDurabilityMemoryVolumeSymlinksAtFull(t *testing.T) {
	t.Parallel()
	if !memoryVolumeFullHelper(t) {
		return
	}
	base := t.TempDir()
	volume := filepath.Join(base, "volume")
	if err := os.Mkdir(volume, 0o700); err != nil {
		t.Fatal(err)
	}
	on := filepath.Join(volume, "file")
	off := filepath.Join(base, "file")
	for _, path := range []string{on, off} {
		if err := os.WriteFile(path, nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	onto := filepath.Join(base, "onto-volume")
	away := filepath.Join(volume, "off-volume")
	if err := os.Symlink(on, onto); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(off, away); err != nil {
		t.Fatal(err)
	}
	onInfo, err := os.Stat(on)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ path, want string }{{onto, "kernel"}, {away, "full"}} {
		file, err := os.Open(tc.path)
		if err != nil {
			t.Fatal(err)
		}
		defer file.Close()
		fstatfs := func(fd int, fs *syscall.Statfs_t) error {
			var stat syscall.Stat_t
			if err := syscall.Fstat(fd, &stat); err != nil {
				return err
			}
			path := "/"
			if stat.Ino == onInfo.Sys().(*syscall.Stat_t).Ino {
				path = durable.MemoryVolumeRoot()
			}
			mountName(fs, path)
			return nil
		}
		lstat := func(path string) (os.FileInfo, error) {
			info := volumeInfo{uid: uint32(os.Geteuid())}
			if path == durable.MemoryVolumeRoot() {
				info.mode = os.ModeDir
			}
			return info, nil
		}
		selected := ""
		err = durable.SyncMemoryVolumeForTest(file, fstatfs, lstat,
			func(*os.File) error { selected = "full"; return nil },
			func(*os.File) error { selected = "kernel"; return nil })
		if err != nil || selected != tc.want {
			t.Fatalf("symlink selection=%s want=%s error=%v", selected, tc.want, err)
		}
	}
}

func TestDurabilityRealMemoryVolumeAtFull(t *testing.T) {
	t.Parallel()
	if !memoryVolumeFullHelper(t) {
		return
	}
	root := durable.MemoryVolumeRoot()
	if _, err := os.Lstat(root); errors.Is(err, os.ErrNotExist) {
		t.Skip("real test memory volume is absent")
	} else if err != nil {
		t.Fatal(err)
	}
	marker := root + "/.mst-ram-backed"
	if _, err := os.Lstat(marker); errors.Is(err, os.ErrNotExist) {
		t.Skip("real test memory volume has no RAM-backing attestation")
	} else if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{root, marker} {
		file, err := os.Open(path)
		if err != nil {
			t.Fatal(err)
		}
		defer file.Close()
		selected := ""
		err = durable.SyncOperationsForTest(file,
			func(*os.File) error { selected = "full"; return nil },
			func(f *os.File) error { selected = "kernel"; return durable.TestKernelSync(f) })
		if err != nil || selected != "kernel" || durable.Current() != "full" {
			t.Fatalf("real-volume selection=%s process=%s error=%v", selected, durable.Current(), err)
		}
		if err := durable.Sync(file); err != nil {
			t.Fatal(err)
		}
	}
	if err := durable.SyncDir(root); err != nil {
		t.Fatal(err)
	}
}

func TestDurabilityMemoryVolumeFileErrorsAtFull(t *testing.T) {
	t.Parallel()
	if !memoryVolumeFullHelper(t) {
		return
	}
	if err := durable.Sync(nil); !errors.Is(err, os.ErrInvalid) {
		t.Fatalf("nil file error: %v", err)
	}
	file, err := os.CreateTemp(t.TempDir(), "closed")
	if err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	got, want := durable.Sync(file), file.Sync()
	var path *os.PathError
	if !errors.As(got, &path) || path.Op != "sync" || path.Path != file.Name() || !errors.Is(got, os.ErrClosed) || got.Error() != want.Error() {
		t.Fatalf("closed file mismatch: %v / %v", got, want)
	}
}
