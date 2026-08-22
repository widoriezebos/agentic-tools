package validate

import (
	"fmt"
	"strconv"
	"strings"
)

// The ported message dialect: how decoded JSON renders inside gate refusal
// texts — 'single-quoted' strings, None for null, integral floats without a
// decimal point. The dialect is a fixture-asserted message contract, and
// this file is its one home.

// reprValue is the dialect core every renderer shares. Strings are
// single-quoted when quote is set, bare otherwise. Values outside the core
// go to rest, where the gates deliberately differ: the conformance gate
// renders True/False bools and 'g' floats (conformanceRest), the
// return-completeness gate renders raw JSON bytes.
func reprValue(value any, quote bool, rest func(any) string) string {
	switch v := value.(type) {
	case nil:
		return "None"
	case string:
		if quote {
			return "'" + v + "'"
		}
		return v
	case float64:
		if v == float64(int64(v)) {
			return strconv.FormatInt(int64(v), 10)
		}
	}
	return rest(value)
}

// conformanceRest completes the conformance gate's dialect over the core:
// Python-style booleans, 'g'-formatted floats, %v for anything else.
func conformanceRest(value any) string {
	switch v := value.(type) {
	case bool:
		if v {
			return "True"
		}
		return "False"
	case float64:
		return strconv.FormatFloat(v, 'g', -1, 64)
	}
	return fmt.Sprintf("%v", value)
}

// quoted formats a decoded JSON value for a refusal message with quoted
// strings; scalarText renders the same dialect with strings bare.
func quoted(value any) string { return reprValue(value, true, conformanceRest) }

func scalarText(value any) string { return reprValue(value, false, conformanceRest) }

func quotedList(items []string) string {
	if len(items) == 0 {
		return "[]"
	}
	quoted := make([]string, len(items))
	for i, item := range items {
		quoted[i] = "'" + item + "'"
	}
	return "[" + strings.Join(quoted, ", ") + "]"
}
