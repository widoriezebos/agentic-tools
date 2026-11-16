package diskstore

import (
	"os"
	"strconv"
	"testing"
)

func TestOnTestMemoryRoot(t *testing.T) {
	t.Parallel()
	root := "/tmp/mst-" + strconv.Itoa(os.Geteuid())
	for _, spelling := range []string{root, "/private" + root} {
		for _, suffix := range []string{"", "/", "/home", "/home/../tmp", "/./home", "0", "x", "/../outside"} {
			path := spelling + suffix
			want := suffix != "0" && suffix != "x" && suffix != "/../outside"
			if got := OnTestMemoryRoot(path); got != want {
				t.Errorf("OnTestMemoryRoot(%q) = %v, want %v", path, got, want)
			}
		}
	}
	for _, path := range []string{"", ".", "/tmp", "/private/tmp", "relative" + root} {
		if OnTestMemoryRoot(path) {
			t.Errorf("OnTestMemoryRoot(%q) = true", path)
		}
	}
}

func TestInTestRegistryHome(t *testing.T) {
	t.Parallel()
	memoryRoot := "/tmp/mst-" + strconv.Itoa(os.Geteuid())
	for _, parent := range []string{"/tmp", "/private/tmp", memoryRoot, "/private" + memoryRoot, "/tmp/x", "/var/tmp", "/arbitrary/fallback"} {
		for _, suffix := range []string{"", "/home/.metasystem", "/./nested/../home"} {
			path := parent + "/metasystem-test-registry-1" + suffix
			if !InTestRegistryHome(path) {
				t.Errorf("InTestRegistryHome(%q) = false", path)
			}
		}
	}
	for _, path := range []string{"", ".", "/tmp/metasystem-test-registry", "/tmp/xmetasystem-test-registry-1", "/tmp/metasystem-test-registry-1/../home", "/home/person/.metasystem"} {
		if InTestRegistryHome(path) {
			t.Errorf("InTestRegistryHome(%q) = true", path)
		}
	}
}
