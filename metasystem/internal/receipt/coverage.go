package receipt

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Retro coverage (design engine-owns-disk-lifetimes Part B 3.12 clause 2,
// DL4C-03, DL4D-11): the retro records exactly which ledger lines it read.
// A line's digest is the first twelve hex digits of its sha256; a RETRO row
// with covered=<digests> covers those lines and nothing else, so a merged
// ledger, a delayed line or clock skew never misclassifies one. The reader
// prints, from one snapshot, the lines no RETRO row covers and a token
// "<count>:<sha12 of their digests in order>"; the marker verifies the
// token against its own snapshot's first count uncovered lines before it
// writes anything. A RETRO row without covered= covers nothing.

// LineDigest is one ledger line's digest.
func LineDigest(line string) string {
	sum := sha256.Sum256([]byte(strings.TrimRight(line, "\r\n")))
	return hex.EncodeToString(sum[:])[:12]
}

// Coverage is one snapshot's uncovered lines.
type Coverage struct {
	Lines   []string `json:"lines"`
	Digests []string `json:"digests"`
	Token   string   `json:"token"`
}

// CoverageOf reads the uncovered lines of a ledger's text, in ledger order.
func CoverageOf(data string) Coverage {
	lines := readLines(data)
	covered := map[string]bool{}
	for _, line := range lines {
		if !strings.Contains(line, "|RETRO|") {
			continue
		}
		for _, field := range strings.Split(line, "|") {
			if list, ok := strings.CutPrefix(field, "covered="); ok {
				for _, digest := range strings.Split(list, ",") {
					covered[strings.TrimSpace(digest)] = true
				}
			}
		}
	}
	coverage := Coverage{Lines: []string{}, Digests: []string{}}
	for _, line := range lines {
		if strings.Contains(line, "|RETRO|") {
			continue
		}
		if digest := LineDigest(line); !covered[digest] {
			coverage.Lines = append(coverage.Lines, line)
			coverage.Digests = append(coverage.Digests, digest)
		}
	}
	coverage.Token = CoverageToken(coverage.Digests)
	return coverage
}

// CoverageToken is "<count>:<sha12 of the digests joined by newlines>".
func CoverageToken(digests []string) string {
	sum := sha256.Sum256([]byte(strings.Join(digests, "\n")))
	return fmt.Sprintf("%d:%s", len(digests), hex.EncodeToString(sum[:])[:12])
}

// Uncovered implements `receipt status --uncovered`: the lines no retro
// covered, in ledger order, and the token the marker takes.
func Uncovered(opts Options) Result {
	data, err := os.ReadFile(opts.File)
	if err != nil && !os.IsNotExist(err) {
		return fail(2, "cannot read receipt file: %v", err)
	}
	coverage := CoverageOf(string(data))
	if opts.JSON {
		encoded, err := json.Marshal(coverage)
		if err != nil {
			return fail(2, "%v", err)
		}
		return ok(string(encoded))
	}
	out := append([]string(nil), coverage.Lines...)
	out = append(out, fmt.Sprintf("%d line(s) no retro has covered; token %s (metasystem receipt retro SUMMARY --covered %s records them)",
		len(coverage.Lines), coverage.Token, coverage.Token))
	return ok(out...)
}

// coveredDigests checks a token against the ledger's uncovered lines now and
// returns the digests it covers: the first count of them, in order.
func coveredDigests(file, token string) ([]string, *Result) {
	count, _, found := strings.Cut(token, ":")
	number, err := strconv.Atoi(count)
	if !found || err != nil || number < 0 {
		result := fail(2, "--covered takes the token metasystem receipt status --uncovered prints (<count>:<digest>), got %q; nothing was recorded", token)
		return nil, &result
	}
	data, err := os.ReadFile(file)
	if err != nil && !os.IsNotExist(err) {
		result := fail(2, "cannot read receipt file: %v", err)
		return nil, &result
	}
	coverage := CoverageOf(string(data))
	if number > len(coverage.Digests) || CoverageToken(coverage.Digests[:number]) != token {
		result := fail(1, "the uncovered set changed since the read; run `metasystem receipt status --uncovered` again; nothing was recorded")
		return nil, &result
	}
	return coverage.Digests[:number], nil
}
