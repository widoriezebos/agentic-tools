package identity

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseProcfsHidepid(t *testing.T) {
	cases := []struct {
		name       string
		mounts     string
		value      string
		restricted bool
	}{
		{"absent option", "proc /proc proc rw,nosuid,nodev,noexec,relatime 0 0\n", "", false},
		{"explicit zero", "proc /proc proc rw,hidepid=0 0 0\n", "0", false},
		{"symbolic off", "proc /proc proc rw,hidepid=off 0 0\n", "off", false},
		{"numeric one", "proc /proc proc rw,hidepid=1 0 0\n", "1", true},
		{"numeric two", "proc /proc proc rw,nosuid,hidepid=2,relatime 0 0\n", "2", true},
		{"numeric four", "proc /proc proc rw,hidepid=4 0 0\n", "4", true},
		{"symbolic noaccess", "proc /proc proc rw,hidepid=noaccess 0 0\n", "noaccess", true},
		{"symbolic invisible", "proc /proc proc rw,hidepid=invisible 0 0\n", "invisible", true},
		{"symbolic ptraceable", "proc /proc proc rw,hidepid=ptraceable 0 0\n", "ptraceable", true},
		{"unknown future spelling is restrictive", "proc /proc proc rw,hidepid=paranoid 0 0\n", "paranoid", true},
		{"no proc line at all", "sysfs /sys sysfs rw 0 0\n", "", false},
		{"foreign proc mountpoint ignored", "proc /host/proc proc rw,hidepid=2 0 0\n", "", false},
		{"last proc mount wins", "proc /proc proc rw,hidepid=2 0 0\nproc /proc proc rw 0 0\n", "", false},
		{"last proc mount wins restrictive", "proc /proc proc rw 0 0\nproc /proc proc rw,hidepid=2 0 0\n", "2", true},
	}
	for _, tc := range cases {
		value, restricted := ParseProcfsHidepid(tc.mounts)
		if value != tc.value || restricted != tc.restricted {
			t.Fatalf("%s: got (%q, %v), want (%q, %v)", tc.name, value, restricted, tc.value, tc.restricted)
		}
	}
}

func TestRestrictedProcfsAt(t *testing.T) {
	dir := t.TempDir()
	// A missing mounts file is the darwin case: unrestricted.
	if value, restricted := RestrictedProcfsAt(filepath.Join(dir, "absent")); restricted {
		t.Fatalf("missing mounts file must be unrestricted, got %q", value)
	}
	path := filepath.Join(dir, "mounts")
	os.WriteFile(path, []byte("proc /proc proc rw,hidepid=invisible 0 0\n"), 0o644)
	if value, restricted := RestrictedProcfsAt(path); !restricted || value != "invisible" {
		t.Fatalf("restrictive mounts file: got (%q, %v)", value, restricted)
	}
}

// The same-user-scope invariant (recorded in the package doc): every
// consumer acting on Dead judges pids this engine's own user spawned, and
// the platform prober must never misread another user's LIVE process as
// dead. Pid 1 belongs to root and exists on every supported platform.
func TestForeignUserLiveProcessIsNeverDead(t *testing.T) {
	_, state, _ := (KernelProber{}).Probe(1)
	if state == Dead {
		t.Fatal("pid 1 is alive and owned by another user; Dead here breaks the three-way guarantee")
	}
}
