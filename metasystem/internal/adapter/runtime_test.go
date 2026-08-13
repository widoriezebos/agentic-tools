package adapter

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// --- version parse ---

func TestParseCLIVersion(t *testing.T) {
	cases := map[string]string{
		"claude 1.2.3 (build)": "1.2.3",
		"codex-cli 0.9.0-beta": "0.9.0-beta",
		"v2.10.1\n":            "2.10.1",
	}
	for input, want := range cases {
		got, err := ParseCLIVersion(strings.NewReader(input))
		if err != nil {
			t.Fatalf("ParseCLIVersion(%q): %v", input, err)
		}
		if got != want {
			t.Fatalf("ParseCLIVersion(%q) = %q, want %q", input, got, want)
		}
	}
	if _, err := ParseCLIVersion(strings.NewReader("no version here")); err == nil {
		t.Fatal("a version-less string must be refused")
	}
}

// --- codex events and usage ---

func TestCodexEventField(t *testing.T) {
	dir := t.TempDir()
	events := filepath.Join(dir, "events.jsonl")
	writeFile(t, events, strings.Join([]string{
		`{"type":"turn.started","turn_id":"t-1"}`,
		`not json`,
		`{"type":"thread.started","thread_id":"th-9"}`,
		`{"type":"turn.started","id":"t-2"}`,
	}, "\n"))

	if id, ok := CodexEventField(events, "session"); !ok || id != "th-9" {
		t.Fatalf("session id = %q ok=%v, want th-9", id, ok)
	}
	if id, ok := CodexEventField(events, "turn"); !ok || id != "t-1" {
		t.Fatalf("turn id = %q ok=%v, want t-1 (first)", id, ok)
	}
	if _, ok := CodexEventField(filepath.Join(dir, "absent.jsonl"), "session"); ok {
		t.Fatal("an absent event stream must report not found")
	}
}

func TestCodexUsage(t *testing.T) {
	dir := t.TempDir()
	events := filepath.Join(dir, "events.jsonl")
	// The last usage block wins; camelCase spellings are accepted as fallbacks.
	writeFile(t, events, strings.Join([]string{
		`{"type":"turn","usage":{"input_tokens":10,"output_tokens":3}}`,
		`{"type":"turn","usage":{"input_tokens":21,"cached_input_tokens":5,"output_tokens":7,"reasoning_output_tokens":2}}`,
	}, "\n"))
	out := filepath.Join(dir, "usage.json")
	if err := CodexUsage(events, out); err != nil {
		t.Fatal(err)
	}
	got := readJSONFile(t, out)
	if got["availability"] != "native" || got["cost"] != nil || got["providerUnits"] != nil {
		t.Fatalf("unexpected usage envelope: %v", got)
	}
	if got["inputTokens"] != float64(21) || got["cachedInputTokens"] != float64(5) ||
		got["outputTokens"] != float64(7) || got["reasoningTokens"] != float64(2) {
		t.Fatalf("unexpected token counts: %v", got)
	}

	// An empty stream keeps the shape with null counts.
	empty := filepath.Join(dir, "empty.jsonl")
	writeFile(t, empty, "")
	if err := CodexUsage(empty, out); err != nil {
		t.Fatal(err)
	}
	got = readJSONFile(t, out)
	if got["inputTokens"] != nil || got["availability"] != "native" {
		t.Fatalf("empty stream should yield null counts: %v", got)
	}
}

func TestBuildCodexCommand(t *testing.T) {
	dispatch, err := BuildCodexCommand("dispatch", "gpt-5-sol", "/ws", "/schema.json", "/out.json", "workspace-write", "true", "")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"codex", "exec", "--json", "-m", "gpt-5-sol", "--sandbox", "workspace-write",
		"-C", "/ws", "-c", `approval_policy="never"`,
		"-c", "sandbox_workspace_write.network_access=true",
		"--output-schema", "/schema.json", "-o", "/out.json", "-",
	}
	if strings.Join(dispatch, "\x00") != strings.Join(want, "\x00") {
		t.Fatalf("dispatch argv:\n got %q\nwant %q", dispatch, want)
	}

	resume, err := BuildCodexCommand("follow-up", "gpt-5-sol", "/ws", "/schema.json", "/out.json", "read-only", "false", "sid-7")
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(resume, "\x00")
	for _, needle := range []string{"resume", `model="gpt-5-sol"`, `sandbox_mode="read-only"`, "sid-7"} {
		if !strings.Contains(joined, needle) {
			t.Fatalf("resume argv missing %q: %v", needle, resume)
		}
	}
	if _, err := BuildCodexCommand("follow-up", "m", "", "s", "o", "read-only", "false", ""); err == nil {
		t.Fatal("a follow-up without a session must be refused")
	}
	if _, err := BuildCodexCommand("nonsense", "m", "", "s", "o", "read-only", "false", ""); err == nil {
		t.Fatal("an unknown verb must be refused")
	}
}

// --- claude settings, usage, fields, roots, signal ---

func claudeRecord(writeRoots, network string) string {
	return `{
	  "workspaceRoot": "/ws",
	  "permissions": {"requested": {
	    "readRoots": ["/ws", "/extra", "/more"],
	    "writeRoots": ` + writeRoots + `,
	    "network": "` + network + `"
	  }}
	}`
}

func TestBuildClaudeSettingsWriteAndNetwork(t *testing.T) {
	dir := t.TempDir()
	record := filepath.Join(dir, "job.json")
	out := filepath.Join(dir, "settings.json")
	writeFile(t, record, claudeRecord(`["/ws/sub"]`, "allow"))
	if err := BuildClaudeSettings(record, out, "/opt/bin/metasystem"); err != nil {
		t.Fatal(err)
	}
	got := readJSONFile(t, out)
	perms := got["permissions"].(map[string]any)
	allow := toStringSet(perms["allow"].([]any))
	if !allow["Bash"] || !allow["Write"] || !allow["WebFetch"] {
		t.Fatalf("write+network turn should allow edit and web tools: %v", perms["allow"])
	}
	if len(perms["deny"].([]any)) != 0 {
		t.Fatalf("nothing should be denied: %v", perms["deny"])
	}
	sandbox := got["sandbox"].(map[string]any)
	fs := sandbox["filesystem"].(map[string]any)
	roots := fs["allowWrite"].([]any)
	if len(roots) != 1 || roots[0] != "/ws/sub" {
		t.Fatalf("filesystem allowWrite should carry the requested roots verbatim: %v", roots)
	}
	net := sandbox["network"].(map[string]any)
	if len(net["allowedDomains"].([]any)) != 0 {
		t.Fatalf("networked turn should permit ordinary egress: %v", net)
	}
	hookCommand := hookCommandOf(t, got)
	if hookCommand != "/opt/bin/metasystem adapter claude-session-signal" {
		t.Fatalf("unexpected hook command %q", hookCommand)
	}
}

func TestBuildClaudeSettingsReadOnlyNoNetwork(t *testing.T) {
	dir := t.TempDir()
	record := filepath.Join(dir, "job.json")
	out := filepath.Join(dir, "settings.json")
	writeFile(t, record, claudeRecord(`[]`, "deny"))
	if err := BuildClaudeSettings(record, out, "/opt/bin/metasystem"); err != nil {
		t.Fatal(err)
	}
	got := readJSONFile(t, out)
	perms := got["permissions"].(map[string]any)
	deny := toStringSet(perms["deny"].([]any))
	if !deny["Bash"] || !deny["Write"] || !deny["WebFetch"] || !deny["WebSearch"] {
		t.Fatalf("read-only no-network turn should deny edit and web tools: %v", perms["deny"])
	}
	sandbox := got["sandbox"].(map[string]any)
	fs := sandbox["filesystem"].(map[string]any)
	if len(fs["allowWrite"].([]any)) != 0 {
		t.Fatalf("read-only turn should grant no write roots: %v", fs["allowWrite"])
	}
	net := sandbox["network"].(map[string]any)
	blocked := net["allowedDomains"].([]any)
	if len(blocked) != 1 || blocked[0] != "metasystem.invalid" {
		t.Fatalf("no-network turn should carry the non-resolving sentinel: %v", net)
	}
}

func TestClaudeUsage(t *testing.T) {
	dir := t.TempDir()
	result := filepath.Join(dir, "result.json")
	out := filepath.Join(dir, "usage.json")
	writeFile(t, result, `{
	  "usage": {"input_tokens": 100, "cache_read_input_tokens": 20, "output_tokens": 40, "reasoning_tokens": 8},
	  "total_cost_usd": 0.125
	}`)
	if err := ClaudeUsage(result, out); err != nil {
		t.Fatal(err)
	}
	got := readJSONFile(t, out)
	if got["inputTokens"] != float64(100) || got["cachedInputTokens"] != float64(20) ||
		got["outputTokens"] != float64(40) || got["reasoningTokens"] != float64(8) {
		t.Fatalf("unexpected token counts: %v", got)
	}
	cost := got["cost"].(map[string]any)
	if cost["amount"] != float64(0.125) || cost["currency"] != "USD" {
		t.Fatalf("unexpected cost: %v", cost)
	}

	// An absent result records native availability with null counts and null cost.
	if err := ClaudeUsage(filepath.Join(dir, "absent.json"), out); err != nil {
		t.Fatal(err)
	}
	got = readJSONFile(t, out)
	if got["availability"] != "native" || got["inputTokens"] != nil || got["cost"] != nil {
		t.Fatalf("absent result should yield null counts: %v", got)
	}
}

func TestClaudeResultField(t *testing.T) {
	dir := t.TempDir()
	result := filepath.Join(dir, "result.json")

	writeFile(t, result, `{"session_id": "s-1", "modelUsage": {"claude-opus-4-8": {}}}`)
	if v, p, err := ClaudeResultField(result, "session_id"); err != nil || !p || v != "s-1" {
		t.Fatalf("session_id: %q print=%v err=%v", v, p, err)
	}
	if v, p, err := ClaudeResultField(result, "model"); err != nil || !p || v != "claude-opus-4-8" {
		t.Fatalf("single model should collapse to its key: %q print=%v err=%v", v, p, err)
	}

	writeFile(t, result, `{"modelUsage": {"b": {}, "a": {}}}`)
	if v, _, _ := ClaudeResultField(result, "model"); v != "multi-model:a,b" {
		t.Fatalf("multi model should list sorted keys, got %q", v)
	}

	writeFile(t, result, `{"modelUsage": {}}`)
	if v, _, _ := ClaudeResultField(result, "model"); v != "unobserved" {
		t.Fatalf("no model should be unobserved, got %q", v)
	}

	// A present-null field prints nothing; an absent field prints empty.
	writeFile(t, result, `{"session_id": null}`)
	if _, p, _ := ClaudeResultField(result, "session_id"); p {
		t.Fatal("a present-null field must not print")
	}
	if _, p, _ := ClaudeResultField(result, "absent"); !p {
		t.Fatal("an absent field prints an empty line")
	}

	if _, _, err := ClaudeResultField(filepath.Join(dir, "nope.json"), "session_id"); err == nil {
		t.Fatal("an unreadable document must be an error")
	}
}

func TestClaudeReadRoots(t *testing.T) {
	dir := t.TempDir()
	record := filepath.Join(dir, "job.json")
	writeFile(t, record, claudeRecord(`[]`, "deny"))
	roots, err := ClaudeReadRoots(record)
	if err != nil {
		t.Fatal(err)
	}
	if len(roots) != 2 || roots[0] != "/extra" || roots[1] != "/more" {
		t.Fatalf("read roots should exclude the workspace root: %v", roots)
	}
}

func TestClaudeAppendResult(t *testing.T) {
	dir := t.TempDir()
	events := filepath.Join(dir, "events.jsonl")
	result := filepath.Join(dir, "result.json")
	writeFile(t, result, `{"result": "ok", "usage": {"input_tokens": 3}}`)
	if err := ClaudeAppendResult(result, events); err != nil {
		t.Fatal(err)
	}
	// A malformed result is a no-op that leaves the events file unchanged.
	writeFile(t, result, `not json`)
	if err := ClaudeAppendResult(result, events); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(events)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 1 || !strings.Contains(lines[0], `"result":"ok"`) {
		t.Fatalf("exactly the readable result should be appended, got %q", string(data))
	}
}

func TestClaudeSessionSignal(t *testing.T) {
	dir := t.TempDir()
	signal := filepath.Join(dir, "signal.json")
	events := filepath.Join(dir, "events.jsonl")
	id, err := ClaudeSessionSignal(strings.NewReader(`{"session_id":"s-42","model":"claude-opus-4-8","source":"startup"}`), signal, events)
	if err != nil {
		t.Fatal(err)
	}
	if id != "s-42" {
		t.Fatalf("returned session id = %q", id)
	}
	got := readJSONFile(t, signal)
	if got["session_id"] != "s-42" || got["model"] != "claude-opus-4-8" || got["source"] != "startup" {
		t.Fatalf("unexpected signal file: %v", got)
	}
	data, _ := os.ReadFile(events)
	if !strings.Contains(string(data), `"subtype":"init"`) || !strings.Contains(string(data), `"session_id":"s-42"`) {
		t.Fatalf("session-init event not appended: %q", string(data))
	}

	if _, err := ClaudeSessionSignal(strings.NewReader(`{"model":"x"}`), signal, events); err == nil {
		t.Fatal("a payload without a session id must be refused")
	}
}

// --- devin config, correlation, usage ---

func TestBuildDevinConfig(t *testing.T) {
	dir := t.TempDir()
	// A user config supplies the surviving members and a sandbox to drop.
	configHome := filepath.Join(dir, "config")
	writeFile(t, filepath.Join(configHome, "devin", "config.json"),
		`{"organizationId": "org-1", "onboardingComplete": true, "sandbox": {"x": 1}, "permissions": {"allow": ["old"]}}`)
	t.Setenv("XDG_CONFIG_HOME", configHome)

	record := filepath.Join(dir, "job.json")
	writeFile(t, record, `{"permissions":{"requested":{"readRoots":["/r1","/r2"],"writeRoots":["/w1"]}}}`)
	out := filepath.Join(dir, "devin-config.json")
	prov := filepath.Join(dir, "prov.json")
	if err := BuildDevinConfig(record, out, prov); err != nil {
		t.Fatal(err)
	}
	got := readJSONFile(t, out)
	if _, ok := got["sandbox"]; ok {
		t.Fatalf("sandbox must be dropped: %v", got)
	}
	if got["organizationId"] != "org-1" || got["onboardingComplete"] != true {
		t.Fatalf("inherited members must survive: %v", got)
	}
	perms := got["permissions"].(map[string]any)
	allow := toStringSet(perms["allow"].([]any))
	if !allow["read"] || !allow["edit"] || !allow["Read(/r1/**)"] || !allow["Write(/w1/**)"] {
		t.Fatalf("unexpected allow list: %v", perms["allow"])
	}
	deny := toStringSet(perms["deny"].([]any))
	if !deny["Fetch(*)"] || !deny["mcp__*"] {
		t.Fatalf("unexpected deny list: %v", perms["deny"])
	}
	provenance := readJSONFile(t, prov)
	replaced := toStringSet(provenance["replacedMembers"].([]any))
	if !replaced["permissions"] || !replaced["sandbox"] {
		t.Fatalf("both permissions and sandbox were replaced: %v", provenance["replacedMembers"])
	}
	inherited := toStringSet(provenance["inheritedMembers"].([]any))
	if !inherited["organizationId"] || inherited["permissions"] {
		t.Fatalf("inherited members should exclude permissions: %v", provenance["inheritedMembers"])
	}
}

func TestBuildDevinConfigReadOnly(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(dir, "empty"))
	record := filepath.Join(dir, "job.json")
	writeFile(t, record, `{"permissions":{"requested":{"readRoots":["/r"],"writeRoots":[]}}}`)
	out := filepath.Join(dir, "cfg.json")
	prov := filepath.Join(dir, "prov.json")
	if err := BuildDevinConfig(record, out, prov); err != nil {
		t.Fatal(err)
	}
	perms := readJSONFile(t, out)["permissions"].(map[string]any)
	deny := toStringSet(perms["deny"].([]any))
	if !deny["edit"] || !deny["Write(**)"] {
		t.Fatalf("a read-only role must deny edit and writes: %v", perms["deny"])
	}
	// A missing user config leaves nothing to inherit and nothing replaced.
	provenance := readJSONFile(t, prov)
	if len(provenance["replacedMembers"].([]any)) != 0 || len(provenance["inheritedMembers"].([]any)) != 0 {
		t.Fatalf("a missing user config replaces and inherits nothing: %v", provenance)
	}
}

func TestDevinSessionCorrelate(t *testing.T) {
	dir := t.TempDir()
	ws := t.TempDir()
	before := filepath.Join(dir, "before.json")
	current := filepath.Join(dir, "current.json")
	nosignal := filepath.Join(dir, "nosignal.json")
	writeFile(t, before, `[{"id":"old","working_directory":"`+ws+`"}]`)

	// One new session in the workspace correlates.
	writeFile(t, current, `[{"id":"old","working_directory":"`+ws+`"},{"id":"new","working_directory":"`+ws+`"}]`)
	if id, cand := DevinSessionCorrelate(before, current, nosignal, ws); id != "new" || cand != nil {
		t.Fatalf("expected the new session, got id=%q cand=%v", id, cand)
	}

	// A new session in a different directory does not correlate.
	writeFile(t, current, `[{"id":"new","working_directory":"/somewhere/else"}]`)
	if id, cand := DevinSessionCorrelate(before, current, nosignal, ws); id != "" || len(cand) != 0 {
		t.Fatalf("out-of-workspace session must not correlate, got id=%q cand=%v", id, cand)
	}

	// Two new sessions in the workspace is ambiguous.
	writeFile(t, current, `[{"id":"a","working_directory":"`+ws+`"},{"id":"b","working_directory":"`+ws+`"}]`)
	if id, cand := DevinSessionCorrelate(before, current, nosignal, ws); id != "" || len(cand) != 2 {
		t.Fatalf("two new sessions must be ambiguous, got id=%q cand=%v", id, cand)
	}

	// A hook signal is authoritative over the listing.
	signal := filepath.Join(dir, "signal.json")
	writeFile(t, signal, `{"session_id":"from-hook"}`)
	if id, _ := DevinSessionCorrelate(before, current, signal, ws); id != "from-hook" {
		t.Fatalf("a hook signal must win, got %q", id)
	}
}

func TestWriteUnavailableUsage(t *testing.T) {
	out := filepath.Join(t.TempDir(), "usage.json")
	if err := WriteUnavailableUsage(out); err != nil {
		t.Fatal(err)
	}
	got := readJSONFile(t, out)
	if got["availability"] != "unavailable" || got["providerUnits"] != nil || got["cost"] != nil {
		t.Fatalf("unexpected unavailable usage: %v", got)
	}
}

// --- helpers ---

func toStringSet(items []any) map[string]bool {
	set := map[string]bool{}
	for _, item := range items {
		if s, ok := item.(string); ok {
			set[s] = true
		}
	}
	return set
}

func hookCommandOf(t *testing.T, settings map[string]any) string {
	t.Helper()
	hooks := settings["hooks"].(map[string]any)
	sessionStart := hooks["SessionStart"].([]any)
	entry := sessionStart[0].(map[string]any)
	inner := entry["hooks"].([]any)
	command := inner[0].(map[string]any)
	return command["command"].(string)
}
