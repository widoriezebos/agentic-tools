package contractmerge

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

func TestMergeByIdentityAndSetChanges(t *testing.T) {
	base := mergeFixture()
	ours, theirs := cloneFixture(t, base), cloneFixture(t, base)
	ours.Groups[0].Packages = []string{"pkg/base", "pkg/ours"}
	ours.Groups[0].Tests = json.RawMessage(`["TestBase","TestOurs"]`)
	ours.Groups = append(ours.Groups, fixtureGroup("ours-group", 1000, "TestOursGroup"))
	ours.Surfaces = appendBeforeFallback(ours.Surfaces, fixtureSurface("ours-surface", "ours-group"))
	ours.Surfaces = appendBeforeFallback(ours.Surfaces, fixtureSurface("shared", "ours-group"))
	ours.Unknown = append(ours.Unknown, "ours-group")
	theirs.Groups[0].Packages = []string{"pkg/base", "pkg/removed", "pkg/theirs"}
	theirs.Groups[0].Tests = json.RawMessage(`["TestBase","TestRemoved","TestTheirs"]`)
	theirs.Groups = append(theirs.Groups, fixtureGroup("theirs-group", 1000, "TestTheirsGroup"))
	theirs.Surfaces = appendBeforeFallback(theirs.Surfaces, fixtureSurface("theirs-surface", "theirs-group"))
	theirs.Surfaces = appendBeforeFallback(theirs.Surfaces, fixtureSurface("shared", "theirs-group"))
	theirs.Unknown = append(theirs.Unknown, "theirs-group")
	ours.Surfaces[0].Risk = &testpolicy.RiskRaise{Severity: 2}
	theirs.Surfaces[0].Risk = &testpolicy.RiskRaise{Exposure: 2}

	merged, err := Merge(base, ours, theirs)
	if err != nil {
		t.Fatal(err)
	}
	if got := ids(merged.Surfaces); !reflect.DeepEqual(got, []string{"app", "ours-surface", "shared", "residual", "theirs-surface"}) {
		t.Fatalf("surface identities = %v", got)
	}
	if got := ids(merged.Groups); !reflect.DeepEqual(got, []string{"app-group", "ours-group", "theirs-group"}) {
		t.Fatalf("group identities = %v", got)
	}
	group := merged.Groups[0]
	if !reflect.DeepEqual(group.Packages, []string{"pkg/base", "pkg/ours", "pkg/theirs"}) {
		t.Fatalf("packages did not apply removals and additions as a set: %v", group.Packages)
	}
	_, tests, err := testpolicy.GoTests(group)
	if err != nil || !reflect.DeepEqual(tests, []string{"TestBase", "TestOurs", "TestTheirs"}) {
		t.Fatalf("tests did not apply removals and additions as a set: tests=%v err=%v", tests, err)
	}
	if risk := merged.Surfaces[0].Risk; risk == nil || risk.Severity != 2 || risk.Exposure != 2 {
		t.Fatalf("independent scalar changes inside a new risk raise did not merge: %+v", risk)
	}
	if got := surfaceByID(t, merged, "shared").Standard; !reflect.DeepEqual(got, []string{"ours-group", "theirs-group"}) {
		t.Fatalf("same new surface list changes did not merge: %v", got)
	}

	canonical, err := Render(base)
	if err != nil {
		t.Fatal(err)
	}
	unchanged, err := MergeBytes(canonical, canonical, canonical)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(unchanged, canonical) {
		t.Fatal("an unchanged canonical merge changed bytes")
	}
}

func TestMergeRecomputesFallbackAndContextMirror(t *testing.T) {
	base := mergeFixture()
	base.Surfaces = appendBeforeFallback(base.Surfaces, fixtureSurface("context-budget", "app-group"))
	ours, theirs := cloneFixture(t, base), cloneFixture(t, base)
	ours.Groups = append(ours.Groups, fixtureGroup("ours-group", 1000, "TestOurs"))
	ours.Surfaces[0].Standard = append(ours.Surfaces[0].Standard, "ours-group")
	theirs.Groups = append(theirs.Groups, fixtureGroup("theirs-group", 1000, "TestTheirs"))
	theirs.Surfaces = appendBeforeFallback(theirs.Surfaces, fixtureSurface("theirs-surface", "theirs-group"))
	merged, err := Merge(base, ours, theirs)
	if err != nil {
		t.Fatal(err)
	}
	fallback := surfaceByID(t, merged, "residual")
	context := surfaceByID(t, merged, "context-budget")
	want := []string{"app-group", "ours-group", "theirs-group"}
	if !reflect.DeepEqual(fallback.Standard, want) || !reflect.DeepEqual(context.Standard, want) {
		t.Fatalf("derived lists: fallback=%v context=%v want=%v", fallback.Standard, context.Standard, want)
	}
}

func TestMergeRefusesConflictsByEntityAndField(t *testing.T) {
	tests := []struct {
		name       string
		change     func(*testpolicy.Contract, *testpolicy.Contract, *testpolicy.Contract)
		entityPart string
		field      string
	}{
		{"scalar", func(_ *testpolicy.Contract, ours, theirs *testpolicy.Contract) {
			ours.Groups[0].Kind = "component"
			theirs.Groups[0].Kind = "integration"
		}, `group "app-group"`, "kind"},
		{"remove-changed", func(base, ours, theirs *testpolicy.Contract) {
			group := fixtureGroup("spare", 1000, "TestSpare")
			base.Groups = append(base.Groups, group)
			ours.Groups = append([]testpolicy.Group(nil), base.Groups[:1]...)
			theirs.Groups = append([]testpolicy.Group(nil), base.Groups...)
			theirs.Groups[1].TargetMS = 2000
		}, `group "spare"`, "targetMs"},
		{"different-new-group", func(_ *testpolicy.Contract, ours, theirs *testpolicy.Contract) {
			ours.Groups = append(ours.Groups, fixtureGroup("new-group", 1000, "TestNew"))
			theirs.Groups = append(theirs.Groups, fixtureGroup("new-group", 2000, "TestNew"))
		}, `group "new-group"`, "targetMs"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			base := mergeFixture()
			ours, theirs := cloneFixture(t, base), cloneFixture(t, base)
			test.change(&base, &ours, &theirs)
			_, err := Merge(base, ours, theirs)
			var refusal *Refusal
			if !errors.As(err, &refusal) || refusal.Code != MergeConflictCode || !strings.Contains(refusal.Entity, test.entityPart) || refusal.Field != test.field {
				t.Fatalf("refusal = %#v err=%v", refusal, err)
			}
		})
	}
}

func TestMergeTreatsListReorderingAsTheSameDefinition(t *testing.T) {
	base := mergeFixture()
	base.Groups = append(base.Groups, fixtureGroup("spare", 1000, "TestOne", "TestTwo"))
	ours, theirs := cloneFixture(t, base), cloneFixture(t, base)
	ours.Groups = ours.Groups[:1]
	theirs.Groups[1].Packages = []string{"pkg/removed", "pkg/base"}
	theirs.Groups[1].Tests = json.RawMessage(`["TestTwo","TestOne"]`)
	merged, err := Merge(base, ours, theirs)
	if err != nil || len(merged.Groups) != 1 {
		t.Fatalf("reorder-only change conflicted with removal: groups=%v err=%v", ids(merged.Groups), err)
	}
	newOurs := fixtureGroup("new-group", 1000, "TestOne", "TestTwo")
	newTheirs := newOurs
	newTheirs.Packages = []string{"pkg/removed", "pkg/base"}
	newTheirs.Tests = json.RawMessage(`["TestTwo","TestOne"]`)
	ours, theirs = cloneFixture(t, mergeFixture()), cloneFixture(t, mergeFixture())
	ours.Groups, theirs.Groups = append(ours.Groups, newOurs), append(theirs.Groups, newTheirs)
	if _, err := Merge(mergeFixture(), ours, theirs); err != nil {
		t.Fatalf("equivalent new definitions conflicted: %v", err)
	}
}

func TestMergeRefusesInvalidResultAfterLoadValidation(t *testing.T) {
	base := mergeFixture()
	base.Groups = append(base.Groups, fixtureGroup("spare", 1000, "TestSpare"))
	ours, theirs := cloneFixture(t, base), cloneFixture(t, base)
	ours.Groups = ours.Groups[:1]
	theirs.Surfaces[0].Standard = append(theirs.Surfaces[0].Standard, "spare")
	recomputeDerived(&theirs)
	_, err := Merge(base, ours, theirs)
	var refusal *Refusal
	if !errors.As(err, &refusal) || refusal.Code != InvalidContractCode || !strings.Contains(refusal.Detail, "missing group spare") {
		t.Fatalf("invalid merge refusal = %#v err=%v", refusal, err)
	}
}

func TestAddTestsValidatesGroupSourceAndIsIdempotent(t *testing.T) {
	root := t.TempDir()
	contractPath := filepath.Join(root, "testing.json")
	source := "package example\nfunc TestBase(t any) {}\nfunc TestAdded(t any) {}\n"
	if err := os.WriteFile(filepath.Join(root, "example_test.go"), []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	base := mergeFixture()
	base.Groups[0].CWD = "."
	base.Groups[0].Packages = []string{"."}
	base.Groups[0].Tests = json.RawMessage(`["TestBase"]`)
	if data, err := Render(base); err != nil {
		t.Fatal(err)
	} else if err := os.WriteFile(contractPath, data, 0o644); err != nil {
		t.Fatal(err)
	}

	updated, err := AddTests(base, contractPath, "app-group", []string{"TestAdded", "TestAdded"})
	if err != nil {
		t.Fatal(err)
	}
	_, names, err := testpolicy.GoTests(updated.Groups[0])
	if err != nil || !reflect.DeepEqual(names, []string{"TestAdded", "TestBase"}) {
		t.Fatalf("updated tests=%v err=%v", names, err)
	}
	updatedAgain, err := AddTests(updated, contractPath, "app-group", []string{"TestAdded"})
	if err != nil || !reflect.DeepEqual(updated.Groups[0].Tests, updatedAgain.Groups[0].Tests) {
		t.Fatalf("idempotent add changed tests: err=%v before=%s after=%s", err, updated.Groups[0].Tests, updatedAgain.Groups[0].Tests)
	}
	slash := base
	slash.Groups[0].Packages = []string{"./"}
	if _, err := AddTests(slash, contractPath, "app-group", []string{"TestAdded"}); err != nil {
		t.Fatalf("current package spelling ./ refused: %v", err)
	}
	for _, test := range []struct {
		group, name, detail string
	}{
		{"missing", "TestAdded", "unknown group"},
		{"app-group", "TestAbsent", "no matching func TestAbsent("},
	} {
		_, err := AddTests(base, contractPath, test.group, []string{test.name})
		var refusal *Refusal
		if !errors.As(err, &refusal) || refusal.Code != AddTestsCode || !strings.Contains(refusal.Detail, test.detail) {
			t.Errorf("group=%s name=%s refusal=%#v err=%v", test.group, test.name, refusal, err)
		}
	}
}

func TestCanonicalRenderMatchesRepositoryContractBytes(t *testing.T) {
	data, err := os.ReadFile("../../../testing.json")
	if err != nil {
		t.Fatal(err)
	}
	contract, err := testpolicy.Decode(data)
	if err != nil {
		t.Fatal(err)
	}
	rendered, err := Render(contract)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(rendered, data) {
		limit := len(data)
		if len(rendered) < limit {
			limit = len(rendered)
		}
		at := 0
		for at < limit && rendered[at] == data[at] {
			at++
		}
		end := at + 120
		if end > limit {
			end = limit
		}
		t.Fatalf("canonical renderer changed metasystem/testing.json bytes at %d: got=%q want=%q lengths=%d/%d", at, rendered[at:end], data[at:end], len(rendered), len(data))
	}
	merged, err := MergeBytes(data, data, data)
	if err != nil || !reflect.DeepEqual(merged, data) {
		at := 0
		for err == nil && at < len(data) && at < len(merged) && data[at] == merged[at] {
			at++
		}
		end := at + 100
		if end > len(data) || end > len(merged) {
			end = at
		}
		t.Fatalf("unchanged repository contract merge changed bytes at %d: err=%v got=%q want=%q", at, err, merged[at:end], data[at:end])
	}
	nilLists := mergeFixture()
	nilLists.Surfaces[0].DependsOn, nilLists.Surfaces[0].Deep, nilLists.Surfaces[0].Critical = nil, nil, nil
	nilLists.Surfaces[1].Deep, nilLists.Surfaces[1].Critical = nil, nil
	canonical, _ := Render(nilLists)
	unchanged, err := MergeBytes(canonical, canonical, canonical)
	if err != nil || !reflect.DeepEqual(unchanged, canonical) {
		t.Fatalf("unchanged null lists changed bytes: err=%v got=%s want=%s", err, unchanged, canonical)
	}
	emptyMirror := mergeFixture()
	emptyMirror.Surfaces = appendBeforeFallback(emptyMirror.Surfaces, fixtureSurface("context-budget", "app-group"))
	canonical, _ = Render(emptyMirror)
	unchanged, err = MergeBytes(canonical, canonical, canonical)
	if err != nil || !reflect.DeepEqual(unchanged, canonical) {
		t.Fatalf("unchanged empty context mirror changed bytes: err=%v got=%s want=%s", err, unchanged, canonical)
	}
}

func TestHistoryMergeReplaysParallelTestingContractChanges(t *testing.T) {
	// Trimmed from testing.json at base 7bb2f7e92bb17a294a0042d4c9d79d5796c0d8c1,
	// ours 8977a536b0019c40dbe8439c14f31cccd474770f, and theirs
	// 5354c9441ef44abc5a021d82dad06c7ada6aaebf. The first side added goal
	// branch tests; the second added goal-list tests and batch groups.
	read := func(name string) []byte {
		t.Helper()
		data, err := os.ReadFile(filepath.Join("testdata", "history-"+name+".json"))
		if err != nil {
			t.Fatal(err)
		}
		return data
	}
	merged, err := MergeBytes(read("base"), read("ours"), read("theirs"))
	if err != nil {
		t.Fatal(err)
	}
	want := read("want")
	if !reflect.DeepEqual(merged, want) {
		t.Fatalf("history merge differs from hand-made result:\n%s", merged)
	}
}

func mergeFixture() testpolicy.Contract {
	return testpolicy.Contract{SchemaVersion: 1,
		ProjectRisk: testpolicy.ProjectRisk{Severity: 1, Exposure: 1, Reversibility: "revert", Detection: "immediate", Recovery: "bounded"},
		Fallback:    "residual",
		Surfaces: []testpolicy.Surface{
			fixtureSurface("app", "app-group"),
			{ID: "residual", Paths: []string{}, DependsOn: []string{}, Standard: []string{"app-group"}, Deep: []string{}, Critical: []string{}},
		},
		Groups:  []testpolicy.Group{fixtureGroup("app-group", 1000, "TestBase", "TestRemoved")},
		Always:  testpolicy.Always{Canary: []string{}, Standard: []string{}},
		Unknown: []string{"app-group"}, Cadence: []string{},
	}
}

func fixtureGroup(id string, target int64, tests ...string) testpolicy.Group {
	data, _ := json.Marshal(tests)
	return testpolicy.Group{ID: id, Kind: "unit", Adapter: "go", CWD: ".", Inputs: []string{"go.mod"}, Outputs: []string{}, Tools: []testpolicy.Tool{}, Obligations: []string{}, Platforms: []string{"any"}, TargetMS: target, Packages: []string{"pkg/base", "pkg/removed"}, Tests: data}
}

func fixtureSurface(id, group string) testpolicy.Surface {
	return testpolicy.Surface{ID: id, Paths: []string{id + "/**"}, DependsOn: []string{}, Standard: []string{group}, Deep: []string{}, Critical: []string{}}
}

func appendBeforeFallback(values []testpolicy.Surface, addition testpolicy.Surface) []testpolicy.Surface {
	result := append([]testpolicy.Surface(nil), values[:len(values)-1]...)
	result = append(result, addition, values[len(values)-1])
	return result
}

func cloneFixture(t *testing.T, value testpolicy.Contract) testpolicy.Contract {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	var result testpolicy.Contract
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatal(err)
	}
	return result
}

func ids[T interface {
	testpolicy.Surface | testpolicy.Group
}](values []T) []string {
	result := make([]string, len(values))
	for i, value := range values {
		data, _ := json.Marshal(value)
		var identity struct {
			ID string `json:"id"`
		}
		_ = json.Unmarshal(data, &identity)
		result[i] = identity.ID
	}
	return result
}

func surfaceByID(t *testing.T, contract testpolicy.Contract, id string) testpolicy.Surface {
	t.Helper()
	for _, surface := range contract.Surfaces {
		if surface.ID == id {
			return surface
		}
	}
	t.Fatalf("surface %s missing", id)
	return testpolicy.Surface{}
}
