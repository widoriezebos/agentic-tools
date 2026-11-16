package testenv

import (
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"syscall"

	"golang.org/x/sys/unix"
)

type memoryRootProbe struct {
	path   func() string
	mount  func(string, os.FileInfo) bool
	statfs func(string, *unix.Statfs_t) error
	lease  *memoryRootLeaseState
}

type memoryRootLeaseState struct {
	sync.Mutex
	file *os.File
}

var defaultMemoryRoot = memoryRootProbe{
	path:   func() string { return "/tmp/mst-" + strconv.Itoa(os.Geteuid()) },
	mount:  memoryRootMount,
	statfs: unix.Statfs,
	lease:  &memoryRootLeaseState{},
}

func memoryRootMount(path string, info os.FileInfo) bool {
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return false
	}
	parent, err := os.Stat(filepath.Dir(resolved))
	if err != nil {
		return false
	}
	return info.Sys().(*syscall.Stat_t).Dev != parent.Sys().(*syscall.Stat_t).Dev
}

func (probe memoryRootProbe) acquireLease() bool {
	probe.lease.Lock()
	defer probe.lease.Unlock()
	if probe.lease.file != nil {
		return true
	}
	file, err := os.OpenFile(probe.path()+".lease", os.O_RDWR|os.O_CREATE|unix.O_NOFOLLOW, 0o600)
	if err != nil {
		return false
	}
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Sys().(*syscall.Stat_t).Uid != uint32(os.Geteuid()) {
		_ = file.Close()
		return false
	}
	if err := unix.Flock(int(file.Fd()), unix.LOCK_SH|unix.LOCK_NB); err != nil {
		_ = file.Close()
		return false
	}
	// The keeper may recycle the volume only after every process releases it.
	probe.lease.file = file
	return true
}

func (probe memoryRootProbe) provenRoot() (string, bool) {
	if !probe.acquireLease() {
		return "", false
	}
	path := probe.path()
	info, err := os.Lstat(path)
	if err != nil || !info.IsDir() || info.Sys().(*syscall.Stat_t).Uid != uint32(os.Geteuid()) || !probe.mount(path, info) {
		return "", false
	}
	marker, err := os.Lstat(filepath.Join(path, ".mst-ready"))
	if err != nil || !marker.Mode().IsRegular() {
		return "", false
	}
	var space unix.Statfs_t
	if err := probe.statfs(path, &space); err != nil || space.Bsize <= 0 || uint64(space.Bavail)*uint64(space.Bsize) < 512<<20 {
		return "", false
	}
	return path, true
}
