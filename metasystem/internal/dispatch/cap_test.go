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
			capMin, rule, origin, err := ResolveCap(c.conf, c.role, "codex", "gpt-5.6", c.requested)
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
		capMin, rule, origin, err := ResolveCap(general, "implementer", "codex", "gpt-5.6", "")
		if err != nil || capMin != 45 || rule != "config-general" || origin != "conf" {
			t.Fatalf("got (%d,%s,%s,%v)", capMin, rule, origin, err)
		}
	})
}

func TestResolveCapEnforcesDispatchCapMaxForEverySource(t *testing.T) {
	for _, test := range []struct {
		name      string
		lines     []string
		role      string
		requested string
	}{
		{name: "explicit argument", lines: []string{"dispatch.cap-max=120"}, role: "implementer", requested: "121"},
		{name: "role pair", lines: []string{"dispatch.cap-max=120", "cap.min.implementer.codex.gpt-5.6=121"}, role: "implementer"},
		{name: "runtime model pair", lines: []string{"dispatch.cap-max=120", "cap.min.codex.gpt-5.6=121"}, role: "verifier"},
		{name: "general", lines: []string{"dispatch.cap-max=120", "dispatch.cap-min=121"}, role: "verifier"},
	} {
		t.Run(test.name, func(t *testing.T) {
			conf := writeConf(t, test.lines...)
			_, _, _, err := ResolveCap(conf, test.role, "codex", "gpt-5.6", test.requested)
			if err == nil || !strings.Contains(err.Error(), "dispatch.cap-max=120") {
				t.Fatalf("cap above the maximum from %s was not refused naming the key: %v", test.name, err)
			}
		})
	}
	conf := writeConf(t, "dispatch.cap-max=120")
	if capMin, _, _, err := ResolveCap(conf, "implementer", "codex", "gpt-5.6", "120"); err != nil || capMin != 120 {
		t.Fatalf("cap equal to the maximum was refused: cap=%d err=%v", capMin, err)
	}
	raised := writeConf(t, "dispatch.cap-max=200")
	if capMin, _, _, err := ResolveCap(raised, "implementer", "codex", "gpt-5.6", "150"); err != nil || capMin != 150 {
		t.Fatalf("configured maximum did not control admission: cap=%d err=%v", capMin, err)
	}
}

func TestSTR3MissionCapBypass07GoalBoundMissionAboveCeiling(t *testing.T) {
	t.Skip("STR3-MISSION-CAP-BYPASS-07: a goal-bound mission cap above 120 minutes cannot be proved here because mission fence enforcement is explicitly outside part one")
}

func TestRefuseUnsignedMissionCap(t *testing.T) {
	conf := writeConf(t, "cap.min.codex.gpt-5.6=60")
	if err := RefuseUnsignedMissionCap(conf, "implementer", "codex", "gpt-5.6"); err != nil {
		t.Fatalf("a committed cap key is signed surface: %v", err)
	}
	t.Run("env origin refuses by name", func(t *testing.T) {
		t.Setenv("METASYSTEM_CAP_MIN_CODEX_GPT_5_6", "999")
		err := RefuseUnsignedMissionCap(conf, "implementer", "codex", "gpt-5.6")
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
		err := RefuseUnsignedMissionCap(local, "implementer", "codex", "gpt-5.6")
		want := "mission dispatch refused: the mission fence is cap authority; unsigned conf-local key cap.min.implementer.codex.gpt-5.6 cannot set a mission cap"
		if err == nil || err.Error() != want {
			t.Fatalf("want %q, got %v", want, err)
		}
	})
}

func writeSibling(t *testing.T, confPath, line string) error {
	t.Helper()
	return os.WriteFile(confPath+".local", []byte(line+"\n"), 0o644)
}
