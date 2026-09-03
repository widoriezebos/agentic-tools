package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveModelAlias(t *testing.T) {
	conf := filepath.Join(t.TempDir(), "metasystem.conf")
	putFile(t, conf, "metasystem.runtimes=claude\nruntime.claude.model-alias.claude-fable-5=claude-fable-5-1\n")
	model, aliased, err := ResolveModelAlias(conf, "claude", "claude-fable-5")
	if err != nil || model != "claude-fable-5-1" || !aliased {
		t.Fatalf("alias resolution = (%q, %v, %v)", model, aliased, err)
	}
	model, aliased, err = ResolveModelAlias(conf, "claude", "claude-fable-5-1")
	if err != nil || model != "claude-fable-5-1" || aliased {
		t.Fatalf("unalias resolution = (%q, %v, %v)", model, aliased, err)
	}
}

func TestResolveModelAliasOrigins(t *testing.T) {
	const key = "runtime.claude.model-alias.claude-fable-5"
	t.Run("production local and environment sources are refused", func(t *testing.T) {
		conf := filepath.Join(t.TempDir(), "metasystem.conf")
		putFile(t, conf, "metasystem.runtimes=claude\n"+key+"=claude-fable-5-1\n")
		putFile(t, conf+".local", key+"=claude-fable-5-2\n")
		if _, _, err := ResolveModelAlias(conf, "claude", "claude-fable-5"); err == nil || !strings.Contains(err.Error(), ".local source") {
			t.Fatalf("local alias was not refused by name: %v", err)
		}
		if err := os.Remove(conf + ".local"); err != nil {
			t.Fatal(err)
		}
		t.Setenv(EnvName(key), "claude-fable-5-2")
		if _, _, err := ResolveModelAlias(conf, "claude", "claude-fable-5"); err == nil || !strings.Contains(err.Error(), "environment source "+EnvName(key)) {
			t.Fatalf("environment alias was not refused by name: %v", err)
		}
	})

	t.Run("fixture-authorized sources are accepted", func(t *testing.T) {
		conf := filepath.Join(t.TempDir(), "metasystem.conf")
		putFile(t, conf, "metasystem.runtimes=fake\n")
		localKey := "runtime.fake.model-alias.fake-source"
		putFile(t, conf+".local", localKey+"=fake-local-target\n")
		model, aliased, err := ResolveModelAlias(conf, "fake", "fake-source")
		if err != nil || model != "fake-local-target" || !aliased {
			t.Fatalf("fixture local alias = (%q, %v, %v)", model, aliased, err)
		}
		t.Setenv(EnvName(localKey), "fake-env-target")
		model, aliased, err = ResolveModelAlias(conf, "fake", "fake-source")
		if err != nil || model != "fake-env-target" || !aliased {
			t.Fatalf("fixture environment alias = (%q, %v, %v)", model, aliased, err)
		}
	})
}
