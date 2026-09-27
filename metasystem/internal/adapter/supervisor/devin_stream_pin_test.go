package supervisor

import (
	"os"
	"strings"
	"testing"
)

// TestDevinArgvCarriesNoStreamMode is the devin half of the stream-mode
// implication (formerly a read of adapters/devin.sh): devin's probe
// declares nativeEvents false, so its launch never asks for a streamed
// output mode, and its claim tag rides the private config file's name.
func TestDevinArgvCarriesNoStreamMode(t *testing.T) {
	t.Parallel()
	data, err := os.ReadFile("devin.go")
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	if strings.Contains(content, "stream-json") {
		t.Fatal("devin.go mentions a stream output mode its probe declares false")
	}
	if !strings.Contains(content, `"nativeEvents":false`) {
		t.Fatal("devin.go no longer declares nativeEvents false; revisit the implication pin")
	}
	description, err := registry["devin"].ops.Describe(Deps{})
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, shape := range description.Invocations {
		if shape.TagFlag == "--config" && shape.TagPathBase {
			found = true
		}
	}
	if !found {
		t.Fatal("Devin's CLI config argument does not carry the reservation instance tag")
	}
}
