package config

import (
	"path/filepath"
	"testing"
	"time"
)

// TestBoardSettingsResolveAndValidate (R19, R25; U10c-1's two rows of
// TestBoardAndPipelineSettingsHaveCompiledDefaults): board.keep-hours,
// board.poll-sec and board.handover-lock-wait-sec resolve to 24 hours, 5
// seconds and 30 seconds with no configuration file, to a file's overrides
// when it sets them, and a non-integer value is refused by validate with the
// key named; none is a proof input.
func TestBoardSettingsResolveAndValidate(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	empty := filepath.Join(dir, "empty.conf")
	putFile(t, empty, "# overrides only\n")
	settings, err := ResolveBoard(empty)
	if err != nil || settings.Keep != 24*time.Hour || settings.Poll != 5*time.Second || settings.MailboxKeep != 7*24*time.Hour ||
		settings.HandoverLockWait != 30*time.Second {
		t.Fatalf("compiled board settings = %+v, %v", settings, err)
	}
	if DefaultBoard() != settings {
		t.Fatalf("DefaultBoard %+v differs from the resolved compiled defaults %+v", DefaultBoard(), settings)
	}
	set := filepath.Join(dir, "set.conf")
	putFile(t, set, BoardKeepHoursKey+"=6\n"+BoardPollSecKey+"=2\n"+BoardMailboxKeepDaysKey+"=3\n"+BoardHandoverLockWaitSecKey+"=9\n")
	if settings, err := ResolveBoard(set); err != nil || settings.Keep != 6*time.Hour || settings.Poll != 2*time.Second || settings.MailboxKeep != 3*24*time.Hour ||
		settings.HandoverLockWait != 9*time.Second {
		t.Fatalf("overridden board settings = %+v, %v", settings, err)
	}
	if wait, err := ResolveHandoverLockWait(empty); err != nil || wait != 30*time.Second {
		t.Fatalf("compiled handover lock wait = %v, %v; want 30s", wait, err)
	}
	if wait, err := ResolveHandoverLockWait(set); err != nil || wait != 9*time.Second {
		t.Fatalf("overridden handover lock wait = %v, %v; want 9s", wait, err)
	}
	zero := filepath.Join(dir, "zero.conf")
	putFile(t, zero, BoardHandoverLockWaitSecKey+"=0\n")
	if _, err := ResolveHandoverLockWait(zero); err == nil {
		t.Fatal("a handover lock wait of 0 seconds resolved; it must be a positive integer")
	}
	for _, key := range []string{BoardKeepHoursKey, BoardPollSecKey, BoardMailboxKeepDaysKey, BoardHandoverLockWaitSecKey} {
		if ProofInput(key) {
			t.Fatalf("%s is no proof input", key)
		}
		if problems := validateRepo(t, validConf+key+"=1d\n"); !hasProblem(problems, key) {
			t.Fatalf("validate accepted %s=1d: %v", key, problems)
		}
	}
}
