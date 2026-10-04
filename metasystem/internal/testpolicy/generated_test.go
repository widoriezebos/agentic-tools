package testpolicy

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestGeneratedContractDeclaration(t *testing.T) {
	t.Parallel()
	contract := fixtureContract()
	data, err := json.Marshal(contract)
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := Decode(data)
	if err != nil || len(loaded.Generated) != 0 {
		t.Fatalf("contract without generated: sets=%v err=%v", loaded.Generated, err)
	}
	set := Generated{Paths: []string{"target/openapi/**"}, Command: []string{"mvn", "-B", "generate-sources"}}
	contract.Generated = []Generated{set}
	data, err = json.Marshal(contract)
	if err != nil {
		t.Fatal(err)
	}
	loaded, err = Decode(data)
	if err != nil || !reflect.DeepEqual(loaded.Generated, contract.Generated) {
		t.Fatalf("declared recipe: sets=%v err=%v", loaded.Generated, err)
	}
	if !set.Generates("target/openapi/client.java") || set.Generates("src/client.java") {
		t.Fatal("declaration did not distinguish generated outputs from sources")
	}
}

func TestGeneratedContractRejectsInvalidDeclarations(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name   string
		sets   []Generated
		reason string
	}{
		{"missing paths", []Generated{{Command: []string{"build"}}}, "requires paths"},
		{"missing command", []Generated{{Paths: []string{"out/**"}}}, "requires a command argv"},
		{"empty then", []Generated{{Paths: []string{"out/**"}, Command: []string{"build"}, Then: []string{}}}, "requires a command argv"},
		{"outside path", []Generated{{Paths: []string{"../out/**"}, Command: []string{"build"}}}, "traverses outside"},
		{"outside cwd", []Generated{{Paths: []string{"out/**"}, Command: []string{"build"}, Cwd: "../src"}}, "cwd must be relative"},
		{"nul argument", []Generated{{Paths: []string{"out/**"}, Command: []string{"build", "a\x00b"}}}, "contains a NUL"},
		{"same sorted paths", []Generated{
			{Paths: []string{"out/a", "out/b"}, Command: []string{"first"}},
			{Paths: []string{"out/b", "out/a"}, Command: []string{"second"}},
		}, "repeats the same paths"},
		{"same normalized pattern", []Generated{
			{Paths: []string{"out/**"}, Command: []string{"first"}},
			{Paths: []string{"./out/**", "extra/**"}, Command: []string{"second"}},
		}, "repeats path pattern"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			contract := fixtureContract()
			contract.Generated = test.sets
			if err := contract.Validate(); err == nil || !strings.Contains(err.Error(), test.reason) {
				t.Fatalf("invalid declaration: err=%v; want %q", err, test.reason)
			}
		})
	}
}
