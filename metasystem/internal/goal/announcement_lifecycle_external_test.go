package goal_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/census"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
)

func TestAnnouncementLifecycleTokenStableWhenUnassociated(t *testing.T) {
	base := goal.SessionStopAnnouncementForTest{
		SessionId: "session-original", MainId: "main-1", Pid: 1, PidStartedAt: 2,
		Runtime: "fake", InstanceTag: "tag", CommandHash: strings.Repeat("a", 64),
		AnnouncedAt: "2026-09-15T10:00:00Z", Pgid: 1,
	}
	legacyToken, err := goal.SessionStopLifecycleTokenForTest(base)
	if err != nil {
		t.Fatal(err)
	}
	unassociated := base
	unassociated.RuntimeSession = ""
	unassociated.PreviousRuntimeSession = ""
	unassociated.SessionAssociatedAt = ""
	unassociatedToken, err := goal.SessionStopLifecycleTokenForTest(unassociated)
	if err != nil || unassociatedToken != legacyToken {
		t.Fatalf("empty association fields changed lifecycle identity: %q %v", unassociatedToken, err)
	}
	associated := base
	associated.RuntimeSession = "runtime-session"
	associated.SessionAssociatedAt = "2026-09-15T10:01:00Z"
	associatedToken, err := goal.SessionStopLifecycleTokenForTest(associated)
	if err != nil || associatedToken == legacyToken {
		t.Fatalf("real association did not change lifecycle identity: %q %v", associatedToken, err)
	}
	associated.SessionAssociatedAt = "2026-09-15T10:02:00Z"
	refreshedToken, err := goal.SessionStopLifecycleTokenForTest(associated)
	if err != nil || refreshedToken != associatedToken {
		t.Fatalf("stamp-only refresh changed lifecycle identity: %q %v", refreshedToken, err)
	}

	data, err := json.Marshal(associated)
	if err != nil {
		t.Fatal(err)
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatal(err)
	}
	if err := census.ValidateAnnouncementKeys(func(visit func(string) bool) {
		for key := range raw {
			visit(key)
		}
	}); err != nil {
		t.Fatalf("census strict keys refused association fields: %v", err)
	}
	var leaseAnnouncement lease.Announcement
	if err := json.Unmarshal(data, &leaseAnnouncement); err != nil || leaseAnnouncement.RuntimeSession != "runtime-session" {
		t.Fatalf("lease announcement did not decode association fields: %+v %v", leaseAnnouncement, err)
	}
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	var strict goal.SessionStopAnnouncementForTest
	if err := decoder.Decode(&strict); err != nil || strict.SessionAssociatedAt == "" {
		t.Fatalf("Stop reader did not decode association fields: %+v %v", strict, err)
	}
}
