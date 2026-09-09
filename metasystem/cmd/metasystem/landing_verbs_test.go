package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/validate"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/wiredoc"
)

func TestSTR3Tier1ReceiptProof06RefusesMismatchedIndex(t *testing.T) {
	root := t.TempDir()
	runReceiptGit(t, root, "init", "-q", "-b", "main")
	runReceiptGit(t, root, "config", "user.name", "receipt fixture")
	runReceiptGit(t, root, "config", "user.email", "receipt@example.invalid")
	writeReceiptFixture(t, root, "payload.txt", "base\n")
	runReceiptGit(t, root, "add", "payload.txt")
	runReceiptGit(t, root, "commit", "-qm", "base")

	writeReceiptFixture(t, root, "payload.txt", "candidate\n")
	runReceiptGit(t, root, "add", "payload.txt")
	candidate := runReceiptGit(t, root, "write-tree")

	// The supplied candidate remains a valid Git tree, but the real index is
	// moved before receipt creation. The command must not run and no stale or
	// newly labelled receipt may remain for the supplied tree.
	writeReceiptFixture(t, root, "payload.txt", "different index\n")
	runReceiptGit(t, root, "add", "payload.txt")
	if code := runLandingTestReceipt([]string{
		"--root", root,
		"--tree", candidate,
		"--command", "touch command-must-not-run",
	}); code == 0 {
		t.Fatal("receipt creation accepted a supplied tree that differed from the real index")
	}
	if _, err := os.Stat(filepath.Join(root, "command-must-not-run")); !os.IsNotExist(err) {
		t.Fatalf("test command ran despite the mismatched index: %v", err)
	}
	if _, err := os.Stat(landing.TestReceiptPath(root, candidate)); !os.IsNotExist(err) {
		t.Fatalf("receipt exists after mismatched-index refusal: %v", err)
	}
}

func TestChainLandingRecertifiesAfterBaseMove(t *testing.T) {
	bed := t.TempDir()
	top := filepath.Join(bed, "integration")
	project := filepath.Join(top, "metasystem")
	chainTop := filepath.Join(bed, "chain")
	chainProject := filepath.Join(chainTop, "metasystem")
	if err := os.MkdirAll(filepath.Join(project, "internal", "app"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(project, "scripts", "agents"), 0o755); err != nil {
		t.Fatal(err)
	}
	pathClasses, err := os.ReadFile("../../scripts/agents/path-classes.txt")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(project, "scripts", "agents", "path-classes.txt"), pathClasses, 0o644); err != nil {
		t.Fatal(err)
	}
	promotion, err := os.ReadFile("../../scripts/agents/landing-promotion.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(project, "scripts", "agents", "landing-promotion.json"), promotion, 0o644); err != nil {
		t.Fatal(err)
	}
	landingClasses, err := os.ReadFile("../../scripts/agents/landing-classes.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(project, "scripts", "agents", "landing-classes.json"), landingClasses, 0o644); err != nil {
		t.Fatal(err)
	}
	rulings, err := os.ReadFile("../../memory/rulings.md")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(project, "memory"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(project, "memory", "rulings.md"), rulings, 0o644); err != nil {
		t.Fatal(err)
	}
	baseSource := "base-one\nkeep-two\nuntested-three\nkeep-four\nbase-five\nkeep-six\n"
	writeReceiptFixture(t, project, "internal/app/source.txt", baseSource)
	writeReceiptFixture(t, project, ".gitignore", "artifacts/\n")
	writeReceiptFixture(t, project, "metasystem.conf", "metasystem.version=1\nrole.code-critic.runtime=fake\n")
	writeReceiptFixture(t, top, "sibling.txt", "base sibling\n")
	runReceiptGit(t, top, "init", "-q", "-b", "main")
	runReceiptGit(t, top, "config", "user.name", "recertification fixture")
	runReceiptGit(t, top, "config", "user.email", "recertification@example.invalid")
	runReceiptGit(t, top, "add", ".")
	runReceiptGit(t, top, "commit", "-qm", "base")
	baseCommit := runReceiptGit(t, top, "rev-parse", "HEAD")
	runReceiptGit(t, top, "worktree", "add", "-q", "-b", "chain", chainTop, "HEAD")
	runReceiptGit(t, chainTop, "cat-file", "-e", baseCommit+"^{commit}")

	writeReceiptFixture(t, chainProject, "internal/app/source.txt",
		"CHAIN\nkeep-two\nuntested-three\nkeep-four\nbase-five\nkeep-six\n")
	writeFixtureJSON := func(relative string, value any) {
		t.Helper()
		path := filepath.Join(project, filepath.FromSlash(relative))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		data, err := json.MarshalIndent(value, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	implementer := map[string]any{
		"jobId": "impl", "role": "implementer", "round": 1, "parentJob": nil,
		"workspaceRoot": chainTop, "baseSha": baseCommit, "status": "completed",
		"effectiveModel": "implementer-model", "destructiveReach": "DESIGN-BEARING",
		"chainClosed": true, "gateWidth": "area", "independentCritiqueJobRef": "critic",
		"reviewRoundLimit": 3, "criticRoundsConsumed": 3,
		"goalBudget": map[string]any{"attempts": 1, "minutes": 20},
	}
	writeFixtureJSON("artifacts/agents/jobs/impl.json", implementer)
	writeFixtureJSON("artifacts/agents/impl/rounds/1/return.json", map[string]any{
		"jobId": "impl", "round": 1, "diffBoundary": []string{"metasystem/internal/app/source.txt"},
	})
	if code := runValidateConformance([]string{"--root", project, "--stage", "review", "--job", "impl"}); code != 0 {
		t.Fatalf("review conformance exited %d", code)
	}
	reviewPath := filepath.Join(project, "artifacts", "agents", "impl", "rounds", "1", "review.json")
	patchPath := filepath.Join(project, "artifacts", "agents", "impl", "rounds", "1", "diff.patch")
	reviewBefore, _ := os.ReadFile(reviewPath)
	patchBefore, _ := os.ReadFile(patchPath)
	var review map[string]string
	if err := json.Unmarshal(reviewBefore, &review); err != nil {
		t.Fatal(err)
	}
	reviewedTree := review["reviewedTree"]
	writeFixtureJSON("artifacts/agents/jobs/critic.json", map[string]any{
		"jobId": "critic", "role": "code-critic", "round": 1, "parentJob": nil,
		"reviews": "impl", "status": "completed", "effectiveModel": "critic-model",
		"chainClosed": true, "findingRegister": []any{}, "findingRegisterRound": 1,
		"reviewRoundLimit": 3, "criticRoundsConsumed": 3,
	})
	writeFixtureJSON("artifacts/agents/critic/rounds/1/return.json", map[string]any{
		"jobId": "critic", "round": 1, "reviewedTree": reviewedTree,
		"findings": []any{}, "verdictMaterialCount": 0,
	})
	criticPath := filepath.Join(project, "artifacts", "agents", "critic", "rounds", "1", "return.json")
	criticBefore, _ := os.ReadFile(criticPath)
	rootBefore, _ := os.ReadFile(filepath.Join(project, "artifacts", "agents", "jobs", "impl.json"))
	criticJobPath := filepath.Join(project, "artifacts", "agents", "jobs", "critic.json")
	criticJobBefore, _ := os.ReadFile(criticJobPath)

	writeReceiptFixture(t, project, "internal/app/source.txt",
		"base-one\nkeep-two\nuntested-three\nkeep-four\nMAIN\nkeep-six\n")
	writeReceiptFixture(t, top, "sibling.txt", "main sibling\n")
	runReceiptGit(t, top, "add", "metasystem/internal/app/source.txt", "sibling.txt")
	runReceiptGit(t, top, "commit", "-qm", "main moved")
	targetCommit := runReceiptGit(t, top, "rev-parse", "HEAD")
	targetTree, err := (gittree.Workspace{Dir: project}).HeadTree()
	if err != nil {
		t.Fatal(err)
	}
	baseTree, _ := (gittree.Workspace{Dir: project}).TreeOf(baseCommit)
	merge, err := (gittree.Workspace{Dir: project}).DisjointMerge(baseTree, reviewedTree, targetTree)
	if err != nil {
		t.Fatal(err)
	}
	certifiedPaths, _ := (gittree.Workspace{Dir: project}).ChangedPaths(targetTree, merge.MergedTree)
	if err := (gittree.Workspace{Dir: project}).MaterializePaths(merge.MergedTree, certifiedPaths); err != nil {
		t.Fatal(err)
	}
	runReceiptGit(t, top, "add", "metasystem/internal/app/source.txt")
	candidate := runReceiptGit(t, top, "write-tree")
	candidate = runReceiptGit(t, top, "rev-parse", candidate+":metasystem")
	ordinary := landing.Observe(landing.ObserveParams{RepoRoot: project, CandidateTree: candidate, Chain: "impl"})
	if ordinary.Code != "chain-output-mismatch" || ordinary.Bar != landing.BarRefusal {
		t.Fatalf("ordinary path did not isolate the historical mismatch: %+v", ordinary)
	}

	// Restore T while the recertifier works in the chain worktree.
	runReceiptGit(t, top, "reset", "--hard", targetCommit)
	chainBefore, _ := (gittree.Workspace{Dir: chainProject}).Snapshot("HEAD")
	if _, errs, code := validate.ConformanceWithOptions(project, "recertify", "impl", validate.ConformanceOptions{}); code == 0 ||
		!strings.Contains(strings.Join(errs, "\n"), "chain-recertification-test-command-refused") {
		t.Fatalf("missing area command refusal: code=%d errors=%v", code, errs)
	}
	chainAfterMissing, _ := (gittree.Workspace{Dir: chainProject}).Snapshot("HEAD")
	if chainAfterMissing != chainBefore {
		t.Fatal("missing command materialized the source worktree")
	}
	testCommand := "grep -q '^CHAIN$' internal/app/source.txt && grep -q '^MAIN$' internal/app/source.txt"
	if code := runValidateConformance([]string{"--root", project, "--stage", "recertify", "--job", "impl", "--test-command", testCommand}); code != 0 {
		t.Fatalf("recertification exited %d", code)
	}
	records, err := filepath.Glob(filepath.Join(project, "artifacts", "agents", "landing", "recertifications", "impl", "*", "record.json"))
	if err != nil || len(records) != 1 {
		t.Fatalf("recertification records = %v, error %v", records, err)
	}
	topRoot, _ := (gittree.Workspace{Dir: project}).TopLevel()
	resolvedRecord, _ := filepath.EvalSymlinks(records[0])
	resolvedTop, _ := filepath.EvalSymlinks(topRoot)
	recertification, _ := filepath.Rel(resolvedTop, resolvedRecord)
	recertification = filepath.ToSlash(recertification)
	verified, err := validate.VerifyRecertification(project, "impl", recertification)
	if err != nil {
		t.Fatal(err)
	}
	if verified.Record.TargetCommit != targetCommit || verified.Record.MergedTree != merge.MergedTree {
		t.Fatalf("proof targets differ: %+v", verified.Record)
	}
	recordBeforeRetry, err := os.ReadFile(records[0])
	if err != nil {
		t.Fatal(err)
	}
	workspace := gittree.Workspace{Dir: project}
	sourceAnchorBefore, err := workspace.ResolveTree(verified.Record.SourceAnchorRef)
	if err != nil {
		t.Fatal(err)
	}
	mergedAnchorBefore, err := workspace.ResolveTree(verified.Record.MergedAnchorRef)
	if err != nil {
		t.Fatal(err)
	}
	if code := runValidateConformance([]string{"--root", project, "--stage", "recertify", "--job", "impl", "--test-command", testCommand}); code != 0 {
		t.Fatalf("idempotent recertification retry exited %d", code)
	}
	recordsAfterRetry, err := filepath.Glob(filepath.Join(project, "artifacts", "agents", "landing", "recertifications", "impl", "*", "record.json"))
	if err != nil || len(recordsAfterRetry) != 1 || recordsAfterRetry[0] != records[0] {
		t.Fatalf("idempotent retry published a different record: %v, error %v", recordsAfterRetry, err)
	}
	recordAfterRetry, err := os.ReadFile(recordsAfterRetry[0])
	if err != nil || !bytes.Equal(recordBeforeRetry, recordAfterRetry) {
		t.Fatalf("idempotent retry changed record bytes: %v", err)
	}
	if sourceAnchorAfter, anchorErr := workspace.ResolveTree(verified.Record.SourceAnchorRef); anchorErr != nil || sourceAnchorAfter != sourceAnchorBefore {
		t.Fatalf("idempotent retry changed source anchor: %s %v", sourceAnchorAfter, anchorErr)
	}
	if mergedAnchorAfter, anchorErr := workspace.ResolveTree(verified.Record.MergedAnchorRef); anchorErr != nil || mergedAnchorAfter != mergedAnchorBefore {
		t.Fatalf("idempotent retry changed merged anchor: %s %v", mergedAnchorAfter, anchorErr)
	}

	// Recertified merge runs the same runtime-instruction manifest gate as
	// ordinary merge. Misclassify one declared instruction without changing
	// the proof or candidate, then restore the manifest for the passing path.
	manifestPath := filepath.Join(project, "scripts", "agents", "path-classes.txt")
	misclassified := bytes.Replace(pathClasses, []byte("install:AGENTS.md behavior"), []byte("install:AGENTS.md record"), 1)
	if bytes.Equal(misclassified, pathClasses) {
		t.Fatal("fixture manifest does not contain the declared AGENTS.md behavior row")
	}
	if err := os.WriteFile(manifestPath, misclassified, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, errs, code := validate.ConformanceWithOptions(project, "merge", "impl", validate.ConformanceOptions{Recertification: recertification}); code == 0 ||
		!strings.Contains(strings.Join(errs, "\n"), "runtime instruction file AGENTS.md has manifest class record, not behavior") {
		t.Fatalf("recertified runtime-instruction manifest mismatch was not refused: code=%d errors=%v", code, errs)
	}
	if err := os.WriteFile(manifestPath, pathClasses, 0o644); err != nil {
		t.Fatal(err)
	}

	// A non-mission waiver takes the same branch on both merge paths. This
	// source change is not Markdown, so the common prose-only waiver refuses.
	var waiverRecord map[string]any
	if err := json.Unmarshal(rootBefore, &waiverRecord); err != nil {
		t.Fatal(err)
	}
	waiverRecord["critiqueWaived"] = map[string]any{"class": "prose-under-30"}
	writeFixtureJSON("artifacts/agents/jobs/impl.json", waiverRecord)
	if _, errs, code := validate.ConformanceWithOptions(project, "merge", "impl", validate.ConformanceOptions{Recertification: recertification}); code == 0 ||
		!strings.Contains(strings.Join(errs, "\n"), "prose-under-30 includes non-Markdown paths") ||
		!strings.Contains(strings.Join(errs, "\n"), "'internal/app/source.txt'") {
		t.Fatalf("recertified waiver mismatch was not refused for the source path: code=%d errors=%v", code, errs)
	}
	if err := os.WriteFile(filepath.Join(project, "artifacts", "agents", "jobs", "impl.json"), rootBefore, 0o644); err != nil {
		t.Fatal(err)
	}
	if code := runValidateConformance([]string{"--root", project, "--stage", "merge", "--job", "impl", "--recertification", recertification}); code != 0 {
		t.Fatalf("recertified merge conformance exited %d", code)
	}

	if err := (gittree.Workspace{Dir: project}).MaterializePaths(merge.MergedTree, certifiedPaths); err != nil {
		t.Fatal(err)
	}
	runReceiptGit(t, top, "add", "metasystem/internal/app/source.txt")
	candidate = runReceiptGit(t, top, "write-tree")
	candidate = runReceiptGit(t, top, "rev-parse", candidate+":metasystem")
	if _, err := landing.CreateTestReceipt(project, candidate, testCommand, os.Stdout, os.Stderr); err != nil {
		t.Fatal(err)
	}
	receiptPath := landing.TestReceiptPath(project, candidate)
	passing := landing.Observe(landing.ObserveParams{RepoRoot: project, CandidateTree: candidate, Chain: "impl",
		Recertification: recertification, TestReceipt: receiptPath})
	if passing.Bar != landing.BarChain || passing.Code != "closed-chain" || passing.Verdict != "pass" {
		t.Fatalf("recertified landing did not pass bar a: %+v", passing)
	}

	if _, err := landing.CreateTestReceipt(project, candidate, "true", os.Stdout, os.Stderr); err != nil {
		t.Fatal(err)
	}
	wrongCommand := landing.Observe(landing.ObserveParams{RepoRoot: project, CandidateTree: candidate, Chain: "impl",
		Recertification: recertification, TestReceipt: receiptPath})
	if wrongCommand.Code != "chain-recertification-test-command-refused" || wrongCommand.Mode != "refuse" || wrongCommand.Detail != "receipt-command" {
		t.Fatalf("wrong command refusal = %+v", wrongCommand)
	}
	if _, err := landing.CreateTestReceipt(project, candidate, testCommand, os.Stdout, os.Stderr); err != nil {
		t.Fatal(err)
	}

	// The tiny command deliberately ignores line three. A fresh green
	// receipt therefore cannot hide a quiet edit in a certified file.
	writeReceiptFixture(t, project, "internal/app/source.txt",
		"CHAIN\nkeep-two\nTAMPERED\nkeep-four\nMAIN\nkeep-six\n")
	runReceiptGit(t, top, "add", "metasystem/internal/app/source.txt")
	tampered := runReceiptGit(t, top, "write-tree")
	tampered = runReceiptGit(t, top, "rev-parse", tampered+":metasystem")
	if _, err := landing.CreateTestReceipt(project, tampered, testCommand, os.Stdout, os.Stderr); err != nil {
		t.Fatal(err)
	}
	tamperObservation := landing.Observe(landing.ObserveParams{RepoRoot: project, CandidateTree: tampered, Chain: "impl",
		Recertification: recertification, TestReceipt: landing.TestReceiptPath(project, tampered)})
	if tamperObservation.Code != "chain-output-mismatch" || tamperObservation.Mode != "refuse" {
		t.Fatalf("certified-file tamper refusal = %+v", tamperObservation)
	}

	// Restore the exact merged candidate and add a classified behavior path
	// outside S. This isolates the existing uncarried-path refusal.
	if err := (gittree.Workspace{Dir: project}).MaterializePaths(merge.MergedTree, certifiedPaths); err != nil {
		t.Fatal(err)
	}
	writeReceiptFixture(t, project, "internal/app/extra.go", "package app\n")
	runReceiptGit(t, top, "add", "metasystem/internal/app/source.txt", "metasystem/internal/app/extra.go")
	extraCandidate := runReceiptGit(t, top, "write-tree")
	extraCandidate = runReceiptGit(t, top, "rev-parse", extraCandidate+":metasystem")
	if _, err := landing.CreateTestReceipt(project, extraCandidate, testCommand, os.Stdout, os.Stderr); err != nil {
		t.Fatal(err)
	}
	extraObservation := landing.Observe(landing.ObserveParams{RepoRoot: project, CandidateTree: extraCandidate, Chain: "impl",
		Recertification: recertification, TestReceipt: landing.TestReceiptPath(project, extraCandidate)})
	if extraObservation.Code != "chain-has-uncarried-paths" || extraObservation.Verdict != "would-refuse" ||
		extraObservation.Mode != "observe" || extraObservation.Bar != landing.BarRefusal {
		t.Fatalf("uncarried path refusal = %+v", extraObservation)
	}

	// Fabricate a fully self-consistent result record whose only lie is the
	// merged tree. Hashes, patch equation, anchors, candidate and fresh receipt
	// all agree with M*; independent replay from genuine B/R/T must still name
	// the genuine M and hard-refuse at merged-tree-mismatch.
	runReceiptGit(t, top, "reset", "--hard", targetCommit)
	if err := (gittree.Workspace{Dir: project}).MaterializePaths(merge.MergedTree, certifiedPaths); err != nil {
		t.Fatal(err)
	}
	writeReceiptFixture(t, project, "internal/app/source.txt",
		"CHAIN\nkeep-two\nFORGED\nkeep-four\nMAIN\nkeep-six\n")
	runReceiptGit(t, top, "add", "metasystem/internal/app/source.txt")
	forgedWhole := runReceiptGit(t, top, "write-tree")
	forgedTree := runReceiptGit(t, top, "rev-parse", forgedWhole+":metasystem")
	forgedPatch, err := (gittree.Workspace{Dir: project}).Diff(verified.Record.TargetTree, forgedTree)
	if err != nil {
		t.Fatal(err)
	}
	forged := verified.Record
	forged.MergedTree = forgedTree
	forged.MergedWholeTree = forgedWhole
	forged.MergedPatchDigest = fmt.Sprintf("%x", sha256.Sum256(forgedPatch))
	forged.InputDigest, err = validate.RecertificationInputDigest(forged)
	if err != nil {
		t.Fatal(err)
	}
	if forged.InputDigest != verified.Record.InputDigest {
		t.Fatal("merged-result forgery unexpectedly changed the immutable-input digest")
	}
	forged.RecordDigest, err = validate.RecertificationRecordDigest(forged)
	if err != nil {
		t.Fatal(err)
	}
	genuineRecordBytes, err := os.ReadFile(records[0])
	if err != nil {
		t.Fatal(err)
	}
	genuinePatchBytes, err := os.ReadFile(filepath.Join(filepath.Dir(records[0]), "diff.patch"))
	if err != nil {
		t.Fatal(err)
	}
	forgedRecordBytes, err := wiredoc.RenderValue(forged)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(records[0], forgedRecordBytes, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(filepath.Dir(records[0]), "diff.patch"), forgedPatch, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := (gittree.Workspace{Dir: project}).AnchorRef(forged.MergedAnchorRef, forgedWhole, "tree"); err != nil {
		t.Fatal(err)
	}
	if replay, applyErr := (gittree.Workspace{Dir: project}).Apply(forged.TargetTree, forgedPatch); applyErr != nil || replay != forgedTree {
		t.Fatalf("forged patch equation is not internally consistent: tree=%s err=%v", replay, applyErr)
	}
	if digest, digestErr := validate.RecertificationRecordDigest(forged); digestErr != nil || digest != forged.RecordDigest {
		t.Fatalf("forged record digest is not internally consistent: %s %v", digest, digestErr)
	}
	if anchor, anchorErr := (gittree.Workspace{Dir: project}).ResolveTree(forged.MergedAnchorRef); anchorErr != nil || anchor != forgedWhole {
		t.Fatalf("forged anchor is not internally consistent: %s %v", anchor, anchorErr)
	}
	if _, err := landing.CreateTestReceipt(project, forgedTree, testCommand, os.Stdout, os.Stderr); err != nil {
		t.Fatal(err)
	}
	forgedObservation := landing.Observe(landing.ObserveParams{RepoRoot: project, CandidateTree: forgedTree, Chain: "impl",
		Recertification: recertification, TestReceipt: landing.TestReceiptPath(project, forgedTree)})
	if forgedObservation.Code != "chain-recertification-unproven" || forgedObservation.Mode != "refuse" ||
		forgedObservation.Bar != landing.BarRefusal || forgedObservation.Detail != "merged-tree-mismatch" {
		t.Fatalf("self-consistent forged proof refusal = %+v", forgedObservation)
	}
	if err := os.WriteFile(records[0], genuineRecordBytes, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(filepath.Dir(records[0]), "diff.patch"), genuinePatchBytes, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := (gittree.Workspace{Dir: project}).AnchorRef(verified.Record.MergedAnchorRef, verified.Record.MergedWholeTree, "tree"); err != nil {
		t.Fatal(err)
	}

	// Move only target identity: T' has the exact same tree. Recreate the
	// exact candidate and receipt afterward so target identity is the sole
	// failed prerequisite.
	runReceiptGit(t, top, "reset", "--hard", targetCommit)
	runReceiptGit(t, top, "commit", "--allow-empty", "-qm", "same-tree target move")
	if err := (gittree.Workspace{Dir: project}).MaterializePaths(merge.MergedTree, certifiedPaths); err != nil {
		t.Fatal(err)
	}
	runReceiptGit(t, top, "add", "metasystem/internal/app/source.txt")
	movedCandidate := runReceiptGit(t, top, "write-tree")
	movedCandidate = runReceiptGit(t, top, "rev-parse", movedCandidate+":metasystem")
	if _, err := landing.CreateTestReceipt(project, movedCandidate, testCommand, os.Stdout, os.Stderr); err != nil {
		t.Fatal(err)
	}
	movedObservation := landing.Observe(landing.ObserveParams{RepoRoot: project, CandidateTree: movedCandidate, Chain: "impl",
		Recertification: recertification, TestReceipt: landing.TestReceiptPath(project, movedCandidate)})
	if movedObservation.Code != "chain-recertification-target-moved" || movedObservation.Mode != "refuse" {
		t.Fatalf("same-tree target movement refusal = %+v", movedObservation)
	}

	rootAfter, _ := os.ReadFile(filepath.Join(project, "artifacts", "agents", "jobs", "impl.json"))
	reviewAfter, _ := os.ReadFile(reviewPath)
	patchAfter, _ := os.ReadFile(patchPath)
	criticAfter, _ := os.ReadFile(criticPath)
	criticJobAfter, _ := os.ReadFile(criticJobPath)
	if !bytes.Equal(rootBefore, rootAfter) || !bytes.Equal(reviewBefore, reviewAfter) ||
		!bytes.Equal(patchBefore, patchAfter) || !bytes.Equal(criticBefore, criticAfter) || !bytes.Equal(criticJobBefore, criticJobAfter) {
		t.Fatal("recertification changed review accounting, original review/patch, critic return, or goal budget bytes")
	}
}

func TestRecertifiedLandingParksOnOriginMove(t *testing.T) {
	runRecertifiedLandingFixture(t, "", true)
}

func TestRecertifiedLandingPublishesNestedSchemaTwoCandidate(t *testing.T) {
	runRecertifiedLandingFixture(t, "metasystem", false)
}

func runRecertifiedLandingFixture(t *testing.T, prefix string, moveOrigin bool) {
	bed, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	top := filepath.Join(bed, "local")
	root := filepath.Join(top, prefix)
	sourcePath := filepath.ToSlash(filepath.Join(prefix, "internal/app/source.txt"))
	groupCWD := prefix
	if groupCWD == "" {
		groupCWD = "."
	}
	chainRoot := filepath.Join(bed, "chain")
	peer := filepath.Join(bed, "peer")
	remote := filepath.Join(bed, "origin.git")
	for _, directory := range []string{
		filepath.Join(root, "bin"), filepath.Join(root, "internal", "app"),
		filepath.Join(root, "scripts", "agents"), filepath.Join(root, "memory"),
		filepath.Join(root, "plans", "goals"),
	} {
		if err := os.MkdirAll(directory, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	copyFixture := func(source, destination string, mode os.FileMode) {
		t.Helper()
		data, err := os.ReadFile(source)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(destination)), data, mode); err != nil {
			t.Fatal(err)
		}
	}
	copyFixture("../../scripts/agents/land.sh", "scripts/agents/land.sh", 0o755)
	copyFixture("../../scripts/agents/path-classes.txt", "scripts/agents/path-classes.txt", 0o644)
	copyFixture("../../scripts/agents/landing-classes.json", "scripts/agents/landing-classes.json", 0o644)
	copyFixture("../../scripts/agents/landing-promotion.json", "scripts/agents/landing-promotion.json", 0o644)
	copyFixture("../../memory/rulings.md", "memory/rulings.md", 0o644)

	// This is the established landing fixture's reduced commit boundary: it
	// performs the real Go observation over the exact staged tree and then a
	// real Git commit, while omitting the unrelated full static re-proof.
	writeReceiptFixture(t, root, "scripts/agents/commit.sh", `#!/usr/bin/env bash
set -euo pipefail
root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)
chain= goal= receipt= recertification=
commit_args=()
while (( $# )); do
  case "$1" in
    --chain) chain=$2; shift 2 ;;
    --goal) goal=$2; shift 2 ;;
    --test-receipt) receipt=$2; shift 2 ;;
    --recertification) recertification=$2; shift 2 ;;
    *) commit_args+=("$1"); shift ;;
  esac
done
tree=$(git -C "$root" write-tree)
prefix=$(git -C "$root" rev-parse --show-prefix)
if [[ -n "$prefix" ]]; then
  tree=$(git -C "$root" rev-parse "$tree:${prefix%/}")
fi
actor=$(git -C "$root" config --get metasystem.goal.machine)+${METASYSTEM_OWNER_LINEAGE:?}
observation=$("$root/bin/metasystem" landing observe --root "$root" --tree "$tree" \
  --chain "$chain" --goal "$goal" --actor "$actor" --test-receipt "$receipt" \
  --recertification "$recertification")
verdict=$("$root/bin/metasystem" json get --value "$observation" --field verdictTrailer)
[[ "$verdict" == "pass bar=a" ]] || { printf '%s\n' "$observation" >&2; exit 83; }
provenance=$("$root/bin/metasystem" json get --value "$observation" --field provenance)
git commit "${commit_args[@]}" --trailer "Landing-Provenance: $provenance" \
  --trailer "Landing-Provenance-Verdict: $verdict"
`)
	if err := os.Chmod(filepath.Join(root, "scripts", "agents", "commit.sh"), 0o755); err != nil {
		t.Fatal(err)
	}
	build := exec.Command("go", "build", "-o", filepath.Join(root, "bin", "metasystem"), ".")
	build.Env = gittree.ScrubbedEnviron()
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build fixture engine: %v\n%s", err, out)
	}

	writeReceiptFixture(t, root, ".gitignore", "artifacts/\nbin/\n")
	writeReceiptFixture(t, root, "metasystem.conf", "metasystem.version=1\nmetasystem.runtimes=fake\nrole.code-critic.runtime=fake\ntesting.contract=testing.json\ndispatch.cap-min=1\ndispatch.cap-max=120\n")
	writeReceiptFixture(t, root, "internal/app/source.txt", "base-one\nkeep-two\nkeep-three\nkeep-four\nbase-five\n")
	recertificationContract := testpolicy.Contract{SchemaVersion: 1,
		ProjectRisk: testpolicy.ProjectRisk{Severity: 1, Exposure: 1, Reversibility: "revert", Detection: "immediate", Recovery: "bounded"},
		Surfaces:    []testpolicy.Surface{{ID: "application", Paths: []string{filepath.ToSlash(filepath.Join(prefix, "internal/app/**"))}, Standard: []string{"candidate-smoke"}, Critical: []string{"recertified-landing"}}},
		Groups: []testpolicy.Group{{ID: "candidate-smoke", Kind: "unit", Adapter: "command", CWD: groupCWD, Inputs: []string{filepath.ToSlash(filepath.Join(prefix, "internal/app/**"))},
			Outputs: []string{filepath.ToSlash(filepath.Join(prefix, "reports"))}, Obligations: []string{"recertified-landing"}, Platforms: []string{"any"}, TargetMS: 1000,
			Env:     map[string]string{"PATH": "/usr/bin:/bin:/usr/sbin:/sbin"},
			Argv:    []string{"/bin/sh", "-c", "grep -q '^CHAIN$' internal/app/source.txt && grep -q '^MAIN$' internal/app/source.txt && mkdir -p reports && printf '%s\\n' '<testsuite><testcase classname=\"landing\" name=\"recertified\"/></testsuite>' >reports/result.xml"},
			Reports: []string{filepath.ToSlash(filepath.Join(prefix, "reports"))}, Format: "junit-xml", ExpectedTests: []testpolicy.ExpectedTest{{Report: filepath.ToSlash(filepath.Join(prefix, "reports/result.xml")), Classname: "landing", Name: "recertified"}}}},
		Always: testpolicy.Always{Canary: []string{"candidate-smoke"}, Standard: []string{"candidate-smoke"}}, Unknown: []string{"candidate-smoke"}, Cadence: []string{"candidate-smoke"}}
	contractBytes, err := json.Marshal(recertificationContract)
	if err != nil {
		t.Fatal(err)
	}
	writeReceiptFixture(t, root, "testing.json", string(contractBytes))
	now := time.Now().UTC()
	rootRecord := &goal.RootRecord{
		Identity: "01J5X000000000000000000000", FormatVersion: "1", SyncMode: goal.SyncRemote,
		MigrationEpoch: "2026-09-08T00:00:00Z", ManifestDigest: strings.Repeat("ab", 32),
		MigrationMode: "manifest", Revision: 1,
		History: []goal.HistoryLine{{At: "2026-09-08T00:00:00Z", Opid: "01J5X0000000000000000000A0-race-node-1a2b3c4d", Verb: "migrate", Actor: "race-node+race-lineage", Keep: -1}},
	}
	budget := &goal.Budget{ElapsedLimit: "100000h", AttemptLimit: 2, ReservedJobMinutesLimit: 2, ActiveJobLimit: 1, ReviewRoundLimit: 3}
	risk := &goal.RiskRecord{Severity: 1, Novelty: 1, Exposure: 1, Accumulation: 1, Basis: "The fixture runs one bounded recertified landing check."}
	intent := "Prove a raced landing parks without changing its claim."
	goalFile := &goal.GoalFile{
		Id: "landing-goal", State: goal.StateClaimed, Tier: 1, Risk: risk, Intent: intent,
		Origin: goal.OriginMain, NextStep: "Run the recertified landing.", OpenedAt: "2026-09-08T00:01:00Z", Revision: 3, Budget: budget,
		Claimed: &goal.ClaimRecord{Machine: "race-node", Lineage: "race-lineage", At: "2026-09-08T00:01:30Z", Revision: 2, AccountingRevision: 2},
		Approved: &goal.ApprovalRecord{By: "human:fixture", At: now.Add(-time.Minute).Format(time.RFC3339), Revision: 3,
			Opid: goal.Opid("01J5X0000000000000000000C0", "race-node", "race-lineage"), Authority: goal.ApprovalAuthorityProven,
			Digest: goal.ApprovalDigest(intent, 1, *budget, risk)},
		StopCapability: &goal.StopCapability{Generation: 3, Revision: 2, Machine: "race-node", ClaimEpoch: 1},
		History: []goal.HistoryLine{
			{At: "2026-09-08T00:01:00Z", Opid: "01J5X0000000000000000000B0-race-node-1a2b3c4d", Verb: "open", Actor: "race-node+race-lineage", Targets: []string{"landing-goal"}, Keep: -1},
			{At: "2026-09-08T00:01:30Z", Opid: goal.Opid("01J5X0000000000000000000B1", "race-node", "race-lineage"), Verb: "claim", Actor: "race-node+race-lineage", Targets: []string{"landing-goal"}, Keep: -1},
			{At: now.Add(-time.Minute).Format(time.RFC3339), Opid: goal.Opid("01J5X0000000000000000000C0", "race-node", "race-lineage"), Verb: "approve", Actor: "human:fixture", Targets: []string{"landing-goal"}, Keep: -1},
		},
	}
	if err := os.WriteFile(filepath.Join(root, "plans", "goals", "backlog.md"), goal.RenderRoot(rootRecord), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "plans", "goals", "landing-goal.md"), goal.RenderFile(goalFile), 0o644); err != nil {
		t.Fatal(err)
	}
	writeReceiptFixture(t, top, "sibling.txt", "application outside installation\n")
	runReceiptGit(t, top, "init", "-q", "-b", "main")
	runReceiptGit(t, top, "config", "user.name", "race fixture")
	runReceiptGit(t, top, "config", "user.email", "race@example.invalid")
	runReceiptGit(t, top, "config", "metasystem.goal.machine", "race-node")
	runReceiptGit(t, top, "add", ".")
	runReceiptGit(t, top, "commit", "-qm", "base")
	baseCommit := runReceiptGit(t, top, "rev-parse", "HEAD")
	runReceiptGit(t, top, "worktree", "add", "-q", "-b", "chain", chainRoot, "HEAD")

	writeReceiptFixture(t, chainRoot, sourcePath, "CHAIN\nkeep-two\nkeep-three\nkeep-four\nbase-five\n")
	writeFixtureJSON := func(relative string, value any) {
		t.Helper()
		data, err := json.MarshalIndent(value, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		writeReceiptFixture(t, root, relative, string(append(data, '\n')))
	}
	writeFixtureJSON("artifacts/agents/jobs/park-chain.json", map[string]any{
		"jobId": "park-chain", "role": "implementer", "round": 1, "parentJob": nil,
		"workspaceRoot": chainRoot, "baseSha": baseCommit, "status": "completed",
		"goalId": "landing-goal", "goalRevision": 2, "operationId": "park-chain-reservation", "capMin": 1,
		"effectiveModel": "implementer-model", "destructiveReach": "DESIGN-BEARING",
		"chainClosed": true, "gateWidth": "area", "independentCritiqueJobRef": "park-critic",
		"reviewRoundLimit": 3, "criticRoundsConsumed": 3,
	})
	writeFixtureJSON("artifacts/agents/park-chain/rounds/1/return.json", map[string]any{
		"jobId": "park-chain", "round": 1, "diffBoundary": []string{sourcePath},
	})
	if code := runValidateConformance([]string{"--root", root, "--stage", "review", "--job", "park-chain"}); code != 0 {
		t.Fatalf("review conformance exited %d", code)
	}
	var review map[string]string
	reviewBytes, err := os.ReadFile(filepath.Join(root, "artifacts", "agents", "park-chain", "rounds", "1", "review.json"))
	if err != nil || json.Unmarshal(reviewBytes, &review) != nil {
		t.Fatalf("read review record: %v", err)
	}
	writeFixtureJSON("artifacts/agents/jobs/park-critic.json", map[string]any{
		"jobId": "park-critic", "role": "code-critic", "round": 1, "parentJob": nil,
		"goalId":  nil,
		"reviews": "park-chain", "status": "completed", "effectiveModel": "critic-model",
		"chainClosed": true, "findingRegister": []any{}, "findingRegisterRound": 1,
		"reviewRoundLimit": 3, "criticRoundsConsumed": 3,
	})
	writeFixtureJSON("artifacts/agents/park-critic/rounds/1/return.json", map[string]any{
		"jobId": "park-critic", "round": 1, "reviewedTree": review["reviewedTree"],
		"findings": []any{}, "verdictMaterialCount": 0,
	})

	writeReceiptFixture(t, root, "internal/app/source.txt", "base-one\nkeep-two\nkeep-three\nkeep-four\nMAIN\n")
	runReceiptGit(t, top, "add", sourcePath)
	runReceiptGit(t, top, "commit", "-qm", "target")
	target := runReceiptGit(t, top, "rev-parse", "HEAD")
	runReceiptGit(t, top, "update-ref", goal.AcceptedRef, target)
	runReceiptGit(t, top, "config", "metasystem.steward.landing-ref", "refs/remotes/origin/main")
	runReceiptGit(t, bed, "init", "-q", "--bare", "-b", "main", remote)
	runReceiptGit(t, top, "remote", "add", "origin", remote)
	runReceiptGit(t, top, "push", "-q", "-u", "origin", "main")
	// The recertification selector is owned by the retained destination build,
	// not by the candidate source loaded into this test process. Rebuild the
	// fixture executable with T's source stamp and enroll those exact bytes.
	build = exec.Command("go", "build", "-buildvcs=false", "-ldflags",
		"-X github.com/widoriezebos/agentic-tools/metasystem/internal/supervise.BuildStamp="+target, "-o", filepath.Join(root, "bin", "metasystem"), ".")
	build.Env = gittree.ScrubbedEnviron()
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build source-bound recertification engine: %v\n%s", err, out)
	}
	engineDigest, err := fileSHA256(filepath.Join(root, "bin", "metasystem"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(steward.RepoIdentityPath(root)), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := steward.MintIdentity(steward.RepoIdentityPath(root), steward.InstallIdentity{RepoIdentity: root, Generation: 1,
		InstallPath: filepath.Join(root, "bin", "metasystem"), InstallDigest: "sha256:" + engineDigest, MintedAt: now.Format(time.RFC3339),
		Enrollment: steward.EnrollmentFixture, EngineBuild: target[:12], LandedCommit: target, LandingRef: "refs/remotes/origin/main"}); err != nil {
		t.Fatal(err)
	}
	runReceiptGit(t, bed, "clone", "-q", remote, peer)
	runReceiptGit(t, peer, "config", "user.name", "race peer")
	runReceiptGit(t, peer, "config", "user.email", "race-peer@example.invalid")

	if code := runValidateConformance([]string{"--root", root, "--stage", "recertify", "--job", "park-chain"}); code != 0 {
		t.Fatalf("recertification exited %d", code)
	}
	records, err := filepath.Glob(filepath.Join(root, "artifacts", "agents", "landing", "recertifications", "park-chain", "*", "record.json"))
	if err != nil || len(records) != 1 {
		t.Fatalf("recertification records = %v, error %v", records, err)
	}
	proof, err := filepath.Rel(top, records[0])
	if err != nil {
		t.Fatal(err)
	}
	proof = filepath.ToSlash(proof)
	verified, err := validate.VerifyRecertification(root, "park-chain", proof)
	if err != nil {
		t.Fatal(err)
	}
	if err := (gittree.Workspace{Dir: root}).MaterializePaths(verified.Record.MergedTree, verified.Record.CertifiedPaths); err != nil {
		t.Fatal(err)
	}
	runReceiptGit(t, top, "add", sourcePath)
	candidateTree := runReceiptGit(t, top, "write-tree")
	identityTable := filepath.Join(bed, "process-identities.json")
	if err := os.WriteFile(identityTable, []byte(fmt.Sprintf(`{"%d":{"terminal":true}}`, os.Getpid())), 0o600); err != nil {
		t.Fatal(err)
	}
	testReceiptCommand := exec.Command(filepath.Join(root, "bin", "metasystem"), "landing", "test-receipt", "--root", root,
		"--tree", candidateTree, "--mode", "auto", "--goal", "landing-goal", "--cap-min", "1")
	testReceiptCommand.Env = append(receiptCanaryEnvironment(), "METASYSTEM_FAKE_PROCESS_IDENTITY_FILE="+identityTable)
	if !moveOrigin {
		started, ok := lease.StartedAt(int64(os.Getpid()), nil)
		if !ok {
			t.Fatal("fixture main start time is unreadable")
		}
		if _, err := lease.Announce(root, "nested-receipt", int64(os.Getpid()), started, "nested-receipt", "fake", "race-lineage"); err != nil {
			t.Fatal(err)
		}
		testReceiptCommand.Env = append(testReceiptCommand.Env, "METASYSTEM_OWNER_LINEAGE=race-lineage")
	}
	if output, err := testReceiptCommand.CombinedOutput(); err != nil {
		t.Fatalf("public schema-two recertification receipt failed: %v\n%s", err, output)
	}
	receiptBytes, err := os.ReadFile(landing.TestReceiptPath(root, candidateTree))
	if err != nil {
		t.Fatal(err)
	}
	var receipt landing.TestReceipt
	if err := json.Unmarshal(receiptBytes, &receipt); err != nil || receipt.SchemaVersion != 2 || receipt.Testing == nil || !receipt.Testing.Delivery.Sufficient {
		t.Fatalf("recertification did not retain sufficient schema-two testing evidence: receipt=%+v err=%v", receipt, err)
	}
	landingCandidateTree, err := (gittree.Workspace{Dir: root}).TreeOf(candidateTree)
	if err != nil {
		t.Fatal(err)
	}
	preRace := landing.Observe(landing.ObserveParams{
		RepoRoot: root, CandidateTree: landingCandidateTree, Chain: "park-chain", Goal: "landing-goal",
		Actor: "race-node+race-lineage", Recertification: proof, TestReceipt: landing.TestReceiptPath(root, candidateTree),
	})
	if preRace.Bar != landing.BarChain || preRace.Verdict != "pass" || preRace.Code != "closed-chain" || receipt.Testing == nil {
		t.Fatalf("pre-race evaluator was not green: observation=%+v receipt=%+v", preRace, receipt)
	}
	runReceiptGit(t, top, "reset", "--mixed", "HEAD")

	gitLog := filepath.Join(bed, "git.log")
	pushCount := filepath.Join(bed, "push-count")
	wrapperDir := filepath.Join(bed, "wrapper-bin")
	if err := os.MkdirAll(wrapperDir, 0o755); err != nil {
		t.Fatal(err)
	}
	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	writeReceiptFixture(t, wrapperDir, "git", `#!/usr/bin/env bash
set -euo pipefail
printf '%s\n' "$*" >>"${CLBM_GIT_LOG:-/dev/null}"
if [[ ${1:-} == push ]] && [[ " $* " == *" origin "* ]]; then
  printf 'push\n' >>"$CLBM_PUSH_COUNT"
  if [[ ${CLBM_SKIP_MOVE:-0} != 1 && $(wc -l <"$CLBM_PUSH_COUNT" | tr -d ' ') == 1 ]]; then
    printf 'origin moved\n' >"$CLBM_PEER/peer.txt"
    "$CLBM_REAL_GIT" -C "$CLBM_PEER" add peer.txt
    "$CLBM_REAL_GIT" -C "$CLBM_PEER" commit -qm 'controlled origin move'
    "$CLBM_REAL_GIT" -C "$CLBM_PEER" push -q origin main
  fi
fi
exec "${CLBM_REAL_GIT:-/usr/bin/git}" "$@"
`)
	if err := os.Chmod(filepath.Join(wrapperDir, "git"), 0o755); err != nil {
		t.Fatal(err)
	}
	message := filepath.Join(bed, "message.txt")
	if err := os.WriteFile(message, []byte("recertified landing race\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	goalBefore, _ := os.ReadFile(filepath.Join(root, "plans", "goals", "landing-goal.md"))
	jobBefore, _ := os.ReadFile(filepath.Join(root, "artifacts", "agents", "jobs", "park-chain.json"))

	runAsHolder := func(arguments []string, extraEnv []string) (string, int) {
		t.Helper()
		gate := filepath.Join(bed, "holder-gate-"+strconv.FormatInt(time.Now().UnixNano(), 10))
		script := `while [[ ! -e "$1" ]]; do sleep 0.01; done; shift; "$@"`
		command := exec.Command("bash", append([]string{"-c", script, "holder", gate}, arguments...)...)
		command.Dir = root
		path := wrapperDir + string(os.PathListSeparator) + os.Getenv("PATH")
		env := append(gittree.ScrubbedEnviron(), "PATH="+path, "METASYSTEM_OWNER_LINEAGE=race-lineage")
		env = append(env, extraEnv...)
		command.Env = env
		var output bytes.Buffer
		command.Stdout, command.Stderr = &output, &output
		if err := command.Start(); err != nil {
			t.Fatal(err)
		}
		pid := int64(command.Process.Pid)
		started, ok := lease.StartedAt(pid, nil)
		if !ok {
			_ = command.Process.Kill()
			t.Fatal("holder process start time is unreadable")
		}
		if moveOrigin {
			if _, err := lease.Announce(root, "landing-race", pid, started, "landing-race", "fake", "race-lineage"); err != nil {
				_ = command.Process.Kill()
				t.Fatal(err)
			}
		}
		if err := os.WriteFile(gate, []byte("go\n"), 0o600); err != nil {
			_ = command.Process.Kill()
			t.Fatal(err)
		}
		err := command.Wait()
		if err == nil {
			return output.String(), 0
		}
		var exit *exec.ExitError
		if !errors.As(err, &exit) {
			t.Fatalf("holder command failed without an exit status: %v", err)
		}
		return output.String(), exit.ExitCode()
	}
	if !moveOrigin {
		landOutput, landCode := runAsHolder([]string{
			"bash", filepath.Join(root, "scripts", "agents", "land.sh"), "--skip-transport", "-m", message,
			"--chain", "park-chain", "--goal", "landing-goal", "--recertification", proof,
			"--test-receipt", landing.TestReceiptPath(root, candidateTree), "internal/app/source.txt",
		}, []string{"CLBM_GIT_LOG=" + gitLog, "CLBM_PUSH_COUNT=" + pushCount,
			"CLBM_PEER=" + peer, "CLBM_REAL_GIT=" + realGit, "CLBM_SKIP_MOVE=1"})
		if landCode != 0 {
			t.Fatalf("nested schema-two landing failed: code=%d\n%s", landCode, landOutput)
		}
		candidate := runReceiptGit(t, top, "rev-parse", "HEAD")
		if runReceiptGit(t, top, "rev-parse", "HEAD^1") != target ||
			runReceiptGit(t, top, "rev-parse", "HEAD^{tree}") != candidateTree ||
			runReceiptGit(t, remote, "rev-parse", "refs/heads/main") != candidate {
			t.Fatal("nested landing did not publish exactly the recertified child of the captured destination")
		}
		if receipt.Testing.ProjectRoot != top || receipt.Testing.InstallationPrefix != prefix {
			t.Fatalf("nested receipt lost whole-project authority: %+v", receipt.Testing)
		}
		if got := runReceiptGit(t, top, "show", "HEAD:sibling.txt"); got != "application outside installation" {
			t.Fatalf("nested landing changed application bytes outside its installation: %q", got)
		}
		if trailers := runReceiptGit(t, top, "log", "-1", "--format=%B"); !strings.Contains(trailers, "Landing-Provenance-Verdict: pass bar=a") {
			t.Fatalf("nested landing lost the real observation trailer: %s", trailers)
		}
		pushes, _ := os.ReadFile(pushCount)
		if strings.Count(string(pushes), "push\n") != 1 {
			t.Fatalf("nested landing push count: %q", pushes)
		}
		if got, _ := os.ReadFile(filepath.Join(root, "plans", "goals", "landing-goal.md")); !bytes.Equal(got, goalBefore) {
			t.Fatal("nested landing changed the goal approval or claim")
		}
		return
	}
	unknownWidthOutput, unknownWidthCode := runAsHolder([]string{
		"bash", filepath.Join(root, "scripts", "agents", "land.sh"), "-m", message,
		"--chain", "invalid_chain", "--goal", "landing-goal", "--recertification", proof,
		"--test-receipt", filepath.Join(bed, "missing-receipt.json"), "internal/app/source.txt",
	}, []string{
		"CLBM_GIT_LOG=" + gitLog, "CLBM_PUSH_COUNT=" + pushCount, "CLBM_PEER=" + peer,
		"CLBM_REAL_GIT=" + realGit,
	})
	if unknownWidthCode == 0 || !strings.Contains(unknownWidthOutput, "PARK-FAILED cause=chain-recertification-test-command-refused") ||
		strings.Contains(unknownWidthOutput, "unbound variable") {
		t.Fatalf("unknown gate width did not reach the explicit park failure: code=%d\n%s", unknownWidthCode, unknownWidthOutput)
	}
	runReceiptGit(t, top, "reset", "--mixed", "HEAD")
	landOutput, landCode := runAsHolder([]string{
		"bash", filepath.Join(root, "scripts", "agents", "land.sh"), "-m", message,
		"--chain", "park-chain", "--goal", "landing-goal", "--recertification", proof,
		"--test-receipt", landing.TestReceiptPath(root, candidateTree), "internal/app/source.txt",
	}, []string{
		"CLBM_GIT_LOG=" + gitLog, "CLBM_PUSH_COUNT=" + pushCount, "CLBM_PEER=" + peer,
		"CLBM_REAL_GIT=" + realGit,
	})
	if landCode == 0 || !strings.Contains(landOutput, "chain-recertification-target-moved") ||
		!strings.Contains(landOutput, "PARKED") || strings.Contains(landOutput, "PARK-FAILED") {
		if diagnostics, _ := filepath.Glob(filepath.Join(root, "artifacts", "agents", "landing", "parks", "park-chain", "*.json")); len(diagnostics) > 0 {
			if diagnostic, readErr := os.ReadFile(diagnostics[0]); readErr == nil {
				t.Logf("unexpected park record: %s", diagnostic)
			}
		}
		t.Fatalf("raced landing did not durably park for target movement: code=%d\n%s", landCode, landOutput)
	}
	pushes, _ := os.ReadFile(pushCount)
	commands, _ := os.ReadFile(gitLog)
	pushCommands := 0
	for _, command := range strings.Split(string(commands), "\n") {
		if command == "push" || strings.HasPrefix(command, "push ") {
			pushCommands++
		}
	}
	if strings.Count(string(pushes), "push\n") != 1 || pushCommands != 1 || strings.Contains(string(commands), "rebase") {
		t.Fatalf("raced path retried or rebased: pushes=%q commands=%s", pushes, commands)
	}
	candidate := runReceiptGit(t, top, "rev-parse", "HEAD")
	if candidate == target || runReceiptGit(t, top, "rev-parse", "HEAD^1") != target ||
		runReceiptGit(t, top, "rev-parse", "HEAD^{tree}") != candidateTree {
		t.Fatalf("retained candidate is not the evaluated child of T: %s", candidate)
	}
	parks, err := filepath.Glob(filepath.Join(root, "artifacts", "agents", "landing", "parks", "park-chain", "*.json"))
	if err != nil || len(parks) != 1 {
		t.Fatalf("verified park records = %v, error %v", parks, err)
	}
	parkBytes, err := os.ReadFile(parks[0])
	if err != nil {
		t.Fatal(err)
	}
	var parked landing.ParkRecord
	if err := json.Unmarshal(parkBytes, &parked); err != nil {
		t.Fatal(err)
	}
	recoveryRefs := make(map[string]string, len(parked.RecoveryRefs))
	for _, ref := range parked.RecoveryRefs {
		recoveryRefs[ref.Name] = ref.OID
	}
	digestRecord := parked
	digestRecord.RecordDigest = ""
	digestJSON, err := json.Marshal(digestRecord)
	if err != nil {
		t.Fatal(err)
	}
	var digestValue map[string]any
	if err := json.Unmarshal(digestJSON, &digestValue); err != nil {
		t.Fatal(err)
	}
	delete(digestValue, "recordDigest")
	digestBytes, err := wiredoc.RenderValue(digestValue)
	if err != nil {
		t.Fatal(err)
	}
	wantParkDigest := fmt.Sprintf("%x", sha256.Sum256(digestBytes))
	if parked.State != "parked" || parked.Reason != "chain-recertification-target-moved" ||
		parked.TargetCommit != target || parked.CandidateCommit == nil || *parked.CandidateCommit != candidate ||
		parked.Recertification == nil || *parked.Recertification != proof || parked.Goal == nil || *parked.Goal != "landing-goal" ||
		len(recoveryRefs) != 2 || recoveryRefs[verified.Record.SourceAnchorRef] != verified.Record.SourceWholeTree ||
		recoveryRefs[verified.Record.MergedAnchorRef] != verified.Record.MergedWholeTree ||
		parked.RecordDigest != wantParkDigest || strings.TrimSuffix(filepath.Base(parks[0]), ".json") != wantParkDigest ||
		!strings.Contains(landOutput, filepath.ToSlash(strings.TrimPrefix(parks[0], root+string(filepath.Separator)))) {
		t.Fatalf("raced landing park evidence is incomplete: record=%+v\n%s", parked, landOutput)
	}
	if got, _ := os.ReadFile(filepath.Join(root, "plans", "goals", "landing-goal.md")); !bytes.Equal(got, goalBefore) {
		t.Fatal("raced landing mutated goal/claim bytes")
	}
	if got, _ := os.ReadFile(filepath.Join(root, "artifacts", "agents", "jobs", "park-chain.json")); !bytes.Equal(got, jobBefore) {
		t.Fatal("raced landing mutated chain accounting bytes")
	}

	// Only park publication is obstructed here. The real verb sees the same
	// authenticated claim, candidate, proof and anchors, and no transport runs.
	parkFamily := filepath.Join(root, "artifacts", "agents", "landing", "parks")
	retained := filepath.Join(root, "artifacts", "agents", "landing", "retained-parks")
	if err := os.Rename(parkFamily, retained); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(parkFamily, []byte("obstruction\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	args := []string{
		filepath.Join(root, "bin", "metasystem"), "landing", "park", "--root", root,
		"--chain", "park-chain", "--target", target, "--reason", "chain-recertification-target-moved",
		"--detail", "same captured push refusal", "--recertification", proof, "--candidate-commit", candidate,
		"--recovery-ref", verified.Record.SourceAnchorRef, "--recovery-ref", verified.Record.MergedAnchorRef,
	}
	failedOutput, failedCode := runAsHolder(args, nil)
	if failedCode == 0 || !strings.Contains(failedOutput, "chain-recertification-park-failed") ||
		!strings.Contains(failedOutput, "cause=chain-recertification-target-moved") ||
		strings.Contains(failedOutput, "state=parked") || strings.Contains(failedOutput, "parkRecord=") {
		t.Fatalf("park recording failure was not discriminating: code=%d\n%s", failedCode, failedOutput)
	}
	if got := runReceiptGit(t, top, "rev-parse", "HEAD"); got != candidate {
		t.Fatalf("recording failure moved candidate: %s", got)
	}
	if got, _ := os.ReadFile(filepath.Join(root, "plans", "goals", "landing-goal.md")); !bytes.Equal(got, goalBefore) {
		t.Fatal("recording failure mutated goal/claim bytes")
	}
	if got, _ := os.ReadFile(filepath.Join(root, "artifacts", "agents", "jobs", "park-chain.json")); !bytes.Equal(got, jobBefore) {
		t.Fatal("recording failure mutated chain accounting bytes")
	}
	if after, _ := os.ReadDir(retained); len(after) != 1 {
		t.Fatalf("successful verified park was not retained separately: %v", after)
	}
}

func TestLandingTestReceiptRefusesMismatchedWorkingTreeBeforeCommand(t *testing.T) {
	root := t.TempDir()
	runReceiptGit(t, root, "init", "-q", "-b", "main")
	runReceiptGit(t, root, "config", "user.name", "receipt fixture")
	runReceiptGit(t, root, "config", "user.email", "receipt@example.invalid")
	writeReceiptFixture(t, root, "payload.txt", "base\n")
	runReceiptGit(t, root, "add", "payload.txt")
	runReceiptGit(t, root, "commit", "-qm", "base")

	writeReceiptFixture(t, root, "payload.txt", "candidate\n")
	runReceiptGit(t, root, "add", "payload.txt")
	candidate := runReceiptGit(t, root, "write-tree")
	writeReceiptFixture(t, root, "payload.txt", "different working tree\n")

	if code := runLandingTestReceipt([]string{
		"--root", root,
		"--tree", candidate,
		"--command", "touch command-must-not-run",
	}); code == 0 {
		t.Fatal("receipt creation accepted a working tree that differed from the supplied tree")
	}
	if got := runReceiptGit(t, root, "write-tree"); got != candidate {
		t.Fatalf("fixture index tree moved: got %s, want %s", got, candidate)
	}
	if _, err := os.Stat(filepath.Join(root, "command-must-not-run")); !os.IsNotExist(err) {
		t.Fatalf("test command ran despite the mismatched working tree: %v", err)
	}
	if _, err := os.Stat(landing.TestReceiptPath(root, candidate)); !os.IsNotExist(err) {
		t.Fatalf("receipt exists after mismatched-working-tree refusal: %v", err)
	}
}

func TestLandingTestReceiptRefusesPostCommandTreeDrift(t *testing.T) {
	root := t.TempDir()
	runReceiptGit(t, root, "init", "-q", "-b", "main")
	runReceiptGit(t, root, "config", "user.name", "receipt fixture")
	runReceiptGit(t, root, "config", "user.email", "receipt@example.invalid")
	writeReceiptFixture(t, root, "payload.txt", "base\n")
	runReceiptGit(t, root, "add", "payload.txt")
	runReceiptGit(t, root, "commit", "-qm", "base")
	writeReceiptFixture(t, root, "payload.txt", "candidate\n")
	runReceiptGit(t, root, "add", "payload.txt")
	candidate := runReceiptGit(t, root, "write-tree")

	if code := runLandingTestReceipt([]string{
		"--root", root,
		"--tree", candidate,
		"--command", "printf 'drift\\n' > payload.txt",
	}); code == 0 {
		t.Fatal("receipt creation accepted a working tree changed by the command")
	}
	if _, err := os.Stat(landing.TestReceiptPath(root, candidate)); !os.IsNotExist(err) {
		t.Fatalf("receipt exists after post-command tree drift: %v", err)
	}
}

func TestLandingTestReceiptCanonicalCLIUsesSectionSelector(t *testing.T) {
	for _, frozen := range []bool{false, true} {
		name := "committed-candidate-archive"
		if frozen {
			name = "candidate-with-older-private-head"
		}
		t.Run(name, func(t *testing.T) { runCanonicalReceiptFixture(t, frozen) })
	}
}

func runCanonicalReceiptFixture(t *testing.T, frozen bool) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	runReceiptGit(t, root, "init", "-q", "-b", "main")
	runReceiptGit(t, root, "config", "user.name", "receipt fixture")
	runReceiptGit(t, root, "config", "user.email", "receipt@example.invalid")
	runReceiptGit(t, root, "config", "goal.sync-remote", "local")
	runReceiptGit(t, root, "config", "goal.sync-branch", goal.LocalLedgerBranch)
	writeReceiptFixture(t, root, "metasystem.conf", "metasystem.runtimes=fake\ndispatch.cap-min=1\ndispatch.cap-max=120\n")
	writeReceiptFixture(t, root, ".gitignore", "artifacts/\nbin/\n")
	writeReceiptFixture(t, root, "go.mod", "module github.com/widoriezebos/agentic-tools/metasystem\n\ngo 1.27.0\n")
	for _, name := range []string{"coverage-ratchet.json", "coverage-ratchet-linux.json"} {
		writeReceiptFixture(t, root, filepath.Join("scripts", "agents", name), `{"floors":{"internal/proofrun":1},"exempt":{}}`)
	}
	gateSource, err := os.ReadFile(filepath.Join("..", "..", "scripts", "agents", "go-gate.sh"))
	if err != nil {
		t.Fatal(err)
	}
	writeReceiptFixture(t, root, "scripts/agents/go-gate.sh", string(gateSource))
	witnessSource, err := os.ReadFile(filepath.Join("..", "..", "scripts", "agents", "witness-gate.sh"))
	if err != nil {
		t.Fatal(err)
	}
	writeReceiptFixture(t, root, "scripts/agents/witness-gate.sh", string(witnessSource))
	writeReceiptFixture(t, root, "scripts/agents/go-build.sh", `#!/usr/bin/env bash
set -euo pipefail
target=bin/metasystem
while (($#)); do
  case "$1" in
    --out) target=$2; shift 2 ;;
    *) shift ;;
  esac
done
mkdir -p "$(dirname "$target")"
cp "$RECEIPT_CANARY_ENGINE" "$target"
chmod 755 "$target"
`)
	writeReceiptFixture(t, root, "helpers/gofmt", "#!/usr/bin/env bash\nexit 0\n")
	writeReceiptFixture(t, root, "helpers/go", `#!/usr/bin/env bash
set -euo pipefail
case "${1:-}" in
  version|env) exec "$RECEIPT_CANARY_REAL_GO" "$@" ;;
  run)
    if [[ "${2:-}" == ./cmd/metasystem ]]; then
      shift 2
      exec "$RECEIPT_CANARY_ENGINE" "$@"
    fi
    exit 0
    ;;
  vet|build) exit 0 ;;
  list)
    if [[ " $* " == *" {{.Dir}} "* ]]; then
      printf '%s/internal/proofrun\n' "$PWD"
    else
      printf 'github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun\n'
    fi
    ;;
  test)
    if [[ " $* " == *" ./internal/... "* ]]; then
	  if [[ " $* " == *" -run NoSuchTestEver "* ]]; then exit 0; fi
      measurements=0
      if [[ -f "$RECEIPT_CANARY_MEASUREMENT_COUNT" ]]; then measurements=$(cat "$RECEIPT_CANARY_MEASUREMENT_COUNT"); fi
      printf '%d\n' "$((measurements + 1))" >"$RECEIPT_CANARY_MEASUREMENT_COUNT"
      printf 'ok  github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun 0.1s coverage: 85.0%% of statements\n'
    fi
    exit 0
    ;;
  *) printf 'unexpected tiny go invocation: %q\n' "$*" >&2; exit 97 ;;
esac
`)
	for _, relative := range []string{"scripts/agents/go-gate.sh", "scripts/agents/witness-gate.sh", "scripts/agents/go-build.sh", "helpers/go", "helpers/gofmt"} {
		if err := os.Chmod(filepath.Join(root, relative), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	selector := "#!/usr/bin/env bash\ncase \"${1:-}\" in\n  list) printf 'tiny\\ttiny receipt canary\\n' ;;\n  twice) exit 0 ;;\n  *) exit 2 ;;\nesac\n"
	writeReceiptFixture(t, root, "scripts/agents/validate-section-selector.sh", selector)
	if err := os.Chmod(filepath.Join(root, "scripts", "agents", "validate-section-selector.sh"), 0o755); err != nil {
		t.Fatal(err)
	}
	validator := `#!/usr/bin/env bash
set -euo pipefail
launches=0
if [[ -f "$RECEIPT_CANARY_LAUNCH_COUNT" ]]; then launches=$(cat "$RECEIPT_CANARY_LAUNCH_COUNT"); fi
printf '%d\n' "$((launches + 1))" >"$RECEIPT_CANARY_LAUNCH_COUNT"
progress="$METASYSTEM_PROOF_CONTROL_ROOT/artifacts/agents/proof-runs/delivery/$METASYSTEM_PROOF_ATTEMPT.progress.jsonl"
printf '{"suite":"landing-receipt","section":"tiny","event":"start","at":"%s","depth":0}\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)" >>"$progress"
evidence="$METASYSTEM_PROOF_CONTROL_ROOT/artifacts/agents/proof-runs/delivery/$METASYSTEM_PROOF_ATTEMPT.coverage"
mkdir -p "$evidence"
root=$PWD
delivery_contract=0
export PATH="$root/helpers:$PATH"
WITNESS_GATE_FALLBACK=plain source scripts/agents/witness-gate.sh
printf '%s\n' "$witness_snap" >"$RECEIPT_CANARY_SNAPSHOT_PATH"
[[ "$(cat "$witness_snap/internal/proofrun/candidate.txt")" == 'candidate source' ]]
printf '{"suite":"landing-receipt","section":"tiny","event":"end","at":"%s","depth":0}\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)" >>"$progress"
`
	if frozen {
		// Only this detached measurement worktree's HEAD is moved. Its source
		// stays equal to the candidate; the production witness must freeze it
		// instead of archiving the older commit.
		validator = strings.Replace(validator, "root=$PWD\n", "root=$PWD\ngit symbolic-ref -q HEAD && exit 98\ngit update-ref --no-deref HEAD HEAD^^\n", 1)
	}
	writeReceiptFixture(t, root, landing.CanonicalValidatorCommand, validator)
	if err := os.Chmod(filepath.Join(root, landing.CanonicalValidatorCommand), 0o755); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	rootRecord := &goal.RootRecord{Identity: "01ARZ3NDEKTSV4RRFFQ69G5FAV", FormatVersion: "1", SyncMode: goal.SyncLocal, Revision: 1}
	budget := &goal.Budget{ElapsedLimit: "4h", AttemptLimit: 1, ReservedJobMinutesLimit: 1, ActiveJobLimit: 1, ReviewRoundLimit: 2}
	intent := "Drive the canonical receipt canary."
	goalFile := &goal.GoalFile{Id: "receipt-goal", State: goal.StateClaimed, Tier: 2, Risk: &goal.RiskRecord{Severity: 2, Novelty: 2, Exposure: 2, Accumulation: 1, Basis: "The fixture drives a bounded proof transaction."}, Intent: intent,
		Origin: goal.OriginMain, NextStep: "Run the receipt.", OpenedAt: now.Add(-2 * time.Minute).Format(time.RFC3339), Revision: 3,
		Budget:  budget,
		Claimed: &goal.ClaimRecord{Machine: "fixture-machine", Lineage: "receipt-canary", At: now.Add(-time.Minute).Format(time.RFC3339), Revision: 2, AccountingRevision: 2},
		Approved: &goal.ApprovalRecord{By: "human:fixture", At: now.Add(-30 * time.Second).Format(time.RFC3339), Revision: 3,
			Opid: goal.Opid("01ARZ3NDEKTSV4RRFFQ69G5FAZ", "fixture-machine", "receipt-canary"), Authority: goal.ApprovalAuthorityProven,
			Digest: goal.ApprovalDigest(intent, 2, *budget, &goal.RiskRecord{Severity: 2, Novelty: 2, Exposure: 2, Accumulation: 1, Basis: "The fixture drives a bounded proof transaction."})},
		StopCapability: &goal.StopCapability{Generation: 3, Revision: 2, Machine: "fixture-machine", ClaimEpoch: 1},
		History: []goal.HistoryLine{{At: now.Add(-2 * time.Minute).Format(time.RFC3339), Opid: goal.Opid("01ARZ3NDEKTSV4RRFFQ69G5FAA", "fixture-machine", "receipt-canary"), Verb: "open", Actor: "fixture-machine+receipt-canary", Keep: -1},
			{At: now.Add(-time.Minute).Format(time.RFC3339), Opid: goal.Opid("01ARZ3NDEKTSV4RRFFQ69G5FAB", "fixture-machine", "receipt-canary"), Verb: "claim", Actor: "fixture-machine+receipt-canary", Keep: -1},
			{At: now.Add(-30 * time.Second).Format(time.RFC3339), Opid: goal.Opid("01ARZ3NDEKTSV4RRFFQ69G5FAZ", "fixture-machine", "receipt-canary"), Verb: "approve", Actor: "human:fixture", Keep: -1}}}
	for path, data := range map[string][]byte{filepath.Join(root, "plans", "goals", "backlog.md"): goal.RenderRoot(rootRecord), filepath.Join(root, "plans", "goals", "receipt-goal.md"): goal.RenderFile(goalFile)} {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	writeReceiptFixture(t, root, "internal/proofrun/candidate.txt", "base source\n")
	runReceiptGit(t, root, "add", ".")
	runReceiptGit(t, root, "commit", "-qm", "canonical receipt base")
	writeReceiptFixture(t, root, "internal/proofrun/candidate.txt", "candidate source\n")
	runReceiptGit(t, root, "add", ".")
	runReceiptGit(t, root, "commit", "-qm", "canonical receipt fixture")
	runReceiptGit(t, root, "update-ref", goal.LocalLedgerBranch, "HEAD")
	runReceiptGit(t, root, "update-ref", goal.AcceptedRef, "HEAD")
	tree := runReceiptGit(t, root, "write-tree")
	engine := filepath.Join(t.TempDir(), "metasystem")
	build := exec.Command("go", "build", "-o", engine, ".")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build receipt canary engine: %v\n%s", err, output)
	}
	launchCount := filepath.Join(root, "artifacts", "receipt-validator-launches")
	measurementCount := filepath.Join(root, "artifacts", "receipt-coverage-measurements")
	snapshotPath := filepath.Join(root, "artifacts", "receipt-witness-snapshot-path")
	realGo, err := exec.LookPath("go")
	if err != nil {
		t.Fatal(err)
	}
	receiptsParent := filepath.Dir(landing.TestReceiptPath(root, tree))
	if err := os.MkdirAll(filepath.Dir(receiptsParent), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(receiptsParent, []byte("block the first projection\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	identityTable := filepath.Join(t.TempDir(), "process-identities.json")
	if err := os.WriteFile(identityTable, []byte(fmt.Sprintf(`{"%d":{"terminal":true}}`, os.Getpid())), 0o600); err != nil {
		t.Fatal(err)
	}
	resultPath := filepath.Join(t.TempDir(), "result.json")
	command := exec.Command(engine, "landing", "test-receipt", "--root", root, "--tree", tree,
		"--command", landing.CanonicalValidatorCommand, "--goal", "receipt-goal", "--cap-min", "1", "--result", resultPath)
	command.Env = append(receiptCanaryEnvironment(), "RECEIPT_CANARY_ENGINE="+engine, "RECEIPT_CANARY_LAUNCH_COUNT="+launchCount,
		"RECEIPT_CANARY_MEASUREMENT_COUNT="+measurementCount, "RECEIPT_CANARY_SNAPSHOT_PATH="+snapshotPath,
		"RECEIPT_CANARY_REAL_GO="+realGo, "PATH="+filepath.Join(root, "helpers")+string(os.PathListSeparator)+os.Getenv("PATH"),
		"METASYSTEM_FAKE_PROCESS_IDENTITY_FILE="+identityTable)
	output, err := command.CombinedOutput()
	if exit, ok := err.(*exec.ExitError); !ok || exit.ExitCode() != 1 {
		t.Fatalf("canonical receipt CLI did not retain a successful proof before projection failure: %v\n%s", err, output)
	}
	var result proofrun.LaunchResult
	data, err := os.ReadFile(resultPath)
	if err != nil || json.Unmarshal(data, &result) != nil || result.Disposition != proofrun.DispositionFailed || result.ExitStatus != 1 {
		t.Fatalf("canonical receipt result = %+v readErr=%v bytes=%s", result, err, data)
	}
	attempts, err := proofrun.ReadAttempts(root)
	if err != nil || len(attempts) != 1 || attempts[0].Terminal == nil || attempts[0].Terminal.Result != proofrun.TerminalSuccess ||
		attempts[0].PendingCoverage == nil || attempts[0].PendingCoverage.Evidence == nil || len(attempts[0].ProcessKeys) != 1 || len(attempts[0].DeliveryReceipt) == 0 {
		diagnostics, _ := json.MarshalIndent(attempts, "", "  ")
		t.Fatalf("canonical receipt terminal diagnostics: reservations=%d readErr=%v fixture=%s\ncommand output:\n%s\nstructured result: %+v\nattempts:\n%s",
			len(attempts), err, root, output, result, diagnostics)
	}
	measurements, measureErr := os.ReadFile(measurementCount)
	snapshotBytes, snapshotErr := os.ReadFile(snapshotPath)
	if measureErr != nil || strings.TrimSpace(string(measurements)) != "1" || snapshotErr != nil ||
		strings.TrimSpace(string(snapshotBytes)) == attempts[0].ExecutionRoot {
		t.Fatalf("snapshot coverage handoff measurements=%q measureErr=%v snapshot=%q snapshotErr=%v admitted=%q",
			measurements, measureErr, snapshotBytes, snapshotErr, attempts[0].ExecutionRoot)
	}
	receiptPath := landing.TestReceiptPath(root, tree)
	original := append([]byte(nil), attempts[0].DeliveryReceipt...)
	if err := os.Remove(receiptsParent); err != nil {
		t.Fatal(err)
	}
	repeatResult := filepath.Join(t.TempDir(), "repeat-result.json")
	repeat := exec.Command(engine, "landing", "test-receipt", "--root", root, "--tree", tree,
		"--command", landing.CanonicalValidatorCommand, "--goal", "receipt-goal", "--cap-min", "1", "--result", repeatResult)
	repeat.Env = append(receiptCanaryEnvironment(), "RECEIPT_CANARY_ENGINE="+engine, "RECEIPT_CANARY_LAUNCH_COUNT="+launchCount,
		"RECEIPT_CANARY_MEASUREMENT_COUNT="+measurementCount, "RECEIPT_CANARY_SNAPSHOT_PATH="+snapshotPath,
		"RECEIPT_CANARY_REAL_GO="+realGo, "PATH="+filepath.Join(root, "helpers")+string(os.PathListSeparator)+os.Getenv("PATH"),
		"METASYSTEM_FAKE_PROCESS_IDENTITY_FILE="+identityTable)
	repeatOutput, repeatErr := repeat.CombinedOutput()
	if exit, ok := repeatErr.(*exec.ExitError); !ok || exit.ExitCode() != proofrun.ExitReusableSuccess {
		t.Fatalf("spent-budget receipt recovery exit=%v output:\n%s", repeatErr, repeatOutput)
	}
	var repeated proofrun.LaunchResult
	repeatedData, readErr := os.ReadFile(repeatResult)
	if readErr != nil || json.Unmarshal(repeatedData, &repeated) != nil || repeated.Disposition != proofrun.DispositionReusableSuccess || repeated.AttemptID != attempts[0].AttemptID {
		t.Fatalf("receipt recovery result=%+v readErr=%v bytes=%s", repeated, readErr, repeatedData)
	}
	recovered, err := os.ReadFile(receiptPath)
	if err != nil || !bytes.Equal(bytes.TrimSpace(original), bytes.TrimSpace(recovered)) {
		t.Fatalf("recovered receipt changed committed payload: err=%v", err)
	}
	launches, err := os.ReadFile(launchCount)
	if err != nil || strings.TrimSpace(string(launches)) != "1" {
		t.Fatalf("repeat launched the validator again: launches=%q err=%v", launches, err)
	}
	measurements, err = os.ReadFile(measurementCount)
	if err != nil || strings.TrimSpace(string(measurements)) != "1" {
		t.Fatalf("repeat launched coverage again: measurements=%q err=%v", measurements, err)
	}
	attempts, err = proofrun.ReadAttempts(root)
	if err != nil || len(attempts) != 1 {
		t.Fatalf("spent-budget recovery created a reservation: attempts=%d err=%v", len(attempts), err)
	}
	// Exercise actual public selector admission under adversarial environments.
	// A completed receipt is reusable, but it does not authenticate a caller
	// posing as its still-running child or admit alternate source overlays.
	for _, refusal := range []struct {
		name string
		env  []string
		want string
	}{
		{"overlay", []string{"GOFLAGS=-overlay=foreign.json"}, "canonical validator refuses GOFLAGS"},
		{"modfile", []string{"GOFLAGS=-modfile=foreign.mod"}, "canonical validator refuses GOFLAGS"},
		{"partial-parent", []string{"METASYSTEM_PROOF_CONTROL_ROOT=" + root}, "proof parent locator is incomplete"},
		{"terminal-parent", []string{"METASYSTEM_PROOF_CONTROL_ROOT=" + root, "METASYSTEM_PROOF_ATTEMPT=" + attempts[0].AttemptID}, "proof parent context is not live"},
	} {
		t.Run(refusal.name, func(t *testing.T) {
			probe := exec.Command(engine, "landing", "test-receipt", "--root", root, "--tree", tree,
				"--command", landing.CanonicalValidatorCommand, "--goal", "receipt-goal", "--cap-min", "1")
			probe.Env = append(append([]string(nil), repeat.Env...), refusal.env...)
			out, err := probe.CombinedOutput()
			if err == nil || !strings.Contains(string(out), refusal.want) {
				t.Fatalf("public %s admission did not refuse exactly: %v\n%s", refusal.name, err, out)
			}
		})
	}
	if got, _ := os.ReadFile(launchCount); strings.TrimSpace(string(got)) != "1" {
		t.Fatalf("refused environment/custody launched the validator: %q", got)
	}
	if got, _ := os.ReadFile(measurementCount); strings.TrimSpace(string(got)) != "1" {
		t.Fatalf("refused environment/custody measured coverage: %q", got)
	}
	if got, _ := os.ReadFile(receiptPath); !bytes.Equal(bytes.TrimSpace(original), bytes.TrimSpace(got)) {
		t.Fatal("refused environment/custody changed terminal receipt bytes")
	}

}

func TestLandingSharedTestingReceiptPublicRecoveryKeepsExactOuterAuthority(t *testing.T) {
	for _, prefix := range []string{"", "vendor/metasystem runtime"} {
		name := "root"
		if prefix != "" {
			name = "nested-with-application-outside-installation"
		}
		t.Run(name, func(t *testing.T) { runSharedTestingReceiptRecovery(t, prefix) })
	}
}

func runSharedTestingReceiptRecovery(t *testing.T, prefix string) {
	projectRoot, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(projectRoot, filepath.FromSlash(prefix))
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	runReceiptGit(t, projectRoot, "init", "-q", "-b", "main")
	runReceiptGit(t, root, "config", "user.name", "receipt fixture")
	runReceiptGit(t, root, "config", "user.email", "receipt@example.invalid")
	runReceiptGit(t, root, "config", "goal.sync-remote", "local")
	runReceiptGit(t, root, "config", "goal.sync-branch", goal.LocalLedgerBranch)
	runReceiptGit(t, root, "config", "metasystem.steward.landing-ref", "refs/remotes/origin/main")
	writeReceiptFixture(t, root, ".gitignore", "artifacts/\nbin/\n")
	writeReceiptFixture(t, projectRoot, "payload.txt", "public shared testing input\n")
	writeReceiptFixture(t, root, "metasystem.conf", "testing.contract=testing.json\nmetasystem.runtimes=fake\ndispatch.cap-min=1\ndispatch.cap-max=120\n")
	launchCount := filepath.Join(root, "artifacts", "shared-testing-launches")
	contract := testpolicy.Contract{SchemaVersion: 1,
		ProjectRisk: testpolicy.ProjectRisk{Severity: 1, Exposure: 1, Reversibility: "revert", Detection: "immediate", Recovery: "bounded"},
		Surfaces: []testpolicy.Surface{
			{ID: "application", Paths: []string{"testing.json", "plans/**", "scripts/agents/coverage-ratchet.json", "scripts/agents/coverage-ratchet-linux.json"}, Standard: []string{"policy-protection", "candidate-smoke"}, Critical: []string{"application-proof"}},
			{ID: "proof-and-landing", Paths: []string{"payload.txt"}, DependsOn: []string{"application"}, Standard: []string{"policy-protection", "candidate-smoke"}, Critical: []string{"application-proof"}},
		},
		Groups: []testpolicy.Group{{ID: "policy-protection", Kind: "unit", Adapter: "command", CWD: ".", Inputs: []string{"payload.txt"},
			Outputs: []string{"reports"}, Tools: []testpolicy.Tool{}, Obligations: []string{"application-proof"}, Platforms: []string{"any"}, TargetMS: 1000,
			Env:     map[string]string{"SHARED_TEST_LAUNCH_COUNT": launchCount},
			Argv:    []string{"sh", "-c", "count=0; test ! -f \"$SHARED_TEST_LAUNCH_COUNT\" || count=$(cat \"$SHARED_TEST_LAUNCH_COUNT\"); printf '%s\\n' $((count + 1)) >\"$SHARED_TEST_LAUNCH_COUNT\"; mkdir -p reports; printf '%s\\n' '<testsuite><testcase classname=\"application\" name=\"public-receipt\"/></testsuite>' >reports/result.xml"},
			Reports: []string{"reports"}, Format: "junit-xml", ExpectedTests: []testpolicy.ExpectedTest{{Report: "reports/result.xml", Classname: "application", Name: "public-receipt"}}},
			{ID: "candidate-smoke", Kind: "unit", Adapter: "command", CWD: ".", Inputs: []string{"payload.txt"}, Outputs: []string{"smoke-reports"},
				Tools: []testpolicy.Tool{}, Obligations: []string{"application-proof"}, Platforms: []string{"any"}, TargetMS: 1000,
				Argv:    []string{"sh", "-c", "mkdir -p smoke-reports; printf '%s\\n' '<testsuite><testcase classname=\"application\" name=\"smoke\"/></testsuite>' >smoke-reports/result.xml"},
				Reports: []string{"smoke-reports"}, Format: "junit-xml", ExpectedTests: []testpolicy.ExpectedTest{{Report: "smoke-reports/result.xml", Classname: "application", Name: "smoke"}}}},
		Always:  testpolicy.Always{Canary: []string{"policy-protection", "candidate-smoke"}, Standard: []string{"policy-protection", "candidate-smoke"}},
		Unknown: []string{"policy-protection", "candidate-smoke"}, Cadence: []string{"policy-protection", "candidate-smoke"}}
	if prefix != "" {
		for index := range contract.Surfaces {
			for pathIndex, path := range contract.Surfaces[index].Paths {
				if path != "payload.txt" {
					contract.Surfaces[index].Paths[pathIndex] = prefix + "/" + path
				}
			}
		}
		writeReceiptFixture(t, projectRoot, ".gitignore", "reports/\nsmoke-reports/\n")
	}
	contractBytes, err := json.Marshal(contract)
	if err != nil {
		t.Fatal(err)
	}
	writeReceiptFixture(t, root, "testing.json", string(contractBytes))
	writeReceiptFixture(t, root, "scripts/agents/coverage-ratchet.json", `{"floors":{"fixture/application":80.0}}`)
	writeReceiptFixture(t, root, "scripts/agents/coverage-ratchet-linux.json", `{"floors":{"fixture/application":80.0}}`)
	now := time.Now().UTC()
	rootRecord := &goal.RootRecord{Identity: "01ARZ3NDEKTSV4RRFFQ69G5FBV", FormatVersion: "1", SyncMode: goal.SyncLocal, Revision: 1}
	budget := &goal.Budget{ElapsedLimit: "4h", AttemptLimit: 1, ReservedJobMinutesLimit: 1, ActiveJobLimit: 1, ReviewRoundLimit: 2}
	risk := &goal.RiskRecord{Severity: 1, Novelty: 1, Exposure: 1, Accumulation: 1, Basis: "The fixture executes one tiny public shared testing group."}
	intent := "Drive public schema-two receipt recovery."
	goalFile := &goal.GoalFile{Id: "receipt-goal", State: goal.StateClaimed, Tier: 1, Risk: risk, Intent: intent,
		Origin: goal.OriginMain, NextStep: "Run shared testing.", OpenedAt: now.Add(-2 * time.Minute).Format(time.RFC3339), Revision: 3,
		Budget: budget, Claimed: &goal.ClaimRecord{Machine: "fixture-machine", Lineage: "receipt-canary", At: now.Add(-time.Minute).Format(time.RFC3339), Revision: 2, AccountingRevision: 2},
		Approved: &goal.ApprovalRecord{By: "human:fixture", At: now.Add(-30 * time.Second).Format(time.RFC3339), Revision: 3,
			Opid: goal.Opid("01ARZ3NDEKTSV4RRFFQ69G5FBZ", "fixture-machine", "receipt-canary"), Authority: goal.ApprovalAuthorityProven,
			Digest: goal.ApprovalDigest(intent, 1, *budget, risk)},
		StopCapability: &goal.StopCapability{Generation: 3, Revision: 2, Machine: "fixture-machine", ClaimEpoch: 1},
		History: []goal.HistoryLine{{At: now.Add(-2 * time.Minute).Format(time.RFC3339), Opid: goal.Opid("01ARZ3NDEKTSV4RRFFQ69G5FBA", "fixture-machine", "receipt-canary"), Verb: "open", Actor: "fixture-machine+receipt-canary", Keep: -1},
			{At: now.Add(-time.Minute).Format(time.RFC3339), Opid: goal.Opid("01ARZ3NDEKTSV4RRFFQ69G5FBB", "fixture-machine", "receipt-canary"), Verb: "claim", Actor: "fixture-machine+receipt-canary", Keep: -1},
			{At: now.Add(-30 * time.Second).Format(time.RFC3339), Opid: goal.Opid("01ARZ3NDEKTSV4RRFFQ69G5FBZ", "fixture-machine", "receipt-canary"), Verb: "approve", Actor: "human:fixture", Keep: -1}}}
	for path, data := range map[string][]byte{
		filepath.Join(root, "plans", "goals", "backlog.md"):      goal.RenderRoot(rootRecord),
		filepath.Join(root, "plans", "goals", "receipt-goal.md"): goal.RenderFile(goalFile),
	} {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	runReceiptGit(t, projectRoot, "add", "-A")
	runReceiptGit(t, root, "commit", "-qm", "public shared testing fixture")
	head := runReceiptGit(t, root, "rev-parse", "HEAD")
	runReceiptGit(t, root, "update-ref", "refs/remotes/origin/main", head)
	runReceiptGit(t, root, "update-ref", goal.LocalLedgerBranch, head)
	runReceiptGit(t, root, "update-ref", goal.AcceptedRef, head)
	tree := runReceiptGit(t, root, "write-tree")
	engine := filepath.Join(t.TempDir(), "metasystem")
	build := exec.Command("go", "build", "-buildvcs=false", "-ldflags",
		"-X github.com/widoriezebos/agentic-tools/metasystem/internal/supervise.BuildStamp="+head, "-o", engine, ".")
	if output, buildErr := build.CombinedOutput(); buildErr != nil {
		t.Fatalf("build shared testing receipt engine: %v\n%s", buildErr, output)
	}
	engine, err = canonicalPath(engine)
	if err != nil {
		t.Fatal(err)
	}
	digest, err := fileSHA256(engine)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(steward.RepoIdentityPath(root)), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := steward.MintIdentity(steward.RepoIdentityPath(root), steward.InstallIdentity{RepoIdentity: root, Generation: 1,
		InstallPath: engine, InstallDigest: "sha256:" + digest, MintedAt: now.Format(time.RFC3339), Enrollment: steward.EnrollmentFixture,
		EngineBuild: head[:12], LandedCommit: head}); err != nil {
		t.Fatal(err)
	}
	receiptsParent := filepath.Dir(landing.TestReceiptPath(root, tree))
	if err := os.MkdirAll(filepath.Dir(receiptsParent), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(receiptsParent, []byte("block first public projection\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	identityTable := filepath.Join(t.TempDir(), "process-identities.json")
	if err := os.WriteFile(identityTable, []byte(fmt.Sprintf(`{"%d":{"terminal":true}}`, os.Getpid())), 0o600); err != nil {
		t.Fatal(err)
	}
	runPublic := func() ([]byte, error) {
		command := exec.Command(engine, "landing", "test-receipt", "--root", root, "--tree", tree, "--mode", "auto", "--goal", "receipt-goal", "--cap-min", "1")
		command.Env = append(receiptCanaryEnvironment(), "SHARED_TEST_LAUNCH_COUNT="+launchCount, "METASYSTEM_FAKE_PROCESS_IDENTITY_FILE="+identityTable)
		return command.CombinedOutput()
	}
	firstOutput, firstErr := runPublic()
	if exit, ok := firstErr.(*exec.ExitError); !ok || exit.ExitCode() != 1 {
		t.Fatalf("public schema-two command did not retain success before projection failure: err=%v\n%s", firstErr, firstOutput)
	}
	attempts, err := proofrun.ReadAttempts(root)
	if err != nil || len(attempts) != 1 || attempts[0].Terminal == nil || attempts[0].Terminal.Result != proofrun.TerminalSuccess ||
		attempts[0].TestResult == nil || !attempts[0].TestResult.Delivery.Sufficient || len(attempts[0].DeliveryReceiptBytes) == 0 {
		t.Fatalf("public schema-two terminal authority missing: attempts=%+v err=%v output=%s", attempts, err, firstOutput)
	}
	original := append([]byte(nil), attempts[0].DeliveryReceiptBytes...)
	originalTime := attempts[0].TestResult.EndedAt
	if attempts[0].TestResult.ProjectRoot != projectRoot ||
		strings.TrimSuffix(attempts[0].TestResult.InstallationPrefix, "/") != prefix ||
		attempts[0].TestResult.CandidateTree != tree {
		t.Fatalf("receipt lost whole-project identity across installation prefix: %+v", attempts[0].TestResult)
	}
	if err := os.Remove(receiptsParent); err != nil {
		t.Fatal(err)
	}
	repeatOutput, repeatErr := runPublic()
	if repeatErr != nil {
		t.Fatalf("public schema-two recovery failed: %v\n%s", repeatErr, repeatOutput)
	}
	projected, err := os.ReadFile(landing.TestReceiptPath(root, tree))
	if err != nil || !bytes.Equal(bytes.TrimSpace(original), bytes.TrimSpace(projected)) {
		t.Fatalf("public schema-two recovery changed committed bytes: err=%v\noriginal=%s\nprojected=%s", err, original, projected)
	}
	var receipt landing.TestReceipt
	if err := json.Unmarshal(projected, &receipt); err != nil || receipt.Testing == nil || receipt.Testing.EndedAt != originalTime || receipt.Testing.AttemptID != attempts[0].AttemptID {
		t.Fatalf("public schema-two recovery changed owner/time: receipt=%+v err=%v", receipt, err)
	}
	launches, err := os.ReadFile(launchCount)
	if err != nil || strings.TrimSpace(string(launches)) != "1" {
		t.Fatalf("public schema-two recovery relaunched test body: launches=%q err=%v", launches, err)
	}
	attempts, err = proofrun.ReadAttempts(root)
	if err != nil || len(attempts) != 1 {
		t.Fatalf("public schema-two recovery created another reservation: attempts=%d err=%v", len(attempts), err)
	}

	originalTree := tree
	writeReceiptFixture(t, root, "plans/coordination-note.md", "irrelevant staged coordination change\n")
	runReceiptGit(t, root, "add", "plans/coordination-note.md")
	tree = runReceiptGit(t, projectRoot, "write-tree")
	if tree == originalTree {
		t.Fatal("staged coordination change did not change the whole-project candidate")
	}
	recomposedOutput, recomposedErr := runPublic()
	if recomposedErr != nil {
		t.Fatalf("public schema-two current-candidate recomposition failed: %v\n%s", recomposedErr, recomposedOutput)
	}
	recomposedBytes, err := os.ReadFile(landing.TestReceiptPath(root, tree))
	if err != nil {
		t.Fatal(err)
	}
	var recomposed landing.TestReceipt
	if err := json.Unmarshal(recomposedBytes, &recomposed); err != nil || recomposed.SchemaVersion != 2 || recomposed.Tree != tree ||
		recomposed.ProvedTree != tree || recomposed.Testing == nil || recomposed.Testing.CandidateTree != tree || recomposed.Testing.AttemptID != "" ||
		len(recomposed.AttemptIDs) != 1 || recomposed.AttemptIDs[0] != attempts[0].AttemptID || len(recomposed.Testing.Groups) != 2 ||
		recomposed.Testing.LaunchCounts.Test != 0 || recomposed.Testing.LaunchCounts.ReusedTest != 2 || recomposed.Testing.Cost.ReusedLaunches != 2 {
		t.Fatalf("current-candidate receipt did not compose the original component owners: receipt=%+v err=%v", recomposed, err)
	}
	for _, group := range recomposed.Testing.Groups {
		if group.Status != "reused" || group.ReuseAttempt != attempts[0].AttemptID {
			t.Fatalf("current-candidate receipt changed retained group authority: %+v", group)
		}
		for _, originalGroup := range attempts[0].TestResult.Groups {
			if originalGroup.ID == group.ID && (group.StartedAt != originalGroup.StartedAt || group.EndedAt != originalGroup.EndedAt || group.DurationMS != originalGroup.DurationMS) {
				t.Fatalf("current-candidate receipt changed original measurement time: original=%+v reused=%+v", originalGroup, group)
			}
		}
	}
	verify := exec.Command(engine, "test", "verify", "--root", root, "--tree", tree, "--goal", "receipt-goal")
	verify.Env = append(receiptCanaryEnvironment(), "SHARED_TEST_LAUNCH_COUNT="+launchCount, "METASYSTEM_FAKE_PROCESS_IDENTITY_FILE="+identityTable)
	verifyOutput, verifyErr := verify.CombinedOutput()
	if verifyErr != nil || !strings.Contains(string(verifyOutput), "TEST-RESULT sufficient=true tree="+tree) {
		t.Fatalf("normal test verification did not consume recomposed evidence: %v\n%s", verifyErr, verifyOutput)
	}
	retained, err := proofrun.ReadAttempts(root)
	if err != nil || len(retained) != 1 || retained[0].TestResult == nil || retained[0].TestResult.CandidateTree != originalTree ||
		retained[0].TestResult.EndedAt != originalTime || !bytes.Equal(retained[0].DeliveryReceiptBytes, original) {
		t.Fatalf("current-candidate recomposition changed original committed evidence: attempts=%+v err=%v", retained, err)
	}
	if launches, err := os.ReadFile(launchCount); err != nil || strings.TrimSpace(string(launches)) != "1" {
		t.Fatalf("current-candidate recomposition or verification relaunched a group: launches=%q err=%v", launches, err)
	}
}

func receiptCanaryEnvironment() []string {
	return receiptCanaryEnvironmentFrom(os.Environ())
}

func receiptCanaryEnvironmentFrom(source []string) []string {
	allowed := map[string]bool{
		"GOCACHE": true, "GOMODCACHE": true, "GOPATH": true, "GOROOT": true,
		"HOME": true, "LANG": true, "LC_ALL": true, "PATH": true,
		"SYSTEMROOT": true, "TEMP": true, "TMP": true, "TMPDIR": true, "TZ": true,
	}
	environment := make([]string, 0, len(allowed))
	for _, entry := range source {
		key, _, found := strings.Cut(entry, "=")
		if found && allowed[key] {
			environment = append(environment, entry)
		}
	}
	return environment
}

func TestReceiptCanaryEnvironmentIgnoresAmbientProofControls(t *testing.T) {
	base := []string{"HOME=/fixture", "PATH=/tools", "TMPDIR=/tmp", "GOCACHE=/cache"}
	variations := []string{
		"METASYSTEM_PROOF_ATTEMPT=foreign", "METASYSTEM_HOOK_DELEGATE_JOB=foreign",
		"MS_SUITE_PROGRESS_ACTIVE=1", "METASYSTEM_ENUMERATION_DRIVER=1",
		"METASYSTEM_COVERAGE_RATCHET_SEED=1", "METASYSTEM_GATE_FORCE=1", "METASYSTEM_GATE_WITNESS=/foreign",
		"METASYSTEM_SUITE_PROGRESS_SILENCE_MIN=99", "GOFLAGS=-overlay=foreign.json",
	}
	want := strings.Join(receiptCanaryEnvironmentFrom(base), "\x00")
	for _, variation := range variations {
		got := strings.Join(receiptCanaryEnvironmentFrom(append(append([]string(nil), base...), variation)), "\x00")
		if got != want {
			t.Fatalf("ambient variation %q changed the explicit canary environment: got %q want %q", variation, got, want)
		}
	}
}

func runReceiptGit(t *testing.T, root string, args ...string) string {
	t.Helper()
	command := exec.Command("git", append([]string{"-C", root}, args...)...)
	command.Env = gittree.ScrubbedEnviron()
	out, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func writeReceiptFixture(t *testing.T, root, relative, content string) {
	t.Helper()
	path := filepath.Join(root, relative)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
