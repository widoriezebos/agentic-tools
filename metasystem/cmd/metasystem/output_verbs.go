package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/output"
)

func runOutputPrune(args []string) int {
	flags := flag.NewFlagSet("output prune", flag.ContinueOnError)
	root := pathFlag(flags, "root", "", "control root")
	olderThanText := flags.String("older-than", "7d", "minimum file age")
	if flags.Parse(args) != nil {
		return 2
	}
	if *root == "" || flags.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "usage: metasystem internal output prune --root ROOT [--older-than 7d]")
		return 2
	}
	olderThan, err := parseOutputDuration(*olderThanText)
	if err != nil || olderThan < 0 {
		if err == nil {
			err = fmt.Errorf("must not be negative")
		}
		fmt.Fprintln(os.Stderr, "metasystem internal output prune: invalid --older-than:", err)
		return 2
	}
	absoluteRoot, err := filepath.Abs(*root)
	if err != nil {
		fmt.Fprintln(os.Stderr, "metasystem internal output prune:", err)
		return 1
	}
	removed, err := output.Prune(absoluteRoot, olderThan, time.Now().UTC())
	if err != nil {
		fmt.Fprintln(os.Stderr, "metasystem internal output prune:", err)
		return 1
	}
	for _, path := range removed {
		fmt.Println("pruned", path)
	}
	return 0
}

func parseOutputDuration(value string) (time.Duration, error) {
	if strings.HasSuffix(value, "d") {
		days, err := strconv.ParseInt(strings.TrimSuffix(value, "d"), 10, 64)
		if err != nil {
			return 0, err
		}
		const day = 24 * time.Hour
		if days > int64(^uint64(0)>>1)/int64(day) || days < -int64(^uint64(0)>>1)/int64(day) {
			return 0, fmt.Errorf("duration out of range")
		}
		return time.Duration(days) * day, nil
	}
	return time.ParseDuration(value)
}
