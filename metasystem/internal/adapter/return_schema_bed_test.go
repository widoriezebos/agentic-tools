package adapter

// Ported from scripts/agents/return-schema-fixtures.sh (verb redesign U7b
// part 3), scenarios implementer-v1-v2 and critic-v3. The shell bed drove
// `adapter normalize-return` (through runtime-common.sh's one-line
// normalize_return wrapper), `validate return-complete`, `schema
// materialize` and `adapter fake-return`; these tests call the same owners in
// process against the checkout's shipped role schemas.

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/returnschema"
)

// returnSchemaBedRoot is the checkout whose internal/protocol/schemas the
// validators and materializer read.
func returnSchemaBedRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	return root
}

const returnSchemaBedV1 = `{"jobId":"fixture-job","round":1,"runtime":"fake","sessionId":null,
 "model":{"requested":"requested-model","effective":null},
 "evidence":[],"gaps":[],"mode":"implement","riskiestPart":"fixture",
 "diffBoundary":[],"whatWasDone":"fixture"}`

func returnSchemaBedObject(t *testing.T, text string) map[string]any {
	t.Helper()
	var value map[string]any
	if err := json.Unmarshal([]byte(text), &value); err != nil {
		t.Fatal(err)
	}
	return value
}

func returnSchemaBedWrite(t *testing.T, path string, value any) {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
}

func returnSchemaBedComplete(t *testing.T, root, role, path string) []string {
	t.Helper()
	return returnschema.ReturnCompleteRole(root, role, path)
}

// returnSchemaBedCandidate is the v2 candidate: the v1 shape plus a version,
// a claimed session and model, and an earlier claimed record.
func returnSchemaBedCandidate(t *testing.T) map[string]any {
	t.Helper()
	candidate := returnSchemaBedObject(t, returnSchemaBedV1)
	candidate["schemaVersion"] = 2
	candidate["sessionId"] = "claimed-session"
	candidate["model"] = map[string]any{"requested": "requested-model", "effective": "claimed-model"}
	candidate["claimed"] = map[string]any{"model": "earlier-claim"}
	return candidate
}

func TestReturnSchemaBedImplementerV1V2(t *testing.T) {
	t.Parallel()
	root := returnSchemaBedRoot(t)
	dir := t.TempDir()

	v1 := filepath.Join(dir, "v1.json")
	if err := os.WriteFile(v1, []byte(returnSchemaBedV1+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if violations := returnSchemaBedComplete(t, root, "implementer", v1); len(violations) != 0 {
		t.Fatalf("the version-1 return was refused: %v", violations)
	}

	record := filepath.Join(dir, "record.json")
	if err := os.WriteFile(record, []byte(`{"effectiveModel":"observed-model"}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	normalize := func(candidate map[string]any) map[string]any {
		t.Helper()
		path := filepath.Join(dir, "candidate.json")
		returnSchemaBedWrite(t, path, candidate)
		output := filepath.Join(dir, "return.json")
		if err := NormalizeReturn(path, "", record, output, filepath.Join(dir, "return.md"), "observed-session"); err != nil {
			t.Fatal(err)
		}
		if violations := returnSchemaBedComplete(t, root, "implementer", output); len(violations) != 0 {
			t.Fatalf("the normalized return was refused: %v", violations)
		}
		return readJSONFile(t, output)
	}

	normalized := normalize(returnSchemaBedCandidate(t))
	if normalized["schemaVersion"] != float64(2) {
		t.Fatalf("normalized return lost its schema version: %v", normalized)
	}
	if normalized["sessionId"] != "observed-session" {
		t.Fatalf("normalized return did not adopt the observed session: %v", normalized)
	}
	if model, _ := normalized["model"].(map[string]any); model["effective"] != "observed-model" {
		t.Fatalf("normalized return did not adopt the record's observed model: %v", normalized)
	}
	if want := map[string]any{"sessionId": "claimed-session", "model": "claimed-model"}; !reflect.DeepEqual(normalized["claimed"], want) {
		t.Fatalf("normalized return did not preserve both claims: %v", normalized["claimed"])
	}

	// A claim on ONE member and agreement on the other still carries both
	// keys: structured output rejects an object that leaves one out.
	oneClaim := returnSchemaBedCandidate(t)
	oneClaim["sessionId"] = "observed-session"
	delete(oneClaim, "claimed")
	normalized = normalize(oneClaim)
	if want := map[string]any{"sessionId": nil, "model": "claimed-model"}; !reflect.DeepEqual(normalized["claimed"], want) {
		t.Fatalf("a claim on one member did not keep both claimed keys: %v", normalized["claimed"])
	}

	missingVersion := returnSchemaBedCandidate(t)
	delete(missingVersion, "schemaVersion")
	path := filepath.Join(dir, "missing-version.json")
	returnSchemaBedWrite(t, path, missingVersion)
	if len(returnSchemaBedComplete(t, root, "implementer", path)) == 0 {
		t.Fatal("a version-2-shaped return without schemaVersion passed the frozen v1 schema")
	}
	extra := returnSchemaBedCandidate(t)
	extra["undeclared"] = "refuse"
	path = filepath.Join(dir, "extra.json")
	returnSchemaBedWrite(t, path, extra)
	if len(returnSchemaBedComplete(t, root, "implementer", path)) == 0 {
		t.Fatal("a version-2 return with an undeclared property passed")
	}
}

const returnSchemaBedSafeFacts = `{"local":true,"recoverable":true,"proofBoundaryCrossed":false,"authorityBoundaryCrossed":false,"secretsBoundaryCrossed":false,"irreversibleDataBoundaryCrossed":false,"externalSideEffectBoundaryCrossed":false}`

func returnSchemaBedCritic(findings string, count int, rigor string) string {
	return fmt.Sprintf(`{"schemaVersion":3,"claimed":{"sessionId":null,"model":null},
 "jobId":"fixture-critic","round":1,"runtime":"fake","sessionId":"fake-session",
 "model":{"requested":"fixture-model","effective":"fixture-model"},
 "evidence":[],"gaps":[],"mode":"critique","reviewedCommit":"abc1234",
 "findings":%s,"verdictMaterialCount":%d,"rigor":%s}`, findings, count, rigor)
}

func TestReturnSchemaBedCriticV3(t *testing.T) {
	t.Parallel()
	root := returnSchemaBedRoot(t)
	findingF1 := `[{"id":"F1","severity":"high","material":true,"claim":"fixture claim","evidence":"fixture evidence"}]`
	boundedRow := `[{"findingId":"F1","rigorClass":"bounded","facts":` + returnSchemaBedSafeFacts + `,"reopeningTrigger":"reopen if the finding recurs"}]`
	lawfulFindings := `[{"id":"F1","severity":"high","material":true,"claim":"bounded","evidence":"read"},{"id":"F2","severity":"critical","material":true,"claim":"severe","evidence":"read"},{"id":"F3","severity":"medium","material":true,"claim":"unproven","evidence":"read"}]`
	lawfulRigor := `[{"findingId":"F1","rigorClass":"bounded","facts":` + returnSchemaBedSafeFacts + `,"reopeningTrigger":"reopen if it recurs"},{"findingId":"F2","rigorClass":"severe","facts":` + returnSchemaBedSafeFacts + `,"reopeningTrigger":"reopen until the invariant is proved"},{"findingId":"F3","rigorClass":"unproven","facts":` + returnSchemaBedSafeFacts + `,"reopeningTrigger":"reopen when classification evidence exists"}]`
	malformedFacts := strings.Replace(returnSchemaBedSafeFacts, `,"externalSideEffectBoundaryCrossed":false`, "", 1)
	malformedRow := `[{"findingId":"F1","rigorClass":"bounded","facts":` + malformedFacts + `,"reopeningTrigger":"reopen if it recurs"}]`
	for _, test := range []struct {
		leg, findings string
		count         int
		rigor, want   string
	}{
		{"zero-material-empty-rigor", `[]`, 0, `[]`, ""},
		{"lawful-bounded-severe-unproven", lawfulFindings, 3, lawfulRigor, ""},
		{"missing-rigor-row", findingF1, 1, `[]`, `missing a classification row for material finding "F1"`},
		{"extra-rigor-row", `[]`, 0, boundedRow, `which is not a material finding`},
		{"malformed-facts", findingF1, 1, malformedRow, `$.rigor[0].facts.externalSideEffectBoundaryCrossed is required`},
		{"empty-finding-id", `[{"id":"","severity":"high","material":true,"claim":"empty id","evidence":"read"}]`, 1, `[]`,
			`$.findings[0].id must be a non-empty string without surrounding whitespace`},
		{"whitespace-finding-id", `[{"id":" F1 ","severity":"high","material":true,"claim":"spaced id","evidence":"read"}]`, 1, `[]`,
			`$.findings[0].id must be a non-empty string without surrounding whitespace`},
		{"duplicate-finding-id", `[{"id":"F1","severity":"high","material":true,"claim":"first","evidence":"read"},{"id":"F1","severity":"low","material":true,"claim":"second","evidence":"read"}]`, 2, boundedRow,
			`duplicates finding identifier "F1"`},
	} {
		t.Run(test.leg, func(t *testing.T) {
			t.Parallel()
			path := filepath.Join(t.TempDir(), test.leg+".json")
			if err := os.WriteFile(path, []byte(returnSchemaBedCritic(test.findings, test.count, test.rigor)), 0o644); err != nil {
				t.Fatal(err)
			}
			violations := strings.Join(returnSchemaBedComplete(t, root, "design-critic", path), "\n")
			if test.want == "" {
				if violations != "" {
					t.Fatalf("%s: a lawful version-3 critic return was refused: %s", test.leg, violations)
				}
				return
			}
			if violations == "" {
				t.Fatalf("%s: an invalid version-3 critic return passed", test.leg)
			}
			if !strings.Contains(violations, test.want) {
				t.Fatalf("%s: refusal did not name %q: %s", test.leg, test.want, violations)
			}
		})
	}
}

func TestReturnSchemaBedCodeCriticSchemaVersions(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	materialize := func(version int) string {
		t.Helper()
		output := filepath.Join(dir, fmt.Sprintf("code-critic-v%d.schema.json", version))
		if err := returnschema.Materialize("code-critic", version, output); err != nil {
			t.Fatal(err)
		}
		data, err := os.ReadFile(output)
		if err != nil {
			t.Fatal(err)
		}
		return string(data)
	}
	// Versions 1 and 2 are frozen byte contracts.
	for version, want := range map[int]string{
		1: "fe4ec2d623507feed6a5dbbdf6e4040ced855348d111f79e43ced4129a96943c",
		2: "6161117b74d84c34941d0181030b99108869421b2044b8bf80b539ee26e33056",
	} {
		if got := fmt.Sprintf("%x", sha256.Sum256([]byte(materialize(version)))); got != want {
			t.Fatalf("version-%d critic schema bytes changed: %s", version, got)
		}
	}
	v4 := materialize(4)
	if !strings.Contains(v4, `"artifact"`) || !strings.Contains(v4, `"enum": [`) || !strings.Contains(v4, "4") {
		t.Fatal("version-4 critic schema omitted the artifact member or version marker")
	}
	v5 := materialize(5)
	for _, want := range []string{`"mechanical"`, `"invariant"`, `"behaviour"`, `"fixture"`} {
		if !strings.Contains(v5, want) {
			t.Fatalf("version-5 critic schema omitted %s", want)
		}
	}
}

func TestReturnSchemaBedFakeCriticSpeaksVersionThree(t *testing.T) {
	t.Parallel()
	root := returnSchemaBedRoot(t)
	dir := t.TempDir()
	record := filepath.Join(dir, "fake-critic-record.json")
	prompt := filepath.Join(dir, "fake-critic-prompt.md")
	output := filepath.Join(dir, "fake-critic-return.json")
	writeFile(t, record, `{"jobId":"fake-critic-v3","round":1,"role":"code-critic","sessionId":"fake-session",
 "requestedModel":"fake-model","effectiveModel":"fake-model"}`)
	writeFile(t, prompt, "Working Mode: critique\n")
	if err := WriteFakeReturn(record, prompt, output); err != nil {
		t.Fatal(err)
	}
	if violations := returnSchemaBedComplete(t, root, "code-critic", output); len(violations) != 0 {
		t.Fatalf("the fake critic return was refused: %v", violations)
	}
	got := readJSONFile(t, output)
	if got["schemaVersion"] != float64(3) {
		t.Fatalf("fake critic did not speak return schema version 3: %v", got["schemaVersion"])
	}
	if rigor, ok := got["rigor"].([]any); !ok || len(rigor) != 0 {
		t.Fatalf("zero-finding fake critic did not emit empty rigor: %v", got["rigor"])
	}
}
