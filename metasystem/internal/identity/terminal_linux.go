package identity

import (
	"fmt"
	"os"
	"strings"
)

// The controlling terminal is field 7 (tty_nr) of /proc/<pid>/stat;
// zero means none. The comm field can contain spaces, so fields are
// counted after its closing parenthesis.
func kernelTerminal(pid int64) (bool, bool) {
	terminal, ok := kernelTerminalIdentity(pid)
	return terminal != "", ok
}

func kernelTerminalIdentity(pid int64) (string, bool) {
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
	if err != nil {
		return "", false
	}
	s := string(data)
	close := strings.LastIndexByte(s, ')')
	if close < 0 {
		return "", false
	}
	fields := strings.Fields(s[close+1:])
	// After the comm field: state, ppid, pgrp, session, tty_nr, ...
	if len(fields) < 5 {
		return "", false
	}
	if fields[4] == "0" {
		return "", true
	}
	return "linux-tty:" + fields[4], true
}
