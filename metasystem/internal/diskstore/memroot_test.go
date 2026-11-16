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
