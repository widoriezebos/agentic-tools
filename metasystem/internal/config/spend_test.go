package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func clearSpendEnvironment(t *testing.T) {
	t.Helper()
	for _, key := range []string{
		SpendModeKey, SpendCurrencyKey, SpendCeilingDayTokensKey,
		SpendCeilingDayMoneyKey, SpendCeilingGoalTokensKey, SpendCeilingGoalMoneyKey,
	} {
		name := EnvName(key)
		previous, present := os.LookupEnv(name)
		_ = os.Unsetenv(name)
		t.Cleanup(func() {
			if present {
				_ = os.Setenv(name, previous)
			} else {
				_ = os.Unsetenv(name)
			}
		})
	}
}

func TestSpendCeilingDefaults(t *testing.T) {
	clearSpendEnvironment(t)
	conf := filepath.Join(t.TempDir(), "metasystem.conf")
	putFile(t, conf, "")
	settings, err := ReadSpendSettings(conf)
	if err != nil {
		t.Fatal(err)
	}
	if settings.Mode != "alert" || settings.Currency != "USD" ||
		settings.DayTokenCeiling != 250000000 || settings.DayMoneyCeiling != 750 ||
		settings.GoalTokenCeiling != 125000000 || settings.GoalMoneyCeiling != 300 {
		t.Fatalf("spend defaults changed: %#v", settings)
	}
	putFile(t, conf, "metasystem.runtimes=fake\n"+
		SpendCeilingDayTokensKey+"=100\n"+SpendCeilingDayMoneyKey+"=12.50\n"+
		SpendCeilingGoalTokensKey+"=50\n"+SpendCeilingGoalMoneyKey+"=6.25\n")
	settings, err = ReadSpendSettings(conf)
	if err != nil || settings.DayTokenCeiling != 100 || settings.DayMoneyCeiling != 12.5 ||
		settings.GoalTokenCeiling != 50 || settings.GoalMoneyCeiling != 6.25 {
		t.Fatalf("committed spend ceilings were not read: settings=%#v err=%v", settings, err)
	}
}

func TestValidateSpendKeys(t *testing.T) {
	clearSpendEnvironment(t)
	validSpend := validConf +
		SpendModeKey + "=alert\n" + SpendCurrencyKey + "=USD\n" +
		SpendCeilingDayTokensKey + "=250000000\n" + SpendCeilingDayMoneyKey + "=750\n" +
		SpendCeilingGoalTokensKey + "=125000000\n" + SpendCeilingGoalMoneyKey + "=300\n" +
		"spend.price.codex.gpt-5-6-sol.input=1.25\n"
	if problems := validateRepo(t, validSpend); len(problems) != 0 {
		t.Fatalf("valid spend configuration rejected: %v", problems)
	}

	for _, test := range []struct {
		name, line, want string
	}{
		{"enforce", SpendModeKey + "=enforce\n", "spend.mode=enforce is refused until step 2 lands on Wido's word (R-60-m1)"},
		{"other mode", SpendModeKey + "=off\n", "spend.mode must be alert"},
		{"currency", SpendCurrencyKey + "=usd\n", "spend.currency must be three uppercase letters"},
		{"token ceiling", SpendCeilingDayTokensKey + "=0\n", "spend.ceiling.day.tokens must be a positive integer"},
		{"money ceiling", SpendCeilingGoalMoneyKey + "=many\n", "spend.ceiling.goal.money must be a positive decimal"},
		{"price decimal", "spend.price.codex.gpt-5-6-sol.input=-1\n", "must be a non-negative decimal"},
		{"price model", "spend.price.codex.gpt--5.input=1\n", "non-canonical spend price key"},
		{"price runtime", "spend.price.ghost.gpt-5.input=1\n", "outside metasystem.runtimes"},
		{"price class", "spend.price.codex.gpt-5.total=1\n", "unsupported spend price key"},
	} {
		t.Run(test.name, func(t *testing.T) {
			problems := validateRepo(t, validConf+test.line)
			if !hasProblem(problems, test.want) {
				t.Fatalf("expected %q in %v", test.want, problems)
			}
		})
	}

	t.Run("committed root only", func(t *testing.T) {
		repo := t.TempDir()
		if err := os.MkdirAll(filepath.Join(repo, "development"), 0o755); err != nil {
			t.Fatal(err)
		}
		putFile(t, filepath.Join(repo, "development", "metasystem-design.md"), "template\n")
		conf := filepath.Join(repo, "metasystem.conf")
		putFile(t, conf, strings.ReplaceAll(validConf, "@EVIDENCE@", t.TempDir()))
		putFile(t, conf+".local", SpendCeilingDayTokensKey+"=1\nspend.price.codex.gpt-5-6-sol.input=1\n")
		_, problems, err := Validate(conf, repo)
		if err != nil || !hasProblem(problems, "committed root configuration") {
			t.Fatalf("local spend law was not refused: problems=%v err=%v", problems, err)
		}
		_ = os.Remove(conf + ".local")
		t.Setenv(EnvName(SpendCeilingDayTokensKey), "1")
		_, problems, err = Validate(conf, repo)
		if err != nil || !hasProblem(problems, "environment source "+EnvName(SpendCeilingDayTokensKey)+" is refused") {
			t.Fatalf("environment spend law was not refused: problems=%v err=%v", problems, err)
		}
	})
}
