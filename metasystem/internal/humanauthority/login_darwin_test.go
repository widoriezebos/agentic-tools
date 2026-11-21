package humanauthority

import (
	"path/filepath"
	"testing"
	"time"
)

func TestDarwinLoginRecordAndRefusalBaseline(t *testing.T) {
	t.Parallel()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	r := loginTable()
	_, err = ProveTerminal(root, 50, r, time.Unix(2000, 0))
	wantError := "ANCESTRY_UNREADABLE: process pid 30, owner uid 501 was not admitted because the process's executable path is unreadable\nrun: metasystem system enroll --name <your name>  (in a shell you own up to its session leader, such as one in a tmux session you started)"
	if err == nil || err.Error() != wantError || TerminalCheck() != "" {
		t.Fatalf("refusal differs from main: %v", err)
	}
}
