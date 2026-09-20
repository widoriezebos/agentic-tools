package launch

import (
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
	for _, test := range []struct{ key, value string }{{WaitCapKey, "nope"}, {BuildModelKey, ""}, {BuildEffortKey, "   "}, {SeatWindowKey, "-1"}, {ReadWindowKey, "nope"}} {
		conf := filepath.Join(t.TempDir(), "metasystem.conf")
		if err := os.WriteFile(conf, []byte(test.key+"="+test.value+"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		_, err := ResolveSettings(conf, nil)
		if err == nil || !strings.HasPrefix(err.Error(), "LAUNCH_SETTING_INVALID key="+test.key) {
			t.Fatalf("%s error=%v", test.key, err)
		}
	}
}

func TestTrackedConfCarriesEveryLaunchKey(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "metasystem.conf"))
	if err != nil {
		t.Fatal(err)
	}
	for _, setting := range settingDefaults {
		if !strings.Contains(string(data), setting.Key+"="+setting.Value+"\n") {
			t.Errorf("missing %s", setting.Key)
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
	shipped, err := LoadShippedSeatWindow(root, settings.SeatWindow)
	if err != nil {
		t.Fatal(err)
	}
	if shipped.Tokens != settings.SeatWindow {
		t.Fatalf("shipped autoCompactWindow=%d differs from %s=%d", shipped.Tokens, SeatWindowKey, settings.SeatWindow)
	}
}
