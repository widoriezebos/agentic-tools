package launch

// The host lock: one launch at a time on this host.
//
// A launch spends disk, a build and the human's credentials, and two of them
// at once would race for the same destination names and the same host
// resources. The exclusion has to be host-wide rather than checkout-wide,
// because two interface checkouts on one machine are two servers, so the lock
// is a file in the host's temporary directory held with flock for the whole
// life of the verb — the way proof admission holds its own.
//
// The holder writes its nickname and its launch id into the file, so the
// second launch can be refused with the running one's name rather than with a
// bare "busy". A process that dies releases the flock, which is what makes
// the lock self-healing: a stale file with somebody's nickname in it is taken
// by the next launch, and the record's own process identity — not this file —
// is what says whether that launch died.

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

// LockName is the file every launch on this host contends for.
const LockName = "metasystem-seat-launch.lock"

// LockPath is that file in the host's temporary directory.
func LockPath() string { return filepath.Join(os.TempDir(), LockName) }

// Lock is one held host lock. Release gives it back.
type Lock struct {
	file *os.File
}

// Take holds the host lock for this launch, or refuses with the running
// launch's nickname and id.
//
// The path is a parameter so a test can contend two sequencers on a lock of
// its own rather than on the host's.
func Take(path, machine, id string) (*Lock, error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		held := holder(file)
		_ = file.Close()
		if err == syscall.EWOULDBLOCK || err == syscall.EAGAIN {
			return nil, refuse(CodeRunning,
				"another launch is running on this host%s; one machine joins at a time", held)
		}
		return nil, err
	}
	// The holder's own line replaces whatever a previous holder left, and is
	// written only once the lock is held, so nothing but the holder writes it.
	if err := file.Truncate(0); err == nil {
		_, _ = file.WriteAt([]byte(machine+" "+id+"\n"), 0)
	}
	return &Lock{file: file}, nil
}

// Release gives the lock back. A launch that dies releases it too, because
// the kernel closes the descriptor.
func (l *Lock) Release() error {
	if l == nil || l.file == nil {
		return nil
	}
	_ = syscall.Flock(int(l.file.Fd()), syscall.LOCK_UN)
	err := l.file.Close()
	l.file = nil
	return err
}

// holder reads the running launch's nickname and id out of the lock file, as
// the clause a refusal names them in. A file that says nothing readable adds
// no clause rather than a guess.
func holder(file *os.File) string {
	line := make([]byte, 256)
	read, err := file.ReadAt(line, 0)
	if read == 0 && err != nil {
		return ""
	}
	fields := strings.Fields(string(line[:read]))
	if len(fields) < 2 {
		return ""
	}
	return ": " + fields[0] + " under launch " + fields[1]
}
