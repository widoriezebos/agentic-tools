package config

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"
)

func TestProofDeclarationsIgnoreEveryOverlayAndHaveNoDefault(t *testing.T) {
	t.Parallel()
	conf := filepath.Join(t.TempDir(), "metasystem.conf")
	for _, key := range []string{"proof.full", "proof.cheap"} {
		putFile(t, conf, key+"=committed command\n")
		putFile(t, conf+".local", key+"=local command\n")
		params := GetParams{Key: key, ConfPath: conf, Flag: "flag command", FlagSet: true, Default: "fallback", DefaultSet: true, LookupEnv: mapEnv(map[string]string{EnvName(key): "environment command"})}
		value, code, err := Get(params)
		if err != nil || code != 0 || value != "committed command" {
			t.Fatalf("%s = %q, %d, %v", key, value, code, err)
		}
		source, err := KeyOrigin(params)
		if err != nil || source != "conf; environment value ignored; local value ignored" {
			t.Fatalf("%s source = %q, %v", key, source, err)
		}
		if _, found := CompiledDefault(key); found {
			t.Fatalf("%s has a compiled default", key)
		}
		putFile(t, conf, "# absent\n")
		if _, found, err := CommittedLookup(conf, key); found || err != nil {
			t.Fatalf("absent %s = %v, %v", key, found, err)
		}
		if _, code, err := Get(params); code != 1 || !errors.Is(err, ErrNoValue) {
			t.Fatalf("missing %s = %d, %v", key, code, err)
		}
	}
}

func TestProofDeclarationValidationRequiresBothCommands(t *testing.T) {
	t.Parallel()
	conf := filepath.Join(t.TempDir(), "metasystem.conf")
	for _, body := range []string{"", "proof.full=\nproof.cheap=true\n", "proof.full=first\rsecond\nproof.cheap=true\n", "proof.full=true\n"} {
		putFile(t, conf, "metasystem.template=true\n"+body)
		_, problems, err := Validate(conf, filepath.Dir(conf))
		if err != nil {
			t.Fatal(err)
		}
		text := strings.Join(problems, "\n")
		if body == "proof.full=true\n" {
			if !strings.Contains(text, "proof.cheap is required") {
				t.Fatalf("missing cheap command: %s", text)
			}
		} else if !strings.Contains(text, "proof.full") {
			t.Fatalf("invalid full command accepted: %s", text)
		}
	}
	for _, key := range []string{"proof.full", "proof.cheap"} {
		for _, value := range []string{"", " ", "first\nsecond", "first\rsecond"} {
			if err := SettingValueProblem(key, value); err == nil {
				t.Fatalf("%s accepted %q", key, value)
			}
		}
	}
	if err := SettingKeyProblem("landing.prove.command"); err == nil || !strings.Contains(err.Error(), "proof.full") {
		t.Fatalf("retired key: %v", err)
	}
}

func TestProofCommittedContentUsesTheStrictParser(t *testing.T) {
	t.Parallel()
	for _, key := range []string{"proof.full", "proof.cheap"} {
		value, found, err := CommittedContentLookup("# declaration\n  "+key+" = test x=y  \n", key)
		if err != nil || !found || value != "test x=y" {
			t.Fatalf("%s = %q, %v, %v", key, value, found, err)
		}
		if _, found, err := CommittedContentLookup("# absent\n", key); err != nil || found {
			t.Fatalf("missing %s = %v, %v", key, found, err)
		}
		if _, _, err := CommittedContentLookup(key+"=true\n"+key+"=false\n", key); err == nil || !strings.Contains(err.Error(), key) {
			t.Fatalf("duplicate %s accepted: %v", key, err)
		}
	}
}
