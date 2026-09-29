package config

import (
	"strings"
	"testing"
)

// Every key of Part B 3.13 has a compiled-in default (or none, for the
// three path keys), and Validate has a clause for it in every source: the
// committed file, .local and the environment (the settings witness, R7).
func TestEveryDiskSettingHasADefaultAndAValidationClause(t *testing.T) {
	t.Parallel()
	want := []string{
		"evidence.root", "evidence.segment-cap-gib", "evidence.machine-cap-gib", "evidence.age-floor-days", "evidence.citation-roots",
		"evidence.export-dir", "evidence.blob-grace-hours", "disk.proof-target-gib", "disk.proof-keep-days", "disk.suite-failure-target-gib",
		"disk.suite-failure-distill-hours", "disk.suite-failure-move-days", "disk.launch-target-mib", "disk.launch-keep-days",
		"disk.unit-target-mib", "disk.unit-keep-days", "disk.registry-history-target-mib", "disk.context-keep-days",
		"disk.process-scratch-target-gib", "disk.job-worktree-target-gib", "disk.workspace-target-gib", "disk.workspace-ignored-release-mib", "disk.candidate-engines-keep",
		"disk.compress-above-mib", "disk.session-bootstrap-hours", "disk.floor-gib", "disk.floor-min-age-hours", "disk.sweep-budget-sec",
		"disk.sweep-items-per-lock", "disk.census-min-budget-sec",
	}
	rows := DiskSettings()
	if len(rows) != len(want) {
		t.Fatalf("%d disk settings, want the %d of 3.13 and the Round B3 rulings", len(rows), len(want))
	}
	for index, row := range rows {
		if row.Key != want[index] {
			t.Fatalf("row %d is %s, want %s", index, row.Key, want[index])
		}
		if row.Numeric() && ValidateDiskSetting(row.Key, row.Default) != nil {
			t.Errorf("%s has no valid compiled default: %q", row.Key, row.Default)
		}
		if !row.Numeric() && row.Default != "" {
			t.Errorf("%s is a path setting with a default %q; 3.13 says none", row.Key, row.Default)
		}
		if row.Scope == ScopeHost && row.Conservative == "" {
			t.Errorf("%s is host-scoped without its conservative direction", row.Key)
		}
		if row.Meaning == "" {
			t.Errorf("%s has no meaning line", row.Key)
		}
	}
	for _, row := range rows {
		if !row.Numeric() {
			continue
		}
		for _, bad := range []string{"0", "-1", "ten", "1.5", ""} {
			if ValidateDiskSetting(row.Key, bad) == nil {
				t.Errorf("%s accepted %q", row.Key, bad)
			}
		}
		committed := validateRepo(t, validConf+row.Key+"=0\n")
		if !hasProblem(committed, row.Key+" must be a positive whole number") {
			t.Errorf("a committed %s=0 was not reported: %v", row.Key, committed)
		}
		local := validateRepo(t, validConf, row.Key+"=zero\n")
		if !hasProblem(local, ".local: "+row.Key) {
			t.Errorf("a .local %s=zero was not reported: %v", row.Key, local)
		}
	}
	environment := func(name string) (string, bool) {
		if name == EnvName(DiskFloorKey) {
			return "-5", true
		}
		return "", false
	}
	if problems := validateDiskSettings("metasystem.conf", map[string]string{}, environment); !hasProblem(problems, "environment "+EnvName(DiskFloorKey)) {
		t.Errorf("an environment %s=-5 was not reported: %v", DiskFloorKey, problems)
	}
}

func TestDiskPathSettingsAreJudged(t *testing.T) {
	t.Parallel()
	if err := ValidateDiskSetting(DiskEvidenceExportDirKey, "exports"); err == nil || !strings.Contains(err.Error(), "absolute") {
		t.Fatalf("a relative export dir = %v", err)
	}
	for _, good := range []string{"", "/Volumes/Backup/metasystem-exports"} {
		if err := ValidateDiskSetting(DiskEvidenceExportDirKey, good); err != nil {
			t.Fatalf("export dir %q = %v", good, err)
		}
	}
	if err := ValidateDiskSetting(DiskEvidenceCitationKey, "records,,docs"); err == nil {
		t.Fatal("an empty citation root entry was accepted")
	}
	if err := ValidateDiskSetting(DiskEvidenceCitationKey, "records/extra,/abs/notes"); err != nil {
		t.Fatal(err)
	}
	if problems := validateRepo(t, validConf+DiskEvidenceExportDirKey+"=exports\n"); !hasProblem(problems, "must be an absolute path") {
		t.Fatalf("a relative committed export dir was not reported: %v", problems)
	}
}
