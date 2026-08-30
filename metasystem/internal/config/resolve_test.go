package config

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func putFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func mapEnv(m map[string]string) func(string) (string, bool) {
	return func(k string) (string, bool) { v, ok := m[k]; return v, ok }
}

func noEnv(string) (string, bool) { return "", false }

// TestGetPrecedence walks the resolution order the reference reader defines:
// flag, environment, .local, mode-scoped, committed, default.
func TestGetPrecedence(t *testing.T) {
	dir := t.TempDir()
	conf := filepath.Join(dir, "metasystem.conf")
	putFile(t, conf, "plain.knob=plain-value\n"+
		"role.implementer.runtime=base-runtime\n"+
		"mode.refactor.role.implementer.runtime=mode-runtime\n")

	envName := "METASYSTEM_ROLE_IMPLEMENTER_RUNTIME"

	cases := []struct {
		name   string
		params GetParams
		want   string
		code   int
	}{
		{
			name:   "flag wins over everything",
			params: GetParams{Key: "role.implementer.runtime", Mode: "refactor", Flag: "flag", FlagSet: true, LookupEnv: mapEnv(map[string]string{envName: "environment"}), ConfPath: conf},
			want:   "flag",
		},
		{
			name:   "empty flag still wins",
			params: GetParams{Key: "plain.knob", Flag: "", FlagSet: true, ConfPath: conf, LookupEnv: noEnv},
			want:   "",
		},
		{
			name:   "environment wins over conf and mode",
			params: GetParams{Key: "role.implementer.runtime", Mode: "refactor", LookupEnv: mapEnv(map[string]string{envName: "environment"}), ConfPath: conf},
			want:   "environment",
		},
		{
			name:   "mode-scoped key beats plain key",
			params: GetParams{Key: "role.implementer.runtime", Mode: "refactor", ConfPath: conf, LookupEnv: noEnv},
			want:   "mode-runtime",
		},
		{
			name:   "mode scope does not apply to a non-role key",
			params: GetParams{Key: "plain.knob", Mode: "refactor", ConfPath: conf, LookupEnv: noEnv},
			want:   "plain-value",
		},
		{
			name:   "committed key when no mode",
			params: GetParams{Key: "role.implementer.runtime", ConfPath: conf, LookupEnv: noEnv},
			want:   "base-runtime",
		},
		{
			name:   "default when absent everywhere",
			params: GetParams{Key: "absent.knob", Default: "built-in", DefaultSet: true, ConfPath: conf, LookupEnv: noEnv},
			want:   "built-in",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, code, err := Get(c.params)
			if code != c.code {
				t.Fatalf("code = %d, want %d (err %v)", code, c.code, err)
			}
			if got != c.want {
				t.Fatalf("value = %q, want %q", got, c.want)
			}
		})
	}
}

// TestGetLocalPrecedence proves the .local override wins over the committed file
// and loses to the environment.
func TestGetLocalPrecedence(t *testing.T) {
	dir := t.TempDir()
	conf := filepath.Join(dir, "metasystem.conf")
	putFile(t, conf, "plain.knob=plain-value\n")
	putFile(t, conf+".local", "plain.knob=local-value\n")

	got, _, _ := Get(GetParams{Key: "plain.knob", ConfPath: conf, LookupEnv: noEnv})
	if got != "local-value" {
		t.Fatalf("local override: got %q, want local-value", got)
	}
	got, _, _ = Get(GetParams{Key: "plain.knob", ConfPath: conf, LookupEnv: mapEnv(map[string]string{"METASYSTEM_PLAIN_KNOB": "environment"})})
	if got != "environment" {
		t.Fatalf("environment over local: got %q, want environment", got)
	}
}

func TestGetErrors(t *testing.T) {
	dir := t.TempDir()
	conf := filepath.Join(dir, "metasystem.conf")
	putFile(t, conf, "dup.key=one\ndup.key=two\nok.key=value\n")

	// A missing value with no default is exit 1.
	if _, code, err := Get(GetParams{Key: "absent.key", ConfPath: conf, LookupEnv: noEnv}); code != 1 || err == nil {
		t.Fatalf("missing value: code=%d err=%v, want code 1 and error", code, err)
	}
	// An invalid key is exit 2.
	if _, code, _ := Get(GetParams{Key: "Bad_Key", ConfPath: conf, LookupEnv: noEnv}); code != 2 {
		t.Fatalf("invalid key: code=%d, want 2", code)
	}
	// An invalid mode is exit 2.
	if _, code, _ := Get(GetParams{Key: "ok.key", Mode: "Bad", ConfPath: conf, LookupEnv: noEnv}); code != 2 {
		t.Fatalf("invalid mode: code=%d, want 2", code)
	}
	// A duplicate in the committed file propagates as exit 1.
	if _, code, err := Get(GetParams{Key: "dup.key", ConfPath: conf, LookupEnv: noEnv}); code != 1 || err == nil {
		t.Fatalf("duplicate key: code=%d err=%v, want code 1 and error", code, err)
	}
}

func TestConfLookup(t *testing.T) {
	dir := t.TempDir()
	conf := filepath.Join(dir, "conf")
	putFile(t, conf, "# comment\n\nsingle.key = value \nnoequals\ndup.key=a\ndup.key=b\n")

	if v, found, err := ConfLookup(conf, "single.key"); !found || err != nil || v != "value" {
		t.Fatalf("single: v=%q found=%v err=%v", v, found, err)
	}
	if _, found, err := ConfLookup(conf, "missing.key"); found || err != nil {
		t.Fatalf("absent must be found=false err=nil, got found=%v err=%v", found, err)
	}
	if _, _, err := ConfLookup(conf, "dup.key"); err == nil {
		t.Fatal("duplicate key must error")
	}
	if _, _, err := ConfLookup(filepath.Join(dir, "nope"), "any"); err == nil {
		t.Fatal("unreadable file must error")
	}
}

func TestKeys(t *testing.T) {
	dir := t.TempDir()
	conf := filepath.Join(dir, "metasystem.conf")
	putFile(t, conf, "model.tier.1=a\nmodel.tier.2=b\nother.key=x\n# model.tier.comment\n")
	putFile(t, conf+".local", "model.tier.2=b-local\nmodel.tier.3=c\n")

	// Base first, then .local (deduped), then env-only numeric members. The
	// environment carries model.tier.4 (a real key) and a non-numeric member
	// that must be ignored.
	env := []string{
		"METASYSTEM_MODEL_TIER_4=d",
		"METASYSTEM_MODEL_TIER_X=skip",
		"UNRELATED=1",
	}
	got := Keys(conf, "model.tier.", env)
	want := []string{"model.tier.1", "model.tier.2", "model.tier.3", "model.tier.4"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Keys = %v, want %v", got, want)
	}

	// A prefix matching nothing yields no keys.
	if got := Keys(conf, "nope.", nil); len(got) != 0 {
		t.Fatalf("unmatched prefix: got %v, want empty", got)
	}
}

func TestEnvName(t *testing.T) {
	cases := map[string]string{
		"refactor.max-age-minutes": "METASYSTEM_REFACTOR_MAX_AGE_MINUTES",
		"role.implementer.runtime": "METASYSTEM_ROLE_IMPLEMENTER_RUNTIME",
		"model.tier.":              "METASYSTEM_MODEL_TIER_",
		"plain.knob":               "METASYSTEM_PLAIN_KNOB",
	}
	for in, want := range cases {
		if got := EnvName(in); got != want {
			t.Errorf("EnvName(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestKeyOrigin pins the origin vocabulary — env, conf-local, conf,
// default — to the resolver's own precedence, including the mode-scoped
// hit the retired shell shadow probe could not see.
func TestKeyOrigin(t *testing.T) {
	dir := t.TempDir()
	conf := filepath.Join(dir, "metasystem.conf")
	if err := os.WriteFile(conf, []byte("dispatch.cap-min=45\nmode.deep.role.implementer.runtime=codex\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	origin := func(key, mode string) string {
		t.Helper()
		got, err := KeyOrigin(GetParams{Key: key, Mode: mode, ConfPath: conf})
		if err != nil {
			t.Fatal(err)
		}
		return got
	}
	if got := origin("dispatch.cap-min", ""); got != "conf" {
		t.Fatalf("committed key origin = %q", got)
	}
	if got := origin("cap.min.codex.gpt-5.6", ""); got != "default" {
		t.Fatalf("absent key origin = %q", got)
	}
	if got := origin("role.implementer.runtime", "deep"); got != "conf" {
		t.Fatalf("mode-scoped key origin = %q", got)
	}
	if got := origin("role.implementer.runtime", ""); got != "default" {
		t.Fatalf("mode-scoped key without mode = %q", got)
	}

	if err := os.WriteFile(conf+".local", []byte("dispatch.cap-min=99\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := origin("dispatch.cap-min", ""); got != "conf-local" {
		t.Fatalf("local override origin = %q", got)
	}

	t.Setenv("METASYSTEM_DISPATCH_CAP_MIN", "5")
	if got := origin("dispatch.cap-min", ""); got != "env" {
		t.Fatalf("environment origin = %q", got)
	}

	if _, err := KeyOrigin(GetParams{Key: "Bad Key", ConfPath: conf}); err == nil {
		t.Fatal("an invalid key must refuse")
	}
	if _, err := KeyOrigin(GetParams{Key: "dispatch.cap-min", Mode: "Bad Mode", ConfPath: conf}); err == nil {
		t.Fatal("an invalid mode must refuse")
	}
	dup := filepath.Join(dir, "dup.conf")
	if err := os.WriteFile(dup, []byte("k.a=1\nk.a=2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := KeyOrigin(GetParams{Key: "k.a", ConfPath: dup}); err == nil {
		t.Fatal("a duplicate key must refuse, not pick a winner")
	}
}

// The local overlay carries mode-scoped role keys too (R-25 lanes by
// hand): a mode key in .local outranks everything below the environment,
// and a .local base key still outranks the committed mode key. Found
// live: a design brief dispatched on the implementation lane because the
// overlay's mode keys were skipped.
func TestGetLocalOverlayModeScopedKeys(t *testing.T) {
	dir := t.TempDir()
	conf := filepath.Join(dir, "metasystem.conf")
	putFile(t, conf, "role.implementer.runtime=base-runtime\n"+
		"mode.design.role.implementer.runtime=committed-mode-runtime\n")
	putFile(t, conf+".local", "mode.design.role.implementer.runtime=local-mode-runtime\n")

	value, code, err := Get(GetParams{Key: "role.implementer.runtime", Mode: "design", ConfPath: conf, LookupEnv: noEnv})
	if err != nil || code != 0 || value != "local-mode-runtime" {
		t.Fatalf("local mode key must win: %q %d %v", value, code, err)
	}

	// A .local base key outranks the committed mode key (locality outranks
	// specificity across files; specificity wins within a file).
	putFile(t, conf+".local", "role.implementer.runtime=local-base-runtime\n")
	value, code, err = Get(GetParams{Key: "role.implementer.runtime", Mode: "design", ConfPath: conf, LookupEnv: noEnv})
	if err != nil || code != 0 || value != "local-base-runtime" {
		t.Fatalf("local base key must outrank the committed mode key: %q %d %v", value, code, err)
	}

	// Without any .local, the committed mode key still beats the base.
	if err := os.Remove(conf + ".local"); err != nil {
		t.Fatal(err)
	}
	value, code, err = Get(GetParams{Key: "role.implementer.runtime", Mode: "design", ConfPath: conf, LookupEnv: noEnv})
	if err != nil || code != 0 || value != "committed-mode-runtime" {
		t.Fatalf("committed mode key must still beat base: %q %d %v", value, code, err)
	}
}
