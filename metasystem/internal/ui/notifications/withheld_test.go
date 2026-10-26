package notifications

import (
	"strings"
	"testing"
)

func TestWithheldNeverReachesTheInterface(t *testing.T) {
	t.Parallel()
	kept := strings.Replace(line(t, "02", "alert", "a seat stopped", false), `"delivered":false`, `"delivered":false,"withheld":"seat-idle"`, 1)
	path := journal(t, line(t, "01", "alert", "a goal landed", true))
	page, err := Page(path, DefaultLimit, "")
	if err != nil {
		t.Fatal(err)
	}
	follower, behind, err := Open(path, "01")
	if err != nil {
		t.Fatal(err)
	}
	appendLine(t, path, kept+"\n")
	appendLine(t, path, line(t, "03", "alert", "another delivery", true)+"\n")
	next, err := follower.Next()
	if err != nil {
		t.Fatal(err)
	}
	all, err := Page(path, DefaultLimit, "")
	if err != nil {
		t.Fatal(err)
	}
	equal(t, "the page", ids(page), []string{"01"})
	equal(t, "what the stream resumes", ids(behind), []string{})
	equal(t, "what the stream follows", ids(next), []string{"03"})
	equal(t, "the page after", ids(all), []string{"03", "01"})
}
