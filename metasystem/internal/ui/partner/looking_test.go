package partner

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/uitools"
)

// The one named exception: a call to the interface's own tool server is
// admitted, whichever field the runtime carries the name in, and everything
// else is still refused exactly as before.
func TestThePermissionPointAdmitsTheInterfacesOwnTools(t *testing.T) {
	t.Parallel()
	checkout := t.TempDir()
	for _, probe := range []struct {
		name    string
		request map[string]any
	}{
		{"Claude's own name field", toolPermission("mcp__metasystem__document", "", "")},
		{"the adapter's meta", toolPermission("", "mcp__metasystem__board", "")},
		{"and the title, where a runtime carries nothing else", toolPermission("", "", "metasystem__search")},
		{"a dotted spelling", toolPermission("metasystem.overview", "", "")},
		{"a slashed spelling", toolPermission("mcp__metasystem/questions", "", "")},
	} {
		decided := judge(mustParams(t, probe.request), "s1", checkout)
		testutil.Expect(t, "admits "+probe.name, decided.allowed, true)
		testutil.Expect(t, "selects the offered option for "+probe.name, decided.answer.OptionID, "yes")
		testutil.Expect(t, "and names the operation for "+probe.name,
			strings.HasPrefix(decided.activity, "Allowed a read through the interface's own tools: "), true)
	}
}

// The one operation of this server that reads nothing is admitted like the
// rest, and the line the conversation shows says what actually happened: a
// human reading "allowed a read" under an answer that read nothing would be
// reading a claim this point never had any business making.
func TestThePointSaysWhatAnAdmittedCallActuallyDid(t *testing.T) {
	t.Parallel()
	decided := judge(mustParams(t, toolPermission("mcp__metasystem__suggest", "", "")), "s1", t.TempDir())
	testutil.Expect(t, "it is admitted", decided.allowed, true)
	testutil.Expect(t, "and it is not called a read", decided.activity,
		"Allowed the interface's own tools to prepare a suggestion: nothing was read and nothing was written")
}

// The exception is this server's operations and nothing beside them: another
// server's tools, a tool this server does not have, and a name that merely
// contains the server's are all refused.
func TestThePermissionPointAdmitsOnlyThisServersOperations(t *testing.T) {
	t.Parallel()
	checkout := t.TempDir()
	for _, probe := range []struct {
		name    string
		request map[string]any
	}{
		{"another server's tool", toolPermission("mcp__github__create_issue", "", "")},
		{"an operation this server has not got", toolPermission("mcp__metasystem__write", "", "")},
		{"a server whose name merely ends in ours", toolPermission("mcp__notmetasystem__board", "", "")},
		{"the server with no operation at all", toolPermission("mcp__metasystem__", "", "")},
	} {
		decided := judge(mustParams(t, probe.request), "s1", checkout)
		testutil.Expect(t, "refuses "+probe.name, decided.allowed, false)
		testutil.Expect(t, "and says so for "+probe.name, strings.HasPrefix(decided.activity, "Refused:"), true)
	}
}

// An admitted call still needs an option to select: a server that offers no
// allow_once is a server this point has nothing to answer with, and a guess
// there would be an approval nobody granted.
func TestAnAdmittedToolCallWithNoAllowOptionIsStillRefused(t *testing.T) {
	t.Parallel()
	checkout := t.TempDir()
	request := toolPermission("mcp__metasystem__board", "", "")
	request["options"] = []map[string]any{{"optionId": "no", "kind": "reject_once"}}
	decided := judge(mustParams(t, request), "s1", checkout)
	testutil.Expect(t, "refused", decided.allowed, false)
	testutil.Expect(t, "taking the offered refusal", decided.answer.OptionID, "no")
}

// The tool server is handed to the session at session/new, as a stdio server
// with no type member — which is the shape every adapter reads as stdio.
func TestTheToolServerIsHandedOverAtSessionNew(t *testing.T) {
	t.Parallel()
	tools := &ToolServer{
		Name: uitools.ServerName, Command: "/usr/local/bin/metasystem",
		Args: []string{"ui", "tools", "--root", "/checkout", "--metasystem-root", "/checkout/metasystem"},
		Env:  []string{"NO_COLOR=1"},
	}
	body, err := json.Marshal(map[string]any{"mcpServers": tools.wire()})
	testutil.Require(t, "it marshals", err, nil)
	handed := string(body)
	testutil.Expect(t, "it names the server", strings.Contains(handed, `"name":"metasystem"`), true)
	testutil.Expect(t, "the command", strings.Contains(handed, `"command":"/usr/local/bin/metasystem"`), true)
	testutil.Expect(t, "the verb, with both roots named",
		strings.Contains(handed, `["ui","tools","--root","/checkout","--metasystem-root","/checkout/metasystem"]`), true)
	testutil.Expect(t, "the environment as name and value pairs",
		strings.Contains(handed, `{"name":"NO_COLOR","value":"1"}`), true)
	testutil.Expect(t, "and no type, which is what makes it stdio",
		strings.Contains(handed, `"type"`), false)

	var none *ToolServer
	empty, err := json.Marshal(map[string]any{"mcpServers": none.wire()})
	testutil.Require(t, "a runtime with no tools marshals too", err, nil)
	testutil.Expect(t, "and hands over none", string(empty), `{"mcpServers":[]}`)
}

// The standing rule carries the interface's own words, so the Partner speaks
// of tiers, lanes and arcs the way the pages explain them.
func TestTheStandingRuleCarriesTheInterfacesVocabulary(t *testing.T) {
	t.Parallel()
	terms := Vocabulary()
	testutil.Expect(t, "the register is there", len(terms) > 20, true)
	composed := Compose(Facts{}, Page{Section: "Backlog"}, "Wido", time.Now().UTC())
	testutil.Expect(t, "the block is headed",
		strings.Contains(composed, "What this interface's words mean, as it explains them to the human"), true)
	for _, wanted := range []string{"Tier", "Arc", "Ready for Work", "Priority"} {
		testutil.Expect(t, "it carries "+wanted, strings.Contains(composed, "- "+wanted+": "), true)
	}
	testutil.Expect(t, "and a term's own sentence",
		strings.Contains(composed, "How much proof a goal owes before it lands, 1 to 3"), true)
}

// The block says how much of the page it carries and where the rest is, so a
// bounded context is a number the Partner can act on rather than a silence.
func TestTheBlockSaysHowManyRowsOfHowManyItSupplied(t *testing.T) {
	t.Parallel()
	page := Page{Section: "Backlog", Path: "/backlog", Lanes: []Lane{
		{ID: "waiting", Title: "Waiting", Total: 40, Goals: []string{"waiting", "waiting-two"}},
	}}
	composed := Compose(readingFacts(), page, "Wido", composedAt)
	testutil.Expect(t, "how much of how much",
		strings.Contains(composed, "2 of 40 rows supplied; ask the metasystem tools for the rest, which take a cursor."), true)
	testutil.Expect(t, "and the lane says the same in its own words",
		strings.Contains(composed, "38 more goals are on the board than are named here; the board tool reads the rest, and takes a cursor."), true)
}

// A selected passage goes first and whole, with the revision it was read at:
// it is the one thing in the block the human pointed at.
func TestASelectedPassageGoesFirstAndWhole(t *testing.T) {
	t.Parallel()
	page := Page{
		Section: "Project", Path: "/project/doc/plans/designs/d1.md",
		Quote:     "One client, and nothing else.\nThat is the seam.",
		QuoteFrom: "plans/designs/d1.md", QuoteRevision: "r7", QuoteAnchor: "The seam",
	}
	composed := Compose(readingFacts(), page, "Wido", composedAt)
	testutil.Expect(t, "it says where it came from", strings.Contains(composed,
		"The human selected this passage, from plans/designs/d1.md, revision r7, under The seam:"), true)
	testutil.Expect(t, "the first line, marked off", strings.Contains(composed, "  > One client, and nothing else."), true)
	testutil.Expect(t, "and the second", strings.Contains(composed, "  > That is the seam."), true)
}

// Where the page rendered from a reading older than the server's own, the
// block says both rather than certifying the newer one as what the human saw.
func TestTheBlockSaysBothReadingsWhenTheyDiffer(t *testing.T) {
	t.Parallel()
	page := Page{Section: "Backlog", Path: "/backlog", Tip: "0000old", ObservedAt: "2026-09-23T11:00:00Z"}
	seen := See(readingFacts(), page, composedAt)
	testutil.Expect(t, "the server's own reading",
		strings.HasPrefix(seen.Source, "the accepted tip "+observedTip), true)
	testutil.Expect(t, "and the page's", seen.Displayed, "the accepted tip 0000old, observed 2026-09-23T11:00:00Z")
	testutil.Expect(t, "the stamp says both",
		strings.Contains(seen.Stamp(), "the page had rendered from the accepted tip 0000old"), true)
	testutil.Expect(t, "and so does the block", strings.Contains(
		ComposeSeen(seen, page, "Wido"), "The page itself had rendered from the accepted tip 0000old"), true)

	agreeing := See(readingFacts(), Page{Section: "Backlog", Tip: observedTip, ObservedAt: "2026-09-23T11:29:55Z"}, composedAt)
	testutil.Expect(t, "two readings that agree say one", agreeing.Displayed, "")
	testutil.Expect(t, "and the stamp names it once", strings.HasPrefix(agreeing.Stamp(), "Saw: the accepted tip"), true)
}

/* ---------------------------------------------------------------- driving -- */

// toolPermission is a permission request shaped the way an application tool's
// is: kind "other", no location, and the name in whichever field a runtime
// happens to carry it in.
func toolPermission(name, meta, title string) map[string]any {
	call := map[string]any{"toolCallId": "call-1", "kind": "other", "locations": []map[string]any{}}
	if name != "" {
		call["name"] = name
	}
	if meta != "" {
		call["_meta"] = map[string]any{"claudeCode": map[string]any{"toolName": meta}}
	}
	if title != "" {
		call["title"] = title
	}
	return map[string]any{
		"sessionId": "s1",
		"toolCall":  call,
		"options": []map[string]any{
			{"optionId": "yes", "kind": "allow_once", "name": "Allow"},
			{"optionId": "no", "kind": "reject_once", "name": "Refuse"},
		},
	}
}
