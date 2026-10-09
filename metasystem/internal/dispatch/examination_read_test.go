package dispatch

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/readsubject"
)

func examinationReadFixture(t *testing.T) (string, string, string, string) {
	t.Helper()
	repo := t.TempDir()
	root, job := "critic-root", "critic-correction"
	f := registerFindingValue("reader-local", true, "The correction misses the retained path.")
	f["class"], f["where"], f["change"] = "missing-reader", "metasystem/test.go", "Read the retained path."
	writeCriticRound(t, repo, root, root, 1, nil, nil)
	writeCriticRound(t, repo, root, job, 2, []any{f}, []any{registerRigor("reader-local", "bounded")})
	path := filepath.Join(repo, "artifacts", "agents", "jobs", job+".json")
	record := readJSONFile(t, path)
	record["engineBuild"], record["requestedModel"], record["effectiveModel"] = "executing-engine", "resolved-alias", "observed-model"
	if err := writeRecord(path, record); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(repo, "artifacts", "agents", root, "rounds", "2")
	subject := readsubject.ReadSubject{Kind: SubjectLive, ImplementerRoot: "builder", ReviewedProjectTree: strings.Repeat("a", 40), DiffDigest: strings.Repeat("b", 64)}
	writeJSONFile(t, dir, "subject.json", subject)
	result := readJSONFile(t, filepath.Join(dir, "return.json"))
	result["reviewedTree"], result["verdictMaterialCount"] = subject.ReviewedProjectTree, 1
	writeJSONFile(t, dir, "return.json", result)
	if err := os.WriteFile(filepath.Join(dir, "return.md"), []byte("VERDICT: REVISE material=1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return repo, root, job, dir
}

func TestCollectExaminationUsesJobIdentityAndExecutingProvenance(t *testing.T) {
	t.Parallel()
	repo, root, job, dir := examinationReadFixture(t)
	read, err := CollectExamination(repo, job)
	if err != nil {
		t.Fatal(err)
	}
	if read.ID != job || read.ID == root || read.Findings[0].ID != job+":1" || read.Engine != "executing-engine" || read.Model != "observed-model" || read.Output != filepath.Join(dir, "return.md") || read.Material != 1 {
		t.Fatalf("collected evidence = %+v", read)
	}
	data, digest := read.Canonical()
	again, other := read.Canonical()
	if string(data) != string(again) || digest != other || len(digest) != 64 {
		t.Fatal("canonical read is unstable")
	}
}

func TestCollectExaminationUnknownEvidenceNeverBecomesClean(t *testing.T) {
	t.Parallel()
	for _, mutation := range []string{"class", "count", "prose", "subject", "engine", "output"} {
		t.Run(mutation, func(t *testing.T) {
			t.Parallel()
			repo, _, job, dir := examinationReadFixture(t)
			path := filepath.Join(dir, "return.json")
			result := readJSONFile(t, path)
			switch mutation {
			case "class":
				delete(result["findings"].([]any)[0].(map[string]any), "class")
			case "count":
				result["verdictMaterialCount"] = 0
			case "subject":
				result["reviewedTree"] = strings.Repeat("c", 40)
			case "prose":
				if err := os.WriteFile(filepath.Join(dir, "return.md"), []byte("VERDICT: LAND\n"), 0o644); err != nil {
					t.Fatal(err)
				}
			case "output":
				if err := os.Remove(filepath.Join(dir, "return.md")); err != nil {
					t.Fatal(err)
				}
			case "engine":
				recordPath := filepath.Join(repo, "artifacts", "agents", "jobs", job+".json")
				record := readJSONFile(t, recordPath)
				delete(record, "engineBuild")
				if err := writeRecord(recordPath, record); err != nil {
					t.Fatal(err)
				}
			}
			writeJSONFile(t, dir, "return.json", result)
			if _, err := CollectExamination(repo, job); err == nil {
				t.Fatalf("%s unknown input admitted", mutation)
			}
		})
	}
}

func TestCritiqueTransferClosePreservesFindingAndCannotCertifyClean(t *testing.T) {
	t.Parallel()
	repo, root, path := writeCloseRoot(t, "code-critic", 1, []registerFinding{closeFinding("read-one:1", "invariant", "", "gap", "bounded")}, nil, 3, 1)
	for n := 0; n < 2; n++ {
		if err := CritiqueTransferClose(repo, root, []string{"read-one:1"}, "unit-stop-1"); err != nil {
			t.Fatal(err)
		}
	}
	record := readJSONFile(t, path)
	register, err := decodeFindingRegister(record[findingRegisterField])
	if err != nil {
		t.Fatal(err)
	}
	if len(register) != 1 || register[0].FindingID != "read-one:1" || register[0].Status != "transferred" || register[0].TransferStop != "unit-stop-1" || record["chainClosed"] != true || record["chainCloseReason"] != "transferred" || record[closureField] != nil {
		t.Fatalf("transfer lost evidence: %s", fmt.Sprint(record))
	}
	if clean, err := readsubject.CleanRegister(record[findingRegisterField]); err != nil || clean {
		t.Fatalf("transfer clean=%v,error=%v", clean, err)
	}
	if err := CritiqueTransferClose(repo, root, []string{"absent:1"}, "unit-stop-1"); err == nil {
		t.Fatal("missing finding silently transferred")
	}
}

func TestCritiqueRegisterRetainsQualifiedStopEvidence(t *testing.T) {
	t.Parallel()
	repo, root, job, dir := examinationReadFixture(t)
	rootPath := filepath.Join(repo, "artifacts", "agents", "jobs", root+".json")
	record := readJSONFile(t, rootPath)
	record[findingRegisterRoundField] = 1
	if err := writeRecord(rootPath, record); err != nil {
		t.Fatal(err)
	}
	result := readJSONFile(t, filepath.Join(dir, "return.json"))
	result["schemaVersion"] = 6
	writeJSONFile(t, dir, "return.json", result)
	if outcome, err := CritiqueRegisterAdvance(repo, root, job); err != nil || outcome != "advanced" {
		t.Fatalf("collect register = %s,%v", outcome, err)
	}
	retained, err := os.ReadFile(filepath.Join(dir, "read.json"))
	if err != nil {
		t.Fatal(err)
	}
	collected, err := CollectExamination(repo, job)
	if err != nil {
		t.Fatal(err)
	}
	canonical, _ := collected.Canonical()
	if string(retained) != string(canonical) {
		t.Fatal("round read differs from collected evidence")
	}
	record = readJSONFile(t, rootPath)
	register, err := decodeFindingRegister(record[findingRegisterField])
	if err != nil {
		t.Fatal(err)
	}
	if len(register) != 1 || register[0].FindingID != job+":1" || register[0].Class != "missing-reader" || register[0].Where != "metasystem/test.go" || register[0].Change != "Read the retained path." || record["readDigest"] == nil || record["read"].(map[string]any)["id"] != job {
		t.Fatalf("register discarded canonical read: %+v", record)
	}
	if outcome, err := CritiqueRegisterAdvance(repo, root, job); err != nil || outcome != "unchanged" {
		t.Fatalf("repeat collection = %s,%v", outcome, err)
	}
}

func TestCritiqueRegisterResolutionValidatesOriginalEvidence(t *testing.T) {
	t.Parallel()
	prior := closeFinding("original:1", "invariant", "", "gap", "bounded")
	prior.Class, prior.Where = "missing-reader", "metasystem/test.go"
	for _, mutation := range []string{"valid", "unknown", "class", "path"} {
		t.Run(mutation, func(t *testing.T) {
			t.Parallel()
			f := registerFindingValue("next:1", false, "The reader now reaches the retained evidence.")
			f["class"], f["where"], f["resolves"] = "missing-reader", "metasystem/test.go", "original:1"
			switch mutation {
			case "unknown":
				f["resolves"] = "absent:1"
			case "class":
				f["class"] = "regression"
			case "path":
				f["where"] = "metasystem/another.go"
			}
			got, _, _, err := foldCritiqueFindingsVersioned([]registerFinding{prior}, "code-critic", "next", []any{f}, []any{}, 6, critiqueSubject{legacy: true}, 2)
			if mutation == "valid" {
				if err != nil || got[0].Status != "resolved" {
					t.Fatalf("valid coverage = %+v,%v", got, err)
				}
			} else if err == nil {
				t.Fatalf("%s cleared different prior evidence", mutation)
			}
		})
	}
}

func TestCompletedUnknownExaminationRetriesOnlyOnce(t *testing.T) {
	t.Parallel()
	repo, root, job, dir := examinationReadFixture(t)
	record := readJSONFile(t, filepath.Join(repo, "artifacts", "agents", "jobs", job+".json"))
	if err := ExaminationRetryAdmissible(repo, record); err == nil {
		t.Fatal("completed readable evidence bought another examination")
	}
	result := readJSONFile(t, filepath.Join(dir, "return.json"))
	delete(result["findings"].([]any)[0].(map[string]any), "class")
	writeJSONFile(t, dir, "return.json", result)
	if err := ExaminationRetryAdmissible(repo, record); err != nil {
		t.Fatalf("unknown completed return cannot recover: %v", err)
	}
	if err := ReserveUnknownExaminationRetry(repo, job); err != nil {
		t.Fatal(err)
	}
	retained := readJSONFile(t, filepath.Join(repo, "artifacts", "agents", "jobs", root+".json"))
	if retained["unknownExaminationRetryFrom"] != job {
		t.Fatal("retry did not retain original actual examination")
	}
	if err := ExaminationRetryAdmissible(repo, record); err == nil {
		t.Fatal("second automatic retry admitted")
	}
	if err := ReserveUnknownExaminationRetry(repo, job); err == nil {
		t.Fatal("second retry reservation admitted")
	}
}
