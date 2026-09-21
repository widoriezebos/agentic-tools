package web

import (
	"mime"
	"strconv"
	"strings"
)

// AcceptsHTML reports whether the request asks for HTML by name. A browser
// navigation does; a script fetching a missing asset does not, and neither
// does a bare curl. Only an exact text/html counts: "*/*" and "text/*" are
// what a client sends when it will take anything, and answering those with the
// page turns every typo into a 200.
func AcceptsHTML(accept string) bool {
	for _, element := range strings.Split(accept, ",") {
		mediaType, parameters, err := mime.ParseMediaType(strings.TrimSpace(element))
		if err != nil || mediaType != "text/html" {
			continue
		}
		quality, present := parameters["q"]
		if !present {
			return true
		}
		// "q=0" is a client saying it will not take HTML; anything that does
		// not parse is not a weight this rule can act on.
		if weight, err := strconv.ParseFloat(quality, 64); err == nil && weight > 0 {
			return true
		}
	}
	return false
}
