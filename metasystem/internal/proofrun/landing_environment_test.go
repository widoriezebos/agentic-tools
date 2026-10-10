package proofrun

import (
	"bytes"
	"errors"
	"testing"
)

func TestWriteLandingEnvironmentSharedHeaderAndErrors(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, fingerprint string
		err               error
		wantError         bool
	}{
		{name: "fingerprint", fingerprint: "fixture toolchain"},
		{name: "empty", wantError: true},
		{name: "whitespace", fingerprint: " \t", wantError: true},
		{name: "unreadable", err: errors.New("cannot read toolchain"), wantError: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var out bytes.Buffer
			err := WriteLandingEnvironment(&out, func() (string, error) { return tc.fingerprint, tc.err })
			if tc.wantError {
				if err == nil || out.Len() != 0 {
					t.Fatalf("invalid environment emitted: %q, %v", &out, err)
				}
				if tc.err == nil && err.Error() != "the check environment fingerprint is empty" {
					t.Fatal(err)
				}
			} else if err != nil || out.String() != "landing environment fixture toolchain\n" {
				t.Fatalf("header %q, error=%v", &out, err)
			}
		})
	}
}
