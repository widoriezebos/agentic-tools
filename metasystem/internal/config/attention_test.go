package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLedgerAttentionStaleMinutesUsesDefaultAndRefusesMalformedSources(t *testing.T) {
	name := EnvName(LedgerAttentionStaleMinutesKey)
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
	if minutes, err := LedgerAttentionStaleMinutes(conf); err != nil || minutes != 30 {
		t.Fatalf("absent ledger-attention threshold: minutes=%d err=%v", minutes, err)
	}
	putFile(t, conf, "metasystem.runtimes=fake\n"+LedgerAttentionStaleMinutesKey+"=45\n")
	if minutes, err := LedgerAttentionStaleMinutes(conf); err != nil || minutes != 45 {
		t.Fatalf("configured ledger-attention threshold: minutes=%d err=%v", minutes, err)
	}
	putFile(t, conf+".local", LedgerAttentionStaleMinutesKey+"=0\n")
	if _, err := LedgerAttentionStaleMinutes(conf); err == nil || !strings.Contains(err.Error(), "positive integer") {
		t.Fatalf("zero local ledger-attention threshold was accepted: %v", err)
	}
	putFile(t, conf+".local", LedgerAttentionStaleMinutesKey+"=60\n")
	t.Setenv(name, "half-hour")
	if _, err := LedgerAttentionStaleMinutes(conf); err == nil || !strings.Contains(err.Error(), "positive integer") {
		t.Fatalf("malformed environment ledger-attention threshold was accepted: %v", err)
	}
}

func TestLedgerAttentionStaleMinutesRefusesProductionOverrides(t *testing.T) {
	name := EnvName(LedgerAttentionStaleMinutesKey)
	_ = os.Unsetenv(name)
	t.Cleanup(func() { _ = os.Unsetenv(name) })
	conf := filepath.Join(t.TempDir(), "metasystem.conf")
	putFile(t, conf, LedgerAttentionStaleMinutesKey+"=30\n")
	putFile(t, conf+".local", LedgerAttentionStaleMinutesKey+"=45\n")
	if _, err := LedgerAttentionStaleMinutes(conf); err == nil || !strings.Contains(err.Error(), "committed root configuration") {
		t.Fatalf("production local override was accepted: %v", err)
	}
	_ = os.Remove(conf + ".local")
	t.Setenv(name, "45")
	if _, err := LedgerAttentionStaleMinutes(conf); err == nil || !strings.Contains(err.Error(), "committed root configuration") {
		t.Fatalf("production environment override was accepted: %v", err)
	}
}
