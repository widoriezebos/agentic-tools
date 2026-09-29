package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// The two settings resolve with their compiled defaults and say so; a .local
// threshold of 1 over a committed 2 is the one that binds (g1-s70 D1, S70-03).
func TestLandingGateSettingsResolveThroughTheLayersWithTheirSources(t *testing.T) {
	t.Setenv(EnvName(LandingHumanFromTierKey), "")
	t.Setenv(EnvName(LandingAutoAfterKey), "")
	os.Unsetenv(EnvName(LandingHumanFromTierKey))
	os.Unsetenv(EnvName(LandingAutoAfterKey))
	dir := t.TempDir()
	conf := filepath.Join(dir, "metasystem.conf")
	if err := os.WriteFile(conf, []byte("# overrides only\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gate, err := ResolveLandingGate(conf)
	if err != nil {
		t.Fatal(err)
	}
	if gate.HumanFromTier != 2 || gate.AutoAfter != 4*time.Hour || gate.Tier.Source != "default" || gate.After.Source != "default" || gate.After.Value != "4h" {
		t.Fatalf("defaults: %+v", gate)
	}
	if err := os.WriteFile(conf, []byte(LandingHumanFromTierKey+" = 2\n"+LandingAutoAfterKey+" = 90m\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(conf+".local", []byte(LandingHumanFromTierKey+" = 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gate, err = ResolveLandingGate(conf)
	if err != nil {
		t.Fatal(err)
	}
	if gate.HumanFromTier != 1 || gate.Tier.Source != "conf-local" || gate.AutoAfter != 90*time.Minute || gate.After.Source != "conf" {
		t.Fatalf("the .local threshold over the committed one: %+v", gate)
	}
	t.Setenv(EnvName(LandingAutoAfterKey), "2h")
	if gate, err = ResolveLandingGate(conf); err != nil || gate.AutoAfter != 2*time.Hour || gate.After.Source != "env" {
		t.Fatalf("the environment: %+v %v", gate, err)
	}
	if err := os.WriteFile(conf+".local", []byte(LandingHumanFromTierKey+" = two\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := ResolveLandingGate(conf); err == nil {
		t.Fatal("a malformed threshold resolved")
	}
	if _, problems, _ := Validate(conf, dir); len(problems) == 0 {
		t.Fatal("validate passed a malformed threshold")
	}
}
