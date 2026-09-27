package gittree

import (
	"bytes"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/boundedexec"
)

// InstallPrefix derives the mission project's path under its git
// toplevel ("" at the toplevel, "metasystem" nested) — the one fact the
// whole project scoping hangs on, derived in one place so the validator
// and its witnesses cannot drift apart. Bounded like every other
// external call, and lossless: only git's record terminator is
// stripped, never characters a directory name may lawfully carry.
func InstallPrefix(root string) (string, error) {
	cmd := exec.Command("git", "-C", root, "rev-parse", "--show-prefix")
	cmd.Env = ScrubbedEnviron()
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	limit := boundedexec.Timeout(filepath.Join(root, "metasystem.conf"), boundedexec.Local)
	if err := boundedexec.Run(cmd, limit, "git rev-parse --show-prefix"); err != nil {
		return "", fmt.Errorf("rev-parse --show-prefix: %w", err)
	}
	prefix := strings.TrimSuffix(stdout.String(), "\n")
	return strings.TrimSuffix(prefix, "/"), nil
}
