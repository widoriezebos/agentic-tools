package main

import (
	"strconv"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/events"
)

// runEventEmit appends one flight-recorder event. Arguments are key=value
// pairs: root, component, event, summary, pid, pidStartedAt, seq, and any
// number of id/payload fields. Emitting is best-effort and never fails its
// caller, so this always exits 0.
func runEventEmit(args []string) int {
	fields := map[string]string{}
	for _, arg := range args {
		if i := strings.IndexByte(arg, '='); i >= 0 {
			fields[arg[:i]] = arg[i+1:]
		}
	}
	root := popField(fields, "root")
	component := popField(fields, "component")
	event := popField(fields, "event")
	summary := popField(fields, "summary")
	pid := parseInt64(popField(fields, "pid"))
	pidStartedAt := parseInt64(popField(fields, "pidStartedAt"))
	seq := parseInt64(popField(fields, "seq"))
	if seq == 0 {
		seq = 1
	}
	events.EmitOnce(root, component, event, summary, pid, pidStartedAt, seq, fields)
	return 0
}

func popField(fields map[string]string, key string) string {
	v := fields[key]
	delete(fields, key)
	return v
}

func parseInt64(s string) int64 {
	n, _ := strconv.ParseInt(s, 10, 64)
	return n
}
