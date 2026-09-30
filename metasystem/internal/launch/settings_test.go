package launch

import (
	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSettingsResolveValueAndSource(t *testing.T) {
	dir := t.TempDir()
	conf := filepath.Join(dir, "metasystem.conf")
	if err := os.WriteFile(conf, []byte(BuildWindowKey+"=210000\n"+DesignModelKey+"=from-conf\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(conf+".local", []byte(DesignModelKey+"=from-local\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	settings, err := ResolveSettings(conf, func(name string) (string, bool) {
		if name == "METASYSTEM_LAUNCH_BUILD_WINDOW_TOKENS" {
			return "220000", true
		}
		return "", false
	})
	if err != nil {
		t.Fatal(err)
	}
	if settings.BuildWindow != 220000 || settings.DesignModel != "from-local" || settings.ReadWindow != 0 {
		t.Fatalf("settings=%+v", settings)
	}
	sources := map[string]string{}
	for _, value := range settings.Values {
		sources[value.Key] = value.Source
	}
	if sources[BuildWindowKey] != "env" || sources[DesignModelKey] != "conf-local" || sources[ReadWindowKey] != "default" {
		t.Fatalf("sources=%v", sources)
	}
}

func TestSettingsRefuseAMalformedValue(t *testing.T) {
	// A window of 0 is the valid way to say "impose no cap"; a negative one is a
	// bug upstream and stays a refusal at the settings layer, not only at the adapter.
	// An empty lane model takes the lane runtime's own; with neither, the lane
	// has no model and is refused.
	for _, test := range []struct{ content, refused string }{
		{WaitCapKey + "=nope\n", WaitCapKey}, {BuildEffortKey + "=   \n", BuildEffortKey},
		{SeatWindowKey + "=-1\n", SeatWindowKey}, {ReadWindowKey + "=nope\n", ReadWindowKey},
		{BuildRuntimeKey + "=claude\n" + BuildModelKey + "=\n" + BuildModelKey + ".claude=\n", BuildModelKey},
	} {
		conf := filepath.Join(t.TempDir(), "metasystem.conf")
		if err := os.WriteFile(conf, []byte(test.content), 0o600); err != nil {
			t.Fatal(err)
		}
		_, err := ResolveSettings(conf, func(string) (string, bool) { return "", false })
		if err == nil || !strings.HasPrefix(ErrorDetail(err), "LAUNCH_SETTING_INVALID key="+test.refused) {
			t.Fatalf("%q error=%v", test.content, err)
		}
	}
}

// Every launch default is the compiled one (config.CompiledSettings) and the
// tracked conf, which holds overrides only, repeats none of them.
func TestLaunchDefaultsAreTheCompiledDefaults(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "metasystem.conf"))
	if err != nil {
		t.Fatal(err)
	}
	for _, setting := range settingDefaults {
		if compiled, ok := config.CompiledDefault(setting.Key); !ok || compiled != setting.Value {
			t.Errorf("%s default %q is not the compiled %q", setting.Key, setting.Value, compiled)
		}
		if strings.Contains(string(data), "\n"+setting.Key+"="+setting.Value+"\n") {
			t.Errorf("the tracked conf repeats the compiled default %s", setting.Key)
		}
	}
}

func TestShippedSeatWindowMatchesTrackedConf(t *testing.T) {
	t.Parallel()
	root := filepath.Join("..", "..")
	settings, err := ResolveSettings(filepath.Join(root, "metasystem.conf"), func(string) (string, bool) { return "", false })
	if err != nil {
		t.Fatal(err)
	}
	claude, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(ShippedClaudeSettingsSource)))
	if err != nil {
		t.Fatal(err)
	}
	shipped, err := LoadShippedSeatWindow(claude, settings.SeatWindow)
	if err != nil {
		t.Fatal(err)
	}
	if shipped.Tokens != settings.SeatWindow {
		t.Fatalf("shipped autoCompactWindow=%d differs from %s=%d", shipped.Tokens, SeatWindowKey, settings.SeatWindow)
	}
}

// A lane on auto runs on the first listed runtime on PATH and takes that
// runtime's own model; the runtime-independent key names one for every
// runtime; the sources say how each was chosen.
func TestLaneModelFollowsTheResolvedRuntime(t *testing.T) {
	t.Parallel()
	onPath := func(names ...string) func(string) (string, bool) {
		dir := t.TempDir()
		for _, name := range names {
			if err := testexec.WriteFile(filepath.Join(dir, name), []byte("#!/bin/sh\n"), 0o755); err != nil {
				t.Fatal(err)
			}
		}
		return func(key string) (string, bool) {
			if key == "PATH" {
				return dir, true
			}
			return "", false
		}
	}
	conf := filepath.Join(t.TempDir(), "metasystem.conf")
	if err := os.WriteFile(conf, []byte("# overrides only\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name                                 string
		lookup                               func(string) (string, bool)
		runtime, build, design, read, critic string
	}{
		{"devin alone", onPath("devin"), "devin", "claude-opus-5-5", "claude-opus-5-5", "gpt-6-astra", "gpt-6-astra"},
		{"codex before devin", onPath("devin", "codex"), "codex", "gpt-6-sol", "gpt-6-astra", "gpt-6-astra", "gpt-6-sol"},
		{"claude first", onPath("devin", "codex", "claude"), "claude", "claude-opus-5-5", "claude-fable-5-1", "claude-fable-5-1", "claude-opus-5-5"},
	} {
		settings, err := ResolveSettings(conf, test.lookup)
		if err != nil {
			t.Fatalf("%s: %v", test.name, err)
		}
		if settings.BuildRuntime != test.runtime || settings.ReadRuntime != test.runtime || settings.DesignRuntime != test.runtime || settings.CritiqueRuntime != test.runtime ||
			settings.BuildModel != test.build || settings.DesignModel != test.design || settings.ReadModel != test.read || settings.CritiqueModel != test.critic {
			t.Fatalf("%s: settings=%+v", test.name, settings)
		}
		sources := map[string]string{}
		for _, value := range settings.Values {
			sources[value.Key] = value.Source
		}
		if sources[ReadRuntimeKey] != "default; auto: first of claude,codex,devin on PATH" || sources[ReadModelKey] != "default via "+ReadModelKey+"."+test.runtime {
			t.Fatalf("%s: sources=%v", test.name, sources)
		}
	}
	if err := os.WriteFile(conf, []byte(ReadModelKey+"=pinned\n"+BuildRuntimeKey+"=devin\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	settings, err := ResolveSettings(conf, onPath("claude"))
	if err != nil || settings.ReadModel != "pinned" || settings.ReadRuntime != "claude" || settings.BuildRuntime != "devin" || settings.BuildModel != "claude-opus-5-5" {
		t.Fatalf("explicit settings: %+v err=%v", settings, err)
	}
}
