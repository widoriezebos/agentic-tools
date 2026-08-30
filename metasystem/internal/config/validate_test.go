package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// validateRepo prepares a repository whose registration checks are gated off
// (the template carries development/metasystem-design.md) with the given conf
// body and an evidence root outside the tree, and returns the problems.
func validateRepo(t *testing.T, confBody string) []string {
	t.Helper()
	repo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repo, "development"), 0o755); err != nil {
		t.Fatal(err)
	}
	putFile(t, filepath.Join(repo, "development", "metasystem-design.md"), "template\n")
	conf := filepath.Join(repo, "metasystem.conf")
	evidence := t.TempDir()
	body := strings.ReplaceAll(confBody, "@EVIDENCE@", evidence)
	body = strings.ReplaceAll(body, "@REPO@", repo)
	putFile(t, conf, body)
	tiersAbsent, problems, err := Validate(conf, repo)
	if err != nil {
		t.Fatalf("hard error: %v", err)
	}
	_ = tiersAbsent
	return problems
}

func hasProblem(problems []string, substr string) bool {
	for _, p := range problems {
		if strings.Contains(p, substr) {
			return true
		}
	}
	return false
}

const validConf = "metasystem.version=1\n" +
	"metasystem.runtimes=claude,codex,fake\n" +
	"runtime.claude.maximal-models=claude-fable-5\n" +
	"evidence.root=@EVIDENCE@\n" +
	"role.default.runtime=fake\n" +
	"role.default.model.fake=fake-model\n" +
	"model.tier.1=fake:fake-model\n"

func TestValidateAccepts(t *testing.T) {
	if problems := validateRepo(t, validConf); len(problems) != 0 {
		t.Fatalf("valid configuration rejected: %v", problems)
	}
}

func TestValidateTiersAbsentInfo(t *testing.T) {
	repo := t.TempDir()
	os.MkdirAll(filepath.Join(repo, "development"), 0o755)
	putFile(t, filepath.Join(repo, "development", "metasystem-design.md"), "x\n")
	evidence := t.TempDir()
	conf := filepath.Join(repo, "metasystem.conf")
	putFile(t, conf, "metasystem.runtimes=fake\n"+
		"evidence.root="+evidence+"\n"+
		"role.default.runtime=fake\n"+
		"role.default.model.fake=fake-model\n")
	tiersAbsent, problems, err := Validate(conf, repo)
	if err != nil || len(problems) != 0 {
		t.Fatalf("unexpected: err=%v problems=%v", err, problems)
	}
	if !tiersAbsent {
		t.Fatal("expected tiersAbsent when no model.tier.* is configured")
	}
}

func TestValidateRejections(t *testing.T) {
	cases := []struct {
		name   string
		conf   string
		expect string
	}{
		{
			name:   "duplicate key",
			conf:   validConf + "metasystem.version=2\n",
			expect: "duplicate key metasystem.version",
		},
		{
			name:   "malformed line",
			conf:   validConf + "no-equals-here\n",
			expect: "expected key=value",
		},
		{
			name:   "maximal mapping for unsupported runtime",
			conf:   validConf + "runtime.ghost.maximal-models=ghost-model\n",
			expect: "runtime.ghost.maximal-models names unsupported runtime 'ghost'",
		},
		{
			name:   "empty maximal mapping member",
			conf:   strings.Replace(validConf, "runtime.claude.maximal-models=claude-fable-5", "runtime.claude.maximal-models=claude-fable-5,", 1),
			expect: "runtime.claude.maximal-models must contain only non-empty comma-separated model names",
		},
		{
			name:   "duplicate maximal mapping member",
			conf:   strings.Replace(validConf, "runtime.claude.maximal-models=claude-fable-5", "runtime.claude.maximal-models=claude-fable-5,claude-fable-5", 1),
			expect: "runtime.claude.maximal-models contains duplicate model 'claude-fable-5'",
		},
		{
			name:   "unsupported runtime",
			conf:   "metasystem.runtimes=fake,ghost\nevidence.root=@EVIDENCE@\nrole.default.runtime=fake\nrole.default.model.fake=fake-model\nmodel.tier.1=fake:fake-model\n",
			expect: "names unsupported runtime 'ghost'",
		},
		{
			name:   "role runtime outside roster",
			conf:   validConf + "role.design-critic.runtime=ghost\n",
			expect: "outside metasystem.runtimes",
		},
		{
			name:   "mode-scoped key unsupported",
			conf:   validConf + "mode.refactor.unsupported=x\n",
			expect: "is not a supported mode-scoped key",
		},
		{
			name:   "non-canonical cap key",
			conf:   "metasystem.runtimes=devin,fake\nevidence.root=@EVIDENCE@\nrole.default.runtime=fake\nrole.default.model.fake=fake-model\ncap.min.devin.swe-1.7=5\n",
			expect: "non-canonical cap key cap.min.devin.swe-1.7; use cap.min.devin.swe-1-7",
		},
		{
			name:   "cap not a positive integer",
			conf:   validConf + "cap.min.fake.fake-model=0\n",
			expect: "cap.min.fake.fake-model must be a positive integer",
		},
		{
			name:   "tier member not runtime-qualified",
			conf:   "metasystem.runtimes=fake\nevidence.root=@EVIDENCE@\nrole.default.runtime=fake\nrole.default.model.fake=fake-model\nmodel.tier.1=fake-model\n",
			expect: "not runtime-qualified",
		},
		{
			name:   "model in zero tiers",
			conf:   "metasystem.runtimes=fake\nevidence.root=@EVIDENCE@\nrole.default.runtime=fake\nrole.default.model.fake=fake-model\nmodel.tier.1=\n",
			expect: "appears in 0 model tiers",
		},
		{
			name:   "model in two tiers",
			conf:   "metasystem.runtimes=fake\nevidence.root=@EVIDENCE@\nrole.default.runtime=fake\nrole.default.model.fake=fake-model\nmodel.tier.1=fake:fake-model\nmodel.tier.2=fake:fake-model\n",
			expect: "appears in 2 model tiers",
		},
		{
			name:   "missing runtime model",
			conf:   "metasystem.runtimes=fake\nevidence.root=@EVIDENCE@\nrole.default.runtime=fake\n",
			expect: "has no model.fake value",
		},
		{
			name:   "malformed tier key",
			conf:   "metasystem.runtimes=fake\nevidence.root=@EVIDENCE@\nrole.default.runtime=fake\nrole.default.model.fake=fake-model\nmodel.tier.one=fake:fake-model\n",
			expect: "is not a supported model tier key",
		},
		{
			name:   "tier gap",
			conf:   "metasystem.runtimes=fake\nevidence.root=@EVIDENCE@\nrole.default.runtime=fake\nrole.default.model.fake=fake-model\nmodel.tier.1=fake:fake-model\nmodel.tier.3=fake:fake-model\n",
			expect: "must be numbered contiguously from 1",
		},
		{
			name:   "evidence required",
			conf:   "metasystem.runtimes=fake\nrole.default.runtime=fake\nrole.default.model.fake=fake-model\n",
			expect: "evidence.root is required",
		},
		{
			name:   "evidence must be absolute",
			conf:   "metasystem.runtimes=fake\nevidence.root=relative/dir\nrole.default.runtime=fake\nrole.default.model.fake=fake-model\n",
			expect: "evidence.root must be absolute",
		},
		{
			name:   "evidence inside repository",
			conf:   "metasystem.runtimes=fake\nevidence.root=@REPO@/evidence\nrole.default.runtime=fake\nrole.default.model.fake=fake-model\n",
			expect: "evidence.root must be outside the repository",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			problems := validateRepo(t, c.conf)
			if !hasProblem(problems, c.expect) {
				t.Fatalf("expected a problem containing %q; got %v", c.expect, problems)
			}
		})
	}
}

// TestValidateRegistrationMissing exercises the adopted-repository check: with
// the template marker absent, a rostered runtime whose registration directory
// is missing is reported.
func TestValidateRegistrationMissing(t *testing.T) {
	repo := t.TempDir()
	evidence := t.TempDir()
	conf := filepath.Join(repo, "metasystem.conf")
	putFile(t, conf, "metasystem.runtimes=claude,fake\n"+
		"evidence.root="+evidence+"\n"+
		"role.default.runtime=fake\n"+
		"role.default.model.fake=fake-model\n")
	_, problems, err := Validate(conf, repo)
	if err != nil {
		t.Fatal(err)
	}
	if !hasProblem(problems, "registration directory .claude/skills is missing") {
		t.Fatalf("expected missing-registration problem; got %v", problems)
	}
}

// TestValidateUnreadable reports a hard error when the committed file cannot be
// read.
func TestValidateUnreadable(t *testing.T) {
	if _, _, err := Validate(filepath.Join(t.TempDir(), "absent.conf"), t.TempDir()); err == nil {
		t.Fatal("expected a hard error for an unreadable configuration")
	}
}

// foundations-9: numeric knobs are soft-defaulted at read time, so their
// typos must surface HERE — including the units suffix an operator most
// plausibly types.
func TestValidateNumericKnobs(t *testing.T) {
	for _, tc := range []struct {
		name   string
		line   string
		expect string
	}{
		{"unit suffix", "exec.local-timeout-sec=300s\n", "exec.local-timeout-sec must be a positive integer"},
		{"zero bound", "exec.network-timeout-sec=0\n", "exec.network-timeout-sec must be a positive integer"},
		{"negative interval", "watch.interval-sec=-5\n", "watch.interval-sec must be a positive integer"},
		{"nonsense stale", "watch.stale-min=soon\n", "watch.stale-min must be a positive integer"},
		{"share over 100", "census.max-interval-share-percent=150\n", "census.max-interval-share-percent must be an integer between 1 and 100"},
		{"negative elapsed grace", "metasystem.budget.elapsed-grace-percent=-1\n", "metasystem.budget.elapsed-grace-percent must be an integer between 0 and 200"},
		{"nonnumeric elapsed grace", "metasystem.budget.elapsed-grace-percent=half\n", "metasystem.budget.elapsed-grace-percent must be an integer between 0 and 200"},
		{"elapsed grace over 200", "metasystem.budget.elapsed-grace-percent=201\n", "metasystem.budget.elapsed-grace-percent must be an integer between 0 and 200"},
		{"zero slice norm", "metasystem.budget.slice-norm-hours=0\n", "metasystem.budget.slice-norm-hours must be a positive integer"},
		{"nonnumeric slice norm", "metasystem.budget.slice-norm-hours=four\n", "metasystem.budget.slice-norm-hours must be a positive integer"},
	} {
		problems := validateRepo(t, validConf+tc.line)
		if !hasProblem(problems, tc.expect) {
			t.Fatalf("%s: expected %q in %v", tc.name, tc.expect, problems)
		}
	}
	// Valid knobs raise nothing.
	good := validConf + "exec.local-timeout-sec=120\nwatch.interval-sec=60\ncensus.max-interval-share-percent=50\nmetasystem.budget.elapsed-grace-percent=200\nmetasystem.budget.slice-norm-hours=4\n"
	if problems := validateRepo(t, good); len(problems) != 0 {
		t.Fatalf("valid knobs rejected: %v", problems)
	}
}

func TestValidateBudgetLawOverrideSources(t *testing.T) {
	clearBudgetLawEnvironment(t)
	fixtureConf := strings.Replace(validConf, "metasystem.runtimes=claude,codex,fake", "metasystem.runtimes=fake", 1)
	tests := []struct {
		name        string
		conf        string
		local       string
		environment map[string]string
		expect      []string
	}{
		{
			name:  "production local overrides",
			conf:  validConf,
			local: ElapsedGracePercentKey + "=75\n" + SliceNormHoursKey + "=6\n",
			expect: []string{
				ElapsedGracePercentKey + " accepts only committed root configuration",
				SliceNormHoursKey + " accepts only committed root configuration",
			},
		},
		{
			name: "production environment overrides",
			conf: validConf,
			environment: map[string]string{
				ElapsedGracePercentKey: "75",
				SliceNormHoursKey:      "6",
			},
			expect: []string{
				"environment source " + EnvName(ElapsedGracePercentKey) + " is refused",
				"environment source " + EnvName(SliceNormHoursKey) + " is refused",
			},
		},
		{
			name:  "malformed fixture local overrides",
			conf:  fixtureConf,
			local: ElapsedGracePercentKey + "=201\n" + SliceNormHoursKey + "=0\n",
			expect: []string{
				ElapsedGracePercentKey + " must be an integer between 0 and 200",
				SliceNormHoursKey + " must be a positive integer",
			},
		},
		{
			name: "malformed fixture environment overrides",
			conf: fixtureConf,
			environment: map[string]string{
				ElapsedGracePercentKey: "many",
				SliceNormHoursKey:      "none",
			},
			expect: []string{
				"environment source " + EnvName(ElapsedGracePercentKey),
				"environment source " + EnvName(SliceNormHoursKey),
			},
		},
		{
			name: "ambiguous fixture local overrides",
			conf: fixtureConf,
			local: ElapsedGracePercentKey + "=25\n" + ElapsedGracePercentKey + "=50\n" +
				SliceNormHoursKey + "=4\n" + SliceNormHoursKey + "=6\n",
			expect: []string{
				"duplicate metasystem configuration key: " + ElapsedGracePercentKey,
				"duplicate metasystem configuration key: " + SliceNormHoursKey,
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			repo := t.TempDir()
			if err := os.MkdirAll(filepath.Join(repo, "development"), 0o755); err != nil {
				t.Fatal(err)
			}
			putFile(t, filepath.Join(repo, "development", "metasystem-design.md"), "template\n")
			conf := filepath.Join(repo, "metasystem.conf")
			body := strings.ReplaceAll(test.conf, "@EVIDENCE@", t.TempDir())
			putFile(t, conf, body)
			if test.local != "" {
				putFile(t, conf+".local", test.local)
			}
			for key, value := range test.environment {
				t.Setenv(EnvName(key), value)
			}

			_, problems, err := Validate(conf, repo)
			if err != nil {
				t.Fatal(err)
			}
			for _, expected := range test.expect {
				if !hasProblem(problems, expected) {
					t.Fatalf("expected %q in %v", expected, problems)
				}
			}
		})
	}
}
