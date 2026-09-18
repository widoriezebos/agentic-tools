package identity

import (
	"os"
	"os/exec"
	"testing"
	"time"
)

func TestProcessBirthReadsALiveProcess(t *testing.T) {
	afterBirth := time.Now()
	birth, ok := ProcessBirth(int64(os.Getpid()))
	if !ok || birth.IsZero() || birth.After(afterBirth) {
		t.Fatalf("ProcessBirth(self) = %s, %t; wall clock after birth = %s", birth, ok, afterBirth)
	}
}

func TestProcessBirthOfAMissingPidIsUnreadable(t *testing.T) {
	command := exec.Command("sh", "-c", "exit 0")
	if err := command.Run(); err != nil {
		t.Fatal(err)
	}
	birth, ok := ProcessBirth(int64(command.Process.Pid))
	if ok || !birth.IsZero() {
		t.Fatalf("ProcessBirth(waited pid %d) = %s, %t", command.Process.Pid, birth, ok)
	}
}
