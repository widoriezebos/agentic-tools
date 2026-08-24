package main

// The relay-level differential fixtures for the delegate-seam
// reroute: each rerouted verb runs END TO END — flag parsing, port
// lookup, the port call, printing, exit taxonomy — and its observable
// behavior (exit code, stdout, stderr, output files) is compared
// against expectations computed from the DIRECT owner function plus
// the relay's unchanged printing rules. The port-level differentials
// in the owner packages bind values and bytes on the rich fixture
// beds; these bind the relay composition the shell actually calls.

import (
	"encoding/json"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/adapter"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/host"
)

// captureRelay runs a relay with stdout AND stderr captured.
func captureRelay(t *testing.T, fn func() int) (stdout, stderr string, code int) {
	t.Helper()
	origOut, origErr := os.Stdout, os.Stderr
	outR, outW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	errR, errW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout, os.Stderr = outW, errW
	code = fn()
	outW.Close()
	errW.Close()
	os.Stdout, os.Stderr = origOut, origErr
	outBytes, _ := io.ReadAll(outR)
	errBytes, _ := io.ReadAll(errR)
	return string(outBytes), string(errBytes), code
}

func writeReroute(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func readReroute(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}

// The usage relays: the rerouted verb and the direct owner call
// produce identical output files, and the relay's exit taxonomy
// matches the owner's error behavior — happy and malformed rows.
func TestRerouteUsageRelays(t *testing.T) {
	dir := t.TempDir()
	claudeResult := filepath.Join(dir, "claude-result.json")
	writeReroute(t, claudeResult, `{"usage": {"input_tokens": 9, "output_tokens": 2}, "total_cost_usd": 1.5}`)

	direct := filepath.Join(dir, "claude-direct.json")
	if err := adapter.ClaudeUsage(claudeResult, direct); err != nil {
		t.Fatal(err)
	}
	relayOut := filepath.Join(dir, "claude-relay.json")
	stdout, stderr, code := captureRelay(t, func() int {
		return runAdapterClaudeUsage([]string{"--result", claudeResult, "--output", relayOut})
	})
	if code != 0 || stdout != "" || stderr != "" {
		t.Fatalf("claude-usage relay: code=%d out=%q err=%q", code, stdout, stderr)
	}
	if readReroute(t, direct) != readReroute(t, relayOut) {
		t.Fatal("claude-usage relay output diverges from the direct call")
	}

	codexEvents := filepath.Join(dir, "events.jsonl")
	writeReroute(t, codexEvents, `{"msg":{"type":"token_count","info":{"total_token_usage":{"input_tokens":4,"output_tokens":1}}}}`)
	codexDirect := filepath.Join(dir, "codex-direct.json")
	if err := adapter.CodexUsage(codexEvents, codexDirect); err != nil {
		t.Fatal(err)
	}
	codexRelay := filepath.Join(dir, "codex-relay.json")
	stdout, stderr, code = captureRelay(t, func() int {
		return runAdapterCodexUsage([]string{"--events", codexEvents, "--output", codexRelay})
	})
	if code != 0 || stdout != "" || stderr != "" {
		t.Fatalf("codex-usage relay: code=%d out=%q err=%q", code, stdout, stderr)
	}
	if readReroute(t, codexDirect) != readReroute(t, codexRelay) {
		t.Fatal("codex-usage relay output diverges from the direct call")
	}

	fakeDirect := filepath.Join(dir, "fake-direct.json")
	if err := adapter.WriteFakeUsage(fakeDirect); err != nil {
		t.Fatal(err)
	}
	fakeRelay := filepath.Join(dir, "fake-relay.json")
	stdout, stderr, code = captureRelay(t, func() int {
		return runAdapterFakeUsage([]string{"--output", fakeRelay})
	})
	if code != 0 || stdout != "" || stderr != "" {
		t.Fatalf("fake-usage relay: code=%d out=%q err=%q", code, stdout, stderr)
	}
	if readReroute(t, fakeDirect) != readReroute(t, fakeRelay) {
		t.Fatal("fake-usage relay diverges from the direct call")
	}
}

// The result-field relay prints exactly what the direct call answers,
// present and absent alike, with the unchanged exit taxonomy.
func TestRerouteResultFieldRelay(t *testing.T) {
	dir := t.TempDir()
	result := filepath.Join(dir, "result.json")
	writeReroute(t, result, `{"session_id": "sess-9"}`)

	value, present, err := adapter.ClaudeResultField(result, "session_id")
	if err != nil || !present {
		t.Fatal(err)
	}
	stdout, stderr, code := captureRelay(t, func() int {
		return runAdapterClaudeResultField([]string{"--result", result, "--field", "session_id"})
	})
	if code != 0 || stdout != value+"\n" || stderr != "" {
		t.Fatalf("result-field relay diverges: code=%d out=%q err=%q direct=%q", code, stdout, stderr, value)
	}
	absentValue, absentPresent, absentErr := adapter.ClaudeResultField(result, "no_such")
	absentOut, absentErrStream, absentCode := captureRelay(t, func() int {
		return runAdapterClaudeResultField([]string{"--result", result, "--field", "no_such"})
	})
	wantOut := ""
	if absentErr == nil && absentPresent {
		wantOut = absentValue + "\n"
	}
	wantCode, wantErrStream := 0, ""
	if absentErr != nil {
		wantCode, wantErrStream = 1, absentErr.Error()+"\n"
	}
	if absentCode != wantCode || absentOut != wantOut || absentErrStream != wantErrStream {
		t.Fatalf("absent-field taxonomy diverges: direct (%q,%v,%v) relay code=%d out=%q err=%q",
			absentValue, absentPresent, absentErr, absentCode, absentOut, absentErrStream)
	}
}

// The settle and devin-usage relays fail exactly as the direct calls
// on the same inputs — stderr text and exit taxonomy.
func TestRerouteDevinErrorRelays(t *testing.T) {
	dir := t.TempDir()
	absent := filepath.Join(dir, "absent")
	directModel, directCertified, directErr := adapter.DevinSettle(absent, "", "sess", dir, true)
	stdout, stderr, code := captureRelay(t, func() int {
		return runAdapterDevinSettle([]string{"--transcript", absent, "--session", "sess", "--round-dir", dir, "--require-transcript"})
	})
	wantCode := 1
	if directErr == nil && directCertified {
		wantCode = 0
	}
	wantOut := ""
	if directErr == nil && directModel != "" {
		wantOut = directModel + "\n"
	}
	wantErr := ""
	if directErr != nil {
		wantErr = directErr.Error() + "\n"
	}
	if code != wantCode || stdout != wantOut || stderr != wantErr {
		t.Fatalf("settle relay diverges: direct (%q,%v,%v) relay code=%d out=%q err=%q",
			directModel, directCertified, directErr, code, stdout, stderr)
	}

	directUsageOut := filepath.Join(dir, "du.json")
	relayUsageOut := filepath.Join(dir, "ru.json")
	directUsageErr := adapter.DevinTurnUsage(directUsageOut, absent, "", filepath.Join(dir, "dc.json"), "", false)
	stdout, stderr, code = captureRelay(t, func() int {
		return runAdapterDevinUsage([]string{"--usage", relayUsageOut, "--transcript", absent, "--cumulative", filepath.Join(dir, "rc.json")})
	})
	wantUsageCode, wantUsageErr := 0, ""
	if directUsageErr != nil {
		wantUsageCode, wantUsageErr = 1, directUsageErr.Error()+"\n"
	}
	if code != wantUsageCode || stdout != "" || stderr != wantUsageErr {
		t.Fatalf("devin-usage relay diverges: code=%d out=%q stderr=%q direct=%v", code, stdout, stderr, directUsageErr)
	}
	if directUsageErr == nil {
		if readReroute(t, directUsageOut) != readReroute(t, relayUsageOut) {
			t.Fatal("devin-usage relay output diverges from the direct call")
		}
	}

	hostDirectOut := filepath.Join(dir, "hd.json")
	hostRelayOut := filepath.Join(dir, "hr.json")
	hostUsageErr := host.HostDevinUsage(hostDirectOut, absent, filepath.Join(dir, "hdc.json"), "", false)
	stdout, stderr, code = captureRelay(t, func() int {
		return runHostDevinUsage([]string{"--usage", hostRelayOut, "--transcript", absent, "--cumulative", filepath.Join(dir, "hrc.json")})
	})
	wantHostCode, wantHostErr := 0, ""
	if hostUsageErr != nil {
		wantHostCode, wantHostErr = 1, hostUsageErr.Error()+"\n"
	}
	if code != wantHostCode || stdout != "" || stderr != wantHostErr {
		t.Fatalf("host devin-usage relay diverges: code=%d out=%q stderr=%q direct=%v", code, stdout, stderr, hostUsageErr)
	}
	if hostUsageErr == nil {
		if readReroute(t, hostDirectOut) != readReroute(t, hostRelayOut) {
			t.Fatal("host devin-usage relay output diverges from the direct call")
		}
	}
}

// The host result and return relays write the same bytes as the
// direct calls.
func TestRerouteHostResultRelays(t *testing.T) {
	dir := t.TempDir()
	provider := filepath.Join(dir, "provider.json")
	writeReroute(t, provider, `{"result": "{\"status\":\"done\"}", "usage": {"input_tokens": 2}}`)
	dReturn, dUsage := filepath.Join(dir, "d-ret.json"), filepath.Join(dir, "d-use.json")
	if err := host.ClaudeResult(provider, dReturn, dUsage); err != nil {
		t.Fatal(err)
	}
	rReturn, rUsage := filepath.Join(dir, "r-ret.json"), filepath.Join(dir, "r-use.json")
	stdout, stderr, code := captureRelay(t, func() int {
		return runHostClaudeResult([]string{"--provider", provider, "--return", rReturn, "--usage", rUsage})
	})
	if code != 0 || stdout != "" || stderr != "" {
		t.Fatalf("host claude-result relay: code=%d out=%q err=%q", code, stdout, stderr)
	}
	if readReroute(t, dReturn) != readReroute(t, rReturn) || readReroute(t, dUsage) != readReroute(t, rUsage) {
		t.Fatal("host claude-result relay diverges from the direct call")
	}

	raw := filepath.Join(dir, "raw.out")
	writeReroute(t, raw, `noise {"status":"ok"} tail`)
	dOut := filepath.Join(dir, "d-return-extract.json")
	if err := host.DevinReturn(raw, dOut); err != nil {
		t.Fatal(err)
	}
	rOut := filepath.Join(dir, "r-return-extract.json")
	stdout, stderr, code = captureRelay(t, func() int {
		return runHostDevinReturn([]string{"--raw", raw, "--output", rOut})
	})
	if code != 0 || stdout != "" || stderr != "" {
		t.Fatalf("host devin-return relay: code=%d out=%q err=%q", code, stdout, stderr)
	}
	if readReroute(t, dOut) != readReroute(t, rOut) {
		t.Fatal("host devin-return relay diverges from the direct call")
	}
}

// The fake relays match the direct calls: the result envelope bytes
// and the return-write behavior.
func TestRerouteFakeRelays(t *testing.T) {
	dir := t.TempDir()
	raw := filepath.Join(dir, "raw.out")
	writeReroute(t, raw, "fake raw")
	dResult := filepath.Join(dir, "d-result.json")
	if err := host.FakeResult(dResult, "sess", raw, "", "failed"); err != nil {
		t.Fatal(err)
	}
	rResult := filepath.Join(dir, "r-result.json")
	stdout, stderr, code := captureRelay(t, func() int {
		return runHostFakeResult([]string{"--result", rResult, "--session", "sess", "--raw", raw, "--outcome", "failed"})
	})
	if code != 0 || stdout != "" || stderr != "" {
		t.Fatalf("host fake-result relay: code=%d out=%q err=%q", code, stdout, stderr)
	}
	if readReroute(t, dResult) != readReroute(t, rResult) {
		t.Fatal("host fake-result relay diverges from the direct call")
	}

	prompt := filepath.Join(dir, "prompt.md")
	writeReroute(t, prompt, "FAKEHOST:return-ok")
	directErr := adapter.WriteFakeReturn(filepath.Join(dir, "absent-record"), prompt, filepath.Join(dir, "dr.json"))
	if directErr == nil {
		t.Fatal("the absent record must refuse; the row binds the error path")
	}
	stdout, stderr, code = captureRelay(t, func() int {
		return runAdapterFakeReturn([]string{"--record", filepath.Join(dir, "absent-record"), "--prompt", prompt, "--output", filepath.Join(dir, "rr.json")})
	})
	if code != 1 || stdout != "" || stderr != directErr.Error()+"\n" {
		t.Fatalf("adapter fake-return relay error diverges: code=%d out=%q stderr=%q direct=%v", code, stdout, stderr, directErr)
	}
}

// The collect relays' exit taxonomy and flag mapping, end to end: a
// mechanical failure exits 1 with the owner's error on stderr; the
// delivered-with-validation path is bound at port level on the rich
// fixture bed (TestCollectPortMatchesDirect) — this binds the relay's
// own composition the shell calls.
func TestRerouteCollectRelays(t *testing.T) {
	dir := t.TempDir()
	directVerdict, directErr := adapter.DevinCollect(adapter.CollectParams{
		Root:       dir,
		Job:        "j1",
		RoundDir:   dir,
		RecordPath: filepath.Join(dir, "absent-record.json"),
		Session:    "sess",
	})
	stdout, stderr, code := captureRelay(t, func() int {
		return runAdapterDevinCollect([]string{
			"--root", dir, "--job", "j1", "--round-dir", dir,
			"--record", filepath.Join(dir, "absent-record.json"), "--session", "sess"})
	})
	if directErr != nil {
		if code != 1 || stdout != "" || stderr != directErr.Error()+"\n" {
			t.Fatalf("adapter devin-collect relay error diverges: code=%d out=%q stderr=%q direct=%v", code, stdout, stderr, directErr)
		}
	} else {
		wantBytes, err := json.Marshal(directVerdict)
		if err != nil {
			t.Fatal(err)
		}
		wantCode := 3
		if directVerdict.Delivered {
			wantCode = 0
		}
		if code != wantCode || stdout != string(wantBytes)+"\n" || stderr != "" {
			t.Fatalf("adapter devin-collect relay diverges: code=%d (want %d) err=%q\nrelay: %q\ndirect: %s",
				code, wantCode, stderr, stdout, wantBytes)
		}
	}

	hostVerdict, hostErr := host.HostDevinCollect(host.HostCollectParams{
		Root:           dir,
		TurnRecordPath: filepath.Join(dir, "absent-turn.json"),
		TurnDir:        dir,
	})
	stdout, stderr, code = captureRelay(t, func() int {
		return runHostDevinCollect([]string{
			"--root", dir, "--turn-record", filepath.Join(dir, "absent-turn.json"), "--turn-dir", dir})
	})
	if hostErr != nil {
		if code != 1 || stdout != "" || stderr != hostErr.Error()+"\n" {
			t.Fatalf("host devin-collect relay error diverges: code=%d out=%q stderr=%q direct=%v", code, stdout, stderr, hostErr)
		}
	} else {
		wantBytes, err := json.Marshal(hostVerdict)
		if err != nil {
			t.Fatal(err)
		}
		wantCode := 3
		if hostVerdict.Delivered {
			wantCode = 0
		}
		if code != wantCode || stdout != string(wantBytes)+"\n" || stderr != "" {
			t.Fatalf("host devin-collect relay diverges: code=%d (want %d) err=%q\nrelay: %q\ndirect: %s",
				code, wantCode, stderr, stdout, wantBytes)
		}
	}
}

// rerouteTreePaths lists every file under root as sorted relative
// paths — the side-effect set a verb left behind.
func rerouteTreePaths(t *testing.T, root string) []string {
	t.Helper()
	var paths []string
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		paths = append(paths, rel)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(paths)
	return paths
}

// The host fake-return relay on the dispatch-terminal happy path: the
// return object bytes and the whole side-effect file set (the job
// record the behavior writes) match the direct call, with clean
// streams and exit 0.
func TestRerouteHostFakeReturnHappy(t *testing.T) {
	const turnBody = `{"turnId": "t-1", "missionId": "m-1", "cycle": 2, "model": "fable", "startedAt": "2026-08-10T00:00:00Z", "hostSession": null}`
	const stateBody = `{"streams": {"beta": {"state": "parked"}, "alpha": {"state": "active"}}}`
	directRoot, relayRoot := t.TempDir(), t.TempDir()
	for _, root := range []string{directRoot, relayRoot} {
		writeReroute(t, filepath.Join(root, "turn.json"), turnBody)
		writeReroute(t, filepath.Join(root, "state.json"), stateBody)
	}
	directOut := filepath.Join(directRoot, "return.json")
	if err := host.FakeReturn(filepath.Join(directRoot, "turn.json"), filepath.Join(directRoot, "state.json"),
		directOut, "dispatch-terminal", directRoot); err != nil {
		t.Fatal(err)
	}
	relayOut := filepath.Join(relayRoot, "return.json")
	stdout, stderr, code := captureRelay(t, func() int {
		return runHostFakeReturn([]string{
			"--turn", filepath.Join(relayRoot, "turn.json"), "--state", filepath.Join(relayRoot, "state.json"),
			"--output", relayOut, "--behavior", "dispatch-terminal", "--root", relayRoot})
	})
	if code != 0 || stdout != "" || stderr != "" {
		t.Fatalf("host fake-return relay: code=%d out=%q err=%q", code, stdout, stderr)
	}
	if readReroute(t, directOut) != readReroute(t, relayOut) {
		t.Fatal("host fake-return relay output diverges from the direct call")
	}
	directPaths := rerouteTreePaths(t, directRoot)
	relayPaths := rerouteTreePaths(t, relayRoot)
	if strings.Join(directPaths, "\n") != strings.Join(relayPaths, "\n") {
		t.Fatalf("side-effect file sets diverge:\ndirect: %v\nrelay:  %v", directPaths, relayPaths)
	}
	for _, rel := range directPaths {
		if readReroute(t, filepath.Join(directRoot, rel)) != readReroute(t, filepath.Join(relayRoot, rel)) {
			t.Fatalf("side-effect file %s diverges between the two roots", rel)
		}
	}
	if _, err := os.Stat(filepath.Join(directRoot, "artifacts", "agents", "jobs", "verifier-m-1.json")); err != nil {
		t.Fatal("the dispatch-terminal behavior must write the job record; the row binds the side-effect set")
	}
}

// The devin-usage relay on the resumed-delta path: a transcript with
// full session totals, a predecessor's cumulative file, and a per-side
// snapshot; the usage and cumulative bytes match the direct call with
// clean streams and exit 0.
func TestRerouteDevinUsageResumedDelta(t *testing.T) {
	dir := t.TempDir()
	transcript := filepath.Join(dir, "transcript.json")
	writeReroute(t, transcript, `{"final_metrics":{"total_prompt_tokens":25799,"total_completion_tokens":1200,"total_cached_tokens":900,"total_steps":40}}`)
	previous := filepath.Join(dir, "previous.json")
	writeReroute(t, previous, `{"total_prompt_tokens":12833,"total_completion_tokens":700,"total_cached_tokens":400,"total_steps":22}`)

	directUsage := filepath.Join(dir, "d-usage.json")
	directCumulative := filepath.Join(dir, "d-cumulative.json")
	if err := adapter.DevinTurnUsage(directUsage, transcript, filepath.Join(dir, "d-snapshot.json"),
		directCumulative, previous, true); err != nil {
		t.Fatal(err)
	}
	relayUsage := filepath.Join(dir, "r-usage.json")
	relayCumulative := filepath.Join(dir, "r-cumulative.json")
	stdout, stderr, code := captureRelay(t, func() int {
		return runAdapterDevinUsage([]string{
			"--usage", relayUsage, "--transcript", transcript, "--snapshot", filepath.Join(dir, "r-snapshot.json"),
			"--cumulative", relayCumulative, "--previous", previous, "--expect-previous"})
	})
	if code != 0 || stdout != "" || stderr != "" {
		t.Fatalf("devin-usage relay: code=%d out=%q err=%q", code, stdout, stderr)
	}
	if readReroute(t, directUsage) != readReroute(t, relayUsage) {
		t.Fatal("devin-usage relay usage bytes diverge from the direct call")
	}
	if readReroute(t, directCumulative) != readReroute(t, relayCumulative) {
		t.Fatal("devin-usage relay cumulative bytes diverge from the direct call")
	}
	var usage struct {
		Availability string  `json:"availability"`
		InputTokens  float64 `json:"inputTokens"`
	}
	if err := json.Unmarshal([]byte(readReroute(t, relayUsage)), &usage); err != nil {
		t.Fatal(err)
	}
	if usage.Availability != "native" || usage.InputTokens != 25799-12833 {
		t.Fatalf("the row must exercise the resumed delta: %+v", usage)
	}
}

// The result-field relay's model collapse, end to end: one model
// collapses to its key, several to the derived multi-model list, none
// to unobserved; a present null prints nothing; a malformed document
// is the owner's error verbatim on stderr with exit 1.
func TestRerouteClaudeResultFieldModelRows(t *testing.T) {
	dir := t.TempDir()
	rows := []struct {
		name, body, field string
	}{
		{"single-model", `{"modelUsage": {"claude-fable-5": {"inputTokens": 3}}}`, "model"},
		{"multi-model", `{"modelUsage": {"model-b": {}, "model-a": {}}}`, "model"},
		{"zero-model", `{"modelUsage": {}}`, "model"},
		{"present-null", `{"effectiveModel": null}`, "effectiveModel"},
		{"malformed", `{ torn`, "model"},
	}
	for _, row := range rows {
		result := filepath.Join(dir, row.name+".json")
		writeReroute(t, result, row.body)
		value, print, err := adapter.ClaudeResultField(result, row.field)
		switch row.name {
		case "single-model":
			if err != nil || !print || value != "claude-fable-5" {
				t.Fatalf("single-model must collapse to the one model: (%q,%v,%v)", value, print, err)
			}
		case "multi-model", "zero-model":
			if err != nil || !print {
				t.Fatalf("%s must answer a printable value: (%q,%v,%v)", row.name, value, print, err)
			}
		case "present-null":
			if err != nil || print {
				t.Fatalf("a present null must print nothing: (%q,%v,%v)", value, print, err)
			}
		case "malformed":
			if err == nil {
				t.Fatal("a malformed result document must error")
			}
		}
		wantOut := ""
		if err == nil && print {
			wantOut = value + "\n"
		}
		wantCode, wantErr := 0, ""
		if err != nil {
			wantCode, wantErr = 1, err.Error()+"\n"
		}
		stdout, stderr, code := captureRelay(t, func() int {
			return runAdapterClaudeResultField([]string{"--result", result, "--field", row.field})
		})
		if code != wantCode || stdout != wantOut || stderr != wantErr {
			t.Fatalf("%s: direct (%q,%v,%v) relay code=%d out=%q err=%q",
				row.name, value, print, err, code, stdout, stderr)
		}
	}
}

// The settle relay on the certified and disagreement shapes: exit and
// stream bytes computed from the direct call, and the disagreement
// artifact matching across the two round dirs with roots normalized.
func TestRerouteDevinSettleCertifiedAndDisagreement(t *testing.T) {
	dir := t.TempDir()
	transcript := filepath.Join(dir, "transcript.json")
	writeReroute(t, transcript, `{"session_id":"sess-1","agent":{"model_name":"SWE-1.7"}}`)

	for _, row := range []struct {
		name, session string
	}{
		{"certified", "sess-1"},
		{"disagreement", "sess-OTHER"},
	} {
		directDir, relayDir := t.TempDir(), t.TempDir()
		directModel, directCertified, directErr := adapter.DevinSettle(transcript, "", row.session, directDir, false)
		if directErr != nil {
			t.Fatalf("%s: %v", row.name, directErr)
		}
		if directCertified != (row.name == "certified") {
			t.Fatalf("%s must exercise its own certification shape: (%q,%v)", row.name, directModel, directCertified)
		}
		wantCode := 1
		if directCertified {
			wantCode = 0
		}
		wantOut := ""
		if directModel != "" {
			wantOut = directModel + "\n"
		}
		stdout, stderr, code := captureRelay(t, func() int {
			return runAdapterDevinSettle([]string{"--transcript", transcript, "--session", row.session, "--round-dir", relayDir})
		})
		if code != wantCode || stdout != wantOut || stderr != "" {
			t.Fatalf("%s: direct (%q,%v) relay code=%d out=%q err=%q",
				row.name, directModel, directCertified, code, stdout, stderr)
		}
		directBody, directReadErr := os.ReadFile(filepath.Join(directDir, "session-disagreement.txt"))
		relayBody, relayReadErr := os.ReadFile(filepath.Join(relayDir, "session-disagreement.txt"))
		if directReadErr != nil && !os.IsNotExist(directReadErr) {
			t.Fatalf("%s: %v", row.name, directReadErr)
		}
		if os.IsNotExist(directReadErr) != os.IsNotExist(relayReadErr) {
			t.Fatalf("%s: disagreement artifact presence diverges: direct %v relay %v",
				row.name, directReadErr, relayReadErr)
		}
		if directReadErr == nil {
			normalizedDirect := strings.ReplaceAll(string(directBody), directDir, "ROOT")
			normalizedRelay := strings.ReplaceAll(string(relayBody), relayDir, "ROOT")
			if normalizedDirect != normalizedRelay {
				t.Fatalf("%s: disagreement artifact diverges:\ndirect: %q\nrelay:  %q",
					row.name, normalizedDirect, normalizedRelay)
			}
		}
	}
}

// The host devin-return relay when the raw output holds no JSON
// object: the same exit, the same clean streams, and the same
// output-file absence as the direct call.
func TestRerouteHostDevinReturnNoObject(t *testing.T) {
	dir := t.TempDir()
	raw := filepath.Join(dir, "raw.out")
	writeReroute(t, raw, "no json here")
	directOut := filepath.Join(dir, "d-return.json")
	directErr := host.DevinReturn(raw, directOut)
	wantCode, wantErr := 0, ""
	if directErr != nil {
		wantCode, wantErr = 1, directErr.Error()+"\n"
	}
	relayOut := filepath.Join(dir, "r-return.json")
	stdout, stderr, code := captureRelay(t, func() int {
		return runHostDevinReturn([]string{"--raw", raw, "--output", relayOut})
	})
	if code != wantCode || stdout != "" || stderr != wantErr {
		t.Fatalf("host devin-return no-object relay: code=%d out=%q err=%q direct=%v", code, stdout, stderr, directErr)
	}
	_, directStat := os.Stat(directOut)
	_, relayStat := os.Stat(relayOut)
	if !os.IsNotExist(directStat) || !os.IsNotExist(relayStat) {
		t.Fatalf("no object must leave both outputs absent: direct %v relay %v", directStat, relayStat)
	}
}
