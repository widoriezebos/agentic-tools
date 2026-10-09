package outage

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestObserveInvalidRecoveryIntervalPreservesLimit(t *testing.T) {
	t.Parallel()
	for _, settings := range []string{"provider.recovery-alert-after=soon\n", "provider.recovery-alert-after=6m\nprovider.recovery-alert-after=7m\n"} {
		t.Run(settings, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte(settings), 0600); err != nil {
				t.Fatal(err)
			}
			mark, err := fixtureObserve(root, ProviderLimit, "HTTP 429 Too Many Requests", "failed-seat", t0)
			if err != nil {
				t.Fatalf("invalid recovery declaration discarded provider evidence: %v", err)
			}
			stored, standing := fixtureStanding(root, t0.Add(time.Second))
			if !standing || stored != mark || stored.ConsecutiveFailures != 1 || stored.LastClass != ProviderLimit || stored.RecoveryAfter != "" {
				t.Fatalf("provider limit must stand without an invented recovery interval: %+v, standing %v", stored, standing)
			}
			if _, err := fixtureObserve(root, "", "", "successful-probe", t0.Add(time.Minute)); err != nil {
				t.Fatal(err)
			}
			providers, err := ReadProviders(filepath.Join(root, ".metasystem"))
			if err != nil {
				t.Fatal(err)
			}
			intervals := providers.Current["anthropic"].Intervals
			if len(intervals) != 1 || intervals[0].FirstSuccessAt != t0.Add(time.Minute).Format(time.RFC3339Nano) {
				t.Fatalf("missing retained successful episode: %+v", intervals)
			}
			if due, err := intervals[0].RecoveryDue(); err == nil || !due.IsZero() {
				t.Fatalf("missing declaration must report a source obligation, never a guessed deadline: %s, %v", due, err)
			}
		})
	}
}
