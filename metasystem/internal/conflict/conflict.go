// Package conflict classifies unmerged paths from the original lines each side
// changed. It leaves source resolution to the goal's builder or a person.
package conflict

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

const (
	Generated = "generated"
	Judgement = "judgement"
	Builder   = "builder"
)

type Path struct {
	Path       string `json:"path"`
	Class      string `json:"class"`
	Resolution string `json:"resolution"`
}

type Return struct {
	Main  string `json:"main"`
	Paths []Path `json:"paths"`
}

type Git func(args ...string) (string, error)

// Classify reads index stages, not conflict markers: a common original is
// stage 1, main is stage 2, and the handed-in goal is stage 3.
func Classify(git Git, paths []string, generated func(string) bool) ([]Path, error) {
	result := make([]Path, 0, len(paths))
	for _, name := range paths {
		item := Path{Path: name, Class: Generated}
		if generated(name) {
			result = append(result, item)
			continue
		}
		stages, err := git("ls-files", "--unmerged", "-z", "--", name)
		if err != nil {
			return nil, err
		}
		blobs := map[string]string{}
		for _, record := range strings.Split(stages, "\x00") {
			if record == "" {
				continue
			}
			fields := strings.Fields(strings.SplitN(record, "\t", 2)[0])
			if len(fields) != 3 {
				return nil, fmt.Errorf("cannot read index stages for %s", name)
			}
			blobs[fields[2]] = fields[1]
		}
		item.Class = Judgement
		if blobs["1"] != "" && blobs["2"] != "" && blobs["3"] != "" {
			diffs := [2]string{}
			for i, side := range []string{"2", "3"} {
				diffs[i], err = git("diff", "--no-ext-diff", "--no-textconv", "--unified=0", blobs["1"], blobs[side])
				if err != nil {
					return nil, err
				}
			}
			item.Class, item.Resolution, err = Lines(diffs[0], diffs[1])
			if err != nil {
				return nil, fmt.Errorf("classify %s: %w", name, err)
			}
		}
		result = append(result, item)
	}
	return result, nil
}

// GeneratedBy finds an output using installation-relative declarations while
// Git names paths from the checkout root. Paths outside the installation are sources.
func GeneratedBy(sets []testpolicy.Generated, prefix, name string) bool {
	if !strings.HasPrefix(name, prefix) {
		return false
	}
	name = strings.TrimPrefix(name, prefix)
	for _, set := range sets {
		if set.Generates(name) {
			return true
		}
	}
	return false
}

type span struct{ start, count int }

var hunk = regexp.MustCompile(`^@@ -(\d+)(?:,(\d+))? \+\d+(?:,\d+)? @@`)

func ranges(diff string) ([]span, bool, error) {
	var result []span
	for _, line := range strings.Split(diff, "\n") {
		if strings.HasPrefix(line, "Binary files ") || line == "GIT binary patch" {
			return nil, true, nil
		}
		if !strings.HasPrefix(line, "@@") {
			continue
		}
		fields := hunk.FindStringSubmatch(line)
		if fields == nil {
			return nil, false, fmt.Errorf("cannot read changed original lines: %s", line)
		}
		start, err := strconv.Atoi(fields[1])
		if err != nil {
			return nil, false, err
		}
		count := 1
		if fields[2] != "" {
			count, err = strconv.Atoi(fields[2])
			if err != nil {
				return nil, false, err
			}
		}
		result = append(result, span{start, count})
	}
	return result, false, nil
}

// Lines compares zero-context diffs against the same original. Insertions
// consume no original line, including two insertions at the same location.
func Lines(main, goal string) (string, string, error) {
	a, binaryA, err := ranges(main)
	if err != nil {
		return "", "", err
	}
	b, binaryB, err := ranges(goal)
	if err != nil {
		return "", "", err
	}
	if binaryA || binaryB {
		return Judgement, "", nil
	}
	for _, x := range a {
		for _, y := range b {
			if x.count > 0 && y.count > 0 && x.start < y.start+y.count && y.start < x.start+x.count {
				return Judgement, "", nil
			}
		}
	}
	resolution := ""
	if len(a) == 1 && len(b) == 1 && a[0].count == 0 && b[0].count == 0 && a[0].start == b[0].start {
		resolution = "keep both, main's lines then the goal's"
	}
	return Builder, resolution, nil
}
