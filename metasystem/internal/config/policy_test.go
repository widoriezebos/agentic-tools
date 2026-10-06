package config

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/helm"
)

func TestPolicyReaderPrecedenceAndFreshness(t *testing.T) {
	t.Parallel()
	checkout, coordinator := t.TempDir(), t.TempDir()
	conf, coordinatorConf := filepath.Join(checkout, "settings.conf"), filepath.Join(coordinator, "settings.conf")
	putFile(t, conf, "# empty\n")
	putFile(t, coordinatorConf, "landing.batch=3\n")
	reads := 0
	registry := PolicyRegistry{Coordinator: coordinator}
	readers := PolicyReaders{Registry: func(string) (PolicyRegistry, error) { reads++; return registry, nil }, ConfPath: func(string) (string, error) { return coordinatorConf, nil }, Helm: func(string) helm.State { return helm.State{} }}
	p := GetParams{Key: "landing.batch", ConfPath: conf, LookupEnv: noEnv, Policy: &PolicyContext{Checkout: checkout, CallingCheckout: checkout, Readers: readers}}
	assert := func(value, origin string) {
		t.Helper()
		got, code, err := Get(p)
		if err != nil || code != 0 || got != value {
			t.Fatalf("Get: %q %d %v want %q", got, code, err, value)
		}
		source, err := KeyOrigin(p)
		if err != nil || source != origin {
			t.Fatalf("source: %q %v want %q", source, err, origin)
		}
	}
	assert("3", "coordinator/conf")
	putFile(t, conf, "landing.batch=2\n")
	assert("2", "conf")
	putFile(t, conf+".local", "landing.batch=1\n")
	assert("1", "conf-local")
	p.LookupEnv = mapEnv(map[string]string{EnvName(p.Key): "4"})
	assert("4", "env")
	p.Flag, p.FlagSet = "5", true
	assert("5", "flag")
	p.Policy.CallingCheckout = "another-checkout"
	assert("1", "conf-local")
	p.FlagSet = false
	p.LookupEnv = noEnv
	if err := os.Remove(conf + ".local"); err != nil {
		t.Fatal(err)
	}
	putFile(t, conf, "# empty\n")
	registry.Coordinator = ""
	assert("auto", "built-in")
	if reads == 0 {
		t.Fatal("registry was never consulted")
	}
	p.Policy.Readers.Registry = func(string) (PolicyRegistry, error) { return PolicyRegistry{}, fmt.Errorf("corrupt coordinator") }
	if _, code, err := Get(p); code == 0 || err == nil {
		t.Fatal("corrupt declaration became default")
	}
	p.Policy.Readers.Registry = readers.Registry
	putFile(t, conf+".local", "landing.batch=0\n")
	if _, code, err := Get(p); code == 0 || err == nil {
		t.Fatal("invalid cap accepted")
	}
	putFile(t, conf+".local", "landing.batch=1\nlanding.batch=2\n")
	if _, code, err := Get(p); code == 0 || err == nil {
		t.Fatal("duplicate became default")
	}
	if err := os.Remove(conf + ".local"); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(conf+".local", 0700); err != nil {
		t.Fatal(err)
	}
	if _, code, err := Get(p); code == 0 || err == nil {
		t.Fatal("unreadable local layer became default")
	}
}

func TestPolicyValidationIsStructural(t *testing.T) {
	t.Parallel()
	for _, body := range []string{"landing.batch=2\nreview.stop=0\nsettings.apply=now\n", "landing.batch=9999999999999999999999999999999\n"} {
		if problems := validateRepo(t, validConf+body); len(problems) != 0 {
			t.Fatalf("valid policy without a lane: %v", problems)
		}
	}
	for _, keyValue := range []string{"landing.batch=0", "review.stop=-1", "seat.driver=2", "settings.apply=person", "question.route=maybe"} {
		if problems := validateRepo(t, validConf+keyValue+"\n"); len(problems) == 0 {
			t.Fatalf("accepted %s", keyValue)
		}
		if problems := validateRepo(t, validConf, keyValue+"\n"); len(problems) == 0 {
			t.Fatalf("accepted local %s", keyValue)
		}
	}
	if problems := validateRepo(t, validConf, "settings.apply=now\n"); !hasProblem(problems, "committed repository declaration") {
		t.Fatalf("local timing scope: %v", problems)
	}
	for _, key := range []string{"landing.batch", "landing.proof", "landing.on-red", "landing.trunk-red", "seat.driver", "review.stop", "goal.raise", "question.route", "settings.apply"} {
		setting, ok := compiledSetting(key)
		if !ok || setting.ProofInput {
			t.Fatalf("policy changed proof identity: %s %+v", key, setting)
		}
	}
}
