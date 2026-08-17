package census

import (
	"fmt"
	"strings"
)

// The mains-directory record contract has ONE home (review lease-census-4):
// which files are announcements, which keys an announcement carries, and
// the identity grammars. census and lease both consume these; the census
// once went CENSUS-FAILED in production because its private optional-key
// list missed a field the lease had grown.

// announcementProtocolFiles are the mains-directory files that are not main
// announcements and must never be classified as one.
var announcementProtocolFiles = map[string]bool{
	"worktree-lease.json":        true,
	"worktree-commit-token.json": true,
	"reaped-after-claim.json":    true,
}

// IsAnnouncementFile reports whether a mains-directory basename is a main
// announcement rather than a protocol file or a protocol cursor.
func IsAnnouncementFile(name string) bool {
	return !strings.HasSuffix(name, ".protocol-cursor.json") && !announcementProtocolFiles[name]
}

// AnnouncementRequiredKeys every announcement carries; AnnouncementOptionalKeys
// may appear and nothing else may.
var (
	AnnouncementRequiredKeys = []string{
		"sessionId", "pid", "pidStartedAt", "pgid", "runtime", "instanceTag", "announcedAt",
	}
	AnnouncementOptionalKeys = map[string]bool{
		"mainId": true, "commandHash": true, "ownerLineage": true,
		// The clock-step-immune identity pair (issue #1): optional so
		// legacy announcements stay valid; absent on darwin (omitempty).
		"pidStartTicks": true, "bootId": true,
	}
)

// ValidateAnnouncementKeys checks one decoded announcement against the
// shared key contract: every required key present, no unknown keys.
func ValidateAnnouncementKeys(keys func(func(string) bool)) error {
	present := map[string]bool{}
	var unknown string
	keys(func(key string) bool {
		present[key] = true
		if !AnnouncementOptionalKeys[key] && !isRequiredAnnouncementKey(key) && unknown == "" {
			unknown = key
		}
		return true
	})
	for _, key := range AnnouncementRequiredKeys {
		if !present[key] {
			return fmt.Errorf("announcement lacks required key %s", key)
		}
	}
	if unknown != "" {
		return fmt.Errorf("announcement carries unknown key %s", unknown)
	}
	return nil
}

func isRequiredAnnouncementKey(key string) bool {
	for _, required := range AnnouncementRequiredKeys {
		if key == required {
			return true
		}
	}
	return false
}
