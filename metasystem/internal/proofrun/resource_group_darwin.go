//go:build darwin

package proofrun

import (
	"encoding/binary"
	"errors"
	"fmt"
	"time"
	"unsafe"

	"golang.org/x/sys/unix"
)

// kern.proc.pgrp is a native, complete snapshot of one live process group.
// It avoids traversing unrelated processes during a custodian's last census.
func custodyGroupMembers(group int64) ([]int64, error) {
	var raw []byte
	var err error
	for attempt := 0; attempt < 5; attempt++ {
		raw, err = unix.SysctlRaw("kern.proc.pgrp", int(group))
		if err == nil || !errors.Is(err, unix.ENOMEM) {
			break
		}
		time.Sleep(time.Duration(10*(attempt+1)) * time.Millisecond)
	}
	if err != nil {
		return nil, fmt.Errorf("resource group membership is indeterminable: %w", err)
	}
	const pidOffset = int(unsafe.Offsetof(unix.ExternProc{}.P_pid))
	const groupOffset = int(unsafe.Offsetof(unix.KinfoProc{}.Eproc) + unsafe.Offsetof(unix.Eproc{}.Pgid))
	if pidOffset != 40 || len(raw)%unix.SizeofKinfoProc != 0 {
		return nil, fmt.Errorf("resource group census has unexpected Darwin ABI or length")
	}
	members := make([]int64, 0, len(raw)/unix.SizeofKinfoProc)
	for offset := 0; offset < len(raw); offset += unix.SizeofKinfoProc {
		pid := int64(int32(binary.LittleEndian.Uint32(raw[offset+pidOffset:])))
		pgid := int64(int32(binary.LittleEndian.Uint32(raw[offset+groupOffset:])))
		if pgid != group {
			return nil, fmt.Errorf("resource group census returned pid %d in group %d", pid, pgid)
		}
		if pid > 0 && pid != group {
			members = append(members, pid)
		}
	}
	return members, nil
}
