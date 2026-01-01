package assetmin

import (
	"testing"
)

func TestStripLeadingUseStrictUnit(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "double quotes with semicolon",
			input:    `"use strict";\nconsole.log('A');`,
			expected: `\nconsole.log('A');`,
		},
		{
			name:     "single quotes with semicolon",
			input:    `'use strict';\nconsole.log('B');`,
			expected: `\nconsole.log('B');`,
		},
		{
			name:     "double quotes without semicolon",
			input:    `"use strict"\nconsole.log('C');`,
			expected: `\nconsole.log('C');`,
		},
		{
			name:     "with leading whitespace",
			input:    `   "use strict";\nconsole.log('D');`,
			expected: `\nconsole.log('D');`,
		},
		{
			name:     "no use strict",
			input:    `console.log('E');`,
			expected: `console.log('E');`,
		},
		{
			name:     "use strict in middle",
			input:    `var x = 1;\n"use strict";\nconsole.log('F');`,
			expected: `var x = 1;\n"use strict";\nconsole.log('F');`,
		},
		{
			name:     "empty string",
			input:    ``,
			expected: ``,
		},
		{
			name:     "only use strict",
			input:    `"use strict";`,
			expected: ``,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := string(stripLeadingUseStrict([]byte(tt.input)))
			if result != tt.expected {
				t.Errorf("Input: %q, got: %q, want: %q", tt.input, result, tt.expected)
			}
		})
	}
}
