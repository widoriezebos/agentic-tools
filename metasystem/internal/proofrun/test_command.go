package proofrun

import (
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

type junitSuite struct {
	Cases  []junitCase  `xml:"testcase"`
	Suites []junitSuite `xml:"testsuite"`
}

type junitCase struct {
	Classname string `xml:"classname,attr"`
	Name      string `xml:"name,attr"`
	Failure   *struct {
		Message string `xml:"message,attr"`
	} `xml:"failure"`
	Error *struct {
		Message string `xml:"message,attr"`
	} `xml:"error"`
	Skipped *struct {
		Message string `xml:"message,attr"`
	} `xml:"skipped"`
}

func parseJUnit(root string, group testpolicy.Group) (observed, missing, unexpected []NativeTestIdentity, complete bool, digests map[string]string, err error) {
	digests = map[string]string{}
	expected := map[string]testpolicy.ExpectedTest{}
	for _, test := range group.ExpectedTests {
		expected[junitKey(test.Report, test.Classname, test.Name)] = test
	}
	seen := map[string]bool{}
	for _, report := range group.Reports {
		matched, globErr := filepath.Glob(filepath.Join(root, filepath.FromSlash(report), "*.xml"))
		if globErr != nil {
			return nil, nil, nil, false, nil, globErr
		}
		for _, path := range matched {
			data, readErr := os.ReadFile(path)
			if readErr != nil {
				return nil, nil, nil, false, nil, readErr
			}
			relative, _ := filepath.Rel(root, path)
			relative = filepath.ToSlash(relative)
			digests[relative] = digestBytes(data)
			var suite junitSuite
			if xml.Unmarshal(data, &suite) != nil {
				return nil, nil, nil, false, nil, fmt.Errorf("parse JUnit report %s", relative)
			}
			var cases []junitCase
			collectJUnitCases(suite, &cases)
			for _, test := range cases {
				key := junitKey(relative, test.Classname, test.Name)
				status, reason := "passed", ""
				if test.Skipped != nil {
					status, reason = "skipped", test.Skipped.Message
				} else if test.Failure != nil {
					status, reason = "failed", test.Failure.Message
				} else if test.Error != nil {
					status, reason = "failed", test.Error.Message
				}
				identity := NativeTestIdentity{Report: relative, Classname: test.Classname, Name: test.Name, Status: status, Reason: reason}
				if seen[key] {
					unexpected = append(unexpected, NativeTestIdentity{Report: relative, Classname: test.Classname, Name: test.Name, Status: "duplicate"})
					continue
				}
				seen[key] = true
				if _, ok := expected[key]; !ok {
					unexpected = append(unexpected, identity)
				} else {
					observed = append(observed, identity)
				}
			}
		}
	}
	for key, test := range expected {
		if !seen[key] {
			missing = append(missing, NativeTestIdentity{Report: test.Report, Classname: test.Classname, Name: test.Name, Status: "missing"})
		}
	}
	sortNative(observed)
	sortNative(missing)
	sortNative(unexpected)
	complete = len(observed) > 0 && len(missing) == 0 && len(unexpected) == 0
	return
}

func collectJUnitCases(suite junitSuite, result *[]junitCase) {
	*result = append(*result, suite.Cases...)
	for _, nested := range suite.Suites {
		collectJUnitCases(nested, result)
	}
}
func junitKey(report, classname, name string) string {
	return report + "\x00" + classname + "\x00" + name
}
func sortNative(values []NativeTestIdentity) {
	sort.Slice(values, func(i, j int) bool {
		return junitKey(values[i].Report, values[i].Classname, values[i].Name) < junitKey(values[j].Report, values[j].Classname, values[j].Name)
	})
}
