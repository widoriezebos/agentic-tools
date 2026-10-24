package testenv

import (
	"fmt"
	"os"
	"syscall"
	"testing"
)

// HoldFIFO keeps a reader and writer open until cleanup, so fixture opens
// return at once and handoffs are written data. O_RDWR on a FIFO is defined
// on Darwin and Linux; other systems may not support it.
func HoldFIFO(t testing.TB, path string) *os.File {
	t.Helper()
	file, err := os.OpenFile(path, os.O_RDWR, 0)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = file.Close() })
	return file
}

// AwaitLine reads one line without consuming the next. A raw nonblocking
// descriptor avoids a reader goroutine that Close cannot interrupt on Darwin.
func AwaitLine(t AwaitTB, what, path string, producer func() (bool, string), giveUp func() string) string {
	t.Helper()
	fd, err := syscall.Open(path, syscall.O_RDONLY|syscall.O_NONBLOCK|syscall.O_CLOEXEC, 0)
	if err != nil {
		t.Fatalf("%s: %v", what, err)
		return ""
	}
	defer syscall.Close(fd)
	var line []byte
	complete := false
	AwaitFrom(t, what, func() bool {
		var one [1]byte
		for {
			n, readErr := syscall.Read(fd, one[:])
			if readErr == syscall.EAGAIN {
				return false
			}
			if readErr != nil {
				err = readErr
				return true
			}
			if n == 0 {
				err = fmt.Errorf("every writer closed before a line")
				return true
			}
			if one[0] == '\n' {
				complete = true
				return true
			}
			line = append(line, one[0])
		}
	}, producer, giveUp)
	if err != nil {
		t.Fatalf("%s: %v", what, err)
	}
	if !complete {
		return ""
	}
	return string(line)
}
