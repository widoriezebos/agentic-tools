package main

import "strings"

// environment is the per-invocation environment the gate hands its child
// tools and the build it runs in process. It is a value the gate threads
// through its stages, never the process environment: a nested gate run (the
// witness consumer's frozen export, the arming controller's snapshot) takes a
// copy with its own overrides, exactly as a child shell once inherited an
// exported environment and discarded it on exit.
type environment struct {
	entries []string
}

func newEnvironment(entries []string) *environment {
	return &environment{entries: append([]string(nil), entries...)}
}

func (e *environment) lookup(name string) (string, bool) {
	prefix := name + "="
	value, found := "", false
	for _, entry := range e.entries {
		if strings.HasPrefix(entry, prefix) {
			value, found = entry[len(prefix):], true
		}
	}
	return value, found
}

func (e *environment) get(name string) string {
	value, _ := e.lookup(name)
	return value
}

// set replaces every entry for name with one entry holding value.
func (e *environment) set(name, value string) {
	e.unset(name)
	e.entries = append(e.entries, name+"="+value)
}

func (e *environment) unset(names ...string) {
	kept := e.entries[:0:0]
	for _, entry := range e.entries {
		drop := false
		for _, name := range names {
			if strings.HasPrefix(entry, name+"=") {
				drop = true
				break
			}
		}
		if !drop {
			kept = append(kept, entry)
		}
	}
	e.entries = kept
}

func (e *environment) clone() *environment {
	return newEnvironment(e.entries)
}

// with returns a copy carrying the NAME=VALUE overrides.
func (e *environment) with(pairs ...string) *environment {
	copied := e.clone()
	for _, pair := range pairs {
		name, value, _ := strings.Cut(pair, "=")
		copied.set(name, value)
	}
	return copied
}

func (e *environment) list() []string {
	return append([]string(nil), e.entries...)
}
