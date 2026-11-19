package config

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestUnitCheckKindAndBudgetSettings(t *testing.T) {
	t.Parallel()
	for _, row := range []struct{ key, fallback string }{{"proof.check-kind", "legacy"}, {"proof.check-budget", "6"}} {
		conf := filepath.Join(t.TempDir(), "metasystem.conf")
		putFile(t, conf, "# old tree\n")
		params := GetParams{Key: row.key, ConfPath: conf, FlagSet: true, Flag: "ignored", DefaultSet: true, Default: "ignored",
			LookupEnv: func(string) (string, bool) {
				t.Fatal("committed check settings consulted environment overrides")
				return "", false
			}}
		value, code, err := Get(params)
		if err != nil || code != 0 || value != row.fallback || !CommittedOnly(row.key) {
			t.Fatalf("%s default: %q, %d, %v", row.key, value, code, err)
		}
	}
	for _, row := range []struct {
		key, value string
		valid      bool
	}{
		{"proof.check-kind", "legacy", true}, {"proof.check-kind", "bounded", true},
		{"proof.check-kind", "", false}, {"proof.check-kind", "other", false},
		{"proof.check-budget", "6", true}, {"proof.check-budget", "153722867", true},
		{"proof.check-budget", "153722868", false}, {"proof.check-budget", "0", false},
		{"proof.check-budget", "-1", false}, {"proof.check-budget", "0.5", false},
		{"proof.check-budget", "", false}, {"proof.check-budget", "bad", false},
	} {
		t.Run(row.key+"/"+row.value, func(t *testing.T) {
			t.Parallel()
			conf := filepath.Join(t.TempDir(), "metasystem.conf")
			body := "proof.cheap=true\nproof.audits=true\nproof.deadline=15\nproof.full=true\n" + row.key + "=" + row.value + "\n"
			putFile(t, conf, body)
			_, code, err := Get(GetParams{Key: row.key, ConfPath: conf})
			if (err == nil && code == 0) != row.valid {
				t.Fatalf("value accepted=%v, want %v", err == nil && code == 0, row.valid)
			}
			_, problems, err := Validate(conf, filepath.Dir(conf))
			invalid := false
			for _, problem := range problems {
				invalid = invalid || strings.HasPrefix(problem, row.key+" ")
			}
			if err != nil || invalid == row.valid {
				t.Fatalf("validation did not distinguish the check setting: %v", err)
			}
			if _, _, err := CommittedContentLookup(body+row.key+"="+row.value+"\n", row.key); err == nil {
				t.Fatal("duplicate check setting was accepted")
			}
		})
	}
}
