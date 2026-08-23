package mission

// The guardrail class: the contract-declared set of files that ARE an
// app's net — specs, golden data, gate scripts, budgets. Shared home so
// the wall's consumption check and authorization issuance parse ONE
// grammar and can never disagree about what is guardrail-classed.

import (
	"fmt"
	gopath "path"
	"sort"
	"strings"
)

// GuardrailClass is the contract's declared guardrail set: the files
// that ARE the app's net — specs, golden data, gate scripts, budgets.
// Exact files and directory prefixes (a trailing slash), because a net
// is usually a tree. A change touching this class never rides an
// ordinary implementation authorization: it takes the warden-reviewed
// lane, so the net cannot be quietly weakened by the work it judges.
type GuardrailClass struct {
	files    map[string]bool
	prefixes []string
}

func (g *GuardrailClass) Empty() bool {
	return g == nil || (len(g.files) == 0 && len(g.prefixes) == 0)
}

// covers reports whether a repository-relative path is guardrail-classed.
func (g *GuardrailClass) Covers(path string) bool {
	if g == nil {
		return false
	}
	if g.files[path] {
		return true
	}
	for _, prefix := range g.prefixes {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}
	return false
}

// parseGuardrails parses the contract's wall.guardrails declaration:
// comma-separated canonical repository-relative files, or directory
// prefixes marked by a trailing slash — no globs, no traversal, no
// protected paths (those are already denied to every host artifact and
// custodied by the wall itself). The returned violation string is a
// wall refusal, exactly like the host-artifact parser's: a contract
// declaring an unlawful guardrail set fails the equation, never the
// process.
func ParseGuardrails(value string, protected func(string) bool) (*GuardrailClass, string) {
	class := &GuardrailClass{files: map[string]bool{}}
	if strings.TrimSpace(value) == "" {
		return class, ""
	}
	for _, raw := range strings.Split(value, ",") {
		path := strings.TrimSpace(raw)
		switch {
		case path == "":
			return nil, "contract wall.guardrails declares an empty path"
		case strings.HasPrefix(path, "/"), containsDotDotSegment(path), strings.Contains(path, "\\"):
			return nil, fmt.Sprintf("contract wall.guardrails path %q is not a canonical repository-relative path", path)
		case strings.ContainsAny(path, "*?["):
			return nil, fmt.Sprintf("contract wall.guardrails path %q is a glob; only exact files and directory prefixes may be declared", path)
		}
		// Coverage compares literal canonical Git paths, so a declaration
		// that is not already canonical (dot segments, doubled or
		// trailing-doubled separators) would silently cover nothing.
		base := strings.TrimSuffix(path, "/")
		if base == "" || base == "." || gopath.Clean(base) != base {
			return nil, fmt.Sprintf("contract wall.guardrails path %q is not canonical; declare the exact repository-relative form", path)
		}
		if protected != nil && (protected(path) || protected(strings.TrimSuffix(path, "/"))) {
			return nil, fmt.Sprintf("contract wall.guardrails declares the protected path %q, which the wall already custodies", path)
		}
		if strings.HasSuffix(path, "/") {
			class.prefixes = append(class.prefixes, path)
			continue
		}
		class.files[path] = true
	}
	return class, ""
}

// containsDotDotSegment reports whether any slash-separated segment IS
// "..": traversal. A ".." merely inside a filename (v1..v2.json) is a
// lawful canonical name.
func containsDotDotSegment(path string) bool {
	for _, segment := range strings.Split(strings.TrimSuffix(path, "/"), "/") {
		if segment == ".." {
			return true
		}
	}
	return false
}

// guardrailContradiction refuses a contract that declares any path as
// BOTH a host artifact and a guardrail: the first is the host's lawful
// free write, the second is exactly what the host must never touch
// freely. Empty when the two declarations are disjoint.
func GuardrailContradiction(declared map[string]bool, guardrails *GuardrailClass) string {
	for path := range declared {
		if guardrails.Covers(path) {
			return fmt.Sprintf("contract declares the guardrail %s as a host artifact; a guardrail is never a free host write", path)
		}
	}
	return ""
}

// VerifiedGuardrails reads the mission's guardrail class through the
// authenticated path: the live contract's raw bytes are checked against
// the approved digest recorded in the fences before any value is
// trusted. A mission whose contract declares no guardrails returns the
// empty class. A mission whose fences record NO approved digest (an
// unsealed bed, or one predating the sealed-contract discipline) also
// returns the empty class rather than refusing: issuance-time custody
// is defense in depth, and the wall's consumption check — which reads
// the contract through the runner's own verified parse — remains the
// enforcement authority for every record, stamped or not. A digest
// that IS present and does not match the live contract stays a hard
// refusal: that is a tamper signal, never an absence.
func VerifiedGuardrails(repo, missionID string) (*GuardrailClass, error) {
	empty, _ := ParseGuardrails("", nil)
	if _, fencesPath, _ := fencePaths(repo, missionID); !fileExists(fencesPath) {
		return empty, nil
	}
	fences, err := loadFences(repo, missionID)
	if err != nil {
		return nil, fmt.Errorf("guardrail custody cannot read the fences: %v", err)
	}
	if approved, _ := fences["approvedContractSha256"].(string); !sha256HexRe.MatchString(approved) {
		return empty, nil
	}
	values, err := verifiedContractValues(repo, missionID, fences)
	if err != nil {
		return nil, fmt.Errorf("guardrail custody cannot trust the contract: %v", err)
	}
	class, violation := ParseGuardrails(values["wall.guardrails"], nil)
	if violation != "" {
		return nil, fmt.Errorf("guardrail custody refused: %s", violation)
	}
	return class, nil
}

// Entries lists the class's declarations as written: files, then
// directory prefixes (with their trailing slash). Consumers that must
// compare two declared nets — the covenant's against a contract's —
// compare these entries, never re-derived strings.
func (g *GuardrailClass) Entries() []string {
	if g == nil {
		return nil
	}
	out := make([]string, 0, len(g.files)+len(g.prefixes))
	for file := range g.files {
		out = append(out, file)
	}
	out = append(out, g.prefixes...)
	sort.Strings(out)
	return out
}
