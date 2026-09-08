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
	bed, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(bed, "local")
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

	writeReceiptFixture(t, root, ".gitignore", "artifacts/\n")
	writeReceiptFixture(t, root, "metasystem.conf", "metasystem.version=1\nmetasystem.runtimes=fake\nrole.code-critic.runtime=fake\n")
	writeReceiptFixture(t, root, "internal/app/source.txt", "base-one\nkeep-two\nkeep-three\nkeep-four\nbase-five\n")
	rootRecord := &goal.RootRecord{
		Identity: "01J5X000000000000000000000", FormatVersion: "1", SyncMode: goal.SyncRemote,
		MigrationEpoch: "2026-09-08T00:00:00Z", ManifestDigest: strings.Repeat("ab", 32),
		MigrationMode: "manifest", Revision: 1,
		History: []goal.HistoryLine{{At: "2026-09-08T00:00:00Z", Opid: "01J5X0000000000000000000A0-race-node-1a2b3c4d", Verb: "migrate", Actor: "race-node+race-lineage", Keep: -1}},
	}
	goalFile := &goal.GoalFile{
		Id: "landing-goal", State: goal.StateClaimed, Intent: "Prove a raced landing parks without changing its claim.",
		Origin: goal.OriginMain, OpenedAt: "2026-09-08T00:01:00Z", Revision: 1,
		Claimed: &goal.ClaimRecord{Machine: "race-node", Lineage: "race-lineage", At: "2026-09-08T00:01:00Z"},
		History: []goal.HistoryLine{{At: "2026-09-08T00:01:00Z", Opid: "01J5X0000000000000000000B0-race-node-1a2b3c4d", Verb: "open", Actor: "race-node+race-lineage", Targets: []string{"landing-goal"}, Keep: -1}},
	}
	if err := os.WriteFile(filepath.Join(root, "plans", "goals", "backlog.md"), goal.RenderRoot(rootRecord), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "plans", "goals", "landing-goal.md"), goal.RenderFile(goalFile), 0o644); err != nil {
		t.Fatal(err)
	}
	runReceiptGit(t, root, "init", "-q", "-b", "main")
	runReceiptGit(t, root, "config", "user.name", "race fixture")
	runReceiptGit(t, root, "config", "user.email", "race@example.invalid")
	runReceiptGit(t, root, "config", "metasystem.goal.machine", "race-node")
	runReceiptGit(t, root, "add", ".")
	runReceiptGit(t, root, "commit", "-qm", "base")
	baseCommit := runReceiptGit(t, root, "rev-parse", "HEAD")
	runReceiptGit(t, root, "worktree", "add", "-q", "-b", "chain", chainRoot, "HEAD")

	writeReceiptFixture(t, chainRoot, "internal/app/source.txt", "CHAIN\nkeep-two\nkeep-three\nkeep-four\nbase-five\n")
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
		"goalId": "landing-goal", "effectiveModel": "implementer-model", "destructiveReach": "DESIGN-BEARING",
		"chainClosed": true, "gateWidth": "area", "independentCritiqueJobRef": "park-critic",
		"reviewRoundLimit": 3, "criticRoundsConsumed": 3,
	})
	writeFixtureJSON("artifacts/agents/park-chain/rounds/1/return.json", map[string]any{
		"jobId": "park-chain", "round": 1, "diffBoundary": []string{"internal/app/source.txt"},
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
		"reviews": "park-chain", "status": "completed", "effectiveModel": "critic-model",
		"chainClosed": true, "findingRegister": []any{}, "findingRegisterRound": 1,
		"reviewRoundLimit": 3, "criticRoundsConsumed": 3,
	})
	writeFixtureJSON("artifacts/agents/park-critic/rounds/1/return.json", map[string]any{
		"jobId": "park-critic", "round": 1, "reviewedTree": review["reviewedTree"],
		"findings": []any{}, "verdictMaterialCount": 0,
	})

	writeReceiptFixture(t, root, "internal/app/source.txt", "base-one\nkeep-two\nkeep-three\nkeep-four\nMAIN\n")
	runReceiptGit(t, root, "add", "internal/app/source.txt")
	runReceiptGit(t, root, "commit", "-qm", "target")
	target := runReceiptGit(t, root, "rev-parse", "HEAD")
	runReceiptGit(t, root, "update-ref", goal.AcceptedRef, target)
	runReceiptGit(t, bed, "init", "-q", "--bare", "-b", "main", remote)
	runReceiptGit(t, root, "remote", "add", "origin", remote)
	runReceiptGit(t, root, "push", "-q", "-u", "origin", "main")
	runReceiptGit(t, bed, "clone", "-q", remote, peer)
	runReceiptGit(t, peer, "config", "user.name", "race peer")
	runReceiptGit(t, peer, "config", "user.email", "race-peer@example.invalid")

	testCommand := "grep -q '^CHAIN$' internal/app/source.txt && grep -q '^MAIN$' internal/app/source.txt"
	if code := runValidateConformance([]string{"--root", root, "--stage", "recertify", "--job", "park-chain", "--test-command", testCommand}); code != 0 {
		t.Fatalf("recertification exited %d", code)
	}
	records, err := filepath.Glob(filepath.Join(root, "artifacts", "agents", "landing", "recertifications", "park-chain", "*", "record.json"))
	if err != nil || len(records) != 1 {
		t.Fatalf("recertification records = %v, error %v", records, err)
	}
	proof, err := filepath.Rel(root, records[0])
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
	runReceiptGit(t, root, "add", "internal/app/source.txt")
	candidateTree := runReceiptGit(t, root, "write-tree")
	receipt, err := landing.CreateTestReceipt(root, candidateTree, testCommand, os.Stdout, os.Stderr)
	if err != nil {
		t.Fatal(err)
	}
	preRace := landing.Observe(landing.ObserveParams{
		RepoRoot: root, CandidateTree: candidateTree, Chain: "park-chain", Goal: "landing-goal",
		Actor: "race-node+race-lineage", Recertification: proof, TestReceipt: landing.TestReceiptPath(root, candidateTree),
	})
	if preRace.Bar != landing.BarChain || preRace.Verdict != "pass" || preRace.Code != "closed-chain" || receipt.ExitStatus != 0 {
		t.Fatalf("pre-race evaluator was not green: observation=%+v receipt=%+v", preRace, receipt)
	}
	runReceiptGit(t, root, "reset", "--mixed", "HEAD")

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
printf '%s\n' "$*" >>"$CLBM_GIT_LOG"
if [[ ${1:-} == push ]] && [[ " $* " == *" origin "* ]]; then
  printf 'push\n' >>"$CLBM_PUSH_COUNT"
  if [[ $(wc -l <"$CLBM_PUSH_COUNT" | tr -d ' ') == 1 ]]; then
    printf 'origin moved\n' >"$CLBM_PEER/peer.txt"
    "$CLBM_REAL_GIT" -C "$CLBM_PEER" add peer.txt
    "$CLBM_REAL_GIT" -C "$CLBM_PEER" commit -qm 'controlled origin move'
    "$CLBM_REAL_GIT" -C "$CLBM_PEER" push -q origin main
  fi
fi
exec "$CLBM_REAL_GIT" "$@"
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
		if _, err := lease.Announce(root, "landing-race", pid, started, "landing-race", "fake", "race-lineage"); err != nil {
			_ = command.Process.Kill()
			t.Fatal(err)
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
	runReceiptGit(t, root, "reset", "--mixed", "HEAD")
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
	candidate := runReceiptGit(t, root, "rev-parse", "HEAD")
	if candidate == target || runReceiptGit(t, root, "rev-parse", "HEAD^1") != target ||
		runReceiptGit(t, root, "rev-parse", "HEAD^{tree}") != candidateTree {
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
	if got := runReceiptGit(t, root, "rev-parse", "HEAD"); got != candidate {
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
