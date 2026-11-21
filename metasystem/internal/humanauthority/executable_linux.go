package humanauthority

import (
	"errors"
	"fmt"
	"os"
	"strings"
)

func observeExecutable(pid int64) (path string, known, denied bool, name string) {
	return observeExecutableAt(fmt.Sprintf("/proc/%d", pid))
}

func observeExecutableAt(dir string) (path string, known, denied bool, name string) {
	path, err := os.Readlink(dir + "/exe")
	comm, _ := os.ReadFile(dir + "/comm")
	return path, err == nil && path != "", errors.Is(err, os.ErrPermission), strings.TrimSuffix(string(comm), "\n")
}
