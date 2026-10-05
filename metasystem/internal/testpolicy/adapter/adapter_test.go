package adapter_test

import (
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy/adapter"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy/adapter/fakeadapter"
)

func TestCommandAndSectionAdaptersAnswerMinimally(t *testing.T) {
	t.Parallel()
	failure := adapter.Failure{Report: "junit-xml", Classname: "com.example.PaymentTest", Name: "rejectsInvalidInput"}
	for _, tc := range []struct {
		group    testpolicy.Group
		identity bool
	}{
		{testpolicy.Group{Adapter: "command", Format: "junit-xml"}, true},
		{testpolicy.Group{Adapter: "command", Format: "exit-status"}, false},
		{testpolicy.Group{Adapter: "section"}, false},
	} {
		bound, err := adapter.Resolve(tc.group)
		if err != nil {
			t.Fatal(err)
		}
		closure, err := bound.Closure("root", "base", "tree")
		if err != nil || closure.Tree != "tree" || len(closure.Units()) != 0 {
			t.Fatalf("%+v closure=%+v err=%v", tc.group, closure, err)
		}
		if len(bound.TestSteps(closure)) != 0 {
			t.Fatalf("%+v planned tests without changed units", tc.group)
		}
		if unit, ok := bound.OwnerUnit(failure, closure); ok {
			t.Fatalf("%+v owned the failure by %q", tc.group, unit)
		}
		if _, ok := bound.Identity(failure); ok != tc.identity {
			t.Fatalf("%+v identity ok=%v want %v", tc.group, ok, tc.identity)
		}
	}
	if _, err := adapter.Resolve(testpolicy.Group{Adapter: "cobol"}); err == nil {
		t.Fatal("an unbound adapter resolved")
	}
}

func TestFakeAdapterOwnsByModuleNotByClassname(t *testing.T) {
	t.Parallel()
	fake := fakeadapter.New()
	closure, err := fake.Closure("root", "base", "tip")
	if err != nil || !closure.Contains("payments") || !closure.Contains("ledger") || closure.Tree != "tip" {
		t.Fatalf("fake closure=%+v err=%v", closure, err)
	}
	failure := adapter.Failure{Classname: "com.example.PaymentTest", Name: "rejectsInvalidInput"}
	if closure.Contains(failure.Classname) {
		t.Fatal("the fake's classname equals a unit name; the witness would pass by coincidence")
	}
	if unit, ok := fake.OwnerUnit(failure, closure); !ok || unit != "payments" {
		t.Fatalf("owner=%q ok=%v", unit, ok)
	}
	if _, ok := fake.Identity(adapter.Failure{Classname: "com.example.PaymentTest", Name: "<compile>"}); ok {
		t.Fatal("the scripted unidentified failure carried an identity")
	}
}
