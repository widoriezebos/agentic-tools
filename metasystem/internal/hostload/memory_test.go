package hostload

import (
	"context"
	"errors"
	"os"
	"path"
	"strings"
	"testing"
)

func TestLinuxAvailableMemoryBoundsHostByNestedFiniteCgroup(t *testing.T) {
	t.Parallel()
	read := fixtureMemoryReader(map[string]string{
		procMeminfoPath:    "MemTotal: 20000000 kB\nMemAvailable: 10000000 kB\n",
		procSelfCgroupPath: "0::/user.slice/worker.scope\n",
		path.Join(cgroupV2MountPath, "memory.max"):                                   "max\n",
		path.Join(cgroupV2MountPath, "user.slice", "memory.max"):                     "max\n",
		path.Join(cgroupV2MountPath, "user.slice", "worker.scope", "memory.max"):     "8589934592\n",
		path.Join(cgroupV2MountPath, "user.slice", "worker.scope", "memory.current"): "1073741824\n",
	})
	got, source, available := availableMemoryForOS("linux", read, nil)
	if !available || got != 7*1024*1024*1024 || source != "linux:/proc/meminfo+cgroup-v2" {
		t.Fatalf("available=%t bytes=%d source=%q", available, got, source)
	}
}

func TestLinuxAvailableMemoryUsesLimitingVisibleParent(t *testing.T) {
	t.Parallel()
	read := fixtureMemoryReader(map[string]string{
		procMeminfoPath:    "MemAvailable: 10485760 kB\n",
		procSelfCgroupPath: "0::/parent/worker\n",
		path.Join(cgroupV2MountPath, "memory.max"):                         "max\n",
		path.Join(cgroupV2MountPath, "parent", "memory.max"):               "8589934592\n",
		path.Join(cgroupV2MountPath, "parent", "memory.current"):           "2147483648\n",
		path.Join(cgroupV2MountPath, "parent", "worker", "memory.max"):     "12884901888\n",
		path.Join(cgroupV2MountPath, "parent", "worker", "memory.current"): "1073741824\n",
	})
	got, source, available := availableMemoryForOS("linux", read, nil)
	if !available || got != 6*1024*1024*1024 || source != procMeminfoCgroupSource {
		t.Fatalf("available=%t bytes=%d source=%q", available, got, source)
	}
}

func TestLinuxAvailableMemoryAcceptsUnlimitedRootCgroup(t *testing.T) {
	t.Parallel()
	read := fixtureMemoryReader(map[string]string{
		procMeminfoPath:    "MemAvailable: 7340032 kB\n",
		procSelfCgroupPath: "0::/\n",
		path.Join(cgroupV2MountPath, "memory.max"): "max\n",
	})
	got, source, available := availableMemoryForOS("linux", read, nil)
	if !available || got != 7*1024*1024*1024 || source != "linux:/proc/meminfo" {
		t.Fatalf("available=%t bytes=%d source=%q", available, got, source)
	}
}

func TestLinuxAvailableMemoryAppliesFiniteVisibleRootCgroup(t *testing.T) {
	t.Parallel()
	read := fixtureMemoryReader(map[string]string{
		procMeminfoPath:    "MemAvailable: 1024 kB\n",
		procSelfCgroupPath: "0::/\n",
		path.Join(cgroupV2MountPath, "memory.max"):     "1048576\n",
		path.Join(cgroupV2MountPath, "memory.current"): "262144\n",
	})
	got, source, available := availableMemoryForOS("linux", read, nil)
	if !available || got != 786432 || source != procMeminfoCgroupSource {
		t.Fatalf("available=%t bytes=%d source=%q", available, got, source)
	}
}

func TestLinuxAvailableMemoryAcceptsMissingVisibleRootFiles(t *testing.T) {
	t.Parallel()
	read := fixtureMemoryReader(map[string]string{
		procMeminfoPath:    "MemAvailable: 7340032 kB\n",
		procSelfCgroupPath: "0::/user.slice/user-501.slice/session-4.scope\n",
		path.Join(cgroupV2MountPath, "user.slice", "memory.max"):                                      "max\n",
		path.Join(cgroupV2MountPath, "user.slice", "user-501.slice", "memory.max"):                    "max\n",
		path.Join(cgroupV2MountPath, "user.slice", "user-501.slice", "session-4.scope", "memory.max"): "max\n",
	})
	got, source, available := availableMemoryForOS("linux", read, nil)
	if !available || got != 7*1024*1024*1024 || source != procMeminfoSource {
		t.Fatalf("available=%t bytes=%d source=%q", available, got, source)
	}
}

func TestLinuxAvailableMemoryAppliesFiniteDescendantWhenVisibleRootFilesMissing(t *testing.T) {
	t.Parallel()
	read := fixtureMemoryReader(map[string]string{
		procMeminfoPath:    "MemAvailable: 10485760 kB\n",
		procSelfCgroupPath: "0::/user.slice/user-501.slice/session-4.scope\n",
		path.Join(cgroupV2MountPath, "user.slice", "memory.max"):                                          "max\n",
		path.Join(cgroupV2MountPath, "user.slice", "user-501.slice", "memory.max"):                        "max\n",
		path.Join(cgroupV2MountPath, "user.slice", "user-501.slice", "session-4.scope", "memory.max"):     "8589934592\n",
		path.Join(cgroupV2MountPath, "user.slice", "user-501.slice", "session-4.scope", "memory.current"): "1073741824\n",
	})
	got, source, available := availableMemoryForOS("linux", read, nil)
	if !available || got != 7*1024*1024*1024 || source != procMeminfoCgroupSource {
		t.Fatalf("available=%t bytes=%d source=%q", available, got, source)
	}
}

func TestLinuxAvailableMemoryRejectsMalformedOrOverflowingInputs(t *testing.T) {
	t.Parallel()
	for _, specimen := range []map[string]string{
		{procMeminfoPath: "MemAvailable: unknown kB\n"},
		{procMeminfoPath: "MemAvailable: " + strings.Repeat("9", 30) + " kB\n"},
		{procMeminfoPath: "MemAvailable: 1024 kB\n", procSelfCgroupPath: "malformed\n"},
		{procMeminfoPath: "MemAvailable: 1024 kB\n", procSelfCgroupPath: "0::/parent/../worker\n"},
		{procMeminfoPath: "MemAvailable: 1024 kB\n", procSelfCgroupPath: "0::/\n", path.Join(cgroupV2MountPath, "memory.max"): "broken\n"},
		{procMeminfoPath: "MemAvailable: 1024 kB\n", procSelfCgroupPath: "0::/\n", path.Join(cgroupV2MountPath, "memory.max"): "2048\n"},
		{procMeminfoPath: "MemAvailable: 1024 kB\n", procSelfCgroupPath: "0::/\n", path.Join(cgroupV2MountPath, "memory.max"): "2048\n", path.Join(cgroupV2MountPath, "memory.current"): "broken\n"},
	} {
		if got, source, available := availableMemoryForOS("linux", fixtureMemoryReader(specimen), nil); available || got != 0 || source != "" {
			t.Fatalf("malformed input available=%t bytes=%d source=%q", available, got, source)
		}
	}
}

func TestLinuxAvailableMemoryTreatsUnreadableMembershipOrAncestorAsUnknown(t *testing.T) {
	t.Parallel()
	rootMaximumPath := path.Join(cgroupV2MountPath, "memory.max")
	readableDescendants := map[string]string{
		procMeminfoPath:    "MemAvailable: 1024 kB\n",
		procSelfCgroupPath: "0::/parent/worker\n",
		path.Join(cgroupV2MountPath, "parent", "worker", "memory.max"): "max\n",
		path.Join(cgroupV2MountPath, "parent", "memory.max"):           "max\n",
	}
	for _, specimen := range []struct {
		files    map[string]string
		failures map[string]error
	}{
		{files: map[string]string{procMeminfoPath: "MemAvailable: 1024 kB\n"}},
		{files: map[string]string{procMeminfoPath: "MemAvailable: 1024 kB\n", procSelfCgroupPath: "0::/parent/worker\n"}},
		{files: map[string]string{procMeminfoPath: "MemAvailable: 1024 kB\n", procSelfCgroupPath: "0::/parent/worker\n", path.Join(cgroupV2MountPath, "parent", "worker", "memory.max"): "max\n"}},
		{files: readableDescendants, failures: map[string]error{rootMaximumPath: errors.New("unreadable")}},
		{files: readableDescendants, failures: map[string]error{rootMaximumPath: os.ErrPermission}},
	} {
		if got, source, available := availableMemoryForOS("linux", fixtureMemoryReaderWithErrors(specimen.files, specimen.failures), nil); available || got != 0 || source != "" {
			t.Fatalf("unreadable layout available=%t bytes=%d source=%q", available, got, source)
		}
	}
}

func TestDarwinAvailableMemoryUsesOnlyFreeInactiveAndSpeculativePages(t *testing.T) {
	t.Parallel()
	calls := 0
	run := func(ctx context.Context, name string, args ...string) ([]byte, error) {
		calls++
		if name != "vm_stat" || len(args) != 0 {
			t.Fatalf("command=%q args=%v", name, args)
		}
		if _, ok := ctx.Deadline(); !ok {
			t.Fatal("vm_stat context has no deadline")
		}
		return []byte("Mach Virtual Memory Statistics: (page size of 4096 bytes)\n" +
			"Pages free: 100.\nPages active: 400.\nPages inactive: 200.\n" +
			"Pages speculative: 50.\nPages purgeable: 999.\n"), nil
	}
	got, source, available := availableMemoryForOS("darwin", nil, run)
	if !available || got != 350*4096 || source != "darwin:vm_stat" || calls != 1 {
		t.Fatalf("available=%t bytes=%d source=%q calls=%d", available, got, source, calls)
	}
}

func TestDarwinAndUnsupportedMemoryFailuresAreUnknown(t *testing.T) {
	t.Parallel()
	for _, output := range []string{
		"Mach Virtual Memory Statistics: (page size of 4096 bytes)\nPages free: nope.\nPages inactive: 1.\nPages speculative: 1.\n",
		"Mach Virtual Memory Statistics: (page size of 18446744073709551615 bytes)\nPages free: 2.\nPages inactive: 0.\nPages speculative: 0.\n",
	} {
		got, source, available := availableMemoryForOS("darwin", nil, func(context.Context, string, ...string) ([]byte, error) { return []byte(output), nil })
		if available || got != 0 || source != "" {
			t.Fatalf("malformed vm_stat available=%t bytes=%d source=%q", available, got, source)
		}
	}
	if got, source, available := availableMemoryForOS("plan9", nil, nil); available || got != 0 || source != "" {
		t.Fatalf("unsupported available=%t bytes=%d source=%q", available, got, source)
	}
	if got, source, available := availableMemoryForOS("darwin", nil, func(context.Context, string, ...string) ([]byte, error) { return nil, errors.New("unreadable") }); available || got != 0 || source != "" {
		t.Fatalf("unreadable available=%t bytes=%d source=%q", available, got, source)
	}
	if got, source, available := availableMemoryForOS("darwin", nil, func(context.Context, string, ...string) ([]byte, error) { return nil, context.DeadlineExceeded }); available || got != 0 || source != "" {
		t.Fatalf("expired probe available=%t bytes=%d source=%q", available, got, source)
	}
}

func fixtureMemoryReader(files map[string]string) func(string) ([]byte, error) {
	return fixtureMemoryReaderWithErrors(files, nil)
}

func fixtureMemoryReaderWithErrors(files map[string]string, failures map[string]error) func(string) ([]byte, error) {
	return func(filePath string) ([]byte, error) {
		if err, found := failures[filePath]; found {
			return nil, err
		}
		if content, found := files[filePath]; found {
			return []byte(content), nil
		}
		return nil, os.ErrNotExist
	}
}
