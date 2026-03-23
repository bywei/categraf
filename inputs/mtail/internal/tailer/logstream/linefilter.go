// Copyright 2024 Flashcat. All Rights Reserved.
// This file is available under the Apache license.

package logstream

import (
	"bytes"
)

// LineFilter transforms raw log line bytes before they are converted to a string.
// This operates at the byte level to avoid allocating the full original string,
// which is critical for reducing memory when log lines are very large.
// Returning nil means skip this line entirely.
type LineFilter func(line []byte) []byte

// NewJSONBytesFieldExtractor creates a LineFilter that extracts specified fields
// from a JSON log line at the byte level, without allocating the full string.
// This is much more memory-efficient than parsing at the string level because
// the 50KB original line is never converted to a Go string.
func NewJSONBytesFieldExtractor(fields []string) LineFilter {
	if len(fields) == 0 {
		return nil
	}
	// Pre-build the byte patterns to search for: "fieldName":
	patterns := make([][]byte, len(fields))
	for i, f := range fields {
		patterns[i] = []byte(`"` + f + `":`)
	}
	return func(line []byte) []byte {
		// Quick check: must start with '{'
		trimmed := bytes.TrimSpace(line)
		if len(trimmed) == 0 || trimmed[0] != '{' {
			return line
		}

		var buf bytes.Buffer
		buf.WriteByte('{')
		found := 0

		for _, pattern := range patterns {
			idx := bytes.Index(trimmed, pattern)
			if idx < 0 {
				continue
			}
			if found > 0 {
				buf.WriteByte(',')
			}
			// Extract the key-value pair starting from the quote before field name
			valStart := idx + len(pattern)
			valEnd := findJSONValueEnd(trimmed, valStart)
			if valEnd < 0 {
				continue
			}
			buf.Write(trimmed[idx:valEnd])
			found++
		}

		if found == 0 {
			return line
		}
		buf.WriteByte('}')
		return buf.Bytes()
	}
}

// findJSONValueEnd finds the end position of a JSON value starting at pos.
// Handles strings, numbers, booleans, null, objects, and arrays.
func findJSONValueEnd(data []byte, pos int) int {
	if pos >= len(data) {
		return -1
	}
	// Skip whitespace
	for pos < len(data) && (data[pos] == ' ' || data[pos] == '\t' || data[pos] == '\n' || data[pos] == '\r') {
		pos++
	}
	if pos >= len(data) {
		return -1
	}

	switch data[pos] {
	case '"':
		// String value: find closing quote (handle escaped quotes)
		for i := pos + 1; i < len(data); i++ {
			if data[i] == '\\' {
				i++ // skip escaped char
				continue
			}
			if data[i] == '"' {
				return i + 1
			}
		}
		return -1
	case '{':
		// Object: find matching closing brace
		return findMatchingBrace(data, pos, '{', '}')
	case '[':
		// Array: find matching closing bracket
		return findMatchingBrace(data, pos, '[', ']')
	default:
		// Number, boolean, null: scan until delimiter
		for i := pos; i < len(data); i++ {
			if data[i] == ',' || data[i] == '}' || data[i] == ']' ||
				data[i] == ' ' || data[i] == '\t' || data[i] == '\n' || data[i] == '\r' {
				return i
			}
		}
		return len(data)
	}
}

// findMatchingBrace finds the position after the matching closing brace/bracket.
func findMatchingBrace(data []byte, pos int, open, close byte) int {
	depth := 0
	inString := false
	for i := pos; i < len(data); i++ {
		if inString {
			if data[i] == '\\' {
				i++
				continue
			}
			if data[i] == '"' {
				inString = false
			}
			continue
		}
		switch data[i] {
		case '"':
			inString = true
		case open:
			depth++
		case close:
			depth--
			if depth == 0 {
				return i + 1
			}
		}
	}
	return -1
}

// NewMaxLineLengthBytesFilter creates a LineFilter that truncates lines
// exceeding the specified maximum length at the byte level.
func NewMaxLineLengthBytesFilter(maxLen int) LineFilter {
	if maxLen <= 0 {
		return nil
	}
	return func(line []byte) []byte {
		if len(line) > maxLen {
			return line[:maxLen]
		}
		return line
	}
}

// ChainLineFilters combines multiple LineFilters into one.
func ChainLineFilters(filters ...LineFilter) LineFilter {
	var active []LineFilter
	for _, f := range filters {
		if f != nil {
			active = append(active, f)
		}
	}
	if len(active) == 0 {
		return nil
	}
	if len(active) == 1 {
		return active[0]
	}
	return func(line []byte) []byte {
		for _, f := range active {
			line = f(line)
			if line == nil {
				return nil
			}
		}
		return line
	}
}
