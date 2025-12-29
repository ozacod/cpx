package naming

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSafeIdent(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Simple name",
			input:    "myproject",
			expected: "myproject",
		},
		{
			name:     "Name with hyphens",
			input:    "my-project",
			expected: "my_project",
		},
		{
			name:     "Name with spaces",
			input:    "my project",
			expected: "my_project",
		},
		{
			name:     "Name starting with digit",
			input:    "123project",
			expected: "_123project",
		},
		{
			name:     "Empty name",
			input:    "",
			expected: "project",
		},
		{
			name:     "Special characters",
			input:    "my@project!test",
			expected: "my_project_test",
		},
		{
			name:     "Underscores preserved",
			input:    "my_project_name",
			expected: "my_project_name",
		},
		{
			name:     "Mixed case preserved",
			input:    "MyProject",
			expected: "MyProject",
		},
		{
			name:     "Unicode letters",
			input:    "прoject",
			expected: "прoject",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SafeIdent(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSafeIdentUpper(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Simple name",
			input:    "myproject",
			expected: "MYPROJECT",
		},
		{
			name:     "Name with hyphens",
			input:    "my-project",
			expected: "MY_PROJECT",
		},
		{
			name:     "Empty name",
			input:    "",
			expected: "PROJECT",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SafeIdentUpper(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSafeIdentTitle(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Lowercase name",
			input:    "myproject",
			expected: "Myproject",
		},
		{
			name:     "Already capitalized",
			input:    "MyProject",
			expected: "MyProject",
		},
		{
			name:     "Empty name",
			input:    "",
			expected: "Project",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SafeIdentTitle(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}
