package behaviorsurface

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func mustPolicy(t *testing.T) Policy {
	t.Helper()
	policy, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	return policy
}

func TestPolicyVersionAndDeclaredSkipSet(t *testing.T) {
	policy := mustPolicy(t)
	if policy.Version != 2 {
		t.Fatalf("skip policy changed without updating this version fixture: %d", policy.Version)
	}
	if want := []string{"witness-engine-gate"}; !reflect.DeepEqual(policy.WitnessSkips, want) {
		t.Fatalf("witness skip set changed without a policy-version fixture update:\n got %q\nwant %q", policy.WitnessSkips, want)
	}
	wantDelivery := []string{
		"supervision-go-fixtures", "gate-fence-fixtures", "gate-fail-open-tripwire",
		"supervision and census fixtures", "supervisor fingerprint heal harness", "mission-fixtures",
		"conformance-fixtures", "goal-cli-fixtures", "telemetry-census-fixtures", "return-schema-fixtures",
		"authority-regression-fixtures", "pre-commit-guard-fixtures", "static-reproof-fixtures",
		"project-extra-suites", "record-protocol-fixtures", "evidence-segment-fixtures",
		"lease-succession-fixtures", "flight-recorder-fixtures", "acp-fixtures",
		"delegate-caps-fixtures", "adapter-deadline-fixtures",
		"dispatcher, adapter selftest, and mission-runner process fixtures",
	}
	if !reflect.DeepEqual(policy.DeliveryContractSkips, wantDelivery) {
		t.Fatalf("delivery-contract skip set changed without a policy-version fixture update:\n got %q\nwant %q", policy.DeliveryContractSkips, wantDelivery)
	}
	if !policy.SkipAllowed(WitnessScope, "witness-engine-gate") {
		t.Fatal("the witness scope did not authorize its engine gate")
	}
	if policy.SkipAllowed(DeliveryScope, "witness-engine-gate") {
		t.Fatal("the delivery scope borrowed the witness-only engine gate")
	}
	if policy.SkipAllowed(WitnessScope, "mission-fixtures") {
		t.Fatal("the witness scope borrowed a delivery-only family")
	}
	if policy.SkipAllowed("", "not-declared") {
		t.Fatal("an unlisted validation family was authorized to skip")
	}
	for _, test := range []struct {
		value string
		want  SkipScope
	}{{"witness", WitnessScope}, {"DELIVERY", DeliveryScope}} {
		got, err := ParseSkipScope(test.value)
		if err != nil || got != test.want {
			t.Errorf("ParseSkipScope(%q) = %q, %v; want %q", test.value, got, err, test.want)
		}
	}
	if _, err := ParseSkipScope(""); err == nil {
		t.Fatal("an absent skip scope was accepted")
	}
}

func TestPolicyClassesAndProjectionBoundaries(t *testing.T) {
	policy := mustPolicy(t)
	if want := []string{"cmd/**", "internal/**", "scripts/agents/**", "go.mod", "go.sum"}; !reflect.DeepEqual(policy.EnginePaths, want) {
		t.Fatalf("ENGINE closure drifted: got %q want %q", policy.EnginePaths, want)
	}
	if want := []string{
		".gitattributes", ".gitignore", "AGENTS.md", "CLAUDE.md", "cmd/**", "docs/**",
		"go.mod", "go.sum", "internal/**", "metasystem.conf", "optional-skills/**",
		"plans/README.md", "scripts/**", "skills/**", "wow.md",
	}; !reflect.DeepEqual(policy.PayloadRoots, want) {
		t.Fatalf("PAYLOAD allowlist drifted: got %q want %q", policy.PayloadRoots, want)
	}
	if want := []string{
		"benchmark/__pycache__/**", "benchmark/results/**",
		"benchmark/specs/bm-1/grader/__pycache__/**",
		"benchmark/specs/bm-2/grader/__pycache__/**",
		"benchmark/specs/bm-2d/grader/__pycache__/**",
		"benchmark/specs/bm-2dc/grader/__pycache__/**",
		"benchmark/specs/bm-2s/grader/__pycache__/**",
		"benchmark/trials-root.local", "evidence/**",
	}; !reflect.DeepEqual(policy.RepositoryOperationalDataPaths, want) {
		t.Fatalf("LANDING operational-data exclusions drifted: got %q want %q", policy.RepositoryOperationalDataPaths, want)
	}
	tests := []struct {
		path    string
		class   Class
		engine  bool
		landing bool
		payload bool
	}{
		{"internal/gaterun/weight.go", Standard, true, true, true},
		{"cmd/metasystem/main.go", Standard, true, true, true},
		{"scripts/agents/go-gate.sh", Standard, true, true, true},
		{"go.mod", Standard, true, true, true},
		{"docs/collaboration.md", Standard, false, true, true},
		{"docs/project-rules.md", Tailored, false, true, false},
		{"README.md", Standard, false, true, false},
		{"artifacts/agents/job.json", Coordination, false, false, false},
		{"plans/goals.md", Tailored, false, false, false},
		{"plans/receipts.log", Tailored, false, false, false},
		{"metasystem.conf", Tailored, false, true, false},
		{".agents/skills/verify/SKILL.md", Tailored, false, true, false},
		{".gitignore", Tailored, false, true, false},
		{"plans/instruction-ledger.md", Tailored, false, true, false},
		{"bin/metasystem", NonRepository, false, false, false},
		{".git/index", NonRepository, false, false, false},
		{"metasystem.conf.local", Tailored, false, false, false},
		{"skills/verify/SKILL.md", Standard, false, true, true},
	}
	for _, test := range tests {
		class, err := policy.Classify(test.path, "")
		if err != nil || class != test.class {
			t.Errorf("classify %q = %s, %v; want %s", test.path, class, err, test.class)
		}
		for projection, want := range map[Projection]bool{Engine: test.engine, Landing: test.landing, Payload: test.payload} {
			got, err := policy.Includes(projection, test.path, "")
			if err != nil || got != want {
				t.Errorf("%s includes %q = %v, %v; want %v", projection, test.path, got, err, want)
			}
		}
	}
}

func TestLandingRepositoryOperationalDataBoundaries(t *testing.T) {
	policy := mustPolicy(t)
	excluded := []string{
		"benchmark/__pycache__/extractor.pyc",
		"benchmark/results/run/result.json",
		"benchmark/specs/bm-1/grader/__pycache__/grader.pyc",
		"benchmark/specs/bm-2/grader/__pycache__/grader.pyc",
		"benchmark/specs/bm-2d/grader/__pycache__/grader.pyc",
		"benchmark/specs/bm-2dc/grader/__pycache__/grader.pyc",
		"benchmark/specs/bm-2s/grader/__pycache__/grader.pyc",
		"benchmark/trials-root.local",
		"evidence",
		"evidence/run/envelope.json",
	}
	for _, prefix := range []string{"", "metasystem/"} {
		for _, path := range excluded {
			class, err := policy.Classify(path, prefix)
			if err != nil || class != OperationalData {
				t.Errorf("classify %q with prefix %q = %s, %v; want %s", path, prefix, class, err, OperationalData)
			}
			included, err := policy.Includes(Landing, path, prefix)
			if err != nil || included {
				t.Errorf("LANDING includes operational data %q with prefix %q = %v, %v; want false", path, prefix, included, err)
			}
		}

		for _, path := range []string{
			"benchmark/evidence-drift-fixtures.sh",
			"benchmark/results-fixtures.sh",
			"benchmark/specs/bm-1/grader/grader.py",
			"benchmark/trials-root.local.example",
			"evidence-drift-fixtures.sh",
		} {
			included, err := policy.Includes(Landing, path, prefix)
			if err != nil || !included {
				t.Errorf("repository neighbor %q with prefix %q left LANDING: %v %v", path, prefix, included, err)
			}
		}

		for _, relative := range []string{"internal/gaterun/ignored.go", "cmd/metasystem/ignored.go"} {
			path := relative
			if prefix != "" {
				path = prefix + relative
			}
			included, err := policy.Includes(Landing, path, prefix)
			if err != nil || !included {
				t.Errorf("ignored Go source %q with prefix %q left LANDING: %v %v", path, prefix, included, err)
			}
		}
	}
	included, err := policy.Includes(Landing, "metasystem/evidence/x", "metasystem/")
	if err != nil || !included {
		t.Errorf("a nested evidence path was swallowed by the repository-root exclusion: %v %v", included, err)
	}
	if _, err := policy.Includes(Landing, "benchmark/results/../../internal/x.go", ""); err == nil {
		t.Error("a traversal-shaped path was accepted instead of refused")
	}
}

func TestNestedPrefixAndRenameAcrossClasses(t *testing.T) {
	policy := mustPolicy(t)
	rootClass, _ := policy.Classify("plans/goals.md", "")
	nestedClass, _ := policy.Classify("vendor/meta/plans/goals.md", "vendor/meta/")
	if rootClass != nestedClass {
		t.Fatalf("nested classification drifted: %s vs %s", rootClass, nestedClass)
	}
	toplevelControl, err := policy.Includes(Landing, "go.work", "metasystem/")
	if err != nil || !toplevelControl {
		t.Fatalf("repository-toplevel control was lost under nested prefix: %v %v", toplevelControl, err)
	}
	outside, err := policy.Includes(Landing, "project-owned.txt", "metasystem/")
	if err != nil || !outside {
		t.Fatalf("repository content outside a nested metasystem left LANDING: %v %v", outside, err)
	}
	extraSuite, err := policy.Includes(Landing, "benchmark/evidence-drift-fixtures.sh", "metasystem/")
	if err != nil || !extraSuite {
		t.Fatalf("repository extra suite left nested LANDING: %v %v", extraSuite, err)
	}
	changes, err := policy.ClassifyChanges(Landing, "metasystem/", []Change{
		{Path: "metasystem/docs/old.md", Kind: "remove"},
		{Path: "metasystem/artifacts/new.md", Kind: "add"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(changes) != 2 || !changes[0].Included || changes[1].Included {
		t.Fatalf("rename sides lost their independent policy facts: %+v", changes)
	}
}

func TestDigestIsNULSafeModeIndependentAndDoesNotFollowSymlinks(t *testing.T) {
	policy := mustPolicy(t)
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "docs"), 0o755); err != nil {
		t.Fatal(err)
	}
	odd := filepath.Join(root, "docs", "white space\n$meta;name.md")
	if err := os.WriteFile(odd, []byte("one"), 0o600); err != nil {
		t.Fatal(err)
	}
	first, err := policy.Digest(root, Landing)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(odd, 0o755); err != nil {
		t.Fatal(err)
	}
	second, err := policy.Digest(root, Landing)
	if err != nil || first != second {
		t.Fatalf("mode entered digest: %s then %s (%v)", first, second, err)
	}
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "secret"), []byte("outside"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "docs", "linked")); err != nil {
		t.Fatal(err)
	}
	withLink, err := policy.Digest(root, Landing)
	if err != nil || withLink == second {
		t.Fatalf("symlink target text should enter without following its content: %s %v", withLink, err)
	}
	if err := os.WriteFile(filepath.Join(outside, "secret"), []byte(strings.Repeat("changed", 4)), 0o600); err != nil {
		t.Fatal(err)
	}
	afterOutsideChange, err := policy.Digest(root, Landing)
	if err != nil || afterOutsideChange != withLink {
		t.Fatalf("digest followed a symlink ancestor: %s then %s (%v)", withLink, afterOutsideChange, err)
	}
}

func TestLandingDigestIsRepositoryWideAndExcludesNestedCoordination(t *testing.T) {
	policy := mustPolicy(t)
	nested := t.TempDir()
	for _, path := range []string{filepath.Join(nested, "metasystem", "docs"), filepath.Join(nested, "metasystem", "artifacts")} {
		if err := os.MkdirAll(path, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(nested, "metasystem", "docs", "same.md"), []byte("same"), 0o644); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(nested, "benchmark", "evidence-drift-fixtures.sh")
	if err := os.MkdirAll(filepath.Dir(outside), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(outside, []byte("one"), 0o644); err != nil {
		t.Fatal(err)
	}
	first, err := policy.DigestWithPrefix(nested, Landing, "metasystem/")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(outside, []byte("two"), 0o644); err != nil {
		t.Fatal(err)
	}
	second, err := policy.DigestWithPrefix(nested, Landing, "metasystem/")
	if err != nil || second == first {
		t.Fatalf("outside-prefix repository bytes did not affect LANDING: %s then %s (%v)", first, second, err)
	}
	operational := filepath.Join(nested, "benchmark", "results", "run.json")
	if err := os.MkdirAll(filepath.Dir(operational), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(operational, []byte("one"), 0o644); err != nil {
		t.Fatal(err)
	}
	third, err := policy.DigestWithPrefix(nested, Landing, "metasystem/")
	if err != nil || third != second {
		t.Fatalf("repository operational data affected LANDING: %s then %s (%v)", second, third, err)
	}
	if err := os.WriteFile(operational, []byte("two"), 0o644); err != nil {
		t.Fatal(err)
	}
	fourth, err := policy.DigestWithPrefix(nested, Landing, "metasystem/")
	if err != nil || fourth != third {
		t.Fatalf("repository operational-data content affected LANDING: %s then %s (%v)", third, fourth, err)
	}
	if err := os.WriteFile(filepath.Join(nested, "metasystem", "artifacts", "coordination"), []byte("ignored"), 0o644); err != nil {
		t.Fatal(err)
	}
	fifth, err := policy.DigestWithPrefix(nested, Landing, "metasystem/")
	if err != nil || fifth != fourth {
		t.Fatalf("nested coordination bytes affected LANDING: %s then %s (%v)", fourth, fifth, err)
	}
}

func TestDigestDomainSeparatesPolicyVersionAndProjection(t *testing.T) {
	policy := mustPolicy(t)
	root := t.TempDir()
	engine, err := policy.Digest(root, Engine)
	if err != nil {
		t.Fatal(err)
	}
	payload, err := policy.Digest(root, Payload)
	if err != nil {
		t.Fatal(err)
	}
	if engine == payload {
		t.Fatal("empty ENGINE and PAYLOAD projections shared a digest domain")
	}
	otherVersion := policy
	otherVersion.Version++
	versioned, err := otherVersion.Digest(root, Engine)
	if err != nil {
		t.Fatal(err)
	}
	if versioned == engine {
		t.Fatal("policy version did not enter the digest domain")
	}
}

func TestPayloadManifestIgnoresProjectExtrasButRequiresEverySourcePath(t *testing.T) {
	policy := mustPolicy(t)
	source, target := t.TempDir(), t.TempDir()
	for _, root := range []string{source, target} {
		if err := os.MkdirAll(filepath.Join(root, "docs"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, "docs", "owned.md"), []byte("payload"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(target, "docs", "project-extra.md"), []byte("project"), 0o644); err != nil {
		t.Fatal(err)
	}
	paths, err := policy.ListPaths(source, Payload)
	if err != nil {
		t.Fatal(err)
	}
	left, err := policy.DigestListed(source, Payload, paths)
	if err != nil {
		t.Fatal(err)
	}
	right, err := policy.DigestListed(target, Payload, paths)
	if err != nil || left != right {
		t.Fatalf("project extra changed source-manifest payload equality: %s %s %v", left, right, err)
	}
	if err := os.Remove(filepath.Join(target, "docs", "owned.md")); err != nil {
		t.Fatal(err)
	}
	if _, err := policy.DigestListed(target, Payload, paths); err == nil {
		t.Fatal("missing source payload path was accepted")
	}
	if err := os.RemoveAll(filepath.Join(target, "docs")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(source, "docs"), filepath.Join(target, "docs")); err != nil {
		t.Fatal(err)
	}
	if _, err := policy.DigestListed(target, Payload, paths); err == nil || !strings.Contains(err.Error(), "symlink ancestor") {
		t.Fatalf("payload component walk accepted a symlink ancestor: %v", err)
	}
}

func TestUnsupportedPolicyVersionRefuses(t *testing.T) {
	bad := strings.Replace(string(policyBytes), `"version": 2`, `"version": 999`, 1)
	if _, err := loadPolicy([]byte(bad)); err == nil || !strings.Contains(err.Error(), "unsupported") {
		t.Fatalf("unsupported policy version did not refuse: %v", err)
	}
	if _, err := loadPolicy(append(policyBytes, []byte(` {}`)...)); err == nil {
		t.Fatal("trailing policy content was accepted")
	}
}

func TestSkipFamilyCannotBelongToBothProofScopes(t *testing.T) {
	bad := strings.Replace(string(policyBytes), `"deliveryContractSkips": [`, `"deliveryContractSkips": ["witness-engine-gate",`, 1)
	if _, err := loadPolicy([]byte(bad)); err == nil || !strings.Contains(err.Error(), "both") {
		t.Fatalf("cross-scope skip family did not refuse: %v", err)
	}
}

func TestDirectoryNotationClassifiesAsItsPath(t *testing.T) {
	policy := mustPolicy(t)
	excluded, err := policy.Includes(Landing,
		"metasystem/artifacts/agents/suite-failures/run/agent-fixture/repo/", "metasystem/")
	if err != nil || excluded {
		t.Fatalf("slash-terminated artifact directory was not excluded cleanly: %v %v", excluded, err)
	}
	included, err := policy.Includes(Landing, "metasystem/internal/gaterun/", "metasystem/")
	if err != nil || !included {
		t.Fatalf("slash-terminated source directory left LANDING: %v %v", included, err)
	}
	if _, err := NormalizePath("internal//gaterun/", ""); err == nil {
		t.Fatal("a non-cosmetic unclean path was accepted")
	}
	if _, err := NormalizePath("/", ""); err == nil {
		t.Fatal("the filesystem root was accepted as a path")
	}
}
