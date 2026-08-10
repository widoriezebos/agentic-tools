package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/capability"
)

// runCapabilitySelect ports select-capability-snapshot.py: select and validate
// the capability snapshot for one dispatch, writing the result to --output.
func runCapabilitySelect(args []string) int {
	flags := flag.NewFlagSet("capability select", flag.ContinueOnError)
	root := flags.String("root", "", "checkout root")
	runtime := flags.String("runtime", "", "runtime name")
	role := flags.String("role", "", "role name")
	identity := flags.String("identity", "", "configuration identity JSON")
	maxAge := flags.Int("max-age", -1, "maximum snapshot age in days")
	envelope := flags.String("envelope", "", "envelope JSON path")
	output := flags.String("output", "", "output path")
	if flags.Parse(args) != nil {
		return 2
	}
	if *root == "" || *runtime == "" || *role == "" || *identity == "" || *envelope == "" || *output == "" {
		fmt.Fprintln(os.Stderr, "usage: metasystem capability select --root R --runtime RT --role ROLE --identity JSON --max-age N --envelope E --output O")
		return 2
	}
	if err := capability.Select(*root, *runtime, *role, *identity, *maxAge, *envelope, *output); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}
