package humanauthority

import (
	"bytes"
	"encoding/json"
	"os"
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
	reader := enrolledReader()
	enrolled, err := Enroll(root, 20, reader, "Wido", time.Unix(1000, 0))
	if err != nil {
		t.Fatal(err)
	}
	want := `{"schema":1,"enrolledAt":"1970-01-01T00:16:40Z","generation":1,"human":"Wido","terminalId":"tty-1","terminalRef":{"pid":20,"pidStartedAt":200},"sessionLeaderRef":{"pid":10,"pidStartedAt":100}}`
	got, err := json.Marshal(enrolled)
	if err != nil || string(got) != want {
		t.Fatalf("record differs from main: %s (%v)", got, err)
	}
	raw, err := os.ReadFile(enrollmentPath(root))
	var baseline bytes.Buffer
	if indentErr := json.Indent(&baseline, []byte(want), "", "  "); indentErr != nil {
		t.Fatal(indentErr)
	}
	baseline.WriteByte('\n')
	if err != nil || !bytes.Equal(raw, baseline.Bytes()) {
		t.Fatal("persisted enrollment:", err)
	}
	r := loginTable()
	_, err = ProveTerminal(root, 50, r, time.Unix(2000, 0))
	wantError := "ANCESTRY_UNREADABLE: process pid 30, owner uid 501 was not admitted because the process's executable path is unreadable\nrun: metasystem system enroll --name <your name>  (in a shell you own up to its session leader, such as one in a tmux session you started)"
	if err == nil || err.Error() != wantError || TerminalCheck() != "" {
		t.Fatalf("refusal differs from main: %v", err)
	}
}
