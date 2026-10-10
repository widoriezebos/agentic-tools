package outage

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestObserveSuccessRetainsEpisodeRecoveryFields(t *testing.T) {
	t.Parallel()
	for _, elapsed := range []time.Duration{time.Minute, 10 * time.Minute} {
		t.Run(elapsed.String(), func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte("provider.recovery-alert-after=9m\n"), 0600); err != nil {
				t.Fatal(err)
			}
			reset := t0.Add(5 * time.Minute)
			mark, err := fixtureObserve(root, ProviderLimit, fmt.Sprintf("Claude AI usage limit reached|%d", reset.Unix()), "failed-seat", t0)
			if err != nil {
				t.Fatal(err)
			}
			// Later configuration and successful answers cannot rewrite this episode.
			if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte("provider.recovery-alert-after=1m\n"), 0600); err != nil {
				t.Fatal(err)
			}
			success := t0.Add(elapsed)
			for _, at := range []time.Time{success, success.Add(10 * time.Minute)} {
				if _, err := fixtureObserve(root, "", "", "successful-probe", at); err != nil {
					t.Fatal(err)
				}
			}
			providers, err := ReadProviders(filepath.Join(root, ".metasystem"))
			if err != nil {
				t.Fatal(err)
			}
			intervals := providers.Current["anthropic"].Intervals
			if len(intervals) != 1 {
				t.Fatalf("one retained episode required: %+v", intervals)
			}
			interval := intervals[0]
			until, stale := success, false
			if elapsed >= 7*time.Minute {
				until, stale = reset.Add(ProbeInterval), true
			}
			if interval.Since != mark.Since || interval.Until != until.Format(time.RFC3339Nano) || interval.Stale != stale || interval.FirstSuccessAt != success.Format(time.RFC3339Nano) || interval.RecoveryAfter != "9m" || interval.ResetAt != reset.Format(time.RFC3339Nano) {
				t.Fatalf("episode lost its first answer, closing cause or bound recovery fields: %+v", interval)
			}
			if due, err := interval.RecoveryDue(); err != nil || !due.Equal(success.Add(9*time.Minute)) {
				t.Fatalf("recovery due must use the retained first answer and interval: %s, %v", due, err)
			}
		})
	}
}

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
