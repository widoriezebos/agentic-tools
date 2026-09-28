package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
)

// portableFlakeCheck fails the first run of the fixture and passes every
// later one on identical bytes: a flake, as the tip proof and its
// classification run see one.
const portableFlakeCheck = `#!/bin/sh
set -eu
id="$1"
report="$3"
printf '%s\n' "$id" >> "$PORTABLE_NATIVE_COUNTER"
mkdir -p "$report"
if [ "$(wc -l < "$PORTABLE_NATIVE_COUNTER")" -eq 1 ]; then
  printf '<testsuite><testcase classname="portable" name="%s"><failure message="flake"/></testcase></testsuite>\n' "$id" > "$report/result.xml"
  exit 9
fi
printf '<testsuite><testcase classname="portable" name="%s"/></testsuite>\n' "$id" > "$report/result.xml"
`

// The composed known-flake landing resolves its sources through the real
// retained verifier over real retained attempts: the tip attempt that failed
// the group and the classification attempt that passed it at the same
// execution identity. The red group resolves to the classification attempt
// as reused; before the classification exists it does not resolve at all.
func TestBatchComposedLandingResolvesThroughTheRealRetainedVerifier(t *testing.T) {
	t.Parallel()
	fixture := newPortableFileProof(t)
	fixture.put("scripts/check.sh", portableFlakeCheck, 0o755)
	tree, files := fixture.snapshot()
	plan := fixture.plan(fixture.loadedContract(files), "app/a.txt")
	run := fixture.request(tree, files, plan)
	run.CandidateEngineBuildIdentity = fixture.engineIdentity(tree, run.Environment)

	tip, tipIdentities := fixture.execute(run, false)
	if group := portableGroups(tip)["app-a"]; group.Status != "failed" {
		t.Fatalf("the tip proof did not fail the flaky group: %+v", group)
	}
	tipAttempt := retainedAttemptFor(t, fixture.root, "")
	proof := batch.Proof{Status: "failed", Tree: tree, AttemptID: tipAttempt, SelectedGroups: []string{"app-a"}, Executions: []string{"app-a"},
		GroupIdentities: map[string]string{"app-a": tipIdentities["app-a"]}, RedGroups: []batch.RedGroup{{ID: "app-a", Status: "failed"}}}
	// Only the failed tip attempt is retained: the verifier has nothing
	// passing to compose (it refuses, or the group does not resolve).
	if before, err := fixture.verify(run, files, tip, time.Now().UTC()); err == nil {
		if sources, err := batch.ResolveSources(proof, batchSourcesFromVerification(before)); err == nil {
			t.Fatalf("the red group resolved without a classification attempt: %+v", sources)
		}
	}

	// The classification run is an accountable retry of the failed tip attempt.
	evidence := filepath.Join(t.TempDir(), "tip.log")
	if err := os.WriteFile(evidence, []byte("app-a failed on the tip\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	decision, err := json.Marshal(proofrun.RetryDecision{SchemaVersion: 1, PriorAttempt: tipAttempt, Cause: "known flake classification",
		EvidencePath: evidence, Rationale: "the batch lane runs the red group once more on the identical tip"})
	if err != nil {
		t.Fatal(err)
	}
	fixture.retry = filepath.Join(filepath.Dir(evidence), "retry.json")
	if err := os.WriteFile(fixture.retry, decision, 0o600); err != nil {
		t.Fatal(err)
	}
	classification, classificationIdentities := fixture.execute(run, true)
	if classificationIdentities["app-a"] != tipIdentities["app-a"] {
		t.Fatalf("the classification ran at identity %s, the tip at %s", classificationIdentities["app-a"], tipIdentities["app-a"])
	}
	classificationAttempt := retainedAttemptFor(t, fixture.root, tipAttempt)
	verified, err := fixture.verify(run, files, classification, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	sources, err := batch.ResolveSources(proof, batchSourcesFromVerification(verified))
	if err != nil || sources["app-a"] != (batch.Source{Kind: batch.SourceReused, Attempt: classificationAttempt}) {
		t.Fatalf("composed sources=%+v err=%v, want app-a reused from classification attempt %s (tip %s)", sources, err, classificationAttempt, tipAttempt)
	}
}

// retainedAttemptFor names the one retained attempt other than skip.
func retainedAttemptFor(t *testing.T, root, skip string) string {
	t.Helper()
	attempts, err := readPortableAttempts(root)
	if err != nil {
		t.Fatal(err)
	}
	found := ""
	for _, attempt := range attempts {
		if attempt != skip {
			if found != "" {
				t.Fatalf("more than one new attempt: %s and %s", found, attempt)
			}
			found = attempt
		}
	}
	if found == "" {
		t.Fatal("no retained attempt")
	}
	return found
}

func readPortableAttempts(root string) ([]string, error) {
	attempts, err := proofrun.ReadAttempts(root)
	ids := make([]string, 0, len(attempts))
	for _, attempt := range attempts {
		ids = append(ids, attempt.AttemptID)
	}
	return ids, err
}
