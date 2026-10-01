package main

import (
	"go/parser"
	"go/token"
	"os"
	"strconv"
	"strings"
	"testing"
)

// TestStatusAndWorkStatusHaveNoPathToTheSocket (R25, U10c-2): the one-shot
// views read the board directly; none of their files dials, subscribes or
// names the bridge's socket.
func TestStatusAndWorkStatusHaveNoPathToTheSocket(t *testing.T) {
	t.Parallel()
	for _, file := range []string{"intent_table.go", "intent_selection.go", "intent_process.go"} {
		parsed, err := parser.ParseFile(token.NewFileSet(), file, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		source, _ := os.ReadFile(file)
		for _, forbidden := range []string{"board.Dial", "board.Subscribe", "bridgeNudges", "bridge.sock", "net.Dial"} {
			if strings.Contains(string(source), forbidden) {
				t.Errorf("%s names %s: a one-shot view never uses the bridge", file, forbidden)
			}
		}
		for _, spec := range parsed.Imports {
			if path, _ := strconv.Unquote(spec.Path.Value); path == "net" {
				t.Errorf("%s imports net", file)
			}
		}
	}
}
