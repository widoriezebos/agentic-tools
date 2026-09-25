package landing

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
)

// A schema-2 testing receipt names its policy and candidate engines twice: in
// the receipt and in the testing evidence. Each identity version compares only
// the fields it defines, and any disagreement refuses the receipt.
func TestTestingReceiptEngineIdentityMustAgreeWithItsEvidence(t *testing.T) {
	t.Parallel()
	const policy, candidate, build = "policy-digest", "candidate-digest", "candidate-build"
	receipt := func(version int, receiptPolicy, receiptCandidate, receiptBuild string) TestReceipt {
		return TestReceipt{SchemaVersion: 2, PolicyEngineDigest: receiptPolicy, CandidateEngineDigest: receiptCandidate, CandidateEngineBuildIdentity: receiptBuild,
			Testing: &proofrun.TestResult{CandidateEngineIdentityVersion: version, PolicyEngineDigest: policy, CandidateEngineDigest: candidate, CandidateEngineBuildIdentity: build}}
	}
	current := proofrun.CandidateEngineIdentitySchemaVersion
	for _, test := range []struct {
		name    string
		receipt TestReceipt
		ok      bool
	}{
		{name: "version 1 agreeing digests", receipt: receipt(1, policy, candidate, ""), ok: true},
		{name: "version 1 ignores build identity", receipt: receipt(1, policy, candidate, "other-build"), ok: true},
		{name: "version 1 other policy", receipt: receipt(1, "other-policy", candidate, "")},
		{name: "version 1 other candidate", receipt: receipt(1, policy, "other-candidate", "")},
		{name: "version 1 no candidate", receipt: receipt(1, policy, "", "")},
		{name: "current agreeing identity", receipt: receipt(current, policy, candidate, build), ok: true},
		{name: "current other build", receipt: receipt(current, policy, candidate, "other-build")},
		{name: "current no build", receipt: receipt(current, policy, candidate, "")},
		{name: "unknown version", receipt: receipt(current+1, policy, candidate, build)},
		{name: "no testing evidence", receipt: TestReceipt{SchemaVersion: 2, PolicyEngineDigest: policy, CandidateEngineDigest: candidate}},
	} {
		err := validateTestingReceiptEngineIdentity(test.receipt)
		if test.ok != (err == nil) {
			t.Errorf("%s: identity error = %v, want accepted=%t", test.name, err, test.ok)
		}
		if test.receipt.Testing == nil {
			continue
		}
		payload, marshalErr := json.Marshal(test.receipt)
		if marshalErr != nil {
			t.Fatal(marshalErr)
		}
		decoded, decodeErr := decodeCommittedTestingReceipt(payload)
		if test.ok != (decodeErr == nil) || !test.ok && !strings.Contains(decodeErr.Error(), "contradicts its policy or candidate engine identity") {
			t.Errorf("%s: committed receipt decode error = %v, want accepted=%t", test.name, decodeErr, test.ok)
		}
		if test.ok && decoded.Testing.CandidateEngineDigest != candidate {
			t.Errorf("%s: decoded receipt lost its testing evidence: %+v", test.name, decoded)
		}
	}
}

// Committed testing receipts are strict JSON: unknown fields, trailing data,
// another schema, and missing evidence are all refused before identity checks.
func TestCommittedTestingReceiptDecodeRefusesIncompletePayloads(t *testing.T) {
	t.Parallel()
	for name, payload := range map[string]string{
		"unknown field":    `{"schemaVersion":2,"testing":{},"surplus":true}`,
		"trailing data":    `{"schemaVersion":2,"testing":{}} {}`,
		"schema one":       `{"schemaVersion":1,"testing":{}}`,
		"missing evidence": `{"schemaVersion":2}`,
	} {
		if _, err := decodeCommittedTestingReceipt(json.RawMessage(payload)); err == nil {
			t.Errorf("%s: committed testing receipt %s was accepted", name, payload)
		}
	}
}
