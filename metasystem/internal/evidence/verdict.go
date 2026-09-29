package evidence

import (
	"bufio"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
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
		// Every member must be read completely, or the verdict could miss
		// what failed (Round B2-2, R4).
		reader, closeReader, err := openMember(path)
		if err != nil {
			return fmt.Errorf("member %s cannot be read: %w", rel, err)
		}
		scanErr := scanFacts(reader, add)
		closeReader()
		if scanErr != nil {
			return fmt.Errorf("member %s cannot be read completely: %w", rel, scanErr)
		}
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
			return nil, fmt.Errorf("the blob of %s cannot be read: %w", recipe.Path, err)
		}
		scanErr := scanFacts(file, add)
		file.Close()
		if scanErr != nil {
			return nil, fmt.Errorf("the blob of %s cannot be read completely: %w", recipe.Path, scanErr)
		}
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

// scanFacts reads every line, however long (no line limit), and returns
// the first read error.
func scanFacts(reader io.Reader, add func(string)) error {
	buffered := bufio.NewReader(reader)
	afterFail := false
	for {
		line, err := buffered.ReadString('\n')
		if len(line) > 0 {
			afterFail = factLine(strings.TrimRight(line, "\r\n"), afterFail, add)
		}
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
	}
}

// factLine adds a line's fact and says whether a failure block continues.
func factLine(line string, afterFail bool, add func(string)) bool {
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
			return afterFail
		}
	}
	trimmed := strings.TrimSpace(line)
	switch {
	case strings.HasPrefix(trimmed, "--- FAIL:"):
		add(trimmed)
		return true
	case afterFail && strings.Contains(trimmed, "_test.go:"):
		add(trimmed)
		return true
	case isFailureLine(line):
		add(trimmed)
		return false
	}
	return afterFail && (strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t"))
}

func isFailureLine(line string) bool {
	trimmed := strings.TrimSpace(line)
	return strings.HasPrefix(trimmed, "--- FAIL:") || strings.HasPrefix(trimmed, "FAIL\t") || strings.HasPrefix(trimmed, "FAIL ") ||
		strings.HasPrefix(trimmed, "panic:") || strings.HasPrefix(trimmed, "exit status ") || strings.Contains(trimmed, "_test.go:") && strings.Contains(trimmed, "Error")
}
