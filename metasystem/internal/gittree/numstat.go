package gittree

import (
	"bytes"
	"fmt"
	"math"
	"strconv"
)

// ChangedLines counts added and deleted text lines between two supplied trees.
func (w Workspace) ChangedLines(fromTree, toTree string) (int64, error) {
	raw, err := w.git(nil, "diff", "--numstat", "-z", "--no-renames",
		"--no-ext-diff", "--no-textconv", "--no-color", "--ignore-submodules=none",
		fromTree, toTree, "--")
	if err != nil {
		return 0, fmt.Errorf("gittree changed lines: %w", err)
	}
	count, err := changedLinesFromNumstat(raw)
	if err != nil {
		return 0, fmt.Errorf("gittree changed lines: %w", err)
	}
	return count, nil
}

func changedLinesFromNumstat(data []byte) (int64, error) {
	if len(data) == 0 {
		return 0, nil
	}
	if data[len(data)-1] != 0 {
		return 0, fmt.Errorf("numstat output is not NUL-terminated")
	}

	var total int64
	for index, record := range bytes.Split(data[:len(data)-1], []byte{0}) {
		firstTab := bytes.IndexByte(record, '\t')
		if firstTab < 0 {
			return 0, fmt.Errorf("numstat record %d has no added count", index+1)
		}
		secondOffset := bytes.IndexByte(record[firstTab+1:], '\t')
		if secondOffset < 0 {
			return 0, fmt.Errorf("numstat record %d has no deleted count", index+1)
		}
		secondTab := firstTab + 1 + secondOffset
		addedField, deletedField := record[:firstTab], record[firstTab+1:secondTab]
		if len(record[secondTab+1:]) == 0 {
			return 0, fmt.Errorf("numstat record %d has an empty pathname", index+1)
		}

		addedDash, deletedDash := bytes.Equal(addedField, []byte("-")), bytes.Equal(deletedField, []byte("-"))
		if addedDash || deletedDash {
			if !addedDash || !deletedDash {
				return 0, fmt.Errorf("numstat record %d mixes binary and numeric counts", index+1)
			}
			continue
		}

		added, err := nonnegativeDecimal(addedField)
		if err != nil {
			return 0, fmt.Errorf("numstat record %d has invalid added count: %w", index+1, err)
		}
		deleted, err := nonnegativeDecimal(deletedField)
		if err != nil {
			return 0, fmt.Errorf("numstat record %d has invalid deleted count: %w", index+1, err)
		}
		if added > math.MaxInt64-deleted {
			return 0, fmt.Errorf("numstat record %d count overflow", index+1)
		}
		row := added + deleted
		if total > math.MaxInt64-row {
			return 0, fmt.Errorf("numstat total count overflow at record %d", index+1)
		}
		total += row
	}
	return total, nil
}

func nonnegativeDecimal(field []byte) (int64, error) {
	if len(field) == 0 {
		return 0, fmt.Errorf("empty decimal")
	}
	for _, digit := range field {
		if digit < '0' || digit > '9' {
			return 0, fmt.Errorf("%q is not a nonnegative decimal", field)
		}
	}
	value, err := strconv.ParseInt(string(field), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%q is outside int64: %w", field, err)
	}
	return value, nil
}
