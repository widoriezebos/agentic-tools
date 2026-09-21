package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
)

// O5: every case resolves through the injected environment lookup, so no case
// depends on the ambient environment and none has to set one.
func TestUIListenResolution(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		committed string
		flag      string
		flagSet   bool
		lookup    func(string) (string, bool)
		want      string
	}{
		{name: "default", want: DefaultUIListen},
		{name: "flag over committed value", committed: "127.0.0.1:9000", flag: "localhost:not-validated-here", flagSet: true, want: "localhost:not-validated-here"},
		{name: "empty flag still wins", committed: "127.0.0.1:9000", flagSet: true, want: ""},
		{
			name:      "environment over committed value",
			committed: "127.0.0.1:9000",
			lookup:    mapEnv(map[string]string{EnvName(UIListenKey): "127.0.0.1:9100"}),
			want:      "127.0.0.1:9100",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			confPath := filepath.Join(t.TempDir(), "metasystem.conf")
			body := ""
			if tc.committed != "" {
				body = UIListenKey + "=" + tc.committed + "\n"
			}
			err := os.WriteFile(confPath, []byte(body), 0o644)
			testutil.Require(t, "write configuration", err, nil)
			lookup := tc.lookup
			if lookup == nil {
				lookup = noEnv
			}

			got, err := uiListen(confPath, tc.flag, tc.flagSet, lookup)
			testutil.Require(t, "resolve UI listen address", err, nil)
			testutil.Expect(t, "resolved UI listen address", got, tc.want)
		})
	}
}
