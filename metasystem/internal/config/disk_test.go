package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
)

// The cache trimmer's five settings (disk-lifetimes A12): compiled-in
// defaults, a seat's own numbers, the environment over the file, and a
// value that is not a positive whole number refused rather than defaulted.
func TestCacheTrimSettingsResolution(t *testing.T) {
	t.Parallel()
	const gib = int64(1) << 30
	for _, tc := range []struct {
		name      string
		committed string
		lookup    func(string) (string, bool)
		want      CacheTrim
		refused   string
	}{
		{name: "the shipped defaults", want: CacheTrim{EngineGoCapBytes: 30 * gib, DelegateGoCapBytes: 10 * gib, StaticcheckCapBytes: 2 * gib, Keep: 12 * time.Hour, Budget: 10 * time.Second}},
		{
			name: "a seat's own numbers",
			committed: DiskGoCacheCapGiBKey + "=40\n" + DiskDelegateGoCacheCapGiBKey + "=5\n" + DiskStaticcheckCacheCapGiBKey + "=1\n" +
				DiskGoCacheKeepHoursKey + "=24\n" + DiskCacheTrimBudgetSecKey + "=30\n",
			want: CacheTrim{EngineGoCapBytes: 40 * gib, DelegateGoCapBytes: 5 * gib, StaticcheckCapBytes: gib, Keep: 24 * time.Hour, Budget: 30 * time.Second},
		},
		{
			name:   "the environment over the committed value",
			lookup: mapEnv(map[string]string{EnvName(DiskGoCacheCapGiBKey): "50"}),
			want:   CacheTrim{EngineGoCapBytes: 50 * gib, DelegateGoCapBytes: 10 * gib, StaticcheckCapBytes: 2 * gib, Keep: 12 * time.Hour, Budget: 10 * time.Second},
		},
		{name: "zero is refused", committed: DiskGoCacheKeepHoursKey + "=0\n", refused: DiskGoCacheKeepHoursKey + " must be a whole number from 1 to 100000"},
		{name: "words are refused", committed: DiskGoCacheCapGiBKey + "=lots\n", refused: DiskGoCacheCapGiBKey + " must be a whole number from 1 to 100000"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			confPath := filepath.Join(t.TempDir(), "metasystem.conf")
			testutil.Require(t, "write configuration", os.WriteFile(confPath, []byte(tc.committed), 0o644), nil)
			lookup := tc.lookup
			if lookup == nil {
				lookup = noEnv
			}
			got, err := cacheTrimSettings(confPath, lookup)
			if tc.refused != "" {
				testutil.Require(t, "refused", err != nil, true)
				testutil.Expect(t, "in these words", err.Error(), tc.refused)
				return
			}
			testutil.Require(t, "resolve", err, nil)
			testutil.Expect(t, "the settings", got, tc.want)
		})
	}
}

// The five keys are validated beside the other numeric knobs.
func TestValidateCacheTrimKnobs(t *testing.T) {
	for _, key := range []string{DiskGoCacheCapGiBKey, DiskDelegateGoCacheCapGiBKey, DiskStaticcheckCacheCapGiBKey, DiskGoCacheKeepHoursKey, DiskCacheTrimBudgetSecKey} {
		if problems := validateRepo(t, validConf+key+"=0\n"); !hasProblem(problems, key+" must be a positive integer") {
			t.Fatalf("%s=0: %v", key, problems)
		}
		if problems := validateRepo(t, validConf+key+"=7\n"); hasProblem(problems, key) {
			t.Fatalf("%s=7 rejected: %v", key, problems)
		}
	}
}
