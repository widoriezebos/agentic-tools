//go:build darwin

package identity

import (
	"strings"
	"testing"

	"golang.org/x/sys/unix"
)

func TestBootClockUsesDarwinBootSessionUUID(t *testing.T) {
	want, err := unix.Sysctl("kern.bootsessionuuid")
	if err != nil {
		t.Skipf("kern.bootsessionuuid is unavailable: %v", err)
	}
	want = strings.Trim(want, " \x00")
	got, _, err := BootClock()
	if err != nil || got != want {
		t.Fatalf("BootClock identity = %q, %v; want %q", got, err, want)
	}
}
