package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStewardRequestSettingsRequirePositiveIntegers(t *testing.T) {
	t.Parallel()
	for _, key := range []string{"steward.request-gap-sec", "steward.request-cycle-max"} {
		for _, value := range []string{"0", "-1", "no", "15"} {
			t.Run(key+"/"+value, func(t *testing.T) {
				t.Parallel()
				root := t.TempDir()
				path := filepath.Join(root, "metasystem.conf")
				if err := os.WriteFile(path, []byte(key+"="+value+"\n"), 0600); err != nil {
					t.Fatal(err)
				}
				_, problems, err := Validate(path, root)
				if err != nil {
					t.Fatal(err)
				}
				rejected := false
				for _, problem := range problems {
					if strings.Contains(problem, key) {
						rejected = true
					}
				}
				if rejected != (value != "15") {
					t.Fatalf("key %s value %s: %v", key, value, problems)
				}
			})
		}
	}
}
