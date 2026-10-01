package verbresult

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/enginecause"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/refusal"
)

// TestChildRefusalDetailCrossesTheEnvelope (2026-10-01): a planning child's
// policy-engine refusal carries its cause in its detail only. The envelope
// keeps that detail, and the parent's error carries it to --verbose, while
// the words a person reads stay the child's two lines.
func TestChildRefusalDetailCrossesTheEnvelope(t *testing.T) {
	t.Parallel()
	cause := errors.New("rebuilt engine was built from aaaa, which is not landed on bbbb")
	child := enginecause.RefuseWith("judgment-failed", []enginecause.Fact{enginecause.Path("checkout", "/lane/metasystem")},
		"the pinned engine was not built from the landing branch this run tests against", cause.Error())
	var stdout bytes.Buffer
	if err := Write(&stdout, FromError("test plan", 1, child, nil)); err != nil {
		t.Fatal(err)
	}
	result, err := Read(stdout.Bytes(), "test plan", 1, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Details) != 1 || !strings.Contains(result.Details[0], cause.Error()) {
		t.Fatalf("the envelope dropped the refusal's detail: %+v", result)
	}
	parent := fmt.Errorf("plan joined unit: %w", result.Err())
	if parent.Error() != "plan joined unit: "+child.Error() {
		t.Fatalf("the words a person reads changed: %q", parent.Error())
	}
	if detail := refusal.Detail(parent); detail != enginecause.Detail(child) {
		t.Fatalf("the parent's detail = %q, want the child's %q", detail, enginecause.Detail(child))
	}
	var coded *refusal.Coded
	if !errors.As(parent, &coded) {
		t.Fatal("the parent's error no longer carries its coded refusal")
	}
}
