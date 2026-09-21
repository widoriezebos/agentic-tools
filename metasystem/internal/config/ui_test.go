package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
)

func TestUIListenResolution(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		committed string
		flag      string
		flagSet   bool
		want      string
	}{
		{name: "default", want: DefaultUIListen},
		{name: "flag over committed value", committed: "127.0.0.1:9000", flag: "localhost:not-validated-here", flagSet: true, want: "localhost:not-validated-here"},
		{name: "empty flag still wins", committed: "127.0.0.1:9000", flagSet: true, want: ""},
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

			got, err := UIListen(confPath, tc.flag, tc.flagSet)
			testutil.Require(t, "resolve UI listen address", err, nil)
			testutil.Expect(t, "resolved UI listen address", got, tc.want)
		})
	}
}
