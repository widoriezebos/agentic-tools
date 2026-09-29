package goal

import (
	"errors"
	"testing"
)

// EM-41: the refusal handed over a raw config line padded with double
// spaces and a <nickname> placeholder, after a policy aside. It names the
// problem and the one command that fixes it.
func TestResolveMachineWithoutANicknameNamesTheFix(t *testing.T) {
	t.Parallel()
	_, err := ResolveMachineWithConfig(t.TempDir(), func(string, string) (string, error) { return "", errors.New("exit status 1") })
	want := "no machine nickname is enrolled on this machine; name it once with: git config metasystem.goal.machine NAME"
	if err == nil || err.Error() != want {
		t.Fatalf("no nickname = %v, want %q", err, want)
	}
}
