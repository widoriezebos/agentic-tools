package progress

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestOutputWatermarkNeverResetsAfterTruncation(t *testing.T) {
	dir := t.TempDir()
	stream := filepath.Join(dir, "events.jsonl")
	writeSizedFile(t, stream, 100)
	record := outputOnlyRecord(stream)

	first, err := Evaluate(record, 0)
	if err != nil {
		t.Fatal(err)
	}
	if !first.Output.Grew || first.Output.HighWater != 100 {
		t.Fatalf("first observation = %+v, want growth to high-water 100", first.Output)
	}

	writeSizedFile(t, stream, 0)
	truncated, err := Evaluate(record, first.Output.HighWater)
	if err != nil {
		t.Fatal(err)
	}
	if truncated.Output.Grew || truncated.Output.HighWater != 100 || !hasAnomaly(truncated.Output.Anomalies, "output-truncated") {
		t.Fatalf("truncation observation = %+v, want no growth and retained high-water 100", truncated.Output)
	}

	writeSizedFile(t, stream, 10)
	belowHighWater, err := Evaluate(record, truncated.Output.HighWater)
	if err != nil {
		t.Fatal(err)
	}
	if belowHighWater.Output.Grew || belowHighWater.Output.HighWater != 100 {
		t.Fatalf("post-truncation observation = %+v, want no growth until size exceeds 100", belowHighWater.Output)
	}

	writeSizedFile(t, stream, 101)
	aboveHighWater, err := Evaluate(record, belowHighWater.Output.HighWater)
	if err != nil {
		t.Fatal(err)
	}
	if !aboveHighWater.Output.Grew || aboveHighWater.Output.HighWater != 101 {
		t.Fatalf("recovery observation = %+v, want growth at size 101", aboveHighWater.Output)
	}
}

func TestSupervisorAppendNeverCountsAsOutputProgress(t *testing.T) {
	dir := t.TempDir()
	stream := filepath.Join(dir, "events.jsonl")
	sharedLog := filepath.Join(dir, "job.log")
	writeSizedFile(t, stream, 40)
	writeSizedFile(t, sharedLog, 20)
	record := outputOnlyRecord(stream)

	baseline, err := Evaluate(record, 0)
	if err != nil {
		t.Fatal(err)
	}
	writeSizedFile(t, sharedLog, 200)
	observed, err := Evaluate(record, baseline.Output.HighWater)
	if err != nil {
		t.Fatal(err)
	}
	if observed.Output.Grew || observed.Output.HighWater != 40 {
		t.Fatalf("shared supervisor log append counted as progress: %+v", observed.Output)
	}
}

func TestMixedRootsDemoteIndividually(t *testing.T) {
	base := t.TempDir()
	workspace := filepath.Join(base, "worktree")
	inside := filepath.Join(workspace, "product")
	outside := filepath.Join(base, "shared-product")
	if err := os.MkdirAll(inside, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(outside, 0o755); err != nil {
		t.Fatal(err)
	}
	freshAt := time.Unix(1_900_000_000, 0)
	insideFile := filepath.Join(inside, "fresh.txt")
	outsideFile := filepath.Join(outside, "foreign.txt")
	if err := os.WriteFile(insideFile, []byte("fresh"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(outsideFile, []byte("foreign"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(insideFile, freshAt, freshAt); err != nil {
		t.Fatal(err)
	}

	scopes, err := CaptureProductRootScopes(LaunchModeWorktree, workspace, []string{inside, outside})
	if err != nil {
		t.Fatal(err)
	}
	record := recordWithScopes(filepath.Join(base, "events.jsonl"), workspace, LaunchModeWorktree, scopes)
	evidence, err := Evaluate(record, 0)
	if err != nil {
		t.Fatal(err)
	}
	if evidence.Products.EventAt == nil || !evidence.Products.EventAt.Equal(freshAt) {
		t.Fatalf("contained root did not retain liveness standing: %+v", evidence.Products)
	}
	insideEvidence := rootEvidence(t, evidence.Products.Roots, inside)
	outsideEvidence := rootEvidence(t, evidence.Products.Roots, outside)
	if insideEvidence.ScanStanding != StandingLiveness || insideEvidence.Status != RootStatusScanned {
		t.Fatalf("inside root = %+v, want scanned liveness", insideEvidence)
	}
	if outsideEvidence.ScanStanding != StandingAttributionOnly || outsideEvidence.Reason != ReasonOutsideWorktreeAtLaunch || outsideEvidence.Status != RootStatusNotScanned {
		t.Fatalf("outside root = %+v, want only that root demoted", outsideEvidence)
	}
}

func TestSymlinkDriftIntoExclusionDemotesAtScan(t *testing.T) {
	base := t.TempDir()
	workspace := filepath.Join(base, "worktree")
	declared := filepath.Join(workspace, "product", "future")
	excluded := filepath.Join(workspace, "artifacts", "agents", "private")
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		t.Fatal(err)
	}
	scopes, err := CaptureProductRootScopes(LaunchModeWorktree, workspace, []string{declared})
	if err != nil {
		t.Fatal(err)
	}
	if scopes[0].Standing != StandingLiveness {
		t.Fatalf("missing-tail root should be contained at launch: %+v", scopes[0])
	}
	if err := os.MkdirAll(excluded, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(excluded, filepath.Join(workspace, "product")); err != nil {
		t.Fatal(err)
	}

	record := recordWithScopes(filepath.Join(base, "events.jsonl"), workspace, LaunchModeWorktree, scopes)
	evidence, err := Evaluate(record, 0)
	if err != nil {
		t.Fatal(err)
	}
	root := rootEvidence(t, evidence.Products.Roots, declared)
	if root.ScanStanding != StandingAttributionOnly || root.Reason != ReasonResolutionExcluded || root.Status != RootStatusDemoted {
		t.Fatalf("drifted root = %+v, want a labeled scan-time exclusion demotion", root)
	}
	if evidence.Products.EventAt != nil {
		t.Fatalf("excluded files supplied product freshness: %v", evidence.Products.EventAt)
	}
}

func TestSymlinkDriftOutsideWorktreeDemotesAtScan(t *testing.T) {
	base := t.TempDir()
	workspace := filepath.Join(base, "worktree")
	declared := filepath.Join(workspace, "product", "future")
	outside := filepath.Join(base, "outside")
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		t.Fatal(err)
	}
	scopes, err := CaptureProductRootScopes(LaunchModeWorktree, workspace, []string{declared})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(outside, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(workspace, "product")); err != nil {
		t.Fatal(err)
	}

	record := recordWithScopes(filepath.Join(base, "events.jsonl"), workspace, LaunchModeWorktree, scopes)
	evidence, err := Evaluate(record, 0)
	if err != nil {
		t.Fatal(err)
	}
	root := rootEvidence(t, evidence.Products.Roots, declared)
	if root.ScanStanding != StandingAttributionOnly || root.Reason != ReasonResolutionOutsideWorktree || root.Status != RootStatusDemoted {
		t.Fatalf("outside-drifted root = %+v, want a labeled scan-time containment demotion", root)
	}
}

func TestProductFreshnessUsesRecursiveFileEventTime(t *testing.T) {
	base := t.TempDir()
	workspace := filepath.Join(base, "worktree")
	product := filepath.Join(workspace, "product")
	nested := filepath.Join(product, "nested")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	olderAt := time.Unix(1_700_000_000, 0)
	newerAt := olderAt.Add(5 * time.Minute)
	directoryAt := newerAt.Add(24 * time.Hour)
	older := filepath.Join(product, "older.txt")
	newer := filepath.Join(nested, "newer.txt")
	for _, path := range []string{older, newer} {
		if err := os.WriteFile(path, []byte(path), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Chtimes(older, olderAt, olderAt); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(newer, newerAt, newerAt); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(nested, directoryAt, directoryAt); err != nil {
		t.Fatal(err)
	}
	scopes, err := CaptureProductRootScopes(LaunchModeWorktree, workspace, []string{product})
	if err != nil {
		t.Fatal(err)
	}
	record := recordWithScopes(filepath.Join(base, "events.jsonl"), workspace, LaunchModeWorktree, scopes)
	evidence, err := Evaluate(record, 0)
	if err != nil {
		t.Fatal(err)
	}
	if evidence.Products.EventAt == nil || !evidence.Products.EventAt.Equal(newerAt) {
		t.Fatalf("product event time = %v, want newest recursive file mtime %v", evidence.Products.EventAt, newerAt)
	}
}

func TestDelegateWorktreeUnderAgentArtifactsReadsProductFreshness(t *testing.T) {
	repository := t.TempDir()
	workspace := filepath.Join(repository, "artifacts", "agents", "worktrees", "job-a")
	product := filepath.Join(workspace, "source.txt")
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(product, []byte("product"), 0o644); err != nil {
		t.Fatal(err)
	}
	freshAt := time.Unix(1_900_000_000, 0)
	if err := os.Chtimes(product, freshAt, freshAt); err != nil {
		t.Fatal(err)
	}
	scopes, err := CaptureProductRootScopes(LaunchModeWorktree, workspace, []string{workspace})
	if err != nil {
		t.Fatal(err)
	}
	if len(scopes) != 1 || scopes[0].Standing != StandingLiveness {
		t.Fatalf("delegate worktree launch scope = %+v, want liveness", scopes)
	}
	record := recordWithScopes(filepath.Join(repository, "events.jsonl"), workspace, LaunchModeWorktree, scopes)
	evidence, err := Evaluate(record, 0)
	if err != nil {
		t.Fatal(err)
	}
	if evidence.Products.EventAt == nil || !evidence.Products.EventAt.Equal(freshAt) {
		t.Fatalf("delegate worktree product freshness = %v, want %v", evidence.Products.EventAt, freshAt)
	}
}

func TestBroadProductRootPrunesOperationalFiles(t *testing.T) {
	base := t.TempDir()
	workspace := filepath.Join(base, "worktree")
	productFile := filepath.Join(workspace, "source.txt")
	operationalFile := filepath.Join(workspace, "artifacts", "agents", "heartbeat")
	if err := os.MkdirAll(filepath.Dir(operationalFile), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(productFile, []byte("product"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(operationalFile, []byte("presence"), 0o644); err != nil {
		t.Fatal(err)
	}
	productAt := time.Unix(1_700_000_000, 0)
	presenceAt := productAt.Add(24 * time.Hour)
	if err := os.Chtimes(productFile, productAt, productAt); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(operationalFile, presenceAt, presenceAt); err != nil {
		t.Fatal(err)
	}
	scopes, err := CaptureProductRootScopes(LaunchModeWorktree, workspace, []string{workspace})
	if err != nil {
		t.Fatal(err)
	}
	record := recordWithScopes(filepath.Join(base, "events.jsonl"), workspace, LaunchModeWorktree, scopes)
	evidence, err := Evaluate(record, 0)
	if err != nil {
		t.Fatal(err)
	}
	if evidence.Products.EventAt == nil || !evidence.Products.EventAt.Equal(productAt) {
		t.Fatalf("operational state supplied product freshness: got %v want %v", evidence.Products.EventAt, productAt)
	}
}

func TestSharedCheckoutNeverReadsProductFreshness(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, "shared-product")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	product := filepath.Join(root, "fresh.txt")
	if err := os.WriteFile(product, []byte("fresh"), 0o644); err != nil {
		t.Fatal(err)
	}
	freshAt := time.Unix(1_900_000_000, 0)
	if err := os.Chtimes(product, freshAt, freshAt); err != nil {
		t.Fatal(err)
	}
	scopes, err := CaptureProductRootScopes(LaunchModeSharedCheckout, dir, []string{root})
	if err != nil {
		t.Fatal(err)
	}
	record := recordWithScopes(filepath.Join(dir, "events.jsonl"), dir, LaunchModeSharedCheckout, scopes)
	evidence, err := Evaluate(record, 0)
	if err != nil {
		t.Fatal(err)
	}
	got := rootEvidence(t, evidence.Products.Roots, root)
	if got.ScanStanding != StandingAttributionOnly || got.Reason != ReasonSharedCheckout || got.Status != RootStatusNotScanned {
		t.Fatalf("shared-checkout root = %+v, want attribution-only without a scan", got)
	}
	if evidence.Products.EventAt != nil {
		t.Fatalf("shared-checkout product mtime became progress: %v", evidence.Products.EventAt)
	}
}

func outputOnlyRecord(stream string) map[string]any {
	return map[string]any{
		"outputStream":      stream,
		"workspaceRoot":     filepath.Dir(stream),
		"launchMode":        LaunchModeWorktree,
		"productRoots":      []any{},
		"productRootScopes": []any{},
	}
}

func recordWithScopes(stream, workspace, launchMode string, scopes []ProductRootScope) map[string]any {
	roots := make([]any, 0, len(scopes))
	for _, scope := range scopes {
		roots = append(roots, scope.Path)
	}
	return map[string]any{
		"outputStream":      stream,
		"workspaceRoot":     workspace,
		"launchMode":        launchMode,
		"productRoots":      roots,
		"productRootScopes": scopes,
	}
}

func writeSizedFile(t *testing.T, path string, size int) {
	t.Helper()
	if err := os.WriteFile(path, make([]byte, size), 0o644); err != nil {
		t.Fatal(err)
	}
}

func hasAnomaly(anomalies []Anomaly, label string) bool {
	for _, anomaly := range anomalies {
		if anomaly.Label == label {
			return true
		}
	}
	return false
}

func rootEvidence(t *testing.T, roots []ProductRootEvidence, path string) ProductRootEvidence {
	t.Helper()
	for _, root := range roots {
		if root.Path == path {
			return root
		}
	}
	t.Fatalf("no evidence for root %s in %+v", path, roots)
	return ProductRootEvidence{}
}
