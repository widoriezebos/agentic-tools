package config

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
)

// Every case resolves through the injected environment lookup, so no case
// depends on the ambient environment and none has to set one.
func TestUISubjectResolution(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		committed string
		lookup    func(string) (string, bool)
		want      string
	}{
		{name: "no value configured resolves empty", want: ""},
		{name: "the committed value", committed: "Ledger", want: "Ledger"},
		{
			name:      "environment over committed value",
			committed: "Ledger",
			lookup:    mapEnv(map[string]string{EnvName(UISubjectKey): "Ledger staging"}),
			want:      "Ledger staging",
		},
		{
			name:      "a set but empty environment value still wins",
			committed: "Ledger",
			lookup:    mapEnv(map[string]string{EnvName(UISubjectKey): ""}),
			want:      "",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			confPath := filepath.Join(t.TempDir(), "metasystem.conf")
			body := ""
			if tc.committed != "" {
				body = UISubjectKey + "=" + tc.committed + "\n"
			}
			err := os.WriteFile(confPath, []byte(body), 0o644)
			testutil.Require(t, "write configuration", err, nil)
			lookup := tc.lookup
			if lookup == nil {
				lookup = noEnv
			}

			got, err := uiSubject(confPath, lookup)

			testutil.Require(t, "resolve the workspace subject", err, nil)
			testutil.Expect(t, "resolved workspace subject", got, tc.want)
		})
	}
}

// A duplicate key is a malformed source, and resolution says so rather than
// picking a winner.
func TestUISubjectRefusesADuplicateKey(t *testing.T) {
	t.Parallel()

	confPath := filepath.Join(t.TempDir(), "metasystem.conf")
	body := UISubjectKey + "=one\n" + UISubjectKey + "=two\n"
	testutil.Require(t, "write configuration", os.WriteFile(confPath, []byte(body), 0o644), nil)

	_, err := uiSubject(confPath, noEnv)

	testutil.Require(t, "resolution refuses", err != nil, true)
	testutil.ExpectMatch(t, "the refusal names the key", err.Error(), regexp.MustCompile(regexp.QuoteMeta(UISubjectKey)))
}
