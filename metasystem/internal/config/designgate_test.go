package config

import (
	"testing"
)

func TestDesignGateMode(t *testing.T) {
	t.Parallel()
	value, code, err := Get(GetParams{Key: DesignGateModeKey, LookupEnv: func(string) (string, bool) { return "", false }})
	if err != nil || code != 0 || value != "warn" {
		t.Fatalf("default mode: %q code=%d err=%v", value, code, err)
	}
	for _, mode := range []string{"warn", "refuse", "invalid"} {
		t.Run(mode, func(t *testing.T) {
			t.Parallel()
			parsed, err := ParseDesignGateMode(mode)
			if mode == "invalid" {
				if err == nil || parsed != "warn" {
					t.Fatalf("invalid mode must report a problem and fall back to warn: %q %v", parsed, err)
				}
			} else if err != nil || parsed != mode {
				t.Fatalf("valid mode: %q %v", parsed, err)
			}
			for _, local := range []bool{false, true} {
				body, overlay := "metasystem.runtimes=fake\n", ""
				if local {
					overlay = DesignGateModeKey + "=" + mode + "\n"
				} else {
					body += DesignGateModeKey + "=" + mode + "\n"
				}
				problems := validateRepo(t, body, overlay)
				if hasProblem(problems, DesignGateModeKey) != (mode == "invalid") {
					t.Fatalf("settings validation mode=%s local=%v: %v", mode, local, problems)
				}
			}
		})
	}
}
