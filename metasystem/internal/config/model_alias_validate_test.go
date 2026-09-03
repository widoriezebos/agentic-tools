package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func aliasValidationConf(lines ...string) string {
	base := strings.Replace(validConf,
		"runtime.claude.maximal-models=claude-fable-5",
		"runtime.claude.maximal-models=claude-fable-5-1", 1)
	return base + strings.Join(lines, "\n") + "\n"
}

func aliasValidationRepo(t *testing.T, body string) (repo, conf string) {
	t.Helper()
	repo = t.TempDir()
	if err := os.MkdirAll(filepath.Join(repo, "development"), 0o755); err != nil {
		t.Fatal(err)
	}
	putFile(t, filepath.Join(repo, "development", "metasystem-design.md"), "template\n")
	conf = filepath.Join(repo, "metasystem.conf")
	body = strings.ReplaceAll(body, "@EVIDENCE@", t.TempDir())
	body = strings.ReplaceAll(body, "@REPO@", repo)
	putFile(t, conf, body)
	return repo, conf
}

func validateAliasFiles(t *testing.T, repo, conf string) []string {
	t.Helper()
	_, problems, err := Validate(conf, repo)
	if err != nil {
		t.Fatal(err)
	}
	return problems
}

func TestValidateModelAliasRejections(t *testing.T) {
	cases := []struct {
		name, conf, expect string
	}{
		{"unrostered runtime", aliasValidationConf("runtime.devin.maximal-models=target", "runtime.devin.model-alias.source=target"), "outside metasystem.runtimes"},
		{"empty source", aliasValidationConf("runtime.claude.model-alias.=claude-fable-5-1"), "source must be non-empty"},
		{"empty target", aliasValidationConf("runtime.claude.model-alias.claude-fable-5="), "target must be non-empty"},
		{"noncanonical source", aliasValidationConf("runtime.claude.model-alias.claude.fable.5=claude-fable-5-1"), "source 'claude.fable.5' is non-canonical"},
		{"noncanonical target", aliasValidationConf("runtime.claude.model-alias.claude-fable-5=Claude Fable 5.1"), "target 'Claude Fable 5.1' is non-canonical"},
		{"self alias", aliasValidationConf("runtime.claude.model-alias.claude-fable-5-1=claude-fable-5-1"), "must not alias a model to itself"},
		{"chain", aliasValidationConf("runtime.claude.model-alias.family=claude-fable-5", "runtime.claude.model-alias.claude-fable-5=claude-fable-5-1"), "model-alias chains are not allowed"},
		{"source tracked maximal", strings.Replace(aliasValidationConf("runtime.claude.model-alias.claude-fable-5=claude-fable-5-1"), "runtime.claude.maximal-models=claude-fable-5-1", "runtime.claude.maximal-models=claude-fable-5-1,claude-fable-5", 1), "must be absent from tracked"},
		{"target absent from tracked maximal", strings.Replace(aliasValidationConf("runtime.claude.model-alias.claude-fable-5=claude-fable-5-1"), "runtime.claude.maximal-models=claude-fable-5-1", "runtime.claude.maximal-models=other", 1), "must be present in tracked runtime.claude.maximal-models"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			problems := validateRepo(t, tc.conf)
			if !hasProblem(problems, tc.expect) {
				t.Fatalf("want problem containing %q, got %v", tc.expect, problems)
			}
		})
	}
}

func TestFMA_R2_TargetAdmissionEnvShadow(t *testing.T) {
	body := aliasValidationConf("runtime.claude.model-alias.claude-fable-5=claude-fable-5-1")
	repo, conf := aliasValidationRepo(t, body)
	name := EnvName("runtime.claude.maximal-models")
	t.Setenv(name, "claude-fable-5")
	if problems := validateAliasFiles(t, repo, conf); !hasProblem(problems, name) || !hasProblem(problems, "omits model-alias target") {
		t.Fatalf("environment omission was not named: %v", problems)
	}
	t.Setenv(name, "claude-fable-5-1,claude-fable-5")
	if problems := validateAliasFiles(t, repo, conf); len(problems) != 0 {
		t.Fatalf("target plus draining source environment value rejected: %v", problems)
	}
}

func TestFMA_R2_ValidatorAllowancesUntested(t *testing.T) {
	t.Run("direct fan-in", func(t *testing.T) {
		body := aliasValidationConf(
			"runtime.claude.model-alias.claude-fable-5=claude-fable-5-1",
			"runtime.claude.model-alias.claude-fable-five=claude-fable-5-1")
		if problems := validateRepo(t, body); len(problems) != 0 {
			t.Fatalf("direct fan-in rejected: %v", problems)
		}
	})
	t.Run("local target plus draining source", func(t *testing.T) {
		body := aliasValidationConf("runtime.claude.model-alias.claude-fable-5=claude-fable-5-1")
		repo, conf := aliasValidationRepo(t, body)
		putFile(t, conf+".local", "runtime.claude.maximal-models=claude-fable-5-1,claude-fable-5\n")
		if problems := validateAliasFiles(t, repo, conf); len(problems) != 0 {
			t.Fatalf("draining local maximal-model overlay rejected: %v", problems)
		}
	})
}

func TestValidateModelAliasUncommittedSources(t *testing.T) {
	body := aliasValidationConf("runtime.claude.model-alias.claude-fable-5=claude-fable-5-1")
	t.Run("local maximal omission", func(t *testing.T) {
		repo, conf := aliasValidationRepo(t, body)
		putFile(t, conf+".local", "runtime.claude.maximal-models=claude-fable-5\n")
		if problems := validateAliasFiles(t, repo, conf); !hasProblem(problems, conf+".local") || !hasProblem(problems, "omits model-alias target") {
			t.Fatalf("local omission was not named: %v", problems)
		}
	})
	t.Run("local alias", func(t *testing.T) {
		repo, conf := aliasValidationRepo(t, body)
		putFile(t, conf+".local", "runtime.claude.model-alias.claude-family=claude-fable-5-1\n")
		if problems := validateAliasFiles(t, repo, conf); !hasProblem(problems, ".local source") {
			t.Fatalf("local alias was not refused: %v", problems)
		}
	})
	t.Run("environment alias", func(t *testing.T) {
		repo, conf := aliasValidationRepo(t, body)
		name := "METASYSTEM_RUNTIME_CLAUDE_MODEL_ALIAS_ENV_ONLY_SOURCE"
		t.Setenv(name, "claude-fable-5-1")
		if problems := validateAliasFiles(t, repo, conf); !hasProblem(problems, "environment source "+name) {
			t.Fatalf("environment alias was not refused: %v", problems)
		}
	})
}
