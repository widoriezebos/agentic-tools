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
// window. Every number is a setting with a compiled-in default.
const (
	// DiskGoCacheCapGiBKey caps the engine's Go build cache: Go's default
	// cache under the user cache dir, the one every engine build and every
	// person's plain `go build` shares.
	DiskGoCacheCapGiBKey     = "disk.go-cache-cap-gib"
	DefaultDiskGoCacheCapGiB = 30
	// DiskDelegateGoCacheCapGiBKey caps the one machine delegate cache.
	DiskDelegateGoCacheCapGiBKey     = "disk.delegate-go-cache-cap-gib"
	DefaultDiskDelegateGoCacheCapGiB = 10
	// DiskStaticcheckCacheCapGiBKey caps each staticcheck cache (the
	// engine's and the delegates').
	DiskStaticcheckCacheCapGiBKey     = "disk.staticcheck-cache-cap-gib"
	DefaultDiskStaticcheckCacheCapGiB = 2
	// DiskGoCacheKeepHoursKey is the keep window: no entry used within it
	// is ever deleted, whatever the cap. It is never lowered by any floor.
	DiskGoCacheKeepHoursKey     = "disk.go-cache-keep-hours"
	DefaultDiskGoCacheKeepHours = 12
	// DiskCacheTrimBudgetSecKey bounds one trim pass over all caches; a
	// pass cut short resumes at its checkpoint on the next.
	DiskCacheTrimBudgetSecKey     = "disk.cache-trim-budget-sec"
	DefaultDiskCacheTrimBudgetSec = 10
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
}

// CacheTrimSettings resolves the five disk keys: the environment over the
// file over the compiled-in default.
func CacheTrimSettings(confPath string) (CacheTrim, error) {
	return cacheTrimSettings(confPath, os.LookupEnv)
}

func cacheTrimSettings(confPath string, lookupEnv func(string) (string, bool)) (CacheTrim, error) {
	var numbers [5]int64
	for index, setting := range []struct {
		key      string
		fallback int
	}{
		{DiskGoCacheCapGiBKey, DefaultDiskGoCacheCapGiB},
		{DiskDelegateGoCacheCapGiBKey, DefaultDiskDelegateGoCacheCapGiB},
		{DiskStaticcheckCacheCapGiBKey, DefaultDiskStaticcheckCacheCapGiB},
		{DiskGoCacheKeepHoursKey, DefaultDiskGoCacheKeepHours},
		{DiskCacheTrimBudgetSecKey, DefaultDiskCacheTrimBudgetSec},
	} {
		value, _, err := Get(GetParams{Key: setting.key, Default: strconv.Itoa(setting.fallback), DefaultSet: true, ConfPath: confPath, LookupEnv: lookupEnv})
		if err != nil {
			return CacheTrim{}, fmt.Errorf("resolve %s: %w", setting.key, err)
		}
		number, convErr := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
		if convErr != nil || number < 1 || number > maxDiskSetting {
			return CacheTrim{}, fmt.Errorf("%s must be a whole number from 1 to %d", setting.key, maxDiskSetting)
		}
		numbers[index] = number
	}
	const gib = int64(1) << 30
	return CacheTrim{
		EngineGoCapBytes: numbers[0] * gib, DelegateGoCapBytes: numbers[1] * gib, StaticcheckCapBytes: numbers[2] * gib,
		Keep: time.Duration(numbers[3]) * time.Hour, Budget: time.Duration(numbers[4]) * time.Second,
	}, nil
}
