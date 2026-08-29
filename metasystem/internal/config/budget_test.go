package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func clearBudgetLawEnvironment(t *testing.T) {
	t.Helper()
	for _, key := range []string{ElapsedGracePercentKey, SliceNormHoursKey} {
		name := EnvName(key)
		previous, present := os.LookupEnv(name)
		if err := os.Unsetenv(name); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if present {
				_ = os.Setenv(name, previous)
			} else {
				_ = os.Unsetenv(name)
			}
		})
	}
}

func TestBudgetLawConfigurationRequiresCommittedProductionSources(t *testing.T) {
	clearBudgetLawEnvironment(t)
	directory := t.TempDir()
	conf := filepath.Join(directory, "metasystem.conf")
	putFile(t, conf, "")

	percent, err := ElapsedGracePercent(conf)
	if err != nil || percent != 50 {
		t.Fatalf("absent key: percent=%d err=%v", percent, err)
	}
	putFile(t, conf, ElapsedGracePercentKey+"=25\n")
	percent, err = ElapsedGracePercent(conf)
	if err != nil || percent != 25 {
		t.Fatalf("committed value: percent=%d err=%v", percent, err)
	}
	putFile(t, conf+".local", ElapsedGracePercentKey+"=75\n")
	if _, err = ElapsedGracePercent(conf); err == nil || !strings.Contains(err.Error(), "committed root configuration") {
		t.Fatalf("production accepted local grace authority: %v", err)
	}
	_ = os.Remove(conf + ".local")
	t.Setenv(EnvName(ElapsedGracePercentKey), "125")
	if _, err = ElapsedGracePercent(conf); err == nil || !strings.Contains(err.Error(), "committed root configuration") {
		t.Fatalf("production accepted environment grace authority: %v", err)
	}

	_ = os.Unsetenv(EnvName(ElapsedGracePercentKey))
	putFile(t, conf, SliceNormHoursKey+"=6\n")
	putFile(t, conf+".local", SliceNormHoursKey+"=7\n")
	if _, err = SliceNormHours(conf); err == nil || !strings.Contains(err.Error(), "committed root configuration") {
		t.Fatalf("production accepted local slice-norm authority: %v", err)
	}
	_ = os.Remove(conf + ".local")
	t.Setenv(EnvName(SliceNormHoursKey), "8")
	if _, err = SliceNormHours(conf); err == nil || !strings.Contains(err.Error(), "committed root configuration") {
		t.Fatalf("production accepted environment slice-norm authority: %v", err)
	}
}

func TestBudgetLawConfigurationAllowsFixtureAuthorizedOverrides(t *testing.T) {
	clearBudgetLawEnvironment(t)
	conf := filepath.Join(t.TempDir(), "metasystem.conf")
	putFile(t, conf, "metasystem.runtimes=fake\n"+ElapsedGracePercentKey+"=25\n"+SliceNormHoursKey+"=6\n")
	putFile(t, conf+".local", ElapsedGracePercentKey+"=75\n"+SliceNormHoursKey+"=7\n")

	if percent, err := ElapsedGracePercent(conf); err != nil || percent != 75 {
		t.Fatalf("fixture local grace: percent=%d err=%v", percent, err)
	}
	if hours, err := SliceNormHours(conf); err != nil || hours != 7 {
		t.Fatalf("fixture local slice norm: hours=%d err=%v", hours, err)
	}
	t.Setenv(EnvName(ElapsedGracePercentKey), "125")
	t.Setenv(EnvName(SliceNormHoursKey), "8")
	if percent, err := ElapsedGracePercent(conf); err != nil || percent != 125 {
		t.Fatalf("fixture environment grace: percent=%d err=%v", percent, err)
	}
	if hours, err := SliceNormHours(conf); err != nil || hours != 8 {
		t.Fatalf("fixture environment slice norm: hours=%d err=%v", hours, err)
	}
}

func TestBudgetLawConfigurationRefusesMalformedLocalSource(t *testing.T) {
	clearBudgetLawEnvironment(t)
	conf := filepath.Join(t.TempDir(), "metasystem.conf")
	putFile(t, conf, ElapsedGracePercentKey+"=50\n")
	putFile(t, conf+".local", ElapsedGracePercentKey+"=75\n"+ElapsedGracePercentKey+"=100\n")

	if _, err := ElapsedGracePercent(conf); err == nil || !strings.Contains(err.Error(), "duplicate") {
		t.Fatalf("ambiguous local grace source was accepted: %v", err)
	}
}

func TestElapsedGracePercentRefusesMalformedValues(t *testing.T) {
	clearBudgetLawEnvironment(t)
	for _, value := range []string{"", "-1", "1.5", "201", "999999999999999999999999"} {
		t.Run(strings.ReplaceAll(value, ".", "_"), func(t *testing.T) {
			conf := filepath.Join(t.TempDir(), "metasystem.conf")
			putFile(t, conf, ElapsedGracePercentKey+"="+value+"\n")
			if _, err := ElapsedGracePercent(conf); err == nil || !strings.Contains(err.Error(), "integer between 0 and 200") {
				t.Fatalf("value %q was not refused by range: %v", value, err)
			}
		})
	}

	conf := filepath.Join(t.TempDir(), "metasystem.conf")
	putFile(t, conf, ElapsedGracePercentKey+"=0\n"+ElapsedGracePercentKey+"=50\n")
	if _, err := ElapsedGracePercent(conf); err == nil || !strings.Contains(err.Error(), "duplicate") {
		t.Fatalf("duplicate grace key was accepted: %v", err)
	}

	conf = filepath.Join(t.TempDir(), "metasystem.conf")
	putFile(t, conf, "metasystem.runtimes=fake\n"+ElapsedGracePercentKey+"=50\n")
	putFile(t, conf+".local", ElapsedGracePercentKey+"=201\n")
	if _, err := ElapsedGracePercent(conf); err == nil || !strings.Contains(err.Error(), "integer between 0 and 200") {
		t.Fatalf("malformed local grace override was accepted: %v", err)
	}
	putFile(t, conf+".local", ElapsedGracePercentKey+"=50\n")
	t.Setenv(EnvName(ElapsedGracePercentKey), "-1")
	if _, err := ElapsedGracePercent(conf); err == nil || !strings.Contains(err.Error(), "integer between 0 and 200") {
		t.Fatalf("malformed environment grace override was accepted: %v", err)
	}
}

func TestSliceNormHoursUsesDefaultAndRefusesMalformedSources(t *testing.T) {
	name := EnvName(SliceNormHoursKey)
	previous, present := os.LookupEnv(name)
	_ = os.Unsetenv(name)
	t.Cleanup(func() {
		if present {
			_ = os.Setenv(name, previous)
		} else {
			_ = os.Unsetenv(name)
		}
	})
	conf := filepath.Join(t.TempDir(), "metasystem.conf")
	putFile(t, conf, "")
	if hours, err := SliceNormHours(conf); err != nil || hours != 4 {
		t.Fatalf("absent slice norm: hours=%d err=%v", hours, err)
	}
	putFile(t, conf, "metasystem.runtimes=fake\n"+SliceNormHoursKey+"=6\n")
	if hours, err := SliceNormHours(conf); err != nil || hours != 6 {
		t.Fatalf("configured slice norm: hours=%d err=%v", hours, err)
	}
	putFile(t, conf+".local", SliceNormHoursKey+"=0\n")
	if _, err := SliceNormHours(conf); err == nil || !strings.Contains(err.Error(), "positive integer") {
		t.Fatalf("zero local slice norm was accepted: %v", err)
	}
	putFile(t, conf+".local", SliceNormHoursKey+"=7\n")
	t.Setenv(name, "four")
	if _, err := SliceNormHours(conf); err == nil || !strings.Contains(err.Error(), "positive integer") {
		t.Fatalf("malformed environment slice norm was accepted: %v", err)
	}
}
