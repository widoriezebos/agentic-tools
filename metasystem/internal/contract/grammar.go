package contract

import "regexp"

// The mission contract document's block grammar. It lives with the contract
// because it IS contract grammar: were it to sit in the mission state
// file, every reader of a contract document would become a reader of
// mission's internals.

var authoredBlockRe = regexp.MustCompile("(?ms)^```mission[ \t]*\n(.*?)^```[ \t]*$")
var sealBlockRe = regexp.MustCompile("(?ms)^```mission-seal[ \t]*\n(.*?)^```[ \t]*$")

// AuthoredBlocks returns every authored mission block in a contract
// document, each as a submatch slice whose element 1 is the block body.
// Callers decide what "not exactly one" means to them and keep their own
// refusal text (error text is contract).
// ParseAuthoredValues parses one authored block's key=value lines under
// THE contract grammar — strict: every line key=value, no empty or
// whitespace-padded keys or values, no duplicates. Exported so
// mission-side consumers cannot grow lax copies that accept padded
// keys the seal-time parser refuses; any
// signed contract already satisfies the strict grammar, so consolidation
// tightens nothing a valid mission can carry.
func ParseAuthoredValues(block, label string) (map[string]string, error) {
	return contractParseKeyValues(block, label)
}

func AuthoredBlocks(text string) [][]string {
	return authoredBlockRe.FindAllStringSubmatch(text, -1)
}

// SealBlock returns the seal block's submatch slice, or nil when the
// document carries none. Element 1 is the block body.
func SealBlock(text string) []string {
	return sealBlockRe.FindStringSubmatch(text)
}
