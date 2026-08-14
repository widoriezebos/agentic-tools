package main

import (
	"errors"
	"flag"
	"fmt"
	"os"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/jsonedit"
)

// The json family relays: the shell-facing rendering and edit decisions
// live in internal/jsonedit (review architecture-2); these verbs parse
// flags, read files, and print.

// runJSONSet reads a JSON object file, sets the given top-level fields, and
// writes it back atomically (indented, key-sorted). --int fields parse as
// integers; --field fields stay strings. For fixture and maintenance edits.
func runJSONSet(args []string) int {
	flags := flag.NewFlagSet("json set", flag.ContinueOnError)
	file := flags.String("file", "", "JSON object file to edit in place")
	var stringFields, intFields []string
	flags.Func("field", "KEY=VALUE string field to set (repeatable)", func(v string) error {
		stringFields = append(stringFields, v)
		return nil
	})
	flags.Func("int", "KEY=VALUE integer field to set (repeatable)", func(v string) error {
		intFields = append(intFields, v)
		return nil
	})
	if flags.Parse(args) != nil {
		return 2
	}
	if *file == "" || (len(stringFields) == 0 && len(intFields) == 0) {
		fmt.Fprintln(os.Stderr, "json set: --file and at least one --field/--int are required")
		return 2
	}
	data, err := os.ReadFile(*file)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	object, err := jsonedit.SetFields(data, stringFields, intFields)
	if err != nil {
		if errors.Is(err, jsonedit.ErrUsage) {
			fmt.Fprintf(os.Stderr, "json set: %v\n", err)
			return 2
		}
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if err := writeIdentityJSON(*file, object); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}

// runJSONObject builds a compact JSON object from key=value arguments (string
// values, split on the first '='), printed without HTML escaping. For shell
// callers that need to construct a small JSON object from strings.
func runJSONObject(args []string) int {
	line, err := jsonedit.Object(args)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	fmt.Println(line)
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
	var defValue *string
	flags.Visit(func(f *flag.Flag) {
		if f.Name == "default" {
			defValue = def
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
	out, ok := jsonedit.Get(content, *field, defValue)
	if !ok {
		return 1
	}
	fmt.Println(out)
	return 0
}
