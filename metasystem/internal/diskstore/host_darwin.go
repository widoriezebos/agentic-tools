package diskstore

import (
	"fmt"
	"os/exec"
	"strings"
)

// readHostTempRoot asks confstr(_CS_DARWIN_USER_TEMP_DIR) through getconf,
// which reads the per-user directory the system assigns and ignores TMPDIR.
var readHostTempRoot = func() (string, error) {
	output, err := exec.Command("/usr/bin/getconf", "DARWIN_USER_TEMP_DIR").Output()
	if err != nil {
		return "", fmt.Errorf("read the host temporary root (getconf DARWIN_USER_TEMP_DIR): %w", err)
	}
	return strings.TrimSpace(string(output)), nil
}
