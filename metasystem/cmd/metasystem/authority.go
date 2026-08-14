package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/authority"
)

// runAuthorityCheck exits 0 if the classified caller may perform the mode's
// write, else exits 1 with the refusal.
func runAuthorityCheck(args []string) int {
	flags := flag.NewFlagSet("job authority-check", flag.ContinueOnError)
	mode := flags.String("mode", "", "control-plane write mode")
	classification := flags.String("classification", "", "caller classification JSON")
	job := flags.String("job", "", "job named by a record-mutating call")
	if flags.Parse(args) != nil {
		return 2
	}
	if !authority.ValidMode(*mode) {
		fmt.Fprintf(os.Stderr, "unknown control-plane mode %q\n", *mode)
		return 2
	}
	var caller map[string]any
	if err := json.Unmarshal([]byte(*classification), &caller); err != nil {
		fmt.Fprintf(os.Stderr, "caller classification is not JSON: %v\n", err)
		return 2
	}
	if err := authority.Authorize(*mode, caller, *job); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}
