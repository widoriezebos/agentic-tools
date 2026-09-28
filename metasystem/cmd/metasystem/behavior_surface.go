package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/behaviorsurface"
)

func runBehaviorSurfaceSelect(args []string) int {
	flags := flag.NewFlagSet("behavior-surface select", flag.ContinueOnError)
	projectionName := flags.String("projection", "", "ENGINE, LANDING, or PAYLOAD")
	prefix := flags.String("prefix", "", "metasystem prefix relative to the Git toplevel")
	nul := flags.Bool("nul", false, "read and write NUL-terminated paths")
	if flags.Parse(args) != nil || *projectionName == "" || flags.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "usage: metasystem internal behavior-surface select --projection ENGINE|LANDING|PAYLOAD [--prefix PREFIX] [--nul]")
		return 2
	}
	policy, err := behaviorsurface.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	projection, err := behaviorsurface.ParseProjection(*projectionName)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	separator := byte('\n')
	if *nul {
		separator = 0
	}
	reader := bufio.NewReader(os.Stdin)
	writer := bufio.NewWriter(os.Stdout)
	for {
		value, readErr := reader.ReadString(separator)
		if len(value) > 0 {
			if value[len(value)-1] == separator {
				value = value[:len(value)-1]
			}
			included, err := policy.Includes(projection, value, *prefix)
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				return 1
			}
			if included {
				if _, err := writer.WriteString(value); err != nil {
					fmt.Fprintln(os.Stderr, "behavior-surface output:", err)
					return 1
				}
				if err := writer.WriteByte(separator); err != nil {
					fmt.Fprintln(os.Stderr, "behavior-surface output:", err)
					return 1
				}
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			fmt.Fprintln(os.Stderr, readErr)
			return 1
		}
	}
	if err := writer.Flush(); err != nil {
		fmt.Fprintln(os.Stderr, "behavior-surface output:", err)
		return 1
	}
	return 0
}

func runBehaviorSurfaceSkipAllowed(args []string) int {
	flags := flag.NewFlagSet("behavior-surface skip-allowed", flag.ContinueOnError)
	family := flags.String("family", "", "validation family name")
	scopeName := flags.String("scope", "", "WITNESS or DELIVERY proof scope")
	if flags.Parse(args) != nil || *family == "" || *scopeName == "" || flags.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "usage: metasystem internal behavior-surface skip-allowed --scope WITNESS|DELIVERY --family NAME")
		return 2
	}
	scope, err := behaviorsurface.ParseSkipScope(*scopeName)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	policy, err := behaviorsurface.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if !policy.SkipAllowed(scope, *family) {
		fmt.Fprintf(os.Stderr, "behavior-surface policy version %d does not authorize %s skip family %q\n", policy.Version, scope, *family)
		return 3
	}
	return 0
}
