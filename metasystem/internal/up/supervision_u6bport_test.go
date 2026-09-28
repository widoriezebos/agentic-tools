package up

import (
	"os"
	"path/filepath"
	"testing"
)

// delegate-caps AUTH-R2-007 (U6b port): arming derives the watcher ceiling
// from the installation's configuration only. With a 200-minute pair cap
// (above dispatch.cap-min 120 and watch.cap-min 180) the armed ceiling is
// 230; declaring --max-cap 300 raises it to 330. An unrelated mission
// contract under plans/ that names a 900-minute cap is not a configuration
// source and never raises the ceiling.
func TestArmingDerivesTheWatcherCeilingFromConfigurationOnly(t *testing.T) {
	t.Parallel()
	root := syntheticFingerprintRoot(t)
	conf := "metasystem.runtimes=\nwatch.stale-min=20\nwatch.cap-min=180\nwatch.interval-sec=1\n" +
		"dispatch.cap-min=120\ndispatch.cap-max=500\ncap.min.fake.fake-model=200\n"
	if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte(conf), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "plans"), 0o755); err != nil {
		t.Fatal(err)
	}
	contract := "```mission\nfence.wall-clock-hours=10\nfence.cycles=5\nfence.jobs=5\nfence.concurrency=2\nfence.job-cap-min=120\ncap.min.fake.fake-model=900\n```\n"
	if err := os.WriteFile(filepath.Join(root, "plans", "mission-ignored.contract.md"), []byte(contract), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		maxCap int64
		want   int64
	}{{0, 230}, {300, 330}} {
		options, err := supervisionOptions(Options{Root: root, MetasystemRoot: root, Scope: root, Binary: filepath.Join(root, "bin", "metasystem"), MaxCap: c.maxCap})
		if err != nil {
			t.Fatal(err)
		}
		if options.WatcherCap != c.want {
			t.Fatalf("--max-cap %d armed a %dm watcher ceiling, want %dm", c.maxCap, options.WatcherCap, c.want)
		}
		if options.IntervalSec != 1 {
			t.Fatalf("interval %d, want the configured 1", options.IntervalSec)
		}
	}
}
