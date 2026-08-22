package adapter

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// collectFixture stages a checkout root with the shipped implementer
// schema, a running record, and a schema-valid return produced by the
// fake simulator — the same bytes a compliant delegate would deliver.
type collectFixture struct {
	root, roundDir, workspace string
	record                    string
	validReturn               []byte
	params                    CollectParams
}

func newCollectFixture(t *testing.T) *collectFixture {
	t.Helper()
	f := &collectFixture{root: t.TempDir()}
	schemaDir := filepath.Join(f.root, "scripts", "agents", "schemas")
	if err := os.MkdirAll(schemaDir, 0o755); err != nil {
		t.Fatal(err)
	}
	shipped, err := os.ReadFile(filepath.Join("..", "..", "scripts", "agents", "schemas", "implementer.schema.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(schemaDir, "implementer.schema.json"), shipped, 0o644); err != nil {
		t.Fatal(err)
	}

	f.record = filepath.Join(f.root, "artifacts", "agents", "jobs", "job-1.json")
	if err := os.MkdirAll(filepath.Dir(f.record), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, f.record, `{
	  "jobId": "job-1", "round": 1, "role": "implementer", "status": "running", "runtime": "devin",
	  "sessionId": "sess-1", "requestedModel": "devin-model", "effectiveModel": null
	}`)

	// A schema-valid devin return built locally: devin's collect tests
	// must not depend on the fake runtime's production writer.
	f.validReturn = []byte(`{
	  "schemaVersion": 2, "jobId": "job-1", "round": 1, "runtime": "devin",
	  "sessionId": "sess-1",
	  "model": {"requested": "devin-model", "effective": "unobserved"},
	  "evidence": [{"command": "local fixture", "observed": "canned role return", "level": "ran"}],
	  "gaps": [], "mode": "implement",
	  "riskiestPart": "fixture boundary", "diffBoundary": [],
	  "whatWasDone": "fixture implementation",
	  "claimed": {"sessionId": null, "model": null}
	}`)

	f.roundDir = filepath.Join(f.root, "artifacts", "agents", "job-1", "rounds", "1")
	f.workspace = filepath.Join(f.root, "workspace")
	if err := os.MkdirAll(f.roundDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(f.workspace, 0o755); err != nil {
		t.Fatal(err)
	}
	f.params = CollectParams{
		Root: f.root, Job: "job-1", RoundDir: f.roundDir, Workspace: f.workspace,
		StdoutPath: filepath.Join(f.roundDir, "raw.out"),
		NamedPath:  filepath.Join(f.roundDir, "devin-return.json"),
		RecordPath: f.record, Attempt: "initial", Session: "sess-1",
	}
	return f
}

func (f *collectFixture) transcriptWith(t *testing.T, steps string) string {
	t.Helper()
	path := filepath.Join(f.root, "export.atif.json")
	writeFile(t, path, `{"session_id":"sess-1","steps":[`+steps+`]}`)
	return path
}

func writeStep(id int, filePath, content string) string {
	args, _ := json.Marshal(map[string]string{"file_path": filePath, "content": content})
	return fmt.Sprintf(`{"step_id":%d,"tool_calls":[{"tool_call_id":"t%d","function_name":"write","arguments":%s}]}`, id, id, args)
}

func TestCollectJunkStdoutFallsThroughToValidNamedFile(t *testing.T) {
	f := newCollectFixture(t)
	writeFile(t, f.params.StdoutPath, "I could not produce JSON, sorry")
	writeFile(t, f.params.NamedPath, string(f.validReturn))

	verdict, err := DevinCollect(f.params)
	if err != nil {
		t.Fatal(err)
	}
	if !verdict.Delivered || verdict.Channel != "named-file" {
		t.Fatalf("named file must win over junk stdout: %+v", verdict)
	}
	if len(verdict.Rejected) == 0 || !strings.Contains(verdict.Rejected[0], "stdout") {
		t.Fatalf("the stdout rejection must be named: %+v", verdict.Rejected)
	}
	accepted, err := os.ReadFile(verdict.Reply)
	if err != nil || string(accepted) != string(f.validReturn) {
		t.Fatalf("accepted snapshot must be the raw candidate bytes: %v", err)
	}
	source, err := os.ReadFile(filepath.Join(f.roundDir, "reply-source.json"))
	if err != nil || !strings.Contains(string(source), `"channel": "named-file"`) {
		t.Fatalf("provenance must bind the channel: %v %s", err, source)
	}
}

func TestCollectValidStdoutWins(t *testing.T) {
	f := newCollectFixture(t)
	writeFile(t, f.params.StdoutPath, string(f.validReturn))
	verdict, err := DevinCollect(f.params)
	if err != nil || !verdict.Delivered || verdict.Channel != "stdout" {
		t.Fatalf("valid stdout must deliver: %+v %v", verdict, err)
	}
}

func TestCollectWrongJobStdoutRejectedAtSelection(t *testing.T) {
	f := newCollectFixture(t)
	wrong := strings.Replace(string(f.validReturn), `"job-1"`, `"job-9"`, 1)
	writeFile(t, f.params.StdoutPath, wrong)
	writeFile(t, f.params.NamedPath, string(f.validReturn))
	verdict, err := DevinCollect(f.params)
	if err != nil {
		t.Fatal(err)
	}
	if verdict.Channel != "named-file" {
		t.Fatalf("a schema-valid wrong-job stdout must not shadow the valid named file: %+v", verdict)
	}
}

func TestCollectMiningDesignationAndSuccessOracle(t *testing.T) {
	f := newCollectFixture(t)
	elsewhere := filepath.Join(f.root, "elsewhere")
	if err := os.MkdirAll(elsewhere, 0o755); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(elsewhere, "devin-return.json")
	writeFile(t, target, string(f.validReturn))
	decoy := writeStep(1, filepath.Join(elsewhere, "notes.json"), `{"jobId":"job-1"}`)
	designated := writeStep(2, target, string(f.validReturn))
	f.params.TranscriptPath = f.transcriptWith(t, decoy+","+designated)

	verdict, err := DevinCollect(f.params)
	if err != nil {
		t.Fatal(err)
	}
	if !verdict.Delivered || verdict.Channel != "transcript" {
		t.Fatalf("the designated persisted write must deliver: %+v", verdict)
	}
	source, _ := os.ReadFile(filepath.Join(f.roundDir, "reply-source.json"))
	if !strings.Contains(string(source), `"mining"`) || !strings.Contains(string(source), `"stepId": "2"`) {
		t.Fatalf("mining provenance must carry the audit: %s", source)
	}
}

func TestCollectMiningRefusesNonPersistedWrite(t *testing.T) {
	f := newCollectFixture(t)
	ghost := filepath.Join(f.root, "elsewhere", "devin-return.json")
	f.params.TranscriptPath = f.transcriptWith(t, writeStep(1, ghost, string(f.validReturn)))
	verdict, err := DevinCollect(f.params)
	if err != nil {
		t.Fatal(err)
	}
	if verdict.Delivered {
		t.Fatalf("a write with no file on disk must not deliver: %+v", verdict)
	}
	if !strings.Contains(strings.Join(verdict.Rejected, "|"), "did not persist") {
		t.Fatalf("the oracle rejection must be named: %+v", verdict.Rejected)
	}
}

func TestCollectRepairWatermarkExcludesInitialSteps(t *testing.T) {
	f := newCollectFixture(t)
	target := filepath.Join(f.workspace, "devin-return.json")
	writeFile(t, target, string(f.validReturn))
	early := writeStep(1, target, string(f.validReturn))

	// Initial attempt: two steps, the qualifying write among them, but no
	// stdout/named — mining delivers and the watermark lands at 2.
	f.params.TranscriptPath = f.transcriptWith(t, early+","+writeStep(2, "/x/other.txt", "notes"))
	if _, err := DevinCollect(f.params); err != nil {
		t.Fatal(err)
	}

	// Repair attempt over the CUMULATIVE export: the same early write is
	// now pre-watermark and must not be credited to the repair.
	repair := f.params
	repair.Attempt = "repair"
	repair.NamedPath = filepath.Join(f.roundDir, "devin-return.repair-1.json")
	repair.TranscriptPath = f.transcriptWith(t, early+","+writeStep(2, "/x/other.txt", "notes"))
	verdict, err := DevinCollect(repair)
	if err != nil {
		t.Fatal(err)
	}
	if verdict.Delivered {
		t.Fatalf("pre-watermark material must not deliver in the repair window: %+v", verdict)
	}
	if !verdict.WatermarkValid {
		t.Fatal("the repair read a valid watermark and must say so")
	}
}

func TestCollectPresenceBars(t *testing.T) {
	f := newCollectFixture(t)
	f.params.PresenceOnly = true
	f.params.Session = ""

	verdict, err := DevinCollect(f.params)
	if err != nil || verdict.CandidatesPresent {
		t.Fatalf("nothing on disk means nothing present: %+v %v", verdict, err)
	}

	writeFile(t, f.params.NamedPath, "{ torn")
	verdict, _ = DevinCollect(f.params)
	if verdict.CandidatesPresent {
		t.Fatalf("a torn named file is not present under the shipped bar: %+v", verdict)
	}

	writeFile(t, f.params.StdoutPath, "malformed but non-empty")
	verdict, _ = DevinCollect(f.params)
	if !verdict.CandidatesPresent {
		t.Fatalf("any non-empty stdout is present under the shipped bar: %+v", verdict)
	}
}

func TestCollectFullWalkWithoutSessionRefuses(t *testing.T) {
	f := newCollectFixture(t)
	f.params.Session = ""
	if _, err := DevinCollect(f.params); err == nil {
		t.Fatal("a full collect without a session must refuse; presence-only is the sessionless mode")
	}
}

func TestCollectOversizeCandidateFallsThrough(t *testing.T) {
	f := newCollectFixture(t)
	big := make([]byte, MaxCandidateBytes+1)
	for i := range big {
		big[i] = 'x'
	}
	if err := os.WriteFile(f.params.StdoutPath, big, 0o644); err != nil {
		t.Fatal(err)
	}
	writeFile(t, f.params.NamedPath, string(f.validReturn))
	verdict, err := DevinCollect(f.params)
	if err != nil {
		t.Fatal(err)
	}
	if verdict.Channel != "named-file" {
		t.Fatalf("an oversize stdout must fall through, never go mechanical: %+v", verdict)
	}
	if !strings.Contains(strings.Join(verdict.Rejected, "|"), "over the candidate ceiling") {
		t.Fatalf("the ceiling rejection must be named: %+v", verdict.Rejected)
	}
}

func TestCollectMiningRefusesDivergedOnDiskContent(t *testing.T) {
	f := newCollectFixture(t)
	target := filepath.Join(f.workspace, "devin-return.json")
	writeFile(t, target, `{"tampered": true}`)
	f.params.TranscriptPath = f.transcriptWith(t, writeStep(1, target, string(f.validReturn)))
	verdict, err := DevinCollect(f.params)
	if err != nil {
		t.Fatal(err)
	}
	if verdict.Delivered {
		t.Fatalf("diverged on-disk content must not deliver: %+v", verdict)
	}
	if !strings.Contains(strings.Join(verdict.Rejected, "|"), "diverged") {
		t.Fatalf("the divergence must be named: %+v", verdict.Rejected)
	}
}

func TestCollectRefusesUnknownAttempt(t *testing.T) {
	f := newCollectFixture(t)
	f.params.Attempt = "third-time-lucky"
	f.params.TranscriptPath = f.transcriptWith(t, writeStep(1, "/x.json", "{}"))
	if _, err := DevinCollect(f.params); err == nil {
		t.Fatal("an unknown attempt label must be a mechanical error")
	}
}

func TestCollectNothingAnywhereIsNothingQualified(t *testing.T) {
	f := newCollectFixture(t)
	verdict, err := DevinCollect(f.params)
	if err != nil {
		t.Fatal(err)
	}
	if verdict.Delivered || verdict.CandidatesPresent || verdict.Channel != "none" {
		t.Fatalf("empty everything must report nothing cleanly: %+v", verdict)
	}
	if !strings.Contains(strings.Join(verdict.Rejected, "|"), "no export") {
		t.Fatalf("the absent transcript must be named: %+v", verdict.Rejected)
	}
}

func TestCollectScratchFailureRejectsWithoutMechanicalVerdict(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("root ignores directory modes")
	}
	f := newCollectFixture(t)
	writeFile(t, f.params.StdoutPath, string(f.validReturn))
	if err := os.Chmod(f.roundDir, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(f.roundDir, 0o755) })
	// The candidate is rejected at the scratch branch, and the turn then
	// goes MECHANICAL because provenance itself cannot be written — a
	// harness failure by the design's taxonomy, never a delivery verdict.
	if _, err := DevinCollect(f.params); err == nil {
		t.Fatal("an unwritable round dir must be a mechanical error")
	}
}
