package identity

import (
	"os"
	"os/exec"
	"testing"
)

func TestProcessBirthReadsALiveProcess(t *testing.T) {
	self := int64(os.Getpid())
	start, state, err := (KernelProber{}).ReadStart(self)
	if err != nil || state != Alive || start.StartedAt.IsZero() || !start.Ref().NativeExact() {
		t.Fatalf("ReadStart(self) = %+v, %s, %v", start, state, err)
	}
	birth, ok := ProcessBirth(self)
	if !ok || birth.IsZero() {
		t.Fatalf("ProcessBirth(self) = %s, %t; ReadStart = %s", birth, ok, start.StartedAt)
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
