package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
)

// runJSONObject builds a compact JSON object from key=value arguments (string
// values, split on the first '='), printed without HTML escaping. For shell
// callers that need to construct a small JSON object from strings.
func runJSONObject(args []string) int {
	object := map[string]any{}
	for _, arg := range args {
		if i := strings.IndexByte(arg, '='); i >= 0 {
			object[arg[:i]] = arg[i+1:]
		}
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(object); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	fmt.Println(strings.TrimRight(buf.String(), "\n"))
	return 0
}

// runJSONGet prints a dotted field from a JSON file, exiting 1 when
// the file or field is absent or unparseable. Scalar values print bare
// (no quotes) and composite values print as compact JSON, because
// dozens of shell call sites string-compare the output.
func runJSONGet(args []string) int {
	flags := flag.NewFlagSet("json get", flag.ContinueOnError)
	file := flags.String("file", "", "JSON file to read")
	value := flags.String("value", "", "JSON string to read (instead of --file)")
	field := flags.String("field", "", "dotted field path (a.b.c)")
	def := flags.String("default", "", "value to print when the field is missing or null (exit 0)")
	if flags.Parse(args) != nil {
		return 2
	}
	defaultSet := false
	flags.Visit(func(f *flag.Flag) {
		if f.Name == "default" {
			defaultSet = true
		}
	})
	if *field == "" || (*file == "") == (*value == "") {
		fmt.Fprintln(os.Stderr, "json get: --field and exactly one of --file or --value are required")
		return 2
	}
	content := []byte(*value)
	if *file != "" {
		read, err := os.ReadFile(*file)
		if err != nil {
			return 1
		}
		content = read
	}
	var current any
	if err := json.Unmarshal(content, &current); err != nil {
		return 1
	}
	for _, key := range strings.Split(*field, ".") {
		object, ok := current.(map[string]any)
		if !ok {
			if defaultSet {
				fmt.Println(*def)
				return 0
			}
			return 1
		}
		current, ok = object[key]
		if !ok {
			// A missing field prints the default when one was given, matching
			// the lenient readers that treat absent and null the same.
			if defaultSet {
				fmt.Println(*def)
				return 0
			}
			return 1
		}
	}
	switch typed := current.(type) {
	case string:
		fmt.Println(typed)
	case float64:
		// Integers print without a decimal point, so whole numbers
		// are emitted without a trailing ".0".
		if typed == float64(int64(typed)) {
			fmt.Println(int64(typed))
		} else {
			fmt.Println(typed)
		}
	case bool:
		fmt.Println(typed)
	case nil:
		if defaultSet {
			fmt.Println(*def)
			return 0
		}
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
