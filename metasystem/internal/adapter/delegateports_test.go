package adapter

// The differential fixtures the seam registration answers to: the
// same inputs through the direct owner function and through the
// registered port must produce identical outcomes — bytes, values,
// and error text. For the direct-assignment ports the compiler binds
// the signatures; these fixtures bind the REGISTRY (the right
// function under the right key) and the two mapped ports (the fake
// usage closure, the collect field copy).

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/atif"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/delegate"
)

// The registry serves the owner functions under the right keys: the
// claude usage port and the direct call produce identical bytes on
// the same input, and the codex port is not the claude one.
func TestUsagePortsMatchDirectCalls(t *testing.T) {
	dir := t.TempDir()
	result := filepath.Join(dir, "result.json")
	writeFile(t, result, `{"usage": {"input_tokens": 7, "output_tokens": 3}, "total_cost_usd": 0.5}`)

	direct := filepath.Join(dir, "direct.json")
	ported := filepath.Join(dir, "ported.json")
	if err := ClaudeUsage(result, direct); err != nil {
		t.Fatal(err)
	}
	claudePorts, err := delegate.PortsFor("claude")
	if err != nil || claudePorts.Usage == nil {
		t.Fatalf("claude usage port: %v", err)
	}
	if err := claudePorts.Usage(result, ported); err != nil {
		t.Fatal(err)
	}
	mustEqualFiles(t, direct, ported)

	// The codex key serves codex's extractor, not claude's: on a
	// claude-shaped result the two ports must disagree (codex reads
	// an event log and reports unavailability).
	codexPorts, err := delegate.PortsFor("codex")
	if err != nil || codexPorts.Usage == nil {
		t.Fatalf("codex usage port: %v", err)
	}
	crossed := filepath.Join(dir, "crossed.json")
	if err := codexPorts.Usage(result, crossed); err != nil {
		t.Fatal(err)
	}
	if a, b := readFileBytes(t, ported), readFileBytes(t, crossed); string(a) == string(b) {
		t.Fatal("the codex key must not serve the claude extractor")
	}
}

// The result-field port answers exactly as the direct call, present
// and absent alike.
func TestResultFieldPortMatchesDirect(t *testing.T) {
	dir := t.TempDir()
	result := filepath.Join(dir, "result.json")
	writeFile(t, result, `{"session_id": "sess-1", "model": "m"}`)
	ports, err := delegate.PortsFor("claude")
	if err != nil || ports.ResultField == nil {
		t.Fatalf("claude result-field port: %v", err)
	}
	for _, field := range []string{"session_id", "absent_field"} {
		dv, dp, derr := ClaudeResultField(result, field)
		pv, pp, perr := ports.ResultField(result, field)
		if dv != pv || dp != pp || errorText(derr) != errorText(perr) {
			t.Fatalf("field %q diverges: direct (%q,%v,%v) port (%q,%v,%v)",
				field, dv, dp, derr, pv, pp, perr)
		}
	}
}

// The settle and turn-usage ports fail exactly as the direct calls
// on the same missing-artifact inputs — error text included.
func TestDevinPortsMatchDirectErrors(t *testing.T) {
	dir := t.TempDir()
	absent := filepath.Join(dir, "absent")
	ports, err := delegate.PortsFor("devin")
	if err != nil {
		t.Fatal(err)
	}
	if ports.Settle == nil || ports.TurnUsage == nil {
		t.Fatal("devin must register settle and turn-usage ports")
	}
	dm, dc, derr := DevinSettle(absent, "", "sess", dir, true)
	pm, pc, perr := ports.Settle(absent, "", "sess", dir, true)
	if dm != pm || dc != pc || errorText(derr) != errorText(perr) {
		t.Fatalf("settle diverges: (%q,%v,%v) vs (%q,%v,%v)", dm, dc, derr, pm, pc, perr)
	}
	dOut := filepath.Join(dir, "direct-usage.json")
	pOut := filepath.Join(dir, "ported-usage.json")
	derr = DevinTurnUsage(dOut, absent, "", "", "", false)
	perr = ports.TurnUsage(pOut, absent, "", "", "", false)
	if errorText(derr) != errorText(perr) {
		t.Fatalf("turn-usage errors diverge: %v vs %v", derr, perr)
	}
	if portFileExists(dOut) != portFileExists(pOut) {
		t.Fatal("turn-usage output presence diverges")
	}
	if portFileExists(dOut) {
		mustEqualFiles(t, dOut, pOut)
	}
}

// The fake ports match their direct calls byte for byte.
func TestFakePortsMatchDirect(t *testing.T) {
	dir := t.TempDir()
	ports, err := delegate.PortsFor("fake")
	if err != nil || ports.Usage == nil || ports.Return == nil {
		t.Fatalf("fake ports: %v", err)
	}
	direct := filepath.Join(dir, "direct-usage.json")
	ported := filepath.Join(dir, "ported-usage.json")
	if err := WriteFakeUsage(direct); err != nil {
		t.Fatal(err)
	}
	if err := ports.Usage("ignored", ported); err != nil {
		t.Fatal(err)
	}
	mustEqualFiles(t, direct, ported)

	derr := WriteFakeReturn(filepath.Join(dir, "absent-record"), "", filepath.Join(dir, "d.json"))
	perr := ports.Return(filepath.Join(dir, "absent-record"), "", filepath.Join(dir, "p.json"))
	if errorText(derr) != errorText(perr) {
		t.Fatalf("fake return errors diverge: %v vs %v", derr, perr)
	}
}

// The collect port's field mapping and marshaling match the direct
// walk on cloned fixtures: identical verdict bytes (paths
// normalized), identical delivered flag, identical accepted-snapshot
// bytes.
func TestCollectPortMatchesDirect(t *testing.T) {
	directFixture := newCollectFixture(t)
	portFixture := newCollectFixture(t)
	writeFile(t, directFixture.params.StdoutPath, string(directFixture.validReturn))
	writeFile(t, portFixture.params.StdoutPath, string(portFixture.validReturn))

	directVerdict, err := DevinCollect(directFixture.params)
	if err != nil {
		t.Fatal(err)
	}
	ports, err := delegate.PortsFor("devin")
	if err != nil || ports.Collect == nil {
		t.Fatalf("devin collect port: %v", err)
	}
	in := portFixture.params
	encoded, delivered, err := ports.Collect(delegate.CollectInputs{
		Root:           in.Root,
		Job:            in.Job,
		RoundDir:       in.RoundDir,
		Workspace:      in.Workspace,
		StdoutPath:     in.StdoutPath,
		NamedPath:      in.NamedPath,
		TranscriptPath: in.TranscriptPath,
		ACPOutcomePath: in.ACPOutcomePath,
		RecordPath:     in.RecordPath,
		Attempt:        in.Attempt,
		Session:        in.Session,
		PresenceOnly:   in.PresenceOnly,
	})
	if err != nil {
		t.Fatal(err)
	}
	if delivered != directVerdict.Delivered {
		t.Fatalf("delivered flag diverges: direct %v, port %v", directVerdict.Delivered, delivered)
	}
	directBytes, err := json.Marshal(directVerdict)
	if err != nil {
		t.Fatal(err)
	}
	normalizedDirect := strings.ReplaceAll(string(directBytes), directFixture.params.Root, "ROOT")
	normalizedPort := strings.ReplaceAll(string(encoded), portFixture.params.Root, "ROOT")
	if normalizedDirect != normalizedPort {
		t.Fatalf("verdict bytes diverge:\ndirect: %s\nport:   %s", normalizedDirect, normalizedPort)
	}
}

// collectDifferential holds one channel comparison: two independent
// beds with identical setup, the direct owner run on one and the
// registered port on the other.
type collectDifferential struct {
	directBed, portBed *collectFixture
	verdict            *CollectVerdict
	directErr, portErr error
}

// runCollectDifferential applies the same setup to two fresh beds,
// runs both sides, and binds error text, delivered flag, and verdict
// bytes with each bed's root normalized to ROOT. On errors the two
// texts must match; the verdict comparison is only reached when both
// sides succeeded.
func runCollectDifferential(t *testing.T, setup func(f *collectFixture)) collectDifferential {
	t.Helper()
	d := collectDifferential{directBed: newCollectFixture(t), portBed: newCollectFixture(t)}
	setup(d.directBed)
	setup(d.portBed)

	d.verdict, d.directErr = DevinCollect(d.directBed.params)
	ports, err := delegate.PortsFor("devin")
	if err != nil || ports.Collect == nil {
		t.Fatalf("devin collect port: %v", err)
	}
	in := d.portBed.params
	encoded, delivered, portErr := ports.Collect(delegate.CollectInputs{
		Root:           in.Root,
		Job:            in.Job,
		RoundDir:       in.RoundDir,
		Workspace:      in.Workspace,
		StdoutPath:     in.StdoutPath,
		NamedPath:      in.NamedPath,
		TranscriptPath: in.TranscriptPath,
		ACPOutcomePath: in.ACPOutcomePath,
		RecordPath:     in.RecordPath,
		Attempt:        in.Attempt,
		Session:        in.Session,
		PresenceOnly:   in.PresenceOnly,
	})
	d.portErr = portErr
	if errorText(d.directErr) != errorText(d.portErr) {
		t.Fatalf("collect errors diverge: direct %v, port %v", d.directErr, d.portErr)
	}
	if d.directErr != nil {
		return d
	}
	if delivered != d.verdict.Delivered {
		t.Fatalf("delivered flag diverges: direct %v, port %v", d.verdict.Delivered, delivered)
	}
	directBytes, err := json.Marshal(d.verdict)
	if err != nil {
		t.Fatal(err)
	}
	normalizedDirect := strings.ReplaceAll(string(directBytes), d.directBed.params.Root, "ROOT")
	normalizedPort := strings.ReplaceAll(string(encoded), d.portBed.params.Root, "ROOT")
	if normalizedDirect != normalizedPort {
		t.Fatalf("verdict bytes diverge:\ndirect: %s\nport:   %s", normalizedDirect, normalizedPort)
	}
	return d
}

// The named-file channel through the port: junk stdout falls through
// and the valid named file delivers identically on both sides.
func TestCollectPortNamedFileChannel(t *testing.T) {
	d := runCollectDifferential(t, func(f *collectFixture) {
		writeFile(t, f.params.StdoutPath, "I could not produce JSON, sorry")
		writeFile(t, f.params.NamedPath, string(f.validReturn))
	})
	if !d.verdict.Delivered || d.verdict.Channel != "named-file" {
		t.Fatalf("the walk must deliver via the named file: %+v", d.verdict)
	}
}

// The transcript channel through the port: a designated, persisted
// write delivers identically on both sides.
func TestCollectPortTranscriptChannel(t *testing.T) {
	d := runCollectDifferential(t, func(f *collectFixture) {
		target := filepath.Join(f.workspace, "devin-return.json")
		writeFile(t, target, string(f.validReturn))
		f.params.TranscriptPath = f.transcriptWith(t, writeStep(1, target, string(f.validReturn)))
	})
	if !d.verdict.Delivered || d.verdict.Channel != "transcript" {
		t.Fatalf("the walk must deliver via the transcript: %+v", d.verdict)
	}
}

// Presence-only through the port: the empty bed and the bed with a
// torn named file plus non-empty stdout answer identically, and the
// presence bar itself behaves as shipped on both sides.
func TestCollectPortPresenceOnly(t *testing.T) {
	empty := runCollectDifferential(t, func(f *collectFixture) {
		f.params.PresenceOnly = true
		f.params.Session = ""
	})
	if empty.verdict.CandidatesPresent {
		t.Fatalf("nothing on disk means nothing present: %+v", empty.verdict)
	}
	present := runCollectDifferential(t, func(f *collectFixture) {
		f.params.PresenceOnly = true
		f.params.Session = ""
		writeFile(t, f.params.NamedPath, "{ torn")
		writeFile(t, f.params.StdoutPath, "malformed but non-empty")
	})
	if !present.verdict.CandidatesPresent {
		t.Fatalf("non-empty stdout is present under the shipped bar: %+v", present.verdict)
	}
}

// The acp channel through the port: exclusive delivery beside a
// poisoned legacy channel, identical on both sides.
func TestCollectPortACPChannel(t *testing.T) {
	d := runCollectDifferential(t, func(f *collectFixture) {
		candidate, err := json.Marshal(string(f.validReturn))
		if err != nil {
			t.Fatal(err)
		}
		f.params.ACPOutcomePath = f.acpOutcomeWith(t,
			`{"row":"delivered","sessionId":"sess-1","stopReason":"end_turn","candidate":`+string(candidate)+`}`)
		writeFile(t, f.params.StdoutPath, "junk that would reject")
	})
	if !d.verdict.Delivered || d.verdict.Channel != "acp" {
		t.Fatalf("the walk must deliver via acp: %+v", d.verdict)
	}
}

// An over-ceiling transcript is the same mechanical error on both
// sides, and both errors carry the oversize sentinel.
func TestCollectPortOversizeTranscript(t *testing.T) {
	d := runCollectDifferential(t, func(f *collectFixture) {
		big := filepath.Join(f.root, "big.json")
		body := append([]byte(`{"steps":[`), make([]byte, 9<<20)...)
		if err := os.WriteFile(big, append(body, ']', '}'), 0o644); err != nil {
			t.Fatal(err)
		}
		f.params.TranscriptPath = big
	})
	if d.directErr == nil || d.portErr == nil {
		t.Fatalf("an oversize transcript must error on both sides: direct %v, port %v", d.directErr, d.portErr)
	}
	if !errors.Is(d.directErr, atif.ErrOversize) || !errors.Is(d.portErr, atif.ErrOversize) {
		t.Fatalf("both sides must carry the oversize sentinel: direct %v, port %v", d.directErr, d.portErr)
	}
}

// Provenance bytes: after a delivered walk the two beds' round-dir
// reply-source.json records match with roots normalized.
func TestCollectPortProvenanceBytes(t *testing.T) {
	d := runCollectDifferential(t, func(f *collectFixture) {
		writeFile(t, f.params.StdoutPath, "I could not produce JSON, sorry")
		writeFile(t, f.params.NamedPath, string(f.validReturn))
	})
	if !d.verdict.Delivered {
		t.Fatalf("the provenance row needs a delivered walk: %+v", d.verdict)
	}
	directSource := readFileBytes(t, filepath.Join(d.directBed.roundDir, "reply-source.json"))
	portSource := readFileBytes(t, filepath.Join(d.portBed.roundDir, "reply-source.json"))
	normalizedDirect := strings.ReplaceAll(string(directSource), d.directBed.params.Root, "ROOT")
	normalizedPort := strings.ReplaceAll(string(portSource), d.portBed.params.Root, "ROOT")
	if normalizedDirect != normalizedPort {
		t.Fatalf("provenance bytes diverge:\ndirect: %s\nport:   %s", normalizedDirect, normalizedPort)
	}
}

func mustEqualFiles(t *testing.T, a, b string) {
	t.Helper()
	da, db := readFileBytes(t, a), readFileBytes(t, b)
	if string(da) != string(db) {
		t.Fatalf("outputs diverge:\n%s: %s\n%s: %s", a, da, b, db)
	}
}

func readFileBytes(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func errorText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func portFileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
