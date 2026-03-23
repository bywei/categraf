// Copyright 2024 Flashcat. All Rights Reserved.
// This file is available under the Apache license.

package runtime

import (
	"encoding/json"
	"strings"

	"flashcat.cloud/categraf/inputs/mtail/internal/logline"
)

// LinePreprocessor defines a function that transforms a log line before it reaches the VM.
// Returning an empty string causes the line to be skipped entirely.
type LinePreprocessor func(line string) string

// NewJSONFieldExtractor creates a LinePreprocessor that extracts only the specified
// fields from a JSON log line and reconstructs a smaller JSON string.
// This dramatically reduces memory usage when log lines are large but only a few
// fields are needed for metric extraction.
func NewJSONFieldExtractor(fields []string) LinePreprocessor {
	if len(fields) == 0 {
		return nil
	}
	fieldSet := make(map[string]struct{}, len(fields))
	for _, f := range fields {
		fieldSet[strings.TrimSpace(f)] = struct{}{}
	}
	return func(line string) string {
		// Quick check: if it doesn't look like JSON, return as-is
		trimmed := strings.TrimSpace(line)
		if len(trimmed) == 0 || (trimmed[0] != '{' && trimmed[0] != '[') {
			return line
		}

		var raw map[string]json.RawMessage
		if err := json.Unmarshal([]byte(trimmed), &raw); err != nil {
			// Not valid JSON, return original line
			return line
		}

		filtered := make(map[string]json.RawMessage, len(fieldSet))
		for field := range fieldSet {
			if val, ok := raw[field]; ok {
				filtered[field] = val
			}
		}

		if len(filtered) == 0 {
			return line
		}

		result, err := json.Marshal(filtered)
		if err != nil {
			return line
		}
		return string(result)
	}
}

// NewMaxLineLengthTrimmer creates a LinePreprocessor that truncates lines
// exceeding the specified maximum length.
func NewMaxLineLengthTrimmer(maxLen int) LinePreprocessor {
	if maxLen <= 0 {
		return nil
	}
	return func(line string) string {
		if len(line) > maxLen {
			return line[:maxLen]
		}
		return line
	}
}

// ChainPreprocessors combines multiple LinePreprocessors into one.
// They are applied in order. If any returns empty string, the line is skipped.
func ChainPreprocessors(preprocessors ...LinePreprocessor) LinePreprocessor {
	var active []LinePreprocessor
	for _, p := range preprocessors {
		if p != nil {
			active = append(active, p)
		}
	}
	if len(active) == 0 {
		return nil
	}
	if len(active) == 1 {
		return active[0]
	}
	return func(line string) string {
		for _, p := range active {
			line = p(line)
			if line == "" {
				return ""
			}
		}
		return line
	}
}

// preprocessLogLine applies the preprocessor to a LogLine, returning a new LogLine
// with the transformed content. Returns nil if the line should be skipped.
func preprocessLogLine(ll *logline.LogLine, preprocessor LinePreprocessor) *logline.LogLine {
	if preprocessor == nil {
		return ll
	}
	newLine := preprocessor(ll.Line)
	if newLine == "" {
		return nil
	}
	if newLine == ll.Line {
		return ll
	}
	return logline.New(ll.Context, ll.Filename, newLine)
}
