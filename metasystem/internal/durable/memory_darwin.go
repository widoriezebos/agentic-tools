package durable

import (
	"os"
	"strconv"
	"syscall"
)

// MemoryVolumeRoot is the exact kernel mount name admitted for test storage.
// The test harness reaches the same volume through Darwin's /tmp symlink.
func MemoryVolumeRoot() string { return "/private/tmp/mst-" + strconv.Itoa(os.Geteuid()) }

func memoryVolume(f *os.File) bool {
	return memoryVolumeWith(f, syscall.Fstatfs, os.Lstat)
}

func memoryVolumeWith(f *os.File, fstatfs func(int, *syscall.Statfs_t) error, lstat func(string) (os.FileInfo, error)) bool {
	if f == nil {
		return false
	}
	conn, err := f.SyscallConn()
	if err != nil {
		return false
	}
	attested := false
	err = conn.Control(func(fd uintptr) {
		var fs syscall.Statfs_t
		if fstatfs(int(fd), &fs) != nil {
			return
		}
		root := MemoryVolumeRoot()
		// Compare the complete NUL-terminated kernel name, never the file's path.
		name := make([]byte, 0, len(fs.Mntonname))
		for _, c := range fs.Mntonname {
			if c == 0 {
				break
			}
			name = append(name, byte(c))
		}
		if string(name) != root {
			return
		}
		uid := uint32(os.Geteuid())
		info, err := lstat(root)
		if err != nil || info == nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 || !ownedBy(info, uid) {
			return
		}
		marker, err := lstat(root + "/.mst-ram-backed")
		attested = err == nil && marker != nil && marker.Mode().IsRegular() && ownedBy(marker, uid)
	})
	return err == nil && attested
}

func ownedBy(info os.FileInfo, uid uint32) bool {
	stat, ok := info.Sys().(*syscall.Stat_t)
	return ok && stat != nil && stat.Uid == uid
}
