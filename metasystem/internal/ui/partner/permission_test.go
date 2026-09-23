package partner

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
)

// The permission point admits exactly one shape and refuses everything else,
// including everything it cannot read. ACP does not promise that a request
// carries anything but a tool-call id, so "could this write?" is not
// answerable from the wire and this point does not pretend it is.
func TestThePermissionPointRefusesEverythingItCannotClassify(t *testing.T) {
	t.Parallel()
	checkout := t.TempDir()
	for _, probe := range []struct {
		name    string
		request map[string]any
	}{
		{"an edit", permissionOf("Edit files", "edit", filepath.Join(checkout, "goal.md"))},
		{"a command", permissionOf("Run command", "execute", "")},
		{"a fetch", permissionOf("Fetch a page", "fetch", "")},
		{"a read with no path at all", permissionOf("Read something", "read")},
		{"a read outside the checkout", permissionOf("Read a secret", "read", "/etc/passwd")},
		{"a tool call with no kind", permissionOf("Something", "", filepath.Join(checkout, "goal.md"))},
	} {
		decided := judge(mustParams(t, probe.request), "s1", checkout)
		testutil.Expect(t, "refuses "+probe.name, decided.allowed, false)
		testutil.Expect(t, "answers an offered option for "+probe.name, decided.answer.OptionID, "no")
		testutil.Expect(t, "says so for "+probe.name, strings.HasPrefix(decided.activity, "Refused:"), true)
	}
}

// A read inside the checkout is the one thing a request can be, and it is
// allowed with the server's own offered option id.
func TestThePermissionPointAllowsAReadInsideTheCheckout(t *testing.T) {
	t.Parallel()
	checkout := t.TempDir()
	decided := judge(mustParams(t, permissionOf("Read a goal", "read", filepath.Join(checkout, "plans", "goals", "g1.md"))), "s1", checkout)
	testutil.Expect(t, "allowed", decided.allowed, true)
	testutil.Expect(t, "selects the offered option", decided.answer.OptionID, "yes")
	testutil.Expect(t, "names it", strings.HasPrefix(decided.activity, "Allowed a read"), true)
}

// A relative path is the session's own working directory, which is the
// checkout, so it is inside it.
func TestThePermissionPointResolvesARelativeReadAgainstTheCheckout(t *testing.T) {
	t.Parallel()
	checkout := t.TempDir()
	decided := judge(mustParams(t, permissionOf("Read a goal", "read", "plans/goals/g1.md")), "s1", checkout)
	testutil.Expect(t, "allowed", decided.allowed, true)
}

// A path that climbs out of the checkout is outside it however it is spelled.
func TestThePermissionPointRefusesAReadThatClimbsOut(t *testing.T) {
	t.Parallel()
	checkout := t.TempDir()
	decided := judge(mustParams(t, permissionOf("Read a secret", "read", "../../etc/passwd")), "s1", checkout)
	testutil.Expect(t, "refused", decided.allowed, false)
}

// A request for another session is not this turn's and is refused as one.
func TestThePermissionPointRefusesAnotherSessionsRequest(t *testing.T) {
	t.Parallel()
	checkout := t.TempDir()
	decided := judge(mustParams(t, permissionOf("Read a goal", "read", filepath.Join(checkout, "g1.md"))), "other", checkout)
	testutil.Expect(t, "refused", decided.allowed, false)
	testutil.Expect(t, "says which", strings.Contains(decided.activity, "another session"), true)
}

// A body this client cannot read is a request it cannot classify.
func TestThePermissionPointRefusesAnUnreadableRequest(t *testing.T) {
	t.Parallel()
	decided := judge(json.RawMessage("not json"), "s1", t.TempDir())
	testutil.Expect(t, "refused", decided.allowed, false)
	testutil.Expect(t, "cancels where no option was offered", decided.answer.Outcome, "cancelled")
}

// A server that offers no single refusing option gets a cancellation rather
// than a guess: the wire has no abstract deny.
func TestThePermissionPointCancelsWhenNoRefusingOptionIsOffered(t *testing.T) {
	t.Parallel()
	checkout := t.TempDir()
	request := permissionOf("Edit files", "edit", filepath.Join(checkout, "g1.md"))
	request["options"] = []map[string]any{{"optionId": "yes", "kind": "allow_once"}}
	decided := judge(mustParams(t, request), "s1", checkout)
	testutil.Expect(t, "refused", decided.allowed, false)
	testutil.Expect(t, "cancelled", decided.answer.Outcome, "cancelled")
}

// A read this client would allow, from a server that offers no allow option,
// is refused rather than guessed at.
func TestThePermissionPointRefusesAnAllowableReadWithNoAllowOption(t *testing.T) {
	t.Parallel()
	checkout := t.TempDir()
	request := permissionOf("Read a goal", "read", filepath.Join(checkout, "g1.md"))
	request["options"] = []map[string]any{{"optionId": "no", "kind": "reject_once"}}
	decided := judge(mustParams(t, request), "s1", checkout)
	testutil.Expect(t, "refused", decided.allowed, false)
	testutil.Expect(t, "takes the offered refusal", decided.answer.OptionID, "no")
}

func permissionOf(title, kind string, paths ...string) map[string]any {
	locations := make([]map[string]any, 0, len(paths))
	for _, path := range paths {
		if path == "" {
			continue
		}
		locations = append(locations, map[string]any{"path": path})
	}
	return map[string]any{
		"sessionId": "s1",
		"toolCall": map[string]any{
			"toolCallId": "call-1", "title": title, "kind": kind, "locations": locations,
		},
		"options": []map[string]any{
			{"optionId": "yes", "kind": "allow_once", "name": "Allow"},
			{"optionId": "no", "kind": "reject_once", "name": "Refuse"},
		},
	}
}

func mustParams(t *testing.T, request map[string]any) json.RawMessage {
	t.Helper()
	body, err := json.Marshal(request)
	if err != nil {
		t.Fatalf("the probe request must marshal: %v", err)
	}
	return body
}
