package evidence

import (
	"bufio"
	"compress/gzip"
	"encoding/json"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/diskstore"
)

// verdictLineMax bounds one fact line; verdictFactsMax the facts kept.
const (
	verdictLineMax  = 300
	verdictFactsMax = 200
)

// failureFacts reads a bundle's failure facts from every member: plain,
// gzipped (inflated) and blob-backed (read from the host's store); a
// packed .git is not read. Facts are test2json fail events and their
// failure output, "--- FAIL:" lines with the _test.go lines that follow,
// "FAIL <package>" lines, panics and exit lines, each once, in order.
func failureFacts(bundle string, blobs diskstore.BlobStore) ([]string, error) {
	var facts []string
	seen := map[string]bool{}
	add := func(line string) {
		line = strings.TrimRight(line, "\r\n")
		if len(line) > verdictLineMax {
			line = line[:verdictLineMax]
		}
		if line == "" || seen[line] || len(facts) >= verdictFactsMax {
			return
		}
		seen[line] = true
		facts = append(facts, line)
	}
	_, recipes, _, err := diskstore.ReadDistilled(bundle)
	if err != nil {
		return nil, err
	}
	err = filepath.WalkDir(bundle, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel := filepath.ToSlash(strings.TrimPrefix(path, bundle+string(filepath.Separator)))
		switch {
		case entry.IsDir(), !entry.Type().IsRegular(), rel == diskstore.DistilledName, rel == diskstore.OwnerFileName,
			strings.HasSuffix(rel, ".git.tar.gz"), rel == diskstore.VerdictName, rel == diskstore.CompactTombstoneName:
			return nil
		}
		reader, closeReader, err := openMember(path)
		if err != nil {
			return nil // an unreadable member contributes nothing
		}
		scanFacts(reader, add)
		closeReader()
		return nil
	})
	if err != nil {
		return nil, err
	}
	for _, recipe := range recipes {
		if recipe.Kind != diskstore.RecipeBlob {
			continue
		}
		file, err := os.Open(blobs.Path(recipe.SHA256))
		if err != nil {
			continue
		}
		scanFacts(file, add)
		file.Close()
	}
	return facts, nil
}

func openMember(path string) (io.Reader, func(), error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}
	if !strings.HasSuffix(path, ".gz") {
		return file, func() { file.Close() }, nil
	}
	inflated, err := gzip.NewReader(file)
	if err != nil {
		file.Close()
		return nil, nil, err
	}
	return inflated, func() { inflated.Close(); file.Close() }, nil
}

func scanFacts(reader io.Reader, add func(string)) {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 64*1024), 4<<20)
	afterFail := false
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "{") && strings.Contains(line, `"Action"`) {
			var event struct {
				Action, Package, Test, Output string
			}
			if json.Unmarshal([]byte(line), &event) == nil {
				switch {
				case event.Action == "fail" && event.Test != "":
					add("FAIL " + event.Package + " " + event.Test)
				case event.Action == "fail":
					add("FAIL " + event.Package)
				case event.Action == "output" && isFailureLine(event.Output):
					add(strings.TrimSpace(event.Output))
				}
				continue
			}
		}
		trimmed := strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(trimmed, "--- FAIL:"):
			afterFail = true
			add(trimmed)
		case afterFail && strings.Contains(trimmed, "_test.go:"):
			add(trimmed)
		case isFailureLine(line):
			afterFail = false
			add(trimmed)
		default:
			if !strings.HasPrefix(line, " ") && !strings.HasPrefix(line, "\t") {
				afterFail = false
			}
		}
	}
}

func isFailureLine(line string) bool {
	trimmed := strings.TrimSpace(line)
	return strings.HasPrefix(trimmed, "--- FAIL:") || strings.HasPrefix(trimmed, "FAIL\t") || strings.HasPrefix(trimmed, "FAIL ") ||
		strings.HasPrefix(trimmed, "panic:") || strings.HasPrefix(trimmed, "exit status ") || strings.Contains(trimmed, "_test.go:") && strings.Contains(trimmed, "Error")
}
