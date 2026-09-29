package main

import (
	"os"
	"path/filepath"
	"testing"
)

// settings show answers a disk-lifetime key from its compiled-in default
// with source "default" when no source sets it, and names the source that
// overrides it (engine-owns-disk-lifetimes 3.13).
func TestSettingsShowAnswersDiskDefaults(t *testing.T) {
	t.Parallel()
	conf := filepath.Join(t.TempDir(), "metasystem.conf")
	if err := os.WriteFile(conf, []byte("metasystem.version=1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	value, source, code, err := configSettingWithDefault("disk.floor-gib", conf)
	if err != nil || code != 0 || value != "50" || source != "default" {
		t.Fatalf("disk.floor-gib = %q from %q (%d, %v); want 50 from default", value, source, code, err)
	}
	if err := os.WriteFile(conf+".local", []byte("disk.floor-gib=80\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if value, source, _, _ := configSettingWithDefault("disk.floor-gib", conf); value != "80" || source != "conf-local" {
		t.Fatalf("an override = %q from %q", value, source)
	}
	if _, _, code, _ := configSettingWithDefault("no.such.key", conf); code == 0 {
		t.Fatal("an unknown key resolved")
	}
}
