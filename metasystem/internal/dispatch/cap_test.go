package dispatch

import (
	"os"
	"strings"
	"testing"
)

func TestResolveCapChain(t *testing.T) {
	conf := writeConf(t,
		"cap.min.implementer.codex.gpt-5.6=90",
		"cap.min.codex.gpt-5.6=60",
		"dispatch.cap-min=45")
	bare := writeConf(t, "metasystem.runtimes=claude")

	cases := []struct {
		name       string
		conf       string
		role       string
		requested  string
		wantCap    int64
		wantRule   string
		wantOrigin string
		refusal    string
	}{
		{name: "explicit argument wins the chain", conf: conf, role: "implementer", requested: "30",
			wantCap: 30, wantRule: "argument", wantOrigin: "argument"},
		{name: "role pair key ranks first", conf: conf, role: "implementer",
			wantCap: 90, wantRule: "config-role-pair", wantOrigin: "conf"},
		{name: "pair key fills a role gap", conf: conf, role: "verifier",
			wantCap: 60, wantRule: "config-pair", wantOrigin: "conf"},
		{name: "built-in floor when nothing is configured", conf: bare, role: "verifier",
			wantCap: 120, wantRule: "built-in", wantOrigin: "default"},
		{name: "non-integer refuses", conf: conf, role: "implementer", requested: "ninety",
			refusal: "dispatch cap must be a positive integer"},
		{name: "zero refuses", conf: conf, role: "implementer", requested: "0",
			refusal: "dispatch cap must be a positive integer"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			capMin, rule, origin, err := ResolveCap(c.conf, c.role, "codex", "gpt-5.6", "", c.requested)
			if c.refusal != "" {
				if err == nil || !strings.Contains(err.Error(), c.refusal) {
					t.Fatalf("want refusal %q, got cap=%d err=%v", c.refusal, capMin, err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if capMin != c.wantCap || rule != c.wantRule || origin != c.wantOrigin {
				t.Fatalf("got (%d,%s,%s), want (%d,%s,%s)", capMin, rule, origin, c.wantCap, c.wantRule, c.wantOrigin)
			}
		})
	}

	t.Run("general key carries its origin", func(t *testing.T) {
		general := writeConf(t, "dispatch.cap-min=45")
		capMin, rule, origin, err := ResolveCap(general, "implementer", "codex", "gpt-5.6", "", "")
		if err != nil || capMin != 45 || rule != "config-general" || origin != "conf" {
			t.Fatalf("got (%d,%s,%s,%v)", capMin, rule, origin, err)
		}
	})
}

func TestResolveCapAliasOrder(t *testing.T) {
	sourceOnly := writeConf(t,
		"cap.min.implementer.claude.claude-fable-5=30",
		"dispatch.cap-min=120")
	capMin, rule, _, err := ResolveCap(sourceOnly, "implementer", "claude", "claude-fable-5-1", "claude-fable-5", "")
	if err != nil || capMin != 30 || rule != "config-role-pair-alias-source" {
		t.Fatalf("source role-pair = (%d, %s, %v), want (30, config-role-pair-alias-source, nil)", capMin, rule, err)
	}

	canonicalFirst := writeConf(t,
		"cap.min.claude.claude-fable-5-1=200",
		"cap.min.implementer.claude.claude-fable-5=30")
	capMin, rule, _, err = ResolveCap(canonicalFirst, "implementer", "claude", "claude-fable-5-1", "claude-fable-5", "")
	if err != nil || capMin != 200 || rule != "config-pair" {
		t.Fatalf("canonical pair = (%d, %s, %v), want (200, config-pair, nil)", capMin, rule, err)
	}

	capMin, rule, _, err = ResolveCap(sourceOnly, "implementer", "claude", "claude-fable-5-1", "", "")
	if err != nil || capMin != 120 || rule != "config-general" {
		t.Fatalf("no source = (%d, %s, %v), want (120, config-general, nil)", capMin, rule, err)
	}
}

func TestRefuseUnsignedMissionCap(t *testing.T) {
	conf := writeConf(t, "cap.min.codex.gpt-5.6=60")
	if err := RefuseUnsignedMissionCap(conf, "implementer", "codex", "gpt-5.6", ""); err != nil {
		t.Fatalf("a committed cap key is signed surface: %v", err)
	}
	t.Run("env origin refuses by name", func(t *testing.T) {
		t.Setenv("METASYSTEM_CAP_MIN_CODEX_GPT_5_6", "999")
		err := RefuseUnsignedMissionCap(conf, "implementer", "codex", "gpt-5.6", "")
		want := "mission dispatch refused: the mission fence is cap authority; unsigned env key cap.min.codex.gpt-5.6 cannot set a mission cap"
		if err == nil || err.Error() != want {
			t.Fatalf("want %q, got %v", want, err)
		}
	})
	t.Run("conf-local origin refuses by name", func(t *testing.T) {
		local := writeConf(t, "metasystem.runtimes=claude")
		if err := writeSibling(t, local, "cap.min.implementer.codex.gpt-5.6=200"); err != nil {
			t.Fatal(err)
		}
		err := RefuseUnsignedMissionCap(local, "implementer", "codex", "gpt-5.6", "")
		want := "mission dispatch refused: the mission fence is cap authority; unsigned conf-local key cap.min.implementer.codex.gpt-5.6 cannot set a mission cap"
		if err == nil || err.Error() != want {
			t.Fatalf("want %q, got %v", want, err)
		}
	})
}

func TestFMA_R2_MissionCapSourceBypassUnsigned(t *testing.T) {
	conf := writeConf(t, "metasystem.runtimes=claude")
	t.Setenv("METASYSTEM_CAP_MIN_CLAUDE_CLAUDE_FABLE_5", "30")
	err := RefuseUnsignedMissionCap(conf, "implementer", "claude", "claude-fable-5-1", "claude-fable-5")
	want := "unsigned env key cap.min.claude.claude-fable-5 cannot set a mission cap"
	if err == nil || !strings.Contains(err.Error(), want) {
		t.Fatalf("want refusal containing %q, got %v", want, err)
	}
}

func writeSibling(t *testing.T, confPath, line string) error {
	t.Helper()
	return os.WriteFile(confPath+".local", []byte(line+"\n"), 0o644)
}
