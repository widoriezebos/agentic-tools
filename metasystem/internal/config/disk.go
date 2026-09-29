package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// The steward's cache trimmer (disk-lifetimes A12) keeps each machine cache
// to a hard cap by last use, never deleting an entry used inside the keep
// window. Every number is a setting whose default is in the one compiled
// table (defaults.go).
const (
	// DiskGoCacheCapGiBKey caps the engine's Go build cache: Go's default
	// cache under the user cache dir, the one every engine build and every
	// person's plain `go build` shares.
	DiskGoCacheCapGiBKey = "disk.go-cache-cap-gib"
	// DiskDelegateGoCacheCapGiBKey caps the one machine delegate cache.
	DiskDelegateGoCacheCapGiBKey = "disk.delegate-go-cache-cap-gib"
	// DiskStaticcheckCacheCapGiBKey caps each staticcheck cache (the
	// engine's and the delegates').
	DiskStaticcheckCacheCapGiBKey = "disk.staticcheck-cache-cap-gib"
	// DiskGoCacheKeepHoursKey is the keep window: no entry used within it
	// is ever deleted, whatever the cap. It is never lowered by any floor.
	DiskGoCacheKeepHoursKey = "disk.go-cache-keep-hours"
	// DiskCacheTrimBudgetSecKey bounds one trim pass over all caches; a
	// pass cut short resumes at its checkpoint on the next.
	DiskCacheTrimBudgetSecKey = "disk.cache-trim-budget-sec"
	// DiskCacheTrimPersonBudgetSecKey bounds `metasystem disk clean` run at
	// a terminal: it runs pass after pass of disk.cache-trim-budget-sec
	// until every cache is measured and trimmed, within this many seconds.
	// The steward's background pass keeps disk.cache-trim-budget-sec.
	DiskCacheTrimPersonBudgetSecKey = "disk.cache-trim-person-budget-sec"
)

// maxDiskSetting is the ceiling a disk number is admitted within, so a
// mistyped value is refused with the range in it rather than accepted.
const maxDiskSetting = 100000

// CacheTrim is the trimmer's settings as this seat resolves them.
type CacheTrim struct {
	EngineGoCapBytes    int64
	DelegateGoCapBytes  int64
	StaticcheckCapBytes int64
	Keep                time.Duration
	Budget              time.Duration
	PersonBudget        time.Duration
}

// CacheTrimSettings resolves the six disk keys: the environment over the
// file over the compiled-in default.
func CacheTrimSettings(confPath string) (CacheTrim, error) {
	return cacheTrimSettings(confPath, os.LookupEnv)
}

func cacheTrimSettings(confPath string, lookupEnv func(string) (string, bool)) (CacheTrim, error) {
	var numbers [6]int64
	for index, key := range []string{DiskGoCacheCapGiBKey, DiskDelegateGoCacheCapGiBKey, DiskStaticcheckCacheCapGiBKey,
		DiskGoCacheKeepHoursKey, DiskCacheTrimBudgetSecKey, DiskCacheTrimPersonBudgetSecKey} {
		value, _, err := Get(GetParams{Key: key, ConfPath: confPath, LookupEnv: lookupEnv})
		if err != nil {
			return CacheTrim{}, fmt.Errorf("resolve %s: %w", key, err)
		}
		number, convErr := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
		if convErr != nil || number < 1 || number > maxDiskSetting {
			return CacheTrim{}, fmt.Errorf("%s must be a whole number from 1 to %d", key, maxDiskSetting)
		}
		numbers[index] = number
	}
	const gib = int64(1) << 30
	return CacheTrim{
		EngineGoCapBytes: numbers[0] * gib, DelegateGoCapBytes: numbers[1] * gib, StaticcheckCapBytes: numbers[2] * gib,
		Keep: time.Duration(numbers[3]) * time.Hour, Budget: time.Duration(numbers[4]) * time.Second,
		PersonBudget: time.Duration(numbers[5]) * time.Second,
	}, nil
}
