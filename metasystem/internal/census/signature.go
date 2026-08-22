// Package census classifies the machine's processes: which processes are
// agent-shaped, ancestor lookups, and signature checks. This file holds the
// runtime-signature classification that decides which processes are
// agent-shaped — the core of census classification, find-ancestor, and
// signature-check.
package census

import (
	"fmt"
	"regexp"
)

// Signature is one runtime's compiled match/exclude patterns. A process
// argv is that runtime iff SOME match pattern hits and NO exclude pattern
// does. The patterns are POSIX ERE as the adapters emit them; Go's RE2
// matches them identically for the anchored word-boundary shapes in use.
type Signature struct {
	Runtime  string
	Matches  []*regexp.Regexp
	Excludes []*regexp.Regexp
}

// CompileSignature builds a Signature from raw ERE pattern strings.
func CompileSignature(runtime string, matches, excludes []string) (Signature, error) {
	sig := Signature{Runtime: runtime}
	for _, pattern := range matches {
		compiled, err := regexp.Compile(pattern)
		if err != nil {
			return Signature{}, fmt.Errorf("runtime %s match pattern %q: %w", runtime, pattern, err)
		}
		sig.Matches = append(sig.Matches, compiled)
	}
	for _, pattern := range excludes {
		compiled, err := regexp.Compile(pattern)
		if err != nil {
			return Signature{}, fmt.Errorf("runtime %s exclude pattern %q: %w", runtime, pattern, err)
		}
		sig.Excludes = append(sig.Excludes, compiled)
	}
	return sig, nil
}

// matches reports whether argv is this runtime: some match hits and no
// exclude does. Exclude wins ties: a shell that merely mentions a
// runtime name in an excluded path is not that runtime.
func (s Signature) matches(argv string) bool {
	excluded := false
	for _, pattern := range s.Excludes {
		if pattern.MatchString(argv) {
			excluded = true
			break
		}
	}
	if excluded {
		return false
	}
	for _, pattern := range s.Matches {
		if pattern.MatchString(argv) {
			return true
		}
	}
	return false
}

// Runtime classifies one argv against an ORDERED signature list, returning
// the first runtime that matches or "" for none. Order is load-bearing: the
// first runtime (in declaration order) that claims an argv wins.
func Runtime(argv string, signatures []Signature) string {
	for _, sig := range signatures {
		if sig.matches(argv) {
			return sig.Runtime
		}
	}
	return ""
}

// Assignment pairs a process index with the runtime it classified as.
type Assignment struct {
	Index   int    `json:"index"`
	Runtime string `json:"runtime"`
}

// Classify assigns runtimes to a batch of argvs, first-match-wins per argv
// in signature order. Only agent-shaped argvs appear in the result; unmatched
// processes are omitted, so the result holds only assigned indices.
func Classify(argvs []string, signatures []Signature) []Assignment {
	var assigned []Assignment
	for index, argv := range argvs {
		if runtime := Runtime(argv, signatures); runtime != "" {
			assigned = append(assigned, Assignment{Index: index, Runtime: runtime})
		}
	}
	return assigned
}
