//go:build darwin

package identity

import (
	"encoding/binary"
	"slices"
	"strings"
	"testing"
	"unsafe"

	"golang.org/x/sys/unix"
)

func TestProcessCensusDecodesParentsFromOneBuffer(t *testing.T) {
	t.Parallel()

	pidOffset := int(unsafe.Offsetof(unix.ExternProc{}.P_pid))
	parentOffset := int(unsafe.Offsetof(unix.KinfoProc{}.Eproc) + unsafe.Offsetof(unix.Eproc{}.Ppid))
	raw := make([]byte, 4*unix.SizeofKinfoProc)
	for index, process := range []struct {
		pid, parent int32
	}{{100, 1}, {101, 100}, {102, 102}, {0, 1}} {
		entry := raw[index*unix.SizeofKinfoProc:]
		binary.LittleEndian.PutUint32(entry[pidOffset:], uint32(process.pid))
		binary.LittleEndian.PutUint32(entry[parentOffset:], uint32(process.parent))
	}
	census, err := decodeProcessCensus(raw)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := census.Pids(), []int64{100, 101, 102}; !slices.Equal(got, want) {
		t.Fatalf("process census pids = %v, want %v", got, want)
	}
	for _, test := range []struct {
		pid, parent int64
		known       bool
	}{{100, 1, true}, {101, 100, true}, {102, 0, true}, {999, 0, false}} {
		parent, known := census.Parent(test.pid)
		if parent != test.parent || known != test.known {
			t.Errorf("Parent(%d) = (%d, %v), want (%d, %v)", test.pid, parent, known, test.parent, test.known)
		}
	}
	if _, err := decodeProcessCensus(make([]byte, unix.SizeofKinfoProc-1)); err == nil || !strings.Contains(err.Error(), "ABI drift") {
		t.Fatalf("short process table drift error = %v", err)
	}
}
