// Package cliflags builds the engine's flag sets. A parse error is answered
// in the public style, naming the command, the option it does not take and
// the options it takes ("metasystem internal config get: does not take --x;
// it takes --key, --mode; nothing was done"), never Go's raw "flag provided
// but not defined" with a usage dump. A help request prints the command's
// options on standard output.
package cliflags

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"sync/atomic"
)

// helpAnswered counts help requests answered with a usage.
var helpAnswered atomic.Int64

// HelpAnswered is how many help requests this process has answered with a
// usage; a caller compares it before and after a parse to tell a help
// request from a refusal.
func HelpAnswered() int64 { return helpAnswered.Load() }

// New returns a flag set named name whose errors read as label's (the
// command as a person types it) and are written to errs (standard error at
// the time of the error when errs is nil).
func New(name, label string, errs io.Writer) *flag.FlagSet {
	flags := flag.NewFlagSet(name, flag.ContinueOnError)
	writer := &errorWriter{flags: flags, command: label, out: errs}
	flags.SetOutput(writer)
	flags.Usage = func() {
		if writer.errored {
			return
		}
		fmt.Fprintf(os.Stdout, "usage: %s [options]\n", label)
		flags.SetOutput(os.Stdout)
		flags.PrintDefaults()
		flags.SetOutput(writer)
		helpAnswered.Add(1)
	}
	return flags
}

// errorWriter rewrites the flag package's parse errors.
type errorWriter struct {
	flags   *flag.FlagSet
	command string
	out     io.Writer
	errored bool
}

func (w *errorWriter) Write(p []byte) (int, error) {
	out := w.out
	if out == nil {
		out = os.Stderr
	}
	w.errored = true
	for _, line := range strings.Split(strings.TrimRight(string(p), "\n"), "\n") {
		switch {
		case strings.HasPrefix(line, "flag provided but not defined: -"):
			name := strings.TrimPrefix(line, "flag provided but not defined: -")
			fmt.Fprintf(out, "%s: does not take --%s; %s; nothing was done\n", w.command, strings.TrimPrefix(name, "-"), Takes(w.flags))
		case strings.HasPrefix(line, "flag needs an argument: -"):
			name := strings.TrimPrefix(line, "flag needs an argument: -")
			fmt.Fprintf(out, "%s: --%s needs a value; nothing was done\n", w.command, strings.TrimPrefix(name, "-"))
		case strings.HasPrefix(line, "invalid value "), strings.HasPrefix(line, "invalid boolean value "):
			fmt.Fprintf(out, "%s: %s; nothing was done\n", w.command, strings.Replace(line, " for flag -", " for --", 1))
		case line == "":
		default:
			fmt.Fprintf(out, "%s: %s\n", w.command, line)
		}
	}
	return len(p), nil
}

// Takes names a flag set's options: "it takes --a, --b" or "it takes no
// options".
func Takes(flags *flag.FlagSet) string {
	var names []string
	flags.VisitAll(func(f *flag.Flag) { names = append(names, "--"+f.Name) })
	if len(names) == 0 {
		return "it takes no options"
	}
	return "it takes " + strings.Join(names, ", ")
}
