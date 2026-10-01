package main

import (
	"bytes"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	refusalpkg "github.com/widoriezebos/agentic-tools/metasystem/internal/refusal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testrun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/verbresult"
)

// fakeTestRunResult is an internal test run child's envelope as a fake
// runner returns it: the outcome its exit stands for, code and data.
func fakeTestRunResult(exit int, code string, data any) verbresult.Result {
	result := verbresult.FromError("internal test run", exit, nil, data)
	result.Code = code
	if code != "" && exit == 1 {
		result.Outcome = verbresult.Refused
	}
	return result
}

// testRunEnvelopeLine is that envelope as the child prints it on stdout.
func testRunEnvelopeLine(t *testing.T, exit int, code, summary string, data any) string {
	t.Helper()
	result := fakeTestRunResult(exit, code, data)
	result.Summary = summary
	var out bytes.Buffer
	if err := verbresult.Write(&out, result); err != nil {
		t.Fatal(err)
	}
	return out.String()
}

// planOfEnvelope reads a test plan --json answer: one envelope of verb whose
// data is the plan (R1).
func planOfEnvelope(stdout, verb string, status int) (testrun.PlanOutput, error) {
	result, err := verbresult.Read([]byte(stdout), verb, status, "")
	if err != nil {
		return testrun.PlanOutput{}, err
	}
	var output testrun.PlanOutput
	return output, result.DecodeData(&output)
}

// testRunEnvelopeBytes is a fake in-process runner's stdout: the envelope
// its exit stands for, with data (raw JSON).
func testRunEnvelopeBytes(t *testing.T, exit int, data string) []byte {
	t.Helper()
	return []byte(testRunEnvelopeLine(t, exit, "", "", json.RawMessage(data)))
}

// TestAuditEnvelopeFieldsMatchThePublicResult: the internal verbs'
// envelope (internal/verbresult) is the public intentResult on the wire,
// plus its code: every field a public verb prints has the same name there.
func TestAuditEnvelopeFieldsMatchThePublicResult(t *testing.T) {
	t.Parallel()
	names := func(value any) map[string]bool {
		fields := map[string]bool{}
		kind := reflect.TypeOf(value)
		for index := range kind.NumField() {
			if tag, _, _ := strings.Cut(kind.Field(index).Tag.Get("json"), ","); tag != "" && tag != "-" {
				fields[tag] = true
			}
		}
		return fields
	}
	envelope := names(verbresult.Result{})
	for name := range names(intentResult{}) {
		if !envelope[name] {
			t.Errorf("the public result's field %q is missing from verbresult.Result", name)
		}
	}
	if !envelope["code"] {
		t.Error("verbresult.Result has no code field")
	}
}

// refusalDetail is a refusal's code-first detail ("CODE facts: reason"),
// where its code and facts live now that the words a person reads carry
// neither.
func refusalDetail(err error) string { return refusalpkg.DetailOf(err) }
