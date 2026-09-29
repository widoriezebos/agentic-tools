package config

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
)

// The disk-lifetime settings (design engine-owns-disk-lifetimes Part B, 3.13;
// Wido 2026-09-28: "I want these configurable; with sensible defaults").
// Every bound, target and timing of the design is one row here with its
// compiled-in default. metasystem.conf and metasystem.conf.local only
// override; the shipped metasystem.conf lists none of them. This table is
// the one place the numbers are spelled: diskstore.Settings reads every key
// through Get with the row's default, and Validate checks every source that
// sets one.

// DiskSettingScope says whose settings a key is read from.
type DiskSettingScope string

const (
	// ScopeSegment keys are read from an evidence segment's context checkout.
	ScopeSegment DiskSettingScope = "segment"
	// ScopeCheckout keys are read by the pass that runs for the checkout.
	ScopeCheckout DiskSettingScope = "checkout"
	// ScopeHost keys are resolved over every armed checkout; differing values
	// resolve to the most conservative one, and an unreadable participant
	// makes the key Unknown for the pass.
	ScopeHost DiskSettingScope = "host"
)

// DiskSettingUnit is how a value is read.
type DiskSettingUnit string

const (
	UnitGiB   DiskSettingUnit = "GiB"
	UnitMiB   DiskSettingUnit = "MiB"
	UnitDays  DiskSettingUnit = "days"
	UnitHours DiskSettingUnit = "hours"
	UnitSec   DiskSettingUnit = "seconds"
	UnitCount DiskSettingUnit = "count"
	UnitPath  DiskSettingUnit = "path"
	UnitPaths DiskSettingUnit = "paths"
)

// Conservative says which of two differing host values is in force.
type Conservative string

const (
	ConservativeLargest  Conservative = "largest"
	ConservativeSmallest Conservative = "smallest"
)

// DiskSetting is one row of 3.13.
type DiskSetting struct {
	Key     string
	Default string // "" for "none"
	Unit    DiskSettingUnit
	Scope   DiskSettingScope
	// Conservative is set for host-scoped numbers: the direction that
	// deletes less, holds a lock for less, or measures more.
	Conservative Conservative
	Meaning      string
}

// Disk-lifetime keys read by name elsewhere.
const (
	DiskEvidenceSegmentCapKey  = "evidence.segment-cap-gib"
	DiskEvidenceMachineCapKey  = "evidence.machine-cap-gib"
	DiskEvidenceAgeFloorKey    = "evidence.age-floor-days"
	DiskEvidenceCitationKey    = "evidence.citation-roots"
	DiskEvidenceExportDirKey   = "evidence.export-dir"
	DiskEvidenceBlobGraceKey   = "evidence.blob-grace-hours"
	DiskProofTargetKey         = "disk.proof-target-gib"
	DiskProofKeepKey           = "disk.proof-keep-days"
	DiskSuiteTargetKey         = "disk.suite-failure-target-gib"
	DiskSuiteDistillKey        = "disk.suite-failure-distill-hours"
	DiskSuiteMoveKey           = "disk.suite-failure-move-days"
	DiskLaunchTargetKey        = "disk.launch-target-mib"
	DiskLaunchKeepKey          = "disk.launch-keep-days"
	DiskUnitTargetKey          = "disk.unit-target-mib"
	DiskUnitKeepKey            = "disk.unit-keep-days"
	DiskRegistryHistoryKey     = "disk.registry-history-target-mib"
	DiskContextKeepKey         = "disk.context-keep-days"
	DiskProcessScratchKey      = "disk.process-scratch-target-gib"
	DiskJobWorktreeKey         = "disk.job-worktree-target-gib"
	DiskWorkspaceKey           = "disk.workspace-target-gib"
	DiskCandidateEnginesKey    = "disk.candidate-engines-keep"
	DiskCompressAboveKey       = "disk.compress-above-mib"
	DiskSessionBootstrapKey    = "disk.session-bootstrap-hours"
	DiskFloorKey               = "disk.floor-gib"
	DiskFloorMinAgeKey         = "disk.floor-min-age-hours"
	DiskSweepBudgetKey         = "disk.sweep-budget-sec"
	DiskSweepItemsPerLockKey   = "disk.sweep-items-per-lock"
	DiskCensusMinBudgetKey     = "disk.census-min-budget-sec"
	DiskEvidenceRootDefaultDoc = "$HOME/metasystem-evidence/<checkout basename>"
)

var diskSettings = []DiskSetting{
	{Key: EvidenceRootKey, Unit: UnitPath, Scope: ScopeSegment, Meaning: "this checkout's durable evidence root (default " + DiskEvidenceRootDefaultDoc + "); read only through ResolveEvidenceRoot"},
	{Key: DiskEvidenceSegmentCapKey, Default: "10", Unit: UnitGiB, Scope: ScopeSegment, Meaning: "per checkout segment: measured bytes plus blob charges past which the machine pass reports the segment over its bound (it never removes)"},
	{Key: DiskEvidenceMachineCapKey, Default: "50", Unit: UnitGiB, Scope: ScopeHost, Conservative: ConservativeLargest, Meaning: "every evidence root on the host plus the blob store, physical bytes, past which the machine pass reports (it never removes)"},
	{Key: DiskEvidenceAgeFloorKey, Default: "90", Unit: UnitDays, Scope: ScopeSegment, Meaning: "an item younger than this by its recorded end time is never selected by evidence dispose --over-bound"},
	{Key: DiskEvidenceCitationKey, Unit: UnitPaths, Scope: ScopeSegment, Meaning: "extra locations scanned for citations, comma-separated, absolute or relative to the state root"},
	{Key: DiskEvidenceExportDirKey, Unit: UnitPath, Scope: ScopeSegment, Meaning: "where evidence export writes archives when --to is omitted; outside every evidence root and checkout"},
	{Key: DiskEvidenceBlobGraceKey, Default: "24", Unit: UnitHours, Scope: ScopeHost, Conservative: ConservativeLargest, Meaning: "how long an unreferenced blob stays before the sweep removes it"},
	{Key: DiskProofTargetKey, Default: "2", Unit: UnitGiB, Scope: ScopeCheckout, Meaning: "per checkout: proof attempts, their records and retained proof"},
	{Key: DiskProofKeepKey, Default: "14", Unit: UnitDays, Scope: ScopeCheckout, Meaning: "attempts younger than this are retention roots"},
	{Key: DiskSuiteTargetKey, Default: "2", Unit: UnitGiB, Scope: ScopeCheckout, Meaning: "per checkout: suite-failure bundles still in the checkout"},
	{Key: DiskSuiteDistillKey, Default: "24", Unit: UnitHours, Scope: ScopeCheckout, Meaning: "a bundle idle this long is distilled"},
	{Key: DiskSuiteMoveKey, Default: "7", Unit: UnitDays, Scope: ScopeCheckout, Meaning: "a distilled bundle this old is moved into its segment"},
	{Key: DiskLaunchTargetKey, Default: "512", Unit: UnitMiB, Scope: ScopeHost, Conservative: ConservativeLargest, Meaning: "~/.metasystem/launch/"},
	{Key: DiskLaunchKeepKey, Default: "14", Unit: UnitDays, Scope: ScopeHost, Conservative: ConservativeLargest, Meaning: "launches younger than this are kept"},
	{Key: DiskUnitTargetKey, Default: "256", Unit: UnitMiB, Scope: ScopeHost, Conservative: ConservativeLargest, Meaning: "~/.metasystem/unit/"},
	{Key: DiskUnitKeepKey, Default: "14", Unit: UnitDays, Scope: ScopeHost, Conservative: ConservativeLargest, Meaning: "a disposable unit record younger than this is kept"},
	{Key: DiskRegistryHistoryKey, Default: "64", Unit: UnitMiB, Scope: ScopeHost, Conservative: ConservativeLargest, Meaning: "armed-checkouts.jsonl; over target is reported"},
	{Key: DiskContextKeepKey, Default: "14", Unit: UnitDays, Scope: ScopeCheckout, Meaning: "context handoffs"},
	{Key: DiskProcessScratchKey, Default: "4", Unit: UnitGiB, Scope: ScopeCheckout, Meaning: "one process's scratch root"},
	{Key: DiskJobWorktreeKey, Default: "8", Unit: UnitGiB, Scope: ScopeCheckout, Meaning: "one delegate workspace"},
	{Key: DiskWorkspaceKey, Default: "8", Unit: UnitGiB, Scope: ScopeCheckout, Meaning: "one handed-out workspace"},
	{Key: DiskCandidateEnginesKey, Default: "8", Unit: UnitCount, Scope: ScopeCheckout, Meaning: "newest candidate engines kept"},
	{Key: DiskCompressAboveKey, Default: "1", Unit: UnitMiB, Scope: ScopeCheckout, Meaning: "files at or above this are gzipped by the launcher and the distiller"},
	{Key: DiskSessionBootstrapKey, Default: "24", Unit: UnitHours, Scope: ScopeCheckout, Meaning: "a reserved session whose bootstrap died with no main announced is reported after this"},
	{Key: DiskFloorKey, Default: "50", Unit: UnitGiB, Scope: ScopeHost, Conservative: ConservativeLargest, Meaning: "free space below which floor mode runs"},
	{Key: DiskFloorMinAgeKey, Default: "1", Unit: UnitHours, Scope: ScopeCheckout, Meaning: "the ageing threshold floor mode lowers to, never zero"},
	{Key: DiskSweepBudgetKey, Default: "20", Unit: UnitSec, Scope: ScopeHost, Conservative: ConservativeSmallest, Meaning: "one pass's whole budget (the machine pass takes the smallest across checkouts)"},
	{Key: DiskSweepItemsPerLockKey, Default: "8", Unit: UnitCount, Scope: ScopeHost, Conservative: ConservativeSmallest, Meaning: "usage pairs retired per maintenance-lock acquisition"},
	{Key: DiskCensusMinBudgetKey, Default: "5", Unit: UnitSec, Scope: ScopeHost, Conservative: ConservativeLargest, Meaning: "remaining budget below which the use census is skipped"},
}

// DiskSettings returns the rows of 3.13, in the design's order.
func DiskSettings() []DiskSetting { return append([]DiskSetting(nil), diskSettings...) }

// DiskSettingFor returns one row.
func DiskSettingFor(key string) (DiskSetting, bool) {
	for _, setting := range diskSettings {
		if setting.Key == key {
			return setting, true
		}
	}
	return DiskSetting{}, false
}

// Numeric reports whether the row holds a positive whole number.
func (s DiskSetting) Numeric() bool {
	switch s.Unit {
	case UnitGiB, UnitMiB, UnitDays, UnitHours, UnitSec, UnitCount:
		return true
	}
	return false
}

// ValidateDiskSetting judges one value of one disk key: a number is a
// positive whole number (so days and hours are at least 1 and nothing is
// zero); a path is absolute; a path list is comma-separated, non-empty
// entries on one line. The evidence root is judged by ResolveEvidenceRoot.
func ValidateDiskSetting(key, value string) error {
	setting, ok := DiskSettingFor(key)
	if !ok {
		return fmt.Errorf("%s is not a disk-lifetime setting", key)
	}
	if setting.Numeric() {
		parsed, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
		if err != nil || parsed < 1 {
			return fmt.Errorf("%s must be a positive whole number of %s, got %s", key, setting.Unit, pyRepr(value))
		}
		return nil
	}
	switch setting.Unit {
	case UnitPath:
		if key == EvidenceRootKey {
			return nil
		}
		if strings.TrimSpace(value) != "" && !filepath.IsAbs(strings.TrimSpace(value)) {
			return fmt.Errorf("%s must be an absolute path, got %s", key, pyRepr(value))
		}
	case UnitPaths:
		if strings.ContainsAny(value, "\n\r") {
			return fmt.Errorf("%s must be one line of comma-separated paths", key)
		}
		for _, entry := range strings.Split(value, ",") {
			if strings.TrimSpace(value) != "" && strings.TrimSpace(entry) == "" {
				return fmt.Errorf("%s has an empty entry in %s", key, pyRepr(value))
			}
		}
	}
	return nil
}

// validateDiskSettings is Validate's clause for every key of 3.13, over the
// committed file, its .local sibling and the environment.
func validateDiskSettings(confPath string, committed map[string]string, lookupEnv func(string) (string, bool)) []string {
	var problems []string
	localPath := confPath + ".local"
	for _, setting := range diskSettings {
		if setting.Key == EvidenceRootKey {
			continue
		}
		if value, present := committed[setting.Key]; present {
			if err := ValidateDiskSetting(setting.Key, value); err != nil {
				problems = append(problems, fmt.Sprintf("%s: %v", confPath, err))
			}
		}
		if isFile(localPath) {
			value, present, err := ConfLookup(localPath, setting.Key)
			if err != nil {
				problems = append(problems, err.Error())
			} else if present {
				if err := ValidateDiskSetting(setting.Key, value); err != nil {
					problems = append(problems, fmt.Sprintf("%s: %v", localPath, err))
				}
			}
		}
		if value, present := lookupEnv(EnvName(setting.Key)); present {
			if err := ValidateDiskSetting(setting.Key, value); err != nil {
				problems = append(problems, fmt.Sprintf("environment %s: %v", EnvName(setting.Key), err))
			}
		}
	}
	return problems
}
