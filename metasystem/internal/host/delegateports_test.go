package host

// The host-side differential fixtures: the same inputs through the
// direct owner function and through the registered port must produce
// identical outcomes — bytes, flags, and error text.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/delegate"
)

// The claude host result port writes the same envelope, return, and
// usage bytes as the direct call.
func TestHostResultPortMatchesDirect(t *testing.T) {
	dir := t.TempDir()
	provider := filepath.Join(dir, "provider.json")
	if err := os.WriteFile(provider, []byte(
		`{"result": "{\"status\":\"done\"}", "usage": {"input_tokens": 5}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	ports, err := delegate.PortsFor("claude")
	if err != nil || ports.HostResult == nil {
		t.Fatalf("claude host-result port: %v", err)
	}
	dReturn, dUsage := filepath.Join(dir, "d-return.json"), filepath.Join(dir, "d-usage.json")
	pReturn, pUsage := filepath.Join(dir, "p-return.json"), filepath.Join(dir, "p-usage.json")
	if err := ClaudeResult(provider, dReturn, dUsage); err != nil {
		t.Fatal(err)
	}
	if err := ports.HostResult(provider, pReturn, pUsage); err != nil {
		t.Fatal(err)
	}
	hostMustEqualFiles(t, dReturn, pReturn)
	hostMustEqualFiles(t, dUsage, pUsage)
}

// The devin host ports match the direct calls on extraction and on
// the shared error shapes.
func TestHostDevinPortsMatchDirect(t *testing.T) {
	dir := t.TempDir()
	raw := filepath.Join(dir, "raw.out")
	if err := os.WriteFile(raw, []byte("noise before {\"status\": \"ok\"} noise after"), 0o644); err != nil {
		t.Fatal(err)
	}
	ports, err := delegate.PortsFor("devin")
	if err != nil || ports.HostReturn == nil || ports.HostTurnUsage == nil {
		t.Fatalf("devin host ports: %v", err)
	}
	dOut, pOut := filepath.Join(dir, "d.json"), filepath.Join(dir, "p.json")
	if err := DevinReturn(raw, dOut); err != nil {
		t.Fatal(err)
	}
	if err := ports.HostReturn(raw, pOut); err != nil {
		t.Fatal(err)
	}
	hostMustEqualFiles(t, dOut, pOut)

	absent := filepath.Join(dir, "absent")
	dUsage, pUsage := filepath.Join(dir, "d-usage.json"), filepath.Join(dir, "p-usage.json")
	derr := HostDevinUsage(dUsage, absent, "", "", false)
	perr := ports.HostTurnUsage(pUsage, absent, "", "", false)
	if hostErrorText(derr) != hostErrorText(perr) {
		t.Fatalf("host devin-usage errors diverge: %v vs %v", derr, perr)
	}
	if hostFileExists(dUsage) != hostFileExists(pUsage) {
		t.Fatal("usage output presence diverges")
	}
	if hostFileExists(dUsage) {
		hostMustEqualFiles(t, dUsage, pUsage)
	}
}

// The host collect port carries the field mapping and the delivered
// flag exactly; the missing-record error matches the direct call.
func TestHostCollectPortMatchesDirect(t *testing.T) {
	dir := t.TempDir()
	ports, err := delegate.PortsFor("devin")
	if err != nil || ports.HostCollect == nil {
		t.Fatalf("devin host-collect port: %v", err)
	}
	in := delegate.HostCollectInputs{
		Root:           dir,
		TurnRecordPath: filepath.Join(dir, "absent-turn.json"),
		TurnDir:        dir,
	}
	_, err = HostDevinCollect(HostCollectParams{
		Root:           in.Root,
		TurnRecordPath: in.TurnRecordPath,
		TurnDir:        in.TurnDir,
	})
	_, _, perr := ports.HostCollect(in)
	if hostErrorText(err) != hostErrorText(perr) {
		t.Fatalf("host collect errors diverge: %v vs %v", err, perr)
	}
}

// runHostCollectDifferential runs the direct owner and the registered
// port on the two beds' current params and binds the delivered flag
// and the verdict bytes with each bed's root normalized to ROOT.
// These walk rows never go mechanical; any error is loud.
func runHostCollectDifferential(t *testing.T, directBed, portBed *hostCollectFixture) (*HostCollectVerdict, []byte) {
	t.Helper()
	directVerdict, directErr := HostDevinCollect(directBed.params)
	ports, err := delegate.PortsFor("devin")
	if err != nil || ports.HostCollect == nil {
		t.Fatalf("devin host-collect port: %v", err)
	}
	in := portBed.params
	encoded, delivered, portErr := ports.HostCollect(delegate.HostCollectInputs{
		Root:           in.Root,
		TurnRecordPath: in.TurnRecordPath,
		TurnDir:        in.TurnDir,
		Workspace:      in.Workspace,
		StdoutPath:     in.StdoutPath,
		NamedPath:      in.NamedPath,
		TranscriptPath: in.TranscriptPath,
		RejectDigests:  in.RejectDigests,
	})
	if directErr != nil || portErr != nil {
		t.Fatalf("the walk must not go mechanical: direct %v, port %v", directErr, portErr)
	}
	if delivered != directVerdict.Delivered {
		t.Fatalf("delivered flag diverges: direct %v, port %v", directVerdict.Delivered, delivered)
	}
	directBytes, err := json.Marshal(directVerdict)
	if err != nil {
		t.Fatal(err)
	}
	normalizedDirect := strings.ReplaceAll(string(directBytes), directBed.params.Root, "ROOT")
	normalizedPort := strings.ReplaceAll(string(encoded), portBed.params.Root, "ROOT")
	if normalizedDirect != normalizedPort {
		t.Fatalf("verdict bytes diverge:\ndirect: %s\nport:   %s", normalizedDirect, normalizedPort)
	}
	return directVerdict, encoded
}

// Valid stdout delivers identically through the port: verdict bytes
// and the delivered flag.
func TestHostCollectPortStdoutDelivers(t *testing.T) {
	directBed, portBed := newHostCollectFixture(t), newHostCollectFixture(t)
	for _, f := range []*hostCollectFixture{directBed, portBed} {
		if err := os.WriteFile(f.params.StdoutPath, f.validReturn, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	verdict, _ := runHostCollectDifferential(t, directBed, portBed)
	if !verdict.Delivered || verdict.Channel != "stdout" {
		t.Fatalf("valid stdout must deliver: %+v", verdict)
	}
}

// An empty walk reports no delivery identically through the port.
func TestHostCollectPortEmptyWalkUndelivered(t *testing.T) {
	directBed, portBed := newHostCollectFixture(t), newHostCollectFixture(t)
	verdict, _ := runHostCollectDifferential(t, directBed, portBed)
	if verdict.Delivered {
		t.Fatalf("an empty walk must not deliver: %+v", verdict)
	}
}

// The reject-digest fallthrough: each side's accepted stdout digest,
// fed back as that side's rejection with a byte-distinct named
// candidate staged, resumes the walk to the named file on both sides.
func TestHostCollectPortRejectDigestResumes(t *testing.T) {
	directBed, portBed := newHostCollectFixture(t), newHostCollectFixture(t)
	for _, f := range []*hostCollectFixture{directBed, portBed} {
		if err := os.WriteFile(f.params.StdoutPath, f.validReturn, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	verdict, encoded := runHostCollectDifferential(t, directBed, portBed)
	if !verdict.Delivered || verdict.Channel != "stdout" {
		t.Fatalf("the first walk must deliver via stdout: %+v", verdict)
	}
	directAccepted, err := os.ReadFile(verdict.Reply)
	if err != nil {
		t.Fatal(err)
	}
	var portVerdict HostCollectVerdict
	if err := json.Unmarshal(encoded, &portVerdict); err != nil {
		t.Fatal(err)
	}
	portAccepted, err := os.ReadFile(portVerdict.Reply)
	if err != nil {
		t.Fatal(err)
	}
	directBed.params.RejectDigests = []string{sha256Hex(directAccepted)}
	portBed.params.RejectDigests = []string{sha256Hex(portAccepted)}
	for _, f := range []*hostCollectFixture{directBed, portBed} {
		distinct := append([]byte(nil), f.validReturn...)
		distinct = append(distinct[:len(distinct)-1], []byte(` }`)...)
		if !json.Valid(distinct) {
			t.Fatal("test bug: distinct candidate must stay valid JSON")
		}
		if err := os.WriteFile(f.params.NamedPath, distinct, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	verdict, _ = runHostCollectDifferential(t, directBed, portBed)
	if !verdict.Delivered || verdict.Channel != "named-file" {
		t.Fatalf("the walk must resume past the rejected digest: %+v", verdict)
	}
}

// The fake host ports match the direct calls byte for byte on the
// result envelope.
func TestHostFakePortsMatchDirect(t *testing.T) {
	dir := t.TempDir()
	ports, err := delegate.PortsFor("fake")
	if err != nil || ports.HostFakeResult == nil || ports.HostFakeReturn == nil {
		t.Fatalf("fake host ports: %v", err)
	}
	dResult, pResult := filepath.Join(dir, "d.json"), filepath.Join(dir, "p.json")
	raw := filepath.Join(dir, "raw.out")
	if err := os.WriteFile(raw, []byte("fake"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := FakeResult(dResult, "sess", raw, "", "failed"); err != nil {
		t.Fatal(err)
	}
	if err := ports.HostFakeResult(pResult, "sess", raw, "", "failed"); err != nil {
		t.Fatal(err)
	}
	hostMustEqualFiles(t, dResult, pResult)

	derr := FakeReturn(filepath.Join(dir, "absent"), "", filepath.Join(dir, "dr.json"), "return-ok", dir)
	perr := ports.HostFakeReturn(filepath.Join(dir, "absent"), "", filepath.Join(dir, "pr.json"), "return-ok", dir)
	if hostErrorText(derr) != hostErrorText(perr) {
		t.Fatalf("fake return errors diverge: %v vs %v", derr, perr)
	}
}

// The registry keys are not crossed between the sides: claude's
// entry has no devin host ports and vice versa.
func TestHostPortKeysAreNotCrossed(t *testing.T) {
	claude, err := delegate.PortsFor("claude")
	if err != nil {
		t.Fatal(err)
	}
	if claude.HostReturn != nil || claude.HostCollect != nil {
		t.Fatal("claude must not carry devin's host ports")
	}
	devin, err := delegate.PortsFor("devin")
	if err != nil {
		t.Fatal(err)
	}
	if devin.HostResult != nil || devin.HostFakeResult != nil {
		t.Fatal("devin must not carry claude's or fake's host ports")
	}
}

func hostMustEqualFiles(t *testing.T, a, b string) {
	t.Helper()
	da, err := os.ReadFile(a)
	if err != nil {
		t.Fatal(err)
	}
	db, err := os.ReadFile(b)
	if err != nil {
		t.Fatal(err)
	}
	if string(da) != string(db) {
		t.Fatalf("outputs diverge:\n%s: %s\n%s: %s", a, da, b, db)
	}
}

func hostErrorText(err error) string {
	if err == nil {
		return ""
	}
	return strings.TrimSpace(err.Error())
}

func hostFileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
