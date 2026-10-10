package goadapter

import (
	"os"
	"path/filepath"
	"testing"
)

func TestTestPackageCountHonorsBuildTagsAndTestFiles(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	for path, source := range map[string]string{
		"go.mod":                     "module example.invalid/count\n\ngo 1.27\n",
		"internaltest/value.go":      "package internaltest\n",
		"internaltest/value_test.go": "package internaltest\nimport \"testing\"\nfunc TestValue(t *testing.T) { t.Parallel() }\n",
		"externaltest/value.go":      "package externaltest\n",
		"externaltest/value_test.go": "package externaltest_test\nimport \"testing\"\nfunc TestValue(t *testing.T) { t.Parallel() }\n",
		"untested/value.go":          "package untested\n",
		"tagged/value.go":            "package tagged\n",
		"tagged/value_test.go":       "//go:build count_one && count_two\n\npackage tagged\nimport \"testing\"\nfunc TestValue(t *testing.T) { t.Parallel() }\n",
	} {
		absolute := filepath.Join(root, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(absolute), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(absolute, []byte(source), 0600); err != nil {
			t.Fatal(err)
		}
	}
	for _, row := range []struct {
		name string
		tags []string
		want int
	}{
		{name: "default", want: 2},
		{name: "one tag", tags: []string{"count_one"}, want: 2},
		{name: "both tags", tags: []string{"count_one", "count_two"}, want: 3},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			count, err := TestPackageCount(root, row.tags)
			if err != nil || count != row.want {
				t.Fatalf("test packages=%d error=%v; want %d", count, err, row.want)
			}
		})
	}
}

func TestTestPackageCountReturnsDiscoveryFailure(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("invalid module declaration\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if count, err := TestPackageCount(root, nil); err == nil || count != 0 {
		t.Fatalf("invalid module: count=%d error=%v; want no count and a discovery error", count, err)
	}
}
