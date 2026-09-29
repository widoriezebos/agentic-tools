package config

import (
	"path/filepath"
	"testing"
	"time"
)

// TestBoardSettingsResolveAndValidate (R19, R25; U10c-1's two rows of
// TestBoardAndPipelineSettingsHaveCompiledDefaults): board.keep-hours and
// board.poll-sec resolve to 24 hours and 5 seconds with no configuration
// file, to a file's overrides when it sets them, and a non-integer value is
// refused by validate with the key named; neither is a proof input.
func TestBoardSettingsResolveAndValidate(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	empty := filepath.Join(dir, "empty.conf")
	putFile(t, empty, "# overrides only\n")
	settings, err := ResolveBoard(empty)
	if err != nil || settings.Keep != 24*time.Hour || settings.Poll != 5*time.Second || settings.MailboxKeep != 7*24*time.Hour {
		t.Fatalf("compiled board settings = %+v, %v", settings, err)
	}
	if DefaultBoard() != settings {
		t.Fatalf("DefaultBoard %+v differs from the resolved compiled defaults %+v", DefaultBoard(), settings)
	}
	set := filepath.Join(dir, "set.conf")
	putFile(t, set, BoardKeepHoursKey+"=6\n"+BoardPollSecKey+"=2\n"+BoardMailboxKeepDaysKey+"=3\n")
	if settings, err := ResolveBoard(set); err != nil || settings.Keep != 6*time.Hour || settings.Poll != 2*time.Second || settings.MailboxKeep != 3*24*time.Hour {
		t.Fatalf("overridden board settings = %+v, %v", settings, err)
	}
	for _, key := range []string{BoardKeepHoursKey, BoardPollSecKey, BoardMailboxKeepDaysKey} {
		if ProofInput(key) {
			t.Fatalf("%s is no proof input", key)
		}
		if problems := validateRepo(t, validConf+key+"=1d\n"); !hasProblem(problems, key) {
			t.Fatalf("validate accepted %s=1d: %v", key, problems)
		}
	}
}
