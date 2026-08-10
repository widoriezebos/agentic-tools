package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
)

// runJSONGet replaces the `json_field` python heredoc every shell
// script carries: print a dotted field from a JSON file, exit 1 when
// the file or field is absent or unparseable. It is a PORT of the
// heredoc's observable contract — scalar values print bare (no
// quotes), composite values print as compact JSON — because dozens
// of call sites string-compare the output.
func runJSONGet(args []string) int {
	flags := flag.NewFlagSet("json get", flag.ContinueOnError)
	file := flags.String("file", "", "JSON file to read")
	field := flags.String("field", "", "dotted field path (a.b.c)")
	if flags.Parse(args) != nil {
		return 2
	}
	if *file == "" || *field == "" {
		fmt.Fprintln(os.Stderr, "json get: --file and --field are required")
		return 2
	}
	content, err := os.ReadFile(*file)
	if err != nil {
		return 1
	}
	var value any
	if err := json.Unmarshal(content, &value); err != nil {
		return 1
	}
	for _, key := range strings.Split(*field, ".") {
		object, ok := value.(map[string]any)
		if !ok {
			return 1
		}
		value, ok = object[key]
		if !ok {
			return 1
		}
	}
	switch typed := value.(type) {
	case string:
		fmt.Println(typed)
	case float64:
		// Integers print without a decimal point, matching python's
		// json round-trip for whole numbers stored as ints.
		if typed == float64(int64(typed)) {
			fmt.Println(int64(typed))
		} else {
			fmt.Println(typed)
		}
	case bool:
		fmt.Println(typed)
	case nil:
		fmt.Println("null")
	default:
		encoded, err := json.Marshal(typed)
		if err != nil {
			return 1
		}
		fmt.Println(string(encoded))
	}
	return 0
}
