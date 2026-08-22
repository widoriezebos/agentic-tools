package identity

import (
	"os"
	"strings"
)

// Restricted-procfs detection
// (a human-approved reserved decision). With /proc mounted hidepid, another
// user's live process can be indistinguishable from a dead one — a stat
// that fails ENOENT — which silently breaks the three-way guarantee this
// package exists to keep: a false Dead authorizes lock takeover and
// process-lost. Supervision therefore refuses to arm under a restrictive
// mount. The rule is configuration-based, not privilege-based: root's
// exemption from hidepid does not relax the refusal, so behavior does not
// depend on who runs the engine.

// ParseProcfsHidepid extracts the hidepid option of the /proc mount from
// mounts-file content (/proc/self/mounts format: device mountpoint fstype
// options ...). It accepts the numeric spellings 0, 1, 2, and 4 and the
// symbolic off, noaccess, invisible, and ptraceable; an absent option or an
// absent /proc line is unrestricted. When /proc appears more than once the
// last line wins, matching mount shadowing. Any value other than 0/off is
// restrictive — hidepid=1 already blocks reading another user's stat, and
// an unrecognized future spelling is treated as restrictive rather than
// silently trusted.
func ParseProcfsHidepid(mounts string) (value string, restricted bool) {
	found := ""
	for _, line := range strings.Split(mounts, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 4 || fields[1] != "/proc" || fields[2] != "proc" {
			continue
		}
		found = ""
		for _, option := range strings.Split(fields[3], ",") {
			if rest, ok := strings.CutPrefix(option, "hidepid="); ok {
				found = rest
			}
		}
	}
	switch found {
	case "", "0", "off":
		return found, false
	}
	return found, true
}

// RestrictedProcfsAt reads a mounts file and reports whether its /proc
// mount is restrictive. A missing file is unrestricted — darwin has no
// /proc/self/mounts and its prober does not read procfs — but any other
// read failure is treated as restrictive: a procfs whose own mount table
// cannot be read is not a procfs whose liveness answers deserve trust.
func RestrictedProcfsAt(path string) (value string, restricted bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", false
		}
		return "unreadable-mounts", true
	}
	return ParseProcfsHidepid(string(data))
}
